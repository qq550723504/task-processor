// Package browser 提供Amazon浏览器自动化的邮编验证功能
package browser

import (
	"fmt"

	"github.com/playwright-community/playwright-go"
)

// ZipcodeValidator 邮编验证器
type ZipcodeValidator struct {
	getter *ZipcodeGetter
}

// NewZipcodeValidator 创建邮编验证器实例
func NewZipcodeValidator() *ZipcodeValidator {
	return &ZipcodeValidator{
		getter: NewZipcodeGetter(),
	}
}

// VerifyZipcode 验证邮编是否设置成功
func (zv *ZipcodeValidator) VerifyZipcode(page playwright.Page, expectedZipcode string) (bool, error) {
	// 获取当前邮编并验证
	currentZipcode, err := zv.getter.GetCurrentZipcode(page)
	if err != nil {
		return false, fmt.Errorf("获取当前邮编失败: %w", err)
	}

	if currentZipcode == expectedZipcode {
		return true, nil
	}

	return false, nil
}
