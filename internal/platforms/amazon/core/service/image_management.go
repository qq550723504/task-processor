// Package service 提供Amazon图片管理服务
package service

import (
	"context"
	"fmt"
	"sync"
	"task-processor/internal/pkg/downloader"
	"task-processor/internal/platforms/amazon/api"
	"time"

	"github.com/sirupsen/logrus"
)

// ImageManagementService Amazon图片管理服务（合并下载和上传功能）
type ImageManagementService struct {
	apiClient     *api.Client
	downloader    *downloader.ImageDownloader
	logger        *logrus.Entry
	uploadCache   sync.Map // 缓存已上传的图片，避免重复上传
	downloadCache sync.Map // 缓存已下载的图片
}

// NewImageManagementService 创建图片管理服务
func NewImageManagementService(apiClient *api.Client) *ImageManagementService {
	return &ImageManagementService{
		apiClient:  apiClient,
		downloader: downloader.NewImageDownloader(),
		logger:     logrus.WithField("service", "ImageManagementService"),
	}
}

// ImageUploadResult 图片上传结果
type ImageUploadResult struct {
	ImageID     string `json:"image_id"`
	URL         string `json:"url"`
	OriginalURL string `json:"original_url"`
	Size        int64  `json:"size"`
	Format      string `json:"format"`
}

// ImageDownloadResult 图片下载结果
type ImageDownloadResult struct {
	Data     []byte `json:"-"`
	Size     int64  `json:"size"`
	Format   string `json:"format"`
	Filename string `json:"filename"`
	MD5      string `json:"md5"` // 添加MD5字段
}

// DownloadImage 下载单张图片（为Amazon平台优化）
func (s *ImageManagementService) DownloadImage(url string) (*ImageDownloadResult, error) {
	s.logger.Infof("开始下载图片: %s", url)

	// 检查缓存
	cacheKey := fmt.Sprintf("amazon_%s", url)
	if cached, ok := s.downloadCache.Load(cacheKey); ok {
		s.logger.Info("使用缓存的图片数据")
		return cached.(*ImageDownloadResult), nil
	}

	// 使用平台特定下载器下载（为Amazon平台生成唯一MD5，每次都不同）
	data, filename, md5Hash, err := s.downloader.DownloadImageForPlatformUnique(url, "amazon")
	if err != nil {
		return nil, fmt.Errorf("下载图片失败: %w", err)
	}

	// 检测图片格式
	format := s.detectImageFormat(data)
	if format == "" {
		return nil, fmt.Errorf("不支持的图片格式")
	}

	result := &ImageDownloadResult{
		Data:     data,
		Size:     int64(len(data)),
		Format:   format,
		Filename: filename,
		MD5:      md5Hash, // 添加MD5字段
	}

	// 缓存结果
	s.downloadCache.Store(cacheKey, result)

	s.logger.WithFields(logrus.Fields{
		"size":   result.Size,
		"format": result.Format,
		"md5":    result.MD5,
	}).Info("图片下载完成")

	return result, nil
}

// DownloadImages 批量下载图片
func (s *ImageManagementService) DownloadImages(urls []string) ([]*ImageDownloadResult, error) {
	s.logger.WithField("count", len(urls)).Info("开始批量下载图片")

	results := make([]*ImageDownloadResult, 0, len(urls))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	// 并发下载，但限制并发数
	semaphore := make(chan struct{}, 5) // 最多5个并发

	for _, url := range urls {
		wg.Add(1)
		go func(imageURL string) {
			defer wg.Done()

			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			result, err := s.DownloadImage(imageURL)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errors = append(errors, fmt.Errorf("下载 %s 失败: %w", imageURL, err))
			} else {
				results = append(results, result)
			}
		}(url)
	}

	wg.Wait()

	if len(errors) > 0 {
		s.logger.WithField("error_count", len(errors)).Warn("部分图片下载失败")
		// 返回部分成功的结果和第一个错误
		return results, errors[0]
	}

	s.logger.WithField("success_count", len(results)).Info("批量图片下载完成")
	return results, nil
}

// UploadImage 上传图片到Amazon
func (s *ImageManagementService) UploadImage(ctx context.Context, imageData []byte, filename, marketplaceID string) (*ImageUploadResult, error) {
	s.logger.WithFields(logrus.Fields{
		"filename":    filename,
		"size":        len(imageData),
		"marketplace": marketplaceID,
	}).Info("开始上传图片到Amazon")

	// 生成缓存键
	cacheKey := fmt.Sprintf("%s_%s_%d", filename, marketplaceID, len(imageData))

	// 检查上传缓存
	if cached, ok := s.uploadCache.Load(cacheKey); ok {
		s.logger.Info("使用缓存的上传结果")
		return cached.(*ImageUploadResult), nil
	}

	// 调用Amazon API上传
	apiResult, err := s.apiClient.UploadImage(ctx, imageData, filename, marketplaceID)
	if err != nil {
		return nil, fmt.Errorf("上传图片到Amazon失败: %w", err)
	}

	// 检测图片格式
	format := s.detectImageFormat(imageData)

	result := &ImageUploadResult{
		ImageID:     apiResult.ImageID,
		URL:         apiResult.URL,
		OriginalURL: apiResult.OriginalURL,
		Size:        int64(len(imageData)),
		Format:      format,
	}

	// 缓存上传结果
	s.uploadCache.Store(cacheKey, result)

	s.logger.WithFields(logrus.Fields{
		"image_id": result.ImageID,
		"size":     result.Size,
	}).Info("图片上传完成")

	return result, nil
}

