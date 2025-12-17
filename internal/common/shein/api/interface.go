package api

import (
	"task-processor/internal/common/shein/api/attribute"
	"task-processor/internal/common/shein/api/category"
	"task-processor/internal/common/shein/api/image"
	"task-processor/internal/common/shein/api/other"
	"task-processor/internal/common/shein/api/pricing"
	"task-processor/internal/common/shein/api/product"
	"task-processor/internal/common/shein/api/translate"
	"task-processor/internal/common/shein/api/warehouse"
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
