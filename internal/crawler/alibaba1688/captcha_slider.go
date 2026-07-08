// Package alibaba1688 提供1688滑动验证码处理功能
package alibaba1688

import (
	"fmt"
	"math"
	"strings"
	"task-processor/internal/core/logger"
	"time"

	"github.com/mxschmitt/playwright-go"
)

// handleSliderCaptchaWithResult 处理滑动验证码并返回结果
func (ch *CaptchaHandler) handleSliderCaptchaWithResult(page playwright.Page) CaptchaResult {
	startTime := time.Now()
	attempts := 0
	maxRetries := ch.maxRetries

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
		sliderBtn, err := page.QuerySelector(selector)
		if err != nil || sliderBtn == nil {
			continue
		}

		isVisible, err := sliderBtn.IsVisible()
		if err != nil || !isVisible {
			continue
		}

		logger.GetGlobalLogger("crawler/alibaba1688").Info("检测到滑动验证码，使用人类行为策略滑动")

		for attempt := 1; attempt <= maxRetries; attempt++ {
			attempts++
			logger.GetGlobalLogger("crawler/alibaba1688").Infof("第 %d 次尝试人类行为滑动", attempt)

			if err := ch.performSliderAction(page, sliderBtn, "human"); err != nil {
				logger.GetGlobalLogger("crawler/alibaba1688").Warnf("人类行为滑动失败: %v", err)
			} else {
				time.Sleep(2 * time.Second)
				if ch.checkSliderSuccess(page) {
					logger.GetGlobalLogger("crawler/alibaba1688").Info("人类行为策略滑动验证码成功")
					
					if err := ch.waitForPageRedirect(page); err != nil {
						logger.GetGlobalLogger("crawler/alibaba1688").Warnf("等待页面跳转失败: %v", err)
					}
					
					return CaptchaResult{
						Type:       CaptchaTypeSlider,
						Status:     CaptchaStatusSuccess,
						Attempts:   attempts,
						UsedMethod: "human_behavior",
						Duration:   time.Since(startTime),
					}
				}
			}

			if attempt < maxRetries {
				logger.GetGlobalLogger("crawler/alibaba1688").Infof("第 %d 次滑动失败，刷新页面重试", attempt)
				if _, err := page.Reload(); err != nil {
					logger.GetGlobalLogger("crawler/alibaba1688").Warnf("刷新页面失败: %v", err)
				} else {
					time.Sleep(3 * time.Second)
					
					newSliderBtn, err := page.QuerySelector(selector)
					if err != nil || newSliderBtn == nil {
						logger.GetGlobalLogger("crawler/alibaba1688").Warn("刷新后未找到滑动按钮")
						continue
					}

					isVisible, err := newSliderBtn.IsVisible()
					if err != nil || !isVisible {
						logger.GetGlobalLogger("crawler/alibaba1688").Warn("刷新后滑动按钮不可见")
						continue
					}

					sliderBtn = newSliderBtn
					logger.GetGlobalLogger("crawler/alibaba1688").Info("页面刷新完成，准备重新尝试滑动")
				}
			}

			time.Sleep(time.Duration(1000+ch.randomDelay(1000)) * time.Millisecond)
		}

		logger.GetGlobalLogger("crawler/alibaba1688").Warn("人类行为滑动重试失败，等待用户手动操作")
		result := ch.waitForManualSliderWithResult(page)
		result.Attempts = attempts
		result.Type = CaptchaTypeSlider
		result.Duration = time.Since(startTime)
		return result
	}

	return CaptchaResult{
		Type:       CaptchaTypeSlider,
		Status:     CaptchaStatusFailed,
		Attempts:   attempts,
		UsedMethod: "none",
		Duration:   time.Since(startTime),
	}
}

// performSliderAction 执行滑动操作
func (ch *CaptchaHandler) performSliderAction(page playwright.Page, sliderBtn playwright.ElementHandle, strategy string) error {
	box, err := sliderBtn.BoundingBox()
	if err != nil {
		return fmt.Errorf("获取滑动按钮位置失败: %w", err)
	}
	if box == nil {
		return fmt.Errorf("滑动按钮位置信息为空")
	}

	slideDistance, err := ch.calculateSlideDistance(page, box)
	if err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Warnf("计算滑动距离失败，使用默认距离: %v", err)
		slideDistance = 260.0
	}

	logger.GetGlobalLogger("crawler/alibaba1688").Infof("开始滑动验证码，策略: %s, 滑动距离: %.2f", strategy, slideDistance)

	return ch.optimizedSlideWithHumanBehavior(page, box, slideDistance)
}

