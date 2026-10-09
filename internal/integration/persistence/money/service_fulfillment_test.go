package money

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"strings"
	m "task-processor/internal/ledger/money"
	"testing"
	"time"
)

func fulfillmentInput(t *testing.T, r *Repository) m.ServiceFulfillmentAdmission {
	input := refundReviewInput(t, r)
	return m.ServiceFulfillmentAdmission{Payment: input.Payment, PaymentReceiptID: input.PaymentReceiptID, OrganizationID: input.Payment.ProviderOrganizationID, ActorID: "provider-user", Kind: "start", Key: "original-fulfillment-key", CommandFingerprint: strings.Repeat("a", 64), RequestVersion: 5, QuoteVersion: 1}
}
func TestServiceFulfillmentExactProofChargebackAndFailure(t *testing.T) {
	for _, scenario := range []string{"chargeback-first", "proof-first", "effect-chargeback-first", "storage-failure", "partial-refund"} {
		t.Run(scenario, func(t *testing.T) {
			r := newMoneyRepository(t)
			ctx := context.Background()
			in := fulfillmentInput(t, r)
			var original m.ServiceFulfillmentProof
			if scenario == "proof-first" {
				var err error
				original, err = r.AdmitServiceFulfillment(ctx, in)
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "storage-failure" {
				callback := "test:fulfillment-save-failure"
				if err := r.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
					if tx.Statement.Table == "ledger_service_fulfillment_admissions" {
						tx.AddError(m.ErrUnavailable)
					}
				}); err != nil {
					t.Fatal(err)
				}
				proof, err := r.AdmitServiceFulfillment(ctx, in)
				if !errors.Is(err, m.ErrUnavailable) || proof.ReceiptID != "" {
					t.Fatalf("failed save leaked proof: %+v %v", proof, err)
				}
				if err := r.db.Callback().Create().Remove(callback); err != nil {
					t.Fatal(err)
				}
			} else if scenario == "partial-refund" {
				applyServiceEffect(t, r, m.ServiceShare, "share", 10)
				applyServiceEffect(t, r, m.ServiceReturn, "return", 1)
				applyServiceEffect(t, r, m.ServiceRefundRelease, "release", 91)
				applyServiceEffect(t, r, m.ServiceRefund, "refund", 2)
			} else if scenario == "effect-chargeback-first" {
				applyServiceEffect(t, r, m.ServiceChargeback, "chargeback", 60)
			} else {
				if err := r.RecordChargebackSettlement(ctx, m.ChargebackSettlement{ChargebackID: "verified-fulfillment-cb", PaymentID: in.Payment.Payment.PaymentID, AmountMinor: 60, OccurredAt: time.Now().UTC(), ProviderReference: "original-cb"}); err != nil {
					t.Fatal(err)
				}
			}
			proof, err := r.AdmitServiceFulfillment(ctx, in)
			if scenario == "chargeback-first" || scenario == "effect-chargeback-first" {
				if !errors.Is(err, m.ErrConflict) || proof.ReceiptID != "" {
					t.Fatalf("canonical CB allowed new proof %+v %v", proof, err)
				}
				return
			}
			if err != nil || !proof.Matches(in) || scenario == "proof-first" && proof != original {
				t.Fatalf("exact admitted proof lost: %+v %v", proof, err)
			}
			for _, field := range []string{"actor", "payload", "version", "receipt", "payment", "quote", "kind"} {
				changed := in
				switch field {
				case "actor":
					changed.ActorID = "another"
				case "payload":
					changed.CommandFingerprint = strings.Repeat("b", 64)
				case "version":
					changed.RequestVersion++
				case "receipt":
					changed.PaymentReceiptID = "other-receipt"
				case "payment":
					changed.Payment.RequestID = "other-request"
				case "quote":
					changed.QuoteVersion++
				case "kind":
					changed.Kind = "accept"
				}
				if _, err := r.AdmitServiceFulfillment(ctx, changed); err == nil {
					t.Fatalf("changed %s reused proof", field)
				}
			}
			f, err := r.ReadServiceFunds(ctx, in.Payment.OrderID)
			if err != nil || f.PendingOperationID != "" {
				t.Fatalf("proof reserved money %+v %v", f, err)
			}
		})
	}
}
