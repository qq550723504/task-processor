package api

// PricingRuleRespDTO 自动核价规则响应DTO
type PricingRuleRespDTO struct {
	ID                 int64    `json:"id"`
	Name               string   `json:"name"`
	RuleCode           string   `json:"ruleCode"`
	Description        *string  `json:"description"`
	StoreID            *int64   `json:"storeId"`
	CategoryID         *int64   `json:"categoryId"`
	PriceMin           *float64 `json:"priceMin"`
	PriceMax           *float64 `json:"priceMax"`
	RuleType           string   `json:"ruleType"`
	RuleValue          *float64 `json:"ruleValue"`
	FixedValue         *float64 `json:"fixedValue"`
	AcceptCondition    *string  `json:"acceptCondition"`
	RejectCondition    *string  `json:"rejectCondition"`
	Status             int      `json:"status"`
	Remark             *string  `json:"remark"`
	CreateTime         int64    `json:"createTime"`
	TenantID           int64    `json:"tenantId"`
	TargetProfitMargin float64  `json:"targetProfitMargin"`
	MinProfitMargin    float64  `json:"minProfitMargin"`
	AcceptBelowTarget  bool     `json:"acceptBelowTarget"`
	ReappealBelowMin   bool     `json:"reappealBelowMin"`
}

// PricingRuleReqDTO 自动核价规则请求DTO
type PricingRuleReqDTO struct {
	StoreID *int64 `json:"storeId,omitempty"`
}

// PricingRuleAPI 自动核价规则API接口定义
type PricingRuleAPI interface {
	GetPricingRule(req *PricingRuleReqDTO) ([]PricingRuleRespDTO, error)
}
