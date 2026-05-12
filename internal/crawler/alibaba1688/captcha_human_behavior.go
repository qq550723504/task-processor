// Package alibaba1688 提供人类行为模拟功能
package alibaba1688

import (
	"fmt"
	"math"
	"time"

	"github.com/playwright-community/playwright-go"
)

// easeInOutCubic 缓动函数
func (ch *CaptchaHandler) easeInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - math.Pow(-2*t+2, 3)/2
}

// complexEasing 复杂的缓动函数，模拟人类滑动的速度变化
func (ch *CaptchaHandler) complexEasing(t float64) float64 {
	if t < 0.1 {
		return 0.5 * t * t
	} else if t < 0.3 {
		adjusted := (t - 0.1) / 0.2
		return 0.005 + 0.1*adjusted*adjusted
	} else if t < 0.7 {
		return 0.105 + 0.7*(t-0.3)/0.4
	} else if t < 0.9 {
		adjusted := (t - 0.7) / 0.2
		return 0.805 + 0.15*(1-(1-adjusted)*(1-adjusted))
	} else {
		adjusted := (t - 0.9) / 0.1
		return 0.955 + 0.045*adjusted*adjusted
	}
}

// randomDelay 生成随机延迟（毫秒）
func (ch *CaptchaHandler) randomDelay(maxMs int) int {
	if maxMs <= 0 {
		return 0
	}
	return int(time.Now().UnixNano()%int64(maxMs)) + 1
}

// 基于真实轨迹的人类滑动算法
// 完整版：包含预热、探索、滑动、结束四个阶段
// 真实轨迹特征分析：
// 1. 总时长: ~2000ms
// 2. 距离: ~262px
// 3. 启动延迟: ~250ms
// 4. 速度阶段: 慢启动(0-30%) -> 匀速(30-70%) -> 减速(70-90%) -> 终点停留(90-100%)
// 5. 垂直抖动: ±10px
// 6. 时间间隔: 7-56ms随机，不均匀

func (ch *CaptchaHandler) optimizedSlideWithRealTrack(page playwright.Page, sliderBox *playwright.Rect, slideDistance float64) error {
	startX := sliderBox.X + sliderBox.Width/2
	startY := sliderBox.Y + sliderBox.Height/2
	endX := startX + slideDistance
	endY := startY + 15 // 真实轨迹显示终点Y会下降约15px

	// === 第一阶段：预热行为 (模拟人类从远处移动到滑块附近) ===
	// 真人通常不会突然出现在滑块上，而是从附近慢慢靠近
	page.Mouse().Move(startX-100+float64(ch.randomDelay(50)), startY-50+float64(ch.randomDelay(30)))
	time.Sleep(300 + time.Duration(ch.randomDelay(400))*time.Millisecond)

	// 移动到滑块上方附近（但不是直接在上面）
	page.Mouse().Move(startX+float64(ch.randomDelay(40)-20), startY-30+float64(ch.randomDelay(20)))
	time.Sleep(200 + time.Duration(ch.randomDelay(300))*time.Millisecond)

	// === 第二阶段：探索行为 (在滑块附近徘徊，模拟人类确认目标) ===
	// 真人会在按下去之前确认一下位置
	for i := 0; i < 2+ch.randomDelay(2); i++ {
		offsetX := float64(ch.randomDelay(20) - 10)
		offsetY := float64(ch.randomDelay(20) - 10)
		page.Mouse().Move(startX+offsetX, startY+offsetY)
		time.Sleep(100 + time.Duration(ch.randomDelay(150))*time.Millisecond)
	}

	// 犹豫一下（真实人类按下前会有明显犹豫）
	time.Sleep(300 + time.Duration(ch.randomDelay(400))*time.Millisecond)

	// === 第三阶段：按下并滑动 ===
	// 移动到精确起点位置
	page.Mouse().Move(startX, startY)
	time.Sleep(50 + time.Duration(ch.randomDelay(50))*time.Millisecond)

	// 按下鼠标
	page.Mouse().Down()
	time.Sleep(50 + time.Duration(ch.randomDelay(50))*time.Millisecond)

	// 真实轨迹显示有~250ms的启动犹豫
	time.Sleep(200 + time.Duration(ch.randomDelay(150)) * time.Millisecond)

	// 生成基于真实轨迹的滑动点
	points := ch.generateRealHumanTrackPoints(startX, startY, endX, endY, 60)

	// 回放轨迹
	for i, pt := range points {
		page.Mouse().Move(pt.x, pt.y)

		// 真实轨迹的时间间隔是不均匀的：7-56ms随机
		if i < len(points)-1 {
			delay := 7 + ch.randomDelay(49) // 7-56ms
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}

		// 在滑动过程中添加随机停顿（模拟人类"思考"）
		if ch.randomDelay(100) < 15 { // 15%概率停顿
			stopDuration := 30 + ch.randomDelay(80)
			time.Sleep(time.Duration(stopDuration) * time.Millisecond)
		}
	}

	// === 第四阶段：结束行为 ===
	// 终点停留（真实轨迹显示松开前有~500ms停留）
	time.Sleep(400 + time.Duration(ch.randomDelay(200)) * time.Millisecond)

	// 松开鼠标
	page.Mouse().Up()

	// 滑动后可能有一些微小移动（真实人类特征）
	time.Sleep(50 + time.Duration(ch.randomDelay(100))*time.Millisecond)

	return nil
}

