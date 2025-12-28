// Package handlers 提供TEMU平台主图验证功能
package handlers

import (
	"fmt"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/services"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// MainImageValidator 主图验证器（重构后使用服务层）
type MainImageValidator struct {
	logger            *logrus.Entry
	configService     *services.ImageConfigService
	parallelValidator *ParallelImageValidator
}

// NewMainImageValidator 创建新的主图验证器
func NewMainImageValidator() *MainImageValidator {
	return &MainImageValidator{
		logger:            logrus.WithField("component", "MainImageValidator"),
		configService:     services.NewImageConfigService(),
		parallelValidator: NewParallelImageValidator(),
	}
}

// Name 返回处理器名称
func (v *MainImageValidator) Name() string {
	return "主图验证器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (v *MainImageValidator) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return v.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (v *MainImageValidator) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	// 使用默认的图片要求
	requirement := services.ImageRequirement{
		MinImageCount: 1,
		MaxImageCount: 10,
		MinWidth:      800,
		MinHeight:     800,
		MaxSizeMB:     5.0,
		AspectRatio:   1.0,
	}
	return v.ValidateMainImages(temuCtx, requirement)
}

// ValidateMainImages 验证商品主图（并行处理）
func (v *MainImageValidator) ValidateMainImages(temuCtx *temucontext.TemuTaskContext, requirement services.ImageRequirement) error {
	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	mainImages := temuCtx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage

	if len(mainImages) == 0 {
		v.logger.Warn("未找到商品主图")
		return nil
	}

	v.logger.Infof("🔄 开始并行验证 %d 张主图", len(mainImages))

	// 使用并行处理
	results := v.parallelValidator.ValidateImagesInParallel(mainImages, "主图", requirement)

	validImages := []types.ImageInfo{}
	paddedImagesMap := make(map[string][]byte) // URL -> 填充后的图片数据
	paddedSizesMap := make(map[string][2]int)  // URL -> [宽度, 高度]

	for i, result := range results {
		img := mainImages[i]
		if result.IsValid {
			// 更新图片信息
			if result.NeedsPadding {
				// 使用填充后的尺寸
				img.Width = result.PaddedWidth
				img.Height = result.PaddedHeight
				// 保存填充后的图片数据和尺寸
				paddedImagesMap[img.URL] = result.PaddedImage
				paddedSizesMap[img.URL] = [2]int{result.PaddedWidth, result.PaddedHeight}
			} else {
				img.Width = result.Width
				img.Height = result.Height
			}
			validImages = append(validImages, img)
		} else {
			v.logger.Warnf("主图[%d] 验证失败，将被过滤: %v", i, result.Violations)
		}
	}

	// 检查图片数量限制
	if len(validImages) > requirement.MaxImageCount {
		validImages = validImages[:requirement.MaxImageCount]
	}

	if len(validImages) < requirement.MinImageCount {
		return fmt.Errorf("主图数量不足: %d < %d", len(validImages), requirement.MinImageCount)
	}

	// 保存填充后的图片数据和尺寸到强类型上下文
	if len(paddedImagesMap) > 0 {
		if temuCtx.PaddedImages == nil {
			temuCtx.PaddedImages = make(map[string][]byte)
		}
		if temuCtx.PaddedImageSizes == nil {
			temuCtx.PaddedImageSizes = make(map[string][2]int)
		}

		for url, data := range paddedImagesMap {
			temuCtx.PaddedImages[url] = data
		}
		for url, size := range paddedSizesMap {
			temuCtx.PaddedImageSizes[url] = size
		}
	}

	// 更新有效图片
	temuCtx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage = validImages

	return nil
}
