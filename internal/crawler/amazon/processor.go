// Package amazon 提供Amazon处理器核心功能
package amazon

import (
	"context"
	"fmt"
	"sync"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/amazon/browser"
	"task-processor/internal/model"
	"time"
)

// AmazonProcessor Amazon爬虫处理器
type AmazonProcessor struct {
	browserPool          *browser.BrowserPool
	poolManager          *browser.PoolManager
	config               *config.Config
	batchProcessor       *BatchProcessor
	urlHelper            *URLHelper
	productChecker       *ProductChecker
	resultValidator      *ProductResultValidator
	failureArtifactStore *FailureArtifactStore
	timeoutManager       *TimeoutManager
	qualityMetrics       *qualityMetrics
	initErr              error
	shutdownOnce         sync.Once    // 确保只关闭一次
	closed               bool         // 标记是否已关闭
	mu                   sync.RWMutex // 保护 closed 字段
}

const defaultAmazonBrowserPoolSize = 3

func effectiveBrowserPoolSize(cfg *config.Config) int {
	if cfg != nil && cfg.Browser.PoolSize > 0 {
		return cfg.Browser.PoolSize
	}
	return defaultAmazonBrowserPoolSize
}

// NewAmazonProcessor 使用全局配置创建Amazon处理器
func NewAmazonProcessor(cfg *config.Config) *AmazonProcessor {
	// 创建浏览器池配置
	poolConfig := browser.DefaultBrowserPoolConfig()
	poolConfig.Size = effectiveBrowserPoolSize(cfg)

	// 应用随机配置设置
	if cfg != nil && cfg.Browser.RandomConfig.Enabled {
		poolConfig.UseRandomFingerprint = true
		poolConfig.FingerprintStrategy = cfg.Browser.RandomConfig.FingerprintStrategy
		poolConfig.PresetName = cfg.Browser.RandomConfig.PresetName
		poolConfig.HealthCheckEnabled = cfg.Browser.RandomConfig.HealthCheckEnabled
		poolConfig.MaxRetries = cfg.Browser.RandomConfig.MaxRetries
		poolConfig.MaxInstanceUses = cfg.Browser.RandomConfig.MaxUsesPerInstance

		logger.GetGlobalLogger("crawler/amazon").Infof("启用随机配置 - 策略: %s, 预设: %s, 指纹策略: %s",
			cfg.Browser.RandomConfig.Strategy, cfg.Browser.RandomConfig.PresetName, cfg.Browser.RandomConfig.FingerprintStrategy)
	} else {
		poolConfig.UseRandomFingerprint = false
		logger.GetGlobalLogger("crawler/amazon").Info("使用传统浏览器配置")
	}

	configuredSize := 0
	if cfg != nil {
		configuredSize = cfg.Browser.PoolSize
	}
	logger.GetGlobalLogger("crawler/amazon").Infof("创建Amazon处理器，浏览器池大小: %d (配置值: %d)", poolConfig.Size, configuredSize)
	browserPool := browser.NewBrowserPool(cfg, poolConfig)

	// 创建辅助组件（需要在浏览器池初始化前创建，因为池管理器需要它们）
	urlHelper := NewURLHelper()
	productChecker := NewProductChecker()

	var poolManager *browser.PoolManager
	var initErr error
	if err := browserPool.Initialize(); err != nil {
		logger.GetGlobalLogger("crawler/amazon").Errorf("初始化浏览器池失败: %v", err)
		initErr = newProcessorUnavailableError(fmt.Sprintf("初始化浏览器池失败: %v", err), err)
	} else {
		logger.GetGlobalLogger("crawler/amazon").Info("浏览器池初始化成功")
		poolManager = browser.NewPoolManager(browserPool)
	}

	batchProcessor := NewBatchProcessor()
	timeoutManager := NewTimeoutManager(5 * time.Minute) // 默认5分钟超时
	failureArtifactStore := NewFailureArtifactStore(cfg)
	qualityMetrics := newQualityMetrics()

	return &AmazonProcessor{
		browserPool:          browserPool,
		poolManager:          poolManager,
		config:               cfg,
		batchProcessor:       batchProcessor,
		urlHelper:            urlHelper,
		productChecker:       productChecker,
		resultValidator:      NewProductResultValidator(),
		failureArtifactStore: failureArtifactStore,
		timeoutManager:       timeoutManager,
		qualityMetrics:       qualityMetrics,
		initErr:              initErr,
		closed:               false,
	}
}

// Process 处理Amazon产品页面
func (ap *AmazonProcessor) Process(url string, zipcode string) (*model.Product, error) {
	return ap.ProcessWithContext(context.Background(), url, zipcode)
}

