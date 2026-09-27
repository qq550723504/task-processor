package referral

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"

	money "task-processor/internal/ledger/money"
	economics "task-processor/internal/referraleconomics"
)

// This fixture exercises repository transactions and calculations, not PostgreSQL
// permissions or concurrency. A single connection owns its private public schema.
func settlementRepository(t *testing.T) (*Repository, money.PaymentSettlement) {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	if err := db.Exec("ATTACH DATABASE ':memory:' AS public").Error; err != nil {
		t.Fatal(err)
	}
	for _, model := range []any{&canonicalPaymentRow{}, &canonicalRefundRow{}, &canonicalChargebackRow{}, &earningClaim{}, &earningLedgerEntry{}, &earningProjection{}, &economicsAuditRow{}, &refundOperationRow{}, &chargebackOperationRow{}} {
		if err := db.AutoMigrate(model); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("CREATE TABLE public.referral_relations (issuer TEXT, subject TEXT, referrer TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO public.referral_relations VALUES ('issuer', 'payer', 'promoter')").Error; err != nil {
		t.Fatal(err)
	}
	p := money.PaymentSettlement{PaymentID: "payment", PayerUserID: "payer", Currency: "CNY", GrossAmountMinor: 10000, CommissionableAmountMinor: 10000, Status: money.PaymentSettled, SettledAt: time.Date(2026, 9, 27, 12, 0, 0, 0, time.FixedZone("UTC+8", 8*3600)), ProviderReference: "confirmed-payment", Version: 1}
	canonical := canonicalPaymentRow{PaymentID: p.PaymentID, PayerUserID: p.PayerUserID, Currency: p.Currency, GrossAmountMinor: p.GrossAmountMinor, CommissionableAmountMinor: p.CommissionableAmountMinor, Status: string(p.Status), SettledAt: p.SettledAt.UTC(), ProviderReference: p.ProviderReference, Version: p.Version}
	if err := db.Create(&canonical).Error; err != nil {
		t.Fatal(err)
	}
	return &Repository{db: db}, p
}

func settlementEarnings(t *testing.T, r *Repository) economics.Earnings {
	t.Helper()
	got, err := r.ReadEarnings(context.Background(), "promoter", "CNY")
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestSettlementMaturesAtThirtyDaysOnlyOnce(t *testing.T) {
	r, payment := settlementRepository(t)
	ctx := context.Background()
	if err := r.RecordSettledPayment(ctx, payment); err != nil {
		t.Fatal(err)
	}
	deadline := payment.SettledAt.UTC().Add(30 * 24 * time.Hour)
	var claim earningClaim
	if err := r.db.Take(&claim).Error; err != nil {
		t.Fatal(err)
	}
	if !claim.AvailableAt.Equal(deadline) {
		t.Errorf("available at %s, want %s (30 days from canonical settlement)", claim.AvailableAt, deadline)
	}
	for _, at := range []time.Time{payment.SettledAt.UTC().Add(14 * 24 * time.Hour), deadline.Add(-time.Microsecond)} {
		if err := r.Mature(ctx, at); err != nil {
			t.Fatal(err)
		}
		got := settlementEarnings(t, r)
		if got.PendingMinor != 1000 || got.AvailableMinor != 0 {
			t.Fatalf("premature earnings at %s: %+v", at, got)
		}
	}
	if err := r.Mature(ctx, deadline); err != nil {
		t.Fatal(err)
	}
	first := settlementEarnings(t, r)
	if first.PendingMinor != 0 || first.AvailableMinor != 1000 {
		t.Fatalf("due earnings: %+v", first)
	}
	if err := r.RecordSettledPayment(ctx, payment); err != nil {
		t.Fatal(err)
	}
	if err := r.Mature(ctx, deadline.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := settlementEarnings(t, r); !reflect.DeepEqual(got, first) {
		t.Fatalf("replay changed earnings: before=%+v after=%+v", first, got)
	}
	for _, table := range []string{"public.referral_earning_claims", "public.referral_earnings_ledger", "public.referral_earnings_audit_events"} {
		var count int64
		if err := r.db.Table(table).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s count=%d err=%v", table, count, err)
		}
	}
}

func TestSettlementReplayKeepsSavedDeadlineAndRejectsChangedFacts(t *testing.T) {
	r, payment := settlementRepository(t)
	ctx := context.Background()
	if err := r.RecordSettledPayment(ctx, payment); err != nil {
		t.Fatal(err)
	}
	// Model a previously committed deadline independently of the current policy.
	stored := payment.SettledAt.UTC().Add(21 * 24 * time.Hour)
	if err := r.db.Model(&earningClaim{}).Where("payment_id=?", payment.PaymentID).Update("available_at", stored).Error; err != nil {
		t.Fatal(err)
	}
	var before, after earningClaim
	if err := r.db.Take(&before).Error; err != nil {
		t.Fatal(err)
	}
	projection := settlementEarnings(t, r)
	if err := r.RecordSettledPayment(ctx, payment); err != nil {
		t.Fatalf("exact canonical replay re-derived the saved deadline: %v", err)
	}
	if err := r.db.Take(&after).Error; err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("claim was rewritten: before=%+v after=%+v err=%v", before, after, err)
	}
	if got := settlementEarnings(t, r); !reflect.DeepEqual(projection, got) {
		t.Fatalf("replay changed projection: %+v", got)
	}
	for _, change := range []func(*money.PaymentSettlement){
		func(p *money.PaymentSettlement) { p.SettledAt = p.SettledAt.Add(time.Second) },
		func(p *money.PaymentSettlement) { p.CommissionableAmountMinor-- },
		func(p *money.PaymentSettlement) { p.PayerUserID = "other-payer" },
	} {
		changed := payment
		change(&changed)
		if err := r.RecordSettledPayment(ctx, changed); !errors.Is(err, economics.ErrInvalid) {
			t.Fatalf("changed canonical fact accepted: %v", err)
		}
	}
	if err := r.db.Exec("UPDATE public.referral_relations SET referrer='other-promoter'").Error; err != nil {
		t.Fatal(err)
	}
	if err := r.RecordSettledPayment(ctx, payment); !errors.Is(err, economics.ErrConflict) {
		t.Fatalf("changed attribution accepted: %v", err)
	}
	for _, table := range []string{"public.referral_earnings_ledger", "public.referral_earnings_audit_events"} {
		var count int64
		if err := r.db.Table(table).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s count=%d err=%v", table, count, err)
		}
	}
}

