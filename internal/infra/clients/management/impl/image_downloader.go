// Package impl 提供管理服务的具体实现
package impl

import (
	"context"
	"io"
	"task-processor/internal/infra/clients/management/api"
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
	return &ImageDownloader{
		httpClient:    NewHTTPClient(timeout),
		antiBot:       NewAntiBotManager(),
		rateLimit:     NewRateLimit(),
		blockDetector: NewBlockDetector(),
	}
}

// DownloadImage 下载图片并返回图片数据
func (d *ImageDownloader) DownloadImage(url string) ([]byte, error) {
	processor := NewImageDownloadProcessor(d.httpClient, d.antiBot, d.rateLimit, d.blockDetector)
	return processor.DownloadImage(url)
}

// DownloadImageToWriter 下载图片并写入到指定的writer
func (d *ImageDownloader) DownloadImageToWriter(ctx context.Context, url string, writer io.Writer) error {
	processor := NewImageDownloadProcessor(d.httpClient, d.antiBot, d.rateLimit, d.blockDetector)
	return processor.DownloadImageToWriter(ctx, url, writer)
}

// GetImageInfo 获取图片信息
func (d *ImageDownloader) GetImageInfo(ctx context.Context, url string) (*api.ImageInfo, error) {
	processor := NewImageDownloadProcessor(d.httpClient, d.antiBot, d.rateLimit, d.blockDetector)
	return processor.GetImageInfo(ctx, url)
}

var _ api.ImageDownloader = (*ImageDownloader)(nil)
