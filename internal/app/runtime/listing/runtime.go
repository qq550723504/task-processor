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
