// Package browser 提供Amazon浏览器自动化的货币设置功能
package browser

import (
	"fmt"
	"strings"
	"task-processor/internal/core/logger"
	"time"

	"github.com/playwright-community/playwright-go"
)

// CurrencySetter 货币设置器
type CurrencySetter struct {
	browserManager *BrowserManager
	maxRetries     int
}

// NewCurrencySetter 创建货币设置器实例
func NewCurrencySetter(browserManager *BrowserManager) *CurrencySetter {
	return &CurrencySetter{
		browserManager: browserManager,
		maxRetries:     3,
	}
}

// SetAndVerifyCurrency 设置并验证货币
func (cs *CurrencySetter) SetAndVerifyCurrency(page playwright.Page, expectedCurrency string) error {
	// 如果货币为空，跳过设置
	if expectedCurrency == "" {
		logger.GetGlobalLogger("crawler/amazon").Infof("货币为空，跳过设置")
		return nil
	}

	for attempt := 1; attempt <= cs.maxRetries; attempt++ {
		logger.GetGlobalLogger("crawler/amazon").Infof("尝试设置货币 (第 %d/%d 次): %s", attempt, cs.maxRetries, expectedCurrency)

		// 检查页面是否仍然有效
		if page.IsClosed() {
			return fmt.Errorf("页面已关闭，无法继续操作")
		}

		// 先验证当前货币是否正确
		currentCurrency, err := cs.getCurrentCurrency(page)
		if err == nil && strings.EqualFold(currentCurrency, expectedCurrency) {
			logger.GetGlobalLogger("crawler/amazon").Infof("当前货币已经是目标货币: %s，无需设置", expectedCurrency)
			return nil
		}

		logger.GetGlobalLogger("crawler/amazon").Infof("当前货币: %s, 目标货币: %s，需要设置", currentCurrency, expectedCurrency)

		// 设置货币
		err = cs.setCurrency(page, expectedCurrency)
		if err != nil {
			logger.GetGlobalLogger("crawler/amazon").Infof("设置货币失败: %v", err)
			if page.IsClosed() {
				return fmt.Errorf("页面已关闭: %w", err)
			}
			if attempt == cs.maxRetries {
				return fmt.Errorf("设置货币失败，已达到最大重试次数: %w", err)
			}
			time.Sleep(2 * time.Second)
			continue
		}

		// 验证货币
		currentCurrency, err = cs.getCurrentCurrency(page)
		if err != nil || !strings.EqualFold(currentCurrency, expectedCurrency) {
			logger.GetGlobalLogger("crawler/amazon").Warnf("货币验证失败 - 期望: %s, 当前: %s", expectedCurrency, currentCurrency)
			if page.IsClosed() {
				return fmt.Errorf("页面已关闭: %w", err)
			}
			if attempt == cs.maxRetries {
				return fmt.Errorf("验证货币失败，已达到最大重试次数")
			}
			time.Sleep(2 * time.Second)
			continue
		}

		logger.GetGlobalLogger("crawler/amazon").Infof("成功设置并验证货币: %s", expectedCurrency)
		return nil
	}

	return fmt.Errorf("设置并验证货币失败，已达到最大重试次数")
}

// getCurrentCurrency 获取当前页面的货币设置
func (cs *CurrencySetter) getCurrentCurrency(page playwright.Page) (string, error) {
	// 方法1: 从价格元素中提取货币符号(最可靠且最快的方法)
	priceSelectors := []string{
		".a-price-symbol",       // 价格符号
		"span.a-price-symbol",   // 价格符号
		".a-price .a-offscreen", // 屏幕外价格文本
	}

	for _, selector := range priceSelectors {
		locator := page.Locator(selector).First()
		count, _ := locator.Count()
		if count > 0 {
			text, err := locator.TextContent()
			if err == nil && text != "" {
				text = strings.TrimSpace(text)
				// 根据货币符号判断货币类型
				if strings.Contains(text, "£") {
					logger.GetGlobalLogger("crawler/amazon").Infof("从价格符号获取到货币: GBP (符号: %s)", text)
					return "GBP", nil
				} else if strings.Contains(text, "HK$") {
					logger.GetGlobalLogger("crawler/amazon").Infof("从价格符号获取到货币: HKD (符号: %s)", text)
					return "HKD", nil
				} else if strings.Contains(text, "S$") {
					logger.GetGlobalLogger("crawler/amazon").Infof("从价格符号获取到货币: SGD (符号: %s)", text)
					return "SGD", nil
				} else if strings.Contains(text, "C$") || strings.Contains(text, "CA$") {
					logger.GetGlobalLogger("crawler/amazon").Infof("从价格符号获取到货币: CAD (符号: %s)", text)
					return "CAD", nil
				} else if strings.Contains(text, "A$") || strings.Contains(text, "AU$") {
					logger.GetGlobalLogger("crawler/amazon").Infof("从价格符号获取到货币: AUD (符号: %s)", text)
					return "AUD", nil
				} else if strings.Contains(text, "¥") {
					logger.GetGlobalLogger("crawler/amazon").Infof("从价格符号获取到货币: JPY (符号: %s)", text)
					return "JPY", nil
				} else if strings.Contains(text, "$") {
					logger.GetGlobalLogger("crawler/amazon").Infof("从价格符号获取到货币: USD (符号: %s)", text)
					return "USD", nil
				} else if strings.Contains(text, "€") {
					logger.GetGlobalLogger("crawler/amazon").Infof("从价格符号获取到货币: EUR (符号: %s)", text)
					return "EUR", nil
				}
			}
		}
	}

	// 方法2: 从页脚的货币选择器中提取
	currencySelectors := []string{
		"span.icp-nav-currency",             // 货币代码
		"#icp-nav-flyout .icp-nav-currency", // 货币显示区域
		".icp-nav-currency",                 // 通用货币类
	}

	for _, selector := range currencySelectors {
		locator := page.Locator(selector).First()

		// 减少等待时间到1秒
		if err := locator.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateAttached,
			Timeout: playwright.Float(1000),
		}); err != nil {
			continue
		}

		count, err := locator.Count()
		if err == nil && count > 0 {
			text, err := locator.TextContent()
			if err == nil {
				text = strings.TrimSpace(text)
				if text != "" {
					// 提取货币代码（通常是3个字母）
					parts := strings.Fields(text)
					for _, part := range parts {
						if len(part) == 3 && strings.ToUpper(part) == part {
							logger.GetGlobalLogger("crawler/amazon").Infof("从选择器 %s 获取到货币: %s", selector, part)
							return part, nil
						}
					}
				}
			}
		}
	}

	logger.GetGlobalLogger("crawler/amazon").Warnf("无法从任何选择器获取货币信息")
	return "", fmt.Errorf("无法获取当前货币")
}

