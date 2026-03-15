package product

import (
	"fmt"
	"sync"
	"task-processor/internal/platforms/temu/api/inventory"
	"time"

	"github.com/sirupsen/logrus"
)

// ProductProcessor 产品处理器
type ProductProcessor struct {
	inventoryAPI *inventory.API
	filter       *ProductFilter
	logger       *logrus.Entry
}

// NewProductProcessor 创建产品处理器
func NewProductProcessor(inventoryAPI *inventory.API, filter *ProductFilter, logger *logrus.Entry) *ProductProcessor {
	return &ProductProcessor{
		inventoryAPI: inventoryAPI,
		filter:       filter,
		logger:       logger,
	}
}

// ProcessProducts 处理产品列表
func (pp *ProductProcessor) ProcessProducts(products []inventory.Item, options *BulkRelistOptions) (*RelistAllResult, error) {
	pageResult := &RelistAllResult{
		TotalOfflineCount: len(products),
		ProcessedCount:    0,
		SuccessCount:      0,
		FailCount:         0,
		SkippedCount:      0,
		Results:           make([]RelistDetailResult, 0),
	}

	// 按商品ID分组
	goodsSkuMap := make(map[string][]string)
	productInfoMap := make(map[string]*inventory.Item)

	for _, product := range products {
		if _, exists := goodsSkuMap[product.GoodsID]; !exists {
			goodsSkuMap[product.GoodsID] = []string{}
			productInfoMap[product.GoodsID] = &product
		}
		goodsSkuMap[product.GoodsID] = append(goodsSkuMap[product.GoodsID], product.SkuID)
	}

	// 检查是否启用并发
	maxConcurrency := options.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 1 // 默认串行处理
	}

	if maxConcurrency == 1 {
		// 串行处理
		return pp.processSequential(goodsSkuMap, productInfoMap, options, pageResult)
	} else {
		// 并发处理
		return pp.processConcurrent(goodsSkuMap, productInfoMap, options, pageResult, maxConcurrency)
	}
}

// processSequential 串行处理产品
func (pp *ProductProcessor) processSequential(
	goodsSkuMap map[string][]string,
	productInfoMap map[string]*inventory.Item,
	options *BulkRelistOptions,
	result *RelistAllResult,
) (*RelistAllResult, error) {
	for goodsID, skuIDs := range goodsSkuMap {
		result.ProcessedCount++
		productInfo := productInfoMap[goodsID]

		detailResult := RelistDetailResult{
			GoodsID:   goodsID,
			GoodsName: productInfo.GoodsName,
			SkuIDs:    skuIDs,
			SkuCount:  len(skuIDs),
		}

		// 检查是否应该跳过
		if pp.filter.ShouldSkipProduct(productInfo, options.SkipConditions) {
			detailResult.Success = false
			detailResult.Skipped = true
			detailResult.Error = pp.filter.GetSkipReason(productInfo, options.SkipConditions)
			result.SkippedCount++
		} else {
			// 尝试上架
			success, lastError := pp.relistProduct(goodsID, skuIDs, productInfo, options)

			// 设置结果
			if success {
				detailResult.Success = true
				result.SuccessCount++
			} else {
				detailResult.Success = false
				detailResult.Error = lastError
				result.FailCount++
			}
		}

		result.Results = append(result.Results, detailResult)

		// 添加延迟 - 只有在实际处理商品（非跳过）且不是最后一个商品时才延迟
		if !detailResult.Skipped && result.ProcessedCount < len(goodsSkuMap) && options.DelayBetweenRequests > 0 {
			time.Sleep(time.Duration(options.DelayBetweenRequests) * time.Millisecond)
		}
	}

	return result, nil
}

