// Package alibaba1688 提供其他类型验证码处理功能
package alibaba1688

import (
	"fmt"
	"strings"
	"task-processor/internal/core/logger"
	"time"

	"github.com/playwright-community/playwright-go"
)

// handleLoginPrompt 处理登录提示
func (ch *CaptchaHandler) handleLoginPrompt(page playwright.Page) error {
	loginSelectors := []string{
		".login-popup",
		".login-modal",
		"[data-testid=\"login-modal\"]",
		".signin-popup",
		".login-dialog",
	}

	for _, selector := range loginSelectors {
		elements, err := page.QuerySelectorAll(selector)
		if err == nil && len(elements) > 0 {
			logger.GetGlobalLogger("crawler/alibaba1688").Info("检测到登录提示，尝试关闭")

			// 尝试关闭登录弹窗
			closeSelectors := []string{
				selector + " .close",
				selector + " .close-btn",
				selector + " [data-testid='close']",
				selector + " .modal-close",
				selector + " .btn-close",
				selector + " .icon-close",
			}

			for _, closeSelector := range closeSelectors {
				closeBtn, err := page.QuerySelector(closeSelector)
				if err == nil && closeBtn != nil {
					if err := closeBtn.Click(); err == nil {
						logger.GetGlobalLogger("crawler/alibaba1688").Info("成功关闭登录弹窗")
						time.Sleep(1 * time.Second)
						return nil
					}
				}
			}

			// 如果找不到关闭按钮，尝试按ESC键
			if err := page.Keyboard().Press("Escape"); err == nil {
				logger.GetGlobalLogger("crawler/alibaba1688").Info("通过ESC键关闭登录弹窗")
				time.Sleep(1 * time.Second)
				return nil
			}
			break
		}
	}
	return nil
}

// handleOtherCaptcha 处理其他类型的验证码
func (ch *CaptchaHandler) handleOtherCaptcha(page playwright.Page) error {
	// 检查点击验证码
	clickCaptchaSelectors := []string{
		".captcha-click",
		"[class*='click-captcha']",
		".verify-click",
	}

	for _, selector := range clickCaptchaSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			isVisible, _ := element.IsVisible()
			if isVisible {
				logger.GetGlobalLogger("crawler/alibaba1688").Warn("检测到点击验证码，需要手动处理")
				return ch.waitForManualCaptcha(page, "点击验证码")
			}
		}
	}

	return nil
}

// waitForManualCaptcha 等待用户手动处理验证码
func (ch *CaptchaHandler) waitForManualCaptcha(page playwright.Page, captchaType string) error {
	logger.GetGlobalLogger("crawler/alibaba1688").Warnf("检测到%s，请手动完成验证", captchaType)
	logger.GetGlobalLogger("crawler/alibaba1688").Info("等待用户手动操作...")

	timeout := 120 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		// 检查页面标题是否变化
		title, _ := page.Title()
		if !strings.Contains(strings.ToLower(title), "验证") &&
			!strings.Contains(strings.ToLower(title), "captcha") &&
			!strings.Contains(strings.ToLower(title), "verify") {
			logger.GetGlobalLogger("crawler/alibaba1688").Info("页面标题已变化，验证可能已完成")
			return nil
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("等待用户手动处理%s超时", captchaType)
}