// setCurrency 设置货币
func (cs *CurrencySetter) setCurrency(page playwright.Page, currency string) error {
	logger.GetGlobalLogger("crawler/amazon").Infof("使用导航栏货币选择器设置货币: %s", currency)
	return cs.setCurrencyViaNavBar(page, currency)
}

// setCurrencyViaNavBar 通过导航栏货币选择器设置货币
func (cs *CurrencySetter) setCurrencyViaNavBar(page playwright.Page, currency string) error {
	logger.GetGlobalLogger("crawler/amazon").Infof("通过导航栏设置货币: %s", currency)

	// 1. 点击导航栏的语言/货币按钮
	navButtonSelectors := []string{
		"#icp-nav-flyout button.nav-flyout-button",
		"#icp-nav-flyout .nav-flyout-button",
		"button[name='Expand to Change Language or Country']", // 正确的货币选择器
		"button:has-text('Expand to Change Language or Country')",
		"button[aria-label*='Change Language or Country']",
		"button[aria-label*='Language or Country']",
		"button[aria-label*='言語または国を変更']",
		"button[aria-label*='Country']",
	}

	clicked := false
	for _, selector := range navButtonSelectors {
		locator := page.Locator(selector).First()

		if err := locator.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(3000),
		}); err != nil {
			logger.GetGlobalLogger("crawler/amazon").Debugf("导航栏按钮 %s 未找到", selector)
			continue
		}

		count, err := locator.Count()
		if err == nil && count > 0 {
			logger.GetGlobalLogger("crawler/amazon").Infof("找到导航栏货币按钮: %s", selector)
			if err := locator.Click(); err == nil {
				logger.GetGlobalLogger("crawler/amazon").Infof("成功点击导航栏货币按钮")
				clicked = true
				time.Sleep(1 * time.Second) // 等待弹窗出现
				break
			} else {
				logger.GetGlobalLogger("crawler/amazon").Warnf("点击导航栏按钮失败: %v", err)
			}
		}
	}

	if !clicked {
		return fmt.Errorf("无法找到或点击导航栏货币按钮")
	}

	// 2. 等待货币选择弹窗出现 - 直接等待货币链接出现
	time.Sleep(1 * time.Second) // 给弹窗一点时间完全加载

	// 3. 点击目标货币选项
	currencyLinkSelectors := []string{
		fmt.Sprintf("a[href*='switch-currency=%s']", currency),
		fmt.Sprintf("a:has-text('%s - %s')", getCurrencySymbol(currency), currency),
		fmt.Sprintf("a:has-text('%s')", currency),
	}

	currencyClicked := false
	for _, selector := range currencyLinkSelectors {
		locator := page.Locator(selector).First()

		if err := locator.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(2000),
		}); err != nil {
			logger.GetGlobalLogger("crawler/amazon").Debugf("货币选项 %s 未找到", selector)
			continue
		}

		count, err := locator.Count()
		if err == nil && count > 0 {
			logger.GetGlobalLogger("crawler/amazon").Infof("找到货币选项: %s", selector)
			if err := locator.Click(); err == nil {
				logger.GetGlobalLogger("crawler/amazon").Infof("成功点击货币选项: %s", currency)
				currencyClicked = true
				break
			} else {
				logger.GetGlobalLogger("crawler/amazon").Warnf("点击货币选项失败: %v", err)
			}
		}
	}

	if !currencyClicked {
		return fmt.Errorf("无法找到或点击货币选项: %s", currency)
	}

	// 4. 等待页面刷新完成
	logger.GetGlobalLogger("crawler/amazon").Infof("等待页面刷新...")
	time.Sleep(2 * time.Second)

	// 5. 验证URL是否包含货币参数
	currentURL := page.URL()
	expectedParam := fmt.Sprintf("currency=%s", currency)
	if strings.Contains(currentURL, expectedParam) {
		logger.GetGlobalLogger("crawler/amazon").Infof("货币设置成功,URL已更新: %s", currentURL)
		return nil
	}

	logger.GetGlobalLogger("crawler/amazon").Warnf("URL未包含预期的货币参数,当前URL: %s", currentURL)
	return nil // 即使URL未更新也返回成功,因为货币可能已经生效
}

// getCurrencySymbol 获取货币符号
func getCurrencySymbol(currency string) string {
	symbols := map[string]string{
		"GBP": "£",
		"USD": "US$",
		"EUR": "€",
		"CNY": "CN¥",
	}
	if symbol, ok := symbols[currency]; ok {
		return symbol
	}
	return currency
}
