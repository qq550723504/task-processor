package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/ledger/money"
	"task-processor/internal/ledger/orgresource"
)

const CurrencyCNY = money.WalletCurrencyCNY

var (
	ErrInvalid                        = errors.New("commercial billing input is invalid")
	ErrOfferUnavailable               = errors.New("commercial offer is unavailable")
	ErrQuoteExpired                   = errors.New("commercial quote is expired")
	ErrInsufficientFunds              = errors.New("commercial wallet funds are insufficient")
	ErrConflict                       = errors.New("commercial billing operation conflict")
	ErrNotFound                       = errors.New("commercial billing resource not found")
	ErrOrderCancelled                 = errors.New("commercial order was cancelled")
	ErrResourceGrantRejected          = errors.New("commercial resource grant was rejected")
	ErrReconciliationRequired         = errors.New("commercial billing reconciliation is required")
	ErrFeatureUnavailable             = errors.New("commercial billing feature is unavailable")
	ErrPaymentMethodUnavailable       = errors.New("commercial payment method is unavailable")
	ErrActiveSubscriptionExists       = errors.New("an active subscription already exists")
	ErrPlanChanged                    = errors.New("the quoted subscription plan changed")
	ErrAuthorizationRevoked           = errors.New("commercial purchase authorization was revoked")
	ErrSubscriptionActivationNotFound = errors.New("subscription activation decision is not found")
)

type ProductKind string

const (
	ProductStoreRenewalPeriod ProductKind = "STORE_RENEWAL_PERIOD"
	ProductAIPoint            ProductKind = "AI_POINT"
	ProductDataRow            ProductKind = "DATA_ROW"
	ProductSubscriptionPlan   ProductKind = "SUBSCRIPTION_PLAN"
)

type SettlementMode string

const (
	SettlementZeroPrice       SettlementMode = "ZERO_PRICE"
	SettlementWallet          SettlementMode = "WALLET"
	SettlementExternalPayment SettlementMode = "EXTERNAL_PAYMENT"
)

func ResourceTypeForProduct(kind ProductKind) (orgresource.ResourceType, bool) {
	switch kind {
	case ProductStoreRenewalPeriod:
		return orgresource.ResourceStoreRenewalPeriod, true
	case ProductAIPoint:
		return orgresource.ResourceAIPoint, true
	case ProductDataRow:
		return orgresource.ResourceDataRow, true
	default:
		return "", false
	}
}

type OfferStatus string

const (
	OfferActive   OfferStatus = "ACTIVE"
	OfferDisabled OfferStatus = "DISABLED"
)

type Offer struct {
	OfferID        string
	ProductKind    ProductKind
	ResourceType   orgresource.ResourceType
	Currency       string
	PricingVersion string
	// UnitPriceMinor is server-owned pricing data. It is intentionally absent
	// from browser requests and remains unset until an approved offer exists.
	UnitPriceMinor int64
	MinQuantity    int64
	MaxQuantity    int64
	Status         OfferStatus
	StartsAt       *time.Time
	ExpiresAt      *time.Time
	PlanCode       string
	TermMonths     int
	SettlementMode SettlementMode
}

