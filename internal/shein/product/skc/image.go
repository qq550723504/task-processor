// Package skc 提供SHEIN平台SKC图片处理功能
package skc

import (
	"slices"

	"task-processor/internal/core/logger"
	"task-processor/internal/shein"
	"task-processor/internal/shein/product/image"
)

// SKCImageHandler SKC图片处理器
type SKCImageHandler struct {
	imageProcessor *image.ImageProcessor
	runtime        *SKCRuntimeInput
}

// NewSKCImageHandler 创建新的SKC图片处理器
func NewSKCImageHandler(imageProcessor *image.ImageProcessor, runtime *SKCRuntimeInput) *SKCImageHandler {
	return &SKCImageHandler{
		imageProcessor: imageProcessor,
		runtime:        runtime,
	}
}

// GetVariantSpecificImages 从变体数据中获取变体特定的图片
func (h *SKCImageHandler) GetVariantSpecificImages(variant shein.Variant) ([]string, error) {
	if h.runtime == nil || h.runtime.AmazonProduct == nil {
		return nil, nil
	}

	if variant.ASIN == h.runtime.AmazonProduct.Asin {
		logger.GetGlobalLogger("shein/product").Infof("变体ASIN与主产品相同，使用主产品图片，ASIN: %s", variant.ASIN)
		return h.runtime.AmazonProduct.Images, nil
	}

	for _, variation := range h.runtime.Variants {
		if variation.Asin == variant.ASIN {
			if len(variation.Images) >= 3 {
				return variation.Images, nil
			}
			if len(variation.Images) > 0 {
				combinedImages := make([]string, len(variation.Images))
				copy(combinedImages, variation.Images)
				for _, img := range h.runtime.AmazonProduct.Images {
					if !slices.Contains(combinedImages, img) {
						combinedImages = append(combinedImages, img)
					}
				}
				return combinedImages, nil
			}
			break
		}
	}

	logger.GetGlobalLogger("shein/product").Infof("未找到变体 %s 的特定图片，使用主产品图片", variant.ASIN)
	return h.runtime.AmazonProduct.Images, nil
}
