package product

// CostPriceQueryRequest 成本价格查询请求（半托店铺）
type CostPriceQueryRequest struct {
	SpuName     string   `json:"spu_name"`
	SkcNameList []string `json:"skc_name_list"`
}

// CostPriceQueryResponse 成本价格查询响应
type CostPriceQueryResponse struct {
	Code string        `json:"code"`
	Msg  string        `json:"msg"`
	Info CostPriceInfo `json:"info"`
	BBL  any           `json:"bbl"`
}

// CostPriceInfo 成本价格信息
type CostPriceInfo struct {
	Data []SkcCostData `json:"data"`
	Meta struct {
		Count     int `json:"count"`
		CustomObj any `json:"customObj"`
	} `json:"meta"`
}

// SkcCostData SKC 成本数据
type SkcCostData struct {
	SkcName              string             `json:"skc_name"`
	ActLockInfo          any                `json:"act_lock_info"`
	SortOrder            int                `json:"sort_order"`
	SaleAttribute        CostSaleAttribute  `json:"sale_attribute"`
	SkuCostInfoList      []SkuCostInfo      `json:"sku_cost_info_list"`
	ChangeCostReasonList []ChangeCostReason `json:"change_cost_reason_list"`
	CanRaisePrice        int                `json:"can_raise_price"`
	Deadline             any                `json:"deadline"`
}

// CostSaleAttribute 成本销售属性
type CostSaleAttribute struct {
	AttributeID         any    `json:"attribute_id"`
	AttributeValueID    any    `json:"attribute_value_id"`
	AttributeMulti      string `json:"attribute_multi"`
	AttributeValueMulti string `json:"attribute_value_multi"`
}

// SkuCostInfo SKU 成本信息
type SkuCostInfo struct {
	SkuCode           string              `json:"sku_code"`
	SaleAttributeList []SaleAttributeItem `json:"sale_attribute_list"`
	CostPriceInfo     CostPrice           `json:"cost_price_info"`
	CanChangeCost     bool                `json:"can_change_cost"`
	FailReasonFlag    any                 `json:"fail_reason_flag"`
	FailReasonContent any                 `json:"fail_reason_content"`
}

// SaleAttributeItem 销售属性项
type SaleAttributeItem struct {
	AttributeID         int    `json:"attribute_id"`
	AttributeValueID    int    `json:"attribute_value_id"`
	AttributeMulti      string `json:"attribute_multi"`
	AttributeValueMulti string `json:"attribute_value_multi"`
}

// CostPrice 成本价格
type CostPrice struct {
	CostPrice string `json:"cost_price"`
	Currency  string `json:"currency"`
}

// ChangeCostReason 成本变更原因
type ChangeCostReason struct {
	ReasonContent string `json:"reason_content"`
	ReasonFlag    int    `json:"reason_flag"`
}
