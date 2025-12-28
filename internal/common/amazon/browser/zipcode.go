// Package browser 提供Amazon浏览器自动化的邮编设置功能（向后兼容接口）
package browser

import (
	"github.com/playwright-community/playwright-go"
)

// 为了向后兼容，保留原有的公共接口

// SetAndVerifyZipcode 设置并验证邮编（向后兼容方法）
// 第二次重试前会刷新页面
func SetAndVerifyZipcode(browserManager *BrowserManager, page playwright.Page, zipcode string) error {
	setter := NewZipcodeSetter(browserManager)
	return setter.SetAndVerifyZipcode(page, zipcode)
}

// GetCurrentZipcode 获取当前邮编（向后兼容方法）
func GetCurrentZipcode(page playwright.Page) (string, error) {
	getter := NewZipcodeGetter()
	return getter.GetCurrentZipcode(page)
}

// VerifyZipcode 验证邮编是否设置成功（向后兼容方法）
func VerifyZipcode(page playwright.Page, expectedZipcode string) (bool, error) {
	validator := NewZipcodeValidator()
	return validator.VerifyZipcode(page, expectedZipcode)
}
