// Package utils 提供工具方法
package utils

import (
	commonConfig "task-processor/common/config"
	internalConfig "task-processor/internal/config"
)

// ConvertToCommonConfig 将内部配置转换为common配置
func ConvertToCommonConfig(cfg *internalConfig.Config) *commonConfig.Config {
	return &commonConfig.Config{
		Processor: commonConfig.ProcessorConfig{
			MaxRetries: cfg.Processor.MaxRetries,
			Timeout:    cfg.Processor.Timeout,
		},
		Worker: commonConfig.WorkerConfig{
			Concurrency:      cfg.Worker.Concurrency,
			BufferSize:       cfg.Worker.BufferSize,
			TaskInterval:     cfg.Worker.TaskInterval,
			MaxFetchPerCycle: cfg.Worker.MaxFetchPerCycle,
			QueueThreshold:   cfg.Worker.QueueThreshold,
		},
		OpenAI: commonConfig.OpenAIConfig{
			APIKey:  cfg.OpenAI.APIKey,
			Model:   cfg.OpenAI.Model,
			BaseURL: cfg.OpenAI.BaseURL,
			Timeout: cfg.OpenAI.Timeout,
		},
		Management: commonConfig.ManagementConfig{
			BaseURL:      cfg.Management.BaseURL,
			ClientID:     cfg.Management.ClientID,
			ClientSecret: cfg.Management.ClientSecret,
			TokenURL:     cfg.Management.TokenURL,
			Scopes:       cfg.Management.Scopes,
			TenantID:     cfg.Management.TenantID,
			UserID:       cfg.Management.UserID,
			StoreIDs:     cfg.Management.StoreIDs,
		},
		AutoPricing: commonConfig.AutoPricingConfig{
			Temu: commonConfig.PlatformAutoPricingConfig{
				Enabled:   cfg.AutoPricing.Temu.Enabled,
				Interval:  cfg.AutoPricing.Temu.Interval,
				BatchSize: cfg.AutoPricing.Temu.BatchSize,
			},
			Shein: commonConfig.PlatformAutoPricingConfig{
				Enabled:   cfg.AutoPricing.Shein.Enabled,
				Interval:  cfg.AutoPricing.Shein.Interval,
				BatchSize: cfg.AutoPricing.Shein.BatchSize,
			},
		},
		Amazon: *ConvertAmazonConfig(&cfg.Amazon),
		Updater: commonConfig.UpdaterConfig{
			Enabled:            cfg.Updater.Enabled,
			UpdateURL:          cfg.Updater.UpdateURL,
			CheckInterval:      cfg.Updater.CheckInterval,
			InsecureSkipVerify: cfg.Updater.InsecureSkipVerify,
			CurrentVersion:     cfg.Updater.CurrentVersion,
		},
	}
}
