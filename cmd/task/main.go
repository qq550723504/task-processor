// Package main 提供应用程序入口点
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task-processor/internal/app/bootstrap"
	"task-processor/internal/infra/monitoring"
	"task-processor/internal/pkg/apputil"

	"github.com/sirupsen/logrus"
)

// 版本信息（通过 -ldflags 在编译时注入）
var (
	appVersion = "1.0.0" // 默认版本，编译时会被覆盖
	buildTime  = "unknown"
)

// mainApp 应用主函数
func main() {
	// 记录进程启动时间
	monitoring.RecordProcessStartTime()

	// 设置日志
	logger := apputil.SetupLogger()

	// 创建应用启动器
	app := bootstrap.NewApplicationBootstrap(logger)

	// 运行应用
	if err := runApplication(app, logger); err != nil {
		logger.Fatalf("应用启动失败: %v", err)
	}
}

// runApplication 运行应用
func runApplication(app *bootstrap.ApplicationBootstrap, logger *logrus.Logger) error {
	// 显示版本信息
	versionInfo := apputil.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	}
	apputil.PrintVersionInfo(logger, versionInfo)

	// 初始化应用（使用默认配置文件路径）
	configPath := "config/config-dev.yaml"
	if err := app.Initialize(configPath, appVersion); err != nil {
		return err
	}

	// 创建应用上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动应用
	if err := app.Start(ctx, appVersion); err != nil {
		return err
	}

	// 等待程序退出信号
	waitForShutdown(logger, cancel)

	// 优雅关闭应用
	return gracefulShutdown(app, logger)
}

// waitForShutdown 等待程序退出信号
func waitForShutdown(logger *logrus.Logger, cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 等待信号
	sig := <-sigChan
	logger.Infof("收到信号: %v，开始优雅关闭...", sig)

	// 取消context，通知所有子组件停止
	cancel()
}

// gracefulShutdown 优雅关闭应用
func gracefulShutdown(app *bootstrap.ApplicationBootstrap, logger *logrus.Logger) error {
	// 设置关闭超时
	shutdownTimeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	logger.Infof("开始优雅关闭，超时时间: %v", shutdownTimeout)

	// 停止应用
	done := make(chan error, 1)
	go func() {
		done <- app.Stop(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			logger.Errorf("关闭过程中发生错误: %v", err)
			return err
		}
		logger.Info("✅ 程序已优雅关闭")
		return nil

	case <-ctx.Done():
		logger.Warn("⚠️ 关闭超时，强制退出")
		return ctx.Err()
	}
}
