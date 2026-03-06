// Package utils 提供TEMU平台相关的工具方法
package utils

import (
	"fmt"
	"task-processor/internal/pkg/mathutil"
	"task-processor/internal/platforms/temu/api"

	"github.com/sirupsen/logrus"
)

// ImageDimensionValidator 图片尺寸验证器
type ImageDimensionValidator struct {
	logger *logrus.Entry
}

// NewImageDimensionValidator 创建新的图片尺寸验证器
func NewImageDimensionValidator() *ImageDimensionValidator {
	return &ImageDimensionValidator{
		logger: logrus.WithField("validator", "ImageDimensionValidator"),
	}
}

// ValidateProductImages 验证产品所有图片尺寸
func (v *ImageDimensionValidator) ValidateProductImages(product *api.Product) error {
	v.logger.Info("开始验证产品图片尺寸")

	// 验证主图
	if err := v.validateMainImages(product); err != nil {
		return fmt.Errorf("主图尺寸验证失败: %w", err)
	}

	// 验证SKU图片
	if err := v.validateSkuImages(product); err != nil {
		return fmt.Errorf("SKU图片尺寸验证失败: %w", err)
	}

	v.logger.Info("✅ 所有图片尺寸验证通过")
	return nil
}

// validateMainImages 验证主图尺寸
func (v *ImageDimensionValidator) validateMainImages(product *api.Product) error {
	mainImages := product.GoodsBasic.GoodsGallery.DetailImage
	isClothes := product.GoodsBasic.IsClothes

	for i, img := range mainImages {
		if err := v.validateSingleImage(img, fmt.Sprintf("主图[%d]", i), isClothes); err != nil {
			return err
		}
	}

	return nil
}

// validateSkuImages 验证SKU图片尺寸
func (v *ImageDimensionValidator) validateSkuImages(product *api.Product) error {
	isClothes := product.GoodsBasic.IsClothes

	for skcIndex, skc := range product.SkcList {
		for skuIndex, sku := range skc.SkuList {
			// 验证轮播图
			for imgIndex, img := range sku.CarouselGallery {
				context := fmt.Sprintf("SKU[%d-%d]轮播图[%d]", skcIndex, skuIndex, imgIndex)
				if err := v.validateSingleImage(img, context, isClothes); err != nil {
					return err
				}
			}

			// 验证尺寸图
			for imgIndex, img := range sku.DimensionGallery {
				context := fmt.Sprintf("SKU[%d-%d]尺寸图[%d]", skcIndex, skuIndex, imgIndex)
				if err := v.validateSingleImage(img, context, isClothes); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// validateSingleImage 验证单张图片尺寸
func (v *ImageDimensionValidator) validateSingleImage(img api.ImageInfo, context string, isClothes bool) error {
	if img.Width <= 0 || img.Height <= 0 {
		return fmt.Errorf("%s 尺寸信息无效: %dx%d", context, img.Width, img.Height)
	}

	if isClothes {
		// 服装类：3:4比例 (0.75)
		expectedRatio := 0.75
		actualRatio := float64(img.Width) / float64(img.Height)
		tolerance := 0.001 // 允许极小误差

		if mathutil.Abs(actualRatio-expectedRatio) > tolerance {
			return fmt.Errorf("%s 服装类图片比例不正确: %.4f (期望: %.2f)",
				context, actualRatio, expectedRatio)
		}

		// 检查最小尺寸
		if img.Width < 1340 || img.Height < 1785 {
			return fmt.Errorf("%s 服装类图片尺寸不足: %dx%d (最小: 1340x1785)",
				context, img.Width, img.Height)
		}
	} else {
		// 非服装类：1:1比例
		if img.Width != img.Height {
			return fmt.Errorf("%s 非服装类图片必须为1:1比例: %dx%d",
				context, img.Width, img.Height)
		}

		// 检查最小尺寸
		if img.Width < 800 || img.Height < 800 {
			return fmt.Errorf("%s 非服装类图片尺寸不足: %dx%d (最小: 800x800)",
				context, img.Width, img.Height)
		}
	}

	v.logger.Debugf("✅ %s 尺寸验证通过: %dx%d", context, img.Width, img.Height)
	return nil
}
