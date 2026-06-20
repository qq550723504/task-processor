package listingruntime

import "task-processor/internal/product"

type FilterRule struct {
	ID              int64
	Name            string
	TenantID        int64
	StoreID         int64
	CategoryID      int64
	PriceType       string
	PriceMin        *float64
	PriceMax        *float64
	StockMin        *int
	RatingMin       *float64
	ReviewCountMin  *int
	DeliveryTimeMax *int
	FulfillmentType string
	Status          int16
}

func (r *FilterRule) ToFilterRule() *product.FilterRule {
	if r == nil {
		return nil
	}
	return &product.FilterRule{
		PriceMin:        r.PriceMin,
		PriceMax:        r.PriceMax,
		StockMin:        r.StockMin,
		RatingMin:       r.RatingMin,
		ReviewCountMin:  r.ReviewCountMin,
		DeliveryTimeMax: r.DeliveryTimeMax,
		FulfillmentType: r.FulfillmentType,
	}
}

type ProfitRule struct {
	ID                      int64
	Name                    string
	TenantID                int64
	StoreID                 *int64
	CategoryID              *int64
	SalePriceMultiplier     float64
	DiscountPriceMultiplier float64
	Status                  int16
}

type PricingRule struct {
	ID                 int64
	Name               string
	RuleCode           string
	Description        *string
	StoreID            *int64
	CategoryID         *int64
	PriceMin           *float64
	PriceMax           *float64
	RuleType           string
	RuleValue          *float64
	FixedValue         *float64
	AcceptCondition    *string
	RejectCondition    *string
	Status             int
	Remark             *string
	TenantID           int64
	TargetProfitMargin float64
	MinProfitMargin    float64
	AcceptBelowTarget  bool
	ReappealBelowMin   bool
}
