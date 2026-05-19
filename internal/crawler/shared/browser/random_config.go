package browser

import (
	"math/rand"
	"task-processor/internal/core/logger"
	"time"
)

// RandomConfigGenerator 随机配置生成器
type RandomConfigGenerator struct {
	platforms     []PlatformConfig
	browsers      []BrowserBrandConfig
	gpuVendors    []GPUVendorConfig
	languages     []LanguageConfig
	timezones     []string
	hardwareCores []int
}

// PlatformConfig 平台配置
type PlatformConfig struct {
	Platform string
	Versions []string
}

// BrowserBrandConfig 浏览器品牌配置
type BrowserBrandConfig struct {
	Brand    string
	Versions []string
}

// GPUVendorConfig GPU厂商配置
type GPUVendorConfig struct {
	Vendor    string
	Renderers []string
}

// NewRandomConfigGenerator 创建随机配置生成器
func NewRandomConfigGenerator() *RandomConfigGenerator {
	return &RandomConfigGenerator{
		platforms: []PlatformConfig{
			{
				Platform: "windows",
				Versions: []string{"10.0.19041", "10.0.19042", "10.0.19043", "10.0.19044", "10.0.22000", "10.0.22621"},
			},
			{
				Platform: "linux",
				Versions: []string{"5.4.0", "5.8.0", "5.11.0", "5.15.0", "6.1.0", "6.2.0"},
			},
			{
				Platform: "macos",
				Versions: []string{"10.15.7", "11.7.10", "12.7.1", "13.6.1", "14.1.2", "14.2.1"},
			},
		},
		browsers: []BrowserBrandConfig{
			{
				Brand:    "Chrome",
				Versions: []string{"142.0.7444.175", "141.0.7364.172", "140.0.7311.135", "139.0.7258.154"},
			},
			{
				Brand:    "Edge",
				Versions: []string{"142.0.2739.67", "141.0.2704.106", "140.0.2661.102", "139.0.2619.55"},
			},
			{
				Brand:    "Opera",
				Versions: []string{"106.0.4998.70", "105.0.4970.48", "104.0.4944.54", "103.0.4928.34"},
			},
			{
				Brand:    "Vivaldi",
				Versions: []string{"6.5.3206.63", "6.4.3160.47", "6.2.3105.58", "6.1.3035.111"},
			},
		},
		gpuVendors: []GPUVendorConfig{
			{
				Vendor: "NVIDIA Corporation",
				Renderers: []string{
					"NVIDIA GeForce RTX 4090",
					"NVIDIA GeForce RTX 4080",
					"NVIDIA GeForce RTX 4070",
					"NVIDIA GeForce RTX 4060",
					"NVIDIA GeForce RTX 3080",
					"NVIDIA GeForce RTX 3070",
					"NVIDIA GeForce RTX 3060",
					"NVIDIA GeForce GTX 1660 Ti",
					"NVIDIA GeForce GTX 1650",
					"NVIDIA GeForce GTX 1080",
				},
			},
			{
				Vendor: "Intel Inc.",
				Renderers: []string{
					"Intel Iris Xe Graphics",
					"Intel UHD Graphics 630",
					"Intel UHD Graphics 620",
					"Intel HD Graphics 530",
					"Intel Iris Pro Graphics 580",
				},
			},
			{
				Vendor: "AMD",
				Renderers: []string{
					"AMD Radeon RX 6800 XT",
					"AMD Radeon RX 6700 XT",
					"AMD Radeon RX 5700 XT",
					"AMD Radeon RX 580",
					"AMD Radeon Vega 64",
				},
			},
		},
		languages: []LanguageConfig{
			{HTTP: "en-US,en;q=0.9", JS: "en-US"},
			{HTTP: "zh-CN,zh;q=0.9,en;q=0.8", JS: "zh-CN"},
			{HTTP: "ja-JP,ja;q=0.9,en;q=0.8", JS: "ja-JP"},
			{HTTP: "ko-KR,ko;q=0.9,en;q=0.8", JS: "ko-KR"},
			{HTTP: "de-DE,de;q=0.9,en;q=0.8", JS: "de-DE"},
			{HTTP: "fr-FR,fr;q=0.9,en;q=0.8", JS: "fr-FR"},
			{HTTP: "es-ES,es;q=0.9,en;q=0.8", JS: "es-ES"},
		},
		timezones: []string{
			"America/New_York",
			"America/Los_Angeles",
			"Europe/London",
			"Europe/Berlin",
			"Asia/Tokyo",
			"Asia/Shanghai",
			"Asia/Seoul",
			"Australia/Sydney",
			"America/Chicago",
			"Europe/Paris",
		},
		hardwareCores: []int{4, 6, 8, 12, 16, 20, 24, 32},
	}
}

