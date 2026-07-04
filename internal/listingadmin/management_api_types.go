package listingadmin

import (
	"context"
	"io"

	"task-processor/internal/pkg/types"
	"task-processor/internal/product"
)

// DailyListingCountGetReqDTO 获取每日上架数量请求DTO
type DailyListingCountGetReqDTO struct {
	TenantID int64  `json:"tenantId"`
	StoreID  int64  `json:"storeId"`
	UserID   int64  `json:"userId"`
	Date     string `json:"date"`
}

// DailyListingCountSetReqDTO 设置每日上架数量请求DTO
type DailyListingCountSetReqDTO struct {
	TenantID int64  `json:"tenantId"`
	StoreID  int64  `json:"storeId"`
	UserID   int64  `json:"userId"`
	Date     string `json:"date"`
	Count    int64  `json:"count"`
}

// DailyListingCountRespDTO 每日上架数量响应DTO
type DailyListingCountRespDTO struct {
	TenantID int64  `json:"tenantId"`
	StoreID  int64  `json:"storeId"`
	UserID   int64  `json:"userId"`
	Date     string `json:"date"`
	Count    int64  `json:"count"`
}

// TryConsumeDailyQuotaReqDTO 原子预占每日上架额度请求DTO
type TryConsumeDailyQuotaReqDTO struct {
	TenantID  int64  `json:"tenantId"`
	StoreID   int64  `json:"storeId"`
	UserID    int64  `json:"userId"`
	Date      string `json:"date"`
	Increment int64  `json:"increment"`
	Limit     int64  `json:"limit"`
}

// TryConsumeDailyQuotaRespDTO 原子预占每日上架额度响应DTO
type TryConsumeDailyQuotaRespDTO struct {
	Allowed      bool  `json:"allowed"`
	NewCount     int64 `json:"newCount"`
	Remaining    int64 `json:"remaining"`
	ReachedLimit bool  `json:"reachedLimit"`
}

// RollbackDailyQuotaReqDTO 回滚每日上架额度请求DTO
type RollbackDailyQuotaReqDTO struct {
	TenantID  int64  `json:"tenantId"`
	StoreID   int64  `json:"storeId"`
	UserID    int64  `json:"userId"`
	Date      string `json:"date"`
	Decrement int64  `json:"decrement"`
}

// DailyListingCountAPI 每日上架数量API接口定义
type DailyListingCountAPI interface {
	GetDailyListingCount(tenantID, storeID, userID int64, date string) (*DailyListingCountRespDTO, error)
	SetDailyListingCount(req *DailyListingCountSetReqDTO) error
	TryConsumeDailyQuota(req *TryConsumeDailyQuotaReqDTO) (*TryConsumeDailyQuotaRespDTO, error)
	RollbackDailyQuota(req *RollbackDailyQuotaReqDTO) (int64, error)
	SetRemainingListingQuota(tenantID, storeID int64, quota int) (bool, error)
}

