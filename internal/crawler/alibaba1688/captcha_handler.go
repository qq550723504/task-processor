// Package alibaba1688 提供1688验证码处理功能
package alibaba1688

import (
	"strings"
	"task-processor/internal/core/logger"
	"time"

	"github.com/playwright-community/playwright-go"
)

// HandlePageCaptcha 处理页面中的各种验证码
func (ch *CaptchaHandler) HandlePageCaptcha(page playwright.Page) error {
	// 先等待一下，让验证码有时间显示
	time.Sleep(2 * time.Second)

	// 检查页面URL和标题，判断是否在验证码页面
	currentURL := page.URL()
	title, _ := page.Title()

	logger.GetGlobalLogger("crawler/alibaba1688").Debugf("当前页面URL: %s, 标题: %s", currentURL, title)

	// 如果在验证码拦截页面，直接处理验证码
	if strings.Contains(strings.ToLower(title), "captcha") ||
		strings.Contains(strings.ToLower(title), "验证") ||
		strings.Contains(currentURL, "captcha") {
		logger.GetGlobalLogger("crawler/alibaba1688").Info("检测到验证码拦截页面，开始处理验证码")
	}

	// 检查是否有登录提示
	if err := ch.handleLoginPrompt(page); err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Warnf("处理登录提示失败: %v", err)
	}

	// 检查并处理滑动验证码
	if err := ch.handleSliderCaptcha(page); err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Warnf("处理滑动验证码失败: %v", err)
		return err // 滑动验证码失败时返回错误
	}

	// 检查其他类型的验证码
	if err := ch.handleOtherCaptcha(page); err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Warnf("处理其他验证码失败: %v", err)
		return err
	}

	return nil
}
