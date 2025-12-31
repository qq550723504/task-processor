package browser

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// BrowserConfig 浏览器配置
type BrowserConfig struct {
	Headless       bool   `json:"headless"`
	BrowserPath    string `json:"browserPath,omitempty"`
	ProxyServer    string `json:"proxyServer,omitempty"`
	ViewportWidth  int    `json:"viewportWidth"`
	ViewportHeight int    `json:"viewportHeight"`
	UserAgent      string `json:"userAgent,omitempty"`
}

// GetBrowserLaunchArgs 获取浏览器启动参数（精简版，与Python版本保持一致）
func GetBrowserLaunchArgs() []string {
	return []string{
		"--disable-infobars",     // 禁用信息栏
		"--no-sandbox",           // 禁用沙箱
		"--disable-web-security", // 禁用Web安全
		"--start-maximized",      // 最大化启动
		"--lang=en-US",           // 设置语言为英文
		"--disable-gpu",          // 禁用GPU（与Python版本一致）
	}
}

// GetIgnoreDefaultArgs 获取需要排除的默认参数（关键！）
func GetIgnoreDefaultArgs() []string {
	return []string{
		"--enable-automation",                           // 排除自动化标志
		"--disable-blink-features=AutomationControlled", // 排除自动化控制特征
	}
}

// AddFingerprintArgs 添加指纹参数到启动参数
func AddFingerprintArgs(args []string, fingerprint *FingerprintConfig) []string {
	if fingerprint != nil && fingerprint.Enable {
		// 序列化整个指纹配置（不只是GPU字段）
		fingerprintJSON, err := json.Marshal(fingerprint)
		if err != nil {
			logrus.Infof("序列化指纹配置失败: %v", err)
		} else {
			kfingerprintArg := fmt.Sprintf("--kfingerprint=%s", string(fingerprintJSON))
			args = append(args, kfingerprintArg)

		}
	}
	return args
}

// CreateLaunchOptions 创建浏览器启动选项
func CreateLaunchOptions(cfg *BrowserConfig, fingerprint *FingerprintConfig) playwright.BrowserTypeLaunchOptions {
	// 获取基础启动参数
	args := GetBrowserLaunchArgs()

	// 添加指纹参数
	args = AddFingerprintArgs(args, fingerprint)

	// 获取需要排除的默认参数（关键！与Python版本的ignore_default_args一致）
	ignoreDefaultArgs := GetIgnoreDefaultArgs()

	launchOptions := playwright.BrowserTypeLaunchOptions{
		Headless:          playwright.Bool(cfg.Headless),
		Args:              args,
		IgnoreDefaultArgs: ignoreDefaultArgs, // 排除自动化相关的默认参数
	}

	// 检查浏览器路径是否存在，如果不存在则使用系统默认的Chrome
	if cfg.BrowserPath != "" {
		// 检查文件是否存在
		if _, err := os.Stat(cfg.BrowserPath); err == nil {
			launchOptions.ExecutablePath = &cfg.BrowserPath
			logrus.Infof("使用指定的浏览器路径: %s", cfg.BrowserPath)
		} else {
			logrus.Infof("警告: 指定的浏览器路径不存在: %s，错误: %v", cfg.BrowserPath, err)
			logrus.Infof("将使用系统默认Chrome或Playwright自带的浏览器")
			// 不设置ExecutablePath，让Playwright使用默认的Chrome
		}
	} else {
		logrus.Infof("未指定浏览器路径，使用Playwright默认浏览器")
	}

	// 配置代理（如果设置了）
	if cfg.ProxyServer != "" {
		launchOptions.Proxy = &playwright.Proxy{
			Server: cfg.ProxyServer,
		}
		logrus.Infof("使用代理服务器: %s", cfg.ProxyServer)
	}

	return launchOptions
}

// CreateContextOptions 创建浏览器上下文选项（与Python版本保持一致）
func CreateContextOptions(cfg *BrowserConfig, userAgent string) playwright.BrowserNewContextOptions {
	contextOptions := playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  cfg.ViewportWidth,
			Height: cfg.ViewportHeight,
		},
		UserAgent: playwright.String(userAgent),
		Locale:    playwright.String("en-US"),
		// 设置时区以匹配IP地理位置
		TimezoneId: GetTimezoneForRegion(cfg.ProxyServer),
		// 忽略HTTPS错误（与Python版本的ignore_https_errors=True一致）
		IgnoreHttpsErrors: playwright.Bool(true),
	}

	return contextOptions
}
