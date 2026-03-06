// Package browser 提供Amazon浏览器自动化的邮编输入策略
package browser

import (
	"github.com/playwright-community/playwright-go"
)

// ZipcodeStrategy 邮编输入策略接口
type ZipcodeStrategy interface {
	// CanHandle 判断是否可以处理该邮编输入场景
	CanHandle(page playwright.Page, zipcode string) bool

	// Handle 处理邮编输入
	Handle(page playwright.Page, zipcode string) error

	// GetName 获取策略名称
	GetName() string
}

// BaseZipcodeStrategy 基础邮编策略（提供通用功能）
type BaseZipcodeStrategy struct{}

// isElementVisible 检查元素是否可见
func (b *BaseZipcodeStrategy) isElementVisible(page playwright.Page, selector string) bool {
	locator := page.Locator(selector).First()
	count, err := locator.Count()
	if err != nil || count == 0 {
		return false
	}

	isVisible, err := locator.IsVisible()
	return err == nil && isVisible
}

// findVisibleElement 查找第一个可见的元素
func (b *BaseZipcodeStrategy) findVisibleElement(page playwright.Page, selectors []string) playwright.Locator {
	for _, selector := range selectors {
		locator := page.Locator(selector).First()
		count, err := locator.Count()
		if err == nil && count > 0 {
			if isVisible, err := locator.IsVisible(); err == nil && isVisible {
				return locator
			}
		}
	}
	return nil
}
