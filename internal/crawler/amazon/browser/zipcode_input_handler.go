// Package browser 提供Amazon浏览器自动化的邮编输入处理功能
package browser

import (
	"fmt"
	"net/url"
	"strings"
	"task-processor/internal/core/logger"
	"time"

	"github.com/mxschmitt/playwright-go"
)

var zipcodeInterfaceReadySelectors = []string{
	"#GLUXZipUpdateInput",
	"select#GLUXCountryList",
	"#GLUXZipUpdate",
}

var zipcodeSuccessConfirmationSelectors = []string{
	"input#GLUXConfirmClose",
	"input[aria-labelledby='GLUXConfirmClose-announce']",
	"button:has-text('Continue')",
	"text=Continue",
}

const (
	zipcodeAddressSyncWaitAfterConfirm = 1500 * time.Millisecond
	zipcodeAddressSyncWaitDefault      = 4 * time.Second
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
	targetURL  string
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

func (zih *ZipcodeInputHandler) SetTargetURL(targetURL string) {
	zih.targetURL = targetURL
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

	// 先处理跨站点访问提示，例如 "Visiting from Singapore"
	DismissRegionalPrompt(page, zih.targetURL)

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
	if zih.isZipcodeInterfaceReady(page) {
		logger.GetGlobalLogger("crawler/amazon").Infof("邮编设置界面已就绪，跳过重复触发")
		return nil
	}

	triggerSelectorGroups := [][]string{
		{
			"#nav-global-location-slot",           // 导航栏位置槽（英语页面主要入口）
			"#glow-ingress-block",                 // 配送区块
			"#glow-ingress-line2",                 // Amazon配送地址显示区域
			"#nav-global-location-popover-link",   // 导航栏位置链接
			"a#nav-global-location-popover-link",  // 带标签的导航栏位置链接
			"span#glow-ingress-line2",             // 带标签的配送地址
			".nav-line-2",                         // 导航第二行
			"[data-csa-c-content-id='nav_cs_gb']", // 全球配送
		},
		{
			"button:has-text('Delivering to')", // 沙特站点的配送按钮
		},
		{
			"a[href*='address']",  // 地址链接
			"a[href*='zip-code']", // 邮编链接
		},
	}

	triggered := false
	for _, group := range triggerSelectorGroups {
		if page.IsClosed() {
			return fmt.Errorf("页面在触发元素查找过程中被关闭")
		}

		selector := zih.findFirstVisibleSelector(page, group)
		if selector == "" {
			continue
		}

		if err := zih.clickTriggerSelector(page, selector); err != nil {
			// 检查是否是页面关闭导致的错误
			if page.IsClosed() {
				return fmt.Errorf("页面在点击触发元素时被关闭: %w", err)
			}
			logger.GetGlobalLogger("crawler/amazon").Infof("点击触发元素失败，继续尝试下一组入口: selector=%s err=%v", selector, err)
			continue
		}

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
				Timeout: playwright.Float(1500),
			}); err == nil {
				logger.GetGlobalLogger("crawler/amazon").Infof("弹窗已出现: %s", dialogSelector)
				waitSuccess = true
				break
			}
		}

		if !waitSuccess {
			logger.GetGlobalLogger("crawler/amazon").Infof("等待弹窗超时,继续尝试")
		}
		zih.waitForZipcodeInterfaceReady(page)
		break
	}

	if !triggered {
		if DismissRegionalPrompt(page, zih.targetURL) {
			logger.GetGlobalLogger("crawler/amazon").Infof("已处理跨站点访问提示弹窗，重新尝试查找地址触发元素")
			return zih.triggerZipcodeInterface(page)
		}

		logger.GetGlobalLogger("crawler/amazon").Infof("警告: 未找到任何可点击的触发元素")
		// 页面已经有价格，但地址弹层/输入框没起来时，继续尝试输入通常只会空耗时间。
		// 让外层在下一轮直接刷新页面，再重新对齐配送上下文。
		if shouldRefreshAfterPriceVisibleTriggerFailure(CheckIfPriceAvailable(page), zih.isZipcodeInterfaceReady(page)) {
			logger.GetGlobalLogger("crawler/amazon").Infof("页面已有价格但邮编界面不可用，返回刷新信号给上层重试")
			return fmt.Errorf("ZIPCODE_INTERFACE_REFRESH_REQUIRED: 页面已有价格但邮编界面未就绪")
		}

		// 在英语页面上，某些地区可能不需要设置邮编
		if CheckIfPriceAvailable(page) {
			logger.GetGlobalLogger("crawler/amazon").Infof("页面已显示价格信息，可能不需要设置邮编，跳过")
			return nil
		}

	}

	return nil
}

