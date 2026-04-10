package main

import (
	"context"
	"flag"

	"task-processor/internal/app/bootstrap"
	"task-processor/internal/app/consumer"
	"task-processor/internal/core/config"
	"task-processor/internal/pkg/appenv"
	"task-processor/internal/prompt"
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

	logger.Info("starting SHEIN listing service")
	logger.Infof("config path: %s", *configPath)

	cfg, err := config.LoadConfigWithFallback(*configPath, logger)
	if err != nil {
		logger.Fatalf("load config failed: %v", err)
	}
	if cfg.RabbitMQ == nil || !cfg.RabbitMQ.Enabled {
		logger.Fatal("RabbitMQ is not enabled")
	}
	if !cfg.Platforms.Shein.Enabled {
		logger.Fatal("SHEIN platform is not enabled")
	}

	serviceManager, err := consumer.NewServiceManager(cfg.RabbitMQ, logger)
	if err != nil {
		logger.Fatalf("create service manager failed: %v", err)
	}

	consumerDeps := bootstrap.BuildConsumerDependencies()
	platformRegistry := consumer.NewPlatformRegistry(cfg, logger, "shein", consumerDeps)
	ctx := context.Background()

	promptsDir := cfg.Prompts.Dir
	if promptsDir == "" {
		promptsDir = "./prompts"
	}
	if err := prompt.InitGlobal(ctx, promptsDir, cfg.Prompts.HotReload, logger.WithField("component", "prompt")); err != nil {
		logger.Warnf("prompt init failed, fallback will be used: %v", err)
	}

	if err := platformRegistry.RegisterSheinProcessor(ctx, serviceManager); err != nil {
		logger.Fatalf("register SHEIN processor failed: %v", err)
	}

	if managementClient := platformRegistry.GetManagementClient(); managementClient != nil {
		serviceManager.SetStoreComponents(
			managementClient.GetStoreClient(),
			cfg.RabbitMQ.Node.OwnedStores,
			nil,
		)
		logger.Info("store dispatch guard initialized")
	} else {
		logger.Warn("management client unavailable; store dispatch guard is disabled")
	}

	if cfg.RabbitMQ.Node.UseStoreQueues && cfg.Redis != nil {
		provider, providerErr := consumer.NewRedisStoreAssignmentProvider(cfg.Redis, logger)
		if providerErr != nil {
			logger.Fatalf("create dynamic store assignment provider failed: %v", providerErr)
		}
		serviceManager.SetStoreAssignmentProvider(provider)
		logger.Infof("dynamic store assignment provider enabled: nodeID=%s", cfg.RabbitMQ.Node.NodeID)
	}
	if cfg.RabbitMQ.AutoShard.Enabled {
		if managementClient := platformRegistry.GetManagementClient(); managementClient != nil && cfg.Redis != nil {
			autoShardService, autoShardErr := consumer.NewAutoShardCoordinator(
				cfg.RabbitMQ.AutoShard,
				managementClient.GetStoreClient(),
				cfg.Redis,
				cfg.RabbitMQ.Node.NodeID,
				logger,
			)
			if autoShardErr != nil {
				logger.Fatalf("create auto shard coordinator failed: %v", autoShardErr)
			}
			serviceManager.SetAutoShardService(autoShardService)
			logger.Infof("auto shard coordinator enabled: platform=%s, candidateNodes=%v", cfg.RabbitMQ.AutoShard.Platform, cfg.RabbitMQ.AutoShard.CandidateNodes)
		} else {
			logger.Warn("auto shard is enabled but management client or redis config is unavailable")
		}
	}

	if err := serviceManager.Start(ctx); err != nil {
		logger.Fatalf("start service manager failed: %v", err)
	}

	logger.Info("SHEIN listing service started")
	logger.Info("health: http://localhost:8081/health")
	logger.Info("ready: http://localhost:8081/ready")
	logger.Info("metrics: http://localhost:8082/metrics")
	logger.Info("press Ctrl+C to exit")

	serviceManager.Wait()
}
