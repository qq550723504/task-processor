// Package image 提供SHEIN平台的各种处理模块，包括图片处理、上传等功能
package image

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"sync"
	"task-processor/internal/pkg/imagex"
	"task-processor/internal/pkg/recovery"
	"task-processor/internal/shein"
	"task-processor/internal/shein/api/product"

	"github.com/sirupsen/logrus"
)

// ImageProcessor 图片处理器
type ImageProcessor struct {
	imageDownloader interface {
		DownloadImage(url string) ([]byte, error)
	}
}

// NewImageProcessor 创建新的图片处理器
// 参数:
//   - imageDownloader: 图片下载器接口，用于下载图片数据
//
// 返回值:
//   - *ImageProcessor: 图片处理器实例
func NewImageProcessor(imageDownloader interface {
	DownloadImage(url string) ([]byte, error)
}) *ImageProcessor {
	return &ImageProcessor{
		imageDownloader: imageDownloader,
	}
}

// BuildImageInfo 构建图片信息
// 处理图片列表，包括下载、上传、排序等操作，生成SHEIN平台所需的图片信息结构
// 参数:
//   - ctx: 任务上下文，包含店铺客户端等信息
//   - images: 图片URL列表，第一张为主图
//
// 返回值:
//   - product.ImageInfo: 构建完成的图片信息结构
//   - error: 处理过程中的错误，如果为nil表示处理成功
func (p *ImageProcessor) BuildImageInfo(ctx *shein.TaskContext, images []string) (product.ImageInfo, error) {
	imageInfo := product.ImageInfo{
		ImageInfoList:         []product.ImageDetail{},
		OriginalImageInfoList: &[]any{},
	}

	// 统计有效图片
	validImageCount := 0
	for _, img := range images {
		if img != "" {
			validImageCount++
		}
	}
	if validImageCount == 0 {
		return imageInfo, nil
	}

	// 主图会产生两条记录（类型1和类型5）
	totalImageSort := validImageCount + 1

	// 上传结果
	// 设置并发数（默认 5）
	maxConcurrent := 5
	semaphore := make(chan struct{}, maxConcurrent)
	resultChan := make(chan shein.ImageUploadResult, len(images)+1)
	var wg sync.WaitGroup

	// 并行上传图片
	for i, imageURL := range images {
		if imageURL == "" {
			continue
		}

		wg.Add(1)
		go func(index int, url string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			logrus.Debugf("处理图片URL[%d]: %s", index, url)

			// 下载、处理并上传图片
			uploadedURL, err := ctx.ImageAPI.DownloadAndUploadImage(url)

			resultChan <- shein.ImageUploadResult{
				Index:  index,
				URL:    uploadedURL,
				Err:    err,
				IsMain: index == 0,
			}
		}(i, imageURL)
	}

	// 主图提取色块图
	if len(images) > 0 && images[0] != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			logrus.Info("并行提取色块图")
			colorBlockData, err := p.extractColorBlockImage(images[0])

			resultChan <- shein.ImageUploadResult{
				Index:     -1,
				ColorData: colorBlockData,
				Err:       err,
				IsColor:   true,
			}
		}()
	}

	// 等待结果
	go func() {
		defer recovery.Recover("等待结果", logrus.WithField("component", "ImageProcessor"))
		wg.Wait()
		close(resultChan)
	}()

	uploadResults := make(map[int]shein.ImageUploadResult)
	var colorBlockResult *shein.ImageUploadResult
	var mainImageURL string

	for result := range resultChan {
		if result.IsColor {
			tmp := result // ⚠️ 必须复制，避免 &result 指针复用问题
			colorBlockResult = &tmp
		} else {
			uploadResults[result.Index] = result
			if result.IsMain && result.Err == nil {
				mainImageURL = result.URL
			}
		}
	}

	// 检查上传失败
	var uploadErrors []string
	for i, r := range uploadResults {
		if r.Err != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("图片[%d]上传失败: %v", i, r.Err))
		}
	}
	if len(uploadErrors) > 0 {
		logrus.Warnf("部分图片上传失败: %s", strings.Join(uploadErrors, "; "))
		if uploadResults[0].Err != nil {
			return product.ImageInfo{}, fmt.Errorf("主图上传失败: %w", uploadResults[0].Err)
		}
	}

	// 构建图片信息 - 增强主图排序逻辑
	mainImageProcessed := false
	currentImageSort := 2 // 非主图从2开始排序

	for i := 0; i < len(images); i++ {
		r, ok := uploadResults[i]
		if !ok || r.Err != nil || r.URL == "" {
			continue
		}

		if i == 0 && !mainImageProcessed {
			// 主图（类型1）- 确保排序始终为1
			imageInfo.ImageInfoList = append(imageInfo.ImageInfoList, product.ImageDetail{
				ImageURL:             r.URL,
				ImageType:            1,
				ImageSort:            1, // 主图排序必须为1
				AISStatus:            0,
				MarketingMainImage:   false,
				PSTypes:              []string{},
				SizeImgFlag:          false,
				TransformCVSizeImage: false,
			})
			// 主图复制为类型5，放在最后
			imageInfo.ImageInfoList = append(imageInfo.ImageInfoList, product.ImageDetail{
				ImageURL:  r.URL,
				ImageType: 5,
				ImageSort: totalImageSort,
			})
			mainImageProcessed = true
			logrus.Infof("✅ 主图处理完成，ImageSort=1, ImageType=1, URL: %s", r.URL)
		} else if !mainImageProcessed {
			// 如果第一张图片失败，将第一张成功的图片作为主图
			imageInfo.ImageInfoList = append(imageInfo.ImageInfoList, product.ImageDetail{
				ImageURL:             r.URL,
				ImageType:            1,
				ImageSort:            1, // 主图排序必须为1
				AISStatus:            0,
				MarketingMainImage:   false,
				PSTypes:              []string{},
				SizeImgFlag:          false,
				TransformCVSizeImage: false,
			})
			// 主图复制为类型5，放在最后
			imageInfo.ImageInfoList = append(imageInfo.ImageInfoList, product.ImageDetail{
				ImageURL:  r.URL,
				ImageType: 5,
				ImageSort: totalImageSort,
			})
			mainImageProcessed = true
			logrus.Warnf("⚠️ 原主图失败，使用第%d张图片作为主图，ImageSort=1, ImageType=1, URL: %s", i+1, r.URL)
		} else {
			// 其他图片（类型2）
			imageInfo.ImageInfoList = append(imageInfo.ImageInfoList, product.ImageDetail{
				ImageURL:             r.URL,
				ImageType:            2,
				ImageSort:            currentImageSort,
				AISStatus:            0,
				MarketingMainImage:   false,
				PSTypes:              []string{},
				SizeImgFlag:          false,
				TransformCVSizeImage: false,
			})
			logrus.Debugf("✅ 副图处理完成，ImageSort=%d, ImageType=2, URL: %s", currentImageSort, r.URL)
			currentImageSort++
		}
	}

	// 验证主图是否已处理
	if !mainImageProcessed {
		logrus.Errorf("❌ 严重错误：没有成功处理任何主图，这将导致发布失败")
		return product.ImageInfo{}, fmt.Errorf("没有成功上传任何图片作为主图")
	}

	// 处理色块图
	if colorBlockResult != nil && colorBlockResult.Err == nil && len(colorBlockResult.ColorData) > 0 {

		colorBlockURL, err := ctx.ImageAPI.UploadOriginalImage(colorBlockResult.ColorData)
		if err != nil {
			logrus.Warnf("上传色块图失败: %v，回退为主图", err)
			colorBlockURL = mainImageURL
		}
		if colorBlockURL != "" {
			imageInfo.ImageInfoList = append(imageInfo.ImageInfoList, product.ImageDetail{
				ImageURL:  colorBlockURL,
				ImageType: 6,
				ImageSort: totalImageSort + 1,
			})
		}
	} else if colorBlockResult != nil && colorBlockResult.Err != nil {
		logrus.Warnf("提取色块图失败: %v，回退为主图", colorBlockResult.Err)
		if mainImageURL != "" {
			imageInfo.ImageInfoList = append(imageInfo.ImageInfoList, product.ImageDetail{
				ImageURL:  mainImageURL,
				ImageType: 6,
				ImageSort: totalImageSort + 1,
			})
		}
	}

	// 最终验证图片排序
	if err := p.validateImageSorting(&imageInfo); err != nil {
		logrus.Errorf("❌ 图片排序验证失败: %v", err)
		return product.ImageInfo{}, fmt.Errorf("图片排序验证失败: %w", err)
	}

	logrus.Infof("🎉 图片信息构建完成，共%d张图片", len(imageInfo.ImageInfoList))
	return imageInfo, nil
}

