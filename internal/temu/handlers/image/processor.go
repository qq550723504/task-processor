// Package image 提供TEMU平台图片处理核心功能
package image

import (
	"fmt"
	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"

	"github.com/sirupsen/logrus"
)

// ImageProcessor 图片处理器（重构后的核心协调器）
type ImageProcessor struct {
	carouselBuilder  *ImageCarouselBuilder
	dimensionBuilder *ImageDimensionBuilder
	uploadUtils      *ImageUploadUtils
	logger           *logrus.Entry
}

// NewImageProcessor 创建新的图片处理器
func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		carouselBuilder:  NewImageCarouselBuilder(),
		dimensionBuilder: NewImageDimensionBuilder(),
		uploadUtils:      NewImageUploadUtils(),
		logger:           logrus.WithField("component", "ImageProcessor"),
	}
}

// BuildCarouselImagesWithoutAnnotation 构建轮播图片（排除标注过的图片）
func (ip *ImageProcessor) BuildCarouselImagesWithoutAnnotation(temuCtx *temucontext.TemuTaskContext, variant *model.Product) ([]models.ImageInfo, error) {

	return ip.carouselBuilder.BuildCarouselImagesWithoutAnnotation(temuCtx, variant), nil
}

// BuildVariantImagesWithUpload 构建变体图片并上传到TEMU
func (ip *ImageProcessor) BuildVariantImagesWithUpload(ctx pipeline.TaskContext, variant *model.Product) ([]models.ImageInfo, error) {
	// 类型断言：将通用上下文转换为TEMU强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return nil, fmt.Errorf("上下文类型错误：期望 *TemuTaskContext，实际 %T", ctx)
	}

	return ip.carouselBuilder.BuildVariantImagesWithUpload(temuCtx, variant), nil
}

// BuildDimensionImages 构建尺寸图片（通常是第一张图片）
func (ip *ImageProcessor) BuildDimensionImages(variant *model.Product) []models.ImageInfo {
	return ip.dimensionBuilder.BuildDimensionImages(variant)
}

// BuildMainImageWithDimensionAnnotation 为主图添加尺寸标注（专用于主图展示）
func (ip *ImageProcessor) BuildMainImageWithDimensionAnnotation(ctx pipeline.TaskContext, variant *model.Product) ([]models.ImageInfo, error) {
	// 类型断言：将通用上下文转换为TEMU强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return nil, fmt.Errorf("上下文类型错误：期望 *TemuTaskContext，实际 %T", ctx)
	}

	return ip.dimensionBuilder.BuildMainImageWithDimensionAnnotation(temuCtx, variant), nil
}

// BuildDimensionImagesWithUpload 构建尺寸图片并上传到TEMU（检测所有图片，优先使用已有标注的图片）
func (ip *ImageProcessor) BuildDimensionImagesWithUpload(temuCtx *temucontext.TemuTaskContext, variant *model.Product) ([]models.ImageInfo, error) {

	return ip.dimensionBuilder.BuildDimensionImagesWithUpload(temuCtx, variant), nil
}

// GetProductImagesWithUpload 获取产品图片并上传到TEMU
func (ip *ImageProcessor) GetProductImagesWithUpload(temuCtx *temucontext.TemuTaskContext) ([]models.ImageInfo, error) {

	return ip.carouselBuilder.GetProductImagesWithUpload(temuCtx), nil
}

