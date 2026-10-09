package ecoservices

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	e "task-processor/internal/ecoservices"
	"task-processor/internal/integration/ecoservicesbilling"
	"task-processor/internal/ledger/money"
)

type refundApprovalRaceTrading struct {
	e.TradingPort
	afterRead func()
}

type refundReviewHookTrading struct {
	e.TradingPort
	after        func()
	lostResponse bool
	badProof     bool
	calls        int
}

func (p *refundReviewHookTrading) AdmitServiceRefundReview(ctx context.Context, in e.RefundReviewAdmission) (e.RefundReviewProof, error) {
	p.calls++
	proof, err := p.TradingPort.(e.RefundReviewTradingPort).AdmitServiceRefundReview(ctx, in)
	if err != nil {
		return proof, err
	}
	if p.after != nil {
		p.after()
		p.after = nil
	}
	if p.lostResponse {
		p.lostResponse = false
		return e.RefundReviewProof{}, e.ErrUnavailable
	}
	if p.badProof {
		proof.InputFingerprint = "different-original-command"
	}
	return proof, nil
}

func agreeOriginalRefund(t *testing.T, s *e.Service, request e.Request) e.Request {
	t.Helper()
	ctx := context.Background()
	started, err := s.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "provider", ActorID: "provider"}, Key: uuid.NewString(), Kind: "start", ID: request.ID, Version: request.Version})
	if err != nil {
		t.Fatal(err)
	}
	request = *started.Request
	proposed, err := s.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer"}, Key: uuid.NewString(), Kind: "refund_propose", ID: request.ID, Version: request.Version, RefundAmountMinor: 101, Reason: "exact original proposal"})
	if err != nil {
		t.Fatal(err)
	}
	request = *proposed.Request
	confirmed, err := s.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "provider", ActorID: "provider"}, Key: uuid.NewString(), Kind: "refund_confirm", ID: request.ID, Version: request.Version, RefundVersion: request.Refund.Version})
	if err != nil {
		t.Fatal(err)
	}
	return *confirmed.Request
}

func TestRefundReviewExactAdmissionSurvivesOnlyOriginalCASAndCannotDispatchAfterChargeback(t *testing.T) {
	for _, scenario := range []string{"chargeback-after-admission", "lost-response", "newer-proposal-decision", "invalid-proof"} {
		t.Run(scenario, func(t *testing.T) {
			r, service, funds, purchases, b, request, _ := fulfillmentFundsFixture(t)
			request = agreeOriginalRefund(t, service, request)
			ctx := context.Background()
			order, err := b.ReadServicePurchase(ctx, request.OrderID)
			if err != nil {
				t.Fatal(err)
			}
			trading := &refundReviewHookTrading{TradingPort: ecoservicesbilling.Trading{Purchases: purchases}, lostResponse: scenario == "lost-response", badProof: scenario == "invalid-proof"}
			trading.after = func() {
				if scenario == "newer-proposal-decision" {
					if _, err := service.Mutate(ctx, e.Command{Scope: e.Scope{Platform: true, ActorID: "platform"}, Key: uuid.NewString(), Kind: "refund_review_reject", ID: request.ID, Version: request.Version, RefundVersion: request.Refund.Version, Reason: "current decision changed"}); err != nil {
						t.Fatal(err)
					}
				} else if scenario != "invalid-proof" {
					if err := funds.RecordChargebackSettlement(ctx, money.ChargebackSettlement{ChargebackID: "verified-after-admission", PaymentID: order.MoneyInput().Payment.PaymentID, AmountMinor: 60, OccurredAt: time.Now().UTC(), ProviderReference: "original-chargeback"}); err != nil {
						t.Fatal(err)
					}
					if scenario == "lost-response" {
						// B has projected the new canonical fence while E has not
						// consumed the original proof/response. Exact readback is
						// not a new admission or a refund dispatch.
						if result, err := purchases.Execute(ctx, order.Source); err != nil || result.State != "RECONCILIATION_REQUIRED" {
							t.Fatalf("B fence projection: %+v %v", result, err)
						}
					}
				}
			}
			racing, err := e.NewService(r, trading, 180)
			if err != nil {
				t.Fatal(err)
			}
			command := e.Command{Scope: e.Scope{Platform: true, ActorID: "platform"}, Key: uuid.NewString(), Kind: "refund_review", ID: request.ID, Version: request.Version, RefundVersion: request.Refund.Version, Reason: "review original agreement"}
			approved, err := racing.Mutate(ctx, command)
			if scenario == "lost-response" {
				if !errors.Is(err, e.ErrUnavailable) {
					t.Fatalf("proof response loss not exercised: %v", err)
				}
				approved, err = racing.Mutate(ctx, command)
			}
			if scenario == "newer-proposal-decision" || scenario == "invalid-proof" {
				if err == nil {
					t.Fatal("old/invalid admission overwrote current E decision")
				}
				var count int64
				if err := r.db.Model(&financialRow{}).Where("order_id=? AND kind='REFUND'", request.OrderID).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("unconsumed proof created refund: %d %v", count, err)
				}
			} else {
				if err != nil || approved.Request == nil || approved.Request.Refund.State != "APPROVED" {
					t.Fatalf("original admitted intent lost: %+v %v", approved, err)
				}
				if err := racing.Recover(ctx); err != nil {
					t.Fatal(err)
				}
				page, err := r.Read(ctx, e.Query{Scope: e.Scope{OrganizationID: "buyer"}, Kind: "requests", ID: request.ID, Page: 1, PageSize: 1})
				if err != nil || len(page.Requests) != 1 || !page.Requests[0].FinancialFence || page.Requests[0].FinancialState != "RECONCILIATION_REQUIRED" {
					t.Fatalf("later chargeback not fenced: %+v %v", page, err)
				}
				calls := trading.calls
				replay, err := racing.Mutate(ctx, command)
				if err != nil || replay.Request.Version != approved.Request.Version || trading.calls != calls {
					t.Fatalf("original immutable replay re-admitted: %+v %v", replay, err)
				}
				changed := command
				changed.Reason = "different same-key payload"
				if _, err := racing.Mutate(ctx, changed); !errors.Is(err, e.ErrConflict) {
					t.Fatalf("same key changed review accepted: %v", err)
				}
			}
			view, err := funds.ReadServiceFunds(ctx, request.OrderID)
			if err != nil || view.PendingOperationID != "" || view.RefundedMinor != 0 || view.SharedMinor != 0 {
				t.Fatalf("review proof reserved/dispatched money: %+v %v", view, err)
			}
		})
	}
}

