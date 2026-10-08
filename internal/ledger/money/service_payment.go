package money

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Service facts describe funds at the original channel submerchant. They never
// credit a platform wallet or a referral balance.
type ServicePaymentInput struct {
	OrderID, RequestID, BuyerOrganizationID, ProviderOrganizationID string
	PlatformMerchantID, ProviderMerchantID, PolicyVersion           string
	Binding                                                         ProviderPaymentBinding
	Payment                                                         PaymentSettlement
}

func (in ServicePaymentInput) Validate() error {
	for _, id := range []string{in.OrderID, in.RequestID, in.BuyerOrganizationID, in.ProviderOrganizationID, in.PlatformMerchantID, in.ProviderMerchantID, in.PolicyVersion} {
		if !isCanonicalWalletIdentifier(id) {
			return ErrInvalid
		}
	}
	if in.BuyerOrganizationID == in.ProviderOrganizationID || in.PlatformMerchantID == in.ProviderMerchantID || in.Binding.Validate() != nil || in.Binding.Provider != "WECHAT_PAY" || in.Binding.MerchantID != in.ProviderMerchantID || in.Payment.Validate() != nil || in.Payment.PaymentPurpose != PaymentPurposeServicePurchase {
		return ErrInvalid
	}
	return nil
}
func ServiceFingerprint(value any) string {
	b, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	s := sha256.Sum256(append([]byte("service-purchase:v1:"), b...))
	return hex.EncodeToString(s[:])
}
func (in ServicePaymentInput) Fingerprint() string {
	in.Payment.SettledAt = NormalizeTimestamp(in.Payment.SettledAt)
	return ServiceFingerprint(in)
}

type ServiceEffectKind string

const (
	ServiceShare            ServiceEffectKind = "SHARE"
	ServiceFinish           ServiceEffectKind = "FINISH"
	ServiceReturn           ServiceEffectKind = "RETURN"
	ServiceRefund           ServiceEffectKind = "REFUND"
	ServiceChargeback       ServiceEffectKind = "CHARGEBACK"
	ServiceAutomaticRelease ServiceEffectKind = "AUTOMATIC_RELEASE"
)

type ServiceOperation struct {
	OrderID, OperationID string
	Kind                 ServiceEffectKind
	AmountMinor          int64
	SourceProofID        string
	// Return effects bind the exact successful share operation.
	OriginalShareID string
}

func (in ServiceOperation) Validate() error {
	if !isCanonicalWalletIdentifier(in.OrderID) || !isCanonicalWalletIdentifier(in.OperationID) || !isCanonicalWalletIdentifier(in.SourceProofID) || in.AmountMinor < 0 {
		return ErrInvalid
	}
	switch in.Kind {
	case ServiceShare, ServiceFinish:
	case ServiceReturn:
		if !isCanonicalWalletIdentifier(in.OriginalShareID) {
			return ErrInvalid
		}
	case ServiceRefund, ServiceChargeback:
		if in.AmountMinor == 0 {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

type ServiceEffect struct {
	Operation         ServiceOperation
	ProviderReference string
	OccurredAt        time.Time
}

func (e ServiceEffect) Validate() error {
	if e.Operation.Validate() != nil || !isCanonicalWalletIdentifier(e.ProviderReference) || e.OccurredAt.IsZero() {
		return ErrInvalid
	}
	return nil
}
func (e ServiceEffect) Fingerprint() string {
	e.OccurredAt = NormalizeTimestamp(e.OccurredAt)
	return ServiceFingerprint(e)
}

type ServiceFundsView struct {
	OrderID, PaymentID, BuyerOrganizationID, ProviderOrganizationID, Currency string
	GrossMinor, RefundedMinor, ChargedBackMinor, PlatformMinor, ProviderMinor int64
	SharedMinor, ReturnedMinor, ReleasedMinor, AutomaticReleasedMinor         int64
	PendingOperationID, ReconciliationReason                                  string
}
type ServiceReceipt struct {
	ReceiptID, OrderID, OperationID, RequestFingerprint, ResultFingerprint string
	Kind                                                                   ServiceEffectKind
	AmountMinor                                                            int64
	ProviderReference                                                      string
	OccurredAt                                                             time.Time
}

func (r ServiceReceipt) Fingerprint() string { r.ResultFingerprint = ""; return ServiceFingerprint(r) }
func (r ServiceReceipt) Validate() error {
	if !isCanonicalWalletIdentifier(r.ReceiptID) || !isCanonicalWalletIdentifier(r.OrderID) || !isCanonicalWalletIdentifier(r.OperationID) || r.RequestFingerprint == "" || r.ResultFingerprint != r.Fingerprint() || r.OccurredAt.IsZero() || r.AmountMinor < 0 {
		return ErrInvalid
	}
	return nil
}
func ServiceAllocation(gross, refunded int64) (platform, provider int64, err error) {
	if gross <= 0 || refunded < 0 || refunded > gross {
		return 0, 0, ErrInvalid
	}
	net := gross - refunded
	platform = net / 10
	return platform, net - platform, nil
}

type ServiceFundsStore interface {
	AcceptServicePayment(context.Context, ServicePaymentInput) (ServiceReceipt, error)
	ReadServicePayment(context.Context, ServicePaymentInput) (ServiceReceipt, error)
	ReadServiceFunds(context.Context, string) (ServiceFundsView, error)
	PrepareServiceOperation(context.Context, ServiceOperation) (ServiceReceipt, error)
	AcceptServiceEffect(context.Context, ServiceEffect) (ServiceReceipt, error)
	ReadServiceEffect(context.Context, ServiceOperation) (ServiceReceipt, error)
	ObserveServiceChargeback(context.Context, string, ChargebackSettlement) error
}
