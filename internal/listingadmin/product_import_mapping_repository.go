package listingadmin

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

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
	rows, total, page, pageSize, err := findProductImportMappingRows(ctx, r.db.WithContext(ctx).Table("listing_product_import_mapping"), query)
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
	err := takeOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_product_import_mapping"), tenantID, id, "owner_user_id", &row, ErrProductImportMappingNotFound)
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
		applyProductImportMappingAuditFields(&row, ownerUserID, true)
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
		applyProductImportMappingAuditFields(&row, ownerUserID, false)
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
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_product_import_mapping"), row.TenantID, row.ID, "owner_user_id", updates, ErrProductImportMappingNotFound); err != nil {
		return nil, err
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
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_product_import_mapping"), tenantID, id, "owner_user_id", updates, ErrProductImportMappingNotFound); err != nil {
		return nil, err
	}
	return r.GetProductImportMapping(ctx, tenantID, id)
}

func (r *GormProductImportMappingRepository) DeleteProductImportMapping(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	return updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_product_import_mapping"), tenantID, id, "owner_user_id", updates, ErrProductImportMappingNotFound)
}
