package api

// PricingRuleRespDTO 自动核价规则响应DTO
type PricingRuleRespDTO struct {
	ID              int64    `json:"id"`              // 主键ID
	Name            string   `json:"name"`            // 规则名称
	RuleCode        string   `json:"ruleCode"`        // 规则编码
	Description     string   `json:"description"`     // 规则描述
	StoreID         *int64   `json:"storeId"`         // 店铺ID（业务ID），为空时为通用规则
	CategoryID      *int64   `json:"categoryId"`      // 分类ID（业务ID），为空时为通用规则
	PriceMin        *float64 `json:"priceMin"`        // 最低价格（用于筛选产品）
	PriceMax        *float64 `json:"priceMax"`        // 最高价格（用于筛选产品）
	RuleType        string   `json:"ruleType"`        // 规则类型：multiple=倍率，fixed=固定值
	RuleValue       *float64 `json:"ruleValue"`       // 规则值：倍率或固定值
	AcceptCondition string   `json:"acceptCondition"` // 接受条件：JSON格式，如{"priceRatio":">0.8"}
	RejectCondition string   `json:"rejectCondition"` // 拒绝条件：JSON格式，如{"priceRatio":"<=0.5"}
	Status          int16    `json:"status"`          // 状态：0-禁用，1-启用
	Remark          string   `json:"remark"`          // 备注
	CreateTime      int64    `json:"createTime"`      // 创建时间
	TenantID        int64    `json:"tenantId"`        // 租户ID
}

// PricingRuleReqDTO 自动核价规则请求DTO
type PricingRuleReqDTO struct {
	TenantID   int64  `json:"tenantId" binding:"required"` // 租户ID
	StoreID    *int64 `json:"storeId,omitempty"`           // 店铺ID
	CategoryID *int64 `json:"categoryId,omitempty"`        // 分类ID
}

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

// FilterRuleReqDTO 过滤规则请求DTO
type FilterRuleReqDTO struct {
	StoreID    int64 `json:"storeId" binding:"required"`
	TenantID   int64 `json:"tenantId" binding:"omitempty"`
	CategoryID int64 `json:"categoryId" binding:"omitempty"`
}

// FilterRuleRespDTO 过滤规则响应DTO
type FilterRuleRespDTO struct {
	ID              int64    `json:"id"`              // 主键ID
	Name            string   `json:"name"`            // 规则名称
	RuleCode        string   `json:"ruleCode"`        // 规则编码
	Description     string   `json:"description"`     // 规则描述
	TenantID        int64    `json:"tenantId"`        // 租户编号
	StoreID         int64    `json:"storeId"`         // 店铺ID（业务ID），为空时为通用规则
	CategoryID      int64    `json:"categoryId"`      // 分类ID
	PriceType       string   `json:"priceType"`       // 价格类型
	PriceMin        *float64 `json:"priceMin"`        // 最低价格
	PriceMax        *float64 `json:"priceMax"`        // 最高价格
	StockMin        *int     `json:"stockMin"`        // 最低库存
	RatingMin       *float64 `json:"ratingMin"`       // 最低评分
	ReviewCountMin  *int     `json:"reviewCountMin"`  // 最低评论数量
	DeliveryTimeMax *int     `json:"deliveryTimeMax"` // 最大发货时效（小时）
	Status          int16    `json:"status"`          // 状态：0-禁用，1-启用
	Remark          string   `json:"remark"`          // 备注
	CreateTime      int64    `json:"createTime"`      // 创建时间
}

// PricingRuleAPI 自动核价规则API接口定义
type PricingRuleAPI interface {
	// GetPricingRule 获取自动核价规则
	GetPricingRule(req *PricingRuleReqDTO) (*PricingRuleRespDTO, error)
}

// ProfitRuleAPI 利润规则API接口定义
type ProfitRuleAPI interface {
	// GetProfitRule 获取利润规则
	GetProfitRule(req *ProfitRuleReqDTO) (*ProfitRuleRespDTO, error)
}

// FilterRuleAPI 过滤规则API接口定义
type FilterRuleAPI interface {
	// GetFilterRule 获取过滤规则
	GetFilterRule(req *FilterRuleReqDTO) (*[]FilterRuleRespDTO, error)
}
