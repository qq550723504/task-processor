package money

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	ledgermoney "task-processor/internal/ledger/money"
)

func walletRepository(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:commercial-wallet-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repository, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	return repository, db
}

func TestOrganizationWalletTopUpReserveAndCommitAreIdempotent(t *testing.T) {
	repository, _ := walletRepository(t)
	ctx := context.Background()
	payment := ledgermoney.PaymentSettlement{PaymentID: "pay-1", PayerUserID: "user-1", Currency: "CNY", GrossAmountMinor: 1000, CommissionableAmountMinor: 1000, Status: ledgermoney.PaymentSettled, SettledAt: time.Now().UTC(), ProviderReference: "provider-1", Version: 1}
	if err := repository.RecordPaymentSettlement(ctx, payment); err != nil {
		t.Fatal(err)
	}
	settlement := ledgermoney.OrganizationTopUpSettlement{PaymentID: "pay-1", CommercialOrderID: "order-topup-1", OrganizationID: "org-a", Currency: "CNY", AmountMinor: 1000, SettledAt: payment.SettledAt, ProviderReference: "provider-1", Version: 1}
	first, err := repository.CreditSettledTopUp(ctx, "", settlement)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.CreditSettledTopUp(ctx, "", settlement)
	if err != nil || first != second {
		t.Fatalf("replay snapshot=%#v err=%v", second, err)
	}
	reservation, err := repository.ReserveCommercialPurchase(ctx, ledgermoney.ReserveWalletFundsInput{OperationID: "order-1", OrganizationID: "org-a", CommercialOrderID: "order-1", Currency: "CNY", AmountMinor: 400})
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := repository.ReserveCommercialPurchase(ctx, ledgermoney.ReserveWalletFundsInput{OperationID: "order-1", OrganizationID: "org-a", CommercialOrderID: "order-1", Currency: "CNY", AmountMinor: 400})
	if err != nil || replayed.ReservationID != reservation.ReservationID {
		t.Fatalf("reservation replay=%#v err=%v", replayed, err)
	}
	if _, err := repository.CommitCommercialPurchase(ctx, ledgermoney.CommitWalletReservationInput{OperationID: "commit-1", OrganizationID: "org-a", CommercialOrderID: "order-1", ReservationID: reservation.ReservationID}); err != nil {
		t.Fatal(err)
	}
	wallet, err := repository.ReadOrganizationWallet(ctx, "org-a", "CNY")
	if err != nil || wallet.AvailableMinor != 600 || wallet.ReservedMinor != 0 || wallet.LifetimeSpendMinor != 400 {
		t.Fatalf("wallet=%#v err=%v", wallet, err)
	}
}

