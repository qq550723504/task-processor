// Package alibaba1688 提供1688滑动验证码处理功能
package alibaba1688

import (
	"task-processor/internal/core/logger"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// handleSliderCaptcha 处理滑动验证码
func (ch *CaptchaHandler) handleSliderCaptcha(page playwright.Page) error {
	// 常见的滑动验证码选择器（包含新版验证码）
	sliderSelectors := []string{
		".nc_iconfont.btn_slide",                       // 阿里系滑动验证码（旧版）
		"#nc_1_n1z",                                    // 阿里系滑动验证码（新版ID）
		".nc-container .nc_iconfont",                   // 阿里系容器内的滑块
		".slider-button",                               // 通用滑动按钮
		".captcha-slider-button",                       // 验证码滑动按钮
		"[class*='slider'][class*='button']",           // 包含slider和button的类名
		".verify-slider-track .verify-slider-button",   // 另一种滑动验证码
		".slide-verify .slide-verify-slider-mask-item", // 其他滑动验证码
		"span.btn_slide",                               // 新版span标签滑块
	}

	for _, selector := range sliderSelectors {
		sliderBtn, err := page.QuerySelector(selector)
		if err != nil || sliderBtn == nil {
			continue
		}

		// 检查滑动按钮是否可见
		isVisible, err := sliderBtn.IsVisible()
		if err != nil || !isVisible {
			continue
		}

		logger.GetGlobalLogger("crawler/alibaba1688").Info("检测到滑动验证码，使用人类行为策略滑动")

		// 只使用人类行为策略，最多重试3次
		maxRetries := 3
		for attempt := 1; attempt <= maxRetries; attempt++ {
			logger.GetGlobalLogger("crawler/alibaba1688").Infof("第 %d 次尝试人类行为滑动", attempt)

			if err := ch.performSliderAction(page, sliderBtn, "human"); err != nil {
				logger.GetGlobalLogger("crawler/alibaba1688").Warnf("人类行为滑动失败: %v", err)
			} else {
				// 检查是否成功
				if ch.checkSliderSuccess(page) {
					logger.GetGlobalLogger("crawler/alibaba1688").Info("人类行为策略滑动验证码成功")
					// 等待页面跳转到商品页面
					if err := ch.waitForPageRedirect(page); err != nil {
						logger.GetGlobalLogger("crawler/alibaba1688").Warnf("等待页面跳转失败: %v", err)
					}
					return nil
				}
			}

			// 如果不是最后一次尝试，刷新页面重试
			if attempt < maxRetries {
				logger.GetGlobalLogger("crawler/alibaba1688").Infof("第 %d 次滑动失败，刷新页面重试", attempt)
				_, err := page.Reload()
				if err != nil {
					logger.GetGlobalLogger("crawler/alibaba1688").Warnf("刷新页面失败: %v", err)
				} else {
					// 等待页面加载
					time.Sleep(3 * time.Second)

					// 重新查找滑动按钮
					newSliderBtn, err := page.QuerySelector(selector)
					if err != nil || newSliderBtn == nil {
						logger.GetGlobalLogger("crawler/alibaba1688").Warn("刷新后未找到滑动按钮")
						continue
					}

					// 检查新按钮是否可见
					isVisible, err := newSliderBtn.IsVisible()
					if err != nil || !isVisible {
						logger.GetGlobalLogger("crawler/alibaba1688").Warn("刷新后滑动按钮不可见")
						continue
					}

					sliderBtn = newSliderBtn
					logger.GetGlobalLogger("crawler/alibaba1688").Info("页面刷新完成，准备重新尝试滑动")
				}
			}

			// 等待一下再尝试
			time.Sleep(2 * time.Second)
		}

		// 所有重试都失败
		logger.GetGlobalLogger("crawler/alibaba1688").Warn("人类行为滑动重试失败，等待用户手动操作")
		return ch.waitForManualSlider(page)
	}

	return nil
}

// performSliderAction 执行滑动操作（仅支持人类行为策略）
func (ch *CaptchaHandler) performSliderAction(page playwright.Page, sliderBtn playwright.ElementHandle, strategy string) error {
	// 获取滑动按钮的位置和大小
	box, err := sliderBtn.BoundingBox()
	if err != nil {
		return fmt.Errorf("获取滑动按钮位置失败: %w", err)
	}
	if box == nil {
		return fmt.Errorf("滑动按钮位置信息为空")
	}

	// 获取滑动轨道的宽度
	slideDistance, err := ch.calculateSlideDistance(page, box)
	if err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Warnf("计算滑动距离失败，使用默认距离: %v", err)
		slideDistance = 260.0 // 使用经过验证的默认距离
	}

	logger.GetGlobalLogger("crawler/alibaba1688").Infof("开始滑动验证码，策略: %s, 滑动距离: %.2f", strategy, slideDistance)

	// 只支持人类行为策略
	return ch.optimizedSlideWithHumanBehavior(page, box, slideDistance)
}

