package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task-processor/internal/core/errors"
	"task-processor/internal/infra/container"
	"task-processor/internal/infra/monitoring"
	"task-processor/internal/utils"

	"github.com/sirupsen/logrus"
)

// 版本信息（通过 -ldflags 在编译时注入）
var (
	appVersion = "1.0.0" // 默认版本，编译时会被覆盖
	buildTime  = "unknown"
)

func main() {
	// 记录进程启动时间
	monitoring.RecordProcessStartTime()

	// 设置日志
	logger := utils.SetupLogger()

	// 创建依赖注入容器
	container := container.NewContainer(logger)

	// 运行应用
	if err := runApplication(container); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			logger.WithFields(logrus.Fields{
				"code":    appErr.Code,
				"details": appErr.Details,
				"file":    appErr.File,
				"line":    appErr.Line,
			}).Fatalf("应用启动失败: %v", appErr)
		} else {
			logger.Fatalf("应用启动失败: %v", err)
		}
	}
}

// runApplication 运行应用
func runApplication(container *container.Container) error {
	logger := container.GetLogger()

	// 显示版本信息
	versionInfo := utils.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	}
	utils.PrintVersionInfo(logger, versionInfo)

	// 初始化容器
	if err := container.Initialize(); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "初始化容器失败")
	}

	// 加载配置
	if err := container.LoadConfig(""); err != nil {
		return err
	}

	// 初始化认证
	if err := container.InitializeAuth(); err != nil {
		return err
	}

	// 创建应用上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动所有组件
	if err := container.StartAll(ctx); err != nil {
		return err
	}

	// 等待程序退出信号
	waitForShutdown(logger, cancel)

	// 优雅关闭所有组件
	return gracefulShutdown(container)
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

// gracefulShutdown 优雅关闭
func gracefulShutdown(container *container.Container) error {
	logger := container.GetLogger()

	// 设置关闭超时
	shutdownTimeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	logger.Infof("开始优雅关闭，超时时间: %v", shutdownTimeout)

	// 停止所有组件
	done := make(chan error, 1)
	go func() {
		done <- container.StopAll(ctx)
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
		return errors.New(errors.ErrCodeTimeout, "优雅关闭超时")
	}
}
