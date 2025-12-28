// Package handlers 提供TEMU平台并行图片验证功能
package handlers

import (
	"fmt"
	"sync"
	"task-processor/internal/platforms/temu/types"

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
		logger:          logrus.WithField("component", "ParallelImageValidator"),
		singleValidator: NewSingleImageValidator(),
	}
}

// ValidateImagesInParallel 并行验证多张图片
func (v *ParallelImageValidator) ValidateImagesInParallel(images []types.ImageInfo, imageType string, requirement ImageRequirement) []*types.ImageValidationResult {
	if len(images) == 0 {
		return []*types.ImageValidationResult{}
	}

	// 控制并发数，避免过多goroutine
	maxConcurrency := 5
	if len(images) < maxConcurrency {
		maxConcurrency = len(images)
	}

	semaphore := make(chan struct{}, maxConcurrency)
	results := make([]*types.ImageValidationResult, len(images))
	var wg sync.WaitGroup

	for i, img := range images {
		wg.Add(1)
		go func(index int, imageURL string) {
			defer func() {
				if r := recover(); r != nil {
					v.logger.Errorf("并行图片验证goroutine panic recovered: %v", r)
				}
			}()
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
