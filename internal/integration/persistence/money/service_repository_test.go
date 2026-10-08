package money

import (
	"context"
	"errors"
	m "task-processor/internal/ledger/money"
	"testing"
	"time"
)

func servicePayment() m.ServicePaymentInput {
	return m.ServicePaymentInput{OrderID: "service-order", RequestID: "service-request", BuyerOrganizationID: "buyer", ProviderOrganizationID: "provider", PlatformMerchantID: "platform-mch", ProviderMerchantID: "sub-mch", PolicyVersion: "eco-v1-10-platform-fee-ar1", Binding: m.ProviderPaymentBinding{Provider: "WECHAT_PAY", Environment: "PRODUCTION", MerchantID: "sub-mch", AppID: "platform-app", TradeID: "original-tx"}, Payment: m.PaymentSettlement{PaymentID: "service-pay", PaymentPurpose: m.PaymentPurposeServicePurchase, CommissionTreatment: m.CommissionNonCommissionable, PayerBinding: m.PayerOrganizationServiceBuyer, Currency: "CNY", GrossAmountMinor: 101, Status: m.PaymentSettled, SettledAt: time.Date(2026, 10, 8, 1, 0, 0, 0, time.UTC), ProviderReference: "original-tx", Version: 1}}
}
func TestServiceFundsOriginalIdentityAndNoOrdinaryBypass(t *testing.T) {
	r := newMoneyRepository(t)
	ctx := context.Background()
	in := servicePayment()
	if err := r.RecordPaymentSettlement(ctx, in.Payment); !errors.Is(err, m.ErrUnsupportedMutation) {
		t.Fatalf("ordinary payment bypass: %v", err)
	}
	first, err := r.AcceptServicePayment(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := r.AcceptServicePayment(ctx, in)
	if err != nil || first != again {
		t.Fatalf("unstable receipt %+v %+v %v", first, again, err)
	}
	readback, err := r.ReadServicePayment(ctx, in)
	if err != nil || readback != first {
		t.Fatalf("original payment readback changed: %+v %v", readback, err)
	}
	in.BuyerOrganizationID = "other"
	if _, err := r.AcceptServicePayment(ctx, in); !errors.Is(err, m.ErrConflict) {
		t.Fatalf("different payload accepted: %v", err)
	}
	for _, mutate := range []func() error{
		func() error {
			return r.RecordRefundSettlement(ctx, m.RefundSettlement{RefundID: "bypass", PaymentID: "service-pay", AmountMinor: 1, OccurredAt: time.Now(), ProviderReference: "ref"})
		},
		func() error {
			return r.RecordChargebackSettlement(ctx, m.ChargebackSettlement{ChargebackID: "bypass", PaymentID: "service-pay", AmountMinor: 1, OccurredAt: time.Now(), ProviderReference: "cb"})
		},
	} {
		if err := mutate(); !errors.Is(err, m.ErrUnsupportedMutation) {
			t.Fatalf("ordinary reversal bypass: %v", err)
		}
	}
	var wallets int64
	if err := r.db.Model(&organizationWalletRow{}).Count(&wallets).Error; err != nil || wallets != 0 {
		t.Fatalf("service generated wallet balance: %d %v", wallets, err)
	}
}
func applyServiceEffect(t *testing.T, r *Repository, kind m.ServiceEffectKind, id string, amount int64) {
	t.Helper()
	ctx := context.Background()
	op := m.ServiceOperation{OrderID: "service-order", OperationID: id, Kind: kind, AmountMinor: amount, SourceProofID: "approved-original-business-proof"}
	if kind == m.ServiceReturn {
		op.OriginalShareID = "share"
	}
	if _, err := r.PrepareServiceOperation(ctx, op); err != nil {
		t.Fatal(err)
	}
	effect := m.ServiceEffect{Operation: op, ProviderReference: "verified-" + id, OccurredAt: time.Date(2026, 10, 8, 2, 0, 0, 0, time.UTC)}
	if err := r.AdmitServiceOperation(ctx, op); err != nil {
		t.Fatal(err)
	}
	a, err := r.AcceptServiceEffect(ctx, effect)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.AcceptServiceEffect(ctx, effect)
	if err != nil || a != b {
		t.Fatalf("duplicate effect: %v", err)
	}
}
func TestServiceRefundUsesGrossAndCumulativeAllocation(t *testing.T) {
	r := newMoneyRepository(t)
	ctx := context.Background()
	if _, err := r.AcceptServicePayment(ctx, servicePayment()); err != nil {
		t.Fatal(err)
	}
	applyServiceEffect(t, r, m.ServiceRefund, "refund-before-accept", 2)
	funds, err := r.ReadServiceFunds(ctx, "service-order")
	if err != nil || funds.PlatformMinor != 9 || funds.ProviderMinor != 90 {
		t.Fatalf("net allocation %+v %v", funds, err)
	}
	applyServiceEffect(t, r, m.ServiceShare, "share", 9)
	applyServiceEffect(t, r, m.ServiceFinish, "finish", 90)
	// Commission must return before the customer refund can be confirmed.
	bad := m.ServiceOperation{OrderID: "service-order", OperationID: "unsafe-refund", Kind: m.ServiceRefund, AmountMinor: 99, SourceProofID: "approved-refund"}
	if _, err := r.PrepareServiceOperation(ctx, bad); !errors.Is(err, m.ErrConflict) {
		t.Fatalf("refund before commission return: %v", err)
	}
	applyServiceEffect(t, r, m.ServiceReturn, "return", 9)
	applyServiceEffect(t, r, m.ServiceRefund, "full-refund", 99)
	funds, err = r.ReadServiceFunds(ctx, "service-order")
	if err != nil || funds.RefundedMinor != 101 || funds.PlatformMinor != 0 || funds.ProviderMinor != 0 || funds.SharedMinor != 9 || funds.ReturnedMinor != 9 {
		t.Fatalf("full refund allocation %+v %v", funds, err)
	}
}
func TestServiceZeroCommissionAndPendingReservation(t *testing.T) {
	r := newMoneyRepository(t)
	ctx := context.Background()
	in := servicePayment()
	in.Payment.GrossAmountMinor = 9
	if _, err := r.AcceptServicePayment(ctx, in); err != nil {
		t.Fatal(err)
	}
	applyServiceEffect(t, r, m.ServiceShare, "zero-share", 0)
	op := m.ServiceOperation{OrderID: in.OrderID, OperationID: "refund", Kind: m.ServiceRefund, AmountMinor: 9, SourceProofID: "cancel-proof"}
	if _, err := r.PrepareServiceOperation(ctx, op); err != nil {
		t.Fatal(err)
	}
	other := op
	other.OperationID = "second-refund"
	if _, err := r.PrepareServiceOperation(ctx, other); !errors.Is(err, m.ErrConflict) {
		t.Fatalf("overlapping financial effect: %v", err)
	}
	op.AmountMinor = 8
	if _, err := r.PrepareServiceOperation(ctx, op); !errors.Is(err, m.ErrConflict) {
		t.Fatalf("changed same operation: %v", err)
	}
}

func TestServiceExternalChargebackKeepsTruthAndBlocksNewEffects(t *testing.T) {
	r := newMoneyRepository(t)
	ctx := context.Background()
	in := servicePayment()
	if _, err := r.AcceptServicePayment(ctx, in); err != nil {
		t.Fatal(err)
	}
	applyServiceEffect(t, r, m.ServiceShare, "share", 10)
	cb := m.ChargebackSettlement{ChargebackID: "external-chargeback", PaymentID: in.Payment.PaymentID, AmountMinor: 101, OccurredAt: time.Date(2026, 10, 8, 4, 0, 0, 0, time.UTC), ProviderReference: "signed-chargeback-observation"}
	if err := r.ObserveServiceChargeback(ctx, in.OrderID, cb); err != nil {
		t.Fatal(err)
	}
	if err := r.ObserveServiceChargeback(ctx, in.OrderID, cb); err != nil {
		t.Fatal(err)
	}
	funds, err := r.ReadServiceFunds(ctx, in.OrderID)
	if err != nil || funds.ChargedBackMinor != 101 || funds.PlatformMinor != 0 || funds.ReconciliationReason == "" {
		t.Fatalf("external chargeback truth was lost: %+v %v", funds, err)
	}
	if _, err := r.PrepareServiceOperation(ctx, m.ServiceOperation{OrderID: in.OrderID, OperationID: "new-finish", Kind: m.ServiceFinish, AmountMinor: 0, SourceProofID: "acceptance"}); !errors.Is(err, m.ErrConflict) {
		t.Fatalf("new finance command after unplanned chargeback: %v", err)
	}
}

func TestServiceInFlightShareSuccessAfterChargebackRetainsBothFacts(t *testing.T) {
	r := newMoneyRepository(t)
	ctx := context.Background()
	in := servicePayment()
	if _, err := r.AcceptServicePayment(ctx, in); err != nil {
		t.Fatal(err)
	}
	op := m.ServiceOperation{OrderID: in.OrderID, OperationID: "in-flight-share", Kind: m.ServiceShare, AmountMinor: 10, SourceProofID: "accepted-delivery"}
	if _, err := r.PrepareServiceOperation(ctx, op); err != nil {
		t.Fatal(err)
	}
	cb := m.ChargebackSettlement{ChargebackID: "late-cb", PaymentID: in.Payment.PaymentID, AmountMinor: 101, OccurredAt: time.Now(), ProviderReference: "signed-cb"}
	if err := r.AdmitServiceOperation(ctx, op); err != nil {
		t.Fatal(err)
	}
	if err := r.ObserveServiceChargeback(ctx, in.OrderID, cb); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AcceptServiceEffect(ctx, m.ServiceEffect{Operation: op, ProviderReference: "signed-original-share-success", OccurredAt: time.Now()}); err != nil {
		t.Fatalf("in-flight financial truth was rejected after external chargeback: %v", err)
	}
	funds, err := r.ReadServiceFunds(ctx, in.OrderID)
	if err != nil || funds.SharedMinor != 10 || funds.ChargedBackMinor != 101 || funds.ReconciliationReason == "" {
		t.Fatalf("lost original effects: %+v %v", funds, err)
	}
}

func TestServiceSourceDenialReleasesOnlyUndispatchedReservation(t *testing.T) {
	r := newMoneyRepository(t)
	ctx := context.Background()
	in := servicePayment()
	if _, err := r.AcceptServicePayment(ctx, in); err != nil {
		t.Fatal(err)
	}
	share := m.ServiceOperation{OrderID: in.OrderID, OperationID: "denied-share", Kind: m.ServiceShare, AmountMinor: 10, SourceProofID: "original-acceptance"}
	if _, err := r.PrepareServiceOperation(ctx, share); err != nil {
		t.Fatal(err)
	}
	if err := r.AbandonUndispatchedServiceOperation(ctx, share, "source-denied-receipt"); err != nil {
		t.Fatal(err)
	}
	refund := m.ServiceOperation{OrderID: in.OrderID, OperationID: "refund", Kind: m.ServiceRefund, AmountMinor: 101, SourceProofID: "approved-refund"}
	if _, err := r.PrepareServiceOperation(ctx, refund); err != nil {
		t.Fatal(err)
	}
	if err := r.AdmitServiceOperation(ctx, refund); err != nil {
		t.Fatal(err)
	}
	if err := r.AbandonUndispatchedServiceOperation(ctx, refund, "claimed-no-response"); !errors.Is(err, m.ErrConflict) {
		t.Fatalf("unknown dispatched refund reservation released: %v", err)
	}
}
