package billing

import (
	"context"
	"errors"
	"strings"
	"time"

	"task-processor/internal/ledger/money"
)

// Only returned when the adapter proves it issued no checkout request or action.
var ErrCheckoutNotDispatched = errors.New("top-up checkout was not dispatched")

type PaymentProvider string

const (
	PaymentAlipay PaymentProvider = "ALIPAY"
	PaymentWeChat PaymentProvider = "WECHAT_PAY"
)

type TopUpPhase string

const (
	TopUpCreated                TopUpPhase = "CREATED"
	TopUpAwaitingPayment        TopUpPhase = "AWAITING_PAYMENT"
	TopUpReconciliationRequired TopUpPhase = "RECONCILIATION_REQUIRED"
	TopUpPaidPendingCredit      TopUpPhase = "PAID_PENDING_CREDIT"
	TopUpCompleted              TopUpPhase = "COMPLETED"
	TopUpClosedUnpaid           TopUpPhase = "CLOSED_UNPAID"
)

type TopUpMerchant struct {
	Provider       PaymentProvider
	Environment    string
	ProfileVersion string
	MerchantID     string
	AppID          string
	Product        string
}

func (m TopUpMerchant) Validate() error {
	if !isCanonicalIdentifier(m.ProfileVersion) || moneyBinding(m, "validation").Validate() != nil || (m.Provider == PaymentAlipay && m.Product != "PAGE_PAY") || (m.Provider == PaymentWeChat && m.Product != "NATIVE") {
		return ErrInvalid
	}
	return nil
}
func moneyBinding(m TopUpMerchant, trade string) money.ProviderPaymentBinding {
	return money.ProviderPaymentBinding{Provider: string(m.Provider), Environment: m.Environment, MerchantID: m.MerchantID, AppID: m.AppID, TradeID: trade}
}

type TopUpAmountPolicy struct {
	MinMinor      int64
	MaxMinor      int64
	QuickAmounts  []int64
	PaymentWindow time.Duration
}

func (p TopUpAmountPolicy) Validate() error {
	if p.MinMinor <= 0 || p.MaxMinor < p.MinMinor || len(p.QuickAmounts) == 0 || len(p.QuickAmounts) > 12 || p.PaymentWindow < 2*time.Minute || p.PaymentWindow > 2*time.Hour {
		return ErrInvalid
	}
	seen := map[int64]bool{}
	for _, a := range p.QuickAmounts {
		if a < p.MinMinor || a > p.MaxMinor || seen[a] {
			return ErrInvalid
		}
		seen[a] = true
	}
	return nil
}

type CheckoutAction struct {
	Kind               string
	Payload            string
	OrderID            string
	AttemptID          string
	Provider           PaymentProvider
	ExpiresAt          time.Time
	RequestFingerprint string
}

func (a CheckoutAction) Matches(attempt TopUpPaymentAttempt) bool {
	return ((a.Kind == "REDIRECT" && attempt.Merchant.Provider == PaymentAlipay) || (a.Kind == "QR_CODE" && attempt.Merchant.Provider == PaymentWeChat)) && a.Payload != "" && len(a.Payload) <= 64*1024 && a.OrderID == attempt.OrderID && a.AttemptID == attempt.AttemptID && a.Provider == attempt.Merchant.Provider && a.ExpiresAt.Equal(attempt.ExpiresAt) && a.RequestFingerprint == attempt.Fingerprint()
}

// The secret checkout payload is sealed separately. Ordinary read projections
// deliberately omit it and all channel credentials.
type TopUpPaymentAttempt struct {
	AttemptID              string
	OrderID                string
	OrganizationID         string
	ActorID                string
	IdempotencyKey         string
	Merchant               TopUpMerchant
	MerchantOrderID        string
	Currency               string
	AmountMinor            int64
	ExpiresAt              time.Time
	CreatedAt              time.Time
	Phase                  TopUpPhase
	Version                int64
	CheckoutAdmittedAt     *time.Time
	CheckoutKind           string
	CheckoutCiphertext     []byte
	CloseRequestedAt       *time.Time
	ClosedAt               *time.Time
	LatePaymentCorrectedAt *time.Time
	MoneyReceiptID         string
	PaymentID              string
	NextCheckAt            time.Time
	NeedsReconcile         bool
	RetryCount             int
	LastSafeError          string
	LeaseToken             string
	LeaseUntil             time.Time
}

