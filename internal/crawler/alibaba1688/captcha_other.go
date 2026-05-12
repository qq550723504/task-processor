// Package alibaba1688 提供其他类型验证码处理功能
package alibaba1688

import (
	"fmt"
	"regexp"
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
		".modal-login",
		".am-login-modal",
	}

	for _, selector := range loginSelectors {
		elements, err := page.QuerySelectorAll(selector)
		if err == nil && len(elements) > 0 {
			logger.GetGlobalLogger("crawler/alibaba1688").Info("检测到登录提示，尝试关闭")

			closeSelectors := []string{
				selector + " .close",
				selector + " .close-btn",
				selector + " [data-testid='close']",
				selector + " .modal-close",
				selector + " .btn-close",
				selector + " .icon-close",
				selector + " .popup-close",
				selector + " button[aria-label*='close']",
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

// handleClickCaptchaWithResult 处理点击验证码并返回结果
func (ch *CaptchaHandler) handleClickCaptchaWithResult(page playwright.Page) CaptchaResult {
	logger.GetGlobalLogger("crawler/alibaba1688").Warn("检测到点击验证码，需要手动处理")
	return ch.waitForManualCaptchaWithResult(page, CaptchaTypeClick, "点击验证码")
}

// handleImageCaptchaWithResult 处理图片验证码并返回结果
func (ch *CaptchaHandler) handleImageCaptchaWithResult(page playwright.Page) CaptchaResult {
	startTime := time.Now()
	
	if result := ch.tryOCRCaptcha(page); result.Status == CaptchaStatusSuccess {
		result.Duration = time.Since(startTime)
		return result
	}

	logger.GetGlobalLogger("crawler/alibaba1688").Warn("图片验证码OCR识别失败，需要手动处理")
	return ch.waitForManualCaptchaWithResult(page, CaptchaTypeImage, "图片验证码")
}

// handleTextCaptchaWithResult 处理文字验证码并返回结果
func (ch *CaptchaHandler) handleTextCaptchaWithResult(page playwright.Page) CaptchaResult {
	startTime := time.Now()
	
	if result := ch.tryTextCaptcha(page); result.Status == CaptchaStatusSuccess {
		result.Duration = time.Since(startTime)
		return result
	}

	logger.GetGlobalLogger("crawler/alibaba1688").Warn("文字验证码自动填写失败，需要手动处理")
	return ch.waitForManualCaptchaWithResult(page, CaptchaTypeText, "文字验证码")
}

// handleMathCaptchaWithResult 处理数学验证码并返回结果
func (ch *CaptchaHandler) handleMathCaptchaWithResult(page playwright.Page) CaptchaResult {
	startTime := time.Now()
	
	if result := ch.solveMathCaptcha(page); result.Status == CaptchaStatusSuccess {
		result.Duration = time.Since(startTime)
		return result
	}

	logger.GetGlobalLogger("crawler/alibaba1688").Warn("数学验证码求解失败，需要手动处理")
	return ch.waitForManualCaptchaWithResult(page, CaptchaTypeMath, "数学验证码")
}

// tryOCRCaptcha 尝试使用OCR识别图片验证码
func (ch *CaptchaHandler) tryOCRCaptcha(page playwright.Page) CaptchaResult {
	captchaImageSelectors := []string{
		"img[src*='captcha']",
		"img[src*='verify']",
		".captcha-image",
		".verify-code-img",
		".img-code",
		"#captcha_img",
	}

	for _, selector := range captchaImageSelectors {
		element, err := page.QuerySelector(selector)
		if err != nil || element == nil {
			continue
		}

		isVisible, _ := element.IsVisible()
		if !isVisible {
			continue
		}

		src, err := element.GetAttribute("src")
		if err != nil || src == "" {
			continue
		}

		logger.GetGlobalLogger("crawler/alibaba1688").Infof("找到验证码图片: %s", src)
		
		if strings.HasPrefix(src, "data:") {
			logger.GetGlobalLogger("crawler/alibaba1688").Info("检测到base64验证码图片")
		}

		inputSelectors := []string{
			"input[name*='captcha']",
			"input[name*='verify']",
			"input[placeholder*='验证码']",
			"#captcha",
			".captcha-input",
		}

		for _, inputSelector := range inputSelectors {
			input, err := page.QuerySelector(inputSelector)
			if err != nil || input == nil {
				continue
			}

			isVisible, _ := input.IsVisible()
			if !isVisible {
				continue
			}

			if recognizedCode := ch.recognizeCaptchaCode(page, element); recognizedCode != "" {
				logger.GetGlobalLogger("crawler/alibaba1688").Infof("OCR识别结果: %s", recognizedCode)
				
				if err := input.Fill(recognizedCode); err != nil {
					logger.GetGlobalLogger("crawler/alibaba1688").Warnf("填写验证码失败: %v", err)
					continue
				}

				time.Sleep(500 * time.Millisecond)

				submitSelectors := []string{
					"button[type='submit']",
					".btn-submit",
					".submit-btn",
					"[class*='submit']",
				}

				for _, submitSelector := range submitSelectors {
					submitBtn, err := page.QuerySelector(submitSelector)
					if err == nil && submitBtn != nil {
						isVisible, _ := submitBtn.IsVisible()
						if isVisible {
							if err := submitBtn.Click(); err == nil {
								time.Sleep(2 * time.Second)
								if !ch.hasImageCaptcha(page) {
									return CaptchaResult{
										Type:       CaptchaTypeImage,
										Status:     CaptchaStatusSuccess,
										UsedMethod: "ocr_recognition",
									}
								}
							}
							break
						}
					}
				}
			}
			break
		}
	}

	return CaptchaResult{
		Type:   CaptchaTypeImage,
		Status: CaptchaStatusFailed,
		Error:  fmt.Errorf("OCR识别失败"),
	}
}

// recognizeCaptchaCode 识别验证码（简化版本，实际项目中应集成专业OCR服务）
func (ch *CaptchaHandler) recognizeCaptchaCode(page playwright.Page, imageElement playwright.ElementHandle) string {
	return ""
}

// tryTextCaptcha 尝试处理文字验证码
func (ch *CaptchaHandler) tryTextCaptcha(page playwright.Page) CaptchaResult {
	inputSelectors := []string{
		"input[name*='captcha']",
		"input[name*='verify']",
		"input[placeholder*='验证码']",
		"#captcha",
		".captcha-input",
	}

	for _, selector := range inputSelectors {
		input, err := page.QuerySelector(selector)
		if err != nil || input == nil {
			continue
		}

		isVisible, _ := input.IsVisible()
		if !isVisible {
			continue
		}

		placeholder, _ := input.GetAttribute("placeholder")
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("验证码输入框placeholder: %s", placeholder)

		if placeholder != "" && strings.Contains(strings.ToLower(placeholder), "请输入") {
			return CaptchaResult{
				Type:   CaptchaTypeText,
				Status: CaptchaStatusFailed,
				Error:  fmt.Errorf("无法自动识别文字验证码"),
			}
		}
	}

	return CaptchaResult{
		Type:   CaptchaTypeText,
		Status: CaptchaStatusFailed,
		Error:  fmt.Errorf("未找到验证码输入框"),
	}
}

// solveMathCaptcha 尝试解决数学验证码
func (ch *CaptchaHandler) solveMathCaptcha(page playwright.Page) CaptchaResult {
	mathSelectors := []string{
		".math-captcha",
		".calculate-code",
		".compute-code",
		"[class*='math']",
		"[class*='calculate']",
	}

	for _, selector := range mathSelectors {
		element, err := page.QuerySelector(selector)
		if err != nil || element == nil {
			continue
		}

		text, err := element.TextContent()
		if err != nil || text == "" {
			continue
		}

		logger.GetGlobalLogger("crawler/alibaba1688").Infof("检测到数学表达式: %s", text)

		if result := ch.calculateMathExpression(text); result != "" {
			inputSelectors := []string{
				"input[name*='captcha']",
				"input[name*='verify']",
				"input[class*='math']",
				".captcha-input",
			}

			for _, inputSelector := range inputSelectors {
				input, err := page.QuerySelector(inputSelector)
				if err == nil && input != nil {
					isVisible, _ := input.IsVisible()
					if isVisible {
						if err := input.Fill(result); err == nil {
							time.Sleep(500 * time.Millisecond)

							submitSelectors := []string{
								"button[type='submit']",
								".btn-submit",
								".submit-btn",
							}

							for _, submitSelector := range submitSelectors {
								submitBtn, err := page.QuerySelector(submitSelector)
								if err == nil && submitBtn != nil {
									isVisible, _ := submitBtn.IsVisible()
									if isVisible {
										if err := submitBtn.Click(); err == nil {
											time.Sleep(2 * time.Second)
											if !ch.hasMathCaptcha(page) {
												return CaptchaResult{
													Type:       CaptchaTypeMath,
													Status:     CaptchaStatusSuccess,
													UsedMethod: "math_solver",
												}
											}
										}
										break
									}
								}
							}
						}
						break
					}
				}
			}
		}
	}

	return CaptchaResult{
		Type:   CaptchaTypeMath,
		Status: CaptchaStatusFailed,
		Error:  fmt.Errorf("无法解析数学表达式"),
	}
}

// calculateMathExpression 计算数学表达式
func (ch *CaptchaHandler) calculateMathExpression(expr string) string {
	re := regexp.MustCompile(`(\d+)\s*([+\-*/])\s*(\d+)`)
	matches := re.FindStringSubmatch(expr)

	if len(matches) == 4 {
		num1 := parseInt(matches[1])
		num2 := parseInt(matches[3])
		operator := matches[2]

		var result int
		switch operator {
		case "+":
			result = num1 + num2
		case "-":
			result = num1 - num2
		case "*":
			result = num1 * num2
		case "/":
			if num2 != 0 {
				result = num1 / num2
			}
		}

		return fmt.Sprintf("%d", result)
	}

	return ""
}

func parseInt(s string) int {
	result := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

// waitForManualCaptchaWithResult 等待用户手动处理验证码并返回结果
func (ch *CaptchaHandler) waitForManualCaptchaWithResult(page playwright.Page, captchaType CaptchaType, captchaName string) CaptchaResult {
	logger.GetGlobalLogger("crawler/alibaba1688").Warnf("检测到%s，请手动完成验证", captchaName)
	logger.GetGlobalLogger("crawler/alibaba1688").Info("等待用户手动操作...")

	timeout := manualCaptchaTimeout
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		title, _ := page.Title()
		if !strings.Contains(strings.ToLower(title), "验证") &&
			!strings.Contains(strings.ToLower(title), "captcha") &&
			!strings.Contains(strings.ToLower(title), "verify") {
			logger.GetGlobalLogger("crawler/alibaba1688").Info("页面标题已变化，验证可能已完成")
			return CaptchaResult{
				Type:       captchaType,
				Status:     CaptchaStatusSuccess,
				UsedMethod: "manual",
			}
		}

		time.Sleep(2 * time.Second)
	}

	return CaptchaResult{
		Type:       captchaType,
		Status:     CaptchaStatusManualRequired,
		Error:      fmt.Errorf("等待用户手动处理%s超时", captchaName),
		UsedMethod: "manual_timeout",
	}
}
