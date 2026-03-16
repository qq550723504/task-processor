// Package amazon 提供爬虫处理器实现
package amazon

import (
	"context"
	"encoding/json"
	"fmt"

	"task-processor/internal/domain/task"
	"task-processor/internal/infra/worker"
)

// CrawlerProcessor 实现 worker.Processor 接口
type CrawlerProcessor struct {
	service *Service
}

// Start 启动处理器
func (p *CrawlerProcessor) Start(ctx context.Context) error {
	return nil
}

// ProcessTask 处理任务
func (p *CrawlerProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	// 从 WorkerJob 中解析出 CrawlerTask
	var crawlerTask task.CrawlerTask
	if err := json.Unmarshal([]byte(job.TaskData), &crawlerTask); err != nil {
		return fmt.Errorf("解析任务数据失败: %w", err)
	}

	// 获取邮编
	zipcode := p.service.getZipcodeForTask(&crawlerTask)

	// 执行爬取
	product, err := p.service.amazonProcessor.Process(crawlerTask.URL, zipcode)
	if err != nil {
		return err
	}

	// 保存结果（原子操作）
	p.service.updateResult(crawlerTask.TaskID, func(result *task.CrawlerResult) {
		result.ProductData = productToMap(product, p.service.logger)
	})

	return nil
}

// Close 关闭处理器
func (p *CrawlerProcessor) Close(ctx context.Context) {
	// 清理资源（如果需要）
}
