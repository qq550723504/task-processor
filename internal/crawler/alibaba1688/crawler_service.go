// Package alibaba1688 提供1688爬虫应用服务
package alibaba1688

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/shared"
	"task-processor/internal/infra/httpx"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// 编译时检查Service是否实现了CrawlerService接口
var _ httpx.CrawlerService = (*Service)(nil)

// Service 1688爬虫应用服务
type Service struct {
	config        *config.Config
	logger        *logrus.Logger
	processor1688 *Alibaba1688Processor
	workerPool    worker.WorkerPool
	results       sync.Map
	resultMu      sync.Mutex
}

// NewService 创建1688爬虫应用服务
func NewService(cfg *config.Config, logger *logrus.Logger) *Service {
	processor1688 := NewAlibaba1688Processor(cfg)

	service := &Service{
		config:        cfg,
		logger:        logger,
		processor1688: processor1688,
	}

	poolConfig := worker.DefaultPoolConfig()
	poolConfig.Concurrency = 3
	poolConfig.BufferSize = 500
	poolConfig.EnableMetrics = true

	service.workerPool = worker.NewPoolWithConfig(&Crawler1688Processor{service: service}, poolConfig)
	service.workerPool.SetJobHandler(&Crawler1688JobHandler{service: service})

	return service
}

// Start 启动服务
func (s *Service) Start(ctx context.Context) error {
	s.workerPool.Start(ctx)
	s.logger.Info("1688爬虫应用服务已启动")
	return nil
}

// Stop 停止服务
func (s *Service) Stop(ctx context.Context) error {
	s.workerPool.Stop(ctx)
	s.processor1688.Shutdown()
	s.logger.Info("1688爬虫应用服务已停止")
	return nil
}

// SubmitTask 提交任务
func (s *Service) SubmitTask(crawlerTask *shared.CrawlerTask) error {
	if err := crawlerTask.Validate(); err != nil {
		return err
	}

	s.results.Store(crawlerTask.TaskID, shared.NewCrawlerResult(crawlerTask.TaskID))

	taskData, err := json.Marshal(crawlerTask)
	if err != nil {
		return fmt.Errorf("序列化任务失败: %w", err)
	}

	if err := s.workerPool.Submit(worker.WorkerJob{
		TaskID:   crawlerTask.CreatedAt.UnixNano(),
		TaskData: string(taskData),
	}); err != nil {
		return err
	}

	s.logger.Infof("📥 1688任务已提交: %s", crawlerTask.TaskID)
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
		tasks = append(tasks, value.(*shared.CrawlerResult))
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

	statusCount := make(map[string]int)
	s.results.Range(func(key, value any) bool {
		statusCount[string(value.(*shared.CrawlerResult).Status)]++
		return true
	})
	stats["status_count"] = statusCount

	if metrics := s.workerPool.GetMetrics(); metrics != nil {
		snapshot := metrics.GetSnapshot()
		stats["total_submitted"] = snapshot.TotalSubmitted
		stats["total_processed"] = snapshot.TotalProcessed
		stats["total_succeeded"] = snapshot.TotalSucceeded
		stats["total_failed"] = snapshot.TotalFailed
		stats["success_rate"] = snapshot.SuccessRate()
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
