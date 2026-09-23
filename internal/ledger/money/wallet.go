package money

import (
	"context"
	"errors"
	"strings"
	"time"
)

const WalletCurrencyCNY = "CNY"

var (
	ErrWalletInsufficientBalance = errors.New("organization wallet balance is insufficient")
	ErrWalletReservationConflict = errors.New("organization wallet reservation conflict")
	ErrWalletSettlementConflict  = errors.New("organization wallet settlement binding conflict")
)

type WalletReservationState string

const (
	WalletReservationReserved  WalletReservationState = "RESERVED"
	WalletReservationCommitted WalletReservationState = "COMMITTED"
	WalletReservationReleased  WalletReservationState = "RELEASED"
)

type WalletEntryKind string

const (
	WalletEntryTopUpCredit        WalletEntryKind = "TOP_UP_CREDIT"
	WalletEntryPurchaseReserve    WalletEntryKind = "PURCHASE_RESERVE"
	WalletEntryPurchaseCommit     WalletEntryKind = "PURCHASE_COMMIT"
	WalletEntryPurchaseRelease    WalletEntryKind = "PURCHASE_RELEASE"
	WalletEntryRefundReversal     WalletEntryKind = "REFUND_REVERSAL"
	WalletEntryChargebackReversal WalletEntryKind = "CHARGEBACK_REVERSAL"
	WalletEntryDebtRepayment      WalletEntryKind = "DEBT_REPAYMENT"
)

type OrganizationWalletSnapshot struct {
	OrganizationID     string
	Currency           string
	AvailableMinor     int64
	ReservedMinor      int64
	DebtMinor          int64
	LifetimeTopUpMinor int64
	LifetimeSpendMinor int64
	Version            int64
	UpdatedAt          time.Time
}

func (snapshot OrganizationWalletSnapshot) Validate() error {
	if strings.TrimSpace(snapshot.OrganizationID) == "" ||
		snapshot.Currency != WalletCurrencyCNY ||
		snapshot.AvailableMinor < 0 ||
		snapshot.ReservedMinor < 0 ||
		snapshot.DebtMinor < 0 ||
		(snapshot.DebtMinor > 0 && snapshot.AvailableMinor > 0) ||
		snapshot.LifetimeTopUpMinor < 0 ||
		snapshot.LifetimeSpendMinor < 0 ||
		snapshot.Version < 1 ||
		snapshot.UpdatedAt.IsZero() {
		return ErrInvalid
	}
	return nil
}

type WalletEntry struct {
	EntryID           string
	OrganizationID    string
	Currency          string
	Kind              WalletEntryKind
	AvailableDelta    int64
	ReservedDelta     int64
	DebtDelta         int64
	AvailableAfter    int64
	ReservedAfter     int64
	DebtAfter         int64
	CommercialOrderID string
	PaymentID         string
	SourceIdentity    string
	OccurredAt        time.Time
}

func (entry WalletEntry) Validate() error {
	if strings.TrimSpace(entry.EntryID) == "" ||
		strings.TrimSpace(entry.OrganizationID) == "" ||
		entry.Currency != WalletCurrencyCNY ||
		entry.AvailableAfter < 0 ||
		entry.ReservedAfter < 0 ||
		entry.DebtAfter < 0 ||
		(entry.DebtAfter > 0 && entry.AvailableAfter > 0) ||
		!isCanonicalWalletSourceIdentity(entry.SourceIdentity) ||
		entry.OccurredAt.IsZero() {
		return ErrInvalid
	}
	switch entry.Kind {
	case WalletEntryTopUpCredit, WalletEntryRefundReversal, WalletEntryChargebackReversal:
		if strings.TrimSpace(entry.PaymentID) == "" {
			return ErrInvalid
		}
	case WalletEntryPurchaseReserve, WalletEntryPurchaseCommit, WalletEntryPurchaseRelease, WalletEntryDebtRepayment:
		return nil
	default:
		return ErrInvalid
	}
	return nil
}

func isCanonicalWalletSourceIdentity(value string) bool {
	return value != "" && strings.TrimSpace(value) == value
}

type WalletEntryPage struct {
	Items      []WalletEntry
	NextCursor string
}

