package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"task-processor/common/amazon"
	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// ImageProcessor 图片处理器
type ImageProcessor struct {
	uploadProcessor  *ImageUploadProcessor
	paddingProcessor *ImagePaddingProcessor
	logger           *logrus.Entry
}

// NewImageProcessor 创建新的图片处理器
func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		uploadProcessor:  NewImageUploadProcessor(),
		paddingProcessor: NewImagePaddingProcessor(),
		logger:           logrus.WithField("component", "ImageProcessor"),
	}
}

// BuildVariantImagesWithUpload 构建变体图片并上传到TEMU
func (ip *ImageProcessor) BuildVariantImagesWithUpload(ctx *pipeline.TaskContext, variant *amazon.Product) []types.ImageInfo {
	// 收集需要上传的图片URL
	var imageURLs []string
	for _, img := range variant.Images {
		if img != "" {
			imageURLs = append(imageURLs, img)
		}
	}

	if len(imageURLs) == 0 {
		return []types.ImageInfo{}
	}

	// 限制图片数量不超过10张
	const maxImages = 10
	if len(imageURLs) > maxImages {
		imageURLs = imageURLs[:maxImages]
	}

	// 先进行图片填充处理
	ip.padImagesIfNeeded(ctx, imageURLs)

	// 批量上传图片到TEMU，失败时使用降级处理
	return ip.batchUploadImagesWithFallback(ctx, imageURLs, "carousel", 1500, 1500)
}

// BuildDimensionImages 构建尺寸图片（通常是第一张图片）
func (ip *ImageProcessor) BuildDimensionImages(variant *amazon.Product) []types.ImageInfo {
	var images []types.ImageInfo

	// 使用第4张图片作为尺寸图片（如果存在）
	if len(variant.Images) > 3 && variant.Images[3] != "" {
		images = append(images, types.ImageInfo{
			URL:    variant.Images[3],
			Width:  1500,
			Height: 1500,
			Type:   intPtr(1), // 设置type为1
		})
	} else if len(variant.Images) > 0 && variant.Images[0] != "" {
		// 如果没有第4张图片，使用第1张作为备选
		images = append(images, types.ImageInfo{
			URL:    variant.Images[0],
			Width:  1500,
			Height: 1500,
			Type:   intPtr(1),
		})
	}

	return images
}

// BuildDimensionImagesWithUpload 构建尺寸图片并上传到TEMU（检测所有图片，优先使用已有标注的图片）
func (ip *ImageProcessor) BuildDimensionImagesWithUpload(ctx *pipeline.TaskContext, variant *amazon.Product) []types.ImageInfo {
	var images []types.ImageInfo

	if len(variant.Images) == 0 {
		return images
	}

	// 暂时注释掉检测逻辑，直接使用第三张图片添加标注
	// // 先检测所有图片，看是否已有尺寸标注
	// annotator := NewImageDimensionAnnotator()
	// for i, imageURL := range variant.Images {
	// 	if imageURL == "" {
	// 		continue
	// 	}

	// 	// 下载并检测图片
	// 	img, _, err := annotator.DownloadImage(imageURL)
	// 	if err != nil {
	// 		ip.logger.Warnf("下载图片%d失败，跳过检测: %v", i+1, err)
	// 		continue
	// 	}

	// 	// 检测是否已有标注
	// 	hasAnnotation, details := annotator.HasDimensionAnnotationWithDetails(img)
	// 	if hasAnnotation {
	// 		ip.logger.Infof("✅ 图片%d已包含尺寸标注，直接使用 - %s", i+1, details)

	// 		// 先进行图片填充处理
	// 		ip.padImagesIfNeeded(ctx, []string{imageURL})

	// 		// 直接使用这张已有标注的图片
	// 		image := ip.uploadImageWithFallback(ctx, imageURL, "dimension", 1500, 1500)
	// 		images = append(images, image)
	// 		return images
	// 	}

	// 	ip.logger.Debugf("图片%d未检测到尺寸标注", i+1)
	// }

	// 优先使用第三张图片添加标注（如果有的话）
	var imageURL string
	if len(variant.Images) >= 3 && variant.Images[2] != "" {
		ip.logger.Info("将为第三张图片添加尺寸标注")
		imageURL = variant.Images[2]
	} else {
		ip.logger.Info("图片数量不足3张，将为第一张图片添加尺寸标注")
		imageURL = variant.Images[0]
	}

	// 先进行图片填充处理
	ip.padImagesIfNeeded(ctx, []string{imageURL})

	// 添加尺寸标注（直接将标注后的图片数据存储到context）
	ip.logger.Infof("🎨 开始为图片添加尺寸标注: %s", imageURL)
	annotatedImageData, err := ip.addDimensionAnnotationToContext(ctx, imageURL, variant)
	if err != nil {
		// 如果标注失败，记录错误并返回空图片列表
		ip.logger.Errorf("❌ 添加尺寸标注失败，跳过该图片: %v", err)
		return images // 返回空列表
	} else {
		ip.logger.Infof("✅ 尺寸标注成功，图片大小: %d bytes", len(annotatedImageData))

		// 将标注后的图片数据存储到padded_images中，这样uploadSingleImage可以直接使用
		paddedImages, _ := ctx.GetData("padded_images")
		if paddedImages == nil {
			paddedImages = make(map[string][]byte)
			ip.logger.Info("📦 创建新的padded_images map")
		}
		paddedImagesMap := paddedImages.(map[string][]byte)
		paddedImagesMap[imageURL] = annotatedImageData
		ctx.SetData("padded_images", paddedImagesMap)
		ip.logger.Infof("💾 标注图片已存储到context，key: %s", imageURL)

		// 使用标注后的图片（使用原URL作为key，因为数据已经存储在context中）
		ip.logger.Infof("📤 开始上传标注后的图片...")
		image := ip.uploadImageWithFallback(ctx, imageURL, "dimension", 1500, 1500)
		ip.logger.Infof("✅ 图片上传完成，URL: %s", image.URL)
		images = append(images, image)
	}

	return images
}

