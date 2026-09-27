package money

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	m "task-processor/internal/ledger/money"
)

func providerTopUpInput() m.ProviderTopUpInput {
	return m.ProviderTopUpInput{
		OperationID: "order-1", OrganizationID: "org-1", CommercialOrderID: "order-1", Currency: "CNY", AmountMinor: 10000,
		Binding: m.ProviderPaymentBinding{Provider: "ALIPAY", Environment: "PRODUCTION", MerchantID: "merchant-1", AppID: "app-1", TradeID: "trade-1"},
		Payment: m.PaymentSettlement{PaymentID: "payment-1", PaymentPurpose: m.PaymentPurposeWalletTopUp, CommissionTreatment: m.CommissionNonCommissionable, PayerBinding: m.PayerUnattributedExternal, Currency: "CNY", GrossAmountMinor: 10000, Status: m.PaymentSettled, SettledAt: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC), ProviderReference: "evidence-1", Version: 1},
	}
}

func providerReversal(kind m.WalletReversalKind, id string, amount int64) m.OrganizationWalletReversal {
	return m.OrganizationWalletReversal{ReversalID: id, PaymentID: "payment-1", CommercialOrderID: "order-1", OrganizationID: "org-1", Kind: kind, Currency: "CNY", AmountMinor: amount, OccurredAt: time.Date(2026, 9, 27, 1, 0, 0, 0, time.UTC), ProviderReference: "evidence-" + id}
}

func TestProviderTopUpExactReceiptAndImmutableReplay(t *testing.T) {
	r, _ := walletRepository(t)
	ctx := context.Background()
	input := providerTopUpInput()
	first, err := r.AcceptAndPostProviderTopUp(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.GrossCreditMinor != 10000 || first.AvailableAddedMinor != 10000 || first.DebtRepaidMinor != 0 || first.Validate() != nil {
		t.Fatalf("receipt: %+v", first)
	}
	_, err = r.ReserveCommercialPurchase(ctx, m.ReserveWalletFundsInput{OperationID: "purchase-1", OrganizationID: "org-1", CommercialOrderID: "purchase-1", Currency: "CNY", AmountMinor: 500})
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.AcceptAndPostProviderTopUp(ctx, input)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("replay changed: %+v %v", second, err)
	}
	read, err := r.ReadTopUpPosting(ctx, "org-1", "order-1", "payment-1")
	if err != nil || !reflect.DeepEqual(first, read) {
		t.Fatalf("readback changed: %+v %v", read, err)
	}
	if _, err := r.ReadTopUpPosting(ctx, "org-2", "order-1", "payment-1"); !errors.Is(err, m.ErrNotFound) {
		t.Fatalf("cross-org read: %v", err)
	}
	input.AmountMinor--
	if _, err := r.AcceptAndPostProviderTopUp(ctx, input); !errors.Is(err, m.ErrInvalid) {
		t.Fatalf("inexact amount: %v", err)
	}
	input = providerTopUpInput()
	input.CommercialOrderID = "other-order"
	input.OperationID = "other-order"
	input.Payment.PaymentID = "another-payment"
	if _, err := r.AcceptAndPostProviderTopUp(ctx, input); !errors.Is(err, m.ErrConflict) {
		t.Fatalf("same provider transaction claimed twice: %v", err)
	}
}

