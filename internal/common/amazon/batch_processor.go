// Package amazon 提供Amazon批量处理功能
package amazon

import (
	"fmt"
	"task-processor/internal/common/amazon/browser"
	"task-processor/internal/common/amazon/model"

	"github.com/sirupsen/logrus"
)

// BatchProcessor 批量处理器
type BatchProcessor struct {
	urlHelper      *URLHelper
	productChecker *ProductChecker
}

// NewBatchProcessor 创建批量处理器
func NewBatchProcessor(browserPool *browser.BrowserPool, urlHelper *URLHelper, productChecker *ProductChecker) *BatchProcessor {
	return &BatchProcessor{
		urlHelper:      urlHelper,
		productChecker: productChecker,
	}
}

// ProcessWithPool 使用浏览器池批量处理
func (bp *BatchProcessor) ProcessWithPool(requests []model.ProductRequest, browserPool *browser.BrowserPool) []model.ProductResult {
	results := make([]model.ProductResult, len(requests))

	// 使用浏览器池批量处理
	instance, err := browserPool.Acquire()
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

	logrus.Infof("使用浏览器实例 %d 批量处理", instance.ID)

	// 跟踪是否有严重错误需要重建实例
	var lastError error

	// 使用同一个实例处理所有请求
	instanceProcessor := NewInstanceProcessor(bp.urlHelper, bp.productChecker)
	for i, req := range requests {
		product, err := instanceProcessor.ProcessWithInstance(instance, req.URL, req.Zipcode)
		results[i] = model.ProductResult{
			Product: product,
			Error:   err,
		}

		if err != nil {
			lastError = err
			logrus.Infof("批量处理 [%d/%d] 失败: %s - %v", i+1, len(requests), req.URL, err)

			// 如果检测到风控或严重错误，尝试重建实例并继续
			if browserPool.IsBlockedOrSeriousError(err) {
				logrus.Warnf("检测到浏览器实例 %d 出现严重错误: %v", instance.ID, err)

				// 尝试同步重建实例
				newInstance := browserPool.RecreateInstanceSync(instance)
				if newInstance != nil {
					logrus.Infof("成功重建浏览器实例，继续处理剩余任务")
					instance = newInstance
					// 继续处理，不跳出循环
				} else {
					logrus.Errorf("重建浏览器实例失败，停止批量处理")
					// 将剩余任务标记为失败
					for j := i + 1; j < len(requests); j++ {
						results[j] = model.ProductResult{
							Product: nil,
							Error:   fmt.Errorf("浏览器实例重建失败，跳过处理"),
						}
					}
					break
				}
			}
		} else {
			logrus.Infof("批量处理 [%d/%d] 成功: %s", i+1, len(requests), product.Asin)
		}
	}

	// 使用带错误检测的释放方法
	browserPool.ReleaseWithError(instance, lastError)

	return results
}

// ProcessWithSingleBrowser 使用单浏览器模式批量处理
func (bp *BatchProcessor) ProcessWithSingleBrowser(requests []model.ProductRequest, processor interface {
	Process(string, string) (*model.Product, error)
}) []model.ProductResult {
	results := make([]model.ProductResult, len(requests))

	// 单浏览器模式，逐个处理
	for i, req := range requests {
		product, err := processor.Process(req.URL, req.Zipcode)
		results[i] = model.ProductResult{
			Product: product,
			Error:   err,
		}
	}

	return results
}
