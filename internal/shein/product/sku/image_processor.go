// Package sku 提供SHEIN平台SKU图片处理相关功能
package sku

import (
	"fmt"
	"strings"
	"task-processor/internal/shein"
	"task-processor/internal/shein/api/product"

	"github.com/sirupsen/logrus"
)

// buildSKUImageInfoForMultiPiece 为多件商品构建SKU图片信息
func (b *SKUBuilder) buildSKUImageInfoForMultiPiece(ctx *shein.TaskContext, params shein.SKUCreationParams) *product.ImageInfo {
	logrus.Infof("🖼️ 开始为多件商品构建SKU图片，ASIN: %s", params.ASIN)

	// 初始化空的图片信息
	imageInfo := &product.ImageInfo{
		ImageInfoList:         []product.ImageDetail{},
		OriginalImageInfoList: &[]any{},
	}

	// 查找对应变体的图片
	var variantImages []string
	var imageSource string

	// 通过ASIN查找对应的变体图片
	if ctx.Variants != nil {
		for _, variant := range *ctx.Variants {
			if variant.Asin == params.ASIN && len(variant.Images) > 0 {
				variantImages = variant.Images
				imageSource = "变体图片"
				logrus.Infof("✅ 找到变体图片，ASIN: %s, 图片数量: %d", params.ASIN, len(variantImages))
				break
			}
		}
	}

	// 如果没找到变体图片，使用主产品图片
	if len(variantImages) == 0 && ctx.AmazonProduct != nil && len(ctx.AmazonProduct.Images) > 0 {
		variantImages = ctx.AmazonProduct.Images
		imageSource = "主产品图片"
		logrus.Infof("⚠️ 未找到变体图片，使用主产品图片，ASIN: %s, 图片数量: %d", params.ASIN, len(variantImages))
	}

	// 如果仍然没有图片，这是一个严重问题
	if len(variantImages) == 0 {
		logrus.Errorf("❌ 严重错误：多件商品必须有SKU图片，但未找到任何图片，ASIN: %s", params.ASIN)
		logrus.Errorf("   - 变体数据: %v", ctx.Variants != nil)
		if ctx.Variants != nil {
			logrus.Errorf("   - 变体数量: %d", len(*ctx.Variants))
		}
		logrus.Errorf("   - 主产品数据: %v", ctx.AmazonProduct != nil)
		if ctx.AmazonProduct != nil {
			logrus.Errorf("   - 主产品图片数量: %d", len(ctx.AmazonProduct.Images))
		}
		return imageInfo
	}

	// 增强的图片上传逻辑：支持重试和多图片处理
	uploadSuccess := b.uploadSKUImagesWithRetry(ctx, params, variantImages, imageSource, imageInfo)

	if !uploadSuccess {
		logrus.Errorf("❌ 所有图片上传都失败了，多件商品SKU将缺少必需的图片，ASIN: %s", params.ASIN)
	} else {
		logrus.Infof("🎉 多件商品SKU图片构建完成，ASIN: %s, 图片数量: %d", params.ASIN, len(imageInfo.ImageInfoList))
	}

	return imageInfo
}

// uploadSKUImagesWithRetry 带重试机制的SKU图片上传
func (b *SKUBuilder) uploadSKUImagesWithRetry(ctx *shein.TaskContext, params shein.SKUCreationParams, variantImages []string, imageSource string, imageInfo *product.ImageInfo) bool {
	const maxRetries = 3
	const maxImages = 1 // 只保留一张图片

	uploadedCount := 0

	// 遍历可用图片，只上传一张图片
	for i, imageURL := range variantImages {
		if imageURL == "" || uploadedCount >= maxImages {
			continue
		}

		// 对每张图片进行重试上传
		uploaded := b.uploadSingleImageWithRetry(ctx, params, imageURL, imageSource, i+1, maxRetries)
		if uploaded != "" {
			// 创建SKU图片信息
			// 重要：SKU图片的排序必须从1开始，第一张SKU图片排序为1
			skuImageSort := 1 // 排序编号固定为1
			imageDetail := product.ImageDetail{
				ImageURL:             uploaded,
				ImageType:            1,            // SKU图片类型
				ImageSort:            skuImageSort, // SKU图片排序固定为1
				AISStatus:            0,
				MarketingMainImage:   false,
				PSTypes:              []string{},
				SizeImgFlag:          false,
				TransformCVSizeImage: false,
			}
			imageInfo.ImageInfoList = append(imageInfo.ImageInfoList, imageDetail)
			uploadedCount++
			logrus.Infof("✅ 成功上传第%d张SKU图片，ASIN: %s, ImageSort: %d, URL: %s", uploadedCount, params.ASIN, skuImageSort, uploaded)

			// 对于多件商品，至少需要1张图片
			if uploadedCount >= 1 {
				logrus.Infof("✅ 已成功上传%d张SKU图片，满足多件商品要求，ASIN: %s", uploadedCount, params.ASIN)
				break // 只上传一张图片
			}
		}
	}

	// 验证SKU图片排序
	if uploadedCount > 0 {
		if err := b.validateSKUImageSorting(imageInfo); err != nil {
			logrus.Errorf("❌ SKU图片排序验证失败: %v", err)
			// 不返回错误，而是尝试修复
			b.fixSKUImageSorting(imageInfo)
		}
	}

	return uploadedCount > 0
}

