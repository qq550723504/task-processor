package listingadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrProductImportMappingNotFound = errors.New("product import mapping not found")

type ProductImportMapping struct {
	ID                      int64      `json:"id"`
	TenantID                int64      `json:"tenantId"`
	ImportTaskID            int64      `json:"importTaskId"`
	StoreID                 int64      `json:"storeId"`
	Platform                string     `json:"platform"`
	Region                  string     `json:"region"`
	ProductID               string     `json:"productId"`
	ParentProductID         string     `json:"parentProductId,omitempty"`
	SKU                     string     `json:"sku,omitempty"`
	CostPrice               *float64   `json:"costPrice,omitempty"`
	PlatformProductID       string     `json:"platformProductId,omitempty"`
	PlatformParentProductID string     `json:"platformParentProductId,omitempty"`
	FilterRuleID            *int64     `json:"filterRuleId,omitempty"`
	FilterRuleRange         string     `json:"filterRuleRange,omitempty"`
	ProfitRuleID            *int64     `json:"profitRuleId,omitempty"`
	SalePriceMultiplier     float64    `json:"salePriceMultiplier"`
	DiscountPriceMultiplier float64    `json:"discountPriceMultiplier"`
	Status                  int16      `json:"status"`
	Remark                  string     `json:"remark,omitempty"`
	CreateTime              *time.Time `json:"createTime,omitempty"`
	UpdateTime              *time.Time `json:"updateTime,omitempty"`
}

type ProductImportMappingQuery struct {
	TenantID                int64
	OwnerUserID             string
	Page                    int
	PageSize                int
	ImportTaskID            *int64
	StoreID                 *int64
	Platform                string
	Region                  string
	ProductID               string
	ParentProductID         string
	SKU                     string
	PlatformProductID       string
	PlatformParentProductID string
	Status                  *int16
}

