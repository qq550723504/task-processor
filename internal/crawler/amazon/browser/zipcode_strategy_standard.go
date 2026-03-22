// Package browser 提供标准邮编输入策略
package browser

import (
	"task-processor/internal/core/logger"
	"fmt"

	"github.com/playwright-community/playwright-go"
)

// StandardZipcodeStrategy 标准单一输入框策略（美国、欧洲等大部分站点）
type StandardZipcodeStrategy struct {
	BaseZipcodeStrategy
}

// NewStandardZipcodeStrategy 创建标准邮编策略
func NewStandardZipcodeStrategy() *StandardZipcodeStrategy {
	return &StandardZipcodeStrategy{}
}

// GetName 获取策略名称
func (s *StandardZipcodeStrategy) GetName() string {
	return "Standard"
}

// CanHandle 判断是否可以处理（标准策略作为兜底，总是返回true）
func (s *StandardZipcodeStrategy) CanHandle(page playwright.Page, zipcode string) bool {
	// 标准策略作为兜底策略，总是可以处理
	return true
}

// Handle 处理标准单一输入框
func (s *StandardZipcodeStrategy) Handle(page playwright.Page, zipcode string) error {
	zipInputSelectors := []string{
		"#GLUXZipUpdateInput",
		"input[name='zipCode']",
		"input[name='postalCode']",
		"input#GLUXZipUpdateInput",
		"input[placeholder*='ZIP']",
		"input[placeholder*='zip']",
		"input[placeholder*='Zip']",
		"input[placeholder*='postal']",
		"input[placeholder*='Postal']",
		"input[aria-label*='ZIP']",
		"input[aria-label*='zip']",
		"input[aria-label*='Zip']",
		"input[aria-label*='postal']",
		"input[aria-label*='Postal']",
		"input[type='text'][maxlength='10']",
		"input[type='text'][maxlength='5']",
		"#zip-code",
		"#postal-code",
		"input.a-input-text[type='text']",
	}

	zipInput := s.findVisibleElement(page, zipInputSelectors)
	if zipInput == nil {
		logger.GetGlobalLogger("crawler/amazon").Infof("[%s] 所有邮编输入框选择器都未找到元素", s.GetName())
		return fmt.Errorf("未找到邮编输入框")
	}

	// 先清空输入框
	if err := zipInput.Clear(); err != nil {
		logger.GetGlobalLogger("crawler/amazon").Infof("[%s] 清空输入框失败: %v", s.GetName(), err)
	}

	// 填写邮编 - 使用 Type 而不是 Fill,模拟真实用户输入
	if err := zipInput.Type(zipcode, playwright.LocatorTypeOptions{
		Delay: playwright.Float(50), // 每个字符间隔50ms,模拟真实输入
	}); err != nil {
		if page.IsClosed() {
			return fmt.Errorf("页面在填写邮编时被关闭: %w", err)
		}
		return fmt.Errorf("填写邮编失败: %w", err)
	}

	// 触发 input 和 change 事件,确保 Amazon 识别到输入
	if _, err := zipInput.Evaluate("el => { el.dispatchEvent(new Event('input', { bubbles: true })); el.dispatchEvent(new Event('change', { bubbles: true })); }", nil); err != nil {
		logger.GetGlobalLogger("crawler/amazon").Infof("[%s] 触发事件失败: %v", s.GetName(), err)
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("[%s] 成功填写标准邮编: %s", s.GetName(), zipcode)
	return nil
}
