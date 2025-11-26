package api

import (
	"task-processor/common/shein/api/attribute"
	"task-processor/common/shein/api/category"
	"task-processor/common/shein/api/image"
	"task-processor/common/shein/api/other"
	"task-processor/common/shein/api/pricing"
	"task-processor/common/shein/api/product"
	"task-processor/common/shein/api/translate"
	"task-processor/common/shein/api/warehouse"
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
