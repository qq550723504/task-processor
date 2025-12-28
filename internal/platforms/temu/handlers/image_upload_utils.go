// Package handlers 提供TEMU平台图片上传工具功能
package handlers

import (
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/services"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// ImageUploadUtils 图片上传工具类
type ImageUploadUtils struct {
	uploadProcessor  *ImageUploadProcessor
	paddingProcessor *ImagePaddingProcessor
	logger           *logrus.Entry
}

// NewImageUploadUtils 创建新的图片上传工具
func NewImageUploadUtils() *ImageUploadUtils {
	return &ImageUploadUtils{
		uploadProcessor:  NewImageUploadProcessor(),
		paddingProcessor: NewImagePaddingProcessor(),
		logger:           logrus.WithField("component", "ImageUploadUtils"),
	}
}

// uploadImageWithFallback 上传图片，失败时返回空图片信息（不使用Amazon原始链接）
func (iuu *ImageUploadUtils) uploadImageWithFallback(temuCtx *temucontext.TemuTaskContext, imageURL, imageType string, defaultWidth, defaultHeight int) types.ImageInfo {
	// 检查是否需要上传
	configService := services.NewImageConfigService()
	if configService.NeedsUpload(imageURL) {
		uploadedImage, err := iuu.uploadProcessor.UploadSingleImage(temuCtx, imageURL, imageType)
		if err == nil && uploadedImage != nil {
			return *uploadedImage
		}
		// 上传失败，记录错误日志
		iuu.logger.Errorf("❌ 图片上传失败，不使用Amazon原始链接: %s, 错误: %v", imageURL, err)
		// 返回空的图片信息，不使用Amazon原始链接
		return types.ImageInfo{
			URL:    "",
			Width:  0,
			Height: 0,
			Type:   nil,
		}
	}

	// 如果是TEMU的CDN地址，直接使用
	return types.ImageInfo{
		URL:    imageURL,
		Width:  defaultWidth,
		Height: defaultHeight,
		Type:   nil,
	}
}

// batchUploadImagesWithFallback 批量上传图片，失败时跳过该图片（不使用Amazon原始链接）
func (iuu *ImageUploadUtils) batchUploadImagesWithFallback(temuCtx *temucontext.TemuTaskContext, imageURLs []string, imageType string, defaultWidth, defaultHeight int) []types.ImageInfo {
	var images []types.ImageInfo

	// 尝试批量上传
	uploadedImages, err := iuu.uploadProcessor.BatchUploadImages(temuCtx, imageURLs, imageType)
	if err == nil && len(uploadedImages) == len(imageURLs) {
		// 上传成功，转换为值切片
		for _, imgPtr := range uploadedImages {
			if imgPtr != nil {
				images = append(images, *imgPtr)
			}
		}
		return images
	}

	// 批量上传失败，逐个尝试上传
	for _, imageURL := range imageURLs {
		image := iuu.uploadImageWithFallback(temuCtx, imageURL, imageType, defaultWidth, defaultHeight)
		// 只添加上传成功的图片（URL不为空）
		if image.URL != "" {
			images = append(images, image)
		} else {
			iuu.logger.Warnf("⚠️ 跳过上传失败的图片: %s", imageURL)
		}
	}

	return images
}

// padImagesIfNeeded 对图片进行填充处理（如果需要）
func (iuu *ImageUploadUtils) padImagesIfNeeded(temuCtx *temucontext.TemuTaskContext, imageURLs []string) {
	// 获取或创建填充图片存储map
	if temuCtx.PaddedImages == nil {
		temuCtx.PaddedImages = make(map[string][]byte)
	}
	if temuCtx.PaddedImageSizes == nil {
		temuCtx.PaddedImageSizes = make(map[string][2]int)
	}

	// 对每张图片进行填充处理
	for _, imageURL := range imageURLs {
		// 跳过已经处理过的图片
		if _, exists := temuCtx.PaddedImages[imageURL]; exists {
			continue
		}

		// 使用1:1宽高比，最小尺寸800x800
		result, err := iuu.paddingProcessor.PadImageToAspectRatio(imageURL, 1.0, 800, 800)
		if err != nil {
			// 填充失败，记录日志但继续（将使用原图）
			continue
		}

		// 只有需要填充的图片才存储
		if result.NeedsPadding && result.Success {
			temuCtx.PaddedImages[imageURL] = result.PaddedImage
			temuCtx.PaddedImageSizes[imageURL] = [2]int{result.NewWidth, result.NewHeight}
		}
	}
}
