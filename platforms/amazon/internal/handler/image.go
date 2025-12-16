// Package handler 提供Amazon图片处理器
package handler

import (
	"context"
	"fmt"
	"task-processor/platforms/amazon/internal/model"
	"task-processor/platforms/amazon/service"
)

// ImageHandler 图片处理器
type ImageHandler struct {
	*BaseHandler
	downloader *service.ImageDownloader
	processor  *service.ImageProcessor
	uploader   *service.S3Uploader
}

// NewImageHandler 创建图片处理器
func NewImageHandler() *ImageHandler {
	return &ImageHandler{
		BaseHandler: NewBaseHandler("图片处理器"),
	}
}

// Execute 处理逻辑
func (h *ImageHandler) Execute(services *model.Services, data map[string]any) error {
	h.logger.Info("开始处理产品图片")

	// 初始化服务
	if err := h.initServices(services); err != nil {
		return fmt.Errorf("初始化服务失败: %w", err)
	}

	// 获取解析后的产品数据
	rawData, exists := data["raw_product_data"]
	if !exists {
		return fmt.Errorf("产品数据不存在")
	}

	productData, ok := rawData.(map[string]any)
	if !ok {
		return fmt.Errorf("产品数据格式错误")
	}

	// 提取图片URL列表
	imageURLs := h.extractImageURLs(productData)
	if len(imageURLs) == 0 {
		return fmt.Errorf("未找到产品图片")
	}

	h.logger.Infof("找到 %d 张图片", len(imageURLs))

	// 下载图片
	images, err := h.downloadImages(imageURLs)
	if err != nil {
		return fmt.Errorf("下载图片失败: %w", err)
	}

	// 处理图片（调整大小）
	processedImages, err := h.processImages(images)
	if err != nil {
		return fmt.Errorf("处理图片失败: %w", err)
	}

	// 获取产品ID
	productID, err := h.GetRequiredString(data, "product_id")
	if err != nil {
		return err
	}

	// 上传到S3
	s3URLs, err := h.uploadImages(productID, processedImages)
	if err != nil {
		return fmt.Errorf("上传图片失败: %w", err)
	}

	// 保存S3 URL到上下文
	h.SetResult(data, "image_urls", s3URLs)
	h.SetResult(data, "main_image_url", s3URLs[0])

	h.logger.Infof("图片处理完成，共 %d 张", len(s3URLs))
	return nil
}

// initServices 初始化服务
func (h *ImageHandler) initServices(services *model.Services) error {
	if h.downloader == nil {
		h.downloader = service.NewImageDownloader()
	}
	if h.processor == nil {
		h.processor = service.NewImageProcessor()
	}
	// S3Uploader 需要从配置中获取
	// h.uploader = services.GetS3Uploader()
	return nil
}

// extractImageURLs 提取图片URL列表
func (h *ImageHandler) extractImageURLs(data map[string]any) []string {
	var urls []string

	// 尝试从不同字段提取图片
	// 1. images 数组
	if images, ok := data["images"].([]any); ok {
		for _, img := range images {
			if url, ok := img.(string); ok && url != "" {
				urls = append(urls, url)
			}
		}
	}

	// 2. imageUrl 单个图片
	if imageURL, ok := data["imageUrl"].(string); ok && imageURL != "" {
		urls = append(urls, imageURL)
	}

	// 3. mainImage 主图
	if mainImage, ok := data["mainImage"].(string); ok && mainImage != "" {
		urls = append(urls, mainImage)
	}

	// 限制图片数量（Amazon最多9张）
	if len(urls) > 9 {
		urls = urls[:9]
	}

	return urls
}

// downloadImages 下载图片
func (h *ImageHandler) downloadImages(urls []string) ([][]byte, error) {
	return h.downloader.DownloadMultiple(urls)
}

// processImages 处理图片
func (h *ImageHandler) processImages(images [][]byte) ([][]byte, error) {
	processed := make([][]byte, 0, len(images))

	for i, imageData := range images {
		// 验证格式
		if err := h.processor.ValidateFormat(imageData); err != nil {
			h.logger.Warnf("图片格式验证失败 [%d]: %v", i+1, err)
			continue
		}

		// 调整大小（Amazon推荐至少1000x1000）
		resized, err := h.processor.Resize(imageData, 2000, 2000)
		if err != nil {
			h.logger.Warnf("调整图片大小失败 [%d]: %v", i+1, err)
			// 使用原图
			processed = append(processed, imageData)
			continue
		}

		processed = append(processed, resized)
	}

	return processed, nil
}

// uploadImages 上传图片到S3
func (h *ImageHandler) uploadImages(productID string, images [][]byte) ([]string, error) {
	if h.uploader == nil {
		return nil, fmt.Errorf("S3上传器未配置")
	}

	ctx := context.Background()
	prefix := fmt.Sprintf("products/%s", productID)

	return h.uploader.UploadMultiple(ctx, prefix, images)
}
