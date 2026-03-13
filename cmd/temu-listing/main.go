// Package main TEMU 上架服务入口
package main

import (
	"context"
	"flag"

	"task-processor/internal/app/messaging"
	appservice "task-processor/internal/app/service"
	"task-processor/internal/core/config"
	"task-processor/internal/pkg/utils"
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

	logger.Info("🚀 启动 TEMU 上架服务...")
	logger.Infof("📋 配置文件路径: %s", *configPath)

	// 加载配置
	cfg := config.LoadConfigWithFallback(*configPath, logger)

	// 验证 RabbitMQ 配置
	if cfg.RabbitMQ == nil || !cfg.RabbitMQ.Enabled {
		logger.Fatal("❌ RabbitMQ 配置未启用")
	}

	// 验证 TEMU 平台配置
	if !cfg.Platforms.Temu.Enabled {
		logger.Fatal("❌ TEMU 平台未启用")
	}

	// 创建消息服务（当前由 ServiceManager 实现）
	msgSvc, err := appservice.NewMessagingService(cfg.RabbitMQ, logger)
	if err != nil {
		logger.Fatalf("❌ 创建消息服务失败: %v", err)
	}
	serviceManager := msgSvc.(*messaging.ServiceManager)

	// 只注册 TEMU 平台处理器
	platformRegistry := messaging.NewPlatformRegistry(cfg, logger, "temu")
	ctx := context.Background()
	if err := platformRegistry.RegisterTemuProcessor(ctx, serviceManager); err != nil {
		logger.Fatalf("❌ 注册 TEMU 处理器失败: %v", err)
	}

	// 启动服务
	if err := serviceManager.Start(ctx); err != nil {
		logger.Fatalf("❌ 启动服务失败: %v", err)
	}

	logger.Info("✅ TEMU 上架服务启动完成")
	logger.Info("📊 监控地址:")
	logger.Info("   - 健康检查: http://localhost:8081/health")
	logger.Info("   - 就绪检查: http://localhost:8081/ready")
	logger.Info("   - 指标监控: http://localhost:8082/metrics")
	logger.Info("🔄 按 Ctrl+C 触发优雅关闭")

	// 阻塞等待服务管理器完成（内部通过 ShutdownCoordinator 监听信号并优雅关闭）
	serviceManager.Wait()
}
