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
