// Package browser 提供Amazon浏览器自动化的邮编获取功能
package browser

import (
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// ZipcodeGetter 邮编获取器
type ZipcodeGetter struct{}

// NewZipcodeGetter 创建邮编获取器实例
func NewZipcodeGetter() *ZipcodeGetter {
	return &ZipcodeGetter{}
}

// GetCurrentZipcode 获取当前邮编
func (zg *ZipcodeGetter) GetCurrentZipcode(page playwright.Page) (string, error) {
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
			zipcode := ExtractZipcode(text)
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
			zipcode := ExtractZipcode(text)
			if zipcode != "" {
				return zipcode, nil
			}
		}
	}

	return "", fmt.Errorf("未找到当前邮编信息")
}
