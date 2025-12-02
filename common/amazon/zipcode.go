package amazon

import (
	"fmt"
	"regexp"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// ZipcodeSetter 邮编设置器
type ZipcodeSetter struct {
	browserManager *BrowserManager
	maxRetries     int
}

// NewZipcodeSetter 创建邮编设置器实例
func NewZipcodeSetter(browserManager *BrowserManager) *ZipcodeSetter {
	return &ZipcodeSetter{
		browserManager: browserManager,
		maxRetries:     3,
	}
}

// SetAndVerifyZipcode 设置并验证邮编（基础方法）
// 第二次重试前会刷新页面
func (zs *ZipcodeSetter) SetAndVerifyZipcode(page playwright.Page, zipcode string) error {
	// 如果邮编为空，跳过设置
	if zipcode == "" {
		logrus.Infof("邮编为空，跳过设置")
		return nil
	}

	for attempt := 1; attempt <= zs.maxRetries; attempt++ {
		logrus.Infof("尝试设置邮编 (第 %d/%d 次): %s", attempt, zs.maxRetries, zipcode)

		// 检查页面是否仍然有效
		if page.IsClosed() {
			return fmt.Errorf("页面已关闭，无法继续操作")
		}

		// 如果是第二次尝试，先刷新页面
		if attempt == 2 {
			logrus.Infof("第二次尝试前刷新页面")
			if _, err := page.Reload(); err != nil {
				logrus.Infof("刷新页面失败: %v", err)
				return fmt.Errorf("刷新页面失败: %w", err)
			}

			// 等待页面加载完成
			if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
				State: playwright.LoadStateNetworkidle,
			}); err != nil {
				logrus.Infof("等待页面加载失败: %v", err)
			}

			logrus.Infof("页面已刷新，继续尝试设置邮编")
		}

		// 先验证当前邮编是否正确
		currentZipcode, err := zs.getCurrentZipcode(page)
		if err == nil && currentZipcode == zipcode {
			logrus.Infof("当前邮编已经是目标邮编: %s，无需设置", zipcode)
			return nil
		}

		logrus.Infof("当前邮编不匹配，需要设置邮编。当前: %s, 目标: %s", currentZipcode, zipcode)

		// 设置邮编
		if err := zs.setZipcode(page, zipcode); err != nil {
			logrus.Infof("设置邮编失败: %v", err)
			// 检查是否是页面关闭导致的错误
			if page.IsClosed() {
				return fmt.Errorf("页面已关闭: %w", err)
			}
			if attempt == zs.maxRetries {
				return fmt.Errorf("设置邮编失败，已达到最大重试次数: %w", err)
			}

			// 第一次失败后等待，第二次失败会在下次循环开始时刷新页面
			if attempt == 1 {
				logrus.Infof("等待 2 秒后重试")
				time.Sleep(2 * time.Second)
			}
			continue
		}

		// 验证邮编
		if verified, err := zs.verifyZipcode(page, zipcode); err != nil || !verified {
			logrus.Infof("验证邮编失败: %v", err)
			// 检查是否是页面关闭导致的错误
			if page.IsClosed() {
				return fmt.Errorf("页面已关闭: %w", err)
			}
			if attempt == zs.maxRetries {
				return fmt.Errorf("验证邮编失败，已达到最大重试次数: %w", err)
			}

			// 第一次失败后等待，第二次失败会在下次循环开始时刷新页面
			if attempt == 1 {
				logrus.Infof("等待 2 秒后重试")
				time.Sleep(2 * time.Second)
			}
			continue
		}

		logrus.Infof("成功设置并验证邮编: %s", zipcode)
		return nil
	}

	return fmt.Errorf("设置并验证邮编失败，已达到最大重试次数")
}