// StoreRespDTO 店铺信息响应DTO
type StoreRespDTO struct {
	ID                       int64               `json:"id"`
	TenantID                 int64               `json:"tenantId"`
	StoreID                  string              `json:"storeId"`
	Name                     string              `json:"name"`
	Username                 string              `json:"username"`
	Password                 string              `json:"password"`
	LoginUrl                 string              `json:"loginUrl"`
	ShopType                 string              `json:"shopType"`
	Region                   string              `json:"region"`
	Platform                 string              `json:"platform"`
	DailyLimit               *int                `json:"dailyLimit,omitempty"`
	DailyLimitType           string              `json:"dailyLimitType,omitempty"`
	FixedStockCount          *int                `json:"fixedStockCount,omitempty"`
	SkuGenerateStrategy      string              `json:"skuGenerateStrategy"`
	Prefix                   string              `json:"prefix"`
	Suffix                   string              `json:"suffix"`
	Proxy                    string              `json:"proxy"`
	EnableAutoListing        *bool               `json:"enableAutoListing,omitempty"`
	DedicatedQueueEnabled    *bool               `json:"dedicatedQueueEnabled,omitempty"`
	EnableAutoLogin          *bool               `json:"enableAutoLogin,omitempty"`
	EnableDraft              *bool               `json:"enableDraft,omitempty"`
	EnableAutoPrice          *bool               `json:"enableAutoPrice,omitempty"`
	EnableRebargain          *bool               `json:"enableRebargain,omitempty"`
	EnableBrandAuthorization *bool               `json:"enableBrandAuthorization,omitempty"`
	AuthorizedBrandCode      string              `json:"authorizedBrandCode,omitempty"`
	AuthorizedBrandName      string              `json:"authorizedBrandName,omitempty"`
	TemuPriceRejectStrategy  string              `json:"temuPriceRejectStrategy,omitempty"`
	PriceType                string              `json:"priceType,omitempty"`
	Remark                   string              `json:"remark"`
	Status                   int16               `json:"status"`
	CreateTime               *types.FlexibleTime `json:"createTime"`
	Creator                  string              `json:"creator"`
}

// StorePageReqDTO 分页查询店铺请求
type StorePageReqDTO struct {
	Platform        string `json:"platform,omitempty"`
	TenantID        int64  `json:"tenantId,omitempty"`
	PageNo          int    `json:"pageNo"`
	PageSize        int    `json:"pageSize"`
	EnableAutoPrice *bool  `json:"enableAutoPrice,omitempty"`
}

// StoreStatusUpdateReqDTO 店铺状态更新请求DTO
type StoreStatusUpdateReqDTO struct {
	ID     int64  `json:"id"`
	Status int16  `json:"status"`
	Remark string `json:"remark,omitempty"`
}

// StoreIdUpdateReqDTO 修改店铺StoreID请求DTO
type StoreIdUpdateReqDTO struct {
	ID      int64  `json:"id"`
	StoreID string `json:"storeId"`
}

// StorePauseStatusRespDTO 店铺暂停状态详情
type StorePauseStatusRespDTO struct {
	Paused     bool   `json:"paused"`
	PauseType  string `json:"pauseType"`
	Reason     string `json:"reason"`
	PausedAt   int64  `json:"pausedAt"`
	PauseUntil int64  `json:"pauseUntil"`
	TTLSeconds int64  `json:"ttlSeconds"`
}

// StoreAPI 店铺管理API接口定义
type StoreAPI interface {
	GetStore(id int64) (*StoreRespDTO, error)
	PageStores(req *StorePageReqDTO) (*PageResult[*StoreRespDTO], error)
	GetStoreCookie(id int64) (string, error)
	UpdateStoreId(req *StoreIdUpdateReqDTO) (bool, error)
	UpdateStoreStatus(req *StoreStatusUpdateReqDTO) (bool, error)
	DeleteStoreCookie(id int64) (bool, error)
	SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error)
	GetStorePauseStatus(id int64) (bool, error)
	GetStorePauseStatusDetail(id int64) (*StorePauseStatusRespDTO, error)
}

// FilterRuleReqDTO 筛选规则请求DTO
type FilterRuleReqDTO struct {
	StoreID    int64 `json:"storeId" binding:"required"`
	TenantID   int64 `json:"tenantId" binding:"omitempty"`
	CategoryID int64 `json:"categoryId" binding:"omitempty"`
}

// FilterRuleRespDTO 筛选规则响应DTO
type FilterRuleRespDTO struct {
	ID              int64              `json:"id"`
	Name            string             `json:"name"`
	RuleCode        string             `json:"ruleCode"`
	Description     string             `json:"description"`
	TenantID        int64              `json:"tenantId"`
	StoreID         int64              `json:"storeId"`
	PriceType       string             `json:"priceType"`
	CategoryID      int64              `json:"categoryId"`
	PriceMin        *float64           `json:"priceMin"`
	PriceMax        *float64           `json:"priceMax"`
	StockMin        *int               `json:"stockMin"`
	RatingMin       *float64           `json:"ratingMin"`
	ReviewCountMin  *int               `json:"reviewCountMin"`
	DeliveryTimeMax *int               `json:"deliveryTimeMax"`
	FulfillmentType string             `json:"fulfillmentType"`
	Status          int16              `json:"status"`
	Remark          string             `json:"remark"`
	CreateTime      types.FlexibleTime `json:"createTime"`
}

