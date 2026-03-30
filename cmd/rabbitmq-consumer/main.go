package main

import (
	"context"
	"flag"
	"strings"

	"task-processor/internal/app/bootstrap"
	"task-processor/internal/app/consumer"
	"task-processor/internal/app/runner"
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
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	logger.Info("starting RabbitMQ consumer")
	logger.Infof("config path: %s", *appConfig)

	configPath := *appConfig
	if !strings.HasSuffix(configPath, ".yaml") && !strings.HasSuffix(configPath, ".yml") {
		configPath += ".yaml"
		logger.Infof("config path missing extension, completed to %s", configPath)
	}

	appCfg, err := config.LoadConfigWithFallback(configPath, logger)
	if err != nil {
		logger.Fatalf("load config failed: %v", err)
	}

	if err := appenv.ApplyLoggingConfig(logger, appenv.LoggingConfig{
		Level:        appCfg.Logging.Level,
		Format:       appCfg.Logging.Format,
		File:         appCfg.Logging.File,
		SplitByLevel: appCfg.Logging.SplitByLevel,
	}); err != nil {
		logger.Warnf("apply logging config failed: %v", err)
	}

	if appCfg.RabbitMQ == nil {
		logger.Fatal("RabbitMQ config is required, please set rabbitmq.enabled=true")
	}
	nodeRole := appCfg.RabbitMQ.Node.NormalizedRole()
	logger.Infof("node role: %s", nodeRole)

	ctx := context.Background()

	if err := bootstrap.InitializePrompts(ctx, appCfg, logger); err != nil {
		logger.Warnf("initialize prompts failed, using fallback behavior: %v", err)
	}

	serviceManager, err := consumer.NewServiceManager(appCfg.RabbitMQ, logger)
	if err != nil {
		logger.Fatalf("create service manager failed: %v", err)
	}

	consumerDeps := bootstrap.BuildConsumerDependencies()
	crawlerDeps := bootstrap.BuildCrawlerDependencies()
	platformRegistry := consumer.NewPlatformRegistry(appCfg, logger, "", consumerDeps)
	if appCfg.RabbitMQ.Node.HandlesTaskWork() {
		if err := platformRegistry.RegisterAllProcessors(ctx, serviceManager); err != nil {
			logger.Fatalf("register platform processors failed: %v", err)
		}
	} else {
		logger.Info("skip platform processor registration for crawler-only node")
	}

	if appCfg.RabbitMQ.Node.HandlesCrawlerWork() {
		logger.Info("registering crawler processor")
		crawlerRegistry := consumer.NewCrawlerRegistry(appCfg, logger, serviceManager.GetClient(), crawlerDeps)
		if err := crawlerRegistry.RegisterCrawlerProcessor(serviceManager, platformRegistry.GetSharedAmazonProcessor()); err != nil {
			logger.Fatalf("register crawler processor failed: %v", err)
		}
	} else {
		logger.Info("skip crawler processor registration for task-only node")
	}

	ownedStores := appCfg.RabbitMQ.Node.OwnedStores
	if appCfg.RabbitMQ.Node.HandlesTaskWork() && len(ownedStores) > 0 {
		logger.Infof("owned stores: %v", ownedStores)
		serviceManager.SetStoreComponents(nil, ownedStores, nil)
	} else if appCfg.RabbitMQ.Node.HandlesTaskWork() {
		logger.Warn("rabbitmq.node.ownedStores is empty, this node will subscribe to platform-level queues")
	}

	if appCfg.RabbitMQ.Node.HandlesTaskWork() && (appCfg.Platforms.Temu.SchedulerEnabled || appCfg.Platforms.Shein.SchedulerEnabled) {
		schedulerSvc := runner.NewSchedulerServiceWithDependencies(
			logger,
			platformRegistry.GetManagementClient(),
			appCfg,
			serviceManager.GetClient(),
			bootstrap.BuildSchedulerDependencies(
				platformRegistry.GetManagementClient(),
				appCfg,
				platformRegistry.GetSharedAmazonProcessor(),
				serviceManager.GetClient(),
			),
		)
		serviceManager.SetSchedulerService(schedulerSvc)
		logger.Info("scheduler service injected")
	} else if !appCfg.RabbitMQ.Node.HandlesTaskWork() {
		logger.Info("skip scheduler injection for crawler-only node")
	}

	if err := serviceManager.Start(ctx); err != nil {
		logger.Fatalf("start service manager failed: %v", err)
	}

	logger.Info("RabbitMQ consumer started")
	logger.Infof("services started for role %s", nodeRole)
	logger.Info("monitoring endpoints: http://localhost:8081/health, http://localhost:8081/ready, http://localhost:8082/metrics, http://localhost:8082/stats")
	logger.Info("press Ctrl+C to exit")

	serviceManager.Wait()
	logger.Info("process exited")
}