// generateRealHumanTrackPoints 生成基于真实轨迹特征的点
func (ch *CaptchaHandler) generateRealHumanTrackPoints(startX, startY, endX, endY float64, numPoints int) []point {
	points := make([]point, numPoints)

	baseY := startY
	verticalNoise := 12.0 // 稍微增加垂直抖动范围

	// 预先生成一些随机回退点的位置
	backtrackPositions := make(map[int]bool)
	backtrackCount := 2 + ch.randomDelay(3) // 2-5次回退
	for i := 0; i < backtrackCount; i++ {
		idx := 5 + ch.randomDelay(numPoints-10)
		backtrackPositions[idx] = true
	}

	// 预先生成随机停顿点（模拟人类思考）
	pausePositions := make(map[int]int)
	pauseCount := 3 + ch.randomDelay(5) // 3-8次停顿
	for i := 0; i < pauseCount; i++ {
		idx := 3 + ch.randomDelay(numPoints-6)
		pausePositions[idx] = 30 + ch.randomDelay(100) // 停顿时长30-130ms
	}

	for i := 0; i < numPoints; i++ {
		t := float64(i) / float64(numPoints-1)

		// 真实轨迹的速度曲线（基于采样分析）
		var progress float64
		if t < 0.15 {
			// 0-15%: 慢启动阶段（真实轨迹显示这个阶段速度很慢）
			progress = 0.3 * math.Pow(t/0.15, 2)
		} else if t < 0.50 {
			// 15-50%: 匀速阶段（真实轨迹显示这个阶段速度最快且均匀）
			progress = 0.3 + 0.5*(t-0.15)/0.35
		} else if t < 0.85 {
			// 50-85%: 减速阶段（真实轨迹显示接近终点时明显减速）
			progress = 0.8 + 0.15*math.Pow((t-0.50)/0.35, 0.5)
		} else {
			// 85-100%: 终点停留阶段（真实轨迹显示最后有明显停留）
			progress = 0.95 + 0.05*math.Pow((t-0.85)/0.15, 0.3)
		}

		// 水平位移（线性）
		x := startX + (endX-startX)*progress

		// 垂直位移（带抖动的正弦波 + 随机噪声）
		// 真实轨迹显示Y坐标在中间偏高（730-745），终点下降到724
		yOffset := math.Sin(t*math.Pi*4) * verticalNoise * 0.3 // 正弦波动
		yOffset += float64(ch.randomDelay(24) - 12)            // 随机噪声扩大范围
		yOffset += t * (endY - startY)                         // 终点Y下降趋势

		// 确保Y在合理范围内
		y := baseY + yOffset
		if y < endY-5 {
			y = endY - 5
		}
		if y > startY+18 {
			y = startY + 18
		}

		points[i] = point{x: x, y: y}
	}

	// 添加微小的回退点（真实轨迹显示有少量回退）
	for idx := range backtrackPositions {
		if idx < numPoints-2 && idx > 2 {
			backtrackAmount := float64(ch.randomDelay(8) + 2) // 2-10px回退
			points[idx].x -= backtrackAmount
		}
	}

	return points
}

