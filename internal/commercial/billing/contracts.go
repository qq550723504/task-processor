package billing

import (
	"context"
	"errors"
	"strings"
	"time"

	"task-processor/internal/ledger/money"
	"task-processor/internal/ledger/orgresource"
)

const CurrencyCNY = money.WalletCurrencyCNY

var (
	ErrInvalid                = errors.New("commercial billing input is invalid")
	ErrOfferUnavailable       = errors.New("commercial offer is unavailable")
	ErrQuoteExpired           = errors.New("commercial quote is expired")
	ErrInsufficientFunds      = errors.New("commercial wallet funds are insufficient")
	ErrConflict               = errors.New("commercial billing operation conflict")
	ErrReconciliationRequired = errors.New("commercial billing reconciliation is required")
	ErrFeatureUnavailable     = errors.New("commercial billing feature is unavailable")
)

type ProductKind string

const (
	ProductStoreRenewalPeriod ProductKind = "STORE_RENEWAL_PERIOD"
	ProductAIPoint            ProductKind = "AI_POINT"
	ProductDataRow            ProductKind = "DATA_ROW"
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
	MinQuantity    int64
	MaxQuantity    int64
	Status         OfferStatus
	StartsAt       *time.Time
	ExpiresAt      *time.Time
}

func (offer Offer) Validate() error {
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
}

func (quote Quote) Validate() error {
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
	return nil
}

type OrderKind string

const (
	OrderWalletTopUp      OrderKind = "WALLET_TOP_UP"
	OrderResourcePurchase OrderKind = "RESOURCE_PURCHASE"
)

type OrderStatus string

const (
	OrderPending                OrderStatus = "PENDING"
	OrderFundsReserved          OrderStatus = "FUNDS_RESERVED"
	OrderFulfilling             OrderStatus = "FULFILLING"
	OrderFulfilled              OrderStatus = "FULFILLED"
	OrderCancelled              OrderStatus = "CANCELLED"
	OrderReconciliationRequired OrderStatus = "RECONCILIATION_REQUIRED"
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
	OrderID                     string
	OrganizationID              string
	Kind                        OrderKind
	QuoteID                     string
	Currency                    string
	AmountMinor                 int64
	Status                      OrderStatus
	WalletReservationID         string
	WalletReservationState      money.WalletReservationState
	PaymentID                   string
	ResourceGrantOperationID    string
	ResourceGrantSourceType     string
	ResourceGrantSourceIdentity string
	Items                       []OrderItem
	IdempotencyKey              string
	RequestFingerprint          string
	Version                     int64
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

func (order Order) Validate() error {
	if !isCanonicalIdentifier(order.OrderID) ||
		!isCanonicalIdentifier(order.OrganizationID) ||
		(order.Kind != OrderWalletTopUp && order.Kind != OrderResourcePurchase) ||
		order.Currency != CurrencyCNY ||
		order.AmountMinor <= 0 ||
		!validOrderStatus(order.Status) ||
		strings.TrimSpace(order.IdempotencyKey) == "" ||
		strings.TrimSpace(order.RequestFingerprint) == "" ||
		order.Version < 1 ||
		order.CreatedAt.IsZero() ||
		order.UpdatedAt.IsZero() {
		return ErrInvalid
	}
	switch order.Kind {
	case OrderResourcePurchase:
		if strings.TrimSpace(order.QuoteID) == "" || len(order.Items) != 1 || order.Items[0].Validate() != nil || order.Items[0].AmountMinor != order.AmountMinor || (requiresWalletReservation(order.Status) && strings.TrimSpace(order.WalletReservationID) == "") || !validWalletReservationStateForOrder(order) || (order.Status == OrderFulfilled && !hasResourceGrantProof(order)) || ((order.Status == OrderPending || order.Status == OrderFundsReserved || order.Status == OrderCancelled) && hasResourceGrantEvidence(order)) {
			return ErrInvalid
		}
	case OrderWalletTopUp:
		if len(order.Items) != 0 || hasWalletReservationEvidence(order) || hasResourceGrantEvidence(order) || topUpUsesResourcePurchaseLifecycle(order.Status) || (order.Status == OrderFulfilled && strings.TrimSpace(order.PaymentID) == "") || (order.Status != OrderFulfilled && strings.TrimSpace(order.PaymentID) != "") {
			return ErrInvalid
		}
	}
	return nil
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