// UploadImages 批量上传图片
func (s *ImageManagementService) UploadImages(ctx context.Context, images []ImageUploadRequest, marketplaceID string) ([]*ImageUploadResult, error) {
	s.logger.WithField("count", len(images)).Info("开始批量上传图片")

	results := make([]*ImageUploadResult, 0, len(images))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	// 限制并发上传数量
	semaphore := make(chan struct{}, 3) // 最多3个并发上传

	for _, img := range images {
		wg.Add(1)
		go func(image ImageUploadRequest) {
			defer wg.Done()

			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			result, err := s.UploadImage(ctx, image.Data, image.Filename, marketplaceID)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errors = append(errors, fmt.Errorf("上传 %s 失败: %w", image.Filename, err))
			} else {
				results = append(results, result)
			}
		}(img)
	}

	wg.Wait()

	if len(errors) > 0 {
		s.logger.WithField("error_count", len(errors)).Warn("部分图片上传失败")
		return results, errors[0]
	}

	s.logger.WithField("success_count", len(results)).Info("批量图片上传完成")
	return results, nil
}

// ImageUploadRequest 图片上传请求
type ImageUploadRequest struct {
	Data     []byte
	Filename string
}

// DownloadAndUpload 下载并上传图片（一站式服务）
func (s *ImageManagementService) DownloadAndUpload(ctx context.Context, imageURL, marketplaceID string) (*ImageUploadResult, error) {
	s.logger.WithFields(logrus.Fields{
		"url":         imageURL,
		"marketplace": marketplaceID,
	}).Info("开始下载并上传图片")

	// 1. 下载图片
	downloadResult, err := s.DownloadImage(imageURL)
	if err != nil {
		return nil, fmt.Errorf("下载图片失败: %w", err)
	}

	// 2. 上传图片
	uploadResult, err := s.UploadImage(ctx, downloadResult.Data, downloadResult.Filename, marketplaceID)
	if err != nil {
		return nil, fmt.Errorf("上传图片失败: %w", err)
	}

	// 3. 设置原始URL
	uploadResult.OriginalURL = imageURL

	s.logger.WithField("image_id", uploadResult.ImageID).Info("图片下载并上传完成")
	return uploadResult, nil
}

// ProcessProductImages 处理产品图片（批量下载并上传）
func (s *ImageManagementService) ProcessProductImages(ctx context.Context, imageURLs []string, marketplaceID string) ([]*ImageUploadResult, error) {
	s.logger.WithFields(logrus.Fields{
		"count":       len(imageURLs),
		"marketplace": marketplaceID,
	}).Info("开始处理产品图片")

	var results []*ImageUploadResult
	var errors []error

	for i, url := range imageURLs {
		s.logger.WithFields(logrus.Fields{
			"index": i + 1,
			"total": len(imageURLs),
			"url":   url,
		}).Info("处理图片")

		result, err := s.DownloadAndUpload(ctx, url, marketplaceID)
		if err != nil {
			s.logger.WithError(err).Warnf("处理图片 %d 失败", i+1)
			errors = append(errors, err)
			continue
		}

		results = append(results, result)

		// 添加延迟，避免过快的API调用
		if i < len(imageURLs)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if len(errors) > 0 && len(results) == 0 {
		return nil, fmt.Errorf("所有图片处理失败，第一个错误: %w", errors[0])
	}

	s.logger.WithFields(logrus.Fields{
		"success_count": len(results),
		"error_count":   len(errors),
	}).Info("产品图片处理完成")

	return results, nil
}

// detectImageFormat 检测图片格式
func (s *ImageManagementService) detectImageFormat(data []byte) string {
	if len(data) < 12 {
		return ""
	}

	// JPEG
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return "jpeg"
	}

	// PNG
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return "png"
	}

	// GIF
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return "gif"
	}

	// WebP
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "webp"
	}

	return ""
}

// ClearCache 清理缓存
func (s *ImageManagementService) ClearCache() {
	s.uploadCache = sync.Map{}
	s.downloadCache = sync.Map{}
	s.logger.Info("图片缓存已清理")
}

// GetCacheStats 获取缓存统计
func (s *ImageManagementService) GetCacheStats() map[string]int {
	uploadCount := 0
	downloadCount := 0

	s.uploadCache.Range(func(_, _ interface{}) bool {
		uploadCount++
		return true
	})

	s.downloadCache.Range(func(_, _ interface{}) bool {
		downloadCount++
		return true
	})

	return map[string]int{
		"upload_cache":   uploadCount,
		"download_cache": downloadCount,
	}
}
