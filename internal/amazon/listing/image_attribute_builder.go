package listing

import (
	"context"
	"slices"
	"strings"
	"task-processor/internal/amazon/model"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// ImageAttributeBuilder 图片属性构建器
type ImageAttributeBuilder struct {
	logger *logrus.Entry
}

// NewImageAttributeBuilder 创建图片属性构建器
func NewImageAttributeBuilder() *ImageAttributeBuilder {
	return &ImageAttributeBuilder{
		logger: logger.GetGlobalLogger("ImageAttributeBuilder"),
	}
}

// AddImageAttributes 添加图片属性
func (iab *ImageAttributeBuilder) AddImageAttributes(ctx context.Context, attrs map[string]any, data *model.ProductData, marketplaceID string, productSchema *model.ProductTypeSchema) {
	// 处理主图 - Amazon支持直接使用外部图片URL
	if data.MainImageURL != "" {
		attrs["main_product_image_locator"] = []map[string]any{
			{
				"media_location": data.MainImageURL,
				"marketplace_id": marketplaceID,
			},
		}
		iab.logger.WithField("main_image_url", data.MainImageURL).Info("设置主图URL")
	}

	// 处理附加图片 - 根据Schema判断支持的图片属性
	if len(data.AdditionalImages) > 0 {
		iab.addAdditionalImagesBySchema(attrs, data.AdditionalImages, marketplaceID, productSchema)
	}
}

// addAdditionalImagesBySchema 根据Schema添加附加图片
func (iab *ImageAttributeBuilder) addAdditionalImagesBySchema(attrs map[string]any, imageURLs []string, marketplaceID string, productSchema *model.ProductTypeSchema) {
	if productSchema == nil {
		iab.logger.Warn("无Schema信息，跳过附加图片设置")
		return
	}

	// 检查Schema中支持的图片属性
	supportedImageAttrs := iab.getSupportedImageAttributes(productSchema)
	if len(supportedImageAttrs) == 0 {
		iab.logger.Info("该产品类型不支持附加图片属性")
		return
	}

	// 构建图片数据
	var imageData []map[string]any
	maxImages := 9 // Amazon最多支持9张附加图片

	for i, imageURL := range imageURLs {
		if i >= maxImages {
			iab.logger.WithField("total_images", len(imageURLs)).
				Warn("附加图片数量超过Amazon限制，只使用前9张")
			break
		}

		if imageURL != "" {
			imageData = append(imageData, map[string]any{
				"media_location": imageURL,
				"marketplace_id": marketplaceID,
			})
		}
	}

	if len(imageData) == 0 {
		return
	}

	// 如果有带数字后缀的属性（如 other_product_image_locator_1），分别设置每张图片
	if len(supportedImageAttrs) > 1 && strings.Contains(supportedImageAttrs[0], "_") {
		iab.distributeImagesToNumberedAttrs(attrs, imageData, supportedImageAttrs, marketplaceID)
	} else if len(supportedImageAttrs) > 0 {
		// 使用第一个支持的图片属性
		selectedAttr := supportedImageAttrs[0]
		attrs[selectedAttr] = imageData

		iab.logger.WithFields(logrus.Fields{
			"attribute": selectedAttr,
			"count":     len(imageData),
		}).Info("设置附加图片URL")
	}
}

// getSupportedImageAttributes 获取Schema中支持的图片属性
func (iab *ImageAttributeBuilder) getSupportedImageAttributes(productSchema *model.ProductTypeSchema) []string {
	var supportedAttrs []string
	var allImageAttrs []string

	// 扫描Schema中所有包含"image"的属性
	for attrName := range productSchema.Properties {
		if strings.Contains(strings.ToLower(attrName), "image") {
			allImageAttrs = append(allImageAttrs, attrName)
		}
	}

	// 记录所有找到的图片相关属性
	iab.logger.WithFields(logrus.Fields{
		"all_image_attrs": allImageAttrs,
	}).Info("Schema中所有图片相关属性")

	// 从所有图片属性中筛选出附加图片属性（排除主图）
	for _, attrName := range allImageAttrs {
		// 排除主图属性
		if strings.Contains(attrName, "main_") {
			continue
		}
		// 优先选择 other_product_image_locator 系列
		if strings.HasPrefix(attrName, "other_product_image_locator") {
			supportedAttrs = append(supportedAttrs, attrName)
		}
	}

	// 如果没有找到 other_product_image_locator 系列，尝试其他图片属性
	if len(supportedAttrs) == 0 {
		for _, attrName := range allImageAttrs {
			if strings.Contains(attrName, "main_") {
				continue
			}
			if strings.Contains(attrName, "swatch_") ||
				strings.Contains(attrName, "other_") ||
				strings.Contains(attrName, "additional_") {
				supportedAttrs = append(supportedAttrs, attrName)
			}
		}
	}

	if len(supportedAttrs) > 0 {
		iab.logger.WithFields(logrus.Fields{
			"supported_attrs": supportedAttrs,
		}).Info("找到支持的附加图片属性")
	} else {
		iab.logger.Info("该产品类型仅支持主图，不支持附加图片")
	}

	return supportedAttrs
}

// distributeImagesToNumberedAttrs 将图片分配到带数字后缀的属性中
func (iab *ImageAttributeBuilder) distributeImagesToNumberedAttrs(attrs map[string]any, imageData []map[string]any, supportedAttrs []string, _ string) {
	// 按数字后缀排序属性
	slices.Sort(supportedAttrs)

	// 为每张图片分配一个属性
	assignedCount := 0
	for i, imageItem := range imageData {
		if i >= len(supportedAttrs) {
			iab.logger.WithField("total_images", len(imageData)).
				WithField("max_attrs", len(supportedAttrs)).
				Warn("图片数量超过可用属性数量，部分图片将被忽略")
			break
		}

		attrName := supportedAttrs[i]
		attrs[attrName] = []map[string]any{imageItem}
		assignedCount++
	}

	iab.logger.WithFields(logrus.Fields{
		"assigned_count": assignedCount,
		"total_images":   len(imageData),
		"attributes":     supportedAttrs[:assignedCount],
	}).Info("设置附加图片到带数字后缀的属性")
}
