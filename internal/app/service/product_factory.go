package service

import (
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	amazonpkg "task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/product"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/productcrawler"
	infraproduct "task-processor/internal/infra/repository/product"

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

// CreateProductService 创建产品服务
func (f *ProductServiceFactory) CreateProductService(
	rawJsonDataClient api.RawJsonDataAPI,
	amazonConfig *config.AmazonConfig,
	amazonProcessor *amazon.AmazonProcessor,
) *product.ProductService {
	cacheRepo := infraproduct.NewCacheRepositoryImpl(rawJsonDataClient, f.logger)

	domainResolver := amazonpkg.NewDomainResolver()
	crawlerRepo := productcrawler.NewCrawlerRepositoryImpl(
		amazonProcessor,
		amazonConfig,
		domainResolver,
		f.logger,
	)

	validator := product.NewProductValidator(f.logger)

	productService := product.NewProductService(
		cacheRepo,
		crawlerRepo,
		validator,
		f.logger,
	)

	f.logger.Info("产品服务创建成功")
	return productService
}
