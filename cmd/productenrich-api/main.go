package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/pkg/appenv"
)

var (
	configPath = flag.String("config", "config/config-dev.yaml", "config file path")
	logLevel   = flag.String("log-level", "info", "log level")
	port       = flag.Int("port", 8085, "API service port")
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	logger := appenv.SetupLoggerWithLevel(*logLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	logger.Info("starting productenrich API service")
	logger.Infof("config path: %s", *configPath)
	logger.Infof("API port: %d", *port)

	if err := run(logger); err != nil {
		logger.Fatalf("service startup failed: %v", err)
	}
}

func run(logger *logrus.Logger) error {
	bootstrap, err := buildBootstrap(logger)
	if err != nil {
		return fmt.Errorf("build bootstrap: %w", err)
	}
	defer func() {
		for _, closeFn := range bootstrap.closers {
			if closeFn == nil {
				continue
			}
			if closeErr := closeFn(); closeErr != nil {
				logger.Warnf("close resource failed: %v", closeErr)
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, pool := range bootstrap.pools {
		pool.Start(ctx)
	}
	logger.Infof("worker pools started: %d", len(bootstrap.pools))

	go func() {
		logger.Infof("API service listening on port %d", *port)
		logger.Info("endpoints:")
		logger.Info("  - POST /api/v1/products/generate")
		logger.Info("  - GET  /api/v1/products/tasks/:task_id")
		logger.Info("  - POST /api/v1/images/process")
		logger.Info("  - GET  /api/v1/images/tasks/:task_id")
		logger.Info("  - POST /api/v1/images/tasks/:task_id/review")
		logger.Info("  - GET  /health")
		if listenErr := bootstrap.server.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			logger.Fatalf("HTTP service exited unexpectedly: %v", listenErr)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan
	logger.Infof("received signal %v, shutting down", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := bootstrap.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	cancel()
	for _, pool := range bootstrap.pools {
		pool.Stop(shutdownCtx)
	}
	logger.Info("service shut down gracefully")
	return nil
}
