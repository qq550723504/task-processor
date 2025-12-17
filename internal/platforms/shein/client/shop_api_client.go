package client

import (
	"task-processor/internal/platforms/shein/client/api"
	"task-processor/internal/platforms/shein/client/impl"

	"github.com/imroc/req/v3"
)

// ShopAPIClient 实际的API客户端实现
type ShopAPIClient struct {
	*impl.BaseAPIClient
	*impl.ProductAPI
	*impl.TranslateAPI
	*impl.WarehouseAPI
	*impl.AttributeAPI
	*impl.CategoryAPI
	*impl.PricingAPI
	*impl.ImageAPI
	*impl.MarketingAPI
	*impl.OtherAPI
}

// NewShopAPIClient 创建新的店铺API客户端
func NewShopAPIClient(baseURL string, tenantID, shopID int64, httpClient *req.Client) *ShopAPIClient {
	// 创建基础客户端
	baseClient := impl.NewBaseAPIClient(baseURL, tenantID, shopID, httpClient)

	// 创建各个功能模块
	productAPI := impl.NewProductAPI(baseClient)
	translateAPI := impl.NewTranslateAPI(baseClient)
	warehouseAPI := impl.NewWarehouseAPI(baseClient)
	attributeAPI := impl.NewAttributeAPI(baseClient)
	categoryAPI := impl.NewCategoryAPI(baseClient)
	pricingAPI := impl.NewPricingAPI(baseClient)
	imageAPI := impl.NewImageAPI(baseClient)
	marketingAPI := impl.NewMarketingAPI(baseClient)
	otherAPI := impl.NewOtherAPI(baseClient)

	return &ShopAPIClient{
		BaseAPIClient: baseClient,
		ProductAPI:    productAPI,
		TranslateAPI:  translateAPI,
		WarehouseAPI:  warehouseAPI,
		AttributeAPI:  attributeAPI,
		CategoryAPI:   categoryAPI,
		PricingAPI:    pricingAPI,
		ImageAPI:      imageAPI,
		MarketingAPI:  marketingAPI,
		OtherAPI:      otherAPI,
	}
}

// 确保ShopAPIClient实现了api.APIClient接口
var _ api.APIClient = (*ShopAPIClient)(nil)
