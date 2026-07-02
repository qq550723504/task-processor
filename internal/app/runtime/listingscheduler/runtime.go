package listingscheduler

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	bootstrapschedulers "task-processor/internal/app/bootstrap/schedulers"
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/pkg/appenv"

	"github.com/sirupsen/logrus"
)

type runtimeDependencies struct {
	LoadConfig      func(configPath string, logger *logrus.Logger) (*config.Config, error)
	BuildResources  func(cfg *config.Config, logger *logrus.Logger) (bootstrapresources.SharedResources, error)
	NewScheduler    func(logger *logrus.Logger, cfg *config.Config, resources bootstrapresources.SharedResources) runner.SchedulerService
	WaitForShutdown func(ctx context.Context)
}

func defaultRuntimeDependencies() runtimeDependencies {
	return runtimeDependencies{
		LoadConfig: config.LoadConfigWithFallback,
		BuildResources: func(cfg *config.Config, logger *logrus.Logger) (bootstrapresources.SharedResources, error) {
			return bootstrapresources.BuildSharedResources(cfg, logger, bootstrapresources.SharedResourceOptions{})
		},
		NewScheduler: func(logger *logrus.Logger, cfg *config.Config, resources bootstrapresources.SharedResources) runner.SchedulerService {
			schedulerResources := resources.Scheduler()
			return runner.NewSchedulerServiceWithDependencies(
				logger,
				schedulerResources.Runtime(),
				cfg,
				resources.RabbitMQClient(),
				bootstrapschedulers.BuildDependencies(
					schedulerResources.FactoryRuntime(),
					cfg,
					schedulerResources.CrawlSource(),
					resources.RabbitMQClient(),
				),
			)
		},
		WaitForShutdown: waitForShutdownSignal,
	}
}

func Run(ctx context.Context, opts Options) error {
	return runWithDependencies(ctx, opts, defaultRuntimeDependencies())
}

func runWithDependencies(ctx context.Context, opts Options, deps runtimeDependencies) error {
	defaults := defaultRuntimeDependencies()
	if deps.LoadConfig == nil {
		deps.LoadConfig = defaults.LoadConfig
	}
	if deps.BuildResources == nil {
		deps.BuildResources = defaults.BuildResources
	}
	if deps.NewScheduler == nil {
		deps.NewScheduler = defaults.NewScheduler
	}
	if deps.WaitForShutdown == nil {
		deps.WaitForShutdown = defaults.WaitForShutdown
	}

	logger := appenv.SetupLoggerWithLevel(opts.LogLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{Version: opts.Version, BuildTime: opts.BuildTime})

	configPath := opts.ConfigPath()
	logger.Infof("starting listing scheduler service")
	logger.Infof("config path: %s", configPath)

	cfg, err := deps.LoadConfig(configPath, logger)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	if err := applyLoggingConfigFromConfig(logger, cfg); err != nil {
		return fmt.Errorf("apply logging config failed: %w", err)
	}

	resources, err := deps.BuildResources(cfg, logger)
	if err != nil {
		return fmt.Errorf("initialize scheduler resources: %w", err)
	}
	schedulerService := deps.NewScheduler(logger, cfg, resources)
	if schedulerService == nil {
		return fmt.Errorf("scheduler service is not configured")
	}
	if err := schedulerService.Start(ctx); err != nil {
		return fmt.Errorf("start scheduler service: %w", err)
	}
	defer func() {
		if err := schedulerService.Stop(context.Background()); err != nil {
			logger.WithError(err).Warn("stop scheduler service failed")
		}
	}()

	logger.Info("listing scheduler service started")
	deps.WaitForShutdown(ctx)
	return nil
}

func waitForShutdownSignal(ctx context.Context) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
	case <-sigCh:
	}
}

func applyLoggingConfigFromConfig(logger *logrus.Logger, cfg *config.Config) error {
	if logger == nil || cfg == nil {
		return nil
	}
	return appenv.ApplyLoggingConfig(logger, appenv.LoggingConfig{
		Level:        cfg.Logging.Level,
		Format:       cfg.Logging.Format,
		File:         cfg.Logging.File,
		SplitByLevel: cfg.Logging.SplitByLevel,
	})
}
