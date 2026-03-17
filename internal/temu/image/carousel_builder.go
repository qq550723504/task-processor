// Package image 提供TEMU平台轮播图片构建功能
package image

import (
	"task-processor/internal/model"
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"

	"github.com/sirupsen/logrus"
)

// ImageCarouselBuilder 轮播图片构建器
type ImageCarouselBuilder struct {
	uploadProcessor  *ImageUploadProcessor
	paddingProcessor *ImagePaddingProcessor
	uploadUtils      *ImageUploadUtils
	logger           *logrus.Entry
}

// NewImageCarouselBuilder 创建新的轮播图片构建器
func NewImageCarouselBuilder() *ImageCarouselBuilder {
	return &ImageCarouselBuilder{
		uploadProcessor:  NewImageUploadProcessor(),
		paddingProcessor: NewImagePaddingProcessor(),
		uploadUtils:      NewImageUploadUtils(),
		logger:           logrus.WithField("component", "ImageCarouselBuilder"),
	}
}

// BuildCarouselImagesWithoutAnnotation 构建轮播图片（排除标注过的图片）
func (icb *ImageCarouselBuilder) BuildCarouselImagesWithoutAnnotation(temuCtx *temucontext.TemuTaskContext, variant *model.Product) []models.ImageInfo {
	// 收集需要上传的图片URL，但排除用于标注的图片
	var imageURLs []string

	// 获取用于标注的图片URL（通常是第3张或第1张）
	var annotationImageURL string
	if len(variant.Images) >= 3 && variant.Images[2] != "" {
		annotationImageURL = variant.Images[2]
	} else if len(variant.Images) > 0 && variant.Images[0] != "" {
		annotationImageURL = variant.Images[0]
	}

	// 添加所有图片，但排除用于标注的图片
	for _, img := range variant.Images {
		if img != "" && img != annotationImageURL {
			imageURLs = append(imageURLs, img)
		}
	}

	if len(imageURLs) == 0 {
		icb.logger.Warn("⚠️ 排除标注图片后，没有可用的轮播图片")
		return []models.ImageInfo{}
	}

	// 限制图片数量不超过9张（为尺寸图预留1张位置）
	const maxImages = 9
	if len(imageURLs) > maxImages {
		imageURLs = imageURLs[:maxImages]
		icb.logger.Warnf("⚠️ 轮播图数量超限，从%d截断为%d张", len(imageURLs), maxImages)
	}

	// 先进行图片填充处理
	icb.uploadUtils.padImagesIfNeeded(temuCtx, imageURLs)

	// 批量上传图片到TEMU，失败时使用降级处理
	icb.logger.Infof("📤 开始上传%d张轮播图（已排除标注图片）", len(imageURLs))
	return icb.uploadUtils.batchUploadImagesWithFallback(temuCtx, imageURLs, "carousel", 1500, 1500)
}

// BuildVariantImagesWithUpload 构建变体图片并上传到TEMU
func (icb *ImageCarouselBuilder) BuildVariantImagesWithUpload(temuCtx *temucontext.TemuTaskContext, variant *model.Product) []models.ImageInfo {
	// 收集需要上传的图片URL
	var imageURLs []string
	for _, img := range variant.Images {
		if img != "" {
			imageURLs = append(imageURLs, img)
		}
	}

	if len(imageURLs) == 0 {
		return []models.ImageInfo{}
	}

	// 限制图片数量不超过10张
	const maxImages = 10
	if len(imageURLs) > maxImages {
		imageURLs = imageURLs[:maxImages]
	}

	// 先进行图片填充处理
	icb.uploadUtils.padImagesIfNeeded(temuCtx, imageURLs)

	// 批量上传图片到TEMU，失败时使用降级处理
	return icb.uploadUtils.batchUploadImagesWithFallback(temuCtx, imageURLs, "carousel", 1500, 1500)
}

// GetProductImagesWithUpload 获取产品图片并上传到TEMU
func (icb *ImageCarouselBuilder) GetProductImagesWithUpload(temuCtx *temucontext.TemuTaskContext) []models.ImageInfo {
	// 收集需要上传的图片URL
	var imageURLs []string

	// 从强类型上下文获取Amazon产品信息
	if amazonProduct := temuCtx.GetAmazonProduct(); amazonProduct != nil {
		for _, img := range amazonProduct.Images {
			if img != "" {
				imageURLs = append(imageURLs, img)
			}
		}
	}

	// 限制图片数量不超过10张
	const maxImages = 10
	if len(imageURLs) > maxImages {
		imageURLs = imageURLs[:maxImages]
	}

	// 先进行图片填充处理
	icb.uploadUtils.padImagesIfNeeded(temuCtx, imageURLs)

	// 批量上传图片到TEMU，失败时使用降级处理
	return icb.uploadUtils.batchUploadImagesWithFallback(temuCtx, imageURLs, "main", 800, 800)
}

