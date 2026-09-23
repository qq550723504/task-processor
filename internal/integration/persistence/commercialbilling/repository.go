package commercialbilling

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
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
	"task-processor/internal/ledger/money"
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
	OrderID                     string    `gorm:"column:order_id;primaryKey;size:128"`
	OrganizationID              string    `gorm:"column:organization_id;size:128;not null;index;uniqueIndex:uq_commercial_orders_org_idempotency,priority:1"`
	Kind                        string    `gorm:"column:kind;size:32;not null"`
	QuoteID                     string    `gorm:"column:quote_id;size:128"`
	Description                 string    `gorm:"column:description;size:256;not null;default:''"`
	Currency                    string    `gorm:"column:currency;size:3;not null"`
	AmountMinor                 int64     `gorm:"column:amount_minor;not null"`
	Status                      string    `gorm:"column:status;size:32;not null;index"`
	FailureCode                 string    `gorm:"column:failure_code;size:32;not null;default:''"`
	WalletReservationID         string    `gorm:"column:wallet_reservation_id;size:128"`
	WalletReservationState      string    `gorm:"column:wallet_reservation_state;size:16;not null;default:''"`
	PaymentID                   string    `gorm:"column:payment_id;size:128"`
	ResourceGrantOperationID    string    `gorm:"column:resource_grant_operation_id;size:128"`
	ResourceGrantSourceType     string    `gorm:"column:resource_grant_source_type;size:64"`
	ResourceGrantSourceIdentity string    `gorm:"column:resource_grant_source_identity;size:192"`
	IdempotencyKey              string    `gorm:"column:idempotency_key;size:192;not null;uniqueIndex:uq_commercial_orders_org_idempotency,priority:2"`
	RequestFingerprint          string    `gorm:"column:request_fingerprint;size:64;not null"`
	Version                     int64     `gorm:"column:version;not null;default:1"`
	CreatedAt                   time.Time `gorm:"column:created_at;not null"`
	UpdatedAt                   time.Time `gorm:"column:updated_at;not null"`
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
		row := orderRow{OrderID: uuid.NewString(), OrganizationID: request.OrganizationID, Kind: string(billing.OrderResourcePurchase), QuoteID: quote.QuoteID, Description: billing.DescribeOrder(billing.OrderResourcePurchase, quote.ProductKind, quote.ResourceQuantity), Currency: quote.Currency, AmountMinor: quote.TotalMinor, Status: string(billing.OrderPending), IdempotencyKey: request.IdempotencyKey, RequestFingerprint: fingerprint, Version: 1, CreatedAt: now, UpdatedAt: now}
		item := orderItemRow{OrderItemID: uuid.NewString(), OrderID: row.OrderID, ProductKind: string(quote.ProductKind), ResourceType: string(quote.ResourceType), ResourceQuantity: quote.ResourceQuantity, AmountMinor: quote.TotalMinor}
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "organization_id"}, {Name: "idempotency_key"}}, DoNothing: true}).Create(&row)
		if result.Error != nil {
			return billing.ErrFeatureUnavailable
		}
		if result.RowsAffected == 0 {
			var existing orderRow
			if err := tx.Where("organization_id = ? AND idempotency_key = ?", request.OrganizationID, request.IdempotencyKey).Take(&existing).Error; err != nil {
				return billing.ErrFeatureUnavailable
			}
			if existing.RequestFingerprint != fingerprint {
				return billing.ErrConflict
			}
			var existingItem orderItemRow
			if err := tx.Where("order_id = ?", existing.OrderID).Take(&existingItem).Error; err != nil {
				return billing.ErrFeatureUnavailable
			}
			out = orderFromRows(existing, existingItem)
			return nil
		}
		if err := tx.Create(&item).Error; err != nil {
			return billing.ErrFeatureUnavailable
		}
		out = orderFromRows(row, item)
		return nil
	})
	return out, err
}

