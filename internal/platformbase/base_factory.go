// Package platformbase 提供多平台通用的基础功能
package platformbase

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/model"
	"task-processor/internal/infra/clients/management"

	"github.com/sirupsen/logrus"
)

// AmazonCrawler 定义平台工厂对 Amazon 爬虫的依赖（消费者定义接口原则）。
// 平台工厂只需要抓取能力，不需要关心具体实现。
type AmazonCrawler interface {
	Process(url string, zipcode string) (*model.Product, error)
}

// BaseTaskFactory 基础任务工厂接口
type BaseTaskFactory interface {
	CreateTask(ctx context.Context, config appscheduler.TaskConfig) (appscheduler.Task, error)
	SupportedPlatform() string
	SupportedTaskTypes() []appscheduler.TaskType
}

// BaseFactoryConfig 基础工厂配置
type BaseFactoryConfig struct {
	Platform         string
	ManagementClient *management.ClientManager
	AmazonProcessor  AmazonCrawler
	AmazonConfig     *config.AmazonConfig
	MonitorConfig    *config.MonitorConfig
}

// BaseFactory 基础工厂实现
type BaseFactory struct {
	config BaseFactoryConfig
	logger *logrus.Entry
}

// NewBaseFactory 创建基础工厂
func NewBaseFactory(config BaseFactoryConfig) *BaseFactory {
	return &BaseFactory{
		config: config,
		logger: logrus.WithFields(logrus.Fields{
			"component": "BaseFactory",
			"platform":  config.Platform,
		}),
	}
}

// ValidatePlatform 验证平台
func (f *BaseFactory) ValidatePlatform(config appscheduler.TaskConfig) error {
	if config.Platform != f.config.Platform {
		return fmt.Errorf("不支持的平台: %s, 当前工厂仅支持: %s", config.Platform, f.config.Platform)
	}
	return nil
}

// ValidateTaskType 验证任务类型
func (f *BaseFactory) ValidateTaskType(taskType appscheduler.TaskType) error {
	supportedTypes := f.SupportedTaskTypes()
	for _, supported := range supportedTypes {
		if supported == taskType {
			return nil
		}
	}
	return fmt.Errorf("不支持的任务类型: %s, 支持的类型: %v", taskType, supportedTypes)
}

// GetManagementClient 获取管理客户端
func (f *BaseFactory) GetManagementClient() *management.ClientManager {
	return f.config.ManagementClient
}

// GetAmazonProcessor 获取Amazon处理器
func (f *BaseFactory) GetAmazonProcessor() AmazonCrawler {
	return f.config.AmazonProcessor
}

// GetAmazonConfig 获取Amazon配置
func (f *BaseFactory) GetAmazonConfig() *config.AmazonConfig {
	return f.config.AmazonConfig
}

// GetMonitorConfig 获取监控配置
func (f *BaseFactory) GetMonitorConfig() *config.MonitorConfig {
	return f.config.MonitorConfig
}

// GetLogger 获取日志记录器
func (f *BaseFactory) GetLogger() *logrus.Entry {
	return f.logger
}

// SupportedPlatform 支持的平台
func (f *BaseFactory) SupportedPlatform() string {
	return f.config.Platform
}

// SupportedTaskTypes 支持的任务类型（子类需要重写）
func (f *BaseFactory) SupportedTaskTypes() []appscheduler.TaskType {
	// 默认支持所有常见任务类型
	return []appscheduler.TaskType{
		appscheduler.TaskTypePricing,
		appscheduler.TaskTypeProductSync,
		appscheduler.TaskTypeInventory,
		appscheduler.TaskTypeActivity,
	}
}

// CreateTask 创建任务（子类需要重写）
func (f *BaseFactory) CreateTask(ctx context.Context, config appscheduler.TaskConfig) (appscheduler.Task, error) {
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


