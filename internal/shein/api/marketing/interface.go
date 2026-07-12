// Package marketing 提供SHEIN营销活动相关API接口定义
package marketing

// MarketingAPI 营销活动相关API接口
type MarketingAPI interface {
	// GetAvailableSkcList 获取可报名活动的产品列表
	GetAvailableSkcList(req *GetAvailableSkcListRequest) (*GetAvailableSkcListResponse, error)
	// SaveConfig 保存活动配置（报名活动）
	SaveConfig(req *SaveConfigRequest) (*SaveConfigResponse, error)
	// GetConfigList 获取已报名活动的产品列表
	GetConfigList(req *GetConfigListRequest) (*GetConfigListResponse, error)
	// UpdateConfigState 更新活动配置开关状态
	UpdateConfigState(req *UpdateConfigStateRequest) (*UpdateConfigStateResponse, error)
	// QueryPromotionGoods 查询促销活动商品列表
	QueryPromotionGoods(req *QueryPromotionGoodsRequest) (*QueryPromotionGoodsResponse, error)
	// CalculateSupplyPrice 计算供货价格和利润
	CalculateSupplyPrice(req *CalculateSupplyPriceRequest) (*CalculateSupplyPriceResponse, error)
	// CreateActivity 创建促销活动
	CreateActivity(req *CreateActivityRequest) (*CreateActivityResponse, error)
}

const (
	AutoPartakeActivityTypeRegular = 1
	AutoPartakeActivityTypeLimited = 2
	DefaultAutoPartakeActivityType = AutoPartakeActivityTypeRegular

	AutoPartakeConfigStateOpen   = 1
	AutoPartakeConfigStateClosed = 2
)

// GetAvailableSkcListRequest 获取可报名活动产品列表请求参数
type GetAvailableSkcListRequest struct {
	PageNum  int `json:"page_num"`  // 页码
	PageSize int `json:"page_size"` // 每页数量
}

// GetAvailableSkcListResponse 获取可报名活动产品列表响应
type GetAvailableSkcListResponse struct {
	Code string                `json:"code"` // 响应码
	Msg  string                `json:"msg"`  // 响应消息
	Info *AvailableSkcListInfo `json:"info"` // 响应数据
	BBL  any                   `json:"bbl"`  // BBL字段
}

// AvailableSkcListInfo 可报名活动产品列表信息
type AvailableSkcListInfo struct {
	Total       int       `json:"total"`         // 总数量
	SkcInfoList []SkcInfo `json:"skc_info_list"` // SKC信息列表
}

// SkcInfo SKC产品信息
type SkcInfo struct {
	Skc                       string             `json:"skc"`                                // SKC编码
	GoodsName                 string             `json:"goods_name"`                         // 商品名称
	Image                     string             `json:"image"`                              // 商品图片URL
	SupplierNo                string             `json:"supplier_no"`                        // 供应商编号
	Stock                     int                `json:"stock"`                              // 库存数量
	SupplyPrice               float64            `json:"supply_price"`                       // 供货价格
	SupplyPriceCurrency       string             `json:"supply_price_currency"`              // 供货价格币种
	IsConfigured              bool               `json:"is_configured"`                      // 是否已配置
	EnableNewPriceDisplay     bool               `json:"enable_new_price_display"`           // 是否启用新版价格展示
	GradeName                 string             `json:"grade_name"`                         // 商品等级名称
	ConfiguredTypeList        []int              `json:"configured_type_list"`               // 已配置活动类型列表
	SitePriceInfoList         []SitePriceInfo    `json:"site_price_info_list"`               // 站点价格信息列表
	SkuPriceInfoList          []SkuSitePriceInfo `json:"sku_price_info_list,omitempty"`      // SKU站点价格信息列表
	SkuCostPriceInfoList      []SkuCostPriceInfo `json:"sku_cost_price_info_list,omitempty"` // SKU成本价格信息列表
	UseLivePromotionSKUPrices bool               `json:"-"`                                  // 促销报名时以活动商品接口返回的SKU供货价计算折扣
}

// SitePriceInfo 站点价格信息
type SitePriceInfo struct {
	SiteCode    string  `json:"site_code"`    // 站点代码
	SalePrice   float64 `json:"sale_price"`   // 销售价格
	Currency    string  `json:"currency"`     // 币种
	IsAvailable bool    `json:"is_available"` // 是否可用
}

// SkuSitePriceInfo SKU站点价格信息
type SkuSitePriceInfo struct {
	SkuCode           string          `json:"sku_code"`             // SKU编码
	SitePriceInfoList []SitePriceInfo `json:"site_price_info_list"` // 站点价格信息列表
}

// SkuCostPriceInfo SKU成本价格信息
type SkuCostPriceInfo struct {
	SkuCode   string  `json:"sku_code"`   // SKU编码
	CostPrice float64 `json:"cost_price"` // 成本价格
	Currency  string  `json:"currency"`   // 币种
}

// SaveConfigRequest 保存活动配置请求参数
type SaveConfigRequest struct {
	ConfigList []ActivityConfig `json:"config_list"` // 配置列表
	Type       int              `json:"type"`        // 活动类型，常规为1，限量为2
}

func (r *SaveConfigRequest) ActivityType() int {
	if r.Type != 0 {
		return r.Type
	}
	return DefaultAutoPartakeActivityType
}

