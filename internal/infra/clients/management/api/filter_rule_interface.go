package api

// FilterRuleInterface 筛选规则API接口定义
type FilterRuleInterface interface {
	GetFilterRule(req *FilterRuleReqDTO) (*[]FilterRuleRespDTO, error)
}

// FilterRuleReqDTO 筛选规则请求DTO
type FilterRuleReqDTO struct {
	StoreID    int64 `json:"storeId" binding:"required"`
	TenantID   int64 `json:"tenantId" binding:"omitempty"`
	CategoryID int64 `json:"categoryId" binding:"omitempty"`
}

// FilterRuleRespDTO 筛选规则响应DTO
type FilterRuleRespDTO struct {
	ID              int64    `json:"id"`
	Name            string   `json:"name"`
	RuleCode        string   `json:"ruleCode"`
	Description     string   `json:"description"`
	TenantID        int64    `json:"tenantId"`
	StoreID         int64    `json:"storeId"`
	CategoryID      int64    `json:"categoryId"`
	PriceMin        *float64 `json:"priceMin"`
	PriceMax        *float64 `json:"priceMax"`
	StockMin        *int     `json:"stockMin"`
	RatingMin       *float64 `json:"ratingMin"`
	ReviewCountMin  *int     `json:"reviewCountMin"`
	DeliveryTimeMax *int     `json:"deliveryTimeMax"`
	FulfillmentType string   `json:"fulfillmentType"`
	Status          int16    `json:"status"`
	Remark          string   `json:"remark"`
	CreateTime      int64    `json:"createTime"`
}

// FilterRuleAPI 筛选规则API接口（别名，用于兼容性）
type FilterRuleAPI = FilterRuleInterface
