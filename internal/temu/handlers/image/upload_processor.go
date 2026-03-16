// Package image 提供TEMU平台图片上传核心处理器
package image

import (
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/ptr"
	temuimage "task-processor/internal/temu/api/image"
	temuproduct "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"

	"github.com/sirupsen/logrus"
)

// ImageUploadProcessor 图片上传处理器（简化版）
type ImageUploadProcessor struct {
	logger *logrus.Entry
}

// NewImageUploadProcessor 创建新的图片上传处理器
func NewImageUploadProcessor() *ImageUploadProcessor {
	return &ImageUploadProcessor{
		logger: logger.GetGlobalLogger("temu.handlers.image_upload"),
	}
}

// Name 返回处理器名称
func (h *ImageUploadProcessor) Name() string {
	return "图片上传处理器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *ImageUploadProcessor) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *ImageUploadProcessor) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始处理图片上传")

	// 检查API客户端
	if temuCtx.APIClient == nil {
		return fmt.Errorf("TEMU API客户端为空")
	}

	// 上传主图
	if err := h.uploadMainImages(temuCtx); err != nil {
		return fmt.Errorf("上传主图失败: %w", err)
	}

	// 上传SKU图片
	if err := h.uploadSkuImages(temuCtx); err != nil {
		return fmt.Errorf("上传SKU图片失败: %w", err)
	}

	h.logger.Info("图片上传处理完成")
	return nil
}

// UploadSingleImage 上传单张图片（核心方法）
func (h *ImageUploadProcessor) UploadSingleImage(temuCtx *temucontext.TemuTaskContext, imageURL, imageType string) (*temuproduct.ImageInfo, error) {
	// 检查是否需要上传
	if !needsUpload(imageURL) {
		// 如果是TEMU的CDN地址，直接使用
		width, height := h.getDefaultImageDimensions(imageType)
		return h.createImageInfo(imageURL, width, height), nil
	}

	// 检查API客户端
	if temuCtx.APIClient == nil {
		return nil, fmt.Errorf("TEMU API客户端为空")
	}

	// 获取上传签名
	signature, err := getUploadSignature(temuCtx.APIClient)
	if err != nil {
		return nil, fmt.Errorf("获取上传签名失败: %w", err)
	}

	// 获取图片数据
	imageData, filename, err := h.getImageData(temuCtx, imageURL)
	if err != nil {
		return nil, fmt.Errorf("获取图片数据失败: %w", err)
	}

	// 上传图片
	uploadResult, err := uploadImageWithSignature(temuCtx.APIClient, imageData, filename, signature)
	if err != nil {
		return nil, fmt.Errorf("上传图片失败: %w", err)
	}

	// 处理上传结果
	imageInfo := h.processUploadResult(temuCtx, imageURL, uploadResult)

	return imageInfo, nil
}

// BatchUploadImages 批量上传图片
func (h *ImageUploadProcessor) BatchUploadImages(temuCtx *temucontext.TemuTaskContext, imageURLs []string, imageType string) ([]*temuproduct.ImageInfo, error) {
	var results []*temuproduct.ImageInfo

	for i, url := range imageURLs {
		uploadedImg, err := h.UploadSingleImage(temuCtx, url, imageType)
		if err != nil {
			h.logger.WithError(err).WithFields(logrus.Fields{
				"image_index": i + 1,
				"image_url":   url,
			}).Error("批量上传图片失败")
			continue
		}
		results = append(results, uploadedImg)
	}

	h.logger.WithFields(logrus.Fields{
		"success_count": len(results),
		"total_count":   len(imageURLs),
	}).Info("批量上传完成")
	return results, nil
}

// uploadMainImages 上传主图
func (h *ImageUploadProcessor) uploadMainImages(temuCtx *temucontext.TemuTaskContext) error {
	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	mainImages := temuCtx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage
	if len(mainImages) == 0 {
		return nil
	}

	var imageURLs []string
	for _, img := range mainImages {
		if img.URL != "" {
			imageURLs = append(imageURLs, img.URL)
		}
	}

	// 批量上传主图
	uploadedImages, err := h.BatchUploadImages(temuCtx, imageURLs, "main")
	if err != nil {
		return err
	}

	// 更新主图信息
	for i, uploadedImg := range uploadedImages {
		if i < len(temuCtx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage) {
			temuCtx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage[i] = *uploadedImg
		}
	}

	return nil
}

