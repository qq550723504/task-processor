// Package browser 提供Amazon浏览器自动化的货币设置功能
package browser

import (
	"fmt"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
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
		logrus.Infof("货币为空，跳过设置")
		return nil
	}

	for attempt := 1; attempt <= cs.maxRetries; attempt++ {
		logrus.Infof("尝试设置货币 (第 %d/%d 次): %s", attempt, cs.maxRetries, expectedCurrency)

		// 检查页面是否仍然有效
		if page.IsClosed() {
			return fmt.Errorf("页面已关闭，无法继续操作")
		}

		// 先验证当前货币是否正确
		currentCurrency, err := cs.getCurrentCurrency(page)
		if err == nil && strings.EqualFold(currentCurrency, expectedCurrency) {
			logrus.Infof("当前货币已经是目标货币: %s，无需设置", expectedCurrency)
			return nil
		}

		logrus.Infof("当前货币: %s, 目标货币: %s，需要设置", currentCurrency, expectedCurrency)

		// 设置货币
		if err := cs.setCurrency(page, expectedCurrency); err != nil {
			logrus.Infof("设置货币失败: %v", err)
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
			logrus.Warnf("货币验证失败 - 期望: %s, 当前: %s", expectedCurrency, currentCurrency)
			if page.IsClosed() {
				return fmt.Errorf("页面已关闭: %w", err)
			}
			if attempt == cs.maxRetries {
				return fmt.Errorf("验证货币失败，已达到最大重试次数")
			}
			time.Sleep(2 * time.Second)
			continue
		}

		logrus.Infof("成功设置并验证货币: %s", expectedCurrency)
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
					logrus.Infof("从价格符号获取到货币: GBP (符号: %s)", text)
					return "GBP", nil
				} else if strings.Contains(text, "$") {
					logrus.Infof("从价格符号获取到货币: USD (符号: %s)", text)
					return "USD", nil
				} else if strings.Contains(text, "€") {
					logrus.Infof("从价格符号获取到货币: EUR (符号: %s)", text)
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
							logrus.Infof("从选择器 %s 获取到货币: %s", selector, part)
							return part, nil
						}
					}
				}
			}
		}
	}

	logrus.Warnf("无法从任何选择器获取货币信息")
	return "", fmt.Errorf("无法获取当前货币")
}

