package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// Manager 通用浏览器管理器
type Manager struct {
	pw          *playwright.Playwright
	browser     playwright.Browser
	context     playwright.BrowserContext
	config      *BrowserConfig
	fingerprint *FingerprintConfig
	userAgents  []string
	generator   *FingerprintGenerator
	userDataDir string // 用户数据目录（用于持久化上下文）
}

// NewManager 创建浏览器管理器
func NewManager(cfg *BrowserConfig) *Manager {
	return &Manager{
		config:     cfg,
		userAgents: GetUserAgentPool(),
		generator:  NewFingerprintGenerator(),
	}
}

// SetFingerprint 设置浏览器指纹
func (m *Manager) SetFingerprint(fingerprint *FingerprintConfig) {
	m.fingerprint = fingerprint
}

// SetUserDataDir 设置用户数据目录（用于持久化上下文，避免隐身模式检测）
func (m *Manager) SetUserDataDir(dir string) {
	m.userDataDir = dir
}

// GenerateRandomFingerprint 生成随机指纹
func (m *Manager) GenerateRandomFingerprint(publicIP string) *FingerprintConfig {
	return m.generator.GenerateRandomFingerprint(publicIP)
}

// GenerateStableFingerprint 生成稳定指纹
func (m *Manager) GenerateStableFingerprint(userID string) *FingerprintConfig {
	return m.generator.GenerateStableFingerprint(userID)
}

// Install 初始化Playwright
func (m *Manager) Install() error {
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("初始化playwright失败: %w", err)
	}
	m.pw = pw
	return nil
}

// Launch 启动浏览器（使用持久化上下文，与Python版本一致）
func (m *Manager) Launch() error {
	if m.pw == nil {
		return fmt.Errorf("playwright未初始化")
	}

	// 动态生成用户代理
	userAgent := GenerateUserAgent(m.config.UserAgent, m.userAgents)

	// 如果设置了用户数据目录，使用持久化上下文（与Python的launch_persistent_context一致）
	if m.userDataDir != "" {
		return m.launchPersistentContext(userAgent)
	}

	// 否则使用普通启动方式
	return m.launchNormalContext(userAgent)
}

// launchPersistentContext 使用持久化上下文启动（与Python版本一致，避免隐身模式检测）
func (m *Manager) launchPersistentContext(userAgent string) error {
	// 确保用户数据目录存在
	if err := os.MkdirAll(m.userDataDir, 0755); err != nil {
		return fmt.Errorf("创建用户数据目录失败: %w", err)
	}

	// 构建启动参数
	args := GetBrowserLaunchArgs()
	args = AddFingerprintArgs(args, m.fingerprint)

	// 获取需要排除的默认参数
	ignoreDefaultArgs := GetIgnoreDefaultArgs()

	// 创建持久化上下文选项（与Python版本的launch_persistent_context参数一致）
	options := playwright.BrowserTypeLaunchPersistentContextOptions{
		Headless:          playwright.Bool(m.config.Headless),
		Args:              args,
		IgnoreDefaultArgs: ignoreDefaultArgs,
		Viewport: &playwright.Size{
			Width:  m.config.ViewportWidth,
			Height: m.config.ViewportHeight,
		},
		UserAgent:         playwright.String(userAgent),
		Locale:            playwright.String(GetLocaleForRegion(m.config.ProxyServer)),
		TimezoneId:        GetTimezoneForRegion(m.config.ProxyServer),
		IgnoreHttpsErrors: playwright.Bool(true),
	}

	// 设置浏览器路径
	if m.config.BrowserPath != "" {
		if _, err := os.Stat(m.config.BrowserPath); err == nil {
			options.ExecutablePath = &m.config.BrowserPath
			logrus.Infof("使用指定的浏览器路径: %s", m.config.BrowserPath)
		}
	}

	// 配置代理
	if m.config.ProxyServer != "" {
		options.Proxy = &playwright.Proxy{
			Server: m.config.ProxyServer,
		}
		logrus.Infof("使用代理服务器: %s", m.config.ProxyServer)
	}

	logrus.Infof("使用持久化上下文启动浏览器，用户数据目录: %s", m.userDataDir)

	// 启动持久化上下文
	context, err := (*m.pw).Chromium.LaunchPersistentContext(m.userDataDir, options)
	if err != nil {
		return fmt.Errorf("启动持久化上下文失败: %w", err)
	}
	m.context = context

	return nil
}

// launchNormalContext 使用普通方式启动（兼容旧代码）
func (m *Manager) launchNormalContext(userAgent string) error {
	// 创建启动选项
	launchOptions := CreateLaunchOptions(m.config, m.fingerprint)

	browser, err := (*m.pw).Chromium.Launch(launchOptions)
	if err != nil {
		return fmt.Errorf("启动浏览器失败: %w", err)
	}
	m.browser = browser

	// 创建浏览器上下文选项
	contextOptions := CreateContextOptions(m.config, userAgent)

	context, err := browser.NewContext(contextOptions)
	if err != nil {
		return fmt.Errorf("创建浏览器上下文失败: %w", err)
	}
	m.context = context

	return nil
}

// NewPage 创建新页面
func (m *Manager) NewPage() (playwright.Page, error) {
	if m.context == nil {
		return nil, fmt.Errorf("浏览器上下文未初始化")
	}

	page, err := m.context.NewPage()
	if err != nil {
		return nil, fmt.Errorf("创建页面失败: %w", err)
	}

	// 应用反检测措施
	if err := ApplyAntiDetectionToPage(page, m.fingerprint); err != nil {
		return nil, fmt.Errorf("应用反检测措施失败: %w", err)
	}

	return page, nil
}

// NavigateTo 导航到URL（通用版本，子类可以重写）
func (m *Manager) NavigateTo(page playwright.Page, url string) error {
	_, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(30000),
	})

	if err != nil {
		// 重试一次
		_, err = page.Goto(url, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateLoad,
			Timeout:   playwright.Float(30000),
		})
	}

	return err
}

// Close 关闭浏览器
func (m *Manager) Close() {
	if m.context != nil {
		m.context.Close()
	}
	if m.browser != nil {
		m.browser.Close()
	}
	if m.pw != nil {
		(*m.pw).Stop()
	}
}

// GetContext 获取浏览器上下文（供子类使用）
func (m *Manager) GetContext() playwright.BrowserContext {
	return m.context
}

// GetDefaultUserDataDir 获取默认用户数据目录
func GetDefaultUserDataDir(identifier string) string {
	// 获取当前可执行文件目录
	execPath, err := os.Executable()
	if err != nil {
		return filepath.Join(".", "chromeData", identifier)
	}
	execDir := filepath.Dir(execPath)
	return filepath.Join(execDir, "chromeData", identifier)
}

// LogFingerprintConfig 打印指纹配置（调试用）
func LogFingerprintConfig(fingerprint *FingerprintConfig) {
	if fingerprint == nil {
		return
	}
	data, _ := json.MarshalIndent(fingerprint, "", "  ")
	logrus.Debugf("指纹配置: %s", string(data))
}
