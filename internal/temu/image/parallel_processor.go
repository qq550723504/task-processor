// Package image 提供并行图片处理功能
package image

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/model"
	"task-processor/internal/pkg/goroutine"
	"task-processor/internal/pkg/perf"
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// ParallelImageProcessor 并行图片处理器
type ParallelImageProcessor struct {
	imageProcessor *ImageProcessor
	maxWorkers     int
	timeout        time.Duration
	logger         *logrus.Entry
	mutex          sync.RWMutex // 保护并发访问
}

// NewParallelImageProcessor 创建并行图片处理器
func NewParallelImageProcessor(maxWorkers int) *ParallelImageProcessor {
	if maxWorkers <= 0 {
		maxWorkers = 3 // 默认3个并发
	}

	return &ParallelImageProcessor{
		imageProcessor: NewImageProcessor(),
		maxWorkers:     maxWorkers,
		timeout:        5 * time.Minute, // 每个变体图片处理5分钟超时
		logger:         logger.GetGlobalLogger("ParallelImageProcessor"),
	}
}

// ImageProcessingTask 图片处理任务
type ImageProcessingTask struct {
	VariantIndex int
	Variant      *model.Product
	TemuCtx      *temucontext.TemuTaskContext
}

// ImageProcessingResult 图片处理结果
type ImageProcessingResult struct {
	VariantIndex     int
	DimensionGallery []models.ImageInfo
	CarouselGallery  []models.ImageInfo
	Error            error
	Success          bool
}

// ProcessVariantImagesParallel 并行处理多个变体的图片
func (pip *ParallelImageProcessor) ProcessVariantImagesParallel(temuCtx *temucontext.TemuTaskContext, variants []*model.Product) ([]*ImageProcessingResult, error) {
	if len(variants) == 0 {
		return []*ImageProcessingResult{}, nil
	}

	// 创建性能跟踪器
	tracker := perf.NewTracker(fmt.Sprintf("并行图片处理-%d个变体", len(variants)), pip.logger)
	defer tracker.Finish()

	tracker.StartStep("准备并行图片处理任务")

	// 创建并行处理器
	processor := goroutine.NewProcessor(pip.maxWorkers, pip.timeout, pip.logger)

	// 创建处理任务
	tasks := make([]*goroutine.Task, len(variants))
	for i, variant := range variants {
		tasks[i] = &goroutine.Task{
			Index: i,
			ID:    fmt.Sprintf("variant-%s", variant.Asin),
			Data: &ImageProcessingTask{
				VariantIndex: i,
				Variant:      variant,
				TemuCtx:      temuCtx,
			},
		}
	}

	tracker.EndStep()
	tracker.StartStep("执行并行图片处理")

	// 定义处理函数
	processFunc := func(ctx context.Context, task *goroutine.Task) (any, error) {
		imageTask, ok := task.Data.(*ImageProcessingTask)
		if !ok {
			return nil, fmt.Errorf("任务数据类型错误")
		}
		return pip.processVariantImages(ctx, imageTask), nil
	}

	// 并行执行处理
	results := processor.ProcessParallel(context.Background(), tasks, processFunc)

	tracker.EndStep()
	tracker.StartStep("收集处理结果")

	// 转换结果
	imageResults := make([]*ImageProcessingResult, len(results))
	successCount := 0

	for _, result := range results {
		if result.Success && result.Data != nil {
			if imgResult, ok := result.Data.(*ImageProcessingResult); ok {
				imageResults[result.Index] = imgResult
				if imgResult.Success {
					successCount++
				}
			}
		} else {
			// 创建失败结果
			imageResults[result.Index] = &ImageProcessingResult{
				VariantIndex:     result.Index,
				DimensionGallery: []models.ImageInfo{},
				CarouselGallery:  []models.ImageInfo{},
				Error:            result.Error,
				Success:          false,
			}
		}
	}

	tracker.EndStep()

	pip.logger.Infof("🎉 并行图片处理完成: 成功 %d/%d 个变体", successCount, len(variants))

	return imageResults, nil
}

