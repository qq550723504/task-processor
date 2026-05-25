package listingadmin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrProductDataNotFound = errors.New("product data not found")

type ProductData struct {
	ID                int64           `json:"id"`
	TenantID          int64           `json:"tenantId"`
	Source            string          `json:"source,omitempty"`
	ImportTaskID      *int64          `json:"importTaskId,omitempty"`
	RawJSONDataID     *int64          `json:"rawJsonDataId,omitempty"`
	StoreID           *int64          `json:"storeId,omitempty"`
	CategoryID        *int64          `json:"categoryId,omitempty"`
	Platform          string          `json:"platform,omitempty"`
	Region            string          `json:"region,omitempty"`
	ParentProductID   string          `json:"parentProductId,omitempty"`
	ProductID         string          `json:"productId"`
	Title             string          `json:"title,omitempty"`
	Description       string          `json:"description,omitempty"`
	OriginalPrice     float64         `json:"originalPrice,omitempty"`
	SpecialPrice      float64         `json:"specialPrice,omitempty"`
	PriceCurrency     string          `json:"priceCurrency,omitempty"`
	Stock             string          `json:"stock,omitempty"`
	Brand             string          `json:"brand,omitempty"`
	Category          string          `json:"category,omitempty"`
	MainImageURL      string          `json:"mainImageUrl,omitempty"`
	ImageURLs         json.RawMessage `json:"imageUrls,omitempty"`
	Attributes        json.RawMessage `json:"attributes,omitempty"`
	SourceURL         string          `json:"sourceUrl,omitempty"`
	Status            int16           `json:"status"`
	PlatformProductID string          `json:"platformProductId,omitempty"`
	PlatformStatus    string          `json:"platformStatus,omitempty"`
	ShelfStatus       *int            `json:"shelfStatus,omitempty"`
	PublishTime       *time.Time      `json:"publishTime,omitempty"`
	ShelfTime         *time.Time      `json:"shelfTime,omitempty"`
	LastSyncTime      *time.Time      `json:"lastSyncTime,omitempty"`
	PlatformData      json.RawMessage `json:"platformData,omitempty"`
	CreateTime        *time.Time      `json:"createTime,omitempty"`
	UpdateTime        *time.Time      `json:"updateTime,omitempty"`
}

type ProductDataQuery struct {
	TenantID          int64
	OwnerUserID       string
	Page              int
	PageSize          int
	StoreID           *int64
	CategoryID        *int64
	Platform          string
	Region            string
	ProductID         string
	ParentProductID   string
	Title             string
	Brand             string
	Status            *int16
	PlatformProductID string
	ShelfStatus       *int
}

