package downloader

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math/rand"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// ImageDownloader 图片下载器 - 基于SHEIN实现的增强反风控版本
type ImageDownloader struct {
	httpClient    *req.Client
	timeout       time.Duration
	userAgents    []string
	rateLimit     *RateLimit
	blockDetector *BlockDetector
	logger        *logrus.Entry
	processor     *ImageProcessor // 图片处理器V2
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

// NewImageDownloader 创建新的图片下载器 - 参考SHEIN实现
func NewImageDownloader() *ImageDownloader {
	timeout := 30 * time.Second

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

	// 创建增强的HTTP客户端 - 完全参考SHEIN的实现
	client := req.C().
		SetTLSFingerprintChrome(). // 关键：使用Chrome TLS指纹
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
		logger:    logrus.WithField("component", "ImageDownloader"),
		processor: NewImageProcessor(), // 初始化图片处理器V2
	}

	return downloader
}

// DownloadImage 下载图片数据 - 增强反风控版本，完全参考SHEIN实现
func (d *ImageDownloader) DownloadImage(imageURL string) ([]byte, string, error) {
	startTime := time.Now()
	originalURL := imageURL

	// 清理URL中的双引号和其他异常字符
	imageURL = strings.Trim(imageURL, "\"") // 去除前后的双引号
	imageURL = strings.TrimSpace(imageURL)  // 去除前后空格

	// 预检查URL格式
	if imageURL == "" {
		return nil, "", fmt.Errorf("图片URL为空")
	}

	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return nil, "", fmt.Errorf("无效的图片URL格式: %s", imageURL)
	}

	// 检查是否被风控阻止
	if d.isBlocked() {
		blockTime := d.getBlockRemainTime()
		d.logger.Printf("🚫 检测到风控阻止，等待 %v 后重试", blockTime)
		time.Sleep(blockTime)
	}

	// 应用速率限制
	d.applyRateLimit()

	// 创建动态请求
	req := d.createDynamicRequest(imageURL)

	resp, err := req.Get(imageURL)
	elapsed := time.Since(startTime)

	if err != nil {
		d.logger.Printf("❌ 下载失败 [耗时: %v]: %v", elapsed, err)
		d.handleRequestError(err)

		// 分析错误类型
		errStr := err.Error()
		if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
			d.logger.Printf("   🕐 超时分析: 设置超时=%v, 实际耗时=%v", d.timeout, elapsed)
			d.logger.Printf("   🌐 网络建议: Amazon CDN在中国访问较慢，建议检查网络连接")
		} else if strings.Contains(errStr, "connection") {
			d.logger.Printf("   🔌 连接分析: 网络连接问题，可能需要代理")
		} else if strings.Contains(errStr, "dns") || strings.Contains(errStr, "no such host") {
			d.logger.Printf("   🔍 DNS分析: 域名解析问题")
		}

		// 如果清理后的URL失败，尝试使用原始URL
		if originalURL != imageURL {
			d.logger.Printf("🔄 尝试原始URL: %s", originalURL)
			retryStart := time.Now()
			retryReq := d.createDynamicRequest(originalURL)
			resp, retryErr := retryReq.Get(originalURL)
			retryElapsed := time.Since(retryStart)

			if retryErr == nil && resp.IsSuccessState() {
				d.logger.Printf("✅ 原始URL成功 [耗时: %v]", retryElapsed)
				d.handleRequestSuccess()
				// 由于resp.IsSuccessState()为true，所以resp肯定不为nil
				return resp.Bytes(), d.getFilenameFromURL(originalURL), nil
			}
			d.logger.Printf("❌ 原始URL也失败 [耗时: %v]: %v", retryElapsed, retryErr)
		}

		return nil, "", fmt.Errorf("下载图片失败: %w", err)
	}

	// 检测风控响应
	if d.detectBlock(resp) {
		// 由于detectBlock内部已经检查了resp != nil，这里resp肯定不为nil
		statusCode := resp.StatusCode
		d.handleBlockDetection(statusCode)
		return nil, "", fmt.Errorf("触发风控检测，HTTP状态码: %d", statusCode)
	}

	if resp == nil || !resp.IsSuccessState() {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		d.logger.Printf("❌ HTTP错误 [耗时: %v]: 状态码=%d", elapsed, statusCode)
		return nil, "", fmt.Errorf("下载图片失败，HTTP状态码: %d", statusCode)
	}

	// 验证图片数据
	imageData := resp.Bytes()
	if len(imageData) == 0 {
		return nil, "", fmt.Errorf("下载的图片数据为空")
	}

	// 检查图片大小限制 (例如最大10MB)
	maxSize := 10 * 1024 * 1024 // 10MB
	if len(imageData) > maxSize {
		return nil, "", fmt.Errorf("图片文件过大: %d bytes (最大: %d bytes)", len(imageData), maxSize)
	}

	// 成功处理
	d.handleRequestSuccess()
	filename := d.getFilenameFromURL(imageURL)

	return imageData, filename, nil
}

