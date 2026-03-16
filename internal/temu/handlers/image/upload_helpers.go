// Package image 提供TEMU平台图片上传辅助方法
package image

import (
	"fmt"
	"path/filepath"
	temuimage "task-processor/internal/temu/api/image"
	temucontext "task-processor/internal/temu/context"

	"github.com/sirupsen/logrus"
)

// ImageUploadHelpers 图片上传辅助方法
type ImageUploadHelpers struct {
	logger *logrus.Entry
}

// NewImageUploadHelpers 创建新的图片上传辅助工具
func NewImageUploadHelpers() *ImageUploadHelpers {
	return &ImageUploadHelpers{
		logger: logrus.WithField("component", "ImageUploadHelpers"),
	}
}

// GetImageData 获取图片数据（优先使用填充后的图片）
func (h *ImageUploadHelpers) GetImageData(temuCtx *temucontext.TemuTaskContext, imageURL string) ([]byte, string, bool, int, int, error) {
	var imageData []byte
	var filename string
	var paddedWidth, paddedHeight int
	usePaddedImage := false

	// 检查是否有填充后的图片数据
	if temuCtx.PaddedImages != nil {
		if paddedData, hasPadded := temuCtx.PaddedImages[imageURL]; hasPadded {
			imageData = paddedData
			filename = filepath.Base(imageURL)
			if filename == "." || filename == "/" {
				filename = "padded_image.jpg"
			}
			usePaddedImage = true

			// 获取填充后的尺寸信息
			if temuCtx.PaddedImageSizes != nil {
				if size, hasSize := temuCtx.PaddedImageSizes[imageURL]; hasSize {
					paddedWidth = size[0]
					paddedHeight = size[1]
				}
			}

			h.logger.Debugf("✅ 使用填充后的图片数据: %s", imageURL)
		}
	}

	return imageData, filename, usePaddedImage, paddedWidth, paddedHeight, nil
}

// ProcessUploadResult 处理上传结果
func (h *ImageUploadHelpers) ProcessUploadResult(uploadResult *temuimage.UploadResult,
	usePaddedImage bool, paddedWidth, paddedHeight int) (string, int, int) {

	resultURL := uploadResult.ImageURL
	if resultURL == "" {
		resultURL = uploadResult.URL // 兼容处理
	}

	// 如果使用了填充图片，使用填充后的尺寸
	width := uploadResult.Width
	height := uploadResult.Height
	if usePaddedImage && paddedWidth > 0 && paddedHeight > 0 {
		width = paddedWidth
		height = paddedHeight
		h.logger.Infof("🔧 使用填充后的尺寸: %dx%d (API返回: %dx%d)",
			width, height, uploadResult.Width, uploadResult.Height)
	}

	return resultURL, width, height
}

// ValidateUploadedImages 验证已上传的图片
func (h *ImageUploadHelpers) ValidateUploadedImages(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("验证已上传的图片")

	// 获取TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	temuProduct := temuCtx.TemuProduct

	// 验证主图
	for i, img := range temuProduct.GoodsBasic.GoodsGallery.DetailImage {
		if img.URL == "" {
			return fmt.Errorf("主图[%d] URL为空", i)
		}
		if img.Width <= 0 || img.Height <= 0 {
			h.logger.Warnf("主图[%d] 尺寸信息缺失: %dx%d", i, img.Width, img.Height)
		}
	}

	// 验证SKU图片
	for skcIndex, skc := range temuProduct.SkcList {
		for skuIndex, sku := range skc.SkuList {
			for imgIndex, img := range sku.CarouselGallery {
				if img.URL == "" {
					h.logger.Warnf("SKU[%d-%d]轮播图[%d] URL为空", skcIndex, skuIndex, imgIndex)
				}
			}
			for imgIndex, img := range sku.DimensionGallery {
				if img.URL == "" {
					h.logger.Warnf("SKU[%d-%d]尺寸图[%d] URL为空", skcIndex, skuIndex, imgIndex)
				}
			}
		}
	}

	h.logger.Info("图片验证完成")
	return nil
}

// GetUploadProgress 获取上传进度
func (h *ImageUploadHelpers) GetUploadProgress(temuCtx *temucontext.TemuTaskContext) map[string]any {
	progress := map[string]any{
		"total_images":    0,
		"uploaded_images": 0,
		"failed_images":   0,
		"skipped_images":  0,
	}

	// 获取TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return progress
	}

	temuProduct := temuCtx.TemuProduct

	// 统计图片数量
	totalImages := len(temuProduct.GoodsBasic.GoodsGallery.DetailImage)
	for _, skc := range temuProduct.SkcList {
		for _, sku := range skc.SkuList {
			totalImages += len(sku.CarouselGallery) + len(sku.DimensionGallery)
		}
	}

	progress["total_images"] = totalImages
	return progress
}