// processVariantImages 处理单个变体的图片
func (pip *ParallelImageProcessor) processVariantImages(_ context.Context, task *ImageProcessingTask) *ImageProcessingResult {
	result := &ImageProcessingResult{
		VariantIndex:     task.VariantIndex,
		DimensionGallery: []models.ImageInfo{},
		CarouselGallery:  []models.ImageInfo{},
		Success:          false,
	}

	pip.logger.Infof("📸 开始处理变体[%d]图片: %s", task.VariantIndex, task.Variant.Asin)

	dimensionGallery, err := pip.imageProcessor.BuildDimensionImagesWithUpload(task.TemuCtx, task.Variant)
	if err != nil {
		pip.logger.Errorf("❌ 变体[%d]尺寸图处理失败: %v", task.VariantIndex, err)
		result.Error = fmt.Errorf("尺寸图处理失败: %w", err)
		return result
	}
	result.DimensionGallery = dimensionGallery

	carouselGallery, err := pip.imageProcessor.BuildCarouselImagesWithoutAnnotation(task.TemuCtx, task.Variant)
	if err != nil {
		pip.logger.Errorf("❌ 变体[%d]轮播图处理失败: %v", task.VariantIndex, err)
		result.Error = fmt.Errorf("轮播图处理失败: %w", err)
		return result
	}
	result.CarouselGallery = carouselGallery

	result.Success = true
	pip.logger.Infof("✅ 变体[%d]图片处理完成: 尺寸图%d张, 轮播图%d张",
		task.VariantIndex, len(dimensionGallery), len(carouselGallery))

	return result
}

// ApplyImageResults 将图片处理结果应用到SKU中
func (pip *ParallelImageProcessor) ApplyImageResults(skuList []models.Sku, imageResults []*ImageProcessingResult) {
	pip.logger.Info("📋 开始应用图片处理结果到SKU")

	for i, result := range imageResults {
		if result == nil || !result.Success {
			pip.logger.Warnf("⚠️ 跳过变体[%d]的图片结果应用（处理失败）", i)
			continue
		}

		if i >= len(skuList) {
			pip.logger.Warnf("⚠️ 变体索引[%d]超出SKU列表范围[%d]", i, len(skuList))
			continue
		}

		// 应用图片结果到对应的SKU
		sku := &skuList[i]
		sku.DimensionGallery = result.DimensionGallery
		sku.CarouselGallery = result.CarouselGallery

		// 限制图片总数不超过10张
		const maxTotalImages = 10
		totalImages := len(sku.DimensionGallery) + len(sku.CarouselGallery)
		if totalImages > maxTotalImages {
			// 优先保留尺寸图，然后是轮播图
			remainingSlots := maxTotalImages - len(sku.DimensionGallery)
			if remainingSlots < 0 {
				// 如果尺寸图就超过10张，只保留前10张尺寸图
				sku.DimensionGallery = sku.DimensionGallery[:maxTotalImages]
				sku.CarouselGallery = []models.ImageInfo{}
				pip.logger.Warnf("⚠️ SKU[%d]图片总数超限，尺寸图截断为%d张，轮播图清空", i, maxTotalImages)
			} else if remainingSlots < len(sku.CarouselGallery) {
				// 截断轮播图
				sku.CarouselGallery = sku.CarouselGallery[:remainingSlots]
				pip.logger.Warnf("⚠️ SKU[%d]图片总数超限，轮播图截断为%d张", i, remainingSlots)
			}
		}

		pip.logger.Infof("✅ SKU[%d]图片应用完成: 尺寸图%d张, 轮播图%d张",
			i, len(sku.DimensionGallery), len(sku.CarouselGallery))
	}

	pip.logger.Info("📋 图片处理结果应用完成")
}