// calculateSlideDistance 计算滑动距离
func (ch *CaptchaHandler) calculateSlideDistance(page playwright.Page, buttonBox *playwright.Rect) (float64, error) {
	if buttonBox == nil {
		return 0, fmt.Errorf("按钮位置信息为空")
	}

	// 尝试多种轨道选择器（包含新版验证码）
	trackSelectors := []string{
		".nc-lang-cnt",      // 阿里系主要轨道
		"#nc_1__scale_text", // 新版轨道文本容器
		".nc_scale",         // 阿里系轨道容器
		".nc_wrapper",       // 阿里系包装器
		".slider-track",
		".captcha-slider-track",
		".slide-verify",
		"[class*='track']",
		"[class*='slider-container']",
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

		// 计算滑动距离，针对阿里系验证码优化
		var distance float64
		if strings.Contains(selector, "nc-lang-cnt") || strings.Contains(selector, "nc_") {
			// 阿里系验证码：需要滑动到轨道最右边
			// 根据实际测试和Chrome DevTools测量，需要滑动到轨道宽度减去按钮宽度
			// 新版验证码可能需要更精确的距离
			distance = trackBox.Width - buttonBox.Width + 5 // 稍微超出一点，确保滑动到底
		} else {
			// 其他验证码：滑动到轨道末端减去按钮宽度和余量
			distance = trackBox.Width - buttonBox.Width - 15
		}

		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("找到轨道: %s, 轨道宽度: %.2f, 按钮宽度: %.2f, 计算距离: %.2f",
			selector, trackBox.Width, buttonBox.Width, distance)

		if distance > 50 && distance < 400 { // 合理的滑动距离范围
			return distance, nil
		}
	}

	return 0, fmt.Errorf("未找到合适的滑动轨道")
}

// checkSliderSuccess 检查滑动验证是否成功
func (ch *CaptchaHandler) checkSliderSuccess(page playwright.Page) bool {
	// 检查成功标识
	successSelectors := []string{
		".nc-lang-cnt[data-nc-lang='_ddddd']", // 阿里系成功标识
		".slider-success",
		".captcha-success",
		"[class*='success']",
		".verify-success",
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

	// 检查是否还有验证码存在
	captchaSelectors := []string{
		".nc_iconfont.btn_slide",
		".slider-button",
		".captcha-slider-button",
	}

	for _, selector := range captchaSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			isVisible, _ := element.IsVisible()
			if isVisible {
				return false // 验证码仍然存在
			}
		}
	}

	logger.GetGlobalLogger("crawler/alibaba1688").Info("滑动验证码已消失，可能验证成功")
	return true
}

