// Package browser 提供Amazon浏览器自动化的邮编处理工具方法
package browser

import (
	"task-processor/internal/core/logger"
	"time"

	"github.com/mxschmitt/playwright-go"
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
	if match := extractJapanZipcodePattern.FindString(text); match != "" {
		return match
	}

	// 匹配巴西邮编格式 (XXXXX-XXX)
	if match := extractBrazilZipcodePattern.FindString(text); match != "" {
		return match
	}

	// 匹配美国邮编格式 (XXXXX 或 XXXXX-XXXX)
	if match := extractUSZipcodePattern.FindString(text); match != "" {
		return match
	}

	// 匹配英国邮编格式 (例如: SW1A 1AA, EC1A 1BB)
	if match := extractUKZipcodePattern.FindString(text); match != "" {
		return match
	}

	// 匹配加拿大邮编格式 (例如: M5H 2N2, K1A 0B1)
	if match := extractCanadaZipcodePattern.FindString(text); match != "" {
		return match
	}

	// 匹配加拿大不完整邮编（仅 FSA，如 "Delivering to Balzac T4B 2T" 中的 T4B）
	// Amazon 有时只显示前向码 + 部分后向码
	if match := extractCanadaFSAPattern.FindString(text); match != "" {
		return match
	}

	// 如果没有找到任何格式，尝试查找纯数字邮编（德国、法国、意大利等）
	if match := extractSimpleZipcodePattern.FindString(text); match != "" {
		return match
	}

	return ""
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
				logger.GetGlobalLogger("crawler/amazon").Infof("检测到价格元素: %s", selector)
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
				logger.GetGlobalLogger("crawler/amazon").Infof("发现 Continue Shopping 按钮，尝试点击")
				if err := locator.Click(playwright.LocatorClickOptions{
					Timeout: playwright.Float(5000), // 5秒超时
				}); err != nil {
					logger.GetGlobalLogger("crawler/amazon").Infof("点击 Continue Shopping 按钮失败: %v", err)
				} else {
					logger.GetGlobalLogger("crawler/amazon").Infof("成功点击 Continue Shopping 按钮")
					time.Sleep(1 * time.Second)
					break
				}
			}
		}
	}
}
