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
	ChannelAmounts                                                  *ServicePaymentAmounts `json:",omitempty"`
	Allocation                                                      ServiceAllocationPolicy
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
	if in.Allocation.Validate() != nil || in.BuyerOrganizationID == in.ProviderOrganizationID || in.PlatformMerchantID == in.ProviderMerchantID || in.Binding.Validate() != nil || in.Binding.Provider != "WECHAT_PAY" || in.Binding.MerchantID != in.ProviderMerchantID || in.Payment.Validate() != nil || in.Payment.PaymentPurpose != PaymentPurposeServicePurchase {
		return ErrInvalid
	}
	if in.Allocation.Basis == ServiceAllocationChannelNetFloorV2 {
		if in.ChannelAmounts == nil || in.ChannelAmounts.Validate(in.Payment.GrossAmountMinor) != nil {
			return ErrInvalid
		}
	} else if in.ChannelAmounts != nil {
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
	if in.ChannelAmounts != nil {
		a := in.ChannelAmounts.Normalize()
		in.ChannelAmounts = &a
	}
	in.Payment.SettledAt = NormalizeTimestamp(in.Payment.SettledAt)
	return ServiceFingerprint(in)
}

type ServiceEffectKind string

const (
	ServiceShare            ServiceEffectKind = "SHARE"
	ServiceFinish           ServiceEffectKind = "FINISH"
	ServiceRefundRelease    ServiceEffectKind = "REFUND_RELEASE"
	ServiceReturn           ServiceEffectKind = "RETURN"
	ServiceRefund           ServiceEffectKind = "REFUND"
	ServiceChargeback       ServiceEffectKind = "CHARGEBACK"
	ServiceAutomaticRelease ServiceEffectKind = "AUTOMATIC_RELEASE"
)

