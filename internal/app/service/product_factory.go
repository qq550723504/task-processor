package service

import (
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	amazonpkg "task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/product/repo/impl"
	productservice "task-processor/internal/domain/product/service"
	"task-processor/internal/infra/productcrawler"
	"task-processor/internal/infra/clients/management/api"

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
) *productservice.ProductService {
	// 创建仓储层
	cacheRepo := impl.NewCacheRepositoryImpl(rawJsonDataClient, f.logger)

	domainResolver := amazonpkg.NewDomainResolver()
	crawlerRepo := productcrawler.NewCrawlerRepositoryImpl(
		amazonProcessor,
		amazonConfig,
		domainResolver,
		f.logger,
	)

	// 创建验证器
	validator := productservice.NewProductValidator(f.logger)

	// 创建服务层
	productService := productservice.NewProductService(
		cacheRepo,
		crawlerRepo,
		validator,
		f.logger,
	)

	f.logger.Info("产品服务创建成功")
	return productService
}
