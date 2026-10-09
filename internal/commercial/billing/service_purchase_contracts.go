package billing

import (
	"context"
	"strings"
	"task-processor/internal/ledger/money"
	"time"
)

// Service purchases use the original qualified submerchant, never a wallet.
type ServiceMerchantProfile struct {
	Version, Environment, PlatformMerchantID, AppID string
	FreezeDays                                      int
}

func (p ServiceMerchantProfile) Validate() error {
	for _, s := range []string{p.Version, p.Environment, p.PlatformMerchantID, p.AppID} {
		if !isCanonicalIdentifier(s) {
			return ErrInvalid
		}
	}
	if p.FreezeDays < 1 || p.FreezeDays > 180 {
		return ErrInvalid
	}
	return nil
}

type ServicePurchaseCommand struct {
	Allocation                                                      money.ServiceAllocationPolicy
	ID, RequestID, OrderID, Kind, SourceProofID, ActorID            string
	BuyerOrganizationID, ProviderOrganizationID, ProviderMerchantID string
	QuoteVersion                                                    int64
	AmountMinor                                                     int64
	DeliveryDays                                                    int
	PolicyVersion                                                   string
}

func (c ServicePurchaseCommand) Validate() error {
	for _, v := range []string{c.ID, c.RequestID, c.OrderID, c.SourceProofID, c.ActorID, c.BuyerOrganizationID, c.ProviderOrganizationID, c.ProviderMerchantID, c.PolicyVersion} {
		if !isCanonicalIdentifier(v) {
			return ErrInvalid
		}
	}
	if c.Allocation.Validate() != nil || c.BuyerOrganizationID == c.ProviderOrganizationID || c.AmountMinor <= 0 || c.QuoteVersion < 1 || c.DeliveryDays < 1 {
		return ErrInvalid
	}
	switch c.Kind {
	case "CREATE_PURCHASE", "SETTLE", "CANCEL", "REFUND":
	default:
		return ErrInvalid
	}
	return nil
}
func (c ServicePurchaseCommand) Fingerprint() string { return money.ServiceFingerprint(c) }

type ServicePurchaseOrder struct {
	RefundPlans                              map[string]money.ServiceRefundPlan `json:",omitempty"`
	Source                                   ServicePurchaseCommand
	Profile                                  ServiceMerchantProfile
	TradeNo                                  string
	CreatedAt, ExpiresAt                     time.Time
	PaymentDispatched, CancelRequested       bool
	CheckoutCiphertext                       []byte
	Payment                                  *ServicePaymentObservation
	PaymentReceiptID                         string
	FundsExpireAt                            *time.Time
	ActiveCommand                            *ServicePurchaseCommand
	Operation                                *ServiceFinancialOperation
	ShareOperationID, ShareProviderRequestID string
	Operations                               map[string]ServiceFinancialOperation
	Effects                                  map[string]money.ServiceReceipt
	DeniedOperations                         map[string]string
	AttemptGenerations                       map[string]int
	CompletedCommands                        map[string]ServicePurchaseResult
	State, Reason                            string
	Version                                  int64
	LeaseToken                               string
	LeaseUntil                               time.Time
}

func (o ServicePurchaseOrder) Fingerprint() string {
	return money.ServiceFingerprint(struct {
		Source           ServicePurchaseCommand
		Profile          ServiceMerchantProfile
		Trade            string
		Created, Expires time.Time
	}{o.Source, o.Profile, o.TradeNo, o.CreatedAt, o.ExpiresAt})
}
func (o ServicePurchaseOrder) MoneyInput() money.ServicePaymentInput {
	p := o.Payment
	binding := money.ProviderPaymentBinding{Provider: "WECHAT_PAY", Environment: o.Profile.Environment, MerchantID: o.Source.ProviderMerchantID, AppID: o.Profile.AppID, TradeID: p.TransactionID}
	out := money.ServicePaymentInput{Allocation: o.Source.Allocation, OrderID: o.Source.OrderID, RequestID: o.Source.RequestID, BuyerOrganizationID: o.Source.BuyerOrganizationID, ProviderOrganizationID: o.Source.ProviderOrganizationID, PlatformMerchantID: o.Profile.PlatformMerchantID, ProviderMerchantID: o.Source.ProviderMerchantID, PolicyVersion: o.Source.PolicyVersion, Binding: binding, Payment: money.PaymentSettlement{PaymentID: "provider-payment:" + binding.ClaimID(), PaymentPurpose: money.PaymentPurposeServicePurchase, CommissionTreatment: money.CommissionNonCommissionable, PayerBinding: money.PayerOrganizationServiceBuyer, Currency: "CNY", GrossAmountMinor: o.Source.AmountMinor, Status: money.PaymentSettled, SettledAt: money.NormalizeTimestamp(p.OccurredAt), ProviderReference: "provider-trade:" + binding.ClaimID(), Version: 1}}

	if o.Source.Allocation.Basis == money.ServiceAllocationChannelNetFloorV2 {
		out.ChannelAmounts = p.ChannelAmounts
	}
	return out
}

