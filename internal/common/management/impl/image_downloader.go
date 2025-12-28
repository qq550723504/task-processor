// Package impl 提供管理服务的具体实现
package impl

import (
	"context"
	"io"
	"task-processor/internal/common/management/api"
	"time"
)

// ImageDownloader 图片下载客户端实现
type ImageDownloader struct {
	httpClient    *HTTPClient
	antiBot       *AntiBotManager
	rateLimit     *RateLimit
	blockDetector *BlockDetector
}

// NewImageDownloader 创建新的图片下载客户端
func NewImageDownloader(timeout time.Duration) *ImageDownloader {

	downloader := &ImageDownloader{
		httpClient:    NewHTTPClient(timeout),
		antiBot:       NewAntiBotManager(),
		rateLimit:     NewRateLimit(),
		blockDetector: NewBlockDetector(),
	}

	return downloader
}

// DownloadImage 下载图片并返回图片数据 - 增强反风控版本
func (d *ImageDownloader) DownloadImage(url string) ([]byte, error) {
	processor := NewImageDownloadProcessor(d.httpClient, d.antiBot, d.rateLimit, d.blockDetector)
	return processor.DownloadImage(url)
}

// DownloadImageToWriter 下载图片并写入到指定的writer - 增强反风控版本
func (d *ImageDownloader) DownloadImageToWriter(ctx context.Context, url string, writer io.Writer) error {
	processor := NewImageDownloadProcessor(d.httpClient, d.antiBot, d.rateLimit, d.blockDetector)
	return processor.DownloadImageToWriter(ctx, url, writer)
}

// GetImageInfo 获取图片信息（大小、格式等）- 增强反风控版本
func (d *ImageDownloader) GetImageInfo(ctx context.Context, url string) (*api.ImageInfo, error) {
	processor := NewImageDownloadProcessor(d.httpClient, d.antiBot, d.rateLimit, d.blockDetector)
	return processor.GetImageInfo(ctx, url)
}

// 确保ImageDownloader实现了image_downloader.ImageDownloader接口
var _ api.ImageDownloader = (*ImageDownloader)(nil)