func shouldRefreshAfterPriceVisibleTriggerFailure(priceAvailable bool, interfaceReady bool) bool {
	return priceAvailable && !interfaceReady
}

func (zih *ZipcodeInputHandler) findFirstVisibleSelector(page playwright.Page, selectors []string) string {
	if page == nil || page.IsClosed() {
		return ""
	}

	for _, selector := range selectors {
		locator := page.Locator(selector).First()
		count, err := locator.Count()
		if err != nil || count == 0 {
			continue
		}
		if visible, err := locator.IsVisible(); err == nil && visible {
			return selector
		}
	}

	return ""
}

func (zih *ZipcodeInputHandler) clickTriggerSelector(page playwright.Page, selector string) error {
	if page == nil || page.IsClosed() {
		return fmt.Errorf("页面已关闭，无法点击触发元素")
	}

	locator := page.Locator(selector).First()
	if err := locator.Click(playwright.LocatorClickOptions{
		Timeout: playwright.Float(1800),
	}); err == nil {
		return nil
	}

	_, err := page.Evaluate(`(sel) => {
		const el = document.querySelector(sel);
		if (!el) return false;
		el.click();
		return true;
	}`, selector)
	if err != nil {
		return err
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

	// 优先根据目标 Amazon 站点确定配送国家/货币环境，再用邮编做补充兜底。
	targetCountry := inferDeliveryCountry(zih.targetURL, zipcode)
	if targetCountry == "" {
		logger.GetGlobalLogger("crawler/amazon").Infof("检测到国家选择框，但无法从目标URL=%s 或邮编=%s 推断目标配送国家，跳过国家选择直接尝试填写邮编", zih.targetURL, zipcode)
		return nil
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("检测到国家选择框，尝试选择目标配送国家: %s (目标URL: %s, 邮编: %s)", targetCountry, zih.targetURL, zipcode)

	countryQueries := buildCountrySelectionQueries(targetCountry)
	if len(countryQueries) == 0 {
		logger.GetGlobalLogger("crawler/amazon").Infof("目标配送国家 %s 不适合通过 GLUXCountryList 切换，跳过国家选择，继续尝试填写邮编", targetCountry)
		return nil
	}

	// 先通过 JS 获取最佳匹配的 option value（避免对隐藏元素调用 Playwright 的 SelectOption 超时）
	result, err := page.Evaluate(`(queries) => {
		const sel = document.querySelector('select#GLUXCountryList');
		if (!sel) return null;
		const normalize = (text) => (text || '')
			.toLowerCase()
			.normalize('NFKC')
			.replace(/[\s\-_()（）]/g, '');
		const options = Array.from(sel.options).map((opt) => ({
			value: opt.value,
			text: (opt.text || '').trim(),
			normalized: normalize(opt.text || ''),
			valueNormalized: normalize(opt.value || ''),
		}));

		for (const query of queries) {
			const normalizedQuery = normalize(query);
			if (!normalizedQuery) continue;

			const exact = options.find((opt) => opt.normalized === normalizedQuery);
			if (exact) {
				return { value: exact.value, text: exact.text };
			}

			const byValue = options.find((opt) => opt.valueNormalized === normalizedQuery);
			if (byValue) {
				return { value: byValue.value, text: byValue.text };
			}

			if (normalizedQuery.length >= 4) {
				const partial = options.find((opt) => opt.normalized.includes(normalizedQuery));
				if (partial) {
					return { value: partial.value, text: partial.text };
				}
			}
		}

		return {
			options: options.map((opt) => ({ value: opt.value, text: opt.text })),
		};
	}`, countryQueries)
	if err != nil {
		return fmt.Errorf("查找国家选项失败: %w", err)
	}
	if result == nil {
		logger.GetGlobalLogger("crawler/amazon").Warnf("国家选择框存在，但未拿到任何选项结果，跳过国家切换并继续尝试填写邮编: %s", targetCountry)
		return nil
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		logger.GetGlobalLogger("crawler/amazon").Warnf("国家选项结果格式无效，跳过国家切换并继续尝试填写邮编: %v", result)
		return nil
	}

	countryValue, _ := resultMap["value"].(string)
	matchedText, _ := resultMap["text"].(string)
	if countryValue == "" {
		if options, ok := resultMap["options"]; ok {
			logger.GetGlobalLogger("crawler/amazon").Warnf("未找到匹配的国家选项: %s，可用选项=%v，跳过国家切换并继续尝试填写邮编", targetCountry, options)
		} else {
			logger.GetGlobalLogger("crawler/amazon").Warnf("未找到匹配的国家选项: %s，跳过国家切换并继续尝试填写邮编", targetCountry)
		}
		return nil
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

	logger.GetGlobalLogger("crawler/amazon").Infof("成功选择国家: %s (命中文案: %s, value: %s)", targetCountry, matchedText, countryValue)
	zih.waitForZipcodeInterfaceReady(page)
	return nil
}

func buildCountrySelectionQueries(targetCountry string) []string {
	switch strings.ToLower(strings.TrimSpace(targetCountry)) {
	case "united states":
		return nil
	case "united kingdom":
		return nil
	case "japan":
		return []string{"Japan", "JP", "日本"}
	case "canada":
		return []string{"Canada", "CA", "加拿大"}
	default:
		return []string{targetCountry}
	}
}

func inferDeliveryCountry(targetURL string, zipcode string) string {
	if country := inferCountryFromTargetURL(targetURL); country != "" {
		return country
	}
	return inferCountryFromZipcode(zipcode)
}

func inferCountryFromTargetURL(targetURL string) string {
	if targetURL == "" {
		return ""
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		return ""
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Host))
	host = strings.TrimPrefix(host, "www.")

	targetCountries := map[string]string{
		"amazon.com":    "United States",
		"amazon.ca":     "Canada",
		"amazon.co.uk":  "United Kingdom",
		"amazon.de":     "Germany",
		"amazon.fr":     "France",
		"amazon.it":     "Italy",
		"amazon.es":     "Spain",
		"amazon.co.jp":  "Japan",
		"amazon.com.au": "Australia",
		"amazon.in":     "India",
		"amazon.com.mx": "Mexico",
		"amazon.com.br": "Brazil",
		"amazon.nl":     "Netherlands",
		"amazon.se":     "Sweden",
		"amazon.pl":     "Poland",
	}

	return targetCountries[host]
}

// inferCountryFromZipcode 根据邮编格式推断目标国家名称（用于 GLUXCountryList 选项匹配）
func inferCountryFromZipcode(zipcode string) string {
	return inferCountryFromZipcodeValue(zipcode)
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

func (zih *ZipcodeInputHandler) waitForZipcodeInterfaceReady(page playwright.Page) {
	if page.IsClosed() {
		return
	}

	for _, selector := range zipcodeInterfaceReadySelectors {
		if err := page.Locator(selector).First().WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(1500),
		}); err == nil {
			return
		}
	}
}

func (zih *ZipcodeInputHandler) isZipcodeInterfaceReady(page playwright.Page) bool {
	if page == nil || page.IsClosed() {
		return false
	}

	for _, selector := range zipcodeInterfaceReadySelectors {
		locator := page.Locator(selector).First()
		count, err := locator.Count()
		if err != nil || count == 0 {
			continue
		}

		if visible, err := locator.IsVisible(); err == nil && visible {
			return true
		}
	}

	return false
}

func isZipcodeSuccessConfirmationText(text string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(text), " "))
	if normalized == "" {
		return false
	}

	return strings.Contains(normalized, "you're now shopping for delivery to") ||
		strings.Contains(normalized, "we will use your selected location to show all products available")
}

