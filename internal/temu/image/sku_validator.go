// Package image 提供TEMU平台SKU图片验证功能
package image

import (
	"fmt"
	"task-processor/internal/pipeline"
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// SkuImageValidator SKU图片验证器
type SkuImageValidator struct {
	logger          *logrus.Entry
	singleValidator *SingleImageValidator
}

// NewSkuImageValidator 创建新的SKU图片验证器
func NewSkuImageValidator() *SkuImageValidator {
	return &SkuImageValidator{
		logger:          logger.GetGlobalLogger("SkuImageValidator"),
		singleValidator: NewSingleImageValidator(),
	}
}

// Name 返回处理器名称
func (v *SkuImageValidator) Name() string {
	return "SKU图片验证器"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (v *SkuImageValidator) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return v.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (v *SkuImageValidator) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	// 使用默认的图片要求
	requirement := ImageRequirement{
		MaxSizeMB:     3.0,
		MinWidth:      1340,
		MinHeight:     1785,
		AspectRatio:   0.75,
		MinImageCount: 1,
		MaxImageCount: 10,
	}
	return v.ValidateSkuImages(temuCtx, requirement)
}

// ValidateSkuImages 验证SKU图片
func (v *SkuImageValidator) ValidateSkuImages(temuCtx *temucontext.TemuTaskContext, requirement ImageRequirement) error {
	totalSkuImages := 0
	totalPaddedImages := 0

	// 获取或创建填充图片映射
	if temuCtx.PaddedImages == nil {
		temuCtx.PaddedImages = make(map[string][]byte)
	}
	if temuCtx.PaddedImageSizes == nil {
		temuCtx.PaddedImageSizes = make(map[string][2]int)
	}

	// 检查TEMU产品数据
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品数据不存在")
	}

	for skcIndex, skc := range temuCtx.TemuProduct.SkcList {
		for skuIndex, sku := range skc.SkuList {
			// 验证轮播图片
			validCarouselImages := v.validateCarouselImages(sku.CarouselGallery, skcIndex, skuIndex, requirement, temuCtx.PaddedImages, temuCtx.PaddedImageSizes, &totalPaddedImages)

			// 验证尺寸图片
			validDimensionImages := v.validateDimensionImages(sku.DimensionGallery, skcIndex, skuIndex, requirement, temuCtx.PaddedImages, temuCtx.PaddedImageSizes, &totalPaddedImages)

			// 更新SKU图片
			temuCtx.TemuProduct.SkcList[skcIndex].SkuList[skuIndex].CarouselGallery = validCarouselImages
			temuCtx.TemuProduct.SkcList[skcIndex].SkuList[skuIndex].DimensionGallery = validDimensionImages

			totalSkuImages += len(validCarouselImages) + len(validDimensionImages)
		}
	}

	return nil
}

// validateCarouselImages 验证轮播图片
func (v *SkuImageValidator) validateCarouselImages(images []models.ImageInfo, skcIndex, skuIndex int, requirement ImageRequirement, paddedImagesMap map[string][]byte, paddedSizesMap map[string][2]int, totalPaddedImages *int) []models.ImageInfo {
	return v.validateSkuGallery(images, "轮播图", skcIndex, skuIndex, requirement, paddedImagesMap, paddedSizesMap, totalPaddedImages)
}

// validateDimensionImages 验证尺寸图片
func (v *SkuImageValidator) validateDimensionImages(images []models.ImageInfo, skcIndex, skuIndex int, requirement ImageRequirement, paddedImagesMap map[string][]byte, paddedSizesMap map[string][2]int, totalPaddedImages *int) []models.ImageInfo {
	return v.validateSkuGallery(images, "尺寸图", skcIndex, skuIndex, requirement, paddedImagesMap, paddedSizesMap, totalPaddedImages)
}

// validateSkuGallery 验证SKU图片集合（通用实现）
func (v *SkuImageValidator) validateSkuGallery(images []models.ImageInfo, label string, skcIndex, skuIndex int, requirement ImageRequirement, paddedImagesMap map[string][]byte, paddedSizesMap map[string][2]int, totalPaddedImages *int) []models.ImageInfo {
	validImages := []models.ImageInfo{}

	for imgIndex, img := range images {
		result := v.singleValidator.ValidateSingleImage(img.URL, fmt.Sprintf("SKU[%d-%d]%s[%d]", skcIndex, skuIndex, label, imgIndex), requirement)

		if result.IsValid {
			if result.NeedsPadding {
				img.Width = result.PaddedWidth
				img.Height = result.PaddedHeight
				paddedImagesMap[img.URL] = result.PaddedImage
				paddedSizesMap[img.URL] = [2]int{result.PaddedWidth, result.PaddedHeight}
				*totalPaddedImages++
			} else {
				img.Width = result.Width
				img.Height = result.Height
			}
			validImages = append(validImages, img)
		} else {
			v.logger.Warnf("SKU[%d-%d]%s[%d] 验证失败: %v", skcIndex, skuIndex, label, imgIndex, result.Violations)
		}
	}

	return validImages
}
