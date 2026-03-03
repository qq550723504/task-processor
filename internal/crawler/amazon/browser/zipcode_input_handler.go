// Package browser 提供Amazon浏览器自动化的邮编输入处理功能
package browser

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// ZipcodeInputHandler 邮编输入处理器
type ZipcodeInputHandler struct{}

// NewZipcodeInputHandler 创建邮编输入处理器实例
func NewZipcodeInputHandler() *ZipcodeInputHandler {
	return &ZipcodeInputHandler{}
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

// handleZipcodeInput 处理邮编输入
func (zih *ZipcodeInputHandler) handleZipcodeInput(page playwright.Page, zipcode string) error {
	if page.IsClosed() {
		return fmt.Errorf("页面在查找邮编输入框前被关闭")
	}

	// 尝试城市下拉框选择(沙特、阿联酋等站点)
	if handled, err := zih.handleCityDropdown(page, zipcode); err != nil {
		return err
	} else if handled {
		return nil
	}

	// 尝试日本站的分离式输入
	if handled, err := zih.handleJapaneseZipcode(page, zipcode); err != nil {
		return err
	} else if handled {
		return nil
	}

	// 处理标准单一输入框
	return zih.handleStandardZipcode(page, zipcode)
}

// handleCityDropdown 处理城市下拉框选择(沙特、阿联酋等站点)
func (zih *ZipcodeInputHandler) handleCityDropdown(page playwright.Page, zipcode string) (bool, error) {
	// 城市下拉框选择器(沙特站点使用城市选择而非邮编输入)
	cityDropdownSelectors := []string{
		"div[role='dialog'] [role='combobox']",    // 沙特站点对话框中的combobox(主要选择器)
		"[role='combobox'][aria-haspopup='menu']", // 带菜单弹出的combobox
		"select#GLUXCityList",                     // 城市列表select
		// 注意：不包含 select#GLUXCountryList，因为那是国家选择器，不是城市选择器
		"span.a-dropdown-container select[name='city']",     // 下拉容器中的城市select
		"span.a-dropdown-container select[name='district']", // 下拉容器中的地区select
		"select[name='locationType']",                       // 位置类型选择
		"select[name='district']",                           // 地区选择
		"select.a-native-dropdown",                          // Amazon原生下拉框
		"div[role='dialog'] select[name='city']",            // 对话框中的城市select
		"div[aria-label*='location'] select[name='city']",   // 位置对话框中的城市select
		"div[aria-label*='delivery'] select[name='city']",   // 配送对话框中的城市select
		"#GLUXChangePostalCodeLink + select[name='city']",   // 邮编链接旁的城市select
	}

	var cityDropdown playwright.Locator
	var isCombobox bool

	// 查找城市下拉框
	for _, selector := range cityDropdownSelectors {
		locator := page.Locator(selector).First()
		if count, err := locator.Count(); err == nil && count > 0 {
			if isVisible, err := locator.IsVisible(); err == nil && isVisible {
				// 额外检查：确保不是国家选择器
				if id, err := locator.GetAttribute("id"); err == nil && id == "GLUXCountryList" {
					logrus.Infof("跳过国家选择器: %s", selector)
					continue
				}
				if name, err := locator.GetAttribute("name"); err == nil && name == "country" {
					logrus.Infof("跳过国家选择器(name=country): %s", selector)
					continue
				}

				cityDropdown = locator
				// 检查是否是combobox类型
				if role, err := locator.GetAttribute("role"); err == nil && role == "combobox" {
					isCombobox = true
				}
				logrus.Infof("找到城市下拉框: %s (combobox: %v)", selector, isCombobox)
				break
			}
		}
	}

	// 如果没有找到下拉框,返回false让其他方法处理
	if cityDropdown == nil {
		return false, nil
	}

	// 根据邮编映射到城市
	cityName := zih.mapZipcodeToCityName(zipcode)
	if cityName == "" {
		logrus.Infof("无法将邮编 %s 映射到城市名称，跳过城市下拉框处理", zipcode)
		return false, nil
	}

	logrus.Infof("尝试选择城市: %s (邮编: %s)", cityName, zipcode)

	// 如果是combobox,使用点击方式选择
	if isCombobox {
		// 点击combobox打开选项列表
		if err := cityDropdown.Click(); err != nil {
			return false, fmt.Errorf("点击combobox失败: %w", err)
		}
		time.Sleep(500 * time.Millisecond) // 等待选项列表出现

		// 查找并点击匹配的选项
		optionSelectors := []string{
			fmt.Sprintf("div[role='option']:has-text('%s')", cityName),
			fmt.Sprintf("li[role='option']:has-text('%s')", cityName),
			fmt.Sprintf("[role='option']:has-text('%s')", cityName),
			fmt.Sprintf("div.a-popover-inner [data-value='%s']", cityName),
			fmt.Sprintf("div.a-popover-inner:has-text('%s')", cityName),
		}

		for _, optSelector := range optionSelectors {
			optionLocator := page.Locator(optSelector).First()
			if count, err := optionLocator.Count(); err == nil && count > 0 {
				if err := optionLocator.Click(); err == nil {
					logrus.Infof("成功点击城市选项: %s", cityName)
					time.Sleep(1 * time.Second)
					return true, nil
				}
			}
		}

		return false, fmt.Errorf("无法找到或点击城市选项: %s", cityName)
	}

	// 如果是select元素,使用SelectOption方式
	if _, err := cityDropdown.SelectOption(playwright.SelectOptionValues{
		Labels: &[]string{cityName},
	}); err != nil {
		// 如果通过标签选择失败,尝试通过值选择
		logrus.Infof("通过标签选择失败,尝试其他方式: %v", err)

		// 获取所有选项并查找匹配项
		options := page.Locator("select option")
		count, _ := options.Count()

		for i := 0; i < count; i++ {
			option := options.Nth(i)
			text, _ := option.TextContent()
			if text != "" && (text == cityName || containsIgnoreCase(text, cityName)) {
				value, _ := option.GetAttribute("value")
				if value != "" {
					if _, err := cityDropdown.SelectOption(playwright.SelectOptionValues{
						Values: &[]string{value},
					}); err == nil {
						logrus.Infof("成功通过值选择城市: %s", cityName)
						time.Sleep(1 * time.Second)
						return true, nil
					}
				}
			}
		}

		return false, fmt.Errorf("无法选择城市 %s: %w", cityName, err)
	}

	logrus.Infof("成功选择城市: %s", cityName)
	time.Sleep(1 * time.Second) // 等待选择生效
	return true, nil
}

// mapZipcodeToCityName 将邮编映射到城市名称
func (zih *ZipcodeInputHandler) mapZipcodeToCityName(zipcode string) string {
	// 沙特城市映射
	saudiCityMap := map[string]string{
		"11564": "Riyadh",   // 利雅得
		"21432": "Jeddah",   // 吉达
		"23218": "Dammam",   // 达曼
		"31952": "Mecca",    // 麦加
		"24231": "Medina",   // 麦地那
		"32272": "Khobar",   // 胡拜尔
		"13521": "Buraidah", // 布赖代
		"51431": "Abha",     // 艾卜哈
		"82723": "Tabuk",    // 塔布克
		"41311": "Hail",     // 哈伊勒
	}

	// 阿联酋城市映射
	uaeCityMap := map[string]string{
		"00000": "Dubai",     // 迪拜
		"00001": "Abu Dhabi", // 阿布扎比
		"00002": "Sharjah",   // 沙迦
		"00003": "Ajman",     // 阿治曼
	}

	// 先尝试沙特映射
	if city, exists := saudiCityMap[zipcode]; exists {
		return city
	}

	// 再尝试阿联酋映射
	if city, exists := uaeCityMap[zipcode]; exists {
		return city
	}

	return ""
}

// containsIgnoreCase 不区分大小写的字符串包含检查
func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// handleJapaneseZipcode 处理日本站的分离式邮编输入
func (zih *ZipcodeInputHandler) handleJapaneseZipcode(page playwright.Page, zipcode string) (bool, error) {
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
					return false, fmt.Errorf("页面在填写日本邮编第一部分时被关闭: %w", err)
				}
				return false, fmt.Errorf("填写日本邮编第一部分失败: %w", err)
			}

			// 等待一下，确保第一个输入框的值已经设置
			time.Sleep(300 * time.Millisecond)

			// 清空并填写第二个输入框（后4位）
			if err := jpZipInput2.Clear(); err != nil {
				logrus.Infof("清空第二个输入框失败: %v", err)
			}
			if err := jpZipInput2.Fill(part2); err != nil {
				if page.IsClosed() {
					return false, fmt.Errorf("页面在填写日本邮编第二部分时被关闭: %w", err)
				}
				return false, fmt.Errorf("填写日本邮编第二部分失败: %w", err)
			}
			logrus.Infof("成功填写日本站分离式邮编")
			return true, nil
		} else {
			return false, fmt.Errorf("日本邮编格式不正确，应为 XXX-XXXX 格式: %s", zipcode)
		}
	}

	return false, nil
}

