package httpserver

import (
	"context"
	"encoding/json"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func recordMachineInventoryAudit(
	ctx context.Context,
	app *api.HTTPApplication,
	machineID uuid.UUID,
	operatorSessionID *uuid.UUID,
	action string,
	meta map[string]any,
) {
	if app == nil || app.EnterpriseAudit == nil || machineID == uuid.Nil {
		return
	}
	md, err := json.Marshal(meta)
	if err != nil {
		md = []byte("{}")
	}
	md = compliance.SanitizeJSONBytes(md)
	mid := machineID.String()
	at, aid := compliance.ActorUser, ""
	if p, ok := auth.PrincipalFromContext(ctx); ok {
		at, aid = p.Actor()
	}
	_ = app.EnterpriseAudit.Record(ctx, compliance.EnterpriseAuditRecord{
		ActorType:    at,
		ActorID:      stringPtrOrNil(aid),
		Action:       action,
		ResourceType: "fleet.machine",
		ResourceID:   &mid,
		MachineID:    &machineID,
		Metadata:     md,
		Outcome:      compliance.OutcomeSuccess,
	})
	if app.TelemetryStore == nil || operatorSessionID == nil || *operatorSessionID == uuid.Nil {
		return
	}
	pool := app.TelemetryStore.Pool()
	if pool == nil {
		return
	}
	attrMeta, _ := json.Marshal(map[string]any{
		"action": action,
	})
	_, _ = db.New(pool).InsertMachineActionAttribution(ctx, db.InsertMachineActionAttributionParams{
		OperatorSessionID: pgtype.UUID{Bytes: *operatorSessionID, Valid: true},
		MachineID:         machineID,
		ActionOriginType:  operator.ActionOriginOperatorSession,
		ResourceType:      "fleet.machine",
		ResourceID:        mid,
		Metadata:          attrMeta,
	})
}
