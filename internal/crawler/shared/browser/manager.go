package browser

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

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

// Install 初始化Playwright（自动安装驱动）
func (m *Manager) Install() error {
	// 先尝试运行 Playwright
	pw, err := playwright.Run()
	if err != nil {
		// 如果失败，检查是否是驱动未安装的错误
		if strings.Contains(err.Error(), "please install the driver") {
			logrus.Warn("检测到 Playwright 驱动未安装，开始自动安装...")

			// 自动安装驱动
			if installErr := m.installPlaywrightDriver(); installErr != nil {
				return fmt.Errorf("自动安装 Playwright 驱动失败: %w", installErr)
			}

			logrus.Info("Playwright 驱动安装成功，重新初始化...")

			// 重新尝试运行
			pw, err = playwright.Run()
			if err != nil {
				return fmt.Errorf("初始化playwright失败: %w", err)
			}
		} else {
			return fmt.Errorf("初始化playwright失败: %w", err)
		}
	}

	m.pw = pw
	logrus.Info("Playwright 初始化成功")
	return nil
}

// installPlaywrightDriver 安装 Playwright 驱动
func (m *Manager) installPlaywrightDriver() error {
	logrus.Info("开始安装 Playwright 驱动（Chromium）...")

	// 获取 go.mod 中的 playwright 版本
	version, err := getPlaywrightVersion()
	if err != nil {
		logrus.Warnf("无法读取 playwright 版本，使用默认安装方式: %v", err)
		// 如果读取失败，使用不指定版本的方式安装
		version = ""
	} else {
		logrus.Infof("检测到 playwright-go 版本: %s", version)
	}

	// 构建安装命令
	var cmd *exec.Cmd
	if version != "" {
		// 使用指定版本安装: go run github.com/playwright-community/playwright-go/cmd/playwright@v0.5700.1 install chromium
		cmd = exec.Command("go", "run",
			fmt.Sprintf("github.com/playwright-community/playwright-go/cmd/playwright@%s", version),
			"install", "chromium")
	} else {
		// 使用当前项目依赖的版本安装
		cmd = exec.Command("go", "run",
			"github.com/playwright-community/playwright-go/cmd/playwright",
			"install", "chromium")
	}

	// 设置输出
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 执行命令
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("执行安装命令失败: %w", err)
	}

	logrus.Info("Playwright 驱动安装完成")
	return nil
}

// getPlaywrightVersion 从 go.mod 读取 playwright-go 的版本
func getPlaywrightVersion() (string, error) {
	// 查找 go.mod 文件
	goModPath, err := findGoMod()
	if err != nil {
		return "", err
	}

	// 打开文件
	file, err := os.Open(goModPath)
	if err != nil {
		return "", fmt.Errorf("打开 go.mod 失败: %w", err)
	}
	defer file.Close()

	// 正则匹配 playwright-go 版本
	re := regexp.MustCompile(`github\.com/playwright-community/playwright-go\s+v(\d+\.\d+\.\d+)`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			return "v" + matches[1], nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取 go.mod 失败: %w", err)
	}

	return "", fmt.Errorf("未找到 playwright-go 依赖")
}

// findGoMod 查找 go.mod 文件路径
func findGoMod() (string, error) {
	// 从当前工作目录开始向上查找
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath, nil
		}

		// 到达根目录
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("未找到 go.mod 文件")
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
	args = AddFingerprintArgs(args, m.config, m.fingerprint)

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

// GetConfig 获取浏览器配置（供子类使用）
func (m *Manager) GetConfig() *BrowserConfig {
	return m.config
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