// FilterRuleAPI 筛选规则API接口定义
type FilterRuleAPI interface {
	GetFilterRule(req *FilterRuleReqDTO) (*[]FilterRuleRespDTO, error)
}

// ToFilterRule 将 DTO 转换为 domain 层的 FilterRule 值对象
func (r *FilterRuleRespDTO) ToFilterRule() *product.FilterRule {
	return &product.FilterRule{
		PriceMin:        r.PriceMin,
		PriceMax:        r.PriceMax,
		StockMin:        r.StockMin,
		RatingMin:       r.RatingMin,
		ReviewCountMin:  r.ReviewCountMin,
		DeliveryTimeMax: r.DeliveryTimeMax,
		FulfillmentType: r.FulfillmentType,
	}
}

// ProfitRuleRespDTO 利润规则响应DTO
type ProfitRuleRespDTO struct {
	ID                      int64              `json:"id"`
	Name                    string             `json:"name"`
	RuleCode                string             `json:"ruleCode"`
	Description             string             `json:"description"`
	StoreID                 *int64             `json:"storeId,omitempty"`
	CategoryID              *int64             `json:"categoryId,omitempty"`
	SalePriceMultiplier     float64            `json:"salePriceMultiplier"`
	DiscountPriceMultiplier float64            `json:"discountPriceMultiplier,omitempty"`
	Status                  int16              `json:"status"`
	Remark                  string             `json:"remark"`
	CreateTime              types.FlexibleTime `json:"createTime"`
	TenantID                int64              `json:"tenantId"`
}

// ProfitRuleReqDTO 利润规则请求DTO
type ProfitRuleReqDTO struct {
	TenantID int64 `json:"tenantId" binding:"required"`
	StoreID  int64 `json:"storeId" binding:"omitemtp"`
}

// ProfitRuleAPI 利润规则API接口定义
type ProfitRuleAPI interface {
	GetProfitRule(req *ProfitRuleReqDTO) (*ProfitRuleRespDTO, error)
}

// PricingRuleRespDTO 自动核价规则响应DTO
type PricingRuleRespDTO struct {
	ID                 int64              `json:"id"`
	Name               string             `json:"name"`
	RuleCode           string             `json:"ruleCode"`
	Description        *string            `json:"description"`
	StoreID            *int64             `json:"storeId"`
	CategoryID         *int64             `json:"categoryId"`
	PriceMin           *float64           `json:"priceMin"`
	PriceMax           *float64           `json:"priceMax"`
	RuleType           string             `json:"ruleType"`
	RuleValue          *float64           `json:"ruleValue"`
	FixedValue         *float64           `json:"fixedValue"`
	AcceptCondition    *string            `json:"acceptCondition"`
	RejectCondition    *string            `json:"rejectCondition"`
	Status             int                `json:"status"`
	Remark             *string            `json:"remark"`
	CreateTime         types.FlexibleTime `json:"createTime"`
	TenantID           int64              `json:"tenantId"`
	TargetProfitMargin float64            `json:"targetProfitMargin"`
	MinProfitMargin    float64            `json:"minProfitMargin"`
	AcceptBelowTarget  bool               `json:"acceptBelowTarget"`
	ReappealBelowMin   bool               `json:"reappealBelowMin"`
}

// PricingRuleReqDTO 自动核价规则请求DTO
type PricingRuleReqDTO struct {
	StoreID *int64 `json:"storeId,omitempty"`
}