// GenerateRandomBrowserConfig 生成随机浏览器配置
func (rcg *RandomConfigGenerator) GenerateRandomBrowserConfig() *BrowserConfig {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 随机选择平台
	platform := rcg.platforms[r.Intn(len(rcg.platforms))]
	platformVersion := platform.Versions[r.Intn(len(platform.Versions))]

	// 随机选择浏览器品牌
	browser := rcg.browsers[r.Intn(len(rcg.browsers))]
	browserVersion := browser.Versions[r.Intn(len(browser.Versions))]

	// 随机选择GPU
	gpu := rcg.gpuVendors[r.Intn(len(rcg.gpuVendors))]
	gpuRenderer := gpu.Renderers[r.Intn(len(gpu.Renderers))]

	// 随机选择语言
	language := rcg.languages[r.Intn(len(rcg.languages))]

	// 随机选择时区
	timezone := rcg.timezones[r.Intn(len(rcg.timezones))]

	// 随机选择CPU核心数
	cores := rcg.hardwareCores[r.Intn(len(rcg.hardwareCores))]

	return &BrowserConfig{
		Headless:                       false,
		BrowserPath:                    "./.local/chrome/chrome.exe",
		ViewportWidth:                  1920,
		ViewportHeight:                 1080,
		FingerprintSeed:                0, // 0表示使用随机种子
		FingerprintPlatform:            platform.Platform,
		FingerprintPlatformVersion:     platformVersion,
		FingerprintBrand:               browser.Brand,
		FingerprintBrandVersion:        browserVersion,
		FingerprintHardwareConcurrency: cores,
		FingerprintGPUVendor:           gpu.Vendor,
		FingerprintGPURenderer:         gpuRenderer,
		Language:                       language.JS,
		AcceptLanguage:                 language.HTTP,
		Timezone:                       timezone,
		DisableGPUFingerprint:          r.Float32() < 0.1, // 10%概率禁用GPU指纹
	}
}

// GenerateStableBrowserConfig 生成稳定的浏览器配置（基于种子）
func (rcg *RandomConfigGenerator) GenerateStableBrowserConfig(seed int64) *BrowserConfig {
	r := rand.New(rand.NewSource(seed))

	// 稳定选择平台
	platform := rcg.platforms[r.Intn(len(rcg.platforms))]
	platformVersion := platform.Versions[r.Intn(len(platform.Versions))]

	// 稳定选择浏览器品牌
	browser := rcg.browsers[r.Intn(len(rcg.browsers))]
	browserVersion := browser.Versions[r.Intn(len(browser.Versions))]

	// 稳定选择GPU
	gpu := rcg.gpuVendors[r.Intn(len(rcg.gpuVendors))]
	gpuRenderer := gpu.Renderers[r.Intn(len(gpu.Renderers))]

	// 稳定选择语言
	language := rcg.languages[r.Intn(len(rcg.languages))]

	// 稳定选择时区
	timezone := rcg.timezones[r.Intn(len(rcg.timezones))]

	// 稳定选择CPU核心数
	cores := rcg.hardwareCores[r.Intn(len(rcg.hardwareCores))]

	return &BrowserConfig{
		Headless:                       false,
		BrowserPath:                    "./.local/chrome/chrome.exe",
		ViewportWidth:                  1920,
		ViewportHeight:                 1080,
		FingerprintSeed:                int32(seed),
		FingerprintPlatform:            platform.Platform,
		FingerprintPlatformVersion:     platformVersion,
		FingerprintBrand:               browser.Brand,
		FingerprintBrandVersion:        browserVersion,
		FingerprintHardwareConcurrency: cores,
		FingerprintGPUVendor:           gpu.Vendor,
		FingerprintGPURenderer:         gpuRenderer,
		Language:                       language.JS,
		AcceptLanguage:                 language.HTTP,
		Timezone:                       timezone,
		DisableGPUFingerprint:          r.Float32() < 0.1, // 10%概率禁用GPU指纹
	}
}

// GenerateWindowsConfig 生成Windows专用配置
func (rcg *RandomConfigGenerator) GenerateWindowsConfig() *BrowserConfig {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 只选择Windows平台
	windowsPlatform := rcg.platforms[0] // Windows是第一个
	platformVersion := windowsPlatform.Versions[r.Intn(len(windowsPlatform.Versions))]

	// 偏向选择Chrome
	browser := rcg.browsers[0] // Chrome是第一个
	browserVersion := browser.Versions[r.Intn(len(browser.Versions))]

	// 偏向选择NVIDIA GPU
	gpu := rcg.gpuVendors[0] // NVIDIA是第一个
	gpuRenderer := gpu.Renderers[r.Intn(len(gpu.Renderers))]

	// 使用英语
	language := rcg.languages[0] // 英语是第一个

	// 使用美国时区
	timezone := "America/New_York"

	// 选择较高的CPU核心数
	cores := rcg.hardwareCores[r.Intn(4)+4] // 选择后4个（较高的核心数）

	return &BrowserConfig{
		Headless:                       false,
		BrowserPath:                    "./.local/chrome/chrome.exe",
		ViewportWidth:                  1920,
		ViewportHeight:                 1080,
		FingerprintSeed:                0,
		FingerprintPlatform:            windowsPlatform.Platform,
		FingerprintPlatformVersion:     platformVersion,
		FingerprintBrand:               browser.Brand,
		FingerprintBrandVersion:        browserVersion,
		FingerprintHardwareConcurrency: cores,
		FingerprintGPUVendor:           gpu.Vendor,
		FingerprintGPURenderer:         gpuRenderer,
		Language:                       language.JS,
		AcceptLanguage:                 language.HTTP,
		Timezone:                       timezone,
		DisableGPUFingerprint:          false, // Windows配置不禁用GPU指纹
	}
}

