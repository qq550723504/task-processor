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

	logger.Info("正在启动 productenrich API 服务")
	logger.Infof("配置文件路径：%s", *configPath)
	logger.Infof("API 端口：%d", *port)

	if err := run(logger); err != nil {
		logger.Fatalf("服务启动失败：%v", err)
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
				logger.Warnf("关闭资源失败：%v", closeErr)
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, pool := range bootstrap.pools {
		pool.Start(ctx)
	}
	logger.Infof("工作池已启动：%d", len(bootstrap.pools))

	go func() {
		logger.Infof("API 服务正在监听端口 %d", *port)
		logger.Info("可用端点：")
		logger.Info("  - POST /api/v1/products/generate（生成产品）")
		logger.Info("  - GET  /api/v1/products/tasks/:task_id（查询任务）")
		logger.Info("  - POST /api/v1/images/process（处理图片）")
		logger.Info("  - GET  /api/v1/images/tasks/:task_id（查询图片任务）")
		logger.Info("  - POST /api/v1/images/tasks/:task_id/review（审核任务）")
		logger.Info("  - GET  /health（健康检查）")
		if listenErr := bootstrap.server.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			logger.Fatalf("HTTP 服务异常退出：%v", listenErr)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan
	logger.Infof("收到信号 %v，正在关闭", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := bootstrap.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("关闭 HTTP 服务器：%w", err)
	}

	cancel()
	for _, pool := range bootstrap.pools {
		pool.Stop(shutdownCtx)
	}
	logger.Info("服务已正常关闭")
	return nil
}
