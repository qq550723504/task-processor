// Package amazon 提供爬虫应用服务
package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"task-processor/internal/core/config"
	"task-processor/internal/model"
	"task-processor/internal/crawler/shared"
	"task-processor/internal/infra/httpx"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// 编译时检查Service是否实现了CrawlerService接口
var _ httpx.CrawlerService = (*Service)(nil)

// Service 爬虫应用服务
type Service struct {
	config          *config.Config
	logger          *logrus.Logger
	amazonProcessor *AmazonProcessor
	domainResolver  *DomainResolver
	workerPool      worker.WorkerPool
	results         sync.Map
	resultMu        sync.Mutex
}

// NewService 创建爬虫应用服务
func NewService(cfg *config.Config, logger *logrus.Logger) *Service {
	amazonProcessor := CreateProcessor(cfg, logger)
	domainResolver := NewDomainResolver()

	// 创建服务实例
	service := &Service{
		config:          cfg,
		logger:          logger,
		amazonProcessor: amazonProcessor,
		domainResolver:  domainResolver,
	}

	// 创建 Worker 池配置
	poolConfig := worker.DefaultPoolConfig()
	poolConfig.Concurrency = 5
	poolConfig.BufferSize = 1000
	poolConfig.EnableMetrics = true

	// 创建处理器
	processor := &CrawlerProcessor{service: service}

	// 创建 Worker 池
	service.workerPool = worker.NewPoolWithConfig(processor, poolConfig)

	// 设置任务钩子
	jobHandler := &CrawlerJobHandler{service: service}
	service.workerPool.SetJobHandler(jobHandler)

	return service
}

// Start 启动服务
func (s *Service) Start(ctx context.Context) error {
	s.workerPool.Start(ctx)
	s.logger.Info("爬虫应用服务已启动")
	return nil
}

// Stop 停止服务
func (s *Service) Stop(ctx context.Context) error {
	s.workerPool.Stop(ctx)
	s.logger.Info("爬虫应用服务已停止")
	return nil
}

// SubmitTask 提交任务
func (s *Service) SubmitTask(crawlerTask *shared.CrawlerTask) error {
	// 如果只提供了 ASIN，构造 URL
	if crawlerTask.URL == "" && crawlerTask.ASIN != "" {
		crawlerTask.BuildURLFromASIN(s.domainResolver)
	}

	// 验证任务
	if err := crawlerTask.Validate(); err != nil {
		return err
	}

	// 初始化结果
	result := shared.NewCrawlerResult(crawlerTask.TaskID)
	s.results.Store(crawlerTask.TaskID, result)

	// 序列化任务
	taskData, err := json.Marshal(crawlerTask)
	if err != nil {
		return fmt.Errorf("序列化任务失败: %w", err)
	}

	// 创建 WorkerJob
	job := worker.WorkerJob{
		TaskID:   crawlerTask.CreatedAt.UnixNano(),
		TaskData: string(taskData),
	}

	// 提交到 Worker 池
	if err := s.workerPool.Submit(job); err != nil {
		return err
	}

	s.logger.Infof("📥 任务已提交: %s", crawlerTask.TaskID)
	return nil
}

// GetTask 获取任务结果
func (s *Service) GetTask(taskID string) (*shared.CrawlerResult, error) {
	value, ok := s.results.Load(taskID)
	if !ok {
		return nil, shared.ErrTaskNotFound
	}
	return value.(*shared.CrawlerResult), nil
}

// DeleteTask 删除任务
func (s *Service) DeleteTask(taskID string) {
	s.results.Delete(taskID)
}

// GetAllTasks 获取所有任务
func (s *Service) GetAllTasks() []*shared.CrawlerResult {
	tasks := make([]*shared.CrawlerResult, 0)
	s.results.Range(func(key, value any) bool {
		result := value.(*shared.CrawlerResult)
		tasks = append(tasks, result)
		return true
	})
	return tasks
}

// GetStats 获取统计信息
func (s *Service) GetStats() map[string]any {
	queueStats := s.workerPool.GetQueueStats()

	stats := map[string]any{
		"queue_size":      queueStats.QueueSize,
		"queue_capacity":  queueStats.BufferSize,
		"available_slots": queueStats.AvailableSlots,
		"usage_percent":   queueStats.UsagePercent,
	}

	// 统计各状态任务数
	statusCount := make(map[string]int)
	s.results.Range(func(key, value any) bool {
		result := value.(*shared.CrawlerResult)
		statusCount[string(result.Status)]++
		return true
	})
	stats["status_count"] = statusCount

	// Worker 池指标
	if metrics := s.workerPool.GetMetrics(); metrics != nil {
		snapshot := metrics.GetSnapshot()
		stats["total_submitted"] = snapshot.TotalSubmitted
		stats["total_processed"] = snapshot.TotalProcessed
		stats["total_succeeded"] = snapshot.TotalSucceeded
		stats["total_failed"] = snapshot.TotalFailed
		stats["total_panicked"] = snapshot.TotalPanicked
		stats["queue_full_count"] = snapshot.QueueFullCount
		stats["success_rate"] = snapshot.SuccessRate()
		stats["failure_rate"] = snapshot.FailureRate()
		stats["panic_rate"] = snapshot.PanicRate()
		stats["uptime"] = snapshot.Uptime.String()
	}

	return stats
}

// IsReady 检查服务是否就绪
func (s *Service) IsReady() bool {
	return s.workerPool.AvailableSlots() > 0
}

// IsHealthy 检查服务是否健康
func (s *Service) IsHealthy() bool {
	return true
}

// updateResult 线程安全地更新任务结果
func (s *Service) updateResult(taskID string, updateFn func(*shared.CrawlerResult)) {
	s.resultMu.Lock()
	defer s.resultMu.Unlock()

	if value, ok := s.results.Load(taskID); ok {
		result := value.(*shared.CrawlerResult)
		updateFn(result)
		s.results.Store(taskID, result)
	}
}

// getZipcodeForTask 获取任务的邮编
func (s *Service) getZipcodeForTask(crawlerTask *shared.CrawlerTask) string {
	// 1. 如果任务指定了邮编，直接使用
	if crawlerTask.Zipcode != "" {
		return crawlerTask.Zipcode
	}

	// 2. 如果任务指定了地区，使用地区对应的邮编
	if crawlerTask.Region != "" {
		return s.getZipcodeForRegion(crawlerTask.Region)
	}

	// 3. 尝试从URL中提取地区
	if crawlerTask.URL != "" {
		region := s.domainResolver.ExtractRegionFromURL(crawlerTask.URL)
		if region != "" {
			return s.getZipcodeForRegion(region)
		}
	}

	// 4. 使用默认邮编（美国）
	return s.getZipcodeForRegion("us")
}

// getZipcodeForRegion 获取地区对应的邮编
func (s *Service) getZipcodeForRegion(region string) string {
	region = strings.ToLower(region)

	// 优先从配置获取
	if s.config.Amazon.Zipcodes != nil {
		if zipcode, exists := s.config.Amazon.Zipcodes[region]; exists && zipcode != "" {
			return zipcode
		}
	}

	// 使用域名解析器获取默认邮编
	return s.domainResolver.GetZipcodeByRegion(region)
}

// productToMap 将 Product 转换为 map
func productToMap(product *model.Product, logger *logrus.Logger) map[string]any {
	if product == nil {
		return nil
	}

	data, err := json.Marshal(product)
	if err != nil {
		logger.Errorf("序列化产品数据失败: %v", err)
		return nil
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		logger.Errorf("反序列化产品数据失败: %v", err)
		return nil
	}

	return result
}


