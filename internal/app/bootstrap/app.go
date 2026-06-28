package bootstrap

import (
	"context"
	"fmt"

	bootstrapprocessors "task-processor/internal/app/bootstrap/processors"
	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	bootstrapschedulers "task-processor/internal/app/bootstrap/schedulers"
	"task-processor/internal/app/consumer"
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/platformbase"
	"task-processor/internal/product"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

type appServices struct {
	cfg               *config.Config
	authClient        *auth.ClientCredentialsAuthClient
	rawJSONDataClient product.RawJsonDataClient
	processorRuntime  consumer.ProcessorRuntime
	amazonCrawler     runner.CrawlSource
	rabbitmqClient    *rabbitmq.Client
	temuProcessor     *temu.TemuProcessor
	sheinProcessor    *pipeline.SheinProcessor
	processorService  runner.ProcessorService
	schedulerService  runner.SchedulerService
}

type ApplicationBootstrap struct {
	logger           *logrus.Logger
	configManager    config.ConfigManager
	lifecycleManager lifecycle.LifecycleManager
	services         *appServices
	appVersion       string
}

func NewApplicationBootstrap(logger *logrus.Logger) *ApplicationBootstrap {
	return &ApplicationBootstrap{
		logger:           logger,
		configManager:    config.NewConfigManager(logger),
		lifecycleManager: lifecycle.NewLifecycleManager(logger),
	}
}

func (a *ApplicationBootstrap) Initialize(configPath, appVersion string) error {
	a.logger.Info("initializing application bootstrap")
	a.appVersion = appVersion

	if err := a.loadConfiguration(configPath); err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	if err := a.initializeServices(); err != nil {
		return err
	}

	if err := a.registerLifecycleComponents(); err != nil {
		return err
	}

	a.logger.Info("application bootstrap initialized")
	return nil
}

func (a *ApplicationBootstrap) Start(ctx context.Context, appVersion string) error {
	a.logger.Info("starting application bootstrap")
	if err := a.lifecycleManager.StartAll(ctx); err != nil {
		return fmt.Errorf("start lifecycle components: %w", err)
	}
	a.logger.Info("application bootstrap started")
	return nil
}

func (a *ApplicationBootstrap) Stop(ctx context.Context) error {
	a.logger.Info("stopping application bootstrap")
	if err := a.lifecycleManager.StopAll(ctx); err != nil {
		a.logger.Errorf("stop lifecycle components: %v", err)
	}
	a.logger.Info("application bootstrap stopped")
	return nil
}

func (a *ApplicationBootstrap) GetConfigManager() config.ConfigManager {
	return a.configManager
}

func (a *ApplicationBootstrap) GetLifecycleManager() lifecycle.LifecycleManager {
	return a.lifecycleManager
}

func (a *ApplicationBootstrap) loadConfiguration(configPath string) error {
	a.logger.Infof("loading configuration from %s", configPath)
	source := config.NewFileConfigSource(configPath)
	cfg, err := a.configManager.Load(source)
	if err != nil {
		return err
	}
	a.logger.Infof("browser config loaded: enabled=%v path=%s poolSize=%d",
		cfg.Browser.Enabled, cfg.Browser.BrowserPath, cfg.Browser.PoolSize)
	return nil
}

func (a *ApplicationBootstrap) initializeServices() error {
	svc, err := buildServices(a.configManager.GetCurrent(), a.logger)
	if err != nil {
		return fmt.Errorf("build services: %w", err)
	}

	a.services = svc
	return nil
}

func (a *ApplicationBootstrap) registerLifecycleComponents() error {
	if err := registerComponents(a.lifecycleManager, a.services, a.logger, a.appVersion); err != nil {
		return fmt.Errorf("register lifecycle components: %w", err)
	}

	return nil
}

func buildServices(cfg *config.Config, logger *logrus.Logger) (*appServices, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	if err := bootstrapresources.InitializePrompts(context.Background(), cfg, logger); err != nil {
		logger.Warnf("prompt initialization failed, fallback will be used: %v", err)
	}

	resources, err := bootstrapresources.BuildSharedResources(cfg, logger, bootstrapresources.SharedResourceOptions{
		// cmd/task should not initialize the in-process Amazon crawler by default.
		// Scheduler/processor fetch paths can use remote API or distributed crawl instead.
		NeedAmazonCrawler: platformbase.PlatformUsesLocalFetcher(cfg, "temu") || platformbase.PlatformUsesLocalFetcher(cfg, "shein"),
	})
	if err != nil {
		return nil, fmt.Errorf("build shared resources: %w", err)
	}

	return buildAppServices(cfg, logger, resources), nil
}

func buildAppServices(cfg *config.Config, logger *logrus.Logger, resources *bootstrapresources.SharedResources) *appServices {
	return &appServices{
		cfg:               cfg,
		authClient:        resources.AuthClient,
		rawJSONDataClient: resources.RawJSONDataClient,
		processorRuntime:  resources.ProcessorRuntime,
		amazonCrawler:     resources.AmazonCrawler,
		rabbitmqClient:    resources.RabbitMQClient,
		processorService:  buildProcessorService(logger, resources),
		schedulerService:  buildSchedulerService(logger, cfg, resources),
	}
}

func buildProcessorService(logger *logrus.Logger, resources *bootstrapresources.SharedResources) runner.ProcessorService {
	return runner.NewProcessorServiceWithCreators(
		logger,
		resources.RawJSONDataClient,
		resources.ProcessorRuntime,
		resources.SchedulerRuntime,
		resources.SchedulerFactoryRuntime,
		resources.AmazonCrawler,
		resources.RabbitMQClient,
		bootstrapprocessors.BuildRunnerProcessorDependencies(),
	)
}

func buildSchedulerService(logger *logrus.Logger, cfg *config.Config, resources *bootstrapresources.SharedResources) runner.SchedulerService {
	return runner.NewSchedulerServiceWithDependencies(
		logger,
		resources.SchedulerRuntime,
		cfg,
		resources.RabbitMQClient,
		bootstrapschedulers.BuildDependencies(resources.SchedulerFactoryRuntime, cfg, resources.AmazonCrawler, resources.RabbitMQClient),
	)
}
