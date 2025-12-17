package handlers

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"task-processor/internal/common/pipeline"
	"task-processor/internal/platforms/temu/types"
	"task-processor/internal/platforms/temu/utils"

	"github.com/sirupsen/logrus"
)

// ImageUploadProcessor 图片上传处理器
type ImageUploadProcessor struct {
	logger      *logrus.Entry
	uploadCache sync.Map // URL -> 上传结果的缓存，使用sync.Map保证并发安全
}

// ImageUploadRequest 图片上传请求
type ImageUploadRequest struct {
	ImageURL string `json:"image_url"`
	Type     string `json:"type"` // main, carousel, dimension
}

// ImageUploadResponse 图片上传响应
type ImageUploadResponse struct {
	Success   bool         `json:"success"`
	ErrorCode int          `json:"error_code"`
	Result    UploadResult `json:"result"`
}

// TemuImageUploadResponse Temu实际的图片上传响应格式
type TemuImageUploadResponse struct {
	URL           string   `json:"url"`
	Width         int      `json:"width"`
	Height        int      `json:"height"`
	Size          int64    `json:"size"`
	ProcessedURLs []string `json:"processed_urls"`
	Etag          string   `json:"etag"`
}

// UploadResult 上传结果
type UploadResult struct {
	ImageURL string `json:"image_url"`
	ImageID  string `json:"image_id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Size     int64  `json:"size"`
	Format   string `json:"format"`
	URL      string `json:"url"` // 兼容字段
}

// SignatureResponse 签名响应
type SignatureResponse struct {
	Success      bool            `json:"success"`
	ErrorCode    int             `json:"error_code"`
	Result       UploadSignature `json:"result"`
	ErrorMessage string          `json:"error_message,omitempty"`
}

// UploadSignature 上传签名
type UploadSignature struct {
	Signature string `json:"signature"`
}

// NewImageUploadProcessor 创建新的图片上传处理器
func NewImageUploadProcessor() *ImageUploadProcessor {
	return &ImageUploadProcessor{
		logger:      logrus.WithField("handler", "ImageUploadProcessor"),
		uploadCache: sync.Map{}, // 使用sync.Map，无需make
	}
}

// Name 返回处理器名称
func (h *ImageUploadProcessor) Name() string {
	return "图片上传处理器"
}

// Handle 处理任务
func (h *ImageUploadProcessor) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始处理图片上传")

	// 检查是否需要上传图片
	requiresUpload, exists := ctx.GetData("requires_image_upload")
	if !exists || !requiresUpload.(bool) {
		h.logger.Info("无需上传图片，跳过处理")
		return nil
	}

	if ctx.APIClient == nil {
		return fmt.Errorf("API客户端未初始化")
	}

	// 上传主图
	if err := h.uploadMainImages(ctx); err != nil {
		return fmt.Errorf("上传主图失败: %w", err)
	}

	// 上传SKU图片
	if err := h.uploadSkuImages(ctx); err != nil {
		return fmt.Errorf("上传SKU图片失败: %w", err)
	}

	// 上传完成后，进行最终的尺寸验证
	dimensionValidator := utils.NewImageDimensionValidator()
	if err := dimensionValidator.ValidateProductImages(ctx.TemuProduct); err != nil {
		h.logger.Errorf("❌ 上传后图片尺寸验证失败: %v", err)
		return fmt.Errorf("上传后图片尺寸验证失败: %w", err)
	}

	h.logger.Info("图片上传处理完成")
	return nil
}

// uploadMainImages 上传主图（并行处理）
func (h *ImageUploadProcessor) uploadMainImages(ctx *pipeline.TaskContext) error {
	mainImages := ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage

	if len(mainImages) == 0 {
		return nil
	}

	h.logger.Infof("开始并行上传 %d 张主图", len(mainImages))

	// 使用channel收集结果
	type uploadResult struct {
		index int
		img   *types.ImageInfo
		err   error
	}

	resultChan := make(chan uploadResult, len(mainImages))

	// 并行上传所有主图
	for i, img := range mainImages {
		go func(index int, imageURL string) {
			if h.needsUpload(imageURL) {
				uploadedImg, err := h.uploadSingleImage(ctx, imageURL, "main")
				resultChan <- uploadResult{index: index, img: uploadedImg, err: err}
			} else {
				// 不需要上传，保持原有信息
				resultChan <- uploadResult{index: index, img: &mainImages[index], err: nil}
			}
		}(i, img.URL)
	}

	// 收集所有结果
	successCount := 0
	failCount := 0
	for i := 0; i < len(mainImages); i++ {
		result := <-resultChan
		if result.err != nil {
			h.logger.Errorf("主图[%d] 上传失败: %v", result.index, result.err)
			failCount++
		} else {
			ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage[result.index] = *result.img
			successCount++
		}
	}

	h.logger.Infof("主图上传完成: 成功 %d/%d, 失败 %d", successCount, len(mainImages), failCount)

	if failCount > 0 && successCount == 0 {
		return fmt.Errorf("所有主图上传失败")
	}

	return nil
}

// uploadSkuImages 上传SKU图片（并行处理）
func (h *ImageUploadProcessor) uploadSkuImages(ctx *pipeline.TaskContext) error {
	// 统计总图片数
	totalImages := 0
	for _, skc := range ctx.TemuProduct.SkcList {
		for _, sku := range skc.SkuList {
			totalImages += len(sku.CarouselGallery) + len(sku.DimensionGallery)
		}
	}

	if totalImages == 0 {
		return nil
	}

	// 使用channel收集结果
	type uploadResult struct {
		skcIndex int
		skuIndex int
		imgIndex int
		imgType  string // "carousel" or "dimension"
		img      *types.ImageInfo
		err      error
	}

	resultChan := make(chan uploadResult, totalImages)

	// 并行上传所有SKU图片
	for skcIndex, skc := range ctx.TemuProduct.SkcList {
		for skuIndex, sku := range skc.SkuList {
			// 上传轮播图片
			for imgIndex, img := range sku.CarouselGallery {
				go func(si, ski, ii int, imageURL string) {
					if h.needsUpload(imageURL) {
						uploadedImg, err := h.uploadSingleImage(ctx, imageURL, "carousel")
						resultChan <- uploadResult{
							skcIndex: si,
							skuIndex: ski,
							imgIndex: ii,
							imgType:  "carousel",
							img:      uploadedImg,
							err:      err,
						}
					} else {
						resultChan <- uploadResult{
							skcIndex: si,
							skuIndex: ski,
							imgIndex: ii,
							imgType:  "carousel",
							img:      &ctx.TemuProduct.SkcList[si].SkuList[ski].CarouselGallery[ii],
							err:      nil,
						}
					}
				}(skcIndex, skuIndex, imgIndex, img.URL)
			}

			// 上传尺寸图片
			for imgIndex, img := range sku.DimensionGallery {
				go func(si, ski, ii int, imageURL string) {
					if h.needsUpload(imageURL) {
						uploadedImg, err := h.uploadSingleImage(ctx, imageURL, "dimension")
						resultChan <- uploadResult{
							skcIndex: si,
							skuIndex: ski,
							imgIndex: ii,
							imgType:  "dimension",
							img:      uploadedImg,
							err:      err,
						}
					} else {
						resultChan <- uploadResult{
							skcIndex: si,
							skuIndex: ski,
							imgIndex: ii,
							imgType:  "dimension",
							img:      &ctx.TemuProduct.SkcList[si].SkuList[ski].DimensionGallery[ii],
							err:      nil,
						}
					}
				}(skcIndex, skuIndex, imgIndex, img.URL)
			}
		}
	}

	// 收集所有结果
	successCount := 0
	failCount := 0
	for i := 0; i < totalImages; i++ {
		result := <-resultChan
		if result.err != nil {
			h.logger.Errorf("SKU[%d-%d]%s图[%d] 上传失败: %v",
				result.skcIndex, result.skuIndex, result.imgType, result.imgIndex, result.err)
			failCount++
		} else {
			if result.imgType == "carousel" {
				ctx.TemuProduct.SkcList[result.skcIndex].SkuList[result.skuIndex].CarouselGallery[result.imgIndex] = *result.img
			} else {
				ctx.TemuProduct.SkcList[result.skcIndex].SkuList[result.skuIndex].DimensionGallery[result.imgIndex] = *result.img
			}
			h.logger.Debugf("SKU[%d-%d]%s图[%d] 上传成功",
				result.skcIndex, result.skuIndex, result.imgType, result.imgIndex)
			successCount++
		}
	}

	h.logger.Infof("SKU图片上传完成: 成功 %d/%d, 失败 %d", successCount, totalImages, failCount)

	return nil
}

// uploadSingleImage 上传单张图片
func (h *ImageUploadProcessor) uploadSingleImage(ctx *pipeline.TaskContext, imageURL, _ string) (*types.ImageInfo, error) {
	// 0. 检查缓存，避免重复上传相同的图片
	if cachedValue, exists := h.uploadCache.Load(imageURL); exists {
		cachedImg := cachedValue.(*types.ImageInfo)
		h.logger.Infof("✅ 使用缓存的图片: %s -> %s", imageURL, cachedImg.URL)
		return cachedImg, nil
	}

	// 1. 先获取上传签名
	signature, err := h.getUploadSignature(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取上传签名失败: %w", err)
	}

	// 2. 检查是否有填充后的图片数据和尺寸信息
	var imageData []byte
	var filename string
	var paddedWidth, paddedHeight int
	usePaddedImage := false

	if data, exists := ctx.GetData("padded_images"); exists {
		paddedImages := data.(map[string][]byte)
		if paddedData, hasPadded := paddedImages[imageURL]; hasPadded {
			imageData = paddedData
			filename = filepath.Base(imageURL)
			if filename == "." || filename == "/" {
				filename = "padded_image.jpg"
			}
			usePaddedImage = true

			// 获取填充后的尺寸信息
			if sizeData, sizeExists := ctx.GetData("padded_image_sizes"); sizeExists {
				sizes := sizeData.(map[string][2]int)
				if size, hasSize := sizes[imageURL]; hasSize {
					paddedWidth = size[0]
					paddedHeight = size[1]
				}
			}
		}
	}

	// 3. 如果没有填充数据，下载原图
	if !usePaddedImage {
		var err error
		imageData, filename, err = h.downloadImage(imageURL)
		if err != nil {
			return nil, fmt.Errorf("下载图片失败: %w", err)
		}
	}

	// 4. 使用签名上传图片
	uploadResult, err := h.uploadImageDataWithSignature(ctx, imageData, filename, signature)
	if err != nil {
		return nil, fmt.Errorf("上传图片失败: %w", err)
	}

	// 构造返回的图片信息
	resultURL := uploadResult.ImageURL
	if resultURL == "" {
		resultURL = uploadResult.URL // 兼容处理
	}

	// 如果使用了填充图片，强制使用填充后的尺寸，忽略API返回的尺寸
	width := uploadResult.Width
	height := uploadResult.Height
	if usePaddedImage && paddedWidth > 0 && paddedHeight > 0 {
		width = paddedWidth
		height = paddedHeight
		h.logger.Infof("🔧 使用填充后的尺寸: %dx%d (API返回: %dx%d)",
			width, height, uploadResult.Width, uploadResult.Height)
	}

	// 验证最终尺寸是否为1:1比例（非服装类产品）
	if width != height {
		h.logger.Errorf("❌ 图片尺寸不是1:1比例: %dx%d, URL: %s", width, height, imageURL)
		// 对于非1:1的图片，强制调整为正方形（取较大值）
		if width > height {
			height = width
		} else {
			width = height
		}
		h.logger.Warnf("🔧 强制调整为1:1比例: %dx%d", width, height)
	}

	imageInfo := &types.ImageInfo{
		URL:    resultURL,
		Width:  width,
		Height: height,
		Type:   intPtr(1), // 设置type为1
	}

	// 将上传结果缓存起来，避免重复上传
	h.uploadCache.Store(imageURL, imageInfo)

	return imageInfo, nil
}

// getUploadSignature 获取上传签名
func (h *ImageUploadProcessor) getUploadSignature(ctx *pipeline.TaskContext) (*UploadSignature, error) {
	// 构造请求体
	requestBody := map[string]any{
		"upload_file_type": 1,
	}

	// 构造获取签名的API请求
	apiReq := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/edit/commit/get_signature",
		"headers": map[string]string{
			"accept":             "application/json, text/plain, */*",
			"accept-language":    "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
			"content-type":       "application/json;charset=UTF-8",
			"priority":           "u=1, i",
			"sec-ch-ua":          "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\"",
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-fetch-dest":     "empty",
			"sec-fetch-mode":     "cors",
			"sec-fetch-site":     "same-origin",
		},
		"body": requestBody,
	}

	// 发送请求获取签名
	response := &SignatureResponse{}
	err := ctx.APIClient.SendTEMURequest(apiReq, response)
	if err != nil {
		return nil, fmt.Errorf("发送获取签名请求失败: %w", err)
	}

	// 检查响应结果
	if !response.Success {
		return nil, fmt.Errorf("获取签名失败: error_code=%d", response.ErrorCode)
	}

	return &response.Result, nil
}

// uploadImageDataWithSignature 使用签名上传图片数据
func (h *ImageUploadProcessor) uploadImageDataWithSignature(ctx *pipeline.TaskContext, imageData []byte, filename string, signature *UploadSignature) (*UploadResult, error) {
	// 1. 构造API请求 - 使用fileFields和formFields
	apiReq := map[string]any{
		"method": "POST",
		"url":    "/api/galerie/v3/store_image?sdk_version=js-1.0.6&tag_name=local-goods-image",
		"headers": map[string]string{
			"accept":             "*/*",
			"accept-language":    "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
			"priority":           "u=1, i",
			"sec-ch-ua":          "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\"",
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-fetch-dest":     "empty",
			"sec-fetch-mode":     "cors",
			"sec-fetch-site":     "same-origin",
		},
		"formFields": map[string]string{
			"url_width_height": "true",
			"pic_operations":   `{"original_needed":false,"rules":[{"rule":"imageMogr2/format/jpg|imageMogr2/size-limit/3m!/ignore-error/0","suffix":"format"}]}`,
			"upload_sign":      signature.Signature,
		},
		"fileFields": map[string]any{
			"image": map[string]any{
				"filename": filename,
				"content":  imageData,
			},
		},
	}

	// 2. 发送上传请求
	response := &TemuImageUploadResponse{}
	err := ctx.APIClient.SendTEMURequest(apiReq, response)
	if err != nil {
		return nil, fmt.Errorf("发送图片上传请求失败: %w", err)
	}

	// 3. 检查响应
	if response.URL == "" {
		return nil, fmt.Errorf("图片上传失败: 响应中没有URL")
	}

	// 4. 构造返回结果
	result := &UploadResult{
		ImageURL: response.URL,
		URL:      response.URL,
		Width:    response.Width,
		Height:   response.Height,
		Size:     response.Size,
		Format:   "jpg",
	}

	return result, nil
}

// downloadImage 下载图片
func (h *ImageUploadProcessor) downloadImage(imageURL string) ([]byte, string, error) {
	resp, err := http.Get(imageURL)
	if err != nil {
		return nil, "", fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP状态码错误: %d", resp.StatusCode)
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取响应体失败: %w", err)
	}

	// 从URL中提取文件名
	filename := filepath.Base(imageURL)
	if filename == "." || filename == "/" {
		filename = "image.jpg"
	}

	return imageData, filename, nil
}

// needsUpload 判断图片是否需要上传
func (h *ImageUploadProcessor) needsUpload(imageURL string) bool {
	// 如果图片URL已经是TEMU的CDN地址，则不需要重新上传
	temuDomains := []string{
		"img.kwcdn.com",
		"local-goods-image",
		"temu.com",
	}

	for _, domain := range temuDomains {
		if strings.Contains(imageURL, domain) {
			return false
		}
	}

	return true
}

// BatchUploadImages 批量上传图片
func (h *ImageUploadProcessor) BatchUploadImages(ctx *pipeline.TaskContext, imageURLs []string, imageType string) ([]*types.ImageInfo, error) {
	var results []*types.ImageInfo

	// 统计缓存命中
	cacheHits := 0
	for i, url := range imageURLs {
		// 先检查缓存
		if cachedValue, exists := h.uploadCache.Load(url); exists {
			cachedImg := cachedValue.(*types.ImageInfo)
			h.logger.Debugf("✅ 批量上传使用缓存: %s", url)
			results = append(results, cachedImg)
			cacheHits++
			continue
		}

		if h.needsUpload(url) {
			uploadedImg, err := h.uploadSingleImage(ctx, url, imageType)
			if err != nil {
				h.logger.Errorf("批量上传第 %d 张图片失败: %v", i+1, err)
				continue
			}
			results = append(results, uploadedImg)
		} else {
			// 如果不需要上传，保持原有信息
			results = append(results, &types.ImageInfo{
				URL:    url,
				Width:  1500, // 默认尺寸
				Height: 1500,
				Type:   intPtr(1),
			})
		}
	}

	if cacheHits > 0 {
		h.logger.Infof("📊 批量上传缓存命中: %d/%d", cacheHits, len(imageURLs))
	}

	return results, nil
}

// GetUploadProgress 获取上传进度
func (h *ImageUploadProcessor) GetUploadProgress(ctx *pipeline.TaskContext) map[string]any {
	progress := map[string]any{
		"total_images":    0,
		"uploaded_images": 0,
		"failed_images":   0,
		"skipped_images":  0,
	}

	// 统计图片数量
	totalImages := len(ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage)
	for _, skc := range ctx.TemuProduct.SkcList {
		for _, sku := range skc.SkuList {
			totalImages += len(sku.CarouselGallery) + len(sku.DimensionGallery)
		}
	}

	progress["total_images"] = totalImages

	// 这里可以添加更详细的进度统计逻辑
	// 实际实现中可以在上传过程中更新这些计数器

	return progress
}

// ClearCache 清理上传缓存
func (h *ImageUploadProcessor) ClearCache() {
	// 计算缓存大小
	cacheSize := 0
	h.uploadCache.Range(func(_, _ interface{}) bool {
		cacheSize++
		return true
	})

	// 清空缓存
	h.uploadCache = sync.Map{}
	h.logger.Infof("🗑️ 已清理图片上传缓存，释放 %d 条记录", cacheSize)
}

// GetCacheStats 获取缓存统计信息
func (h *ImageUploadProcessor) GetCacheStats() map[string]int {
	// 计算缓存大小
	cacheSize := 0
	h.uploadCache.Range(func(_, _ interface{}) bool {
		cacheSize++
		return true
	})

	return map[string]int{
		"cached_images": cacheSize,
	}
}

// ValidateUploadedImages 验证已上传的图片
func (h *ImageUploadProcessor) ValidateUploadedImages(ctx *pipeline.TaskContext) error {
	h.logger.Info("验证已上传的图片")

	// 验证主图
	for i, img := range ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage {
		if img.URL == "" {
			return fmt.Errorf("主图[%d] URL为空", i)
		}
		if img.Width <= 0 || img.Height <= 0 {
			h.logger.Warnf("主图[%d] 尺寸信息缺失: %dx%d", i, img.Width, img.Height)
		}
	}

	// 验证SKU图片
	for skcIndex, skc := range ctx.TemuProduct.SkcList {
		for skuIndex, sku := range skc.SkuList {
			for imgIndex, img := range sku.CarouselGallery {
				if img.URL == "" {
					h.logger.Warnf("SKU[%d-%d]轮播图[%d] URL为空", skcIndex, skuIndex, imgIndex)
				}
			}
			for imgIndex, img := range sku.DimensionGallery {
				if img.URL == "" {
					h.logger.Warnf("SKU[%d-%d]尺寸图[%d] URL为空", skcIndex, skuIndex, imgIndex)
				}
			}
		}
	}

	h.logger.Info("图片验证完成")
	return nil
}
