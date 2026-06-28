package bootstrap

import (
	"task-processor/internal/app/bootstrap/fetchers"
	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	"task-processor/internal/app/consumer"
	"task-processor/internal/core/config"
	"task-processor/internal/platforms"

	"github.com/sirupsen/logrus"
)

type ListingRuntimeDependencies struct {
	platformModules []consumer.PlatformModule
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

func (d ListingRuntimeDependencies) ConsumerDependencies(cfg *config.Config, platformsStr string) consumer.PlatformProcessorRegistryDependencies {
	return consumer.NewPlatformProcessorRegistryDependencies(cfg, platformsStr, d.platformModules)
}

func BuildListingRuntimeDependencies() ListingRuntimeDependencies {
	var sharedResources *SharedResources
	return ListingRuntimeDependencies{
		platformModules: platforms.All(),
		buildResources: buildConsumerSharedResourcesFunc(func(resources *SharedResources) {
			sharedResources = resources
		}),
		sharedResources: func() *SharedResources {
			return sharedResources
		},
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
