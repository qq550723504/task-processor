//go:build integration

package money

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	m "task-processor/internal/ledger/money"
)

func TestOrdinaryReversalPostgresRuntime(t *testing.T) {
	ctx, db, r, observer := newMoneyPostgresRuntime(t)
	// Prove the reversal transaction supplies READ COMMITTED itself rather
	// than relying on a deployment's session default for post-lock visibility.
	if err := db.Exec("ALTER ROLE money_owner_runtime SET default_transaction_isolation = 'repeatable read'").Error; err != nil {
		t.Fatal(err)
	}
	pool, err := r.db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxIdleConns(0)
	var isolation string
	if err := r.db.Raw("SHOW default_transaction_isolation").Scan(&isolation).Error; err != nil || isolation != "repeatable read" {
		t.Fatalf("fixture isolation=%q: %v", isolation, err)
	}
	newPayment := func(t *testing.T) m.PaymentSettlement {
		t.Helper()
		p := paymentFact()
		p.PaymentID = t.Name()
		if err := r.RecordPaymentSettlement(ctx, p); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// This is the serving role, including the immutable column-UPDATE guard,
	// rather than the schema installer used to arrange the isolated fixture.
	if err := VerifyProviderTopUpRuntime(ctx, r.db); err != nil {
		t.Fatal(err)
	}
	t.Run("refund", func(t *testing.T) {
		p := newPayment(t)
		in := m.RefundSettlement{RefundID: t.Name(), PaymentID: p.PaymentID, AmountMinor: 4000, OccurredAt: time.Now().UTC(), ProviderReference: "controlled-refund"}
		if err := r.RecordRefundSettlement(ctx, in); err != nil {
			t.Fatalf("ordinary refund with immutable runtime role: %v", err)
		}
		if err := r.RecordRefundSettlement(ctx, in); err != nil {
			t.Fatalf("refund replay: %v", err)
		}
		in.AmountMinor++
		if err := r.RecordRefundSettlement(ctx, in); !errors.Is(err, m.ErrConflict) {
			t.Fatalf("refund conflict: %v", err)
		}
		in.RefundID += "-remainder"
		in.AmountMinor = 6000
		for range 2 {
			if err := r.RecordRefundSettlement(ctx, in); err != nil {
				t.Fatalf("full refund and replay: %v", err)
			}
		}
		in.RefundID += "-excess"
		in.AmountMinor = 1
		if err := r.RecordRefundSettlement(ctx, in); !errors.Is(err, m.ErrInvalid) {
			t.Fatalf("excess refund: %v", err)
		}
	})
	t.Run("chargeback", func(t *testing.T) {
		p := newPayment(t)
		in := m.ChargebackSettlement{ChargebackID: t.Name(), PaymentID: p.PaymentID, AmountMinor: 4000, OccurredAt: time.Now().UTC(), ProviderReference: "controlled-chargeback"}
		if err := r.RecordChargebackSettlement(ctx, in); err != nil {
			t.Fatalf("ordinary chargeback with immutable runtime role: %v", err)
		}
		if err := r.RecordChargebackSettlement(ctx, in); err != nil {
			t.Fatalf("chargeback replay: %v", err)
		}
		in.AmountMinor++
		if err := r.RecordChargebackSettlement(ctx, in); !errors.Is(err, m.ErrConflict) {
			t.Fatalf("chargeback conflict: %v", err)
		}
	})
	t.Run("mixed_concurrency", func(t *testing.T) {
		p := newPayment(t)
		start := make(chan struct{})
		results := make(chan error, 8)
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				<-start
				id := fmt.Sprintf("%s-%d", t.Name(), i)
				if i%2 == 0 {
					results <- r.RecordRefundSettlement(ctx, m.RefundSettlement{RefundID: id, PaymentID: p.PaymentID, AmountMinor: 3000, OccurredAt: time.Now().UTC(), ProviderReference: id})
				} else {
					results <- r.RecordChargebackSettlement(ctx, m.ChargebackSettlement{ChargebackID: id, PaymentID: p.PaymentID, AmountMinor: 3000, OccurredAt: time.Now().UTC(), ProviderReference: id})
				}
			})
		}
		close(start)
		wg.Wait()
		close(results)
		accepted, rejected := 0, 0
		for err := range results {
			switch {
			case err == nil:
				accepted++
			case errors.Is(err, m.ErrInvalid):
				rejected++
			default:
				t.Fatalf("unexpected concurrent reversal error: %v", err)
			}
		}
		if accepted != 3 || rejected != 5 {
			t.Fatalf("accepted=%d rejected=%d, want 3/5 within 10000", accepted, rejected)
		}
		last := m.RefundSettlement{RefundID: t.Name() + "-remainder", PaymentID: p.PaymentID, AmountMinor: 1000, OccurredAt: time.Now().UTC(), ProviderReference: "remainder"}
		for range 2 {
			if err := r.RecordRefundSettlement(ctx, last); err != nil {
				t.Fatalf("remainder and replay at cap: %v", err)
			}
		}
		var total int64
		if err := db.Raw(`SELECT COALESCE(SUM(amount_minor),0) FROM (
 SELECT amount_minor FROM ledger_refund_settlements WHERE payment_id=?
 UNION ALL SELECT amount_minor FROM ledger_chargeback_settlements WHERE payment_id=?) reversals`, p.PaymentID, p.PaymentID).Scan(&total).Error; err != nil || total != 10000 {
			t.Fatalf("combined reversal facts=%d: %v", total, err)
		}
	})
	t.Run("canceled_wait", func(t *testing.T) {
		p := newPayment(t)
		holder := db.Begin()
		if holder.Error != nil {
			t.Fatal(holder.Error)
		}
		defer holder.Rollback()
		if err := holder.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "money:ordinary-reversal:"+p.PaymentID).Error; err != nil {
			t.Fatal(err)
		}
		in := m.RefundSettlement{RefundID: t.Name(), PaymentID: p.PaymentID, AmountMinor: 1000, OccurredAt: time.Now().UTC(), ProviderReference: "canceled-wait"}
		waiting, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
		defer cancel()
		err := r.RecordRefundSettlement(waiting, in)
		if !errors.Is(waiting.Err(), context.DeadlineExceeded) || err == nil {
			t.Fatalf("must wait for the payment lock until cancellation: context=%v result=%v", waiting.Err(), err)
		}
		var count int64
		if err := db.Model(&refundRow{}).Where("refund_id=?", in.RefundID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("canceled request committed a refund: %d %v", count, err)
		}
		if err := holder.Rollback().Error; err != nil {
			t.Fatal(err)
		}
		if err := r.RecordRefundSettlement(ctx, in); err != nil {
			t.Fatalf("request after canceled waiter and lock release: %v", err)
		}
	})
	t.Run("referral_observer", func(t *testing.T) {
		// Arrange a consumed registration only in the disposable test database.
		// Runtime mutations below use the independent money/referral serving roles.
		if err := db.Exec(`INSERT INTO referral_codes VALUES ('issuer','promoter','test-code');
 INSERT INTO registration_intents(id,issuer,instance,organization,subject,referrer,key_hash,email_hash,fingerprint,secret_hash,key_id,created_at,create_expires_at,completion_expires_at,lease_until,state)
 VALUES ('test-intent','issuer','instance','org','user-1','promoter','key','email','fingerprint','secret','key-id',now(),now()+interval '15 minutes',now()+interval '24 hours',now(),'CONSUMED');
 INSERT INTO referral_relations VALUES ('issuer','user-1','promoter','test-intent',now());`).Error; err != nil {
			t.Fatal(err)
		}
		p := newPayment(t)
		if err := r.RecordPaymentSettlementAndNotify(ctx, p, observer); err != nil {
			t.Fatal(err)
		}
		refund := m.RefundSettlement{RefundID: t.Name(), PaymentID: p.PaymentID, AmountMinor: 4000, OccurredAt: time.Now().UTC(), ProviderReference: "observer-refund"}
		// Model interruption after the canonical commit, before notification.
		if err := r.RecordRefundSettlement(ctx, refund); err != nil {
			t.Fatal(err)
		}
		beforeNotify, err := observer.ReadEarningsSnapshot(ctx, "promoter", "CNY", 100)
		if err != nil || beforeNotify.Earnings.PendingMinor != 1000 || len(beforeNotify.Entries) != 1 {
			t.Fatalf("canonical-only stage=%+v: %v", beforeNotify, err)
		}
		var replay sync.WaitGroup
		results := make(chan error, 4)
		for range 4 {
			replay.Go(func() { results <- r.RecordRefundSettlementAndNotify(ctx, refund, observer) })
		}
		replay.Wait()
		close(results)
		for err := range results {
			if err != nil {
				t.Fatalf("concurrent canonical and observer replay: %v", err)
			}
		}
		partial, err := observer.ReadEarningsSnapshot(ctx, "promoter", "CNY", 100)
		if err != nil || partial.Earnings.PendingMinor != 600 || len(partial.Entries) != 2 {
			t.Fatalf("partial reversal projection=%+v: %v", partial, err)
		}
		chargeback := m.ChargebackSettlement{ChargebackID: t.Name(), PaymentID: p.PaymentID, AmountMinor: 6000, OccurredAt: time.Now().UTC(), ProviderReference: "observer-chargeback"}
		for range 2 {
			if err := r.RecordChargebackSettlementAndNotify(ctx, chargeback, observer); err != nil {
				t.Fatal(err)
			}
		}
		full, err := observer.ReadEarningsSnapshot(ctx, "promoter", "CNY", 100)
		if err != nil || full.Earnings.PendingMinor != 0 || full.Earnings.AvailableMinor != 0 || full.Earnings.ReservedMinor != 0 || len(full.Entries) != 3 {
			t.Fatalf("full reversal projection=%+v: %v", full, err)
		}
		if err := VerifyProviderTopUpRuntime(ctx, r.db); err != nil {
			t.Fatalf("immutable runtime grants changed: %v", err)
		}
	})
}
