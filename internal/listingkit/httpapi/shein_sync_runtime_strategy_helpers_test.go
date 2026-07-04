package httpapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
)

func TestLocalRuntimePromotionStrategyProviderUsesPromotionStrategyByDefault(t *testing.T) {
	t.Parallel()

	repo := &runtimePromotionStrategyRepoStub{
		strategies: map[string]*listingadmin.OperationStrategy{
			"PROMOTION": {
				StoreID:               870,
				ActivityPriceMode:     "DISCOUNT",
				ActivityPartakeType:   "LIMITED",
				ActivityDiscountRate:  runtimeStrategyFloat64Ptr(0.2),
				ActivityMinProfitRate: runtimeStrategyFloat64Ptr(0.15),
				ActivityStockRatio:    runtimeStrategyFloat64Ptr(0.5),
				FixedPriceAdjustment:  runtimeStrategyFloat64Ptr(1.2),
			},
		},
	}
	provider := localRuntimePromotionStrategyProvider{repo: repo}

	strategy, err := provider.GetPromotionStrategy(listingkit.WithTenantID(context.Background(), "227"), 870, "PROMOTION:227:870")

	require.NoError(t, err)
	require.NotNil(t, strategy)
	require.Equal(t, []int64{227}, repo.requestedTenantIDs)
	require.Equal(t, []int64{870}, repo.requestedStoreIDs)
	require.Equal(t, []string{"SHEIN"}, repo.requestedPlatforms)
	require.Equal(t, []string{"PROMOTION"}, repo.requestedActivityTypes)
	require.Equal(t, "DISCOUNT", strategy.ActivityPriceMode)
	require.Equal(t, "LIMITED", strategy.ActivityPartakeType)
	require.Equal(t, 0.2, strategy.ActivityDiscountRate)
	require.Equal(t, 0.15, strategy.ActivityMinProfitRate)
	require.Equal(t, 0.5, strategy.ActivityStockRatio)
	require.Equal(t, 1.2, strategy.FixedPriceAdjustment)
}

func TestLocalRuntimePromotionStrategyProviderUsesTimeLimitedFieldsForTimeLimitedActivity(t *testing.T) {
	t.Parallel()

	repo := &runtimePromotionStrategyRepoStub{
		strategies: map[string]*listingadmin.OperationStrategy{
			"TIME_LIMITED": {
				StoreID:                      870,
				ActivityPriceMode:            "DISCOUNT",
				ActivityDiscountRate:         runtimeStrategyFloat64Ptr(0.2),
				ActivityMinProfitRate:        runtimeStrategyFloat64Ptr(0.15),
				ActivityStockRatio:           runtimeStrategyFloat64Ptr(0.5),
				FixedPriceAdjustment:         runtimeStrategyFloat64Ptr(1.2),
				TimeLimitedPriceMode:         "PROFIT",
				TimeLimitedDiscountRate:      runtimeStrategyFloat64Ptr(0.25),
				TimeLimitedMinProfitRate:     runtimeStrategyFloat64Ptr(0),
				TimeLimitedStockLimit:        true,
				TimeLimitedStockLimitPercent: intPtr(30),
				TimeLimitedUserLimit:         true,
				TimeLimitedUserLimitNum:      intPtr(2),
			},
		},
	}
	provider := localRuntimePromotionStrategyProvider{repo: repo}

	strategy, err := provider.GetPromotionStrategy(listingkit.WithTenantID(context.Background(), "227"), 870, "TIME_LIMITED:227:870")

	require.NoError(t, err)
	require.NotNil(t, strategy)
	require.Equal(t, []int64{227}, repo.requestedTenantIDs)
	require.Equal(t, []int64{870}, repo.requestedStoreIDs)
	require.Equal(t, []string{"SHEIN"}, repo.requestedPlatforms)
	require.Equal(t, []string{"TIME_LIMITED"}, repo.requestedActivityTypes)
	require.Equal(t, "PROFIT", strategy.ActivityPriceMode)
	require.Equal(t, 0.25, strategy.ActivityDiscountRate)
	require.Equal(t, 0.0, strategy.ActivityMinProfitRate)
	require.Equal(t, 0.3, strategy.ActivityStockRatio)
	require.Equal(t, "PROFIT", strategy.TimeLimitedPriceMode)
	require.Equal(t, 0.25, strategy.TimeLimitedDiscountRate)
	require.Equal(t, 0.0, strategy.TimeLimitedMinProfitRate)
	require.True(t, strategy.TimeLimitedStockLimit)
	require.Equal(t, 30, strategy.TimeLimitedStockLimitPercent)
	require.True(t, strategy.TimeLimitedUserLimit)
	require.Equal(t, 2, strategy.TimeLimitedUserLimitNum)
}

type runtimePromotionStrategyRepoStub struct {
	listingadmin.OperationStrategyRepository
	strategies             map[string]*listingadmin.OperationStrategy
	requestedTenantIDs     []int64
	requestedStoreIDs      []int64
	requestedPlatforms     []string
	requestedActivityTypes []string
}

func (r *runtimePromotionStrategyRepoStub) GetActiveActivityStrategy(
	_ context.Context,
	tenantID, storeID int64,
	platform, activityType string,
) (*listingadmin.OperationStrategy, error) {
	r.requestedTenantIDs = append(r.requestedTenantIDs, tenantID)
	r.requestedStoreIDs = append(r.requestedStoreIDs, storeID)
	r.requestedPlatforms = append(r.requestedPlatforms, platform)
	r.requestedActivityTypes = append(r.requestedActivityTypes, activityType)
	if r.strategies == nil {
		return nil, nil
	}
	return r.strategies[activityType], nil
}

func intPtr(v int) *int {
	return &v
}

func runtimeStrategyFloat64Ptr(v float64) *float64 {
	return &v
}
