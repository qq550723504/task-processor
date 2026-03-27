package management

import (
	"task-processor/internal/core/logger"
	"sync"
	"time"

)

// RateLimit 速率限制器
type RateLimit struct {
	lastRequest time.Time
	minInterval time.Duration
	isActive    bool
	mu          sync.RWMutex
}

// NewRateLimit 创建新的速率限制器
func NewRateLimit() *RateLimit {
	return &RateLimit{
		minInterval: time.Millisecond * 500,
		isActive:    false,
	}
}

// Apply 应用速率限制
func (r *RateLimit) Apply() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isActive {
		elapsed := time.Since(r.lastRequest)
		if elapsed < r.minInterval {
			sleepTime := r.minInterval - elapsed
			logger.GetGlobalLogger("infra/clients").Infof("🕐 应用速率限制，等待 %v", sleepTime)
			time.Sleep(sleepTime)
		}
	}
	r.lastRequest = time.Now()
}

// RelaxOnTimeout 超时时放宽限制
func (r *RateLimit) RelaxOnTimeout() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.minInterval > time.Millisecond*100 {
		r.minInterval = r.minInterval * 9 / 10
	}
}

// RelaxOnSuccess 成功时放松限制
func (r *RateLimit) RelaxOnSuccess() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isActive {
		if r.minInterval > time.Millisecond*100 {
			r.minInterval = r.minInterval * 95 / 100
		} else {
			r.isActive = false
			logger.GetGlobalLogger("infra/clients").Infof("✅ 风控解除，关闭速率限制")
		}
	}
}

// Activate 激活速率限制
func (r *RateLimit) Activate(blockCount int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.isActive = true
	r.minInterval = time.Duration(blockCount*2) * time.Second
	if r.minInterval > 10*time.Second {
		r.minInterval = 10 * time.Second
	}
}
