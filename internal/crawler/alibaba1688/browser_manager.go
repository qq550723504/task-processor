// Package alibaba1688 提供1688浏览器管理功能
package alibaba1688

import (
	"fmt"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	sharedbrowser "task-processor/internal/crawler/shared/browser"

	"github.com/playwright-community/playwright-go"
)

// BrowserManager 1688专用的浏览器管理器，继承shared的功能
type BrowserManager struct {
	*sharedbrowser.Manager
	config *config.Config
}

// NewBrowserManager 创建1688浏览器管理器
func NewBrowserManager(cfg *config.Config) *BrowserManager {
	// 创建浏览器配置
	browserConfig := &sharedbrowser.BrowserConfig{
		Headless:       cfg.Browser.Headless,
		BrowserPath:    cfg.Browser.BrowserPath,
		ViewportWidth:  cfg.Browser.ViewportWidth,
		ViewportHeight: cfg.Browser.ViewportHeight,
		UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}

	logger.GetGlobalLogger("crawler/alibaba1688").Info("创建1688浏览器管理器，使用共享浏览器组件")

	return &BrowserManager{
		Manager: sharedbrowser.NewManager(browserConfig),
		config:  cfg,
	}
}

// CreateBrowser 创建浏览器实例（保持向后兼容的API）
func (bm *BrowserManager) CreateBrowser() (playwright.Browser, playwright.BrowserContext, playwright.Page, func(), error) {
	// 初始化Playwright
	if err := bm.Manager.Install(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("初始化Playwright失败: %w", err)
	}

	// 启动浏览器
	if err := bm.Manager.Launch(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("启动浏览器失败: %w", err)
	}

	// 获取上下文
	context := bm.GetContext()
	if context == nil {
		return nil, nil, nil, nil, fmt.Errorf("浏览器上下文未初始化")
	}

	// 创建页面
	page, err := bm.Manager.NewPage()
	if err != nil {
		bm.Manager.Close()
		return nil, nil, nil, nil, fmt.Errorf("创建页面失败: %w", err)
	}

	// 设置1688特定的超时
	timeout := float64(bm.config.Platforms.Alibaba1688.Timeout * 1000)
	page.SetDefaultTimeout(timeout)
	logger.GetGlobalLogger("crawler/alibaba1688").Infof("设置1688页面超时: %.0fms", timeout)

	// 返回清理函数
	cleanup := func() {
		bm.Manager.Close()
	}

	// 注意：这里返回nil作为browser，因为使用持久化上下文时没有单独的browser对象
	return nil, context, page, cleanup, nil
}
