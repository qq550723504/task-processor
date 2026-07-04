package sheinsync

import (
	"context"

	"task-processor/internal/listingruntime"
	"task-processor/internal/shein/activity"
	"task-processor/internal/shein/api/marketing"
)

func NewSheinActivityAdapter(strategyProvider SheinPromotionStrategyProvider, promotionBridge activity.PromotionRegistrationBridge) SheinActivityAdapter {
	return newSheinActivityAdapter(strategyProvider, wrapLegacyPromotionBridge(promotionBridge))
}

type LegacySheinPromotionBridgeFactory interface {
	BuildPromotionBridge(ctx context.Context, storeID int64) (activity.PromotionRegistrationBridge, error)
}

func NewSheinActivityAdapterWithFactory(strategyProvider SheinPromotionStrategyProvider, promotionBridgeFactory LegacySheinPromotionBridgeFactory) SheinActivityAdapter {
	return newSheinActivityAdapterWithFactory(strategyProvider, wrapLegacyPromotionBridgeFactory(promotionBridgeFactory))
}

func wrapLegacyPromotionBridge(bridge activity.PromotionRegistrationBridge) SheinPromotionBridge {
	if bridge == nil {
		return nil
	}
	return &legacyPromotionBridgeAdapter{bridge: bridge}
}

func wrapLegacyPromotionBridgeFactory(factory LegacySheinPromotionBridgeFactory) SheinPromotionBridgeFactory {
	if factory == nil {
		return nil
	}
	return legacyPromotionBridgeFactoryAdapter{factory: factory}
}

type legacyPromotionBridgeAdapter struct {
	bridge activity.PromotionRegistrationBridge
}

func (a *legacyPromotionBridgeAdapter) RegisterPromotionProducts(ctx context.Context, strategy *SheinPromotionStrategy, activityKey string, products []marketing.SkcInfo) (*SheinPromotionRegistrationResult, error) {
	result, err := a.bridge.RegisterPromotionProducts(ctx, strategy.runtimeOperationStrategy(), activityKey, products)
	if result == nil {
		return nil, err
	}
	return &SheinPromotionRegistrationResult{
		Request:          result.Request,
		Response:         result.Response,
		ActivityRequest:  result.ActivityRequest,
		ActivityResponse: result.ActivityResponse,
		FilterReasons:    result.FilterReasons,
	}, err
}

func (a *legacyPromotionBridgeAdapter) StartPromotionRegistrationSession(
	ctx context.Context,
	strategy *SheinPromotionStrategy,
	activityKey string,
) (SheinPromotionRegistrationSession, error) {
	factory, ok := a.bridge.(activity.PromotionRegistrationSessionFactory)
	if !ok {
		return nil, nil
	}
	session, err := factory.NewPromotionRegistrationSession(ctx, strategy.runtimeOperationStrategy(), activityKey)
	if err != nil || session == nil {
		return nil, err
	}
	return legacyPromotionRegistrationSessionAdapter{session: session}, nil
}

type legacyPromotionRegistrationSessionAdapter struct {
	session activity.PromotionRegistrationSession
}

func (a legacyPromotionRegistrationSessionAdapter) RegisterPromotionProducts(
	ctx context.Context,
	activityKey string,
	products []marketing.SkcInfo,
) (*SheinPromotionRegistrationResult, error) {
	result, err := a.session.RegisterPromotionProducts(ctx, activityKey, products)
	if result == nil {
		return nil, err
	}
	return &SheinPromotionRegistrationResult{
		Request:          result.Request,
		Response:         result.Response,
		ActivityRequest:  result.ActivityRequest,
		ActivityResponse: result.ActivityResponse,
		FilterReasons:    result.FilterReasons,
	}, err
}

type legacyPromotionBridgeFactoryAdapter struct {
	factory LegacySheinPromotionBridgeFactory
}

func (a legacyPromotionBridgeFactoryAdapter) BuildPromotionBridge(ctx context.Context, storeID int64) (SheinPromotionBridge, error) {
	bridge, err := a.factory.BuildPromotionBridge(ctx, storeID)
	if err != nil || bridge == nil {
		return nil, err
	}
	return &legacyPromotionBridgeAdapter{bridge: bridge}, nil
}

func (s *SheinPromotionStrategy) runtimeOperationStrategy() *listingruntime.OperationStrategy {
	if s == nil {
		return nil
	}
	return &listingruntime.OperationStrategy{
		StoreID:                      s.StoreID,
		ActivityPriceMode:            s.EffectiveActivityPriceMode(),
		ActivityPartakeType:          s.EffectiveActivityPartakeType(),
		ActivityDiscountRate:         s.EffectiveActivityDiscountRate(),
		ActivityLimitedDiscountRate:  s.EffectiveActivityLimitedDiscountRate(),
		ActivityMinProfitRate:        s.EffectiveActivityMinProfitRate(),
		ActivityLimitedMinProfitRate: s.EffectiveActivityLimitedMinProfitRate(),
		ActivityStockRatio:           s.EffectiveActivityStockRatio(),
		FixedPriceAdjustment:         s.FixedPriceAdjustment,
		TimeLimitedDiscountRate:      s.TimeLimitedDiscountRate,
		TimeLimitedMinProfitRate:     s.TimeLimitedMinProfitRate,
		TimeLimitedPriceMode:         s.TimeLimitedPriceMode,
		TimeLimitedUserLimit:         s.TimeLimitedUserLimit,
		TimeLimitedUserLimitNum:      s.TimeLimitedUserLimitNum,
		TimeLimitedStockLimit:        s.TimeLimitedStockLimit,
		TimeLimitedStockLimitPercent: s.TimeLimitedStockLimitPercent,
	}
}
