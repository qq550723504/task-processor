// Package amazon 提供爬虫处理器实现
package amazon

import (
	"context"
	"encoding/json"
	"fmt"

	"task-processor/internal/crawler/shared"
	"task-processor/internal/infra/worker"
)

// AsyncTaskProcessor 实现 worker.Processor 接口，负责 Amazon 异步任务消费。
type AsyncTaskProcessor struct {
	service *Service
}

// Start 启动处理器
func (p *AsyncTaskProcessor) Start(_ context.Context) error { return nil }

// Close 关闭处理器
func (p *AsyncTaskProcessor) Close(_ context.Context) {}

// ProcessTask 处理任务
func (p *AsyncTaskProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	var crawlerTask shared.CrawlerTask
	if err := json.Unmarshal([]byte(job.TaskData), &crawlerTask); err != nil {
		return fmt.Errorf("解析任务数据失败: %w", err)
	}

	zipcode := p.service.getZipcodeForTask(&crawlerTask)
	region := p.service.resolveMetricsRegion(crawlerTask.Region, crawlerTask.URL)
	if err := p.service.checkRegionGuard(region); err != nil {
		p.service.metrics.RecordFailure("async_task", region, err)
		return err
	}

	product, _, err := p.service.fetchProduct(ctx, "async_task", crawlerTask.URL, crawlerTask.ASIN, crawlerTask.Region, zipcode)
	if err != nil {
		return err
	}

	if err := p.service.UpdateResult(crawlerTask.TaskID, func(result *shared.CrawlerResult) {
		result.ProductData = shared.ProductToMap(product)
	}); err != nil {
		return fmt.Errorf("update crawler result: %w", err)
	}

	return nil
}
