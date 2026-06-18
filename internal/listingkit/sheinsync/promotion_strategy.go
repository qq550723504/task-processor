package sheinsync

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
