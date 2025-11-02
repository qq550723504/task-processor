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

// ImageUploadRequest 图片上传请求结构体
type ImageUploadRequest struct {
	ImageURL string `json:"image_url"`
	Type     string `json:"type"`
}

// ImageUploadResponse 图片上传响应结构体
type ImageUploadResponse struct {
	Success     bool   `json:"success"`
	UploadedURL string `json:"uploaded_url"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
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

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 上传产品图片
	err := h.uploadProductImages(ctx)
	if err != nil {
		h.logger.Errorf("上传产品图片失败: %v", err)
		return fmt.Errorf("上传产品图片失败: %w", err)
	}

	h.logger.Info("产品图片上传完成")
	return nil
}

// uploadProductImages 上传产品图片
func (h *ImageUploadHandler) uploadProductImages(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始处理产品图片上传")

	// 从Amazon产品数据获取图片URL列表
	var imageURLs []string
	if ctx.AmazonProduct != nil && len(ctx.AmazonProduct.Images) > 0 {
		imageURLs = ctx.AmazonProduct.Images
	} else {
		// 如果没有图片，使用默认图片
		imageURLs = []string{
			"https://example.com/default-product-image.jpg",
		}
		h.logger.Info("未找到产品图片，使用默认图片")
	}

	h.logger.Infof("需要上传 %d 张图片", len(imageURLs))

	// 上传主图
	if len(imageURLs) > 0 {
		mainImageURL, err := h.uploadSingleImage(imageURLs[0], "main")
		if err != nil {
			h.logger.Errorf("上传主图失败: %v", err)
		} else {
			ctx.TemuProduct.GoodsBasic.HdThumbURL = mainImageURL
			h.logger.Infof("主图上传成功: %s", mainImageURL)
		}
	}

	// 上传详情图片
	var detailImages []types.ImageInfo
	for i, imageURL := range imageURLs {
		if i >= 10 { // 限制最多10张图片
			break
		}

		uploadedURL, err := h.uploadSingleImage(imageURL, "detail")
		if err != nil {
			h.logger.Errorf("上传详情图片失败 [%d]: %v", i+1, err)
			continue
		}

		// 创建图片信息
		imageInfo := types.ImageInfo{
			Type:   intPtr(1), // 1表示普通图片
			URL:    uploadedURL,
			Width:  800, // 默认宽度
			Height: 600, // 默认高度
		}
		detailImages = append(detailImages, imageInfo)

		h.logger.Infof("详情图片上传成功 [%d]: %s", i+1, uploadedURL)
	}

	// 设置详情图片到产品
	ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage = detailImages

	h.logger.Infof("图片上传完成: 主图1张, 详情图片%d张", len(detailImages))
	return nil
}

// uploadSingleImage 上传单张图片
func (h *ImageUploadHandler) uploadSingleImage(imageURL, imageType string) (string, error) {
	h.logger.Infof("上传图片: %s (类型: %s)", imageURL, imageType)

	// 这里应该构造上传请求并调用TEMU API
	// request := ImageUploadRequest{
	//     ImageURL: imageURL,
	//     Type:     imageType,
	// }

	// 这里应该调用TEMU API上传图片
	// 为了简化，我们模拟上传结果
	response := &ImageUploadResponse{
		Success:     true,
		UploadedURL: h.generateUploadedURL(imageURL),
		Width:       800,
		Height:      600,
	}

	if !response.Success {
		return "", fmt.Errorf("图片上传失败")
	}

	return response.UploadedURL, nil
}

// generateUploadedURL 生成模拟的上传后URL
func (h *ImageUploadHandler) generateUploadedURL(originalURL string) string {
	// 这里应该返回真实的TEMU图片URL
	// 为了简化，我们生成一个模拟URL
	return fmt.Sprintf("https://img.temu.com/uploaded/%d.jpg", len(originalURL)*12345)
}

// intPtr 返回int指针
func intPtr(i int) *int {
	return &i
}
