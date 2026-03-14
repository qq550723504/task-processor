package api

import (
	"context"
	"io"
)

// ImageDownloader 图片下载客户端接口
type ImageDownloader interface {
	DownloadImage(url string) ([]byte, error)
	DownloadImageToWriter(ctx context.Context, url string, writer io.Writer) error
	GetImageInfo(ctx context.Context, url string) (*ImageInfo, error)
}

// ImageInfo 图片信息结构
type ImageInfo struct {
	Size     int64
	Format   string
	Width    int
	Height   int
	MimeType string
}
