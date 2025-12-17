package downloader

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

// 反风控相关方法 - 完全参考SHEIN实现

// isBlocked 检查是否被风控阻止
func (d *ImageDownloader) isBlocked() bool {
	d.blockDetector.mu.RLock()
	defer d.blockDetector.mu.RUnlock()
	return time.Now().Before(d.blockDetector.blockedUntil)
}

// getBlockRemainTime 获取剩余阻止时间
func (d *ImageDownloader) getBlockRemainTime() time.Duration {
	d.blockDetector.mu.RLock()
	defer d.blockDetector.mu.RUnlock()
	if time.Now().Before(d.blockDetector.blockedUntil) {
		return time.Until(d.blockDetector.blockedUntil)
	}
	return 0
}

// applyRateLimit 应用速率限制
func (d *ImageDownloader) applyRateLimit() {
	d.rateLimit.mu.Lock()
	defer d.rateLimit.mu.Unlock()

	if d.rateLimit.isActive {
		elapsed := time.Since(d.rateLimit.lastRequest)
		if elapsed < d.rateLimit.minInterval {
			sleepTime := d.rateLimit.minInterval - elapsed
			d.logger.Printf("🕐 应用速率限制，等待 %v", sleepTime)
			time.Sleep(sleepTime)
		}
	}
	d.rateLimit.lastRequest = time.Now()
}

// createDynamicRequest 创建动态请求，随机化请求头
func (d *ImageDownloader) createDynamicRequest(targetURL string) *req.Request {
	req := d.httpClient.R()

	// 随机选择User-Agent
	if len(d.userAgents) > 0 {
		userAgent := d.userAgents[rand.Intn(len(d.userAgents))]
		req.SetHeader("User-Agent", userAgent)
	}

	// 动态设置Referer
	if parsedURL, err := d.parseURL(targetURL); err == nil {
		referer := fmt.Sprintf("%s://%s/", parsedURL.Scheme, parsedURL.Host)
		req.SetHeader("Referer", referer)
	}

	// 随机化一些请求头
	acceptLanguages := []string{
		"en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		"en-US,en;q=0.9",
		"zh-CN,zh;q=0.9,en;q=0.8",
		"en-GB,en;q=0.9,en-US;q=0.8",
	}
	req.SetHeader("Accept-Language", acceptLanguages[rand.Intn(len(acceptLanguages))])

	// 随机化DNT头
	if rand.Float32() < 0.5 {
		req.SetHeader("DNT", "1")
	}

	// 模拟真实浏览器行为：随机添加一些可选头
	if rand.Float32() < 0.3 {
		req.SetHeader("Sec-GPC", "1")
	}

	return req
}

// detectBlock 检测是否触发风控
func (d *ImageDownloader) detectBlock(resp *req.Response) bool {
	// 添加空指针检查
	if resp == nil {
		d.logger.Warnf("🚨 检测风控时收到nil响应")
		return false
	}

	// 检查状态码
	if resp.StatusCode == 403 || resp.StatusCode == 429 || resp.StatusCode == 503 {
		return true
	}

	// 检查响应内容
	body := resp.String()
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

// handleBlockDetection 处理风控检测
func (d *ImageDownloader) handleBlockDetection(statusCode int) {
	d.blockDetector.mu.Lock()
	defer d.blockDetector.mu.Unlock()

	d.blockDetector.blockCount++
	// 根据被封次数动态调整阻止时间
	blockDuration := time.Duration(d.blockDetector.blockCount*d.blockDetector.blockCount) * time.Minute
	if blockDuration > 30*time.Minute {
		blockDuration = 30 * time.Minute // 最大阻止30分钟
	}
	d.blockDetector.blockedUntil = time.Now().Add(blockDuration)

	// 激活速率限制
	d.rateLimit.mu.Lock()
	d.rateLimit.isActive = true
	d.rateLimit.minInterval = time.Duration(d.blockDetector.blockCount*2) * time.Second
	if d.rateLimit.minInterval > 10*time.Second {
		d.rateLimit.minInterval = 10 * time.Second // 最大间隔10秒
	}
	d.rateLimit.mu.Unlock()

	d.logger.Printf("🚨 触发风控检测 (状态码: %d)，阻止时间: %v，速率限制: %v",
		statusCode, blockDuration, d.rateLimit.minInterval)
}

// handleRequestError 处理请求错误
func (d *ImageDownloader) handleRequestError(err error) {
	// 对于某些错误，可能需要调整策略
	errStr := err.Error()
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		// 超时错误，可能需要放宽速率限制
		d.rateLimit.mu.Lock()
		if d.rateLimit.minInterval > time.Millisecond*100 {
			d.rateLimit.minInterval = d.rateLimit.minInterval * 9 / 10 // 减少10%
		}
		d.rateLimit.mu.Unlock()
	}
}

// handleRequestSuccess 处理请求成功
func (d *ImageDownloader) handleRequestSuccess() {
	// 成功请求后，逐渐放松限制
	d.rateLimit.mu.Lock()
	defer d.rateLimit.mu.Unlock()

	if d.rateLimit.isActive {
		// 逐渐减少最小间隔
		if d.rateLimit.minInterval > time.Millisecond*100 {
			d.rateLimit.minInterval = d.rateLimit.minInterval * 95 / 100 // 减少5%
		} else {
			d.rateLimit.isActive = false
			d.logger.Printf("✅ 风控解除，关闭速率限制")
		}
	}

	// 重置阻止计数器（成功请求表明没有被风控）
	d.blockDetector.mu.Lock()
	if d.blockDetector.blockCount > 0 {
		d.blockDetector.blockCount--
		if d.blockDetector.blockCount == 0 {
			d.blockDetector.blockedUntil = time.Time{}
		}
	}
	d.blockDetector.mu.Unlock()
}

// parseURL 解析URL的辅助函数
func (d *ImageDownloader) parseURL(rawURL string) (*URLInfo, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("invalid URL scheme")
	}

	var scheme, host string
	if strings.HasPrefix(rawURL, "https://") {
		scheme = "https"
		rawURL = rawURL[8:] // 移除 "https://"
	} else {
		scheme = "http"
		rawURL = rawURL[7:] // 移除 "http://"
	}

	// 找到第一个 '/' 或字符串结尾
	slashIndex := strings.Index(rawURL, "/")
	if slashIndex == -1 {
		host = rawURL
	} else {
		host = rawURL[:slashIndex]
	}

	return &URLInfo{Scheme: scheme, Host: host}, nil
}

// URLInfo URL信息结构
type URLInfo struct {
	Scheme string
	Host   string
}
