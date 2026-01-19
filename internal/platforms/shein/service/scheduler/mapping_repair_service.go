// Package scheduler 提供SHEIN平台SKU映射关系修复服务
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/repo"

	"github.com/sirupsen/logrus"
)

// MappingRepairService SKU映射关系修复服务接口
type MappingRepairService interface {
	// RepairMapping 修复单个SKU映射关系
	RepairMapping(ctx context.Context, request *MappingRepairRequest) (*MappingRepairResult, error)

	// BatchRepairMapping 批量修复SKU映射关系
	BatchRepairMapping(ctx context.Context, requests []*MappingRepairRequest) ([]*MappingRepairResult, error)

	// AutoRepairMapping 自动修复SKU映射关系（在查询失败时调用）
	AutoRepairMapping(ctx context.Context, skuCode string, storeID int64, reason string) (*MappingRepairResult, error)

	// GetRepairStats 获取修复统计信息
	GetRepairStats() *MappingRepairStats

	// RegisterStrategy 注册修复策略
	RegisterStrategy(strategy MappingRepairStrategy)
}

// mappingRepairServiceImpl SKU映射关系修复服务实现
type mappingRepairServiceImpl struct {
	mappingClient managementapi.ProductImportMappingAPI
	storeAPI      managementapi.StoreAPI
	productAPI    repo.ProductAPIInterface
	config        *MappingRepairConfig
	strategies    []MappingRepairStrategy
	stats         *MappingRepairStats
	statsMutex    sync.RWMutex
	logger        *logrus.Entry
}

// NewMappingRepairService 创建SKU映射关系修复服务
func NewMappingRepairService(
	mappingClient managementapi.ProductImportMappingAPI,
	storeAPI managementapi.StoreAPI,
	productAPI repo.ProductAPIInterface,
	config *MappingRepairConfig,
) MappingRepairService {
	if config == nil {
		config = DefaultMappingRepairConfig()
	}

	service := &mappingRepairServiceImpl{
		mappingClient: mappingClient,
		storeAPI:      storeAPI,
		productAPI:    productAPI,
		config:        config,
		strategies:    make([]MappingRepairStrategy, 0),
		stats: &MappingRepairStats{
			LastRepairTime: time.Now(),
		},
		logger: logrus.WithField("component", "MappingRepairService"),
	}

	// 注册默认修复策略
	service.registerDefaultStrategies()

	return service
}

// RepairMapping 修复单个SKU映射关系
func (s *mappingRepairServiceImpl) RepairMapping(ctx context.Context, request *MappingRepairRequest) (*MappingRepairResult, error) {
	s.logger.WithFields(logrus.Fields{
		"sku_code": request.SkuCode,
		"store_id": request.StoreID,
		"reason":   request.Reason,
	}).Info("开始修复SKU映射关系")

	s.updateStats(func(stats *MappingRepairStats) {
		stats.TotalRequests++
	})

	startTime := time.Now()

	// 构建修复上下文
	repairCtx, err := s.buildRepairContext(ctx, request)
	if err != nil {
		s.logger.WithError(err).WithField("sku_code", request.SkuCode).Error("构建修复上下文失败")
		return s.createFailedResult(request.SkuCode, fmt.Sprintf("构建修复上下文失败: %v", err)), nil
	}

	// 尝试使用各种策略进行修复
	for _, strategy := range s.strategies {
		if !strategy.CanRepair(repairCtx) {
			s.logger.WithFields(logrus.Fields{
				"sku_code": request.SkuCode,
				"strategy": strategy.GetStrategyName(),
			}).Debug("策略不适用，跳过")
			continue
		}

		s.logger.WithFields(logrus.Fields{
			"sku_code": request.SkuCode,
			"strategy": strategy.GetStrategyName(),
		}).Info("使用策略进行修复")

		result, err := strategy.Repair(repairCtx)
		if err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"sku_code": request.SkuCode,
				"strategy": strategy.GetStrategyName(),
			}).Warn("策略修复失败，尝试下一个策略")
			continue
		}

		if result.Success {
			duration := time.Since(startTime)
			s.updateStats(func(stats *MappingRepairStats) {
				stats.SuccessCount++
				stats.LastRepairTime = time.Now()
				stats.AverageTime = (stats.AverageTime*float64(stats.SuccessCount-1) + duration.Seconds()) / float64(stats.SuccessCount)
			})

			s.logger.WithFields(logrus.Fields{
				"sku_code": request.SkuCode,
				"strategy": strategy.GetStrategyName(),
				"duration": duration,
			}).Info("SKU映射关系修复成功")

			return result, nil
		}
	}

	// 所有策略都失败
	s.updateStats(func(stats *MappingRepairStats) {
		stats.FailedCount++
	})

	s.logger.WithField("sku_code", request.SkuCode).Warn("所有修复策略都失败")
	return s.createFailedResult(request.SkuCode, "所有修复策略都失败"), nil
}

