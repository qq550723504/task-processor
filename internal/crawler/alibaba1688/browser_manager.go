// Package alibaba1688 提供1688浏览器管理功能
package alibaba1688

import (
	"fmt"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/shared/browser"

	"github.com/playwright-community/playwright-go"
)

// BrowserManager 浏览器管理器
type BrowserManager struct {
	config *config.Config
}

// NewBrowserManager 创建浏览器管理器
func NewBrowserManager(cfg *config.Config) *BrowserManager {
	return &BrowserManager{
		config: cfg,
	}
}

// CreateBrowser 创建浏览器实例
func (bm *BrowserManager) CreateBrowser() (playwright.Browser, playwright.BrowserContext, playwright.Page, func(), error) {
	// 创建浏览器配置
	browserConfig := &browser.BrowserConfig{
		Headless:       bm.config.Browser.Headless,
		BrowserPath:    bm.config.Browser.BrowserPath,
		ViewportWidth:  bm.config.Browser.ViewportWidth,
		ViewportHeight: bm.config.Browser.ViewportHeight,
		UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}

	// 启动Playwright
	pw, err := playwright.Run()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("启动Playwright失败: %w", err)
	}

	// 启动浏览器
	launchOptions := browser.CreateLaunchOptions(browserConfig, nil)
	browserInstance, err := pw.Chromium.Launch(launchOptions)
	if err != nil {
		pw.Stop()
		return nil, nil, nil, nil, fmt.Errorf("启动浏览器失败: %w", err)
	}

	// 创建上下文
	contextOptions := browser.CreateContextOptions(browserConfig, browserConfig.UserAgent)
	context, err := browserInstance.NewContext(contextOptions)
	if err != nil {
		browserInstance.Close()
		pw.Stop()
		return nil, nil, nil, nil, fmt.Errorf("创建浏览器上下文失败: %w", err)
	}

	// 创建页面
	page, err := context.NewPage()
	if err != nil {
		context.Close()
		browserInstance.Close()
		pw.Stop()
		return nil, nil, nil, nil, fmt.Errorf("创建页面失败: %w", err)
	}

	// 设置超时
	page.SetDefaultTimeout(float64(bm.config.Platforms.Alibaba1688.Timeout * 1000))

	// 返回清理函数
	cleanup := func() {
		context.Close()
		browserInstance.Close()
		pw.Stop()
	}

	return browserInstance, context, page, cleanup, nil
}
