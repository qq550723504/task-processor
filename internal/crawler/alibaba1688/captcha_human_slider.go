package alibaba1688

import (
	"math"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

type humanSlider struct {
	logger *logrus.Entry
}

func newHumanSlider() *humanSlider {
	return &humanSlider{
		logger: logrus.WithFields(logrus.Fields{"component": "crawler/alibaba1688/human-slider"}),
	}
}

// HumanSlide 真正模拟人类滑动的核心算法 - 使用Playwright Mouse API
func (hs *humanSlider) HumanSlide(page playwright.Page, box *playwright.Rect, distance float64) error {
	startX := box.X + box.Width/2
	startY := box.Y + box.Height/2
	endX := startX + distance

	hs.logger.Infof("开始人类滑动: 起点(%.1f,%.1f) -> 终点(%.1f,%.1f), 距离: %.1fpx", 
		startX, startY, endX, startY, distance)

	// 1. 快速移动到起点附近
	currentX := 100 + float64(randomInt(400))
	currentY := 100 + float64(randomInt(300))
	
	steps := 5 + randomInt(5)
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps-1)
		x := currentX + (startX-currentX)*t + float64(randomInt(10)-5)
		y := currentY + (startY-currentY)*t + float64(randomInt(10)-5)
		page.Mouse().Move(x, y)
		time.Sleep(time.Duration(20 + randomInt(40)) * time.Millisecond)
	}
	
	// 精确定位到起点
	page.Mouse().Move(startX, startY)

	// 2. 悬停等待 - 模拟观察
	hoverDelay := 80 + randomInt(120)
	time.Sleep(time.Duration(hoverDelay) * time.Millisecond)

	// 3. 按下鼠标
	page.Mouse().Down()

	// 4. 微小抖动 - 模拟按下瞬间的不稳
	for i := 0; i < 3; i++ {
		wobbleX := startX + float64(randomInt(4)-2)
		wobbleY := startY + float64(randomInt(4)-2)
		page.Mouse().Move(wobbleX, wobbleY)
		time.Sleep(time.Duration(5 + randomInt(10)) * time.Millisecond)
	}
	page.Mouse().Move(startX, startY)

	// 5. 核心滑动 - 快速但自然
	totalDuration := 400 + randomInt(200) // 400-600ms
	steps = 20 + randomInt(15)
	
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		
		// 人类缓动曲线
		var progress float64
		if t < 0.15 {
			progress = 4 * t * t // 缓慢启动
		} else if t < 0.75 {
			progress = 0.09 + 0.82 * ((t - 0.15) / 0.6) // 快速移动
		} else {
			remaining := 1 - t
			progress = 1 - remaining * remaining // 缓慢停止
		}
		
		// 计算位置
		x := startX + distance*progress
		
		// 添加自然扰动
		x += math.Sin(t*math.Pi*3) * float64(1.5+randomFloat())
		x += float64(randomInt(6)-3) * 0.5
		
		y := startY + math.Sin(t*math.Pi*4) * float64(1+randomFloat())
		y += float64(randomInt(4)-2)
		
		// 随机回退
		if i > steps/4 && i < steps*3/4 && randomInt(100) < 8 {
			x -= float64(randomInt(8)+3)
			y += float64(randomInt(4)-2)
		}
		
		page.Mouse().Move(x, y)
		
		// 动态延迟
		baseDelay := float64(totalDuration) / float64(steps)
		var multiplier float64
		if t < 0.2 {
			multiplier = 1.5 + randomFloat()*0.5
		} else if t < 0.8 {
			multiplier = 0.6 + randomFloat()*0.4
		} else {
			multiplier = 1.2 + randomFloat()*0.6
		}
		time.Sleep(time.Duration(baseDelay*multiplier) * time.Millisecond)
	}

	// 6. 到达终点后微调
	adjustments := randomInt(2)
	for i := 0; i < adjustments; i++ {
		adjustX := endX + float64(randomInt(6)-3)
		adjustY := startY + float64(randomInt(4)-2)
		page.Mouse().Move(adjustX, adjustY)
		time.Sleep(time.Duration(20 + randomInt(30)) * time.Millisecond)
	}
	
	// 确保在终点
	page.Mouse().Move(endX, startY)

	// 7. 短暂保持
	time.Sleep(time.Duration(20 + randomInt(30)) * time.Millisecond)

	// 8. 松开鼠标
	page.Mouse().Up()

	// 等待验证结果
	time.Sleep(2 * time.Second)
	
	hs.logger.Info("滑动完成")
	return nil
}

// moveToStart 模拟人类移动鼠标到起点的过程
func (hs *humanSlider) moveToStart(page playwright.Page, targetX, targetY float64) {
	// 从随机位置开始
	startX := 100 + float64(randomInt(400))
	startY := 100 + float64(randomInt(300))

	steps := 8 + randomInt(12)
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps-1)
		t = hs.easeInOutQuad(t)

		x := startX + (targetX-startX)*t
		y := startY + (targetY-startY)*t

		// 添加随机偏移
		x += float64(randomInt(20) - 10)
		y += float64(randomInt(15) - 7)

		page.Mouse().Move(x, y)
		time.Sleep(time.Duration(30 + randomInt(50)) * time.Millisecond)
	}

	// 最后精确定位
	page.Mouse().Move(targetX, targetY)
}

