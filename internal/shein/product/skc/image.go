// Package skc 提供SHEIN平台SKC图片处理功能
package skc

import (
	"task-processor/internal/core/logger"
	"slices"
	"task-processor/internal/shein"
	"task-processor/internal/shein/product/image"

)

// SKCImageHandler SKC图片处理器
type SKCImageHandler struct {
	imageProcessor *image.ImageProcessor
	taskContext    *shein.TaskContext
}

// NewSKCImageHandler 创建新的SKC图片处理器
func NewSKCImageHandler(imageProcessor *image.ImageProcessor, taskContext *shein.TaskContext) *SKCImageHandler {
	return &SKCImageHandler{
		imageProcessor: imageProcessor,
		taskContext:    taskContext,
	}
}

// GetVariantSpecificImages 从变体数据中获取变体特定的图片
func (h *SKCImageHandler) GetVariantSpecificImages(ctx *shein.TaskContext, variant shein.Variant) ([]string, error) {
	// 如果变体ASIN与主产品相同，使用主产品图片
	if variant.ASIN == ctx.AmazonProduct.Asin {
		logger.GetGlobalLogger("shein/product").Infof("变体ASIN与主产品相同，使用主产品图片，ASIN: %s", variant.ASIN)
		return ctx.AmazonProduct.Images, nil
	}

	// 尝试从fallback产品的变体数据中查找图片
	for _, variation := range *ctx.Variants {
		if variation.Asin == variant.ASIN {
			if len(variation.Images) >= 3 {
				return variation.Images, nil
			} else if len(variation.Images) > 0 {
				// 如果变体图片少于3张，补充主产品图片
				combinedImages := make([]string, len(variation.Images))
				copy(combinedImages, variation.Images)
				for _, img := range ctx.AmazonProduct.Images {
					// 避免重复添加相同的图片
					if !slices.Contains(combinedImages, img) {
						combinedImages = append(combinedImages, img)
					}
				}
				return combinedImages, nil
			}
			// 找到匹配的变体但没有图片，跳出循环继续查找其他变体
			break
		}
	}

	// 如果没有找到特定变体的图片，返回主产品图片
	logger.GetGlobalLogger("shein/product").Infof("未找到变体 %s 的特定图片，使用主产品图片", variant.ASIN)
	return ctx.AmazonProduct.Images, nil
}
