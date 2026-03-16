// Package image 提供TEMU平台图片上传并发工作器
package image

import (
	"fmt"
	"task-processor/internal/pkg/recovery"
	models "task-processor/internal/platforms/temu/api/product"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// uploadResult 内部上传结果结构体（用于并行上传结果收集）
type uploadResult struct {
	index int
	img   *models.ImageInfo
	err   error
}

// uploadSkuResult SKU图片上传结果
type uploadSkuResult struct {
	skcIndex int
	skuIndex int
	imgIndex int
	imgType  string // "carousel" or "dimension"
	img      *models.ImageInfo
	err      error
}

// ImageUploadWorker 图片上传工作器（处理并发上传）
type ImageUploadWorker struct {
	logger *logrus.Entry
}

// NewImageUploadWorker 创建新的图片上传工作器
func NewImageUploadWorker() *ImageUploadWorker {
	return &ImageUploadWorker{
		logger: logrus.WithField("component", "ImageUploadWorker"),
	}
}

// UploadMainImagesParallel 并行上传主图
func (w *ImageUploadWorker) UploadMainImagesParallel(ctx *temucontext.TemuTaskContext,
	mainImages []models.ImageInfo, uploadFunc func(*temucontext.TemuTaskContext, string, string) (*models.ImageInfo, error)) error {

	if len(mainImages) == 0 {
		return nil
	}

	w.logger.Infof("开始并行上传 %d 张主图", len(mainImages))

	// 使用带缓冲的channel收集结果，避免goroutine阻塞
	resultChan := make(chan uploadResult, len(mainImages))

	// 并行上传所有主图
	for i, img := range mainImages {
		go func(index int, imageURL string) {
			defer recovery.RecoverWithCallback("主图上传", w.logger, func(r any) {
				select {
				case resultChan <- uploadResult{index: index, img: nil, err: fmt.Errorf("goroutine panic: %v", r)}:
				default:
					w.logger.Errorf("无法发送panic结果到channel")
				}
			})

			uploadedImg, err := uploadFunc(ctx, imageURL, "main")
			select {
			case resultChan <- uploadResult{index: index, img: uploadedImg, err: err}:
			default:
				w.logger.Errorf("无法发送上传结果到channel")
			}
		}(i, img.URL)
	}

	// 收集所有结果
	successCount := 0
	failCount := 0

	// 检查TEMU产品信息
	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	for i := 0; i < len(mainImages); i++ {
		result := <-resultChan
		if result.err != nil {
			w.logger.Errorf("主图[%d] 上传失败: %v", result.index, result.err)
			failCount++
		} else if result.img != nil {
			// 修复：添加nil检查
			ctx.TemuProduct.GoodsBasic.GoodsGallery.DetailImage[result.index] = *result.img
			successCount++
		}
	}

	w.logger.Infof("主图上传完成: 成功 %d/%d, 失败 %d", successCount, len(mainImages), failCount)

	if failCount > 0 && successCount == 0 {
		return fmt.Errorf("所有主图上传失败")
	}

	return nil
}

// UploadSkuImagesParallel 并行上传SKU图片
func (w *ImageUploadWorker) UploadSkuImagesParallel(ctx *temucontext.TemuTaskContext,
	uploadFunc func(*temucontext.TemuTaskContext, string, string) (*models.ImageInfo, error)) error {

	// 检查TEMU产品信息
	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

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

	w.logger.Infof("开始并行上传 %d 张SKU图片", totalImages)

	// 使用带缓冲的channel收集结果
	resultChan := make(chan uploadSkuResult, totalImages)

	// 并行上传所有SKU图片
	for skcIndex, skc := range ctx.TemuProduct.SkcList {
		for skuIndex, sku := range skc.SkuList {
			// 构建SKU上下文键
			skuContextKey := fmt.Sprintf("skc%d-sku%d", skcIndex, skuIndex)

			// 上传轮播图片
			for imgIndex, img := range sku.CarouselGallery {
				contextKey := fmt.Sprintf("%s-carousel", skuContextKey)
				go w.uploadSkuImageWorkerWithContext(ctx, skcIndex, skuIndex, imgIndex, img.URL, "carousel", contextKey, uploadFunc, resultChan)
			}

			// 上传尺寸图片（每张尺寸图都有独立的上下文）
			for imgIndex, img := range sku.DimensionGallery {
				contextKey := fmt.Sprintf("%s-dimension-%d", skuContextKey, imgIndex)
				go w.uploadSkuImageWorkerWithContext(ctx, skcIndex, skuIndex, imgIndex, img.URL, "dimension", contextKey, uploadFunc, resultChan)
			}
		}
	}

	// 收集所有结果
	successCount := 0
	failCount := 0
	for i := 0; i < totalImages; i++ {
		result := <-resultChan
		if result.err != nil {
			w.logger.Errorf("SKU[%d-%d]%s图[%d] 上传失败: %v",
				result.skcIndex, result.skuIndex, result.imgType, result.imgIndex, result.err)
			failCount++
		} else if result.img != nil {
			// 修复：添加nil检查和安全的数组访问
			w.updateSkuImage(ctx, result)
			w.logger.Debugf("SKU[%d-%d]%s图[%d] 上传成功",
				result.skcIndex, result.skuIndex, result.imgType, result.imgIndex)
			successCount++
		}
	}

	w.logger.Infof("SKU图片上传完成: 成功 %d/%d, 失败 %d", successCount, totalImages, failCount)

	return nil
}

// uploadSkuImageWorkerWithContext SKU图片上传工作协程（带上下文）
func (w *ImageUploadWorker) uploadSkuImageWorkerWithContext(ctx *temucontext.TemuTaskContext, skcIndex, skuIndex, imgIndex int,
	imageURL, imgType, contextKey string, uploadFunc func(*temucontext.TemuTaskContext, string, string) (*models.ImageInfo, error),
	resultChan chan uploadSkuResult) {

	defer recovery.RecoverWithCallback("SKU图片上传", w.logger, func(r any) {
		select {
		case resultChan <- uploadSkuResult{
			skcIndex: skcIndex,
			skuIndex: skuIndex,
			imgIndex: imgIndex,
			imgType:  imgType,
			img:      nil,
			err:      fmt.Errorf("goroutine panic: %v", r),
		}:
		default:
			w.logger.Errorf("无法发送panic结果到channel")
		}
	})

	var result uploadSkuResult
	result.skcIndex = skcIndex
	result.skuIndex = skuIndex
	result.imgIndex = imgIndex
	result.imgType = imgType

	// 创建带上下文的上传函数
	uploadedImg, err := w.uploadWithContext(ctx, imageURL, imgType, contextKey, uploadFunc)
	result.img = uploadedImg
	result.err = err

	select {
	case resultChan <- result:
	default:
		w.logger.Errorf("无法发送SKU图片结果到channel")
	}
}

// uploadWithContext 带上下文的图片上传（通过反射调用带上下文的方法）
func (w *ImageUploadWorker) uploadWithContext(ctx *temucontext.TemuTaskContext, imageURL, imgType, contextKey string,
	uploadFunc func(*temucontext.TemuTaskContext, string, string) (*models.ImageInfo, error)) (*models.ImageInfo, error) {

	// 临时将上下文存储在TemuTaskContext中
	originalContext := ctx.CurrentSkuContext
	ctx.CurrentSkuContext = contextKey

	defer func() {
		ctx.CurrentSkuContext = originalContext
	}()

	return uploadFunc(ctx, imageURL, imgType)
}

// updateSkuImage 更新SKU图片信息
func (w *ImageUploadWorker) updateSkuImage(ctx *temucontext.TemuTaskContext, result uploadSkuResult) {
	// 检查TEMU产品信息
	if ctx.TemuProduct == nil {
		w.logger.Error("无法获取TEMU产品信息")
		return
	}

	if result.imgType == "carousel" &&
		result.skcIndex < len(ctx.TemuProduct.SkcList) &&
		result.skuIndex < len(ctx.TemuProduct.SkcList[result.skcIndex].SkuList) &&
		result.imgIndex < len(ctx.TemuProduct.SkcList[result.skcIndex].SkuList[result.skuIndex].CarouselGallery) {
		ctx.TemuProduct.SkcList[result.skcIndex].SkuList[result.skuIndex].CarouselGallery[result.imgIndex] = *result.img
	} else if result.imgType == "dimension" &&
		result.skcIndex < len(ctx.TemuProduct.SkcList) &&
		result.skuIndex < len(ctx.TemuProduct.SkcList[result.skcIndex].SkuList) &&
		result.imgIndex < len(ctx.TemuProduct.SkcList[result.skcIndex].SkuList[result.skuIndex].DimensionGallery) {
		ctx.TemuProduct.SkcList[result.skcIndex].SkuList[result.skuIndex].DimensionGallery[result.imgIndex] = *result.img
	}
}
