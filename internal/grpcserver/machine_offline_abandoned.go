package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgjson"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
)

func (s *machineOfflineSyncServer) offlineSequenceGapTolerant() bool {
	if s.deps.Config == nil {
		return true
	}
	return s.deps.Config.OfflineSync.GapTolerant
}

func (s *machineOfflineSyncServer) applyAbandonedOfflineSequences(
	ctx context.Context,
	q *db.Queries,
	machineID uuid.UUID,
	streamName string,
	cursor *db.MachineSyncCursor,
	abandoned []*machinev1.AbandonedOfflineSequence,
) ([]*machinev1.OfflineEventResult, error) {
	if len(abandoned) == 0 {
		return nil, nil
	}
	sorted := append([]*machinev1.AbandonedOfflineSequence(nil), abandoned...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].GetOfflineSequence() < sorted[j].GetOfflineSequence()
	})
	results := make([]*machinev1.OfflineEventResult, 0, len(sorted))
	now := time.Now().UTC()
	for _, entry := range sorted {
		if entry == nil {
			continue
		}
		seq := entry.GetOfflineSequence()
		if seq <= 0 {
			return nil, fmt.Errorf("invalid abandoned offline_sequence %d", seq)
		}
		if seq <= cursor.LastSequence {
			results = append(results, &machinev1.OfflineEventResult{
				OfflineSequence: seq,
				Status:          machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED,
				Reason:          "offline sequence already synced",
			})
			continue
		}
		reason := strings.TrimSpace(entry.GetReason())
		if reason == "" {
			reason = "device_dead_letter"
		}
		clientEventID := strings.TrimSpace(entry.GetClientEventId())
		if _, err := q.InsertMachineOfflineEvent(ctx, db.InsertMachineOfflineEventParams{
			MachineID:        machineID,
			OfflineSequence:  seq,
			EventType:        "stream.abandoned",
			EventID:          "",
			ClientEventID:    clientEventID,
			OccurredAt:       now,
			Payload:          pgjson.RequiredString([]byte("{}")),
			ProcessingStatus: "rejected",
			ProcessingError:  reason,
			IdempotencyKey:   "",
		}); err != nil {
			return nil, fmt.Errorf("abandoned offline_sequence %d insert failed: %w", seq, err)
		}
		cursor.LastSequence = seq
		if _, err := q.UpsertMachineSyncCursor(ctx, db.UpsertMachineSyncCursorParams{
			MachineID:    machineID,
			StreamName:   streamName,
			LastSequence: seq,
		}); err != nil {
			return nil, errors.New("offline cursor update failed after abandon")
		}
		results = append(results, &machinev1.OfflineEventResult{
			OfflineSequence: seq,
			Status:          machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED,
			Reason:          "offline sequence abandoned by device",
		})
	}
	return results, nil
}
