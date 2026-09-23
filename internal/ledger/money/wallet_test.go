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
			entry := validWalletEntry(kind)
			entry.PaymentID = "payment-1"
			if err := entry.Validate(); err != nil {
				t.Fatalf("entry with payment binding rejected: %v", err)
			}
			entry.PaymentID = ""
			if !errors.Is(entry.Validate(), ErrInvalid) {
				t.Fatalf("entry without payment binding = nil, want ErrInvalid")
			}
		})
	}

	if err := validWalletEntry(WalletEntryPurchaseCommit).Validate(); err != nil {
		t.Fatalf("purchase entry without payment binding rejected: %v", err)
	}
}

func TestWalletEntryRejectsUnknownKind(t *testing.T) {
	if err := validWalletEntry(WalletEntryKind("UNKNOWN")).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown wallet entry kind = %v, want ErrInvalid", err)
	}
}

func TestWalletEntryRequiresCanonicalSourceIdentity(t *testing.T) {
	entry := validWalletEntry(WalletEntryPurchaseCommit)
	entry.SourceIdentity = ""
	if !errors.Is(entry.Validate(), ErrInvalid) {
		t.Fatalf("wallet entry without source identity = nil, want ErrInvalid")
	}

	entry.SourceIdentity = " source-1"
	if !errors.Is(entry.Validate(), ErrInvalid) {
		t.Fatalf("wallet entry with non-canonical source identity = nil, want ErrInvalid")
	}
}

func TestWalletEntryRejectsDebtAndAvailableAfterBalance(t *testing.T) {
	entry := validWalletEntry(WalletEntryPurchaseCommit)
	entry.AvailableAfter = 100
	entry.DebtAfter = 50
	if !errors.Is(entry.Validate(), ErrInvalid) {
		t.Fatalf("wallet entry with debt and available after balance = nil, want ErrInvalid")
	}
}

func TestWalletEntryRequiresKindSpecificDeltas(t *testing.T) {
	entry := validWalletEntry(WalletEntryTopUpCredit)
	entry.PaymentID = "payment-1"
	entry.AvailableDelta = -100
	if !errors.Is(entry.Validate(), ErrInvalid) {
		t.Fatalf("top-up with negative available delta = nil, want ErrInvalid")
	}

	entry = validWalletEntry(WalletEntryPurchaseReserve)
	entry.AvailableDelta = 0
	if !errors.Is(entry.Validate(), ErrInvalid) {
		t.Fatalf("purchase reserve with zero available delta = nil, want ErrInvalid")
	}

	entry = validWalletEntry(WalletEntryPurchaseReserve)
	entry.AvailableDelta = 100
	entry.ReservedDelta = -100
	if !errors.Is(entry.Validate(), ErrInvalid) {
		t.Fatalf("purchase reserve with reversed deltas = nil, want ErrInvalid")
	}
}

func TestWalletPurchaseEntryRequiresCommercialOrderBinding(t *testing.T) {
	for _, kind := range []WalletEntryKind{
		WalletEntryPurchaseReserve,
		WalletEntryPurchaseCommit,
		WalletEntryPurchaseRelease,
	} {
		t.Run(string(kind), func(t *testing.T) {
			entry := validWalletEntry(kind)
			entry.CommercialOrderID = ""
			if !errors.Is(entry.Validate(), ErrInvalid) {
				t.Fatalf("purchase entry without commercial order binding = nil, want ErrInvalid")
			}
		})
	}
}

func validWalletEntry(kind WalletEntryKind) WalletEntry {
	entry := WalletEntry{
		EntryID:           "entry-1",
		OrganizationID:    "org-1",
		Currency:          WalletCurrencyCNY,
		Kind:              kind,
		CommercialOrderID: "order-1",
		AvailableAfter:    100,
		ReservedAfter:     0,
		DebtAfter:         0,
		SourceIdentity:    "wallet-entry-1",
		OccurredAt:        time.Now().UTC(),
	}
	switch kind {
	case WalletEntryTopUpCredit, WalletEntryRefundReversal, WalletEntryChargebackReversal:
		entry.AvailableDelta = 100
	case WalletEntryPurchaseReserve:
		entry.AvailableDelta = -100
		entry.ReservedDelta = 100
	case WalletEntryPurchaseCommit:
		entry.ReservedDelta = -100
	case WalletEntryPurchaseRelease:
		entry.AvailableDelta = 100
		entry.ReservedDelta = -100
	case WalletEntryDebtRepayment:
		entry.DebtDelta = -100
	}
	return entry
}
