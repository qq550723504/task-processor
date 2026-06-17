package bootstrap

import (
	"task-processor/internal/app/bootstrap/fetchers"
	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	"task-processor/internal/app/consumer"
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/platforms"
	"task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

func BuildConsumerDependencies() consumer.PlatformProcessorRegistryDependencies {
	return consumer.PlatformProcessorRegistryDependencies{
		PlatformModules: platforms.All(),
		SharedResourceProvider: func(cfg *config.Config, logger *logrus.Logger, needsAmazon bool) (*consumer.SharedResources, error) {
			resources, err := bootstrapresources.BuildSharedResources(cfg, logger, bootstrapresources.SharedResourceOptions{
				NeedAmazonCrawler: needsAmazon,
			})
			if err != nil {
				return nil, err
			}

			productFetcher, err := fetchers.BuildSharedProductFetcher(
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
		AmazonCrawlerCreator: func(cfg *config.Config, logger *logrus.Logger) runner.CrawlSource {
			return bootstrapresources.BuildAmazonCrawler(cfg, logger)
		},
		ProductFetcherProvider: func(cfg *config.Config, logger *logrus.Logger, crawlSource runner.CrawlSource) (*product.ProductFetcher, error) {
			resources, err := bootstrapresources.BuildSharedResources(cfg, logger, bootstrapresources.SharedResourceOptions{})
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
