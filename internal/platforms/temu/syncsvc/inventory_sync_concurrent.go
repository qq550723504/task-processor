// Package syncsvc 提供TEMU平台库存监控并发处理逻辑
package syncsvc

import (
	"context"
	"fmt"
	"sync"
	"time"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/recovery"

	"github.com/sirupsen/logrus"
)

// monitorInventoryChangesConcurrent 并发监控库存和价格变化
func (s *inventorySyncServiceImpl) monitorInventoryChangesConcurrent(
	ctx context.Context,
	products []*managementapi.ProductDataDTO,
	tenantID, storeID int64,
	operationStrategy *managementapi.OperationStrategyDTO,
) (*MonitorResult, error) {
	totalCount := len(products)

	result := &MonitorResult{
		TotalProducts: totalCount,
	}

	// 使用互斥锁保护结果统计
	var resultMutex sync.Mutex

	// 控制并发数量，避免过多goroutine
	maxConcurrency := s.getMaxConcurrency()
	semaphore := make(chan struct{}, maxConcurrency)

	// 等待所有goroutine完成
	var wg sync.WaitGroup

	// 收集所有需要批量更新的库存信息
	inventoryUpdateChan := make(chan *InventoryUpdateBatch, totalCount)

	// 启动批量更新处理器
	batchUpdateCtx, batchUpdateCancel := context.WithCancel(ctx)
	defer batchUpdateCancel()

	batchUpdateWg := sync.WaitGroup{}
	batchUpdateWg.Add(1)
	go s.batchInventoryUpdateProcessor(batchUpdateCtx, inventoryUpdateChan, &batchUpdateWg)

	// 进度监控
	progressTicker := time.NewTicker(10 * time.Second)
	defer progressTicker.Stop()

	// 启动进度监控goroutine
	progressCtx, progressCancel := context.WithCancel(ctx)
	defer progressCancel()

	progressWg := sync.WaitGroup{}
	progressWg.Add(1)
	go s.progressMonitor(progressCtx, progressTicker.C, result, &resultMutex, totalCount, &progressWg)

	s.logger.WithFields(logrus.Fields{
		"total_products":  totalCount,
		"max_concurrency": maxConcurrency,
	}).Info("开始并发处理TEMU产品库存监控")

	// 并发处理每个产品
	for i, prod := range products {
		wg.Add(1)

		go func(index int, product *managementapi.ProductDataDTO) {
			defer recovery.Recover("处理TEMU产品", s.logger)
			defer wg.Done()

			// 获取信号量，控制并发数
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			s.logger.WithFields(logrus.Fields{
				"current":    index + 1,
				"total":      totalCount,
				"product_id": product.ProductID,
			}).Debug("开始并发处理TEMU产品")

			// 处理单个产品的库存监控
			s.monitorSingleProduct(ctx, product, tenantID, storeID, result, &resultMutex, operationStrategy)
			resultMutex.Lock()
			result.ProcessedProducts++
			resultMutex.Unlock()
		}(i, prod)
	}

	// 等待所有产品处理完成
	wg.Wait()

	// 关闭批量更新通道并等待处理完成
	close(inventoryUpdateChan)
	batchUpdateCancel()
	batchUpdateWg.Wait()

	// 停止进度监控
	progressCancel()
	progressWg.Wait()

	s.logger.WithFields(logrus.Fields{
		"total":          result.TotalProducts,
		"processed":      result.ProcessedProducts,
		"skipped":        result.SkippedProducts,
		"price_changes":  result.PriceChanges,
		"stock_changes":  result.StockChanges,
		"amazon_fetched": result.AmazonFetched,
		"amazon_failed":  result.AmazonFailed,
	}).Info("并发监控TEMU产品库存和价格变化完成")

	return result, nil
}

// getMaxConcurrency 获取最大并发数
func (s *inventorySyncServiceImpl) getMaxConcurrency() int {
	// 默认并发数为20
	return 3
}

// batchInventoryUpdateProcessor 批量库存更新处理器
func (s *inventorySyncServiceImpl) batchInventoryUpdateProcessor(
	ctx context.Context,
	updateChan <-chan *InventoryUpdateBatch,
	wg *sync.WaitGroup,
) {
	defer recovery.Recover("批量库存更新处理器", s.logger)
	defer wg.Done()

	for {
		select {
		case batch, ok := <-updateChan:
			if !ok {
				s.logger.Debug("批量库存更新通道已关闭")
				return
			}

			if batch != nil && len(batch.Updates) > 0 {
				if err := s.batchUpdateTemuInventoryInAttributes(ctx, batch); err != nil {
					s.logger.WithError(err).WithField("product_id", batch.Product.ProductID).Error("批量更新TEMU产品库存失败")
				}
			}

		case <-ctx.Done():
			s.logger.Debug("批量库存更新处理器收到取消信号")
			return
		}
	}
}

// progressMonitor 进度监控器
func (s *inventorySyncServiceImpl) progressMonitor(
	ctx context.Context,
	ticker <-chan time.Time,
	result *MonitorResult,
	resultMutex *sync.Mutex,
	totalCount int,
	wg *sync.WaitGroup,
) {
	defer recovery.Recover("进度监控器", s.logger)
	defer wg.Done()

	for {
		select {
		case <-ticker:
			resultMutex.Lock()
			currentProcessed := result.ProcessedProducts
			currentSkipped := result.SkippedProducts
			currentPriceChanges := result.PriceChanges
			currentStockChanges := result.StockChanges
			currentAmazonFetched := result.AmazonFetched
			currentAmazonFailed := result.AmazonFailed
			resultMutex.Unlock()

			progress := float64(currentProcessed+currentSkipped) / float64(totalCount) * 100
			s.logger.WithFields(logrus.Fields{
				"processed":      currentProcessed,
				"skipped":        currentSkipped,
				"total":          totalCount,
				"progress":       fmt.Sprintf("%.1f%%", progress),
				"price_changes":  currentPriceChanges,
				"stock_changes":  currentStockChanges,
				"amazon_fetched": currentAmazonFetched,
				"amazon_failed":  currentAmazonFailed,
			}).Infof("TEMU库存监控进度: %d/%d (%.1f%%)", currentProcessed+currentSkipped, totalCount, progress)

		case <-ctx.Done():
			s.logger.Debug("进度监控器收到取消信号")
			return
		}
	}
}
