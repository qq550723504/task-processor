// Package amazon 提供Amazon处理器核心功能
package amazon

import (
	"context"
	"fmt"
	"sync"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon/browser"
	"task-processor/internal/domain/model"
	"time"

	"github.com/sirupsen/logrus"
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

		logrus.Infof("启用随机配置 - 策略: %s, 预设: %s, 指纹策略: %s",
			cfg.Browser.RandomConfig.Strategy, cfg.Browser.RandomConfig.PresetName, cfg.Browser.RandomConfig.FingerprintStrategy)
	} else {
		poolConfig.UseRandomFingerprint = false
		logrus.Info("使用传统浏览器配置")
	}

	logrus.Infof("创建Amazon处理器，浏览器池大小: %d (配置值: %d)", poolConfig.Size, cfg.Browser.PoolSize)
	browserPool := browser.NewBrowserPool(cfg, poolConfig)

	// 创建辅助组件（需要在浏览器池初始化前创建，因为池管理器需要它们）
	urlHelper := NewURLHelper()
	productChecker := NewProductChecker()

	// 初始化浏览器池
	usePool := true
	var poolManager *browser.PoolManager
	if err := browserPool.Initialize(); err != nil {
		logrus.Infof("初始化浏览器池失败: %v，将使用单浏览器模式", err)
		usePool = false
		browserPool = nil
	} else {
		logrus.Info("浏览器池初始化成功")
		poolManager = browser.NewPoolManager(browserPool)
	}

	singleProcessor := NewSingleProcessor(cfg, urlHelper, productChecker)
	batchProcessor := NewBatchProcessor(browserPool, urlHelper, productChecker)
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
	// 检查处理器是否已关闭
	ap.mu.RLock()
	if ap.closed {
		ap.mu.RUnlock()
		return nil, fmt.Errorf("Amazon处理器已关闭")
	}
	ap.mu.RUnlock()

	startTime := time.Now()
	logrus.Infof("开始处理Amazon产品: %s", url)

	if ap.usePool && ap.poolManager != nil {
		return ap.processWithPoolManager(url, zipcode)
	}
	return ap.singleProcessor.ProcessWithSingleBrowser(url, zipcode, startTime)
}

// processWithPoolManager 使用池管理器处理
func (ap *AmazonProcessor) processWithPoolManager(url string, zipcode string) (*model.Product, error) {
	ctx := context.Background()
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

	logrus.Infof("开始批量处理 %d 个Amazon产品", len(requests))
	startTime := time.Now()

	var results []model.ProductResult
	if ap.usePool {
		results = ap.batchProcessor.ProcessWithPool(requests, ap.browserPool)
	} else {
		results = ap.batchProcessor.ProcessWithSingleBrowser(requests, ap)
	}

	duration := time.Since(startTime)
	successCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
		}
	}

	logrus.Infof("批量处理完成: 成功 %d/%d, 耗时: %v", successCount, len(requests), duration)
	return results
}

// processWithPool 使用浏览器池处理
func (ap *AmazonProcessor) processWithPool(url string, zipcode string) (*model.Product, error) {
	maxRetries := 2 // 最多重试2次（即总共尝试3次）

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			logrus.Infof("开始第 %d 次重试处理产品: %s", attempt, url)
		}

		// 从池中获取浏览器实例
		instance, err := ap.browserPool.Acquire()
		if err != nil {
			return nil, fmt.Errorf("获取浏览器实例失败: %w", err)
		}

		// 使用实例处理产品
		product, processErr := ap.processWithInstance(instance, url, zipcode)

		// 检查是否为严重错误
		if processErr != nil && ap.browserPool.IsBlockedOrSeriousError(processErr) {
			logrus.Warnf("检测到浏览器实例 %d 出现严重错误: %v", instance.ID, processErr)

			// 同步重建浏览器实例
			newInstance := ap.browserPool.RecreateInstanceSync(instance)

			// 如果重建失败
			if newInstance == nil {
				logrus.Errorf("重建浏览器实例失败，任务失败: %s", url)
				return nil, fmt.Errorf("重建浏览器实例失败: %w", processErr)
			}

			// 如果是最后一次尝试，返回错误
			if attempt >= maxRetries {
				logrus.Errorf("已达到最大重试次数，任务失败: %s", url)
				// 将重建的实例放回池中
				ap.browserPool.Release(newInstance)
				return nil, processErr
			}

			// 否则继续下一次重试
			logrus.Infof("浏览器实例已重建为 %d，准备重试", newInstance.ID)
			continue
		}

		// 如果没有错误或不是严重错误，正常释放实例
		ap.browserPool.ReleaseWithError(instance, processErr)

		if processErr != nil {
			return nil, processErr
		}

		return product, nil
	}

	// 理论上不会到这里
	return nil, fmt.Errorf("处理产品失败，已达到最大重试次数")
}

// processWithInstance 使用指定实例处理产品
func (ap *AmazonProcessor) processWithInstance(instance *browser.BrowserInstance, url string, zipcode string) (*model.Product, error) {
	processor := NewInstanceProcessor(ap.urlHelper, ap.productChecker)
	return processor.ProcessWithInstance(instance, url, zipcode)
}

// Shutdown 关闭处理器
func (ap *AmazonProcessor) Shutdown() {
	ap.shutdownOnce.Do(func() {
		logrus.Info("开始关闭Amazon处理器")

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
			logrus.Info("关闭浏览器池...")
			ap.browserPool.Shutdown()
		}

		logrus.Info("Amazon处理器已关闭")
	})
}