// GetImageInfo 获取图片信息（宽高和大小）- 只下载图片头部数据以提高性能
func (d *ImageDownloader) GetImageInfo(imageURL string) (width, height int, size int64, err error) {
	startTime := time.Now()
	originalURL := imageURL

	// 清理URL中的双引号和其他异常字符
	imageURL = strings.Trim(imageURL, "\"")
	imageURL = strings.TrimSpace(imageURL)

	// 预检查URL格式
	if imageURL == "" {
		return 0, 0, 0, fmt.Errorf("图片URL为空")
	}

	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return 0, 0, 0, fmt.Errorf("无效的图片URL格式: %s", imageURL)
	}

	// 检查是否被风控阻止
	if d.isBlocked() {
		blockTime := d.getBlockRemainTime()
		d.logger.Printf("🚫 检测到风控阻止，等待 %v 后重试", blockTime)
		time.Sleep(blockTime)
	}

	// 应用速率限制
	d.applyRateLimit()

	// 创建动态请求，使用Range请求只下载前8KB数据（足够解析图片头部）
	req := d.createDynamicRequest(imageURL).
		SetHeader("Range", "bytes=0-8191") // 只下载前8KB

	resp, err := req.Get(imageURL)
	elapsed := time.Since(startTime)

	if err != nil {
		d.logger.Printf("❌ 获取图片信息失败 [耗时: %v]: %v", elapsed, err)
		d.handleRequestError(err)

		// 如果清理后的URL失败，尝试使用原始URL
		if originalURL != imageURL {
			d.logger.Printf("🔄 尝试原始URL: %s", originalURL)
			retryStart := time.Now()
			retryReq := d.createDynamicRequest(originalURL).
				SetHeader("Range", "bytes=0-8191")
			resp, retryErr := retryReq.Get(originalURL)
			retryElapsed := time.Since(retryStart)

			if retryErr == nil && (resp.IsSuccessState() || resp.StatusCode == 206) {
				d.logger.Printf("✅ 原始URL成功 [耗时: %v]", retryElapsed)
				d.handleRequestSuccess()
				return d.parseImageConfig(resp, originalURL)
			}
			d.logger.Printf("❌ 原始URL也失败 [耗时: %v]: %v", retryElapsed, retryErr)
		}

		return 0, 0, 0, fmt.Errorf("获取图片信息失败: %w", err)
	}

	// 检测风控响应
	if d.detectBlock(resp) {
		statusCode := resp.StatusCode
		d.handleBlockDetection(statusCode)
		return 0, 0, 0, fmt.Errorf("触发风控检测，HTTP状态码: %d", statusCode)
	}

	// 检查响应状态（200 OK 或 206 Partial Content）
	if resp == nil || (!resp.IsSuccessState() && resp.StatusCode != 206) {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		d.logger.Printf("❌ HTTP错误 [耗时: %v]: 状态码=%d", elapsed, statusCode)
		return 0, 0, 0, fmt.Errorf("获取图片信息失败，HTTP状态码: %d", statusCode)
	}

	// 成功处理
	d.handleRequestSuccess()
	return d.parseImageConfig(resp, imageURL)
}