// validateImageSorting 验证图片排序的正确性
func (p *ImageProcessor) validateImageSorting(imageInfo *product.ImageInfo) error {
	if len(imageInfo.ImageInfoList) == 0 {
		return fmt.Errorf("图片列表为空")
	}

	hasMainImage := false
	mainImageSort := 0
	sortMap := make(map[int]bool)

	for i, img := range imageInfo.ImageInfoList {
		// 检查主图（类型1）
		if img.ImageType == 1 {
			if hasMainImage {
				return fmt.Errorf("发现多个主图（ImageType=1），这是不允许的")
			}
			hasMainImage = true
			mainImageSort = img.ImageSort

			// 主图排序必须为1
			if img.ImageSort != 1 {
				logrus.Errorf("❌ 主图排序错误：期望=1，实际=%d，正在修复...", img.ImageSort)
				imageInfo.ImageInfoList[i].ImageSort = 1
				mainImageSort = 1
				logrus.Infof("✅ 主图排序已修复为1")
			}
		}

		// 检查排序重复（除了类型5，它可能与主图有相同排序）
		if img.ImageType != 5 {
			if sortMap[img.ImageSort] {
				return fmt.Errorf("发现重复的图片排序: %d", img.ImageSort)
			}
			sortMap[img.ImageSort] = true
		}
	}

	// 必须有主图
	if !hasMainImage {
		return fmt.Errorf("缺少主图（ImageType=1）")
	}

	// 主图排序必须为1
	if mainImageSort != 1 {
		return fmt.Errorf("主图排序必须为1，当前为: %d", mainImageSort)
	}

	logrus.Infof("✅ 图片排序验证通过，主图排序=%d", mainImageSort)
	return nil
}

