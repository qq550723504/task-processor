// Package service 提供通用核价服务的实现。
package service

import (
	"context"
	"fmt"
	"sync"
	"task-processor/common/pricing/model"
	"time"

	"github.com/sirupsen/logrus"
)

// DefaultCommonPricingService 默认通用核价服务实现
type DefaultCommonPricingService struct {
	decisionMaker PricingDecisionMaker
	adapters      map[string]PlatformAdapter
	adaptersMutex sync.RWMutex
	logger        *logrus.Entry
}

// NewDefaultCommonPricingService 创建默认通用核价服务
func NewDefaultCommonPricingService(decisionMaker PricingDecisionMaker) *DefaultCommonPricingService {
	return &DefaultCommonPricingService{
		decisionMaker: decisionMaker,
		adapters:      make(map[string]PlatformAdapter),
		logger:        logrus.WithField("component", "DefaultCommonPricingService"),
	}
}

// ProcessSingleProduct 处理单个商品核价
func (s *DefaultCommonPricingService) ProcessSingleProduct(ctx context.Context, pricingCtx *model.PricingContext) (*model.PricingResult, error) {
	startTime := time.Now()

	s.logger.Infof("开始处理商品核价: %s (ID: %s)", pricingCtx.ProductName, pricingCtx.ProductID)

	// 获取平台适配器
	var adapter PlatformAdapter
	if platformData, ok := pricingCtx.PlatformData.(map[string]interface{}); ok {
		if platformName, exists := platformData["platform"]; exists {
			if name, ok := platformName.(string); ok {
				if a, err := s.GetAdapter(name); err == nil {
					adapter = a
				}
			}
		}
	}

	// 预处理上下文
	if adapter != nil {
		if err := adapter.PreprocessContext(ctx, pricingCtx); err != nil {
			s.logger.Errorf("预处理上下文失败: %v", err)
			return nil, fmt.Errorf("预处理上下文失败: %w", err)
		}
	}

	// 制定核价决策
	result, err := s.decisionMaker.MakeDecision(ctx, pricingCtx)
	if err != nil {
		s.logger.Errorf("制定核价决策失败: %v", err)
		return nil, fmt.Errorf("制定核价决策失败: %w", err)
	}

	// 后处理结果
	if adapter != nil {
		if err := adapter.PostprocessResult(ctx, result); err != nil {
			s.logger.Errorf("后处理结果失败: %v", err)
			// 后处理失败不影响核价结果，只记录日志
		}
	}

	// 记录处理时间
	duration := time.Since(startTime)
	s.logger.Infof("商品 %s 核价完成: %s (耗时: %v)",
		pricingCtx.ProductName, result.Action, duration)

	return result, nil
}

// ProcessBatchProducts 批量处理商品核价
func (s *DefaultCommonPricingService) ProcessBatchProducts(ctx context.Context, contexts []*model.PricingContext) (*model.BatchPricingResult, error) {
	batchResult := &model.BatchPricingResult{
		StartTime: time.Now(),
		Results:   make([]*model.PricingResult, 0, len(contexts)),
	}

	s.logger.Infof("开始批量处理 %d 个商品的核价", len(contexts))

	// 并发处理商品核价
	const maxConcurrency = 10
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var resultsMutex sync.Mutex

	for _, pricingCtx := range contexts {
		wg.Add(1)
		go func(ctx context.Context, pricingCtx *model.PricingContext) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 处理单个商品
			result, err := s.ProcessSingleProduct(ctx, pricingCtx)
			if err != nil {
				s.logger.Errorf("处理商品 %s 失败: %v", pricingCtx.ProductName, err)
				// 创建失败结果
				result = &model.PricingResult{
					ProductID:   pricingCtx.ProductID,
					SkuID:       pricingCtx.SkuID,
					ProductName: pricingCtx.ProductName,
				}
				result.SetDecision(model.ActionSkip, fmt.Sprintf("处理失败: %v", err))
			}

			// 线程安全地添加结果
			resultsMutex.Lock()
			batchResult.AddResult(result)
			resultsMutex.Unlock()
		}(ctx, pricingCtx)
	}

	// 等待所有任务完成
	wg.Wait()
	batchResult.Finish()

	s.logger.Infof("批量核价完成: 总数=%d, 成功=%d, 失败=%d, 跳过=%d, 耗时=%v",
		batchResult.TotalCount, batchResult.SuccessCount,
		batchResult.FailCount, batchResult.SkipCount, batchResult.Duration)

	return batchResult, nil
}

// RegisterAdapter 注册平台适配器
func (s *DefaultCommonPricingService) RegisterAdapter(platformName string, adapter PlatformAdapter) {
	s.adaptersMutex.Lock()
	defer s.adaptersMutex.Unlock()

	s.adapters[platformName] = adapter
	s.logger.Infof("注册平台适配器: %s", platformName)
}

// GetAdapter 获取平台适配器
func (s *DefaultCommonPricingService) GetAdapter(platformName string) (PlatformAdapter, error) {
	s.adaptersMutex.RLock()
	defer s.adaptersMutex.RUnlock()

	adapter, exists := s.adapters[platformName]
	if !exists {
		return nil, fmt.Errorf("未找到平台 %s 的适配器", platformName)
	}

	return adapter, nil
}

// GetRegisteredPlatforms 获取已注册的平台列表
func (s *DefaultCommonPricingService) GetRegisteredPlatforms() []string {
	s.adaptersMutex.RLock()
	defer s.adaptersMutex.RUnlock()

	platforms := make([]string, 0, len(s.adapters))
	for platform := range s.adapters {
		platforms = append(platforms, platform)
	}

	return platforms
}