// getCurrentZipcode 获取当前邮编
func (zs *ZipcodeSetter) getCurrentZipcode(page playwright.Page) (string, error) {
	// 等待页面稳定
	time.Sleep(500 * time.Millisecond)

	// 查找显示当前邮编的元素（按优先级排序）
	zipDisplaySelectors := []string{
		"#glow-ingress-line2",         // 主要的邮编显示位置（最常见）
		"#glow-ingress-block",         // 地址块
		"#GLUXZipConfirmationMessage", // 确认消息中的邮编
		"#nav-global-location-slot",   // 导航栏位置槽
	}

	logrus.Infof("开始查找当前邮编信息")

	// 优先从主要位置获取邮编
	for _, selector := range zipDisplaySelectors {
		locator := page.Locator(selector)
		count, err := locator.Count()
		if err != nil || count == 0 {
			continue
		}

		// 检查元素是否可见
		isVisible, err := locator.IsVisible()
		if err != nil || !isVisible {
			continue
		}

		text, err := locator.TextContent()
		if err == nil && text != "" {
			// 提取邮编（通常是数字）
			zipcode := extractZipcode(text)
			if zipcode != "" {
				logrus.Infof("成功提取邮编: %s", zipcode)
				return zipcode, nil
			}
		}
	}

	// 如果主要位置没有找到，尝试从导航栏区域查找
	navLocator := page.Locator("#nav-global-location-popover-link, #nav-packard-glow-loc-icon")
	if count, err := navLocator.Count(); err == nil && count > 0 {
		if text, err := navLocator.TextContent(); err == nil && text != "" {
			zipcode := extractZipcode(text)
			if zipcode != "" {
				return zipcode, nil
			}
		}
	}

	return "", fmt.Errorf("未找到当前邮编信息")
}

