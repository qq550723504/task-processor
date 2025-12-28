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
	return &BlockDetector{
		blockCount: 0,
	}
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
	// 添加空指针检查
	if resp == nil {
		logrus.Warnf("🚨 检测风控时收到nil响应")
		return false
	}

	// 检查状态码
	statusCode := b.getStatusCode(resp)
	if statusCode == 403 || statusCode == 429 || statusCode == 503 {
		return true
	}

	// 检查响应内容
	body := b.getResponseBody(resp)
	return b.containsBlockKeywords(body)
}

// HandleBlockDetection 处理风控检测
func (b *BlockDetector) HandleBlockDetection(statusCode int, rateLimit *RateLimit) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.blockCount++
	// 根据被封次数动态调整阻止时间
	blockDuration := time.Duration(b.blockCount*b.blockCount) * time.Minute
	if blockDuration > 30*time.Minute {
		blockDuration = 30 * time.Minute // 最大阻止30分钟
	}
	b.blockedUntil = time.Now().Add(blockDuration)

	// 激活速率限制
	rateLimit.Activate(b.blockCount)

	logrus.Infof("🚨 触发风控检测 (状态码: %d)，阻止时间: %v，速率限制激活",
		statusCode, blockDuration)
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

// getStatusCode 获取响应状态码
func (b *BlockDetector) getStatusCode(resp interface{}) int {
	type statusCodeGetter interface {
		StatusCode() int
	}
	if r, ok := resp.(statusCodeGetter); ok {
		return r.StatusCode()
	}
	// 尝试直接访问字段
	if r, ok := resp.(struct{ StatusCode int }); ok {
		return r.StatusCode
	}
	return 0
}

// getResponseBody 获取响应体内容
func (b *BlockDetector) getResponseBody(resp interface{}) string {
	type stringGetter interface {
		String() string
	}
	if r, ok := resp.(stringGetter); ok {
		return r.String()
	}
	return ""
}

// containsBlockKeywords 检查是否包含风控关键词
func (b *BlockDetector) containsBlockKeywords(body string) bool {
	blockKeywords := []string{
		"Robot Check",
		"blocked",
		"captcha",
		"Too Many Requests",
		"Service Temporarily Unavailable",
		"Access Denied",
		"Forbidden",
		"Rate limit exceeded",
		"Please try again later",
	}

	bodyLower := strings.ToLower(body)
	for _, keyword := range blockKeywords {
		if strings.Contains(bodyLower, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}
