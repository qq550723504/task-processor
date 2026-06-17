package fetcher

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	coreLogger "task-processor/internal/core/logger"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/model"
	"task-processor/internal/ports"
	domainProduct "task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

type FetcherType string

const (
	LocalFetcher       FetcherType = "local"
	DistributedFetcher FetcherType = "distributed"
	RemoteAPIFetcher   FetcherType = "remote-api"
)

type ProductFetcher interface {
	FetchProduct(ctx context.Context, req *domainProduct.FetchRequest) (*model.Product, error)
	FetchVariants(ctx context.Context, req *domainProduct.FetchRequest, variantASINs []string) ([]*model.Product, error)
	CacheProduct(req *domainProduct.FetchRequest, product *model.Product) error
	CacheVariants(req *domainProduct.FetchRequest, variants []*model.Product) error
	GetStats() map[string]any
}

type FetcherFactory struct {
	logger *logrus.Entry
}

func NewFetcherFactory() *FetcherFactory {
	return &FetcherFactory{
		logger: coreLogger.GetGlobalLogger("FetcherFactory"),
	}
}

func (f *FetcherFactory) CreateFetcher(
	fetcherType FetcherType,
	rawJsonDataClient domainProduct.RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	crawlSource ports.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) (ProductFetcher, error) {
	if f == nil || f.logger == nil {
		f = NewFetcherFactory()
	}
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
		return f.CreateFetcher(RemoteAPIFetcher, rawJsonDataClient, &cfg.Amazon, crawlSource, rabbitmqClient)
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
	return f.CreateFetcher(DistributedFetcher, rawJsonDataClient, &cfg.Amazon, crawlSource, rabbitmqClient)
}

func (f *FetcherFactory) GetRecommendedFetcher(cfg *config.Config) FetcherType {
	if cfg != nil && cfg.RabbitMQ != nil && cfg.RabbitMQ.Enabled && cfg.RabbitMQ.URL != "" {
		return DistributedFetcher
	}
	return LocalFetcher
}

func NewDistributedProductFetcher(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	rabbitmqClient *rabbitmq.Client,
) (ProductFetcher, error) {
	return newDistributedProductFetcher(rawJsonDataClient, amazonConfig, rabbitmqClient)
}

func NewRemoteAPIProductFetcher(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
) (ProductFetcher, error) {
	return newRemoteAPIProductFetcher(rawJsonDataClient, amazonConfig)
}
