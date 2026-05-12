// Package alibaba1688 提供1688验证码处理功能
package alibaba1688

import (
	"fmt"
	"strings"
	"task-processor/internal/core/logger"
	"time"

	"github.com/playwright-community/playwright-go"
)

const manualCaptchaTimeout = 5 * time.Minute

// DetectCaptchaType 检测验证码类型
func (ch *CaptchaHandler) DetectCaptchaType(page playwright.Page) CaptchaType {
	// 检查滑动验证码
	if ch.hasSliderCaptcha(page) {
		return CaptchaTypeSlider
	}

	// 检查点击验证码
	if ch.hasClickCaptcha(page) {
		return CaptchaTypeClick
	}

	// 检查图片验证码
	if ch.hasImageCaptcha(page) {
		return CaptchaTypeImage
	}

	// 检查文字验证码
	if ch.hasTextCaptcha(page) {
		return CaptchaTypeText
	}

	// 检查数学验证码
	if ch.hasMathCaptcha(page) {
		return CaptchaTypeMath
	}

	return CaptchaTypeUnknown
}

// hasSliderCaptcha 检查是否存在滑动验证码
func (ch *CaptchaHandler) hasSliderCaptcha(page playwright.Page) bool {
	sliderSelectors := []string{
		".nc_iconfont.btn_slide",
		"#nc_1_n1z",
		".nc-container .nc_iconfont",
		".slider-button",
		".captcha-slider-button",
		"[class*='slider'][class*='button']",
		".verify-slider-track .verify-slider-button",
		".slide-verify .slide-verify-slider-mask-item",
		"span.btn_slide",
		".geetest_slider_button",
		".geetest_slider",
		".captcha-slider-wrap .slider-btn",
	}

	for _, selector := range sliderSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			isVisible, _ := element.IsVisible()
			if isVisible {
				return true
			}
		}
	}
	return false
}

// hasClickCaptcha 检查是否存在点击验证码
func (ch *CaptchaHandler) hasClickCaptcha(page playwright.Page) bool {
	clickSelectors := []string{
		".captcha-click",
		"[class*='click-captcha']",
		".verify-click",
		".geetest_item_img",
		".captcha-check-item",
		".click-target",
		".img-captcha-item",
	}

	for _, selector := range clickSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			isVisible, _ := element.IsVisible()
			if isVisible {
				return true
			}
		}
	}
	return false
}

// hasImageCaptcha 检查是否存在图片验证码
func (ch *CaptchaHandler) hasImageCaptcha(page playwright.Page) bool {
	imageSelectors := []string{
		"img[src*='captcha']",
		"img[src*='verify']",
		".captcha-image",
		".verify-code-img",
		".img-code",
		"#captcha_img",
		".geetest_widget",
	}

	for _, selector := range imageSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			isVisible, _ := element.IsVisible()
			if isVisible {
				return true
			}
		}
	}
	return false
}

// hasTextCaptcha 检查是否存在文字验证码输入框
func (ch *CaptchaHandler) hasTextCaptcha(page playwright.Page) bool {
	textSelectors := []string{
		"input[name*='captcha']",
		"input[name*='verify']",
		"input[placeholder*='验证码']",
		"input[placeholder*='captcha']",
		"#captcha",
		".captcha-input",
		".verify-input",
	}

	for _, selector := range textSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			isVisible, _ := element.IsVisible()
			if isVisible {
				return true
			}
		}
	}
	return false
}

// hasMathCaptcha 检查是否存在数学验证码
func (ch *CaptchaHandler) hasMathCaptcha(page playwright.Page) bool {
	mathSelectors := []string{
		"[class*='math']",
		"[class*='calculate']",
		"[class*='compute']",
	}

	for _, selector := range mathSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			isVisible, _ := element.IsVisible()
			if isVisible {
				return true
			}
		}
	}
	return false
}

// HandlePageCaptcha 处理页面中的各种验证码
func (ch *CaptchaHandler) HandlePageCaptcha(page playwright.Page) error {
	startTime := time.Now()
	logger.GetGlobalLogger("crawler/alibaba1688").Debugf("开始处理页面验证码")

	time.Sleep(2 * time.Second)

	currentURL := page.URL()
	title, _ := page.Title()

	logger.GetGlobalLogger("crawler/alibaba1688").Debugf("当前页面URL: %s, 标题: %s", currentURL, title)

	if strings.Contains(strings.ToLower(title), "captcha") ||
		strings.Contains(strings.ToLower(title), "验证") ||
		strings.Contains(currentURL, "captcha") {
		logger.GetGlobalLogger("crawler/alibaba1688").Info("检测到验证码拦截页面")
	}

	if err := ch.handleLoginPrompt(page); err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Warnf("处理登录提示失败: %v", err)
	}

	captchaType := ch.DetectCaptchaType(page)
	logger.GetGlobalLogger("crawler/alibaba1688").Infof("检测到验证码类型: %s", captchaType.String())

	var result CaptchaResult

	switch captchaType {
	case CaptchaTypeSlider:
		result = ch.handleSliderCaptchaWithResult(page)
	case CaptchaTypeClick:
		result = ch.handleClickCaptchaWithResult(page)
	case CaptchaTypeImage:
		result = ch.handleImageCaptchaWithResult(page)
	case CaptchaTypeText:
		result = ch.handleTextCaptchaWithResult(page)
	case CaptchaTypeMath:
		result = ch.handleMathCaptchaWithResult(page)
	case CaptchaTypeUnknown:
		logger.GetGlobalLogger("crawler/alibaba1688").Info("未检测到验证码")
		return nil
	}

	result.Duration = time.Since(startTime)
	logger.GetGlobalLogger("crawler/alibaba1688").Infof("验证码处理完成: 类型=%s, 状态=%d, 尝试次数=%d, 耗时=%v",
		captchaType.String(), result.Status, result.Attempts, result.Duration)

	if result.Status == CaptchaStatusSuccess {
		ch.recordSuccess(captchaType)
	} else if result.Status == CaptchaStatusManualRequired {
		ch.recordManual(captchaType)
	} else {
		ch.recordFailure(captchaType)
	}

	return result.Error
}

