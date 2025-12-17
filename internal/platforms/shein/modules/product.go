package modules

import (
	"task-processor/internal/common/amazon/model"
	"task-processor/internal/common/shein/api/product"
)

// SKUCreationParams SKU创建参数
type SKUCreationParams struct {
	ASIN              string
	ProductInfo       *model.Product
	WarehouseCode     string
	SaleAttributeList []product.SaleAttribute
	Variant           Variant
}

// SKCCreationParams SKC创建参数
type SKCCreationParams struct {
	AttributeID      int
	AttributeValueID int
	SKUS             []product.SKU
	ImageInfo        product.ImageInfo
	SupplierCode     string
	Sort             int
}

type SKUBuildRequest struct {
	SaleAttributeData ResultSaleAttribute
	Strategy          AttributeStrategy
	PrimaryAttrValue  string
	WarehouseCode     string
}

type variantInfo struct {
	variant   Variant
	attrID    int
	valueID   int
	attrValue string // 添加属性值用于追踪和去重
}
