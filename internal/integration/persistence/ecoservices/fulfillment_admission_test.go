package ecoservices

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	e "task-processor/internal/ecoservices"
	"task-processor/internal/integration/ecoservicesbilling"
	"task-processor/internal/ledger/money"
	"testing"
	"time"
)

type fulfillmentHookTrading struct {
	e.TradingPort
	after         func()
	lost, invalid bool
	calls         int
}

func (p *fulfillmentHookTrading) AdmitServiceFulfillment(ctx context.Context, in e.FulfillmentAdmission) (e.FulfillmentProof, error) {
	p.calls++
	proof, err := p.TradingPort.(e.FulfillmentTradingPort).AdmitServiceFulfillment(ctx, in)
	if err != nil {
		return proof, err
	}
	if p.after != nil {
		p.after()
		p.after = nil
	}
	if p.lost {
		p.lost = false
		return e.FulfillmentProof{}, e.ErrUnavailable
	}
	if p.invalid {
		proof.InputFingerprint = "different-command"
	}
	return proof, nil
}

func TestExactFulfillmentAdmissionAfterChargebackKeepsCurrentEGuards(t *testing.T) {
	for _, kind := range []string{"start", "deliver", "accept", "reject"} {
		for _, scenario := range []string{"admitted-first", "lost-response", "newer-cas", "current-fence", "bad-proof", "E-save-failure"} {
			t.Run(kind+"/"+scenario, func(t *testing.T) {
				r, service, funds, purchases, b, req, _ := fulfillmentFundsFixture(t)
				ctx := context.Background()
				if kind != "start" {
					out, err := service.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "provider", ActorID: "provider"}, Key: uuid.NewString(), Kind: "start", ID: req.ID, Version: req.Version})
					if err != nil {
						t.Fatal(err)
					}
					req = *out.Request
				}
				if kind == "accept" || kind == "reject" {
					out, err := service.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "provider", ActorID: "provider"}, Key: uuid.NewString(), Kind: "deliver", ID: req.ID, Version: req.Version, Delivery: &e.Delivery{Content: "complete"}})
					if err != nil {
						t.Fatal(err)
					}
					req = *out.Request
				}
				order, err := b.ReadServicePurchase(ctx, req.OrderID)
				if err != nil {
					t.Fatal(err)
				}
				port := &fulfillmentHookTrading{TradingPort: ecoservicesbilling.Trading{Purchases: purchases}, lost: scenario == "lost-response", invalid: scenario == "bad-proof"}
				port.after = func() {
					if scenario == "newer-cas" {
						if _, err := service.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer"}, Key: uuid.NewString(), Kind: "refund_propose", ID: req.ID, Version: req.Version, RefundAmountMinor: 2, Reason: "new current hold"}); kind == "start" { // PAID_READY cannot propose; a real cancel changes its CAS.
							if err == nil {
								t.Fatal("prestart proposal unexpectedly legal")
							}
							if _, err = service.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer"}, Key: uuid.NewString(), Kind: "cancel", ID: req.ID, Version: req.Version}); err != nil {
								t.Fatal(err)
							}
						} else if err != nil {
							t.Fatal(err)
						}
					}
					if err := funds.RecordChargebackSettlement(ctx, money.ChargebackSettlement{ChargebackID: "after-exact-fulfillment", PaymentID: order.MoneyInput().Payment.PaymentID, AmountMinor: 60, OccurredAt: time.Now().UTC(), ProviderReference: "original-cb"}); err != nil {
						t.Fatal(err)
					}
					if scenario == "lost-response" || scenario == "current-fence" {
						result, err := purchases.Execute(ctx, order.Source)
						if err != nil || result.State != "RECONCILIATION_REQUIRED" {
							t.Fatalf("B fence %+v %v", result, err)
						}
						if scenario == "current-fence" {
							r.db.Model(&financialRow{}).Where("order_id=?", req.OrderID).Update("next_attempt_at", time.Time{})
							if err := service.Recover(ctx); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
				racing, err := e.NewService(r, port, 180)
				if err != nil {
					t.Fatal(err)
				}
				c := e.Command{Scope: e.Scope{OrganizationID: "provider", ActorID: "provider"}, Key: uuid.NewString(), Kind: kind, ID: req.ID, Version: req.Version, Delivery: &e.Delivery{Content: "complete"}, DeliveryVersion: 1, Reason: "missing document"}
				if kind == "accept" || kind == "reject" {
					c.Scope.OrganizationID = "buyer"
				}
				callback := "test:fulfillment-E-save"
				if scenario == "E-save-failure" {
					if err := r.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
						if tx.Statement.Table == "ecoservices_operations" {
							tx.AddError(e.ErrUnavailable)
						}
					}); err != nil {
						t.Fatal(err)
					}
				}
				out, err := racing.Mutate(ctx, c)
				if scenario == "lost-response" || scenario == "E-save-failure" {
					if !errors.Is(err, e.ErrUnavailable) {
						t.Fatalf("failure not exercised %v", err)
					}
					if scenario == "E-save-failure" {
						if err := r.db.Callback().Create().Remove(callback); err != nil {
							t.Fatal(err)
						}
					}
					out, err = racing.Mutate(ctx, c)
				}
				if scenario == "newer-cas" || scenario == "current-fence" || scenario == "bad-proof" {
					if err == nil {
						t.Fatal("old proof bypassed current E guard")
					}
				} else {
					if err != nil || out.Request == nil || out.Request.Version != req.Version+1 {
						t.Fatalf("original admitted fulfillment %+v %v", out, err)
					}
					calls := port.calls
					replay, err := racing.Mutate(ctx, c)
					if err != nil || replay.Request.Version != out.Request.Version || port.calls != calls {
						t.Fatalf("replay readmitted %+v %v", replay, err)
					}
					changed := c
					changed.Reason = "different payload"
					if _, err := racing.Mutate(ctx, changed); !errors.Is(err, e.ErrConflict) {
						t.Fatalf("same key changed payload %v", err)
					}
					if err := racing.Recover(ctx); err != nil {
						t.Fatal(err)
					}
				}
				f, err := funds.ReadServiceFunds(ctx, req.OrderID)
				if err != nil || f.ChargedBackMinor != 60 || f.SharedMinor != 0 || f.RefundedMinor != 0 || f.PendingOperationID != "" {
					t.Fatalf("proof dispatched/reserved money %+v %v", f, err)
				}
			})
		}
	}
}
