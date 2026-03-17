// Package main Amazon 爬虫 API 服务入口（不依赖 RabbitMQ）
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task-processor/internal/core/config"
	crawleramazon "task-processor/internal/crawler/amazon"
	"task-processor/internal/pkg/appenv"

	"github.com/sirupsen/logrus"
)

var (
	configPath = flag.String("config", "config/config-prod.yaml", "配置文件路径")
	logLevel   = flag.String("log-level", "info", "日志级别")
	port       = flag.Int("port", 8080, "API 服务端口")
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	// 设置日志
	logger := appenv.SetupLoggerWithLevel(*logLevel)

	// 打印版本信息
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	logger.Info("🚀 启动 Amazon 爬虫 API 服务...")
	logger.Infof("📋 配置文件路径: %s", *configPath)
	logger.Infof("🌐 API 端口: %d", *port)

	// 加载配置
	cfg := config.LoadConfigWithFallback(*configPath, logger)

	// 创建 API 服务
	apiService := crawleramazon.NewAPIService(cfg, logger, *port)

	// 启动服务
	ctx := context.Background()
	if err := apiService.Start(ctx); err != nil {
		logger.Fatalf("❌ 启动服务失败: %v", err)
	}

	logger.Info("✅ Amazon 爬虫 API 服务启动完成")
	logger.Infof("📊 API 地址: http://localhost:%d", *port)
	logger.Info("📊 API 端点:")
	logger.Info("   - POST   /api/v1/crawl          - 提交爬虫任务")
	logger.Info("   - GET    /api/v1/tasks/{id}     - 查询任务状态")
	logger.Info("   - GET    /api/v1/tasks          - 查询所有任务")
	logger.Info("   - DELETE /api/v1/tasks/{id}     - 删除任务")
	logger.Info("   - GET    /api/v1/stats          - 查询统计信息")
	logger.Info("📊 健康检查:")
	logger.Info("   - GET    /health                - 健康检查")
	logger.Info("   - GET    /ready                 - 就绪检查")
	logger.Info("🔄 按 Ctrl+C 优雅关闭服务")

	// 等待退出信号
	waitForShutdown(apiService, logger)
}

func waitForShutdown(apiService *crawleramazon.APIService, logger *logrus.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	logger.Infof("收到信号: %v，开始优雅关闭...", sig)

	// 创建超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 关闭服务
	if err := apiService.Stop(ctx); err != nil {
		logger.Errorf("关闭服务失败: %v", err)
	}

	logger.Info("✅ 服务已优雅关闭")
}
