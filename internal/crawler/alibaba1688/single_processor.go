// Package alibaba1688 提供1688单个产品处理器
package alibaba1688

import (
	"fmt"
	"strings"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688/extractor"
	"task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/crawler/shared/browser"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// SingleProcessor 单个产品处理器
type SingleProcessor struct {
	config         *config.Config
	urlHelper      *URLHelper
	productChecker *ProductChecker
	extractor      *extractor.ProductExtractor
	captchaHandler *CaptchaHandler
}

// NewSingleProcessor 创建新的单个产品处理器
func NewSingleProcessor(cfg *config.Config, urlHelper *URLHelper, productChecker *ProductChecker) *SingleProcessor {
	return &SingleProcessor{
		config:         cfg,
		urlHelper:      urlHelper,
		productChecker: productChecker,
		extractor:      extractor.NewProductExtractor(),
		captchaHandler: NewCaptchaHandler(),
	}
}

// ProcessWithSingleBrowser 使用单个浏览器处理产品
func (sp *SingleProcessor) ProcessWithSingleBrowser(url string, startTime time.Time) (*model.Product1688, error) {
	logrus.Infof("使用单浏览器模式处理1688产品: %s", url)

	// 验证和标准化URL
	normalizedURL, err := sp.urlHelper.ValidateAndNormalizeURL(url)
	if err != nil {
		return nil, fmt.Errorf("URL验证失败: %w", err)
	}

	// 创建浏览器配置
	browserConfig := &browser.BrowserConfig{
		Headless:       sp.config.Browser.Headless,
		BrowserPath:    sp.config.Browser.BrowserPath,
		ViewportWidth:  sp.config.Browser.ViewportWidth,
		ViewportHeight: sp.config.Browser.ViewportHeight,
		UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}

	// 启动Playwright
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("启动Playwright失败: %w", err)
	}
	defer pw.Stop()

	// 启动浏览器
	launchOptions := browser.CreateLaunchOptions(browserConfig, nil)
	browserInstance, err := pw.Chromium.Launch(launchOptions)
	if err != nil {
		return nil, fmt.Errorf("启动浏览器失败: %w", err)
	}
	defer browserInstance.Close()

	// 创建上下文
	contextOptions := browser.CreateContextOptions(browserConfig, browserConfig.UserAgent)
	context, err := browserInstance.NewContext(contextOptions)
	if err != nil {
		return nil, fmt.Errorf("创建浏览器上下文失败: %w", err)
	}
	defer context.Close()

	// 创建页面
	page, err := context.NewPage()
	if err != nil {
		return nil, fmt.Errorf("创建页面失败: %w", err)
	}

	// 设置超时
	page.SetDefaultTimeout(float64(sp.config.Platforms.Alibaba1688.Timeout * 1000)) // 转换为毫秒

	// 导航到产品页面
	if err := sp.navigateToProduct(page, normalizedURL); err != nil {
		return nil, fmt.Errorf("导航到产品页面失败: %w", err)
	}

	// 提取产品信息
	product, err := sp.extractor.ExtractProductFromPage(page, normalizedURL)
	if err != nil {
		return nil, fmt.Errorf("提取产品信息失败: %w", err)
	}

	// 验证产品信息
	if err := sp.productChecker.ValidateProduct(product); err != nil {
		return nil, fmt.Errorf("产品信息验证失败: %w", err)
	}

	duration := time.Since(startTime)
	logrus.Infof("单浏览器模式处理完成: %s, 耗时: %v", product.Title, duration)

	return product, nil
}

// navigateToProduct 导航到产品页面
func (sp *SingleProcessor) navigateToProduct(page playwright.Page, url string) error {
	logrus.Debugf("导航到1688产品页面: %s", url)

	// 导航到页面
	_, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(30000), // 30秒超时
	})
	if err != nil {
		return fmt.Errorf("导航失败: %w", err)
	}

	// 等待页面加载
	if err := sp.waitForPageReady(page); err != nil {
		return fmt.Errorf("等待页面就绪失败: %w", err)
	}

	// 处理验证码和页面交互
	if err := sp.handlePageInteractions(page); err != nil {
		return fmt.Errorf("处理页面交互失败: %w", err)
	}

	return nil
}

// waitForPageReady 等待页面就绪
func (sp *SingleProcessor) waitForPageReady(page playwright.Page) error {
	// 等待页面基本元素加载
	selectors := []string{
		"body",
		".main-content, .content, .page-content",
		".product-info, .offer-info, .detail-info",
	}

	for _, selector := range selectors {
		_, err := page.WaitForSelector(selector, playwright.PageWaitForSelectorOptions{
			Timeout: playwright.Float(10000), // 10秒超时
		})
		if err != nil {
			logrus.Debugf("等待元素 %s 失败: %v", selector, err)
			continue
		}
		break
	}

	// 等待JavaScript执行
	time.Sleep(3 * time.Second)

	// 检查页面是否正常加载
	title, err := page.Title()
	if err != nil {
		return fmt.Errorf("获取页面标题失败: %w", err)
	}

	if title == "" || strings.Contains(strings.ToLower(title), "error") {
		return fmt.Errorf("页面加载异常，标题: %s", title)
	}

	logrus.Debugf("页面加载完成，标题: %s", title)
	return nil
}

// handlePageInteractions 处理页面交互
func (sp *SingleProcessor) handlePageInteractions(page playwright.Page) error {
	// 使用专门的验证码处理器
	if err := sp.captchaHandler.HandlePageCaptcha(page); err != nil {
		return fmt.Errorf("验证码处理失败: %w", err)
	}

	// 滚动页面以触发懒加载
	if err := sp.scrollPage(page); err != nil {
		logrus.Warnf("滚动页面失败: %v", err)
	}

	return nil
}

// scrollPage 滚动页面以触发懒加载
func (sp *SingleProcessor) scrollPage(page playwright.Page) error {
	// 获取页面高度
	pageHeight, err := page.Evaluate("document.body.scrollHeight")
	if err != nil {
		return err
	}

	height, ok := pageHeight.(float64)
	if !ok || height <= 0 {
		return nil
	}

	// 分段滚动
	scrollSteps := 5
	stepHeight := int(height) / scrollSteps

	for i := 1; i <= scrollSteps; i++ {
		scrollY := stepHeight * i
		_, err := page.Evaluate(fmt.Sprintf("window.scrollTo(0, %d)", scrollY))
		if err != nil {
			logrus.Warnf("滚动到位置 %d 失败: %v", scrollY, err)
			continue
		}

		// 等待内容加载
		time.Sleep(500 * time.Millisecond)
	}

	// 滚动回顶部
	_, err = page.Evaluate("window.scrollTo(0, 0)")
	if err != nil {
		logrus.Warnf("滚动回顶部失败: %v", err)
	}

	return nil
}
