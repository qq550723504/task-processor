// Package amazon 提供Amazon批量处理功能
package amazon

import (
	"context"

	"task-processor/internal/model"
)

type batchRequestProcessor interface {
	ProcessWithContext(ctx context.Context, url, zipcode string) (*model.Product, error)
}

// BatchProcessor 批量处理器。
// 这里只负责批量请求的顺序编排，实例获取、超时和重建统一交给单请求处理链路。
type BatchProcessor struct{}

// NewBatchProcessor 创建批量处理器
func NewBatchProcessor() *BatchProcessor {
	return &BatchProcessor{}
}

// Process 兼容不传 context 的旧调用方。
func (bp *BatchProcessor) Process(requests []model.ProductRequest, processor batchRequestProcessor) []model.ProductResult {
	return bp.ProcessWithContext(context.Background(), requests, processor)
}

// ProcessWithContext 顺序处理批量请求，复用单请求处理链路。
func (bp *BatchProcessor) ProcessWithContext(ctx context.Context, requests []model.ProductRequest, processor batchRequestProcessor) []model.ProductResult {
	if len(requests) == 0 {
		return []model.ProductResult{}
	}

	results := make([]model.ProductResult, len(requests))
	if processor == nil {
		for i := range requests {
			results[i] = model.ProductResult{Error: context.Canceled}
		}
		return results
	}

	for i, req := range requests {
		if err := ctx.Err(); err != nil {
			for j := i; j < len(requests); j++ {
				results[j] = model.ProductResult{Error: err}
			}
			return results
		}

		product, err := processor.ProcessWithContext(ctx, req.URL, req.Zipcode)
		results[i] = model.ProductResult{
			Product: product,
			Error:   err,
		}
	}

	return results
}
