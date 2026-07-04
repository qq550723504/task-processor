package httpapi

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	sheinsync "task-processor/internal/listingkit/sheinsync"
)

type localRuntimePromotionStrategyProvider struct {
	repo listingadmin.OperationStrategyRepository
}

func (p localRuntimePromotionStrategyProvider) GetPromotionStrategy(ctx context.Context, storeID int64, activityKey string) (*sheinsync.SheinPromotionStrategy, error) {
	if p.repo == nil {
		return nil, fmt.Errorf("SHEIN promotion strategy repository is not configured")
	}
	tenantID, err := sheinPromotionTenantID(ctx)
	if err != nil {
		return nil, err
	}
	activityType := sheinPromotionActivityTypeFromKey(activityKey)
	strategy, err := p.repo.GetActiveActivityStrategy(ctx, tenantID, storeID, "SHEIN", activityType)
	if err != nil {
		return nil, err
	}
	if strategy == nil {
		return nil, nil
	}
	return sheinsync.NewSheinPromotionStrategy(sheinPromotionStrategyInput(activityType, strategy)), nil
}

func buildSheinPromotionStrategyProvider(repositories *builtRepositories) (localRuntimePromotionStrategyProvider, error) {
	if repositories == nil {
		return localRuntimePromotionStrategyProvider{}, nil
	}
	if repositories.operationStrategyRepository == nil {
		return localRuntimePromotionStrategyProvider{}, nil
	}
	return localRuntimePromotionStrategyProvider{repo: repositories.operationStrategyRepository}, nil
}

func sheinPromotionFloat64(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func sheinPromotionInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func sheinPromotionActivityTypeFromKey(activityKey string) string {
	parts := strings.SplitN(strings.ToUpper(strings.TrimSpace(activityKey)), ":", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "PROMOTION"
	}
	switch parts[0] {
	case "PROMOTION", "TIME_LIMITED":
		return parts[0]
	default:
		return "PROMOTION"
	}
}

func sheinPromotionStrategyInput(activityType string, strategy *listingadmin.OperationStrategy) sheinsync.SheinPromotionStrategyInput {
	input := sheinsync.SheinPromotionStrategyInput{
		ActivityType:                 activityType,
		StoreID:                      strategy.StoreID,
		ActivityPriceMode:            strategy.ActivityPriceMode,
		ActivityPartakeType:          strategy.ActivityPartakeType,
		ActivityDiscountRate:         sheinPromotionFloat64(strategy.ActivityDiscountRate),
		ActivityLimitedDiscountRate:  sheinPromotionFloat64(strategy.ActivityLimitedDiscountRate),
		ActivityMinProfitRate:        sheinPromotionFloat64(strategy.ActivityMinProfitRate),
		ActivityLimitedMinProfitRate: sheinPromotionFloat64(strategy.ActivityLimitedMinProfitRate),
		ActivityStockRatio:           sheinPromotionFloat64(strategy.ActivityStockRatio),
		FixedPriceAdjustment:         sheinPromotionFloat64(strategy.FixedPriceAdjustment),
	}
	if activityType != "TIME_LIMITED" {
		return input
	}

	input.TimeLimitedPriceMode = strings.ToUpper(strings.TrimSpace(strategy.TimeLimitedPriceMode))
	input.TimeLimitedDiscountRate = sheinPromotionFloat64(strategy.TimeLimitedDiscountRate)
	input.TimeLimitedMinProfitRate = sheinPromotionFloat64(strategy.TimeLimitedMinProfitRate)
	input.TimeLimitedUserLimit = strategy.TimeLimitedUserLimit
	input.TimeLimitedUserLimitNum = sheinPromotionInt(strategy.TimeLimitedUserLimitNum)
	input.TimeLimitedStockLimit = strategy.TimeLimitedStockLimit
	input.TimeLimitedStockLimitPercent = sheinPromotionInt(strategy.TimeLimitedStockLimitPercent)

	if input.TimeLimitedPriceMode != "" {
		input.ActivityPriceMode = input.TimeLimitedPriceMode
	}
	if input.TimeLimitedDiscountRate > 0 {
		input.ActivityDiscountRate = input.TimeLimitedDiscountRate
	}
	if strategy.TimeLimitedMinProfitRate != nil {
		input.ActivityMinProfitRate = input.TimeLimitedMinProfitRate
	}
	if input.TimeLimitedStockLimit && input.TimeLimitedStockLimitPercent > 0 {
		input.ActivityStockRatio = float64(input.TimeLimitedStockLimitPercent) / 100
	}
	return input
}

func sheinPromotionTenantID(ctx context.Context) (int64, error) {
	value := strings.TrimSpace(listingkit.TenantIDFromContext(ctx))
	tenantID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || tenantID <= 0 {
		return 0, fmt.Errorf("numeric tenant id is required for SHEIN promotion strategy")
	}
	return tenantID, nil
}
