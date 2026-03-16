// Package system 提供系统级初始化功能
package system

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/goroutine"

	"github.com/sirupsen/logrus"
)

// SystemInitializer 系统初始化器
type SystemInitializer struct {
	logManager       *logger.LogManager
	goroutineManager *goroutine.Manager
	logger           *logrus.Entry
	ctx              context.Context
	cancel           context.CancelFunc
}

// SystemConfig 系统配置
type SystemConfig struct {
	LogConfig *logger.LogConfig `yaml:"log" json:"log"`
	AppName   string            `yaml:"app_name" json:"app_name"`
	Version   string            `yaml:"version" json:"version"`
}

// NewSystemInitializer 创建系统初始化器
func NewSystemInitializer(config *SystemConfig) *SystemInitializer {
	if config == nil {
		config = &SystemConfig{
			LogConfig: logger.DefaultLogConfig(),
			AppName:   "task-processor",
			Version:   "1.0.0",
		}
	}

	// 初始化日志管理器
	logManager := logger.NewLogManager(config.LogConfig)
	componentLogger := logManager.GetLoggerWithFields(logrus.Fields{
		"component": "system_initializer",
		"app_name":  config.AppName,
		"version":   config.Version,
	})

	// 创建系统上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 创建goroutine管理器
	goroutineManager := goroutine.NewManager(ctx, componentLogger)

	return &SystemInitializer{
		logManager:       logManager,
		goroutineManager: goroutineManager,
		logger:           componentLogger,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Initialize 初始化系统
func (si *SystemInitializer) Initialize() error {
	si.logger.Info("开始初始化系统")

	// 设置全局日志管理器
	logger.InitGlobalLogger(nil)

	// 注册信号处理
	si.setupSignalHandling()

	si.logger.Info("系统初始化完成")
	return nil
}

// GetContext 获取系统上下文
func (si *SystemInitializer) GetContext() context.Context {
	return si.ctx
}

// GetGoroutineManager 获取goroutine管理器
func (si *SystemInitializer) GetGoroutineManager() *goroutine.Manager {
	return si.goroutineManager
}

// GetLogger 获取日志记录器
func (si *SystemInitializer) GetLogger(component string) *logrus.Entry {
	return si.logManager.GetLogger(component)
}

// Shutdown 优雅关闭系统
func (si *SystemInitializer) Shutdown() error {
	si.logger.Info("开始关闭系统")

	// 取消上下文，通知所有goroutine停止
	si.cancel()

	// 等待所有goroutine完成
	shutdownTimeout := 30 * time.Second
	si.logger.WithField("timeout", shutdownTimeout).Info("等待所有goroutine完成")

	if err := si.goroutineManager.WaitWithTimeout(shutdownTimeout); err != nil {
		si.logger.WithError(err).Warn("等待goroutine完成超时")
	}

	// 关闭日志管理器
	if err := si.logManager.Close(); err != nil {
		si.logger.WithError(err).Error("关闭日志管理器失败")
		return err
	}

	si.logger.Info("系统关闭完成")
	return nil
}

// setupSignalHandling 设置信号处理
func (si *SystemInitializer) setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	si.goroutineManager.Start("signal_handler", func(ctx context.Context) error {
		select {
		case sig := <-sigChan:
			si.logger.WithField("signal", sig).Info("收到关闭信号")
			si.cancel()
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}

// GetSystemStatus 获取系统状态
func (si *SystemInitializer) GetSystemStatus() map[string]any {
	goroutineStatus := si.goroutineManager.GetStatus()

	return map[string]any{
		"log_level":          si.logManager.GetLevel(),
		"running_goroutines": si.goroutineManager.GetRunningCount(),
		"goroutine_details":  goroutineStatus,
		"context_cancelled":  si.ctx.Err() != nil,
	}
}

// 全局系统初始化器实例
var globalSystemInitializer *SystemInitializer

// InitializeGlobalSystem 初始化全局系统
func InitializeGlobalSystem(config *SystemConfig) error {
	globalSystemInitializer = NewSystemInitializer(config)
	return globalSystemInitializer.Initialize()
}

// GetGlobalSystemInitializer 获取全局系统初始化器
func GetGlobalSystemInitializer() *SystemInitializer {
	return globalSystemInitializer
}

// ShutdownGlobalSystem 关闭全局系统
func ShutdownGlobalSystem() error {
	if globalSystemInitializer != nil {
		return globalSystemInitializer.Shutdown()
	}
	return nil
}

// LoadSystemConfigFromFile 从文件加载系统配置
func LoadSystemConfigFromFile(configPath string) (*SystemConfig, error) {
	// 这里可以根据实际需要实现配置文件加载逻辑
	// 目前返回默认配置
	return &SystemConfig{
		LogConfig: &logger.LogConfig{
			Level:      "info",
			Format:     "json",
			OutputFile: "logs/system.log",
			Console:    true,
		},
		AppName: "task-processor",
		Version: "1.0.0",
	}, nil
}

// CreateProcessorInitializer 为处理器创建初始化器
func CreateProcessorInitializer(processorName string) (*SystemInitializer, error) {
	config := &SystemConfig{
		LogConfig: &logger.LogConfig{
			Level:      "info",
			Format:     "json",
			OutputFile: fmt.Sprintf("logs/%s.log", processorName),
			Console:    true,
		},
		AppName: fmt.Sprintf("task-processor-%s", processorName),
		Version: "1.0.0",
	}

	initializer := NewSystemInitializer(config)
	if err := initializer.Initialize(); err != nil {
		return nil, fmt.Errorf("初始化%s处理器失败: %w", processorName, err)
	}

	return initializer, nil
}
