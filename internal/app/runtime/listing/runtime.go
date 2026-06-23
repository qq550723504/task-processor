package listing

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/app/bootstrap"
	"task-processor/internal/app/consumer"
	"task-processor/internal/core/config"
	"task-processor/internal/pkg/appenv"
	"task-processor/internal/platformbase"

	"github.com/sirupsen/logrus"
)

func Run(ctx context.Context, opts Options) error {
	platform := strings.ToLower(strings.TrimSpace(opts.Platform))
	if platform == "" {
		return fmt.Errorf("platform is required")
	}

	logger := appenv.SetupLoggerWithLevel(opts.LogLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{Version: opts.Version, BuildTime: opts.BuildTime})

	displayName := platformbase.GetPlatformDisplayName(platform)
	configPath := opts.ConfigPath()
	logger.Infof("starting %s listing service", displayName)
	logger.Infof("config path: %s", configPath)

	cfg, err := config.LoadConfigWithFallback(configPath, logger)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	if err := applyLoggingConfigFromConfig(logger, cfg); err != nil {
		return fmt.Errorf("apply logging config failed: %w", err)
	}

	debugTaskID := ResolveDebugTaskID()
	if debugTaskID > 0 {
		return runDebugTask(ctx, cfg, logger, platform, displayName, debugTaskID)
	}

	if cfg.RabbitMQ == nil || !cfg.RabbitMQ.Enabled {
		return fmt.Errorf("RabbitMQ is not enabled")
	}

	serviceManager, err := consumer.NewServiceManager(cfg.RabbitMQ, logger)
	if err != nil {
		return fmt.Errorf("create service manager failed: %w", err)
	}

	consumerDeps := bootstrap.BuildConsumerDependencies()
	processorRegistry := consumer.NewPlatformProcessorRegistry(cfg, logger, platform, consumerDeps)

	module, err := processorRegistry.ResolvePlatformModule(platform)
	if err != nil {
		return err
	}

	if err := processorRegistry.RegisterPlatforms(ctx, serviceManager, platform); err != nil {
		return fmt.Errorf("register %s processor failed: %w", displayName, err)
	}
	if err := validateListingLocalRuntime(platform, processorRegistry.GetManagementClient(), logger); err != nil {
		return err
	}

	runtimeContext := processorRegistry.RuntimeContext(serviceManager, bootstrap.BuildSchedulerDependencies)
	if err := module.ConfigureListingRuntime(ctx, runtimeContext); err != nil {
		return fmt.Errorf("configure %s runtime failed: %w", displayName, err)
	}

	if err := serviceManager.Start(ctx); err != nil {
		return fmt.Errorf("start service manager failed: %w", err)
	}

	logger.Infof("%s listing service started", displayName)
	logHealthEndpoints(logger)
	logger.Info("press Ctrl+C to exit")

	serviceManager.Wait()
	return nil
}

func runDebugTask(
	ctx context.Context,
	cfg *config.Config,
	logger *logrus.Logger,
	platform string,
	displayName string,
	taskID int64,
) error {
	logger.Infof("starting %s debug single-task mode: taskID=%d", displayName, taskID)

	consumerDeps := bootstrap.BuildConsumerDependencies()
	processorRegistry := consumer.NewPlatformProcessorRegistry(cfg, logger, platform, consumerDeps)
	module, err := processorRegistry.ResolvePlatformModule(platform)
	if err != nil {
		return err
	}

	needsAmazon := module.NeedsAmazon(cfg) || consumer.PlatformUsesLocalFetcher(cfg, platform)
	resources, err := consumerDeps.SharedResourceProvider(cfg, logger, needsAmazon)
	if err != nil {
		return fmt.Errorf("initialize shared resources: %w", err)
	}

	rt := consumer.PlatformRuntimeContext{
		Config:                  cfg,
		Logger:                  logger,
		ManagementClient:        resources.ManagementClient,
		RawJSONDataClient:       resources.RawJSONDataClient,
		StoreAPI:                resources.StoreAPI,
		SchedulerRuntime:        resources.SchedulerRuntime,
		SchedulerFactoryRuntime: resources.SchedulerFactoryRuntime,
		ProcessorRuntime:        resources.ProcessorRuntime,
		CrawlSource:             resources.CrawlSource,
		ProductFetcher:          resources.ProductFetcher,
	}
	if err := module.ConfigureListingRuntime(ctx, rt); err != nil {
		return fmt.Errorf("configure %s debug runtime failed: %w", displayName, err)
	}
	if err := validateListingLocalRuntime(platform, resources.ManagementClient, logger); err != nil {
		return err
	}

	registrar := &debugProcessorRegistrar{}
	if err := module.RegisterConsumer(ctx, rt, registrar); err != nil {
		return fmt.Errorf("register %s debug processor failed: %w", displayName, err)
	}
	if registrar.processor == nil {
		return fmt.Errorf("%s debug processor is not available", displayName)
	}

	taskDTO, err := resources.ManagementClient.GetImportTaskClient().GetTaskByID(taskID)
	if err != nil {
		return fmt.Errorf("load debug task %d: %w", taskID, err)
	}
	runner := debugTaskRunner{
		displayName: displayName,
		logger:      logger,
		taskLoader:  staticDebugTaskLoader{task: taskDTO},
		processor:   registrar.processor,
	}
	return runner.run(ctx, taskID)
}

func applyLoggingConfigFromConfig(log *logrus.Logger, cfg *config.Config) error {
	if cfg == nil {
		return nil
	}

	return appenv.ApplyLoggingConfig(log, appenv.LoggingConfig{
		Level:        cfg.Logging.Level,
		Format:       cfg.Logging.Format,
		File:         cfg.Logging.File,
		SplitByLevel: cfg.Logging.SplitByLevel,
	})
}
