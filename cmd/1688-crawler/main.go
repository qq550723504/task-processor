// Package main 1688 爬虫服务入口
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"task-processor/internal/app/messaging"
	"task-processor/internal/core/config"
	"task-processor/internal/pkg/utils"

	"github.com/sirupsen/logrus"
)

var (
	configPath = flag.String("config", "config/config-prod.yaml", "配置文件路径")
	logLevel   = flag.String("log-level", "info", "日志级别")
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	// 设置日志
	logger := utils.SetupLoggerWithLevel(*logLevel)

	// 打印版本信息
	utils.PrintVersionInfo(logger, utils.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	logger.Info("🚀 启动 1688 爬虫服务...")
	logger.Infof("📋 配置文件路径: %s", *configPath)

	// 加载配置
	cfg := config.LoadConfigWithFallback(*configPath, logger)

	// 验证 RabbitMQ 配置
	if cfg.RabbitMQ == nil || !cfg.RabbitMQ.Enabled {
		logger.Fatal("❌ RabbitMQ 配置未启用")
	}

	// 创建服务管理器
	serviceManager, err := messaging.NewServiceManager(cfg.RabbitMQ, logger)
	if err != nil {
		logger.Fatalf("❌ 创建服务管理器失败: %v", err)
	}

	// 只注册 1688 爬虫处理器
	crawlerRegistry := messaging.NewCrawlerRegistry(cfg, logger, serviceManager.GetClient())
	if err := crawlerRegistry.Register1688Crawler(serviceManager); err != nil {
		logger.Fatalf("❌ 注册 1688 爬虫失败: %v", err)
	}

	// 启动服务
	ctx := context.Background()
	if err := serviceManager.Start(ctx); err != nil {
		logger.Fatalf("❌ 启动服务失败: %v", err)
	}

	logger.Info("✅ 1688 爬虫服务启动完成")
	logger.Info("📊 监控地址:")
	logger.Info("   - 健康检查: http://localhost:8081/health")
	logger.Info("   - 就绪检查: http://localhost:8081/ready")
	logger.Info("   - 指标监控: http://localhost:8082/metrics")
	logger.Info("🔄 按 Ctrl+C 优雅关闭服务")

	// 等待退出信号
	waitForShutdown(serviceManager, logger)
}

func waitForShutdown(sm *messaging.ServiceManager, logger *logrus.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	logger.Infof("收到信号: %v，开始优雅关闭...", sig)

	ctx := context.Background()
	sm.Stop(ctx)
	logger.Info("✅ 服务已优雅关闭")
}