func (r *Repository) FindResourceOrderByIdempotency(ctx context.Context, organizationID, idempotencyKey string) (billing.Order, bool, error) {
	if r == nil || r.db == nil || strings.TrimSpace(organizationID) == "" || strings.TrimSpace(idempotencyKey) == "" {
		return billing.Order{}, false, billing.ErrInvalid
	}
	var row orderRow
	if err := r.db.WithContext(ctx).Where("organization_id = ? AND idempotency_key = ?", organizationID, idempotencyKey).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return billing.Order{}, false, nil
		}
		return billing.Order{}, false, billing.ErrFeatureUnavailable
	}
	if row.Kind != string(billing.OrderResourcePurchase) {
		return billing.Order{}, false, billing.ErrConflict
	}
	var item orderItemRow
	if err := r.db.WithContext(ctx).Where("order_id = ?", row.OrderID).Take(&item).Error; err != nil {
		return billing.Order{}, false, billing.ErrFeatureUnavailable
	}
	return orderFromRows(row, item), true, nil
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
		if row.Version != order.Version {
			return billing.ErrConflict
		}
		row.Status = string(order.Status)
		row.FailureCode = string(order.FailureCode)
		row.WalletReservationID = order.WalletReservationID
		row.WalletReservationState = string(order.WalletReservationState)
		row.PaymentID = order.PaymentID
		row.ResourceGrantOperationID = order.ResourceGrantOperationID
		row.ResourceGrantSourceType = order.ResourceGrantSourceType
		row.ResourceGrantSourceIdentity = order.ResourceGrantSourceIdentity
		row.Version++
		row.UpdatedAt = order.UpdatedAt.UTC()
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
			return billing.Order{}, billing.ErrNotFound
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
	if r == nil || r.db == nil || strings.TrimSpace(organizationID) == "" || filter.Limit < 0 || filter.Limit > billing.MaxOrderPageSize {
		return billing.OrderPage{}, billing.ErrInvalid
	}
	limit := filter.Limit
	if limit == 0 {
		limit = 50
	}
	query := r.db.WithContext(ctx).Where("commercial_orders.organization_id = ?", organizationID).Order("commercial_orders.created_at DESC, commercial_orders.order_id DESC").Limit(limit + 1)
	if filter.Query != "" {
		needle := "%" + strings.TrimSpace(filter.Query) + "%"
		query = query.Joins("LEFT JOIN commercial_order_items ON commercial_order_items.order_id = commercial_orders.order_id").Where("commercial_orders.order_id LIKE ? OR commercial_orders.quote_id LIKE ? OR commercial_orders.description LIKE ? OR commercial_order_items.product_kind LIKE ?", needle, needle, needle, needle)
	}
	if filter.Kind != nil {
		query = query.Where("commercial_orders.kind = ?", string(*filter.Kind))
	}
	if filter.ProductKind != nil {
		query = query.Joins("JOIN commercial_order_items AS filtered_items ON filtered_items.order_id = commercial_orders.order_id").Where("filtered_items.product_kind = ?", string(*filter.ProductKind))
	}
	if filter.Status != nil {
		query = query.Where("commercial_orders.status = ?", string(*filter.Status))
	}
	if filter.From != nil {
		query = query.Where("commercial_orders.created_at >= ?", filter.From.UTC())
	}
	if filter.Until != nil {
		query = query.Where("commercial_orders.created_at < ?", filter.Until.UTC())
	}
	if filter.Cursor != "" {
		createdAt, orderID, err := decodeOrderCursor(filter.Cursor)
		if err != nil {
			return billing.OrderPage{}, billing.ErrInvalid
		}
		query = query.Where("(commercial_orders.created_at < ?) OR (commercial_orders.created_at = ? AND commercial_orders.order_id < ?)", createdAt, createdAt, orderID)
	}
	var rows []orderRow
	if err := query.Find(&rows).Error; err != nil {
		return billing.OrderPage{}, billing.ErrFeatureUnavailable
	}
	page := billing.OrderPage{Items: make([]billing.Order, 0, minInt(len(rows), limit))}
	if len(rows) > limit {
		page.NextCursor = encodeOrderCursor(rows[limit-1].CreatedAt, rows[limit-1].OrderID)
		rows = rows[:limit]
	}
	for _, row := range rows {
		var item orderItemRow
		if err := r.db.WithContext(ctx).Where("order_id = ?", row.OrderID).Take(&item).Error; err != nil {
			return billing.OrderPage{}, billing.ErrFeatureUnavailable
		}
		page.Items = append(page.Items, orderFromRows(row, item))
	}
	return page, nil
}

