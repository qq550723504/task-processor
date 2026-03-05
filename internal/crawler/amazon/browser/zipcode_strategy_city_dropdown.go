// Package browser 提供城市下拉框邮编输入策略（沙特、阿联酋等站点）
package browser

import (
	"fmt"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// CityDropdownStrategy 城市下拉框策略（沙特、阿联酋等站点使用城市选择而非邮编输入）
type CityDropdownStrategy struct {
	BaseZipcodeStrategy
}

// NewCityDropdownStrategy 创建城市下拉框策略
func NewCityDropdownStrategy() *CityDropdownStrategy {
	return &CityDropdownStrategy{}
}

// GetName 获取策略名称
func (s *CityDropdownStrategy) GetName() string {
	return "CityDropdown"
}

// CanHandle 判断是否可以处理（检查是否存在城市下拉框）
func (s *CityDropdownStrategy) CanHandle(page playwright.Page, zipcode string) bool {
	selectors := []string{
		"div[role='dialog'] [role='combobox']",
		"[role='combobox'][aria-haspopup='menu']",
		"select#GLUXCityList",
		"span.a-dropdown-container select[name='city']",
		"span.a-dropdown-container select[name='district']",
		"select[name='locationType']",
		"select[name='district']",
		"select.a-native-dropdown",
		"div[role='dialog'] select[name='city']",
	}

	for _, selector := range selectors {
		if s.isElementVisible(page, selector) {
			// 额外检查：确保不是国家选择器
			locator := page.Locator(selector).First()
			if id, err := locator.GetAttribute("id"); err == nil && id == "GLUXCountryList" {
				continue
			}
			if name, err := locator.GetAttribute("name"); err == nil && name == "country" {
				continue
			}
			return true
		}
	}

	return false
}

// Handle 处理城市下拉框选择
func (s *CityDropdownStrategy) Handle(page playwright.Page, zipcode string) error {
	cityDropdownSelectors := []string{
		"div[role='dialog'] [role='combobox']",
		"[role='combobox'][aria-haspopup='menu']",
		"select#GLUXCityList",
		"span.a-dropdown-container select[name='city']",
		"span.a-dropdown-container select[name='district']",
		"select[name='locationType']",
		"select[name='district']",
		"select.a-native-dropdown",
		"div[role='dialog'] select[name='city']",
		"div[aria-label*='location'] select[name='city']",
		"div[aria-label*='delivery'] select[name='city']",
		"#GLUXChangePostalCodeLink + select[name='city']",
	}

	cityDropdown := s.findVisibleElement(page, cityDropdownSelectors)
	if cityDropdown == nil {
		return fmt.Errorf("未找到城市下拉框")
	}

	// 检查是否是combobox类型
	isCombobox := false
	if role, err := cityDropdown.GetAttribute("role"); err == nil && role == "combobox" {
		isCombobox = true
	}

	// 根据邮编映射到城市
	cityName := s.mapZipcodeToCityName(zipcode)
	if cityName == "" {
		return fmt.Errorf("无法将邮编 %s 映射到城市名称", zipcode)
	}

	logrus.Infof("[%s] 尝试选择城市: %s (邮编: %s)", s.GetName(), cityName, zipcode)

	// 如果是combobox,使用点击方式选择
	if isCombobox {
		return s.handleCombobox(page, cityDropdown, cityName)
	}

	// 如果是select元素,使用SelectOption方式
	return s.handleSelectElement(page, cityDropdown, cityName)
}

// handleCombobox 处理combobox类型的下拉框
func (s *CityDropdownStrategy) handleCombobox(page playwright.Page, cityDropdown playwright.Locator, cityName string) error {
	// 点击combobox打开选项列表
	if err := cityDropdown.Click(); err != nil {
		return fmt.Errorf("点击combobox失败: %w", err)
	}
	time.Sleep(500 * time.Millisecond)

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
				logrus.Infof("[%s] 成功点击城市选项: %s", s.GetName(), cityName)
				time.Sleep(1 * time.Second)
				return nil
			}
		}
	}

	return fmt.Errorf("无法找到或点击城市选项: %s", cityName)
}

// handleSelectElement 处理select元素类型的下拉框
func (s *CityDropdownStrategy) handleSelectElement(page playwright.Page, cityDropdown playwright.Locator, cityName string) error {
	// 尝试通过标签选择
	if _, err := cityDropdown.SelectOption(playwright.SelectOptionValues{
		Labels: &[]string{cityName},
	}); err == nil {
		logrus.Infof("[%s] 成功通过标签选择城市: %s", s.GetName(), cityName)
		time.Sleep(1 * time.Second)
		return nil
	}

	// 如果通过标签选择失败,尝试通过值选择
	logrus.Infof("[%s] 通过标签选择失败,尝试其他方式", s.GetName())

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
					logrus.Infof("[%s] 成功通过值选择城市: %s", s.GetName(), cityName)
					time.Sleep(1 * time.Second)
					return nil
				}
			}
		}
	}

	return fmt.Errorf("无法选择城市 %s", cityName)
}

// mapZipcodeToCityName 将邮编映射到城市名称
func (s *CityDropdownStrategy) mapZipcodeToCityName(zipcode string) string {
	// 沙特城市映射
	saudiCityMap := map[string]string{
		"11564": "Riyadh",
		"21432": "Jeddah",
		"23218": "Dammam",
		"31952": "Mecca",
		"24231": "Medina",
		"32272": "Khobar",
		"13521": "Buraidah",
		"51431": "Abha",
		"82723": "Tabuk",
		"41311": "Hail",
	}

	// 阿联酋城市映射
	uaeCityMap := map[string]string{
		"00000": "Dubai",
		"00001": "Abu Dhabi",
		"00002": "Sharjah",
		"00003": "Ajman",
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
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
