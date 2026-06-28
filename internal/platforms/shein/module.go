package shein

import (
	"context"
	"fmt"

	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	"task-processor/internal/app/consumer"
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
	"task-processor/internal/prompt"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/taskstatus"

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
	return consumer.PlatformUsesLocalFetcher(cfg, m.Name())
}

func (m Module) RegisterConsumer(ctx context.Context, rt consumer.PlatformRuntimeContext, registry consumer.ProcessorRegistrar) error {
	productFetcher := rt.ProductFetcher
	if productFetcher == nil {
		return fmt.Errorf("SHEIN product fetcher is not configured")
	}

	processor, err := pipeline.NewSheinProcessor(ctx, rt.Config, rt.Logger, pipeline.BuildDependencies(ctx, sheinDependencyRuntimeAdapter{ProcessorRuntime: rt.ProcessorRuntime}, productFetcher, rt.RabbitMQClient))
	if err != nil {
		return fmt.Errorf("create SHEIN processor: %w", err)
	}
	if err := registry.RegisterProcessor(m.Name(), processor); err != nil {
		return fmt.Errorf("register SHEIN processor: %w", err)
	}
	return nil
}

func (Module) ConfigureListingRuntime(ctx context.Context, rt consumer.PlatformRuntimeContext) error {
	if err := initPrompts(ctx, rt); err != nil {
		rt.Logger.Warnf("prompt init failed, fallback will be used: %v", err)
	}
	if err := configureTenantPromptStore(rt, bootstrapresources.NewDBTenantPromptStore); err != nil {
		rt.Logger.Warnf("tenant prompt store init failed, tenant overrides disabled: %v", err)
	}

	configureScheduler(rt)
	configureStoreGuard(rt)
	configureTaskRecoveryWatchdogs(rt)

	cfg := rt.Config
	if cfg == nil {
		return nil
	}

	if shouldEnableDynamicStoreAssignment(cfg) {
		if err := consumer.EnableDynamicStoreAssignment(cfg, rt.Logger, rt.ServiceManager); err != nil {
			return err
		}
	} else if cfg.RabbitMQ != nil && cfg.RabbitMQ.Node.UseStoreQueues && len(cfg.RabbitMQ.Node.OwnedStores) > 0 {
		rt.Logger.Infof("static store assignment enabled: nodeID=%s, ownedStores=%v", cfg.RabbitMQ.Node.NodeID, cfg.RabbitMQ.Node.OwnedStores)
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
	if rt.Config == nil {
		return nil
	}
	promptsDir := rt.Config.Prompts.Dir
	if promptsDir == "" {
		promptsDir = "./prompts"
	}
	return prompt.InitGlobal(ctx, promptsDir, rt.Config.Prompts.HotReload, rt.Logger.WithField("component", "prompt"))
}

type tenantPromptStoreOpener func(*config.DatabaseConfig, *logrus.Logger) (prompt.TenantPromptStore, func() error, error)

func configureTenantPromptStore(rt consumer.PlatformRuntimeContext, opener tenantPromptStoreOpener) error {
	if rt.Config == nil || rt.Config.Database == nil {
		return nil
	}
	if opener == nil {
		return fmt.Errorf("tenant prompt store opener is nil")
	}

	store, _, err := opener(rt.Config.Database, rt.Logger)
	if err != nil {
		return fmt.Errorf("create tenant prompt store: %w", err)
	}
	if err := prompt.SetTenantPromptStore(store); err != nil {
		return fmt.Errorf("attach tenant prompt store: %w", err)
	}
	return nil
}

func configureScheduler(rt consumer.PlatformRuntimeContext) {
	cfg := rt.Config
	if cfg == nil || rt.ServiceManager == nil || !cfg.Platforms.Shein.SchedulerEnabled {
		return
	}
	if rt.SchedulerRuntime == nil {
		rt.Logger.Warn("SHEIN scheduler is enabled but scheduler runtime is unavailable")
		return
	}
	if rt.SchedulerBuilder == nil {
		rt.Logger.Warn("SHEIN scheduler dependencies builder is unavailable")
		return
	}

	schedulerService := runner.NewSchedulerServiceWithDependencies(
		rt.Logger,
		rt.SchedulerRuntime,
		cfg,
		rt.ServiceManager.GetClient(),
		rt.SchedulerBuilder(rt.SchedulerFactoryRuntime, cfg, rt.CrawlSource, rt.ServiceManager.GetClient()),
	)
	rt.ServiceManager.SetSchedulerService(schedulerService)
	rt.Logger.Infof(
		"SHEIN scheduler enabled: autoPricing=%v interval=%ds batchSize=%d",
		cfg.Platforms.Shein.AutoPricing.Enabled,
		cfg.Platforms.Shein.AutoPricing.Interval,
		cfg.Platforms.Shein.AutoPricing.BatchSize,
	)
}

func configureTaskRecoveryWatchdogs(rt consumer.PlatformRuntimeContext) {
	cfg := rt.Config
	if cfg == nil || cfg.RabbitMQ == nil || rt.ServiceManager == nil {
		return
	}
	if !cfg.RabbitMQ.AutoShard.IsCoordinator() {
		return
	}
	if !cfg.RabbitMQ.ProcessingTimeout.Enabled && !cfg.RabbitMQ.StaleQueued.Enabled {
		return
	}
	repo := rt.ListingRuntimeImportTaskRepository
	if repo == nil {
		rt.Logger.Warn("task recovery watchdog is enabled but local import task repository is unavailable")
		return
	}
	configureProcessingTimeoutWatchdog(rt, repo)
	configureStaleQueuedWatchdog(rt, repo)
}

func configureProcessingTimeoutWatchdog(rt consumer.PlatformRuntimeContext, repo consumer.ProcessingTimeoutRepository) {
	cfg := rt.Config
	if cfg == nil || cfg.RabbitMQ == nil || rt.ServiceManager == nil || !cfg.RabbitMQ.ProcessingTimeout.Enabled {
		return
	}
	watchdog := consumer.NewProcessingTimeoutWatchdog(consumer.ProcessingTimeoutWatchdogConfig{
		Enabled:        cfg.RabbitMQ.ProcessingTimeout.Enabled,
		Interval:       cfg.RabbitMQ.ProcessingTimeout.Interval,
		TimeoutMinutes: cfg.RabbitMQ.ProcessingTimeout.TimeoutMinutes,
		RecoveryLimit:  cfg.RabbitMQ.ProcessingTimeout.RecoveryLimit,
		Repository:     repo,
		Logger:         rt.Logger,
	})
	rt.ServiceManager.SetProcessingTimeoutWatchdog(watchdog)
	rt.Logger.Infof(
		"processing timeout watchdog enabled: interval=%s timeoutMinutes=%d recoveryLimit=%d",
		cfg.RabbitMQ.ProcessingTimeout.Interval,
		cfg.RabbitMQ.ProcessingTimeout.TimeoutMinutes,
		cfg.RabbitMQ.ProcessingTimeout.RecoveryLimit,
	)
}

func configureStaleQueuedWatchdog(rt consumer.PlatformRuntimeContext, repo consumer.StaleQueuedRepository) {
	cfg := rt.Config
	if cfg == nil || cfg.RabbitMQ == nil || rt.ServiceManager == nil || !cfg.RabbitMQ.StaleQueued.Enabled {
		return
	}
	watchdog := consumer.NewStaleQueuedWatchdog(consumer.StaleQueuedWatchdogConfig{
		Enabled:        cfg.RabbitMQ.StaleQueued.Enabled,
		Interval:       cfg.RabbitMQ.StaleQueued.Interval,
		TimeoutMinutes: cfg.RabbitMQ.StaleQueued.TimeoutMinutes,
		RecoveryLimit:  cfg.RabbitMQ.StaleQueued.RecoveryLimit,
		Repository:     repo,
		Logger:         rt.Logger,
	})
	rt.ServiceManager.SetStaleQueuedWatchdog(watchdog)
	rt.Logger.Infof(
		"stale queued watchdog enabled: interval=%s timeoutMinutes=%d recoveryLimit=%d",
		cfg.RabbitMQ.StaleQueued.Interval,
		cfg.RabbitMQ.StaleQueued.TimeoutMinutes,
		cfg.RabbitMQ.StaleQueued.RecoveryLimit,
	)
}

func configureStoreGuard(rt consumer.PlatformRuntimeContext) {
	if rt.Config == nil || rt.ServiceManager == nil {
		return
	}
	if rt.StoreAPI == nil {
		consumer.ConfigureStaticStoreGuard(rt.Config, rt.Logger, rt.ServiceManager, nil)
		return
	}
	consumer.ConfigureStaticStoreGuard(rt.Config, rt.Logger, rt.ServiceManager, rt.StoreAPI)
}

func configureAutoShard(rt consumer.PlatformRuntimeContext) error {
	cfg := rt.Config
	if cfg == nil || rt.ServiceManager == nil {
		return nil
	}
	if rt.StoreAPI == nil || cfg.Redis == nil {
		rt.Logger.Warn("auto shard is enabled but store API or redis config is unavailable")
		return nil
	}

	autoShardService, err := consumer.NewAutoShardCoordinator(
		cfg.RabbitMQ.AutoShard,
		rt.StoreAPI,
		cfg.Redis,
		cfg.RabbitMQ.URL,
		cfg.RabbitMQ.Node.NodeID,
		rt.Logger,
	)
	if err != nil {
		return fmt.Errorf("create auto shard coordinator failed: %w", err)
	}
	rt.ServiceManager.SetAutoShardService(autoShardService)
	rt.Logger.Infof("auto shard coordinator enabled: platform=%s, candidateNodes=%v", cfg.RabbitMQ.AutoShard.Platform, cfg.RabbitMQ.AutoShard.CandidateNodes)
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
