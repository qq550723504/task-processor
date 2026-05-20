package browser

import (
	"fmt"
	"os"
	"task-processor/internal/core/logger"
	"time"

	"github.com/playwright-community/playwright-go"
)

// BrowserConfig 浏览器配置
type BrowserConfig struct {
	Headless                       bool   `json:"headless"`
	BrowserPath                    string `json:"browserPath,omitempty"`
	ChromeVersion                  string `json:"chromeVersion,omitempty"`     // fingerprint-chromium 版本（如 "144"）
	ChromeDownloadDir              string `json:"chromeDownloadDir,omitempty"` // Chrome 下载目录
	ProxyServer                    string `json:"proxyServer,omitempty"`
	ViewportWidth                  int    `json:"viewportWidth"`
	ViewportHeight                 int    `json:"viewportHeight"`
	UserAgent                      string `json:"userAgent,omitempty"`
	FingerprintSeed                int32  `json:"fingerprintSeed"`                // 指纹种子
	FingerprintPlatform            string `json:"fingerprintPlatform"`            // 操作系统类型
	FingerprintPlatformVersion     string `json:"fingerprintPlatformVersion"`     // 操作系统版本
	FingerprintBrand               string `json:"fingerprintBrand"`               // 浏览器品牌
	FingerprintBrandVersion        string `json:"fingerprintBrandVersion"`        // 浏览器版本
	FingerprintHardwareConcurrency int    `json:"fingerprintHardwareConcurrency"` // CPU核心数
	FingerprintGPUVendor           string `json:"fingerprintGPUVendor"`           // GPU厂商
	FingerprintGPURenderer         string `json:"fingerprintGPURenderer"`         // GPU渲染器
	Language                       string `json:"language"`                       // 浏览器语言
	AcceptLanguage                 string `json:"acceptLanguage"`                 // 接受的语言
	Timezone                       string `json:"timezone"`                       // 时区
	DisableGPUFingerprint          bool   `json:"disableGPUFingerprint"`          // 禁用GPU指纹
	StealthProvider                string `json:"stealthProvider,omitempty"`      // stealth provider, e.g. default/cloakbrowser
}

const (
	StealthProviderDefault      = "default"
	StealthProviderCloakBrowser = "cloakbrowser"
)

// GetBrowserLaunchArgs 获取浏览器启动参数（针对fingerprint-chromium优化）
func GetBrowserLaunchArgs(cfg *BrowserConfig) []string {
	if cfg != nil && cfg.StealthProvider == StealthProviderCloakBrowser {
		seed := cfg.FingerprintSeed
		if seed == 0 {
			seed = int32(time.Now().UnixNano() % 90000)
			if seed < 10000 {
				seed += 10000
			}
		}
		return []string{
			"--no-sandbox",
			fmt.Sprintf("--fingerprint=%d", seed),
			"--fingerprint-platform=windows",
		}
	}
	return []string{
		"--start-maximized", // 最大化启动
		"--disable-gpu",     // 禁用GPU加速（避免检测）
	}
}

// GetIgnoreDefaultArgs 获取需要排除的默认参数（关键！）
func GetIgnoreDefaultArgs(cfg *BrowserConfig) []string {
	if cfg != nil && cfg.StealthProvider == StealthProviderCloakBrowser {
		return []string{
			"--enable-automation",
			"--enable-unsafe-swiftshader",
		}
	}
	return []string{
		"--enable-automation",                           // 排除自动化标志
		"--disable-blink-features=AutomationControlled", // 排除自动化控制特征
	}
}