type ServiceFinancialOperation struct {
	Reservation                                          money.ServiceOperation
	CommandID, ProviderRequestID, OriginalShareRequestID string
	Dispatched                                           bool
}
type ServicePaymentObservation struct {
	ChannelAmounts                                                                  *money.ServicePaymentAmounts `json:",omitempty"`
	EventID, ProfileVersion, PlatformMerchantID, AppID, ProviderMerchantID, TradeNo string
	TransactionID, Currency, State, VerificationVersion                             string
	AmountMinor                                                                     int64
	OccurredAt                                                                      time.Time
	CheckoutRecovery                                                                string
}

const (
	ServiceCheckoutUnpaidProof = "NATIVE_UNPAID_MATCHED"
	ServiceCheckoutAbsentProof = "NATIVE_ORDER_NOT_EXIST_VERIFIED"
)

// Only a fresh, verified original Native query can authorize original-parameter
// reentry. Ordinary UNPAID observations do not confer payment capability.
func (p ServicePaymentObservation) AllowsCheckoutReplay(o ServicePurchaseOrder) bool {
	if !p.Matches(o) || p.State != "UNPAID" {
		return false
	}
	switch p.CheckoutRecovery {
	case ServiceCheckoutUnpaidProof:
		return p.AmountMinor == o.Source.AmountMinor
	case ServiceCheckoutAbsentProof:
		return p.AmountMinor == 0 && p.TransactionID == ""
	}
	return false
}

func (p ServicePaymentObservation) Matches(o ServicePurchaseOrder) bool {
	if p.EventID == "" || p.VerificationVersion == "" || p.ProfileVersion != o.Profile.Version || p.PlatformMerchantID != o.Profile.PlatformMerchantID || p.AppID != o.Profile.AppID || p.ProviderMerchantID != o.Source.ProviderMerchantID || p.TradeNo != o.TradeNo || p.Currency != "CNY" {
		return false
	}
	switch p.State {
	case "PAID", "PAID_REFUND_UNKNOWN":
		if o.Source.Allocation.Basis == money.ServiceAllocationChannelNetFloorV2 {
			if p.ChannelAmounts == nil || p.ChannelAmounts.Validate(p.AmountMinor) != nil {
				return false
			}
		} else if p.ChannelAmounts != nil && (p.ChannelAmounts.Validate(p.AmountMinor) != nil || len(p.ChannelAmounts.Vouchers) != 0) {
			return false
		}
		return p.AmountMinor == o.Source.AmountMinor && p.TransactionID != "" && !p.OccurredAt.IsZero()
	case "UNPAID", "CLOSED":
		return true
	}
	return false
}

type ServiceOperationObservation struct {
	RefundAmounts                                                                 *money.ServiceRefundAmounts `json:",omitempty"`
	EventID, ProfileVersion, ProviderMerchantID, TransactionID, ProviderRequestID string
	Kind                                                                          money.ServiceEffectKind
	AmountMinor                                                                   int64
	State, ProviderReference, VerificationVersion, Reason                         string
	OccurredAt                                                                    time.Time
}

type ServiceUnsplitObservation struct {
	ProfileVersion, ProviderMerchantID, TransactionID, ProofID, VerificationVersion string
	UnsplitMinor                                                                    int64
	OccurredAt                                                                      time.Time
}

func (p ServiceUnsplitObservation) Matches(o ServicePurchaseOrder) bool {
	return o.Payment != nil && p.ProfileVersion == o.Profile.Version && p.ProviderMerchantID == o.Source.ProviderMerchantID && p.TransactionID == o.Payment.TransactionID && p.UnsplitMinor >= 0 && p.UnsplitMinor <= o.Source.AmountMinor && p.ProofID != "" && p.VerificationVersion != "" && !p.OccurredAt.IsZero()
}

type ServiceUnsplitProvider interface {
	QueryServiceUnsplit(context.Context, ServicePurchaseOrder) (ServiceUnsplitObservation, error)
}

