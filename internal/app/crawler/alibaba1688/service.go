// Package alibaba1688 提供1688爬虫应用服务
package alibaba1688

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688"
	domainService "task-processor/internal/domain/service"
	"task-processor/internal/domain/task"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// 编译时检查Service是否实现了CrawlerService接口
var _ domainService.CrawlerService = (*Service)(nil)

// Service 1688爬虫应用服务
type Service struct {
	config        *config.Config
	logger        *logrus.Logger
	processor1688 *alibaba1688.Alibaba1688Processor
	workerPool    worker.WorkerPool
	results       sync.Map   // task_id -> *task.CrawlerResult
	resultMu      sync.Mutex // 保护结果的修改
}

// NewService 创建1688爬虫应用服务
func NewService(cfg *config.Config, logger *logrus.Logger) *Service {
	// 创建 1688 处理器
	processor1688 := alibaba1688.NewAlibaba1688Processor(cfg)

	// 创建服务实例
	service := &Service{
		config:        cfg,
		logger:        logger,
		processor1688: processor1688,
	}

	// 创建 Worker 池配置
	poolConfig := worker.DefaultPoolConfig()
	poolConfig.Concurrency = 3 // 1688爬虫并发数较低
	poolConfig.BufferSize = 500
	poolConfig.EnableMetrics = true

	// 创建处理器
	processor := &Crawler1688Processor{service: service}

	// 创建 Worker 池
	service.workerPool = worker.NewPoolWithConfig(processor, poolConfig)

	// 设置任务钩子
	jobHandler := &Crawler1688JobHandler{service: service}
	service.workerPool.SetJobHandler(jobHandler)

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
func (s *Service) SubmitTask(crawlerTask *task.CrawlerTask) error {
	// 验证任务
	if err := crawlerTask.Validate(); err != nil {
		return err
	}

	// 初始化结果
	result := task.NewCrawlerResult(crawlerTask.TaskID)
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

	s.logger.Infof("📥 1688任务已提交: %s", crawlerTask.TaskID)
	return nil
}

// GetTask 获取任务结果
func (s *Service) GetTask(taskID string) (*task.CrawlerResult, error) {
	value, ok := s.results.Load(taskID)
	if !ok {
		return nil, task.ErrTaskNotFound
	}
	return value.(*task.CrawlerResult), nil
}

// DeleteTask 删除任务
func (s *Service) DeleteTask(taskID string) {
	s.results.Delete(taskID)
}

// GetAllTasks 获取所有任务
func (s *Service) GetAllTasks() []*task.CrawlerResult {
	tasks := make([]*task.CrawlerResult, 0)
	s.results.Range(func(key, value interface{}) bool {
		result := value.(*task.CrawlerResult)
		tasks = append(tasks, result)
		return true
	})
	return tasks
}

// GetStats 获取统计信息
func (s *Service) GetStats() map[string]interface{} {
	queueStats := s.workerPool.GetQueueStats()

	stats := map[string]interface{}{
		"queue_size":      queueStats.QueueSize,
		"queue_capacity":  queueStats.BufferSize,
		"available_slots": queueStats.AvailableSlots,
		"usage_percent":   queueStats.UsagePercent,
	}

	// 统计各状态任务数
	statusCount := make(map[string]int)
	s.results.Range(func(key, value interface{}) bool {
		result := value.(*task.CrawlerResult)
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
func (s *Service) updateResult(taskID string, updateFn func(*task.CrawlerResult)) {
	s.resultMu.Lock()
	defer s.resultMu.Unlock()

	if value, ok := s.results.Load(taskID); ok {
		result := value.(*task.CrawlerResult)
		updateFn(result)
		s.results.Store(taskID, result)
	}
}
