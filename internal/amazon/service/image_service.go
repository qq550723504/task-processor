// Package service 提供Amazon图片处理服务
package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"task-processor/platforms/amazon/api"
	"time"

	"github.com/sirupsen/logrus"
)

// ImageService Amazon图片服务
type ImageService struct {
	apiClient   *api.Client
	logger      *logrus.Entry
	uploadCache sync.Map // 缓存已上传的图片，避免重复上传
}

// NewImageService 创建图片服务
func NewImageService(apiClient *api.Client) *ImageService {
	return &ImageService{
		apiClient: apiClient,
		logger:    logrus.WithField("service", "ImageService"),
	}
}

// ProcessImageURL 处理图片URL，如果是外部链接则上传到Amazon
func (s *ImageService) ProcessImageURL(ctx context.Context, imageURL, marketplaceID string) (string, error) {
	if imageURL == "" {
		return "", nil
	}

	// 检查是否已经是Amazon的图片ID或内部URL
	if s.isAmazonImageID(imageURL) {
		s.logger.WithField("image_url", imageURL).Debug("已经是Amazon图片ID，无需处理")
		return imageURL, nil
	}

	// 检查缓存
	if cachedID, exists := s.uploadCache.Load(imageURL); exists {
		s.logger.WithField("image_url", imageURL).Debug("使用缓存的图片ID")
		return cachedID.(string), nil
	}

	// 下载并上传图片
	imageID, err := s.downloadAndUploadImage(ctx, imageURL, marketplaceID)
	if err != nil {
		return "", fmt.Errorf("处理图片失败: %w", err)
	}

	// 缓存结果
	s.uploadCache.Store(imageURL, imageID)

	return imageID, nil
}

// ProcessImageURLs 批量处理图片URL
func (s *ImageService) ProcessImageURLs(ctx context.Context, imageURLs []string, marketplaceID string) ([]string, error) {
	if len(imageURLs) == 0 {
		return nil, nil
	}

	results := make([]string, len(imageURLs))
	errors := make([]error, len(imageURLs))

	// 并发处理图片
	var wg sync.WaitGroup
	for i, url := range imageURLs {
		wg.Add(1)
		go func(index int, imageURL string) {
			defer wg.Done()

			processedURL, err := s.ProcessImageURL(ctx, imageURL, marketplaceID)
			results[index] = processedURL
			errors[index] = err
		}(i, url)
	}

	wg.Wait()

	// 检查是否有错误
	var firstError error
	for i, err := range errors {
		if err != nil {
			s.logger.WithError(err).Errorf("处理第%d张图片失败: %s", i+1, imageURLs[i])
			if firstError == nil {
				firstError = err
			}
		}
	}

	if firstError != nil {
		return results, fmt.Errorf("部分图片处理失败: %w", firstError)
	}

	return results, nil
}

// downloadAndUploadImage 下载并上传图片
func (s *ImageService) downloadAndUploadImage(ctx context.Context, imageURL, marketplaceID string) (string, error) {
	s.logger.WithField("image_url", imageURL).Info("开始下载并上传图片")

	// 1. 下载图片
	imageData, filename, err := s.downloadImage(imageURL)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}

	// 2. 上传到Amazon
	result, err := s.apiClient.UploadImage(ctx, imageData, filename, marketplaceID)
	if err != nil {
		return "", fmt.Errorf("上传图片到Amazon失败: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"original_url": imageURL,
		"image_id":     result.ImageID,
	}).Info("图片上传成功")

	return result.ImageID, nil
}

// downloadImage 下载图片
func (s *ImageService) downloadImage(imageURL string) ([]byte, string, error) {
	// 创建HTTP客户端，设置超时
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(imageURL)
	if err != nil {
		return nil, "", fmt.Errorf("下载图片失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("下载图片失败，状态码: %d", resp.StatusCode)
	}

	// 读取图片数据
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取图片数据失败: %w", err)
	}

	// 生成文件名
	filename := s.generateFilename(imageURL, resp.Header.Get("Content-Type"))

	s.logger.WithFields(logrus.Fields{
		"url":      imageURL,
		"size":     len(imageData),
		"filename": filename,
	}).Debug("图片下载完成")

	return imageData, filename, nil
}

// generateFilename 生成文件名
func (s *ImageService) generateFilename(imageURL, contentType string) string {
	// 从URL中提取文件名
	parts := strings.Split(imageURL, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		if strings.Contains(lastPart, ".") {
			return lastPart
		}
	}

	// 根据Content-Type生成扩展名
	ext := ".jpg"
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	}

	// 生成基于时间戳的文件名
	timestamp := time.Now().Unix()
	return fmt.Sprintf("image_%d%s", timestamp, ext)
}

// isAmazonImageID 判断是否已经是Amazon的图片ID
func (s *ImageService) isAmazonImageID(imageURL string) bool {
	// Amazon图片ID通常是UUID格式或者以特定前缀开头
	if len(imageURL) < 10 {
		return false
	}

	// 如果包含Amazon域名，认为是Amazon内部URL
	amazonDomains := []string{
		"amazon.com",
		"amazonaws.com",
		"ssl-images-amazon.com",
	}

	for _, domain := range amazonDomains {
		if strings.Contains(imageURL, domain) {
			return true
		}
	}

	// 如果是纯ID格式（不包含http://或https://）
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return true
	}

	return false
}

// ValidateImageURL 验证图片URL格式
func (s *ImageService) ValidateImageURL(imageURL string) error {
	if imageURL == "" {
		return nil // 空URL是允许的
	}

	// 检查是否是有效的URL或Amazon图片ID
	if s.isAmazonImageID(imageURL) {
		return nil
	}

	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return fmt.Errorf("无效的图片URL格式: %s", imageURL)
	}

	// 检查文件扩展名
	ext := strings.ToLower(filepath.Ext(imageURL))
	validExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}

	for _, validExt := range validExts {
		if ext == validExt {
			return nil
		}
	}

	// 如果没有扩展名，也可能是有效的（某些CDN URL）
	if ext == "" {
		return nil
	}

	return fmt.Errorf("不支持的图片格式: %s", ext)
}