// setCurrency 设置货币
func (cs *CurrencySetter) setCurrency(page playwright.Page, currency string) error {
	// 1. 在当前页面点击页脚的货币选择器
	logrus.Infof("在当前页面设置货币: %s", currency)

	// 页脚货币选择器的可能位置
	currencySelectorLocators := []string{
		"#icp-nav-flyout",                 // 货币选择器容器
		"a.icp-button",                    // 货币按钮
		"span.icp-nav-flag",               // 货币标志
		"#nav-footer-currency",            // 页脚货币区域
		"a[href*='customer-preferences']", // 货币设置链接
	}

	// 尝试点击货币选择器
	clicked := false
	for _, selector := range currencySelectorLocators {
		locator := page.Locator(selector).First()

		// 等待元素出现（最多等待3秒）
		if err := locator.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(3000),
		}); err != nil {
			logrus.Debugf("选择器 %s 未找到或不可见", selector)
			continue
		}

		count, err := locator.Count()
		if err == nil && count > 0 {
			logrus.Infof("找到货币选择器: %s", selector)
			if err := locator.Click(); err == nil {
				logrus.Infof("成功点击货币选择器: %s", selector)
				clicked = true
				time.Sleep(1 * time.Second) // 等待弹窗或菜单出现
				break
			} else {
				logrus.Warnf("点击货币选择器失败: %v", err)
			}
		}
	}

	if !clicked {
		logrus.Warnf("未找到货币选择器，尝试滚动到页脚")
		// 尝试滚动到页脚
		if _, err := page.Evaluate("window.scrollTo(0, document.body.scrollHeight)"); err != nil {
			logrus.Warnf("滚动到页脚失败: %v", err)
		}
		time.Sleep(1 * time.Second)

		// 再次尝试点击
		for _, selector := range currencySelectorLocators {
			locator := page.Locator(selector).First()
			count, err := locator.Count()
			if err == nil && count > 0 {
				if err := locator.Click(); err == nil {
					logrus.Infof("滚动后成功点击货币选择器: %s", selector)
					clicked = true
					time.Sleep(1 * time.Second)
					break
				}
			}
		}
	}

	if !clicked {
		return fmt.Errorf("无法点击货币选择器")
	}

	// 2. 在弹出的菜单或对话框中选择目标货币
	// 货币选项可能的格式:
	// - 链接: a:has-text("USD")
	// - 单选按钮: input[type='radio']:has-text("USD")
	// - 列表项: li:has-text("USD")
	logrus.Infof("查找货币选项: %s", currency)

	// 等待弹窗完全加载
	time.Sleep(500 * time.Millisecond)

	currencyOptionSelectors := []string{
		// 方法1: 查找包含货币代码的链接(最常见,优先)
		fmt.Sprintf("a:has-text('%s')", currency),
		// 方法2: 更宽泛的文本匹配
		fmt.Sprintf("text=/%s/i", currency),
		// 方法3: 查找包含货币代码的单选按钮
		fmt.Sprintf("input[type='radio']:has-text('%s')", currency),
		// 方法4: 查找包含货币代码的列表项
		fmt.Sprintf("li:has-text('%s')", currency),
		// 方法5: 查找包含货币代码的按钮
		fmt.Sprintf("button:has-text('%s')", currency),
	}

	selected := false
	for _, selector := range currencyOptionSelectors {
		locator := page.Locator(selector).First()

		// 恢复等待时间到2.5秒,确保元素加载完成
		if err := locator.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(2500),
		}); err != nil {
			logrus.Debugf("货币选项选择器 %s 未找到或不可见", selector)
			continue
		}

		count, err := locator.Count()
		if err == nil && count > 0 {
			logrus.Infof("找到货币选项: %s (选择器: %s)", currency, selector)
			if err := locator.Click(); err == nil {
				logrus.Infof("成功选择货币: %s", currency)
				selected = true
				time.Sleep(500 * time.Millisecond)
				break
			} else {
				logrus.Warnf("点击货币选项失败: %v", err)
			}
		}
	}

	if !selected {
		logrus.Warnf("未找到货币选项: %s", currency)
		return fmt.Errorf("未找到货币选项: %s", currency)
	}

	// 3. 先关闭可能存在的Cookie对话框
	// Cookie对话框可能会挡住"Save changes"按钮
	logrus.Infof("尝试关闭Cookie对话框...")
	cookieDialogSelectors := []string{
		"button:has-text('Accept')",
		"button:has-text('Accept All')",
		"button:has-text('Accept Cookies')",
		"input[type='submit'][name='accept']",
		"#sp-cc-accept",
		"button[id*='accept']",
		"button[id*='cookie']",
	}

	for _, selector := range cookieDialogSelectors {
		locator := page.Locator(selector).First()
		count, err := locator.Count()
		if err == nil && count > 0 {
			// 检查元素是否可见
			if visible, _ := locator.IsVisible(); visible {
				if err := locator.Click(); err == nil {
					logrus.Infof("成功关闭Cookie对话框: %s", selector)
					time.Sleep(500 * time.Millisecond)
					break
				}
			}
		}
	}

	// 4. 必须点击保存按钮
	// 根据实际测试,选择货币后必须点击"Save changes"按钮才能生效
	logrus.Infof("查找保存按钮...")

	saveButtonSelectors := []string{
		"span.a-button-inner:has-text('Save') >> input", // 嵌套在span中的input (最常见)
		"button:has-text('Save changes')",               // 主要的保存按钮
		"button:has-text('Save')",                       // 备用
		"input[type='submit']:has-text('Save')",         // 提交按钮
		"input[type='submit'][value*='Save']",           // 通过value属性查找
	}

	saved := false
	for _, selector := range saveButtonSelectors {
		locator := page.Locator(selector).First()

		// 等待保存按钮出现（最多等待2秒,减少等待时间）
		if err := locator.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(2000),
		}); err != nil {
			logrus.Debugf("保存按钮选择器 %s 未找到或不可见", selector)
			continue
		}

		count, err := locator.Count()
		if err == nil && count > 0 {
			logrus.Infof("找到保存按钮: %s", selector)
			if err := locator.Click(); err == nil {
				logrus.Infof("成功点击保存按钮: %s", selector)
				saved = true
				time.Sleep(500 * time.Millisecond)
				break
			} else {
				logrus.Warnf("点击保存按钮失败: %v", err)
			}
		}
	}

	if !saved {
		logrus.Warnf("未找到或无法点击保存按钮")
		return fmt.Errorf("未找到保存按钮")
	}

	// 5. 等待页面更新并刷新
	logrus.Infof("等待页面更新...")
	time.Sleep(2 * time.Second) // 减少等待时间

	// 刷新页面以确保货币设置生效
	logrus.Infof("刷新页面以确保货币设置生效...")
	if _, err := page.Reload(playwright.PageReloadOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded, // 改用domcontentloaded,更快
		Timeout:   playwright.Float(15000),                   // 减少超时时间
	}); err != nil {
		logrus.Warnf("刷新页面失败: %v", err)
	} else {
		time.Sleep(1 * time.Second) // 减少等待时间
	}

	// 6. 关闭可能存在的对话框
	for range 2 {
		if err := page.Keyboard().Press("Escape"); err == nil {
			time.Sleep(500 * time.Millisecond)
		}
	}

	return nil
}
