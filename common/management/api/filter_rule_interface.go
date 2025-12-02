package api

type FilterRuleInterface interface {
	GetFilterRule(req *FilterRuleReqDTO) (*[]FilterRuleRespDTO, error)
}

type FilterRuleReqDTO struct {
	StoreID    int64 `json:"storeId" binding:"required"`
	TenantID   int64 `json:"tenantId" binding:"omitempty"`
	CategoryID int64 `json:"categoryId" binding:"omitempty"`
}

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
	FulfillmentType string   `json:"fulfillmentType"` // 配送方式：FBA-亚马逊配送，FBM-商家配送，ALL-都可以
	Status          int16    `json:"status"`          // 状态：1-禁用，0-启用
	Remark          string   `json:"remark"`          // 备注
	CreateTime      int64    `json:"createTime"`      // 创建时间
}

// FilterRuleAPI 筛选规则API接口（别名，用于兼容性）
type FilterRuleAPI = FilterRuleInterface
