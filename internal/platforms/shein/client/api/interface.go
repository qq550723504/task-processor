package api

import (
	"task-processor/internal/platforms/shein/client/api/attribute"
	"task-processor/internal/platforms/shein/client/api/category"
	"task-processor/internal/platforms/shein/client/api/image"
	"task-processor/internal/platforms/shein/client/api/marketing"
	"task-processor/internal/platforms/shein/client/api/other"
	"task-processor/internal/platforms/shein/client/api/pricing"
	"task-processor/internal/platforms/shein/client/api/product"
	"task-processor/internal/platforms/shein/client/api/translate"
	"task-processor/internal/platforms/shein/client/api/warehouse"
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