// uploadSkuImages 上传SKU图片
func (h *ImageUploadProcessor) uploadSkuImages(temuCtx *temucontext.TemuTaskContext) error {
	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	for skcIndex := range temuCtx.TemuProduct.SkcList {
		for skuIndex := range temuCtx.TemuProduct.SkcList[skcIndex].SkuList {
			sku := &temuCtx.TemuProduct.SkcList[skcIndex].SkuList[skuIndex]

			// 上传轮播图
			if err := h.uploadCarouselImages(temuCtx, sku); err != nil {
				return fmt.Errorf("上传SKU[%d-%d]轮播图失败: %w", skcIndex, skuIndex, err)
			}

			// 上传尺寸图
			if err := h.uploadDimensionImages(temuCtx, sku); err != nil {
				return fmt.Errorf("上传SKU[%d-%d]尺寸图失败: %w", skcIndex, skuIndex, err)
			}
		}
	}

	return nil
}

// uploadCarouselImages 上传轮播图
func (h *ImageUploadProcessor) uploadCarouselImages(temuCtx *temucontext.TemuTaskContext, sku *temuproduct.Sku) error {
	if len(sku.CarouselGallery) == 0 {
		return nil
	}

	var imageURLs []string
	for _, img := range sku.CarouselGallery {
		if img.URL != "" {
			imageURLs = append(imageURLs, img.URL)
		}
	}

	uploadedImages, err := h.BatchUploadImages(temuCtx, imageURLs, "carousel")
	if err != nil {
		return err
	}

	// 更新轮播图信息
	for i, uploadedImg := range uploadedImages {
		if i < len(sku.CarouselGallery) {
			sku.CarouselGallery[i] = *uploadedImg
		}
	}

	return nil
}

// uploadDimensionImages 上传尺寸图
func (h *ImageUploadProcessor) uploadDimensionImages(temuCtx *temucontext.TemuTaskContext, sku *temuproduct.Sku) error {
	if len(sku.DimensionGallery) == 0 {
		return nil
	}

	var imageURLs []string
	for _, img := range sku.DimensionGallery {
		if img.URL != "" {
			imageURLs = append(imageURLs, img.URL)
		}
	}

	uploadedImages, err := h.BatchUploadImages(temuCtx, imageURLs, "dimension")
	if err != nil {
		return err
	}

	// 更新尺寸图信息
	for i, uploadedImg := range uploadedImages {
		if i < len(sku.DimensionGallery) {
			sku.DimensionGallery[i] = *uploadedImg
		}
	}

	return nil
}

// getImageData 获取图片数据
func (h *ImageUploadProcessor) getImageData(temuCtx *temucontext.TemuTaskContext, imageURL string) ([]byte, string, error) {
	// 优先使用填充后的图片数据
	if temuCtx.PaddedImages != nil {
		if data, found := temuCtx.PaddedImages[imageURL]; found {
			return data, "padded_image.jpg", nil
		}
	}

	// 如果没有填充数据，下载原图
	return downloadImage(imageURL)
}

// processUploadResult 处理上传结果
func (h *ImageUploadProcessor) processUploadResult(temuCtx *temucontext.TemuTaskContext, imageURL string, uploadResult *temuimage.UploadResult) *temuproduct.ImageInfo {
	// 获取填充信息
	var width, height int
	if temuCtx.PaddedImageSizes != nil {
		if size, found := temuCtx.PaddedImageSizes[imageURL]; found {
			width, height = size[0], size[1]
		}
	}

	// 如果没有填充尺寸，使用上传结果的尺寸
	if width == 0 || height == 0 {
		width, height = uploadResult.Width, uploadResult.Height
	}

	// 获取产品信息以判断是否为服装类
	var isClothes bool
	if temuCtx.TemuProduct != nil {
		isClothes = temuCtx.TemuProduct.GoodsBasic.IsClothes
	}

	// 根据产品类型调整尺寸
	width, height = h.adjustImageDimensions(width, height, isClothes)

	return h.createImageInfo(uploadResult.URL, width, height)
}

// getDefaultImageDimensions 获取默认图片尺寸
func (h *ImageUploadProcessor) getDefaultImageDimensions(imageType string) (int, int) {
	switch imageType {
	case "main", "carousel":
		return 1500, 1500
	case "dimension":
		return 1500, 1500
	default:
		return 800, 800
	}
}

// createImageInfo 创建图片信息
func (h *ImageUploadProcessor) createImageInfo(url string, width, height int) *temuproduct.ImageInfo {
	return &temuproduct.ImageInfo{
		URL:    url,
		Width:  width,
		Height: height,
		Type:   ptr.IntPtr(1),
	}
}

// adjustImageDimensions 根据产品类型调整图片尺寸
func (h *ImageUploadProcessor) adjustImageDimensions(width, height int, isClothes bool) (int, int) {
	if isClothes {
		// 服装类产品：3:4比例
		if width > height {
			height = width * 4 / 3
		} else {
			width = height * 3 / 4
		}
	} else {
		// 其他产品：1:1比例
		if width != height {
			size := h.maxInt(width, height)
			width, height = size, size
		}
	}
	return width, height
}

// maxInt 返回两个整数中的较大值
func (h *ImageUploadProcessor) maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
