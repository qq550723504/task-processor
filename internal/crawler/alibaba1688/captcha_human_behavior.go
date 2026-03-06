// Package alibaba1688 提供人类行为模拟功能
package alibaba1688

import (
	"math"
	"time"
)

// randomDelay 生成随机延迟（毫秒）
func (ch *CaptchaHandler) randomDelay(maxMs int) int {
	if maxMs <= 0 {
		return 0
	}
	// 使用时间戳生成伪随机数
	return int(time.Now().UnixNano() % int64(maxMs))
}

// easeInOutCubic 缓动函数
func (ch *CaptchaHandler) easeInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - math.Pow(-2*t+2, 3)/2
}

// complexEasing 复杂的缓动函数，模拟人类滑动的速度变化
func (ch *CaptchaHandler) complexEasing(t float64) float64 {
	// 组合多种缓动效果
	if t < 0.1 {
		// 开始阶段：非常慢的启动
		return 0.5 * t * t
	} else if t < 0.3 {
		// 加速阶段
		adjusted := (t - 0.1) / 0.2
		return 0.005 + 0.1*adjusted*adjusted
	} else if t < 0.7 {
		// 匀速阶段
		return 0.105 + 0.7*(t-0.3)/0.4
	} else if t < 0.9 {
		// 减速阶段
		adjusted := (t - 0.7) / 0.2
		return 0.805 + 0.15*(1-(1-adjusted)*(1-adjusted))
	} else {
		// 最后阶段：非常慢的结束
		adjusted := (t - 0.9) / 0.1
		return 0.955 + 0.045*adjusted*adjusted
	}
}