func TestSettlementReversalsRespectThirtyDayMaturity(t *testing.T) {
	for _, kind := range []string{"refund", "chargeback"} {
		for _, matured := range []bool{false, true} {
			name := kind + "/pending"
			if matured {
				name = kind + "/available"
			}
			t.Run(name, func(t *testing.T) {
				r, p := settlementRepository(t)
				ctx := context.Background()
				if err := r.RecordSettledPayment(ctx, p); err != nil {
					t.Fatal(err)
				}
				deadline := p.SettledAt.UTC().Add(30 * 24 * time.Hour)
				at := deadline.Add(-time.Hour)
				if matured {
					if err := r.Mature(ctx, deadline); err != nil {
						t.Fatal(err)
					}
					at = deadline.Add(time.Hour)
				}
				var reverse func() error
				if kind == "refund" {
					fact := money.RefundSettlement{RefundID: "refund", PaymentID: p.PaymentID, AmountMinor: 4000, OccurredAt: at, ProviderReference: "confirmed-refund"}
					if err := r.db.Create(&canonicalRefundRow{RefundID: fact.RefundID, PaymentID: fact.PaymentID, AmountMinor: fact.AmountMinor, OccurredAt: at, ProviderReference: fact.ProviderReference}).Error; err != nil {
						t.Fatal(err)
					}
					reverse = func() error { return r.RecordRefund(ctx, fact) }
				} else {
					fact := money.ChargebackSettlement{ChargebackID: "chargeback", PaymentID: p.PaymentID, AmountMinor: 4000, OccurredAt: at, ProviderReference: "confirmed-chargeback"}
					if err := r.db.Create(&canonicalChargebackRow{ChargebackID: fact.ChargebackID, PaymentID: fact.PaymentID, AmountMinor: fact.AmountMinor, OccurredAt: at, ProviderReference: fact.ProviderReference}).Error; err != nil {
						t.Fatal(err)
					}
					reverse = func() error { return r.RecordChargeback(ctx, fact) }
				}
				if err := reverse(); err != nil {
					t.Fatal(err)
				}
				first := settlementEarnings(t, r)
				if err := reverse(); err != nil {
					t.Fatal(err)
				}
				if got := settlementEarnings(t, r); !reflect.DeepEqual(got, first) {
					t.Fatalf("duplicate reversal changed earnings: %+v", got)
				}
				if !matured {
					if first.PendingMinor != 600 || first.AvailableMinor != 0 {
						t.Fatalf("pending reversal: %+v", first)
					}
					if err := r.Mature(ctx, deadline); err != nil {
						t.Fatal(err)
					}
				}
				final := settlementEarnings(t, r)
				if final.PendingMinor != 0 || final.AvailableMinor+final.AdjustmentMinor != 600 {
					t.Fatalf("net earnings after maturity/reversal: %+v", final)
				}
			})
		}
	}
}
