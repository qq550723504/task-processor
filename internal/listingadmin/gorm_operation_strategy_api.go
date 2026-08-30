package listingadmin

import (
	"context"
	"time"

	"task-processor/internal/pkg/types"
)

// NewGormOperationStrategyAPI adapts the operation strategy repository for
// callers using the management API DTO contract.
func NewGormOperationStrategyAPI(repository *GormOperationStrategyRepository) OperationStrategyAPI {
	if repository == nil {
		return nil
	}
	return gormOperationStrategyAPI{repository: repository}
}

type gormOperationStrategyAPI struct {
	repository *GormOperationStrategyRepository
}

func (a gormOperationStrategyAPI) GetOperationStrategyByStoreId(storeID int64) (*OperationStrategyDTO, error) {
	if a.repository == nil {
		return nil, nil
	}
	strategy, err := a.repository.GetLatestByStoreID(context.Background(), storeID)
	if err != nil || strategy == nil {
		return nil, err
	}
	return OperationStrategyToDTO(strategy), nil
}

// OperationStrategyToDTO exposes the management API projection for an
// operation strategy.
func OperationStrategyToDTO(strategy *OperationStrategy) *OperationStrategyDTO {
	if strategy == nil {
		return nil
	}
	return &OperationStrategyDTO{
		ID:                           strategy.ID,
		TenantID:                     strategy.TenantID,
		StoreID:                      strategy.StoreID,
		Name:                         strategy.Name,
		Platform:                     strategy.Platform,
		Status:                       strategy.Status,
		StockChangeThreshold:         operationStrategyIntValue(strategy.StockChangeThreshold),
		StockChangeAction:            strategy.StockChangeAction,
		OutOfStockAction:             strategy.OutOfStockAction,
		MinProfitRate:                operationStrategyFloat64Value(strategy.MinProfitRate),
		LowProfitAction:              strategy.LowProfitAction,
		PriceUpdateMultiplier:        operationStrategyFloat64Value(strategy.PriceUpdateMultiplier),
		StockUpdateRatio:             operationStrategyFloat64Value(strategy.StockUpdateRatio),
		ActivityEnabled:              strategy.ActivityEnabled,
		ActivityType:                 strategy.ActivityType,
		ActivityDiscountRate:         operationStrategyFloat64Value(strategy.ActivityDiscountRate),
		ActivityLimitedDiscountRate:  operationStrategyFloat64Value(strategy.ActivityLimitedDiscountRate),
		ActivityStockRatio:           operationStrategyFloat64Value(strategy.ActivityStockRatio),
		PromotionRatio:               operationStrategyFloat64Value(strategy.PromotionRatio),
		ActivityMinProfitRate:        operationStrategyFloat64Value(strategy.ActivityMinProfitRate),
		ActivityLimitedMinProfitRate: operationStrategyFloat64Value(strategy.ActivityLimitedMinProfitRate),
		ActivityPriceMode:            strategy.ActivityPriceMode,
		ActivityPartakeType:          strategy.ActivityPartakeType,
		TimeLimitedDiscountRate:      operationStrategyFloat64Value(strategy.TimeLimitedDiscountRate),
		TimeLimitedMinProfitRate:     operationStrategyFloat64Value(strategy.TimeLimitedMinProfitRate),
		TimeLimitedPriceMode:         strategy.TimeLimitedPriceMode,
		TimeLimitedUserLimit:         strategy.TimeLimitedUserLimit,
		TimeLimitedUserLimitNum:      operationStrategyIntValue(strategy.TimeLimitedUserLimitNum),
		TimeLimitedStockLimit:        strategy.TimeLimitedStockLimit,
		TimeLimitedStockLimitPercent: operationStrategyIntValue(strategy.TimeLimitedStockLimitPercent),
		FixedPriceAdjustment:         operationStrategyFloat64Value(strategy.FixedPriceAdjustment),
		PriceIncreaseThreshold:       operationStrategyFloat64Value(strategy.PriceIncreaseThreshold),
		PriceDecreaseThreshold:       operationStrategyFloat64Value(strategy.PriceDecreaseThreshold),
		PriceIncreaseAction:          strategy.PriceIncreaseAction,
		PriceDecreaseAction:          strategy.PriceDecreaseAction,
		RestoreStockAmount:           operationStrategyIntValue(strategy.RestoreStockAmount),
		Remark:                       strategy.Remark,
		CreateTime:                   operationStrategyFlexibleString(strategy.CreateTime),
	}
}

func operationStrategyIntValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func operationStrategyFloat64Value(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func operationStrategyFlexibleString(value *time.Time) types.FlexibleString {
	if value == nil {
		return ""
	}
	return types.FlexibleString(value.Format(time.RFC3339))
}
