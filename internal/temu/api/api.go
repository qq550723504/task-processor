// Package api 提供TEMU平台API的统一入口
package api

import (
	"task-processor/internal/temu/api/category"
	"task-processor/internal/temu/api/client"
	"task-processor/internal/temu/api/image"
	"task-processor/internal/temu/api/inventory"
	"task-processor/internal/temu/api/pricing"
	"task-processor/internal/temu/api/product"
	"task-processor/internal/temu/api/query"
)

// 重新导出客户端相关类型
type (
	APIClient          = client.APIClient
	APIClientManager   = client.APIClientManager
	APIClientInterface = client.ClientAPI
	Config             = client.Config
	CookieManager      = client.CookieManager
	HTTPManager        = client.HTTPManager
	AuthManager        = client.AuthManager
)

// 重新导出各功能包的 API 类型
type (
	ProductAPI   = product.API
	CategoryAPI  = category.API
	ImageAPI     = image.API
	InventoryAPI = inventory.API
	PricingAPI   = pricing.API
	QueryAPI     = query.API
)

// 重新导出产品相关类型
type (
	Product                  = product.Product
	SaveRequest              = product.SaveRequest
	SaveResponse             = product.SaveResponse
	SaveResult               = product.SaveResult
	SubmitRequest            = product.SubmitRequest
	SubmitResponse           = product.SubmitResponse
	SubmitResult             = product.SubmitResult
	ExtraInfo                = product.ExtraInfo
	CreateCommitRequest      = product.CreateCommitRequest
	CreateCommitResponse     = product.CreateCommitResponse
	GoodsBasicInfo           = product.GoodsBasicInfo
	GoodsSaleInfo            = product.GoodsSaleInfo
	ServicePromise           = product.ServicePromise
	ExtensionInfo            = product.ExtensionInfo
	Skc                      = product.Skc
	Sku                      = product.Sku
	SpecInfo                 = product.SpecInfo
	ImageInfo                = product.ImageInfo
	ProductExpressInfo       = product.ProductExpressInfo
	BatchSkuInfo             = product.BatchSkuInfo
	GoodsSearchResponse      = product.GoodsSearchResponse
	GoodsSearchItem          = product.GoodsSearchItem
	PriceQueryRequest        = product.PriceQueryRequest
	PriceQueryResponse       = product.PriceQueryResponse
	MaxRetailPriceQueryItem  = product.MaxRetailPriceQueryItem
	MaxRetailPriceResultItem = product.MaxRetailPriceResultItem
	BulkRelistOptions        = product.BulkRelistOptions
	BulkRelistSummary        = product.BulkRelistSummary
)

// 重新导出定价相关类型
type (
	PendingListRequest  = pricing.PendingListRequest
	PendingListResponse = pricing.PendingListResponse
	SalesBoostGoods     = pricing.SalesBoostGoods
	SalesBoostSku       = pricing.SalesBoostSku
	PriceVO             = pricing.PriceVO
	DecisionAction      = pricing.DecisionAction
	Decision            = pricing.Decision
)

// 重新导出库存相关类型
type (
	StockEditRequest  = inventory.StockEditRequest
	SkuStockChange    = inventory.SkuStockChange
	StockEditResponse = inventory.StockEditResponse
	OnlineRequest     = inventory.OnlineRequest
	OnlineResponse    = inventory.OnlineResponse
	OfflineRequest    = inventory.OfflineRequest
	OfflineResponse   = inventory.OfflineResponse
	RelistRequest     = inventory.RelistRequest
	RelistResponse    = inventory.RelistResponse
	SearchResponse    = inventory.SearchResponse
)

// 重新导出图片相关类型
type (
	UploadSignature   = image.UploadSignature
	SignatureResponse = image.SignatureResponse
	UploadResult      = image.UploadResult
	ValidationResult  = image.ValidationResult
)

// 重新导出分类相关类型
type (
	CategoryDisclaimResponse  = category.DisclaimResponse
	CategoryRecommendRequest  = category.RecommendRequest
	CategoryRecommendResponse = category.RecommendResponse
)

// 重新导出查询相关类型
type (
	TextCheckRequest               = query.TextCheckRequest
	TextCheckResponse              = query.TextCheckResponse
	SpecQueryRequest               = query.SpecQueryRequest
	SpecQueryResponse              = query.SpecQueryResponse
	SkuSnCheckRequest              = query.SkuSnCheckRequest
	SkuSnCheckResponse             = query.SkuSnCheckResponse
	CostTemplateRequest            = query.CostTemplateRequest
	CostTemplateResponse           = query.CostTemplateResponse
	CommitDetailRequest            = query.CommitDetailRequest
	CommitDetailResponse           = query.CommitDetailResponse
	CommitDetailResult             = query.CommitDetailResult
	CommitDetailGoodsBasic         = query.CommitDetailGoodsBasic
	CommitDetailCategoryTree       = query.CommitDetailCategoryTree
	CommitDetailCategoryDisclaimer = query.CommitDetailCategoryDisclaimer
	CommitDetailGoodsSaleInfo      = query.CommitDetailGoodsSaleInfo
	CommitDetailExtra              = query.CommitDetailExtra
	OutSkuSnItem                   = query.OutSkuSnItem
	OutGoodsSnCheckResult          = query.OutGoodsSnCheckResult
	SkuQueryResponse               = query.SkuQueryResponse
)

// 重新导出查询相关选项类型
type (
	GoodsSearchOptions = product.GoodsSearchOptions
	SkuQueryOptions    = query.SkuQueryOptions
)

// 重新导出构造函数
var (
	NewAPIClient          = client.NewAPIClient
	NewAPIClientManager   = client.NewAPIClientManager
	NewCookieManager      = client.NewCookieManager
	NewHTTPManager        = client.NewHTTPManager
	NewAuthManager        = client.NewAuthManager
	DefaultConfig         = client.DefaultConfig
	GetDefaultHeaders     = client.GetDefaultHeaders
	NewProductAPI         = product.NewAPI
	NewSubmitAPI          = product.NewAPI // Submit/Save/CreateCommit 均在 product.API
	NewCategoryAPI        = category.NewAPI
	NewImageAPI           = image.NewAPI
	NewInventoryAPI       = inventory.NewAPI
	NewPricingAPI         = pricing.NewAPI
	NewQueryAPI           = query.NewAPI
	NewGoodsSearchOptions = product.NewGoodsSearchOptions
	NewSkuQueryOptions    = query.NewSkuQueryOptions
)
