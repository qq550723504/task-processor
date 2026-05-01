package consumer

import (
	"context"

	appfetcher "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

type SharedResources struct {
	ManagementClient *management.ClientManager
	CrawlSource      *amazon.AmazonProcessor
	ProductFetcher   appfetcher.ProductFetcher
}

type SharedResourceProvider func(cfg *config.Config, logger *logrus.Logger, needsAmazon bool) (*SharedResources, error)

type SchedulerDependenciesBuilder func(
	managementClient *management.ClientManager,
	cfg *config.Config,
	crawlSource runner.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) runner.SchedulerDependencies

type PlatformRuntimeContext struct {
	Config           *config.Config
	Logger           *logrus.Logger
	ManagementClient *management.ClientManager
	CrawlSource      *amazon.AmazonProcessor
	ProductFetcher   appfetcher.ProductFetcher
	RabbitMQClient   *rabbitmq.Client
	ServiceManager   *ServiceManager
	SchedulerBuilder SchedulerDependenciesBuilder
}

type PlatformModule interface {
	Name() string
	Enabled(cfg *config.Config) bool
	NeedsAmazon(cfg *config.Config) bool
	RegisterConsumer(ctx context.Context, rt PlatformRuntimeContext, registry ProcessorRegistrar) error
	ConfigureListingRuntime(ctx context.Context, rt PlatformRuntimeContext) error
}

type ProcessorRegistrar interface {
	RegisterProcessor(platform string, processor worker.Processor) error
}