// setZipcode 设置邮编
func (zs *ZipcodeSetter) setZipcode(page playwright.Page, zipcode string) error {
	// 检查页面状态
	if page.IsClosed() {
		return fmt.Errorf("页面已关闭，无法设置邮编")
	}

	// 检测是否出现 "Sign in to update your location" 弹窗
	signInSelectors := []string{
		"text=Sign in to update your location",
		"text=Sign in to see your location",
		"text=登录以更新您的位置",
		"text=ログインして配送先を更新",
		"h1:has-text('Sign in')",
		"#ap_email", // 登录页面的邮箱输入框
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

	// 尝试点击各种可能触发邮编设置界面的元素（适配英语页面）
	triggerSelectors := []string{
		"#nav-global-location-slot",           // 导航栏位置槽（英语页面主要入口）
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
				time.Sleep(2 * time.Second) // 等待弹窗出现
				break
			}
		}
	}

	if !triggered {
		logrus.Infof("警告: 未找到任何可点击的触发元素")
		// 在英语页面上，某些地区可能不需要设置邮编
		// 尝试检查页面是否已经显示了价格信息
		if zs.checkIfPriceAvailable(page) {
			logrus.Infof("页面已显示价格信息，可能不需要设置邮编，跳过")
			return nil
		}

		// 调试：打印导航栏区域的所有元素
		if err := zs.debugNavigationElements(page); err != nil {
			logrus.Infof("调试导航栏元素失败: %v", err)
		}
	}

	// 查找邮编输入框
	if page.IsClosed() {
		return fmt.Errorf("页面在查找邮编输入框前被关闭")
	}

	// 尝试多种可能的日本邮编输入框选择器
	jpZipSelectors1 := []string{
		"input[name='zipCode1']",
		"input[id='zipCode1']",
		"input[name='zip1']",
		"input[id='zip1']",
		"input[maxlength='3'][type='text']", // 日本站前3位通常限制长度为3
		"input[placeholder*='〒']",
	}

	jpZipSelectors2 := []string{
		"input[name='zipCode2']",
		"input[id='zipCode2']",
		"input[name='zip2']",
		"input[id='zip2']",
		"input[maxlength='4'][type='text']", // 日本站后4位通常限制长度为4
	}

	var jpZipInput1, jpZipInput2 playwright.Locator

	// 查找第一个输入框
	for _, selector := range jpZipSelectors1 {
		locator := page.Locator(selector).First()
		if count, err := locator.Count(); err == nil && count > 0 {
			if isVisible, err := locator.IsVisible(); err == nil && isVisible {
				logrus.Infof("找到日本邮编第一个输入框: %s", selector)
				jpZipInput1 = locator
				break
			}
		}
	}

	// 查找第二个输入框
	for _, selector := range jpZipSelectors2 {
		locator := page.Locator(selector).First()
		if count, err := locator.Count(); err == nil && count > 0 {
			if isVisible, err := locator.IsVisible(); err == nil && isVisible {
				logrus.Infof("找到日本邮编第二个输入框: %s", selector)
				jpZipInput2 = locator
				break
			}
		}
	}

	// 如果两个输入框都找到了，说明是日本站的分离式输入
	if jpZipInput1 != nil && jpZipInput2 != nil {
		// 日本邮编格式: XXX-XXXX，需要分成两部分填写
		parts := regexp.MustCompile(`(\d{3})-?(\d{4})`).FindStringSubmatch(zipcode)
		if len(parts) == 3 {
			part1 := parts[1] // 前3位
			part2 := parts[2] // 后4位

			// 清空并填写第一个输入框（前3位）
			if err := jpZipInput1.Clear(); err != nil {
				logrus.Infof("清空第一个输入框失败: %v", err)
			}
			if err := jpZipInput1.Fill(part1); err != nil {
				if page.IsClosed() {
					return fmt.Errorf("页面在填写日本邮编第一部分时被关闭: %w", err)
				}
				return fmt.Errorf("填写日本邮编第一部分失败: %w", err)
			}

			// 等待一下，确保第一个输入框的值已经设置
			time.Sleep(300 * time.Millisecond)

			// 清空并填写第二个输入框（后4位）
			if err := jpZipInput2.Clear(); err != nil {
				logrus.Infof("清空第二个输入框失败: %v", err)
			}
			if err := jpZipInput2.Fill(part2); err != nil {
				if page.IsClosed() {
					return fmt.Errorf("页面在填写日本邮编第二部分时被关闭: %w", err)
				}
				return fmt.Errorf("填写日本邮编第二部分失败: %w", err)
			}
			logrus.Infof("成功填写日本站分离式邮编")
		} else {
			return fmt.Errorf("日本邮编格式不正确，应为 XXX-XXXX 格式: %s", zipcode)
		}
	} else {
		// 标准单一输入框（适配英语页面）
		zipInputSelectors := []string{
			"#GLUXZipUpdateInput",                // Amazon主要的邮编输入框
			"input[name='zipCode']",              // 邮编输入框（name属性）
			"input[name='postalCode']",           // 邮编输入框（name属性）
			"input#GLUXZipUpdateInput",           // 带标签的邮编输入框
			"input[placeholder*='ZIP']",          // 英语：ZIP code
			"input[placeholder*='zip']",          // 小写zip
			"input[placeholder*='Zip']",          // 首字母大写Zip
			"input[placeholder*='postal']",       // postal code
			"input[placeholder*='Postal']",       // Postal code
			"input[aria-label*='ZIP']",           // 英语：ZIP code
			"input[aria-label*='zip']",           // 小写zip
			"input[aria-label*='Zip']",           // 首字母大写Zip
			"input[aria-label*='postal']",        // postal code
			"input[aria-label*='Postal']",        // Postal code
			"input[type='text'][maxlength='10']", // 美国邮编最大长度
			"input[type='text'][maxlength='5']",  // 美国邮编5位
			"#zip-code",                          // ID为zip-code
			"#postal-code",                       // ID为postal-code
			"input.a-input-text[type='text']",    // Amazon通用输入框
		}

		var zipInput playwright.Locator
		for _, selector := range zipInputSelectors {
			if page.IsClosed() {
				return fmt.Errorf("页面在查找邮编输入框过程中被关闭")
			}

			locator := page.Locator(selector).First()
			count, err := locator.Count()
			if err == nil && count > 0 {
				logrus.Infof("找到邮编输入框: %s", selector)
				zipInput = locator
				break
			}
		}

		if zipInput == nil {
			logrus.Infof("所有邮编输入框选择器都未找到元素")
			return fmt.Errorf("未找到邮编输入框")
		}

		// 填写邮编
		if err := zipInput.Fill(zipcode); err != nil {
			if page.IsClosed() {
				return fmt.Errorf("页面在填写邮编时被关闭: %w", err)
			}
			return fmt.Errorf("填写邮编失败: %w", err)
		}
	}

	// 查找并点击应用按钮
	if page.IsClosed() {
		return fmt.Errorf("页面在查找Apply按钮前被关闭")
	}

	applyButtonSelectors := []string{
		"button:has-text('Apply')",                        // 英文版的Apply按钮（推荐）
		"input[aria-labelledby='GLUXZipUpdate-announce']", // Amazon邮编更新按钮
		"#GLUXZipUpdate",                                  // Amazon主要的设置按钮
		"button:text('Apply')",                            // 英文版的Apply按钮（精确匹配）
		"input[type='submit'][aria-labelledby]",           // 带aria-labelledby的提交按钮
		"span#GLUXZipUpdate input",                        // GLUXZipUpdate内的input
		"button:has-text('Done')",                         // Done按钮
		"button:has-text('Save')",                         // Save按钮
		"input[type='submit']",                            // 通用提交按钮
		"button[type='submit']",                           // 通用提交按钮
		"button.a-button-primary",                         // Amazon主要按钮
		"input.a-button-input",                            // Amazon输入按钮
		"#zip-code-apply",                                 // 邮编应用按钮
		"#postal-code-apply",                              // 邮编应用按钮
		".apply-button",                                   // 应用按钮类
		".save-button",                                    // 保存按钮类
	}

	var applyButton playwright.Locator
	for _, selector := range applyButtonSelectors {
		if page.IsClosed() {
			return fmt.Errorf("页面在查找Apply按钮过程中被关闭")
		}

		locator := page.Locator(selector).First()
		count, err := locator.Count()
		if err == nil && count > 0 {
			// 检查按钮是否可见
			if isVisible, err := locator.IsVisible(); err == nil && isVisible {
				applyButton = locator
				break
			}
		}
	}

	if applyButton == nil {
		// 如果没有找到应用按钮，尝试按回车键
		if err := page.Keyboard().Press("Enter"); err != nil {
			if page.IsClosed() {
				return fmt.Errorf("页面在按回车键时被关闭: %w", err)
			}
			return fmt.Errorf("按回车键失败: %w", err)
		}
	} else {
		// 点击Apply按钮
		if err := applyButton.Click(); err != nil {
			if page.IsClosed() {
				return fmt.Errorf("页面在点击Apply按钮时被关闭: %w", err)
			}
			return fmt.Errorf("点击Apply按钮失败: %w", err)
		}
	}

	// 等待Apply按钮处理完成
	time.Sleep(2 * time.Second)

	// 检查页面状态
	if page.IsClosed() {
		return fmt.Errorf("页面在Apply按钮处理后被关闭")
	}

	if err := page.Keyboard().Press("Escape"); err != nil {
		logrus.Infof("按ESC键失败: %v", err)
	} else {
		logrus.Infof("成功按ESC键")
		// 等待ESC键生效
		time.Sleep(1 * time.Second)
	}

	// 检查是否有 "Continue Shopping" 按钮需要点击（使用公共方法）
	handleContinueShoppingButtonInZipcode(page)

	logrus.Infof("邮编设置操作完成")
	return nil
}

