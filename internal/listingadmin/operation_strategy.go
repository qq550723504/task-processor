package listingadmin

import (
	"context"
	"errors"
	"time"
)

var ErrOperationStrategyNotFound = errors.New("operation strategy not found")

type OperationStrategy struct {
	ID                           int64      `json:"id"`
	TenantID                     int64      `json:"tenantId"`
	StoreID                      int64      `json:"storeId"`
	Name                         string     `json:"name"`
	Platform                     string     `json:"platform"`
	Status                       int16      `json:"status"`
	StockChangeThreshold         *int       `json:"stockChangeThreshold,omitempty"`
	StockChangeAction            string     `json:"stockChangeAction,omitempty"`
	OutOfStockAction             string     `json:"outOfStockAction,omitempty"`
	MinProfitRate                *float64   `json:"minProfitRate,omitempty"`
	LowProfitAction              string     `json:"lowProfitAction,omitempty"`
	PriceUpdateMultiplier        *float64   `json:"priceUpdateMultiplier,omitempty"`
	FixedPriceAdjustment         *float64   `json:"fixedPriceAdjustment,omitempty"`
	StockUpdateRatio             *float64   `json:"stockUpdateRatio,omitempty"`
	ActivityEnabled              bool       `json:"activityEnabled"`
	ActivityType                 string     `json:"activityType,omitempty"`
	ActivityDiscountRate         *float64   `json:"activityDiscountRate,omitempty"`
	ActivityStockRatio           *float64   `json:"activityStockRatio,omitempty"`
	PromotionRatio               *float64   `json:"promotionRatio,omitempty"`
	ActivityMinProfitRate        *float64   `json:"activityMinProfitRate,omitempty"`
	ActivityPriceMode            string     `json:"activityPriceMode,omitempty"`
	TimeLimitedDiscountRate      *float64   `json:"timeLimitedDiscountRate,omitempty"`
	TimeLimitedMinProfitRate     *float64   `json:"timeLimitedMinProfitRate,omitempty"`
	TimeLimitedPriceMode         string     `json:"timeLimitedPriceMode,omitempty"`
	TimeLimitedUserLimit         bool       `json:"timeLimitedUserLimit"`
	TimeLimitedUserLimitNum      *int       `json:"timeLimitedUserLimitNum,omitempty"`
	TimeLimitedStockLimit        bool       `json:"timeLimitedStockLimit"`
	TimeLimitedStockLimitPercent *int       `json:"timeLimitedStockLimitPercent,omitempty"`
	PriceIncreaseThreshold       *float64   `json:"priceIncreaseThreshold,omitempty"`
	PriceDecreaseThreshold       *float64   `json:"priceDecreaseThreshold,omitempty"`
	PriceIncreaseAction          string     `json:"priceIncreaseAction,omitempty"`
	PriceDecreaseAction          string     `json:"priceDecreaseAction,omitempty"`
	RestoreStockAmount           *int       `json:"restoreStockAmount,omitempty"`
	Remark                       string     `json:"remark,omitempty"`
	CreateTime                   *time.Time `json:"createTime,omitempty"`
	UpdateTime                   *time.Time `json:"updateTime,omitempty"`
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
	GetLatestByStoreID(ctx context.Context, storeID int64) (*OperationStrategy, error)
	GetActiveActivityStrategy(ctx context.Context, tenantID, storeID int64, platform, activityType string) (*OperationStrategy, error)
	CreateOperationStrategy(ctx context.Context, strategy *OperationStrategy) (*OperationStrategy, error)
	UpdateOperationStrategy(ctx context.Context, strategy *OperationStrategy) (*OperationStrategy, error)
	SaveActivityStrategy(ctx context.Context, strategy *OperationStrategy) (*OperationStrategy, error)
	UpdateOperationStrategyStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*OperationStrategy, error)
	DeleteOperationStrategy(ctx context.Context, tenantID, id int64) error
}

type listingOperationStrategy struct {
	ID                           int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID                     int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID                  string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	StoreID                      int64      `gorm:"column:store_id;not null;index"`
	Name                         string     `gorm:"column:name;not null"`
	Platform                     string     `gorm:"column:platform;not null;index"`
	Status                       int16      `gorm:"column:status;not null;default:0;index"`
	StockChangeThreshold         int        `gorm:"column:stock_change_threshold"`
	StockChangeAction            string     `gorm:"column:stock_change_action"`
	OutOfStockAction             string     `gorm:"column:out_of_stock_action"`
	MinProfitRate                float64    `gorm:"column:min_profit_rate"`
	LowProfitAction              string     `gorm:"column:low_profit_action"`
	PriceUpdateMultiplier        float64    `gorm:"column:price_update_multiplier"`
	FixedPriceAdjustment         float64    `gorm:"column:fixed_price_adjustment"`
	StockUpdateRatio             float64    `gorm:"column:stock_update_ratio"`
	Remark                       string     `gorm:"column:remark"`
	ActivityEnabled              int16      `gorm:"column:activity_enabled;not null;default:0"`
	ActivityType                 string     `gorm:"column:activity_type"`
	ActivityDiscountRate         float64    `gorm:"column:activity_discount_rate"`
	ActivityStockRatio           float64    `gorm:"column:activity_stock_ratio"`
	PromotionRatio               float64    `gorm:"column:promotion_ratio"`
	ActivityMinProfitRate        float64    `gorm:"column:activity_min_profit_rate"`
	ActivityPriceMode            string     `gorm:"column:activity_price_mode"`
	TimeLimitedDiscountRate      float64    `gorm:"column:time_limited_discount_rate"`
	TimeLimitedMinProfitRate     float64    `gorm:"column:time_limited_min_profit_rate"`
	TimeLimitedPriceMode         string     `gorm:"column:time_limited_price_mode"`
	TimeLimitedUserLimit         bool       `gorm:"column:time_limited_user_limit"`
	TimeLimitedUserLimitNum      int        `gorm:"column:time_limited_user_limit_num"`
	TimeLimitedStockLimit        bool       `gorm:"column:time_limited_stock_limit"`
	TimeLimitedStockLimitPercent int        `gorm:"column:time_limited_stock_limit_percent"`
	PriceIncreaseThreshold       float64    `gorm:"column:price_increase_threshold"`
	PriceDecreaseThreshold       float64    `gorm:"column:price_decrease_threshold"`
	PriceIncreaseAction          string     `gorm:"column:price_increase_action"`
	PriceDecreaseAction          string     `gorm:"column:price_decrease_action"`
	RestoreStockAmount           int        `gorm:"column:restore_stock_amount"`
	Creator                      string     `gorm:"column:creator"`
	CreatedBy                    string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime                   *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater                      string     `gorm:"column:updater"`
	UpdatedBy                    string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime                   *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted                      int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingOperationStrategy) TableName() string { return "listing_operation_strategy" }
