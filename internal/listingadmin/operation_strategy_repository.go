package listingadmin

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

type GormOperationStrategyRepository struct{ db *gorm.DB }

func NewGormOperationStrategyRepository(db *gorm.DB) *GormOperationStrategyRepository {
	return &GormOperationStrategyRepository{db: db}
}

func AutoMigrateOperationStrategyRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	return ensureOwnerAuditColumns(db, (listingOperationStrategy{}).TableName())
}

func (r *GormOperationStrategyRepository) ListOperationStrategies(ctx context.Context, query OperationStrategyQuery) (*OperationStrategyPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("operation strategy repository database is not configured")
	}
	rows, total, page, pageSize, err := findOperationStrategyRows(ctx, r.db.WithContext(ctx).Table("listing_operation_strategy"), query)
	if err != nil {
		return nil, err
	}
	items := make([]OperationStrategy, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toOperationStrategy())
	}
	return &OperationStrategyPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormOperationStrategyRepository) GetOperationStrategy(ctx context.Context, tenantID, id int64) (*OperationStrategy, error) {
	var row listingOperationStrategy
	err := takeOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_operation_strategy"), tenantID, id, "owner_user_id", &row, ErrOperationStrategyNotFound)
	if err != nil {
		return nil, err
	}
	strategy := row.toOperationStrategy()
	return &strategy, nil
}

func (r *GormOperationStrategyRepository) CreateOperationStrategy(ctx context.Context, strategy *OperationStrategy) (*OperationStrategy, error) {
	row := listingOperationStrategyFromOperationStrategy(strategy)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		applyOperationStrategyAuditFields(&row, ownerUserID, true)
	}
	if err := r.db.WithContext(ctx).Table("listing_operation_strategy").Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toOperationStrategy()
	return &created, nil
}

func (r *GormOperationStrategyRepository) UpdateOperationStrategy(ctx context.Context, strategy *OperationStrategy) (*OperationStrategy, error) {
	row := listingOperationStrategyFromOperationStrategy(strategy)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		applyOperationStrategyAuditFields(&row, ownerUserID, false)
	}
	updates := map[string]any{
		"owner_user_id":           row.OwnerUserID,
		"store_id":                row.StoreID,
		"name":                    row.Name,
		"platform":                row.Platform,
		"status":                  row.Status,
		"stock_change_threshold":  row.StockChangeThreshold,
		"stock_change_action":     row.StockChangeAction,
		"out_of_stock_action":     row.OutOfStockAction,
		"min_profit_rate":         row.MinProfitRate,
		"low_profit_action":       row.LowProfitAction,
		"price_update_multiplier": row.PriceUpdateMultiplier,
		"fixed_price_adjustment":  row.FixedPriceAdjustment,
		"stock_update_ratio":      row.StockUpdateRatio,
		"remark":                  row.Remark,
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_operation_strategy"), row.TenantID, row.ID, "owner_user_id", updates, ErrOperationStrategyNotFound); err != nil {
		return nil, err
	}
	return r.GetOperationStrategy(ctx, row.TenantID, row.ID)
}

func (r *GormOperationStrategyRepository) UpdateOperationStrategyStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*OperationStrategy, error) {
	updates := map[string]any{"status": status}
	if strings.TrimSpace(remark) != "" {
		updates["remark"] = strings.TrimSpace(remark)
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	if err := updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_operation_strategy"), tenantID, id, "owner_user_id", updates, ErrOperationStrategyNotFound); err != nil {
		return nil, err
	}
	return r.GetOperationStrategy(ctx, tenantID, id)
}

func (r *GormOperationStrategyRepository) DeleteOperationStrategy(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	return updateOwnedTenantRow(ctx, r.db.WithContext(ctx).Table("listing_operation_strategy"), tenantID, id, "owner_user_id", updates, ErrOperationStrategyNotFound)
}
