package commercialbilling

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/orgresource"
)

type offerRow struct {
	OfferID        string     `gorm:"column:offer_id;primaryKey;size:128"`
	ProductKind    string     `gorm:"column:product_kind;size:64;not null"`
	ResourceType   string     `gorm:"column:resource_type;size:64;not null"`
	Currency       string     `gorm:"column:currency;size:3;not null"`
	UnitPriceMinor int64      `gorm:"column:unit_price_minor;not null;default:0"`
	PricingVersion string     `gorm:"column:pricing_version;size:96;not null"`
	MinQuantity    int64      `gorm:"column:min_quantity;not null"`
	MaxQuantity    int64      `gorm:"column:max_quantity;not null"`
	Status         string     `gorm:"column:status;size:16;not null"`
	StartsAt       *time.Time `gorm:"column:starts_at"`
	ExpiresAt      *time.Time `gorm:"column:expires_at"`
}

func (offerRow) TableName() string { return "commercial_offers" }

type quoteRow struct {
	QuoteID          string    `gorm:"column:quote_id;primaryKey;size:128"`
	OrganizationID   string    `gorm:"column:organization_id;size:128;not null;index"`
	OfferID          string    `gorm:"column:offer_id;size:128;not null"`
	ProductKind      string    `gorm:"column:product_kind;size:64;not null"`
	ResourceType     string    `gorm:"column:resource_type;size:64;not null"`
	ResourceQuantity int64     `gorm:"column:resource_quantity;not null"`
	Currency         string    `gorm:"column:currency;size:3;not null"`
	TotalMinor       int64     `gorm:"column:total_minor;not null"`
	PricingVersion   string    `gorm:"column:pricing_version;size:96;not null"`
	ExpiresAt        time.Time `gorm:"column:expires_at;not null;index"`
	Fingerprint      string    `gorm:"column:fingerprint;size:64;not null"`
	CreatedAt        time.Time `gorm:"column:created_at;not null"`
}

func (quoteRow) TableName() string { return "commercial_quotes" }

type orderRow struct {
	OrderID             string    `gorm:"column:order_id;primaryKey;size:128"`
	OrganizationID      string    `gorm:"column:organization_id;size:128;not null;index"`
	Kind                string    `gorm:"column:kind;size:32;not null"`
	QuoteID             string    `gorm:"column:quote_id;size:128"`
	Currency            string    `gorm:"column:currency;size:3;not null"`
	AmountMinor         int64     `gorm:"column:amount_minor;not null"`
	Status              string    `gorm:"column:status;size:32;not null;index"`
	WalletReservationID string    `gorm:"column:wallet_reservation_id;size:128"`
	PaymentID           string    `gorm:"column:payment_id;size:128"`
	IdempotencyKey      string    `gorm:"column:idempotency_key;size:192;not null"`
	RequestFingerprint  string    `gorm:"column:request_fingerprint;size:64;not null"`
	Version             int64     `gorm:"column:version;not null;default:1"`
	CreatedAt           time.Time `gorm:"column:created_at;not null"`
	UpdatedAt           time.Time `gorm:"column:updated_at;not null"`
}

func (orderRow) TableName() string { return "commercial_orders" }

type orderItemRow struct {
	OrderItemID      string `gorm:"column:order_item_id;primaryKey;size:128"`
	OrderID          string `gorm:"column:order_id;size:128;not null;index"`
	ProductKind      string `gorm:"column:product_kind;size:64;not null"`
	ResourceType     string `gorm:"column:resource_type;size:64;not null"`
	ResourceQuantity int64  `gorm:"column:resource_quantity;not null"`
	AmountMinor      int64  `gorm:"column:amount_minor;not null"`
}

func (orderItemRow) TableName() string { return "commercial_order_items" }

type Repository struct {
	db  *gorm.DB
	now func() time.Time
}

func New(db *gorm.DB) (*Repository, error) {
	if db == nil {
		return nil, billing.ErrFeatureUnavailable
	}
	return &Repository{db: db, now: func() time.Time { return time.Now().UTC() }}, nil
}

func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return billing.ErrFeatureUnavailable
	}
	return db.AutoMigrate(&offerRow{}, &quoteRow{}, &orderRow{}, &orderItemRow{})
}

