// Package config 提供配置管理功能
// 此文件包含类型别名以保持向后兼容
package config

import (
	"task-processor/internal/core/config/types"
)

// 类型别名 - 保持向后兼容
type (
	// 核心配置类型
	ProcessorConfig  = types.ProcessorConfig
	WorkerConfig     = types.WorkerConfig
	OpenAIConfig     = types.OpenAIConfig
	ManagementConfig = types.ManagementConfig

	// 平台配置类型
	PlatformsConfig     = types.PlatformsConfig
	PlatformConfig      = types.PlatformConfig
	PlatformConfigPaths = types.PlatformConfigPaths
	AutoPricingConfig   = types.AutoPricingConfig
	ScheduledTaskConfig = types.ScheduledTaskConfig
	SyncProductConfig   = types.SyncProductConfig
	MonitorConfig       = types.MonitorConfig
	Alibaba1688Config   = types.Alibaba1688Config

	// 浏览器配置类型
	BrowserConfig       = types.BrowserConfig
	BrowserRandomConfig = types.BrowserRandomConfig

	// Amazon配置类型
	AmazonConfig      = types.AmazonConfig
	AmazonConfigPaths = types.AmazonConfigPaths
	SPAPIConfig       = types.SPAPIConfig
	MarketplaceConfig = types.MarketplaceConfig

	// RabbitMQ配置类型
	RabbitMQConfig         = types.RabbitMQConfig
	RabbitMQConsumerConfig = types.RabbitMQConsumerConfig
	QueueConfig            = types.QueueConfig
	ResultReporterConfig   = types.ResultReporterConfig
	LoadMonitorConfig      = types.LoadMonitorConfig
	NodeConfig             = types.NodeConfig
	DeduplicatorConfig     = types.DeduplicatorConfig
	StoreAPIConfig         = types.StoreAPIConfig

	// 更新器配置类型
	UpdaterConfig = types.UpdaterConfig
)