func (r *SaveConfigRequest) Payload() map[string]any {
	activityType := r.ActivityType()
	configs := make([]map[string]any, 0, len(r.ConfigList))
	for _, config := range r.ConfigList {
		sitePriceInfoList := config.SitePriceInfoList
		if sitePriceInfoList == nil {
			sitePriceInfoList = []ActivitySitePriceInfo{}
		}
		item := map[string]any{
			"skc":                  config.Skc,
			"drop_rate":            config.DropRate,
			"site_price_info_list": sitePriceInfoList,
		}
		if activityType == AutoPartakeActivityTypeLimited {
			item["act_stock"] = config.ActStock
		}
		configs = append(configs, item)
	}
	return map[string]any{
		"config_list": configs,
		"type":        activityType,
	}
}

// ActivityConfig 活动配置信息
type ActivityConfig struct {
	Skc               string                  `json:"skc"`                  // SKC编码
	ActStock          int                     `json:"act_stock"`            // 活动库存
	DropRate          int                     `json:"drop_rate"`            // 降价幅度（百分比）
	ReservedActStock  int                     `json:"reserved_act_stock"`   // 预留活动库存
	SitePriceInfoList []ActivitySitePriceInfo `json:"site_price_info_list"` // 站点价格信息列表
}

// ActivitySitePriceInfo 活动站点价格信息
type ActivitySitePriceInfo struct {
	SiteCode    string  `json:"site_code"`    // 站点代码
	SalePrice   float64 `json:"sale_price"`   // 销售价格
	Currency    string  `json:"currency"`     // 币种
	IsAvailable bool    `json:"is_available"` // 是否可用
}

// SaveConfigResponse 保存活动配置响应
type SaveConfigResponse struct {
	Code string `json:"code"` // 响应码
	Msg  string `json:"msg"`  // 响应消息
	Info any    `json:"info"` // 响应数据（通常为null）
	BBL  any    `json:"bbl"`  // BBL字段（通常为null）
}

// GetConfigListRequest 获取已报名活动产品列表请求参数
type GetConfigListRequest struct {
	PageNum  int `json:"page_num"`  // 页码
	PageSize int `json:"page_size"` // 每页数量
}

// GetConfigListResponse 获取已报名活动产品列表响应
type GetConfigListResponse struct {
	Code string          `json:"code"` // 响应码
	Msg  string          `json:"msg"`  // 响应消息
	Info *ConfigListInfo `json:"info"` // 响应数据
	BBL  any             `json:"bbl"`  // BBL字段
}

// ConfigListInfo 已报名活动产品列表信息
type ConfigListInfo struct {
	Total      int                  `json:"total"`       // 总数量
	ConfigList []ActivityConfigInfo `json:"config_list"` // 配置列表
}

// ActivityConfigInfo 已报名活动配置信息
type ActivityConfigInfo struct {
	ID                    int64                   `json:"id"`                       // 配置ID
	Skc                   string                  `json:"skc"`                      // SKC编码
	GoodsName             string                  `json:"goods_name"`               // 商品名称
	Image                 string                  `json:"image"`                    // 商品图片URL
	SupplierNo            string                  `json:"supplier_no"`              // 供应商编号
	SupplyPrice           float64                 `json:"supply_price"`             // 供货价格
	SupplyPriceCurrency   string                  `json:"supply_price_currency"`    // 供货价格币种
	DropRate              float64                 `json:"drop_rate"`                // 降价幅度（百分比）
	ActStock              int                     `json:"act_stock"`                // 活动库存
	ReservedActStock      int                     `json:"reserved_act_stock"`       // 预留活动库存
	State                 int                     `json:"state"`                    // 状态（1:正常）
	Stock                 int                     `json:"stock"`                    // 当前库存
	EnableNewPriceDisplay bool                    `json:"enable_new_price_display"` // 是否启用新版价格展示
	SitePriceInfoList     []ActivitySitePriceInfo `json:"site_price_info_list"`     // 站点价格信息列表
	ActivityConfigList    []ActivityConfigDetail  `json:"activity_config_list"`     // 新版活动配置列表
}

// ActivityConfigDetail 新版已报名活动配置详情
type ActivityConfigDetail struct {
	ID                   int64                   `json:"id"`                     // 配置ID
	ActivityType         int                     `json:"activity_type"`          // 活动类型
	DropRate             float64                 `json:"drop_rate"`              // 降价幅度（百分比）
	ActStock             *int                    `json:"act_stock"`              // 活动库存
	State                int                     `json:"state"`                  // 状态
	ConfigRuleType       int                     `json:"config_rule_type"`       // 配置规则类型
	SitePriceInfoList    []ActivitySitePriceInfo `json:"site_price_info_list"`   // 站点价格信息列表
	PartakeActivityInfos []any                   `json:"partake_activity_infos"` // 已参与活动信息
}

// UpdateConfigStateRequest 更新活动配置开关状态请求参数
type UpdateConfigStateRequest struct {
	IDs   []int64 `json:"ids"`   // 配置ID列表
	State int     `json:"state"` // 目标状态：1开启，2关闭
}

// UpdateConfigStateResponse 更新活动配置开关状态响应
type UpdateConfigStateResponse struct {
	Code string `json:"code"` // 响应码
	Msg  string `json:"msg"`  // 响应消息
	Info any    `json:"info"` // 响应数据（通常为null）
	BBL  any    `json:"bbl"`  // BBL字段（通常为null）
}
