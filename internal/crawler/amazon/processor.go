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
	browserPool     *browser.BrowserPool
	poolManager     *browser.PoolManager
	config          *config.Config
	usePool         bool
	singleProcessor *SingleProcessor
	batchProcessor  *BatchProcessor
	urlHelper       *URLHelper
	productChecker  *ProductChecker
	timeoutManager  *TimeoutManager
	shutdownOnce    sync.Once    // 确保只关闭一次
	closed          bool         // 标记是否已关闭
	mu              sync.RWMutex // 保护 closed 字段
}

// NewAmazonProcessor 使用全局配置创建Amazon处理器
func NewAmazonProcessor(cfg *config.Config) *AmazonProcessor {
	// 创建浏览器池配置
	poolConfig := browser.DefaultBrowserPoolConfig()

	// 如果配置中的PoolSize为0，使用默认值
	if cfg.Browser.PoolSize > 0 {
		poolConfig.Size = cfg.Browser.PoolSize
	}

	// 应用随机配置设置
	if cfg.Browser.RandomConfig.Enabled {
		poolConfig.UseRandomFingerprint = true
		poolConfig.FingerprintStrategy = cfg.Browser.RandomConfig.FingerprintStrategy
		poolConfig.PresetName = cfg.Browser.RandomConfig.PresetName
		poolConfig.HealthCheckEnabled = cfg.Browser.RandomConfig.HealthCheckEnabled
		poolConfig.MaxRetries = cfg.Browser.RandomConfig.MaxRetries

		logger.GetGlobalLogger("crawler/amazon").Infof("启用随机配置 - 策略: %s, 预设: %s, 指纹策略: %s",
			cfg.Browser.RandomConfig.Strategy, cfg.Browser.RandomConfig.PresetName, cfg.Browser.RandomConfig.FingerprintStrategy)
	} else {
		poolConfig.UseRandomFingerprint = false
		logger.GetGlobalLogger("crawler/amazon").Info("使用传统浏览器配置")
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("创建Amazon处理器，浏览器池大小: %d (配置值: %d)", poolConfig.Size, cfg.Browser.PoolSize)
	browserPool := browser.NewBrowserPool(cfg, poolConfig)

	// 创建辅助组件（需要在浏览器池初始化前创建，因为池管理器需要它们）
	urlHelper := NewURLHelper()
	productChecker := NewProductChecker()

	// 初始化浏览器池
	usePool := true
	var poolManager *browser.PoolManager
	if err := browserPool.Initialize(); err != nil {
		logger.GetGlobalLogger("crawler/amazon").Infof("初始化浏览器池失败: %v，将使用单浏览器模式", err)
		usePool = false
		browserPool = nil
	} else {
		logger.GetGlobalLogger("crawler/amazon").Info("浏览器池初始化成功")
		poolManager = browser.NewPoolManager(browserPool)
	}

	singleProcessor := NewSingleProcessor(cfg, urlHelper, productChecker)
	batchProcessor := NewBatchProcessor(urlHelper, productChecker)
	timeoutManager := NewTimeoutManager(5 * time.Minute) // 默认5分钟超时

	return &AmazonProcessor{
		browserPool:     browserPool,
		poolManager:     poolManager,
		config:          cfg,
		usePool:         usePool,
		singleProcessor: singleProcessor,
		batchProcessor:  batchProcessor,
		urlHelper:       urlHelper,
		productChecker:  productChecker,
		timeoutManager:  timeoutManager,
		closed:          false,
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

	logger.GetGlobalLogger("crawler/amazon").Infof("开始处理Amazon产品: %s", url)

	if ap.usePool && ap.poolManager != nil {
		return ap.processWithPoolManager(ctx, url, zipcode)
	}
	return ap.singleProcessor.ProcessWithSingleBrowser(url, zipcode)
}

// processWithPoolManager 使用池管理器处理
func (ap *AmazonProcessor) processWithPoolManager(ctx context.Context, url string, zipcode string) (*model.Product, error) {
	timeout := 3 * time.Minute // 单个产品处理超时3分钟

	// 创建实例处理器
	processor := NewInstanceProcessor(ap.urlHelper, ap.productChecker)

	return ap.poolManager.ProcessWithTimeout(ctx, url, zipcode, timeout, processor)
}

// ProcessBatch 批量处理多个Amazon产品页面
func (ap *AmazonProcessor) ProcessBatch(requests []model.ProductRequest) []model.ProductResult {
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

	var results []model.ProductResult
	if ap.usePool {
		results = ap.batchProcessor.ProcessWithPool(requests, ap.browserPool)
	} else {
		// 单浏览器降级模式：逐个调用 Process
		results = make([]model.ProductResult, len(requests))
		for i, req := range requests {
			product, err := ap.Process(req.URL, req.Zipcode)
			results[i] = model.ProductResult{Product: product, Error: err}
		}
	}

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
		if ap.usePool && ap.browserPool != nil {
			logger.GetGlobalLogger("crawler/amazon").Info("关闭浏览器池...")
			ap.browserPool.Shutdown()
		}

		logger.GetGlobalLogger("crawler/amazon").Info("Amazon处理器已关闭")
	})
}
