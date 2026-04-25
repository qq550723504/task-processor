package listing

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/app/bootstrap"
	"task-processor/internal/app/consumer"
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/pkg/appenv"
	"task-processor/internal/prompt"

	"github.com/sirupsen/logrus"
)

func Run(ctx context.Context, opts Options) error {
	platform := strings.ToLower(strings.TrimSpace(opts.Platform))
	if platform == "" {
		return fmt.Errorf("platform is required")
	}

	logger := appenv.SetupLoggerWithLevel(opts.LogLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{Version: opts.Version, BuildTime: opts.BuildTime})

	displayName := platformDisplayName(platform)
	configPath := opts.ConfigPath()
	logger.Infof("starting %s listing service", displayName)
	logger.Infof("config path: %s", configPath)

	cfg, err := config.LoadConfigWithFallback(configPath, logger)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	if err := validatePlatformEnabled(cfg, platform); err != nil {
		return err
	}

	serviceManager, err := consumer.NewServiceManager(cfg.RabbitMQ, logger)
	if err != nil {
		return fmt.Errorf("create service manager failed: %w", err)
	}

	consumerDeps := bootstrap.BuildConsumerDependencies()
	processorRegistry := consumer.NewPlatformProcessorRegistry(cfg, logger, platform, consumerDeps)

	if err := initPrompts(ctx, cfg, platform, logger); err != nil {
		logger.Warnf("prompt init failed, fallback will be used: %v", err)
	}

	if err := registerProcessor(ctx, platform, processorRegistry, serviceManager); err != nil {
		return err
	}

	if err := configurePlatformServices(platform, cfg, logger, processorRegistry, serviceManager); err != nil {
		return err
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

func validatePlatformEnabled(cfg *config.Config, platform string) error {
	if cfg.RabbitMQ == nil || !cfg.RabbitMQ.Enabled {
		return fmt.Errorf("RabbitMQ is not enabled")
	}

	switch platform {
	case "amazon":
		if !cfg.Amazon.Enabled {
			return fmt.Errorf("Amazon platform is not enabled")
		}
	case "shein":
		if !cfg.Platforms.Shein.Enabled {
			return fmt.Errorf("SHEIN platform is not enabled")
		}
	case "temu":
		if !cfg.Platforms.Temu.Enabled {
			return fmt.Errorf("TEMU platform is not enabled")
		}
	default:
		return fmt.Errorf("unsupported platform: %s", platform)
	}

	return nil
}

func initPrompts(ctx context.Context, cfg *config.Config, platform string, logger *logrus.Logger) error {
	if platform != "shein" && platform != "temu" {
		return nil
	}

	promptsDir := cfg.Prompts.Dir
	if promptsDir == "" {
		promptsDir = "./prompts"
	}
	return prompt.InitGlobal(ctx, promptsDir, cfg.Prompts.HotReload, logger.WithField("component", "prompt"))
}

func registerProcessor(ctx context.Context, platform string, registry *consumer.PlatformProcessorRegistry, serviceManager *consumer.ServiceManager) error {
	switch platform {
	case "amazon":
		if err := registry.RegisterAmazonProcessor(ctx, serviceManager); err != nil {
			return fmt.Errorf("register Amazon processor failed: %w", err)
		}
	case "shein":
		if err := registry.RegisterSheinProcessor(ctx, serviceManager); err != nil {
			return fmt.Errorf("register SHEIN processor failed: %w", err)
		}
	case "temu":
		if err := registry.RegisterTemuProcessor(ctx, serviceManager); err != nil {
			return fmt.Errorf("register TEMU processor failed: %w", err)
		}
	default:
		return fmt.Errorf("unsupported platform: %s", platform)
	}
	return nil
}

func configurePlatformServices(
	platform string,
	cfg *config.Config,
	logger *logrus.Logger,
	registry *consumer.PlatformProcessorRegistry,
	serviceManager *consumer.ServiceManager,
) error {
	if platform == "shein" {
		configureSheinScheduler(cfg, logger, registry, serviceManager)
		configureSheinStoreGuard(cfg, logger, registry, serviceManager)
		return configureSheinStoreAssignment(cfg, logger, registry, serviceManager)
	}

	return configureDynamicStoreAssignment(cfg, logger, serviceManager)
}

func configureSheinScheduler(
	cfg *config.Config,
	logger *logrus.Logger,
	registry *consumer.PlatformProcessorRegistry,
	serviceManager *consumer.ServiceManager,
) {
	if !cfg.Platforms.Shein.SchedulerEnabled {
		return
	}

	managementClient := registry.GetManagementClient()
	if managementClient == nil {
		logger.Warn("SHEIN scheduler is enabled but management client is unavailable")
		return
	}

	schedulerService := runner.NewSchedulerServiceWithDependencies(
		logger,
		managementClient,
		cfg,
		serviceManager.GetClient(),
		bootstrap.BuildSchedulerDependencies(
			managementClient,
			cfg,
			registry.GetSharedAmazonProcessor(),
			serviceManager.GetClient(),
		),
	)
	serviceManager.SetSchedulerService(schedulerService)
	logger.Infof(
		"SHEIN scheduler enabled: autoPricing=%v interval=%ds batchSize=%d",
		cfg.Platforms.Shein.AutoPricing.Enabled,
		cfg.Platforms.Shein.AutoPricing.Interval,
		cfg.Platforms.Shein.AutoPricing.BatchSize,
	)
}

func configureSheinStoreGuard(
	cfg *config.Config,
	logger *logrus.Logger,
	registry *consumer.PlatformProcessorRegistry,
	serviceManager *consumer.ServiceManager,
) {
	managementClient := registry.GetManagementClient()
	if managementClient == nil {
		logger.Warn("management client unavailable; store dispatch guard is disabled")
		return
	}

	serviceManager.SetStoreComponents(
		managementClient.GetStoreClient(),
		cfg.RabbitMQ.Node.OwnedStores,
		nil,
	)
	logger.Info("store dispatch guard initialized")
}

func configureSheinStoreAssignment(
	cfg *config.Config,
	logger *logrus.Logger,
	registry *consumer.PlatformProcessorRegistry,
	serviceManager *consumer.ServiceManager,
) error {
	if cfg.RabbitMQ.Node.UseStoreQueues && len(cfg.RabbitMQ.Node.OwnedStores) == 0 && cfg.Redis != nil {
		if err := setRedisStoreAssignmentProvider(cfg, logger, serviceManager); err != nil {
			return err
		}
		logger.Infof("dynamic store assignment provider enabled: nodeID=%s", cfg.RabbitMQ.Node.NodeID)
	} else if cfg.RabbitMQ.Node.UseStoreQueues && len(cfg.RabbitMQ.Node.OwnedStores) > 0 {
		logger.Infof("static store assignment enabled: nodeID=%s, ownedStores=%v", cfg.RabbitMQ.Node.NodeID, cfg.RabbitMQ.Node.OwnedStores)
	}

	if cfg.RabbitMQ.AutoShard.Enabled {
		if err := configureAutoShard(cfg, logger, registry, serviceManager); err != nil {
			return err
		}
	}

	return nil
}

func configureDynamicStoreAssignment(cfg *config.Config, logger *logrus.Logger, serviceManager *consumer.ServiceManager) error {
	if cfg.RabbitMQ.Node.UseStoreQueues && cfg.Redis != nil {
		if err := setRedisStoreAssignmentProvider(cfg, logger, serviceManager); err != nil {
			return err
		}
		logger.Infof("dynamic store assignment provider enabled: nodeID=%s", cfg.RabbitMQ.Node.NodeID)
	}
	return nil
}

func setRedisStoreAssignmentProvider(cfg *config.Config, logger *logrus.Logger, serviceManager *consumer.ServiceManager) error {
	provider, err := consumer.NewRedisStoreAssignmentProvider(cfg.Redis, logger)
	if err != nil {
		return fmt.Errorf("create dynamic store assignment provider failed: %w", err)
	}
	serviceManager.SetStoreAssignmentProvider(provider)
	return nil
}

func configureAutoShard(
	cfg *config.Config,
	logger *logrus.Logger,
	registry *consumer.PlatformProcessorRegistry,
	serviceManager *consumer.ServiceManager,
) error {
	managementClient := registry.GetManagementClient()
	if managementClient == nil || cfg.Redis == nil {
		logger.Warn("auto shard is enabled but management client or redis config is unavailable")
		return nil
	}

	autoShardService, err := consumer.NewAutoShardCoordinator(
		cfg.RabbitMQ.AutoShard,
		managementClient.GetStoreClient(),
		cfg.Redis,
		cfg.RabbitMQ.URL,
		cfg.RabbitMQ.Node.NodeID,
		logger,
	)
	if err != nil {
		return fmt.Errorf("create auto shard coordinator failed: %w", err)
	}
	serviceManager.SetAutoShardService(autoShardService)
	logger.Infof("auto shard coordinator enabled: platform=%s, candidateNodes=%v", cfg.RabbitMQ.AutoShard.Platform, cfg.RabbitMQ.AutoShard.CandidateNodes)
	return nil
}

func platformDisplayName(platform string) string {
	switch platform {
	case "amazon":
		return "Amazon"
	case "shein":
		return "SHEIN"
	case "temu":
		return "TEMU"
	default:
		return strings.ToUpper(platform)
	}
}
