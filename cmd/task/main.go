package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"task-processor/internal/app/bootstrap"
	"task-processor/internal/infra/monitoring"
	"task-processor/internal/pkg/appenv"

	"github.com/sirupsen/logrus"
)

var (
	appConfig = flag.String("app-config", "config/config-task.yaml", "application config path")
	logLevel  = flag.String("log-level", "info", "log level")
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	monitoring.RecordProcessStartTime()

	logger := appenv.SetupLoggerWithLevel(*logLevel)
	app := bootstrap.NewApplicationBootstrap(logger)

	if err := runApplication(app, logger); err != nil {
		logger.Fatalf("application start failed: %v", err)
	}
}

func runApplication(app *bootstrap.ApplicationBootstrap, logger *logrus.Logger) error {
	versionInfo := appenv.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	}
	appenv.PrintVersionInfo(logger, versionInfo)

	configPath := *appConfig
	if !strings.HasSuffix(configPath, ".yaml") && !strings.HasSuffix(configPath, ".yml") {
		configPath += ".yaml"
		logger.Infof("config path missing extension, completed to %s", configPath)
	}

	if err := app.Initialize(configPath, appVersion); err != nil {
		return err
	}
	if cfg := app.GetConfigManager().GetCurrent(); cfg != nil {
		if err := appenv.ApplyLoggingConfig(logger, appenv.LoggingConfig{
			Level:        cfg.Logging.Level,
			Format:       cfg.Logging.Format,
			File:         cfg.Logging.File,
			SplitByLevel: cfg.Logging.SplitByLevel,
		}); err != nil {
			logger.Warnf("apply logging config failed: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.Start(ctx, appVersion); err != nil {
		return err
	}

	waitForShutdown(logger, cancel)

	return gracefulShutdown(app, logger)
}

func waitForShutdown(logger *logrus.Logger, cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	logger.Infof("received signal: %v, starting graceful shutdown", sig)

	cancel()
}

func gracefulShutdown(app *bootstrap.ApplicationBootstrap, logger *logrus.Logger) error {
	shutdownTimeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	logger.Infof("starting graceful shutdown, timeout: %v", shutdownTimeout)

	done := make(chan error, 1)
	go func() {
		done <- app.Stop(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			logger.Errorf("shutdown failed: %v", err)
			return err
		}
		logger.Info("application shutdown completed")
		return nil
	case <-ctx.Done():
		logger.Warn("shutdown timed out, forcing exit")
		return ctx.Err()
	}
}
