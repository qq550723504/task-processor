// Package alibaba1688 提供1688爬虫处理器
package alibaba1688

import (
	"context"
	"encoding/json"
	"fmt"

	"task-processor/internal/domain/task"
	"task-processor/internal/infra/worker"
)

// Crawler1688Processor 实现 worker.Processor 接口
type Crawler1688Processor struct {
	service *Service
}

// Start 启动处理器
func (p *Crawler1688Processor) Start(ctx context.Context) error {
	return nil
}

// ProcessTask 处理任务
func (p *Crawler1688Processor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	// 从 WorkerJob 中解析出 CrawlerTask
	var crawlerTask task.CrawlerTask
	if err := json.Unmarshal([]byte(job.TaskData), &crawlerTask); err != nil {
		return fmt.Errorf("解析任务数据失败: %w", err)
	}

	// 执行爬取
	product, err := p.service.processor1688.Process(crawlerTask.URL)
	if err != nil {
		return err
	}

	// 保存结果（原子操作）
	p.service.updateResult(crawlerTask.TaskID, func(result *task.CrawlerResult) {
		result.ProductData = product1688ToMap(product, p.service.logger)
	})

	return nil
}

// Close 关闭处理器
func (p *Crawler1688Processor) Close(ctx context.Context) {
	// 清理资源（如果需要）
}

// product1688ToMap 将 1688 Product 转换为 map
func product1688ToMap(product any, _ any) map[string]any {
	if product == nil {
		return nil
	}

	data, err := json.Marshal(product)
	if err != nil {
		return nil
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}

	return result
}
