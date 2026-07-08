// Package browser 提供Amazon浏览器自动化的邮编获取功能
package browser

import (
	"fmt"
	"strings"
	"task-processor/internal/core/logger"

	"github.com/mxschmitt/playwright-go"
)

// ZipcodeGetter 邮编获取器
type ZipcodeGetter struct{}

// NewZipcodeGetter 创建邮编获取器实例
func NewZipcodeGetter() *ZipcodeGetter {
	return &ZipcodeGetter{}
}

// GetCurrentZipcode 获取当前邮编或城市名称
func (zg *ZipcodeGetter) GetCurrentZipcode(page playwright.Page) (string, error) {
	// 查找显示当前邮编的元素（按优先级排序）
	zipDisplaySelectors := []string{
		"#glow-ingress-line2",         // 主要的邮编显示位置（最常见）
		"#glow-ingress-block",         // 地址块
		"#GLUXZipConfirmationMessage", // 确认消息中的邮编
		"#nav-global-location-slot",   // 导航栏位置槽
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("开始查找当前邮编或城市信息")

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
			logger.GetGlobalLogger("crawler/amazon").Infof("从选择器 %s 获取到文本: %s", selector, text)

			// 先尝试提取邮编（通常是数字）
			zipcode := ExtractZipcode(text)
			if zipcode != "" {
				logger.GetGlobalLogger("crawler/amazon").Infof("成功提取邮编: %s", zipcode)
				return zipcode, nil
			}

			// 如果没有提取到邮编,尝试提取城市名称(沙特等站点)
			cityName := extractCityName(text)
			if cityName != "" {
				logger.GetGlobalLogger("crawler/amazon").Infof("成功提取城市名称: %s", cityName)
				return cityName, nil
			}

			// 如果既没有提取到邮编也没有提取到城市名，但文本不为空
			// 直接返回文本内容（用于英国等站点的部分邮编显示）
			// 但需要过滤掉已知的占位文字（如 "Update location"、"Deliver to" 等）
			if len(text) > 0 && !isLocationPlaceholder(text) {
				logger.GetGlobalLogger("crawler/amazon").Infof("无法提取标准邮编格式，返回原始文本: %s", text)
				return text, nil
			}
		}
	}

	// 如果主要位置没有找到，尝试从导航栏区域查找
	navLocator := page.Locator("#nav-global-location-popover-link, #nav-packard-glow-loc-icon")
	if count, err := navLocator.Count(); err == nil && count > 0 {
		if text, err := navLocator.TextContent(); err == nil && text != "" {
			zipcode := ExtractZipcode(text)
			if zipcode != "" {
				return zipcode, nil
			}

			cityName := extractCityName(text)
			if cityName != "" {
				return cityName, nil
			}
		}
	}

	return "", fmt.Errorf("未找到当前邮编或城市信息")
}

// isLocationPlaceholder 判断文本是否为纯位置占位符（未设置邮编时的提示文字）
// 注意：只有当文本清理后完全等于占位符时才返回 true，
// 避免误杀包含真实地址的容器元素（如 "Delivering to Balzac T4B 2T\nUpdate location"）
func isLocationPlaceholder(text string) bool {
	// 清理文本：压缩所有空白为单个空格
	cleaned := strings.ToUpper(strings.TrimSpace(text))
	cleaned = locationWhitespacePattern.ReplaceAllString(cleaned, " ")

	// 纯占位符：清理后的文本完全匹配
	purePlaceholders := []string{
		"UPDATE LOCATION",
		"DELIVER TO",
		"SELECT YOUR ADDRESS",
		"CHOOSE YOUR LOCATION",
		"SET YOUR ADDRESS",
		"HELLO, SIGN IN",
		"HELLO SELECT YOUR ADDRESS",
	}
	for _, p := range purePlaceholders {
		if cleaned == p {
			logger.GetGlobalLogger("crawler/amazon").Infof("检测到位置占位符文字，跳过: %s", text)
			return true
		}
	}
	return false
}

// extractCityName 从文本中提取城市名称
func extractCityName(text string) string {
	// 已知的城市列表(沙特、阿联酋等)
	knownCities := []string{
		"Riyadh", "Jeddah", "Makkah Al Mukarramah", "Dammam",
		"Al Madinah Al Munawwarah", "Al Khobar", "Al Ahsa", "At Taif",
		"Al Jubail", "Tabuk", "Abha", "Khamis Mushayt", "Hail", "Yanbu", "Jazan",
		"Dubai", "Abu Dhabi", "Sharjah", "Ajman",
		"Mecca", "Medina", "Khobar", "Buraidah", // 别名
	}

	// 检查文本中是否包含已知城市名称
	lowerText := strings.ToLower(text)
	for _, city := range knownCities {
		if strings.Contains(lowerText, strings.ToLower(city)) {
			return city
		}
	}

	return ""
}