// handleStandardZipcode 处理标准单一输入框
func (zih *ZipcodeInputHandler) handleStandardZipcode(page playwright.Page, zipcode string) error {
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

	return nil
}

// submitZipcodeChange 提交邮编设置
func (zih *ZipcodeInputHandler) submitZipcodeChange(page playwright.Page) error {
	if page.IsClosed() {
		return fmt.Errorf("页面在查找Apply按钮前被关闭")
	}

	applyButtonSelectors := []string{
		// 最精确的选择器 - 排除购物车按钮
		"div[role='dialog'] button:has-text('Apply'):not(:has-text('Cart')):not(:has-text('Buy'))",
		"div[role='dialog'] button:has-text('Done'):not(:has-text('Cart')):not(:has-text('Buy'))",
		"input[aria-labelledby='GLUXZipUpdate-announce']", // Amazon邮编更新按钮
		"#GLUXZipUpdate",           // Amazon主要的设置按钮
		"span#GLUXZipUpdate input", // GLUXZipUpdate内的input
		"button:has-text('Apply'):not(:has-text('Cart')):not(:has-text('Buy'))", // 英文版的Apply按钮(排除购物车)
		"button:text('Apply')", // 英文版的Apply按钮（精确匹配）
		"input[type='submit'][aria-labelledby='GLUXZipUpdate-announce']",       // 带特定aria-labelledby的提交按钮
		"button:has-text('Done'):not(:has-text('Cart')):not(:has-text('Buy'))", // Done按钮(排除购物车)
		"button:has-text('Save'):not(:has-text('Cart')):not(:has-text('Buy'))", // Save按钮(排除购物车)
		"#zip-code-apply",    // 邮编应用按钮
		"#postal-code-apply", // 邮编应用按钮
		".apply-button",      // 应用按钮类
		".save-button",       // 保存按钮类
	}

	var applyButton playwright.Locator
	var selectedSelector string
	var buttonText string

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
					logrus.Infof("检查按钮文本: '%s' (选择器: %s)", text, selector)

					// 严格排除购物车相关按钮
					if strings.Contains(lowerText, "cart") ||
						strings.Contains(lowerText, "buy") ||
						strings.Contains(lowerText, "add to") ||
						strings.Contains(lowerText, "purchase") {
						logrus.Infof("跳过购物车相关按钮: %s", text)
						continue
					}

					// 只接受明确的确认按钮文本
					validTexts := []string{"apply", "done", "save", "ok", "confirm", "submit"}
					isValid := false
					for _, validText := range validTexts {
						if strings.Contains(lowerText, validText) {
							isValid = true
							break
						}
					}

					if !isValid {
						logrus.Infof("按钮文本不符合确认按钮要求: %s", text)
						continue
					}

					buttonText = text
				}

				applyButton = locator
				selectedSelector = selector
				logrus.Infof("找到确认按钮: %s (文本: %s)", selector, buttonText)
				break
			}
		}
	}

	if applyButton == nil {
		// 如果没有找到应用按钮，尝试按回车键
		logrus.Infof("未找到Apply按钮,尝试按回车键")
		if err := page.Keyboard().Press("Enter"); err != nil {
			if page.IsClosed() {
				return fmt.Errorf("页面在按回车键时被关闭: %w", err)
			}
			return fmt.Errorf("按回车键失败: %w", err)
		}
	} else {
		// 点击Apply按钮
		logrus.Infof("准备点击确认按钮: %s", selectedSelector)
		if err := applyButton.Click(); err != nil {
			if page.IsClosed() {
				return fmt.Errorf("页面在点击Apply按钮时被关闭: %w", err)
			}
			return fmt.Errorf("点击Apply按钮失败: %w", err)
		}
		logrus.Infof("成功点击确认按钮")
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

	// 检查是否有 "Continue Shopping" 按钮需要点击
	HandleContinueShoppingButtonInZipcode(page)

	return nil
}