// PricingRuleAPI 自动核价规则API接口定义
type PricingRuleAPI interface {
	GetPricingRule(req *PricingRuleReqDTO) ([]PricingRuleRespDTO, error)
}

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

// InventoryRecordCreateReqDTO 库存记录创建请求DTO
type InventoryRecordCreateReqDTO struct {
	Platform           string   `json:"platform" binding:"required"`
	ProductId          string   `json:"productId" binding:"required"`
	Region             string   `json:"region" binding:"required"`
	Stock              *int     `json:"stock"`
	StockStatus        string   `json:"stockStatus"`
	IsAvailable        bool     `json:"isAvailable" binding:"required"`
	OriginalPrice      *float64 `json:"originalPrice"`
	CurrentPrice       *float64 `json:"currentPrice"`
	Currency           string   `json:"currency"`
	PriceChangePercent *float64 `json:"priceChangePercent"`
	SyncSource         string   `json:"syncSource"`
	Remark             string   `json:"remark"`
}

// InventoryRecordRespDTO 库存记录响应DTO
type InventoryRecordRespDTO struct {
	ID                 int64              `json:"id"`
	Platform           string             `json:"platform"`
	ProductId          string             `json:"productId"`
	Region             string             `json:"region"`
	Stock              *int               `json:"stock"`
	StockStatus        string             `json:"stockStatus"`
	IsAvailable        bool               `json:"isAvailable"`
	OriginalPrice      *float64           `json:"originalPrice"`
	CurrentPrice       *float64           `json:"currentPrice"`
	Currency           string             `json:"currency"`
	PriceChangePercent *float64           `json:"priceChangePercent"`
	SyncSource         string             `json:"syncSource"`
	Remark             string             `json:"remark"`
	CreateTime         types.FlexibleTime `json:"createTime"`
}

// InventoryRecordAPI 库存记录API接口定义
type InventoryRecordAPI interface {
	CreateInventoryRecord(req *InventoryRecordCreateReqDTO) (int64, error)
	GetLatestInventoryRecord(platform, productId, region string) (*InventoryRecordRespDTO, error)
}

// RawJsonDataReqDTO 原始JSON数据请求DTO
type RawJsonDataReqDTO struct {
	TenantID   int64  `json:"tenantId" binding:"required"`
	Platform   string `json:"platform" binding:"required"`
	ProductID  string `json:"productId" binding:"required"`
	Region     string `json:"region" binding:"required"`
	StoreID    int64  `json:"storeId" binding:"required"`
	CategoryID int64  `json:"categoryId" binding:"required"`
	Creator    string `json:"creator" binding:"required"`
}

// RawJsonDataRespDTO 原始JSON数据响应DTO
type RawJsonDataRespDTO struct {
	ID          int64              `json:"id"`
	TaskID      int64              `json:"taskId"`
	Platform    string             `json:"platform"`
	ProductID   string             `json:"productId"`
	Region      string             `json:"region"`
	RawJSONData string             `json:"rawJsonData"`
	CreateTime  types.FlexibleTime `json:"createTime"`
	UpdateTime  types.FlexibleTime `json:"updateTime"`
}

// ProductVariantConfirmationReqDTO 产品变体确认请求DTO
type ProductVariantConfirmationReqDTO struct {
	ProductID  string   `json:"productId" binding:"required"`
	Platform   string   `json:"platform" binding:"required"`
	Region     string   `json:"region" binding:"required"`
	VariantIds []string `json:"variantIds" binding:"required"`
}

// RawJsonDataCreateReqDTO 原始JSON数据创建请求DTO
type RawJsonDataCreateReqDTO struct {
	TenantID     int64  `json:"tenantId"`
	StoreID      int64  `json:"storeId"`
	ImportTaskID int64  `json:"importTaskId"`
	Platform     string `json:"platform"`
	Region       string `json:"region"`
	ProductID    string `json:"productId"`
	CategoryID   int64  `json:"categoryId"`
	RawJsonData  string `json:"rawJsonData"`
	Creator      string `json:"creator"`
}

