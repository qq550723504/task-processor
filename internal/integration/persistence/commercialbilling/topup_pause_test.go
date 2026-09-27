package commercialbilling

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/commercial/billing"
)

type pausedTopUpProvider struct{ *topUpTestProvider }

func (pausedTopUpProvider) Available() bool { return false }

func TestTopUpPausePreservesOnlyAuthorizedOriginalCheckout(t *testing.T) {
	for _, channel := range []billing.PaymentProvider{billing.PaymentAlipay, billing.PaymentWeChat} {
		for _, scenario := range []string{"stored", "never_issued", "revoked", "other_actor"} {
			t.Run(string(channel)+"/"+scenario, func(t *testing.T) {
				r, s, wallet, p, a := reviewTopUp(t, channel)
				ctx := context.Background()
				var original billing.CheckoutAction
				var err error
				if scenario != "never_issued" {
					original, err = s.CheckoutTopUp(ctx, a.OrganizationID, a.ActorID, a.OrderID, a.Version)
					if err != nil {
						t.Fatal(err)
					}
				}
				a, _ = r.ReadTopUpAttempt(ctx, a.OrganizationID, a.OrderID)
				var ali, wx billing.TopUpProviderPort
				if channel == billing.PaymentAlipay {
					ali = pausedTopUpProvider{p}
				} else {
					wx = pausedTopUpProvider{p}
				}
				restarted, _ := billing.NewService(r, r, r, r, wallet, nil)
				if err = restarted.EnableWalletTopUps(r, wallet, ali, wx, s.WalletTopUpOptions().Policy, topUpTestProtection{}, &topUpTestAuthorizer{denied: scenario == "revoked"}); err != nil {
					t.Fatal(err)
				}
				actor := a.ActorID
				if scenario == "other_actor" {
					actor = "other-admin"
				}
				before := p.checkouts
				replayed, err := restarted.CheckoutTopUp(ctx, a.OrganizationID, actor, a.OrderID, a.Version)
				if scenario == "stored" {
					if err != nil || replayed != original {
						t.Fatalf("paused original checkout lost: %+v %v", replayed, err)
					}
				} else if scenario == "never_issued" {
					if !errors.Is(err, billing.ErrPaymentMethodUnavailable) {
						t.Fatalf("new checkout not disabled: %v", err)
					}
				} else if err == nil {
					t.Fatal("unauthorized checkout replay")
				}
				if p.checkouts != before {
					t.Fatal("replay called provider")
				}
			})
		}
	}
}
