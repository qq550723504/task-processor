package listing

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/infra/redisclient"
	"task-processor/internal/listingadmin"
	"task-processor/internal/pkg/appenv"

	"github.com/sirupsen/logrus"
)

func RunPausedTaskRecovery(ctx context.Context, opts PausedTaskRecoveryOptions) error {
	platform := strings.ToLower(strings.TrimSpace(opts.Platform))
	if platform == "" {
		return fmt.Errorf("platform is required")
	}

	logger := appenv.SetupLoggerWithLevel(opts.LogLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{Version: opts.Version, BuildTime: opts.BuildTime})
	configPath := opts.ConfigPath()
	logger.Infof("starting %s paused task recovery", platform)
	logger.Infof("config path: %s", configPath)

	cfg, err := config.LoadConfigWithFallback(configPath, logger)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	if err := applyLoggingConfigFromConfig(logger, cfg); err != nil {
		return fmt.Errorf("apply logging config failed: %w", err)
	}

	db, err := database.NewSharedDatabaseFromConfig(cfg.Database)
	if err != nil {
		return fmt.Errorf("initialize recovery database: %w", err)
	}
	defer func() {
		if err := database.CloseSharedDatabase(cfg.Database, db); err != nil {
			logger.WithError(err).Warn("close recovery database failed")
		}
	}()

	runtimePauseReader, closeRuntimePauseReader, err := newRuntimePauseReader(cfg)
	if err != nil {
		return err
	}
	defer closeRuntimePauseReader(logger)

	service := listingadmin.PausedTaskRecoveryService{
		Platform:           platform,
		ImportTasks:        listingadmin.NewGormImportTaskRepository(db),
		Stores:             listingadmin.NewGormStoreRepository(db),
		RuntimePauses:      runtimePauseReader,
		AllowedReasonCodes: opts.AllowedReasonCodes,
		StoreIDs:           opts.StoreIDs,
	}
	plan, err := service.Plan(ctx)
	if err != nil {
		return fmt.Errorf("build recovery plan failed: %w", err)
	}
	writePausedTaskRecoveryPlan(os.Stdout, plan, opts.Execute)
	if !opts.Execute {
		return nil
	}

	result, err := service.Execute(ctx, plan)
	if err != nil {
		return fmt.Errorf("execute recovery failed: %w", err)
	}
	fmt.Fprintf(os.Stdout, "recovered=%d\n", result.Recovered)
	return nil
}

func newRuntimePauseReader(cfg *config.Config) (listingadmin.RuntimePauseReader, func(*logrus.Logger), error) {
	if cfg == nil || cfg.Redis == nil {
		return nil, nil, fmt.Errorf("redis config is required for paused task recovery")
	}
	client, err := redisclient.New(cfg.Redis)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize recovery redis client: %w", err)
	}
	closeFn := func(logger *logrus.Logger) {
		if err := client.Close(); err != nil {
			logger.WithError(err).Warn("close recovery redis client failed")
		}
	}
	return client, closeFn, nil
}

func writePausedTaskRecoveryPlan(w io.Writer, plan listingadmin.PausedTaskRecoveryPlan, execute bool) {
	mode := "dry-run"
	if execute {
		mode = "execute"
	}
	fmt.Fprintf(w, "mode=%s total_paused=%d total_recoverable=%d groups=%d\n", mode, plan.TotalPaused, plan.TotalRecoverable, len(plan.Groups))
	for _, group := range plan.Groups {
		status := "skip"
		if group.Recoverable {
			status = "recover"
		}
		fmt.Fprintf(
			w,
			"%s tenant=%d store=%d name=%q count=%d reason=%s stage=%s completed_today=%d daily_limit=%d runtime_pause=%q skip_reason=%q\n",
			status,
			group.TenantID,
			group.StoreID,
			group.Store.StoreName,
			group.Count,
			group.ReasonCode,
			group.Stage,
			group.Store.CompletedToday,
			group.Store.DailyLimit,
			group.Store.RuntimePauseReason,
			group.SkipReason,
		)
	}
}
