package browser

import (
	"fmt"
	"strings"
	"task-processor/internal/core/config"
	sharedbrowser "task-processor/internal/crawler/shared/browser"

	"github.com/sirupsen/logrus"
)

// ConfigManager 配置管理器
type ConfigManager struct {
	configGen *sharedbrowser.RandomConfigGenerator
	presets   map[string]*sharedbrowser.BrowserConfig
}

// NewConfigManager 创建配置管理器
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		configGen: sharedbrowser.NewRandomConfigGenerator(),
		presets:   sharedbrowser.GenerateConfigPresets(),
	}
}

// GenerateBrowserConfig 根据策略生成浏览器配置
func (cm *ConfigManager) GenerateBrowserConfig(cfg *config.AmazonConfig, strategy string, presetName string, instanceID int) *sharedbrowser.BrowserConfig {
	var browserConfig *sharedbrowser.BrowserConfig

	switch strategy {
	case "random":
		// 完全随机配置
		browserConfig = cm.configGen.GenerateRandomBrowserConfig()
		logrus.Infof("实例 %d 使用随机配置策略", instanceID)

	case "stable":
		// 基于实例ID的稳定配置
		seed := int64(instanceID * 1000) // 确保不同实例有不同的种子
		browserConfig = cm.configGen.GenerateStableBrowserConfig(seed)
		logrus.Infof("实例 %d 使用稳定配置策略 (种子: %d)", instanceID, seed)

	case "preset":
		// 使用预设配置
		if preset, exists := cm.presets[presetName]; exists {
			browserConfig = cm.copyBrowserConfig(preset)
			logrus.Infof("实例 %d 使用预设配置: %s", instanceID, presetName)
		} else {
			// 预设不存在，回退到Windows配置
			browserConfig = cm.configGen.GenerateWindowsConfig()
			logrus.Warnf("预设 %s 不存在，实例 %d 回退到Windows配置", presetName, instanceID)
		}

	case "windows":
		// Windows专用配置
		browserConfig = cm.configGen.GenerateWindowsConfig()
		logrus.Infof("实例 %d 使用Windows专用配置", instanceID)

	default:
		// 默认使用Windows配置
		browserConfig = cm.configGen.GenerateWindowsConfig()
		logrus.Infof("实例 %d 使用默认Windows配置", instanceID)
	}

	// 覆盖基础配置
	cm.applyBaseConfig(browserConfig, cfg)

	// 验证配置
	if issues := sharedbrowser.ValidateConfig(browserConfig); len(issues) > 0 {
		logrus.Warnf("配置验证发现问题: %v", issues)
	}

	return browserConfig
}

// applyBaseConfig 应用基础配置
func (cm *ConfigManager) applyBaseConfig(browserConfig *sharedbrowser.BrowserConfig, cfg *config.AmazonConfig) {
	browserConfig.Headless = cfg.Headless
	browserConfig.BrowserPath = cfg.BrowserPath
	browserConfig.ProxyServer = cfg.ProxyServer
	browserConfig.ViewportWidth = cfg.ViewportWidth
	browserConfig.ViewportHeight = cfg.ViewportHeight
}

// copyBrowserConfig 复制浏览器配置
func (cm *ConfigManager) copyBrowserConfig(src *sharedbrowser.BrowserConfig) *sharedbrowser.BrowserConfig {
	return &sharedbrowser.BrowserConfig{
		Headless:                       src.Headless,
		BrowserPath:                    src.BrowserPath,
		ProxyServer:                    src.ProxyServer,
		ViewportWidth:                  src.ViewportWidth,
		ViewportHeight:                 src.ViewportHeight,
		UserAgent:                      src.UserAgent,
		FingerprintSeed:                src.FingerprintSeed,
		FingerprintPlatform:            src.FingerprintPlatform,
		FingerprintPlatformVersion:     src.FingerprintPlatformVersion,
		FingerprintBrand:               src.FingerprintBrand,
		FingerprintBrandVersion:        src.FingerprintBrandVersion,
		FingerprintHardwareConcurrency: src.FingerprintHardwareConcurrency,
		FingerprintGPUVendor:           src.FingerprintGPUVendor,
		FingerprintGPURenderer:         src.FingerprintGPURenderer,
		Language:                       src.Language,
		AcceptLanguage:                 src.AcceptLanguage,
		Timezone:                       src.Timezone,
		DisableGPUFingerprint:          src.DisableGPUFingerprint,
	}
}

// ShouldUseRandomConfig 判断是否应该使用随机配置
func (cm *ConfigManager) ShouldUseRandomConfig(cfg *config.AmazonConfig) bool {
	// 首先检查主配置中的随机配置开关
	if cfg.RandomConfig.Enabled {
		return true
	}

	// 如果浏览器路径包含fingerprint-chromium相关关键词
	browserPath := strings.ToLower(cfg.BrowserPath)
	if strings.Contains(browserPath, "fingerprint") ||
		strings.Contains(browserPath, "ungoogled") ||
		strings.Contains(browserPath, "chromium") {
		return true
	}

	// 可以根据其他条件判断
	return false
}

// GetAvailablePresets 获取可用的预设配置列表
func (cm *ConfigManager) GetAvailablePresets() []string {
	presets := make([]string, 0, len(cm.presets))
	for name := range cm.presets {
		presets = append(presets, name)
	}
	return presets
}

// GetPresetInfo 获取预设配置信息
func (cm *ConfigManager) GetPresetInfo(presetName string) (map[string]interface{}, error) {
	preset, exists := cm.presets[presetName]
	if !exists {
		return nil, fmt.Errorf("预设 %s 不存在", presetName)
	}

	return map[string]interface{}{
		"name":                    presetName,
		"platform":                preset.FingerprintPlatform,
		"platform_version":        preset.FingerprintPlatformVersion,
		"brand":                   preset.FingerprintBrand,
		"brand_version":           preset.FingerprintBrandVersion,
		"hardware_concurrency":    preset.FingerprintHardwareConcurrency,
		"gpu_vendor":              preset.FingerprintGPUVendor,
		"gpu_renderer":            preset.FingerprintGPURenderer,
		"language":                preset.Language,
		"timezone":                preset.Timezone,
		"disable_gpu_fingerprint": preset.DisableGPUFingerprint,
		"viewport":                fmt.Sprintf("%dx%d", preset.ViewportWidth, preset.ViewportHeight),
	}, nil
}

// LogConfigStrategy 记录配置策略信息
func (cm *ConfigManager) LogConfigStrategy(strategy string, presetName string, instanceID int) {
	logrus.WithFields(logrus.Fields{
		"instance_id":       instanceID,
		"strategy":          strategy,
		"preset_name":       presetName,
		"available_presets": cm.GetAvailablePresets(),
	}).Info("配置策略详情")
}
