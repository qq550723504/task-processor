// Package browser 提供Amazon浏览器自动化的邮编输入处理功能
package browser

import (
	"fmt"
	"regexp"
	"strings"
	"task-processor/internal/core/logger"
	"time"

	"github.com/playwright-community/playwright-go"
)

// truncateString 截断字符串到指定长度
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ZipcodeInputHandler 邮编输入处理器
type ZipcodeInputHandler struct {
	strategies []ZipcodeStrategy // 邮编输入策略列表
}

// NewZipcodeInputHandler 创建邮编输入处理器实例
func NewZipcodeInputHandler() *ZipcodeInputHandler {
	return &ZipcodeInputHandler{
		strategies: []ZipcodeStrategy{
			NewCityDropdownStrategy(),    // 城市下拉框策略（沙特、阿联酋）
			NewJapaneseZipcodeStrategy(), // 日本站分离式输入策略
			NewStandardZipcodeStrategy(), // 标准单一输入框策略（兜底）
		},
	}
}

// SetZipcode 设置邮编
func (zih *ZipcodeInputHandler) SetZipcode(page playwright.Page, zipcode string) error {
	// 检查页面状态
	if page.IsClosed() {
		return fmt.Errorf("页面已关闭，无法设置邮编")
	}

	// 检测登录弹窗
	if err := zih.checkSignInDialog(page); err != nil {
		return err
	}

	// 触发邮编设置界面
	if err := zih.triggerZipcodeInterface(page); err != nil {
		return err
	}

	// 如果弹出了国家选择框，先选国家（Amazon 国家 select 是隐藏元素，用 JS 设置）
	if err := zih.handleCountrySelection(page, zipcode); err != nil {
		return err
	}

	// 处理邮编输入
	if err := zih.handleZipcodeInput(page, zipcode); err != nil {
		return err
	}

	// 提交邮编设置
	if err := zih.submitZipcodeChange(page); err != nil {
		return err
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("邮编设置操作完成")
	return nil
}

// checkSignInDialog 检测是否出现登录弹窗
func (zih *ZipcodeInputHandler) checkSignInDialog(page playwright.Page) error {
	signInSelectors := []string{
		"text=Sign in to update your location",
		"text=Sign in to see your location",
		"text=登录以更新您的位置",
		"text=ログインして配送先を更新",
		"text=Inicia sesión para actualizar tu ubicación", // 西班牙语
		"text=Iniciar sesión para ver tu ubicación",       // 西班牙语
		"h1:has-text('Sign in')",
		"h1:has-text('Iniciar sesión')", // 西班牙语
		"#ap_email",                     // 登录页面的邮箱输入框
	}

	for _, selector := range signInSelectors {
		locator := page.Locator(selector)
		if count, err := locator.Count(); err == nil && count > 0 {
			if isVisible, err := locator.IsVisible(); err == nil && isVisible {
				logger.GetGlobalLogger("crawler/amazon").Infof("检测到登录弹窗: %s", selector)
				return fmt.Errorf("SIGN_IN_REQUIRED: 检测到需要登录才能更新位置，需要重建浏览器实例")
			}
		}
	}
	return nil
}

// triggerZipcodeInterface 触发邮编设置界面
func (zih *ZipcodeInputHandler) triggerZipcodeInterface(page playwright.Page) error {
	triggerSelectors := []string{
		"#nav-global-location-slot",           // 导航栏位置槽（英语页面主要入口）
		"button:has-text('Delivering to')",    // 沙特站点的配送按钮
		"#glow-ingress-block",                 // 配送区块
		"#glow-ingress-line2",                 // Amazon配送地址显示区域
		"#nav-global-location-popover-link",   // 导航栏位置链接
		"a#nav-global-location-popover-link",  // 带标签的导航栏位置链接
		"span#glow-ingress-line2",             // 带标签的配送地址
		"a[href*='address']",                  // 地址链接
		"a[href*='zip-code']",                 // 邮编链接
		".nav-line-2",                         // 导航第二行
		"[data-csa-c-content-id='nav_cs_gb']", // 全球配送
		"#GLUXCountryList",                    // 国家列表
	}

	triggered := false
	for _, selector := range triggerSelectors {
		if page.IsClosed() {
			return fmt.Errorf("页面在触发元素查找过程中被关闭")
		}

		locator := page.Locator(selector).First()
		count, err := locator.Count()
		if err == nil && count > 0 {
			if err := locator.Click(playwright.LocatorClickOptions{
				Timeout: playwright.Float(5000), // 5s 超时，防止 WebSocket 断连时 hang
			}); err != nil {
				// 检查是否是页面关闭导致的错误
				if page.IsClosed() {
					return fmt.Errorf("页面在点击触发元素时被关闭: %w", err)
				}
			} else {
				logger.GetGlobalLogger("crawler/amazon").Infof("成功点击触发元素: %s", selector)
				triggered = true

				// 等待弹窗出现(等待对话框或下拉框)
				dialogSelectors := []string{
					"div[role='dialog']",
					"select#GLUXCountryList",
					"span.a-dropdown-container select",
					"#GLUXZipUpdateInput",
				}

				waitSuccess := false
				for _, dialogSelector := range dialogSelectors {
					if err := page.Locator(dialogSelector).First().WaitFor(playwright.LocatorWaitForOptions{
						State:   playwright.WaitForSelectorStateVisible,
						Timeout: playwright.Float(3000),
					}); err == nil {
						logger.GetGlobalLogger("crawler/amazon").Infof("弹窗已出现: %s", dialogSelector)
						waitSuccess = true
						break
					}
				}

				if !waitSuccess {
					logger.GetGlobalLogger("crawler/amazon").Infof("等待弹窗超时,继续尝试")
				}

				time.Sleep(1 * time.Second) // 额外等待确保弹窗完全加载
				break
			}
		}
	}

	if !triggered {
		logger.GetGlobalLogger("crawler/amazon").Infof("警告: 未找到任何可点击的触发元素")
		// 在英语页面上，某些地区可能不需要设置邮编
		if CheckIfPriceAvailable(page) {
			logger.GetGlobalLogger("crawler/amazon").Infof("页面已显示价格信息，可能不需要设置邮编，跳过")
			return nil
		}

	}

	return nil
}

// handleCountrySelection 处理国家选择框（当 IP 被识别为非目标国家时出现）
// Amazon 的国家 select 是隐藏的原生元素（外层有自定义 UI），IsVisible() 会返回 false，
// 因此只检查元素是否存在，不检查可见性，并通过 JS 直接设置值以绕过 Playwright 的可见性限制。
func (zih *ZipcodeInputHandler) handleCountrySelection(page playwright.Page, zipcode string) error {
	countryList := page.Locator("select#GLUXCountryList")
	count, err := countryList.Count()
	if err != nil || count == 0 {
		return nil
	}

	// 根据邮编格式推断目标国家
	targetCountry := inferCountryFromZipcode(zipcode)
	if targetCountry == "" {
		logger.GetGlobalLogger("crawler/amazon").Infof("检测到国家选择框，但当前邮编 %s 不需要切换国家（可能是 IP 被误识别），跳过国家选择直接尝试填写邮编", zipcode)
		return nil
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("检测到国家选择框，尝试选择国家: %s (邮编: %s)", targetCountry, zipcode)

	// 先通过 JS 获取匹配的 option value（避免对隐藏元素调用 Playwright 的 SelectOption 超时）
	result, err := page.Evaluate(`(targetCountry) => {
		const sel = document.querySelector('select#GLUXCountryList');
		if (!sel) return null;
		const target = targetCountry.toLowerCase();
		for (const opt of sel.options) {
			if (opt.text.toLowerCase().includes(target)) {
				return opt.value;
			}
		}
		return null;
	}`, targetCountry)
	if err != nil {
		return fmt.Errorf("查找国家选项失败: %w", err)
	}
	if result == nil {
		return fmt.Errorf("未找到匹配的国家选项: %s", targetCountry)
	}

	countryValue, ok := result.(string)
	if !ok || countryValue == "" {
		return fmt.Errorf("国家选项值无效: %v", result)
	}

	// 通过 JS 设置 select 值并触发 change 事件（绕过隐藏元素限制）
	_, err = page.Evaluate(`(value) => {
		const sel = document.querySelector('select#GLUXCountryList');
		if (!sel) return false;
		sel.value = value;
		sel.dispatchEvent(new Event('change', { bubbles: true }));
		return true;
	}`, countryValue)
	if err != nil {
		return fmt.Errorf("设置国家值失败: %w", err)
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("成功选择国家: %s (value: %s)", targetCountry, countryValue)
	time.Sleep(1 * time.Second)
	return nil
}

// inferCountryFromZipcode 根据邮编格式推断目标国家名称（用于 GLUXCountryList 选项匹配）
// 注意：只有在 IP 被识别为非目标国家时才会出现国家选择框，加拿大站本身不需要选国家
func inferCountryFromZipcode(zipcode string) string {
	// 英国邮编: SW1A 1AA 等（字母开头，含空格或纯字母数字混合 5-7 位）
	ukRegex := regexp.MustCompile(`(?i)^[A-Z]{1,2}\d{1,2}[A-Z]?\s?\d[A-Z]{2}$`)
	if ukRegex.MatchString(strings.TrimSpace(zipcode)) {
		// 英国站弹窗里的国家下拉是"配送到英国以外"的选项，不需要选国家，直接输入邮编即可
		return ""
	}
	return ""
}

// handleZipcodeInput 处理邮编输入（使用策略模式）
func (zih *ZipcodeInputHandler) handleZipcodeInput(page playwright.Page, zipcode string) error {
	if page.IsClosed() {
		return fmt.Errorf("页面在查找邮编输入框前被关闭")
	}

	// 遍历所有策略，找到第一个可以处理的策略
	for _, strategy := range zih.strategies {
		if strategy.CanHandle(page, zipcode) {
			logger.GetGlobalLogger("crawler/amazon").Infof("使用策略: %s", strategy.GetName())
			return strategy.Handle(page, zipcode)
		}
	}

	return fmt.Errorf("没有找到合适的邮编输入策略")
}

// submitZipcodeChange 提交邮编设置
// submitZipcodeChange 提交邮编设置
func (zih *ZipcodeInputHandler) submitZipcodeChange(page playwright.Page) error {
	if page.IsClosed() {
		return fmt.Errorf("页面在查找Apply按钮前被关闭")
	}

	// 优先查找 Apply 按钮（保存邮编）
	applyButtonSelectors := []string{
		"input[aria-labelledby='GLUXZipUpdate-announce']", // Amazon邮编更新按钮（最常见）
		"#GLUXZipUpdate",           // Amazon主要的设置按钮
		"span#GLUXZipUpdate input", // GLUXZipUpdate内的input
		"input[type='submit'][aria-labelledby='GLUXZipUpdate-announce']", // 带特定aria-labelledby的提交按钮
		"div[role='dialog'] button:has-text('Apply'):not(:has-text('Cart')):not(:has-text('Buy'))",
		"button:has-text('Apply'):not(:has-text('Cart')):not(:has-text('Buy'))",
		"button:text('Apply')", // 英文版的Apply按钮（精确匹配）
		"#zip-code-apply",      // 邮编应用按钮
		"#postal-code-apply",   // 邮编应用按钮
		".apply-button",        // 应用按钮类
	}

	var applyButton playwright.Locator
	var selectedSelector string
	var buttonText string

	// 第一步：查找并点击 Apply 按钮
	for _, selector := range applyButtonSelectors {
		if page.IsClosed() {
			return fmt.Errorf("页面在查找Apply按钮过程中被关闭")
		}

		locator := page.Locator(selector).First()
		count, err := locator.Count()
		if err == nil && count > 0 {
			// 检查按钮是否可见
			if isVisible, err := locator.IsVisible(); err == nil && isVisible {
				// 双重检查:确保不是购物车相关按钮
				if text, err := locator.TextContent(); err == nil {
					lowerText := strings.ToLower(strings.TrimSpace(text))
					logger.GetGlobalLogger("crawler/amazon").Infof("检查Apply按钮文本: '%s' (选择器: %s)", text, selector)

					// 严格排除购物车相关按钮
					if strings.Contains(lowerText, "cart") ||
						strings.Contains(lowerText, "buy") ||
						strings.Contains(lowerText, "add to") ||
						strings.Contains(lowerText, "purchase") {
						logger.GetGlobalLogger("crawler/amazon").Infof("跳过购物车相关按钮: %s", text)
						continue
					}

					buttonText = text
				}

				applyButton = locator
				selectedSelector = selector
				logger.GetGlobalLogger("crawler/amazon").Infof("找到Apply按钮: %s (文本: %s)", selector, buttonText)
				break
			}
		}
	}

	if applyButton == nil {
		// 如果没有找到Apply按钮，尝试按回车键
		logger.GetGlobalLogger("crawler/amazon").Infof("未找到Apply按钮,尝试按回车键")
		if err := page.Keyboard().Press("Enter"); err != nil {
			if page.IsClosed() {
				return fmt.Errorf("页面在按回车键时被关闭: %w", err)
			}
			return fmt.Errorf("按回车键失败: %w", err)
		}
	} else {
		// 点击Apply按钮
		logger.GetGlobalLogger("crawler/amazon").Infof("准备点击Apply按钮: %s", selectedSelector)
		if err := applyButton.Click(playwright.LocatorClickOptions{
			Timeout: playwright.Float(5000), // 5s 超时，防止 WebSocket 断连时 hang
		}); err != nil {
			if page.IsClosed() {
				return fmt.Errorf("页面在点击Apply按钮时被关闭: %w", err)
			}
			return fmt.Errorf("点击Apply按钮失败: %w", err)
		}
		logger.GetGlobalLogger("crawler/amazon").Infof("成功点击Apply按钮")
	}

	// 等待Apply按钮处理完成，等待页面更新
	logger.GetGlobalLogger("crawler/amazon").Infof("等待Apply按钮处理完成，等待页面更新...")
	time.Sleep(2 * time.Second)

	// 检查页面状态
	if page.IsClosed() {
		return fmt.Errorf("页面在Apply按钮处理后被关闭")
	}

	// 尝试关闭可能存在的任何对话框（Cookie对话框、邮编确认对话框等）
	logger.GetGlobalLogger("crawler/amazon").Infof("尝试关闭可能存在的对话框...")
	for i := 0; i < 3; i++ {
		if err := page.Keyboard().Press("Escape"); err == nil {
			logger.GetGlobalLogger("crawler/amazon").Infof("已按 ESC 键 (第%d次)", i+1)
			time.Sleep(500 * time.Millisecond)
		}
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("邮编设置操作完成，等待页面稳定...")
	time.Sleep(2 * time.Second)

	return nil
}
