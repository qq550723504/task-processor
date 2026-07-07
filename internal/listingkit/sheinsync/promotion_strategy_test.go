package sheinsync

import "testing"

func TestSheinPromotionStrategyTimeLimitedEffectivePriceConfig(t *testing.T) {
	strategy := NewSheinPromotionStrategy(SheinPromotionStrategyInput{
		ActivityType:             "TIME_LIMITED",
		StoreID:                  177,
		ActivityPriceMode:        "PROFIT",
		ActivityMinProfitRate:    0.2,
		ActivityStockRatio:       0.5,
		TimeLimitedPriceMode:     "DISCOUNT",
		TimeLimitedDiscountRate:  0.18,
		TimeLimitedMinProfitRate: 0.05,
	})

	if err := strategy.ValidateForPromotionEnrollment(); err != nil {
		t.Fatalf("ValidateForPromotionEnrollment error = %v", err)
	}
	runtimeStrategy := strategy.runtimeOperationStrategy()
	if runtimeStrategy.ActivityPriceMode != "DISCOUNT" {
		t.Fatalf("runtime activity price mode = %q, want time-limited mode DISCOUNT", runtimeStrategy.ActivityPriceMode)
	}
	if runtimeStrategy.ActivityDiscountRate != 0.18 {
		t.Fatalf("runtime activity discount rate = %.2f, want time-limited discount 0.18", runtimeStrategy.ActivityDiscountRate)
	}
	if runtimeStrategy.ActivityMinProfitRate != 0.05 {
		t.Fatalf("runtime activity minimum profit rate = %.2f, want time-limited rate 0.05", runtimeStrategy.ActivityMinProfitRate)
	}
}

func TestSheinPromotionStrategyTimeLimitedFallsBackToActivityPriceConfig(t *testing.T) {
	strategy := NewSheinPromotionStrategy(SheinPromotionStrategyInput{
		ActivityType:          "TIME_LIMITED",
		StoreID:               177,
		ActivityPriceMode:     "DISCOUNT",
		ActivityDiscountRate:  0.22,
		ActivityMinProfitRate: 0.1,
		ActivityStockRatio:    0.5,
	})

	if err := strategy.ValidateForPromotionEnrollment(); err != nil {
		t.Fatalf("ValidateForPromotionEnrollment error = %v", err)
	}
	runtimeStrategy := strategy.runtimeOperationStrategy()
	if runtimeStrategy.ActivityPriceMode != "DISCOUNT" {
		t.Fatalf("runtime activity price mode = %q, want activity fallback DISCOUNT", runtimeStrategy.ActivityPriceMode)
	}
	if runtimeStrategy.ActivityDiscountRate != 0.22 {
		t.Fatalf("runtime activity discount rate = %.2f, want activity fallback discount 0.22", runtimeStrategy.ActivityDiscountRate)
	}
}

func TestSheinPromotionStrategyTimeLimitedDefaultsToFullStockWhenStockLimitDisabled(t *testing.T) {
	strategy := NewSheinPromotionStrategy(SheinPromotionStrategyInput{
		ActivityType:         "TIME_LIMITED",
		StoreID:              177,
		ActivityPriceMode:    "DISCOUNT",
		ActivityDiscountRate: 0.22,
	})

	if err := strategy.ValidateForPromotionEnrollment(); err != nil {
		t.Fatalf("ValidateForPromotionEnrollment error = %v", err)
	}
	runtimeStrategy := strategy.runtimeOperationStrategy()
	if runtimeStrategy.ActivityStockRatio != 1 {
		t.Fatalf("runtime activity stock ratio = %.2f, want default full stock", runtimeStrategy.ActivityStockRatio)
	}
}

func TestSheinPromotionStrategyPassesPromotionPartakeTypeToRuntimeStrategy(t *testing.T) {
	strategy := NewSheinPromotionStrategy(SheinPromotionStrategyInput{
		ActivityType:          "PROMOTION",
		StoreID:               177,
		ActivityPriceMode:     "DISCOUNT",
		ActivityPartakeType:   "LIMITED",
		ActivityDiscountRate:  0.22,
		ActivityMinProfitRate: 0.1,
		ActivityStockRatio:    0.5,
	})

	if err := strategy.ValidateForPromotionEnrollment(); err != nil {
		t.Fatalf("ValidateForPromotionEnrollment error = %v", err)
	}
	runtimeStrategy := strategy.runtimeOperationStrategy()
	if runtimeStrategy.ActivityPartakeType != "LIMITED" {
		t.Fatalf("runtime activity partake type = %q, want LIMITED", runtimeStrategy.ActivityPartakeType)
	}
}

func TestSheinPromotionStrategyRejectsBothProfitWhenLimitedMinProfitIsNotLower(t *testing.T) {
	strategy := NewSheinPromotionStrategy(SheinPromotionStrategyInput{
		ActivityType:                 "PROMOTION",
		StoreID:                      177,
		ActivityPriceMode:            "PROFIT",
		ActivityPartakeType:          "BOTH",
		ActivityMinProfitRate:        0.2,
		ActivityLimitedMinProfitRate: 0.2,
		ActivityStockRatio:           0.5,
	})

	if err := strategy.ValidateForPromotionEnrollment(); err == nil {
		t.Fatalf("ValidateForPromotionEnrollment error = nil, want limited min profit validation")
	}
}

func TestSheinPromotionStrategyAcceptsBreakevenPriceMode(t *testing.T) {
	strategy := NewSheinPromotionStrategy(SheinPromotionStrategyInput{
		ActivityType:        "PROMOTION",
		StoreID:             177,
		ActivityPriceMode:   "BREAKEVEN",
		ActivityPartakeType: "REGULAR",
	})

	if err := strategy.ValidateForPromotionEnrollment(); err != nil {
		t.Fatalf("ValidateForPromotionEnrollment error = %v", err)
	}
	runtimeStrategy := strategy.runtimeOperationStrategy()
	if runtimeStrategy.ActivityPriceMode != "BREAKEVEN" {
		t.Fatalf("runtime activity price mode = %q, want BREAKEVEN", runtimeStrategy.ActivityPriceMode)
	}
}
