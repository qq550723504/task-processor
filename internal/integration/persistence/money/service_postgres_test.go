//go:build integration

package money

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"sync"
	m "task-processor/internal/ledger/money"
	"testing"
)

func TestServicePostgresOriginalClaimAndConcurrentRefundReservation(t *testing.T) {
	ctx, db, r, observer := newMoneyPostgresRuntime(t)
	in := servicePayment()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	receipts := make(chan m.ServiceReceipt, 8)
	for range 8 {
		wg.Go(func() {
			v, err := r.AcceptServicePayment(ctx, in)
			if err != nil {
				errs <- err
			} else {
				receipts <- v
			}
		})
	}
	wg.Wait()
	close(errs)
	close(receipts)
	for err := range errs {
		t.Fatal(err)
	}
	var first m.ServiceReceipt
	for v := range receipts {
		if first.ReceiptID == "" {
			first = v
		} else if first != v {
			t.Fatal("concurrent receipt changed")
		}
	}
	// The fresh installer must admit service payments without admitting referral
	// commission or a different payer ownership into the same canonical table.
	for _, variant := range []string{"commission", "payer", "treatment", "binding"} {
		invalid := paymentRowFromFact(in.Payment)
		invalid.PaymentID += ":" + variant
		switch variant {
		case "commission":
			invalid.CommissionableAmountMinor = 1
		case "payer":
			invalid.PayerUserID = "individual"
		case "treatment":
			invalid.CommissionTreatment = ""
		case "binding":
			invalid.PayerBinding = m.PayerUnattributedExternal
		}
		var pgErr *pgconn.PgError
		if err := r.db.Create(&invalid).Error; !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Fatalf("installer admitted invalid service %s: %v", variant, err)
		}
	}
	if err := observer.ObservePaymentSettlement(ctx, in.Payment); err != nil {
		t.Fatal(err)
	}
	// Original channel identity cannot also be credited as a top-up.
	top := providerTopUpInput()
	top.Binding = in.Binding
	top.Payment.PaymentID = "different-purpose-payment"
	if _, err := r.AcceptAndPostProviderTopUp(ctx, top); !errors.Is(err, m.ErrConflict) {
		t.Fatalf("cross-purpose original payment accepted: %v", err)
	}
	successes := make(chan string, 2)
	failures := make(chan error, 2)
	for _, id := range []string{"refund-a", "refund-b"} {
		wg.Go(func() {
			_, err := r.PrepareServiceOperation(ctx, m.ServiceOperation{OrderID: in.OrderID, OperationID: id, Kind: m.ServiceRefund, AmountMinor: 80, SourceProofID: "same-original-approval"})
			if err == nil {
				successes <- id
			} else {
				failures <- err
			}
		})
	}
	wg.Wait()
	close(successes)
	close(failures)
	if len(successes) != 1 || len(failures) != 1 {
		t.Fatalf("concurrent financial reservations: success=%d failure=%d", len(successes), len(failures))
	}
	for err := range failures {
		if !errors.Is(err, m.ErrConflict) {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Table("ledger_payment_settlements").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("canonical facts=%d error=%v", count, err)
	}
	if err := db.Exec("GRANT UPDATE(payment_id) ON ledger_channel_payment_claims TO money_owner_runtime").Error; err != nil {
		t.Fatal(err)
	}
	if err := VerifyProviderTopUpRuntime(ctx, r.db); !errors.Is(err, m.ErrUnavailable) {
		t.Fatalf("immutable claim column mutation allowed: %v", err)
	}
}
