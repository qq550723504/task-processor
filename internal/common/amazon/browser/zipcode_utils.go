// Package browser 提供Amazon浏览器自动化的邮编处理工具方法
package browser

import (
	"regexp"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// ExtractZipcode 从文本中提取邮编
func ExtractZipcode(text string) string {
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

// DebugNavigationElements 调试导航栏元素（帮助找到正确的选择器）
func DebugNavigationElements(page playwright.Page) error {
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

// CheckIfPriceAvailable 检查页面是否已经显示价格信息
func CheckIfPriceAvailable(page playwright.Page) bool {
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

// HandleContinueShoppingButtonInZipcode 处理"继续购物"按钮（在zipcode设置流程中使用）
func HandleContinueShoppingButtonInZipcode(page playwright.Page) {
	if page.IsClosed() {
		return
	}

	continueShoppingSelectors := GetContinueShoppingSelectors()

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