// RawJsonDataAPI 原始JSON数据API接口定义
type RawJsonDataAPI interface {
	GetRawJsonData(req *RawJsonDataReqDTO) (*RawJsonDataRespDTO, error)
	CreateRawJsonData(req *RawJsonDataCreateReqDTO) (int64, error)
}

// ImageDownloader 图片下载客户端接口
type ImageDownloader interface {
	DownloadImage(url string) ([]byte, error)
	DownloadImageToWriter(ctx context.Context, url string, writer io.Writer) error
	GetImageInfo(ctx context.Context, url string) (*ImageInfo, error)
}

// ImageInfo 图片信息结构
type ImageInfo struct {
	Size     int64
	Format   string
	Width    int
	Height   int
	MimeType string
}

// OperationStrategyAPI 自动化运营策略 API 接口
type OperationStrategyAPI interface {
	GetOperationStrategyByStoreId(storeId int64) (*OperationStrategyDTO, error)
}

// OperationStrategyDTO 自动化运营策略 DTO
type OperationStrategyDTO struct {
	ID                           int64                `json:"id"`
	TenantID                     int64                `json:"tenantId"`
	StoreID                      int64                `json:"storeId"`
	Name                         string               `json:"name"`
	Platform                     string               `json:"platform"`
	Status                       int16                `json:"status"`
	StockChangeThreshold         int                  `json:"stockChangeThreshold"`
	StockChangeAction            string               `json:"stockChangeAction"`
	OutOfStockAction             string               `json:"outOfStockAction"`
	MinProfitRate                float64              `json:"minProfitRate"`
	LowProfitAction              string               `json:"lowProfitAction"`
	PriceUpdateMultiplier        float64              `json:"priceUpdateMultiplier"`
	StockUpdateRatio             float64              `json:"stockUpdateRatio"`
	ActivityEnabled              bool                 `json:"activityEnabled"`
	ActivityType                 string               `json:"activityType"`
	ActivityDiscountRate         float64              `json:"activityDiscountRate"`
	ActivityLimitedDiscountRate  float64              `json:"activityLimitedDiscountRate"`
	ActivityStockRatio           float64              `json:"activityStockRatio"`
	PromotionRatio               float64              `json:"promotionRatio"`
	ActivityMinProfitRate        float64              `json:"activityMinProfitRate"`
	ActivityLimitedMinProfitRate float64              `json:"activityLimitedMinProfitRate"`
	ActivityPriceMode            string               `json:"activityPriceMode"`
	ActivityPartakeType          string               `json:"activityPartakeType"`
	TimeLimitedDiscountRate      float64              `json:"timeLimitedDiscountRate"`
	TimeLimitedMinProfitRate     float64              `json:"timeLimitedMinProfitRate"`
	TimeLimitedPriceMode         string               `json:"timeLimitedPriceMode"`
	TimeLimitedUserLimit         bool                 `json:"timeLimitedUserLimit"`
	TimeLimitedUserLimitNum      int                  `json:"timeLimitedUserLimitNum"`
	TimeLimitedStockLimit        bool                 `json:"timeLimitedStockLimit"`
	TimeLimitedStockLimitPercent int                  `json:"timeLimitedStockLimitPercent"`
	FixedPriceAdjustment         float64              `json:"fixedPriceAdjustment"`
	PriceIncreaseThreshold       float64              `json:"priceIncreaseThreshold"`
	PriceDecreaseThreshold       float64              `json:"priceDecreaseThreshold"`
	PriceIncreaseAction          string               `json:"priceIncreaseAction"`
	PriceDecreaseAction          string               `json:"priceDecreaseAction"`
	RestoreStockAmount           int                  `json:"restoreStockAmount"`
	Remark                       string               `json:"remark"`
	CreateTime                   types.FlexibleString `json:"createTime"`
}

// IsEnabled 判断策略是否启用
func (s *OperationStrategyDTO) IsEnabled() bool {
	return s.Status == 0
}