// extractColorBlockImage 从图片中提取色块图
func (p *ImageProcessor) extractColorBlockImage(imageURL string) ([]byte, error) {
	logrus.Debugf("提取色块图，图片URL: %s", imageURL)
	// 下载图片数据
	imageData, err := p.imageDownloader.DownloadImage(imageURL)
	if err != nil {
		return nil, fmt.Errorf("下载图片失败: %w", err)
	}

	// 2. 解码图片
	img, err := imagex.FromBytes(imageData)
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	// 3. 提取主要颜色
	dominantColor := p.extractDominantColor(img)

	// 4. 创建900x900的色块图
	colorBlockImg := image.NewRGBA(image.Rect(0, 0, 900, 900))

	// 5. 填充颜色
	for y := 0; y < 900; y++ {
		for x := 0; x < 900; x++ {
			colorBlockImg.Set(x, y, dominantColor)
		}
	}

	// 6. 编码为JPEG
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, colorBlockImg, &jpeg.Options{Quality: 95})
	if err != nil {
		return nil, fmt.Errorf("编码图片失败: %w", err)
	}

	return buf.Bytes(), nil
}

// extractDominantColor 提取图片的主要颜色
func (p *ImageProcessor) extractDominantColor(img image.Image) color.Color {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 取中间1/3区域
	startX := bounds.Min.X + width/3
	endX := bounds.Min.X + width*2/3
	startY := bounds.Min.Y + height/3
	endY := bounds.Min.Y + height*2/3

	colorCount := make(map[uint32]int)
	step := 5 // 中间区域像素较少，步长可适当减小

	for y := startY; y < endY; y += step {
		for x := startX; x < endX; x += step {
			r, g, b, a := img.At(x, y).RGBA()
			if a < 32768 {
				continue
			}
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)
			colorKey := uint32(r8)<<16 | uint32(g8)<<8 | uint32(b8)
			colorCount[colorKey]++
		}
	}

	var dominantColorKey uint32
	maxCount := 0
	for colorKey, count := range colorCount {
		if count > maxCount {
			maxCount = count
			dominantColorKey = colorKey
		}
	}

	if maxCount == 0 {
		return color.RGBA{255, 255, 255, 255}
	}

	r := uint8((dominantColorKey >> 16) & 0xFF)
	g := uint8((dominantColorKey >> 8) & 0xFF)
	b := uint8(dominantColorKey & 0xFF)
	return color.RGBA{r, g, b, 255}
}
