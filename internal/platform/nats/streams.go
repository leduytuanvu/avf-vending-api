package nats

import (
	"errors"
	"fmt"
	"time"

	natssrv "github.com/nats-io/nats.go"
)

// EnsureInternalStreams creates or updates the internal outbox and DLQ streams.
func EnsureInternalStreams(js natssrv.JetStreamContext) error {
	if js == nil {
		return fmt.Errorf("nats: nil jetstream context")
	}
	if err := ensureStream(js, streamSpec{
		Name:       StreamOutbox,
		Subjects:   []string{SubjectPatternOutbox},
		Retention:  natssrv.InterestPolicy,
		Duplicates: 2 * time.Minute,
		MaxAge:     7 * 24 * time.Hour,
		Discard:    natssrv.DiscardOld,
		Storage:    natssrv.FileStorage,
	}); err != nil {
		return err
	}
	return ensureStream(js, streamSpec{
		Name:       StreamDLQ,
		Subjects:   []string{SubjectPatternDLQ},
		Retention:  natssrv.LimitsPolicy,
		MaxAge:     30 * 24 * time.Hour,
		Discard:    natssrv.DiscardOld,
		Storage:    natssrv.FileStorage,
		Duplicates: time.Minute,
	})
}

type streamSpec struct {
	Name       string
	Subjects   []string
	Retention  natssrv.RetentionPolicy
	Duplicates time.Duration
	MaxAge     time.Duration
	MaxBytes   int64
	Discard    natssrv.DiscardPolicy
	Storage    natssrv.StorageType
}

func ensureStream(js natssrv.JetStreamContext, spec streamSpec) error {
	cfg := &natssrv.StreamConfig{
		Name:       spec.Name,
		Subjects:   spec.Subjects,
		Retention:  spec.Retention,
		Duplicates: spec.Duplicates,
		MaxAge:     spec.MaxAge,
		Discard:    spec.Discard,
		Storage:    spec.Storage,
	}
	if spec.MaxBytes > 0 {
		cfg.MaxBytes = spec.MaxBytes
	}
	_, err := js.StreamInfo(spec.Name)
	if err == nil {
		_, err = js.UpdateStream(cfg)
		if err != nil {
			return fmt.Errorf("nats: update stream %s: %w", spec.Name, err)
		}
		return nil
	}
	if !errors.Is(err, natssrv.ErrStreamNotFound) {
		return fmt.Errorf("nats: stream info %s: %w", spec.Name, err)
	}
	_, err = js.AddStream(cfg)
	if err != nil {
		return fmt.Errorf("nats: add stream %s: %w", spec.Name, err)
	}
	return nil
}
