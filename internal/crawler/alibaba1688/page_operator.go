// Package alibaba1688 提供1688页面操作功能
package alibaba1688

import (
	"task-processor/internal/core/logger"
	"fmt"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// PageOperator 页面操作器
type PageOperator struct {
	captchaHandler *CaptchaHandler
}

// NewPageOperator 创建页面操作器
func NewPageOperator() *PageOperator {
	return &PageOperator{
		captchaHandler: NewCaptchaHandler(),
	}
}

// NavigateToProduct 导航到产品页面
func (po *PageOperator) NavigateToProduct(page playwright.Page, url string) error {
	logger.GetGlobalLogger("crawler/alibaba1688").Debugf("导航到1688产品页面: %s", url)

	// 导航到页面
	if err := po.navigate(page, url); err != nil {
		return err
	}

	// 处理验证码
	if err := po.handleCaptcha(page); err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Warnf("验证码处理失败: %v", err)
	}

	// 等待页面就绪
	if err := po.waitForPageReady(page); err != nil {
		return fmt.Errorf("等待页面就绪失败: %w", err)
	}

	// 再次处理可能出现的验证码
	if err := po.handleCaptcha(page); err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Warnf("二次验证码处理失败: %v", err)
	}

	// 滚动页面以触发懒加载
	if err := po.ScrollPage(page); err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Warnf("滚动页面失败: %v", err)
	}

	return nil
}

// navigate 执行页面导航
func (po *PageOperator) navigate(page playwright.Page, url string) error {
	_, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(60000),
	})
	if err != nil {
		return fmt.Errorf("导航失败: %w", err)
	}

	time.Sleep(3 * time.Second)
	return nil
}

// handleCaptcha 处理验证码
func (po *PageOperator) handleCaptcha(page playwright.Page) error {
	return po.captchaHandler.HandlePageCaptcha(page)
}

// waitForPageReady 等待页面就绪
func (po *PageOperator) waitForPageReady(page playwright.Page) error {
	// 等待页面基本元素加载
	selectors := []string{
		"body",
		".main-content, .content, .page-content",
		".product-info, .offer-info, .detail-info",
	}

	for _, selector := range selectors {
		_, err := page.WaitForSelector(selector, playwright.PageWaitForSelectorOptions{
			Timeout: playwright.Float(10000),
		})
		if err != nil {
			logger.GetGlobalLogger("crawler/alibaba1688").Debugf("等待元素 %s 失败: %v", selector, err)
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

	logger.GetGlobalLogger("crawler/alibaba1688").Debugf("页面加载完成，标题: %s", title)
	return nil
}

// ScrollPage 滚动页面以触发懒加载
func (po *PageOperator) ScrollPage(page playwright.Page) error {
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
		if _, scrollErr := page.Evaluate(fmt.Sprintf("window.scrollTo(0, %d)", scrollY)); scrollErr != nil {
			logger.GetGlobalLogger("crawler/alibaba1688").Warnf("滚动到位置 %d 失败: %v", scrollY, scrollErr)
		}

		time.Sleep(500 * time.Millisecond)
	}

	// 滚动回顶部
	_, err = page.Evaluate("window.scrollTo(0, 0)")
	if err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Warnf("滚动回顶部失败: %v", err)
	}

	return nil
}