// simulateHumanTyping 模拟人类打字行为
func (ch *CaptchaHandler) simulateHumanTyping(page playwright.Page, input playwright.ElementHandle, text string) error {
	for i, char := range text {
		if err := input.Type(string(char)); err != nil {
			return err
		}

		baseDelay := 50
		if i == 0 || i == len(text)-1 {
			baseDelay = 100
		}

		delay := baseDelay + ch.randomDelay(100)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	return nil
}

// simulateHumanMouseMovement 模拟人类鼠标移动
func (ch *CaptchaHandler) simulateHumanMouseMovement(page playwright.Page, startX, startY, endX, endY float64) error {
	points := ch.generateBezierCurvePoints(startX, startY, endX, endY, 50)

	for i, point := range points {
		if err := page.Mouse().Move(point.x, point.y); err != nil {
			return err
		}

		if i < len(points)-1 {
			delay := 10 + ch.randomDelay(20)
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}
	return nil
}

type point struct {
	x, y float64
}

// generateBezierCurvePoints 生成贝塞尔曲线点
func (ch *CaptchaHandler) generateBezierCurvePoints(startX, startY, endX, endY float64, numPoints int) []point {
	points := make([]point, numPoints)

	controlX1 := startX + (endX-startX)*0.3 + float64(ch.randomDelay(100)-50)
	controlY1 := startY + float64(ch.randomDelay(200)-100)
	controlX2 := startX + (endX-startX)*0.7 + float64(ch.randomDelay(100)-50)
	controlY2 := endY + float64(ch.randomDelay(200)-100)

	for i := 0; i < numPoints; i++ {
		t := float64(i) / float64(numPoints-1)
		oneMinusT := 1 - t

		x := math.Pow(oneMinusT, 3)*startX +
			3*math.Pow(oneMinusT, 2)*t*controlX1 +
			3*oneMinusT*math.Pow(t, 2)*controlX2 +
			math.Pow(t, 3)*endX

		y := math.Pow(oneMinusT, 3)*startY +
			3*math.Pow(oneMinusT, 2)*t*controlY1 +
			3*oneMinusT*math.Pow(t, 2)*controlY2 +
			math.Pow(t, 3)*endY

		points[i] = point{x, y}
	}

	return points
}

// simulateHumanScrolling 模拟人类滚动行为
func (ch *CaptchaHandler) simulateHumanScrolling(page playwright.Page, targetY int) error {
	currentY := 0
	stepSize := 50 + ch.randomDelay(100)

	for currentY < targetY {
		scrollAmount := stepSize
		if currentY+scrollAmount > targetY {
			scrollAmount = targetY - currentY
		}

		if _, err := page.Evaluate(fmt.Sprintf("window.scrollBy(0, %d)", scrollAmount)); err != nil {
			return err
		}

		currentY += scrollAmount

		delay := 100 + ch.randomDelay(200)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	return nil
}

// simulateRandomMouseWiggling 模拟随机鼠标抖动
func (ch *CaptchaHandler) simulateRandomMouseWiggling(page playwright.Page, centerX, centerY float64, durationMs int) error {
	startTime := time.Now()

	for time.Since(startTime) < time.Duration(durationMs)*time.Millisecond {
		offsetX := float64(ch.randomDelay(20) - 10)
		offsetY := float64(ch.randomDelay(20) - 10)

		if err := page.Mouse().Move(centerX+offsetX, centerY+offsetY); err != nil {
			return err
		}

		time.Sleep(time.Duration(50+ch.randomDelay(100)) * time.Millisecond)
	}

	return page.Mouse().Move(centerX, centerY)
}

// simulateTypingMistakes 模拟打字错误和修正
func (ch *CaptchaHandler) simulateTypingMistakes(page playwright.Page, input playwright.ElementHandle, text string, mistakeProbability float64) error {
	for _, char := range text {
		if math.Float64frombits(uint64(ch.randomDelay(1000))) / 1000.0 < mistakeProbability {
			mistakeChar := rune('a' + ch.randomDelay(26))
			if err := input.Type(string(mistakeChar)); err != nil {
				return err
			}
			time.Sleep(time.Duration(200+ch.randomDelay(100)) * time.Millisecond)
			if err := input.Press("Backspace"); err != nil {
				return err
			}
			time.Sleep(time.Duration(100+ch.randomDelay(100)) * time.Millisecond)
		}

		if err := input.Type(string(char)); err != nil {
			return err
		}

		delay := 50 + ch.randomDelay(100)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	return nil
}
