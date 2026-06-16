package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/app/task"
	"task-processor/internal/infra/worker"
	"task-processor/internal/model"
	"task-processor/internal/pkg/timeout"
	"task-processor/internal/product"
	"task-processor/internal/product/sourcing"

	"github.com/sirupsen/logrus"
)

// CrawlerProcessor Amazon爬虫处理器
type CrawlerProcessor struct {
	productFetcher *product.ProductFetcher
	taskSubmitter  VariantTaskSubmitter
	rabbitmqClient RabbitMQPublisher
	messageAdapter *task.MessageAdapter
	logger         *logrus.Logger
}

// RabbitMQPublisher RabbitMQ 发布接口（直接发原始 body，不包装）
type RabbitMQPublisher interface {
	Publish(ctx context.Context, queueName string, body []byte, priority uint8) error
}

// NewCrawlerProcessor 创建爬虫处理器
func NewCrawlerProcessor(
	logger *logrus.Logger,
	productFetcher *product.ProductFetcher,
	taskSubmitter VariantTaskSubmitter,
	rabbitmqClient RabbitMQPublisher,
) *CrawlerProcessor {
	return &CrawlerProcessor{
		productFetcher: productFetcher,
		taskSubmitter:  taskSubmitter,
		rabbitmqClient: rabbitmqClient,
		messageAdapter: task.NewMessageAdapter(),
		logger:         logger,
	}
}

func (p *CrawlerProcessor) Start(ctx context.Context) error {
	p.logger.Info("🌐 Amazon爬虫处理器启动完成")
	return nil
}

// ProcessTask 处理任务 - 实现worker.Processor接口
func (p *CrawlerProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	var messageWrapper map[string]any
	if err := json.Unmarshal([]byte(job.TaskData), &messageWrapper); err != nil {
		return fmt.Errorf("解析任务数据失败: %w", err)
	}

	var taskData map[string]any
	if payloadVal, ok := messageWrapper["payload"]; ok {
		if payloadMap, ok := payloadVal.(map[string]any); ok {
			taskData = payloadMap
		} else {
			return fmt.Errorf("payload 字段类型错误")
		}
	} else {
		taskData = messageWrapper
	}

	var replyTo string
	if replyToVal, ok := taskData["reply_to"]; ok {
		if replyToStr, ok := replyToVal.(string); ok {
			replyTo = replyToStr
		}
	}

	var taskIDStr string
	if v, ok := taskData["id"]; ok {
		switch val := v.(type) {
		case string:
			taskIDStr = val
		case float64:
			taskIDStr = fmt.Sprintf("%d", int64(val))
		}
	}

	p.normalizeTaskData(taskData)

	taskBytes, _ := json.Marshal(taskData)
	var task model.Task
	if err := json.Unmarshal(taskBytes, &task); err != nil {
		return fmt.Errorf("解析任务结构失败: %w", err)
	}

	p.logger.Infof("🔍 开始爬取任务: ID=%d, ProductID=%s, ReplyTo=%s", task.ID, task.ProductID, replyTo)

	startTime := time.Now()
	platform := p.crawlerPlatformFromTask(task)

	fetchReq := p.fetchRequestFromTask(task, platform)

	productData, err := p.productFetcher.FetchProduct(ctx, fetchReq)
	if err == nil && productData == nil {
		err = fmt.Errorf("爬取产品数据为空")
	}

	if replyTo != "" {
		p.sendCrawlResult(replyTo, taskIDStr, productData, err, time.Since(startTime))
	}

	if err != nil {
		p.logger.Errorf("❌ 爬取失败: ID=%d, ProductID=%s, Error=%v", task.ID, task.ProductID, err)
		return fmt.Errorf("爬取产品数据失败: %w", err)
	}

	p.logger.Infof("📦 产品ASIN: %s", productData.Asin)
	p.logger.Infof("💰 产品价格: %.2f %s", productData.FinalPrice, productData.Currency)

	if err := p.productFetcher.CacheProduct(fetchReq, productData); err != nil {
		p.logger.Warnf("⚠️ 保存产品数据到服务器失败: %v", err)
	}

	duration := time.Since(startTime)
	p.logger.Infof("✅ 爬取完成: ID=%d, ProductID=%s, 耗时=%v", task.ID, task.ProductID, duration)

	return nil
}

func (p *CrawlerProcessor) Close(ctx context.Context) {
	p.logger.Info("🔒 关闭Amazon爬虫处理器")
}

func (p *CrawlerProcessor) GetStatus() map[string]any {
	return map[string]any{
		"name":   "Amazon爬虫处理器",
		"status": "running",
	}
}

func (p *CrawlerProcessor) fetchRequestFromTask(task model.Task, platform string) *product.FetchRequest {
	return &product.FetchRequest{
		TenantID:   task.TenantID,
		Platform:   platform,
		Region:     task.Region,
		ProductID:  task.ProductID,
		Zipcode:    task.Zipcode,
		StoreID:    task.StoreID,
		CategoryID: task.CategoryID,
		Creator:    "crawler-consumer",
	}
}

func (p *CrawlerProcessor) crawlerPlatformFromTask(task model.Task) string {
	platform := strings.TrimSpace(task.SourcePlatform)
	if platform == "" {
		platform = strings.TrimSpace(task.Platform)
	}
	if idx := strings.Index(platform, ".crawler"); idx != -1 {
		platform = platform[:idx]
	}
	return sourcing.CrawlerPlatformForSource(platform)
}

func (p *CrawlerProcessor) sendCrawlResult(replyTo string, taskID string, product *model.Product, err error, duration time.Duration) {
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

func (p *CrawlerProcessor) normalizeTaskData(data map[string]any) {
	int64Fields := []string{"id", "tenantId", "storeId", "categoryId"}
	for _, field := range int64Fields {
		if val, ok := data[field]; ok {
			switch v := val.(type) {
			case float64:
				data[field] = int64(v)
			case string:
				if v != "" {
					var num int64
					fmt.Sscanf(v, "%d", &num)
					data[field] = num
				}
			}
		}
	}

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

	if val, ok := data["status"]; ok {
		switch v := val.(type) {
		case string:
			data["status"] = p.messageAdapter.ConvertStatusStringToInt16(v)
		case float64:
			data["status"] = int16(v)
		}
	}
}
