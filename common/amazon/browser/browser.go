package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"task-processor/common/config"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// BrowserManager 浏览器管理器
type BrowserManager struct {
	pw          *playwright.Playwright
	browser     playwright.Browser
	context     playwright.BrowserContext
	config      *config.AmazonConfig
	fingerprint *FingerprintConfig
}

// FingerprintConfig 浏览器指纹配置
type FingerprintConfig struct {
	Enable bool                   `json:"enable"`
	GPU    map[string]interface{} `json:"gpu,omitempty"`
}

// NewBrowserManager 创建浏览器管理器
func NewBrowserManager(cfg *config.AmazonConfig) *BrowserManager {
	return &BrowserManager{
		config: cfg,
	}
}

// SetFingerprint 设置浏览器指纹
func (bm *BrowserManager) SetFingerprint(fingerprint *FingerprintConfig) {
	bm.fingerprint = fingerprint
}

// Install 初始化Playwright
func (bm *BrowserManager) Install() error {
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("初始化playwright失败: %w", err)
	}
	bm.pw = pw
	return nil
}

// Launch 启动浏览器
func (bm *BrowserManager) Launch() error {
	if bm.pw == nil {
		return fmt.Errorf("playwright未初始化")
	}

	// 准备启动参数 - 添加反检测参数
	args := []string{
		"--no-first-run",                                // 禁用首次运行体验
		"--no-default-browser-check",                    // 禁用默认浏览器检查
		"--disable-blink-features=AutomationControlled", // 禁用自动化控制特征
		"--disable-features=VizDisplayCompositor",       // 禁用显示合成器
		"--disable-ipc-flooding-protection",             // 禁用IPC洪水保护
		"--disable-renderer-backgrounding",              // 禁用渲染器后台化
		"--disable-backgrounding-occluded-windows",      // 禁用被遮挡窗口的后台化
		"--disable-client-side-phishing-detection",      // 禁用客户端钓鱼检测
		"--disable-sync",                                // 禁用同步
		"--disable-background-networking",               // 禁用后台网络
		"--disable-background-timer-throttling",         // 禁用后台定时器节流
		"--disable-dev-shm-usage",                       // 禁用/dev/shm使用
		"--disable-extensions",                          // 禁用扩展
		"--disable-hang-monitor",                        // 禁用挂起监视器
		"--disable-popup-blocking",                      // 禁用弹窗阻止
		"--disable-prompt-on-repost",                    // 禁用重新提交提示
		"--disable-domain-reliability",                  // 禁用域可靠性
		"--disable-component-update",                    // 禁用组件更新
		"--no-sandbox",                                  // 禁用沙箱（注意：这会降低安全性）
		"--disable-web-security",                        // 禁用Web安全（注意：这会降低安全性）
		"--allow-running-insecure-content",              // 允许运行不安全内容
		"--disable-features=TranslateUI",                // 禁用翻译UI
		"--disable-features=Translate",                  // 禁用翻译功能
		"--lang=en-US",                                  // 设置语言为英文
	}

	// 如果设置了指纹，通过--kfingerprint参数传递
	if bm.fingerprint != nil && bm.fingerprint.Enable {
		fingerprintJSON, err := json.Marshal(bm.fingerprint)
		if err != nil {
			logrus.Infof("序列化指纹配置失败: %v", err)
		} else {
			kfingerprintArg := fmt.Sprintf("--kfingerprint=%s", string(fingerprintJSON))
			args = append(args, kfingerprintArg)
			logrus.Infof("通过--kfingerprint参数注入指纹配置")
		}
	}

	launchOptions := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(bm.config.Headless),
		Args:     args,
	}

	// 检查浏览器路径是否存在，如果不存在则使用系统默认的Chrome
	if bm.config.BrowserPath != "" {
		// 检查文件是否存在
		if _, err := os.Stat(bm.config.BrowserPath); err == nil {
			launchOptions.ExecutablePath = &bm.config.BrowserPath
			logrus.Infof("使用指定的浏览器路径: %s", bm.config.BrowserPath)
		} else {
			logrus.Infof("警告: 指定的浏览器路径不存在: %s，错误: %v", bm.config.BrowserPath, err)
			logrus.Infof("将使用系统默认Chrome或Playwright自带的浏览器")
			// 不设置ExecutablePath，让Playwright使用默认的Chrome
		}
	} else {
		logrus.Infof("未指定浏览器路径，使用Playwright默认浏览器")
	}

	// 配置代理（如果设置了）
	if bm.config.ProxyServer != "" {
		launchOptions.Proxy = &playwright.Proxy{
			Server: bm.config.ProxyServer,
		}
		logrus.Infof("使用代理服务器: %s", bm.config.ProxyServer)
	}

	browser, err := (*bm.pw).Chromium.Launch(launchOptions)
	if err != nil {
		return fmt.Errorf("启动浏览器失败: %w", err)
	}
	bm.browser = browser

	// 使用默认的用户代理和视口配置
	contextOptions := playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  bm.config.ViewportWidth,
			Height: bm.config.ViewportHeight,
		},
		UserAgent: playwright.String("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		Locale:    playwright.String("en-US"), // Force English locale for all regions
	}

	// Initialize extra HTTP headers
	if contextOptions.ExtraHttpHeaders == nil {
		contextOptions.ExtraHttpHeaders = make(map[string]string)
	}
	// Force English language in HTTP headers for all regions
	contextOptions.ExtraHttpHeaders["Accept-Language"] = "en-US,en;q=0.9"

	// 如果设置了指纹，应用指纹配置（但保持英语语言）
	if bm.fingerprint != nil && bm.fingerprint.Enable {
		logrus.Info("应用浏览器指纹配置")
		// Note: Language is forced to English regardless of fingerprint settings
		// This ensures Amazon pages are always displayed in English
	}

	context, err := browser.NewContext(contextOptions)
	if err != nil {
		return fmt.Errorf("创建浏览器上下文失败: %w", err)
	}
	bm.context = context

	return nil
}

