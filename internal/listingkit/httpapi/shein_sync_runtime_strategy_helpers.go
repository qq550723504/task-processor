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

func (p localRuntimePromotionStrategyProvider) GetPromotionStrategy(ctx context.Context, storeID int64, _ string) (*sheinsync.SheinPromotionStrategy, error) {
	if p.repo == nil {
		return nil, fmt.Errorf("SHEIN promotion strategy repository is not configured")
	}
	tenantID, err := sheinPromotionTenantID(ctx)
	if err != nil {
		return nil, err
	}
	strategy, err := p.repo.GetActiveActivityStrategy(ctx, tenantID, storeID, "SHEIN", "PROMOTION")
	if err != nil {
		return nil, err
	}
	if strategy == nil {
		return nil, nil
	}
	return sheinsync.NewSheinPromotionStrategy(sheinsync.SheinPromotionStrategyInput{
		StoreID:               strategy.StoreID,
		ActivityPriceMode:     strategy.ActivityPriceMode,
		ActivityDiscountRate:  sheinPromotionFloat64(strategy.ActivityDiscountRate),
		ActivityMinProfitRate: sheinPromotionFloat64(strategy.ActivityMinProfitRate),
		ActivityStockRatio:    sheinPromotionFloat64(strategy.ActivityStockRatio),
		FixedPriceAdjustment:  sheinPromotionFloat64(strategy.FixedPriceAdjustment),
	}), nil
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

func sheinPromotionTenantID(ctx context.Context) (int64, error) {
	value := strings.TrimSpace(listingkit.TenantIDFromContext(ctx))
	tenantID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || tenantID <= 0 {
		return 0, fmt.Errorf("numeric tenant id is required for SHEIN promotion strategy")
	}
	return tenantID, nil
}
