package api

import "io"

// ImageDownloader 图片下载客户端接口
type ImageDownloader interface {
	// DownloadImage 下载图片并返回图片数据
	DownloadImage(url string) ([]byte, error)

	// DownloadImageToWriter 下载图片并写入到指定的writer
	DownloadImageToWriter(url string, writer io.Writer) error

	// GetImageInfo 获取图片信息（大小、格式等）
	GetImageInfo(url string) (*ImageInfo, error)
}

// ImageInfo 图片信息结构
type ImageInfo struct {
	Size     int64  // 图片大小（字节）
	Format   string // 图片格式（JPEG, PNG等）
	Width    int    // 图片宽度
	Height   int    // 图片高度
	MimeType string // MIME类型
}
