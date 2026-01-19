// Package main 提供应用程序入口点（新架构版本）
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task-processor/internal/infra/bootstrap"
	"task-processor/internal/infra/monitoring"
	"task-processor/internal/pkg/utils"

	"github.com/sirupsen/logrus"
)

// 版本信息（通过 -ldflags 在编译时注入）
var (
	appVersionNew = "1.0.0" // 默认版本，编译时会被覆盖
	buildTimeNew  = "unknown"
)

// mainNew 新架构的主函数
func mainNew() {
	// 记录进程启动时间
	monitoring.RecordProcessStartTime()

	// 设置日志
	logger := utils.SetupLogger()

	// 创建应用启动器
	app := bootstrap.NewApplicationBootstrap(logger)

	// 运行应用
	if err := runNewApplication(app, logger); err != nil {
		logger.Fatalf("应用启动失败: %v", err)
	}
}

// runNewApplication 运行新架构应用
func runNewApplication(app *bootstrap.ApplicationBootstrap, logger *logrus.Logger) error {
	// 显示版本信息
	versionInfo := utils.VersionInfo{
		Version:   appVersionNew,
		BuildTime: buildTimeNew,
	}
	utils.PrintVersionInfo(logger, versionInfo)

	// 初始化应用
	if err := app.Initialize(""); err != nil {
		return err
	}

	// 创建应用上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动应用
	if err := app.Start(ctx, appVersionNew); err != nil {
		return err
	}

	// 等待程序退出信号
	waitForShutdownNew(logger, cancel)

	// 优雅关闭应用
	return gracefulShutdownNew(app, logger)
}

// waitForShutdownNew 等待程序退出信号
func waitForShutdownNew(logger *logrus.Logger, cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 等待信号
	sig := <-sigChan
	logger.Infof("收到信号: %v，开始优雅关闭...", sig)

	// 取消context，通知所有子组件停止
	cancel()
}

// gracefulShutdownNew 优雅关闭新架构应用
func gracefulShutdownNew(app *bootstrap.ApplicationBootstrap, logger *logrus.Logger) error {
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

// testNewArchitecture 测试新架构运行
func testNewArchitecture() {
	fmt.Println("🚀 开始测试新架构...")

	// 设置日志
	logger := utils.SetupLogger()
	logger.SetLevel(logrus.InfoLevel)

	// 创建应用启动器
	app := bootstrap.NewApplicationBootstrap(logger)

	// 初始化应用
	fmt.Println("📋 初始化应用...")
	if err := app.Initialize("config/config-dev.yaml"); err != nil {
		fmt.Printf("❌ 应用初始化失败: %v\n", err)
		return
	}
	fmt.Println("✅ 应用初始化成功")

	// 测试依赖注入容器
	fmt.Println("🔧 测试依赖注入容器...")
	container := app.GetContainer()

	// 测试获取配置
	config, err := container.Get("config")
	if err != nil {
		fmt.Printf("❌ 获取配置失败: %v\n", err)
		return
	}
	fmt.Printf("✅ 配置获取成功: %T\n", config)

	// 测试获取服务
	services := []string{"logger", "configService", "authService", "updaterService"}
	for _, serviceName := range services {
		service, err := container.Get(serviceName)
		if err != nil {
			fmt.Printf("❌ 获取服务 %s 失败: %v\n", serviceName, err)
			continue
		}
		fmt.Printf("✅ 服务 %s 获取成功: %T\n", serviceName, service)
	}

	// 测试生命周期管理器
	fmt.Println("🔄 测试生命周期管理器...")
	lifecycleManager := app.GetLifecycleManager()
	status := lifecycleManager.GetStatus()
	fmt.Printf("✅ 发现 %d 个注册组件:\n", len(status))
	for name, componentStatus := range status {
		fmt.Printf("  - %s: 优先级=%d, 依赖=%v\n",
			name, componentStatus.Priority, componentStatus.Dependencies)
	}

	// 短时间启动测试
	fmt.Println("🎯 测试应用启动...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 启动应用
	if err := app.Start(ctx, "test-1.0.0"); err != nil {
		fmt.Printf("❌ 应用启动失败: %v\n", err)
		return
	}
	fmt.Println("✅ 应用启动成功")

	// 等待一段时间让组件运行
	fmt.Println("⏳ 让应用运行3秒...")
	time.Sleep(3 * time.Second)

	// 检查组件状态
	fmt.Println("📊 检查组件运行状态...")
	status = lifecycleManager.GetStatus()
	for name, componentStatus := range status {
		runningStatus := "❌ 未运行"
		if componentStatus.Running {
			runningStatus = "✅ 运行中"
		}
		fmt.Printf("  - %s: %s\n", name, runningStatus)
	}

	// 停止应用
	fmt.Println("🛑 停止应用...")
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()

	if err := app.Stop(stopCtx); err != nil {
		fmt.Printf("❌ 应用停止失败: %v\n", err)
		return
	}
	fmt.Println("✅ 应用停止成功")

	fmt.Println("🎉 新架构测试完成！")
}
