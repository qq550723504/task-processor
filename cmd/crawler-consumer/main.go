package main

import (
	"context"
	"flag"

	"task-processor/internal/app/bootstrap"
	"task-processor/internal/app/consumer"
	"task-processor/internal/core/config"
	"task-processor/internal/pkg/appenv"
)

var (
	appConfig = flag.String("app-config", "config/config-dev.yaml", "application config path")
	logLevel  = flag.String("log-level", "info", "log level")
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	logger := appenv.SetupLoggerWithLevel(*logLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{Version: appVersion, BuildTime: buildTime})
	logger.Info("starting crawler consumer")

	appCfg := config.LoadConfigWithFallback(*appConfig, logger)
	if appCfg.RabbitMQ == nil {
		logger.Fatal("RabbitMQ config is required")
	}

	serviceManager, err := consumer.NewServiceManager(appCfg.RabbitMQ, logger)
	if err != nil {
		logger.Fatalf("create service manager failed: %v", err)
	}

	crawlerDeps := bootstrap.BuildCrawlerDependencies()
	crawlerRegistry := consumer.NewCrawlerRegistryWithDependencies(appCfg, logger, serviceManager.GetClient(), crawlerDeps)
	if err := crawlerRegistry.RegisterCrawlerProcessor(serviceManager, nil); err != nil {
		logger.Fatalf("register crawler processor failed: %v", err)
	}

	ctx := context.Background()
	if err := serviceManager.Start(ctx); err != nil {
		logger.Fatalf("start service manager failed: %v", err)
	}

	logger.Info("crawler consumer started")
	logger.Info("health: http://localhost:8081/health")
	logger.Info("ready: http://localhost:8081/ready")
	logger.Info("metrics: http://localhost:8082/metrics")
	logger.Info("stats: http://localhost:8082/stats")
	logger.Info("press Ctrl+C to exit")

	serviceManager.Wait()
	logger.Info("process exited")
}
