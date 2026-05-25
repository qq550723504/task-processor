package listingadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrOperationStrategyNotFound = errors.New("operation strategy not found")

type OperationStrategy struct {
	ID                    int64      `json:"id"`
	TenantID              int64      `json:"tenantId"`
	StoreID               int64      `json:"storeId"`
	Name                  string     `json:"name"`
	Platform              string     `json:"platform"`
	Status                int16      `json:"status"`
	StockChangeThreshold  *int       `json:"stockChangeThreshold,omitempty"`
	StockChangeAction     string     `json:"stockChangeAction,omitempty"`
	OutOfStockAction      string     `json:"outOfStockAction,omitempty"`
	MinProfitRate         *float64   `json:"minProfitRate,omitempty"`
	LowProfitAction       string     `json:"lowProfitAction,omitempty"`
	PriceUpdateMultiplier *float64   `json:"priceUpdateMultiplier,omitempty"`
	FixedPriceAdjustment  *float64   `json:"fixedPriceAdjustment,omitempty"`
	StockUpdateRatio      *float64   `json:"stockUpdateRatio,omitempty"`
	Remark                string     `json:"remark,omitempty"`
	CreateTime            *time.Time `json:"createTime,omitempty"`
	UpdateTime            *time.Time `json:"updateTime,omitempty"`
}

type OperationStrategyQuery struct {
	TenantID    int64
	OwnerUserID string
	Page        int
	PageSize    int
	Name        string
	StoreID     *int64
	Platform    string
	Status      *int16
}

