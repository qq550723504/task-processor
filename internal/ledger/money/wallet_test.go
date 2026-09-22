package money

import (
	"errors"
	"testing"
	"time"
)

func TestOrganizationWalletSnapshotValidation(t *testing.T) {
	now := time.Now().UTC()
	valid := OrganizationWalletSnapshot{
		OrganizationID:     "org-1",
		Currency:           WalletCurrencyCNY,
		AvailableMinor:     100,
		ReservedMinor:      20,
		DebtMinor:          0,
		LifetimeTopUpMinor: 200,
		LifetimeSpendMinor: 80,
		Version:            1,
		UpdatedAt:          now,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid wallet rejected: %v", err)
	}

	invalid := valid
	invalid.AvailableMinor = -1
	if !errors.Is(invalid.Validate(), ErrInvalid) {
		t.Fatalf("negative available must be invalid")
	}

	invalid = valid
	invalid.DebtMinor = 50
	if !errors.Is(invalid.Validate(), ErrInvalid) {
		t.Fatalf("wallet with debt and available balance must be invalid")
	}
}

func TestOrganizationTopUpSettlementRequiresExplicitOrganizationBinding(t *testing.T) {
	settlement := OrganizationTopUpSettlement{
		PaymentID:         "payment-1",
		CommercialOrderID: "order-1",
		OrganizationID:    "org-1",
		Currency:          WalletCurrencyCNY,
		AmountMinor:       1000,
		SettledAt:         time.Now().UTC(),
		ProviderReference: "provider-1",
		Version:           1,
	}
	if err := settlement.Validate(); err != nil {
		t.Fatalf("valid settlement rejected: %v", err)
	}

	settlement.OrganizationID = ""
	if !errors.Is(settlement.Validate(), ErrInvalid) {
		t.Fatalf("top-up settlement without organization binding must be invalid")
	}
}

func TestOrganizationWalletReversalRequiresCanonicalKind(t *testing.T) {
	reversal := OrganizationWalletReversal{
		ReversalID:        "refund-1",
		PaymentID:         "payment-1",
		CommercialOrderID: "order-1",
		OrganizationID:    "org-1",
		Kind:              WalletReversalRefund,
		Currency:          WalletCurrencyCNY,
		AmountMinor:       500,
		OccurredAt:        time.Now().UTC(),
		ProviderReference: "provider-refund-1",
	}
	if err := reversal.Validate(); err != nil {
		t.Fatalf("valid reversal rejected: %v", err)
	}

	reversal.Kind = "OTHER"
	if !errors.Is(reversal.Validate(), ErrInvalid) {
		t.Fatalf("unknown reversal kind must be invalid")
	}
}

func TestWalletEntryRequiresPaymentBindingForTopUpAndReversals(t *testing.T) {
	for _, kind := range []WalletEntryKind{
		WalletEntryTopUpCredit,
		WalletEntryRefundReversal,
		WalletEntryChargebackReversal,
	} {
		t.Run(string(kind), func(t *testing.T) {
			entry := WalletEntry{Kind: kind, PaymentID: "payment-1"}
			if err := entry.Validate(); err != nil {
				t.Fatalf("entry with payment binding rejected: %v", err)
			}
			entry.PaymentID = ""
			if !errors.Is(entry.Validate(), ErrInvalid) {
				t.Fatalf("entry without payment binding = nil, want ErrInvalid")
			}
		})
	}

	if err := (WalletEntry{Kind: WalletEntryPurchaseCommit}).Validate(); err != nil {
		t.Fatalf("purchase entry without payment binding rejected: %v", err)
	}
}
