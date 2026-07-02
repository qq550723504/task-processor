package listingcontrol

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/listingadmin"
	controllib "task-processor/internal/listingcontrol"
	"task-processor/internal/pkg/appenv"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type redisRuntime interface {
	controllib.StringRuntime
	leaderLockRuntime
	Close() error
}

type rabbitRuntime interface {
	Publisher() controllib.AMQPPublisher
	QueueDepthSource(platform string) controllib.QueueDepthSource
	Close() error
}

type runtimeDependencies struct {
	LoadConfig        func(configPath string, logger *logrus.Logger) (*config.Config, error)
	OpenDB            func(ctx context.Context, cfg *config.DatabaseConfig) (*gorm.DB, error)
	CloseDB           func(cfg *config.DatabaseConfig, db *gorm.DB) error
	MigrateImportTask func(db *gorm.DB) error
	OpenRedis         func(ctx context.Context, cfg *config.RedisConfig) (redisRuntime, error)
	OpenRabbitMQ      func(ctx context.Context, cfg *config.RabbitMQConfig, logger *logrus.Logger) (rabbitRuntime, error)
}

func defaultRuntimeDependencies() runtimeDependencies {
	return runtimeDependencies{
		LoadConfig: config.LoadConfigWithFallback,
		OpenDB: func(ctx context.Context, cfg *config.DatabaseConfig) (*gorm.DB, error) {
			return database.NewSharedDatabaseFromConfig(cfg)
		},
		CloseDB: database.CloseSharedDatabase,
		MigrateImportTask: func(db *gorm.DB) error {
			return listingadmin.AutoMigrateImportTaskRepository(db)
		},
		OpenRedis: func(ctx context.Context, cfg *config.RedisConfig) (redisRuntime, error) {
			return newRedisStringRuntime(cfg)
		},
		OpenRabbitMQ: func(ctx context.Context, cfg *config.RabbitMQConfig, logger *logrus.Logger) (rabbitRuntime, error) {
			return openRabbitRuntime(ctx, cfg, logger)
		},
	}
}

func Run(ctx context.Context, opts Options) error {
	return runWithDependencies(ctx, opts, defaultRuntimeDependencies())
}

