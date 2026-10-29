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
	"github.com/jackc/pgx/v5/pgconn"
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
			results = append(results, &machinev1.OfflineEventResult{
				OfflineSequence: seq,
				Status:          machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REJECTED,
				Reason:          fmt.Sprintf("invalid abandoned offline_sequence %d", seq),
			})
			continue
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
		row, err := insertAbandonedOfflineEvent(ctx, q, machineID, seq, clientEventID, reason, now)
		if err != nil && isMachineOfflineUniqueViolation(err) && clientEventID != "" {
			logMachineOfflineInsertError(offlineEventPersistenceContext{
				MachineID:       machineID,
				OfflineSequence: seq,
				ClientEventID:   clientEventID,
				EventType:       "stream.abandoned",
			}, err)
			row, err = insertAbandonedOfflineEvent(ctx, q, machineID, seq, "", reason, now)
		}
		if err != nil {
			logMachineOfflineInsertError(offlineEventPersistenceContext{
				MachineID:       machineID,
				OfflineSequence: seq,
				ClientEventID:   clientEventID,
				EventType:       "stream.abandoned",
			}, err)
			results = append(results, &machinev1.OfflineEventResult{
				OfflineSequence: seq,
				Status:          machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REJECTED,
				Reason:          abandonedOfflineInsertFailedReason(seq, err),
			})
			continue
		}
		if err := advanceAbandonedCursorIfContiguous(ctx, q, machineID, streamName, cursor, seq); err != nil {
			logMachineOfflineInsertError(offlineEventPersistenceContext{
				MachineID:       machineID,
				OfflineSequence: seq,
				ClientEventID:   clientEventID,
				EventType:       "stream.abandoned",
			}, err)
		}
		status := machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED
		resultReason := "offline sequence abandoned by device"
		if !row.Inserted {
			status = machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_REPLAYED
			resultReason = "offline sequence already synced"
		}
		results = append(results, &machinev1.OfflineEventResult{
			OfflineSequence: seq,
			Status:          status,
			Reason:          resultReason,
		})
	}
	return results, nil
}

func insertAbandonedOfflineEvent(
	ctx context.Context,
	q *db.Queries,
	machineID uuid.UUID,
	seq int64,
	clientEventID string,
	reason string,
	now time.Time,
) (db.InsertMachineOfflineEventRow, error) {
	return q.InsertMachineOfflineEvent(ctx, db.InsertMachineOfflineEventParams{
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
	})
}

func advanceAbandonedCursorIfContiguous(
	ctx context.Context,
	q *db.Queries,
	machineID uuid.UUID,
	streamName string,
	cursor *db.MachineSyncCursor,
	seq int64,
) error {
	if cursor == nil || seq != cursor.LastSequence+1 {
		return nil
	}
	if _, err := q.UpsertMachineSyncCursor(ctx, db.UpsertMachineSyncCursorParams{
		MachineID:    machineID,
		StreamName:   streamName,
		LastSequence: seq,
	}); err != nil {
		return errors.New("offline cursor update failed after abandon")
	}
	cursor.LastSequence = seq
	return nil
}

func isMachineOfflineUniqueViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505"
}

func abandonedOfflineInsertFailedReason(seq int64, err error) string {
	if err == nil {
		return fmt.Sprintf("abandoned offline_sequence %d insert failed", seq)
	}
	return fmt.Sprintf("abandoned offline_sequence %d insert failed: %s", seq, offlineEventInsertFailedReason(err))
}
