package api

import (
	"task-processor/internal/platforms/shein/api/attribute"
	"task-processor/internal/platforms/shein/api/category"
	"task-processor/internal/platforms/shein/api/image"
	"task-processor/internal/platforms/shein/api/other"
	"task-processor/internal/platforms/shein/api/pricing"
	"task-processor/internal/platforms/shein/api/product"
	"task-processor/internal/platforms/shein/api/translate"
	"task-processor/internal/platforms/shein/api/warehouse"
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
	other.OtherAPI
}
