// Package amazon 提供Amazon处理器包装功能
package amazon

import (
	"context"
	"fmt"
	"task-processor/internal/model"
	"time"

	"github.com/sirupsen/logrus"
)

// ProcessorWrapper Amazon处理器包装器，提供超时控制
type ProcessorWrapper struct {
	processor *AmazonProcessor
	logger    *logrus.Entry
}

// NewProcessorWrapper 创建处理器包装器
func NewProcessorWrapper(processor *AmazonProcessor) *ProcessorWrapper {
	return &ProcessorWrapper{
		processor: processor,
		logger:    logrus.WithField("component", "ProcessorWrapper"),
	}
}

// ProcessWithTimeout 带超时处理产品
func (pw *ProcessorWrapper) ProcessWithTimeout(url, zipcode string, timeout time.Duration) (*model.Product, error) {
	if timeout <= 0 {
		timeout = 3 * time.Minute // 默认3分钟超时
	}

	pw.logger.Infof("开始处理产品，超时时间: %v, URL: %s", timeout, url)

	// 创建超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 使用channel来处理结果
	resultChan := make(chan *ProcessResult, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				pw.logger.Errorf("处理产品时发生panic: %v", r)
				resultChan <- &ProcessResult{
					Product: nil,
					Error:   fmt.Errorf("处理产品时发生panic: %v", r),
				}
			}
		}()

		// 调用原始处理器
		product, err := pw.processor.Process(url, zipcode)
		resultChan <- &ProcessResult{
			Product: product,
			Error:   err,
		}
	}()

	select {
	case result := <-resultChan:
		if result.Error != nil {
			pw.logger.Errorf("处理产品失败: %v", result.Error)
		} else {
			pw.logger.Infof("处理产品成功: ASIN=%s", result.Product.Asin)
		}
		return result.Product, result.Error
	case <-ctx.Done():
		pw.logger.Errorf("处理产品超时: URL=%s, 超时时间=%v", url, timeout)
		return nil, fmt.Errorf("处理产品超时: %v", timeout)
	}
}

// ProcessResult 处理结果
type ProcessResult struct {
	Product *model.Product
	Error   error
}
