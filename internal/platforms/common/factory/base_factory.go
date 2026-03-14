// Package factory 提供平台任务工厂的公共实现
package factory

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/clients/management"

	"github.com/sirupsen/logrus"
)

// BaseTaskFactory 基础任务工厂接口
type BaseTaskFactory interface {
	// CreateTask 创建任务
	CreateTask(ctx context.Context, config appscheduler.TaskConfig) (appscheduler.Task, error)
	// SupportedPlatform 支持的平台
	SupportedPlatform() string
	// SupportedTaskTypes 支持的任务类型
	SupportedTaskTypes() []appscheduler.TaskType
}

// BaseFactoryConfig 基础工厂配置
type BaseFactoryConfig struct {
	// Platform 平台名称
	Platform string
	// ManagementClient 管理客户端
	ManagementClient *management.ClientManager
	// AmazonProcessor Amazon处理器
	AmazonProcessor *amazon.AmazonProcessor
	// AmazonConfig Amazon配置
	AmazonConfig *config.AmazonConfig
	// MonitorConfig 监控配置
	MonitorConfig *config.MonitorConfig
}

// BaseFactoryImpl 基础工厂实现
type BaseFactoryImpl struct {
	config BaseFactoryConfig
	logger *logrus.Entry
}

// NewBaseFactory 创建基础工厂
func NewBaseFactory(config BaseFactoryConfig) *BaseFactoryImpl {
	return &BaseFactoryImpl{
		config: config,
		logger: logrus.WithFields(logrus.Fields{
			"component": "BaseFactory",
			"platform":  config.Platform,
		}),
	}
}

// ValidatePlatform 验证平台
func (f *BaseFactoryImpl) ValidatePlatform(config appscheduler.TaskConfig) error {
	if config.Platform != f.config.Platform {
		return fmt.Errorf("不支持的平台: %s, 当前工厂仅支持: %s", config.Platform, f.config.Platform)
	}
	return nil
}

// ValidateTaskType 验证任务类型
func (f *BaseFactoryImpl) ValidateTaskType(taskType appscheduler.TaskType) error {
	supportedTypes := f.SupportedTaskTypes()
	for _, supported := range supportedTypes {
		if supported == taskType {
			return nil
		}
	}
	return fmt.Errorf("不支持的任务类型: %s, 支持的类型: %v", taskType, supportedTypes)
}

// GetManagementClient 获取管理客户端
func (f *BaseFactoryImpl) GetManagementClient() *management.ClientManager {
	return f.config.ManagementClient
}

// GetAmazonProcessor 获取Amazon处理器
func (f *BaseFactoryImpl) GetAmazonProcessor() *amazon.AmazonProcessor {
	return f.config.AmazonProcessor
}

// GetAmazonConfig 获取Amazon配置
func (f *BaseFactoryImpl) GetAmazonConfig() *config.AmazonConfig {
	return f.config.AmazonConfig
}

// GetMonitorConfig 获取监控配置
func (f *BaseFactoryImpl) GetMonitorConfig() *config.MonitorConfig {
	return f.config.MonitorConfig
}

// GetLogger 获取日志记录器
func (f *BaseFactoryImpl) GetLogger() *logrus.Entry {
	return f.logger
}

// SupportedPlatform 支持的平台
func (f *BaseFactoryImpl) SupportedPlatform() string {
	return f.config.Platform
}

// SupportedTaskTypes 支持的任务类型（子类需要重写）
func (f *BaseFactoryImpl) SupportedTaskTypes() []appscheduler.TaskType {
	// 默认支持所有常见任务类型
	return []appscheduler.TaskType{
		appscheduler.TaskTypePricing,
		appscheduler.TaskTypeProductSync,
		appscheduler.TaskTypeInventory,
		appscheduler.TaskTypeActivity,
	}
}

// CreateTask 创建任务（子类需要重写）
func (f *BaseFactoryImpl) CreateTask(ctx context.Context, config appscheduler.TaskConfig) (appscheduler.Task, error) {
	// 验证平台
	if err := f.ValidatePlatform(config); err != nil {
		return nil, err
	}

	// 验证任务类型
	if err := f.ValidateTaskType(config.TaskType); err != nil {
		return nil, err
	}

	// 子类需要实现具体的任务创建逻辑
	return nil, fmt.Errorf("CreateTask方法需要由子类实现")
}
