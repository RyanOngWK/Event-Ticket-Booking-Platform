package payment

import (
	"context"
	"fmt"
	"strings"

	"github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/webhook"
)

type StripeGateway struct {
	client *stripe.Client
}

func NewStripeGateway(secretKey string) (*StripeGateway, error) {
	if !strings.HasPrefix(secretKey, "sk_test_") {
		return nil, fmt.Errorf("Stripe checkout simulation only accepts a test secret key (sk_test_)")
	}
	return &StripeGateway{client: stripe.NewClient(secretKey)}, nil
}

func (g *StripeGateway) CreateIntent(ctx context.Context, reservationID string, amount int64, currency string) (Intent, error) {
	params := &stripe.PaymentIntentCreateParams{
		Amount:                    stripe.Int64(amount),
		Currency:                  stripe.String(strings.ToLower(currency)),
		AllowedPaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Metadata: map[string]string{
			"reservation_id": reservationID,
		},
	}
	params.SetIdempotencyKey("reservation:" + reservationID)

	intent, err := g.client.V1PaymentIntents.Create(ctx, params)
	if err != nil {
		return Intent{}, fmt.Errorf("create Stripe PaymentIntent: %w", err)
	}
	return Intent{ID: intent.ID, ClientKey: intent.ClientSecret}, nil
}

func (g *StripeGateway) CancelIntent(ctx context.Context, intentID string) error {
	_, err := g.client.V1PaymentIntents.Cancel(ctx, intentID, &stripe.PaymentIntentCancelParams{})
	return err
}

func (g *StripeGateway) RefundIntent(ctx context.Context, intentID, idempotencyKey string) error {
	params := &stripe.RefundCreateParams{PaymentIntent: stripe.String(intentID)}
	params.SetIdempotencyKey(idempotencyKey)
	_, err := g.client.V1Refunds.Create(ctx, params)
	return err
}

func VerifyWebhook(payload []byte, signature, secret string) (stripe.Event, error) {
	return webhook.ConstructEventWithOptions(payload, signature, secret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
}
