package legacypayment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateSessionRequest is the legacy PaymentSession create body.
type CreateSessionRequest struct {
	MachineCode     string `json:"machine_code"`
	PaymentAmount   int64  `json:"payment_amount"`
	PaymentMethodID string `json:"payment_method_id"`
	OrderCode       string `json:"order_code"`
	Currency        string `json:"currency"`
	ProductID       string `json:"product_id"`
	SlotIndex       *int32 `json:"slot_index"`
	CabinetCode     string `json:"cabinet_code"`
	SlotCode        string `json:"slot_code"`
	StoreID         string `json:"store_id"`
}

// CreateSessionResult is returned to the HTTP facade.
type CreateSessionResult struct {
	QRCodeURL         string
	PaymentProviderID string
	OrderCode         string
	PaymentRefcode    string
}

// QueryStatusRequest is the legacy payment query body.
type QueryStatusRequest struct {
	PaymentMethodID string `json:"payment_method_id"`
	OrderCode       string `json:"order_code"`
}

// Service bridges legacy /payment-service/payment HTTP into commerce + PSP adapters.
type Service struct {
	pool     *pgxpool.Pool
	commerce *appcommerce.Service
	orders   domaincommerce.OrderVendWorkflow
	registry *platformpayments.Registry
	cfg      *config.Config
}

// NewService wires a legacy payment facade.
func NewService(
	pool *pgxpool.Pool,
	commerce *appcommerce.Service,
	orders domaincommerce.OrderVendWorkflow,
	registry *platformpayments.Registry,
	cfg *config.Config,
) *Service {
	return &Service{
		pool:     pool,
		commerce: commerce,
		orders:   orders,
		registry: registry,
		cfg:      cfg,
	}
}

