// Package image 提供TEMU平台尺寸图片构建功能
package image

import (
	"fmt"
	"task-processor/internal/domain/model"
	"task-processor/internal/pkg/ptrutil"
	models "task-processor/internal/platforms/temu/api/product"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// ImageDimensionBuilder 尺寸图片构建器
type ImageDimensionBuilder struct {
	uploadProcessor  *ImageUploadProcessor
	paddingProcessor *ImagePaddingProcessor
	uploadUtils      *ImageUploadUtils
	logger           *logrus.Entry
}

// NewImageDimensionBuilder 创建新的尺寸图片构建器
func NewImageDimensionBuilder() *ImageDimensionBuilder {
	return &ImageDimensionBuilder{
		uploadProcessor:  NewImageUploadProcessor(),
		paddingProcessor: NewImagePaddingProcessor(),
		uploadUtils:      NewImageUploadUtils(),
		logger:           logrus.WithField("component", "ImageDimensionBuilder"),
	}
}

// BuildDimensionImages 构建尺寸图片（通常是第一张图片）
func (idb *ImageDimensionBuilder) BuildDimensionImages(variant *model.Product) []models.ImageInfo {
	var images []models.ImageInfo

	// 使用第4张图片作为尺寸图片（如果存在）
	if len(variant.Images) > 3 && variant.Images[3] != "" {
		images = append(images, models.ImageInfo{
			URL:    variant.Images[3],
			Width:  1500,
			Height: 1500,
			Type:   ptrutil.IntPtr(1), // 设置type为1
		})
	} else if len(variant.Images) > 0 && variant.Images[0] != "" {
		// 如果没有第4张图片，使用第1张作为备选
		images = append(images, models.ImageInfo{
			URL:    variant.Images[0],
			Width:  1500,
			Height: 1500,
			Type:   ptrutil.IntPtr(1),
		})
	}

	return images
}

// BuildMainImageWithDimensionAnnotation 为主图添加尺寸标注（专用于主图展示）
func (idb *ImageDimensionBuilder) BuildMainImageWithDimensionAnnotation(temuCtx *temucontext.TemuTaskContext, variant *model.Product) []models.ImageInfo {
	var images []models.ImageInfo

	if len(variant.Images) == 0 {
		return images
	}

	// 优先使用第三张图片添加标注（如果有的话）
	var imageURL string
	if len(variant.Images) >= 3 && variant.Images[2] != "" {
		idb.logger.Info("将为第三张图片添加尺寸标注（主图用）")
		imageURL = variant.Images[2]
	} else {
		idb.logger.Info("图片数量不足3张，将为第一张图片添加尺寸标注（主图用）")
		imageURL = variant.Images[0]
	}

	// 先进行图片填充处理
	idb.uploadUtils.padImagesIfNeeded(temuCtx, []string{imageURL})

	// 添加尺寸标注（直接将标注后的图片数据存储到context）
	idb.logger.Infof("🎨 开始为主图添加尺寸标注: %s", imageURL)
	annotatedImageData, err := idb.addDimensionAnnotationToContext(temuCtx, imageURL, variant)
	if err != nil {
		// 如果标注失败，记录错误并返回空图片列表
		idb.logger.Errorf("❌ 添加主图尺寸标注失败，跳过该图片: %v", err)
		return images // 返回空列表
	} else {
		idb.logger.Infof("✅ 主图尺寸标注成功，图片大小: %d bytes", len(annotatedImageData))

		// 将标注后的图片数据存储到padded_images中，这样uploadSingleImage可以直接使用
		if temuCtx.PaddedImages == nil {
			temuCtx.PaddedImages = make(map[string][]byte)
			idb.logger.Info("📦 创建新的padded_images map")
		}
		temuCtx.PaddedImages[imageURL] = annotatedImageData
		idb.logger.Infof("💾 标注图片已存储到context，key: %s", imageURL)

		// 使用标注后的图片（使用原URL作为key，因为数据已经存储在context中）
		idb.logger.Infof("📤 开始上传标注后的主图...")
		image := idb.uploadUtils.uploadImageWithFallback(temuCtx, imageURL, "dimension", 1500, 1500)
		idb.logger.Infof("✅ 主图上传完成，URL: %s", image.URL)
		images = append(images, image)
	}

	return images
}

// BuildDimensionImagesWithUpload 构建尺寸图片并上传到TEMU（检测所有图片，优先使用已有标注的图片）
// 用途：为SKU的DimensionGallery提供标注过的尺寸图
func (idb *ImageDimensionBuilder) BuildDimensionImagesWithUpload(temuCtx *temucontext.TemuTaskContext, variant *model.Product) []models.ImageInfo {
	return idb.BuildMainImageWithDimensionAnnotation(temuCtx, variant)
}

// addDimensionAnnotationToContext 为图片添加尺寸标注并返回图片数据（不使用缓存）
func (idb *ImageDimensionBuilder) addDimensionAnnotationToContext(temuCtx *temucontext.TemuTaskContext, imageURL string, variant *model.Product) ([]byte, error) {
	// 从上下文获取AI生成的尺寸信息
	if temuCtx.AISkuMapping == nil {
		return nil, fmt.Errorf("未找到AI SKU映射数据")
	}

	mapping := temuCtx.AISkuMapping

	// 查找当前变体的尺寸信息
	var dimensions DimensionInfo
	for _, aiSku := range mapping.SkuList {
		if aiSku.Asin == variant.Asin {
			dimensions = DimensionInfo{
				Length: aiSku.Length,
				Width:  aiSku.Width,
				Height: aiSku.Height,
			}
			break
		}
	}

	// 如果没有找到尺寸信息，返回错误
	if dimensions.Length == "" && dimensions.Width == "" && dimensions.Height == "" {
		return nil, fmt.Errorf("未找到变体 %s 的尺寸信息", variant.Asin)
	}

	// 检查是否有填充后的图片数据
	var imageData []byte
	if temuCtx.PaddedImages != nil {
		if data, found := temuCtx.PaddedImages[imageURL]; found {
			imageData = data
			idb.logger.Info("✅ 使用填充后的图片数据进行标注")
		}
	}

	// 6. 创建标注器
	annotator := NewImageDimensionAnnotator()

	var annotatedImageBytes []byte
	var err error

	// 7. 如果有填充后的图片数据，使用它；否则从URL下载
	if imageData != nil {
		annotatedImageBytes, err = annotator.AnnotateImageFromBytes(imageData, dimensions)
	} else {
		idb.logger.Warn("⚠️ 未找到填充后的图片数据，从URL下载原图（可能不是1:1比例）")
		annotatedImageBytes, err = annotator.AnnotateImage(imageURL, dimensions)
	}

	if err != nil {
		return nil, fmt.Errorf("标注图片失败: %w", err)
	}

	idb.logger.Infof("✅ 尺寸标注图片已生成 (L:%s W:%s H:%s), 大小: %d bytes",
		dimensions.Length, dimensions.Width, dimensions.Height, len(annotatedImageBytes))

	// 直接返回图片数据，不使用缓存
	return annotatedImageBytes, nil
}
