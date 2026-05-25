package listingadmin

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

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
	rows, total, page, pageSize, err := findProductDataRows(ctx, r.db.WithContext(ctx).Table("listing_product_data"), query)
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
	err := takeOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_product_data"), tenantID, id, "owner_user_id", &row, ErrProductDataNotFound)
	if err != nil {
		return nil, err
	}
	product := row.toProductData()
	return &product, nil
}

func (r *GormProductDataRepository) CreateProductData(ctx context.Context, product *ProductData) (*ProductData, error) {
	row := listingProductDataFromProductData(product)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		applyProductDataAuditFields(&row, ownerUserID, true)
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
		applyProductDataAuditFields(&row, ownerUserID, false)
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
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_product_data"), row.TenantID, row.ID, "owner_user_id", updates, ErrProductDataNotFound); err != nil {
		return nil, err
	}
	return r.GetProductData(ctx, row.TenantID, row.ID)
}

func (r *GormProductDataRepository) UpdateProductDataStatus(ctx context.Context, tenantID, id int64, status int16) (*ProductData, error) {
	updates := map[string]any{"status": status}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_product_data"), tenantID, id, "owner_user_id", updates, ErrProductDataNotFound); err != nil {
		return nil, err
	}
	return r.GetProductData(ctx, tenantID, id)
}

func (r *GormProductDataRepository) DeleteProductData(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	return updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_product_data"), tenantID, id, "owner_user_id", updates, ErrProductDataNotFound)
}
