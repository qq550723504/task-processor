package alibaba1688

import (
	"math"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/sirupsen/logrus"
)

type fastSlider struct {
	logger *logrus.Entry
}

func newFastSlider() *fastSlider {
	return &fastSlider{
		logger: logrus.WithFields(logrus.Fields{"component": "crawler/alibaba1688/fast-slider"}),
	}
}

// FastSlide 超快速人类滑动算法
func (fs *fastSlider) FastSlide(page playwright.Page, box *playwright.Rect, distance float64) error {
	startX := box.X + box.Width/2
	startY := box.Y + box.Height/2
	endX := startX + distance

	fs.logger.Infof("快速滑动: 起点(%.0f,%.0f) -> 终点(%.0f,%.0f), 距离: %.0fpx", 
		startX, startY, endX, startY, distance)

	// 快速移动到起点
	page.Mouse().Move(startX, startY)
	
	// 极短悬停 (50-150ms)
	time.Sleep(time.Duration(50+randomInt(100)) * time.Millisecond)

	// 按下鼠标
	page.Mouse().Down()
	
	// 微小抖动
	for i := 0; i < 2; i++ {
		page.Mouse().Move(
			startX+float64(randomInt(4)-2),
			startY+float64(randomInt(4)-2),
		)
		time.Sleep(time.Duration(3+randomInt(5)) * time.Millisecond)
	}
	page.Mouse().Move(startX, startY)

	// 核心滑动 - 超快速 (200-400ms)
	totalTime := float64(200 + randomInt(200))
	steps := 15 + randomInt(10)
	
	generateAndExecutePath(page, startX, startY, endX, distance, totalTime, steps)

	// 到达终点
	time.Sleep(time.Duration(20+randomInt(30)) * time.Millisecond)
	
	// 松开鼠标
	page.Mouse().Up()

	// 等待验证结果
	time.Sleep(1500 * time.Millisecond)
	
	fs.logger.Info("快速滑动完成")
	return nil
}

// generateAndExecutePath 生成并执行滑动路径
func generateAndExecutePath(page playwright.Page, startX, startY, endX, distance, totalTime float64, steps int) {
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		
		// 人类缓动曲线
		var progress float64
		if t < 0.1 {
			progress = 5 * t * t // 非常缓慢启动
		} else if t < 0.85 {
			progress = 0.05 + 0.9 * ((t - 0.1) / 0.75) // 快速移动
		} else {
			remaining := 1 - t
			progress = 1 - remaining*remaining*remaining // 缓慢停止
		}
		
		// 计算位置
		x := startX + distance*progress
		
		// 添加自然扰动
		x += math.Sin(t*math.Pi*3+randomFloat()) * 1.5
		x += math.Sin(t*math.Pi*7+randomFloat()) * 0.8
		if randomInt(100) < 10 {
			x += float64(randomInt(8)-4)
		}
		
		y := startY + math.Sin(t*math.Pi*4) * (1 + randomFloat())
		y += float64(randomInt(6)-3) * 0.5
		
		// 随机回退
		if i > steps/3 && i < steps*2/3 && randomInt(100) < 12 {
			x -= float64(randomInt(10)+3)
		}
		
		page.Mouse().Move(x, y)
		
		// 动态时间延迟
		baseDelay := totalTime / float64(steps)
		var multiplier float64
		if t < 0.15 {
			multiplier = 2.0 + randomFloat()
		} else if t < 0.85 {
			multiplier = 0.5 + randomFloat()*0.5
		} else {
			multiplier = 1.5 + randomFloat()*1.0
		}
		time.Sleep(time.Duration(baseDelay*multiplier) * time.Millisecond)
	}
	
	// 最终定位
	page.Mouse().Move(endX, startY)
}

