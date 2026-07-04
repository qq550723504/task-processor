package sheinsync

import (
	"fmt"
	"strings"
)

type SheinPromotionStrategyInput struct {
	ActivityType                 string
	StoreID                      int64
	ActivityPriceMode            string
	ActivityPartakeType          string
	ActivityDiscountRate         float64
	ActivityLimitedDiscountRate  float64
	ActivityMinProfitRate        float64
	ActivityLimitedMinProfitRate float64
	ActivityStockRatio           float64
	FixedPriceAdjustment         float64
	TimeLimitedDiscountRate      float64
	TimeLimitedMinProfitRate     float64
	TimeLimitedPriceMode         string
	TimeLimitedUserLimit         bool
	TimeLimitedUserLimitNum      int
	TimeLimitedStockLimit        bool
	TimeLimitedStockLimitPercent int
}

type SheinPromotionStrategy struct {
	ActivityType                 string
	StoreID                      int64
	ActivityPriceMode            string
	ActivityPartakeType          string
	ActivityDiscountRate         float64
	ActivityLimitedDiscountRate  float64
	ActivityMinProfitRate        float64
	ActivityLimitedMinProfitRate float64
	ActivityStockRatio           float64
	FixedPriceAdjustment         float64
	TimeLimitedDiscountRate      float64
	TimeLimitedMinProfitRate     float64
	TimeLimitedPriceMode         string
	TimeLimitedUserLimit         bool
	TimeLimitedUserLimitNum      int
	TimeLimitedStockLimit        bool
	TimeLimitedStockLimitPercent int
}

func NewSheinPromotionStrategy(input SheinPromotionStrategyInput) *SheinPromotionStrategy {
	return &SheinPromotionStrategy{
		ActivityType:                 normalizeSheinActivityType(input.ActivityType),
		StoreID:                      input.StoreID,
		ActivityPriceMode:            input.ActivityPriceMode,
		ActivityPartakeType:          normalizeSheinPartakeType(input.ActivityPartakeType),
		ActivityDiscountRate:         input.ActivityDiscountRate,
		ActivityLimitedDiscountRate:  input.ActivityLimitedDiscountRate,
		ActivityMinProfitRate:        input.ActivityMinProfitRate,
		ActivityLimitedMinProfitRate: input.ActivityLimitedMinProfitRate,
		ActivityStockRatio:           input.ActivityStockRatio,
		FixedPriceAdjustment:         input.FixedPriceAdjustment,
		TimeLimitedDiscountRate:      input.TimeLimitedDiscountRate,
		TimeLimitedMinProfitRate:     input.TimeLimitedMinProfitRate,
		TimeLimitedPriceMode:         input.TimeLimitedPriceMode,
		TimeLimitedUserLimit:         input.TimeLimitedUserLimit,
		TimeLimitedUserLimitNum:      input.TimeLimitedUserLimitNum,
		TimeLimitedStockLimit:        input.TimeLimitedStockLimit,
		TimeLimitedStockLimitPercent: input.TimeLimitedStockLimitPercent,
	}
}

func (s *SheinPromotionStrategy) ValidateForPromotionEnrollment() error {
	if s == nil {
		return fmt.Errorf("SHEIN promotion strategy is required")
	}
	if s.StoreID <= 0 {
		return fmt.Errorf("SHEIN promotion strategy store id is required")
	}
	if s.requiresActivityStockRatio() && (s.EffectiveActivityStockRatio() <= 0 || s.EffectiveActivityStockRatio() > 1) {
		return fmt.Errorf("SHEIN promotion strategy activity stock ratio must be between 0 and 1")
	}
	switch s.EffectiveActivityPriceMode() {
	case "", "DISCOUNT":
		if s.EffectiveActivityDiscountRate() <= 0 || s.EffectiveActivityDiscountRate() >= 1 {
			return fmt.Errorf("SHEIN promotion strategy activity discount rate must be between 0 and 1")
		}
		if s.EffectiveActivityPartakeType() == "BOTH" {
			if s.EffectiveActivityLimitedDiscountRate() <= 0 || s.EffectiveActivityLimitedDiscountRate() >= 1 {
				return fmt.Errorf("SHEIN promotion strategy activity limited discount rate must be between 0 and 1")
			}
			if s.EffectiveActivityLimitedDiscountRate() <= s.EffectiveActivityDiscountRate() {
				return fmt.Errorf("SHEIN promotion strategy activity limited discount rate must be greater than regular discount rate")
			}
		}
	case "PROFIT":
		if s.EffectiveActivityMinProfitRate() < 0 || s.EffectiveActivityMinProfitRate() >= 1 {
			return fmt.Errorf("SHEIN promotion strategy activity minimum profit rate must be between 0 and 1")
		}
		if s.EffectiveActivityPartakeType() == "BOTH" {
			if s.EffectiveActivityLimitedMinProfitRate() < 0 || s.EffectiveActivityLimitedMinProfitRate() >= 1 {
				return fmt.Errorf("SHEIN promotion strategy activity limited minimum profit rate must be between 0 and 1")
			}
			if s.EffectiveActivityLimitedMinProfitRate() >= s.EffectiveActivityMinProfitRate() {
				return fmt.Errorf("SHEIN promotion strategy activity limited minimum profit rate must be less than regular minimum profit rate")
			}
		}
	default:
		return fmt.Errorf("unsupported SHEIN promotion activity price mode %q", s.EffectiveActivityPriceMode())
	}
	return nil
}