type WalletReservation struct {
	ReservationID     string
	OperationID       string
	OrganizationID    string
	CommercialOrderID string
	Currency          string
	AmountMinor       int64
	State             WalletReservationState
	Version           int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ReserveWalletFundsInput struct {
	OperationID       string
	OrganizationID    string
	CommercialOrderID string
	Currency          string
	AmountMinor       int64
}

type CommitWalletReservationInput struct {
	OperationID       string
	OrganizationID    string
	CommercialOrderID string
	ReservationID     string
}

type ReleaseWalletReservationInput struct {
	OperationID       string
	OrganizationID    string
	CommercialOrderID string
	ReservationID     string
	Reason            string
}

// OrganizationTopUpSettlement is the accepted binding between a canonical
// settled payment and one Organization-scoped commercial top-up order. It is
// deliberately separate from PaymentSettlement: beneficiary Organization
// cannot be inferred from the payer's current membership after payment.
type OrganizationTopUpSettlement struct {
	PaymentID         string
	CommercialOrderID string
	OrganizationID    string
	Currency          string
	AmountMinor       int64
	SettledAt         time.Time
	ProviderReference string
	Version           int64
}

func (settlement OrganizationTopUpSettlement) Validate() error {
	if strings.TrimSpace(settlement.PaymentID) == "" ||
		strings.TrimSpace(settlement.CommercialOrderID) == "" ||
		strings.TrimSpace(settlement.OrganizationID) == "" ||
		settlement.Currency != WalletCurrencyCNY ||
		settlement.AmountMinor <= 0 ||
		settlement.SettledAt.IsZero() ||
		strings.TrimSpace(settlement.ProviderReference) == "" ||
		settlement.Version < 1 {
		return ErrInvalid
	}
	return nil
}

type WalletReversalKind string

const (
	WalletReversalRefund     WalletReversalKind = "REFUND"
	WalletReversalChargeback WalletReversalKind = "CHARGEBACK"
)

type OrganizationWalletReversal struct {
	ReversalID        string
	PaymentID         string
	CommercialOrderID string
	OrganizationID    string
	Kind              WalletReversalKind
	Currency          string
	AmountMinor       int64
	OccurredAt        time.Time
	ProviderReference string
}

func (reversal OrganizationWalletReversal) Validate() error {
	if strings.TrimSpace(reversal.ReversalID) == "" ||
		strings.TrimSpace(reversal.PaymentID) == "" ||
		strings.TrimSpace(reversal.CommercialOrderID) == "" ||
		strings.TrimSpace(reversal.OrganizationID) == "" ||
		(reversal.Kind != WalletReversalRefund && reversal.Kind != WalletReversalChargeback) ||
		reversal.Currency != WalletCurrencyCNY ||
		reversal.AmountMinor <= 0 ||
		reversal.OccurredAt.IsZero() ||
		strings.TrimSpace(reversal.ProviderReference) == "" {
		return ErrInvalid
	}
	return nil
}

// OrganizationWalletReader exposes only Organization-scoped money facts.
// Implementations must bind organizationID to a verified effective
// Organization before this port is called.
type OrganizationWalletReader interface {
	ReadOrganizationWallet(context.Context, string, string) (OrganizationWalletSnapshot, error)
	ListOrganizationWalletEntries(context.Context, string, string, string, int) (WalletEntryPage, error)
}

// OrganizationWalletCommander is intentionally narrow. It does not expose a
// generic credit/debit method that a browser or unrelated domain could use to
// mint or destroy monetary value.
type OrganizationWalletCommander interface {
	CreditSettledTopUp(context.Context, string, OrganizationTopUpSettlement) (OrganizationWalletSnapshot, error)
	ApplyTopUpReversal(context.Context, string, OrganizationWalletReversal) (OrganizationWalletSnapshot, error)
	ReserveCommercialPurchase(context.Context, ReserveWalletFundsInput) (WalletReservation, error)
	CommitCommercialPurchase(context.Context, CommitWalletReservationInput) (WalletReservation, error)
	ReleaseCommercialPurchase(context.Context, ReleaseWalletReservationInput) (WalletReservation, error)
}
