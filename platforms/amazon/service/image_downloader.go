// Package service 提供Amazon平台业务逻辑
package service

import (
	"fmt"
	"task-processor/common/downloader"

	"github.com/sirupsen/logrus"
)

// ImageDownloader Amazon图片下载器（封装通用下载器）
type ImageDownloader struct {
	downloader *downloader.ImageDownloader
	logger     *logrus.Entry
}

// NewImageDownloader 创建图片下载器
func NewImageDownloader() *ImageDownloader {
	return &ImageDownloader{
		downloader: downloader.NewImageDownloader(),
		logger:     logrus.WithField("service", "AmazonImageDownloader"),
	}
}

// Download 下载单张图片
func (d *ImageDownloader) Download(url string) ([]byte, error) {
	d.logger.Infof("开始下载图片: %s", url)

	data, _, err := d.downloader.DownloadImage(url)
	if err != nil {
		return nil, fmt.Errorf("下载图片失败: %w", err)
	}

	d.logger.Infof("图片下载成功，大小: %d bytes", len(data))
	return data, nil
}

// DownloadMultiple 批量下载图片
func (d *ImageDownloader) DownloadMultiple(urls []string) ([][]byte, error) {
	results := make([][]byte, 0, len(urls))

	for i, url := range urls {
		d.logger.Infof("下载图片 [%d/%d]: %s", i+1, len(urls), url)

		data, err := d.Download(url)
		if err != nil {
			d.logger.Warnf("下载图片失败 [%d/%d]: %v", i+1, len(urls), err)
			continue
		}

		results = append(results, data)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("所有图片下载失败")
	}

	d.logger.Infof("成功下载 %d/%d 张图片", len(results), len(urls))
	return results, nil
}
