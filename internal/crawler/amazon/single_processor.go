// Package amazon 提供Amazon单浏览器处理功能（降级模式）
package amazon

import (
	"context"
	"fmt"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon/browser"
	"task-processor/internal/model"
)

// SingleProcessor 单浏览器处理器，用于浏览器池初始化失败时的降级模式。
// 每次调用都会启动一个独立的浏览器实例，处理完毕后关闭。
type SingleProcessor struct {
	config               *config.Config
	urlHelper            *URLHelper
	productChecker       *ProductChecker
	failureArtifactStore *FailureArtifactStore
	qualityMetrics       qualityMetricsRecorder
}

// NewSingleProcessor 创建单浏览器处理器
func NewSingleProcessor(cfg *config.Config, urlHelper *URLHelper, productChecker *ProductChecker) *SingleProcessor {
	return &SingleProcessor{
		config:               cfg,
		urlHelper:            urlHelper,
		productChecker:       productChecker,
		failureArtifactStore: NewFailureArtifactStore(cfg),
		qualityMetrics:       nil,
	}
}

func (sp *SingleProcessor) SetQualityMetricsRecorder(recorder qualityMetricsRecorder) {
	sp.qualityMetrics = recorder
}

// ProcessWithSingleBrowser 使用独立浏览器实例处理单个产品（降级路径）。
// 复用 InstanceProcessor 的完整处理逻辑，包括货币检查和数据验证。
func (sp *SingleProcessor) ProcessWithSingleBrowser(url string, zipcode string) (*model.Product, error) {
	mgr := browser.NewBrowserManager(sp.config)

	if err := mgr.Install(); err != nil {
		return nil, fmt.Errorf("初始化Playwright失败: %w", err)
	}
	if err := mgr.Launch(); err != nil {
		return nil, fmt.Errorf("启动浏览器失败: %w", err)
	}
	defer mgr.Close()

	// 构造临时实例，复用 InstanceProcessor 的完整处理逻辑
	instance := &browser.BrowserInstance{
		ID:      -1, // 降级模式标识
		Manager: mgr,
	}

	ip := NewInstanceProcessor(sp.urlHelper, sp.productChecker, NewProductResultValidator())
	ip.SetFailureArtifactStore(sp.failureArtifactStore)
	ip.SetQualityControlOptions(sp.config.Amazon.QualityControl.RetryOnValidationFailure, sp.config.Amazon.QualityControl.ValidationRetryMaxAttempts)
	ip.SetQualityMetricsRecorder(sp.qualityMetrics)
	return ip.ProcessWithInstance(context.Background(), instance, url, zipcode)
}