// calculateSlideDistance 计算滑动距离（优化版）
func (ch *CaptchaHandler) calculateSlideDistance(page playwright.Page, buttonBox *playwright.Rect) (float64, error) {
	if buttonBox == nil {
		return 0, fmt.Errorf("按钮位置信息为空")
	}

	trackSelectors := []string{
		".nc-lang-cnt",
		"#nc_1__scale_text",
		".nc_scale",
		".nc_wrapper",
		".slider-track",
		".captcha-slider-track",
		".slide-verify",
		"[class*='track']",
		"[class*='slider-container']",
		".geetest_slider_track",
		".geetest_canvas_bg",
	}

	for _, selector := range trackSelectors {
		track, err := page.QuerySelector(selector)
		if err != nil || track == nil {
			continue
		}

		trackBox, err := track.BoundingBox()
		if err != nil || trackBox == nil {
			continue
		}

		var distance float64
		if strings.Contains(selector, "nc-lang-cnt") || strings.Contains(selector, "nc_") {
			distance = trackBox.Width - buttonBox.Width + 5
		} else if strings.Contains(selector, "geetest") {
			distance = trackBox.Width - buttonBox.Width - 8
		} else {
			distance = trackBox.Width - buttonBox.Width - 15
		}

		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("找到轨道: %s, 轨道宽度: %.2f, 按钮宽度: %.2f, 计算距离: %.2f",
			selector, trackBox.Width, buttonBox.Width, distance)

		if distance > 50 && distance < 400 {
			return distance, nil
		}
	}

	return 0, fmt.Errorf("未找到合适的滑动轨道")
}

// checkSliderSuccess 检查滑动验证是否成功（增强版）
func (ch *CaptchaHandler) checkSliderSuccess(page playwright.Page) bool {
	successSelectors := []string{
		".nc-lang-cnt[data-nc-lang='_ddddd']",
		".slider-success",
		".captcha-success",
		"[class*='success']",
		".verify-success",
		".geetest_success",
		".geetest_success_popup",
		".nc_iconfont.nc_success",
	}

	for _, selector := range successSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			isVisible, _ := element.IsVisible()
			if isVisible {
				logger.GetGlobalLogger("crawler/alibaba1688").Info("检测到滑动验证成功标识")
				return true
			}
		}
	}

	captchaSelectors := []string{
		".nc_iconfont.btn_slide",
		".slider-button",
		".captcha-slider-button",
		".geetest_slider_button",
	}

	captchaExists := false
	for _, selector := range captchaSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			isVisible, _ := element.IsVisible()
			if isVisible {
				captchaExists = true
				break
			}
		}
	}

	if !captchaExists {
		title, err := page.Title()
		if err == nil && isProductPageReadyAfterCaptcha(title, false) {
			logger.GetGlobalLogger("crawler/alibaba1688").Info("滑动验证码已消失，验证成功")
			return true
		}
	}

	return false
}

func isProductPageReadyAfterCaptcha(title string, hasPageData bool) bool {
	if hasPageData {
		return true
	}
	title = strings.TrimSpace(strings.ToLower(title))
	return title != "" && !strings.Contains(title, "captcha")
}

// waitForManualSliderWithResult 等待用户手动完成滑动验证
func (ch *CaptchaHandler) waitForManualSliderWithResult(page playwright.Page) CaptchaResult {
	logger.GetGlobalLogger("crawler/alibaba1688").Warn("自动滑动失败，请手动完成滑动验证码")
	logger.GetGlobalLogger("crawler/alibaba1688").Info("程序将等待您手动操作，请在浏览器中完成滑动验证...")

	timeout := manualCaptchaTimeout
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		if ch.checkSliderSuccess(page) {
			logger.GetGlobalLogger("crawler/alibaba1688").Info("检测到验证码已完成，继续处理")
			return CaptchaResult{
				Type:   CaptchaTypeSlider,
				Status: CaptchaStatusSuccess,
				UsedMethod: "manual",
			}
		}
		time.Sleep(1 * time.Second)
	}

	return CaptchaResult{
		Type:   CaptchaTypeSlider,
		Status: CaptchaStatusManualRequired,
		Error:  fmt.Errorf("等待用户手动操作超时"),
		UsedMethod: "manual_timeout",
	}
}