// uploadSingleImageWithRetry 单张图片重试上传
func (b *SKUBuilder) uploadSingleImageWithRetry(ctx *shein.TaskContext, params shein.SKUCreationParams, imageURL, imageSource string, imageIndex, maxRetries int) string {
	for retry := 1; retry <= maxRetries; retry++ {
		logrus.Infof("🔄 尝试上传第%d张%s作为SKU图片 (重试%d/%d): %s", imageIndex, imageSource, retry, maxRetries, imageURL)

		// 验证图片URL格式
		if !b.isValidImageURL(imageURL) {
			logrus.Warnf("⚠️ 无效的图片URL格式，跳过: %s", imageURL)
			break
		}

		uploadedURL, err := ctx.ImageAPI.DownloadAndUploadImage(imageURL)
		if err != nil {
			logrus.Warnf("⚠️ 上传第%d张SKU图片失败 (重试%d/%d)，ASIN: %s, 错误: %v", imageIndex, retry, maxRetries, params.ASIN, err)

			// 如果是最后一次重试，记录详细错误
			if retry == maxRetries {
				logrus.Errorf("❌ 第%d张SKU图片上传彻底失败，已重试%d次，ASIN: %s, URL: %s, 最终错误: %v",
					imageIndex, maxRetries, params.ASIN, imageURL, err)
			}
			continue
		}

		if uploadedURL != "" {
			// 验证上传结果
			if b.isValidImageURL(uploadedURL) {
				return uploadedURL
			} else {
				logrus.Warnf("⚠️ 上传返回的URL格式无效，重试: %s", uploadedURL)
			}
		}
	}

	return ""
}

// isValidImageURL 验证图片URL格式
func (b *SKUBuilder) isValidImageURL(url string) bool {
	if url == "" {
		return false
	}

	// 基本URL格式检查
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return false
	}

	// 检查是否包含常见图片扩展名或图片服务域名
	lowerURL := strings.ToLower(url)
	imageExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp"}
	imageServices := []string{"amazonaws.com", "cloudfront.net", "ssl-images-amazon.com", "media-amazon.com"}

	// 检查扩展名
	for _, ext := range imageExtensions {
		if strings.Contains(lowerURL, ext) {
			return true
		}
	}

	// 检查图片服务域名
	for _, service := range imageServices {
		if strings.Contains(lowerURL, service) {
			return true
		}
	}

	return false
}

// validateSKUImageSorting 验证SKU图片排序的正确性
func (b *SKUBuilder) validateSKUImageSorting(imageInfo *product.ImageInfo) error {
	if len(imageInfo.ImageInfoList) == 0 {
		return nil
	}

	// 检查第一张SKU图片的排序是否为1
	firstImage := imageInfo.ImageInfoList[0]
	if firstImage.ImageType == 1 && firstImage.ImageSort != 1 {
		return fmt.Errorf("第一张SKU图片排序应为1，当前为: %d", firstImage.ImageSort)
	}

	// 如果只有一张图片，不需要检查排序连续性
	if len(imageInfo.ImageInfoList) == 1 {
		logrus.Infof("✅ SKU图片排序验证通过，共%d张图片", len(imageInfo.ImageInfoList))
		return nil
	}

	// 检查排序连续性（仅在有多张图片时）
	for i, img := range imageInfo.ImageInfoList {
		expectedSort := i + 1
		if img.ImageSort != expectedSort {
			return fmt.Errorf("SKU图片排序不连续，第%d张图片期望排序%d，实际%d", i+1, expectedSort, img.ImageSort)
		}
	}

	logrus.Infof("✅ SKU图片排序验证通过，共%d张图片", len(imageInfo.ImageInfoList))
	return nil
}

// fixSKUImageSorting 修复SKU图片排序
func (b *SKUBuilder) fixSKUImageSorting(imageInfo *product.ImageInfo) {
	logrus.Infof("🔧 开始修复SKU图片排序...")

	// 如果只有一张图片，确保其排序为1
	if len(imageInfo.ImageInfoList) == 1 {
		if imageInfo.ImageInfoList[0].ImageSort != 1 {
			oldSort := imageInfo.ImageInfoList[0].ImageSort
			imageInfo.ImageInfoList[0].ImageSort = 1
			logrus.Infof("✅ 修复SKU图片排序：唯一图片 %d -> %d", oldSort, 1)
		}
		return
	}

	// 多张图片的情况，保持原有逻辑
	for i := range imageInfo.ImageInfoList {
		correctSort := i + 1
		if imageInfo.ImageInfoList[i].ImageSort != correctSort {
			oldSort := imageInfo.ImageInfoList[i].ImageSort
			imageInfo.ImageInfoList[i].ImageSort = correctSort
			logrus.Infof("✅ 修复SKU图片排序：第%d张图片 %d -> %d", i+1, oldSort, correctSort)
		}
	}

	logrus.Infof("🎉 SKU图片排序修复完成")
}
