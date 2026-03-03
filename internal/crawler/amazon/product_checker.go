// Package amazon 提供Amazon产品检查功能
package amazon

import (
	"fmt"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// ProductChecker 产品检查器
type ProductChecker struct {
	captchaHandler *CaptchaHandler
}

// NewProductChecker 创建产品检查器
func NewProductChecker() *ProductChecker {
	return &ProductChecker{
		captchaHandler: NewCaptchaHandler(),
	}
}

// HandleContinueShoppingButton 处理"Continue shopping"按钮
func (pc *ProductChecker) HandleContinueShoppingButton(page playwright.Page) error {
	// 多种可能的Continue shopping按钮选择器
	continueSelectors := []string{
		"input[name='continue-shopping']",
		"button:has-text('Continue shopping')",
		"a:has-text('Continue shopping')",
		"input[value*='Continue shopping']",
		"button[name='continue-shopping']",
		".a-button:has-text('Continue shopping')",
	}

	for _, selector := range continueSelectors {
		continueButton := page.Locator(selector)
		count, err := continueButton.Count()
		if err != nil {
			continue
		}

		if count > 0 {
			// 检查元素是否可见
			isVisible, err := continueButton.IsVisible()
			if err != nil || !isVisible {
				continue
			}

			logrus.Infof("发现Continue shopping按钮 (选择器: %s)，尝试点击", selector)

			// 尝试点击按钮，设置短超时避免长时间等待
			err = continueButton.Click(playwright.LocatorClickOptions{
				Timeout: playwright.Float(5000), // 5秒超时
			})
			if err != nil {
				logrus.Warnf("点击Continue shopping按钮失败: %v", err)
				continue // 尝试下一个选择器
			}

			// 等待页面跳转
			time.Sleep(2 * time.Second)
			logrus.Info("已成功点击Continue shopping按钮")
			return nil
		}
	}

	// 没有找到Continue shopping按钮，这是正常情况
	return nil
}

// CheckProductAvailability 检查产品可用性
func (pc *ProductChecker) CheckProductAvailability(page playwright.Page) (bool, error) {
	// 检查产品是否可用的各种指标

	// 1. 检查是否有价格信息
	priceSelectors := []string{
		"span.a-price-whole",
		"span.a-price.a-text-price.a-size-medium.apexPriceToPay",
		"span.a-price-range",
		".a-price .a-offscreen",
		"#priceblock_dealprice",
		"#priceblock_ourprice",
	}

	for _, selector := range priceSelectors {
		count, err := page.Locator(selector).Count()
		if err == nil && count > 0 {
			return true, nil
		}
	}

	// 2. 检查是否有"Add to Cart"按钮
	addToCartSelectors := []string{
		"#add-to-cart-button",
		"input[name='submit.add-to-cart']",
		"#buy-now-button",
	}

	for _, selector := range addToCartSelectors {
		count, err := page.Locator(selector).Count()
		if err == nil && count > 0 {
			return true, nil
		}
	}

	// 3. 检查是否显示不可用信息
	unavailableSelectors := []string{
		"#availability span:has-text('Currently unavailable')",
		"#availability span:has-text('Out of stock')",
		"#availability span:has-text('Temporarily out of stock')",
		".a-alert-error",
	}

	for _, selector := range unavailableSelectors {
		count, err := page.Locator(selector).Count()
		if err == nil && count > 0 {
			return false, fmt.Errorf("产品不可用: %s", selector)
		}
	}

	return true, nil
}

// CheckForCaptcha 检查是否遇到验证码
func (pc *ProductChecker) CheckForCaptcha(page playwright.Page) (bool, error) {
	// 首先检查页面是否为空白
	bodyText, err := page.Locator("body").TextContent()
	if err == nil && strings.TrimSpace(bodyText) == "" {
		return false, fmt.Errorf("页面为空白")
	}

	// 检查页面标题，如果为空或异常也不是验证码
	title, err := page.Title()
	if err == nil && (title == "" || strings.Contains(title, "404") || strings.Contains(title, "Error")) {
		return false, fmt.Errorf("页面加载异常: %s", title)
	}

	// 精确的验证码检测选择器
	captchaSelectors := []string{
		"form[action*='validateCaptcha']",
		"#captchacharacters",
		".a-box-inner h4:has-text('Enter the characters you see below')",
		"img[src*='captcha']",
		"#auth-captcha-image",
		"input[name='captchacharacters']",
	}

	// 文本内容检测（更精确）
	captchaTexts := []string{
		"Sorry, we just need to make sure you're not a robot",
		"To continue, please type the characters below",
		"Enter the characters you see below",
		"Type the characters you see in this image",
	}

	// 检查选择器
	for _, selector := range captchaSelectors {
		count, err := page.Locator(selector).Count()
		if err != nil {
			continue
		}

		if count > 0 {
			return true, fmt.Errorf("遇到验证码页面")
		}
	}

	// 检查文本内容
	for _, text := range captchaTexts {
		count, err := page.Locator(fmt.Sprintf("body:has-text('%s')", text)).Count()
		if err != nil {
			continue
		}

		if count > 0 {
			return true, fmt.Errorf("遇到验证码页面")
		}
	}

	return false, nil
}

// CheckForBlocking 检查是否被阻止访问
func (pc *ProductChecker) CheckForBlocking(page playwright.Page) (bool, error) {
	blockingSelectors := []string{
		"body:has-text('Sorry, we just need to make sure you\\'re not a robot')",
		"body:has-text('To discuss automated access to Amazon data please contact')",
		".a-alert-error:has-text('blocked')",
		"#blocked",
	}

	for _, selector := range blockingSelectors {
		count, err := page.Locator(selector).Count()
		if err != nil {
			continue
		}

		if count > 0 {
			return true, fmt.Errorf("访问被阻止")
		}
	}

	return false, nil
}

// CheckPageLoadStatus 检查页面加载状态
func (pc *ProductChecker) CheckPageLoadStatus(page playwright.Page) error {
	// 检查页面标题
	title, err := page.Title()
	if err != nil {
		return fmt.Errorf("无法获取页面标题: %w", err)
	}

	// 检查是否为错误页面
	if title == "" {
		return fmt.Errorf("页面标题为空")
	}

	// 检查页面URL
	url := page.URL()
	if url == "" {
		return fmt.Errorf("页面URL为空")
	}

	// 检查页面内容是否为空
	bodyText, err := page.Locator("body").TextContent()
	if err != nil {
		return fmt.Errorf("无法获取页面内容: %w", err)
	}

	// 如果页面内容为空或过短，可能是加载失败
	if strings.TrimSpace(bodyText) == "" {
		return fmt.Errorf("页面内容为空")
	}

	if len(strings.TrimSpace(bodyText)) < 50 {
		return fmt.Errorf("页面内容过短，可能加载失败")
	}

	// 检查是否为404页面
	if strings.Contains(strings.ToLower(title), "404") ||
		strings.Contains(strings.ToLower(title), "not found") ||
		strings.Contains(strings.ToLower(bodyText), "page not found") {
		return fmt.Errorf("页面不存在(404)")
	}

	// 检查验证码
	hasCaptcha, err := pc.CheckForCaptcha(page)
	if err != nil {
		return err
	}
	if hasCaptcha {
		// 尝试处理验证码
		logrus.Info("检测到验证码，尝试处理")
		captchaErr := pc.captchaHandler.HandleCaptchaWithRetry(page, 2)
		if captchaErr != nil {
			return fmt.Errorf("遇到验证码: %w", captchaErr)
		}
		logrus.Info("验证码处理成功")
	}

	// 检查是否被阻止
	isBlocked, err := pc.CheckForBlocking(page)
	if err != nil {
		return err
	}
	if isBlocked {
		return fmt.Errorf("访问被阻止")
	}

	return nil
}

// WaitForPageReady 等待页面准备就绪
func (pc *ProductChecker) WaitForPageReady(page playwright.Page, timeout time.Duration) error {
	// 使用更宽松的加载策略，避免NetworkIdle导致的无限等待
	err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		Timeout: playwright.Float(float64(timeout.Milliseconds())),
		State:   playwright.LoadStateDomcontentloaded, // 改为DOMContentLoaded，更快更可靠
	})
	if err != nil {
		return fmt.Errorf("等待页面加载超时: %w", err)
	}

	// 额外等待一小段时间让动态内容加载
	time.Sleep(2 * time.Second)

	// 检查页面状态
	return pc.CheckPageLoadStatus(page)
}

// IsProductPage 检查是否为产品页面
func (pc *ProductChecker) IsProductPage(page playwright.Page) bool {
	// 检查产品页面的特征元素
	productSelectors := []string{
		"#productTitle",
		"#feature-bullets",
		"#add-to-cart-button",
		"#availability",
		".a-price",
	}

	foundCount := 0
	for _, selector := range productSelectors {
		count, err := page.Locator(selector).Count()
		if err == nil && count > 0 {
			foundCount++
		}
	}

	// 如果找到至少2个特征元素，认为是产品页面
	return foundCount >= 2
}
