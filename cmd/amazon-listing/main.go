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
	configPath = flag.String("config", "config/config-prod.yaml", "config path")
	logLevel   = flag.String("log-level", "info", "log level")
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	logger := appenv.SetupLoggerWithLevel(*logLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{Version: appVersion, BuildTime: buildTime})

	logger.Info("starting Amazon listing service")
	logger.Infof("config path: %s", *configPath)

	cfg, err := config.LoadConfigWithFallback(*configPath, logger)
	if err != nil {
		logger.Fatalf("load config failed: %v", err)
	}
	if cfg.RabbitMQ == nil || !cfg.RabbitMQ.Enabled {
		logger.Fatal("RabbitMQ is not enabled")
	}
	if !cfg.Amazon.Enabled {
		logger.Fatal("Amazon platform is not enabled")
	}

	serviceManager, err := consumer.NewServiceManager(cfg.RabbitMQ, logger)
	if err != nil {
		logger.Fatalf("create service manager failed: %v", err)
	}

	consumerDeps := bootstrap.BuildConsumerDependencies()
	platformRegistry := consumer.NewPlatformRegistry(cfg, logger, "amazon", consumerDeps)
	ctx := context.Background()

	if err := platformRegistry.RegisterAmazonProcessor(ctx, serviceManager); err != nil {
		logger.Fatalf("register Amazon processor failed: %v", err)
	}

	if cfg.RabbitMQ.Node.UseStoreQueues && cfg.Redis != nil {
		provider, providerErr := consumer.NewRedisStoreAssignmentProvider(cfg.Redis, logger)
		if providerErr != nil {
			logger.Fatalf("create dynamic store assignment provider failed: %v", providerErr)
		}
		serviceManager.SetStoreAssignmentProvider(provider)
		logger.Infof("dynamic store assignment provider enabled: nodeID=%s", cfg.RabbitMQ.Node.NodeID)
	}

	if err := serviceManager.Start(ctx); err != nil {
		logger.Fatalf("start service manager failed: %v", err)
	}

	logger.Info("Amazon listing service started")
	logger.Info("health: http://localhost:8081/health")
	logger.Info("ready: http://localhost:8081/ready")
	logger.Info("metrics: http://localhost:8082/metrics")
	logger.Info("press Ctrl+C to exit")

	serviceManager.Wait()
}