// PrintConfigSummary 打印配置摘要
func PrintConfigSummary(config *BrowserConfig) {
	log := logger.GetGlobalLogger("browser_config")
	log.Infof("=== 浏览器配置摘要 === 平台: %s %s | 浏览器: %s %s | GPU: %s - %s | CPU核心数: %d | 语言: %s (%s) | 时区: %s | 指纹种子: %d | 禁用GPU指纹: %t",
		config.FingerprintPlatform, config.FingerprintPlatformVersion,
		config.FingerprintBrand, config.FingerprintBrandVersion,
		config.FingerprintGPUVendor, config.FingerprintGPURenderer,
		config.FingerprintHardwareConcurrency,
		config.Language, config.AcceptLanguage,
		config.Timezone,
		config.FingerprintSeed,
		config.DisableGPUFingerprint,
	)
}

// GenerateConfigPresets 生成配置预设
func GenerateConfigPresets() map[string]*BrowserConfig {
	presets := make(map[string]*BrowserConfig)

	// Windows 高端配置
	presets["windows_high_end"] = &BrowserConfig{
		Headless:                       false,
		BrowserPath:                    "./.local/chrome/chrome.exe",
		ViewportWidth:                  1920,
		ViewportHeight:                 1080,
		FingerprintPlatform:            "windows",
		FingerprintPlatformVersion:     "10.0.22621",
		FingerprintBrand:               "Chrome",
		FingerprintBrandVersion:        "142.0.7444.175",
		FingerprintHardwareConcurrency: 16,
		FingerprintGPUVendor:           "NVIDIA Corporation",
		FingerprintGPURenderer:         "NVIDIA GeForce RTX 4090",
		Language:                       "en-US",
		AcceptLanguage:                 "en-US,en;q=0.9",
		Timezone:                       "America/New_York",
		DisableGPUFingerprint:          false,
	}

	// Windows 中端配置
	presets["windows_mid_range"] = &BrowserConfig{
		Headless:                       false,
		BrowserPath:                    "./.local/chrome/chrome.exe",
		ViewportWidth:                  1920,
		ViewportHeight:                 1080,
		FingerprintPlatform:            "windows",
		FingerprintPlatformVersion:     "10.0.19044",
		FingerprintBrand:               "Chrome",
		FingerprintBrandVersion:        "141.0.7364.172",
		FingerprintHardwareConcurrency: 8,
		FingerprintGPUVendor:           "NVIDIA Corporation",
		FingerprintGPURenderer:         "NVIDIA GeForce GTX 1660 Ti",
		Language:                       "en-US",
		AcceptLanguage:                 "en-US,en;q=0.9",
		Timezone:                       "America/New_York",
		DisableGPUFingerprint:          false,
	}

	// Mac 高端配置
	presets["mac_high_end"] = &BrowserConfig{
		Headless:                       false,
		BrowserPath:                    "./.local/chrome/chrome.exe",
		ViewportWidth:                  1920,
		ViewportHeight:                 1080,
		FingerprintPlatform:            "macos",
		FingerprintPlatformVersion:     "14.2.1",
		FingerprintBrand:               "Chrome",
		FingerprintBrandVersion:        "142.0.7444.175",
		FingerprintHardwareConcurrency: 12,
		FingerprintGPUVendor:           "Apple",
		FingerprintGPURenderer:         "Apple M2 Pro",
		Language:                       "en-US",
		AcceptLanguage:                 "en-US,en;q=0.9",
		Timezone:                       "America/New_York",
		DisableGPUFingerprint:          false,
	}

	return presets
}

// ValidateConfig 验证浏览器配置
func ValidateConfig(config *BrowserConfig) []string {
	var issues []string

	if config == nil {
		return []string{"配置为空"}
	}

	// 验证平台
	if config.FingerprintPlatform == "" {
		issues = append(issues, "平台不能为空")
	}

	// 验证浏览器品牌
	if config.FingerprintBrand == "" {
		issues = append(issues, "浏览器品牌不能为空")
	}

	// 验证视口尺寸
	if config.ViewportWidth <= 0 || config.ViewportHeight <= 0 {
		issues = append(issues, "视口尺寸必须大于0")
	}

	// 验证硬件并发数
	if config.FingerprintHardwareConcurrency <= 0 {
		issues = append(issues, "硬件并发数必须大于0")
	}

	return issues
}

// TestRandomConfigPerformance 性能测试
func TestRandomConfigPerformance() {
	log := logger.GetGlobalLogger("browser_config")
	generator := NewRandomConfigGenerator()

	start := time.Now()
	for i := 0; i < 1000; i++ {
		_ = generator.GenerateRandomBrowserConfig()
	}
	duration := time.Since(start)

	log.Infof("生成1000个随机配置耗时: %v，平均每个配置: %v", duration, duration/1000)
}
