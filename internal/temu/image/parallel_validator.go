// Package image 提供TEMU平台并行图片验证功能
package image

import (
	"fmt"
	"sync"
	"task-processor/internal/pkg/recovery"
	temuimage "task-processor/internal/temu/api/image"
	temuproduct "task-processor/internal/temu/api/product"
	"task-processor/internal/temu/handlerbase"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// ParallelImageValidator 并行图片验证器
type ParallelImageValidator struct {
	logger          *logrus.Entry
	singleValidator *SingleImageValidator
}

// NewParallelImageValidator 创建新的并行图片验证器
func NewParallelImageValidator() *ParallelImageValidator {
	return &ParallelImageValidator{
		logger:          logger.GetGlobalLogger("ParallelImageValidator"),
		singleValidator: NewSingleImageValidator(),
	}
}

// ValidateImagesInParallel 并行验证多张图片
func (v *ParallelImageValidator) ValidateImagesInParallel(images []temuproduct.ImageInfo, imageType string, requirement handlerbase.ImageRequirement) []*temuimage.ValidationResult {
	if len(images) == 0 {
		return []*temuimage.ValidationResult{}
	}

	// 控制并发数，避免过多goroutine
	maxConcurrency := 5
	if len(images) < maxConcurrency {
		maxConcurrency = len(images)
	}

	semaphore := make(chan struct{}, maxConcurrency)
	results := make([]*temuimage.ValidationResult, len(images))
	var wg sync.WaitGroup

	for i, img := range images {
		wg.Add(1)
		go func(index int, imageURL string) {
			defer recovery.Recover("并行图片验证", v.logger)
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 验证图片
			context := fmt.Sprintf("%s[%d]", imageType, index)
			results[index] = v.singleValidator.ValidateSingleImage(imageURL, context, requirement)
		}(i, img.URL)
	}

	wg.Wait()
	v.logger.Infof("✅ 并行验证完成: %d 张图片", len(images))

	return results
}
