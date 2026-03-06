// Package browser 提供Amazon浏览器自动化的邮编输入处理功能
package browser

import (
	"fmt"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

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

	// 处理邮编输入
	if err := zih.handleZipcodeInput(page, zipcode); err != nil {
		return err
	}

	// 提交邮编设置
	if err := zih.submitZipcodeChange(page); err != nil {
		return err
	}

	logrus.Infof("邮编设置操作完成")
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
				logrus.Infof("检测到登录弹窗: %s", selector)
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
			if err := locator.Click(); err != nil {
				// 检查是否是页面关闭导致的错误
				if page.IsClosed() {
					return fmt.Errorf("页面在点击触发元素时被关闭: %w", err)
				}
			} else {
				logrus.Infof("成功点击触发元素: %s", selector)
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
						logrus.Infof("弹窗已出现: %s", dialogSelector)
						waitSuccess = true
						break
					}
				}

				if !waitSuccess {
					logrus.Infof("等待弹窗超时,继续尝试")
				}

				time.Sleep(1 * time.Second) // 额外等待确保弹窗完全加载
				break
			}
		}
	}

	if !triggered {
		logrus.Infof("警告: 未找到任何可点击的触发元素")
		// 在英语页面上，某些地区可能不需要设置邮编
		if CheckIfPriceAvailable(page) {
			logrus.Infof("页面已显示价格信息，可能不需要设置邮编，跳过")
			return nil
		}

		// 调试：打印导航栏区域的所有元素
		if err := DebugNavigationElements(page); err != nil {
			logrus.Infof("调试导航栏元素失败: %v", err)
		}
	}

	return nil
}

// handleZipcodeInput 处理邮编输入（使用策略模式）
func (zih *ZipcodeInputHandler) handleZipcodeInput(page playwright.Page, zipcode string) error {
	if page.IsClosed() {
		return fmt.Errorf("页面在查找邮编输入框前被关闭")
	}

	// 遍历所有策略，找到第一个可以处理的策略
	for _, strategy := range zih.strategies {
		if strategy.CanHandle(page, zipcode) {
			logrus.Infof("使用策略: %s", strategy.GetName())
			return strategy.Handle(page, zipcode)
		}
	}

	return fmt.Errorf("没有找到合适的邮编输入策略")
}

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

	// Done/Save 按钮（次要选择，通常在 Apply 之后出现）
	doneButtonSelectors := []string{
		"div[role='dialog'] button:has-text('Done'):not(:has-text('Cart')):not(:has-text('Buy'))",
		"button:has-text('Done'):not(:has-text('Cart')):not(:has-text('Buy'))",
		"button:has-text('Save'):not(:has-text('Cart')):not(:has-text('Buy'))",
		".save-button",
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
					logrus.Infof("检查Apply按钮文本: '%s' (选择器: %s)", text, selector)

					// 严格排除购物车相关按钮
					if strings.Contains(lowerText, "cart") ||
						strings.Contains(lowerText, "buy") ||
						strings.Contains(lowerText, "add to") ||
						strings.Contains(lowerText, "purchase") {
						logrus.Infof("跳过购物车相关按钮: %s", text)
						continue
					}

					buttonText = text
				}

				applyButton = locator
				selectedSelector = selector
				logrus.Infof("找到Apply按钮: %s (文本: %s)", selector, buttonText)
				break
			}
		}
	}

	if applyButton == nil {
		// 如果没有找到Apply按钮，尝试按回车键
		logrus.Infof("未找到Apply按钮,尝试按回车键")
		if err := page.Keyboard().Press("Enter"); err != nil {
			if page.IsClosed() {
				return fmt.Errorf("页面在按回车键时被关闭: %w", err)
			}
			return fmt.Errorf("按回车键失败: %w", err)
		}
	} else {
		// 点击Apply按钮
		logrus.Infof("准备点击Apply按钮: %s", selectedSelector)
		if err := applyButton.Click(); err != nil {
			if page.IsClosed() {
				return fmt.Errorf("页面在点击Apply按钮时被关闭: %w", err)
			}
			return fmt.Errorf("点击Apply按钮失败: %w", err)
		}
		logrus.Infof("成功点击Apply按钮")
	}

	// 等待Apply按钮处理完成
	time.Sleep(2 * time.Second)

	// 检查页面状态
	if page.IsClosed() {
		return fmt.Errorf("页面在Apply按钮处理后被关闭")
	}

	// 第二步：查找并点击 Done 按钮（如果存在）
	var doneButton playwright.Locator
	for _, selector := range doneButtonSelectors {
		if page.IsClosed() {
			return fmt.Errorf("页面在查找Done按钮过程中被关闭")
		}

		locator := page.Locator(selector).First()
		count, err := locator.Count()
		if err == nil && count > 0 {
			// 检查按钮是否可见
			if isVisible, err := locator.IsVisible(); err == nil && isVisible {
				// 双重检查:确保不是购物车相关按钮
				if text, err := locator.TextContent(); err == nil {
					lowerText := strings.ToLower(strings.TrimSpace(text))
					logrus.Infof("检查Done按钮文本: '%s' (选择器: %s)", text, selector)

					// 严格排除购物车相关按钮
					if strings.Contains(lowerText, "cart") ||
						strings.Contains(lowerText, "buy") ||
						strings.Contains(lowerText, "add to") ||
						strings.Contains(lowerText, "purchase") {
						logrus.Infof("跳过购物车相关按钮: %s", text)
						continue
					}

					// 只接受明确的确认按钮文本
					validTexts := []string{"done", "save", "ok", "confirm"}
					isValid := false
					for _, validText := range validTexts {
						if strings.Contains(lowerText, validText) {
							isValid = true
							break
						}
					}

					if !isValid {
						logrus.Infof("按钮文本不符合Done按钮要求: %s", text)
						continue
					}
				}

				doneButton = locator
				logrus.Infof("找到Done按钮: %s", selector)
				break
			}
		}
	}

	if doneButton != nil {
		// 点击Done按钮
		logrus.Infof("准备点击Done按钮")
		if err := doneButton.Click(); err != nil {
			logrus.Infof("点击Done按钮失败: %v", err)
		} else {
			logrus.Infof("成功点击Done按钮")
			time.Sleep(1 * time.Second)
		}
	} else {
		logrus.Infof("未找到Done按钮，可能已自动关闭")
	}

	// 按ESC键关闭可能残留的弹窗

	// 检查是否有 "Continue Shopping" 按钮需要点击
	HandleContinueShoppingButtonInZipcode(page)

	return nil
}
