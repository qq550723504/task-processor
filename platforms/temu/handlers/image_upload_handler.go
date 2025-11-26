package handlers

import (
	"fmt"
	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// ImageUploadHandler 图片上传处理器
type ImageUploadHandler struct {
	logger *logrus.Entry
}

// HandlerUploadResult 处理器上传结果
type HandlerUploadResult struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// NewImageUploadHandler 创建新的图片上传处理器
func NewImageUploadHandler() *ImageUploadHandler {
	return &ImageUploadHandler{
		logger: logrus.WithField("handler", "ImageUploadHandler"),
	}
}

// Name 返回处理器名称
func (h *ImageUploadHandler) Name() string {
	return "图片上传处理器"
}

// Handle 处理任务
func (h *ImageUploadHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始上传产品图片")

	if err := h.validateContext(ctx); err != nil {
		return err
	}

	if err := h.processProductImages(ctx); err != nil {
		h.logger.Errorf("上传产品图片失败: %v", err)
		return fmt.Errorf("上传产品图片失败: %w", err)
	}

	h.logger.Info("产品图片上传完成")
	return nil
}

// validateContext 验证上下文
func (h *ImageUploadHandler) validateContext(ctx *pipeline.TaskContext) error {
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	if ctx.APIClient == nil {
		return fmt.Errorf("API客户端未初始化")
	}

	return nil
}

// processProductImages 处理产品图片上传
func (h *ImageUploadHandler) processProductImages(ctx *pipeline.TaskContext) error {
	imageURLs := h.extractImageURLs(ctx)
	if len(imageURLs) == 0 {
		h.logger.Warn("没有找到需要上传的图片")
		return nil
	}

	// 上传主图
	if err := h.uploadMainImage(ctx, imageURLs); err != nil {
		h.logger.Errorf("上传主图失败: %v", err)
	}

	// 上传详情图片
	detailImages := h.uploadDetailImages(ctx, imageURLs)

	// 设置详情图片到产品
	ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage = detailImages

	return nil
}

// extractImageURLs 提取图片URL列表
func (h *ImageUploadHandler) extractImageURLs(ctx *pipeline.TaskContext) []string {
	var imageURLs []string
	if ctx.AmazonProduct != nil && len(ctx.AmazonProduct.Images) > 0 {
		imageURLs = ctx.AmazonProduct.Images
	}
	return imageURLs
}

// uploadMainImage 上传主图
func (h *ImageUploadHandler) uploadMainImage(ctx *pipeline.TaskContext, imageURLs []string) error {
	if len(imageURLs) == 0 {
		return fmt.Errorf("没有图片可上传")
	}

	mainImageURL, err := h.uploadSingleImage(ctx, imageURLs[0], "main")
	if err != nil {
		return err
	}

	ctx.TemuProduct.GoodsBasic.HdThumbURL = mainImageURL
	return nil
}

// uploadDetailImages 上传详情图片
func (h *ImageUploadHandler) uploadDetailImages(ctx *pipeline.TaskContext, imageURLs []string) []types.ImageInfo {
	var detailImages []types.ImageInfo
	maxImages := 10 // 限制最多10张图片

	for i, imageURL := range imageURLs {
		if i >= maxImages {
			break
		}

		uploadResult, err := h.uploadSingleImageWithDetails(ctx, imageURL, "detail")
		if err != nil {
			h.logger.Errorf("上传详情图片失败 [%d]: %v", i+1, err)
			continue
		}

		imageInfo := types.ImageInfo{
			Type:   intPtr(1), // 1表示普通图片
			URL:    uploadResult.URL,
			Width:  uploadResult.Width,
			Height: uploadResult.Height,
		}
		detailImages = append(detailImages, imageInfo)

	}

	return detailImages
}

// uploadSingleImage 上传单张图片，返回URL
func (h *ImageUploadHandler) uploadSingleImage(ctx *pipeline.TaskContext, imageURL, imageType string) (string, error) {
	result, err := h.uploadSingleImageWithDetails(ctx, imageURL, imageType)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

// uploadSingleImageWithDetails 上传单张图片，返回完整结果
func (h *ImageUploadHandler) uploadSingleImageWithDetails(ctx *pipeline.TaskContext, imageURL, imageType string) (*HandlerUploadResult, error) {

	// 使用与ImageUploadProcessor相同的上传逻辑
	processor := NewImageUploadProcessor()
	imageInfo, err := processor.uploadSingleImage(ctx, imageURL, imageType)
	if err != nil {
		return nil, fmt.Errorf("上传图片失败: %w", err)
	}

	// 转换为HandlerUploadResult格式
	result := &HandlerUploadResult{
		URL:    imageInfo.URL,
		Width:  imageInfo.Width,
		Height: imageInfo.Height,
	}

	return result, nil
}

// intPtr 返回int指针
func intPtr(i int) *int {
	return &i
}