// waitForPageRedirect 等待页面跳转到商品页面
func (ch *CaptchaHandler) waitForPageRedirect(page playwright.Page) error {
	logger.GetGlobalLogger("crawler/alibaba1688").Info("等待页面跳转到商品页面...")

	timeout := 30 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		title, err := page.Title()
		if err == nil && title != "Captcha Interception" && title != "" {
			logger.GetGlobalLogger("crawler/alibaba1688").Infof("页面已跳转，新标题: %s", title)

			time.Sleep(3 * time.Second)

			productSelectors := []string{
				"h1",
				".product-title",
				".offer-title",
				".main-content",
				".content",
			}

			for _, selector := range productSelectors {
				element, err := page.QuerySelector(selector)
				if err == nil && element != nil {
					isVisible, _ := element.IsVisible()
					if isVisible {
						logger.GetGlobalLogger("crawler/alibaba1688").Info("商品页面加载完成")
						return nil
					}
				}
			}
		}

		currentURL := page.URL()
		if strings.Contains(currentURL, "offer/") && strings.Contains(currentURL, ".html") {
			logger.GetGlobalLogger("crawler/alibaba1688").Infof("URL已跳转到商品页面: %s", currentURL)

			ready := false
			for i := 0; i < 10; i++ {
				time.Sleep(1 * time.Second)

				title, _ := page.Title()
				if isProductPageReadyAfterCaptcha(title, false) {
					logger.GetGlobalLogger("crawler/alibaba1688").Infof("页面标题已更新: %s", title)
					ready = true
					break
				}

				hasData, err := page.Evaluate(`() => {
					return (typeof window.__INIT_DATA !== 'undefined' && window.__INIT_DATA !== null) ||
						   (typeof window.context !== 'undefined' && window.context !== null);
				}`)
				if err == nil && hasData == true {
					logger.GetGlobalLogger("crawler/alibaba1688").Info("检测到页面数据已加载")
					ready = true
					break
				}

				if i == 4 {
					logger.GetGlobalLogger("crawler/alibaba1688").Info("页面数据未加载，尝试刷新页面")
					page.Reload()
					time.Sleep(3 * time.Second)
				}

				logger.GetGlobalLogger("crawler/alibaba1688").Debugf("等待页面数据加载... (%d/10)", i+1)
			}

			if !ready {
				return fmt.Errorf("验证码未放行，页面仍停留在拦截状态")
			}

			time.Sleep(2 * time.Second)
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("等待页面跳转超时")
}

// optimizedSlideWithHumanBehavior 优化的人类行为滑动（使用真实轨迹算法）
func (ch *CaptchaHandler) optimizedSlideWithHumanBehavior(page playwright.Page, box *playwright.Rect, slideDistance float64) error {
	return ch.optimizedSlideWithRealTrack(page, box, slideDistance)
}

// generateQuickDragPoints 生成快速滑动轨迹
func (ch *CaptchaHandler) generateQuickDragPoints(startX, startY, distance float64, numPoints int) []point {
	points := make([]point, numPoints)
	
	for i := 0; i < numPoints; i++ {
		t := float64(i) / float64(numPoints - 1)
		
		// 快速但自然的缓动曲线
		xProgress := ch.quickEaseFunction(t)
		
		// 添加少量随机性
		if t > 0.1 && t < 0.9 {
			xProgress += (float64(ch.randomDelay(60) - 30)) / 1000.0
		}
		xProgress = math.Max(0, math.Min(1, xProgress))
		
		x := startX + distance * xProgress
		
		// 垂直方向有小幅度抖动
		yWobble := math.Sin(t * math.Pi * 3) * float64(2 + ch.randomDelay(2))
		yWobble += float64(ch.randomDelay(6) - 3)
		
		y := startY + yWobble
		
		points[i] = point{x, y}
	}
	
	return points
}

// quickEaseFunction 快速滑动的缓动函数
func (ch *CaptchaHandler) quickEaseFunction(t float64) float64 {
	// 快速启动，中间快，结尾稍微减速
	if t < 0.1 {
		return 3 * t * t * t
	} else if t < 0.85 {
		return 0.03 + 0.94 * ((t - 0.1) / 0.75)
	} else {
		remaining := 1 - t
		return 1 - remaining * remaining * remaining
	}
}

// generateHumanDragPoints 生成更真实的人类拖拽路径
func (ch *CaptchaHandler) generateHumanDragPoints(startX, startY, distance float64, numPoints int) []point {
	points := make([]point, numPoints)
	
	for i := 0; i < numPoints; i++ {
		t := float64(i) / float64(numPoints - 1)
		
		// 使用改进的缓动曲线
		xProgress := ch.humanEaseFunction(t)
		
		// 加入轨迹随机性
		randomness := (float64(ch.randomDelay(200)) - 100) / 1000.0
		if t > 0.1 && t < 0.9 {
			xProgress += randomness
		}
		xProgress = math.Max(0, math.Min(1, xProgress))
		
		x := startX + distance * xProgress
		
		// 垂直方向的随机移动，模拟人手在滑动时的上下抖动
		verticalWobble := math.Sin(t * math.Pi * 2) * 3.0    // 主要正弦波动
		verticalWobble += math.Sin(t * math.Pi * 5) * 1.5   // 高频小波动
		verticalWobble += float64(ch.randomDelay(8) - 4)     // 纯随机
		
		// 在滑动前半段加入更大的随机变化
		if t < 0.4 {
			verticalWobble *= 1.5
		}
		
		y := startY + verticalWobble
		
		points[i] = point{x, y}
	}
	
	return points
}

// humanEaseFunction 更符合人类操作的缓动函数
func (ch *CaptchaHandler) humanEaseFunction(t float64) float64 {
	// 人类滑动特点：开始慢，中间加速，结尾减速
	if t < 0.15 {
		// 起始阶段：慢启动
		return 0.3 * t * t * t + 0.7 * t * t
	} else if t < 0.7 {
		// 中间阶段：快速移动
		return 0.5 * math.Pow(2*t - 0.3, 2) + 0.12
	} else {
		// 结束阶段：减速接近终点
		return 1 - 0.5 * math.Pow(2*(1-t)-0.3, 2)
	}
}

// calculateHumanStepDelay 计算更符合人类操作的每步延迟
func (ch *CaptchaHandler) calculateHumanStepDelay(progress float64, totalDuration float64, steps int) int {
	baseDelay := totalDuration / float64(steps)
	
	// 根据进度调整延迟
	var multiplier float64
	if progress < 0.15 {
		// 开始阶段：较慢的移动
		multiplier = 2.0 + float64(ch.randomDelay(100))/100.0
	} else if progress < 0.4 {
		// 加速阶段：较快移动
		multiplier = 0.6 + float64(ch.randomDelay(50))/100.0
	} else if progress < 0.75 {
		// 快速阶段：最快
		multiplier = 0.4 + float64(ch.randomDelay(40))/100.0
	} else if progress < 0.9 {
		// 减速阶段
		multiplier = 0.8 + float64(ch.randomDelay(60))/100.0
	} else {
		// 接近终点：再次放慢
		multiplier = 1.8 + float64(ch.randomDelay(100))/100.0
	}
	
	// 添加额外的随机
	return int(baseDelay * multiplier)
}

// calculateVerticalWobble 计算垂直抖动
func (ch *CaptchaHandler) calculateVerticalWobble(progress, i, steps float64) float64 {
	wobble1 := math.Sin(progress*math.Pi*4+float64(ch.randomDelay(500))/100) * 2.5
	wobble2 := math.Sin(progress*math.Pi*7+float64(ch.randomDelay(300))/50) * 1.5
	return wobble1 + wobble2
}

// calculateStepDuration 计算每步的延迟
func (ch *CaptchaHandler) calculateStepDuration(progress float64, totalDuration, steps float64) int {
	baseDelay := totalDuration / steps
	
	if progress < 0.1 {
		return int(baseDelay * (1.5 + float64(ch.randomDelay(100))/100))
	} else if progress < 0.2 {
		return int(baseDelay * (1.2 + float64(ch.randomDelay(80))/100))
	} else if progress < 0.6 {
		return int(baseDelay * (0.7 + float64(ch.randomDelay(60))/100))
	} else if progress < 0.85 {
		return int(baseDelay * (1.1 + float64(ch.randomDelay(80))/100))
	} else {
		return int(baseDelay * (2.0 + float64(ch.randomDelay(100))/100))
	}
}

// complexEasingWithVariation 带随机变化的复杂缓动函数
func (ch *CaptchaHandler) complexEasingWithVariation(t float64) float64 {
	baseEasing := ch.complexEasing(t)
	variation := (float64(ch.randomDelay(20)) - 10) / 100

	if t > 0.1 && t < 0.9 {
		baseEasing += variation * 0.05
	}

	return math.Max(0, math.Min(1, baseEasing))
}
