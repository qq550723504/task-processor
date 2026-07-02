package shein

import (
	"context"
	"fmt"

	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	"task-processor/internal/app/consumer"
	"task-processor/internal/app/runner"
	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
	"task-processor/internal/platformbase"
	"task-processor/internal/prompt"
	"task-processor/internal/shein/pipeline"

	"github.com/sirupsen/logrus"
)

type Module struct{}

func NewModule() Module {
	return Module{}
}

func (Module) Name() string {
	return "shein"
}

func (Module) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Platforms.Shein.Enabled
}

func (m Module) NeedsAmazon(cfg *config.Config) bool {
	return platformbase.PlatformUsesLocalFetcher(cfg, m.Name())
}

func (m Module) RegisterConsumer(ctx context.Context, rt consumer.PlatformRuntimeContext, registry consumer.ProcessorRegistrar) error {
	productFetcher := rt.ProductFetcher()
	if productFetcher == nil {
		return fmt.Errorf("SHEIN product fetcher is not configured")
	}

	processor, err := pipeline.NewSheinProcessor(ctx, rt.Config(), rt.Logger(), pipeline.BuildDependencies(ctx, sheinDependencyRuntimeAdapter{ProcessorRuntime: rt.ProcessorRuntime()}, productFetcher, rt.RabbitMQClient()))
	if err != nil {
		return fmt.Errorf("create SHEIN processor: %w", err)
	}
	if err := registry.RegisterProcessor(m.Name(), processor); err != nil {
		return fmt.Errorf("register SHEIN processor: %w", err)
	}
	return nil
}

func (Module) ConfigureListingRuntime(ctx context.Context, rt consumer.PlatformRuntimeContext) error {
	logger := rt.Logger()
	if err := initPrompts(ctx, rt); err != nil {
		logger.Warnf("prompt init failed, fallback will be used: %v", err)
	}
	if err := configureTenantPromptStore(rt, bootstrapresources.NewDBTenantPromptStore); err != nil {
		logger.Warnf("tenant prompt store init failed, tenant overrides disabled: %v", err)
	}

	configureScheduler(ctx, rt)
	configureStoreGuard(rt)
	configureTaskRecoveryWatchdogs(rt)

	cfg := rt.Config()
	if cfg == nil {
		return nil
	}

	if shouldEnableDynamicStoreAssignment(cfg) {
		if err := consumer.EnableDynamicStoreAssignment(cfg, logger, rt.StoreAssignmentRuntime()); err != nil {
			return err
		}
	} else if cfg.RabbitMQ != nil && cfg.RabbitMQ.Node.UseStoreQueues && len(cfg.RabbitMQ.Node.OwnedStores) > 0 {
		logger.Infof("static store assignment enabled: nodeID=%s, ownedStores=%v", cfg.RabbitMQ.Node.NodeID, cfg.RabbitMQ.Node.OwnedStores)
	}

	if shouldConfigureAutoShard(cfg) {
		if err := configureAutoShard(rt); err != nil {
			return err
		}
	}

	return nil
}

func shouldEnableDynamicStoreAssignment(cfg *config.Config) bool {
	if cfg == nil || cfg.RabbitMQ == nil {
		return false
	}
	return cfg.RabbitMQ.Node.UseStoreQueues &&
		len(cfg.RabbitMQ.Node.OwnedStores) == 0 &&
		cfg.Redis != nil &&
		(!cfg.RabbitMQ.AutoShard.Enabled || cfg.RabbitMQ.AutoShard.IsWorker())
}

func shouldConfigureAutoShard(cfg *config.Config) bool {
	return cfg != nil &&
		cfg.RabbitMQ != nil &&
		cfg.RabbitMQ.AutoShard.IsCoordinator()
}

func initPrompts(ctx context.Context, rt consumer.PlatformRuntimeContext) error {
	cfg := rt.Config()
	if cfg == nil {
		return nil
	}
	promptsDir := cfg.Prompts.Dir
	if promptsDir == "" {
		promptsDir = "./prompts"
	}
	return prompt.InitGlobal(ctx, promptsDir, cfg.Prompts.HotReload, rt.Logger().WithField("component", "prompt"))
}

type tenantPromptStoreOpener func(*config.DatabaseConfig, *logrus.Logger) (prompt.TenantPromptStore, func() error, error)

func configureTenantPromptStore(rt consumer.PlatformRuntimeContext, opener tenantPromptStoreOpener) error {
	cfg := rt.Config()
	if cfg == nil || cfg.Database == nil {
		return nil
	}
	if opener == nil {
		return fmt.Errorf("tenant prompt store opener is nil")
	}

	store, _, err := opener(cfg.Database, rt.Logger())
	if err != nil {
		return fmt.Errorf("create tenant prompt store: %w", err)
	}
	if err := prompt.SetTenantPromptStore(store); err != nil {
		return fmt.Errorf("attach tenant prompt store: %w", err)
	}
	return nil
}

