// Package processor 提供爬虫任务处理器
package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// CrawlerProcessor Amazon爬虫处理器
type CrawlerProcessor struct {
	amazonProcessor *amazon.AmazonProcessor
	productFetcher  *product.ProductFetcher
	taskSubmitter   VariantTaskSubmitter
	logger          *logrus.Logger
}

// NewCrawlerProcessor 创建爬虫处理器
func NewCrawlerProcessor(
	logger *logrus.Logger,
	amazonProcessor *amazon.AmazonProcessor,
	productFetcher *product.ProductFetcher,
	taskSubmitter VariantTaskSubmitter,
) *CrawlerProcessor {
	return &CrawlerProcessor{
		amazonProcessor: amazonProcessor,
		productFetcher:  productFetcher,
		taskSubmitter:   taskSubmitter,
		logger:          logger,
	}
}

// Start 启动处理器
func (p *CrawlerProcessor) Start(ctx context.Context) error {
	p.logger.Info("🌐 Amazon爬虫处理器启动完成")
	return nil
}

// ProcessTask 处理任务 - 实现worker.Processor接口
func (p *CrawlerProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	// 解析任务数据
	var task model.Task
	if err := json.Unmarshal([]byte(job.TaskData), &task); err != nil {
		return fmt.Errorf("解析任务数据失败: %w", err)
	}

	p.logger.Infof("🔍 开始爬取任务: ID=%d, ProductID=%s", task.ID, task.ProductID)

	startTime := time.Now()

	// 构建获取请求
	fetchReq := &product.FetchRequest{
		TenantID:   task.TenantID,
		Platform:   task.Platform,
		Region:     task.Region,
		ProductID:  task.ProductID,
		StoreID:    task.StoreID,
		CategoryID: task.CategoryID,
		Creator:    "crawler-consumer",
	}

	// 获取产品数据（会自动使用浏览器池，浏览器实例会被放回池中复用）
	productData, err := p.productFetcher.FetchProduct(fetchReq)
	if err != nil {
		p.logger.Errorf("❌ 爬取失败: ID=%d, ProductID=%s, Error=%v", task.ID, task.ProductID, err)
		return fmt.Errorf("爬取产品数据失败: %w", err)
	}

	// 打印产品基本信息
	p.logger.Infof("📦 产品ASIN: %s", productData.Asin)
	p.logger.Infof("💰 产品价格: %.2f %s", productData.FinalPrice, productData.Currency)

	// 保存产品数据到服务器（如果服务器已有数据则跳过）
	if err := p.productFetcher.CacheProduct(fetchReq, productData); err != nil {
		p.logger.Warnf("⚠️ 保存产品数据到服务器失败: %v", err)
		// 不返回错误，因为数据已经获取成功
	}

	// 只有主产品任务才提交变体任务（避免无限递归）
	// 通过Remark字段判断：如果Remark为"variant"，说明这是变体任务，不再提交变体
	if task.Remark != "variant" && len(productData.Variations) > 0 {
		p.logger.Infof("🔄 发现 %d 个变体，准备提交爬虫任务", len(productData.Variations))
		successCount, failCount := p.taskSubmitter.SubmitVariantTasks(ctx, &task, productData.Variations, productData.ParentAsin)
		p.logger.Infof("📤 变体任务提交完成: 成功=%d, 失败=%d, 总数=%d",
			successCount, failCount, len(productData.Variations))
	} else if task.Remark == "variant" {
		p.logger.Debugf("这是变体任务，跳过变体提交（避免递归）")
	}

	duration := time.Since(startTime)
	p.logger.Infof("✅ 爬取完成: ID=%d, ProductID=%s, 耗时=%v", task.ID, task.ProductID, duration)

	return nil
}

// Close 关闭处理器
func (p *CrawlerProcessor) Close(ctx context.Context) {
	p.logger.Info("🔒 关闭Amazon爬虫处理器")
	// 注意：不要在这里关闭amazonProcessor，因为它是共享的
	// amazonProcessor会在main函数退出时由serviceManager统一关闭
}

// GetStatus 获取处理器状态
func (p *CrawlerProcessor) GetStatus() map[string]any {
	return map[string]any{
		"name":   "Amazon爬虫处理器",
		"status": "running",
	}
}