// checkAndHandleCaptcha 检查并处理验证码，包括鼠标轨迹录制
func (ch *CaptchaHandler) checkAndHandleCaptcha(page playwright.Page) error {
	// 检查是否需要处理验证码
	title, err := page.Title()
	if err != nil {
		return nil
	}

	titleLower := strings.ToLower(title)
	if strings.Contains(titleLower, "验证码") || strings.Contains(titleLower, "captcha") {
		logger.GetGlobalLogger("crawler/alibaba1688").Info("检测到验证码拦截页面")

		// 检查滑块元素
		captchaType := ch.DetectCaptchaType(page)
		if captchaType == CaptchaTypeSlider {
			logger.GetGlobalLogger("crawler/alibaba1688").Info("是否录制鼠标轨迹? (y/n)")
			logger.GetGlobalLogger("crawler/alibaba1688").Info("提示: 录制一次后可自动回放")

			// 尝试读取用户输入 (非阻塞)
			var input string
			fmt.Scanln(&input)

			if strings.ToLower(input) == "y" || strings.ToLower(input) == "yes" {
				logger.GetGlobalLogger("crawler/alibaba1688").Info("开始录制鼠标轨迹...")
				// 录制功能将在后续实现
			}
		}
	}
	return nil
}

// HandleCaptchaWithRetry 带重试的验证码处理
func (ch *CaptchaHandler) HandleCaptchaWithRetry(page playwright.Page, maxRetries int) error {
	for attempt := 0; attempt < maxRetries; attempt++ {
		logger.GetGlobalLogger("crawler/alibaba1688").Infof("验证码处理尝试 %d/%d", attempt+1, maxRetries)

		err := ch.HandlePageCaptcha(page)
		if err == nil {
			return nil
		}

		if attempt < maxRetries-1 {
			logger.GetGlobalLogger("crawler/alibaba1688").Infof("第 %d 次尝试失败，等待后重试", attempt+1)

			if _, err := page.Reload(); err != nil {
				logger.GetGlobalLogger("crawler/alibaba1688").Warnf("刷新页面失败: %v", err)
			} else {
				time.Sleep(3 * time.Second)
			}
		}
	}

	return nil
}

// handleSliderCaptchaWithRecording 处理滑动验证码（支持录制）
func (ch *CaptchaHandler) handleSliderCaptchaWithRecording(page playwright.Page, box *playwright.Rect, slideDistance float64) error {
	recorder := globalRecorder

	// 如果有保存的轨迹，尝试使用
	if recorder.HasTrack() {
		logger.GetGlobalLogger("crawler/alibaba1688").Info("找到已保存的轨迹，尝试使用...")
		if err := recorder.replayTrack(page, box, recorder.GetSavedTrack()); err != nil {
			logger.GetGlobalLogger("crawler/alibaba1688").Warnf("回放轨迹失败: %v", err)
		} else {
			time.Sleep(2 * time.Second)
			if !ch.hasSliderCaptcha(page) {
				logger.GetGlobalLogger("crawler/alibaba1688").Info("轨迹回放成功！")
				return nil
			}
			logger.GetGlobalLogger("crawler/alibaba1688").Warnf("轨迹回放后仍检测到验证码")
		}
	}

	// 录制新轨迹
	logger.GetGlobalLogger("crawler/alibaba1688").Info("准备录制新轨迹...")
	logger.GetGlobalLogger("crawler/alibaba1688").Info("请在弹出的浏览器中手动滑动验证码")

	if err := recorder.RecordAndPlay(page, box, 3); err != nil {
		return fmt.Errorf("录制失败: %w", err)
	}

	time.Sleep(2 * time.Second)
	if !ch.hasSliderCaptcha(page) {
		logger.GetGlobalLogger("crawler/alibaba1688").Info("录制回放成功！")
		return nil
	}

	return fmt.Errorf("录制回放后仍检测到验证码")
}
