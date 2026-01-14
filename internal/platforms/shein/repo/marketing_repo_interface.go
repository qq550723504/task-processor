package repo

import (
	"task-processor/internal/platforms/shein/api/marketing"
)

// MarketingAPIInterface 营销API接口
type MarketingAPIInterface interface {
	// GetAvailableSkcList 获取可报名活动的产品列表
	GetAvailableSkcList(req *marketing.GetAvailableSkcListRequest) (*marketing.GetAvailableSkcListResponse, error)

	// SaveConfig 保存活动配置（报名活动）
	SaveConfig(req *marketing.SaveConfigRequest) (*marketing.SaveConfigResponse, error)

	// GetConfigList 获取已报名活动的产品列表
	GetConfigList(req *marketing.GetConfigListRequest) (*marketing.GetConfigListResponse, error)

	// QueryPromotionGoods 查询促销活动商品列表
	QueryPromotionGoods(req *marketing.QueryPromotionGoodsRequest) (*marketing.QueryPromotionGoodsResponse, error)

	// CalculateSupplyPrice 计算供货价格和利润
	CalculateSupplyPrice(req *marketing.CalculateSupplyPriceRequest) (*marketing.CalculateSupplyPriceResponse, error)

	// CreateActivity 创建促销活动
	CreateActivity(req *marketing.CreateActivityRequest) (*marketing.CreateActivityResponse, error)
}
