// Package alibaba1688 提供1688爬虫处理器
package alibaba1688

import (
	"context"
	"encoding/json"
	"fmt"

	"task-processor/internal/crawler/shared"
	"task-processor/internal/infra/worker"
)

// Crawler1688Processor 实现 worker.Processor 接口
type Crawler1688Processor struct {
	service *Service
}

// Start 启动处理器
func (p *Crawler1688Processor) Start(_ context.Context) error { return nil }

// Close 关闭处理器
func (p *Crawler1688Processor) Close(_ context.Context) {}

// ProcessTask 处理任务
func (p *Crawler1688Processor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	var crawlerTask shared.CrawlerTask
	if err := json.Unmarshal([]byte(job.TaskData), &crawlerTask); err != nil {
		return fmt.Errorf("解析任务数据失败: %w", err)
	}

	product, err := p.service.processor1688.Process(crawlerTask.URL)
	if err != nil {
		return err
	}

	p.service.UpdateResult(crawlerTask.TaskID, func(result *shared.CrawlerResult) {
		result.ProductData = shared.ProductToMap(product)
	})

	return nil
}