type ProductImportMappingPage struct {
	Items    []ProductImportMapping `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

type ProductImportMappingRepository interface {
	ListProductImportMappings(ctx context.Context, query ProductImportMappingQuery) (*ProductImportMappingPage, error)
	GetProductImportMapping(ctx context.Context, tenantID, id int64) (*ProductImportMapping, error)
	CreateProductImportMapping(ctx context.Context, mapping *ProductImportMapping) (*ProductImportMapping, error)
	UpdateProductImportMapping(ctx context.Context, mapping *ProductImportMapping) (*ProductImportMapping, error)
	UpdateProductImportMappingStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*ProductImportMapping, error)
	DeleteProductImportMapping(ctx context.Context, tenantID, id int64) error
}

type listingProductImportMapping struct {
	ID                      int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID                int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID             string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	ImportTaskID            int64      `gorm:"column:import_task_id;not null;index"`
	StoreID                 int64      `gorm:"column:store_id;not null;index"`
	Platform                string     `gorm:"column:platform;not null;index"`
	Region                  string     `gorm:"column:region;not null;index"`
	ProductID               string     `gorm:"column:product_id;not null;index"`
	ParentProductID         string     `gorm:"column:parent_product_id;index"`
	SKU                     string     `gorm:"column:sku;index"`
	CostPrice               float64    `gorm:"column:cost_price"`
	PlatformProductID       string     `gorm:"column:platform_product_id;index"`
	PlatformParentProductID string     `gorm:"column:platform_parent_product_id;index"`
	FilterRuleID            int64      `gorm:"column:filter_rule_id;index"`
	FilterRuleRange         string     `gorm:"column:filter_rule_range"`
	ProfitRuleID            int64      `gorm:"column:profit_rule_id;index"`
	SalePriceMultiplier     float64    `gorm:"column:sale_price_multiplier;not null;default:1"`
	DiscountPriceMultiplier float64    `gorm:"column:discount_price_multiplier;not null;default:1"`
	Status                  int16      `gorm:"column:status;not null;default:0;index"`
	Remark                  string     `gorm:"column:remark"`
	Creator                 string     `gorm:"column:creator"`
	CreatedBy               string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime              *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater                 string     `gorm:"column:updater"`
	UpdatedBy               string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime              *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted                 int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingProductImportMapping) TableName() string {
	return "listing_product_import_mapping"
}

func (r listingProductImportMapping) toProductImportMapping() ProductImportMapping {
	return ProductImportMapping{
		ID:                      r.ID,
		TenantID:                r.TenantID,
		ImportTaskID:            r.ImportTaskID,
		StoreID:                 r.StoreID,
		Platform:                r.Platform,
		Region:                  r.Region,
		ProductID:               r.ProductID,
		ParentProductID:         r.ParentProductID,
		SKU:                     r.SKU,
		CostPrice:               floatPtrIfPositive(r.CostPrice),
		PlatformProductID:       r.PlatformProductID,
		PlatformParentProductID: r.PlatformParentProductID,
		FilterRuleID:            int64PtrIfPositive(r.FilterRuleID),
		FilterRuleRange:         r.FilterRuleRange,
		ProfitRuleID:            int64PtrIfPositive(r.ProfitRuleID),
		SalePriceMultiplier:     r.SalePriceMultiplier,
		DiscountPriceMultiplier: r.DiscountPriceMultiplier,
		Status:                  r.Status,
		Remark:                  r.Remark,
		CreateTime:              r.CreateTime,
		UpdateTime:              r.UpdateTime,
	}
}

func listingProductImportMappingFromProductImportMapping(mapping *ProductImportMapping) listingProductImportMapping {
	if mapping == nil {
		return listingProductImportMapping{}
	}
	return listingProductImportMapping{
		ID:                      mapping.ID,
		TenantID:                mapping.TenantID,
		ImportTaskID:            mapping.ImportTaskID,
		StoreID:                 mapping.StoreID,
		Platform:                strings.TrimSpace(mapping.Platform),
		Region:                  strings.TrimSpace(mapping.Region),
		ProductID:               strings.TrimSpace(mapping.ProductID),
		ParentProductID:         strings.TrimSpace(mapping.ParentProductID),
		SKU:                     strings.TrimSpace(mapping.SKU),
		CostPrice:               floatValue(mapping.CostPrice),
		PlatformProductID:       strings.TrimSpace(mapping.PlatformProductID),
		PlatformParentProductID: strings.TrimSpace(mapping.PlatformParentProductID),
		FilterRuleID:            int64Value(mapping.FilterRuleID),
		FilterRuleRange:         strings.TrimSpace(mapping.FilterRuleRange),
		ProfitRuleID:            int64Value(mapping.ProfitRuleID),
		SalePriceMultiplier:     mapping.SalePriceMultiplier,
		DiscountPriceMultiplier: mapping.DiscountPriceMultiplier,
		Status:                  mapping.Status,
		Remark:                  strings.TrimSpace(mapping.Remark),
	}
}

type GormProductImportMappingRepository struct{ db *gorm.DB }

func NewGormProductImportMappingRepository(db *gorm.DB) *GormProductImportMappingRepository {
	return &GormProductImportMappingRepository{db: db}
}

func AutoMigrateProductImportMappingRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	return ensureOwnerAuditColumns(db, (listingProductImportMapping{}).TableName())
}

func (r *GormProductImportMappingRepository) ListProductImportMappings(ctx context.Context, query ProductImportMappingQuery) (*ProductImportMappingPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("product import mapping repository database is not configured")
	}
	db := applyProductImportMappingQuery(r.db.WithContext(ctx).Table("listing_product_import_mapping"), query)
	var rows []listingProductImportMapping
	total, page, pageSize, err := findPagedRows(db, query.Page, query.PageSize, &rows)
	if err != nil {
		return nil, err
	}
	items := make([]ProductImportMapping, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toProductImportMapping())
	}
	return &ProductImportMappingPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormProductImportMappingRepository) GetProductImportMapping(ctx context.Context, tenantID, id int64) (*ProductImportMapping, error) {
	var row listingProductImportMapping
	err := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_product_import_mapping").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProductImportMappingNotFound
	}
	if err != nil {
		return nil, err
	}
	mapping := row.toProductImportMapping()
	return &mapping, nil
}

func (r *GormProductImportMappingRepository) CreateProductImportMapping(ctx context.Context, mapping *ProductImportMapping) (*ProductImportMapping, error) {
	row := listingProductImportMappingFromProductImportMapping(mapping)
	applyProductImportMappingDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		row.OwnerUserID = ownerUserID
		row.Creator = ownerUserID
		row.CreatedBy = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
	}
	if err := r.db.WithContext(ctx).Table("listing_product_import_mapping").Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toProductImportMapping()
	return &created, nil
}

func (r *GormProductImportMappingRepository) UpdateProductImportMapping(ctx context.Context, mapping *ProductImportMapping) (*ProductImportMapping, error) {
	row := listingProductImportMappingFromProductImportMapping(mapping)
	applyProductImportMappingDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		row.OwnerUserID = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
	}
	updates := map[string]any{
		"owner_user_id":              row.OwnerUserID,
		"import_task_id":             row.ImportTaskID,
		"store_id":                   row.StoreID,
		"platform":                   row.Platform,
		"region":                     row.Region,
		"product_id":                 row.ProductID,
		"parent_product_id":          row.ParentProductID,
		"sku":                        row.SKU,
		"cost_price":                 row.CostPrice,
		"platform_product_id":        row.PlatformProductID,
		"platform_parent_product_id": row.PlatformParentProductID,
		"filter_rule_id":             row.FilterRuleID,
		"filter_rule_range":          row.FilterRuleRange,
		"profit_rule_id":             row.ProfitRuleID,
		"sale_price_multiplier":      row.SalePriceMultiplier,
		"discount_price_multiplier":  row.DiscountPriceMultiplier,
		"status":                     row.Status,
		"remark":                     row.Remark,
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_product_import_mapping").Where("tenant_id = ? AND id = ? AND deleted = 0", row.TenantID, row.ID),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrProductImportMappingNotFound
	}
	return r.GetProductImportMapping(ctx, row.TenantID, row.ID)
}

func (r *GormProductImportMappingRepository) UpdateProductImportMappingStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*ProductImportMapping, error) {
	updates := map[string]any{"status": status}
	if strings.TrimSpace(remark) != "" {
		updates["remark"] = strings.TrimSpace(remark)
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_product_import_mapping").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrProductImportMappingNotFound
	}
	return r.GetProductImportMapping(ctx, tenantID, id)
}

func (r *GormProductImportMappingRepository) DeleteProductImportMapping(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_product_import_mapping").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrProductImportMappingNotFound
	}
	return nil
}

func applyProductImportMappingDefaults(row *listingProductImportMapping) {
	if row.SalePriceMultiplier == 0 {
		row.SalePriceMultiplier = 1
	}
	if row.DiscountPriceMultiplier == 0 {
		row.DiscountPriceMultiplier = 1
	}
}

func applyProductImportMappingQuery(db *gorm.DB, query ProductImportMappingQuery) *gorm.DB {
	db = db.Where("tenant_id = ? AND deleted = 0", query.TenantID)
	if ownerScopeEnabled() && strings.TrimSpace(query.OwnerUserID) != "" {
		db = db.Where("owner_user_id = ?", strings.TrimSpace(query.OwnerUserID))
	}
	if query.ImportTaskID != nil {
		db = db.Where("import_task_id = ?", *query.ImportTaskID)
	}
	if query.StoreID != nil {
		db = db.Where("store_id = ?", *query.StoreID)
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
	if query.SKU != "" {
		db = db.Where("sku = ?", query.SKU)
	}
	if query.PlatformProductID != "" {
		db = db.Where("platform_product_id = ?", query.PlatformProductID)
	}
	if query.PlatformParentProductID != "" {
		db = db.Where("platform_parent_product_id = ?", query.PlatformParentProductID)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}

func int64PtrIfPositive(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	out := value
	return &out
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
