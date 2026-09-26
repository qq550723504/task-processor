package money

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	money "task-processor/internal/ledger/money"
)

func newMoneyRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:money-owner-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repo, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func paymentFact() money.PaymentSettlement {
	return money.PaymentSettlement{PaymentID: "pay-1", PayerUserID: "user-1", Currency: "CNY", GrossAmountMinor: 12000, DiscountAmountMinor: 2000, CommissionableAmountMinor: 10000, Status: money.PaymentSettled, SettledAt: time.Date(2026, 9, 20, 1, 2, 3, 0, time.UTC), ProviderReference: "provider-pay-1", Version: 1}
}

func TestPaymentSettlementIsCanonicalAndPayloadConflicts(t *testing.T) {
	repo := newMoneyRepository(t)
	ctx := context.Background()
	p := paymentFact()
	p.SettledAt = p.SettledAt.Add(789 * time.Nanosecond)
	if err := repo.RecordPaymentSettlement(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordPaymentSettlement(ctx, p); err != nil {
		t.Fatal(err)
	}
	p.CommissionableAmountMinor--
	if !errors.Is(repo.RecordPaymentSettlement(ctx, p), money.ErrConflict) {
		t.Fatal("expected payment payload conflict")
	}
}

func TestRefundSettlementIsBoundedAndIdempotent(t *testing.T) {
	repo := newMoneyRepository(t)
	ctx := context.Background()
	if err := repo.RecordPaymentSettlement(ctx, paymentFact()); err != nil {
		t.Fatal(err)
	}
	r := money.RefundSettlement{RefundID: "refund-1", PaymentID: "pay-1", AmountMinor: 4000, OccurredAt: time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC), ProviderReference: "provider-refund-1"}
	if err := repo.RecordRefundSettlement(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordRefundSettlement(ctx, r); err != nil {
		t.Fatal(err)
	}
	r.RefundID, r.AmountMinor = "refund-2", 6000
	if err := repo.RecordRefundSettlement(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordRefundSettlement(ctx, r); err != nil {
		t.Fatalf("full refund replay should be idempotent: %v", err)
	}
	r.RefundID, r.AmountMinor = "refund-3", 1
	if !errors.Is(repo.RecordRefundSettlement(ctx, r), money.ErrInvalid) {
		t.Fatal("expected aggregate refund bound")
	}
	r.RefundID, r.AmountMinor = "refund-2", 6001
	if !errors.Is(repo.RecordRefundSettlement(ctx, r), money.ErrConflict) {
		t.Fatal("expected refund payload conflict")
	}
}

func TestChargebackSettlementIsCanonicalAndBounded(t *testing.T) {
	repo := newMoneyRepository(t)
	ctx := context.Background()
	if err := repo.RecordPaymentSettlement(ctx, paymentFact()); err != nil {
		t.Fatal(err)
	}
	chargeback := money.ChargebackSettlement{ChargebackID: "chargeback-1", PaymentID: "pay-1", AmountMinor: 4000, OccurredAt: time.Date(2026, 9, 22, 1, 0, 0, 123456789, time.UTC), ProviderReference: "provider-chargeback-1"}
	if err := repo.RecordChargebackSettlement(ctx, chargeback); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordChargebackSettlement(ctx, chargeback); err != nil {
		t.Fatalf("replay should be idempotent: %v", err)
	}
	chargeback.ChargebackID, chargeback.AmountMinor = "chargeback-2", 6000
	if err := repo.RecordChargebackSettlement(ctx, chargeback); err != nil {
		t.Fatal(err)
	}
	chargeback.ChargebackID, chargeback.AmountMinor = "chargeback-3", 1
	if !errors.Is(repo.RecordChargebackSettlement(ctx, chargeback), money.ErrInvalid) {
		t.Fatal("expected cumulative chargeback bound")
	}
}

func TestPayoutMethodValidationDoesNotExposeSecureReference(t *testing.T) {
	repo := newMoneyRepository(t)
	now := time.Now().UTC()
	method := money.PayoutMethod{MethodID: "method-1", SubjectUserID: "user-1", Type: money.PayoutAlipay, DisplayName: "Alipay", MaskedDestination: "a***@example.com", SecureReference: []byte("encrypted-ciphertext"), EncryptionKeyID: "key-1", Status: money.PayoutMethodActive, CreatedAt: now, UpdatedAt: now, Version: 1}
	if err := repo.CreatePayoutMethod(context.Background(), method); err != nil {
		t.Fatal(err)
	}
	valid, err := repo.HasValidPayoutMethod(context.Background(), "user-1", "method-1")
	if err != nil || !valid {
		t.Fatalf("valid=%v err=%v", valid, err)
	}
	methods, err := repo.ListActivePayoutMethods(context.Background(), "user-1")
	if err != nil || len(methods) != 1 || methods[0].MethodID != "method-1" || methods[0].MaskedDestination != "a***@example.com" {
		t.Fatalf("methods=%#v err=%v", methods, err)
	}
	review, err := repo.ReadPayoutMethodForReview(context.Background(), "method-1")
	if err != nil || string(review.SecureReference) != "encrypted-ciphertext" || review.EncryptionKeyID != "key-1" || review.SubjectUserID != "user-1" {
		t.Fatalf("review method=%#v err=%v", review, err)
	}
}

func TestPayoutMethodCreationIsIdempotent(t *testing.T) {
	repo := newMoneyRepository(t)
	now := time.Now().UTC()
	first := money.PayoutMethod{MethodID: "method-1", SubjectUserID: "user-1", Type: money.PayoutAlipay, DisplayName: "Alipay", MaskedDestination: "a***@example.com", SecureReference: []byte("encrypted-ciphertext"), EncryptionKeyID: "key-1", Status: money.PayoutMethodActive, CreatedAt: now, UpdatedAt: now, Version: 1}
	created, err := repo.CreatePayoutMethodIdempotent(context.Background(), first, "payout-key-1", "fingerprint-1")
	if err != nil || created.MethodID != "method-1" {
		t.Fatalf("first create=%#v err=%v", created, err)
	}
	replay := first
	replay.MethodID = "method-2"
	replayed, err := repo.CreatePayoutMethodIdempotent(context.Background(), replay, "payout-key-1", "fingerprint-1")
	if err != nil || replayed.MethodID != "method-1" {
		t.Fatalf("replay=%#v err=%v", replayed, err)
	}
	if _, err := repo.CreatePayoutMethodIdempotent(context.Background(), replay, "payout-key-1", "different-fingerprint"); !errors.Is(err, money.ErrConflict) {
		t.Fatalf("payload conflict=%v", err)
	}
}
