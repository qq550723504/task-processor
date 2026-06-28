package bootstrap

import (
	"task-processor/internal/app/bootstrap/fetchers"
	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	"task-processor/internal/app/consumer"
	"task-processor/internal/core/config"
	"task-processor/internal/platforms"

	"github.com/sirupsen/logrus"
)

type listingRuntimeDependencies struct {
	platformModules []consumer.PlatformModule
	buildResources  func(cfg *config.Config, logger *logrus.Logger, platform string, needs consumer.SharedResourceNeeds) (*consumer.SharedResources, error)
	sharedResources func() *SharedResources
}

func (d listingRuntimeDependencies) BuildConsumerSharedResources(cfg *config.Config, logger *logrus.Logger, platform string, needs consumer.SharedResourceNeeds) (*consumer.SharedResources, error) {
	if d.buildResources == nil {
		return nil, nil
	}
	return d.buildResources(cfg, logger, platform, needs)
}

func (d listingRuntimeDependencies) SharedResources() *SharedResources {
	if d.sharedResources == nil {
		return nil
	}
	return d.sharedResources()
}

func (d listingRuntimeDependencies) ConsumerDependencies(cfg *config.Config, platformsStr string) consumer.PlatformProcessorRegistryDependencies {
	return consumer.NewPlatformProcessorRegistryDependencies(cfg, platformsStr, d.platformModules)
}

func BuildListingRuntimeDependencies() listingRuntimeDependencies {
	var sharedResources *SharedResources
	return listingRuntimeDependencies{
		platformModules: platforms.All(),
		buildResources: buildConsumerSharedResourcesFunc(func(resources *SharedResources) {
			sharedResources = resources
		}),
		sharedResources: func() *SharedResources {
			return sharedResources
		},
	}
}

func buildConsumerSharedResourcesFunc(onSharedResources func(*SharedResources)) func(*config.Config, *logrus.Logger, string, consumer.SharedResourceNeeds) (*consumer.SharedResources, error) {
	return func(cfg *config.Config, logger *logrus.Logger, platform string, needs consumer.SharedResourceNeeds) (*consumer.SharedResources, error) {
		resources, err := bootstrapresources.BuildSharedResources(cfg, logger, bootstrapresources.SharedResourceOptions{
			NeedAmazonCrawler: needs.NeedAmazonCrawler,
		})
		if err != nil {
			return nil, err
		}

		productFetcher, err := fetchers.BuildPlatformProductFetcher(
			cfg,
			platform,
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
			StoreAPI:                           resources.StoreAPI,
			SchedulerRuntime:                   resources.SchedulerRuntime,
			SchedulerFactoryRuntime:            resources.SchedulerFactoryRuntime,
			ProcessorRuntime:                   resources.ProcessorRuntime,
			CrawlSource:                        resources.AmazonCrawler,
			ProductFetcher:                     productFetcher,
		}, nil
	}
}
