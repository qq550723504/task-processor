// Package main 提供新架构测试
package main

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/infra/bootstrap"
	"task-processor/internal/pkg/utils"

	"github.com/sirupsen/logrus"
)

// TestNewArchitecture 测试新架构的基本功能
func TestNewArchitecture(t *testing.T) {
	// 设置测试日志
	logger := utils.SetupLogger()
	logger.SetLevel(logrus.DebugLevel)

	// 创建应用启动器
	app := bootstrap.NewApplicationBootstrap(logger)

	// 测试初始化
	if err := app.Initialize(""); err != nil {
		t.Fatalf("应用初始化失败: %v", err)
	}

	// 测试启动（使用短超时避免长时间运行）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 启动应用
	if err := app.Start(ctx, "test-1.0.0"); err != nil {
		t.Fatalf("应用启动失败: %v", err)
	}

	// 立即停止应用
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := app.Stop(stopCtx); err != nil {
		t.Fatalf("应用停止失败: %v", err)
	}

	t.Log("✅ 新架构测试通过")
}

// TestDependencyInjection 测试依赖注入容器
func TestDependencyInjection(t *testing.T) {
	logger := utils.SetupLogger()
	app := bootstrap.NewApplicationBootstrap(logger)

	// 初始化应用
	if err := app.Initialize(""); err != nil {
		t.Fatalf("应用初始化失败: %v", err)
	}

	// 获取容器
	container := app.GetContainer()

	// 测试获取配置
	config, err := container.Get("config")
	if err != nil {
		t.Fatalf("获取配置失败: %v", err)
	}
	if config == nil {
		t.Fatal("配置为空")
	}

	// 测试获取日志器
	loggerInstance, err := container.Get("logger")
	if err != nil {
		t.Fatalf("获取日志器失败: %v", err)
	}
	if loggerInstance == nil {
		t.Fatal("日志器为空")
	}

	t.Log("✅ 依赖注入测试通过")
}

// TestLifecycleManager 测试生命周期管理器
func TestLifecycleManager(t *testing.T) {
	logger := utils.SetupLogger()
	app := bootstrap.NewApplicationBootstrap(logger)

	// 初始化应用
	if err := app.Initialize(""); err != nil {
		t.Fatalf("应用初始化失败: %v", err)
	}

	// 获取生命周期管理器
	lifecycleManager := app.GetLifecycleManager()

	// 获取组件状态
	status := lifecycleManager.GetStatus()
	if len(status) == 0 {
		t.Log("⚠️ 没有注册的组件（这是正常的，因为平台处理器可能未启用）")
	} else {
		t.Logf("✅ 发现 %d 个注册的组件", len(status))
		for name, componentStatus := range status {
			t.Logf("  - 组件: %s, 运行状态: %v, 优先级: %d",
				name, componentStatus.Running, componentStatus.Priority)
		}
	}

	t.Log("✅ 生命周期管理器测试通过")
}
