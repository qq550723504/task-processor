package api

// ProfitRuleRespDTO 利润规则响应DTO
type ProfitRuleRespDTO struct {
	ID                      int64   `json:"id"`                                // 主键ID
	Name                    string  `json:"name"`                              // 规则名称
	RuleCode                string  `json:"ruleCode"`                          // 规则编码
	Description             string  `json:"description"`                       // 规则描述
	StoreID                 *int64  `json:"storeId,omitempty"`                 // 店铺ID（业务ID），为空时为通用规则
	CategoryID              *int64  `json:"categoryId,omitempty"`              // 分类ID（业务ID），为空时为通用规则
	SalePriceMultiplier     float64 `json:"salePriceMultiplier"`               // 售价倍数，用于自动上架时设置价格
	DiscountPriceMultiplier float64 `json:"discountPriceMultiplier,omitempty"` // 折扣价倍数，用于自动上架时设置价格
	Status                  int16   `json:"status"`                            // 状态：0-禁用，1-启用
	Remark                  string  `json:"remark"`                            // 备注
	CreateTime              int64   `json:"createTime"`                        // 创建时间
}

// ProfitRuleReqDTO 利润规则请求DTO
type ProfitRuleReqDTO struct {
	TenantID int64 `json:"tenantId" binding:"required"` // 租户ID
	StoreID  int64 `json:"storeId" binding:"omitemtp"`  // 店铺ID
}

// ProfitRuleAPI 利润规则API接口定义
type ProfitRuleAPI interface {
	// GetProfitRule 获取利润规则
	GetProfitRule(req *ProfitRuleReqDTO) (*ProfitRuleRespDTO, error)
}
