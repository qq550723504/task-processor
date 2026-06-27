package sheinsync

import (
	"fmt"
	"strings"
)

type SheinPromotionStrategyInput struct {
	StoreID               int64
	ActivityPriceMode     string
	ActivityDiscountRate  float64
	ActivityMinProfitRate float64
	ActivityStockRatio    float64
	FixedPriceAdjustment  float64
}

type SheinPromotionStrategy struct {
	StoreID               int64
	ActivityPriceMode     string
	ActivityDiscountRate  float64
	ActivityMinProfitRate float64
	ActivityStockRatio    float64
	FixedPriceAdjustment  float64
}

func NewSheinPromotionStrategy(input SheinPromotionStrategyInput) *SheinPromotionStrategy {
	return &SheinPromotionStrategy{
		StoreID:               input.StoreID,
		ActivityPriceMode:     input.ActivityPriceMode,
		ActivityDiscountRate:  input.ActivityDiscountRate,
		ActivityMinProfitRate: input.ActivityMinProfitRate,
		ActivityStockRatio:    input.ActivityStockRatio,
		FixedPriceAdjustment:  input.FixedPriceAdjustment,
	}
}

func (s *SheinPromotionStrategy) ValidateForPromotionEnrollment() error {
	if s == nil {
		return fmt.Errorf("SHEIN promotion strategy is required")
	}
	if s.StoreID <= 0 {
		return fmt.Errorf("SHEIN promotion strategy store id is required")
	}
	if s.ActivityStockRatio <= 0 || s.ActivityStockRatio > 1 {
		return fmt.Errorf("SHEIN promotion strategy activity stock ratio must be between 0 and 1")
	}
	switch strings.ToUpper(strings.TrimSpace(s.ActivityPriceMode)) {
	case "", "DISCOUNT":
		if s.ActivityDiscountRate <= 0 || s.ActivityDiscountRate >= 1 {
			return fmt.Errorf("SHEIN promotion strategy activity discount rate must be between 0 and 1")
		}
	case "PROFIT":
		if s.ActivityMinProfitRate < 0 || s.ActivityMinProfitRate >= 1 {
			return fmt.Errorf("SHEIN promotion strategy activity minimum profit rate must be between 0 and 1")
		}
	default:
		return fmt.Errorf("unsupported SHEIN promotion activity price mode %q", s.ActivityPriceMode)
	}
	return nil
}
