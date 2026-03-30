// Package amazon 提供Amazon批量处理功能
package amazon

import (
	"context"
	"fmt"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/amazon/browser"
	"task-processor/internal/model"
)

// BatchProcessor 批量处理器
type BatchProcessor struct {
	urlHelper      *URLHelper
	productChecker *ProductChecker
	qualityControl config.AmazonQualityControlConfig
	qualityMetrics qualityMetricsRecorder
}

// NewBatchProcessor 创建批量处理器
func NewBatchProcessor(urlHelper *URLHelper, productChecker *ProductChecker, qualityControl config.AmazonQualityControlConfig) *BatchProcessor {
	return &BatchProcessor{
		urlHelper:      urlHelper,
		productChecker: productChecker,
		qualityControl: qualityControl,
		qualityMetrics: nil,
	}
}

func (bp *BatchProcessor) SetQualityMetricsRecorder(recorder qualityMetricsRecorder) {
	bp.qualityMetrics = recorder
}

// batchPool 定义 BatchProcessor 依赖的浏览器池行为
type batchPool interface {
	Acquire() (*browser.BrowserInstance, error)
	IsBlockedOrSeriousError(err error) bool
	RecreateInstanceSync(old *browser.BrowserInstance) *browser.BrowserInstance
	RecreateInstanceAsync(old *browser.BrowserInstance)
	ReleaseWithError(instance *browser.BrowserInstance, err error)
}

// ProcessWithPool 使用浏览器池批量处理
func (bp *BatchProcessor) ProcessWithPool(requests []model.ProductRequest, pool batchPool) []model.ProductResult {
	results := make([]model.ProductResult, len(requests))

	// 使用浏览器池批量处理
	instance, err := pool.Acquire()
	if err != nil {
		// 如果获取实例失败，所有任务都失败
		for i := range results {
			results[i] = model.ProductResult{
				Product: nil,
				Error:   fmt.Errorf("获取浏览器实例失败: %w", err),
			}
		}
		return results
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("使用浏览器实例 %d 批量处理", instance.ID)

	// 跟踪是否有严重错误需要重建实例
	var lastError error

	// 使用同一个实例处理所有请求
	instanceProcessor := NewInstanceProcessor(bp.urlHelper, bp.productChecker)
	instanceProcessor.SetQualityControlOptions(bp.qualityControl.RetryOnValidationFailure, bp.qualityControl.ValidationRetryMaxAttempts)
	instanceProcessor.SetQualityMetricsRecorder(bp.qualityMetrics)
	for i, req := range requests {
		product, err := instanceProcessor.ProcessWithInstance(context.Background(), instance, req.URL, req.Zipcode)
		results[i] = model.ProductResult{
			Product: product,
			Error:   err,
		}

		if err != nil {
			lastError = err
			logger.GetGlobalLogger("crawler/amazon").Infof("批量处理 [%d/%d] 失败: %s - %v", i+1, len(requests), req.URL, err)

			// 如果检测到风控或严重错误，尝试重建实例并继续
			if pool.IsBlockedOrSeriousError(err) {
				logger.GetGlobalLogger("crawler/amazon").Warnf("检测到浏览器实例 %d 出现严重错误: %v", instance.ID, err)

				// 尝试同步重建实例
				newInstance := pool.RecreateInstanceSync(instance)
				if newInstance != nil {
					logger.GetGlobalLogger("crawler/amazon").Infof("成功重建浏览器实例，继续处理剩余任务")
					instance = newInstance
					// 继续处理，不跳出循环
				} else {
					logger.GetGlobalLogger("crawler/amazon").Errorf("重建浏览器实例失败，停止批量处理")
					// 旧实例已被关闭，异步补充避免池永久缩容
					pool.RecreateInstanceAsync(instance)
					// 将剩余任务标记为失败
					for j := i + 1; j < len(requests); j++ {
						results[j] = model.ProductResult{
							Product: nil,
							Error:   fmt.Errorf("浏览器实例重建失败，跳过处理"),
						}
					}
					return results
				}
			}
		} else {
			logger.GetGlobalLogger("crawler/amazon").Infof("批量处理 [%d/%d] 成功: %s", i+1, len(requests), product.Asin)
		}
	}

	// 使用带错误检测的释放方法
	pool.ReleaseWithError(instance, lastError)

	return results
}

// ProcessWithSingleBrowser 已移除：逻辑已内联到 AmazonProcessor.ProcessBatch
