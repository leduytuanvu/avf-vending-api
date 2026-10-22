package machinepaymentmethods

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNilDeps              = errors.New("machinepaymentmethods: nil dependencies")
	ErrInvalidMethodKey     = errors.New("machinepaymentmethods: invalid method key")
	ErrUnsupportedMethodKey = errors.New("machinepaymentmethods: method not supported by deployment")
)

var validMethodKeys = map[string]struct{}{
	"cash": {}, "momo": {}, "zalopay": {}, "vietqr": {}, "vnpay": {}, "shopeepay": {},
}

// MethodRow is one configured payment method for a machine.
type MethodRow struct {
	MethodKey string
	Enabled   bool
	SortOrder int32
}

// GetView is the admin read model for machine payment methods.
type GetView struct {
	MachineID           uuid.UUID
	Configured          bool
	Methods             []MethodRow
	DeploymentSupported []string
}

// ReplaceInput replaces the full machine payment method list.
type ReplaceInput struct {
	MachineID uuid.UUID
	Methods   []MethodRow
}

type overrideCacheEntry struct {
	override  platformpayments.MachineMethodOverride
	expiresAt time.Time
}

// Service manages per-machine payment method configuration.
type Service struct {
	pool          *pgxpool.Pool
	cfg           *config.Config
	reg           *platformpayments.Registry
	audit         compliance.EnterpriseRecorder
	overrideCache sync.Map
}

// NewService constructs the machine payment methods admin service.
func NewService(pool *pgxpool.Pool, cfg *config.Config, reg *platformpayments.Registry, audit compliance.EnterpriseRecorder) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("%w: pool", ErrNilDeps)
	}
	return &Service{pool: pool, cfg: cfg, reg: reg, audit: audit}, nil
}

func (s *Service) cachedOverride(machineID uuid.UUID) (platformpayments.MachineMethodOverride, bool) {
	if s == nil {
		return platformpayments.MachineMethodOverride{}, false
	}
	v, ok := s.overrideCache.Load(machineID)
	if !ok {
		return platformpayments.MachineMethodOverride{}, false
	}
	e, ok := v.(overrideCacheEntry)
	if !ok || time.Now().After(e.expiresAt) {
		s.overrideCache.Delete(machineID)
		return platformpayments.MachineMethodOverride{}, false
	}
	return e.override, true
}

func (s *Service) storeOverride(machineID uuid.UUID, override platformpayments.MachineMethodOverride) {
	if s == nil {
		return
	}
	s.overrideCache.Store(machineID, overrideCacheEntry{
		override:  override,
		expiresAt: time.Now().Add(30 * time.Second),
	})
}

func (s *Service) invalidateOverride(machineID uuid.UUID) {
	if s == nil {
		return
	}
	s.overrideCache.Delete(machineID)
}

// OverrideForMachine loads per-machine narrowing for bootstrap/commerce resolution. Fail-open on error.
func (s *Service) OverrideForMachine(ctx context.Context, machineID uuid.UUID) platformpayments.MachineMethodOverride {
	if s == nil || machineID == uuid.Nil {
		return platformpayments.MachineMethodOverride{}
	}
	if cached, ok := s.cachedOverride(machineID); ok {
		return cached
	}
	q := db.New(s.pool)
	rows, err := q.ListMachinePaymentMethods(ctx, machineID)
	if err != nil {
		return platformpayments.MachineMethodOverride{}
	}
	if len(rows) == 0 {
		empty := platformpayments.MachineMethodOverride{}
		s.storeOverride(machineID, empty)
		return empty
	}
	enabled := map[string]bool{}
	for _, row := range rows {
		if !row.Enabled {
			continue
		}
		key := platformpayments.NormalizeProviderKey(row.MethodKey)
		if key != "" {
			enabled[key] = true
		}
	}
	out := platformpayments.MachineMethodOverride{Configured: true, Enabled: enabled}
	s.storeOverride(machineID, out)
	return out
}

// DeploymentSupported returns method keys the deployment can offer (cash + wired QR providers).
func (s *Service) DeploymentSupported() []string {
	out := []string{"cash"}
	if s == nil || s.reg == nil {
		return out
	}
	out = append(out, platformpayments.EnabledSessionCreatableProviders(
		platformpayments.ResolveMachinePaymentMethods(s.cfg, s.reg, nil),
	)...)
	// Include wired providers even when session not creatable (for admin UI).
	for _, k := range platformpayments.EnabledWiredProviders(s.cfg, s.reg) {
		if k == "" {
			continue
		}
		found := false
		for _, existing := range out {
			if existing == k {
				found = true
				break
			}
		}
		if !found {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Get returns the configured methods and deployment-supported keys for a machine.
func (s *Service) Get(ctx context.Context, machineID uuid.UUID) (GetView, error) {
	if s == nil || machineID == uuid.Nil {
		return GetView{}, ErrNilDeps
	}
	q := db.New(s.pool)
	rows, err := q.ListMachinePaymentMethods(ctx, machineID)
	if err != nil {
		return GetView{}, err
	}
	methods := make([]MethodRow, 0, len(rows))
	for _, row := range rows {
		methods = append(methods, MethodRow{
			MethodKey: strings.TrimSpace(row.MethodKey),
			Enabled:   row.Enabled,
			SortOrder: row.SortOrder,
		})
	}
	return GetView{
		MachineID:           machineID,
		Configured:          len(methods) > 0,
		Methods:             methods,
		DeploymentSupported: s.DeploymentSupported(),
	}, nil
}

// Replace stores the machine payment method list, narrowing to deployment-supported keys only.
func (s *Service) Replace(ctx context.Context, in ReplaceInput) (GetView, error) {
	if s == nil || in.MachineID == uuid.Nil {
		return GetView{}, ErrNilDeps
	}
	supported := map[string]bool{}
	for _, k := range s.DeploymentSupported() {
		supported[platformpayments.NormalizeProviderKey(k)] = true
	}
	filtered := make([]MethodRow, 0, len(in.Methods))
	seen := map[string]bool{}
	for _, m := range in.Methods {
		key := platformpayments.NormalizeProviderKey(m.MethodKey)
		if key == "" {
			return GetView{}, ErrInvalidMethodKey
		}
		if _, ok := validMethodKeys[key]; !ok {
			return GetView{}, ErrInvalidMethodKey
		}
		if !supported[key] {
			return GetView{}, ErrUnsupportedMethodKey
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		filtered = append(filtered, MethodRow{
			MethodKey: key,
			Enabled:   m.Enabled,
			SortOrder: m.SortOrder,
		})
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].SortOrder != filtered[j].SortOrder {
			return filtered[i].SortOrder < filtered[j].SortOrder
		}
		return filtered[i].MethodKey < filtered[j].MethodKey
	})

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return GetView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)
	if err := q.DeleteMachinePaymentMethods(ctx, in.MachineID); err != nil {
		return GetView{}, err
	}
	for i, m := range filtered {
		sortOrder := m.SortOrder
		if sortOrder == 0 {
			sortOrder = int32(i)
		}
		if _, err := q.InsertMachinePaymentMethod(ctx, db.InsertMachinePaymentMethodParams{
			MachineID: in.MachineID,
			MethodKey: m.MethodKey,
			Enabled:   m.Enabled,
			SortOrder: sortOrder,
		}); err != nil {
			return GetView{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return GetView{}, err
	}
	s.invalidateOverride(in.MachineID)
	return s.Get(ctx, in.MachineID)
}