// microWobble 微小抖动
func (hs *humanSlider) microWobble(page playwright.Page, centerX, centerY float64, durationMs int) {
	start := time.Now()
	for time.Since(start) < time.Duration(durationMs)*time.Millisecond {
		x := centerX + float64(randomInt(6)-3)
		y := centerY + float64(randomInt(6)-3)
		page.Mouse().Move(x, y)
		time.Sleep(time.Duration(5 + randomInt(10)) * time.Millisecond)
	}
	page.Mouse().Move(centerX, centerY)
}

// physicalSlide 物理模拟滑动 - 核心算法
func (hs *humanSlider) physicalSlide(page playwright.Page, startX, startY, endX float64, totalTimeMs int) {
	distance := endX - startX
	startTime := time.Now()

	// 人类滑动的关键参数
	accelerationTime := int(float64(totalTimeMs) * (0.15 + float64(randomInt(10))/100))  // 15-25%
	constantTime := totalTimeMs * 2 / 3                                                   // ~66%
	_ = totalTimeMs - accelerationTime - constantTime  // 减速阶段时间

	// 最大速度
	maxSpeed := distance / float64(totalTimeMs) * 1.5

	for {
		elapsed := int(time.Since(startTime).Milliseconds())
		if elapsed >= totalTimeMs {
			break
		}

		// 计算当前速度（基于时间阶段）
		_ = maxSpeed // 使用变量避免编译警告
		if elapsed < accelerationTime {
			// 加速阶段 - 使用缓动函数
		} else if elapsed < accelerationTime+constantTime {
			// 匀速阶段（带波动）
		} else {
			// 减速阶段
		}

		// 计算位置
		progress := float64(elapsed) / float64(totalTimeMs)
		baseX := startX + distance*progress

		// 添加自然扰动
		x := baseX + hs.calculateNoise(progress, float64(elapsed))
		y := startY + hs.calculateVerticalWobble(progress, float64(elapsed))

		page.Mouse().Move(x, y)

		// 微小随机延迟
		time.Sleep(time.Duration(2 + randomInt(5)) * time.Millisecond)
	}

	// 确保到达终点
	page.Mouse().Move(endX, startY)
}

// calculateNoise 计算水平方向的自然扰动
func (hs *humanSlider) calculateNoise(progress, elapsedMs float64) float64 {
	noise := 0.0

	// 多种频率的正弦波叠加
	noise += math.Sin(elapsedMs*0.02+randomFloat()) * 2.5
	noise += math.Sin(elapsedMs*0.05+randomFloat()) * 1.2
	noise += math.Sin(elapsedMs*0.008+randomFloat()) * 3.5

	// 随机冲击
	if randomInt(100) < 5 {
		noise += float64(randomInt(10) - 5)
	}

	// 接近终点时减少扰动
	if progress > 0.8 {
		noise *= (1 - (progress-0.8)/0.2)
	}

	return noise
}

// calculateVerticalWobble 计算垂直方向的抖动
func (hs *humanSlider) calculateVerticalWobble(progress, elapsedMs float64) float64 {
	wobble := 0.0

	// 基础抖动
	wobble += math.Sin(elapsedMs*0.015) * (2 + randomFloat())
	wobble += math.Sin(elapsedMs*0.03) * (1 + randomFloat())

	// 随机偏移
	wobble += float64(randomInt(8) - 4) * 0.5

	return wobble
}

// finalAdjustments 到达终点后的微小调整
func (hs *humanSlider) finalAdjustments(page playwright.Page, endX, endY float64) {
	adjustments := randomInt(3)
	for i := 0; i < adjustments; i++ {
		x := endX + float64(randomInt(6)-3)
		y := endY + float64(randomInt(4)-2)
		page.Mouse().Move(x, y)
		time.Sleep(time.Duration(30 + randomInt(50)) * time.Millisecond)
	}
	page.Mouse().Move(endX, endY)
}

// easeInOutQuad 缓动函数
func (hs *humanSlider) easeInOutQuad(t float64) float64 {
	if t < 0.5 {
		return 2 * t * t
	}
	return 1 - math.Pow(-2*t+2, 2)/2
}

// easeOutQuad 缓出函数
func (hs *humanSlider) easeOutQuad(t float64) float64 {
	return 1 - math.Pow(1-t, 2)
}

// easeInQuad 缓入函数
func (hs *humanSlider) easeInQuad(t float64) float64 {
	return t * t
}

// randomInt 返回 [0, max) 的随机整数
func randomInt(max int) int {
	return int(randomFloat() * float64(max))
}

// randomFloat 返回 [0, 1) 的随机浮点数
func randomFloat() float64 {
	return float64(math.Floor(math.Mod(float64(time.Now().UnixNano()), 1000000)) / 1000000)
}