func (p ServiceOperationObservation) Matches(o ServicePurchaseOrder, op ServiceFinancialOperation) bool {
	if p.State == "SUCCESS" && op.Reservation.Kind == money.ServiceRefund && o.Source.Allocation.Basis == money.ServiceAllocationChannelNetFloorV2 {
		if o.Payment == nil || o.Payment.ChannelAmounts == nil || p.RefundAmounts == nil || p.RefundAmounts.Validate(*o.Payment.ChannelAmounts, op.Reservation.AmountMinor) != nil {
			return false
		}
	} else if p.RefundAmounts != nil {
		return false
	}
	return p.EventID != "" && p.VerificationVersion != "" && p.ProfileVersion == o.Profile.Version && p.ProviderMerchantID == o.Source.ProviderMerchantID && o.Payment != nil && p.TransactionID == o.Payment.TransactionID && p.ProviderRequestID == op.ProviderRequestID && p.Kind == op.Reservation.Kind && p.AmountMinor == op.Reservation.AmountMinor && (p.State == "PENDING" || p.State == "WAITING_FUNDS" || p.State == "FAILED" || p.State == "REPLAY_ALLOWED" && p.ProviderReference != "" && !p.OccurredAt.IsZero() || p.State == "SUCCESS" && p.ProviderReference != "" && !p.OccurredAt.IsZero())
}

type ServicePurchaseResult struct {
	OrderID, PaymentReceiptID, ReceiptID, State, Reason string
	FundsExpireAt                                       *time.Time
	FullRefund                                          bool
	Revision                                            int64
}
type ServicePurchaseSource interface {
	OriginalServicePurchase(context.Context, string) (ServicePurchaseCommand, error)
	VerifyServiceCommand(context.Context, ServicePurchaseCommand) error
	CanDispatchServiceCommand(context.Context, ServicePurchaseCommand) error
	AdmitServiceCommand(context.Context, ServicePurchaseCommand, string) error
	AuthorizeServiceCheckout(context.Context, string, string, string) error
}
type ServicePurchaseProvider interface {
	Profile() ServiceMerchantProfile
	NewPaymentsEnabled() bool
	CreateServiceCheckout(context.Context, ServicePurchaseOrder) (string, error)
	QueryServicePayment(context.Context, ServicePurchaseOrder) (ServicePaymentObservation, error)
	CloseServicePayment(context.Context, ServicePurchaseOrder) (ServicePaymentObservation, error)
	DispatchServiceOperation(context.Context, ServicePurchaseOrder, ServiceFinancialOperation) (ServiceOperationObservation, error)
	QueryServiceOperation(context.Context, ServicePurchaseOrder, ServiceFinancialOperation) (ServiceOperationObservation, error)
}
type ServicePurchaseStore interface {
	CreateServicePurchase(context.Context, ServicePurchaseCommand, ServiceMerchantProfile) (ServicePurchaseOrder, error)
	ReadServicePurchase(context.Context, string) (ServicePurchaseOrder, error)
	ClaimServicePurchase(context.Context, string, string, time.Time) (ServicePurchaseOrder, error)
	SaveServicePurchase(context.Context, ServicePurchaseOrder) (ServicePurchaseOrder, error)
	ReleaseServicePurchase(context.Context, string, string) error
	RecordServicePaymentObservation(context.Context, ServicePurchaseOrder, ServicePaymentObservation) error
	RecordServiceOperationObservation(context.Context, ServicePurchaseOrder, ServiceFinancialOperation, ServiceOperationObservation) error
	ServicePaymentObservations(context.Context, string) ([]ServicePaymentObservation, error)
	ServiceOperationObservations(context.Context, string) ([]ServiceOperationObservation, error)
}
type ServicePayloadProtection interface {
	Seal(string, string) ([]byte, error)
	Open(string, []byte) (string, error)
}
type ServicePurchases struct {
	store      ServicePurchaseStore
	funds      money.ServiceFundsStore
	provider   ServicePurchaseProvider
	source     ServicePurchaseSource
	protection ServicePayloadProtection
	now        func() time.Time
}

func NewServicePurchases(store ServicePurchaseStore, funds money.ServiceFundsStore, provider ServicePurchaseProvider, source ServicePurchaseSource, protection ServicePayloadProtection) (*ServicePurchases, error) {
	if store == nil || funds == nil || provider == nil || source == nil || protection == nil || provider.Profile().Validate() != nil {
		return nil, ErrFeatureUnavailable
	}
	return &ServicePurchases{store: store, funds: funds, provider: provider, source: source, protection: protection, now: func() time.Time { return time.Now().UTC() }}, nil
}
func serviceProviderID(parts ...string) string { return money.ServiceFingerprint(parts)[:32] }
func serviceCheckoutBinding(o ServicePurchaseOrder) string {
	return "service-checkout:" + o.Fingerprint()
}
func validServiceQR(v string) bool {
	return (strings.HasPrefix(v, "weixin://wxpay/bizpayurl?") || strings.HasPrefix(v, "weixin://wxpay/bizpayurl/up?")) && len(v) <= 8192
}
