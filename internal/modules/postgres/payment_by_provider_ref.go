package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PaymentByProviderRef is the payment + order machine binding resolved from a PSP provider_reference.
type PaymentByProviderRef struct {
	PaymentID         uuid.UUID
	OrderID           uuid.UUID
	Provider          string
	State             string
	AmountMinor       int64
	Currency          string
	ProviderReference string
	MachineID         uuid.UUID
}

const getPaymentByProviderReferenceSQL = `
SELECT p.id AS payment_id, p.order_id, p.provider, p.state, p.amount_minor, p.currency,
       pa.provider_reference, o.machine_id
FROM payment_attempts pa
JOIN payments p ON p.id = pa.payment_id
JOIN orders o ON o.id = p.order_id
WHERE pa.provider_reference = $1
ORDER BY pa.created_at DESC
LIMIT 1
`

// GetPaymentByProviderReference looks up the latest payment attempt for a PSP provider_reference.
func (s *Store) GetPaymentByProviderReference(ctx context.Context, providerReference string) (PaymentByProviderRef, error) {
	var out PaymentByProviderRef
	if s == nil || s.pool == nil {
		return out, errors.New("postgres: store not configured")
	}
	err := s.pool.QueryRow(ctx, getPaymentByProviderReferenceSQL, providerReference).Scan(
		&out.PaymentID,
		&out.OrderID,
		&out.Provider,
		&out.State,
		&out.AmountMinor,
		&out.Currency,
		&out.ProviderReference,
		&out.MachineID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, pgx.ErrNoRows
		}
		return out, err
	}
	return out, nil
}

const getPaymentProviderRefByPaymentIDSQL = `
SELECT p.provider, COALESCE(pa.provider_reference, '')
FROM payments p
LEFT JOIN LATERAL (
  SELECT provider_reference
  FROM payment_attempts
  WHERE payment_id = p.id
  ORDER BY created_at DESC
  LIMIT 1
) pa ON true
WHERE p.id = $1
`

// ResolvePaymentProviderAttempt returns provider key and latest provider_reference for a payment id.
func (s *Store) ResolvePaymentProviderAttempt(ctx context.Context, paymentID uuid.UUID) (provider string, providerReference string, err error) {
	if s == nil || s.pool == nil {
		return "", "", errors.New("postgres: store not configured")
	}
	err = s.pool.QueryRow(ctx, getPaymentProviderRefByPaymentIDSQL, paymentID).Scan(&provider, &providerReference)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", pgx.ErrNoRows
	}
	return provider, providerReference, err
}