func configureScheduler(ctx context.Context, rt consumer.PlatformRuntimeContext) {
	cfg := rt.Config()
	logger := rt.Logger()
	schedulerRuntime := rt.SchedulerServiceRuntime()
	if cfg == nil || schedulerRuntime == nil {
		return
	}
	if !cfg.Processor.SchedulerEnabled {
		if logger != nil {
			logger.Info("processor.schedulerEnabled=false，跳过 SHEIN worker 内置调度服务")
		}
		return
	}
	runtime := rt.SchedulerRuntime()
	if runtime == nil {
		if cfg.Platforms.Shein.SchedulerEnabled {
			logger.Warn("SHEIN scheduler is enabled but scheduler runtime is unavailable")
		}
		return
	}
	if !cfg.Platforms.Shein.SchedulerEnabled && !hasEnabledSheinScheduledTaskConfigs(ctx, runtime, logger) {
		return
	}
	if !rt.HasSchedulerDependenciesBuilder() {
		logger.Warn("SHEIN scheduler dependencies builder is unavailable")
		return
	}

	schedulerService := runner.NewSchedulerServiceWithDependencies(
		logger,
		rt.SchedulerRuntime(),
		cfg,
		schedulerRuntime.GetClient(),
		rt.BuildSchedulerDependencies(schedulerRuntime.GetClient()),
	)
	schedulerRuntime.SetSchedulerService(schedulerService)
	logger.Infof(
		"SHEIN scheduler enabled: autoPricing=%v interval=%ds batchSize=%d",
		cfg.Platforms.Shein.AutoPricing.Enabled,
		cfg.Platforms.Shein.AutoPricing.Interval,
		cfg.Platforms.Shein.AutoPricing.BatchSize,
	)
}

func hasEnabledSheinScheduledTaskConfigs(ctx context.Context, runtime runner.SchedulerRuntimeProvider, logger *logrus.Logger) bool {
	if runtime == nil {
		return false
	}
	for _, taskType := range []appscheduler.TaskType{
		appscheduler.TaskTypePricing,
		appscheduler.TaskTypeProductSync,
		appscheduler.TaskTypeInventory,
		appscheduler.TaskTypeActivity,
	} {
		configs, err := runtime.ListRuntimeScheduledTaskConfigs(ctx, "SHEIN", taskType)
		if err != nil {
			if logger != nil {
				logger.Warnf("SHEIN平台%s任务读取后台配置失败: %v", taskType, err)
			}
			continue
		}
		if len(configs) > 0 {
			if logger != nil {
				logger.Infof("SHEIN schedulerEnabled 未启用，但发现 %d 个%s后台启用配置", len(configs), taskType)
			}
			return true
		}
	}
	return false
}

func configureTaskRecoveryWatchdogs(rt consumer.PlatformRuntimeContext) {
	cfg := rt.Config()
	logger := rt.Logger()
	if cfg == nil || cfg.RabbitMQ == nil || rt.TaskRecoveryRuntime() == nil {
		return
	}
	if !cfg.RabbitMQ.AutoShard.IsCoordinator() {
		return
	}
	if !cfg.RabbitMQ.ProcessingTimeout.Enabled && !cfg.RabbitMQ.StaleQueued.Enabled {
		return
	}
	repo := rt.ListingRuntimeImportTaskRepository()
	if repo == nil {
		logger.Warn("task recovery watchdog is enabled but local import task repository is unavailable")
		return
	}
	configureProcessingTimeoutWatchdog(rt, repo)
	configureStaleQueuedWatchdog(rt, repo)
}

func configureProcessingTimeoutWatchdog(rt consumer.PlatformRuntimeContext, repo consumer.ProcessingTimeoutRepository) {
	cfg := rt.Config()
	logger := rt.Logger()
	taskRecoveryRuntime := rt.TaskRecoveryRuntime()
	if cfg == nil || cfg.RabbitMQ == nil || taskRecoveryRuntime == nil || !cfg.RabbitMQ.ProcessingTimeout.Enabled {
		return
	}
	watchdog := consumer.NewProcessingTimeoutWatchdog(consumer.ProcessingTimeoutWatchdogConfig{
		Enabled:        cfg.RabbitMQ.ProcessingTimeout.Enabled,
		Interval:       cfg.RabbitMQ.ProcessingTimeout.Interval,
		TimeoutMinutes: cfg.RabbitMQ.ProcessingTimeout.TimeoutMinutes,
		RecoveryLimit:  cfg.RabbitMQ.ProcessingTimeout.RecoveryLimit,
		Repository:     repo,
		Logger:         logger,
	})
	taskRecoveryRuntime.SetProcessingTimeoutWatchdog(watchdog)
	logger.Infof(
		"processing timeout watchdog enabled: interval=%s timeoutMinutes=%d recoveryLimit=%d",
		cfg.RabbitMQ.ProcessingTimeout.Interval,
		cfg.RabbitMQ.ProcessingTimeout.TimeoutMinutes,
		cfg.RabbitMQ.ProcessingTimeout.RecoveryLimit,
	)
}