// verifyZipcode 验证邮编是否设置成功
func (zs *ZipcodeSetter) verifyZipcode(page playwright.Page, expectedZipcode string) (bool, error) {
	// 获取当前邮编并验证
	currentZipcode, err := zs.getCurrentZipcode(page)
	if err != nil {
		return false, fmt.Errorf("获取当前邮编失败: %w", err)
	}

	if currentZipcode == expectedZipcode {
		return true, nil
	}

	return false, nil
}

// extractZipcode 从文本中提取邮编
func extractZipcode(text string) string {
	// 支持多种邮编格式：
	// 1. 日本邮编: 100-0001 (3位-4位)
	// 2. 美国邮编: 12345 或 12345-6789 (5位或5位-4位)
	// 3. 英国邮编: SW1A 1AA (字母数字组合)
	// 4. 加拿大邮编: M5H 2N2 (字母数字组合)
	// 5. 巴西邮编: 01310-100 (5位-3位)

	// 优先匹配日本邮编格式 (XXX-XXXX)
	jpZipRegex := regexp.MustCompile(`\b\d{3}-\d{4}\b`)
	jpMatches := jpZipRegex.FindAllString(text, -1)
	if len(jpMatches) > 0 {
		return jpMatches[0]
	}

	// 匹配巴西邮编格式 (XXXXX-XXX)
	brZipRegex := regexp.MustCompile(`\b\d{5}-\d{3}\b`)
	brMatches := brZipRegex.FindAllString(text, -1)
	if len(brMatches) > 0 {
		return brMatches[0]
	}

	// 匹配美国邮编格式 (XXXXX 或 XXXXX-XXXX)
	usZipRegex := regexp.MustCompile(`\b\d{5}(?:-\d{4})?\b`)
	usMatches := usZipRegex.FindAllString(text, -1)
	if len(usMatches) > 0 {
		return usMatches[0]
	}

	// 匹配英国邮编格式 (例如: SW1A 1AA, EC1A 1BB)
	ukZipRegex := regexp.MustCompile(`\b[A-Z]{1,2}\d{1,2}[A-Z]?\s?\d[A-Z]{2}\b`)
	ukMatches := ukZipRegex.FindAllString(text, -1)
	if len(ukMatches) > 0 {
		return ukMatches[0]
	}

	// 匹配加拿大邮编格式 (例如: M5H 2N2, K1A 0B1)
	caZipRegex := regexp.MustCompile(`\b[A-Z]\d[A-Z]\s?\d[A-Z]\d\b`)
	caMatches := caZipRegex.FindAllString(text, -1)
	if len(caMatches) > 0 {
		return caMatches[0]
	}

	// 如果没有找到任何格式，尝试查找纯数字邮编（德国、法国、意大利等）
	simpleZipRegex := regexp.MustCompile(`\b\d{4,6}\b`)
	simpleMatches := simpleZipRegex.FindAllString(text, -1)
	if len(simpleMatches) > 0 {
		return simpleMatches[0]
	}

	return ""
}

