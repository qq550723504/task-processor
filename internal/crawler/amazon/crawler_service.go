// Package amazon 提供爬虫应用服务
package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/shared"
	"task-processor/internal/infra/httpx"
	"task-processor/internal/infra/redisclient"
	"task-processor/internal/infra/worker"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// 编译时检查 Service 是否实现了 CrawlerService 接口
var _ httpx.CrawlerService = (*Service)(nil)

type productDedupeStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Delete(ctx context.Context, key string) error
}

// Service 爬虫应用服务
type Service struct {
	shared.BaseService
	config          *config.Config
	logger          *logrus.Logger
	amazonProcessor *AmazonProcessor
	domainResolver  *DomainResolver
	dedupeStore     productDedupeStore
	processProduct  func(ctx context.Context, url, zipcode string) (*model.Product, error)
	metrics         *serviceMetrics
	regionGuard     *regionGuard
}

// NewService 创建爬虫应用服务
func NewService(cfg *config.Config, logger *logrus.Logger) *Service {
	amazonProcessor := CreateProcessor(cfg, logger)
	domainResolver := NewDomainResolver()
	dedupeStore := buildProductDedupeStore(cfg, logger)

	svc := &Service{
		config:          cfg,
		logger:          logger,
		amazonProcessor: amazonProcessor,
		domainResolver:  domainResolver,
		dedupeStore:     dedupeStore,
		processProduct:  amazonProcessor.ProcessWithContext,
		metrics:         newServiceMetrics(),
		regionGuard:     newRegionGuard(cfg.Amazon.RegionGuard),
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
	if closer, ok := s.dedupeStore.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			s.logger.Warnf("关闭产品去重 Redis 客户端失败: %v", err)
		}
	}
	s.logger.Info("爬虫应用服务已停止")
	return nil
}

// FetchProduct 直接抓取单个商品，供同步 API 使用。
func (s *Service) FetchProduct(ctx context.Context, url, asin, region, zipcode string) (*model.Product, string, error) {
	url, zipcode, err := s.resolveFetchInputs(url, asin, region, zipcode)
	if err != nil {
		classified := ClassifyFetchError(err)
		s.metrics.RecordFailure("sync_api", s.resolveMetricsRegion(region, url), classified)
		return nil, "", classified
	}
	metricsRegion := s.resolveMetricsRegion(region, url)

	if s.dedupeStore == nil {
		if err := s.checkRegionGuard(metricsRegion); err != nil {
			s.metrics.RecordFailure("sync_api", metricsRegion, err)
			return nil, url, err
		}
		product, err := s.fetchProductDirect(ctx, url, zipcode)
		if err != nil {
			classified := ClassifyFetchError(err)
			s.recordRegionGuardFailure(metricsRegion, classified)
			s.metrics.RecordFailure("sync_api", metricsRegion, classified)
			return nil, url, classified
		}
		s.recordRegionGuardSuccess(metricsRegion)
		s.metrics.RecordSuccess("sync_api", metricsRegion)
		return product, url, nil
	}

	product, err := s.fetchProductWithDedupe(ctx, url, asin, region, zipcode)
	if err != nil {
		classified := ClassifyFetchError(err)
		s.recordRegionGuardFailure(metricsRegion, classified)
		s.metrics.RecordFailure("sync_api", metricsRegion, classified)
		return nil, url, classified
	}
	s.recordRegionGuardSuccess(metricsRegion)
	s.metrics.RecordSuccess("sync_api", metricsRegion)
	return product, url, nil
}

