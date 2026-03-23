// Package amazon 提供爬虫处理器实现
package amazon

import (
	"context"
	"encoding/json"
	"fmt"

	"task-processor/internal/crawler/shared"
	"task-processor/internal/infra/worker"
)

// CrawlerProcessor 实现 worker.Processor 接口
type CrawlerProcessor struct {
	service *Service
}

// Start 启动处理器
func (p *CrawlerProcessor) Start(_ context.Context) error { return nil }

// Close 关闭处理器
func (p *CrawlerProcessor) Close(_ context.Context) {}

// ProcessTask 处理任务
func (p *CrawlerProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	var crawlerTask shared.CrawlerTask
	if err := json.Unmarshal([]byte(job.TaskData), &crawlerTask); err != nil {
		return fmt.Errorf("解析任务数据失败: %w", err)
	}

	zipcode := p.service.getZipcodeForTask(&crawlerTask)

	product, err := p.service.amazonProcessor.Process(crawlerTask.URL, zipcode)
	if err != nil {
		return err
	}

	p.service.UpdateResult(crawlerTask.TaskID, func(result *shared.CrawlerResult) {
		result.ProductData = shared.ProductToMap(product)
	})

	return nil
}
