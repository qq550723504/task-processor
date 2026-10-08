package referral

import (
	"context"
	m "task-processor/internal/ledger/money"
	"testing"
	"time"
)

func TestServicePaymentAndReversalsNeverCreateReferralEarnings(t *testing.T) {
	r, p := settlementRepository(t)
	p.PaymentPurpose = m.PaymentPurposeServicePurchase
	p.CommissionTreatment = m.CommissionNonCommissionable
	p.PayerBinding = m.PayerOrganizationServiceBuyer
	p.PayerUserID = ""
	p.CommissionableAmountMinor = 0
	if err := r.db.Model(&canonicalPaymentRow{}).Where("payment_id=?", p.PaymentID).Updates(map[string]any{"payment_purpose": p.PaymentPurpose, "commission_treatment": p.CommissionTreatment, "payer_binding": p.PayerBinding, "payer_user_id": "", "commissionable_amount_minor": 0}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := r.ObservePaymentSettlement(ctx, p); err != nil {
		t.Fatal(err)
	}
	at := m.NormalizeTimestamp(time.Now())
	rf := m.RefundSettlement{RefundID: "service-refund", PaymentID: p.PaymentID, AmountMinor: 6000, OccurredAt: at, ProviderReference: "verified-refund"}
	cb := m.ChargebackSettlement{ChargebackID: "service-chargeback", PaymentID: p.PaymentID, AmountMinor: 4000, OccurredAt: at, ProviderReference: "verified-chargeback"}
	if err := r.db.Create(&canonicalRefundRow{RefundID: rf.RefundID, PaymentID: p.PaymentID, AmountMinor: rf.AmountMinor, OccurredAt: at, ProviderReference: rf.ProviderReference}).Error; err != nil {
		t.Fatal(err)
	}
	if err := r.db.Create(&canonicalChargebackRow{ChargebackID: cb.ChargebackID, PaymentID: p.PaymentID, AmountMinor: cb.AmountMinor, OccurredAt: at, ProviderReference: cb.ProviderReference}).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := r.ObserveRefundSettlement(ctx, rf); err != nil {
			t.Fatalf("valid service refund must be excluded without requiring referral claim: %v", err)
		}
		if err := r.ObserveChargebackSettlement(ctx, cb); err != nil {
			t.Fatal(err)
		}
	}
	for _, model := range []any{&earningClaim{}, &earningLedgerEntry{}, &earningProjection{}, &refundOperationRow{}, &chargebackOperationRow{}} {
		var count int64
		if err := r.db.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("service generated promotion facts: %T count=%d error=%v", model, count, err)
		}
	}
}
