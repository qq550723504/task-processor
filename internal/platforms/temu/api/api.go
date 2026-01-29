// Package api 提供TEMU平台API的统一入口
package api

// 重新导出主要的类型和函数，保持向后兼容
import (
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"
	"task-processor/internal/platforms/temu/api/services"
)

// 重新导出客户端相关类型
type (
	APIClient          = client.APIClient
	APIClientManager   = client.APIClientManager
	APIClientInterface = client.APIClientInterface
)

// 重新导出基础设施相关类型
type (
	Config        = client.Config
	CookieManager = client.CookieManager
	HTTPManager   = client.HTTPManager
	AuthManager   = client.AuthManager
)

// 重新导出服务相关类型
type (
	ProductAPI       = services.ProductAPI
	PricingService   = services.PricingService
	ListingAPI       = services.ListingAPI
	OfflineAPI       = services.OfflineAPI
	ImageUploadAPI   = services.ImageUploadAPI
	CategoryAPI      = services.CategoryAPI
	QueryAPI         = services.QueryAPI
	SubmitAPI        = services.SubmitAPI
	InventoryService = services.InventoryService
)

// 重新导出模型相关类型
type (
	// 产品相关
	Product            = models.Product
	ProductSaveResult  = models.ProductSaveResult
	SourceProduct      = models.SourceProduct
	ProductListRequest = models.ProductListRequest

	// 定价相关
	PendingPriceListRequest  = models.PendingPriceListRequest
	PendingPriceListResponse = models.PendingPriceListResponse
	SalesBoostGoods          = models.SalesBoostGoods
	PriceVO                  = models.PriceVO

	// 上架相关
	RelistProductRequest   = models.RelistProductRequest
	RelistProductResponse  = models.RelistProductResponse
	DelistProductRequest   = models.DelistProductRequest
	DelistProductResponse  = models.DelistProductResponse
	ProductListingInfo     = models.ProductListingInfo
	ListingOperationResult = models.ListingOperationResult
	BatchListingResult     = models.BatchListingResult

	// 下架产品相关
	OfflineProductSearchRequest  = models.OfflineProductSearchRequest
	OfflineProductSearchResponse = models.OfflineProductSearchResponse

	// 图片上传相关
	UploadSignature         = models.UploadSignature
	SignatureResponse       = models.SignatureResponse
	UploadResult            = models.UploadResult
	TemuImageUploadResponse = models.TemuImageUploadResponse
	ImageValidationResult   = models.ImageValidationResult

	// 库存管理相关
	StockEditRequest  = models.StockEditRequest
	SkuStockChange    = models.SkuStockChange
	StockEditResponse = models.StockEditResponse
	StockEditResult   = models.StockEditResult

	// 下架产品相关
	OfflineProductRequest  = models.OfflineProductRequest
	OfflineProductResponse = models.OfflineProductResponse
	OfflineProductResult   = models.OfflineProductResult

	// 上架产品相关
	OnlineProductRequest  = models.OnlineProductRequest
	OnlineProductResponse = models.OnlineProductResponse
	OnlineProductResult   = models.OnlineProductResult

	// 通用类型
	ImageInfo          = models.ImageInfo
	ProductExpressInfo = models.ProductExpressInfo

	// 商品相关
	GoodsBasicInfo = models.GoodsBasicInfo
	GoodsSaleInfo  = models.GoodsSaleInfo

	// SKU相关
	Skc = models.Skc

	// 查询相关
	TextCheckRequest         = models.TextCheckRequest
	TextCheckResponse        = models.TextCheckResponse
	TemplateQueryRequest     = models.TemplateQueryRequest
	TemplateQueryResponse    = models.TemplateQueryResponse
	SpecQueryRequest         = models.SpecQueryRequest
	SpecQueryResponse        = models.SpecQueryResponse
	SkuSnCheckRequest        = models.SkuSnCheckRequest
	SkuSnCheckResponse       = models.SkuSnCheckResponse
	CostTemplateRequest      = models.CostTemplateRequest
	CostTemplateResponse     = models.CostTemplateResponse
	CommitDetailRequest      = models.CommitDetailRequest
	CommitDetailResponse     = models.CommitDetailResponse
	PriceQueryRequest        = models.PriceQueryRequest
	PriceQueryResponse       = models.PriceQueryResponse
	MaxRetailPriceQueryItem  = models.MaxRetailPriceQueryItem
	MaxRetailPriceResultItem = models.MaxRetailPriceResultItem

	// 提交相关
	ProductSubmitRequest  = models.ProductSubmitRequest
	ProductSubmitResponse = models.ProductSubmitResponse
	CreateCommitRequest   = models.CreateCommitRequest
	CreateCommitResponse  = models.CreateCommitResponse

	// 分类相关
	CategoryDisclaimRequest   = models.CategoryDisclaimRequest
	CategoryDisclaimResponse  = models.CategoryDisclaimResponse
	CategoryRecommendRequest  = models.CategoryRecommendRequest
	CategoryRecommendResponse = models.CategoryRecommendResponse
)

// 重新导出构造函数
var (
	NewAPIClient        = client.NewAPIClient
	NewAPIClientManager = client.NewAPIClientManager
	NewProductAPI       = services.NewProductAPI
	NewPricingService   = services.NewPricingService
	NewListingAPI       = services.NewListingAPI
	NewOfflineAPI       = services.NewOfflineAPI
	NewImageUploadAPI   = services.NewImageUploadAPI
	NewCategoryAPI      = services.NewCategoryAPI
	NewQueryAPI         = services.NewQueryAPI
	NewSubmitAPI        = services.NewSubmitAPI
	NewInventoryService = services.NewInventoryService
	NewCookieManager    = client.NewCookieManager
	NewHTTPManager      = client.NewHTTPManager
	NewAuthManager      = client.NewAuthManager
	DefaultConfig       = client.DefaultConfig
	GetDefaultHeaders   = client.GetDefaultHeaders
)
