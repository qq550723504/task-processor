// Package impl 提供图片下载处理核心功能
package impl

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"task-processor/internal/infra/clients/management/api"
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

	url = strings.Trim(url, "\"")
	url = strings.TrimSpace(url)

	if url == "" {
		return nil, fmt.Errorf("图片URL为空")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("无效的图片URL格式: %s", url)
	}

	if p.blockDetector.IsBlocked() {
		time.Sleep(p.blockDetector.GetBlockRemainTime())
	}

	p.rateLimit.Apply()

	req := p.antiBot.CreateDynamicRequest(p.httpClient.GetClient(), url)
	resp, err := req.Get(url)
	elapsed := time.Since(startTime)

	if err != nil {
		logrus.Infof("❌ 下载失败 [耗时: %v]: %v", elapsed, err)
		p.handleRequestError(err)

		if originalURL != url {
			retryReq := p.antiBot.CreateDynamicRequest(p.httpClient.GetClient(), originalURL)
			retryResp, retryErr := retryReq.Get(originalURL)
			if retryErr == nil && retryResp.IsSuccessState() {
				p.handleRequestSuccess()
				return retryResp.Bytes(), nil
			}
		}
		return nil, fmt.Errorf("下载图片失败: %w", err)
	}

	if p.blockDetector.DetectBlock(resp) {
		p.blockDetector.HandleBlockDetection(resp.StatusCode, p.rateLimit)
		return nil, fmt.Errorf("触发风控检测，HTTP状态码: %d", resp.StatusCode)
	}

	if resp == nil || !resp.IsSuccessState() {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return nil, fmt.Errorf("下载图片失败，HTTP状态码: %d", statusCode)
	}

	p.handleRequestSuccess()
	return resp.Bytes(), nil
}

// DownloadImageToWriter 下载图片并写入到指定的writer
func (p *ImageDownloadProcessor) DownloadImageToWriter(ctx context.Context, url string, writer io.Writer) error {
	downloadCtx, cancel := context.WithTimeout(ctx, p.httpClient.GetTimeout())
	defer cancel()

	if p.blockDetector.IsBlocked() {
		time.Sleep(p.blockDetector.GetBlockRemainTime())
	}
	p.rateLimit.Apply()

	req := p.antiBot.CreateDynamicRequest(p.httpClient.GetClient(), url)
	resp, err := req.SetContext(downloadCtx).SetOutput(writer).Get(url)
	if err != nil {
		p.handleRequestError(err)
		return fmt.Errorf("下载图片失败: %w", err)
	}

	if p.blockDetector.DetectBlock(resp) {
		p.blockDetector.HandleBlockDetection(resp.StatusCode, p.rateLimit)
		return fmt.Errorf("触发风控检测，HTTP状态码: %d", resp.StatusCode)
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

// GetImageInfo 获取图片信息
func (p *ImageDownloadProcessor) GetImageInfo(ctx context.Context, url string) (*api.ImageInfo, error) {
	infoCtx, cancel := context.WithTimeout(ctx, p.httpClient.GetTimeout())
	defer cancel()

	if p.blockDetector.IsBlocked() {
		time.Sleep(p.blockDetector.GetBlockRemainTime())
	}
	p.rateLimit.Apply()

	req := p.antiBot.CreateDynamicRequest(p.httpClient.GetClient(), url)
	resp, err := req.SetContext(infoCtx).Head(url)
	if err != nil {
		p.handleRequestError(err)
		return nil, fmt.Errorf("获取图片信息失败: %w", err)
	}

	if p.blockDetector.DetectBlock(resp) {
		p.blockDetector.HandleBlockDetection(resp.StatusCode, p.rateLimit)
		return nil, fmt.Errorf("触发风控检测，HTTP状态码: %d", resp.StatusCode)
	}

	if resp == nil || !resp.IsSuccessState() {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return nil, fmt.Errorf("获取图片信息失败，HTTP状态码: %d", statusCode)
	}

	p.handleRequestSuccess()
	return p.extractImageInfo(resp), nil
}

func (p *ImageDownloadProcessor) extractImageInfo(resp interface{}) *api.ImageInfo {
	contentLength, contentType, width, height := "", "", "", ""

	if r, ok := resp.(interface{ GetHeader(string) string }); ok {
		contentLength = r.GetHeader("Content-Length")
		contentType = r.GetHeader("Content-Type")
		width = r.GetHeader("Width")
		height = r.GetHeader("Height")
	}

	info := &api.ImageInfo{
		MimeType: contentType,
		Format:   getFormatFromMimeType(contentType),
	}

	if contentLength != "" {
		if length, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
			info.Size = length
		}
	}
	if width != "" {
		if w, err := strconv.Atoi(width); err == nil {
			info.Width = w
		}
	}
	if height != "" {
		if h, err := strconv.Atoi(height); err == nil {
			info.Height = h
		}
	}

	return info
}

func (p *ImageDownloadProcessor) handleRequestError(err error) {
	if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
		p.rateLimit.RelaxOnTimeout()
	}
}

func (p *ImageDownloadProcessor) handleRequestSuccess() {
	p.rateLimit.RelaxOnSuccess()
	p.blockDetector.ResetOnSuccess()
}

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