// NewPage 创建新页面
func (bm *BrowserManager) NewPage() (playwright.Page, error) {
	if bm.context == nil {
		return nil, fmt.Errorf("浏览器上下文未初始化")
	}

	page, err := bm.context.NewPage()
	if err != nil {
		return nil, fmt.Errorf("创建页面失败: %w", err)
	}

	// 指纹已通过--kfingerprint参数在Chrome启动时注入，无需额外处理
	if bm.fingerprint != nil && bm.fingerprint.Enable {
		logrus.Info("指纹已通过--kfingerprint参数注入到Chrome")
	}

	return page, nil
}

// NavigateTo 导航到URL
func (bm *BrowserManager) NavigateTo(page playwright.Page, url string) error {
	// Set language preference cookies before navigation
	if err := bm.setLanguageCookies(url); err != nil {
		logrus.Infof("设置语言Cookie失败: %v", err)
	}

	_, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(30000),
	})

	if err != nil {
		logrus.Infof("导航失败，尝试重试: %v", err)
		_, err = page.Goto(url, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateLoad,
			Timeout:   playwright.Float(30000),
		})
	}

	return err
}

// setLanguageCookies 设置语言偏好Cookie，确保页面显示英语，但货币根据地区设置
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
	currency := getCurrencyByRegion(region)

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

	if err := bm.context.AddCookies(cookies); err != nil {
		return fmt.Errorf("添加语言Cookie失败: %w", err)
	}

	return nil
}

// getCurrencyByRegion 根据地区获取货币代码
func getCurrencyByRegion(region string) string {
	switch region {
	case "US":
		return "USD"
	case "FR":
		return "EUR"
	case "DE":
		return "EUR"
	case "IT":
		return "EUR"
	case "ES":
		return "EUR"
	case "UK":
		return "GBP"
	case "AU":
		return "AUD"
	case "JP":
		return "JPY"
	case "CA":
		return "CAD"
	case "MX":
		return "MXN"
	case "SA":
		return "SAR" // 沙特里亚尔
	case "AE":
		return "AED" // 阿联酋迪拉姆
	default:
		return "USD"
	}
}

// Close 关闭浏览器
func (bm *BrowserManager) Close() {
	if bm.context != nil {
		bm.context.Close()
	}
	if bm.browser != nil {
		bm.browser.Close()
	}
	if bm.pw != nil {
		(*bm.pw).Stop()
	}
}
