package api

// ProfitRuleRespDTO 利润规则响应DTO
type ProfitRuleRespDTO struct {
	ID                      int64   `json:"id"`
	Name                    string  `json:"name"`
	RuleCode                string  `json:"ruleCode"`
	Description             string  `json:"description"`
	StoreID                 *int64  `json:"storeId,omitempty"`
	CategoryID              *int64  `json:"categoryId,omitempty"`
	SalePriceMultiplier     float64 `json:"salePriceMultiplier"`
	DiscountPriceMultiplier float64 `json:"discountPriceMultiplier,omitempty"`
	Status                  int16   `json:"status"`
	Remark                  string  `json:"remark"`
	CreateTime              int64   `json:"createTime"`
}

// ProfitRuleReqDTO 利润规则请求DTO
type ProfitRuleReqDTO struct {
	TenantID int64 `json:"tenantId" binding:"required"`
	StoreID  int64 `json:"storeId" binding:"omitemtp"`
}

// ProfitRuleAPI 利润规则API接口定义
type ProfitRuleAPI interface {
	GetProfitRule(req *ProfitRuleReqDTO) (*ProfitRuleRespDTO, error)
}
