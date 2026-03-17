package validation

// FilterRule 筛选规则值对象（domain 层自有定义，不依赖 infra）
type FilterRule struct {
	PriceMin        *float64
	PriceMax        *float64
	StockMin        *int
	RatingMin       *float64
	ReviewCountMin  *int
	DeliveryTimeMax *int
	FulfillmentType string
}
