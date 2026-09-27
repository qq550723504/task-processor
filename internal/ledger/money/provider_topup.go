package money

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// ProviderPaymentBinding is the verified channel identity, never browser input.
type ProviderPaymentBinding struct {
	Provider    string
	Environment string
	MerchantID  string
	AppID       string
	TradeID     string
}

func (b ProviderPaymentBinding) Validate() error {
	if (b.Provider != "ALIPAY" && b.Provider != "WECHAT_PAY") || (b.Environment != "PRODUCTION" && !(b.Provider == "ALIPAY" && b.Environment == "SANDBOX")) || !isCanonicalWalletIdentifier(b.MerchantID) || !isCanonicalWalletIdentifier(b.AppID) || !isCanonicalWalletIdentifier(b.TradeID) {
		return ErrInvalid
	}
	return nil
}

// TopUpFingerprint uses unambiguous, versioned JSON encoding, with no floats.
func TopUpFingerprint(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(append([]byte("wallet-topup:v1:"), encoded...))
	return hex.EncodeToString(sum[:])
}

func (b ProviderPaymentBinding) ClaimID() string {
	// AppID must match, but must not enlarge the provider transaction namespace.
	return TopUpFingerprint([]string{b.Provider, b.Environment, b.MerchantID, b.TradeID})
}

type ProviderTopUpInput struct {
	OperationID       string
	OrganizationID    string
	CommercialOrderID string
	Currency          string
	AmountMinor       int64
	Binding           ProviderPaymentBinding
	Payment           PaymentSettlement
	KnownReversals    []OrganizationWalletReversal
}

func (in ProviderTopUpInput) Validate() error {
	if !isCanonicalWalletIdentifier(in.OperationID) || !isCanonicalWalletIdentifier(in.OrganizationID) || !isCanonicalWalletIdentifier(in.CommercialOrderID) || in.Currency != WalletCurrencyCNY || in.AmountMinor <= 0 || in.Payment.Validate() != nil || in.Binding.Validate() != nil || in.Payment.PaymentPurpose != PaymentPurposeWalletTopUp || in.Payment.Currency != in.Currency || in.Payment.GrossAmountMinor != in.AmountMinor {
		return ErrInvalid
	}
	for _, r := range in.KnownReversals {
		if r.Validate() != nil || r.PaymentID != in.Payment.PaymentID || r.OrganizationID != in.OrganizationID || r.CommercialOrderID != in.CommercialOrderID || r.Currency != in.Currency {
			return ErrInvalid
		}
	}
	return nil
}

func (in ProviderTopUpInput) Fingerprint() string {
	in.KnownReversals = nil // later observations cannot change the original intent
	in.Payment.SettledAt = NormalizeTimestamp(in.Payment.SettledAt)
	return TopUpFingerprint(in)
}

type TopUpPostingReceipt struct {
	ReceiptID               string
	OperationID             string
	PaymentID               string
	OrganizationID          string
	CommercialOrderID       string
	Binding                 ProviderPaymentBinding
	Currency                string
	GrossCreditMinor        int64
	AvailableAddedMinor     int64
	DebtRepaidMinor         int64
	CreditEntryIDs          []string
	KnownReversalReceiptIDs []string
	RequestFingerprint      string
	ResultFingerprint       string
	PostedAt                time.Time
}

func (r TopUpPostingReceipt) Fingerprint() string {
	r.ResultFingerprint = ""
	return TopUpFingerprint(r)
}
func (r TopUpPostingReceipt) Validate() error {
	if !isCanonicalWalletIdentifier(r.ReceiptID) || !isCanonicalWalletIdentifier(r.OperationID) || !isCanonicalWalletIdentifier(r.PaymentID) || !isCanonicalWalletIdentifier(r.OrganizationID) || !isCanonicalWalletIdentifier(r.CommercialOrderID) || r.Binding.Validate() != nil || r.Currency != WalletCurrencyCNY || r.GrossCreditMinor <= 0 || r.AvailableAddedMinor < 0 || r.AvailableAddedMinor > r.GrossCreditMinor || r.DebtRepaidMinor != r.GrossCreditMinor-r.AvailableAddedMinor || len(r.CreditEntryIDs) == 0 || r.RequestFingerprint == "" || r.PostedAt.IsZero() || r.ResultFingerprint != r.Fingerprint() {
		return ErrInvalid
	}
	return nil
}

type TopUpReversalKey struct {
	PaymentID  string
	Kind       WalletReversalKind
	ReversalID string
}

