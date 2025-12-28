// Package impl 提供图片下载处理核心功能
package impl

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"task-processor/internal/common/management/api"
	"time"

	"github.com/sirupsen/logrus"
)

// ImageDownloadProcessor 图片下载处理器
type ImageDownloadProcessor struct {
	httpClient    *HTTPClient
	antiBot       *AntiBotManager
	rateLimit     *RateLimit
	blockDetector *BlockDetector
}

// NewImageDownloadProcessor 创建新的图片下载处理器
func NewImageDownloadProcessor(httpClient *HTTPClient, antiBot *AntiBotManager, rateLimit *RateLimit, blockDetector *BlockDetector) *ImageDownloadProcessor {
	return &ImageDownloadProcessor{
		httpClient:    httpClient,
		antiBot:       antiBot,
		rateLimit:     rateLimit,
		blockDetector: blockDetector,
	}
}

// DownloadImage 下载图片并返回图片数据
func (p *ImageDownloadProcessor) DownloadImage(url string) ([]byte, error) {
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
	if p.blockDetector.IsBlocked() {
		blockTime := p.blockDetector.GetBlockRemainTime()
		logrus.Infof("🚫 检测到风控阻止，等待 %v 后重试", blockTime)
		time.Sleep(blockTime)
	}

	// 应用速率限制
	p.rateLimit.Apply()

	// 创建动态请求
	req := p.antiBot.CreateDynamicRequest(p.httpClient.GetClient(), url)

	resp, err := req.Get(url)
	elapsed := time.Since(startTime)

	if err != nil {
		logrus.Infof("❌ 下载失败 [耗时: %v]: %v", elapsed, err)
		p.handleRequestError(err)

		// 分析错误类型
		p.analyzeError(err, elapsed)

		// 如果清理后的URL失败，尝试使用原始URL
		if originalURL != url {
			logrus.Infof("🔄 尝试原始URL: %s", originalURL)
			retryStart := time.Now()
			retryReq := p.antiBot.CreateDynamicRequest(p.httpClient.GetClient(), originalURL)
			resp, retryErr := retryReq.Get(originalURL)
			retryElapsed := time.Since(retryStart)

			if retryErr == nil && resp.IsSuccessState() {
				logrus.Infof("✅ 原始URL成功 [耗时: %v]", retryElapsed)
				p.handleRequestSuccess()
				return resp.Bytes(), nil
			}
			logrus.Infof("❌ 原始URL也失败 [耗时: %v]: %v", retryElapsed, retryErr)
		}

		return nil, fmt.Errorf("下载图片失败: %w", err)
	}

	// 检测风控响应
	if p.blockDetector.DetectBlock(resp) {
		statusCode := resp.StatusCode
		p.blockDetector.HandleBlockDetection(statusCode, p.rateLimit)
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
	p.handleRequestSuccess()
	return resp.Bytes(), nil
}

// DownloadImageToWriter 下载图片并写入到指定的writer
func (p *ImageDownloadProcessor) DownloadImageToWriter(ctx context.Context, url string, writer io.Writer) error {
	downloadCtx, cancel := context.WithTimeout(ctx, p.httpClient.GetTimeout())
	defer cancel()

	// 检查风控状态
	if p.blockDetector.IsBlocked() {
		blockTime := p.blockDetector.GetBlockRemainTime()
		logrus.Infof("🚫 检测到风控阻止，等待 %v 后重试", blockTime)
		time.Sleep(blockTime)
	}

	// 应用速率限制
	p.rateLimit.Apply()

	// 创建动态请求
	req := p.antiBot.CreateDynamicRequest(p.httpClient.GetClient(), url)

	resp, err := req.
		SetContext(downloadCtx).
		SetOutput(writer).
		Get(url)

	if err != nil {
		p.handleRequestError(err)
		return fmt.Errorf("下载图片失败: %w", err)
	}

	// 检测风控响应
	if p.blockDetector.DetectBlock(resp) {
		statusCode := resp.StatusCode
		p.blockDetector.HandleBlockDetection(statusCode, p.rateLimit)
		return fmt.Errorf("触发风控检测，HTTP状态码: %d", statusCode)
	}

	if resp == nil || !resp.IsSuccessState() {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return fmt.Errorf("下载图片失败，HTTP状态码: %d", statusCode)
	}

	p.handleRequestSuccess()
	return nil
}

// GetImageInfo 获取图片信息（大小、格式等）
func (p *ImageDownloadProcessor) GetImageInfo(ctx context.Context, url string) (*api.ImageInfo, error) {
	infoCtx, cancel := context.WithTimeout(ctx, p.httpClient.GetTimeout())
	defer cancel()

	// 检查风控状态
	if p.blockDetector.IsBlocked() {
		blockTime := p.blockDetector.GetBlockRemainTime()
		logrus.Infof("🚫 检测到风控阻止，等待 %v 后重试", blockTime)
		time.Sleep(blockTime)
	}

	// 应用速率限制
	p.rateLimit.Apply()

	// 创建动态请求
	req := p.antiBot.CreateDynamicRequest(p.httpClient.GetClient(), url)

	resp, err := req.
		SetContext(infoCtx).
		Head(url)

	if err != nil {
		p.handleRequestError(err)
		return nil, fmt.Errorf("获取图片信息失败: %w", err)
	}

	// 检测风控响应
	if p.blockDetector.DetectBlock(resp) {
		statusCode := resp.StatusCode
		p.blockDetector.HandleBlockDetection(statusCode, p.rateLimit)
		return nil, fmt.Errorf("触发风控检测，HTTP状态码: %d", statusCode)
	}

	if resp == nil || !resp.IsSuccessState() {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return nil, fmt.Errorf("获取图片信息失败，HTTP状态码: %d", statusCode)
	}

	p.handleRequestSuccess()

	// 从响应头中提取信息
	info := p.extractImageInfo(resp)
	return info, nil
}

// extractImageInfo 从响应头中提取图片信息
func (p *ImageDownloadProcessor) extractImageInfo(resp interface{}) *api.ImageInfo {
	// 这里需要根据实际的响应类型来实现
	// 由于原代码中使用了req.Response，我们需要适配
	contentLength := ""
	contentType := ""
	width := ""
	height := ""

	// 假设resp有GetHeader方法
	if r, ok := resp.(interface {
		GetHeader(string) string
	}); ok {
		contentLength = r.GetHeader("Content-Length")
		contentType = r.GetHeader("Content-Type")
		width = r.GetHeader("Width")
		height = r.GetHeader("Height")
	}

	info := &api.ImageInfo{
		MimeType: contentType,
		Format:   getFormatFromMimeType(contentType),
	}

	// 解析内容长度
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

	return info
}

// analyzeError 分析错误类型
func (p *ImageDownloadProcessor) analyzeError(err error, elapsed time.Duration) {
	errStr := err.Error()
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		logrus.Infof("   🕐 超时分析: 设置超时=%v, 实际耗时=%v", p.httpClient.GetTimeout(), elapsed)
		logrus.Infof("   🌐 网络建议: Amazon CDN在中国访问较慢，建议检查网络连接")
	} else if strings.Contains(errStr, "connection") {
		logrus.Infof("   🔌 连接分析: 网络连接问题，可能需要代理")
	} else if strings.Contains(errStr, "dns") || strings.Contains(errStr, "no such host") {
		logrus.Infof("   🔍 DNS分析: 域名解析问题")
	}
}

// handleRequestError 处理请求错误
func (p *ImageDownloadProcessor) handleRequestError(err error) {
	// 对于某些错误，可能需要调整策略
	errStr := err.Error()
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		// 超时错误，可能需要放宽速率限制
		p.rateLimit.RelaxOnTimeout()
	}
}

// handleRequestSuccess 处理请求成功
func (p *ImageDownloadProcessor) handleRequestSuccess() {
	// 成功请求后，逐渐放松限制
	p.rateLimit.RelaxOnSuccess()
	p.blockDetector.ResetOnSuccess()
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
