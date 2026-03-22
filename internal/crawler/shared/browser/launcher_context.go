package browser

import (
	"task-processor/internal/core/logger"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// ContextLauncher 浏览器上下文启动器
type ContextLauncher struct {
	pw          *playwright.Playwright
	config      *BrowserConfig
	fingerprint *FingerprintConfig
	userDataDir string
}

// NewContextLauncher 创建上下文启动器
func NewContextLauncher(pw *playwright.Playwright, cfg *BrowserConfig, fingerprint *FingerprintConfig, userDataDir string) *ContextLauncher {
	return &ContextLauncher{
		pw:          pw,
		config:      cfg,
		fingerprint: fingerprint,
		userDataDir: userDataDir,
	}
}

// Launch 启动浏览器上下文
func (cl *ContextLauncher) Launch(userAgent string) (playwright.Browser, playwright.BrowserContext, error) {
	// 如果设置了用户数据目录，使用持久化上下文
	if cl.userDataDir != "" {
		context, err := cl.launchPersistentContext(userAgent)
		return nil, context, err
	}

	// 否则使用普通启动方式
	return cl.launchNormalContext(userAgent)
}

// launchPersistentContext 使用持久化上下文启动（避免隐身模式检测）
func (cl *ContextLauncher) launchPersistentContext(userAgent string) (playwright.BrowserContext, error) {
	// 确保用户数据目录存在
	if err := os.MkdirAll(cl.userDataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建用户数据目录失败: %w", err)
	}

	// 构建启动参数
	args := GetBrowserLaunchArgs()
	args = AddFingerprintArgs(args, cl.config, cl.fingerprint)

	// 获取需要排除的默认参数
	ignoreDefaultArgs := GetIgnoreDefaultArgs()

	// 创建持久化上下文选项
	options := playwright.BrowserTypeLaunchPersistentContextOptions{
		Headless:          playwright.Bool(cl.config.Headless),
		Args:              args,
		IgnoreDefaultArgs: ignoreDefaultArgs,
		Viewport: &playwright.Size{
			Width:  cl.config.ViewportWidth,
			Height: cl.config.ViewportHeight,
		},
		UserAgent:         playwright.String(userAgent),
		Locale:            playwright.String(GetLocaleForRegion(cl.config.ProxyServer)),
		TimezoneId:        GetTimezoneForRegion(cl.config.ProxyServer),
		IgnoreHttpsErrors: playwright.Bool(true),
	}

	// 设置浏览器路径（支持自动下载）
	browserPath, err := cl.ensureBrowserPath()
	if err != nil {
		logger.GetGlobalLogger("crawler/shared").Warnf("获取浏览器路径失败: %v，使用默认浏览器", err)
	} else if browserPath != "" {
		options.ExecutablePath = &browserPath
		logger.GetGlobalLogger("crawler/shared").Infof("使用浏览器路径: %s", browserPath)
	}

	// 配置代理
	if cl.config.ProxyServer != "" {
		options.Proxy = &playwright.Proxy{
			Server: cl.config.ProxyServer,
		}
		logger.GetGlobalLogger("crawler/shared").Infof("使用代理服务器: %s", cl.config.ProxyServer)
	}

	logger.GetGlobalLogger("crawler/shared").Infof("使用持久化上下文启动浏览器，用户数据目录: %s", cl.userDataDir)

	// 启动持久化上下文
	context, err := (*cl.pw).Chromium.LaunchPersistentContext(cl.userDataDir, options)
	if err != nil {
		return nil, fmt.Errorf("启动持久化上下文失败: %w", err)
	}

	return context, nil
}

// launchNormalContext 使用普通方式启动
func (cl *ContextLauncher) launchNormalContext(userAgent string) (playwright.Browser, playwright.BrowserContext, error) {
	// 创建启动选项
	launchOptions := CreateLaunchOptions(cl.config, cl.fingerprint)

	// 设置浏览器路径（支持自动下载）
	browserPath, err := cl.ensureBrowserPath()
	if err != nil {
		logger.GetGlobalLogger("crawler/shared").Warnf("获取浏览器路径失败: %v，使用默认浏览器", err)
	} else if browserPath != "" {
		launchOptions.ExecutablePath = &browserPath
		logger.GetGlobalLogger("crawler/shared").Infof("使用浏览器路径: %s", browserPath)
	}

	browser, err := (*cl.pw).Chromium.Launch(launchOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("启动浏览器失败: %w", err)
	}

	// 创建浏览器上下文选项
	contextOptions := CreateContextOptions(cl.config, userAgent)

	context, err := browser.NewContext(contextOptions)
	if err != nil {
		browser.Close()
		return nil, nil, fmt.Errorf("创建浏览器上下文失败: %w", err)
	}

	return browser, context, nil
}

// ensureBrowserPath 确保浏览器路径存在（支持自动下载）
func (cl *ContextLauncher) ensureBrowserPath() (string, error) {
	// 如果配置了浏览器路径，检查是否适用于当前平台
	if cl.config.BrowserPath != "" {
		// Windows exe 在非 Windows 系统上直接跳过，走自动下载
		if runtime.GOOS != "windows" && strings.HasSuffix(strings.ToLower(cl.config.BrowserPath), ".exe") {
			logger.GetGlobalLogger("crawler/shared").Infof("当前系统为 %s，跳过 Windows 浏览器路径: %s", runtime.GOOS, cl.config.BrowserPath)
		} else if _, err := os.Stat(cl.config.BrowserPath); err == nil {
			return cl.config.BrowserPath, nil
		} else {
			logger.GetGlobalLogger("crawler/shared").Warnf("配置的浏览器路径不存在: %s，尝试自动下载", cl.config.BrowserPath)
		}
	}

	// 从配置中获取 Chrome 版本和下载目录
	version := cl.config.ChromeVersion
	if version == "" {
		version = "144" // 默认版本
	}

	downloadDir := cl.config.ChromeDownloadDir
	if downloadDir == "" {
		downloadDir = "./chrome" // 默认下载目录
	}

	// 创建下载器并检查/下载 Chrome
	downloader := NewChromeDownloader(version, downloadDir)
	chromePath, err := downloader.CheckAndDownload(cl.config.BrowserPath)
	if err != nil {
		return "", fmt.Errorf("自动下载 Chrome 失败: %w", err)
	}

	return chromePath, nil
}