func (zih *ZipcodeInputHandler) getVisibleLocationModalText(page playwright.Page) string {
	if page == nil || page.IsClosed() {
		return ""
	}

	modalLocator := page.Locator(".a-popover-modal:visible").First()
	count, err := modalLocator.Count()
	if err != nil || count == 0 {
		return ""
	}

	text, err := modalLocator.TextContent()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(text)
}

func (zih *ZipcodeInputHandler) getAddressDisplayText(page playwright.Page) string {
	if page.IsClosed() {
		return ""
	}

	addressSelectors := []string{
		"#glow-ingress-line2",
		"#glow-ingress-block",
		"#nav-global-location-slot",
		"#GLUXZipConfirmationMessage",
	}

	for _, selector := range addressSelectors {
		text, err := page.Locator(selector).First().TextContent()
		if err == nil && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}

	return ""
}

func (zih *ZipcodeInputHandler) closeLocationDialogIfVisible(page playwright.Page) bool {
	if page.IsClosed() {
		return false
	}

	if zih.confirmZipcodeSuccessDialog(page) {
		return true
	}

	dialogLocator := page.Locator(".a-popover-modal:visible").First()
	for i := 0; i < 2; i++ {
		isVisible, err := dialogLocator.IsVisible()
		if err != nil || !isVisible {
			return false
		}

		if err := page.Keyboard().Press("Escape"); err != nil {
			return false
		}

		logger.GetGlobalLogger("crawler/amazon").Infof("地址弹层仍存在，发送 ESC 关闭 (第%d次)", i+1)
		if err := dialogLocator.WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateHidden,
			Timeout: playwright.Float(800),
		}); err == nil {
			return false
		}
	}

	return false
}