func (a TopUpPaymentAttempt) Fingerprint() string {
	return money.TopUpFingerprint(struct {
		Attempt, Order, Org, Actor, Key string
		Merchant                        TopUpMerchant
		MerchantOrder, Currency         string
		Amount                          int64
		Expires, Created                time.Time
		Policy                          string
	}{a.AttemptID, a.OrderID, a.OrganizationID, a.ActorID, a.IdempotencyKey, a.Merchant, a.MerchantOrderID, a.Currency, a.AmountMinor, a.ExpiresAt.UTC(), a.CreatedAt.UTC(), "equal-noncommissionable-v1"})
}
func (a TopUpPaymentAttempt) Validate() error {
	if !isCanonicalIdentifier(a.AttemptID) || !isCanonicalIdentifier(a.OrderID) || !isCanonicalIdentifier(a.OrganizationID) || !isCanonicalIdentifier(a.ActorID) || strings.TrimSpace(a.IdempotencyKey) == "" || len(a.IdempotencyKey) > 192 || a.Merchant.Validate() != nil || len(a.MerchantOrderID) != 32 || a.Currency != CurrencyCNY || a.AmountMinor <= 0 || a.CreatedAt.IsZero() || !a.ExpiresAt.After(a.CreatedAt) || a.Version < 1 {
		return ErrInvalid
	}
	switch a.Phase {
	case TopUpCreated, TopUpAwaitingPayment, TopUpReconciliationRequired, TopUpPaidPendingCredit, TopUpCompleted, TopUpClosedUnpaid:
	default:
		return ErrInvalid
	}
	if (a.Phase == TopUpCompleted) && (a.MoneyReceiptID == "" || a.PaymentID == "") {
		return ErrInvalid
	}
	return nil
}
func (a TopUpPaymentAttempt) MoneyInput(o ProviderObservation) money.ProviderTopUpInput {
	binding := moneyBinding(a.Merchant, o.TradeID)
	paymentID := "provider-payment:" + binding.ClaimID()
	return money.ProviderTopUpInput{OperationID: a.OrderID, OrganizationID: a.OrganizationID, CommercialOrderID: a.OrderID, Currency: a.Currency, AmountMinor: a.AmountMinor, Binding: binding, Payment: money.PaymentSettlement{PaymentID: paymentID, PaymentPurpose: money.PaymentPurposeWalletTopUp, CommissionTreatment: money.CommissionNonCommissionable, PayerBinding: money.PayerUnattributedExternal, Currency: a.Currency, GrossAmountMinor: a.AmountMinor, Status: money.PaymentSettled, SettledAt: money.NormalizeTimestamp(o.OccurredAt), ProviderReference: "provider-trade:" + binding.ClaimID(), Version: 1}}
}

// Only channel adapters construct these observations after cryptographic
// verification. HTTP callers cannot submit this type directly.
type ProviderObservation struct {
	Merchant            TopUpMerchant
	MerchantOrderID     string
	EventID             string
	Kind                string // PAYMENT or REFUND
	State               string // PAID, PAID_REFUND_UNKNOWN, UNPAID, NOT_FOUND, CLOSED, UNKNOWN, REFUNDED, REFUND_PENDING, REFUND_CLOSED
	TradeID             string
	RefundRequestID     string
	NativeRefundID      string
	Currency            string
	AmountMinor         int64
	TotalMinor          int64
	OccurredAt          time.Time
	VerificationVersion string
}