func (r *Repository) SaveOffer(ctx context.Context, offer billing.Offer) error {
	if r == nil || r.db == nil || offer.Validate() != nil || offer.UnitPriceMinor <= 0 {
		return billing.ErrInvalid
	}
	row := offerToRow(offer)
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *Repository) ReadOffer(ctx context.Context, offerID string) (billing.Offer, error) {
	if r == nil || r.db == nil || strings.TrimSpace(offerID) == "" {
		return billing.Offer{}, billing.ErrInvalid
	}
	var row offerRow
	if err := r.db.WithContext(ctx).Where("offer_id = ?", strings.TrimSpace(offerID)).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return billing.Offer{}, billing.ErrOfferUnavailable
		}
		return billing.Offer{}, billing.ErrFeatureUnavailable
	}
	offer := offerFromRow(row)
	if offer.Validate() != nil || offer.Status != billing.OfferActive || offer.UnitPriceMinor <= 0 {
		return billing.Offer{}, billing.ErrOfferUnavailable
	}
	now := r.now()
	if offer.StartsAt != nil && now.Before(*offer.StartsAt) || offer.ExpiresAt != nil && !now.Before(*offer.ExpiresAt) {
		return billing.Offer{}, billing.ErrOfferUnavailable
	}
	return offer, nil
}

func (r *Repository) CreateQuote(ctx context.Context, request billing.QuoteRequest) (billing.Quote, error) {
	if r == nil || r.db == nil || strings.TrimSpace(request.OrganizationID) == "" || request.Quantity <= 0 {
		return billing.Quote{}, billing.ErrInvalid
	}
	offer, err := r.ReadOffer(ctx, request.OfferID)
	if err != nil {
		return billing.Quote{}, err
	}
	if request.Quantity < offer.MinQuantity || request.Quantity > offer.MaxQuantity {
		return billing.Quote{}, billing.ErrInvalid
	}
	if offer.UnitPriceMinor > math.MaxInt64/request.Quantity {
		return billing.Quote{}, billing.ErrInvalid
	}
	now := r.now().UTC()
	quote := billing.Quote{QuoteID: uuid.NewString(), OrganizationID: strings.TrimSpace(request.OrganizationID), OfferID: offer.OfferID, ProductKind: offer.ProductKind, ResourceType: offer.ResourceType, ResourceQuantity: request.Quantity, Currency: offer.Currency, TotalMinor: offer.UnitPriceMinor * request.Quantity, PricingVersion: offer.PricingVersion, CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute)}
	quote.Fingerprint = fingerprint(struct {
		OrganizationID string                   `json:"organization_id"`
		OfferID        string                   `json:"offer_id"`
		ProductKind    billing.ProductKind      `json:"product_kind"`
		ResourceType   orgresource.ResourceType `json:"resource_type"`
		Quantity       int64                    `json:"quantity"`
		Currency       string                   `json:"currency"`
		TotalMinor     int64                    `json:"total_minor"`
		PricingVersion string                   `json:"pricing_version"`
	}{quote.OrganizationID, quote.OfferID, quote.ProductKind, quote.ResourceType, quote.ResourceQuantity, quote.Currency, quote.TotalMinor, quote.PricingVersion})
	if err := quote.Validate(); err != nil {
		return billing.Quote{}, err
	}
	row := quoteToRow(quote)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return billing.Quote{}, billing.ErrFeatureUnavailable
	}
	return quote, nil
}

func (r *Repository) ReadQuote(ctx context.Context, organizationID, quoteID string) (billing.Quote, error) {
	if r == nil || r.db == nil || strings.TrimSpace(organizationID) == "" || strings.TrimSpace(quoteID) == "" {
		return billing.Quote{}, billing.ErrInvalid
	}
	var row quoteRow
	if err := r.db.WithContext(ctx).Where("organization_id = ? AND quote_id = ?", strings.TrimSpace(organizationID), strings.TrimSpace(quoteID)).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return billing.Quote{}, billing.ErrQuoteExpired
		}
		return billing.Quote{}, billing.ErrFeatureUnavailable
	}
	quote := quoteFromRow(row)
	if quote.Validate() != nil {
		return billing.Quote{}, billing.ErrFeatureUnavailable
	}
	if !r.now().Before(quote.ExpiresAt) {
		return billing.Quote{}, billing.ErrQuoteExpired
	}
	return quote, nil
}