// debugNavigationElements 调试导航栏元素（帮助找到正确的选择器）
func (zs *ZipcodeSetter) debugNavigationElements(page playwright.Page) error {
	logrus.Infof("=== 开始调试导航栏元素 ===")

	// 查找所有导航栏相关的元素
	navSelectors := []string{
		"#nav-global-location-slot",
		"#nav-packard-glow-loc-icon",
		"#glow-ingress-block",
		"#nav-main",
		"[id*='nav']",
		"[id*='location']",
		"[id*='glow']",
	}

	for _, selector := range navSelectors {
		locator := page.Locator(selector)
		count, err := locator.Count()
		if err == nil && count > 0 {
			logrus.Infof("找到元素: %s (数量: %d)", selector, count)
			// 尝试获取第一个元素的文本
			if text, err := locator.First().TextContent(); err == nil && text != "" {
				logrus.Infof("  文本内容: %s", text)
			}
			// 尝试获取元素的HTML
			if html, err := locator.First().InnerHTML(); err == nil && len(html) < 200 {
				logrus.Infof("  HTML: %s", html)
			}
		}
	}

	logrus.Infof("=== 调试完成 ===")
	return nil
}

// checkIfPriceAvailable 检查页面是否已经显示价格信息
func (zs *ZipcodeSetter) checkIfPriceAvailable(page playwright.Page) bool {
	priceSelectors := []string{
		".a-price-whole",
		".a-price",
		"#priceblock_ourprice",
		"#priceblock_dealprice",
		"span.a-price span.a-offscreen",
	}

	for _, selector := range priceSelectors {
		locator := page.Locator(selector)
		if count, err := locator.Count(); err == nil && count > 0 {
			if isVisible, err := locator.First().IsVisible(); err == nil && isVisible {
				logrus.Infof("检测到价格元素: %s", selector)
				return true
			}
		}
	}

	return false
}

// handleContinueShoppingButtonInZipcode 处理"继续购物"按钮（在zipcode设置流程中使用）
func handleContinueShoppingButtonInZipcode(page playwright.Page) {
	if page.IsClosed() {
		return
	}

	continueShoppingSelectors := getContinueShoppingSelectors()

	for _, selector := range continueShoppingSelectors {
		if page.IsClosed() {
			break
		}

		locator := page.Locator(selector)
		if count, err := locator.Count(); err == nil && count > 0 {
			if isVisible, err := locator.IsVisible(); err == nil && isVisible {
				logrus.Infof("发现 Continue Shopping 按钮，尝试点击")
				if err := locator.Click(); err != nil {
					logrus.Infof("点击 Continue Shopping 按钮失败: %v", err)
				} else {
					logrus.Infof("成功点击 Continue Shopping 按钮")
					time.Sleep(1 * time.Second)
					break
				}
			}
		}
	}
}
