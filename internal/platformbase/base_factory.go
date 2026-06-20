// Package platformbase 提供多平台通用的基础功能
package platformbase

import (
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/infra/rabbitmq"
	domainproduct "task-processor/internal/product"
	appscheduler "task-processor/internal/scheduler"

	"github.com/sirupsen/logrus"
)

type ManagementRuntime interface {
	GetRawJsonDataAdapter() domainproduct.RawJsonDataClient
}

// BaseFactoryConfig 基础工厂配置
type BaseFactoryConfig struct {
	Platform          string
	ManagementRuntime ManagementRuntime
	FetcherBuilder    ProductFetcherBuilder
	AmazonConfig      *config.AmazonConfig
	MonitorConfig     *config.MonitorConfig
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
		logger: logger.GetGlobalLogger("BaseFactory").WithField("platform", config.Platform),
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

func (f *BaseFactory) GetManagementRuntime() ManagementRuntime {
	return f.config.ManagementRuntime
}

// GetFetcherBuilder 获取产品获取器构建器。
func (f *BaseFactory) GetFetcherBuilder() ProductFetcherBuilder {
	if f.config.FetcherBuilder == nil {
		return NewDefaultProductFetcherBuilder()
	}
	return f.config.FetcherBuilder
}

// BuildProductFetcher 构建统一产品获取器。
func (f *BaseFactory) BuildProductFetcher(rabbitmqClient *rabbitmq.Client) (appfetcher.ProductFetcher, error) {
	return f.GetFetcherBuilder().Build(
		f.GetManagementRuntime().GetRawJsonDataAdapter(),
		f.GetAmazonConfig(),
		nil,
		rabbitmqClient,
	)
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

// SupportedTaskTypes 支持的任务类型（子工厂可覆盖）
func (f *BaseFactory) SupportedTaskTypes() []appscheduler.TaskType {
	return []appscheduler.TaskType{
		appscheduler.TaskTypePricing,
		appscheduler.TaskTypeProductSync,
		appscheduler.TaskTypeInventory,
		appscheduler.TaskTypeActivity,
	}
}
