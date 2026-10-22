// Command qr-latency-avf000195 measures CreateQuote + CreatePaymentSession over machine gRPC.
//
// Default providers are sandbox-family only (psp_grpc_int, sandbox). Production live MoMo is refused
// unless PAYMENT_ENV is explicitly not "live". Set AVF_SKIP=1 to print identity and exit 0.
// If AVF_PRODUCT_ID is empty, GetSaleCatalog picks one in-stock product+slot (still needs a token).
package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultMachineCode = "AVF000195"
	defaultMachineID   = "01a0a7e5-3c68-7895-b526-bcb6504bccfb"
	defaultBudgetMS    = 3000
	defaultTarget      = "localhost:9090"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	machineCode := envOr("AVF_MACHINE_CODE", defaultMachineCode)
	wantMachineID := envOr("AVF_MACHINE_ID", defaultMachineID)
	if os.Getenv("AVF_SKIP") == "1" {
		fmt.Printf("skip live call machine=%s id=%s\n", machineCode, wantMachineID)
		return nil
	}

	token := strings.TrimSpace(os.Getenv("MACHINE_ACCESS_TOKEN"))
	if token == "" {
		return errors.New("MACHINE_ACCESS_TOKEN required (or AVF_SKIP=1)")
	}
	productID := strings.TrimSpace(os.Getenv("AVF_PRODUCT_ID"))
	slotFromEnv := strings.TrimSpace(os.Getenv("AVF_SLOT_INDEX")) != ""
	slotIndex := int32(envInt("AVF_SLOT_INDEX", 1))
	providers := splitCSV(envOr("AVF_PROVIDERS", "psp_grpc_int,sandbox"))
	budgetMS := envInt("AVF_LATENCY_BUDGET_MS", defaultBudgetMS)
	target := envOr("AVF_GRPC_TARGET", defaultTarget)
	paymentEnv := strings.ToLower(strings.TrimSpace(envOr("PAYMENT_ENV", "sandbox")))

	if err := forbidProdLiveMoMo(target, paymentEnv, providers); err != nil {
		return err
	}

	gotMachineID, err := decodeMachineIDFromJWT(token)
	if err != nil {
		return fmt.Errorf("decode MACHINE_ACCESS_TOKEN: %w", err)
	}
	if gotMachineID != wantMachineID {
		return fmt.Errorf("jwt machine_id=%s does not match expected %s (AVF000195)", gotMachineID, wantMachineID)
	}
	fmt.Printf("identity machine=%s id=%s\n", machineCode, gotMachineID)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	conn, err := dialMachineGRPC(ctx, target)
	if err != nil {
		return err
	}
	defer conn.Close()
	cli := machinev1.NewMachineCommerceServiceClient(conn)
	md := metadata.New(map[string]string{"authorization": "Bearer " + token})
	rpcCtx := metadata.NewOutgoingContext(ctx, md)

	if productID == "" {
		catCli := machinev1.NewMachineCatalogServiceClient(conn)
		includeImages := false
		catalog, err := catCli.GetSaleCatalog(rpcCtx, &machinev1.GetCatalogSnapshotRequest{
			IncludeUnavailable: false,
			IncludeImages:      &includeImages,
		})
		if err != nil {
			return fmt.Errorf("GetSaleCatalog: %w", err)
		}
		pid, slot, ok := pickFirstInStockCatalogLine(catalog.GetSnapshot().GetItems())
		if !ok {
			return errors.New("GetSaleCatalog returned no in-stock product (set AVF_PRODUCT_ID)")
		}
		productID = pid
		if !slotFromEnv {
			slotIndex = slot
		}
		fmt.Printf("auto catalog product_id=%s slot_index=%d\n", productID, slotIndex)
	}

	quoteStarted := time.Now()
	quote, err := cli.CreateQuote(rpcCtx, &machinev1.CreateQuoteRequest{
		Context:       idemCtx("quote-" + uuid.NewString()),
		Currency:      "VND",
		PaymentMethod: "payment_ui_resolved",
		Lines: []*machinev1.QuoteLineInput{{
			ProductId: productID,
			Quantity:  1,
			Slot:      &machinev1.SlotSelection{SlotIndex: &slotIndex},
		}},
	})
	quoteMS := time.Since(quoteStarted).Milliseconds()
	if err != nil {
		fmt.Printf("CreateQuote duration_ms=%d error=%v\n", quoteMS, err)
		return err
	}
	fmt.Printf("CreateQuote duration_ms=%d quote_id=%s payable_minor=%d\n", quoteMS, quote.GetQuoteId(), quote.GetPayableMinor())
	if strings.TrimSpace(quote.GetQuoteId()) == "" {
		return errors.New("quote response missing quote_id")
	}

	failed := false
	for _, provider := range providers {
		payStarted := time.Now()
		session, err := cli.CreatePaymentSession(rpcCtx, &machinev1.CreatePaymentSessionRequest{
			Context:          idemCtx("pay-" + uuid.NewString()),
			QuoteId:          ptr(quote.GetQuoteId()),
			ClientCheckoutId: ptr(uuid.NewString()),
			Provider:         provider,
			PaymentState:     "created",
			AmountMinor:      quote.GetPayableMinor(),
			Currency:         "VND",
		})
		payMS := time.Since(payStarted).Milliseconds()
		if err != nil {
			fmt.Printf("CreatePaymentSession provider=%s duration_ms=%d error=%v\n", provider, payMS, err)
			failed = true
			continue
		}
		qr := strings.TrimSpace(session.GetQrPayloadOrUrl())
		fmt.Printf("CreatePaymentSession provider=%s duration_ms=%d qr_len=%d payment_id=%s\n",
			provider, payMS, len(qr), session.GetPaymentId())
		if qr == "" {
			fmt.Printf("FAIL missing QR for %s\n", provider)
			failed = true
			continue
		}
		if isSandboxFamilyProvider(provider) && payMS > int64(budgetMS) {
			fmt.Printf("FAIL budget %dms exceeded for %s\n", budgetMS, provider)
			failed = true
		}
	}
	if failed {
		return errors.New("one or more payment sessions failed latency or QR checks")
	}
	return nil
}

