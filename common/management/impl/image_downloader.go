package impl

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"task-processor/common/management/api"
	"time"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// ImageDownloader 图片下载客户端实现
type ImageDownloader struct {
	httpClient    *req.Client
	timeout       time.Duration
	userAgents    []string
	rateLimit     *RateLimit
	blockDetector *BlockDetector
}

// RateLimit 速率限制器
type RateLimit struct {
	lastRequest time.Time
	minInterval time.Duration
	isActive    bool
	mu          sync.RWMutex
}

// BlockDetector 风控检测器
type BlockDetector struct {
	blockCount   int
	blockedUntil time.Time
	mu           sync.RWMutex
}

// NewImageDownloader 创建新的图片下载客户端
func NewImageDownloader(timeout time.Duration) *ImageDownloader {
	logrus.Infof("🔧 创建增强反风控图片下载器，超时时间: %v", timeout)

	// 多样化的User-Agent池，模拟不同浏览器和设备
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Edge/120.0.0.0 Safari/537.36",
	}

	// 创建增强的HTTP客户端
	client := req.C().
		SetTLSFingerprintChrome().
		SetTimeout(timeout).
		// 设置更真实的TLS配置
		SetTLSClientConfig(&tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS13,
			CipherSuites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			},
		}).
		// 基础请求头，会在每次请求时动态调整
		SetCommonHeaders(map[string]string{
			"Accept":                    "image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
			"Accept-Encoding":           "gzip, deflate, br",
			"Accept-Language":           "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
			"Cache-Control":             "no-cache",
			"Pragma":                    "no-cache",
			"Sec-Ch-Ua":                 `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
			"Sec-Ch-Ua-Mobile":          "?0",
			"Sec-Ch-Ua-Platform":        `"Windows"`,
			"Sec-Fetch-Dest":            "image",
			"Sec-Fetch-Mode":            "no-cors",
			"Sec-Fetch-Site":            "cross-site",
			"Upgrade-Insecure-Requests": "1",
		}).
		// 增强重试策略
		SetCommonRetryCount(5).
		SetCommonRetryInterval(func(resp *req.Response, attempt int) time.Duration {
			// 动态退避策略：基础延迟 + 随机抖动
			baseDelay := time.Duration(attempt*attempt) * time.Second
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
			return baseDelay + jitter
		}).
		SetCommonRetryCondition(func(resp *req.Response, err error) bool {
			// 网络错误重试
			if err != nil {
				errStr := err.Error()
				if strings.Contains(errStr, "timeout") ||
					strings.Contains(errStr, "deadline exceeded") ||
					strings.Contains(errStr, "connection reset") ||
					strings.Contains(errStr, "broken pipe") ||
					strings.Contains(errStr, "network is unreachable") ||
					strings.Contains(errStr, "no such host") ||
					strings.Contains(errStr, "connection refused") ||
					strings.Contains(errStr, "i/o timeout") ||
					strings.Contains(errStr, "EOF") {
					return true
				}
			}
			// HTTP错误重试
			if resp != nil {
				// 使用安全的方式访问 StatusCode
				statusCode := 0
				getStatusCodeSafely := func() (int, bool) {
					defer func() {
						if r := recover(); r != nil {
							logrus.Warnf("   ⚠️  访问响应状态码时发生恐慌: %v", r)
						}
					}()
					return resp.StatusCode, true
				}

				if sc, ok := getStatusCodeSafely(); ok {
					statusCode = sc
					// 5xx服务器错误
					if statusCode >= 500 {
						return true
					}
					// 特定的风控相关状态码
					if statusCode == 429 || statusCode == 403 {
						return true
					}
				} else {
					logrus.Warnf("   ⚠️  无法安全获取响应状态码")
				}
			}
			return false
		}).
		SetCommonRetryHook(func(resp *req.Response, err error) {
			// 添加空指针检查
			if resp != nil {
				// 使用最安全的方式访问 StatusCode
				statusCode := 0
				getStatusCodeSafely := func() (int, bool) {
					defer func() {
						if r := recover(); r != nil {
							logrus.Warnf("   ⚠️  访问响应状态码时发生恐慌: %v", r)
						}
					}()
					return resp.StatusCode, true
				}

				if sc, ok := getStatusCodeSafely(); ok {
					statusCode = sc
					if statusCode == 429 {
						logrus.Infof("   🚨 触发速率限制: 状态码=429")
					} else if statusCode == 403 {
						logrus.Infof("   🚨 访问被拒绝: 状态码=403")
					} else if statusCode >= 500 && statusCode <= 999 { // 确保是有效的HTTP状态码范围
						logrus.Infof("   🚨 服务器错误: 状态码=%d", statusCode)
					}
				} else {
					logrus.Warnf("   ⚠️  无法安全获取响应状态码")
				}
			} else if err != nil {
				logrus.Infof("   🚨 请求错误: %v", err)
			}
			// 如果resp和err都为nil，记录警告信息
			if resp == nil && err == nil {
				logrus.Warnf("   ⚠️  重试钩子被调用，但resp和err都为nil")
			}
		})

	downloader := &ImageDownloader{
		httpClient: client,
		timeout:    timeout,
		userAgents: userAgents,
		rateLimit: &RateLimit{
			minInterval: time.Millisecond * 500, // 默认最小间隔500ms
			isActive:    false,
		},
		blockDetector: &BlockDetector{
			blockCount: 0,
		},
	}

	return downloader
}

// DownloadImage 下载图片并返回图片数据 - 增强反风控版本
func (d *ImageDownloader) DownloadImage(url string) ([]byte, error) {
	startTime := time.Now()
	originalURL := url

	// 清理URL中的双引号和其他异常字符
	url = strings.Trim(url, "\"") // 去除前后的双引号
	url = strings.TrimSpace(url)  // 去除前后空格

	// 预检查URL格式
	if url == "" {
		return nil, fmt.Errorf("图片URL为空")
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("无效的图片URL格式: %s", url)
	}

	// 检查是否被风控阻止
	if d.isBlocked() {
		blockTime := d.getBlockRemainTime()
		logrus.Infof("🚫 检测到风控阻止，等待 %v 后重试", blockTime)
		time.Sleep(blockTime)
	}

	// 应用速率限制
	d.applyRateLimit()

	// 创建动态请求
	req := d.createDynamicRequest(url)

	resp, err := req.Get(url)
	elapsed := time.Since(startTime)

	if err != nil {
		logrus.Infof("❌ 下载失败 [耗时: %v]: %v", elapsed, err)
		d.handleRequestError(err)

		// 分析错误类型
		errStr := err.Error()
		if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
			logrus.Infof("   🕐 超时分析: 设置超时=%v, 实际耗时=%v", d.timeout, elapsed)
			logrus.Infof("   🌐 网络建议: Amazon CDN在中国访问较慢，建议检查网络连接")
		} else if strings.Contains(errStr, "connection") {
			logrus.Infof("   🔌 连接分析: 网络连接问题，可能需要代理")
		} else if strings.Contains(errStr, "dns") || strings.Contains(errStr, "no such host") {
			logrus.Infof("   🔍 DNS分析: 域名解析问题")
		}

		// 如果清理后的URL失败，尝试使用原始URL
		if originalURL != url {
			logrus.Infof("🔄 尝试原始URL: %s", originalURL)
			retryStart := time.Now()
			retryReq := d.createDynamicRequest(originalURL)
			resp, retryErr := retryReq.Get(originalURL)
			retryElapsed := time.Since(retryStart)

			if retryErr == nil && resp.IsSuccessState() {
				logrus.Infof("✅ 原始URL成功 [耗时: %v]", retryElapsed)
				d.handleRequestSuccess()
				// 由于resp.IsSuccessState()为true，所以resp肯定不为nil
				return resp.Bytes(), nil
			}
			logrus.Infof("❌ 原始URL也失败 [耗时: %v]: %v", retryElapsed, retryErr)
		}

		return nil, fmt.Errorf("下载图片失败: %w", err)
	}

	// 检测风控响应
	if d.detectBlock(resp) {
		// 由于detectBlock内部已经检查了resp != nil，这里resp肯定不为nil
		statusCode := resp.StatusCode
		d.handleBlockDetection(statusCode)
		return nil, fmt.Errorf("触发风控检测，HTTP状态码: %d", statusCode)
	}

	if resp == nil || !resp.IsSuccessState() {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		logrus.Infof("❌ HTTP错误 [耗时: %v]: 状态码=%d", elapsed, statusCode)
		return nil, fmt.Errorf("下载图片失败，HTTP状态码: %d", statusCode)
	}

	// 成功处理
	d.handleRequestSuccess()
	return resp.Bytes(), nil
}

// DownloadImageToWriter 下载图片并写入到指定的writer - 增强反风控版本
func (d *ImageDownloader) DownloadImageToWriter(url string, writer io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()

	// 检查风控状态
	if d.isBlocked() {
		blockTime := d.getBlockRemainTime()
		logrus.Infof("🚫 检测到风控阻止，等待 %v 后重试", blockTime)
		time.Sleep(blockTime)
	}

	// 应用速率限制
	d.applyRateLimit()

	// 创建动态请求
	req := d.createDynamicRequest(url)

	resp, err := req.
		SetContext(ctx).
		SetOutput(writer).
		Get(url)

	if err != nil {
		d.handleRequestError(err)
		return fmt.Errorf("下载图片失败: %w", err)
	}

	// 检测风控响应
	if d.detectBlock(resp) {
		// 由于detectBlock内部已经检查了resp != nil，这里resp肯定不为nil
		statusCode := resp.StatusCode
		d.handleBlockDetection(statusCode)
		return fmt.Errorf("触发风控检测，HTTP状态码: %d", statusCode)
	}

	if resp == nil || !resp.IsSuccessState() {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return fmt.Errorf("下载图片失败，HTTP状态码: %d", statusCode)
	}

	d.handleRequestSuccess()
	return nil
}

// GetImageInfo 获取图片信息（大小、格式等）- 增强反风控版本
func (d *ImageDownloader) GetImageInfo(url string) (*api.ImageInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()

	// 检查风控状态
	if d.isBlocked() {
		blockTime := d.getBlockRemainTime()
		logrus.Infof("🚫 检测到风控阻止，等待 %v 后重试", blockTime)
		time.Sleep(blockTime)
	}

	// 应用速率限制
	d.applyRateLimit()

	// 创建动态请求
	req := d.createDynamicRequest(url)

	resp, err := req.
		SetContext(ctx).
		Head(url)

	if err != nil {
		d.handleRequestError(err)
		return nil, fmt.Errorf("获取图片信息失败: %w", err)
	}

	// 检测风控响应
	if d.detectBlock(resp) {
		// 由于detectBlock内部已经检查了resp != nil，这里resp肯定不为nil
		statusCode := resp.StatusCode
		d.handleBlockDetection(statusCode)
		return nil, fmt.Errorf("触发风控检测，HTTP状态码: %d", statusCode)
	}

	if resp == nil || !resp.IsSuccessState() {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return nil, fmt.Errorf("获取图片信息失败，HTTP状态码: %d", statusCode)
	}

	d.handleRequestSuccess()

	// 从响应头中提取信息
	contentLength := ""
	contentType := ""
	width := ""
	height := ""

	// 由于前面已经检查了resp不为nil且状态成功，这里resp肯定不为nil
	contentLength = resp.GetHeader("Content-Length")
	contentType = resp.GetHeader("Content-Type")
	width = resp.GetHeader("Width")
	height = resp.GetHeader("Height")

	info := &api.ImageInfo{
		MimeType: contentType,
		Format:   getFormatFromMimeType(contentType),
	}

	// 解析内容长度
	// 注意：有些服务器可能不返回Content-Length头
	if contentLength != "" {
		if length, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
			info.Size = length
		}
	}

	// 解析宽度
	if width != "" {
		if w, err := strconv.Atoi(width); err == nil {
			info.Width = w
		}
	}

	// 解析高度
	if height != "" {
		if h, err := strconv.Atoi(height); err == nil {
			info.Height = h
		}
	}

	return info, nil
}

// getFormatFromMimeType 从MIME类型获取图片格式
func getFormatFromMimeType(mimeType string) string {
	switch mimeType {
	case "image/jpeg", "image/jpg":
		return "JPEG"
	case "image/png":
		return "PNG"
	case "image/gif":
		return "GIF"
	case "image/webp":
		return "WEBP"
	case "image/bmp":
		return "BMP"
	default:
		return "UNKNOWN"
	}
}

// 反风控相关方法

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
			logrus.Infof("🕐 应用速率限制，等待 %v", sleepTime)
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
	if parsedURL, err := parseURL(targetURL); err == nil {
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
		logrus.Warnf("🚨 检测风控时收到nil响应")
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

	logrus.Infof("🚨 触发风控检测 (状态码: %d)，阻止时间: %v，速率限制: %v",
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
			logrus.Infof("✅ 风控解除，关闭速率限制")
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
func parseURL(rawURL string) (*urlInfo, error) {
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

	return &urlInfo{Scheme: scheme, Host: host}, nil
}

// urlInfo URL信息结构
type urlInfo struct {
	Scheme string
	Host   string
}

// 确保ImageDownloader实现了image_downloader.ImageDownloader接口
var _ api.ImageDownloader = (*ImageDownloader)(nil)