func (t refundApprovalRaceTrading) AdmitServiceRefundReview(ctx context.Context, in e.RefundReviewAdmission) (e.RefundReviewProof, error) {
	t.afterRead()
	return t.TradingPort.(e.RefundReviewTradingPort).AdmitServiceRefundReview(ctx, in)
}

func TestRefundApprovalAfterConcurrentChargebackCannotPersist(t *testing.T) {
	r, service, funds, purchases, b, request, _ := fulfillmentFundsFixture(t)
	ctx := context.Background()
	started, err := service.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "provider", ActorID: "provider"}, Key: uuid.NewString(), Kind: "start", ID: request.ID, Version: request.Version})
	if err != nil {
		t.Fatal(err)
	}
	request = *started.Request
	proposed, err := service.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer"}, Key: uuid.NewString(), Kind: "refund_propose", ID: request.ID, Version: request.Version, RefundAmountMinor: request.Quote.AmountMinor, Reason: "agreed original refund"})
	if err != nil {
		t.Fatal(err)
	}
	request = *proposed.Request
	confirmed, err := service.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "provider", ActorID: "provider"}, Key: uuid.NewString(), Kind: "refund_confirm", ID: request.ID, Version: request.Version, RefundVersion: request.Refund.Version})
	if err != nil {
		t.Fatal(err)
	}
	request = *confirmed.Request
	order, err := b.ReadServicePurchase(ctx, request.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	trading := refundApprovalRaceTrading{TradingPort: ecoservicesbilling.Trading{Purchases: purchases}, afterRead: func() {
		if err := funds.RecordChargebackSettlement(ctx, money.ChargebackSettlement{ChargebackID: "verified-refund-approval-race", PaymentID: order.MoneyInput().Payment.PaymentID, AmountMinor: 60, OccurredAt: time.Now().UTC(), ProviderReference: "original-chargeback"}); err != nil {
			t.Fatal(err)
		}
	}}
	racing, err := e.NewService(r, trading, 180)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := racing.Mutate(ctx, e.Command{Scope: e.Scope{Platform: true, ActorID: "platform"}, Key: uuid.NewString(), Kind: "refund_review", ID: request.ID, Version: request.Version, RefundVersion: request.Refund.Version, Reason: "approved exact agreement"})
	if err == nil {
		t.Fatalf("stale funds allowed durable refund approval: %+v", approved.Request)
	}
	view, fundsErr := funds.ReadServiceFunds(ctx, request.OrderID)
	if fundsErr != nil || view.ChargedBackMinor != 60 || view.ReconciliationReason == "" || view.PendingOperationID != "" || view.RefundedMinor != 0 {
		t.Fatalf("real chargeback was not exercised: %+v %v", view, fundsErr)
	}
	page, err := r.Read(ctx, e.Query{Scope: e.Scope{OrganizationID: "buyer", ActorID: "buyer"}, Kind: "requests", ID: request.ID, Page: 1, PageSize: 1})
	if err != nil || len(page.Requests) != 1 || page.Requests[0].Refund.State == "APPROVED" {
		t.Fatalf("chargeback-first original refund approval persisted: %+v %v", page, err)
	}
}