type ServiceOperation struct {
	RefundPlan           *ServiceRefundPlan `json:",omitempty"`
	RefundPhase          string             `json:",omitempty"`
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
	case ServiceShare, ServiceFinish, ServiceRefundRelease:
	case ServiceReturn:
		if !isCanonicalWalletIdentifier(in.OriginalShareID) && !(in.RefundPlan != nil && in.AmountMinor == 0 && in.RefundPlan.OriginalShareID == "") {
			return ErrInvalid
		}
	case ServiceRefund, ServiceChargeback:
		if in.AmountMinor == 0 {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	if in.RefundPlan != nil {
		p := in.RefundPlan
		if p.Validate() != nil || p.OrderID != in.OrderID || p.SourceProofID != in.SourceProofID {
			return ErrInvalid
		}
		switch in.RefundPhase {
		case ServiceReturnPre:
			if in.Kind != ServiceReturn || in.OperationID != p.PreOperationID || in.AmountMinor != p.PreReturnMinor || in.OriginalShareID != p.OriginalShareID {
				return ErrInvalid
			}
		case ServiceReturnPost:
			if in.Kind != ServiceReturn || in.OperationID != p.PostOperationID || in.OriginalShareID != p.OriginalShareID {
				return ErrInvalid
			}
		case ServiceRefundPhase:
			if in.Kind != ServiceRefund || in.OperationID != p.RefundOperationID || in.AmountMinor != p.NominalMinor {
				return ErrInvalid
			}
		case ServiceRefundReleasePhase:
			if in.Kind != ServiceRefundRelease || in.OperationID != p.ReleaseOperationID {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
	} else if in.RefundPhase != "" {
		return ErrInvalid
	}
	return nil
}

type ServiceEffect struct {
	RefundAmounts     *ServiceRefundAmounts `json:",omitempty"`
	Operation         ServiceOperation
	ProviderReference string
	OccurredAt        time.Time
}
type ServiceOperationFailure struct {
	Operation                          ServiceOperation
	ProofID, ProviderReference, Reason string
	OccurredAt                         time.Time
}

func (in ServiceOperationFailure) Validate() error {
	if in.Operation.Validate() != nil || !isCanonicalWalletIdentifier(in.ProofID) || !isCanonicalWalletIdentifier(in.ProviderReference) || !isCanonicalWalletIdentifier(in.Reason) || in.OccurredAt.IsZero() {
		return ErrInvalid
	}
	return nil
}

func (e ServiceEffect) Validate() error {
	if e.Operation.Validate() != nil || !isCanonicalWalletIdentifier(e.ProviderReference) || e.OccurredAt.IsZero() {
		return ErrInvalid
	}
	if e.RefundAmounts != nil && e.Operation.Kind != ServiceRefund {
		return ErrInvalid
	}
	return nil
}
func (e ServiceEffect) Fingerprint() string {
	if e.RefundAmounts != nil {
		a := e.RefundAmounts.Normalize()
		e.RefundAmounts = &a
	}
	e.OccurredAt = NormalizeTimestamp(e.OccurredAt)
	return ServiceFingerprint(e)
}

type ServiceFundsView struct {
	PendingRefundCommandID                                                    string                 `json:",omitempty"`
	ChannelAmounts                                                            *ServicePaymentAmounts `json:",omitempty"`
	SettlementMinor, SettlementRefundedMinor, PayerRefundedMinor              int64
	VoucherRefundedMinor                                                      map[string]int64 `json:",omitempty"`
	Allocation                                                                ServiceAllocationPolicy
	ChannelFeeMinor                                                           int64
	ChannelFeeObserved                                                        bool
	OrderID, PaymentID, BuyerOrganizationID, ProviderOrganizationID, Currency string
	GrossMinor, RefundedMinor, ChargedBackMinor, PlatformMinor, ProviderMinor int64
	SharedMinor, ReturnedMinor, ReleasedMinor, AutomaticReleasedMinor         int64
	PendingOperationID, ReconciliationReason                                  string
}

type ServiceChannelFee struct {
	Payment                     ServicePaymentInput
	FlowID, BusinessID, ProofID string
	AmountMinor                 int64
	Returned                    bool
	OccurredAt                  time.Time
}

func (in ServiceChannelFee) Validate() error {
	if in.Payment.Validate() != nil || !isCanonicalWalletIdentifier(in.FlowID) || !isCanonicalWalletIdentifier(in.BusinessID) || !isCanonicalWalletIdentifier(in.ProofID) || in.AmountMinor < 0 || in.AmountMinor > in.Payment.Payment.GrossAmountMinor || in.OccurredAt.IsZero() {
		return ErrInvalid
	}
	return nil
}
func (in ServiceChannelFee) Fingerprint() string {
	in.ProofID = ""
	in.OccurredAt = NormalizeTimestamp(in.OccurredAt)
	return ServiceFingerprint(in)
}

// A signed remaining-balance query is not proof of the cause of a release.
type ServiceUnsplitObservation struct {
	Payment                                           ServicePaymentInput
	ProfileVersion, ExpectedFundsFingerprint, ProofID string
	UnsplitMinor                                      int64
	OccurredAt                                        time.Time
}

func (in ServiceUnsplitObservation) Validate() error {
	if in.Payment.Validate() != nil || !isCanonicalWalletIdentifier(in.ProfileVersion) || !isCanonicalWalletIdentifier(in.ProofID) || len(in.ExpectedFundsFingerprint) != 64 || in.UnsplitMinor < 0 || in.UnsplitMinor > in.Payment.Payment.GrossAmountMinor || in.OccurredAt.IsZero() {
		return ErrInvalid
	}
	return nil
}
func (f ServiceFundsView) ExpectedUnsplitMinor() int64 {
	remaining := f.GrossMinor - f.RefundedMinor - f.ChargedBackMinor - f.SharedMinor - f.ReleasedMinor - f.AutomaticReleasedMinor
	if f.Allocation.Basis == ServiceAllocationChannelNetFloorV2 {
		remaining = f.SettlementMinor - f.SettlementRefundedMinor - f.ChargedBackMinor - f.SharedMinor - f.ReleasedMinor - f.AutomaticReleasedMinor
	}
	// Commission returns go to available balance, never back to frozen funds.
	if remaining < 0 {
		return 0
	}
	return remaining
}

type ServiceReceipt struct {
	RefundAmounts                                                          *ServiceRefundAmounts `json:",omitempty"`
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

const ServiceAllocationCumulativeNetFloorV1 = "CUMULATIVE_NET_FLOOR_V1"

// The original paid input owns the allocation snapshot. Never substitute a
// current platform rate for a missing or invalid original snapshot.
type ServiceAllocationPolicy struct {
	CommissionBPS int64
	Basis         string
}

func (p ServiceAllocationPolicy) Validate() error {
	if p.Basis != ServiceAllocationCumulativeNetFloorV1 && p.Basis != ServiceAllocationChannelNetFloorV2 || p.CommissionBPS < 0 || p.CommissionBPS > 10000 {
		return ErrInvalid
	}
	return nil
}

func ServiceAllocation(gross, refunded int64, policy ServiceAllocationPolicy) (platform, provider int64, err error) {
	if policy.Validate() != nil || gross < 0 || gross == 0 && policy.Basis != ServiceAllocationChannelNetFloorV2 || refunded < 0 || refunded > gross {
		return 0, 0, ErrInvalid
	}
	net := gross - refunded
	// Quotient/remainder multiplication preserves floor rounding without
	// overflowing int64 for any valid gross and basis-point rate.
	platform = (net/10000)*policy.CommissionBPS + (net%10000)*policy.CommissionBPS/10000
	return platform, net - platform, nil
}

type ServiceFundsStore interface {
	AcceptServicePayment(context.Context, ServicePaymentInput) (ServiceReceipt, error)
	ReadServicePayment(context.Context, ServicePaymentInput) (ServiceReceipt, error)
	ReadServiceFunds(context.Context, string) (ServiceFundsView, error)
	PrepareServiceOperation(context.Context, ServiceOperation) (ServiceReceipt, error)
	AdmitServiceOperation(context.Context, ServiceOperation) error
	AbandonUndispatchedServiceOperation(context.Context, ServiceOperation, string) error
	AcceptServiceEffect(context.Context, ServiceEffect) (ServiceReceipt, error)
	ReadServiceEffect(context.Context, ServiceOperation) (ServiceReceipt, error)
	ObserveServiceChargeback(context.Context, string, ChargebackSettlement) error
	ResolveFailedServiceOperation(context.Context, ServiceOperationFailure) (ServiceReceipt, error)
	ObserveServiceRefundUncertainty(context.Context, ServicePaymentInput, string) (ServiceReceipt, error)
	ObserveServiceUnsplit(context.Context, ServiceUnsplitObservation) (ServiceFundsView, error)
	ObserveServiceChannelFee(context.Context, ServiceChannelFee) (ServiceReceipt, error)
}

func ValidateServiceUncertaintyProof(proof string) error {
	if !isCanonicalWalletIdentifier(proof) {
		return ErrInvalid
	}
	return nil
}
