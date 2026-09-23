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
	secondReservation, err := repository.ReserveCommercialPurchase(ctx, ledgermoney.ReserveWalletFundsInput{OperationID: "order-2", OrganizationID: "org-a", CommercialOrderID: "order-2", Currency: "CNY", AmountMinor: 300})
	if err != nil || secondReservation.State != ledgermoney.WalletReservationReserved {
		t.Fatalf("second reservation=%#v err=%v", secondReservation, err)
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

func TestWalletEntryCursorUsesOccurredAtAndEntryIDOrdering(t *testing.T) {
	repository, db := walletRepository(t)
	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	rows := []organizationWalletEntryRow{
		{EntryID: "entry-a", OrganizationID: "org-cursor", Currency: "CNY", Kind: string(ledgermoney.WalletEntryPurchaseCommit), ReservedDelta: -1, ReservedAfter: 1, CommercialOrderID: "order-a", SourceIdentity: "source-a", OccurredAt: base},
		{EntryID: "entry-b", OrganizationID: "org-cursor", Currency: "CNY", Kind: string(ledgermoney.WalletEntryPurchaseCommit), ReservedDelta: -1, ReservedAfter: 0, CommercialOrderID: "order-b", SourceIdentity: "source-b", OccurredAt: base},
		{EntryID: "entry-c", OrganizationID: "org-cursor", Currency: "CNY", Kind: string(ledgermoney.WalletEntryPurchaseCommit), ReservedDelta: -1, ReservedAfter: 0, CommercialOrderID: "order-c", SourceIdentity: "source-c", OccurredAt: base.Add(-time.Minute)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	first, err := repository.ListOrganizationWalletEntries(context.Background(), "org-cursor", "CNY", "", 1)
	if err != nil || len(first.Items) != 1 || first.Items[0].EntryID != "entry-b" || first.NextCursor == "" {
		t.Fatalf("first page=%#v err=%v", first, err)
	}
	second, err := repository.ListOrganizationWalletEntries(context.Background(), "org-cursor", "CNY", first.NextCursor, 1)
	if err != nil || len(second.Items) != 1 || second.Items[0].EntryID != "entry-a" || second.NextCursor == "" {
		t.Fatalf("second page=%#v err=%v", second, err)
	}
	third, err := repository.ListOrganizationWalletEntries(context.Background(), "org-cursor", "CNY", second.NextCursor, 1)
	if err != nil || len(third.Items) != 1 || third.Items[0].EntryID != "entry-c" || third.NextCursor != "" {
		t.Fatalf("third page=%#v err=%v", third, err)
	}
}

func TestTopUpDebtRepaymentProducesValidImmutableEntry(t *testing.T) {
	repository, db := walletRepository(t)
	ctx := context.Background()
	settledAt := time.Now().UTC()
	if err := repository.RecordPaymentSettlement(ctx, ledgermoney.PaymentSettlement{PaymentID: "pay-debt", PayerUserID: "user-debt", Currency: "CNY", GrossAmountMinor: 100, CommissionableAmountMinor: 100, Status: ledgermoney.PaymentSettled, SettledAt: settledAt, ProviderReference: "provider-debt", Version: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreditSettledTopUp(ctx, "", ledgermoney.OrganizationTopUpSettlement{PaymentID: "pay-debt", CommercialOrderID: "order-debt", OrganizationID: "org-debt", Currency: "CNY", AmountMinor: 100, SettledAt: settledAt, ProviderReference: "provider-debt", Version: 1}); err != nil {
		t.Fatal(err)
	}
	reservation, err := repository.ReserveCommercialPurchase(ctx, ledgermoney.ReserveWalletFundsInput{OperationID: "purchase-debt", OrganizationID: "org-debt", CommercialOrderID: "purchase-debt", Currency: "CNY", AmountMinor: 100})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CommitCommercialPurchase(ctx, ledgermoney.CommitWalletReservationInput{OperationID: "commit-debt", OrganizationID: "org-debt", CommercialOrderID: "purchase-debt", ReservationID: reservation.ReservationID}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ApplyTopUpReversal(ctx, "", ledgermoney.OrganizationWalletReversal{ReversalID: "refund-debt", PaymentID: "pay-debt", CommercialOrderID: "order-debt", OrganizationID: "org-debt", Kind: ledgermoney.WalletReversalRefund, Currency: "CNY", AmountMinor: 100, OccurredAt: settledAt.Add(time.Minute), ProviderReference: "provider-refund-debt"}); err != nil {
		t.Fatal(err)
	}
	if err := repository.RecordPaymentSettlement(ctx, ledgermoney.PaymentSettlement{PaymentID: "pay-repay", PayerUserID: "user-debt", Currency: "CNY", GrossAmountMinor: 100, CommissionableAmountMinor: 100, Status: ledgermoney.PaymentSettled, SettledAt: settledAt.Add(2 * time.Minute), ProviderReference: "provider-repay", Version: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreditSettledTopUp(ctx, "", ledgermoney.OrganizationTopUpSettlement{PaymentID: "pay-repay", CommercialOrderID: "order-repay", OrganizationID: "org-debt", Currency: "CNY", AmountMinor: 100, SettledAt: settledAt.Add(2 * time.Minute), ProviderReference: "provider-repay", Version: 1}); err != nil {
		t.Fatal(err)
	}
	var entries []organizationWalletEntryRow
	if err := db.Where("payment_id = ?", "pay-repay").Find(&entries).Error; err != nil || len(entries) != 1 {
		t.Fatalf("entries=%#v err=%v", entries, err)
	}
	entry := walletEntry(entries[0])
	if entry.Kind != ledgermoney.WalletEntryDebtRepayment || entry.Validate() != nil || entry.DebtDelta != -100 {
		t.Fatalf("debt repayment entry=%#v validation=%v", entry, entry.Validate())
	}
}

func TestReleaseRepaysReversalDebtBeforeReturningAvailableFunds(t *testing.T) {
	repository, db := walletRepository(t)
	ctx := context.Background()
	settledAt := time.Now().UTC()
	if err := repository.RecordPaymentSettlement(ctx, ledgermoney.PaymentSettlement{PaymentID: "pay-release-debt", PayerUserID: "user-release-debt", Currency: "CNY", GrossAmountMinor: 100, CommissionableAmountMinor: 100, Status: ledgermoney.PaymentSettled, SettledAt: settledAt, ProviderReference: "provider-release-debt", Version: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreditSettledTopUp(ctx, "", ledgermoney.OrganizationTopUpSettlement{PaymentID: "pay-release-debt", CommercialOrderID: "topup-release-debt", OrganizationID: "org-release-debt", Currency: "CNY", AmountMinor: 100, SettledAt: settledAt, ProviderReference: "provider-release-debt", Version: 1}); err != nil {
		t.Fatal(err)
	}
	reservation, err := repository.ReserveCommercialPurchase(ctx, ledgermoney.ReserveWalletFundsInput{OperationID: "purchase-release-debt", OrganizationID: "org-release-debt", CommercialOrderID: "purchase-release-debt", Currency: "CNY", AmountMinor: 100})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ApplyTopUpReversal(ctx, "", ledgermoney.OrganizationWalletReversal{ReversalID: "refund-release-debt", PaymentID: "pay-release-debt", CommercialOrderID: "topup-release-debt", OrganizationID: "org-release-debt", Kind: ledgermoney.WalletReversalRefund, Currency: "CNY", AmountMinor: 100, OccurredAt: settledAt.Add(time.Minute), ProviderReference: "provider-refund-release-debt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ReleaseCommercialPurchase(ctx, ledgermoney.ReleaseWalletReservationInput{OperationID: "release-purchase", OrganizationID: "org-release-debt", CommercialOrderID: "purchase-release-debt", ReservationID: reservation.ReservationID, Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	wallet, err := repository.ReadOrganizationWallet(ctx, "org-release-debt", "CNY")
	if err != nil || wallet.AvailableMinor != 0 || wallet.ReservedMinor != 0 || wallet.DebtMinor != 0 {
		t.Fatalf("wallet after release=%#v err=%v", wallet, err)
	}
	var entryRow organizationWalletEntryRow
	if err := db.Where("commercial_order_id = ? AND entry_kind = ?", "purchase-release-debt", ledgermoney.WalletEntryPurchaseRelease).Take(&entryRow).Error; err != nil {
		t.Fatal(err)
	}
	entry := walletEntry(entryRow)
	if entry.AvailableDelta != 0 || entry.ReservedDelta != -100 || entry.DebtDelta != -100 || entry.Validate() != nil {
		t.Fatalf("release entry=%#v validation=%v", entry, entry.Validate())
	}
}

func TestTopUpEntriesPreserveReservedBalance(t *testing.T) {
	repository, db := walletRepository(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := repository.RecordPaymentSettlement(ctx, ledgermoney.PaymentSettlement{PaymentID: "pay-reserved-topup-a", PayerUserID: "user-reserved-topup", Currency: "CNY", GrossAmountMinor: 100, CommissionableAmountMinor: 100, Status: ledgermoney.PaymentSettled, SettledAt: now, ProviderReference: "provider-reserved-topup-a", Version: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreditSettledTopUp(ctx, "", ledgermoney.OrganizationTopUpSettlement{PaymentID: "pay-reserved-topup-a", CommercialOrderID: "order-reserved-topup-a", OrganizationID: "org-reserved-topup", Currency: "CNY", AmountMinor: 100, SettledAt: now, ProviderReference: "provider-reserved-topup-a", Version: 1}); err != nil {
		t.Fatal(err)
	}
	reservation, err := repository.ReserveCommercialPurchase(ctx, ledgermoney.ReserveWalletFundsInput{OperationID: "purchase-reserved-topup", OrganizationID: "org-reserved-topup", CommercialOrderID: "purchase-reserved-topup", Currency: "CNY", AmountMinor: 100})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ApplyTopUpReversal(ctx, "", ledgermoney.OrganizationWalletReversal{ReversalID: "refund-reserved-topup", PaymentID: "pay-reserved-topup-a", CommercialOrderID: "order-reserved-topup-a", OrganizationID: "org-reserved-topup", Kind: ledgermoney.WalletReversalRefund, Currency: "CNY", AmountMinor: 100, OccurredAt: now.Add(time.Minute), ProviderReference: "provider-refund-reserved-topup"}); err != nil {
		t.Fatal(err)
	}
	if err := repository.RecordPaymentSettlement(ctx, ledgermoney.PaymentSettlement{PaymentID: "pay-reserved-topup-b", PayerUserID: "user-reserved-topup", Currency: "CNY", GrossAmountMinor: 150, CommissionableAmountMinor: 150, Status: ledgermoney.PaymentSettled, SettledAt: now.Add(2 * time.Minute), ProviderReference: "provider-reserved-topup-b", Version: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreditSettledTopUp(ctx, "", ledgermoney.OrganizationTopUpSettlement{PaymentID: "pay-reserved-topup-b", CommercialOrderID: "order-reserved-topup-b", OrganizationID: "org-reserved-topup", Currency: "CNY", AmountMinor: 150, SettledAt: now.Add(2 * time.Minute), ProviderReference: "provider-reserved-topup-b", Version: 1}); err != nil {
		t.Fatal(err)
	}
	if reservation.State != ledgermoney.WalletReservationReserved {
		t.Fatalf("reservation unexpectedly changed: %#v", reservation)
	}
	for _, kind := range []ledgermoney.WalletEntryKind{ledgermoney.WalletEntryDebtRepayment, ledgermoney.WalletEntryTopUpCredit} {
		var row organizationWalletEntryRow
		if err := db.Where("payment_id = ? AND entry_kind = ?", "pay-reserved-topup-b", kind).Take(&row).Error; err != nil {
			t.Fatal(err)
		}
		entry := walletEntry(row)
		if entry.ReservedAfter != 100 || entry.Validate() != nil {
			t.Fatalf("%s entry lost reserved snapshot: %#v validation=%v", kind, entry, entry.Validate())
		}
	}
}

func TestWalletEntryPagesRejectOversizedLimit(t *testing.T) {
	repository, _ := walletRepository(t)
	if _, err := repository.ListOrganizationWalletEntries(context.Background(), "org-page-limit", ledgermoney.WalletCurrencyCNY, "", ledgermoney.MaxOrganizationWalletEntryPageSize+1); !errors.Is(err, ledgermoney.ErrInvalid) {
		t.Fatalf("oversized wallet entry page error = %v; want invalid", err)
	}
}

func mustWalletError(_ ledgermoney.OrganizationWalletSnapshot, err error) error { return err }
