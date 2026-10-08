//go:build integration

package money

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	referralstore "task-processor/internal/integration/persistence/referral"
	m "task-processor/internal/ledger/money"

	"github.com/google/uuid"
	pg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMoneyPostgresRuntime(t *testing.T) (context.Context, *gorm.DB, *Repository, *referralstore.Repository) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	c, err := pg.Run(ctx, "postgres:16-alpine", pg.WithDatabase("topup"), pg.WithUsername("topup"), pg.WithPassword(uuid.NewString()), pg.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := referralstore.Install(ctx, db); err != nil {
		t.Fatal(err)
	}
	runtimePassword := uuid.NewString()
	if err := db.Exec(`CREATE ROLE referral_runtime LOGIN PASSWORD '` + runtimePassword + `'; REVOKE ALL ON SCHEMA public FROM PUBLIC; GRANT USAGE ON SCHEMA public TO referral_runtime; REVOKE CREATE,TEMP ON DATABASE topup FROM PUBLIC; GRANT CONNECT ON DATABASE topup TO referral_runtime; GRANT SELECT,INSERT ON ALL TABLES IN SCHEMA public TO referral_runtime; GRANT SELECT,INSERT,UPDATE ON TABLE public.referral_earning_claims, public.referral_earnings_projection, public.referral_withdrawals TO referral_runtime; GRANT UPDATE(state,ciphertext,lease_until) ON public.registration_intents TO referral_runtime; GRANT UPDATE,DELETE ON public.registration_admission_buckets TO referral_runtime; GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO referral_runtime`).Error; err != nil {
		t.Fatal(err)
	}
	runtimeURL, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("invalid fixture DSN")
	}
	runtimeURL.User = url.UserPassword("referral_runtime", runtimePassword)
	runtimeDB, err := gorm.Open(postgres.Open(runtimeURL.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("fixture runtime connection failed")
	}
	runtimePool, err := runtimeDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtimePool.Close() })
	observer, err := referralstore.New(runtimeDB)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`REVOKE INSERT ON public.ledger_payment_settlements,public.ledger_refund_settlements,public.ledger_chargeback_settlements FROM referral_runtime`).Error; err != nil {
		t.Fatal(err)
	}
	moneyPassword := uuid.NewString()
	if err := db.Exec(`CREATE ROLE money_owner_runtime LOGIN PASSWORD '` + moneyPassword + `'; GRANT CONNECT ON DATABASE topup TO money_owner_runtime; GRANT USAGE ON SCHEMA public TO money_owner_runtime;
 GRANT SELECT,INSERT ON public.ledger_payment_settlements,public.ledger_refund_settlements,public.ledger_chargeback_settlements,public.ledger_organization_wallet_entries,public.ledger_organization_wallet_reserve_decisions,public.ledger_organization_topup_settlements,public.ledger_organization_wallet_reversals,public.ledger_topup_reversal_receipts,public.ledger_topup_excess_reconciliations TO money_owner_runtime;
 GRANT SELECT,INSERT,UPDATE ON public.ledger_organization_wallets,public.ledger_organization_wallet_reservations,public.ledger_provider_topup_claims,public.ledger_topup_refund_holds TO money_owner_runtime;`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`GRANT SELECT,INSERT ON public.ledger_channel_payment_claims,public.ledger_service_effect_receipts TO money_owner_runtime; GRANT SELECT,INSERT,UPDATE ON public.ledger_service_payment_bindings,public.ledger_service_operation_reservations TO money_owner_runtime`).Error; err != nil {
		t.Fatal(err)
	}
	moneyURL, _ := url.Parse(dsn)
	moneyURL.User = url.UserPassword("money_owner_runtime", moneyPassword)
	moneyDB, err := gorm.Open(postgres.Open(moneyURL.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("fixture money connection failed")
	}
	moneyPool, _ := moneyDB.DB()
	t.Cleanup(func() { _ = moneyPool.Close() })
	if err = VerifyProviderTopUpRuntime(ctx, moneyDB); err != nil {
		t.Fatal(err)
	}
	r, _ := New(moneyDB)
	return ctx, db, r, observer
}