// AddFingerprintArgs 添加fingerprint-chromium指纹参数
func AddFingerprintArgs(args []string, cfg *BrowserConfig, fingerprint *FingerprintConfig) []string {
	if fingerprint == nil || !fingerprint.Enable {
		return args
	}

	// 指纹种子
	seed := cfg.FingerprintSeed
	if seed == 0 {
		seed = int32(time.Now().Unix())
	}
	args = append(args, fmt.Sprintf("--fingerprint=%d", seed))

	// 平台配置
	platform := cfg.FingerprintPlatform
	if platform == "" {
		platform = "windows"
	}
	args = append(args, fmt.Sprintf("--fingerprint-platform=%s", platform))

	// 平台版本
	if cfg.FingerprintPlatformVersion != "" {
		args = append(args, fmt.Sprintf("--fingerprint-platform-version=%s", cfg.FingerprintPlatformVersion))
	}

	// 浏览器品牌
	brand := cfg.FingerprintBrand
	if brand == "" {
		brand = "Chrome"
	}
	args = append(args, fmt.Sprintf("--fingerprint-brand=%s", brand))

	// 浏览器版本
	if cfg.FingerprintBrandVersion != "" {
		args = append(args, fmt.Sprintf("--fingerprint-brand-version=%s", cfg.FingerprintBrandVersion))
	}

	// CPU核心数
	if cfg.FingerprintHardwareConcurrency > 0 {
		args = append(args, fmt.Sprintf("--fingerprint-hardware-concurrency=%d", cfg.FingerprintHardwareConcurrency))
	}

	// GPU配置
	if cfg.FingerprintGPUVendor != "" {
		args = append(args, fmt.Sprintf("--fingerprint-gpu-vendor=%s", cfg.FingerprintGPUVendor))
	} else if gpu, ok := fingerprint.GPU["description"]; ok && gpu != "" {
		args = append(args, fmt.Sprintf("--fingerprint-gpu-vendor=%s", gpu))
	}

	if cfg.FingerprintGPURenderer != "" {
		args = append(args, fmt.Sprintf("--fingerprint-gpu-renderer=%s", cfg.FingerprintGPURenderer))
	}

	// 禁用GPU指纹
	if cfg.DisableGPUFingerprint {
		args = append(args, "--disable-gpu-fingerprint")
	}

	// 语言配置
	language := cfg.Language
	if language == "" {
		language = "en-US"
	}
	args = append(args, fmt.Sprintf("--lang=%s", language))

	acceptLang := cfg.AcceptLanguage
	if acceptLang == "" {
		acceptLang = "en-US,en;q=0.9"
	}
	args = append(args, fmt.Sprintf("--accept-lang=%s", acceptLang))

	// 时区配置
	if cfg.Timezone != "" {
		args = append(args, fmt.Sprintf("--timezone=%s", cfg.Timezone))
	}

	// WebRTC保护
	args = append(args, "--disable-non-proxied-udp")

	return args
}

// CreateLaunchOptions 创建浏览器启动选项
func CreateLaunchOptions(cfg *BrowserConfig, fingerprint *FingerprintConfig) playwright.BrowserTypeLaunchOptions {
	// 构建启动参数
	args := GetBrowserLaunchArgs(cfg)
	if cfg == nil || cfg.StealthProvider != StealthProviderCloakBrowser {
		args = AddFingerprintArgs(args, cfg, fingerprint)
	}

	launchOptions := playwright.BrowserTypeLaunchOptions{
		Headless:          playwright.Bool(cfg.Headless),
		Args:              args,
		IgnoreDefaultArgs: GetIgnoreDefaultArgs(cfg), // 排除自动化检测参数
	}

	// 设置浏览器路径
	if cfg.BrowserPath != "" {
		if _, err := os.Stat(cfg.BrowserPath); err == nil {
			launchOptions.ExecutablePath = &cfg.BrowserPath
			logger.GetGlobalLogger("crawler/shared").Infof("使用指定浏览器: %s", cfg.BrowserPath)
		} else {
			logger.GetGlobalLogger("crawler/shared").Warnf("浏览器路径不存在: %s，使用默认浏览器", cfg.BrowserPath)
		}
	}

	// 设置代理
	if cfg.ProxyServer != "" {
		if proxy := parseProxyServer(cfg.ProxyServer); proxy != nil {
			launchOptions.Proxy = proxy
			logger.GetGlobalLogger("crawler/shared").Infof("使用代理: %s", proxy.Server)
		}
	}

	return launchOptions
}

// CreateContextOptions 创建浏览器上下文选项
func CreateContextOptions(cfg *BrowserConfig, userAgent string) playwright.BrowserNewContextOptions {
	if cfg != nil && cfg.StealthProvider == StealthProviderCloakBrowser {
		options := playwright.BrowserNewContextOptions{
			Viewport: &playwright.Size{
				Width:  cfg.ViewportWidth,
				Height: cfg.ViewportHeight,
			},
			IgnoreHttpsErrors: playwright.Bool(true),
		}
		if userAgent != "" {
			options.UserAgent = playwright.String(userAgent)
		}
		return options
	}
	return playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  cfg.ViewportWidth,
			Height: cfg.ViewportHeight,
		},
		UserAgent:         playwright.String(userAgent),
		Locale:            playwright.String("en-US"),
		TimezoneId:        GetTimezoneForRegion(cfg.ProxyServer),
		IgnoreHttpsErrors: playwright.Bool(true),
	}
}
