// Package factory 提供产品服务的工厂方法
package factory

import (
	"task-processor/internal/common/amazon"
	"task-processor/internal/common/management/api"
	"task-processor/internal/core/config"
	"task-processor/internal/domain/product"
	"task-processor/internal/domain/product/repo/impl"
	"task-processor/internal/domain/product/service"

	"github.com/sirupsen/logrus"
)

// ProductServiceFactory 产品服务工厂
type ProductServiceFactory struct {
	logger *logrus.Entry
}

// NewProductServiceFactory 创建产品服务工厂
func NewProductServiceFactory(logger *logrus.Entry) *ProductServiceFactory {
	return &ProductServiceFactory{
		logger: logger.WithField("component", "ProductServiceFactory"),
	}
}

// CreateProductService 创建产品服务（推荐使用）
func (f *ProductServiceFactory) CreateProductService(
	rawJsonDataClient api.RawJsonDataAPI,
	amazonConfig *config.AmazonConfig,
	amazonProcessor *amazon.AmazonProcessor,
) *service.ProductService {
	// 创建仓储层
	cacheRepo := impl.NewCacheRepositoryImpl(rawJsonDataClient, f.logger)

	domainResolver := product.NewDomainResolver()
	crawlerRepo := impl.NewCrawlerRepositoryImpl(
		amazonProcessor,
		amazonConfig,
		domainResolver,
		f.logger,
	)

	// 创建验证器
	validator := service.NewProductValidator(f.logger)

	// 创建服务层
	productService := service.NewProductService(
		cacheRepo,
		crawlerRepo,
		validator,
		f.logger,
	)

	f.logger.Info("产品服务创建成功")
	return productService
}

// CreateLegacyProductFetcher 创建旧版产品获取器（向后兼容）
func (f *ProductServiceFactory) CreateLegacyProductFetcher(
	rawJsonDataClient api.RawJsonDataAPI,
	amazonConfig *config.AmazonConfig,
	amazonProcessor *amazon.AmazonProcessor,
) *product.ProductFetcher {
	f.logger.Warn("使用旧版ProductFetcher，建议迁移到新版ProductService")

	return product.NewProductFetcher(
		rawJsonDataClient,
		amazonConfig,
		amazonProcessor,
	)
}
