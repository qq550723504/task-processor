package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"task-processor/internal/core/logger"

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
	userDataDir, err := resolveUserDataDir(cl.userDataDir)
	if err != nil {
		return nil, fmt.Errorf("解析用户数据目录失败: %w", err)
	}

	// 确保用户数据目录存在
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
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
		if proxy := parseProxyServer(cl.config.ProxyServer); proxy != nil {
			options.Proxy = proxy
			logger.GetGlobalLogger("crawler/shared").Infof("使用代理服务器: %s", proxy.Server)
		}
	}

	logger.GetGlobalLogger("crawler/shared").Infof("使用持久化上下文启动浏览器，用户数据目录: %s", userDataDir)

	// 启动持久化上下文
	context, err := (*cl.pw).Chromium.LaunchPersistentContext(userDataDir, options)
	if err != nil {
		return nil, fmt.Errorf("启动持久化上下文失败: %w", err)
	}
	if err := applyContextStealth(context); err != nil {
		context.Close()
		return nil, fmt.Errorf("注入浏览器反检测脚本失败: %w", err)
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
	if err := applyContextStealth(context); err != nil {
		context.Close()
		browser.Close()
		return nil, nil, fmt.Errorf("注入浏览器反检测脚本失败: %w", err)
	}

	return browser, context, nil
}

func applyContextStealth(context playwright.BrowserContext) error {
	if context == nil {
		return nil
	}
	script := `
(() => {
  const patchGetter = (target, key, getter) => {
    try {
      Object.defineProperty(target, key, {
        get: getter,
        configurable: true,
      });
    } catch (_) {}
  };

  patchGetter(Navigator.prototype, 'webdriver', () => undefined);
  patchGetter(Navigator.prototype, 'languages', () => ['zh-CN', 'zh', 'en-US', 'en']);
  patchGetter(Navigator.prototype, 'language', () => 'zh-CN');
  patchGetter(Navigator.prototype, 'plugins', () => [
    { name: 'Chrome PDF Plugin' },
    { name: 'Chrome PDF Viewer' },
    { name: 'Native Client' },
  ]);

  if (!window.chrome) {
    Object.defineProperty(window, 'chrome', {
      value: { runtime: {}, app: {} },
      configurable: true,
    });
  } else if (!window.chrome.runtime) {
    window.chrome.runtime = {};
  }

  const originalQuery = window.navigator.permissions && window.navigator.permissions.query;
  if (originalQuery) {
    window.navigator.permissions.query = (parameters) => (
      parameters && parameters.name === 'notifications'
        ? Promise.resolve({ state: Notification.permission })
        : originalQuery.call(window.navigator.permissions, parameters)
    );
  }
})();
`
	return context.AddInitScript(playwright.Script{
		Content: playwright.String(script),
	})
}

func resolveUserDataDir(dir string) (string, error) {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		return "", nil
	}
	absDir, err := filepath.Abs(trimmed)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absDir), nil
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

	if managedPath, err := findManagedPlaywrightBrowser(); err == nil && managedPath != "" {
		logger.GetGlobalLogger("crawler/shared").Infof("使用 Playwright 预装浏览器: %s", managedPath)
		return managedPath, nil
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

func findManagedPlaywrightBrowser() (string, error) {
	candidates := []string{}
	if browsersPath := os.Getenv("PLAYWRIGHT_BROWSERS_PATH"); browsersPath != "" {
		candidates = append(candidates, browsersPath)
	}
	candidates = append(candidates,
		"/ms-playwright",
		filepath.Join(os.Getenv("HOME"), ".cache", "ms-playwright"),
	)

	patterns := playwrightExecutablePatterns()
	for _, base := range candidates {
		if base == "" {
			continue
		}
		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(base, pattern))
			if err != nil {
				continue
			}
			sort.Strings(matches)
			for i := len(matches) - 1; i >= 0; i-- {
				if info, err := os.Stat(matches[i]); err == nil && !info.IsDir() {
					return matches[i], nil
				}
			}
		}
	}

	return "", fmt.Errorf("managed playwright browser not found")
}

func playwrightExecutablePatterns() []string {
	switch runtime.GOOS {
	case "linux":
		return []string{
			"chromium-*/chrome-linux64/chrome",
			"chromium-*/chrome-linux/chrome",
			"chromium_headless_shell-*/chrome-linux/headless_shell",
			"chromium_headless_shell-*/chrome-linux64/headless_shell",
		}
	case "darwin":
		return []string{
			"chromium-*/chrome-mac/Chromium.app/Contents/MacOS/Chromium",
		}
	case "windows":
		return []string{
			"chromium-*\\chrome-win\\chrome.exe",
		}
	default:
		return nil
	}
}
