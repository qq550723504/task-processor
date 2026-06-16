// Package fetcher 提供产品获取器工厂
package fetcher

import (
	"context"
	"fmt"

	"task-processor/internal/app/ports"
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
	LocalFetcher       FetcherType = "local"
	DistributedFetcher FetcherType = "distributed"
	RemoteAPIFetcher   FetcherType = "remote-api"
)

// ProductFetcher 产品获取器接口
type ProductFetcher interface {
	FetchProduct(ctx context.Context, req *domainProduct.FetchRequest) (*model.Product, error)
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
	crawlSource ports.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) (ProductFetcher, error) {
	switch fetcherType {
	case LocalFetcher:
		f.logger.Info("creating local product fetcher")
		return domainProduct.NewProductFetcher(rawJsonDataClient, amazonConfig, crawlSource), nil
	case RemoteAPIFetcher:
		f.logger.Info("creating remote api product fetcher")
		return NewRemoteAPIProductFetcher(rawJsonDataClient, amazonConfig)
	case DistributedFetcher:
		f.logger.Info("creating distributed product fetcher")
		if rabbitmqClient == nil {
			return nil, fmt.Errorf("distributed fetcher requires RabbitMQ client")
		}
		return NewDistributedProductFetcher(rawJsonDataClient, amazonConfig, rabbitmqClient)
	default:
		return nil, fmt.Errorf("unsupported fetcher type: %s", fetcherType)
	}
}

// CreateFetcherFromConfig 根据配置创建获取器
func (f *FetcherFactory) CreateFetcherFromConfig(
	cfg *config.Config,
	rawJsonDataClient domainProduct.RawJsonDataClient,
	crawlSource ports.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) (ProductFetcher, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	if cfg.Amazon.RemoteAPI.Enabled {
		f.logger.Info("creating remote api fetcher from config")
		return f.CreateFetcher(
			RemoteAPIFetcher,
			rawJsonDataClient,
			&cfg.Amazon,
			crawlSource,
			rabbitmqClient,
		)
	}

	if cfg.RabbitMQ == nil || !cfg.RabbitMQ.Enabled {
		return nil, fmt.Errorf("crawler fetcher requires amazon.remoteAPI.enabled=true or rabbitmq.enabled=true; local fallback is disabled")
	}
	if !cfg.Amazon.Enabled {
		return nil, fmt.Errorf("distributed fetcher requires amazon.enabled=true when amazon.remoteAPI.enabled=false; local fallback is disabled")
	}
	if rabbitmqClient == nil {
		return nil, fmt.Errorf("distributed fetcher requires RabbitMQ client; local fallback is disabled")
	}

	f.logger.Info("creating distributed fetcher from config")
	return f.CreateFetcher(
		DistributedFetcher,
		rawJsonDataClient,
		&cfg.Amazon,
		crawlSource,
		rabbitmqClient,
	)
}

// GetRecommendedFetcher 获取推荐的获取器类型
func (f *FetcherFactory) GetRecommendedFetcher(cfg *config.Config) FetcherType {
	if cfg.RabbitMQ != nil && cfg.RabbitMQ.Enabled && cfg.RabbitMQ.URL != "" {
		return DistributedFetcher
	}

	return LocalFetcher
}