func (r *Repository) CreatePendingResourceOrder(ctx context.Context, request billing.CreateResourceOrderRequest, quote billing.Quote) (billing.Order, error) {
	if r == nil || r.db == nil || quote.Validate() != nil || quote.OrganizationID != request.OrganizationID || strings.TrimSpace(request.IdempotencyKey) == "" {
		return billing.Order{}, billing.ErrInvalid
	}
	fingerprint := fingerprint(struct {
		OrganizationID   string `json:"organization_id"`
		QuoteID          string `json:"quote_id"`
		QuoteFingerprint string `json:"quote_fingerprint"`
		Quantity         int64  `json:"quantity"`
		TotalMinor       int64  `json:"total_minor"`
	}{request.OrganizationID, request.QuoteID, quote.Fingerprint, quote.ResourceQuantity, quote.TotalMinor})
	var out billing.Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing orderRow
		if err := tx.Where("organization_id = ? AND idempotency_key = ?", request.OrganizationID, request.IdempotencyKey).Take(&existing).Error; err == nil {
			if existing.RequestFingerprint != fingerprint {
				return billing.ErrConflict
			}
			var item orderItemRow
			if err := tx.Where("order_id = ?", existing.OrderID).Take(&item).Error; err != nil {
				return billing.ErrFeatureUnavailable
			}
			out = orderFromRows(existing, item)
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return billing.ErrFeatureUnavailable
		}
		now := r.now().UTC()
		row := orderRow{OrderID: uuid.NewString(), OrganizationID: request.OrganizationID, Kind: string(billing.OrderResourcePurchase), QuoteID: quote.QuoteID, Currency: quote.Currency, AmountMinor: quote.TotalMinor, Status: string(billing.OrderPending), IdempotencyKey: request.IdempotencyKey, RequestFingerprint: fingerprint, Version: 1, CreatedAt: now, UpdatedAt: now}
		item := orderItemRow{OrderItemID: uuid.NewString(), OrderID: row.OrderID, ProductKind: string(quote.ProductKind), ResourceType: string(quote.ResourceType), ResourceQuantity: quote.ResourceQuantity, AmountMinor: quote.TotalMinor}
		if err := tx.Create(&row).Error; err != nil {
			return billing.ErrFeatureUnavailable
		}
		if err := tx.Create(&item).Error; err != nil {
			return billing.ErrFeatureUnavailable
		}
		out = orderFromRows(row, item)
		return nil
	})
	return out, err
}

func (r *Repository) UpdateOrder(ctx context.Context, order billing.Order) error {
	if r == nil || r.db == nil || order.Validate() != nil {
		return billing.ErrInvalid
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row orderRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND order_id = ?", order.OrganizationID, order.OrderID).Take(&row).Error; err != nil {
			return billing.ErrFeatureUnavailable
		}
		row.Status, row.WalletReservationID, row.PaymentID, row.Version, row.UpdatedAt = string(order.Status), order.WalletReservationID, order.PaymentID, row.Version+1, order.UpdatedAt.UTC()
		if err := tx.Save(&row).Error; err != nil {
			return billing.ErrFeatureUnavailable
		}
		return nil
	})
}

func (r *Repository) ReadOrder(ctx context.Context, organizationID, orderID string) (billing.Order, error) {
	if r == nil || r.db == nil || strings.TrimSpace(organizationID) == "" || strings.TrimSpace(orderID) == "" {
		return billing.Order{}, billing.ErrInvalid
	}
	var row orderRow
	if err := r.db.WithContext(ctx).Where("organization_id = ? AND order_id = ?", organizationID, orderID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return billing.Order{}, billing.ErrConflict
		}
		return billing.Order{}, billing.ErrFeatureUnavailable
	}
	var item orderItemRow
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Take(&item).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return billing.Order{}, billing.ErrFeatureUnavailable
	}
	return orderFromRows(row, item), nil
}

func (r *Repository) ListOrders(ctx context.Context, organizationID string, filter billing.OrderFilter) (billing.OrderPage, error) {
	if r == nil || r.db == nil || strings.TrimSpace(organizationID) == "" || filter.Limit < 0 || filter.Limit > 100 {
		return billing.OrderPage{}, billing.ErrInvalid
	}
	limit := filter.Limit
	if limit == 0 {
		limit = 50
	}
	query := r.db.WithContext(ctx).Where("organization_id = ?", organizationID).Order("created_at DESC, order_id DESC").Limit(limit + 1)
	if filter.Kind != nil {
		query = query.Where("kind = ?", string(*filter.Kind))
	}
	if filter.Status != nil {
		query = query.Where("status = ?", string(*filter.Status))
	}
	var rows []orderRow
	if err := query.Find(&rows).Error; err != nil {
		return billing.OrderPage{}, billing.ErrFeatureUnavailable
	}
	page := billing.OrderPage{Items: make([]billing.Order, 0, minInt(len(rows), limit))}
	if len(rows) > limit {
		page.NextCursor = rows[limit-1].OrderID
		rows = rows[:limit]
	}
	for _, row := range rows {
		var item orderItemRow
		_ = r.db.WithContext(ctx).Where("order_id = ?", row.OrderID).Take(&item).Error
		page.Items = append(page.Items, orderFromRows(row, item))
	}
	return page, nil
}