// GetProductImagesWithUpload 获取产品图片并上传到TEMU
func (ip *ImageProcessor) GetProductImagesWithUpload(ctx *pipeline.TaskContext) []types.ImageInfo {
	// 收集需要上传的图片URL
	var imageURLs []string
	if ctx.AmazonProduct != nil {
		for _, img := range ctx.AmazonProduct.Images {
			if img != "" {
				imageURLs = append(imageURLs, img)
			}
		}
	}

	// 限制图片数量不超过10张
	const maxImages = 10
	if len(imageURLs) > maxImages {
		imageURLs = imageURLs[:maxImages]
	}

	// 先进行图片填充处理
	ip.padImagesIfNeeded(ctx, imageURLs)

	// 批量上传图片到TEMU，失败时使用降级处理
	return ip.batchUploadImagesWithFallback(ctx, imageURLs, "main", 800, 800)
}

// uploadImageWithFallback 上传图片，失败时返回空图片信息（不使用Amazon原始链接）
func (ip *ImageProcessor) uploadImageWithFallback(ctx *pipeline.TaskContext, imageURL, imageType string, defaultWidth, defaultHeight int) types.ImageInfo {
	// 检查是否需要上传
	if ip.uploadProcessor.needsUpload(imageURL) {
		uploadedImage, err := ip.uploadProcessor.uploadSingleImage(ctx, imageURL, imageType)
		if err == nil && uploadedImage != nil {
			return *uploadedImage
		}
		// 上传失败，记录错误日志
		ip.logger.Errorf("❌ 图片上传失败，不使用Amazon原始链接: %s, 错误: %v", imageURL, err)
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
		Type:   intPtr(1),
	}
}

// batchUploadImagesWithFallback 批量上传图片，失败时跳过该图片（不使用Amazon原始链接）
func (ip *ImageProcessor) batchUploadImagesWithFallback(ctx *pipeline.TaskContext, imageURLs []string, imageType string, defaultWidth, defaultHeight int) []types.ImageInfo {
	var images []types.ImageInfo

	// 尝试批量上传
	uploadedImages, err := ip.uploadProcessor.BatchUploadImages(ctx, imageURLs, imageType)
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
		image := ip.uploadImageWithFallback(ctx, imageURL, imageType, defaultWidth, defaultHeight)
		// 只添加上传成功的图片（URL不为空）
		if image.URL != "" {
			images = append(images, image)
		} else {
			ip.logger.Warnf("⚠️ 跳过上传失败的图片: %s", imageURL)
		}
	}

	return images
}

