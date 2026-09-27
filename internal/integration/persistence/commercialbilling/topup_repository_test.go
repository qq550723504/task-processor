package commercialbilling

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/commercial/billing"
)

func topUpMerchant(provider billing.PaymentProvider) billing.TopUpMerchant {
	product := "PAGE_PAY"
	if provider == billing.PaymentWeChat {
		product = "NATIVE"
	}
	return billing.TopUpMerchant{Provider: provider, Environment: "PRODUCTION", ProfileVersion: "v1", MerchantID: "merchant-1", AppID: "app-1", Product: product}
}
func TestTopUpOrderAndAttemptFreezeChannelAndExpiry(t *testing.T) {
	r := commercialRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	req := billing.CreateWalletTopUpOrderRequest{OrganizationID: "org-1", ActorID: "admin-1", Currency: "CNY", AmountMinor: 10000, IdempotencyKey: "key-1", Provider: billing.PaymentAlipay}
	first, err := r.CreateTopUpAttempt(ctx, req, topUpMerchant(req.Provider), now, now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	replay, err := r.CreateTopUpAttempt(ctx, req, topUpMerchant(req.Provider), now.Add(time.Minute), now.Add(time.Hour))
	if err != nil || replay.AttemptID != first.AttemptID || !replay.ExpiresAt.Equal(first.ExpiresAt) {
		t.Fatalf("replay %+v %v", replay, err)
	}
	req.Provider = billing.PaymentWeChat
	if _, err := r.CreateTopUpAttempt(ctx, req, topUpMerchant(req.Provider), now, now.Add(time.Hour)); !errors.Is(err, billing.ErrConflict) {
		t.Fatalf("changed channel: %v", err)
	}
	order, err := r.ReadOrder(ctx, "org-1", first.OrderID)
	if err != nil || order.Kind != billing.OrderWalletTopUp || order.Status != billing.OrderPending || order.Validate() != nil {
		t.Fatalf("order %+v %v", order, err)
	}
	first.CheckoutAdmittedAt = &now
	first.Phase = billing.TopUpAwaitingPayment
	saved, err := r.SaveTopUpAttempt(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	first.Phase = billing.TopUpClosedUnpaid
	if _, err := r.SaveTopUpAttempt(ctx, first); !errors.Is(err, billing.ErrConflict) {
		t.Fatalf("stale writer: %v", err)
	}
	saved.AmountMinor++
	if _, err := r.SaveTopUpAttempt(ctx, saved); !errors.Is(err, billing.ErrConflict) {
		t.Fatalf("changed immutable intent: %v", err)
	}
}

func TestTopUpInboxDurablyDeduplicatesAndSeparatesChannel(t *testing.T) {
	r := commercialRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	req := billing.CreateWalletTopUpOrderRequest{OrganizationID: "org-1", ActorID: "admin-1", Currency: "CNY", AmountMinor: 10000, IdempotencyKey: "key-1", Provider: billing.PaymentAlipay}
	attempt, err := r.CreateTopUpAttempt(ctx, req, topUpMerchant(req.Provider), now, now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	observation := billing.ProviderObservation{Merchant: attempt.Merchant, MerchantOrderID: attempt.MerchantOrderID, EventID: "event-1", Kind: "PAYMENT", State: "PAID", TradeID: "trade-1", Currency: "CNY", AmountMinor: 10000, OccurredAt: now, VerificationVersion: "v1"}
	if err := r.RecordTopUpObservation(ctx, observation); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordTopUpObservation(ctx, observation); err != nil {
		t.Fatal(err)
	}
	observation.AmountMinor++
	if err := r.RecordTopUpObservation(ctx, observation); !errors.Is(err, billing.ErrConflict) {
		t.Fatalf("event conflict %v", err)
	}
	observation.AmountMinor--
	observation.Merchant = topUpMerchant(billing.PaymentWeChat)
	if err := r.RecordTopUpObservation(ctx, observation); err != nil {
		t.Fatal(err)
	}
	items, err := r.ReadTopUpObservations(ctx, attempt)
	if err != nil || len(items) != 1 || items[0].Merchant.Provider != billing.PaymentAlipay {
		t.Fatalf("cross-channel inbox: %+v %v", items, err)
	}
}
