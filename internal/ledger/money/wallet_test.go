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