// parseImageConfig 解析图片配置信息
func (d *ImageDownloader) parseImageConfig(resp *req.Response, _ string) (width, height int, size int64, err error) {
	// 获取完整文件大小
	size = resp.ContentLength
	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		// 解析 Content-Range: bytes 0-8191/123456 格式
		if strings.Contains(contentRange, "/") {
			parts := strings.Split(contentRange, "/")
			if len(parts) == 2 && parts[1] != "*" {
				if totalSize, parseErr := fmt.Sscanf(parts[1], "%d", &size); parseErr != nil || totalSize != 1 {
					d.logger.Printf("⚠️ 无法解析Content-Range中的文件大小: %s", contentRange)
				}
			}
		}
	}

	// 解析图片头部获取尺寸
	imageData := resp.Bytes()
	if len(imageData) == 0 {
		return 0, 0, 0, fmt.Errorf("获取的图片数据为空")
	}

	// 使用bytes.NewReader创建io.Reader
	reader := bytes.NewReader(imageData)

	// 解析图片配置
	config, _, err := image.DecodeConfig(reader)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("解析图片配置失败: %w", err)
	}

	width = config.Width
	height = config.Height

	return width, height, size, nil
}

// getFilenameFromURL 从URL获取文件名
func (d *ImageDownloader) getFilenameFromURL(imageURL string) string {
	parts := strings.Split(imageURL, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		// 移除查询参数
		if idx := strings.Index(filename, "?"); idx != -1 {
			filename = filename[:idx]
		}
		// 如果没有扩展名，默认使用.jpg
		if filepath.Ext(filename) == "" {
			filename += ".jpg"
		}
		return filename
	}
	return "image.jpg"
}

// DownloadImageForPlatform 为特定平台下载并处理图片，生成不同的MD5
func (d *ImageDownloader) DownloadImageForPlatform(imageURL, platform string) ([]byte, string, string, error) {
	// 先下载原始图片
	originalData, filename, err := d.DownloadImage(imageURL)
	if err != nil {
		return nil, "", "", err
	}

	// 为平台处理图片
	processedData, md5Hash, err := d.processor.ProcessAndGetMD5(originalData, platform)
	if err != nil {
		d.logger.Printf("⚠️ 图片处理失败，使用原始数据: %v", err)
		// 如果处理失败，返回原始数据
		originalMD5 := d.processor.GetImageMD5(originalData)
		return originalData, filename, originalMD5, nil
	}

	d.logger.Printf("✅ 图片已为平台 %s 处理，MD5: %s", platform, md5Hash)
	return processedData, filename, md5Hash, nil
}

// DownloadImageForPlatformUnique 为特定平台下载并处理图片，每次都生成唯一MD5
func (d *ImageDownloader) DownloadImageForPlatformUnique(imageURL, platform string) ([]byte, string, string, error) {
	// 先下载原始图片
	originalData, filename, err := d.DownloadImage(imageURL)
	if err != nil {
		return nil, "", "", err
	}

	// 使用多重策略处理图片，确保每次都不同
	processedData, md5Hash, err := d.processor.ProcessWithMultipleStrategies(originalData, platform)
	if err != nil {
		d.logger.Printf("⚠️ 图片处理失败，使用原始数据: %v", err)
		// 如果处理失败，返回原始数据
		originalMD5 := d.processor.GetImageMD5(originalData)
		return originalData, filename, originalMD5, nil
	}

	d.logger.Printf("✅ 图片已为平台 %s 处理（唯一），MD5: %s", platform, md5Hash)
	return processedData, filename, md5Hash, nil
}

// DownloadImageWithMD5 下载图片并返回MD5值
func (d *ImageDownloader) DownloadImageWithMD5(imageURL string) ([]byte, string, string, error) {
	data, filename, err := d.DownloadImage(imageURL)
	if err != nil {
		return nil, "", "", err
	}

	md5Hash := d.processor.GetImageMD5(data)
	return data, filename, md5Hash, nil
}