// waitForManualSlider 等待用户手动完成滑动验证
func (ch *CaptchaHandler) waitForManualSlider(page playwright.Page) error {
	logger.GetGlobalLogger("crawler/alibaba1688").Warn("自动滑动失败，请手动完成滑动验证码")
	logger.GetGlobalLogger("crawler/alibaba1688").Info("程序将等待您手动操作，请在浏览器中完成滑动验证...")

	timeout := 90 * time.Second // 增加到90秒
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		// 检查验证码是否还存在
		if ch.checkSliderSuccess(page) {
			logger.GetGlobalLogger("crawler/alibaba1688").Info("检测到验证码已完成，继续处理")
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("等待用户手动操作超时")
}

// waitForPageRedirect 等待页面跳转到商品页面
func (ch *CaptchaHandler) waitForPageRedirect(page playwright.Page) error {
	logger.GetGlobalLogger("crawler/alibaba1688").Info("等待页面跳转到商品页面...")

	timeout := 30 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		// 检查页面标题是否已经改变
		title, err := page.Title()
		if err == nil && title != "Captcha Interception" && title != "" {
			logger.GetGlobalLogger("crawler/alibaba1688").Infof("页面已跳转，新标题: %s", title)

			// 等待页面完全加载
			time.Sleep(3 * time.Second)

			// 检查是否有商品相关的元素
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

		// 检查URL是否包含商品ID
		currentURL := page.URL()
		if strings.Contains(currentURL, "offer/") && strings.Contains(currentURL, ".html") {
			logger.GetGlobalLogger("crawler/alibaba1688").Infof("URL已跳转到商品页面: %s", currentURL)

			// 等待页面完全加载，并检查数据是否存在
			for i := 0; i < 10; i++ { // 最多等待10秒
				time.Sleep(1 * time.Second)

				// 检查页面标题是否更新
				title, _ := page.Title()
				if title != "Captcha Interception" && title != "" {
					logger.GetGlobalLogger("crawler/alibaba1688").Infof("页面标题已更新: %s", title)
					break
				}

				// 检查是否有JavaScript数据
				hasData, err := page.Evaluate(`() => {
					return (typeof window.__INIT_DATA !== 'undefined' && window.__INIT_DATA !== null) ||
						   (typeof window.context !== 'undefined' && window.context !== null);
				}`)
				if err == nil && hasData == true {
					logger.GetGlobalLogger("crawler/alibaba1688").Info("检测到页面数据已加载")
					break
				}

				// 如果等待了5秒还没有数据，尝试刷新页面
				if i == 4 {
					logger.GetGlobalLogger("crawler/alibaba1688").Info("页面数据未加载，尝试刷新页面")
					page.Reload()
					time.Sleep(3 * time.Second)
				}

				logger.GetGlobalLogger("crawler/alibaba1688").Debugf("等待页面数据加载... (%d/10)", i+1)
			}

			time.Sleep(2 * time.Second) // 额外等待时间
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("等待页面跳转超时")
}

// optimizedSlideWithHumanBehavior 优化的人类行为滑动
func (ch *CaptchaHandler) optimizedSlideWithHumanBehavior(page playwright.Page, box *playwright.Rect, slideDistance float64) error {
	startX := box.X + box.Width/2
	startY := box.Y + box.Height/2

	logger.GetGlobalLogger("crawler/alibaba1688").Debugf("开始优化人类行为滑动: 起始位置(%.2f, %.2f), 滑动距离: %.2f", startX, startY, slideDistance)

	// 1. 先在按钮上悬停，模拟用户观察
	if err := page.Mouse().Move(startX, startY); err != nil {
		return fmt.Errorf("移动鼠标到起始位置失败: %w", err)
	}
	time.Sleep(time.Duration(400+ch.randomDelay(600)) * time.Millisecond) // 0.4-1秒随机延迟

	// 2. 按下鼠标
	if err := page.Mouse().Down(); err != nil {
		return fmt.Errorf("按下鼠标失败: %w", err)
	}

	// 3. 使用更精细的滑动控制
	steps := 25 + ch.randomDelay(15) // 25-40步，更细腻的滑动

	for i := 1; i <= steps; i++ {
		progress := float64(i) / float64(steps)

		// 使用更复杂的缓动函数，模拟真实的人类滑动
		easedProgress := ch.complexEasing(progress)
		currentX := startX + slideDistance*easedProgress

		// 添加更真实的垂直偏移和水平微调
		verticalOffset := math.Sin(progress*math.Pi*2) * 2 // 正弦波形的垂直偏移
		horizontalJitter := float64(ch.randomDelay(4) - 2) // ±2像素水平抖动

		finalX := currentX + horizontalJitter
		finalY := startY + verticalOffset + float64(ch.randomDelay(3)-1) // 额外的随机偏移

		if err := page.Mouse().Move(finalX, finalY); err != nil {
			return fmt.Errorf("滑动过程中移动鼠标失败: %w", err)
		}

		// 更真实的变速延迟
		var delay int
		if progress < 0.1 {
			delay = 120 + ch.randomDelay(80) // 开始很慢：120-200ms
		} else if progress < 0.3 {
			delay = 60 + ch.randomDelay(40) // 加速阶段：60-100ms
		} else if progress < 0.7 {
			delay = 25 + ch.randomDelay(25) // 快速阶段：25-50ms
		} else if progress < 0.9 {
			delay = 40 + ch.randomDelay(30) // 减速阶段：40-70ms
		} else {
			delay = 80 + ch.randomDelay(60) // 结束很慢：80-140ms
		}

		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// 4. 在目标位置稍作停留，模拟人类确认
	time.Sleep(time.Duration(200+ch.randomDelay(300)) * time.Millisecond)

	// 5. 释放鼠标
	if err := page.Mouse().Up(); err != nil {
		return fmt.Errorf("释放鼠标失败: %w", err)
	}

	// 6. 等待验证结果
	time.Sleep(3 * time.Second)
	return nil
}