func (r *Repository) ReadOrderSummary(ctx context.Context, organizationID string, from, until time.Time) (billing.OrderSummary, error) {
	if r == nil || r.db == nil || strings.TrimSpace(organizationID) == "" || !from.Before(until) {
		return billing.OrderSummary{}, billing.ErrInvalid
	}
	summary := billing.OrderSummary{Currency: billing.CurrencyCNY, From: from.UTC(), Until: until.UTC(), ObservedAt: r.now().UTC()}
	var rows []struct {
		ProductKind string
		SpendMinor  int64
	}
	// Fulfilled orders receive their terminal timestamp in updated_at when the
	// fulfillment/reconciliation path commits. Spend windows follow that event,
	// not when the order was originally created.
	if err := r.db.WithContext(ctx).Table("commercial_orders").Select("commercial_order_items.product_kind AS product_kind, SUM(commercial_orders.amount_minor) AS spend_minor").Joins("JOIN commercial_order_items ON commercial_order_items.order_id = commercial_orders.order_id").Where("commercial_orders.organization_id = ? AND commercial_orders.kind = ? AND commercial_orders.status = ? AND commercial_orders.updated_at >= ? AND commercial_orders.updated_at < ?", organizationID, string(billing.OrderResourcePurchase), string(billing.OrderFulfilled), from.UTC(), until.UTC()).Group("commercial_order_items.product_kind").Scan(&rows).Error; err != nil {
		return billing.OrderSummary{}, billing.ErrFeatureUnavailable
	}
	for _, row := range rows {
		summary.SpendMinor += row.SpendMinor
		switch billing.ProductKind(row.ProductKind) {
		case billing.ProductStoreRenewalPeriod:
			summary.StoreRenewalSpendMinor += row.SpendMinor
		case billing.ProductAIPoint:
			summary.AIPointSpendMinor += row.SpendMinor
		case billing.ProductDataRow:
			summary.DataRowSpendMinor += row.SpendMinor
		default:
			summary.OtherSpendMinor += row.SpendMinor
		}
	}
	return summary, nil
}

func encodeOrderCursor(createdAt time.Time, orderID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(createdAt.UTC().Format(time.RFC3339Nano) + "|" + orderID))
}

func decodeOrderCursor(cursor string) (time.Time, string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", billing.ErrInvalid
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return time.Time{}, "", billing.ErrInvalid
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", billing.ErrInvalid
	}
	return createdAt.UTC(), parts[1], nil
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
	return billing.Order{OrderID: row.OrderID, OrganizationID: row.OrganizationID, Kind: billing.OrderKind(row.Kind), Description: row.Description, QuoteID: row.QuoteID, Currency: row.Currency, AmountMinor: row.AmountMinor, Status: billing.OrderStatus(row.Status), FailureCode: billing.OrderFailureCode(row.FailureCode), WalletReservationID: row.WalletReservationID, WalletReservationState: money.WalletReservationState(row.WalletReservationState), PaymentID: row.PaymentID, ResourceGrantOperationID: row.ResourceGrantOperationID, ResourceGrantSourceType: row.ResourceGrantSourceType, ResourceGrantSourceIdentity: row.ResourceGrantSourceIdentity, Items: items, IdempotencyKey: row.IdempotencyKey, RequestFingerprint: row.RequestFingerprint, Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
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