func TestProviderTopUpPostgresConcurrentReceiptsAndZeroEarnings(t *testing.T) {
	ctx, db, r, observer := newMoneyPostgresRuntime(t)
	input := providerTopUpInput()
	var wg sync.WaitGroup
	results := make(chan m.TopUpPostingReceipt, 8)
	failures := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			result, err := r.AcceptAndPostProviderTopUp(ctx, input)
			if err != nil {
				failures <- err
				return
			}
			results <- result
		})
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	var first m.TopUpPostingReceipt
	for result := range results {
		if first.ReceiptID == "" {
			first = result
		} else if !reflect.DeepEqual(first, result) {
			t.Fatal("concurrent posting receipts changed")
		}
	}
	if err := observer.ObservePaymentSettlement(ctx, input.Payment); err != nil {
		t.Fatal(err)
	}
	var claimCount int64
	if err := db.Table("referral_earning_claims").Count(&claimCount).Error; err != nil || claimCount != 0 {
		t.Fatalf("top-up earnings: %d %v", claimCount, err)
	}
	holdInput := m.TopUpRefundInput{Key: m.TopUpReversalKey{PaymentID: "payment-1", Kind: m.WalletReversalRefund, ReversalID: "refund-1"}, OrganizationID: "org-1", CommercialOrderID: "order-1", AmountMinor: 6000, ApprovalID: "approval-1"}
	if _, err := r.ReadTopUpRefundHold(ctx, holdInput); !errors.Is(err, m.ErrNotFound) {
		t.Fatalf("absent hold read: %v", err)
	}
	if _, err := r.PrepareTopUpRefund(ctx, holdInput); err != nil {
		t.Fatal(err)
	}
	if hold, err := r.ReadTopUpRefundHold(ctx, holdInput); err != nil || hold.Dispatched || hold.State != "RESERVED" {
		t.Fatalf("read changed admission: %+v %v", hold, err)
	}
	if _, err := r.AdmitTopUpRefund(ctx, holdInput); err != nil {
		t.Fatal(err)
	}
	if hold, err := r.ReadTopUpRefundHold(ctx, holdInput); err != nil || !hold.Dispatched || hold.Input != holdInput {
		t.Fatalf("admitted hold read: %+v %v", hold, err)
	}
	wrongHold := holdInput
	wrongHold.AmountMinor++
	if _, err := r.ReadTopUpRefundHold(ctx, wrongHold); !errors.Is(err, m.ErrConflict) {
		t.Fatalf("mismatched hold read: %v", err)
	}
	refund := m.RefundSettlement{PaymentID: "payment-1", RefundID: "refund-1", AmountMinor: 6000, OccurredAt: time.Now().UTC(), ProviderReference: "refund-evidence"}
	chargeback := m.ChargebackSettlement{PaymentID: "payment-1", ChargebackID: "refund-1", AmountMinor: 5000, OccurredAt: time.Now().UTC(), ProviderReference: "chargeback-evidence"}
	failures = make(chan error, 8)
	for range 4 {
		wg.Go(func() {
			if err := r.RecordRefundSettlementAndNotify(ctx, refund, observer); err != nil {
				failures <- err
			}
		})
		wg.Go(func() {
			if err := r.RecordChargebackSettlementAndNotify(ctx, chargeback, observer); err != nil {
				failures <- err
			}
		})
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	wallet, err := r.ReadOrganizationWallet(ctx, "org-1", "CNY")
	if err != nil || wallet.AvailableMinor != 0 || wallet.ReservedMinor != 0 || wallet.DebtMinor != 0 {
		t.Fatalf("wallet %+v %v", wallet, err)
	}
	var total struct {
		Provider  int64
		Principal int64
	}
	if err := db.Model(&topUpReversalReceiptRow{}).Select("SUM(provider_amount_minor) AS provider,SUM(wallet_principal_effect_minor) AS principal").Scan(&total).Error; err != nil || total.Provider != 11000 || total.Principal != 10000 {
		t.Fatalf("totals %+v %v", total, err)
	}
	for _, table := range []string{"referral_earning_claims", "referral_earnings_ledger", "referral_refund_operations", "referral_chargeback_operations"} {
		var n int64
		if err := db.Table(table).Count(&n).Error; err != nil || n != 0 {
			t.Fatalf("%s created earnings: %d %v", table, n, err)
		}
	}
	restarted, _ := New(db)
	read, err := restarted.ReadTopUpPosting(ctx, "org-1", "order-1", "payment-1")
	if err != nil || !reflect.DeepEqual(first, read) {
		t.Fatalf("restart receipt %+v %v", read, err)
	}
	for _, kind := range []m.WalletReversalKind{m.WalletReversalRefund, m.WalletReversalChargeback} {
		if _, err := restarted.ReadTopUpReversal(ctx, "org-1", "order-1", m.TopUpReversalKey{PaymentID: "payment-1", Kind: kind, ReversalID: "refund-1"}); err != nil {
			t.Fatal(err)
		}
	}
}
