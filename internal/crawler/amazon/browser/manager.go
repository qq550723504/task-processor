package browser

import (
	"fmt"
	"strings"
	"task-processor/internal/core/config"
	sharedbrowser "task-processor/internal/crawler/shared/browser"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// BrowserManager Amazon专用的浏览器管理器，继承shared的功能
type BrowserManager struct {
	*sharedbrowser.Manager
	amazonConfig  *config.AmazonConfig
	configManager *ConfigManager
}

// NewBrowserManager 创建Amazon浏览器管理器（保持向后兼容）
func NewBrowserManager(cfg *config.AmazonConfig) *BrowserManager {
	return NewBrowserManagerWithConfig(cfg, "windows", "windows_high_end", 0)
}

// NewBrowserManagerWithConfig 使用指定配置创建Amazon浏览器管理器
func NewBrowserManagerWithConfig(cfg *config.AmazonConfig, strategy string, presetName string, instanceID int) *BrowserManager {
	configManager := NewConfigManager()

	var browserConfig *sharedbrowser.BrowserConfig

	// 如果应该使用随机配置
	if configManager.ShouldUseRandomConfig(cfg) || cfg.RandomConfig.Enabled {
		// 优先使用主配置中的策略设置
		if cfg.RandomConfig.Enabled {
			strategy = cfg.RandomConfig.Strategy
			presetName = cfg.RandomConfig.PresetName
		}

		browserConfig = configManager.GenerateBrowserConfig(cfg, strategy, presetName, instanceID)
		configManager.LogConfigStrategy(strategy, presetName, instanceID)
	} else {
		// 使用传统配置方式
		browserConfig = &sharedbrowser.BrowserConfig{
			Headless:       cfg.Headless,
			BrowserPath:    cfg.BrowserPath,
			ProxyServer:    cfg.ProxyServer,
			ViewportWidth:  cfg.ViewportWidth,
			ViewportHeight: cfg.ViewportHeight,
			UserAgent:      "", // 使用默认用户代理
		}
		logrus.Infof("实例 %d 使用传统浏览器配置", instanceID)
	}

	return &BrowserManager{
		Manager:       sharedbrowser.NewManager(browserConfig),
		amazonConfig:  cfg,
		configManager: configManager,
	}
}

// GetConfigManager 获取配置管理器
func (bm *BrowserManager) GetConfigManager() *ConfigManager {
	return bm.configManager
}

// NavigateTo Amazon特定的导航方法，包含Cookie设置
func (bm *BrowserManager) NavigateTo(page playwright.Page, url string) error {
	// Set language preference cookies before navigation
	if err := bm.setLanguageCookies(url); err != nil {
		logrus.Infof("设置语言Cookie失败: %v", err)
	}

	// 使用父类的导航方法
	return bm.Manager.NavigateTo(page, url)
}

// setLanguageCookies 设置Amazon特定的语言偏好Cookie
func (bm *BrowserManager) setLanguageCookies(url string) error {
	// Extract domain from URL
	domain := ".amazon.com"
	region := "US" // Default region

	if strings.Contains(url, "amazon.co.jp") {
		domain = ".amazon.co.jp"
		region = "JP"
	} else if strings.Contains(url, "amazon.co.uk") {
		domain = ".amazon.co.uk"
		region = "UK"
	} else if strings.Contains(url, "amazon.de") {
		domain = ".amazon.de"
		region = "DE"
	} else if strings.Contains(url, "amazon.fr") {
		domain = ".amazon.fr"
		region = "FR"
	} else if strings.Contains(url, "amazon.it") {
		domain = ".amazon.it"
		region = "IT"
	} else if strings.Contains(url, "amazon.es") {
		domain = ".amazon.es"
		region = "ES"
	} else if strings.Contains(url, "amazon.ca") {
		domain = ".amazon.ca"
		region = "CA"
	} else if strings.Contains(url, "amazon.com.au") {
		domain = ".amazon.com.au"
		region = "AU"
	} else if strings.Contains(url, "amazon.com.mx") {
		domain = ".amazon.com.mx"
		region = "MX"
	} else if strings.Contains(url, "amazon.sa") {
		domain = ".amazon.sa"
		region = "SA"
	} else if strings.Contains(url, "amazon.ae") {
		domain = ".amazon.ae"
		region = "AE"
	}

	// Get currency based on region
	currency := sharedbrowser.GetCurrencyByRegion(region)

	// Set language preference cookie - always English, but currency varies by region
	cookies := []playwright.OptionalCookie{
		{
			Name:   "lc-main",
			Value:  "en_US", // Always English language
			Domain: &domain,
			Path:   playwright.String("/"),
		},
		{
			Name:   "i18n-prefs",
			Value:  currency, // Currency based on region
			Domain: &domain,
			Path:   playwright.String("/"),
		},
	}

	context := bm.GetContext()
	if context == nil {
		return fmt.Errorf("浏览器上下文未初始化")
	}

	if err := context.AddCookies(cookies); err != nil {
		return fmt.Errorf("添加语言Cookie失败: %w", err)
	}

	return nil
}
