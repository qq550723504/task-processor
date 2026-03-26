// Package fetcher 提供产品获取器工厂
package fetcher

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	coreLogger "task-processor/internal/core/logger"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/model"
	domainProduct "task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

// FetcherType 获取器类型
type FetcherType string

const (
	// LocalFetcher 本地获取器（使用本地Amazon处理器）
	LocalFetcher FetcherType = "local"
	// DistributedFetcher 分布式获取器（使用分布式爬虫集群）
	DistributedFetcher FetcherType = "distributed"
)

// ProductFetcher 产品获取器接口
type ProductFetcher interface {
	FetchProduct(ctx context.Context, req *domainProduct.FetchRequest) (*model.Product, error)
	// FetchVariants 批量获取变体数据。所有任务一次性提交，并发等待结果，实现真正的分布式消费。
	FetchVariants(ctx context.Context, req *domainProduct.FetchRequest, variantASINs []string) ([]*model.Product, error)
	CacheProduct(req *domainProduct.FetchRequest, product *model.Product) error
	CacheVariants(req *domainProduct.FetchRequest, variants []*model.Product) error
	GetStats() map[string]any
}

// FetcherFactory 获取器工厂
type FetcherFactory struct {
	logger *logrus.Entry
}

// NewFetcherFactory 创建获取器工厂
func NewFetcherFactory() *FetcherFactory {
	return &FetcherFactory{
		logger: coreLogger.GetGlobalLogger("FetcherFactory"),
	}
}

// CreateFetcher 创建产品获取器
func (f *FetcherFactory) CreateFetcher(
	fetcherType FetcherType,
	rawJsonDataClient domainProduct.RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	amazonProcessor domainProduct.AmazonScraper,
	rabbitmqClient *rabbitmq.Client,
) (ProductFetcher, error) {

	switch fetcherType {
	case LocalFetcher:
		f.logger.Info("创建本地产品获取器")
		return domainProduct.NewProductFetcher(rawJsonDataClient, amazonConfig, amazonProcessor), nil

	case DistributedFetcher:
		f.logger.Info("创建分布式产品获取器")
		if rabbitmqClient == nil {
			return nil, fmt.Errorf("分布式获取器需要RabbitMQ客户端")
		}
		return NewDistributedProductFetcher(rawJsonDataClient, amazonConfig, rabbitmqClient)

	default:
		return nil, fmt.Errorf("不支持的获取器类型: %s", fetcherType)
	}
}

// CreateFetcherFromConfig 根据配置创建获取器
func (f *FetcherFactory) CreateFetcherFromConfig(
	cfg *config.Config,
	rawJsonDataClient domainProduct.RawJsonDataClient,
	amazonProcessor domainProduct.AmazonScraper,
	rabbitmqClient *rabbitmq.Client,
) (ProductFetcher, error) {

	// 检查是否启用分布式爬虫
	if cfg.RabbitMQ != nil && cfg.RabbitMQ.Enabled && rabbitmqClient != nil {
		f.logger.Info("配置启用分布式爬虫，创建分布式获取器")
		return f.CreateFetcher(
			DistributedFetcher,
			rawJsonDataClient,
			&cfg.Amazon,
			amazonProcessor,
			rabbitmqClient,
		)
	}

	// 默认使用本地获取器
	f.logger.Info("使用本地获取器")
	return f.CreateFetcher(
		LocalFetcher,
		rawJsonDataClient,
		&cfg.Amazon,
		amazonProcessor,
		nil,
	)
}

// GetRecommendedFetcher 获取推荐的获取器类型
func (f *FetcherFactory) GetRecommendedFetcher(cfg *config.Config) FetcherType {
	// 如果配置了RabbitMQ，推荐使用分布式获取器
	if cfg.RabbitMQ != nil && cfg.RabbitMQ.Enabled && cfg.RabbitMQ.URL != "" {
		return DistributedFetcher
	}

	// 默认推荐本地获取器
	return LocalFetcher
}
