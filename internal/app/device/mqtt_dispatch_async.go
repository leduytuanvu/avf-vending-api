package device

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type mqttPublishJob struct {
	attemptID   uuid.UUID
	machineID   uuid.UUID
	wireBytes   []byte
	ackDeadline time.Time
}

// StartBackgroundWorkers runs async MQTT publish and stale-pending reclaim loops (API process).
func (d *MQTTCommandDispatcher) StartBackgroundWorkers(ctx context.Context) {
	if d == nil {
		return
	}
	if d.publishQueue == nil {
		d.publishQueue = make(chan mqttPublishJob, 512)
	}
	go d.runMQTTPublishWorker(ctx)
	go d.runMQTTReclaimPending(ctx)
}

func (d *MQTTCommandDispatcher) enqueueMQTTPublish(job mqttPublishJob) {
	if d == nil || d.publishQueue == nil {
		return
	}
	if !d.beginMQTTAttemptPublish(job.attemptID) {
		return
	}
	select {
	case d.publishQueue <- job:
	default:
		// Backpressure: rely on reclaim after publish budget; release in-flight slot.
		d.endMQTTAttemptPublish(job.attemptID)
	}
}

func (d *MQTTCommandDispatcher) runMQTTPublishWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-d.publishQueue:
			if !ok {
				return
			}
			d.executeMQTTTransportPublish(ctx, job)
		}
	}
}

func (d *MQTTCommandDispatcher) executeMQTTTransportPublish(ctx context.Context, job mqttPublishJob) {
	if d == nil || d.store == nil || d.pub == nil {
		d.endMQTTAttemptPublish(job.attemptID)
		return
	}
	defer d.endMQTTAttemptPublish(job.attemptID)
	if !d.store.IsMQTTDispatchAttemptPending(ctx, job.attemptID) {
		return
	}
	pubCtx, cancel := context.WithTimeout(ctx, mqttPublishTimeoutPerTry)
	defer cancel()
	var publishErr error
	for attempt := 0; attempt < d.publishMaxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(d.retryBackoff)
		}
		publishErr = d.pub.PublishDeviceDispatch(pubCtx, job.machineID, job.wireBytes)
		if publishErr == nil {
			break
		}
	}
	if publishErr != nil {
		_ = d.store.MarkMQTTDispatchAttemptPublishFailed(ctx, job.attemptID, "mqtt_publish: "+truncateErr(publishErr, 400))
		return
	}
	ackDeadline := job.ackDeadline
	if ackDeadline.IsZero() {
		ackDeadline = time.Now().Add(d.ackTimeout)
	}
	_ = d.store.MarkMQTTDispatchAttemptSent(ctx, job.attemptID, ackDeadline)
}

func (d *MQTTCommandDispatcher) runMQTTReclaimPending(ctx context.Context) {
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.reclaimStalePendingMQTT(ctx)
		}
	}
}

func (d *MQTTCommandDispatcher) reclaimStalePendingMQTT(ctx context.Context) {
	if d == nil || d.store == nil {
		return
	}
	rows, err := d.store.ListStalePendingMQTTDispatchAttempts(ctx, d.mqttPublishReclaimMinAge(), 32)
	if err != nil {
		return
	}
	for _, row := range rows {
		d.enqueueMQTTPublish(mqttPublishJob{
			attemptID:   row.AttemptID,
			machineID:   row.MachineID,
			wireBytes:   row.Payload,
			ackDeadline: time.Now().Add(d.ackTimeout),
		})
	}
}
