package commercialbilling

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/commercial/billing"
)

func TestTopUpRefundOriginalRequestSurvivesAttemptVersionChange(t *testing.T) {
	for _, scenario := range []string{"same_request", "different_version", "different_amount", "different_reason", "different_actor", "new_key_stale", "revoked"} {
		t.Run(scenario, func(t *testing.T) {
			r, s, wallet, base, a := reviewTopUp(t, billing.PaymentAlipay)
			ctx := context.Background()
			paid := billing.ProviderObservation{Merchant: a.Merchant, MerchantOrderID: a.MerchantOrderID, EventID: "paid", Kind: "PAYMENT", State: "PAID", TradeID: "trade-1", Currency: "CNY", AmountMinor: 10000, OccurredAt: time.Now().UTC(), VerificationVersion: "v1"}
			if err := s.RecordVerifiedPaymentObservation(ctx, paid); err != nil {
				t.Fatal(err)
			}
			if err := s.ReconcileTopUpOrder(ctx, a.OrganizationID, a.OrderID); err != nil {
				t.Fatal(err)
			}
			a, _ = r.ReadTopUpAttempt(ctx, a.OrganizationID, a.OrderID)
			p := &replayRefundProvider{topUpTestProvider: base, confirmed: true}
			auth := &topUpTestAuthorizer{}
			if err := s.EnableWalletTopUps(r, wallet, p, nil, billing.TopUpAmountPolicy{}, topUpTestProtection{}, auth); err != nil {
				t.Fatal(err)
			}
			original, err := s.ApproveTopUpRefund(ctx, "platform-admin", a.OrderID, "refund-key", "requested", 6000, a.Version)
			if err != nil || original.State != "CONFIRMED" {
				t.Fatalf("initial refund failed: %+v %v", original, err)
			}
			current, _ := r.ReadTopUpAttempt(ctx, a.OrganizationID, a.OrderID)
			if current.Version == a.Version {
				t.Fatal("fixture did not advance attempt version")
			}
			// The HTTP response is lost. The global admin retries the original
			// request without reading any tenant-only order endpoint.
			actor, key, reason, amount, version := "platform-admin", "refund-key", "requested", int64(6000), a.Version
			switch scenario {
			case "different_version":
				version = current.Version
			case "different_amount":
				amount = 5999
			case "different_reason":
				reason = "changed"
			case "different_actor":
				actor = "another-admin"
			case "new_key_stale":
				key = "new-refund"
			case "revoked":
				auth.denied = true
			}
			replayed, err := s.ApproveTopUpRefund(ctx, actor, a.OrderID, key, reason, amount, version)
			if scenario == "same_request" {
				if err != nil || replayed != original {
					t.Fatalf("lost response not replayable: %+v %v", replayed, err)
				}
			} else if scenario == "revoked" {
				if err == nil {
					t.Fatal("revoked caller read refund")
				}
			} else if !errors.Is(err, billing.ErrConflict) {
				t.Fatalf("changed original request accepted: %v", err)
			}
			balance, err := s.ReadWallet(ctx, a.OrganizationID)
			if err != nil || p.calls != 1 || balance.AvailableMinor != 4000 || balance.ReservedMinor != 0 {
				t.Fatalf("replay mutated provider/wallet: calls=%d balance=%+v err=%v", p.calls, balance, err)
			}
		})
	}
}
