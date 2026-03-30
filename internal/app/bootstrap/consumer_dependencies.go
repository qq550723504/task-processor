package bootstrap

import (
	"task-processor/internal/app/consumer"
	appfetcher "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

func BuildConsumerDependencies() consumer.PlatformRegistryDependencies {
	return consumer.PlatformRegistryDependencies{
		ProcessorCreators: BuildConsumerProcessorCreators(),
		SharedResourceProvider: func(cfg *config.Config, logger *logrus.Logger, needsAmazon bool) (*consumer.SharedResources, error) {
			resources, err := BuildSharedResources(cfg, logger, SharedResourceOptions{
				NeedAmazonCrawler: needsAmazon,
			})
			if err != nil {
				return nil, err
			}

			productFetcher, err := buildSharedProductFetcher(
				cfg,
				resources.ManagementClient.GetRawJsonDataAdapter(),
				resources.AmazonCrawler,
				resources.RabbitMQClient,
			)
			if err != nil {
				return nil, err
			}

			return &consumer.SharedResources{
				ManagementClient: resources.ManagementClient,
				CrawlSource:      resources.AmazonCrawler,
				ProductFetcher:   productFetcher,
			}, nil
		},
	}
}

func BuildCrawlerDependencies() consumer.CrawlerRegistryDependencies {
	return consumer.CrawlerRegistryDependencies{
		AmazonCrawlerCreator: func(cfg *config.Config, logger *logrus.Logger) *amazon.AmazonProcessor {
			return amazon.CreateProcessor(cfg, logger)
		},
		ProductFetcherProvider: func(cfg *config.Config, logger *logrus.Logger, crawlSource *amazon.AmazonProcessor) (*product.ProductFetcher, error) {
			resources, err := BuildSharedResources(cfg, logger, SharedResourceOptions{})
			if err != nil {
				return nil, err
			}

			productFetcher := product.NewProductFetcher(
				resources.ManagementClient.GetRawJsonDataAdapter(),
				&cfg.Amazon,
				crawlSource,
			)

			return productFetcher, nil
		},
	}
}

func buildSharedProductFetcher(
	cfg *config.Config,
	rawJsonDataClient product.RawJsonDataClient,
	crawlSource *amazon.AmazonProcessor,
	rabbitmqClient *rabbitmq.Client,
) (appfetcher.ProductFetcher, error) {
	factory := appfetcher.NewFetcherFactory()
	return factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, crawlSource, rabbitmqClient)
}
