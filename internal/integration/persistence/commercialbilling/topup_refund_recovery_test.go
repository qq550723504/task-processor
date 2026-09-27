package commercialbilling

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/money"
)

type crashAfterRefundAdmission struct {
	*Repository
	crash       bool
	beforeSave  bool
	releaseSave bool
}

func (r *crashAfterRefundAdmission) SaveTopUpRefund(ctx context.Context, in billing.TopUpRefundIntent) (billing.TopUpRefundIntent, error) {
	if r.releaseSave && in.State == "RELEASED" {
		r.releaseSave = false
		return in, errors.New("process stopped after money release, before billing commit")
	}
	if r.crash && r.beforeSave && in.Dispatched {
		r.crash = false
		return in, errors.New("process stopped after money admission, before billing commit")
	}
	out, err := r.Repository.SaveTopUpRefund(ctx, in)
	if err == nil && r.crash && in.Dispatched {
		r.crash = false
		return out, errors.New("process stopped after commit, before provider call")
	}
	return out, err
}

type replayRefundProvider struct {
	*topUpTestProvider
	calls, effects, queries int
	lost                    bool
	first                   billing.TopUpRefundIntent
	queryErr                error
	pending                 bool
	closed                  bool
	confirmed               bool
	result                  billing.ProviderObservation
	queryHook               func(billing.TopUpRefundIntent)
}

func (p *replayRefundProvider) QueryRefund(_ context.Context, r billing.TopUpRefundIntent) (billing.ProviderObservation, error) {
	p.queries++
	if p.confirmed && p.effects > 0 {
		return p.result, nil
	}
	if p.queryHook != nil {
		p.queryHook(r)
	}
	if p.closed {
		o := refundRecoveryObservation(r)
		o.State = "REFUND_CLOSED"
		o.OccurredAt = time.Time{}
		return o, nil
	}
	if p.pending {
		o := refundRecoveryObservation(r)
		o.State = "REFUND_PENDING"
		return o, nil
	}
	if p.queryErr != nil {
		return billing.ProviderObservation{}, p.queryErr
	}
	return billing.ProviderObservation{}, billing.ErrRefundReplayAllowed
}
func (p *replayRefundProvider) Refund(_ context.Context, r billing.TopUpRefundIntent) (billing.ProviderObservation, error) {
	p.calls++
	if p.closed {
		o := refundRecoveryObservation(r)
		o.State = "REFUND_CLOSED"
		o.OccurredAt = time.Time{}
		return o, nil
	}
	if p.effects == 0 {
		p.first = r
		p.effects++
		p.result = refundRecoveryObservation(r)
	} else if p.first.ProviderRequestID != r.ProviderRequestID || p.first.Merchant != r.Merchant || p.first.TradeID != r.TradeID || p.first.AmountMinor != r.AmountMinor || p.first.TotalMinor != r.TotalMinor {
		return billing.ProviderObservation{}, billing.ErrConflict
	}
	if p.lost && p.calls == 1 {
		return billing.ProviderObservation{}, billing.ErrReconciliationRequired
	}
	return p.result, nil
}
func refundRecoveryObservation(r billing.TopUpRefundIntent) billing.ProviderObservation {
	return billing.ProviderObservation{Merchant: r.Merchant, MerchantOrderID: r.MerchantOrderID, EventID: "refund-success", Kind: "REFUND", State: "REFUNDED", TradeID: r.TradeID, RefundRequestID: r.ProviderRequestID, Currency: "CNY", AmountMinor: r.AmountMinor, TotalMinor: r.TotalMinor, OccurredAt: time.Now().UTC(), VerificationVersion: "fixture-v1"}
}

type failRefundReceiptRead struct{ money.ProviderTopUpOwner }

func (failRefundReceiptRead) ReadTopUpReversal(context.Context, string, string, money.TopUpReversalKey) (money.TopUpReversalReceipt, error) {
	return money.TopUpReversalReceipt{}, money.ErrUnavailable
}

type unavailableRefundAdmission struct{ money.ProviderTopUpOwner }

