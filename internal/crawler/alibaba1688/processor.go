// Package alibaba1688 提供1688平台处理器核心功能
package alibaba1688

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688/model"
	"time"

)

// Alibaba1688Processor 1688爬虫处理器
type Alibaba1688Processor struct {
	config          *config.Config
	singleProcessor *SingleProcessor
	urlHelper       *URLHelper
	productChecker  *ProductChecker
}

// NewAlibaba1688Processor 使用全局配置创建1688处理器
func NewAlibaba1688Processor(cfg *config.Config) *Alibaba1688Processor {
	logger.GetGlobalLogger("crawler/alibaba1688").Infof("创建1688处理器")

	// 创建辅助组件
	urlHelper := NewURLHelper()
	productChecker := NewProductChecker()
	singleProcessor := NewSingleProcessor(cfg, urlHelper, productChecker)

	return &Alibaba1688Processor{
		config:          cfg,
		singleProcessor: singleProcessor,
		urlHelper:       urlHelper,
		productChecker:  productChecker,
	}
}

// Process 处理1688产品页面
func (ap *Alibaba1688Processor) Process(url string) (*model.Product1688, error) {
	startTime := time.Now()
	logger.GetGlobalLogger("crawler/alibaba1688").Infof("开始处理1688产品: %s", url)

	return ap.singleProcessor.ProcessWithSingleBrowser(url, startTime)
}

// ProcessBatch 批量处理多个1688产品页面
func (ap *Alibaba1688Processor) ProcessBatch(requests []model.Product1688Request) []model.Product1688Result {
	if len(requests) == 0 {
		return []model.Product1688Result{}
	}

	logger.GetGlobalLogger("crawler/alibaba1688").Infof("开始批量处理 %d 个1688产品", len(requests))
	startTime := time.Now()

	results := make([]model.Product1688Result, len(requests))

	for i, request := range requests {
		requestStartTime := time.Now()

		logger.GetGlobalLogger("crawler/alibaba1688").Infof("处理产品 %d/%d: %s", i+1, len(requests), request.URL)

		product, err := ap.singleProcessor.ProcessWithSingleBrowser(request.URL, requestStartTime)

		results[i] = model.Product1688Result{
			Request:   request,
			Product:   product,
			Error:     err,
			Duration:  time.Since(requestStartTime),
			Timestamp: time.Now(),
		}

		if err != nil {
			logger.GetGlobalLogger("crawler/alibaba1688").Errorf("处理产品失败 %d/%d: %v", i+1, len(requests), err)
		} else {
			logger.GetGlobalLogger("crawler/alibaba1688").Infof("处理产品成功 %d/%d: %s", i+1, len(requests), product.Title)
		}

		// 添加延迟以避免过于频繁的请求
		if i < len(requests)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	duration := time.Since(startTime)
	successCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
		}
	}

	logger.GetGlobalLogger("crawler/alibaba1688").Infof("批量处理完成: 成功 %d/%d, 耗时: %v", successCount, len(requests), duration)
	return results
}

// Shutdown 关闭处理器
func (ap *Alibaba1688Processor) Shutdown() {
	logger.GetGlobalLogger("crawler/alibaba1688").Info("关闭1688处理器...")
}