// ProcessWithContext 处理Amazon产品页面（支持 context 传递）
func (ap *AmazonProcessor) ProcessWithContext(ctx context.Context, url string, zipcode string) (*model.Product, error) {
	// 检查处理器是否已关闭
	ap.mu.RLock()
	if ap.closed {
		ap.mu.RUnlock()
		return nil, fmt.Errorf("Amazon处理器已关闭")
	}
	ap.mu.RUnlock()

	if ap.initErr != nil {
		return nil, ap.initErr
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("开始处理Amazon产品: %s", url)

	if ap.poolManager == nil {
		return nil, newProcessorUnavailableError("Amazon处理器不可用: 浏览器池管理器未初始化", nil)
	}
	return ap.processWithPoolManager(ctx, url, zipcode)
}

// processWithPoolManager 使用池管理器处理
func (ap *AmazonProcessor) processWithPoolManager(ctx context.Context, url string, zipcode string) (*model.Product, error) {
	timeout := 3 * time.Minute // 单个产品处理超时3分钟

	// 创建实例处理器
	processor := NewInstanceProcessor(ap.urlHelper, ap.productChecker, ap.resultValidator)
	processor.SetFailureArtifactStore(ap.failureArtifactStore)
	processor.SetQualityControlOptions(ap.config.Amazon.QualityControl.RetryOnValidationFailure, ap.config.Amazon.QualityControl.ValidationRetryMaxAttempts)
	processor.SetQualityMetricsRecorder(ap.qualityMetrics)

	return ap.poolManager.ProcessWithTimeout(ctx, url, zipcode, timeout, processor)
}

func (ap *AmazonProcessor) QualityStats() map[string]any {
	if ap == nil || ap.qualityMetrics == nil {
		return nil
	}
	return ap.qualityMetrics.Snapshot()
}

func (ap *AmazonProcessor) ProxyStats() map[string]any {
	if ap == nil || ap.browserPool == nil {
		return nil
	}
	return ap.browserPool.ProxyStats()
}

func (ap *AmazonProcessor) PoolStats() map[string]any {
	stats := map[string]any{}
	if ap != nil && ap.browserPool != nil {
		for key, value := range ap.browserPool.PoolStats() {
			stats[key] = value
		}
	}
	if ap != nil && ap.initErr != nil {
		stats["browser_pool_init_error"] = ap.initErr.Error()
	}
	if len(stats) == 0 {
		return nil
	}
	return stats
}

// ProcessBatch 批量处理多个Amazon产品页面
func (ap *AmazonProcessor) ProcessBatch(requests []model.ProductRequest) []model.ProductResult {
	return ap.ProcessBatchWithContext(context.Background(), requests)
}

// ProcessBatchWithContext 批量处理多个Amazon产品页面，并传递调用方 context。
func (ap *AmazonProcessor) ProcessBatchWithContext(ctx context.Context, requests []model.ProductRequest) []model.ProductResult {
	// 检查处理器是否已关闭
	ap.mu.RLock()
	if ap.closed {
		ap.mu.RUnlock()
		// 返回所有请求的错误结果
		results := make([]model.ProductResult, len(requests))
		for i := range requests {
			results[i] = model.ProductResult{
				Product: nil,
				Error:   fmt.Errorf("Amazon处理器已关闭"),
			}
		}
		return results
	}
	ap.mu.RUnlock()

	if len(requests) == 0 {
		return []model.ProductResult{}
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("开始批量处理 %d 个Amazon产品", len(requests))
	startTime := time.Now()

	if ap.initErr != nil {
		results := make([]model.ProductResult, len(requests))
		for i := range requests {
			results[i] = model.ProductResult{Error: ap.initErr}
		}
		return results
	}
	if ap.browserPool == nil {
		results := make([]model.ProductResult, len(requests))
		for i := range requests {
			results[i] = model.ProductResult{Error: newProcessorUnavailableError("Amazon处理器不可用: 浏览器池未初始化", nil)}
		}
		return results
	}

	results := ap.batchProcessor.ProcessWithContext(ctx, requests, ap)

	duration := time.Since(startTime)
	successCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
		}
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("批量处理完成: 成功 %d/%d, 耗时: %v", successCount, len(requests), duration)
	return results
}

// Shutdown 关闭处理器
func (ap *AmazonProcessor) Shutdown() {
	ap.shutdownOnce.Do(func() {
		logger.GetGlobalLogger("crawler/amazon").Info("开始关闭Amazon处理器")

		ap.mu.Lock()
		ap.closed = true
		ap.mu.Unlock()

		// 取消所有超时管理器中的活跃任务
		if ap.timeoutManager != nil {
			ap.timeoutManager.CancelAll()
		}

		// 关闭池管理器
		if ap.poolManager != nil {
			ap.poolManager.Shutdown()
		}

		// 关闭浏览器池
		if ap.browserPool != nil {
			logger.GetGlobalLogger("crawler/amazon").Info("关闭浏览器池...")
			ap.browserPool.Shutdown()
		}

		logger.GetGlobalLogger("crawler/amazon").Info("Amazon处理器已关闭")
	})
}