// padImagesIfNeeded 对图片进行填充处理（如果需要）
func (ip *ImageProcessor) padImagesIfNeeded(ctx *pipeline.TaskContext, imageURLs []string) {
	// 获取或创建填充图片存储map
	var paddedImages map[string][]byte
	var paddedSizes map[string][2]int

	if data, exists := ctx.GetData("padded_images"); exists {
		paddedImages = data.(map[string][]byte)
	} else {
		paddedImages = make(map[string][]byte)
	}

	if data, exists := ctx.GetData("padded_image_sizes"); exists {
		paddedSizes = data.(map[string][2]int)
	} else {
		paddedSizes = make(map[string][2]int)
	}

	// 对每张图片进行填充处理
	for _, imageURL := range imageURLs {
		// 跳过已经处理过的图片
		if _, exists := paddedImages[imageURL]; exists {
			continue
		}

		// 使用1:1宽高比，最小尺寸800x800
		result, err := ip.paddingProcessor.PadImageToAspectRatio(imageURL, 1.0, 800, 800)
		if err != nil {
			// 填充失败，记录日志但继续（将使用原图）
			continue
		}

		// 只有需要填充的图片才存储
		if result.NeedsPadding && result.Success {
			paddedImages[imageURL] = result.PaddedImage
			paddedSizes[imageURL] = [2]int{result.NewWidth, result.NewHeight}
		}
	}

	// 存储回context
	ctx.SetData("padded_images", paddedImages)
	ctx.SetData("padded_image_sizes", paddedSizes)
}

// addDimensionAnnotationToContext 为图片添加尺寸标注并返回图片数据
func (ip *ImageProcessor) addDimensionAnnotationToContext(ctx *pipeline.TaskContext, imageURL string, variant *amazon.Product) ([]byte, error) {
	// 从上下文获取AI生成的尺寸信息
	aiMapping, exists := ctx.GetData("ai_sku_mapping")
	if !exists {
		return nil, fmt.Errorf("未找到AI SKU映射数据")
	}

	mapping, ok := aiMapping.(*AISkuMappingResponse)
	if !ok {
		return nil, fmt.Errorf("AI SKU映射数据类型错误")
	}

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
	if paddedImages, exists := ctx.GetData("padded_images"); exists {
		if paddedImagesMap, ok := paddedImages.(map[string][]byte); ok {
			if data, found := paddedImagesMap[imageURL]; found {
				imageData = data
				ip.logger.Info("✅ 使用填充后的图片数据进行标注")
			}
		}
	}

	// 创建标注器
	annotator := NewImageDimensionAnnotator()

	var annotatedImageBytes []byte
	var err error

	// 如果有填充后的图片数据，使用它；否则从URL下载
	if imageData != nil {
		annotatedImageBytes, err = annotator.AnnotateImageFromBytes(imageData, dimensions)
	} else {
		ip.logger.Warn("⚠️ 未找到填充后的图片数据，从URL下载原图（可能不是1:1比例）")
		annotatedImageBytes, err = annotator.AnnotateImage(imageURL, dimensions)
	}

	if err != nil {
		return nil, fmt.Errorf("标注图片失败: %w", err)
	}

	ip.logger.Infof("✅ 尺寸标注图片已生成 (L:%s W:%s H:%s), 大小: %d bytes",
		dimensions.Length, dimensions.Width, dimensions.Height, len(annotatedImageBytes))

	// 直接返回图片数据
	return annotatedImageBytes, nil
}

// addDimensionAnnotation 为图片添加尺寸标注（会自动检测是否已有标注）- 已废弃，保留用于兼容
func (ip *ImageProcessor) addDimensionAnnotation(ctx *pipeline.TaskContext, imageURL string, variant *amazon.Product) (string, error) {
	// 从上下文获取AI生成的尺寸信息
	aiMapping, exists := ctx.GetData("ai_sku_mapping")
	if !exists {
		return "", fmt.Errorf("未找到AI SKU映射数据")
	}

	mapping, ok := aiMapping.(*AISkuMappingResponse)
	if !ok {
		return "", fmt.Errorf("AI SKU映射数据类型错误")
	}

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
		return "", fmt.Errorf("未找到变体 %s 的尺寸信息", variant.Asin)
	}

	// 创建标注器
	annotator := NewImageDimensionAnnotator()

	// 生成标注图片（内部会自动检测是否已有标注）
	// 如果已有标注，会返回原图；如果没有，会添加标注
	annotatedImageBytes, err := annotator.AnnotateImage(imageURL, dimensions)
	if err != nil {
		return "", fmt.Errorf("标注图片失败: %w", err)
	}

	// 将标注后的图片保存为临时文件
	tempFile := fmt.Sprintf("temp_annotated_%s.png", variant.Asin)
	tempPath := filepath.Join(os.TempDir(), tempFile)

	if err := os.WriteFile(tempPath, annotatedImageBytes, 0644); err != nil {
		return "", fmt.Errorf("保存标注图片失败: %w", err)
	}

	ip.logger.Infof("✅ 尺寸标注图片已处理: %s (L:%s W:%s H:%s)",
		tempPath, dimensions.Length, dimensions.Width, dimensions.Height)

	// 返回临时文件路径（后续会被上传）
	return tempPath, nil
}
