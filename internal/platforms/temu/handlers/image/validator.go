// Package image 提供TEMU平台图片验证功能
package image

import (
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// ImageValidator 图片验证器
type ImageValidator struct {
	logger             *logrus.Entry
	mainImageValidator *MainImageValidator
	skuImageValidator  *SkuImageValidator
}

// NewImageValidator 创建新的图片验证器
func NewImageValidator() *ImageValidator {
	return &ImageValidator{
		logger:             logger.GetGlobalLogger("temu.handlers.image_validator"),
		mainImageValidator: NewMainImageValidator(),
		skuImageValidator:  NewSkuImageValidator(),
	}
}

// Name 返回处理器名称
func (h *ImageValidator) Name() string {
	return "图片验证处理器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *ImageValidator) Handle(ctx pipeline.TaskContext) error {
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *ImageValidator) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始验证产品图片")

	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 根据产品类型获取图片要求
	isClothes := temuCtx.TemuProduct.GoodsBasic.IsClothes
	requirement := getImageRequirement(isClothes)
	h.logger.WithFields(logrus.Fields{
		"aspect_ratio":    requirement.AspectRatio,
		"min_width":       requirement.MinWidth,
		"min_height":      requirement.MinHeight,
		"max_size_mb":     requirement.MaxSizeMB,
		"min_image_count": requirement.MinImageCount,
		"max_image_count": requirement.MaxImageCount,
	}).Info("获取图片要求配置")

	if err := h.mainImageValidator.ValidateMainImages(temuCtx, requirement); err != nil {
		return fmt.Errorf("主图验证失败: %w", err)
	}

	if err := h.skuImageValidator.ValidateSkuImages(temuCtx, requirement); err != nil {
		return fmt.Errorf("SKU图片验证失败: %w", err)
	}

	h.logger.Info("图片验证完成")
	return nil
}

// ValidateImageUploadRequirement 验证图片上传要求
func (h *ImageValidator) ValidateImageUploadRequirement(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("检查图片上传要求")

	temuProduct := temuCtx.TemuProduct
	totalImages := len(temuProduct.GoodsBasic.GoodsGallery.DetailImage)
	for _, skc := range temuProduct.SkcList {
		for _, sku := range skc.SkuList {
			totalImages += len(sku.CarouselGallery) + len(sku.DimensionGallery)
		}
	}

	if totalImages > 0 {
		temuCtx.DefaultTaskContext.SetData("requires_image_upload", true)
		temuCtx.DefaultTaskContext.SetData("total_image_count", totalImages)
		h.logger.Infof("检测到 %d 张图片需要处理", totalImages)
	}

	return nil
}

// GetImageValidationSummary 获取图片验证摘要
func (h *ImageValidator) GetImageValidationSummary(temuCtx *temucontext.TemuTaskContext) map[string]any {
	temuProduct := temuCtx.TemuProduct
	skuImageCount := 0
	for _, skc := range temuProduct.SkcList {
		for _, sku := range skc.SkuList {
			skuImageCount += len(sku.CarouselGallery) + len(sku.DimensionGallery)
		}
	}
	mainImages := len(temuProduct.GoodsBasic.GoodsGallery.DetailImage)
	total := mainImages + skuImageCount
	return map[string]any{
		"main_images":     mainImages,
		"sku_images":      skuImageCount,
		"total_images":    total,
		"requires_upload": total > 0,
	}
}
