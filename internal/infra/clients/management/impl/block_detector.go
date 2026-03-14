// Package impl 提供风控检测功能
package impl

import (
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// BlockDetector 风控检测器
type BlockDetector struct {
	blockCount   int
	blockedUntil time.Time
	mu           sync.RWMutex
}

// NewBlockDetector 创建新的风控检测器
func NewBlockDetector() *BlockDetector {
	return &BlockDetector{}
}

// IsBlocked 检查是否被风控阻止
func (b *BlockDetector) IsBlocked() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return time.Now().Before(b.blockedUntil)
}

// GetBlockRemainTime 获取剩余阻止时间
func (b *BlockDetector) GetBlockRemainTime() time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if time.Now().Before(b.blockedUntil) {
		return time.Until(b.blockedUntil)
	}
	return 0
}

// DetectBlock 检测是否触发风控
func (b *BlockDetector) DetectBlock(resp interface{}) bool {
	if resp == nil {
		return false
	}
	statusCode := b.getStatusCode(resp)
	if statusCode == 403 || statusCode == 429 || statusCode == 503 {
		return true
	}
	return b.containsBlockKeywords(b.getResponseBody(resp))
}

// HandleBlockDetection 处理风控检测
func (b *BlockDetector) HandleBlockDetection(statusCode int, rateLimit *RateLimit) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.blockCount++
	blockDuration := time.Duration(b.blockCount*b.blockCount) * time.Minute
	if blockDuration > 30*time.Minute {
		blockDuration = 30 * time.Minute
	}
	b.blockedUntil = time.Now().Add(blockDuration)
	rateLimit.Activate(b.blockCount)

	logrus.Infof("🚨 触发风控检测 (状态码: %d)，阻止时间: %v", statusCode, blockDuration)
}

// ResetOnSuccess 成功时重置计数器
func (b *BlockDetector) ResetOnSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.blockCount > 0 {
		b.blockCount--
		if b.blockCount == 0 {
			b.blockedUntil = time.Time{}
		}
	}
}

func (b *BlockDetector) getStatusCode(resp interface{}) int {
	type statusCodeGetter interface{ StatusCode() int }
	if r, ok := resp.(statusCodeGetter); ok {
		return r.StatusCode()
	}
	return 0
}

func (b *BlockDetector) getResponseBody(resp interface{}) string {
	type stringGetter interface{ String() string }
	if r, ok := resp.(stringGetter); ok {
		return r.String()
	}
	return ""
}

func (b *BlockDetector) containsBlockKeywords(body string) bool {
	keywords := []string{
		"Robot Check", "blocked", "captcha", "Too Many Requests",
		"Service Temporarily Unavailable", "Access Denied", "Forbidden",
		"Rate limit exceeded", "Please try again later",
	}
	bodyLower := strings.ToLower(body)
	for _, kw := range keywords {
		if strings.Contains(bodyLower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