func (zih *ZipcodeInputHandler) confirmZipcodeSuccessDialog(page playwright.Page) bool {
	if page == nil || page.IsClosed() {
		return false
	}

	modalText := zih.getVisibleLocationModalText(page)
	if modalText == "" || !isZipcodeSuccessConfirmationText(modalText) {
		return false
	}

	for _, selector := range zipcodeSuccessConfirmationSelectors {
		locator := page.Locator(selector).First()
		count, err := locator.Count()
		if err != nil || count == 0 {
			continue
		}

		visible, err := locator.IsVisible()
		if err != nil || !visible {
			continue
		}

		if err := locator.Click(playwright.LocatorClickOptions{
			Timeout: playwright.Float(2500),
		}); err != nil {
			logger.GetGlobalLogger("crawler/amazon").Infof("点击邮编成功确认按钮失败: selector=%s err=%v", selector, err)
			continue
		}

		logger.GetGlobalLogger("crawler/amazon").Infof("已确认邮编更新成功弹层: %s", selector)
		_ = page.Locator(".a-popover-modal:visible").First().WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateHidden,
			Timeout: playwright.Float(3000),
		})
		return true
	}

	ok, err := page.Evaluate(`() => {
		const modal = document.querySelector('.a-popover-modal');
		if (!modal) return false;
		const candidates = Array.from(modal.querySelectorAll('button,input[type="submit"],input[type="button"]'));
		for (const el of candidates) {
			const text = ((el.innerText || el.textContent || el.value || '') + ' ' + (el.getAttribute('aria-label') || '')).trim().toLowerCase();
			if (el.id === 'GLUXConfirmClose' || text.includes('continue')) {
				el.click();
				return true;
			}
		}
		return false;
	}`)
	if err == nil {
		if clicked, _ := ok.(bool); clicked {
			logger.GetGlobalLogger("crawler/amazon").Infof("已通过脚本兜底确认邮编更新成功弹层")
			_ = page.Locator(".a-popover-modal:visible").First().WaitFor(playwright.LocatorWaitForOptions{
				State:   playwright.WaitForSelectorStateHidden,
				Timeout: playwright.Float(3000),
			})
			return true
		}
	}

	return false
}

func (zih *ZipcodeInputHandler) waitForZipcodeApplyCompletion(page playwright.Page, beforeText string) {
	if page.IsClosed() {
		return
	}

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if page.IsClosed() {
			return
		}

		currentText := zih.getAddressDisplayText(page)
		if beforeText != "" && currentText != "" && currentText != beforeText {
			logger.GetGlobalLogger("crawler/amazon").Infof("检测到地址文本变化: '%s' -> '%s'", beforeText, currentText)
			return
		}

		modalText := zih.getVisibleLocationModalText(page)
		if isZipcodeSuccessConfirmationText(modalText) {
			logger.GetGlobalLogger("crawler/amazon").Infof("检测到邮编成功确认弹层，等待确认按钮处理")
			return
		}

		inputVisible, _ := page.Locator("#GLUXZipUpdateInput").First().IsVisible()
		dialogVisible, _ := page.Locator("div[role='dialog']").First().IsVisible()
		if !inputVisible && !dialogVisible {
			return
		}

		time.Sleep(200 * time.Millisecond)
	}
}