func (offer Offer) Validate() error {
	if offer.ProductKind == ProductSubscriptionPlan {
		if strings.TrimSpace(offer.OfferID) == "" || offer.ResourceType != "" || offer.Currency != CurrencyCNY || strings.TrimSpace(offer.PricingVersion) == "" || strings.TrimSpace(offer.PlanCode) == "" || len(offer.PlanCode) > 64 || offer.TermMonths < 1 || offer.TermMonths > 120 || offer.MinQuantity != 0 || offer.MaxQuantity != 0 || (offer.Status != OfferActive && offer.Status != OfferDisabled) {
			return ErrInvalid
		}
		switch offer.SettlementMode {
		case SettlementZeroPrice:
			if offer.UnitPriceMinor != 0 {
				return ErrInvalid
			}
		case SettlementWallet, SettlementExternalPayment:
			if offer.UnitPriceMinor <= 0 {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
		if offer.StartsAt != nil && offer.ExpiresAt != nil && !offer.StartsAt.Before(*offer.ExpiresAt) {
			return ErrInvalid
		}
		return nil
	}
	resourceType, ok := ResourceTypeForProduct(offer.ProductKind)
	if strings.TrimSpace(offer.OfferID) == "" ||
		!ok ||
		offer.ResourceType != resourceType ||
		offer.Currency != CurrencyCNY ||
		strings.TrimSpace(offer.PricingVersion) == "" ||
		offer.MinQuantity <= 0 ||
		offer.MaxQuantity < offer.MinQuantity ||
		(offer.Status != OfferActive && offer.Status != OfferDisabled) {
		return ErrInvalid
	}
	if offer.PlanCode != "" || offer.TermMonths != 0 || offer.SettlementMode != "" {
		return ErrInvalid
	}
	if offer.StartsAt != nil && offer.ExpiresAt != nil && !offer.StartsAt.Before(*offer.ExpiresAt) {
		return ErrInvalid
	}
	return nil
}

type Quote struct {
	QuoteID          string
	OrganizationID   string
	OfferID          string
	ProductKind      ProductKind
	ResourceType     orgresource.ResourceType
	ResourceQuantity int64
	Currency         string
	TotalMinor       int64
	PricingVersion   string
	ExpiresAt        time.Time
	Fingerprint      string
	CreatedAt        time.Time
	PlanCode         string
	PlanFingerprint  string
	TermMonths       int
	SettlementMode   SettlementMode
}

func (quote Quote) Validate() error {
	if quote.ProductKind == ProductSubscriptionPlan {
		if strings.TrimSpace(quote.QuoteID) == "" || strings.TrimSpace(quote.OrganizationID) == "" || strings.TrimSpace(quote.OfferID) == "" || quote.ResourceType != "" || quote.ResourceQuantity != 0 || quote.Currency != CurrencyCNY || quote.TotalMinor < 0 || strings.TrimSpace(quote.PricingVersion) == "" || strings.TrimSpace(quote.PlanCode) == "" || strings.TrimSpace(quote.PlanFingerprint) == "" || quote.TermMonths < 1 || quote.TermMonths > 120 || quote.ExpiresAt.IsZero() || quote.CreatedAt.IsZero() || !quote.CreatedAt.Before(quote.ExpiresAt) || strings.TrimSpace(quote.Fingerprint) == "" {
			return ErrInvalid
		}
		if quote.SettlementMode == SettlementZeroPrice && quote.TotalMinor != 0 || (quote.SettlementMode == SettlementWallet || quote.SettlementMode == SettlementExternalPayment) && quote.TotalMinor <= 0 {
			return ErrInvalid
		}
		if quote.SettlementMode != SettlementZeroPrice && quote.SettlementMode != SettlementWallet && quote.SettlementMode != SettlementExternalPayment {
			return ErrInvalid
		}
		return nil
	}
	resourceType, ok := ResourceTypeForProduct(quote.ProductKind)
	if strings.TrimSpace(quote.QuoteID) == "" ||
		strings.TrimSpace(quote.OrganizationID) == "" ||
		strings.TrimSpace(quote.OfferID) == "" ||
		!ok ||
		quote.ResourceType != resourceType ||
		quote.ResourceQuantity <= 0 ||
		quote.Currency != CurrencyCNY ||
		quote.TotalMinor <= 0 ||
		strings.TrimSpace(quote.PricingVersion) == "" ||
		quote.ExpiresAt.IsZero() ||
		quote.CreatedAt.IsZero() ||
		!quote.CreatedAt.Before(quote.ExpiresAt) ||
		strings.TrimSpace(quote.Fingerprint) == "" {
		return ErrInvalid
	}
	if quote.PlanCode != "" || quote.PlanFingerprint != "" || quote.TermMonths != 0 || quote.SettlementMode != "" {
		return ErrInvalid
	}
	return nil
}

type OrderKind string

const (
	OrderWalletTopUp          OrderKind = "WALLET_TOP_UP"
	OrderResourcePurchase     OrderKind = "RESOURCE_PURCHASE"
	OrderSubscriptionPurchase OrderKind = "SUBSCRIPTION_PURCHASE"
)

type OrderStatus string

type OrderFailureCode string

const (
	OrderPending                OrderStatus = "PENDING"
	OrderFundsReserved          OrderStatus = "FUNDS_RESERVED"
	OrderFulfilling             OrderStatus = "FULFILLING"
	OrderFulfilled              OrderStatus = "FULFILLED"
	OrderCancelled              OrderStatus = "CANCELLED"
	OrderReconciliationRequired OrderStatus = "RECONCILIATION_REQUIRED"
)

const (
	OrderFailureInsufficientFunds        OrderFailureCode = "INSUFFICIENT_FUNDS"
	OrderFailureGrantRejected            OrderFailureCode = "RESOURCE_GRANT_REJECTED"
	OrderFailureActiveSubscriptionExists OrderFailureCode = "ACTIVE_SUBSCRIPTION_EXISTS"
	OrderFailurePlanChanged              OrderFailureCode = "PLAN_CHANGED"
	OrderFailureAuthorizationRevoked     OrderFailureCode = "AUTHORIZATION_REVOKED"
)

type PendingSubscriptionOrderEffect string

const (
	PendingSubscriptionOrderEffectNone     PendingSubscriptionOrderEffect = ""
	PendingSubscriptionOrderEffectReserve  PendingSubscriptionOrderEffect = "RESERVE"
	PendingSubscriptionOrderEffectActivate PendingSubscriptionOrderEffect = "ACTIVATE"
)

type SubscriptionOrderTerminalIntent string

const (
	SubscriptionOrderTerminalIntentNone   SubscriptionOrderTerminalIntent = ""
	SubscriptionOrderTerminalIntentCancel SubscriptionOrderTerminalIntent = "CANCEL"
)

type OrderItem struct {
	OrderItemID      string
	ProductKind      ProductKind
	ResourceType     orgresource.ResourceType
	ResourceQuantity int64
	AmountMinor      int64
}

func (item OrderItem) Validate() error {
	resourceType, ok := ResourceTypeForProduct(item.ProductKind)
	if !isCanonicalIdentifier(item.OrderItemID) ||
		!ok ||
		item.ResourceType != resourceType ||
		item.ResourceQuantity <= 0 ||
		item.AmountMinor <= 0 {
		return ErrInvalid
	}
	return nil
}

type Order struct {
	OrderID                             string
	OrganizationID                      string
	Kind                                OrderKind
	Description                         string
	QuoteID                             string
	Currency                            string
	AmountMinor                         int64
	Status                              OrderStatus
	FailureCode                         OrderFailureCode
	WalletReservationID                 string
	WalletReservationState              money.WalletReservationState
	PaymentID                           string
	ResourceGrantOperationID            string
	ResourceGrantSourceType             string
	ResourceGrantSourceIdentity         string
	Items                               []OrderItem
	IdempotencyKey                      string
	RequestFingerprint                  string
	Version                             int64
	CreatedAt                           time.Time
	UpdatedAt                           time.Time
	ActorID                             string
	ProductKind                         ProductKind
	PlanCode                            string
	PlanFingerprint                     string
	TermMonths                          int
	SettlementMode                      SettlementMode
	ActivationOperationID               string
	ActivationRequestFingerprint        string
	ActivationOutcome                   SubscriptionActivationOutcome
	ActivationFailureCode               SubscriptionActivationFailureCode
	ActivationSubscriptionID            string
	ActivationStartsAt                  *time.Time
	ActivationExpiresAt                 *time.Time
	ActivationEntitlementSetFingerprint string
	ActivationDecidedAt                 *time.Time
	PendingEffect                       PendingSubscriptionOrderEffect
	PendingEffectAdmittedAt             *time.Time
	TerminalIntent                      SubscriptionOrderTerminalIntent
	TerminalIntentReason                OrderFailureCode
	TerminalIntentAt                    *time.Time
}

// DescribeOrder returns the immutable, server-derived display and search text
// for an order from its canonical kind and item facts.
func DescribeOrder(kind OrderKind, productKind ProductKind, quantity int64) string {
	if kind == OrderWalletTopUp {
		return "钱包充值"
	}
	if kind != OrderResourcePurchase || quantity <= 0 {
		return ""
	}
	var label string
	switch productKind {
	case ProductStoreRenewalPeriod:
		label = "店铺续费期"
	case ProductAIPoint:
		label = "AI 点数"
	case ProductDataRow:
		label = "数据资源"
	default:
		return ""
	}
	return fmt.Sprintf("%s × %d", label, quantity)
}

func (order Order) Validate() error {
	if !isCanonicalIdentifier(order.OrderID) ||
		!isCanonicalIdentifier(order.OrganizationID) ||
		(order.Kind != OrderWalletTopUp && order.Kind != OrderResourcePurchase && order.Kind != OrderSubscriptionPurchase) ||
		order.Currency != CurrencyCNY ||
		(order.Kind != OrderSubscriptionPurchase && order.AmountMinor <= 0) ||
		order.AmountMinor < 0 ||
		!validOrderStatus(order.Status) ||
		strings.TrimSpace(order.IdempotencyKey) == "" ||
		strings.TrimSpace(order.RequestFingerprint) == "" ||
		order.Version < 1 ||
		order.CreatedAt.IsZero() ||
		order.UpdatedAt.IsZero() ||
		(order.FailureCode != "" && order.Status != OrderCancelled) ||
		!validOrderFailureCode(order) {
		return ErrInvalid
	}
	switch order.Kind {
	case OrderResourcePurchase:
		grantMustBeProven := order.Status == OrderFulfilled || (order.Status == OrderReconciliationRequired && order.WalletReservationState == money.WalletReservationCommitted)
		if strings.TrimSpace(order.QuoteID) == "" || order.PaymentID != "" || len(order.Items) != 1 || order.Items[0].Validate() != nil || order.Items[0].AmountMinor != order.AmountMinor || (requiresWalletReservation(order.Status) && strings.TrimSpace(order.WalletReservationID) == "") || !validWalletReservationStateForOrder(order) || (grantMustBeProven && !hasResourceGrantProof(order)) || ((order.Status == OrderPending || order.Status == OrderFundsReserved || order.Status == OrderCancelled) && hasResourceGrantEvidence(order)) {
			return ErrInvalid
		}
	case OrderWalletTopUp:
		if order.QuoteID != "" || len(order.Items) != 0 || hasWalletReservationEvidence(order) || hasResourceGrantEvidence(order) || topUpUsesResourcePurchaseLifecycle(order.Status) || (order.Status == OrderFulfilled && strings.TrimSpace(order.PaymentID) == "") || (order.Status != OrderFulfilled && strings.TrimSpace(order.PaymentID) != "") {
			return ErrInvalid
		}
	case OrderSubscriptionPurchase:
		if !validSubscriptionOrder(order) {
			return ErrInvalid
		}
	}
	return nil
}

func validSubscriptionOrder(order Order) bool {
	if !isCanonicalIdentifier(order.ActorID) || order.ProductKind != ProductSubscriptionPlan || strings.TrimSpace(order.QuoteID) == "" || strings.TrimSpace(order.PlanCode) == "" || strings.TrimSpace(order.PlanFingerprint) == "" || order.TermMonths < 1 || order.TermMonths > 120 || len(order.Items) != 0 || order.PaymentID != "" || hasResourceGrantEvidence(order) {
		return false
	}
	if order.PendingEffect != PendingSubscriptionOrderEffectNone && order.PendingEffect != PendingSubscriptionOrderEffectReserve && order.PendingEffect != PendingSubscriptionOrderEffectActivate {
		return false
	}
	if (order.PendingEffect == PendingSubscriptionOrderEffectNone) != (order.PendingEffectAdmittedAt == nil) {
		return false
	}
	if order.TerminalIntent == SubscriptionOrderTerminalIntentNone {
		if order.TerminalIntentAt != nil || order.TerminalIntentReason != "" {
			return false
		}
	} else if order.TerminalIntent != SubscriptionOrderTerminalIntentCancel || order.TerminalIntentAt == nil || order.TerminalIntentReason != OrderFailureAuthorizationRevoked || order.PendingEffect != PendingSubscriptionOrderEffectNone || order.ActivationOutcome != "" || (order.Status != OrderCancelled && order.Status != OrderReconciliationRequired) {
		return false
	}
	if (order.Status == OrderFulfilled || order.Status == OrderCancelled) && order.PendingEffect != PendingSubscriptionOrderEffectNone {
		return false
	}
	switch order.SettlementMode {
	case SettlementZeroPrice:
		if order.AmountMinor != 0 || hasWalletReservationEvidence(order) || order.PendingEffect == PendingSubscriptionOrderEffectReserve {
			return false
		}
	case SettlementWallet:
		if order.AmountMinor <= 0 {
			return false
		}
	case SettlementExternalPayment:
		return false
	default:
		return false
	}
	if !validSubscriptionActivationProof(order) {
		return false
	}
	switch order.Status {
	case OrderPending, OrderFundsReserved:
		if order.ActivationOutcome != "" {
			return false
		}
	case OrderFulfilling, OrderFulfilled:
		if order.ActivationOutcome != SubscriptionActivationActivated {
			return false
		}
	case OrderCancelled:
		if order.FailureCode == "" || order.ActivationOutcome == SubscriptionActivationActivated {
			return false
		}
	}
	if order.SettlementMode == SettlementWallet {
		switch order.Status {
		case OrderPending:
			if hasWalletReservationEvidence(order) {
				return false
			}
		case OrderFundsReserved, OrderFulfilling:
			if order.WalletReservationID == "" || order.WalletReservationState != money.WalletReservationReserved {
				return false
			}
		case OrderFulfilled:
			if order.WalletReservationID == "" || order.WalletReservationState != money.WalletReservationCommitted {
				return false
			}
		case OrderCancelled:
			if hasWalletReservationEvidence(order) {
				return false
			}
		case OrderReconciliationRequired:
			if hasWalletReservationEvidence(order) && order.WalletReservationState != money.WalletReservationReserved && order.WalletReservationState != money.WalletReservationCommitted {
				return false
			}
		}
	}
	return true
}

func validOrderFailureCode(order Order) bool {
	if order.FailureCode == "" {
		return true
	}
	switch order.Kind {
	case OrderResourcePurchase, OrderWalletTopUp:
		return order.FailureCode == OrderFailureInsufficientFunds || order.FailureCode == OrderFailureGrantRejected
	case OrderSubscriptionPurchase:
		return order.FailureCode == OrderFailureInsufficientFunds ||
			order.FailureCode == OrderFailureActiveSubscriptionExists ||
			order.FailureCode == OrderFailurePlanChanged ||
			order.FailureCode == OrderFailureAuthorizationRevoked
	default:
		return false
	}
}

func validSubscriptionActivationProof(order Order) bool {
	switch order.ActivationOutcome {
	case "":
		return order.ActivationOperationID == "" && order.ActivationRequestFingerprint == "" && order.ActivationFailureCode == "" && order.ActivationSubscriptionID == "" && order.ActivationStartsAt == nil && order.ActivationExpiresAt == nil && order.ActivationEntitlementSetFingerprint == "" && order.ActivationDecidedAt == nil
	case SubscriptionActivationActivated:
		return order.ActivationOperationID != "" && order.ActivationRequestFingerprint != "" && order.ActivationFailureCode == "" && order.ActivationSubscriptionID != "" && order.ActivationStartsAt != nil && order.ActivationExpiresAt != nil && order.ActivationEntitlementSetFingerprint != "" && order.ActivationDecidedAt != nil
	case SubscriptionActivationRejected:
		return order.ActivationOperationID != "" && order.ActivationRequestFingerprint != "" && (order.ActivationFailureCode == SubscriptionActivationActiveSubscriptionExists || order.ActivationFailureCode == SubscriptionActivationPlanChanged) && order.ActivationSubscriptionID == "" && order.ActivationStartsAt == nil && order.ActivationExpiresAt == nil && order.ActivationEntitlementSetFingerprint == "" && order.ActivationDecidedAt != nil
	default:
		return false
	}
}

func isCanonicalIdentifier(value string) bool {
	return value != "" && len(value) <= 128 && strings.TrimSpace(value) == value
}

func hasResourceGrantProof(order Order) bool {
	if len(order.Items) != 1 || order.Items[0].Validate() != nil {
		return false
	}
	expectedSourceIdentity := orgresource.CommercialOrderItemSourceIdentity(
		order.OrganizationID,
		order.OrderID,
		order.Items[0].OrderItemID,
	)
	return strings.TrimSpace(order.ResourceGrantOperationID) != "" &&
		order.ResourceGrantSourceType == orgresource.SourceCommercialOrderItem &&
		order.ResourceGrantSourceIdentity == expectedSourceIdentity
}

func hasResourceGrantEvidence(order Order) bool {
	return strings.TrimSpace(order.ResourceGrantOperationID) != "" ||
		strings.TrimSpace(order.ResourceGrantSourceType) != "" ||
		strings.TrimSpace(order.ResourceGrantSourceIdentity) != ""
}

func hasWalletReservationEvidence(order Order) bool {
	return strings.TrimSpace(order.WalletReservationID) != "" || order.WalletReservationState != ""
}

func validWalletReservationStateForOrder(order Order) bool {
	switch order.Status {
	case OrderPending, OrderCancelled:
		return !hasWalletReservationEvidence(order)
	case OrderFundsReserved, OrderFulfilling:
		return order.WalletReservationState == money.WalletReservationReserved
	case OrderFulfilled:
		return order.WalletReservationState == money.WalletReservationCommitted
	case OrderReconciliationRequired:
		return order.WalletReservationState == money.WalletReservationReserved || order.WalletReservationState == money.WalletReservationCommitted
	default:
		return false
	}
}

func topUpUsesResourcePurchaseLifecycle(status OrderStatus) bool {
	switch status {
	case OrderFundsReserved, OrderFulfilling, OrderReconciliationRequired:
		return true
	default:
		return false
	}
}

func requiresWalletReservation(status OrderStatus) bool {
	switch status {
	case OrderFundsReserved, OrderFulfilling, OrderFulfilled, OrderReconciliationRequired:
		return true
	default:
		return false
	}
}

func validOrderStatus(status OrderStatus) bool {
	switch status {
	case OrderPending, OrderFundsReserved, OrderFulfilling, OrderFulfilled, OrderCancelled, OrderReconciliationRequired:
		return true
	default:
		return false
	}
}

type QuoteRequest struct {
	OrganizationID string
	OfferID        string
	Quantity       int64
}

type CreateResourceOrderRequest struct {
	OrganizationID string
	QuoteID        string
	IdempotencyKey string
}

type CreateWalletTopUpOrderRequest struct {
	OrganizationID string
	Currency       string
	AmountMinor    int64
	IdempotencyKey string
}

type CreateSubscriptionOrderRequest struct {
	OrganizationID string
	ActorID        string
	QuoteID        string
	IdempotencyKey string
}

type SubscriptionQuoteRequest struct {
	OrganizationID string
	OfferID        string
}

type SubscriptionOfferAvailability string

const (
	SubscriptionOfferAvailable          SubscriptionOfferAvailability = "available"
	SubscriptionOfferCurrentPlan        SubscriptionOfferAvailability = "current_plan"
	SubscriptionOfferActiveConflict     SubscriptionOfferAvailability = "active_subscription_conflict"
	SubscriptionOfferPaymentUnavailable SubscriptionOfferAvailability = "payment_unavailable"
	SubscriptionOfferUnavailable        SubscriptionOfferAvailability = "offer_unavailable"
)

type SubscriptionOfferView struct {
	Offer        Offer
	PlanName     string
	Availability SubscriptionOfferAvailability
}

type SubscriptionPlanSnapshot struct {
	PlanCode    string
	DisplayName string
	Fingerprint string
}

type SubscriptionPurchaseState struct {
	PlanCode       string
	BlocksPurchase bool
}

type SubscriptionActivationOutcome string

const (
	SubscriptionActivationActivated SubscriptionActivationOutcome = "ACTIVATED"
	SubscriptionActivationRejected  SubscriptionActivationOutcome = "REJECTED"
)

type SubscriptionActivationFailureCode string

const (
	SubscriptionActivationActiveSubscriptionExists SubscriptionActivationFailureCode = "ACTIVE_SUBSCRIPTION_EXISTS"
	SubscriptionActivationPlanChanged              SubscriptionActivationFailureCode = "PLAN_CHANGED"
)

type SubscriptionActivationRequest struct {
	OperationID       string
	OrganizationID    string
	ActorID           string
	CommercialOrderID string
	PlanCode          string
	PlanFingerprint   string
	TermMonths        int
}

type SubscriptionActivationResult struct {
	OperationID                  string
	OrganizationID               string
	CommercialOrderID            string
	PlanCode                     string
	PlanFingerprint              string
	ActivationRequestFingerprint string
	Outcome                      SubscriptionActivationOutcome
	FailureCode                  SubscriptionActivationFailureCode
	SubscriptionID               string
	StartsAt                     *time.Time
	ExpiresAt                    *time.Time
	EntitlementSetFingerprint    string
	DecidedAt                    time.Time
	Existing                     bool
}

type CommercialPurchaseAuthorization struct {
	OrganizationID string
	ActorID        string
	Roles          []string
	Allowed        bool
	ObservedAt     time.Time
}

const MaxOrderPageSize = 50

type OrderFilter struct {
	Query       string
	Kind        *OrderKind
	ProductKind *ProductKind
	Status      *OrderStatus
	From        *time.Time
	Until       *time.Time
	Cursor      string
	Limit       int
}

type OrderPage struct {
	Items      []Order
	NextCursor string
}

type OrderSummary struct {
	Currency               string
	From                   time.Time
	Until                  time.Time
	SpendMinor             int64
	StoreRenewalSpendMinor int64
	AIPointSpendMinor      int64
	DataRowSpendMinor      int64
	OtherSpendMinor        int64
	ObservedAt             time.Time
}

type OfferCatalog interface {
	ReadOffer(context.Context, string) (Offer, error)
}

type SubscriptionOfferCatalog interface {
	OfferCatalog
	ListSubscriptionOffers(context.Context) ([]Offer, error)
}

type SubscriptionQuoteStore interface {
	CreateSubscriptionQuote(context.Context, SubscriptionQuoteRequest, Offer, SubscriptionPlanSnapshot) (Quote, error)
}

type QuoteEngine interface {
	CreateQuote(context.Context, QuoteRequest) (Quote, error)
	ReadQuote(context.Context, string, string) (Quote, error)
}

type OrderReader interface {
	ReadOrder(context.Context, string, string) (Order, error)
	ListOrders(context.Context, string, OrderFilter) (OrderPage, error)
	ReadOrderSummary(context.Context, string, time.Time, time.Time) (OrderSummary, error)
}

type OrderCommander interface {
	CreateResourceOrder(context.Context, CreateResourceOrderRequest) (Order, error)
	CreateWalletTopUpOrder(context.Context, CreateWalletTopUpOrderRequest) (Order, error)
}

type SubscriptionOrderStore interface {
	CreatePendingSubscriptionOrder(context.Context, CreateSubscriptionOrderRequest, Quote) (Order, error)
	FindSubscriptionOrderByIdempotency(context.Context, string, string) (Order, bool, error)
	AdmitSubscriptionOrderEffect(context.Context, string, string, int64, PendingSubscriptionOrderEffect, time.Time) (Order, bool, error)
	PersistSubscriptionOrder(context.Context, Order) error
	ListRecoverableSubscriptionOrders(context.Context, int) ([]Order, error)
}

// WalletPort is the commercial owner's only money dependency. It consumes the
// canonical money owner instead of duplicating balance or settlement logic.
type WalletPort interface {
	money.OrganizationWalletReader
	money.OrganizationWalletCommander
}

// PurchasedResourceGrantPort is deliberately the narrow source-bound resource
// acquisition use case, never a generic positive-mint interface.
type PurchasedResourceGrantPort interface {
	GrantPurchasedResource(context.Context, orgresource.PurchasedResourceGrantInput) (orgresource.PurchasedResourceGrantResult, error)
}

type SubscriptionPurchasePort interface {
	ResolvePurchasablePlan(context.Context, string) (SubscriptionPlanSnapshot, error)
	ReadPurchasedSubscriptionState(context.Context, string) (SubscriptionPurchaseState, error)
	ActivatePurchasedSubscription(context.Context, SubscriptionActivationRequest) (SubscriptionActivationResult, error)
	ReadPurchasedSubscriptionActivation(context.Context, string, string) (SubscriptionActivationResult, error)
}

type SubscriptionPurchaseRecoveryAuthorizer interface {
	ReauthorizeCommercialPurchase(context.Context, string, string) (CommercialPurchaseAuthorization, error)
}
