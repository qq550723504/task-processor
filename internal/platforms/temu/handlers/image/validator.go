// Package handlers 提供TEMU平台图片验证功能
package image

import (
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/pipeline"
	models "task-processor/internal/platforms/temu/api/product"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/services"

	"github.com/sirupsen/logrus"
)

// ImageValidator 图片验证器（重构后使用服务层）
type ImageValidator struct {
	logger             *logrus.Entry
	validationService  *services.ImageValidationService
	mainImageValidator *MainImageValidator
	skuImageValidator  *SkuImageValidator
}

// NewImageValidator 创建新的图片验证器
func NewImageValidator() *ImageValidator {
	return &ImageValidator{
		logger:             logger.GetGlobalLogger("temu.handlers.image_validator"),
		validationService:  services.NewImageValidationService(),
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
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *ImageValidator) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始验证产品图片")

	// 获取TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 获取图片要求配置（使用服务层）
	// 创建临时适配器来满足服务接口要求
	productProvider := &temuProductProvider{temuCtx: temuCtx}
	requirement := h.validationService.GetImageRequirement(productProvider)
	h.logger.WithFields(logrus.Fields{
		"aspect_ratio":    requirement.AspectRatio,
		"min_width":       requirement.MinWidth,
		"min_height":      requirement.MinHeight,
		"max_size_mb":     requirement.MaxSizeMB,
		"min_image_count": requirement.MinImageCount,
		"max_image_count": requirement.MaxImageCount,
	}).Info("获取图片要求配置")

	// 验证商品主图
	if err := h.mainImageValidator.ValidateMainImages(temuCtx, requirement); err != nil {
		return fmt.Errorf("主图验证失败: %w", err)
	}

	// 验证SKC/SKU图片
	if err := h.skuImageValidator.ValidateSkuImages(temuCtx, requirement); err != nil {
		return fmt.Errorf("SKU图片验证失败: %w", err)
	}

	// 设置需要上传图片的标志（直接赋值到强类型上下文）
	// 可以添加一个标志字段到TemuTaskContext中
	h.logger.Info("图片验证完成")
	return nil
}

// ValidateImageUploadRequirement 验证图片上传要求
func (h *ImageValidator) ValidateImageUploadRequirement(temuCtx *temucontext.TemuTaskContext) error {
	productProvider := &temuProductProvider{temuCtx: temuCtx}
	return h.validationService.ValidateImageUploadRequirement(productProvider)
}

// GetImageValidationSummary 获取图片验证摘要
func (h *ImageValidator) GetImageValidationSummary(temuCtx *temucontext.TemuTaskContext) map[string]any {
	productProvider := &temuProductProvider{temuCtx: temuCtx}
	return h.validationService.GetValidationSummary(productProvider)
}

// temuProductProvider 临时适配器，用于满足服务层接口要求
type temuProductProvider struct {
	temuCtx *temucontext.TemuTaskContext
}

// GetTemuProduct 实现 TemuProductProvider 接口
func (p *temuProductProvider) GetTemuProduct() *models.Product {
	return p.temuCtx.TemuProduct
}

// SetData 实现 TemuProductProvider 接口
func (p *temuProductProvider) SetData(key string, value any) {
	// 可以根据需要将数据存储到强类型上下文的相应字段中
	// 这里暂时使用基础上下文的SetData方法
	p.temuCtx.DefaultTaskContext.SetData(key, value)
}

// GetData 实现 TemuProductProvider 接口
func (p *temuProductProvider) GetData(key string) (any, bool) {
	// 可以根据需要从强类型上下文的相应字段中获取数据
	// 这里暂时使用基础上下文的GetData方法
	return p.temuCtx.DefaultTaskContext.GetData(key)
}
