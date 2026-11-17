package payment

import "context"

type Intent struct {
	ID        string
	ClientKey string
}

// Gateway isolates Stripe from reservation state transitions.
type Gateway interface {
	CreateIntent(ctx context.Context, reservationID string, amount int64, currency string) (Intent, error)
	CancelIntent(ctx context.Context, intentID string) error
	RefundIntent(ctx context.Context, intentID, idempotencyKey string) error
}