func (k TopUpReversalKey) Validate() error {
	if !isCanonicalWalletIdentifier(k.PaymentID) || !isCanonicalWalletIdentifier(k.ReversalID) || (k.Kind != WalletReversalRefund && k.Kind != WalletReversalChargeback) {
		return ErrInvalid
	}
	return nil
}
func (k TopUpReversalKey) StorageID() string { return "topup-reversal:v1:" + TopUpFingerprint(k) }

type TopUpRefundInput struct {
	Key               TopUpReversalKey
	OrganizationID    string
	CommercialOrderID string
	AmountMinor       int64
	ApprovalID        string
}

func (in TopUpRefundInput) Validate() error {
	if in.Key.Validate() != nil || in.Key.Kind != WalletReversalRefund || !isCanonicalWalletIdentifier(in.OrganizationID) || !isCanonicalWalletIdentifier(in.CommercialOrderID) || in.AmountMinor <= 0 || !isCanonicalWalletIdentifier(in.ApprovalID) {
		return ErrInvalid
	}
	return nil
}

type TopUpRefundHold struct {
	HoldID     string
	Input      TopUpRefundInput
	State      string
	Dispatched bool
	CreatedAt  time.Time
}

type TopUpReversalReceipt struct {
	ReceiptID                  string
	Key                        TopUpReversalKey
	OrganizationID             string
	CommercialOrderID          string
	Currency                   string
	ProviderAmountMinor        int64
	WalletPrincipalEffectMinor int64
	ExcessProviderMinor        int64
	ExcessRecordID             string
	HoldID                     string
	HoldState                  string
	HoldConsumedMinor          int64
	HoldReleasedMinor          int64
	HoldDebtRepaidMinor        int64
	EntryIDs                   []string
	RequestFingerprint         string
	ResultFingerprint          string
	PostedAt                   time.Time
}

func (r TopUpReversalReceipt) Fingerprint() string {
	r.ResultFingerprint = ""
	return TopUpFingerprint(r)
}
func (r TopUpReversalReceipt) Validate() error {
	if r.Key.Validate() != nil || r.ReceiptID != r.Key.StorageID() || !isCanonicalWalletIdentifier(r.OrganizationID) || !isCanonicalWalletIdentifier(r.CommercialOrderID) || r.Currency != WalletCurrencyCNY || r.ProviderAmountMinor <= 0 || r.WalletPrincipalEffectMinor < 0 || r.WalletPrincipalEffectMinor > r.ProviderAmountMinor || r.ExcessProviderMinor != r.ProviderAmountMinor-r.WalletPrincipalEffectMinor || (r.ExcessProviderMinor > 0) != (r.ExcessRecordID != "") || r.RequestFingerprint == "" || r.PostedAt.IsZero() || r.ResultFingerprint != r.Fingerprint() {
		return ErrInvalid
	}
	if r.HoldID != "" {
		if r.Key.Kind != WalletReversalRefund || r.HoldState != "CONFIRMED" || r.HoldConsumedMinor != r.WalletPrincipalEffectMinor || r.HoldReleasedMinor != r.ProviderAmountMinor-r.HoldConsumedMinor || r.HoldDebtRepaidMinor < 0 || r.HoldDebtRepaidMinor > r.HoldReleasedMinor {
			return ErrInvalid
		}
	} else if r.HoldState != "" || r.HoldConsumedMinor != 0 || r.HoldReleasedMinor != 0 || r.HoldDebtRepaidMinor != 0 {
		return ErrInvalid
	}
	return nil
}

// ProviderTopUpOwner is only injected into trusted billing orchestration.
// It does not expose a browser-controlled verification flag or credit endpoint.
type ProviderTopUpOwner interface {
	AcceptAndPostProviderTopUp(context.Context, ProviderTopUpInput) (TopUpPostingReceipt, error)
	ReadTopUpPosting(context.Context, string, string, string) (TopUpPostingReceipt, error)
	AcceptProviderTopUpReversal(context.Context, OrganizationWalletReversal) (TopUpReversalReceipt, error)
	ReadTopUpReversal(context.Context, string, string, TopUpReversalKey) (TopUpReversalReceipt, error)
	PrepareTopUpRefund(context.Context, TopUpRefundInput) (TopUpRefundHold, error)
	AdmitTopUpRefund(context.Context, TopUpRefundInput) (TopUpRefundHold, error)
	ReleaseTopUpRefundHold(context.Context, TopUpRefundInput, bool) (TopUpRefundHold, error)
}