func TestOrganizationWalletReversalCreatesDebtAndRepaysOnNextTopUp(t *testing.T) {
	repository, _ := walletRepository(t)
	ctx := context.Background()
	settledAt := time.Now().UTC()
	if err := repository.RecordPaymentSettlement(ctx, ledgermoney.PaymentSettlement{PaymentID: "pay-2", PayerUserID: "user-2", Currency: "CNY", GrossAmountMinor: 700, CommissionableAmountMinor: 700, Status: ledgermoney.PaymentSettled, SettledAt: settledAt, ProviderReference: "provider-2", Version: 1}); err != nil {
		t.Fatal(err)
	}
	settlement := ledgermoney.OrganizationTopUpSettlement{PaymentID: "pay-2", CommercialOrderID: "order-topup-2", OrganizationID: "org-a", Currency: "CNY", AmountMinor: 700, SettledAt: settledAt, ProviderReference: "provider-2", Version: 1}
	if _, err := repository.CreditSettledTopUp(ctx, "", settlement); err != nil {
		t.Fatal(err)
	}
	reservation, err := repository.ReserveCommercialPurchase(ctx, ledgermoney.ReserveWalletFundsInput{OperationID: "order-spend-2", OrganizationID: "org-a", CommercialOrderID: "order-spend-2", Currency: "CNY", AmountMinor: 700})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CommitCommercialPurchase(ctx, ledgermoney.CommitWalletReservationInput{OperationID: "commit-spend-2", OrganizationID: "org-a", CommercialOrderID: "order-spend-2", ReservationID: reservation.ReservationID}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ApplyTopUpReversal(ctx, "", ledgermoney.OrganizationWalletReversal{ReversalID: "refund-2", PaymentID: "pay-2", CommercialOrderID: "order-topup-2", OrganizationID: "org-a", Kind: ledgermoney.WalletReversalRefund, Currency: "CNY", AmountMinor: 700, OccurredAt: settledAt.Add(time.Hour), ProviderReference: "provider-refund-2"}); err != nil {
		t.Fatal(err)
	}
	wallet, err := repository.ReadOrganizationWallet(ctx, "org-a", "CNY")
	if err != nil || wallet.AvailableMinor != 0 || wallet.DebtMinor != 700 {
		t.Fatalf("reversed wallet=%#v err=%v", wallet, err)
	}
	if err := repository.RecordPaymentSettlement(ctx, ledgermoney.PaymentSettlement{PaymentID: "pay-3", PayerUserID: "user-2", Currency: "CNY", GrossAmountMinor: 300, CommissionableAmountMinor: 300, Status: ledgermoney.PaymentSettled, SettledAt: settledAt.Add(2 * time.Hour), ProviderReference: "provider-3", Version: 1}); err != nil {
		t.Fatal(err)
	}
	_, err = repository.CreditSettledTopUp(ctx, "", ledgermoney.OrganizationTopUpSettlement{PaymentID: "pay-3", CommercialOrderID: "order-topup-3", OrganizationID: "org-a", Currency: "CNY", AmountMinor: 300, SettledAt: settledAt.Add(2 * time.Hour), ProviderReference: "provider-3", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	wallet, err = repository.ReadOrganizationWallet(ctx, "org-a", "CNY")
	if err != nil || wallet.AvailableMinor != 0 || wallet.DebtMinor != 400 {
		t.Fatalf("repaid wallet=%#v err=%v", wallet, err)
	}
}

func TestOrganizationWalletRejectsInsufficientFundsAndConflictingSettlement(t *testing.T) {
	repository, _ := walletRepository(t)
	ctx := context.Background()
	settledAt := time.Now().UTC()
	if err := repository.RecordPaymentSettlement(ctx, ledgermoney.PaymentSettlement{PaymentID: "pay-4", PayerUserID: "user-4", Currency: "CNY", GrossAmountMinor: 100, CommissionableAmountMinor: 100, Status: ledgermoney.PaymentSettled, SettledAt: settledAt, ProviderReference: "provider-4", Version: 1}); err != nil {
		t.Fatal(err)
	}
	settlement := ledgermoney.OrganizationTopUpSettlement{PaymentID: "pay-4", CommercialOrderID: "order-topup-4", OrganizationID: "org-a", Currency: "CNY", AmountMinor: 100, SettledAt: settledAt, ProviderReference: "provider-4", Version: 1}
	if _, err := repository.CreditSettledTopUp(ctx, "", settlement); err != nil {
		t.Fatal(err)
	}
	conflict := settlement
	conflict.AmountMinor = 99
	if !errors.Is(mustWalletError(repository.CreditSettledTopUp(ctx, "", conflict)), ledgermoney.ErrConflict) {
		t.Fatal("expected settlement conflict")
	}
	_, err := repository.ReserveCommercialPurchase(ctx, ledgermoney.ReserveWalletFundsInput{OperationID: "order-4", OrganizationID: "org-a", CommercialOrderID: "order-4", Currency: "CNY", AmountMinor: 101})
	if !errors.Is(err, ledgermoney.ErrWalletInsufficientBalance) {
		t.Fatalf("reserve err=%v", err)
	}
}

func mustWalletError(_ ledgermoney.OrganizationWalletSnapshot, err error) error { return err }
