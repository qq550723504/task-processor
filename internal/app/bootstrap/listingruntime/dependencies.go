package listingruntime

import (
	"fmt"

	"task-processor/internal/app/bootstrap/fetchers"
	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	"task-processor/internal/app/consumer"
	"task-processor/internal/app/ports"
	"task-processor/internal/core/config"
	"task-processor/internal/platforms"

	"github.com/sirupsen/logrus"
)

type dependencies struct {
	platformModules               []consumer.PlatformModule
	buildResources                func(cfg *config.Config, logger *logrus.Logger, platform string, needs consumer.SharedResourceNeeds) (consumer.SharedResources, error)
	listingRuntimeHealthValidator func() ports.ListingRuntimeHealthValidator
}

func (d dependencies) BuildConsumerSharedResources(cfg *config.Config, logger *logrus.Logger, platform string, needs consumer.SharedResourceNeeds) (consumer.SharedResources, error) {
	if d.buildResources == nil {
		return consumer.SharedResources{}, nil
	}
	return d.buildResources(cfg, logger, platform, needs)
}

func (d dependencies) ListingRuntimeHealthValidator() ports.ListingRuntimeHealthValidator {
	if d.listingRuntimeHealthValidator == nil {
		return nil
	}
	return d.listingRuntimeHealthValidator()
}

func (d dependencies) ConsumerDependencies(cfg *config.Config, platformsStr string) consumer.PlatformProcessorRegistryDependencies {
	return consumer.NewPlatformProcessorRegistryDependencies(cfg, platformsStr, d.platformModules)
}

func BuildDependencies() dependencies {
	var listingRuntimeHealthValidator ports.ListingRuntimeHealthValidator
	return dependencies{
		platformModules: platforms.All(),
		buildResources: buildConsumerSharedResourcesFunc(func(validator ports.ListingRuntimeHealthValidator) {
			listingRuntimeHealthValidator = validator
		}),
		listingRuntimeHealthValidator: func() ports.ListingRuntimeHealthValidator {
			return listingRuntimeHealthValidator
		},
	}
}

func buildConsumerSharedResourcesFunc(onListingRuntimeHealthValidator func(ports.ListingRuntimeHealthValidator)) func(*config.Config, *logrus.Logger, string, consumer.SharedResourceNeeds) (consumer.SharedResources, error) {
	return func(cfg *config.Config, logger *logrus.Logger, platform string, needs consumer.SharedResourceNeeds) (consumer.SharedResources, error) {
		var capturedListingRuntimeHealthValidator ports.ListingRuntimeHealthValidator
		captureListingRuntimeHealthValidator := func(validator ports.ListingRuntimeHealthValidator) {
			capturedListingRuntimeHealthValidator = validator
		}

		resources, err := bootstrapresources.BuildSharedResources(cfg, logger, bootstrapresources.SharedResourceOptions{
			NeedAmazonCrawler:               needs.NeedAmazonCrawler,
			OnListingRuntimeHealthValidator: captureListingRuntimeHealthValidator,
		})
		if err != nil {
			return consumer.SharedResources{}, err
		}
		if resources == nil {
			return consumer.SharedResources{}, fmt.Errorf("build shared resources: resources are nil")
		}

		productFetcher, err := fetchers.BuildPlatformProductFetcher(
			cfg,
			platform,
			resources.RawJSONDataClient,
			resources.AmazonCrawler,
			resources.RabbitMQClient,
		)
		if err != nil {
			return consumer.SharedResources{}, err
		}
		if onListingRuntimeHealthValidator != nil {
			onListingRuntimeHealthValidator(capturedListingRuntimeHealthValidator)
		}

		return consumer.SharedResources{
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