func TestProviderTopUpInFlightRefundAfterExternalReversal(t *testing.T) {
	r, _ := walletRepository(t)
	ctx := context.Background()
	if _, err := r.AcceptAndPostProviderTopUp(ctx, providerTopUpInput()); err != nil {
		t.Fatal(err)
	}
	key := m.TopUpReversalKey{PaymentID: "payment-1", Kind: m.WalletReversalRefund, ReversalID: "r1"}
	input := m.TopUpRefundInput{Key: key, OrganizationID: "org-1", CommercialOrderID: "order-1", AmountMinor: 6000, ApprovalID: "approval-1"}
	hold, err := r.PrepareTopUpRefund(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.AdmitTopUpRefund(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, err = r.AcceptProviderTopUpReversal(ctx, providerReversal(m.WalletReversalChargeback, "c1", 5000)); err != nil {
		t.Fatal(err)
	}
	wallet, _ := r.ReadOrganizationWallet(ctx, "org-1", "CNY")
	if wallet.AvailableMinor != 0 || wallet.ReservedMinor != 6000 || wallet.DebtMinor != 1000 {
		t.Fatalf("before confirmation: %+v", wallet)
	}
	receipt, err := r.AcceptProviderTopUpReversal(ctx, providerReversal(m.WalletReversalRefund, "r1", 6000))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ProviderAmountMinor != 6000 || receipt.WalletPrincipalEffectMinor != 5000 || receipt.ExcessProviderMinor != 1000 || receipt.HoldConsumedMinor != 5000 || receipt.HoldReleasedMinor != 1000 || receipt.HoldDebtRepaidMinor != 1000 || receipt.HoldID != hold.HoldID || receipt.Validate() != nil {
		t.Fatalf("receipt: %+v", receipt)
	}
	wallet, _ = r.ReadOrganizationWallet(ctx, "org-1", "CNY")
	if wallet.AvailableMinor != 0 || wallet.ReservedMinor != 0 || wallet.DebtMinor != 0 {
		t.Fatalf("final wallet: %+v", wallet)
	}
	for range 2 {
		replay, err := r.AcceptProviderTopUpReversal(ctx, providerReversal(m.WalletReversalRefund, "r1", 6000))
		if err != nil || !reflect.DeepEqual(receipt, replay) {
			t.Fatalf("replay: %+v %v", replay, err)
		}
		read, err := r.ReadTopUpReversal(ctx, "org-1", "order-1", key)
		if err != nil || !reflect.DeepEqual(receipt, read) {
			t.Fatalf("read: %+v %v", read, err)
		}
	}
	input.Key.ReversalID = "r2"
	input.AmountMinor = 1
	if _, err := r.PrepareTopUpRefund(ctx, input); !errors.Is(err, m.ErrWalletInsufficientBalance) {
		t.Fatalf("new refund past principal: %v", err)
	}
}

func TestProviderTopUpTypedReversalCollisionAndFullFacts(t *testing.T) {
	r, db := walletRepository(t)
	ctx := context.Background()
	if _, err := r.AcceptAndPostProviderTopUp(ctx, providerTopUpInput()); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordRefundSettlement(ctx, m.RefundSettlement{RefundID: "full", PaymentID: "payment-1", AmountMinor: 10000, OccurredAt: time.Now(), ProviderReference: "full-refund"}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []m.WalletReversalKind{m.WalletReversalRefund, m.WalletReversalChargeback} {
		receipt, err := r.AcceptProviderTopUpReversal(ctx, providerReversal(kind, "r1", 1))
		if err != nil {
			t.Fatal(err)
		}
		if receipt.WalletPrincipalEffectMinor != 0 || receipt.ExcessProviderMinor != 1 || receipt.Validate() != nil {
			t.Fatalf("receipt %+v", receipt)
		}
		key := m.TopUpReversalKey{PaymentID: "payment-1", Kind: kind, ReversalID: "r1"}
		read, err := r.ReadTopUpReversal(ctx, "org-1", "order-1", key)
		if err != nil || !reflect.DeepEqual(receipt, read) {
			t.Fatalf("read: %+v %v", read, err)
		}
		bad := providerReversal(kind, "r1", 2)
		if _, err := r.AcceptProviderTopUpReversal(ctx, bad); !errors.Is(err, m.ErrConflict) {
			t.Fatalf("same key different payload: %v", err)
		}
	}
	if _, err := r.ReadTopUpReversal(ctx, "org-1", "order-1", m.TopUpReversalKey{PaymentID: "payment-1", ReversalID: "r1"}); !errors.Is(err, m.ErrInvalid) {
		t.Fatalf("untyped read: %v", err)
	}
	var entries int64
	db.Model(&organizationWalletEntryRow{}).Count(&entries)
	if entries != 2 {
		t.Fatalf("zero effect generated entries: %d", entries)
	}
	wallet, _ := r.ReadOrganizationWallet(ctx, "org-1", "CNY")
	if wallet.AvailableMinor != 0 || wallet.DebtMinor != 0 {
		t.Fatalf("wallet %+v", wallet)
	}
}

func TestProviderTopUpKnownRefundBeforePosting(t *testing.T) {
	r, _ := walletRepository(t)
	ctx := context.Background()
	input := providerTopUpInput()
	input.KnownReversals = []m.OrganizationWalletReversal{providerReversal(m.WalletReversalRefund, "full", 10000)}
	if _, err := r.AcceptAndPostProviderTopUp(ctx, input); err != nil {
		t.Fatal(err)
	}
	wallet, _ := r.ReadOrganizationWallet(ctx, "org-1", "CNY")
	if wallet.AvailableMinor != 0 || wallet.DebtMinor != 0 {
		t.Fatalf("refunded money exposed: %+v", wallet)
	}
}

func TestProviderTopUpDebtOnlyReceiptHasNoSpendableCredit(t *testing.T) {
	r, db := walletRepository(t)
	ctx := context.Background()
	if _, err := r.AcceptAndPostProviderTopUp(ctx, providerTopUpInput()); err != nil {
		t.Fatal(err)
	}
	held, err := r.ReserveCommercialPurchase(ctx, m.ReserveWalletFundsInput{OperationID: "spent", OrganizationID: "org-1", CommercialOrderID: "spent", Currency: "CNY", AmountMinor: 10000})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.CommitCommercialPurchase(ctx, m.CommitWalletReservationInput{OperationID: "commit", OrganizationID: "org-1", CommercialOrderID: "spent", ReservationID: held.ReservationID}); err != nil {
		t.Fatal(err)
	}
	if _, err = r.AcceptProviderTopUpReversal(ctx, providerReversal(m.WalletReversalChargeback, "charged-back", 10000)); err != nil {
		t.Fatal(err)
	}
	input := providerTopUpInput()
	input.OperationID = "order-2"
	input.CommercialOrderID = "order-2"
	input.Binding.TradeID = "trade-2"
	input.Payment.PaymentID = "payment-2"
	input.Payment.ProviderReference = "evidence-2"
	input.AmountMinor = 5000
	input.Payment.GrossAmountMinor = 5000
	receipt, err := r.AcceptAndPostProviderTopUp(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.DebtRepaidMinor != 5000 || receipt.AvailableAddedMinor != 0 || len(receipt.CreditEntryIDs) != 1 || receipt.Validate() != nil {
		t.Fatalf("receipt %+v", receipt)
	}
	var credits int64
	if err = db.Model(&organizationWalletEntryRow{}).Where("commercial_order_id = ? AND entry_kind = ?", "order-2", string(m.WalletEntryTopUpCredit)).Count(&credits).Error; err != nil || credits != 0 {
		t.Fatalf("phantom credit: %d %v", credits, err)
	}
	snapshot, err := r.ReadOrganizationWallet(ctx, "org-1", "CNY")
	if err != nil || snapshot.AvailableMinor != 0 || snapshot.DebtMinor != 5000 {
		t.Fatalf("wallet %+v %v", snapshot, err)
	}
}
