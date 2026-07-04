package listingruntime

type OperationStrategy struct {
	ID                           int64
	TenantID                     int64
	StoreID                      int64
	Name                         string
	Platform                     string
	Status                       int16
	StockChangeThreshold         int
	StockChangeAction            string
	OutOfStockAction             string
	MinProfitRate                float64
	LowProfitAction              string
	PriceUpdateMultiplier        float64
	StockUpdateRatio             float64
	ActivityEnabled              bool
	ActivityType                 string
	ActivityDiscountRate         float64
	ActivityLimitedDiscountRate  float64
	ActivityStockRatio           float64
	PromotionRatio               float64
	ActivityMinProfitRate        float64
	ActivityLimitedMinProfitRate float64
	ActivityPriceMode            string
	ActivityPartakeType          string
	TimeLimitedDiscountRate      float64
	TimeLimitedMinProfitRate     float64
	TimeLimitedPriceMode         string
	TimeLimitedUserLimit         bool
	TimeLimitedUserLimitNum      int
	TimeLimitedStockLimit        bool
	TimeLimitedStockLimitPercent int
	FixedPriceAdjustment         float64
	PriceIncreaseThreshold       float64
	PriceDecreaseThreshold       float64
	PriceIncreaseAction          string
	PriceDecreaseAction          string
	RestoreStockAmount           int
	Remark                       string
}

func (s *OperationStrategy) IsEnabled() bool {
	return s != nil && s.Status == 0
}