func runWithDependencies(ctx context.Context, opts Options, deps runtimeDependencies) error {
	if deps.LoadConfig == nil {
		deps.LoadConfig = config.LoadConfigWithFallback
	}
	if deps.OpenDB == nil {
		deps.OpenDB = defaultRuntimeDependencies().OpenDB
	}
	if deps.CloseDB == nil {
		deps.CloseDB = defaultRuntimeDependencies().CloseDB
	}
	if deps.MigrateImportTask == nil {
		deps.MigrateImportTask = defaultRuntimeDependencies().MigrateImportTask
	}
	if deps.OpenRedis == nil {
		deps.OpenRedis = defaultRuntimeDependencies().OpenRedis
	}
	if deps.OpenRabbitMQ == nil {
		deps.OpenRabbitMQ = defaultRuntimeDependencies().OpenRabbitMQ
	}

	logger := appenv.SetupLoggerWithLevel(opts.LogLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{Version: opts.Version, BuildTime: opts.BuildTime})

	configPath := opts.ConfigPath()
	logger.Infof("starting listing control-plane service")
	logger.Infof("config path: %s", configPath)

	cfg, err := deps.LoadConfig(configPath, logger)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	if err := applyLoggingConfigFromConfig(logger, cfg); err != nil {
		return fmt.Errorf("apply logging config failed: %w", err)
	}

	controlCfg := cfg.ListingControlPlane
	if !controlCfg.Enabled && !opts.Force {
		logger.Info("listing control-plane disabled by config")
		return nil
	}
	if err := validateRuntimeConfig(cfg); err != nil {
		return err
	}

	platform := normalizePlatform(controlCfg.Platform)
	logger.Infof("listing control-plane platform: %s", platform)

	db, err := deps.OpenDB(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	defer func() {
		if err := deps.CloseDB(cfg.Database, db); err != nil {
			logger.WithError(err).Warn("close shared database failed")
		}
	}()
	if err := deps.MigrateImportTask(db); err != nil {
		return fmt.Errorf("migrate import task repository: %w", err)
	}

	redisRT, err := deps.OpenRedis(ctx, cfg.Redis)
	if err != nil {
		return fmt.Errorf("initialize redis runtime: %w", err)
	}
	defer func() {
		if err := redisRT.Close(); err != nil {
			logger.WithError(err).Warn("close redis runtime failed")
		}
	}()

	rabbitRT, err := deps.OpenRabbitMQ(ctx, cfg.RabbitMQ, logger)
	if err != nil {
		return fmt.Errorf("initialize RabbitMQ runtime: %w", err)
	}
	defer func() {
		if err := rabbitRT.Close(); err != nil {
			logger.WithError(err).Warn("close RabbitMQ runtime failed")
		}
	}()

	repo := listingadmin.NewGormImportTaskRepository(db)
	storeRuntime := controllib.NewStoreRuntime(NewDirectStoreSource(db), redisRT, controllib.StoreRuntimeConfig{
		MaxQueuedPerStore:     controlCfg.MaxQueuedPerStore,
		OwnerBrowserPoolSize:  ownerBrowserPoolSize(cfg),
		EnableLegacyQuotaKeys: controlCfg.EnableLegacyQuotaKeys,
		QueueDepthSource:      rabbitRT.QueueDepthSource(platform),
		DailyUsageSource:      repo,
	})
	recovery := controllib.NewRecoveryCoordinator(recoveryConfig(cfg, repo))
	scheduler := controllib.NewScheduler(repo, storeRuntime, controllib.NewDispatchPublisher(rabbitRT.Publisher(), platform), controllib.SchedulerConfig{
		Platform:      platform,
		BatchSize:     controlCfg.BatchSize,
		PerStoreLimit: controlCfg.PerStoreBurst,
		DryRun:        controlCfg.DryRun,
	})
	pausedTaskRecoveryService := listingadmin.PausedTaskRecoveryService{
		Platform:           platform,
		ImportTasks:        repo,
		Stores:             listingadmin.NewGormStoreRepository(db),
		RuntimePauses:      redisRT,
		AllowedReasonCodes: []string{"STORE_DISPATCH_DISABLED"},
	}

	status := NewStatusTracker(time.Now())
	status.RecordConfig(controlPlaneConfigStatus(controlCfg, platform))
	if err := startStatusServer(ctx, cfg.RabbitMQ.Node.HealthCheckPort, status, logger); err != nil {
		return err
	}

	service := controlPlaneService{
		Recovery: recovery.RunOnce,
		Dispatch: scheduler.DispatchOnce,
		PausedTaskRecovery: newIntervalRunner(pausedTaskRecoveryInterval(controlCfg), func(ctx context.Context) error {
			plan, err := pausedTaskRecoveryService.Plan(ctx)
			if err != nil {
				return err
			}
			result, err := pausedTaskRecoveryService.Execute(ctx, plan)
			if err != nil {
				return err
			}
			logger.WithFields(logrus.Fields{
				"pausedTasks":      plan.TotalPaused,
				"recoverableTasks": plan.TotalRecoverable,
				"recoveredTasks":   result.Recovered,
				"groups":           len(plan.Groups),
			}).Info("listing control-plane paused task recovery completed")
			return nil
		}, time.Now),
		LeaderLock:          newRedisLeaderLock(redisRT, resolveLeaderLockKey(controlCfg, platform), resolveLeaderOwner(cfg), leaderLockTTL(controlCfg)),
		LeaderRenewInterval: leaderRenewInterval(controlCfg),
		ScanInterval:        controlCfg.ScanInterval,
		Logger:              logger,
		Status:              status,
	}
	return service.Run(ctx)
}

func validateRuntimeConfig(cfg *config.Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if cfg.RabbitMQ == nil || !cfg.RabbitMQ.Enabled || strings.TrimSpace(cfg.RabbitMQ.URL) == "" {
		return errors.New("RabbitMQ must be enabled and configured")
	}
	if cfg.Redis == nil || strings.TrimSpace(cfg.Redis.Host) == "" || cfg.Redis.Port <= 0 {
		return errors.New("Redis must be configured")
	}
	if cfg.Database == nil || strings.TrimSpace(cfg.Database.Host) == "" || cfg.Database.Port <= 0 ||
		strings.TrimSpace(cfg.Database.User) == "" || strings.TrimSpace(cfg.Database.Database) == "" {
		return errors.New("database must be configured")
	}
	return nil
}

type controlPlaneService struct {
	Recovery            func(context.Context) (controllib.RecoverySummary, error)
	Dispatch            func(context.Context) (controllib.DispatchSummary, error)
	PausedTaskRecovery  *intervalRunner
	LeaderLock          leaderLock
	LeaderRenewInterval time.Duration
	ScanInterval        time.Duration
	Logger              *logrus.Logger
	Status              *StatusTracker
}

type leaderLock interface {
	Acquire(context.Context) (LeaderSnapshot, bool, error)
}

func (s controlPlaneService) Run(ctx context.Context) error {
	if s.Recovery == nil {
		return errors.New("recovery runner is not configured")
	}
	if s.Dispatch == nil {
		return errors.New("dispatch runner is not configured")
	}
	interval := s.ScanInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}

	if ctx.Err() != nil {
		return nil
	}
	if err := s.runOnce(ctx); err != nil && s.Logger != nil {
		s.Logger.WithError(err).Warn("listing control-plane cycle failed")
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.runOnce(ctx); err != nil && s.Logger != nil {
				s.Logger.WithError(err).Warn("listing control-plane cycle failed")
			}
		}
	}
}

