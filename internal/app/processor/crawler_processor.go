// Package processor 提供爬虫任务处理器
package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/app/task"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/worker"
	"task-processor/internal/model"
	"task-processor/internal/pkg/timeout"
	"task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

// CrawlerProcessor Amazon爬虫处理器
type CrawlerProcessor struct {
	amazonProcessor *amazon.AmazonProcessor
	productFetcher  *product.ProductFetcher
	taskSubmitter   VariantTaskSubmitter
	rabbitmqClient  RabbitMQPublisher // 添加 RabbitMQ 客户端
	messageAdapter  *task.MessageAdapter
	logger          *logrus.Logger
}

// RabbitMQPublisher RabbitMQ 发布接口（直接发原始 body，不包装）
type RabbitMQPublisher interface {
	Publish(ctx context.Context, queueName string, body []byte, priority uint8) error
}

// NewCrawlerProcessor 创建爬虫处理器
func NewCrawlerProcessor(
	logger *logrus.Logger,
	amazonProcessor *amazon.AmazonProcessor,
	productFetcher *product.ProductFetcher,
	taskSubmitter VariantTaskSubmitter,
	rabbitmqClient RabbitMQPublisher,
) *CrawlerProcessor {
	return &CrawlerProcessor{
		amazonProcessor: amazonProcessor,
		productFetcher:  productFetcher,
		taskSubmitter:   taskSubmitter,
		rabbitmqClient:  rabbitmqClient,
		messageAdapter:  task.NewMessageAdapter(),
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

	// 解析任务数据（爬虫任务使用原始 payload 格式）
	var messageWrapper map[string]any
	if err := json.Unmarshal([]byte(job.TaskData), &messageWrapper); err != nil {
		return fmt.Errorf("解析任务数据失败: %w", err)
	}

	// 提取 payload（真正的任务数据在 payload 字段中）
	var taskData map[string]any
	if payloadVal, ok := messageWrapper["payload"]; ok {
		if payloadMap, ok := payloadVal.(map[string]any); ok {
			taskData = payloadMap
		} else {
			return fmt.Errorf("payload 字段类型错误")
		}
	} else {
		// 如果没有 payload 字段，说明是直接的任务数据（向后兼容）
		taskData = messageWrapper
	}

	// 提取 reply_to 队列（如果存在）
	var replyTo string
	if replyToVal, ok := taskData["reply_to"]; ok {
		if replyToStr, ok := replyToVal.(string); ok {
			replyTo = replyToStr
		}
	}

	// 提取 taskId 字符串（在 normalizeTaskData 之前，避免 float64 精度丢失）
	// 发送方将 taskId 序列化为 string，直接读取；兼容旧格式（数字）
	var taskIDStr string
	if v, ok := taskData["id"]; ok {
		switch val := v.(type) {
		case string:
			taskIDStr = val
		case float64:
			taskIDStr = fmt.Sprintf("%d", int64(val))
		}
	}

	// 修复：JSON 反序列化后，数字类型可能是 float64，需要转换
	// 规范化数据类型
	p.normalizeTaskData(taskData)

	// 重新序列化并反序列化为 Task 结构
	taskBytes, _ := json.Marshal(taskData)
	var task model.Task
	if err := json.Unmarshal(taskBytes, &task); err != nil {
		return fmt.Errorf("解析任务结构失败: %w", err)
	}

	p.logger.Infof("🔍 开始爬取任务: ID=%d, ProductID=%s, ReplyTo=%s", task.ID, task.ProductID, replyTo)

	startTime := time.Now()

	// 使用 SourcePlatform 作为爬虫平台，去掉 .crawler 后缀
	// 例如: "amazon.crawler" -> "amazon"
	platform := task.SourcePlatform
	if platform == "" {
		// 兼容旧消息格式：从 Platform 字段提取
		platform = task.Platform
	}
	if idx := strings.Index(platform, ".crawler"); idx != -1 {
		platform = platform[:idx]
	}

	// 构建获取请求
	fetchReq := &product.FetchRequest{
		TenantID:   task.TenantID,
		Platform:   platform, // 使用去掉 .crawler 后缀的平台名称
		Region:     task.Region,
		ProductID:  task.ProductID,
		StoreID:    task.StoreID,
		CategoryID: task.CategoryID,
		Creator:    "crawler-consumer",
	}

	// 获取产品数据（会自动使用浏览器池，浏览器实例会被放回池中复用）
	productData, err := p.productFetcher.FetchProduct(ctx, fetchReq)

	// 如果有 reply_to 队列，发送结果
	if replyTo != "" {
		p.sendCrawlResult(replyTo, taskIDStr, productData, err, time.Since(startTime))
	}

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

	// 变体自动提交已禁用

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

// sendCrawlResult 发送爬取结果到回复队列
func (p *CrawlerProcessor) sendCrawlResult(replyTo string, taskID string, product *model.Product, err error, duration time.Duration) {
	// taskId 用 string，避免 JSON float64 精度丢失
	result := map[string]any{
		"taskId":   taskID,
		"success":  err == nil,
		"duration": duration.Nanoseconds(),
		"nodeId":   "crawler-node-1",
	}

	if err != nil {
		result["error"] = err.Error()
	} else if product != nil {
		result["product"] = product
	}

	resultBytes, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		p.logger.Errorf("序列化爬取结果失败: %v", marshalErr)
		return
	}

	if p.rabbitmqClient != nil {
		ctx, cancel := timeout.WithHTTPShortTimeout(context.Background())
		defer cancel()

		if publishErr := p.rabbitmqClient.Publish(ctx, replyTo, resultBytes, 5); publishErr != nil {
			p.logger.Errorf("发送爬取结果失败: %v", publishErr)
			return
		}

		p.logger.Infof("✅ 爬取结果已发送到队列: %s, TaskID=%s, Success=%v", replyTo, taskID, err == nil)
	} else {
		p.logger.Warnf("⚠️ RabbitMQ客户端未设置，无法发送结果")
	}
}

// normalizeTaskData 规范化任务数据类型
// JSON 反序列化后，数字可能是 float64 或 string，需要转换为正确的类型
func (p *CrawlerProcessor) normalizeTaskData(data map[string]any) {
	// 需要转换为 int64 的字段
	int64Fields := []string{"id", "tenantId", "storeId", "categoryId"}
	for _, field := range int64Fields {
		if val, ok := data[field]; ok {
			switch v := val.(type) {
			case float64:
				data[field] = int64(v)
			case string:
				// 尝试解析字符串为数字
				if v != "" {
					var num int64
					fmt.Sscanf(v, "%d", &num)
					data[field] = num
				}
			}
		}
	}

	// 需要转换为 int 的字段
	intFields := []string{"priority", "retryCount", "maxRetryCount"}
	for _, field := range intFields {
		if val, ok := data[field]; ok {
			switch v := val.(type) {
			case float64:
				data[field] = int(v)
			case string:
				if v != "" {
					var num int
					fmt.Sscanf(v, "%d", &num)
					data[field] = num
				}
			}
		}
	}

	// 处理 status 字段：字符串 -> int16
	if val, ok := data["status"]; ok {
		switch v := val.(type) {
		case string:
			// 使用 MessageAdapter 的公共方法转换状态
			data["status"] = p.messageAdapter.ConvertStatusStringToInt16(v)
		case float64:
			data["status"] = int16(v)
		}
	}
}
