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

type ListingRuntimeDependencies struct {
	Consumer        consumer.PlatformProcessorRegistryDependencies
	buildResources  func(cfg *config.Config, logger *logrus.Logger, needs consumer.SharedResourceNeeds) (*consumer.SharedResources, error)
	sharedResources func() *SharedResources
}

func (d ListingRuntimeDependencies) BuildConsumerSharedResources(cfg *config.Config, logger *logrus.Logger, needs consumer.SharedResourceNeeds) (*consumer.SharedResources, error) {
	if d.buildResources == nil {
		return nil, nil
	}
	return d.buildResources(cfg, logger, needs)
}

func (d ListingRuntimeDependencies) SharedResources() *SharedResources {
	if d.sharedResources == nil {
		return nil
	}
	return d.sharedResources()
}

func BuildListingRuntimeDependencies() ListingRuntimeDependencies {
	var sharedResources *SharedResources
	return ListingRuntimeDependencies{
		Consumer: buildConsumerDependencies(),
		buildResources: buildConsumerSharedResourcesFunc(func(resources *SharedResources) {
			sharedResources = resources
		}),
		sharedResources: func() *SharedResources {
			return sharedResources
		},
	}
}

func BuildConsumerDependencies() consumer.PlatformProcessorRegistryDependencies {
	return buildConsumerDependencies()
}

func buildConsumerDependencies() consumer.PlatformProcessorRegistryDependencies {
	return consumer.PlatformProcessorRegistryDependencies{
		PlatformModules: platforms.All(),
	}
}

func buildConsumerSharedResourcesFunc(onSharedResources func(*SharedResources)) func(*config.Config, *logrus.Logger, consumer.SharedResourceNeeds) (*consumer.SharedResources, error) {
	return func(cfg *config.Config, logger *logrus.Logger, needs consumer.SharedResourceNeeds) (*consumer.SharedResources, error) {
		resources, err := bootstrapresources.BuildSharedResources(cfg, logger, bootstrapresources.SharedResourceOptions{
			NeedAmazonCrawler: needs.NeedAmazonCrawler,
		})
		if err != nil {
			return nil, err
		}

		productFetcher, err := fetchers.BuildSharedProductFetcher(
			cfg,
			resources.RawJSONDataClient,
			resources.AmazonCrawler,
			resources.RabbitMQClient,
		)
		if err != nil {
			return nil, err
		}
		if onSharedResources != nil {
			onSharedResources(resources)
		}

		return &consumer.SharedResources{
			ListingRuntimeImportTaskRepository: resources.ImportTaskRepository,
			RawJSONDataClient:                  resources.RawJSONDataClient,
			StoreAPI:                           resources.StoreAPI,
			SchedulerRuntime:                   resources.SchedulerRuntime,
			SchedulerFactoryRuntime:            resources.SchedulerFactoryRuntime,
			ProcessorRuntime:                   resources.ProcessorRuntime,
			CrawlSource:                        resources.AmazonCrawler,
			ProductFetcher:                     productFetcher,
		}, nil
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
				resources.RawJSONDataClient,
				&cfg.Amazon,
				crawlSource,
			)

			return productFetcher, nil
		},
	}
}