func (s controlPlaneService) runOnce(ctx context.Context) error {
	if s.Status != nil {
		s.Status.BeginCycle(time.Now())
	}
	cycleCtx := ctx
	stopRenewal := func() {}
	if s.LeaderLock != nil {
		leader, acquired, err := s.LeaderLock.Acquire(ctx)
		if err != nil {
			if s.Status != nil {
				s.Status.RecordError(err, time.Now())
			}
			return err
		}
		if !acquired {
			if s.Status != nil {
				s.Status.RecordStandby(leader, time.Now())
			}
			if s.Logger != nil {
				s.Logger.WithFields(logrus.Fields{
					"leaderKey":   leader.Key,
					"leaderOwner": leader.Owner,
				}).Info("listing control-plane standby; leader lock is held by another instance")
			}
			return nil
		}
		if s.Status != nil {
			s.Status.RecordLeader(leader)
		}
		var cancel context.CancelFunc
		cycleCtx, cancel = context.WithCancel(ctx)
		done := make(chan struct{})
		stopRenewal = func() {
			close(done)
			cancel()
		}
		go s.renewLeaderUntilDone(cycleCtx, done, cancel)
	}
	defer stopRenewal()

	if err := s.PausedTaskRecovery.RunIfDue(cycleCtx); err != nil {
		if s.Status != nil {
			s.Status.RecordError(err, time.Now())
		}
		return err
	}

	recoverySummary, err := s.Recovery(cycleCtx)
	if err != nil {
		if s.Status != nil {
			s.Status.RecordError(err, time.Now())
		}
		return err
	}
	dispatchSummary, err := s.Dispatch(cycleCtx)
	if err != nil {
		if s.Status != nil {
			s.Status.RecordError(err, time.Now())
		}
		return err
	}
	if s.Status != nil {
		s.Status.RecordSuccess(recoverySummary, dispatchSummary, time.Now())
	}
	if s.Logger != nil {
		s.Logger.WithFields(logrus.Fields{
			"processingRecovered":  recoverySummary.ProcessingRecovered,
			"staleQueuedRecovered": recoverySummary.StaleQueuedRecovered,
			"dispatchCandidates":   dispatchSummary.Candidates,
			"dispatched":           dispatchSummary.Dispatched,
			"skipped":              dispatchSummary.Skipped,
			"failed":               dispatchSummary.Failed,
		}).Info("listing control-plane cycle completed")
	}
	return nil
}

func (s controlPlaneService) renewLeaderUntilDone(ctx context.Context, done <-chan struct{}, cancel context.CancelFunc) {
	interval := s.LeaderRenewInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			leader, acquired, err := s.LeaderLock.Acquire(ctx)
			if err != nil {
				if s.Status != nil {
					s.Status.RecordError(err, time.Now())
				}
				if s.Logger != nil {
					s.Logger.WithError(err).Warn("listing control-plane leader lock renewal failed")
				}
				cancel()
				return
			}
			if !acquired {
				if s.Status != nil {
					s.Status.RecordStandby(leader, time.Now())
				}
				if s.Logger != nil {
					s.Logger.WithFields(logrus.Fields{
						"leaderKey":   leader.Key,
						"leaderOwner": leader.Owner,
					}).Warn("listing control-plane leader lock lost")
				}
				cancel()
				return
			}
			if s.Status != nil {
				s.Status.RecordLeader(leader)
			}
		}
	}
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

func normalizePlatform(platform string) string {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return "shein"
	}
	return platform
}

func ownerBrowserPoolSize(cfg *config.Config) int {
	if cfg == nil {
		return 1
	}
	if cfg.Browser.PoolSize > 0 {
		return cfg.Browser.PoolSize
	}
	if cfg.Worker.Concurrency > 0 {
		return cfg.Worker.Concurrency
	}
	return 1
}

func recoveryConfig(cfg *config.Config, repo controllib.RecoveryRepository) controllib.RecoveryConfig {
	rabbitCfg := cfg.RabbitMQ
	processing := rabbitCfg.ProcessingTimeout
	stale := rabbitCfg.StaleQueued
	return controllib.RecoveryConfig{
		Enabled:                   processing.Enabled || stale.Enabled,
		ProcessingTimeoutEnabled:  processing.Enabled,
		ProcessingTimeoutMinutes:  processing.TimeoutMinutes,
		ProcessingRecoveryLimit:   processing.RecoveryLimit,
		StaleQueuedEnabled:        stale.Enabled,
		StaleQueuedTimeoutMinutes: stale.TimeoutMinutes,
		StaleQueuedRecoveryLimit:  stale.RecoveryLimit,
		Repository:                repo,
	}
}