type concurrentOriginalRefundAdmission struct{ money.ProviderTopUpOwner }

func (m concurrentOriginalRefundAdmission) ReadTopUpRefundHold(ctx context.Context, in money.TopUpRefundInput) (money.TopUpRefundHold, error) {
	h, err := m.ProviderTopUpOwner.ReadTopUpRefundHold(ctx, in)
	if err != nil {
		return h, err
	}
	// The original authorized administrator commits admission after the worker
	// reads the unadmitted hold, before its attempted release takes the lock.
	if _, err := m.ProviderTopUpOwner.AdmitTopUpRefund(ctx, in); err != nil {
		return h, err
	}
	return h, nil
}

func (unavailableRefundAdmission) AdmitTopUpRefund(context.Context, money.TopUpRefundInput) (money.TopUpRefundHold, error) {
	return money.TopUpRefundHold{}, money.ErrUnavailable
}
func (unavailableRefundAdmission) ReleaseTopUpRefundHold(context.Context, money.TopUpRefundInput, bool) (money.TopUpRefundHold, error) {
	return money.TopUpRefundHold{}, money.ErrUnavailable
}

func TestTopUpRefundRecoveryReplaysOnlyAdmittedOriginalIdentity(t *testing.T) {
	for _, scenario := range []string{"pre_call_crash", "billing_commit_lost", "response_lost", "receipt_unavailable", "query_unknown", "pending", "never_admitted", "release_unavailable", "admission_race", "stale_worker", "external_reversal", "release_commit_lost"} {
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
			p := &replayRefundProvider{topUpTestProvider: base, lost: scenario == "response_lost", closed: scenario == "release_commit_lost", queryErr: billing.ErrReconciliationRequired}
			store := &crashAfterRefundAdmission{Repository: r, crash: !p.lost && !p.closed, beforeSave: scenario == "billing_commit_lost", releaseSave: p.closed}
			var admissionOwner money.ProviderTopUpOwner = wallet
			unadmitted := scenario == "never_admitted" || scenario == "release_unavailable" || scenario == "admission_race"
			if unadmitted {
				admissionOwner = unavailableRefundAdmission{wallet}
			}
			if err := s.EnableWalletTopUps(store, admissionOwner, p, nil, billing.TopUpAmountPolicy{}, topUpTestProtection{}, &topUpTestAuthorizer{}); err != nil {
				t.Fatal(err)
			}
			_, _ = s.ApproveTopUpRefund(ctx, "platform-admin", a.OrderID, "recover-refund", "requested", 6000, a.Version)
			rows, err := r.ListRecoverableTopUpRefunds(ctx, billing.PaymentAlipay, time.Now().UTC().Add(time.Hour), 25)
			if err != nil || len(rows) != 1 {
				t.Fatalf("durable refund missing: %+v %v", rows, err)
			}
			refund := rows[0]
			wantAdmission := !unadmitted
			wantHoldState := "RESERVED"
			if p.closed {
				wantHoldState = "RELEASED"
			}
			hold, err := wallet.ReadTopUpRefundHold(ctx, refund.HoldInput())
			if err != nil || hold.Dispatched != wantAdmission || hold.State != wantHoldState {
				t.Fatalf("wrong money admission: %+v %v", hold, err)
			}
			if scenario == "billing_commit_lost" && refund.Dispatched {
				t.Fatal("billing unexpectedly persisted admission")
			}
			// Simulate restart after the original durable lease expires.
			refund.LeaseUntil = time.Now().UTC().Add(-time.Minute)
			refund.NextCheckAt = time.Now().UTC().Add(-time.Minute)
			if _, err := r.SaveTopUpRefund(ctx, refund); err != nil {
				t.Fatal(err)
			}
			var owner money.ProviderTopUpOwner = wallet
			if scenario == "receipt_unavailable" {
				owner = failRefundReceiptRead{wallet}
			}
			if scenario == "release_unavailable" {
				owner = unavailableRefundAdmission{wallet}
			}
			if scenario == "admission_race" {
				owner = concurrentOriginalRefundAdmission{wallet}
			}
			p.queryErr = nil
			if scenario == "query_unknown" {
				p.queryErr = billing.ErrReconciliationRequired
			}
			p.pending = scenario == "pending"
			if scenario == "stale_worker" {
				p.queryHook = func(current billing.TopUpRefundIntent) {
					current.LeaseToken = "replacement-worker"
					current.LeaseUntil = time.Now().UTC().Add(time.Minute)
					if _, saveErr := r.SaveTopUpRefund(ctx, current); saveErr != nil {
						t.Error(saveErr)
					}
				}
			}
			if scenario == "external_reversal" {
				_, err = wallet.AcceptProviderTopUpReversal(ctx, money.OrganizationWalletReversal{ReversalID: "external-race", PaymentID: a.PaymentID, Kind: money.WalletReversalChargeback, OrganizationID: a.OrganizationID, CommercialOrderID: a.OrderID, Currency: "CNY", AmountMinor: 5000, OccurredAt: time.Now().UTC(), ProviderReference: "external-race"})
				if err != nil {
					t.Fatal(err)
				}
			}
			restarted, _ := billing.NewService(r, r, r, r, wallet, nil)
			if err := restarted.EnableWalletTopUps(r, owner, p, nil, billing.TopUpAmountPolicy{}, topUpTestProtection{}, &topUpTestAuthorizer{denied: true}); err != nil {
				t.Fatal(err)
			}
			queriesBeforeRestart := p.queries
			if err := restarted.ReconcileRecoverableTopUps(ctx); err != nil {
				t.Fatal(err)
			}
			balance, _ := s.ReadWallet(ctx, a.OrganizationID)
			if scenario == "pre_call_crash" || scenario == "billing_commit_lost" || scenario == "response_lost" || scenario == "external_reversal" {
				wantCalls := 1
				if p.lost {
					wantCalls = 2
				}
				wantAvailable := int64(4000)
				if scenario == "external_reversal" {
					wantAvailable = 0
				}
				if p.calls != wantCalls || p.effects != 1 || p.first.ProviderRequestID != refund.ProviderRequestID || balance.ReservedMinor != 0 || balance.AvailableMinor != wantAvailable || balance.DebtMinor != 0 {
					t.Fatalf("refund not recovered exactly once: calls=%d effects=%d balance=%+v", p.calls, p.effects, balance)
				}
				if err := restarted.ReconcileRecoverableTopUps(ctx); err != nil {
					t.Fatal(err)
				}
				if p.calls != wantCalls {
					t.Fatal("completed refund was replayed")
				}
			} else if scenario == "never_admitted" {
				remaining, listErr := r.ListRecoverableTopUpRefunds(ctx, billing.PaymentAlipay, time.Now().UTC().Add(time.Hour), 25)
				if listErr != nil || len(remaining) != 0 || p.calls != 0 || balance.ReservedMinor != 0 || balance.AvailableMinor != 10000 {
					t.Fatalf("proven unadmitted hold not released: rows=%+v balance=%+v calls=%d err=%v", remaining, balance, p.calls, listErr)
				}
			} else if p.closed {
				remaining, listErr := r.ListRecoverableTopUpRefunds(ctx, billing.PaymentAlipay, time.Now().UTC().Add(time.Hour), 25)
				if listErr != nil || len(remaining) != 0 || p.calls != 1 || p.effects != 0 || p.queries != queriesBeforeRestart || balance.ReservedMinor != 0 || balance.AvailableMinor != 10000 {
					t.Fatalf("released hold not projected without channel call: rows=%+v calls=%d queries=%d balance=%+v err=%v", remaining, p.calls, p.queries, balance, listErr)
				}
			} else if p.calls != 0 || balance.ReservedMinor != 6000 {
				t.Fatalf("unproven replay or hold release: calls=%d balance=%+v", p.calls, balance)
			}
			if (scenario == "receipt_unavailable" || unadmitted) && p.queries != 0 {
				t.Fatal("queried provider without proven money admission")
			}
		})
	}
}
