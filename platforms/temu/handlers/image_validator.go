package handlers

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"task-processor/common/pipeline"
	"task-processor/platforms/temu/types"
	"task-processor/platforms/temu/utils"

	"github.com/sirupsen/logrus"
)

// ImageValidator 图片验证器
type ImageValidator struct {
	logger           *logrus.Entry
	paddingProcessor *ImagePaddingProcessor
}

// ImageValidationResult 图片验证结果
type ImageValidationResult struct {
	IsValid      bool     `json:"is_valid"`
	URL          string   `json:"url"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	Format       string   `json:"format"`
	Size         int64    `json:"size"`
	AspectRatio  float64  `json:"aspect_ratio"`
	Violations   []string `json:"violations"`
	Suggestions  []string `json:"suggestions"`
	NeedsPadding bool     `json:"needs_padding"`
	PaddedImage  []byte   `json:"-"` // 填充后的图片数据
	PaddedWidth  int      `json:"padded_width"`
	PaddedHeight int      `json:"padded_height"`
}

// ImageRequirement 图片要求配置
type ImageRequirement struct {
	MaxSizeMB     float64 // 最大文件大小（MB）
	MinWidth      int     // 最小宽度
	MinHeight     int     // 最小高度
	AspectRatio   float64 // 期望宽高比（严格要求，不允许偏差）
	MinImageCount int     // 最小图片数量
	MaxImageCount int     // 最大图片数量
}

// NewImageValidator 创建新的图片验证器
func NewImageValidator() *ImageValidator {
	return &ImageValidator{
		logger:           logrus.WithField("handler", "ImageValidator"),
		paddingProcessor: NewImagePaddingProcessor(),
	}
}

// Name 返回处理器名称
func (h *ImageValidator) Name() string {
	return "图片验证处理器"
}

// Handle 处理任务
func (h *ImageValidator) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始验证产品图片")

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 记录产品分类信息
	h.logger.Infof("========== 产品分类信息 ==========")
	h.logger.Infof("IsClothes=%v, CatID=%d, GoodsType=%d",
		ctx.TemuProduct.GoodsBasic.IsClothes,
		ctx.TemuProduct.GoodsBasic.CatID,
		ctx.TemuProduct.GoodsBasic.GoodsType)
	if len(ctx.TemuProduct.GoodsBasic.CategoryTree.CateNameList) > 0 {
		h.logger.Infof("分类路径: %v", ctx.TemuProduct.GoodsBasic.CategoryTree.CateNameList)
	}
	h.logger.Infof("====================================")

	// 获取图片要求配置
	requirement := h.getImageRequirement(ctx)
	h.logger.Infof("图片要求: 宽高比=%.2f, 最小尺寸=%dx%d, 最大文件大小=%.1fMB, 图片数量=%d-%d",
		requirement.AspectRatio, requirement.MinWidth, requirement.MinHeight,
		requirement.MaxSizeMB, requirement.MinImageCount, requirement.MaxImageCount)

	// 验证商品主图
	if err := h.validateMainImages(ctx, requirement); err != nil {
		return fmt.Errorf("主图验证失败: %w", err)
	}

	// 验证SKC/SKU图片
	if err := h.validateSkuImages(ctx, requirement); err != nil {
		return fmt.Errorf("SKU图片验证失败: %w", err)
	}

	// 设置需要上传图片的标志
	ctx.SetData("requires_image_upload", true)

	// 使用新的尺寸验证工具进行最终验证
	dimensionValidator := utils.NewImageDimensionValidator()
	if err := dimensionValidator.ValidateProductImages(ctx.TemuProduct); err != nil {
		h.logger.Errorf("❌ 图片尺寸最终验证失败: %v", err)
		return fmt.Errorf("图片尺寸验证失败: %w", err)
	}

	h.logger.Info("图片验证完成")
	return nil
}

// getImageRequirement 根据产品分类获取图片要求
func (h *ImageValidator) getImageRequirement(ctx *pipeline.TaskContext) ImageRequirement {
	isClothes := ctx.TemuProduct.GoodsBasic.IsClothes

	if isClothes {
		// 服装类产品要求
		h.logger.Info("检测到服装类产品，应用服装类图片规则")
		return ImageRequirement{
			MaxSizeMB:     3.0,
			MinWidth:      1340,
			MinHeight:     1785,
			AspectRatio:   0.75, // 3:4 = 0.75 (严格要求)
			MinImageCount: 1,
			MaxImageCount: 10,
		}
	}

	// 其他分类产品要求
	h.logger.Info("检测到非服装类产品，应用通用图片规则")
	return ImageRequirement{
		MaxSizeMB:     3.0,
		MinWidth:      800,
		MinHeight:     800,
		AspectRatio:   1.0, // 1:1 (严格要求)
		MinImageCount: 1,
		MaxImageCount: 10,
	}
}

// validateMainImages 验证商品主图（并行处理）
func (h *ImageValidator) validateMainImages(ctx *pipeline.TaskContext, requirement ImageRequirement) error {
	mainImages := ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage

	if len(mainImages) == 0 {
		h.logger.Warn("未找到商品主图")
		return nil
	}

	h.logger.Infof("🔄 开始并行验证 %d 张主图", len(mainImages))

	// 使用并行处理
	results := h.validateImagesInParallel(mainImages, "主图", requirement)

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
			h.logger.Warnf("主图[%d] 验证失败，将被过滤: %v", i, result.Violations)
		}
	}

	// 检查图片数量限制
	if len(validImages) > requirement.MaxImageCount {
		validImages = validImages[:requirement.MaxImageCount]
	}

	if len(validImages) < requirement.MinImageCount {
		return fmt.Errorf("主图数量不足: %d < %d", len(validImages), requirement.MinImageCount)
	}

	// 保存填充后的图片数据和尺寸到上下文
	if len(paddedImagesMap) > 0 {
		ctx.SetData("padded_images", paddedImagesMap)
		ctx.SetData("padded_image_sizes", paddedSizesMap)
	}

	// 更新有效图片
	ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage = validImages

	return nil
}

// validateSkuImages 验证SKU图片
func (h *ImageValidator) validateSkuImages(ctx *pipeline.TaskContext, requirement ImageRequirement) error {
	totalSkuImages := 0
	totalPaddedImages := 0

	// 获取或创建填充图片映射
	var paddedImagesMap map[string][]byte
	var paddedSizesMap map[string][2]int

	if data, exists := ctx.GetData("padded_images"); exists {
		paddedImagesMap = data.(map[string][]byte)
	} else {
		paddedImagesMap = make(map[string][]byte)
	}

	if data, exists := ctx.GetData("padded_image_sizes"); exists {
		paddedSizesMap = data.(map[string][2]int)
	} else {
		paddedSizesMap = make(map[string][2]int)
	}

	for skcIndex, skc := range ctx.TemuProduct.SkcList {
		for skuIndex, sku := range skc.SkuList {
			// 验证轮播图片
			validCarouselImages := []types.ImageInfo{}
			for imgIndex, img := range sku.CarouselGallery {
				result := h.validateSingleImage(img.URL, fmt.Sprintf("SKU[%d-%d]轮播图[%d]", skcIndex, skuIndex, imgIndex), requirement)

				if result.IsValid {
					if result.NeedsPadding {
						img.Width = result.PaddedWidth
						img.Height = result.PaddedHeight
						paddedImagesMap[img.URL] = result.PaddedImage
						paddedSizesMap[img.URL] = [2]int{result.PaddedWidth, result.PaddedHeight}
						totalPaddedImages++
					} else {
						img.Width = result.Width
						img.Height = result.Height
					}
					validCarouselImages = append(validCarouselImages, img)
				} else {
					h.logger.Warnf("SKU[%d-%d]轮播图[%d] 验证失败: %v", skcIndex, skuIndex, imgIndex, result.Violations)
				}
			}

			// 验证尺寸图片
			validDimensionImages := []types.ImageInfo{}
			for imgIndex, img := range sku.DimensionGallery {
				result := h.validateSingleImage(img.URL, fmt.Sprintf("SKU[%d-%d]尺寸图[%d]", skcIndex, skuIndex, imgIndex), requirement)

				if result.IsValid {
					if result.NeedsPadding {
						img.Width = result.PaddedWidth
						img.Height = result.PaddedHeight
						paddedImagesMap[img.URL] = result.PaddedImage
						paddedSizesMap[img.URL] = [2]int{result.PaddedWidth, result.PaddedHeight}
						totalPaddedImages++
					} else {
						img.Width = result.Width
						img.Height = result.Height
					}
					validDimensionImages = append(validDimensionImages, img)
				} else {
					h.logger.Warnf("SKU[%d-%d]尺寸图[%d] 验证失败: %v", skcIndex, skuIndex, imgIndex, result.Violations)
				}
			}

			// 更新SKU图片
			ctx.TemuProduct.SkcList[skcIndex].SkuList[skuIndex].CarouselGallery = validCarouselImages
			ctx.TemuProduct.SkcList[skcIndex].SkuList[skuIndex].DimensionGallery = validDimensionImages

			totalSkuImages += len(validCarouselImages) + len(validDimensionImages)
		}
	}

	// 更新填充图片映射和尺寸映射
	if len(paddedImagesMap) > 0 {
		ctx.SetData("padded_images", paddedImagesMap)
		ctx.SetData("padded_image_sizes", paddedSizesMap)
	}

	return nil
}

// validateImagesInParallel 并行验证多张图片
func (h *ImageValidator) validateImagesInParallel(images []types.ImageInfo, imageType string, requirement ImageRequirement) []*ImageValidationResult {
	if len(images) == 0 {
		return []*ImageValidationResult{}
	}

	// 控制并发数，避免过多goroutine
	maxConcurrency := 5
	if len(images) < maxConcurrency {
		maxConcurrency = len(images)
	}

	semaphore := make(chan struct{}, maxConcurrency)
	results := make([]*ImageValidationResult, len(images))
	var wg sync.WaitGroup

	for i, img := range images {
		wg.Add(1)
		go func(index int, imageURL string) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 验证图片
			context := fmt.Sprintf("%s[%d]", imageType, index)
			results[index] = h.validateSingleImage(imageURL, context, requirement)
		}(i, img.URL)
	}

	wg.Wait()
	h.logger.Infof("✅ 并行验证完成: %d 张图片", len(images))

	return results
}

// validateSingleImage 验证单张图片
func (h *ImageValidator) validateSingleImage(imageURL, context string, requirement ImageRequirement) *ImageValidationResult {
	result := &ImageValidationResult{
		URL:         imageURL,
		IsValid:     true,
		Violations:  []string{},
		Suggestions: []string{},
	}

	if imageURL == "" {
		result.IsValid = false
		result.Violations = append(result.Violations, "图片URL为空")
		return result
	}

	// 验证图片格式
	format := h.getImageFormat(imageURL)
	result.Format = format
	if !h.isValidFormat(format) {
		result.IsValid = false
		result.Violations = append(result.Violations, fmt.Sprintf("不支持的图片格式: %s (仅支持JPEG, JPG, PNG)", format))
	}

	// 获取图片信息
	width, height, size, err := h.getImageInfo(imageURL)
	if err != nil {
		h.logger.Errorf("%s 获取图片信息失败，无法进行填充处理: %v", context, err)
		result.IsValid = false
		result.Violations = append(result.Violations, fmt.Sprintf("无法获取图片信息进行验证和填充: %v", err))
		return result
	}

	result.Width = width
	result.Height = height
	result.Size = size
	result.AspectRatio = float64(width) / float64(height)

	// 验证尺寸要求
	if width < requirement.MinWidth {
		result.IsValid = false
		result.Violations = append(result.Violations, fmt.Sprintf("图片宽度不足: %dpx < %dpx", width, requirement.MinWidth))
	}
	if height < requirement.MinHeight {
		result.IsValid = false
		result.Violations = append(result.Violations, fmt.Sprintf("图片高度不足: %dpx < %dpx", height, requirement.MinHeight))
	}

	// 验证宽高比 - TEMU要求严格的比例，不允许容差
	expectedRatio := requirement.AspectRatio

	// 计算如果要达到目标比例，图片应该是什么尺寸
	var targetWidth, targetHeight int
	if result.AspectRatio > expectedRatio {
		// 图片太宽，需要增加高度
		targetWidth = width
		targetHeight = int(float64(width) / expectedRatio)
	} else if result.AspectRatio < expectedRatio {
		// 图片太高，需要增加宽度
		targetHeight = height
		targetWidth = int(float64(height) * expectedRatio)
	} else {
		// 宽高比完全匹配
		targetWidth = width
		targetHeight = height
	}

	// 检查是否需要填充
	needsAspectRatioPadding := (targetWidth != width || targetHeight != height)
	needsSizePadding := width < requirement.MinWidth || height < requirement.MinHeight

	if needsAspectRatioPadding || needsSizePadding {

		paddingResult, err := h.paddingProcessor.PadImageToAspectRatio(
			imageURL,
			expectedRatio,
			requirement.MinWidth,
			requirement.MinHeight,
		)

		if err != nil {
			result.IsValid = false
			result.Violations = append(result.Violations,
				fmt.Sprintf("宽高比不符合要求: %.2f (期望: %.2f 严格匹配)，且自动填充失败",
					result.AspectRatio, expectedRatio))
		} else if paddingResult.Success {
			if paddingResult.NeedsPadding {
				result.NeedsPadding = true
				result.PaddedImage = paddingResult.PaddedImage
				result.PaddedWidth = paddingResult.NewWidth
				result.PaddedHeight = paddingResult.NewHeight
				result.Suggestions = append(result.Suggestions, "图片已自动添加白边以符合要求")
			} else {
				h.logger.Infof("%s 图片无需填充", context)
			}
			// 填充成功，图片有效
			result.IsValid = true
		}
	}

	// 验证文件大小
	maxSizeBytes := int64(requirement.MaxSizeMB * 1024 * 1024)
	if size > maxSizeBytes {
		result.IsValid = false
		result.Violations = append(result.Violations,
			fmt.Sprintf("文件大小超限: %.2fMB > %.1fMB",
				float64(size)/(1024*1024), requirement.MaxSizeMB))
	}

	// 提供优化建议
	recommendedWidth := requirement.MinWidth * 2
	recommendedHeight := requirement.MinHeight * 2
	if width < recommendedWidth || height < recommendedHeight {
		result.Suggestions = append(result.Suggestions,
			fmt.Sprintf("建议使用更高分辨率的图片（推荐: %dx%d）以提高显示质量",
				recommendedWidth, recommendedHeight))
	}

	if result.IsValid {
		h.logger.Debugf("%s 验证通过: %dx%d, %.2fMB, 宽高比%.2f",
			context, width, height, float64(size)/(1024*1024), result.AspectRatio)
	}

	return result
}

// getImageFormat 获取图片格式
func (h *ImageValidator) getImageFormat(imageURL string) string {
	ext := strings.ToLower(filepath.Ext(imageURL))
	switch ext {
	case ".jpg", ".jpeg":
		return "JPEG"
	case ".png":
		return "PNG"
	default:
		return ext
	}
}

// isValidFormat 检查是否为有效格式
func (h *ImageValidator) isValidFormat(format string) bool {
	validFormats := []string{"JPEG", "JPG", "PNG"}
	for _, valid := range validFormats {
		if strings.EqualFold(format, valid) {
			return true
		}
	}
	return false
}

// getImageInfo 获取图片信息
func (h *ImageValidator) getImageInfo(imageURL string) (width, height int, size int64, err error) {
	// 使用真实的下载方法获取图片信息
	return h.getImageInfoByDownload(imageURL)
}

// getImageInfoByDownload 通过下载获取真实图片信息
func (h *ImageValidator) getImageInfoByDownload(imageURL string) (width, height int, size int64, err error) {
	resp, err := http.Get(imageURL)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("下载图片失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, 0, fmt.Errorf("图片URL返回错误状态: %d", resp.StatusCode)
	}

	// 解析图片获取尺寸
	img, _, err := image.DecodeConfig(resp.Body)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("解析图片失败: %w", err)
	}

	width = img.Width
	height = img.Height
	size = resp.ContentLength

	return width, height, size, nil
}

// ValidateImageUploadRequirement 验证图片上传要求
func (h *ImageValidator) ValidateImageUploadRequirement(ctx *pipeline.TaskContext) error {
	h.logger.Info("检查图片上传要求")

	// 检查是否需要调用 bg.local.goods.image.upload 进行转换
	totalImages := len(ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage)

	for _, skc := range ctx.TemuProduct.SkcList {
		for _, sku := range skc.SkuList {
			totalImages += len(sku.CarouselGallery) + len(sku.DimensionGallery)
		}
	}

	if totalImages > 0 {
		// 这里可以设置标志，提醒后续处理器需要调用图片上传API
		ctx.SetData("requires_image_upload", true)
		ctx.SetData("total_image_count", totalImages)
	}

	return nil
}

// GetImageValidationSummary 获取图片验证摘要
func (h *ImageValidator) GetImageValidationSummary(ctx *pipeline.TaskContext) map[string]interface{} {
	summary := map[string]interface{}{
		"main_images":     len(ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage),
		"sku_images":      0,
		"total_images":    0,
		"requires_upload": false,
	}

	skuImageCount := 0
	for _, skc := range ctx.TemuProduct.SkcList {
		for _, sku := range skc.SkuList {
			skuImageCount += len(sku.CarouselGallery) + len(sku.DimensionGallery)
		}
	}

	summary["sku_images"] = skuImageCount
	summary["total_images"] = summary["main_images"].(int) + skuImageCount
	summary["requires_upload"] = summary["total_images"].(int) > 0

	return summary
}
