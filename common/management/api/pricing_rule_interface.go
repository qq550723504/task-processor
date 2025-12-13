package api

// PricingRuleRespDTO 自动核价规则响应DTO
type PricingRuleRespDTO struct {
	ID              int64    `json:"id"`          // 主键ID
	Name            string   `json:"name"`        // 规则名称
	RuleCode        string   `json:"ruleCode"`    // 规则编码
	Description     string   `json:"description"` // 规则描述
	StoreID         *int64   `json:"storeId"`     // 店铺ID（业务ID），为空时为通用规则
	CategoryID      *int64   `json:"categoryId"`  // 分类ID（业务ID），为空时为通用规则
	PriceMin        *float64 `json:"priceMin"`    // 最低价格（用于筛选产品）
	PriceMax        *float64 `json:"priceMax"`    // 最高价格（用于筛选产品）
	RuleType        string   `json:"ruleType"`    // 规则类型：multiple=倍率，fixed=固定值
	RuleValue       *float64 `json:"ruleValue"`   // 规则值：倍率或固定值
	FixedValue      *float64 `json:"fixedValue"`
	AcceptCondition string   `json:"acceptCondition"` // 接受条件：JSON格式，如{"priceRatio":">0.8"}
	RejectCondition string   `json:"rejectCondition"` // 拒绝条件：JSON格式，如{"priceRatio":"<=0.5"}
	Status          int16    `json:"status"`          // 状态：0-禁用，1-启用
	Remark          string   `json:"remark"`          // 备注
	CreateTime      int64    `json:"createTime"`      // 创建时间
	TenantID        int64    `json:"tenantId"`        // 租户ID

	// 利润率相关字段
	TargetProfitMargin float64 `json:"targetProfitMargin"` // 目标利润率（百分比）
	MinProfitMargin    float64 `json:"minProfitMargin"`    // 最低利润率（百分比）
	AcceptBelowTarget  bool    `json:"acceptBelowTarget"`  // 是否接受低于目标但高于最低的报价
	ReappealBelowMin   bool    `json:"reappealBelowMin"`   // 是否对低于最低的报价重新报价
}

// PricingRuleReqDTO 自动核价规则请求DTO
type PricingRuleReqDTO struct {
	TenantID   int64  `json:"tenantId,omitempty"`   // 租户ID
	StoreID    *int64 `json:"storeId,omitempty"`    // 店铺ID
	CategoryID *int64 `json:"categoryId,omitempty"` // 分类ID
}

// PricingRuleAPI 自动核价规则API接口定义
type PricingRuleAPI interface {
	// GetPricingRule 获取自动核价规则
	GetPricingRule(req *PricingRuleReqDTO) (*PricingRuleRespDTO, error)
}