type OperationStrategyPage struct {
	Items    []OperationStrategy `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

type OperationStrategyRepository interface {
	ListOperationStrategies(ctx context.Context, query OperationStrategyQuery) (*OperationStrategyPage, error)
	GetOperationStrategy(ctx context.Context, tenantID, id int64) (*OperationStrategy, error)
	CreateOperationStrategy(ctx context.Context, strategy *OperationStrategy) (*OperationStrategy, error)
	UpdateOperationStrategy(ctx context.Context, strategy *OperationStrategy) (*OperationStrategy, error)
	UpdateOperationStrategyStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*OperationStrategy, error)
	DeleteOperationStrategy(ctx context.Context, tenantID, id int64) error
}

type listingOperationStrategy struct {
	ID                    int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID              int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID           string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	StoreID               int64      `gorm:"column:store_id;not null;index"`
	Name                  string     `gorm:"column:name;not null"`
	Platform              string     `gorm:"column:platform;not null;index"`
	Status                int16      `gorm:"column:status;not null;default:0;index"`
	StockChangeThreshold  int        `gorm:"column:stock_change_threshold"`
	StockChangeAction     string     `gorm:"column:stock_change_action"`
	OutOfStockAction      string     `gorm:"column:out_of_stock_action"`
	MinProfitRate         float64    `gorm:"column:min_profit_rate"`
	LowProfitAction       string     `gorm:"column:low_profit_action"`
	PriceUpdateMultiplier float64    `gorm:"column:price_update_multiplier"`
	FixedPriceAdjustment  float64    `gorm:"column:fixed_price_adjustment"`
	StockUpdateRatio      float64    `gorm:"column:stock_update_ratio"`
	Remark                string     `gorm:"column:remark"`
	ActivityEnabled       int16      `gorm:"column:activity_enabled;not null;default:0"`
	ActivityType          string     `gorm:"column:activity_type"`
	ActivityDiscountRate  float64    `gorm:"column:activity_discount_rate"`
	ActivityStockRatio    float64    `gorm:"column:activity_stock_ratio"`
	PromotionRatio        float64    `gorm:"column:promotion_ratio"`
	ActivityMinProfitRate float64    `gorm:"column:activity_min_profit_rate"`
	ActivityPriceMode     string     `gorm:"column:activity_price_mode"`
	Creator               string     `gorm:"column:creator"`
	CreatedBy             string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime            *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater               string     `gorm:"column:updater"`
	UpdatedBy             string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime            *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted               int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingOperationStrategy) TableName() string { return "listing_operation_strategy" }

func (s listingOperationStrategy) toOperationStrategy() OperationStrategy {
	return OperationStrategy{
		ID:                    s.ID,
		TenantID:              s.TenantID,
		StoreID:               s.StoreID,
		Name:                  s.Name,
		Platform:              s.Platform,
		Status:                s.Status,
		StockChangeThreshold:  intPtrIfPositive(s.StockChangeThreshold),
		StockChangeAction:     s.StockChangeAction,
		OutOfStockAction:      s.OutOfStockAction,
		MinProfitRate:         floatPtrIfPositive(s.MinProfitRate),
		LowProfitAction:       s.LowProfitAction,
		PriceUpdateMultiplier: floatPtrIfPositive(s.PriceUpdateMultiplier),
		FixedPriceAdjustment:  floatPtrIfPositive(s.FixedPriceAdjustment),
		StockUpdateRatio:      floatPtrIfPositive(s.StockUpdateRatio),
		Remark:                s.Remark,
		CreateTime:            s.CreateTime,
		UpdateTime:            s.UpdateTime,
	}
}

func listingOperationStrategyFromOperationStrategy(strategy *OperationStrategy) listingOperationStrategy {
	if strategy == nil {
		return listingOperationStrategy{}
	}
	return listingOperationStrategy{
		ID:                    strategy.ID,
		TenantID:              strategy.TenantID,
		StoreID:               strategy.StoreID,
		Name:                  strings.TrimSpace(strategy.Name),
		Platform:              strings.TrimSpace(strategy.Platform),
		Status:                strategy.Status,
		StockChangeThreshold:  intValue(strategy.StockChangeThreshold),
		StockChangeAction:     strings.TrimSpace(strategy.StockChangeAction),
		OutOfStockAction:      strings.TrimSpace(strategy.OutOfStockAction),
		MinProfitRate:         floatValue(strategy.MinProfitRate),
		LowProfitAction:       strings.TrimSpace(strategy.LowProfitAction),
		PriceUpdateMultiplier: floatValue(strategy.PriceUpdateMultiplier),
		FixedPriceAdjustment:  floatValue(strategy.FixedPriceAdjustment),
		StockUpdateRatio:      floatValue(strategy.StockUpdateRatio),
		Remark:                strings.TrimSpace(strategy.Remark),
	}
}

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
	db := applyOperationStrategyQuery(r.db.WithContext(ctx).Table("listing_operation_strategy"), query)
	var rows []listingOperationStrategy
	total, page, pageSize, err := findPagedRows(db, query.Page, query.PageSize, &rows)
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
	err := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_operation_strategy").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOperationStrategyNotFound
	}
	if err != nil {
		return nil, err
	}
	strategy := row.toOperationStrategy()
	return &strategy, nil
}

func (r *GormOperationStrategyRepository) CreateOperationStrategy(ctx context.Context, strategy *OperationStrategy) (*OperationStrategy, error) {
	row := listingOperationStrategyFromOperationStrategy(strategy)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		row.OwnerUserID = ownerUserID
		row.Creator = ownerUserID
		row.CreatedBy = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
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
		row.OwnerUserID = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
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
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_operation_strategy").Where("tenant_id = ? AND id = ? AND deleted = 0", row.TenantID, row.ID),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrOperationStrategyNotFound
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
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_operation_strategy").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrOperationStrategyNotFound
	}
	return r.GetOperationStrategy(ctx, tenantID, id)
}

func (r *GormOperationStrategyRepository) DeleteOperationStrategy(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_operation_strategy").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrOperationStrategyNotFound
	}
	return nil
}

func applyOperationStrategyQuery(db *gorm.DB, query OperationStrategyQuery) *gorm.DB {
	db = applyOwnedTenantQuery(db, query.TenantID, strings.TrimSpace(query.OwnerUserID))
	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.StoreID != nil {
		db = db.Where("store_id = ?", *query.StoreID)
	}
	if query.Platform != "" {
		db = db.Where("platform = ?", query.Platform)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}

func intPtrIfPositive(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func floatPtrIfPositive(value float64) *float64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func floatValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
