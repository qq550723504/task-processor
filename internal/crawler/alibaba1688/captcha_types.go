// Package alibaba1688 提供1688验证码处理功能
package alibaba1688

import (
	"sync/atomic"
	"time"
)

// CaptchaType 验证码类型
type CaptchaType int

const (
	CaptchaTypeUnknown CaptchaType = iota
	CaptchaTypeSlider
	CaptchaTypeClick
	CaptchaTypeImage
	CaptchaTypeText
	CaptchaTypeMath
)

func (ct CaptchaType) String() string {
	switch ct {
	case CaptchaTypeSlider:
		return "slider"
	case CaptchaTypeClick:
		return "click"
	case CaptchaTypeImage:
		return "image"
	case CaptchaTypeText:
		return "text"
	case CaptchaTypeMath:
		return "math"
	default:
		return "unknown"
	}
}

// CaptchaStatus 验证码状态
type CaptchaStatus int

const (
	CaptchaStatusPending CaptchaStatus = iota
	CaptchaStatusDetected
	CaptchaStatusProcessing
	CaptchaStatusSuccess
	CaptchaStatusFailed
	CaptchaStatusManualRequired
)

// CaptchaResult 验证码处理结果
type CaptchaResult struct {
	Type       CaptchaType
	Status     CaptchaStatus
	Attempts   int
	Error      error
	UsedMethod string
	Duration   time.Duration
}

// CaptchaStatistics 验证码统计信息
type CaptchaStatistics struct {
	TotalCount        int64
	SuccessCount      int64
	FailedCount       int64
	ManualCount       int64
	SliderSuccessRate float64
	ImageSuccessRate  float64
}

// CaptchaHandler 验证码处理器
type CaptchaHandler struct {
	statistics   CaptchaStatistics
	lastAttempt  time.Time
	successStreak int32
	maxRetries    int
}

// NewCaptchaHandler 创建新的验证码处理器
func NewCaptchaHandler() *CaptchaHandler {
	return &CaptchaHandler{
		maxRetries: 3,
	}
}

// NewCaptchaHandlerWithConfig 使用配置创建验证码处理器
func NewCaptchaHandlerWithConfig(maxRetries int) *CaptchaHandler {
	return &CaptchaHandler{
		maxRetries: maxRetries,
	}
}

// GetStatistics 获取验证码统计信息
func (ch *CaptchaHandler) GetStatistics() CaptchaStatistics {
	total := atomic.LoadInt64(&ch.statistics.TotalCount)
	success := atomic.LoadInt64(&ch.statistics.SuccessCount)
	
	sliderRate := 0.0
	imageRate := 0.0
	
	if total > 0 {
		sliderRate = float64(success) / float64(total) * 100
		imageRate = sliderRate
	}
	
	return CaptchaStatistics{
		TotalCount:        total,
		SuccessCount:      success,
		FailedCount:       atomic.LoadInt64(&ch.statistics.FailedCount),
		ManualCount:       atomic.LoadInt64(&ch.statistics.ManualCount),
		SliderSuccessRate: sliderRate,
		ImageSuccessRate:  imageRate,
	}
}

// recordSuccess 记录成功
func (ch *CaptchaHandler) recordSuccess(captchaType CaptchaType) {
	atomic.AddInt64(&ch.statistics.TotalCount, 1)
	atomic.AddInt64(&ch.statistics.SuccessCount, 1)
	atomic.AddInt32(&ch.successStreak, 1)
	ch.lastAttempt = time.Now()
}

// recordFailure 记录失败
func (ch *CaptchaHandler) recordFailure(captchaType CaptchaType) {
	atomic.AddInt64(&ch.statistics.TotalCount, 1)
	atomic.AddInt64(&ch.statistics.FailedCount, 1)
	atomic.StoreInt32(&ch.successStreak, 0)
	ch.lastAttempt = time.Now()
}

// recordManual 记录需要手动处理
func (ch *CaptchaHandler) recordManual(captchaType CaptchaType) {
	atomic.AddInt64(&ch.statistics.TotalCount, 1)
	atomic.AddInt64(&ch.statistics.ManualCount, 1)
	atomic.StoreInt32(&ch.successStreak, 0)
	ch.lastAttempt = time.Now()
}

// ResetStatistics 重置统计信息
func (ch *CaptchaHandler) ResetStatistics() {
	atomic.StoreInt64(&ch.statistics.TotalCount, 0)
	atomic.StoreInt64(&ch.statistics.SuccessCount, 0)
	atomic.StoreInt64(&ch.statistics.FailedCount, 0)
	atomic.StoreInt64(&ch.statistics.ManualCount, 0)
	atomic.StoreInt32(&ch.successStreak, 0)
}