func (zih *ZipcodeInputHandler) waitForAddressTextChange(page playwright.Page, beforeText string, timeout time.Duration) {
	if page == nil || page.IsClosed() || strings.TrimSpace(beforeText) == "" {
		return
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if page.IsClosed() {
			return
		}

		currentText := zih.getAddressDisplayText(page)
		if currentText != "" && currentText != beforeText {
			logger.GetGlobalLogger("crawler/amazon").Infof("确认弹层关闭后地址已更新: '%s' -> '%s'", beforeText, currentText)
			return
		}

		time.Sleep(200 * time.Millisecond)
	}
}

func shouldRefreshAfterSuccessfulZipcodeConfirmation(beforeText, currentText string, confirmed bool) bool {
	if !confirmed {
		return false
	}

	beforeText = strings.TrimSpace(beforeText)
	currentText = strings.TrimSpace(currentText)

	if beforeText == "" {
		return currentText == ""
	}

	return currentText == "" || currentText == beforeText
}

// submitZipcodeChange 提交邮编设置
func (zih *ZipcodeInputHandler) submitZipcodeChange(page playwright.Page) error {
	if page.IsClosed() {
		return fmt.Errorf("页面在查找Apply按钮前被关闭")
	}

	beforeText := zih.getAddressDisplayText(page)

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
			logger.GetGlobalLogger("crawler/amazon").Infof("点击Apply按钮失败，尝试脚本点击兜底: %v", err)
			if fallbackErr := zih.fallbackApplyZipcodeChange(page, selectedSelector); fallbackErr != nil {
				return fmt.Errorf("点击Apply按钮失败: %w", err)
			}
		} else {
			logger.GetGlobalLogger("crawler/amazon").Infof("成功点击Apply按钮")
		}
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("等待Apply按钮处理完成，等待地址信息更新...")
	zih.waitForZipcodeApplyCompletion(page, beforeText)

	// 检查页面状态
	if page.IsClosed() {
		return fmt.Errorf("页面在Apply按钮处理后被关闭")
	}

	confirmedSuccess := zih.closeLocationDialogIfVisible(page)
	addressSyncWait := zipcodeAddressSyncWaitDefault
	if confirmedSuccess {
		addressSyncWait = zipcodeAddressSyncWaitAfterConfirm
	}
	zih.waitForAddressTextChange(page, beforeText, addressSyncWait)

	currentText := zih.getAddressDisplayText(page)
	if shouldRefreshAfterSuccessfulZipcodeConfirmation(beforeText, currentText, confirmedSuccess) {
		logger.GetGlobalLogger("crawler/amazon").Infof("成功确认邮编后地址文本仍未更新，立即刷新页面同步配送上下文")
		if _, err := page.Reload(playwright.PageReloadOptions{
			Timeout: playwright.Float(15000),
		}); err == nil {
			_ = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
				State:   playwright.LoadStateDomcontentloaded,
				Timeout: playwright.Float(15000),
			})
		}
	}

	return nil
}

func (zih *ZipcodeInputHandler) fallbackApplyZipcodeChange(page playwright.Page, selector string) error {
	if page == nil || page.IsClosed() {
		return fmt.Errorf("页面已关闭，无法执行Apply兜底")
	}

	if selector != "" {
		ok, err := page.Evaluate(`(sel) => {
			const el = document.querySelector(sel);
			if (!el) return false;
			el.click();
			return true;
		}`, selector)
		if err == nil {
			if clicked, _ := ok.(bool); clicked {
				logger.GetGlobalLogger("crawler/amazon").Infof("已通过脚本兜底点击Apply按钮: %s", selector)
				return nil
			}
		}
	}

	if err := page.Keyboard().Press("Enter"); err != nil {
		return fmt.Errorf("Apply兜底失败，按回车键失败: %w", err)
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("已通过回车键兜底提交邮编更新")
	return nil
}
