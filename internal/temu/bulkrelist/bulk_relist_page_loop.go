package bulkrelist

import (
	"fmt"
	"task-processor/internal/temu/api/inventory"
	"time"

	"github.com/sirupsen/logrus"
)

// PageLoopProcessor 循环处理第一页处理器
type PageLoopProcessor struct {
	inventoryAPI *inventory.API
	processor    *ProductProcessor
	logger       *logrus.Entry
}

// NewPageLoopProcessor 创建循环处理第一页处理器
func NewPageLoopProcessor(inventoryAPI *inventory.API, processor *ProductProcessor, logger *logrus.Entry) *PageLoopProcessor {
	return &PageLoopProcessor{
		inventoryAPI: inventoryAPI,
		processor:    processor,
		logger:       logger,
	}
}

// ProcessFirstPageLoop 循环处理第一页
func (plp *PageLoopProcessor) ProcessFirstPageLoop(options *BulkRelistOptions, result *RelistAllResult, pageSize int) (*RelistAllResult, error) {
	pageNo := 1
	roundCount := 0
	consecutiveNoSuccessRounds := 0 // 连续无成功上架的轮数
	maxNoSuccessRounds := 3         // 最大连续无成功轮数

	for {
		roundCount++
		plp.logger.Infof("=== 第 %d 轮处理第一页 ===", roundCount)

		// 获取第一页的已下架产品
		resp, err := plp.inventoryAPI.SearchOffline(pageNo, pageSize)
		if err != nil {
			plp.logger.WithError(err).Error("获取第一页已下架产品失败")
			return result, fmt.Errorf("获取第一页已下架产品失败: %w", err)
		}

		if resp == nil || len(resp.Result.SkuList) == 0 {
			plp.logger.Info("第一页没有已下架产品，处理完成")
			break
		}

		// 更新总数
		result.TotalOfflineCount = resp.Result.Total
		plp.logger.Infof("第一页获取到 %d 个产品，总下架数: %d", len(resp.Result.SkuList), result.TotalOfflineCount)

		// 记录本轮处理前的成功数量
		beforeSuccessCount := result.SuccessCount

		// 处理当前页的产品
		pageResult, err := plp.processor.ProcessProducts(resp.Result.SkuList, options)
		if err != nil {
			plp.logger.WithError(err).Error("处理第一页产品失败")
			return result, fmt.Errorf("处理第一页产品失败: %w", err)
		}

		// 合并结果
		result.ProcessedCount += pageResult.ProcessedCount
		result.SuccessCount += pageResult.SuccessCount
		result.FailCount += pageResult.FailCount
		result.SkippedCount += pageResult.SkippedCount
		result.Results = append(result.Results, pageResult.Results...)

		// 检查本轮是否有成功上架的商品
		roundSuccessCount := result.SuccessCount - beforeSuccessCount
		if roundSuccessCount == 0 {
			consecutiveNoSuccessRounds++
			plp.logger.Warnf("第 %d 轮无成功上架商品，连续无成功轮数: %d/%d",
				roundCount, consecutiveNoSuccessRounds, maxNoSuccessRounds)
		} else {
			consecutiveNoSuccessRounds = 0 // 重置计数器
		}

		plp.logger.Infof("第 %d 轮处理完成: 处理=%d, 成功=%d, 失败=%d, 跳过=%d",
			roundCount, pageResult.ProcessedCount, pageResult.SuccessCount,
			pageResult.FailCount, pageResult.SkippedCount)

		// 如果连续多轮都没有成功上架，停止循环
		if consecutiveNoSuccessRounds >= maxNoSuccessRounds {
			plp.logger.Warnf("连续 %d 轮无成功上架商品，停止循环处理", consecutiveNoSuccessRounds)
			break
		}

		// 如果第一页处理的商品数量少于页面大小，说明已经处理完了
		if len(resp.Result.SkuList) < pageSize {
			plp.logger.Info("第一页商品数量少于页面大小，可能已处理完成")
		}

		// 添加轮次间的延迟
		if options.DelayBetweenRequests > 0 {
			plp.logger.Infof("等待 %d 毫秒后开始下一轮", options.DelayBetweenRequests)
			time.Sleep(time.Duration(options.DelayBetweenRequests) * time.Millisecond)
		}
	}

	plp.logger.Infof("循环处理第一页完成: 共 %d 轮, 总处理数=%d, 成功=%d, 失败=%d, 跳过=%d",
		roundCount, result.ProcessedCount, result.SuccessCount, result.FailCount, result.SkippedCount)

	return result, nil
}

