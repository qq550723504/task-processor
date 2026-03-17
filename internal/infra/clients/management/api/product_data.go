// Package api 产品数据API接口定义
package api

import "task-processor/internal/pkg/types"

// ProductDataAPI 产品数据API接口定义
type ProductDataAPI interface {
	BatchCreateOrUpdate(req *ProductDataBatchSaveReqDTO) (int, error)
	ListByStore(platform string, tenantID, storeID int64, shelfStatus *int) ([]*ProductDataDTO, error)
	BatchUpdateAttributes(req *ProductDataBatchUpdateAttributesReqDTO) (int, error)
	PageProductDataByStore(req *ProductDataListByStorePageReqDTO) (*PageResult[*ProductDataRespDTO], error)
}

// ProductDataDTO 产品数据传输对象
type ProductDataDTO struct {
	ID                int64                `json:"id"`
	Source            string               `json:"source"`
	ImportTaskID      int64                `json:"import_task_id"`
	StoreID           int64                `json:"store_id"`
	Platform          string               `json:"platform"`
	CategoryID        int64                `json:"category_id"`
	Region            string               `json:"region"`
	ParentProductID   string               `json:"parent_product_id"`
	ProductID         string               `json:"product_id"`
	Title             string               `json:"title"`
	Description       string               `json:"description"`
	OriginalPrice     types.FlexibleString `json:"original_price"`
	SpecialPrice      types.FlexibleString `json:"special_price"`
	PriceCurrency     string               `json:"price_currency"`
	Stock             types.FlexibleString `json:"stock"`
	Brand             string               `json:"brand"`
	Category          string               `json:"category"`
	MainImageURL      string               `json:"main_image_url"`
	ImageURLs         string               `json:"image_urls"`
	Attributes        string               `json:"attributes"`
	SourceURL         string               `json:"source_url"`
	Status            int16                `json:"status"`
	RawJSONDataID     int64                `json:"raw_json_data_id"`
	PlatformProductID string               `json:"platform_product_id"`
	PlatformStatus    string               `json:"platform_status"`
	ShelfStatus       int                  `json:"shelf_status"`
	PublishTime       *types.FlexibleTime  `json:"publish_time"`
	ShelfTime         *types.FlexibleTime  `json:"shelf_time"`
	LastSyncTime      *types.FlexibleTime  `json:"last_sync_time"`
	PlatformData      string               `json:"platform_data"`
	TenantID          int64                `json:"tenant_id"`
	CreateTime        *types.FlexibleTime  `json:"create_time"`
	UpdateTime        *types.FlexibleTime  `json:"update_time"`
	Creator           string               `json:"creator"`
	Updater           string               `json:"updater"`
	Deleted           bool                 `json:"deleted"`
}

// 上架状态枚举
const (
	ShelfStatusPending   = 0
	ShelfStatusReviewing = 1
	ShelfStatusOnShelf   = 2
	ShelfStatusOffShelf  = 3
	ShelfStatusRejected  = 4
	ShelfStatusDeleted   = 5
)

// ProductDataBatchSaveReqDTO 批量保存产品数据请求DTO
type ProductDataBatchSaveReqDTO struct {
	Platform string               `json:"platform" validate:"required"`
	TenantID int64                `json:"tenantId" validate:"required"`
	Region   string               `json:"region"`
	StoreID  int64                `json:"storeId" validate:"required"`
	Products []ProductDataItemDTO `json:"products" validate:"required,dive"`
}

// ProductDataItemDTO 产品数据项DTO
type ProductDataItemDTO struct {
	PlatformProductID  string               `json:"platformProductId" validate:"required"`
	ProductName        string               `json:"productName" validate:"required"`
	ProductSku         string               `json:"productSku"`
	ProductPrice       types.FlexibleString `json:"productPrice" validate:"required"`
	ProductStock       types.FlexibleString `json:"productStock" validate:"required"`
	ProductCategory    string               `json:"productCategory"`
	ProductImage       string               `json:"productImage"`
	ProductDescription string               `json:"productDescription"`
	ShelfStatus        *int                 `json:"shelfStatus"`
	PublishTime        *types.FlexibleTime  `json:"publishTime"`
	ShelfTime          *types.FlexibleTime  `json:"shelfTime"`
	Brand              string               `json:"brand"`
	CategoryID         *int64               `json:"categoryId"`
	SpecialPrice       types.FlexibleString `json:"specialPrice"`
	PriceCurrency      string               `json:"priceCurrency"`
	ImageUrls          string               `json:"imageUrls"`
	Attributes         string               `json:"attributes"`
	PlatformStatus     string               `json:"platformStatus"`
	PlatformData       string               `json:"platformData"`
	ParentProductID    string               `json:"parentProductId"`
	CreateTime         *types.FlexibleTime  `json:"createTime"`
	UpdateTime         *types.FlexibleTime  `json:"updateTime"`
}

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

// NewProductDataItemDTO 将 ProductDataDTO 转换为 ProductDataItemDTO
func NewProductDataItemDTO(prod *ProductDataDTO) ProductDataItemDTO {
	return ProductDataItemDTO{
		PlatformProductID:  prod.PlatformProductID,
		ProductName:        prod.Title,
		ProductSku:         prod.ProductID,
		ProductPrice:       prod.OriginalPrice,
		ProductStock:       prod.Stock,
		ProductCategory:    prod.Category,
		ProductImage:       prod.MainImageURL,
		ProductDescription: prod.Description,
		ShelfStatus:        &prod.ShelfStatus,
		PublishTime:        prod.PublishTime,
		ShelfTime:          prod.ShelfTime,
		Brand:              prod.Brand,
		CategoryID:         &prod.CategoryID,
		SpecialPrice:       prod.SpecialPrice,
		PriceCurrency:      prod.PriceCurrency,
		ImageUrls:          prod.ImageURLs,
		Attributes:         prod.Attributes,
		PlatformStatus:     prod.PlatformStatus,
		PlatformData:       prod.PlatformData,
		ParentProductID:    prod.ParentProductID,
		CreateTime:         prod.CreateTime,
		UpdateTime:         prod.UpdateTime,
	}
}

// NewProductDataBatchSaveReqDTO 构建批量保存请求
func NewProductDataBatchSaveReqDTO(prod *ProductDataDTO, items []ProductDataItemDTO) *ProductDataBatchSaveReqDTO {
	return &ProductDataBatchSaveReqDTO{
		Platform: prod.Platform,
		TenantID: prod.TenantID,
		Region:   prod.Region,
		StoreID:  prod.StoreID,
		Products: items,
	}
}
