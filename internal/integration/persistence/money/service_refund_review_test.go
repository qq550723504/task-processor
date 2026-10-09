package money

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	m "task-processor/internal/ledger/money"
)

func refundReviewInput(t *testing.T, r *Repository) m.ServiceRefundReviewAdmission {
	t.Helper()
	payment := servicePayment()
	receipt, err := r.AcceptServicePayment(context.Background(), payment)
	if err != nil {
		t.Fatal(err)
	}
	return m.ServiceRefundReviewAdmission{Payment: payment, PaymentReceiptID: receipt.ReceiptID, ActorID: "platform-reviewer", Kind: "refund_review", Key: "original-review-key", CommandFingerprint: strings.Repeat("a", 64), RequestVersion: 5, RefundVersion: 2, QuoteVersion: 1, AmountMinor: 101}
}

func TestServiceRefundReviewProofStorageFailureCannotAdmitOrOccupyFunds(t *testing.T) {
	r := newMoneyRepository(t)
	ctx := context.Background()
	in := refundReviewInput(t, r)
	callback := "test:refund-review-storage-failure"
	if err := r.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "ledger_service_refund_review_admissions" {
			tx.AddError(m.ErrUnavailable)
		}
	}); err != nil {
		t.Fatal(err)
	}
	proof, err := r.AdmitServiceRefundReview(ctx, in)
	if !errors.Is(err, m.ErrUnavailable) || proof.ReceiptID != "" {
		t.Fatalf("failed save leaked admission: %+v %v", proof, err)
	}
	if err := r.db.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := r.db.Model(&serviceRefundReviewRow{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed proof persisted: %d %v", count, err)
	}
	proof, err = r.AdmitServiceRefundReview(ctx, in)
	if err != nil || !proof.Matches(in) {
		t.Fatalf("same original recovery failed: %+v %v", proof, err)
	}
	if err := r.db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "ledger_service_refund_review_admissions" {
			tx.AddError(m.ErrUnavailable)
		}
	}); err != nil {
		t.Fatal(err)
	}
	read, err := r.AdmitServiceRefundReview(ctx, in)
	if !errors.Is(err, m.ErrUnavailable) || read.ReceiptID != "" {
		t.Fatalf("failed proof read guessed approval: %+v %v", read, err)
	}
	if err := r.db.Callback().Query().Remove(callback); err != nil {
		t.Fatal(err)
	}
	f, err := r.ReadServiceFunds(ctx, in.Payment.OrderID)
	if err != nil || f.PendingOperationID != "" || f.RefundedMinor != 0 {
		t.Fatalf("non-economic proof reserved money: %+v %v", f, err)
	}
}

func TestServiceRefundReviewAdmissionOrdersChargebackAndExactReadback(t *testing.T) {
	for _, first := range []string{"chargeback", "review"} {
		t.Run(first, func(t *testing.T) {
			r := newMoneyRepository(t)
			ctx := context.Background()
			in := refundReviewInput(t, r)
			var admitted m.ServiceRefundReviewProof
			if first == "review" {
				var err error
				admitted, err = r.AdmitServiceRefundReview(ctx, in)
				if err != nil || !admitted.Matches(in) {
					t.Fatalf("original admission: %+v %v", admitted, err)
				}
			}
			if err := r.RecordChargebackSettlement(ctx, m.ChargebackSettlement{ChargebackID: "original-chargeback", PaymentID: in.Payment.Payment.PaymentID, AmountMinor: 60, OccurredAt: time.Now().UTC(), ProviderReference: "verified-chargeback"}); err != nil {
				t.Fatal(err)
			}
			proof, err := r.AdmitServiceRefundReview(ctx, in)
			if first == "chargeback" {
				if !errors.Is(err, m.ErrConflict) || proof.ReceiptID != "" {
					t.Fatalf("chargeback-first admitted stale amount: %+v %v", proof, err)
				}
				return
			}
			if err != nil || proof != admitted {
				t.Fatalf("same original proof was not read back: %+v %v", proof, err)
			}
			for _, field := range []string{"actor", "payload", "version", "refund-version", "payment-receipt", "amount"} {
				changed := in
				switch field {
				case "actor":
					changed.ActorID = "other-platform"
				case "payload":
					changed.CommandFingerprint = strings.Repeat("b", 64)
				case "version":
					changed.RequestVersion++
				case "refund-version":
					changed.RefundVersion++
				case "payment-receipt":
					changed.PaymentReceiptID = "other-payment"
				case "amount":
					changed.AmountMinor--
				}
				if _, err := r.AdmitServiceRefundReview(ctx, changed); !errors.Is(err, m.ErrConflict) {
					t.Fatalf("same key different %s consumed proof: %v", field, err)
				}
			}
			funds, err := r.ReadServiceFunds(ctx, in.Payment.OrderID)
			if err != nil || funds.PendingOperationID != "" || funds.RefundedMinor != 0 || funds.ChargedBackMinor != 60 || funds.ReconciliationReason == "" {
				t.Fatalf("proof changed funds/reservation: %+v %v", funds, err)
			}
		})
	}
}

func TestServiceRefundReviewAdmissionDoesNotOccupySharedRefundReservation(t *testing.T) {
	r := newMoneyRepository(t)
	ctx := context.Background()
	in := refundReviewInput(t, r)
	applyServiceEffect(t, r, m.ServiceShare, "share", 10)
	in.AmountMinor = 2
	if proof, err := r.AdmitServiceRefundReview(ctx, in); err != nil || !proof.Matches(in) {
		t.Fatalf("shared refund review cannot admit: %+v %v", proof, err)
	}
	applyServiceEffect(t, r, m.ServiceReturn, "return", 1)
	applyServiceEffect(t, r, m.ServiceRefundRelease, "release", 91)
	applyServiceEffect(t, r, m.ServiceRefund, "refund", 2)
	f, err := r.ReadServiceFunds(ctx, in.Payment.OrderID)
	if err != nil || f.ReturnedMinor != 1 || f.RefundedMinor != 2 || f.PendingOperationID != "" {
		t.Fatalf("review proof blocked original refund: %+v %v", f, err)
	}
}
