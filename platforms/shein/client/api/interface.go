package api

import (
	"task-processor/platforms/shein/client/api/attribute"
	"task-processor/platforms/shein/client/api/category"
	"task-processor/platforms/shein/client/api/image"
	"task-processor/platforms/shein/client/api/marketing"
	"task-processor/platforms/shein/client/api/other"
	"task-processor/platforms/shein/client/api/pricing"
	"task-processor/platforms/shein/client/api/product"
	"task-processor/platforms/shein/client/api/translate"
	"task-processor/platforms/shein/client/api/warehouse"
)

// APIClient 店铺API客户端接口（组合所有子接口）
type APIClient interface {
	product.ProductAPI
	category.CategoryAPI
	attribute.AttributeAPI
	warehouse.WarehouseAPI
	translate.TranslateAPI
	pricing.PricingAPI
	image.ImageAPI
	marketing.MarketingAPI
	other.OtherAPI
}
