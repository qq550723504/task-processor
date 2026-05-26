package browser

import (
	"encoding/json"
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
	args := GetBrowserLaunchArgs(cl.config)
	if cl.config == nil || cl.config.StealthProvider != StealthProviderCloakBrowser {
		args = AddFingerprintArgs(args, cl.config, cl.fingerprint)
	}

	// 获取需要排除的默认参数
	ignoreDefaultArgs := GetIgnoreDefaultArgs(cl.config)

	// 创建持久化上下文选项
	options := playwright.BrowserTypeLaunchPersistentContextOptions{
		Headless:          playwright.Bool(cl.config.Headless),
		Args:              args,
		IgnoreDefaultArgs: ignoreDefaultArgs,
		IgnoreHttpsErrors: playwright.Bool(true),
	}
	if cl.config == nil || cl.config.StealthProvider != StealthProviderCloakBrowser {
		options.Viewport = &playwright.Size{
			Width:  cl.config.ViewportWidth,
			Height: cl.config.ViewportHeight,
		}
		options.UserAgent = playwright.String(userAgent)
		options.Locale = playwright.String(contextLocale(cl.config))
		options.TimezoneId = contextTimezone(cl.config)
	} else {
		options.Viewport = &playwright.Size{
			Width:  cl.config.ViewportWidth,
			Height: cl.config.ViewportHeight,
		}
		if userAgent != "" {
			options.UserAgent = playwright.String(userAgent)
		}
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
	if cl.config == nil || cl.config.StealthProvider != StealthProviderCloakBrowser {
		if err := applyContextFingerprint(context, cl.config, cl.fingerprint); err != nil {
			context.Close()
			return nil, fmt.Errorf("注入浏览器反检测脚本失败: %w", err)
		}
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
	if cl.config == nil || cl.config.StealthProvider != StealthProviderCloakBrowser {
		if err := applyContextFingerprint(context, cl.config, cl.fingerprint); err != nil {
			context.Close()
			browser.Close()
			return nil, nil, fmt.Errorf("注入浏览器反检测脚本失败: %w", err)
		}
	}

	return browser, context, nil
}

func applyContextFingerprint(context playwright.BrowserContext, cfg *BrowserConfig, fingerprint *FingerprintConfig) error {
	if context == nil {
		return nil
	}
	headers := map[string]string{
		"Accept-Language": contextAcceptLanguage(cfg, fingerprint),
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
	}
	if err := context.SetExtraHTTPHeaders(headers); err != nil {
		return err
	}
	script, err := buildContextFingerprintScript(cfg, fingerprint)
	if err != nil {
		return err
	}
	return context.AddInitScript(playwright.Script{
		Content: playwright.String(script),
	})
}

func buildContextFingerprintScript(cfg *BrowserConfig, fingerprint *FingerprintConfig) (string, error) {
	payload := map[string]any{
		"languages": map[string]any{
			"http": contextAcceptLanguage(cfg, fingerprint),
			"js":   contextJSLanguages(cfg, fingerprint),
		},
		"platform": platformForScript(cfg),
		"screen": map[string]int{
			"width":  cfg.ViewportWidth,
			"height": cfg.ViewportHeight,
		},
		"viewport": map[string]int{
			"width":  cfg.ViewportWidth,
			"height": cfg.ViewportHeight,
		},
		"gpu": map[string]string{
			"vendor":   contextGPUVendor(cfg, fingerprint),
			"renderer": contextGPURenderer(cfg),
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	script := fmt.Sprintf(`
(() => {
  const fp = %s;
  const langList = (fp.languages && fp.languages.js) || ['en-US', 'en'];
  const platformName = fp.platform || 'Win32';
  const screenInfo = fp.screen || fp.viewport || { width: 1440, height: 900 };
  const gpu = fp.gpu || {};
  const overrideGetter = (obj, key, value) => {
    try {
      Object.defineProperty(obj, key, { get: () => value, configurable: true });
    } catch (_) {}
  };

  overrideGetter(Navigator.prototype, 'webdriver', false);
  overrideGetter(Navigator.prototype, 'language', langList[0] || 'en-US');
  overrideGetter(Navigator.prototype, 'languages', langList);
  overrideGetter(Navigator.prototype, 'platform', platformName);
  overrideGetter(Screen.prototype, 'width', screenInfo.width || 1440);
  overrideGetter(Screen.prototype, 'height', screenInfo.height || 900);
  overrideGetter(Screen.prototype, 'availWidth', screenInfo.width || 1440);
  overrideGetter(Screen.prototype, 'availHeight', screenInfo.height || 900);

  const patchWebgl = (proto) => {
    if (!proto) return;
    const originalGetParameter = proto.getParameter;
    proto.getParameter = function(param) {
      if (param === 37445 && gpu.vendor) return gpu.vendor;
      if (param === 37446 && gpu.renderer) return gpu.renderer;
      return originalGetParameter.apply(this, [param]);
    };
  };

  patchWebgl(window.WebGLRenderingContext && WebGLRenderingContext.prototype);
  patchWebgl(window.WebGL2RenderingContext && WebGL2RenderingContext.prototype);
})();
`, string(data))
	return script, nil
}

func contextLocale(cfg *BrowserConfig) string {
	if cfg != nil && strings.TrimSpace(cfg.Language) != "" {
		return strings.TrimSpace(cfg.Language)
	}
	return "en-US"
}

func contextTimezone(cfg *BrowserConfig) *string {
	value := ""
	if cfg != nil && strings.TrimSpace(cfg.Timezone) != "" {
		value = strings.TrimSpace(cfg.Timezone)
	} else if cfg != nil {
		if inferred := GetTimezoneForRegion(cfg.ProxyServer); inferred != nil {
			value = strings.TrimSpace(*inferred)
		}
	}
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func contextAcceptLanguage(cfg *BrowserConfig, fingerprint *FingerprintConfig) string {
	if cfg != nil && strings.TrimSpace(cfg.AcceptLanguage) != "" {
		return strings.TrimSpace(cfg.AcceptLanguage)
	}
	if fingerprint != nil && strings.TrimSpace(fingerprint.Languages.HTTP) != "" {
		return strings.TrimSpace(fingerprint.Languages.HTTP)
	}
	return "en-US,en;q=0.9"
}

func contextJSLanguages(cfg *BrowserConfig, fingerprint *FingerprintConfig) []string {
	if cfg != nil && strings.TrimSpace(cfg.Language) != "" {
		primary := strings.TrimSpace(cfg.Language)
		if strings.EqualFold(primary, "en-US") {
			return []string{"en-US", "en"}
		}
		return []string{primary, "en-US", "en"}
	}
	if fingerprint != nil && strings.TrimSpace(fingerprint.Languages.JS) != "" {
		primary := strings.TrimSpace(fingerprint.Languages.JS)
		if strings.EqualFold(primary, "en-US") {
			return []string{"en-US", "en"}
		}
		return []string{primary, "en-US", "en"}
	}
	return []string{"en-US", "en"}
}

func platformForScript(cfg *BrowserConfig) string {
	if cfg != nil {
		switch strings.ToLower(strings.TrimSpace(cfg.FingerprintPlatform)) {
		case "linux":
			return "Linux x86_64"
		case "mac", "macos":
			return "MacIntel"
		}
	}
	return "Win32"
}

func contextGPUVendor(cfg *BrowserConfig, fingerprint *FingerprintConfig) string {
	if cfg != nil && strings.TrimSpace(cfg.FingerprintGPUVendor) != "" {
		return strings.TrimSpace(cfg.FingerprintGPUVendor)
	}
	if fingerprint != nil && strings.TrimSpace(fingerprint.GPU["description"]) != "" {
		return strings.TrimSpace(fingerprint.GPU["description"])
	}
	return ""
}

func contextGPURenderer(cfg *BrowserConfig) string {
	if cfg != nil {
		return strings.TrimSpace(cfg.FingerprintGPURenderer)
	}
	return ""
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