// processConcurrent 并发处理产品
func (pp *ProductProcessor) processConcurrent(
	goodsSkuMap map[string][]string,
	productInfoMap map[string]*inventory.Item,
	options *BulkRelistOptions,
	result *RelistAllResult,
	maxConcurrency int,
) (*RelistAllResult, error) {
	// 创建工作任务
	type workItem struct {
		goodsID     string
		skuIDs      []string
		productInfo *inventory.Item
	}

	// 准备工作队列
	workQueue := make(chan workItem, len(goodsSkuMap))
	for goodsID, skuIDs := range goodsSkuMap {
		workQueue <- workItem{
			goodsID:     goodsID,
			skuIDs:      skuIDs,
			productInfo: productInfoMap[goodsID],
		}
	}
	close(workQueue)

	// 结果收集
	resultChan := make(chan RelistDetailResult, len(goodsSkuMap))
	var wg sync.WaitGroup

	// 启动工作协程
	for i := range maxConcurrency {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for work := range workQueue {
				detailResult := RelistDetailResult{
					GoodsID:   work.goodsID,
					GoodsName: work.productInfo.GoodsName,
					SkuIDs:    work.skuIDs,
					SkuCount:  len(work.skuIDs),
				}

				// 检查是否应该跳过
				if pp.filter.ShouldSkipProduct(work.productInfo, options.SkipConditions) {
					detailResult.Success = false
					detailResult.Skipped = true
					detailResult.Error = pp.filter.GetSkipReason(work.productInfo, options.SkipConditions)
				} else {
					// 尝试上架
					success, lastError := pp.relistProduct(work.goodsID, work.skuIDs, work.productInfo, options)

					// 设置结果
					if success {
						detailResult.Success = true
					} else {
						detailResult.Success = false
						detailResult.Error = lastError
					}
				}

				resultChan <- detailResult

				// 并发时的延迟控制 - 只有在实际处理商品（非跳过）时才延迟
				if !detailResult.Skipped && options.DelayBetweenRequests > 0 {
					time.Sleep(time.Duration(options.DelayBetweenRequests) * time.Millisecond)
				}
			}
		}(i)
	}

	// 等待所有工作完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	for detailResult := range resultChan {
		result.ProcessedCount++

		if detailResult.Skipped {
			result.SkippedCount++
		} else if detailResult.Success {
			result.SuccessCount++
		} else {
			result.FailCount++
		}

		result.Results = append(result.Results, detailResult)
	}

	return result, nil
}

// relistProduct 上架单个产品（支持多SKU）
func (pp *ProductProcessor) relistProduct(
	goodsID string,
	skuIDs []string,
	productInfo *inventory.Item,
	options *BulkRelistOptions,
) (bool, string) {
	success := false
	var lastError string

	if len(skuIDs) == 1 {
		// 单SKU商品，直接上架
		resp, err := pp.inventoryAPI.Relist(goodsID, skuIDs)
		if err != nil {
			lastError = err.Error()
		} else if resp != nil && resp.Result.Result {
			success = true
			pp.logger.Infof("✓ 单SKU上架成功: %s", productInfo.GoodsName)
		} else {
			lastError = "上架请求成功但结果为false"
		}
	} else {
		// 多SKU商品，先尝试全部一起上架
		pp.logger.Infof("尝试批量上架多SKU商品: %s (SKU数量: %d)", productInfo.GoodsName, len(skuIDs))
		resp, err := pp.inventoryAPI.Relist(goodsID, skuIDs)
		if err != nil {
			lastError = err.Error()
		} else if resp != nil && resp.Result.Result {
			success = true
			pp.logger.Infof("✓ 批量上架成功: %s", productInfo.GoodsName)
		} else {
			// 批量上架失败，尝试逐个SKU上架
			pp.logger.Infof("批量上架失败，尝试逐个SKU上架: %s", productInfo.GoodsName)
			pp.logger.Warnf("批量上架失败的商品详情: %+v", *productInfo)

			successCount := 0
			for i, skuID := range skuIDs {
				singleResp, singleErr := pp.inventoryAPI.Relist(goodsID, []string{skuID})
				if singleErr != nil {
					pp.logger.Warnf("SKU %s 上架失败: %v", skuID, singleErr)
					lastError = fmt.Sprintf("部分SKU上架失败: %v", singleErr)
				} else if singleResp != nil && singleResp.Result.Result {
					successCount++
					pp.logger.Infof("✓ SKU %s 上架成功 (%d/%d)", skuID, i+1, len(skuIDs))
				} else {
					pp.logger.Warnf("SKU %s 上架结果为false", skuID)
					lastError = "部分SKU上架结果为false"
				}

				// SKU间添加小延迟
				if i < len(skuIDs)-1 && options.DelayBetweenRequests > 0 {
					time.Sleep(time.Duration(options.DelayBetweenRequests/2) * time.Millisecond)
				}
			}

			if successCount > 0 {
				success = true
				if successCount == len(skuIDs) {
					pp.logger.Infof("✓ 所有SKU逐个上架成功: %s (%d/%d)", productInfo.GoodsName, successCount, len(skuIDs))
				} else {
					pp.logger.Infof("✓ 部分SKU上架成功: %s (%d/%d)", productInfo.GoodsName, successCount, len(skuIDs))
					lastError = fmt.Sprintf("仅 %d/%d 个SKU上架成功", successCount, len(skuIDs))
				}
			} else {
				lastError = "所有SKU上架都失败"
			}
		}
	}

	return success, lastError
}

