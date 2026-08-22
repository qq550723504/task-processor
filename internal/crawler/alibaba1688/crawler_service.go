// Package alibaba1688 提供1688爬虫应用服务
package alibaba1688

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688/model"
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
	config                 *config.Config
	logger                 *logrus.Logger
	processor1688          alibaba1688TaskProcessor
	accountProfileResolver AccountProfileResolver
	profileLocksMu         sync.Mutex
	profileLocks           map[string]*accountProfileLock
	sourceAccessMu         sync.Mutex
	sourceAccessCounts     map[string]int64
}

type accountProfileLock struct {
	token chan struct{}
}

func newAccountProfileLock() *accountProfileLock {
	token := make(chan struct{}, 1)
	token <- struct{}{}
	return &accountProfileLock{token: token}
}

func (l *accountProfileLock) lock(ctx context.Context) (func(), error) {
	if l == nil {
		return nil, fmt.Errorf("account profile lock is nil")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-l.token:
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			l.token <- struct{}{}
		})
	}, nil
}

type alibaba1688TaskProcessor interface {
	Process(context.Context, string) (*model.Product1688, error)
	ProcessWithAccountProfile(context.Context, string, AccountProfile) (*model.Product1688, error)
	Shutdown()
}

var sourceAccessMetricKeys = [...]string{
	"public",
	"account_assisted",
	"source_public_unavailable",
	"source_account_unavailable",
	"source_account_disabled",
}

func newSourceAccessCounts() map[string]int64 {
	counts := make(map[string]int64, len(sourceAccessMetricKeys))
	for _, key := range sourceAccessMetricKeys {
		counts[key] = 0
	}
	return counts
}

// NewService 创建1688爬虫应用服务
func NewService(cfg *config.Config, logger *logrus.Logger, resolvers ...AccountProfileResolver) *Service {
	processor1688 := NewAlibaba1688Processor(cfg)
	var resolver AccountProfileResolver
	if len(resolvers) > 0 {
		resolver = resolvers[0]
	}

	svc := &Service{
		config:                 cfg,
		logger:                 logger,
		processor1688:          processor1688,
		accountProfileResolver: resolver,
		profileLocks:           make(map[string]*accountProfileLock),
		sourceAccessCounts:     newSourceAccessCounts(),
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
		UpdateResultForTask: func(task *shared.CrawlerTask, fn func(*shared.CrawlerResult)) error {
			return svc.UpdateResultForTenant(task.TenantID, task.TaskID, fn)
		},
	})
	svc.SetWorkerPool(pool)
	if err := svc.ConfigureRedisResultStore(cfg.Redis, logger, "crawler:1688:task-result", 6*time.Hour); err != nil {
		logger.Warnf("初始化 1688 crawler 异步任务共享结果存储失败，将退化为 Pod 本地内存: %v", err)
	}

	return svc
}

func (s *Service) recordSourceAccess(key string) {
	if s == nil || key == "" {
		return
	}
	s.sourceAccessMu.Lock()
	defer s.sourceAccessMu.Unlock()
	if s.sourceAccessCounts == nil {
		s.sourceAccessCounts = newSourceAccessCounts()
	}
	s.sourceAccessCounts[key]++
}

func (s *Service) sourceAccessStats() map[string]int64 {
	if s == nil {
		return nil
	}
	s.sourceAccessMu.Lock()
	defer s.sourceAccessMu.Unlock()
	stats := make(map[string]int64, len(s.sourceAccessCounts))
	for key, value := range s.sourceAccessCounts {
		stats[key] = value
	}
	return stats
}

func (s *Service) GetStats() map[string]any {
	stats := s.BaseService.GetStats()
	stats["source_access_total"] = s.sourceAccessStats()
	return stats
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

	result := shared.NewCrawlerResult(crawlerTask.TaskID)
	result.TenantID = crawlerTask.TenantID
	if err := s.StoreResult(crawlerTask.TaskID, result); err != nil {
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