// BatchRepairMapping 批量修复SKU映射关系
func (s *mappingRepairServiceImpl) BatchRepairMapping(ctx context.Context, requests []*MappingRepairRequest) ([]*MappingRepairResult, error) {
	s.logger.WithField("count", len(requests)).Info("开始批量修复SKU映射关系")

	results := make([]*MappingRepairResult, 0, len(requests))

	// 按批次处理
	batchSize := s.config.BatchSize
	for i := 0; i < len(requests); i += batchSize {
		end := i + batchSize
		if end > len(requests) {
			end = len(requests)
		}

		batch := requests[i:end]
		s.logger.WithFields(logrus.Fields{
			"batch_start": i,
			"batch_end":   end,
			"batch_size":  len(batch),
		}).Debug("处理批次")

		// 并发处理批次内的请求
		batchResults := s.processBatch(ctx, batch)
		results = append(results, batchResults...)
	}

	s.logger.WithFields(logrus.Fields{
		"total":   len(requests),
		"success": s.countSuccessResults(results),
		"failed":  len(results) - s.countSuccessResults(results),
	}).Info("批量修复SKU映射关系完成")

	return results, nil
}

// AutoRepairMapping 自动修复SKU映射关系
func (s *mappingRepairServiceImpl) AutoRepairMapping(ctx context.Context, skuCode string, storeID int64, reason string) (*MappingRepairResult, error) {
	if !s.config.EnableAutoRepair {
		s.logger.WithField("sku_code", skuCode).Debug("自动修复已禁用")
		return s.createFailedResult(skuCode, "自动修复已禁用"), nil
	}

	request := &MappingRepairRequest{
		SkuCode:  skuCode,
		StoreID:  storeID,
		Reason:   reason,
		Priority: 2, // 中等优先级
	}

	return s.RepairMapping(ctx, request)
}

// GetRepairStats 获取修复统计信息
func (s *mappingRepairServiceImpl) GetRepairStats() *MappingRepairStats {
	s.statsMutex.RLock()
	defer s.statsMutex.RUnlock()

	// 返回副本避免并发修改
	statsCopy := *s.stats
	return &statsCopy
}

// RegisterStrategy 注册修复策略
func (s *mappingRepairServiceImpl) RegisterStrategy(strategy MappingRepairStrategy) {
	s.strategies = append(s.strategies, strategy)
	s.logger.WithField("strategy", strategy.GetStrategyName()).Info("注册修复策略")
}

// buildRepairContext 构建修复上下文
func (s *mappingRepairServiceImpl) buildRepairContext(ctx context.Context, request *MappingRepairRequest) (*MappingRepairContext, error) {
	repairCtx := &MappingRepairContext{
		Request:   request,
		StartTime: time.Now(),
	}

	// 获取店铺信息
	storeInfo, err := s.storeAPI.GetStore(request.StoreID)
	if err != nil {
		return nil, fmt.Errorf("获取店铺信息失败: %w", err)
	}
	repairCtx.StoreInfo = storeInfo

	// 如果有SPU信息，尝试获取产品详情
	if request.SpuCode != "" || request.SpuName != "" {
		// 这里可以根据需要实现产品信息获取逻辑
		s.logger.WithFields(logrus.Fields{
			"spu_code": request.SpuCode,
			"spu_name": request.SpuName,
		}).Debug("获取产品信息")
	}

	return repairCtx, nil
}

// processBatch 处理批次
func (s *mappingRepairServiceImpl) processBatch(ctx context.Context, batch []*MappingRepairRequest) []*MappingRepairResult {
	results := make([]*MappingRepairResult, len(batch))
	var wg sync.WaitGroup

	for i, request := range batch {
		wg.Add(1)
		go func(index int, req *MappingRepairRequest) {
			defer wg.Done()

			result, err := s.RepairMapping(ctx, req)
			if err != nil {
				results[index] = s.createFailedResult(req.SkuCode, fmt.Sprintf("修复失败: %v", err))
			} else {
				results[index] = result
			}
		}(i, request)
	}

	wg.Wait()
	return results
}

// countSuccessResults 统计成功结果数量
func (s *mappingRepairServiceImpl) countSuccessResults(results []*MappingRepairResult) int {
	count := 0
	for _, result := range results {
		if result.Success {
			count++
		}
	}
	return count
}

// createFailedResult 创建失败结果
func (s *mappingRepairServiceImpl) createFailedResult(skuCode, errorMsg string) *MappingRepairResult {
	return &MappingRepairResult{
		SkuCode:    skuCode,
		Success:    false,
		Error:      errorMsg,
		RepairTime: time.Now(),
	}
}

// updateStats 更新统计信息
func (s *mappingRepairServiceImpl) updateStats(updateFunc func(*MappingRepairStats)) {
	s.statsMutex.Lock()
	defer s.statsMutex.Unlock()
	updateFunc(s.stats)
}

// registerDefaultStrategies 注册默认修复策略
func (s *mappingRepairServiceImpl) registerDefaultStrategies() {
	// 注册基于产品信息的修复策略
	s.RegisterStrategy(NewProductBasedRepairStrategy(s.mappingClient, s.productAPI))

	// 注册基于历史记录的修复策略
	s.RegisterStrategy(NewHistoryBasedRepairStrategy(s.mappingClient))
}
