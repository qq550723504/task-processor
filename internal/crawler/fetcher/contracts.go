package fetcher

import (
	"context"
	"fmt"
	"reflect"

	"task-processor/internal/app/ports"
	"task-processor/internal/core/config"
	coreLogger "task-processor/internal/core/logger"
	sourceamazon "task-processor/internal/integration/crawler/amazon"
	"task-processor/internal/marketplace/sourceproduct"
	"task-processor/internal/model"
	"task-processor/internal/platform/queue/rabbitmq"

	"github.com/sirupsen/logrus"
)

type FetcherType string

const (
	LocalFetcher       FetcherType = "local"
	DistributedFetcher FetcherType = "distributed"
	RemoteAPIFetcher   FetcherType = "remote-api"
)

type ProductReader interface {
	FetchProduct(ctx context.Context, req *sourceproduct.FetchRequest) (*model.Product, error)
	FetchVariants(ctx context.Context, req *sourceproduct.FetchRequest, variantASINs []string) ([]*model.Product, error)
}

type ProductCache interface {
	CacheProduct(req *sourceproduct.FetchRequest, product *model.Product) error
	CacheVariants(req *sourceproduct.FetchRequest, variants []*model.Product) error
}

type ProductFetcherStats interface {
	GetStats() map[string]any
}

type ProductFetcher interface {
	ProductReader
	ProductCache
	ProductFetcherStats
}

type amazonSourceFetcherAdapter struct {
	crawlSource ports.CrawlSource
	delegate    sourceamazon.AmazonSourceFetcher
}

func newAmazonSourceFetcher(crawlSource ports.CrawlSource, zipcodes map[string]string) sourceproduct.SourceFetcher {
	zipcodesSnapshot := make(map[string]string, len(zipcodes))
	for region, zipcode := range zipcodes {
		zipcodesSnapshot[region] = zipcode
	}
	return &amazonSourceFetcherAdapter{
		crawlSource: crawlSource,
		delegate: sourceamazon.AmazonSourceFetcher{
			Planner: sourceamazon.AmazonCrawlRequestPlanner{
				DomainResolver: sourceamazon.AmazonDefaultDomainResolver{},
				ZipcodePolicy:  sourceamazon.AmazonDefaultZipcodePolicy{},
				Zipcodes:       zipcodesSnapshot,
			},
			Source: crawlSource,
		},
	}
}

func (a *amazonSourceFetcherAdapter) Configured() bool {
	return a != nil && !isNilCrawlSource(a.crawlSource)
}

func (a *amazonSourceFetcherAdapter) Fetch(ctx context.Context, req sourceproduct.SourceFetchRequest) (*model.Product, error) {
	if !a.Configured() {
		return nil, fmt.Errorf("amazon crawler source is not configured")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	product, err := a.delegate.Fetch(ctx, sourceamazon.AmazonCrawlRequestInput{
		Region:    req.Region,
		ProductID: req.ProductID,
		Zipcode:   req.Zipcode,
	})
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	return product, err
}

func isNilCrawlSource(source ports.CrawlSource) bool {
	if source == nil {
		return true
	}
	value := reflect.ValueOf(source)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

type ProductFetcherBuilder struct {
	factory           *FetcherFactory
	rawJsonDataClient sourceproduct.RawJsonDataClient
	crawlSource       ports.CrawlSource
}

func NewProductFetcherBuilder(
	rawJsonDataClient sourceproduct.RawJsonDataClient,
	crawlSource ports.CrawlSource,
) *ProductFetcherBuilder {
	return &ProductFetcherBuilder{
		factory:           NewFetcherFactory(),
		rawJsonDataClient: rawJsonDataClient,
		crawlSource:       crawlSource,
	}
}

func (b *ProductFetcherBuilder) Build(
	amazonConfig *config.AmazonConfig,
	rabbitmqClient *rabbitmq.Client,
) (ProductFetcher, error) {
	return b.factory.CreateFetcher(
		resolveProductFetcherType(amazonConfig, rabbitmqClient),
		b.rawJsonDataClient,
		amazonConfig,
		b.crawlSource,
		rabbitmqClient,
	)
}

func resolveProductFetcherType(
	amazonConfig *config.AmazonConfig,
	rabbitmqClient *rabbitmq.Client,
) FetcherType {
	if rabbitmqClient != nil {
		return DistributedFetcher
	}
	if amazonConfig != nil && amazonConfig.RemoteAPI.Enabled {
		return RemoteAPIFetcher
	}
	return LocalFetcher
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
	rawJsonDataClient sourceproduct.RawJsonDataClient,
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
		options := sourceproduct.ProductFetcherOptions{}
		var zipcodes map[string]string
		if amazonConfig != nil {
			options.Enabled = amazonConfig.Enabled
			options.DataFreshnessDays = amazonConfig.DataFreshnessDays
			zipcodes = amazonConfig.Zipcodes
		}
		return sourceproduct.NewProductFetcherWithLogger(
			rawJsonDataClient,
			options,
			newAmazonSourceFetcher(crawlSource, zipcodes),
			f.logger,
		), nil
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
	rawJsonDataClient sourceproduct.RawJsonDataClient,
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
	rawJsonDataClient sourceproduct.RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
	rabbitmqClient *rabbitmq.Client,
) (ProductFetcher, error) {
	return newDistributedProductFetcher(rawJsonDataClient, amazonConfig, rabbitmqClient)
}

func NewRemoteAPIProductFetcher(
	rawJsonDataClient sourceproduct.RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
) (ProductFetcher, error) {
	return newRemoteAPIProductFetcher(rawJsonDataClient, amazonConfig)
}