type ProductDataPage struct {
	Items    []ProductData `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

type ProductDataRepository interface {
	ListProductData(ctx context.Context, query ProductDataQuery) (*ProductDataPage, error)
	GetProductData(ctx context.Context, tenantID, id int64) (*ProductData, error)
	CreateProductData(ctx context.Context, product *ProductData) (*ProductData, error)
	UpdateProductData(ctx context.Context, product *ProductData) (*ProductData, error)
	UpdateProductDataStatus(ctx context.Context, tenantID, id int64, status int16) (*ProductData, error)
	DeleteProductData(ctx context.Context, tenantID, id int64) error
}

type listingProductData struct {
	ID                int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID          int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID       string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	Source            string     `gorm:"column:source"`
	ImportTaskID      int64      `gorm:"column:import_task_id;index"`
	RawJSONDataID     int64      `gorm:"column:raw_json_data_id;index"`
	StoreID           int64      `gorm:"column:store_id;index"`
	CategoryID        int64      `gorm:"column:category_id;index"`
	Platform          string     `gorm:"column:platform;index"`
	Region            string     `gorm:"column:region;index"`
	ParentProductID   string     `gorm:"column:parent_product_id;index"`
	ProductID         string     `gorm:"column:product_id;index"`
	Title             string     `gorm:"column:title"`
	Description       string     `gorm:"column:description"`
	OriginalPrice     float64    `gorm:"column:original_price"`
	SpecialPrice      float64    `gorm:"column:special_price"`
	PriceCurrency     string     `gorm:"column:price_currency"`
	Stock             string     `gorm:"column:stock"`
	Brand             string     `gorm:"column:brand"`
	Category          string     `gorm:"column:category"`
	MainImageURL      string     `gorm:"column:main_image_url"`
	ImageURLs         string     `gorm:"column:image_urls"`
	Attributes        string     `gorm:"column:attributes"`
	SourceURL         string     `gorm:"column:source_url"`
	Status            int16      `gorm:"column:status;not null;default:0;index"`
	PlatformProductID string     `gorm:"column:platform_product_id;index"`
	PlatformStatus    string     `gorm:"column:platform_status"`
	ShelfStatus       int        `gorm:"column:shelf_status;index"`
	PublishTime       *time.Time `gorm:"column:publish_time"`
	ShelfTime         *time.Time `gorm:"column:shelf_time"`
	LastSyncTime      *time.Time `gorm:"column:last_sync_time"`
	PlatformData      string     `gorm:"column:platform_data"`
	Creator           string     `gorm:"column:creator"`
	CreatedBy         string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime        *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater           string     `gorm:"column:updater"`
	UpdatedBy         string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime        *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted           int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingProductData) TableName() string {
	return "listing_product_data"
}

func (r listingProductData) toProductData() ProductData {
	return ProductData{
		ID:                r.ID,
		TenantID:          r.TenantID,
		Source:            r.Source,
		ImportTaskID:      int64PtrIfPositive(r.ImportTaskID),
		RawJSONDataID:     int64PtrIfPositive(r.RawJSONDataID),
		StoreID:           int64PtrIfPositive(r.StoreID),
		CategoryID:        int64PtrIfPositive(r.CategoryID),
		Platform:          r.Platform,
		Region:            r.Region,
		ParentProductID:   r.ParentProductID,
		ProductID:         r.ProductID,
		Title:             r.Title,
		Description:       r.Description,
		OriginalPrice:     r.OriginalPrice,
		SpecialPrice:      r.SpecialPrice,
		PriceCurrency:     r.PriceCurrency,
		Stock:             r.Stock,
		Brand:             r.Brand,
		Category:          r.Category,
		MainImageURL:      r.MainImageURL,
		ImageURLs:         rawJSONOrString(r.ImageURLs),
		Attributes:        rawJSONOrString(r.Attributes),
		SourceURL:         r.SourceURL,
		Status:            r.Status,
		PlatformProductID: r.PlatformProductID,
		PlatformStatus:    r.PlatformStatus,
		ShelfStatus:       intPtrIfPositive(r.ShelfStatus),
		PublishTime:       r.PublishTime,
		ShelfTime:         r.ShelfTime,
		LastSyncTime:      r.LastSyncTime,
		PlatformData:      rawJSONOrString(r.PlatformData),
		CreateTime:        r.CreateTime,
		UpdateTime:        r.UpdateTime,
	}
}

func listingProductDataFromProductData(product *ProductData) listingProductData {
	if product == nil {
		return listingProductData{}
	}
	return listingProductData{
		ID:                product.ID,
		TenantID:          product.TenantID,
		Source:            strings.TrimSpace(product.Source),
		ImportTaskID:      int64Value(product.ImportTaskID),
		RawJSONDataID:     int64Value(product.RawJSONDataID),
		StoreID:           int64Value(product.StoreID),
		CategoryID:        int64Value(product.CategoryID),
		Platform:          strings.TrimSpace(product.Platform),
		Region:            strings.TrimSpace(product.Region),
		ParentProductID:   strings.TrimSpace(product.ParentProductID),
		ProductID:         strings.TrimSpace(product.ProductID),
		Title:             strings.TrimSpace(product.Title),
		Description:       strings.TrimSpace(product.Description),
		OriginalPrice:     product.OriginalPrice,
		SpecialPrice:      product.SpecialPrice,
		PriceCurrency:     strings.TrimSpace(product.PriceCurrency),
		Stock:             strings.TrimSpace(product.Stock),
		Brand:             strings.TrimSpace(product.Brand),
		Category:          strings.TrimSpace(product.Category),
		MainImageURL:      strings.TrimSpace(product.MainImageURL),
		ImageURLs:         string(product.ImageURLs),
		Attributes:        string(product.Attributes),
		SourceURL:         strings.TrimSpace(product.SourceURL),
		Status:            product.Status,
		PlatformProductID: strings.TrimSpace(product.PlatformProductID),
		PlatformStatus:    strings.TrimSpace(product.PlatformStatus),
		ShelfStatus:       intValue(product.ShelfStatus),
		PublishTime:       product.PublishTime,
		ShelfTime:         product.ShelfTime,
		LastSyncTime:      product.LastSyncTime,
		PlatformData:      string(product.PlatformData),
	}
}

type GormProductDataRepository struct{ db *gorm.DB }

func NewGormProductDataRepository(db *gorm.DB) *GormProductDataRepository {
	return &GormProductDataRepository{db: db}
}

func AutoMigrateProductDataRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	return ensureOwnerAuditColumns(db, (listingProductData{}).TableName())
}

func (r *GormProductDataRepository) ListProductData(ctx context.Context, query ProductDataQuery) (*ProductDataPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("product data repository database is not configured")
	}
	db := applyProductDataQuery(r.db.WithContext(ctx).Table("listing_product_data"), query)
	var rows []listingProductData
	total, page, pageSize, err := findPagedRows(db, query.Page, query.PageSize, &rows)
	if err != nil {
		return nil, err
	}
	items := make([]ProductData, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toProductData())
	}
	return &ProductDataPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormProductDataRepository) GetProductData(ctx context.Context, tenantID, id int64) (*ProductData, error) {
	var row listingProductData
	err := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_product_data").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProductDataNotFound
	}
	if err != nil {
		return nil, err
	}
	product := row.toProductData()
	return &product, nil
}

func (r *GormProductDataRepository) CreateProductData(ctx context.Context, product *ProductData) (*ProductData, error) {
	row := listingProductDataFromProductData(product)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		row.OwnerUserID = ownerUserID
		row.Creator = ownerUserID
		row.CreatedBy = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
	}
	if err := r.db.WithContext(ctx).Table("listing_product_data").Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toProductData()
	return &created, nil
}

func (r *GormProductDataRepository) UpdateProductData(ctx context.Context, product *ProductData) (*ProductData, error) {
	row := listingProductDataFromProductData(product)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		row.OwnerUserID = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
	}
	updates := map[string]any{
		"owner_user_id":       row.OwnerUserID,
		"source":              row.Source,
		"import_task_id":      row.ImportTaskID,
		"raw_json_data_id":    row.RawJSONDataID,
		"store_id":            row.StoreID,
		"category_id":         row.CategoryID,
		"platform":            row.Platform,
		"region":              row.Region,
		"parent_product_id":   row.ParentProductID,
		"product_id":          row.ProductID,
		"title":               row.Title,
		"description":         row.Description,
		"original_price":      row.OriginalPrice,
		"special_price":       row.SpecialPrice,
		"price_currency":      row.PriceCurrency,
		"stock":               row.Stock,
		"brand":               row.Brand,
		"category":            row.Category,
		"main_image_url":      row.MainImageURL,
		"image_urls":          row.ImageURLs,
		"attributes":          row.Attributes,
		"source_url":          row.SourceURL,
		"status":              row.Status,
		"platform_product_id": row.PlatformProductID,
		"platform_status":     row.PlatformStatus,
		"shelf_status":        row.ShelfStatus,
		"publish_time":        row.PublishTime,
		"shelf_time":          row.ShelfTime,
		"last_sync_time":      row.LastSyncTime,
		"platform_data":       row.PlatformData,
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_product_data").Where("tenant_id = ? AND id = ? AND deleted = 0", row.TenantID, row.ID),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrProductDataNotFound
	}
	return r.GetProductData(ctx, row.TenantID, row.ID)
}

func (r *GormProductDataRepository) UpdateProductDataStatus(ctx context.Context, tenantID, id int64, status int16) (*ProductData, error) {
	updates := map[string]any{"status": status}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_product_data").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrProductDataNotFound
	}
	return r.GetProductData(ctx, tenantID, id)
}

func (r *GormProductDataRepository) DeleteProductData(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_product_data").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrProductDataNotFound
	}
	return nil
}

func applyProductDataQuery(db *gorm.DB, query ProductDataQuery) *gorm.DB {
	db = applyOwnedTenantQuery(db, query.TenantID, strings.TrimSpace(query.OwnerUserID))
	if query.StoreID != nil {
		db = db.Where("store_id = ?", *query.StoreID)
	}
	if query.CategoryID != nil {
		db = db.Where("category_id = ?", *query.CategoryID)
	}
	if query.Platform != "" {
		db = db.Where("platform = ?", query.Platform)
	}
	if query.Region != "" {
		db = db.Where("region = ?", query.Region)
	}
	if query.ProductID != "" {
		db = db.Where("product_id = ?", query.ProductID)
	}
	if query.ParentProductID != "" {
		db = db.Where("parent_product_id = ?", query.ParentProductID)
	}
	if query.Title != "" {
		db = db.Where("title LIKE ?", "%"+query.Title+"%")
	}
	if query.Brand != "" {
		db = db.Where("brand = ?", query.Brand)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if query.PlatformProductID != "" {
		db = db.Where("platform_product_id = ?", query.PlatformProductID)
	}
	if query.ShelfStatus != nil {
		db = db.Where("shelf_status = ?", *query.ShelfStatus)
	}
	return db
}

func rawJSONOrString(value string) json.RawMessage {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if json.Valid([]byte(trimmed)) {
		return json.RawMessage(trimmed)
	}
	encoded, _ := json.Marshal(trimmed)
	return encoded
}