func (r *Repository) ReadOrderSummary(ctx context.Context, organizationID string, from, until time.Time) (billing.OrderSummary, error) {
	page, err := r.ListOrders(ctx, organizationID, billing.OrderFilter{Limit: 100})
	if err != nil {
		return billing.OrderSummary{}, err
	}
	summary := billing.OrderSummary{Currency: billing.CurrencyCNY, From: from.UTC(), Until: until.UTC(), ObservedAt: r.now().UTC()}
	for _, order := range page.Items {
		if order.Status != billing.OrderFulfilled || order.CreatedAt.Before(from) || !order.CreatedAt.Before(until) {
			continue
		}
		summary.SpendMinor += order.AmountMinor
		if len(order.Items) == 1 {
			switch order.Items[0].ProductKind {
			case billing.ProductStoreRenewalPeriod:
				summary.StoreRenewalSpendMinor += order.AmountMinor
			case billing.ProductAIPoint:
				summary.AIPointSpendMinor += order.AmountMinor
			case billing.ProductDataRow:
				summary.DataRowSpendMinor += order.AmountMinor
			default:
				summary.OtherSpendMinor += order.AmountMinor
			}
		}
	}
	return summary, nil
}

func offerToRow(value billing.Offer) offerRow {
	return offerRow{OfferID: value.OfferID, ProductKind: string(value.ProductKind), ResourceType: string(value.ResourceType), Currency: value.Currency, UnitPriceMinor: value.UnitPriceMinor, PricingVersion: value.PricingVersion, MinQuantity: value.MinQuantity, MaxQuantity: value.MaxQuantity, Status: string(value.Status), StartsAt: value.StartsAt, ExpiresAt: value.ExpiresAt}
}
func offerFromRow(row offerRow) billing.Offer {
	return billing.Offer{OfferID: row.OfferID, ProductKind: billing.ProductKind(row.ProductKind), ResourceType: orgresource.ResourceType(row.ResourceType), Currency: row.Currency, UnitPriceMinor: row.UnitPriceMinor, PricingVersion: row.PricingVersion, MinQuantity: row.MinQuantity, MaxQuantity: row.MaxQuantity, Status: billing.OfferStatus(row.Status), StartsAt: row.StartsAt, ExpiresAt: row.ExpiresAt}
}
func quoteToRow(value billing.Quote) quoteRow {
	return quoteRow{QuoteID: value.QuoteID, OrganizationID: value.OrganizationID, OfferID: value.OfferID, ProductKind: string(value.ProductKind), ResourceType: string(value.ResourceType), ResourceQuantity: value.ResourceQuantity, Currency: value.Currency, TotalMinor: value.TotalMinor, PricingVersion: value.PricingVersion, ExpiresAt: value.ExpiresAt, Fingerprint: value.Fingerprint, CreatedAt: value.CreatedAt}
}
func quoteFromRow(row quoteRow) billing.Quote {
	return billing.Quote{QuoteID: row.QuoteID, OrganizationID: row.OrganizationID, OfferID: row.OfferID, ProductKind: billing.ProductKind(row.ProductKind), ResourceType: orgresource.ResourceType(row.ResourceType), ResourceQuantity: row.ResourceQuantity, Currency: row.Currency, TotalMinor: row.TotalMinor, PricingVersion: row.PricingVersion, ExpiresAt: row.ExpiresAt, Fingerprint: row.Fingerprint, CreatedAt: row.CreatedAt}
}
func orderFromRows(row orderRow, item orderItemRow) billing.Order {
	items := []billing.OrderItem(nil)
	if item.OrderItemID != "" {
		items = []billing.OrderItem{{OrderItemID: item.OrderItemID, ProductKind: billing.ProductKind(item.ProductKind), ResourceType: orgresource.ResourceType(item.ResourceType), ResourceQuantity: item.ResourceQuantity, AmountMinor: item.AmountMinor}}
	}
	return billing.Order{OrderID: row.OrderID, OrganizationID: row.OrganizationID, Kind: billing.OrderKind(row.Kind), QuoteID: row.QuoteID, Currency: row.Currency, AmountMinor: row.AmountMinor, Status: billing.OrderStatus(row.Status), WalletReservationID: row.WalletReservationID, PaymentID: row.PaymentID, Items: items, IdempotencyKey: row.IdempotencyKey, RequestFingerprint: row.RequestFingerprint, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}
func fingerprint(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