func pausedTaskRecoveryInterval(controlCfg config.ListingControlPlaneConfig) time.Duration {
	if controlCfg.PausedTaskRecoveryInterval < 0 {
		return 0
	}
	if controlCfg.PausedTaskRecoveryInterval == 0 {
		return time.Hour
	}
	return controlCfg.PausedTaskRecoveryInterval
}

type realRabbitRuntime struct {
	manager   *rabbitmq.ConnectionManager
	client    *rabbitmq.Client
	publisher controllib.AMQPPublisher
}

func openRabbitRuntime(ctx context.Context, cfg *config.RabbitMQConfig, logger *logrus.Logger) (*realRabbitRuntime, error) {
	if cfg == nil {
		return nil, errors.New("RabbitMQ config is nil")
	}
	connectionConfig := rabbitmq.ConnectionConfig{
		URL:               cfg.URL,
		ReconnectInterval: cfg.ReconnectInterval,
		MaxReconnectTries: cfg.MaxReconnectTries,
	}
	manager := rabbitmq.NewConnectionManager(connectionConfig, logger)
	if err := manager.Connect(ctx); err != nil {
		return nil, err
	}
	client := rabbitmq.NewClient(manager, logger)
	publisher, err := manager.CreateChannel()
	if err != nil {
		_ = manager.Close()
		return nil, err
	}
	return &realRabbitRuntime{
		manager: manager,
		client:  client,
		publisher: newRefreshableAMQPPublisher(publisher, func() (controllib.AMQPPublisher, error) {
			return manager.CreateChannel()
		}),
	}, nil
}

func (r *realRabbitRuntime) Publisher() controllib.AMQPPublisher {
	return r.publisher
}

func (r *realRabbitRuntime) QueueDepthSource(platform string) controllib.QueueDepthSource {
	return newRabbitQueueDepthSource(r.inspectQueue, r.client, platform)
}

func (r *realRabbitRuntime) inspectQueue(name string) (amqp.Queue, error) {
	if r == nil || r.manager == nil {
		return amqp.Queue{}, errors.New("RabbitMQ connection manager is not configured")
	}
	channel, err := r.manager.CreateChannel()
	if err != nil {
		return amqp.Queue{}, err
	}
	defer channel.Close()
	return channel.QueueInspect(name)
}

func (r *realRabbitRuntime) Close() error {
	if r == nil {
		return nil
	}
	var err error
	if r.publisher != nil {
		if closer, ok := r.publisher.(interface{ Close() error }); ok {
			err = errors.Join(err, closer.Close())
		}
	}
	if r.client != nil {
		err = errors.Join(err, r.client.Close())
	} else if r.manager != nil {
		err = errors.Join(err, r.manager.Close())
	}
	return err
}

type refreshableAMQPPublisher struct {
	mu      sync.Mutex
	current controllib.AMQPPublisher
	refresh func() (controllib.AMQPPublisher, error)
}

func newRefreshableAMQPPublisher(initial controllib.AMQPPublisher, refresh func() (controllib.AMQPPublisher, error)) *refreshableAMQPPublisher {
	return &refreshableAMQPPublisher{
		current: initial,
		refresh: refresh,
	}
}

func (p *refreshableAMQPPublisher) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	if p == nil {
		return errors.New("RabbitMQ publisher is nil")
	}
	firstErr := p.publish(ctx, exchange, key, mandatory, immediate, msg)
	if firstErr == nil || !isClosedAMQPPublishError(firstErr) {
		return firstErr
	}
	if err := p.refreshChannel(); err != nil {
		return fmt.Errorf("%w; refresh RabbitMQ publish channel: %v", firstErr, err)
	}
	return p.publish(ctx, exchange, key, mandatory, immediate, msg)
}

func (p *refreshableAMQPPublisher) publish(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	p.mu.Lock()
	current := p.current
	p.mu.Unlock()
	if current == nil {
		return errors.New("RabbitMQ publisher channel is nil")
	}
	return current.PublishWithContext(ctx, exchange, key, mandatory, immediate, msg)
}

func (p *refreshableAMQPPublisher) refreshChannel() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.refresh == nil {
		return errors.New("RabbitMQ publisher refresh function is nil")
	}
	next, err := p.refresh()
	if err != nil {
		return err
	}
	if next == nil {
		return errors.New("RabbitMQ publisher refresh returned nil channel")
	}
	if closer, ok := p.current.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
	p.current = next
	return nil
}

func (p *refreshableAMQPPublisher) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if closer, ok := p.current.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

func isClosedAMQPPublishError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "channel/connection is not open") ||
		strings.Contains(message, "channel is closed") ||
		strings.Contains(message, "connection is closed") ||
		strings.Contains(message, "rabbitmq连接不可用") ||
		strings.Contains(message, "rabbitmq通道不可用")
}