// CreateSession resolves the machine, creates an order, and starts a PSP session.
func (s *Service) CreateSession(ctx context.Context, in CreateSessionRequest) (CreateSessionResult, error) {
	var out CreateSessionResult
	if s == nil || s.pool == nil || s.commerce == nil || s.orders == nil || s.cfg == nil {
		return out, errors.New("legacy payment not configured")
	}
	machineCode := strings.TrimSpace(in.MachineCode)
	orderCode := strings.TrimSpace(in.OrderCode)
	method := strings.TrimSpace(strings.ToLower(in.PaymentMethodID))
	if machineCode == "" || orderCode == "" || method == "" {
		return out, errors.New("machine_code, order_code, and payment_method_id are required")
	}
	if in.PaymentAmount <= 0 {
		return out, errors.New("payment_amount must be positive")
	}
	cur := strings.ToUpper(strings.TrimSpace(in.Currency))
	if cur == "" {
		cur = "VND"
	}

	mach, err := db.New(s.pool).GetMachineByCode(ctx, machineCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, errors.New("machine not found")
		}
		return out, err
	}

	preferred := ""
	clientProvider := method
	providerIDForResponse := method
	if method == "vietqr" {
		clientProvider = "zalopay"
		providerIDForResponse = "zalopay"
		preferred = "vietqr"
	}

	productID, slotIndex, err := s.resolveProductSlot(ctx, mach.ID, in)
	if err != nil {
		return out, err
	}

	ordRes, err := s.orders.CreateOrderWithVendSession(ctx, domaincommerce.CreateOrderVendInput{
		MachineID:      mach.ID,
		ProductID:      productID,
		SlotIndex:      slotIndex,
		Currency:       cur,
		SubtotalMinor:  in.PaymentAmount,
		TaxMinor:       0,
		TotalMinor:     in.PaymentAmount,
		IdempotencyKey: orderCode,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	if err != nil {
		return out, err
	}

	topic := strings.TrimSpace(s.cfg.Commerce.PaymentOutboxTopic)
	evType := strings.TrimSpace(s.cfg.Commerce.PaymentOutboxEventType)
	aggType := strings.TrimSpace(s.cfg.Commerce.PaymentOutboxAggregateType)
	if topic == "" {
		topic = "commerce.payment"
	}
	if evType == "" {
		evType = "payment.session_created"
	}
	if aggType == "" {
		aggType = "payment"
	}

	sess, err := s.commerce.CreateMachinePaymentSession(ctx, appcommerce.CreateMachinePaymentSessionInput{
		OrderID:             ordRes.Order.ID,
		MachineID:           mach.ID,
		IdempotencyKey:      orderCode,
		ClientProvider:      clientProvider,
		ClientPayState:      "created",
		AmountMinor:         in.PaymentAmount,
		Currency:            cur,
		AppEnv:              s.cfg.AppEnv,
		OutboxTopic:         topic,
		OutboxEventType:     evType,
		OutboxAggregate:     aggType,
		MachineExternalCode: machineCode,
		StoreID:             strings.TrimSpace(in.StoreID),
		ProviderReference:   orderCode,
		PreferredMethod:     preferred,
	})
	if err != nil {
		return out, err
	}

	out.QRCodeURL = sess.QRPayloadOrURL
	if out.QRCodeURL == "" {
		out.QRCodeURL = sess.PaymentURL
	}
	out.PaymentProviderID = providerIDForResponse
	if sess.ProviderKey != "" && method != "vietqr" {
		out.PaymentProviderID = sess.ProviderKey
	}
	out.OrderCode = orderCode
	out.PaymentRefcode = ""
	return out, nil
}

func (s *Service) resolveProductSlot(ctx context.Context, machineID uuid.UUID, in CreateSessionRequest) (uuid.UUID, int32, error) {
	if pid := strings.TrimSpace(in.ProductID); pid != "" {
		id, err := uuid.Parse(pid)
		if err != nil || id == uuid.Nil {
			return uuid.Nil, 0, errors.New("invalid product_id")
		}
		slot := int32(0)
		if in.SlotIndex != nil {
			slot = *in.SlotIndex
		}
		return id, slot, nil
	}
	rows, err := db.New(s.pool).InventoryAdminListCurrentMachineSlotConfigsByMachine(ctx, machineID)
	if err != nil {
		return uuid.Nil, 0, err
	}
	cab := strings.TrimSpace(in.CabinetCode)
	slotCode := strings.TrimSpace(in.SlotCode)
	for _, row := range rows {
		if !row.ProductID.Valid {
			continue
		}
		pid := uuid.UUID(row.ProductID.Bytes)
		si := int32(0)
		if row.SlotIndex.Valid {
			si = row.SlotIndex.Int32
		}
		switch {
		case in.SlotIndex != nil && row.SlotIndex.Valid && row.SlotIndex.Int32 == *in.SlotIndex:
			return pid, si, nil
		case cab != "" && slotCode != "" &&
			strings.TrimSpace(row.CabinetCode) == cab &&
			strings.TrimSpace(row.SlotCode) == slotCode:
			return pid, si, nil
		}
	}
	return uuid.Nil, 0, errors.New("product_id or matching slot_index/cabinet_code+slot_code required")
}

// QueryStatus queries the PSP for payment status by order_code (provider_reference).
func (s *Service) QueryStatus(ctx context.Context, in QueryStatusRequest) (returnCode int, message string, err error) {
	if s == nil || s.registry == nil {
		return -1, "not configured", errors.New("legacy payment not configured")
	}
	method := strings.TrimSpace(strings.ToLower(in.PaymentMethodID))
	orderCode := strings.TrimSpace(in.OrderCode)
	if method == "" || orderCode == "" {
		return -1, "payment_method_id and order_code required", errors.New("invalid argument")
	}
	if method == "vietqr" {
		method = "zalopay"
	}
	prov := s.registry.Get(method)
	if prov == nil || !prov.SupportsQueryPaymentStatus() {
		return -1, "provider query not supported", errors.New("query not supported")
	}
	snap, qerr := prov.QueryPaymentStatus(ctx, domaincommerce.PaymentProviderLookup{
		Provider:          method,
		ProviderReference: orderCode,
	})
	if qerr != nil {
		return -1, qerr.Error(), qerr
	}
	switch strings.ToLower(strings.TrimSpace(snap.NormalizedState)) {
	case "captured":
		return 0, "success", nil
	case "pending", "authorized", "created":
		return 1, "pending", nil
	default:
		return -1, "failed", nil
	}
}

// CreateSuccessEnvelope builds the legacy create response with exact envelope keys.
func CreateSuccessEnvelope(data CreateSessionResult) map[string]any {
	return map[string]any{
		"code":    200,
		"message": "Thành công",
		"data": map[string]any{
			"qrCodeUrl":           data.QRCodeURL,
			"Payment_Provider_id": data.PaymentProviderID,
			"Order_code":          data.OrderCode,
			"Payment_Refcode":     data.PaymentRefcode,
		},
	}
}

// EncodeCreateSuccessJSON returns the legacy create envelope as JSON bytes (for tests).
func EncodeCreateSuccessJSON(data CreateSessionResult) ([]byte, error) {
	return json.Marshal(CreateSuccessEnvelope(data))
}