func configureStaleQueuedWatchdog(rt consumer.PlatformRuntimeContext, repo consumer.StaleQueuedRepository) {
	cfg := rt.Config()
	logger := rt.Logger()
	taskRecoveryRuntime := rt.TaskRecoveryRuntime()
	if cfg == nil || cfg.RabbitMQ == nil || taskRecoveryRuntime == nil || !cfg.RabbitMQ.StaleQueued.Enabled {
		return
	}
	watchdog := consumer.NewStaleQueuedWatchdog(consumer.StaleQueuedWatchdogConfig{
		Enabled:        cfg.RabbitMQ.StaleQueued.Enabled,
		Interval:       cfg.RabbitMQ.StaleQueued.Interval,
		TimeoutMinutes: cfg.RabbitMQ.StaleQueued.TimeoutMinutes,
		RecoveryLimit:  cfg.RabbitMQ.StaleQueued.RecoveryLimit,
		Repository:     repo,
		Logger:         logger,
	})
	taskRecoveryRuntime.SetStaleQueuedWatchdog(watchdog)
	logger.Infof(
		"stale queued watchdog enabled: interval=%s timeoutMinutes=%d recoveryLimit=%d",
		cfg.RabbitMQ.StaleQueued.Interval,
		cfg.RabbitMQ.StaleQueued.TimeoutMinutes,
		cfg.RabbitMQ.StaleQueued.RecoveryLimit,
	)
}

func configureStoreGuard(rt consumer.PlatformRuntimeContext) {
	staticStoreGuardRuntime := rt.StaticStoreGuardRuntime()
	cfg := rt.Config()
	logger := rt.Logger()
	if cfg == nil || staticStoreGuardRuntime == nil {
		return
	}
	storeAPI := rt.StoreAPI()
	if storeAPI == nil {
		consumer.ConfigureStaticStoreGuard(cfg, logger, staticStoreGuardRuntime, nil)
		return
	}
	consumer.ConfigureStaticStoreGuard(cfg, logger, staticStoreGuardRuntime, storeAPI)
}

func configureAutoShard(rt consumer.PlatformRuntimeContext) error {
	cfg := rt.Config()
	logger := rt.Logger()
	autoShardRuntime := rt.AutoShardRuntime()
	if cfg == nil || autoShardRuntime == nil {
		return nil
	}
	storeAPI := rt.StoreAPI()
	if storeAPI == nil || cfg.Redis == nil {
		logger.Warn("auto shard is enabled but store API or redis config is unavailable")
		return nil
	}

	autoShardService, err := consumer.NewAutoShardCoordinator(
		cfg.RabbitMQ.AutoShard,
		storeAPI,
		cfg.Redis,
		cfg.RabbitMQ.URL,
		cfg.RabbitMQ.Node.NodeID,
		logger,
	)
	if err != nil {
		return fmt.Errorf("create auto shard coordinator failed: %w", err)
	}
	autoShardRuntime.SetAutoShardService(autoShardService)
	logger.Infof("auto shard coordinator enabled: platform=%s, candidateNodes=%v", cfg.RabbitMQ.AutoShard.Platform, cfg.RabbitMQ.AutoShard.CandidateNodes)
	return nil
}

type sheinDependencyRuntimeAdapter struct {
	consumer.ProcessorRuntime
}

func (a sheinDependencyRuntimeAdapter) GetStoreAPI() listingadmin.StoreAPI {
	if a.ProcessorRuntime == nil {
		return nil
	}
	return a.ProcessorRuntime.GetStoreAPI()
}

func (a sheinDependencyRuntimeAdapter) GetImageDownloader() interface {
	DownloadImage(url string) ([]byte, error)
} {
	if a.ProcessorRuntime == nil {
		return nil
	}
	return a.ProcessorRuntime.GetImageDownloader()
}

func (a sheinDependencyRuntimeAdapter) GetTaskStatus(taskID int64) (*taskstatus.TaskStatusSnapshot, error) {
	if a.ProcessorRuntime == nil {
		return nil, nil
	}
	status, err := a.ProcessorRuntime.GetTaskStatus(taskID)
	if err != nil || status == nil {
		return nil, err
	}
	return status, nil
}