// ProcessAllPages 处理所有页面（流式处理）
func (plp *PageLoopProcessor) ProcessAllPages(options *BulkRelistOptions, result *RelistAllResult, pageSize int) (*RelistAllResult, error) {
	pageNo := 1

	for {
		// 获取当前页的已下架产品
		plp.logger.Infof("获取第 %d 页已下架产品", pageNo)
		resp, err := plp.inventoryAPI.SearchOffline(pageNo, pageSize)
		if err != nil {
			plp.logger.WithError(err).Errorf("获取第%d页已下架产品失败", pageNo)
			return result, fmt.Errorf("获取第%d页已下架产品失败: %w", pageNo, err)
		}

		if resp == nil || len(resp.Result.SkuList) == 0 {
			plp.logger.Info("没有更多已下架产品")
			break
		}

		// 更新总数（第一次获取时）
		if pageNo == 1 {
			result.TotalOfflineCount = resp.Result.Total
			plp.logger.Infof("发现总共 %d 个已下架产品", result.TotalOfflineCount)
		}

		plp.logger.Infof("第 %d 页获取到 %d 个产品，开始处理", pageNo, len(resp.Result.SkuList))

		// 立即处理当前页的产品
		pageResult, err := plp.processor.ProcessProducts(resp.Result.SkuList, options)
		if err != nil {
			plp.logger.WithError(err).Errorf("处理第%d页产品失败", pageNo)
			return result, fmt.Errorf("处理第%d页产品失败: %w", pageNo, err)
		}

		// 合并结果
		result.ProcessedCount += pageResult.ProcessedCount
		result.SuccessCount += pageResult.SuccessCount
		result.FailCount += pageResult.FailCount
		result.SkippedCount += pageResult.SkippedCount
		result.Results = append(result.Results, pageResult.Results...)

		plp.logger.Infof("第 %d 页处理完成: 处理=%d, 成功=%d, 失败=%d, 跳过=%d",
			pageNo, pageResult.ProcessedCount, pageResult.SuccessCount,
			pageResult.FailCount, pageResult.SkippedCount)

		// 显示总体进度
		if result.TotalOfflineCount > 0 {
			progress := float64(result.ProcessedCount) / float64(result.TotalOfflineCount) * 100
			plp.logger.Infof("总体进度: %.1f%% (%d/%d)", progress, result.ProcessedCount, result.TotalOfflineCount)
		}

		// 检查是否处理完所有产品
		if result.ProcessedCount >= result.TotalOfflineCount {
			plp.logger.Info("所有产品处理完成")
			break
		}

		pageNo++
	}

	plp.logger.Infof("流式处理完成: 总下架数=%d, 处理数=%d, 成功=%d, 失败=%d, 跳过=%d",
		result.TotalOfflineCount, result.ProcessedCount, result.SuccessCount, result.FailCount, result.SkippedCount)

	return result, nil
}

// ProcessWithFilter 根据过滤条件处理产品（流式处理）
func (plp *PageLoopProcessor) ProcessWithFilter(filter *ProductFilterOptions, options *BulkRelistOptions, result *RelistAllResult, pageSize int, productFilter *ProductFilter) (*RelistAllResult, error) {
	pageNo := 1

	plp.logger.Info("开始流式处理：获取一页，筛选并处理一页")

	for {
		// 获取当前页的已下架产品
		plp.logger.Infof("获取第 %d 页已下架产品", pageNo)
		resp, err := plp.inventoryAPI.SearchOffline(pageNo, pageSize)
		if err != nil {
			plp.logger.WithError(err).Errorf("获取第%d页已下架产品失败", pageNo)
			return result, fmt.Errorf("获取第%d页已下架产品失败: %w", pageNo, err)
		}

		if resp == nil || len(resp.Result.SkuList) == 0 {
			plp.logger.Info("没有更多已下架产品")
			break
		}

		// 更新总数（第一次获取时）
		if pageNo == 1 {
			result.TotalOfflineCount = resp.Result.Total
			plp.logger.Infof("发现总共 %d 个已下架产品", result.TotalOfflineCount)
		}

		// 根据过滤条件筛选当前页产品
		var filteredProducts []inventory.Item
		for _, product := range resp.Result.SkuList {
			if productFilter.MatchesFilter(&product, filter) {
				filteredProducts = append(filteredProducts, product)
			}
		}

		plp.logger.Infof("第 %d 页获取到 %d 个产品，筛选后 %d 个产品，开始处理",
			pageNo, len(resp.Result.SkuList), len(filteredProducts))

		if len(filteredProducts) > 0 {
			// 立即处理筛选后的产品
			pageResult, err := plp.processor.ProcessProducts(filteredProducts, options)
			if err != nil {
				plp.logger.WithError(err).Errorf("处理第%d页产品失败", pageNo)
				return result, fmt.Errorf("处理第%d页产品失败: %w", pageNo, err)
			}

			// 合并结果
			result.ProcessedCount += pageResult.ProcessedCount
			result.SuccessCount += pageResult.SuccessCount
			result.FailCount += pageResult.FailCount
			result.SkippedCount += pageResult.SkippedCount
			result.Results = append(result.Results, pageResult.Results...)

			plp.logger.Infof("第 %d 页处理完成: 处理=%d, 成功=%d, 失败=%d, 跳过=%d",
				pageNo, pageResult.ProcessedCount, pageResult.SuccessCount,
				pageResult.FailCount, pageResult.SkippedCount)
		}

		// 显示总体进度
		if result.TotalOfflineCount > 0 {
			progress := float64((pageNo * pageSize)) / float64(result.TotalOfflineCount) * 100
			if progress > 100 {
				progress = 100
			}
			plp.logger.Infof("总体进度: %.1f%% (已处理页数: %d)", progress, pageNo)
		}

		// 检查是否已经处理完所有页面
		if len(resp.Result.SkuList) < pageSize {
			plp.logger.Info("已处理完所有页面")
			break
		}

		pageNo++
	}

	plp.logger.Infof("流式处理完成: 总下架数=%d, 处理数=%d, 成功=%d, 失败=%d, 跳过=%d",
		result.TotalOfflineCount, result.ProcessedCount, result.SuccessCount, result.FailCount, result.SkippedCount)

	return result, nil
}
