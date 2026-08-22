// Package alibaba1688 提供1688页面操作功能
package alibaba1688

import (
	"context"
	"fmt"
	"strings"
	"task-processor/internal/core/logger"
	"time"

	"github.com/mxschmitt/playwright-go"
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
func (po *PageOperator) NavigateToProduct(ctx context.Context, page playwright.Page, url string) error {
	return po.navigateToProduct(ctx, page, url, true)
}

// NavigateToProductWithoutManualCaptcha navigates a public-first attempt
// without waiting for an interactive CAPTCHA operator.
func (po *PageOperator) NavigateToProductWithoutManualCaptcha(ctx context.Context, page playwright.Page, url string) error {
	return po.navigateToProduct(ctx, page, url, false)
}

func (po *PageOperator) navigateToProduct(ctx context.Context, page playwright.Page, url string, allowManualCaptcha bool) error {
	logger.GetGlobalLogger("crawler/alibaba1688").Debugf("导航到1688产品页面: %s", url)
	if err := ctx.Err(); err != nil {
		return err
	}

	// 导航到页面
	if err := po.navigate(ctx, page, url); err != nil {
		return err
	}

	// 处理验证码
	if err := po.handleCaptcha(page, allowManualCaptcha); err != nil {
		return captchaStageError("验证码处理", err)
	}

	// 等待页面就绪
	if err := po.waitForPageReady(ctx, page); err != nil {
		return fmt.Errorf("等待页面就绪失败: %w", err)
	}

	// 再次处理可能出现的验证码
	if err := po.handleCaptcha(page, allowManualCaptcha); err != nil {
		return captchaStageError("二次验证码处理", err)
	}

	// 滚动页面以触发懒加载
	if err := po.ScrollPage(ctx, page); err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Warnf("滚动页面失败: %v", err)
	}

	return nil
}

func captchaStageError(stage string, err error) error {
	if err == nil {
		return nil
	}
	return NewPublicAccessError(PublicAccessFailureChallenge, fmt.Errorf("%s失败: %w", stage, err))
}

// navigate 执行页面导航
func (po *PageOperator) navigate(ctx context.Context, page playwright.Page, url string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(60000),
	})
	if err != nil {
		return fmt.Errorf("导航失败: %w", err)
	}

	return waitForContext(ctx, 3*time.Second)
}

// handleCaptcha 处理验证码
func (po *PageOperator) handleCaptcha(page playwright.Page, allowManual bool) error {
	if allowManual {
		return po.captchaHandler.HandlePageCaptcha(page)
	}
	return po.captchaHandler.HandlePageCaptchaWithoutManual(page)
}

// waitForPageReady 等待页面就绪
func (po *PageOperator) waitForPageReady(ctx context.Context, page playwright.Page) error {
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
	if err := waitForContext(ctx, 3*time.Second); err != nil {
		return err
	}

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
func (po *PageOperator) ScrollPage(ctx context.Context, page playwright.Page) error {
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

		if err := waitForContext(ctx, 500*time.Millisecond); err != nil {
			return err
		}
	}

	// 滚动回顶部
	_, err = page.Evaluate("window.scrollTo(0, 0)")
	if err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Warnf("滚动回顶部失败: %v", err)
	}

	return nil
}

func waitForContext(ctx context.Context, duration time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