func pickFirstInStockCatalogLine(items []*machinev1.CatalogSlotItem) (productID string, slotIndex int32, ok bool) {
	for _, it := range items {
		if it == nil {
			continue
		}
		pid := strings.TrimSpace(it.GetProductId())
		if pid == "" || !it.GetIsAvailable() || it.GetAvailableQuantity() <= 0 {
			continue
		}
		return pid, it.GetSlotIndex(), true
	}
	return "", 0, false
}

func forbidProdLiveMoMo(target, paymentEnv string, providers []string) error {
	if !isProductionGRPCTarget(target) {
		return nil
	}
	if paymentEnv != "" && paymentEnv != "live" {
		return nil
	}
	for _, p := range providers {
		key := strings.ToLower(strings.TrimSpace(p))
		if key == "momo" || key == "zalopay" || key == "vietqr" || key == "vnpay" || key == "shopeepay" {
			return fmt.Errorf("refusing live PSP %q against production target %s (set PAYMENT_ENV=sandbox or use staging/local)", key, target)
		}
	}
	return nil
}

func isProductionGRPCTarget(target string) bool {
	h := strings.ToLower(strings.TrimSpace(target))
	h = strings.TrimPrefix(h, "grpcs://")
	h = strings.TrimPrefix(h, "grpc://")
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	return strings.Contains(h, "machine-api.ldtv.dev") || strings.Contains(h, "api.ldtv.dev")
}

func isSandboxFamilyProvider(p string) bool {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "sandbox", "psp_grpc_int", "mock", "test", "psp_fixture", "dev":
		return true
	default:
		return false
	}
}

func decodeMachineIDFromJWT(raw string) (string, error) {
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return "", errors.New("token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return "", err
		}
	}
	var claims struct {
		Sub        string   `json:"sub"`
		MachineID  string   `json:"machine_id"`
		MachineIDs []string `json:"machine_ids"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", err
	}
	if id := strings.TrimSpace(claims.MachineID); id != "" {
		if _, err := uuid.Parse(id); err == nil {
			return id, nil
		}
	}
	for _, id := range claims.MachineIDs {
		id = strings.TrimSpace(id)
		if _, err := uuid.Parse(id); err == nil {
			return id, nil
		}
	}
	sub := strings.TrimSpace(claims.Sub)
	sub = strings.TrimPrefix(sub, "machine:")
	if _, err := uuid.Parse(sub); err == nil {
		return sub, nil
	}
	return "", errors.New("jwt missing machine_id")
}

func dialMachineGRPC(ctx context.Context, target string) (*grpc.ClientConn, error) {
	raw := strings.TrimSpace(target)
	useTLS := strings.HasPrefix(strings.ToLower(raw), "grpcs://") || strings.HasPrefix(strings.ToLower(raw), "https://")
	addr := raw
	addr = strings.TrimPrefix(addr, "grpcs://")
	addr = strings.TrimPrefix(addr, "grpc://")
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimPrefix(addr, "http://")
	opts := []grpc.DialOption{grpc.WithBlock()}
	if useTLS {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	return grpc.DialContext(ctx, addr, opts...)
}

func idemCtx(key string) *machinev1.IdempotencyContext {
	return &machinev1.IdempotencyContext{
		IdempotencyKey:  key,
		ClientEventId:   key,
		ClientCreatedAt: timestamppb.Now(),
	}
}

func ptr(s string) *string { return &s }

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
