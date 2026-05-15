package api

import "task-processor/internal/pkg/types"

// ProductImportMappingCreateReqDTO 产品导入映射关系创建请求DTO
type ProductImportMappingCreateReqDTO struct {
	ID                      *int64   `json:"id,omitempty"`
	TenantID                int64    `json:"tenantId"`
	ImportTaskId            int64    `json:"importTaskId"`
	StoreId                 int64    `json:"storeId"`
	Platform                string   `json:"platform"`
	Region                  string   `json:"region"`
	ProductId               string   `json:"productId"`
	Sku                     *string  `json:"sku,omitempty"`
	CostPrice               *float64 `json:"costPrice"`
	PlatformProductId       *string  `json:"platformProductId,omitempty"`
	ProfitRuleId            *int64   `json:"profitRuleId,omitempty"`
	SalePriceMultiplier     *string  `json:"salePriceMultiplier,omitempty"`
	DiscountPriceMultiplier *string  `json:"discountPriceMultiplier,omitempty"`
	Status                  *int16   `json:"status,omitempty"`
	Remark                  *string  `json:"remark,omitempty"`
	ParentProductId         *string  `json:"parentProductId,omitempty"`
	PlatformParentProductId *string  `json:"platformParentProductId,omitempty"`
	FilterRuleId            *int64   `json:"filterRuleId,omitempty"`
	FilterRuleRange         *string  `json:"filterRuleRange,omitempty"`
}

// ProductImportMappingGetReqDTO 通过平台产品ID获取映射关系请求DTO
type ProductImportMappingGetReqDTO struct {
	PlatformProductId string `json:"platformProductId"`
}

// ProductImportMappingCheckReqDTO 检查产品是否已上架请求DTO
type ProductImportMappingCheckReqDTO struct {
	StoreId   int64  `json:"storeId"`
	Platform  string `json:"platform"`
	Region    string `json:"region"`
	ProductId string `json:"productId"`
}

// ProductImportMappingRespDTO 产品导入映射关系响应DTO
type ProductImportMappingRespDTO struct {
	ID                      int64               `json:"id"`
	ImportTaskId            int64               `json:"importTaskId"`
	StoreId                 int64               `json:"storeId"`
	Platform                string              `json:"platform"`
	Region                  string              `json:"region"`
	ProductId               string              `json:"productId"`
	ParentProductId         *string             `json:"parentProductId"`
	PlatformProductId       *string             `json:"platformProductId"`
	PlatformParentProductId *string             `json:"platformParentProductId"`
	Sku                     *string             `json:"sku"`
	CostPrice               *float64            `json:"costPrice"`
	FilterRuleId            *int64              `json:"filterRuleId"`
	FilterRuleRange         *string             `json:"filterRuleRange"`
	ProfitRuleId            *int64              `json:"profitRuleId"`
	SalePriceMultiplier     *float64            `json:"salePriceMultiplier"`
	DiscountPriceMultiplier *float64            `json:"discountPriceMultiplier"`
	Status                  int16               `json:"status"`
	Remark                  *string             `json:"remark"`
	CreateTime              *types.FlexibleTime `json:"createTime"`
	TenantId                int64               `json:"tenantId"`
}

// ProductImportMappingGetBySkuReqDTO 通过SKU获取映射关系请求DTO
type ProductImportMappingGetBySkuReqDTO struct {
	Sku     string `json:"sku"`
	StoreId int64  `json:"storeId"`
}

// ProductImportMappingGetByTaskAndSkuReqDTO 根据任务ID和SKU查询映射关系请求DTO
type ProductImportMappingGetByTaskAndSkuReqDTO struct {
	ImportTaskId int64  `json:"importTaskId"`
	Sku          string `json:"sku"`
}

// ProductImportMappingGetByPlatformProductIdAndStoreReqDTO 通过平台产品ID和店铺ID获取映射关系请求DTO
type ProductImportMappingGetByPlatformProductIdAndStoreReqDTO struct {
	PlatformProductId string `json:"platformProductId"`
	StoreId           int64  `json:"storeId"`
}

// ProductImportMappingAPI 产品导入映射API接口定义
type ProductImportMappingAPI interface {
	CreateProductImportMapping(createReqDTO *ProductImportMappingCreateReqDTO) (int64, error)
	GetProductImportMappingByPlatformProductId(req *ProductImportMappingGetReqDTO) (*ProductImportMappingRespDTO, error)
	CheckProductExists(req *ProductImportMappingCheckReqDTO) (bool, error)
	GetProductImportMappingBySku(req *ProductImportMappingGetBySkuReqDTO) (*ProductImportMappingRespDTO, error)
	GetProductImportMappingByTaskAndSku(importTaskId int64, sku string) (*ProductImportMappingRespDTO, error)
	GetProductImportMappingByPlatformProductIdAndStore(req *ProductImportMappingGetByPlatformProductIdAndStoreReqDTO) (*ProductImportMappingRespDTO, error)
	UpdateProductImportMapping(updateReqDTO *ProductImportMappingCreateReqDTO) error
}