func (o ProviderObservation) Validate() error {
	if o.Merchant.Validate() != nil || !isCanonicalIdentifier(o.MerchantOrderID) || !isCanonicalIdentifier(o.EventID) || !isCanonicalIdentifier(o.VerificationVersion) || o.Currency != CurrencyCNY {
		return ErrInvalid
	}
	if o.Kind == "PAYMENT" {
		switch o.State {
		case "PAID", "PAID_REFUND_UNKNOWN":
			if o.TradeID == "" || o.AmountMinor <= 0 || o.OccurredAt.IsZero() {
				return ErrInvalid
			}
		case "UNPAID", "NOT_FOUND", "CLOSED", "UNKNOWN":
		default:
			return ErrInvalid
		}
	} else if o.Kind == "REFUND" {
		if o.TradeID == "" || o.RefundRequestID == "" || o.AmountMinor <= 0 || o.TotalMinor <= 0 {
			return ErrInvalid
		}
		switch o.State {
		case "REFUNDED":
			if o.OccurredAt.IsZero() {
				return ErrInvalid
			}
		case "REFUND_PENDING", "REFUND_CLOSED":
		default:
			return ErrInvalid
		}
	} else {
		return ErrInvalid
	}
	return nil
}
func (o ProviderObservation) Matches(a TopUpPaymentAttempt) bool {
	if o.Merchant != a.Merchant || o.MerchantOrderID != a.MerchantOrderID || o.Currency != a.Currency {
		return false
	}
	if o.Kind == "PAYMENT" && (o.State == "PAID" || o.State == "PAID_REFUND_UNKNOWN") {
		return o.AmountMinor == a.AmountMinor
	}
	if o.Kind == "REFUND" {
		return o.TotalMinor == a.AmountMinor
	}
	return true
}
func (o ProviderObservation) RefundKey(payment string) money.TopUpReversalKey {
	return money.TopUpReversalKey{PaymentID: payment, Kind: money.WalletReversalRefund, ReversalID: "provider-refund:" + money.TopUpFingerprint([]string{string(o.Merchant.Provider), o.Merchant.Environment, o.Merchant.MerchantID, o.RefundRequestID})}
}

type TopUpRefundIntent struct {
	RefundID          string
	OrderID           string
	OrganizationID    string
	ActorID           string
	IdempotencyKey    string
	AmountMinor       int64
	TotalMinor        int64
	Reason            string
	Merchant          TopUpMerchant
	MerchantOrderID   string
	TradeID           string
	PaymentID         string
	ProviderRequestID string
	State             string
	Dispatched        bool
	Version           int64
	CreatedAt         time.Time
	NextCheckAt       time.Time
	LeaseToken        string
	LeaseUntil        time.Time
}

func (r TopUpRefundIntent) HoldInput() money.TopUpRefundInput {
	return money.TopUpRefundInput{Key: money.TopUpReversalKey{PaymentID: r.PaymentID, Kind: money.WalletReversalRefund, ReversalID: r.RefundID}, OrganizationID: r.OrganizationID, CommercialOrderID: r.OrderID, AmountMinor: r.AmountMinor, ApprovalID: "approval:" + money.TopUpFingerprint([]string{r.ActorID, r.OrderID, r.IdempotencyKey, r.Reason})}
}

type TopUpProviderPort interface {
	Merchant() TopUpMerchant
	Available() bool
	CreateOrReadCheckout(context.Context, TopUpPaymentAttempt) (CheckoutAction, error)
	QueryPayment(context.Context, TopUpPaymentAttempt) (ProviderObservation, error)
	ClosePayment(context.Context, TopUpPaymentAttempt) (ProviderObservation, error)
	Refund(context.Context, TopUpRefundIntent) (ProviderObservation, error)
	QueryRefund(context.Context, TopUpRefundIntent) (ProviderObservation, error)
}
type TopUpPayloadProtection interface {
	Seal(string, string) ([]byte, error)
	Open(string, []byte) (string, error)
}
type TopUpAuthorizer interface {
	AuthorizeTopUp(context.Context, string, string, bool) error
}
type TopUpStore interface {
	CreateTopUpAttempt(context.Context, CreateWalletTopUpOrderRequest, TopUpMerchant, time.Time, time.Time) (TopUpPaymentAttempt, error)
	FindTopUpAttempt(context.Context, string, string) (TopUpPaymentAttempt, error)
	ReadTopUpAttempt(context.Context, string, string) (TopUpPaymentAttempt, error)
	SaveTopUpAttempt(context.Context, TopUpPaymentAttempt) (TopUpPaymentAttempt, error)
	CompleteTopUpOrder(context.Context, TopUpPaymentAttempt, money.TopUpPostingReceipt) (TopUpPaymentAttempt, error)
	RecordTopUpObservation(context.Context, ProviderObservation) error
	ReadTopUpObservations(context.Context, TopUpPaymentAttempt) ([]ProviderObservation, error)
	ListRecoverableTopUps(context.Context, PaymentProvider, time.Time, int) ([]TopUpPaymentAttempt, error)
	ReadTopUpForRefund(context.Context, string) (TopUpPaymentAttempt, error)
	CreateTopUpRefund(context.Context, TopUpRefundIntent) (TopUpRefundIntent, error)
	SaveTopUpRefund(context.Context, TopUpRefundIntent) (TopUpRefundIntent, error)
	ListRecoverableTopUpRefunds(context.Context, PaymentProvider, time.Time, int) ([]TopUpRefundIntent, error)
}
