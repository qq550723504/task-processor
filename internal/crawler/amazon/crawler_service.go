// Package amazon 提供爬虫应用服务
package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/shared"
	"task-processor/internal/infra/httpx"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// 编译时检查 Service 是否实现了 CrawlerService 接口
var _ httpx.CrawlerService = (*Service)(nil)

// Service 爬虫应用服务
type Service struct {
	shared.BaseService
	config          *config.Config
	logger          *logrus.Logger
	amazonProcessor *AmazonProcessor
	domainResolver  *DomainResolver
}

// NewService 创建爬虫应用服务
func NewService(cfg *config.Config, logger *logrus.Logger) *Service {
	amazonProcessor := CreateProcessor(cfg, logger)
	domainResolver := NewDomainResolver()

	svc := &Service{
		config:          cfg,
		logger:          logger,
		amazonProcessor: amazonProcessor,
		domainResolver:  domainResolver,
	}

	poolConfig := worker.DefaultPoolConfig()
	poolConfig.Concurrency = 5
	poolConfig.BufferSize = 1000
	poolConfig.EnableMetrics = true

	processor := &CrawlerProcessor{service: svc}
	pool := worker.NewPoolWithConfig(processor, poolConfig)
	pool.SetJobHandler(&shared.BaseJobHandler{
		Name:         "Amazon",
		Logger:       logger,
		UpdateResult: svc.UpdateResult,
	})
	svc.SetWorkerPool(pool)

	return svc
}

// Start 启动服务
func (s *Service) Start(ctx context.Context) error {
	s.WorkerPool().Start(ctx)
	s.logger.Info("爬虫应用服务已启动")
	return nil
}

// Stop 停止服务
func (s *Service) Stop(ctx context.Context) error {
	s.WorkerPool().Stop(ctx)
	s.logger.Info("爬虫应用服务已停止")
	return nil
}

// SubmitTask 提交任务
func (s *Service) SubmitTask(crawlerTask *shared.CrawlerTask) error {
	// 如果只提供了 ASIN，构造 URL
	if crawlerTask.URL == "" && crawlerTask.ASIN != "" {
		crawlerTask.BuildURLFromASIN(s.domainResolver)
	}

	if err := crawlerTask.Validate(); err != nil {
		return err
	}

	s.StoreResult(crawlerTask.TaskID, shared.NewCrawlerResult(crawlerTask.TaskID))

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

	s.logger.Infof("📥 任务已提交: %s", crawlerTask.TaskID)
	return nil
}

// getZipcodeForTask 获取任务的邮编
func (s *Service) getZipcodeForTask(crawlerTask *shared.CrawlerTask) string {
	if crawlerTask.Zipcode != "" {
		return crawlerTask.Zipcode
	}
	if crawlerTask.Region != "" {
		return s.getZipcodeForRegion(crawlerTask.Region)
	}
	if crawlerTask.URL != "" {
		if region := s.domainResolver.ExtractRegionFromURL(crawlerTask.URL); region != "" {
			return s.getZipcodeForRegion(region)
		}
	}
	return s.getZipcodeForRegion("us")
}

// getZipcodeForRegion 获取地区对应的邮编
func (s *Service) getZipcodeForRegion(region string) string {
	region = strings.ToLower(region)
	if s.config.Amazon.Zipcodes != nil {
		if zipcode, exists := s.config.Amazon.Zipcodes[region]; exists && zipcode != "" {
			return zipcode
		}
	}
	return s.domainResolver.GetZipcodeByRegion(region)
}
