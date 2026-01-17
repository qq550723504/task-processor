// Package api 产品数据API接口定义
package api

import (
	"task-processor/internal/pkg/types"
)

// ProductDataAPI 产品数据API接口定义
type ProductDataAPI interface {
	// BatchCreateOrUpdate 批量创建或更新产品数据
	BatchCreateOrUpdate(products []*ProductDataDTO) error

	// ListByStore 查询店铺的所有产品数据
	ListByStore(platform string, tenantID, storeID int64, shelfStatus *int) ([]*ProductDataDTO, error)

	// BatchUpdateAttributes 批量更新产品属性
	BatchUpdateAttributes(req *ProductDataBatchUpdateAttributesReqDTO) (int, error)

	// PageProductDataByStore 分页查询店铺产品数据
	PageProductDataByStore(req *ProductDataListByStorePageReqDTO) (*PageResult[*ProductDataRespDTO], error)
}

// ProductDataDTO 产品数据传输对象
type ProductDataDTO struct {
	// 基础字段
	ID              int64                `json:"id"`
	Source          string               `json:"source"`
	ImportTaskID    int64                `json:"import_task_id"`
	StoreID         int64                `json:"store_id"`
	Platform        string               `json:"platform"`
	CategoryID      int64                `json:"category_id"`
	Region          string               `json:"region"`
	ParentProductID string               `json:"parent_product_id"`
	ProductID       string               `json:"product_id"`
	Title           string               `json:"title"`
	Description     string               `json:"description"`
	OriginalPrice   types.FlexibleString `json:"original_price"`
	SpecialPrice    types.FlexibleString `json:"special_price"`
	PriceCurrency   string               `json:"price_currency"`
	Stock           types.FlexibleString `json:"stock"`
	Brand           string               `json:"brand"`
	Category        string               `json:"category"`
	MainImageURL    string               `json:"main_image_url"`
	ImageURLs       string               `json:"image_urls"`
	Attributes      string               `json:"attributes"`
	SourceURL       string               `json:"source_url"`
	Status          int16                `json:"status"`
	RawJSONDataID   int64                `json:"raw_json_data_id"`

	// 多平台扩展字段
	PlatformProductID string              `json:"platform_product_id"`
	PlatformStatus    string              `json:"platform_status"`
	ShelfStatus       int                 `json:"shelf_status"`
	PublishTime       *types.FlexibleTime `json:"publish_time"`
	ShelfTime         *types.FlexibleTime `json:"shelf_time"`
	LastSyncTime      *types.FlexibleTime `json:"last_sync_time"`
	PlatformData      string              `json:"platform_data"`

	// 租户字段
	TenantID   int64               `json:"tenant_id"`
	CreateTime *types.FlexibleTime `json:"create_time"`
	UpdateTime *types.FlexibleTime `json:"update_time"`
	Creator    string              `json:"creator"`
	Updater    string              `json:"updater"`
	Deleted    bool                `json:"deleted"`
}

// ShelfStatus 上架状态枚举
const (
	ShelfStatusPending   = 0 // 待上架
	ShelfStatusReviewing = 1 // 审核中
	ShelfStatusOnShelf   = 2 // 已上架
	ShelfStatusOffShelf  = 3 // 已下架
	ShelfStatusRejected  = 4 // 审核拒绝
	ShelfStatusDeleted   = 5 // 已删除
)

// ProductDataBatchUpdateAttributesReqDTO 批量更新产品属性请求DTO
type ProductDataBatchUpdateAttributesReqDTO struct {
	Platform string                     `json:"platform" validate:"required"`
	TenantID int64                      `json:"tenantId" validate:"required"`
	Region   string                     `json:"region"`
	StoreID  int64                      `json:"storeId" validate:"required"`
	Products []ProductAttributesItemDTO `json:"products" validate:"required,dive"`
}

// ProductAttributesItemDTO 产品属性项DTO
type ProductAttributesItemDTO struct {
	PlatformProductID string `json:"platformProductId" validate:"required"`
	Attributes        string `json:"attributes" validate:"required"`
	UpdateTime        *int64 `json:"updateTime"`
}

// ProductDataListByStorePageReqDTO 分页查询店铺产品数据请求DTO
type ProductDataListByStorePageReqDTO struct {
	Platform          string `json:"platform" validate:"required"`
	Region            string `json:"region"`
	TenantID          int64  `json:"tenantId" validate:"required"`
	StoreID           int64  `json:"storeId" validate:"required"`
	ShelfStatus       *int   `json:"shelfStatus"`
	Title             string `json:"title"`
	Brand             string `json:"brand"`
	Category          string `json:"category"`
	PlatformProductID string `json:"platformProductId"`
	PageNo            int    `json:"pageNo"`
	PageSize          int    `json:"pageSize"`
}

// ProductDataRespDTO 产品数据响应DTO
type ProductDataRespDTO struct {
	*ProductDataDTO
}

// PageResult 分页结果
type PageResult[T any] struct {
	List     []T   `json:"list"`
	Total    int64 `json:"total"`
	PageNo   int   `json:"pageNo"`
	PageSize int   `json:"pageSize"`
}

// CommonResult 通用响应结果
type CommonResult[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}