func (s *Service) GetStats() map[string]any {
	stats := s.BaseService.GetStats()
	if serviceStats := s.metrics.Snapshot(); serviceStats != nil {
		for key, value := range serviceStats {
			stats[key] = value
		}
	}
	if s.amazonProcessor != nil {
		if qualityStats := s.amazonProcessor.QualityStats(); qualityStats != nil {
			for key, value := range qualityStats {
				stats[key] = value
			}
		}
	}
	if s.regionGuard != nil {
		stats["region_guard_open_state_by_region"] = s.regionGuard.Snapshot()
	}
	return stats
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

func buildProductDedupeStore(cfg *config.Config, logger *logrus.Logger) productDedupeStore {
	if cfg == nil || cfg.Redis == nil {
		return nil
	}

	client, err := redisclient.New(cfg.Redis)
	if err != nil {
		logger.Warnf("初始化产品去重 Redis 失败，将退化为无去重模式: %v", err)
		return nil
	}

	logger.Info("已启用 Amazon crawler API 产品级去重")
	return client
}

func (s *Service) resolveFetchInputs(url, asin, region, zipcode string) (string, string, error) {
	if s.processProduct == nil {
		return "", "", fmt.Errorf("Amazon crawler is not initialized")
	}

	if url == "" && asin != "" {
		url = s.domainResolver.BuildAmazonProductURL(region, asin)
	}
	if url == "" {
		return "", "", fmt.Errorf("url or asin is required")
	}

	if zipcode == "" {
		zipcode = s.getZipcodeForTask(&shared.CrawlerTask{
			URL:     url,
			ASIN:    asin,
			Region:  region,
			Zipcode: zipcode,
		})
	}

	return url, zipcode, nil
}

func (s *Service) fetchProductDirect(ctx context.Context, url, zipcode string) (*model.Product, error) {
	return s.processProduct(ctx, url, zipcode)
}

func (s *Service) fetchProductWithDedupe(ctx context.Context, url, asin, region, zipcode string) (*model.Product, error) {
	metricsRegion := s.resolveMetricsRegion(region, url)
	if err := s.checkRegionGuard(metricsRegion); err != nil {
		return nil, err
	}

	lockKey, resultKey := s.buildProductDedupeKeys(url, asin, region)
	waitUntil := time.Now().Add(s.productFetchWaitTimeout())
	waitTicker := time.NewTicker(s.productFetchPollInterval())
	defer waitTicker.Stop()

	for {
		if product, ok, err := s.loadSharedProduct(ctx, resultKey); err == nil && ok {
			s.logger.Infof("♻️ 复用共享抓取结果: %s", resultKey)
			s.metrics.RecordDedupeSharedHit(s.resolveMetricsRegion(region, url))
			return product, nil
		}

		acquired, err := s.dedupeStore.SetNX(ctx, lockKey, "1", s.productFetchLockTTL())
		if err != nil {
			s.logger.Warnf("产品去重锁获取失败，改为直接抓取: %v", err)
			return s.fetchProductDirect(ctx, url, zipcode)
		}

		if acquired {
			defer func() {
				if err := s.dedupeStore.Delete(context.Background(), lockKey); err != nil {
					s.logger.Warnf("释放产品去重锁失败: %v", err)
				}
			}()

			product, err := s.fetchProductDirect(ctx, url, zipcode)
			if err != nil {
				return nil, err
			}
			if err := s.storeSharedProduct(ctx, resultKey, product); err != nil {
				s.logger.Warnf("保存共享抓取结果失败: %v", err)
			}
			return product, nil
		}

		if time.Now().After(waitUntil) {
			return nil, fmt.Errorf("crawl already in progress and shared result timed out")
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-waitTicker.C:
		}
	}
}

func (s *Service) checkRegionGuard(region string) error {
	if s.regionGuard == nil {
		return nil
	}
	if openUntil, blocked := s.regionGuard.Check(region); blocked {
		s.metrics.RecordRegionGuardBlocked(region)
		return newRegionCircuitOpenError(region, openUntil)
	}
	return nil
}

func (s *Service) recordRegionGuardFailure(region string, err error) {
	if s.regionGuard == nil {
		return
	}
	if _, opened := s.regionGuard.RecordFailure(region, err); opened {
		s.metrics.RecordRegionGuardOpen(region)
	}
}

func (s *Service) recordRegionGuardSuccess(region string) {
	if s.regionGuard == nil {
		return
	}
	s.regionGuard.RecordSuccess(region)
}

func (s *Service) resolveMetricsRegion(region, url string) string {
	normalizedRegion := strings.TrimSpace(strings.ToLower(region))
	if normalizedRegion != "" {
		return normalizedRegion
	}
	if s.domainResolver != nil && url != "" {
		if derived := s.domainResolver.ExtractRegionFromURL(url); derived != "" {
			return strings.ToLower(derived)
		}
	}
	return "unknown"
}

func (s *Service) buildProductDedupeKeys(url, asin, region string) (string, string) {
	identity := strings.TrimSpace(strings.ToLower(asin))
	if identity == "" {
		h := fnv.New64a()
		_, _ = h.Write([]byte(strings.TrimSpace(strings.ToLower(url))))
		identity = fmt.Sprintf("url-%x", h.Sum64())
	}
	region = strings.TrimSpace(strings.ToLower(region))
	if region == "" {
		region = "default"
	}

	base := fmt.Sprintf("crawler:amazon:product:%s:%s", region, identity)
	return base + ":lock", base + ":result"
}

func (s *Service) loadSharedProduct(ctx context.Context, resultKey string) (*model.Product, bool, error) {
	payload, err := s.dedupeStore.Get(ctx, resultKey)
	if err != nil {
		return nil, false, err
	}

	var product model.Product
	if err := json.Unmarshal([]byte(payload), &product); err != nil {
		return nil, false, err
	}
	return &product, true, nil
}

func (s *Service) storeSharedProduct(ctx context.Context, resultKey string, product *model.Product) error {
	if product == nil {
		return nil
	}
	payload, err := json.Marshal(product)
	if err != nil {
		return err
	}
	return s.dedupeStore.Set(ctx, resultKey, string(payload), s.productFetchResultTTL())
}

func (s *Service) productFetchLockTTL() time.Duration {
	seconds := s.config.Amazon.ProductDedupe.LockTTLSeconds
	if seconds <= 0 {
		seconds = s.config.Amazon.CrawlTimeout
	}
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

func (s *Service) productFetchResultTTL() time.Duration {
	seconds := s.config.Amazon.ProductDedupe.ResultTTLSeconds
	if seconds <= 0 {
		seconds = 600
	}
	return time.Duration(seconds) * time.Second
}

func (s *Service) productFetchWaitTimeout() time.Duration {
	seconds := s.config.Amazon.ProductDedupe.WaitTimeoutSeconds
	if seconds <= 0 {
		seconds = 120
	}
	return time.Duration(seconds) * time.Second
}

func (s *Service) productFetchPollInterval() time.Duration {
	millis := s.config.Amazon.ProductDedupe.PollIntervalMillis
	if millis <= 0 {
		millis = 500
	}
	return time.Duration(millis) * time.Millisecond
}
