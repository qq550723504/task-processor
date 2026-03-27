// Package browser 提供日本站邮编输入策略
package browser

import (
	"task-processor/internal/core/logger"
	"fmt"
	"regexp"
	"time"

	"github.com/playwright-community/playwright-go"
)

// JapaneseZipcodeStrategy 日本站分离式邮编输入策略
type JapaneseZipcodeStrategy struct {
	BaseZipcodeStrategy
}

// NewJapaneseZipcodeStrategy 创建日本站邮编策略
func NewJapaneseZipcodeStrategy() *JapaneseZipcodeStrategy {
	return &JapaneseZipcodeStrategy{}
}

// GetName 获取策略名称
func (s *JapaneseZipcodeStrategy) GetName() string {
	return "Japanese"
}

// CanHandle 判断是否可以处理（检查是否存在日本站的分离式输入框）
func (s *JapaneseZipcodeStrategy) CanHandle(page playwright.Page, zipcode string) bool {
	jpZipSelectors1 := []string{
		"input[name='zipCode1']",
		"input[id='zipCode1']",
		"input[name='zip1']",
		"input[id='zip1']",
		"input[maxlength='3'][type='text']",
		"input[placeholder*='〒']",
	}

	jpZipSelectors2 := []string{
		"input[name='zipCode2']",
		"input[id='zipCode2']",
		"input[name='zip2']",
		"input[id='zip2']",
		"input[maxlength='4'][type='text']",
	}

	// 需要两个输入框都存在才认为是日本站
	hasInput1 := false
	hasInput2 := false

	for _, selector := range jpZipSelectors1 {
		if s.isElementVisible(page, selector) {
			hasInput1 = true
			break
		}
	}

	for _, selector := range jpZipSelectors2 {
		if s.isElementVisible(page, selector) {
			hasInput2 = true
			break
		}
	}

	return hasInput1 && hasInput2
}

// Handle 处理日本站的分离式邮编输入
func (s *JapaneseZipcodeStrategy) Handle(page playwright.Page, zipcode string) error {
	jpZipSelectors1 := []string{
		"input[name='zipCode1']",
		"input[id='zipCode1']",
		"input[name='zip1']",
		"input[id='zip1']",
		"input[maxlength='3'][type='text']",
		"input[placeholder*='〒']",
	}

	jpZipSelectors2 := []string{
		"input[name='zipCode2']",
		"input[id='zipCode2']",
		"input[name='zip2']",
		"input[id='zip2']",
		"input[maxlength='4'][type='text']",
	}

	jpZipInput1 := s.findVisibleElement(page, jpZipSelectors1)
	jpZipInput2 := s.findVisibleElement(page, jpZipSelectors2)

	if jpZipInput1 == nil || jpZipInput2 == nil {
		return fmt.Errorf("未找到日本站的分离式邮编输入框")
	}

	// 日本邮编格式: XXX-XXXX，需要分成两部分填写
	parts := regexp.MustCompile(`(\d{3})-?(\d{4})`).FindStringSubmatch(zipcode)
	if len(parts) != 3 {
		return fmt.Errorf("日本邮编格式不正确，应为 XXX-XXXX 格式: %s", zipcode)
	}

	part1 := parts[1] // 前3位
	part2 := parts[2] // 后4位

	// 清空并填写第一个输入框（前3位）
	if err := jpZipInput1.Clear(); err != nil {
		logger.GetGlobalLogger("crawler/amazon").Infof("[%s] 清空第一个输入框失败: %v", s.GetName(), err)
	}
	if err := jpZipInput1.Fill(part1); err != nil {
		if page.IsClosed() {
			return fmt.Errorf("页面在填写日本邮编第一部分时被关闭: %w", err)
		}
		return fmt.Errorf("填写日本邮编第一部分失败: %w", err)
	}

	// 等待一下，确保第一个输入框的值已经设置
	time.Sleep(300 * time.Millisecond)

	// 清空并填写第二个输入框（后4位）
	if err := jpZipInput2.Clear(); err != nil {
		logger.GetGlobalLogger("crawler/amazon").Infof("[%s] 清空第二个输入框失败: %v", s.GetName(), err)
	}
	if err := jpZipInput2.Fill(part2); err != nil {
		if page.IsClosed() {
			return fmt.Errorf("页面在填写日本邮编第二部分时被关闭: %w", err)
		}
		return fmt.Errorf("填写日本邮编第二部分失败: %w", err)
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("[%s] 成功填写日本站分离式邮编: %s-%s", s.GetName(), part1, part2)
	return nil
}
