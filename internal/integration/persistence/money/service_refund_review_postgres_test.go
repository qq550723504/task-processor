//go:build integration

package money

import (
	"errors"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	m "task-processor/internal/ledger/money"
)

func TestServiceRefundReviewPostgresUsesOriginalPaymentLockAndImmutableRole(t *testing.T) {
	for _, first := range []string{"review", "chargeback"} {
		t.Run(first, func(t *testing.T) {
			ctx, admin, r, _ := newMoneyPostgresRuntime(t)
			in := refundReviewInput(t, r)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			callback := "test:original-refund-review-lock"
			hook := func(tx *gorm.DB) {
				want := "ledger_service_refund_review_admissions"
				if first == "chargeback" {
					want = "ledger_service_payment_bindings"
				}
				if tx.Statement.Table == want {
					close(entered)
					select {
					case <-release:
					case <-ctx.Done():
						tx.AddError(ctx.Err())
					}
				}
			}
			if first == "review" {
				if err := r.db.Callback().Create().Before("gorm:create").Register(callback, hook); err != nil {
					t.Fatal(err)
				}
				defer r.db.Callback().Create().Remove(callback)
			} else {
				if err := r.db.Callback().Update().Before("gorm:update").Register(callback, hook); err != nil {
					t.Fatal(err)
				}
				defer r.db.Callback().Update().Remove(callback)
			}
			type reviewResult struct {
				proof m.ServiceRefundReviewProof
				err   error
			}
			reviewDone := make(chan reviewResult, 1)
			chargebackDone := make(chan error, 1)
			review := func() { proof, err := r.AdmitServiceRefundReview(ctx, in); reviewDone <- reviewResult{proof, err} }
			chargeback := func() {
				chargebackDone <- r.RecordChargebackSettlement(ctx, m.ChargebackSettlement{ChargebackID: "pg-original-chargeback", PaymentID: in.Payment.Payment.PaymentID, AmountMinor: 60, OccurredAt: time.Now().UTC(), ProviderReference: "verified-pg-chargeback"})
			}
			if first == "review" {
				go review()
			} else {
				go chargeback()
			}
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("first owner transaction did not hold payment lock")
			}
			// A separate runtime connection must actually wait on the held row;
			// the test controls commit order, rather than racing goroutine starts.
			if first == "review" {
				go chargeback()
				select {
				case err := <-chargebackDone:
					t.Fatalf("chargeback bypassed held payment lock: %v", err)
				case <-time.After(100 * time.Millisecond):
				}
			} else {
				go review()
				select {
				case result := <-reviewDone:
					t.Fatalf("review bypassed held payment lock: %+v", result)
				case <-time.After(100 * time.Millisecond):
				}
			}
			once.Do(func() { close(release) })
			if err := <-chargebackDone; err != nil {
				t.Fatal(err)
			}
			result := <-reviewDone
			if first == "review" {
				if result.err != nil || !result.proof.Matches(in) {
					t.Fatalf("first exact admission lost: %+v", result)
				}
				// Remove the one-shot barrier before the immutable readback.
				if err := r.db.Callback().Create().Remove(callback); err != nil {
					t.Fatal(err)
				}
				read, err := r.AdmitServiceRefundReview(ctx, in)
				if err != nil || read != result.proof {
					t.Fatalf("exact original readback: %+v %v", read, err)
				}
			} else if !errors.Is(result.err, m.ErrConflict) || result.proof.ReceiptID != "" {
				t.Fatalf("chargeback-first amount admitted: %+v", result)
			}
			var update, deleteAllowed, insert, selectAllowed bool
			if err := admin.Raw(`SELECT has_table_privilege('money_owner_runtime','public.ledger_service_refund_review_admissions','UPDATE'),has_table_privilege('money_owner_runtime','public.ledger_service_refund_review_admissions','DELETE'),has_table_privilege('money_owner_runtime','public.ledger_service_refund_review_admissions','INSERT'),has_table_privilege('money_owner_runtime','public.ledger_service_refund_review_admissions','SELECT')`).Row().Scan(&update, &deleteAllowed, &insert, &selectAllowed); err != nil || update || deleteAllowed || !insert || !selectAllowed {
				t.Fatalf("proof role not immutable: update=%v delete=%v insert=%v select=%v %v", update, deleteAllowed, insert, selectAllowed, err)
			}
			f, err := r.ReadServiceFunds(ctx, in.Payment.OrderID)
			if err != nil || f.ChargedBackMinor != 60 || f.ReconciliationReason == "" || f.PendingOperationID != "" || f.RefundedMinor != 0 {
				t.Fatalf("proof lock changed funds: %+v %v", f, err)
			}
		})
	}
}
