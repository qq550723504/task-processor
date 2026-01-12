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
	ProductAPI     = services.ProductAPI
	PricingAPI     = services.PricingAPI
	ListingAPI     = services.ListingAPI
	OfflineAPI     = services.OfflineAPI
	ImageUploadAPI = services.ImageUploadAPI
)

// 重新导出模型相关类型
type (
	// 产品相关
	Product             = models.Product
	ProductSaveResult   = models.ProductSaveResult
	SourceProduct       = models.SourceProduct
	ProductListRequest  = models.ProductListRequest
	ProductListResponse = models.ProductListResponse
	TemuProductResponse = models.TemuProductResponse

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

	// 通用类型
	ImageInfo          = models.ImageInfo
	ProductExpressInfo = models.ProductExpressInfo

	// 商品相关
	GoodsBasicInfo = models.GoodsBasicInfo
	GoodsSaleInfo  = models.GoodsSaleInfo

	// SKU相关
	Skc = models.Skc
)

// 重新导出构造函数
var (
	NewAPIClient        = client.NewAPIClient
	NewAPIClientManager = client.NewAPIClientManager
	NewProductAPI       = services.NewProductAPI
	NewPricingAPI       = services.NewPricingAPI
	NewListingAPI       = services.NewListingAPI
	NewOfflineAPI       = services.NewOfflineAPI
	NewImageUploadAPI   = services.NewImageUploadAPI
	NewCookieManager    = client.NewCookieManager
	NewHTTPManager      = client.NewHTTPManager
	NewAuthManager      = client.NewAuthManager
	DefaultConfig       = client.DefaultConfig
	GetDefaultHeaders   = client.GetDefaultHeaders
)