func (s *SheinPromotionStrategy) EffectiveActivityPartakeType() string {
	if s == nil {
		return "REGULAR"
	}
	return normalizeSheinPartakeType(s.ActivityPartakeType)
}

func (s *SheinPromotionStrategy) EffectiveActivityPriceMode() string {
	if s == nil {
		return ""
	}
	if s.isTimeLimitedActivity() {
		if mode := strings.ToUpper(strings.TrimSpace(s.TimeLimitedPriceMode)); mode != "" {
			return mode
		}
	}
	return strings.ToUpper(strings.TrimSpace(s.ActivityPriceMode))
}

func (s *SheinPromotionStrategy) EffectiveActivityDiscountRate() float64 {
	if s == nil {
		return 0
	}
	if s.isTimeLimitedActivity() && strings.TrimSpace(s.TimeLimitedPriceMode) != "" {
		return s.TimeLimitedDiscountRate
	}
	return s.ActivityDiscountRate
}

func (s *SheinPromotionStrategy) EffectiveActivityLimitedDiscountRate() float64 {
	if s == nil {
		return 0
	}
	return s.ActivityLimitedDiscountRate
}

func (s *SheinPromotionStrategy) EffectiveActivityMinProfitRate() float64 {
	if s == nil {
		return 0
	}
	if s.isTimeLimitedActivity() && strings.TrimSpace(s.TimeLimitedPriceMode) != "" {
		return s.TimeLimitedMinProfitRate
	}
	return s.ActivityMinProfitRate
}

func (s *SheinPromotionStrategy) EffectiveActivityLimitedMinProfitRate() float64 {
	if s == nil {
		return 0
	}
	return s.ActivityLimitedMinProfitRate
}

func (s *SheinPromotionStrategy) EffectiveActivityStockRatio() float64 {
	if s == nil {
		return 0
	}
	if !s.requiresActivityStockRatio() && s.ActivityStockRatio <= 0 {
		return 1
	}
	if s.isTimeLimitedActivity() && s.TimeLimitedStockLimit && s.TimeLimitedStockLimitPercent > 0 {
		return float64(s.TimeLimitedStockLimitPercent) / 100
	}
	return s.ActivityStockRatio
}

func (s *SheinPromotionStrategy) requiresActivityStockRatio() bool {
	if s == nil {
		return false
	}
	switch s.EffectiveActivityPartakeType() {
	case "LIMITED", "BOTH":
		return true
	default:
		return s.isTimeLimitedActivity()
	}
}

func (s *SheinPromotionStrategy) isTimeLimitedActivity() bool {
	if s == nil {
		return false
	}
	return normalizeSheinActivityType(s.ActivityType) == "TIME_LIMITED"
}

func normalizeSheinActivityType(activityType string) string {
	switch strings.ToUpper(strings.TrimSpace(activityType)) {
	case "TIME_LIMITED":
		return "TIME_LIMITED"
	default:
		return "PROMOTION"
	}
}

func normalizeSheinPartakeType(partakeType string) string {
	normalized := strings.ToUpper(strings.TrimSpace(partakeType))
	switch normalized {
	case "LIMITED", "BOTH":
		return normalized
	default:
		return "REGULAR"
	}
}
