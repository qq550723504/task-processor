// Package alibaba1688 提供1688爬虫应用服务
package alibaba1688

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/shared"
	"task-processor/internal/infra/httpx"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// 编译时检查 Service 是否实现了 CrawlerService 接口
var _ httpx.CrawlerService = (*Service)(nil)

// Service 1688爬虫应用服务
type Service struct {
	shared.BaseService
	config        *config.Config
	logger        *logrus.Logger
	processor1688 *Alibaba1688Processor
}

// NewService 创建1688爬虫应用服务
func NewService(cfg *config.Config, logger *logrus.Logger) *Service {
	processor1688 := NewAlibaba1688Processor(cfg)

	svc := &Service{
		config:        cfg,
		logger:        logger,
		processor1688: processor1688,
	}

	poolConfig := worker.DefaultPoolConfig()
	poolConfig.Concurrency = 3
	poolConfig.BufferSize = 500
	poolConfig.EnableMetrics = true

	pool := worker.NewPoolWithConfig(&Crawler1688Processor{service: svc}, poolConfig)
	pool.SetJobHandler(&shared.BaseJobHandler{
		Name:         "1688",
		Logger:       logger,
		UpdateResult: svc.UpdateResult,
	})
	svc.SetWorkerPool(pool)
	if err := svc.ConfigureRedisResultStore(cfg.Redis, logger, "crawler:1688:task-result", 6*time.Hour); err != nil {
		logger.Warnf("初始化 1688 crawler 异步任务共享结果存储失败，将退化为 Pod 本地内存: %v", err)
	}

	return svc
}

// Start 启动服务
func (s *Service) Start(ctx context.Context) error {
	s.WorkerPool().Start(ctx)
	s.logger.Info("1688爬虫应用服务已启动")
	return nil
}

// Stop 停止服务
func (s *Service) Stop(ctx context.Context) error {
	s.WorkerPool().Stop(ctx)
	s.processor1688.Shutdown()
	if err := s.BaseService.Close(); err != nil {
		s.logger.Warnf("关闭 1688 crawler 结果 Redis 客户端失败: %v", err)
	}
	s.logger.Info("1688爬虫应用服务已停止")
	return nil
}

// SubmitTask 提交任务
func (s *Service) SubmitTask(crawlerTask *shared.CrawlerTask) error {
	if err := crawlerTask.Validate(); err != nil {
		return err
	}

	if err := s.StoreResult(crawlerTask.TaskID, shared.NewCrawlerResult(crawlerTask.TaskID)); err != nil {
		return fmt.Errorf("persist crawler task result: %w", err)
	}

	taskData, err := json.Marshal(crawlerTask)
	if err != nil {
		return fmt.Errorf("序列化任务失败: %w", err)
	}

	if err := s.WorkerPool().Submit(worker.WorkerJob{
		TaskID:   crawlerTask.CreatedAt.UnixNano(),
		TaskData: string(taskData),
	}); err != nil {
		return err
	}

	s.logger.Infof("📥 1688任务已提交: %s", crawlerTask.TaskID)
	return nil
}
