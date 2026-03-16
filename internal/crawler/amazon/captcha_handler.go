// Package amazon 提供Amazon验证码处理功能
package amazon

import (
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// CaptchaHandler 验证码处理器
type CaptchaHandler struct{}

// NewCaptchaHandler 创建验证码处理器
func NewCaptchaHandler() *CaptchaHandler {
	return &CaptchaHandler{}
}

// TryBypassCaptcha 尝试绕过验证码
func (ch *CaptchaHandler) TryBypassCaptcha(page playwright.Page) error {
	logrus.Info("检测到验证码，尝试绕过策略")

	// 策略1: 等待一段时间，有时验证码会自动消失
	logrus.Info("策略1: 等待验证码自动消失")
	time.Sleep(5 * time.Second)

	if !ch.checkCaptchaExists(page) {
		logrus.Info("✅ 验证码已自动消失")
		return nil
	}

	// 策略2: 尝试刷新页面
	logrus.Info("策略2: 刷新页面尝试绕过验证码")
	if _, err := page.Reload(); err != nil {
		logrus.Warnf("页面刷新失败: %v", err)
	} else {
		time.Sleep(3 * time.Second)
		if !ch.checkCaptchaExists(page) {
			logrus.Info("✅ 刷新页面后验证码消失")
			return nil
		}
	}

	// 策略3: 尝试点击页面其他区域
	logrus.Info("策略3: 尝试点击页面其他区域")
	if err := page.Click("body"); err == nil {
		time.Sleep(2 * time.Second)
		if !ch.checkCaptchaExists(page) {
			logrus.Info("✅ 点击页面后验证码消失")
			return nil
		}
	}

	// 所有策略都失败
	logrus.Warn("❌ 所有验证码绕过策略都失败，需要重建浏览器实例")
	return fmt.Errorf("无法绕过验证码")
}

// checkCaptchaExists 检查验证码是否存在
func (ch *CaptchaHandler) checkCaptchaExists(page playwright.Page) bool {
	captchaSelectors := []string{
		"form[action*='validateCaptcha']",
		"#captchacharacters",
		".a-box-inner h4:has-text('Enter the characters you see below')",
		"img[src*='captcha']",
		"body:has-text('Sorry, we just need to make sure you\\'re not a robot')",
		"input[name='captchacharacters']",
		".cvf-widget-form",
	}

	for _, selector := range captchaSelectors {
		count, err := page.Locator(selector).Count()
		if err != nil {
			continue
		}
		if count > 0 {
			return true
		}
	}

	return false
}

// HandleCaptchaWithRetry 带重试的验证码处理
func (ch *CaptchaHandler) HandleCaptchaWithRetry(page playwright.Page, maxRetries int) error {
	for attempt := 0; attempt < maxRetries; attempt++ {
		logrus.Infof("验证码处理尝试 %d/%d", attempt+1, maxRetries)

		err := ch.TryBypassCaptcha(page)
		if err == nil {
			return nil
		}

		if attempt < maxRetries-1 {
			logrus.Infof("第 %d 次尝试失败，等待后重试", attempt+1)
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
		}
	}

	return fmt.Errorf("验证码处理失败，已尝试 %d 次", maxRetries)
}
