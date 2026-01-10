package temu

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// BulkRelistService 批量重新上架服务
type BulkRelistService struct {
	apiClient *APIClient
	logger    *logrus.Entry
}

// NewBulkRelistService 创建批量重新上架服务
func NewBulkRelistService(apiClient *APIClient) *BulkRelistService {
	return &BulkRelistService{
		apiClient: apiClient,
		logger:    apiClient.GetLogger(),
	}
}

// RelistAllOfflineProducts 获取所有已下架产品并逐个全部上架
func (s *BulkRelistService) RelistAllOfflineProducts(options *BulkRelistOptions) (*RelistAllResult, error) {
	s.logger.Info("开始获取所有已下架产品并逐个上架")

	// 使用默认选项
	if options == nil {
		options = &BulkRelistOptions{
			DelayBetweenRequests: 1000, // 默认1秒延迟
			SkipConditions: &SkipConditions{
				SkipNeedRectification: true,
				SkipSeverelyPunished:  true,
				SkipLocked:            true,
			},
		}
	}

	result := &RelistAllResult{
		TotalOfflineCount: 0, // 初始不知道总数
		ProcessedCount:    0,
		SuccessCount:      0,
		FailCount:         0,
		SkippedCount:      0,
		Results:           make([]RelistDetailResult, 0),
	}

	pageSize := 200 // 使用最大页面大小

	if options.ProcessFirstPageOnly {
		s.logger.Info("启用循环处理第一页模式")
		return s.processFirstPageLoop(options, result, pageSize)
	}

	s.logger.Info("开始流式处理：获取一页，处理一页")
	return s.processAllPages(options, result, pageSize)
}

// processFirstPageLoop 循环处理第一页
func (s *BulkRelistService) processFirstPageLoop(options *BulkRelistOptions, result *RelistAllResult, pageSize int) (*RelistAllResult, error) {
	pageNo := 1
	roundCount := 0

	for {
		roundCount++
		s.logger.Infof("=== 第 %d 轮处理第一页 ===", roundCount)

		// 获取第一页的已下架产品
		resp, err := s.apiClient.GetOfflineProducts(pageNo, pageSize)
		if err != nil {
			s.logger.WithError(err).Error("获取第一页已下架产品失败")
			return result, fmt.Errorf("获取第一页已下架产品失败: %w", err)
		}

		if resp == nil || len(resp.Result.SkuList) == 0 {
			s.logger.Info("第一页没有已下架产品，处理完成")
			break
		}

		// 更新总数
		result.TotalOfflineCount = resp.Result.Total
		s.logger.Infof("第一页获取到 %d 个产品，总下架数: %d", len(resp.Result.SkuList), result.TotalOfflineCount)

		// 处理当前页的产品
		pageResult, err := s.processPageProducts(resp.Result.SkuList, options)
		if err != nil {
			s.logger.WithError(err).Error("处理第一页产品失败")
			return result, fmt.Errorf("处理第一页产品失败: %w", err)
		}

		// 合并结果
		result.ProcessedCount += pageResult.ProcessedCount
		result.SuccessCount += pageResult.SuccessCount
		result.FailCount += pageResult.FailCount
		result.SkippedCount += pageResult.SkippedCount
		result.Results = append(result.Results, pageResult.Results...)

		s.logger.Infof("第 %d 轮处理完成: 处理=%d, 成功=%d, 失败=%d, 跳过=%d",
			roundCount, pageResult.ProcessedCount, pageResult.SuccessCount,
			pageResult.FailCount, pageResult.SkippedCount)

		// 如果第一页处理的商品数量少于页面大小，说明已经处理完了
		if len(resp.Result.SkuList) < pageSize {
			s.logger.Info("第一页商品数量少于页面大小，可能已处理完成")
		}

		// 添加轮次间的延迟
		if options.DelayBetweenRequests > 0 {
			s.logger.Infof("等待 %d 毫秒后开始下一轮", options.DelayBetweenRequests)
			time.Sleep(time.Duration(options.DelayBetweenRequests) * time.Millisecond)
		}
	}

	s.logger.Infof("循环处理第一页完成: 共 %d 轮, 总处理数=%d, 成功=%d, 失败=%d, 跳过=%d",
		roundCount, result.ProcessedCount, result.SuccessCount, result.FailCount, result.SkippedCount)

	return result, nil
}

// processAllPages 处理所有页面
func (s *BulkRelistService) processAllPages(options *BulkRelistOptions, result *RelistAllResult, pageSize int) (*RelistAllResult, error) {
	pageNo := 1

	for {
		// 获取当前页的已下架产品
		s.logger.Infof("获取第 %d 页已下架产品", pageNo)
		resp, err := s.apiClient.GetOfflineProducts(pageNo, pageSize)
		if err != nil {
			s.logger.WithError(err).Errorf("获取第%d页已下架产品失败", pageNo)
			return result, fmt.Errorf("获取第%d页已下架产品失败: %w", pageNo, err)
		}

		if resp == nil || len(resp.Result.SkuList) == 0 {
			s.logger.Info("没有更多已下架产品")
			break
		}

		// 更新总数（第一次获取时）
		if pageNo == 1 {
			result.TotalOfflineCount = resp.Result.Total
			s.logger.Infof("发现总共 %d 个已下架产品", result.TotalOfflineCount)
		}

		s.logger.Infof("第 %d 页获取到 %d 个产品，开始处理", pageNo, len(resp.Result.SkuList))

		// 立即处理当前页的产品
		pageResult, err := s.processPageProducts(resp.Result.SkuList, options)
		if err != nil {
			s.logger.WithError(err).Errorf("处理第%d页产品失败", pageNo)
			return result, fmt.Errorf("处理第%d页产品失败: %w", pageNo, err)
		}

		// 合并结果
		result.ProcessedCount += pageResult.ProcessedCount
		result.SuccessCount += pageResult.SuccessCount
		result.FailCount += pageResult.FailCount
		result.SkippedCount += pageResult.SkippedCount
		result.Results = append(result.Results, pageResult.Results...)

		s.logger.Infof("第 %d 页处理完成: 处理=%d, 成功=%d, 失败=%d, 跳过=%d",
			pageNo, pageResult.ProcessedCount, pageResult.SuccessCount,
			pageResult.FailCount, pageResult.SkippedCount)

		// 显示总体进度
		if result.TotalOfflineCount > 0 {
			progress := float64(result.ProcessedCount) / float64(result.TotalOfflineCount) * 100
			s.logger.Infof("总体进度: %.1f%% (%d/%d)", progress, result.ProcessedCount, result.TotalOfflineCount)
		}

		// 检查是否处理完所有产品
		if result.ProcessedCount >= result.TotalOfflineCount {
			s.logger.Info("所有产品处理完成")
			break
		}

		pageNo++
	}

	s.logger.Infof("流式处理完成: 总下架数=%d, 处理数=%d, 成功=%d, 失败=%d, 跳过=%d",
		result.TotalOfflineCount, result.ProcessedCount, result.SuccessCount, result.FailCount, result.SkippedCount)

	return result, nil
}

// processPageProducts 处理单页产品
func (s *BulkRelistService) processPageProducts(products []OfflineProductItem, options *BulkRelistOptions) (*RelistAllResult, error) {
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
	productInfoMap := make(map[string]*OfflineProductItem)

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
		// 串行处理当前页
		return s.processPageProductsSequential(goodsSkuMap, productInfoMap, options, pageResult)
	} else {
		// 并发处理当前页
		return s.processPageProductsConcurrent(goodsSkuMap, productInfoMap, options, pageResult, maxConcurrency)
	}
}

// processPageProductsSequential 串行处理单页产品
func (s *BulkRelistService) processPageProductsSequential(goodsSkuMap map[string][]string, productInfoMap map[string]*OfflineProductItem, options *BulkRelistOptions, result *RelistAllResult) (*RelistAllResult, error) {
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
		if s.shouldSkipProduct(productInfo, options.SkipConditions) {
			detailResult.Success = false
			detailResult.Skipped = true
			detailResult.Error = s.getSkipReason(productInfo, options.SkipConditions)
			result.SkippedCount++
		} else {
			// 尝试上架 - 针对多SKU商品的特殊处理
			success := false
			var lastError string

			if len(skuIDs) == 1 {
				// 单SKU商品，直接上架
				resp, err := s.apiClient.RelistProduct(goodsID, skuIDs)
				if err != nil {
					lastError = err.Error()
				} else if resp != nil && resp.Result.Result {
					success = true
					s.logger.Infof("✓ 单SKU上架成功: %s", productInfo.GoodsName)
				} else {
					// 单SKU上架也失败，打印详细信息
					s.logger.Warnf("单SKU上架失败的商品详情: %+v", *productInfo)
					lastError = "上架请求成功但结果为false"
				}
			} else {
				// 多SKU商品，先尝试全部一起上架
				s.logger.Infof("尝试批量上架多SKU商品: %s (SKU数量: %d)", productInfo.GoodsName, len(skuIDs))
				resp, err := s.apiClient.RelistProduct(goodsID, skuIDs)
				if err != nil {
					lastError = err.Error()
				} else if resp != nil && resp.Result.Result {
					success = true
					s.logger.Infof("✓ 批量上架成功: %s", productInfo.GoodsName)
				} else {
					// 批量上架失败，尝试逐个SKU上架
					s.logger.Infof("批量上架失败，尝试逐个SKU上架: %s", productInfo.GoodsName)

					// 打印失败商品的详细信息用于调试
					s.logger.Warnf("批量上架失败的商品详情: %+v", *productInfo)

					successCount := 0
					for i, skuID := range skuIDs {
						singleResp, singleErr := s.apiClient.RelistProduct(goodsID, []string{skuID})
						if singleErr != nil {
							s.logger.Warnf("SKU %s 上架失败: %v", skuID, singleErr)
							lastError = fmt.Sprintf("部分SKU上架失败: %v", singleErr)
						} else if singleResp != nil && singleResp.Result.Result {
							successCount++
							s.logger.Infof("✓ SKU %s 上架成功 (%d/%d)", skuID, i+1, len(skuIDs))
						} else {
							s.logger.Warnf("SKU %s 上架结果为false", skuID)
							lastError = fmt.Sprintf("部分SKU上架结果为false")
						}

						// SKU间添加小延迟
						if i < len(skuIDs)-1 && options.DelayBetweenRequests > 0 {
							time.Sleep(time.Duration(options.DelayBetweenRequests/2) * time.Millisecond)
						}
					}

					if successCount > 0 {
						success = true
						if successCount == len(skuIDs) {
							s.logger.Infof("✓ 所有SKU逐个上架成功: %s (%d/%d)", productInfo.GoodsName, successCount, len(skuIDs))
						} else {
							s.logger.Infof("✓ 部分SKU上架成功: %s (%d/%d)", productInfo.GoodsName, successCount, len(skuIDs))
							lastError = fmt.Sprintf("仅 %d/%d 个SKU上架成功", successCount, len(skuIDs))
						}
					} else {
						lastError = "所有SKU上架都失败"
					}
				}
			}

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

		// 添加延迟
		if result.ProcessedCount < len(goodsSkuMap) && options.DelayBetweenRequests > 0 {
			time.Sleep(time.Duration(options.DelayBetweenRequests) * time.Millisecond)
		}
	}

	return result, nil
}

// processPageProductsConcurrent 并发处理单页产品
func (s *BulkRelistService) processPageProductsConcurrent(goodsSkuMap map[string][]string, productInfoMap map[string]*OfflineProductItem, options *BulkRelistOptions, result *RelistAllResult, maxConcurrency int) (*RelistAllResult, error) {
	// 创建工作任务
	type workItem struct {
		goodsID     string
		skuIDs      []string
		productInfo *OfflineProductItem
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
				if s.shouldSkipProduct(work.productInfo, options.SkipConditions) {
					detailResult.Success = false
					detailResult.Skipped = true
					detailResult.Error = s.getSkipReason(work.productInfo, options.SkipConditions)
				} else {
					// 尝试上架 - 针对多SKU商品的特殊处理
					success := false
					var lastError string

					if len(work.skuIDs) == 1 {
						// 单SKU商品，直接上架
						resp, err := s.apiClient.RelistProduct(work.goodsID, work.skuIDs)
						if err != nil {
							lastError = err.Error()
						} else if resp != nil && resp.Result.Result {
							success = true
							s.logger.Infof("✓ 单SKU上架成功: %s", work.productInfo.GoodsName)
						} else {
							lastError = "上架请求成功但结果为false"
						}
					} else {
						// 多SKU商品，先尝试全部一起上架
						resp, err := s.apiClient.RelistProduct(work.goodsID, work.skuIDs)
						if err != nil {
							lastError = err.Error()
						} else if resp != nil && resp.Result.Result {
							success = true
							s.logger.Infof("✓ 批量上架成功: %s", work.productInfo.GoodsName)
						} else {
							// 批量上架失败，尝试逐个SKU上架
							s.logger.Warnf("并发批量上架失败的商品详情: %+v", *work.productInfo)

							successCount := 0
							for _, skuID := range work.skuIDs {
								singleResp, singleErr := s.apiClient.RelistProduct(work.goodsID, []string{skuID})
								if singleErr != nil {
									lastError = fmt.Sprintf("部分SKU上架失败: %v", singleErr)
								} else if singleResp != nil && singleResp.Result.Result {
									successCount++
								} else {
									lastError = "部分SKU上架结果为false"
								}

								// SKU间添加小延迟
								if options.DelayBetweenRequests > 0 {
									time.Sleep(time.Duration(options.DelayBetweenRequests/3) * time.Millisecond)
								}
							}

							if successCount > 0 {
								success = true
								if successCount < len(work.skuIDs) {
									lastError = fmt.Sprintf("仅 %d/%d 个SKU上架成功", successCount, len(work.skuIDs))
								} else {
									s.logger.Infof("✓ 所有SKU逐个上架成功: %s", work.productInfo.GoodsName)
								}
							} else {
								lastError = "所有SKU上架都失败"
							}
						}
					}

					// 设置结果
					if success {
						detailResult.Success = true
					} else {
						detailResult.Success = false
						detailResult.Error = lastError
					}
				}

				resultChan <- detailResult

				// 并发时的延迟控制
				if options.DelayBetweenRequests > 0 {
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

// RelistOfflineProductsWithFilter 根据过滤条件重新上架已下架产品（流式处理）
func (s *BulkRelistService) RelistOfflineProductsWithFilter(filter *ProductFilter, options *BulkRelistOptions) (*RelistAllResult, error) {
	s.logger.Info("根据过滤条件重新上架已下架产品（流式处理）")

	// 使用默认选项
	if options == nil {
		options = &BulkRelistOptions{
			DelayBetweenRequests: 1000,
			SkipConditions: &SkipConditions{
				SkipNeedRectification: true,
				SkipSeverelyPunished:  true,
				SkipLocked:            true,
			},
		}
	}

	result := &RelistAllResult{
		TotalOfflineCount: 0,
		ProcessedCount:    0,
		SuccessCount:      0,
		FailCount:         0,
		SkippedCount:      0,
		Results:           make([]RelistDetailResult, 0),
	}

	pageNo := 1
	pageSize := 200

	s.logger.Info("开始流式处理：获取一页，筛选并处理一页")

	for {
		// 获取当前页的已下架产品
		s.logger.Infof("获取第 %d 页已下架产品", pageNo)
		resp, err := s.apiClient.GetOfflineProducts(pageNo, pageSize)
		if err != nil {
			s.logger.WithError(err).Errorf("获取第%d页已下架产品失败", pageNo)
			return result, fmt.Errorf("获取第%d页已下架产品失败: %w", pageNo, err)
		}

		if resp == nil || len(resp.Result.SkuList) == 0 {
			s.logger.Info("没有更多已下架产品")
			break
		}

		// 更新总数（第一次获取时）
		if pageNo == 1 {
			result.TotalOfflineCount = resp.Result.Total
			s.logger.Infof("发现总共 %d 个已下架产品", result.TotalOfflineCount)
		}

		// 根据过滤条件筛选当前页产品
		var filteredProducts []OfflineProductItem
		for _, product := range resp.Result.SkuList {
			if s.matchesFilter(&product, filter) {
				filteredProducts = append(filteredProducts, product)
			}
		}

		s.logger.Infof("第 %d 页获取到 %d 个产品，筛选后 %d 个产品，开始处理",
			pageNo, len(resp.Result.SkuList), len(filteredProducts))

		if len(filteredProducts) > 0 {
			// 立即处理筛选后的产品
			pageResult, err := s.processPageProducts(filteredProducts, options)
			if err != nil {
				s.logger.WithError(err).Errorf("处理第%d页产品失败", pageNo)
				return result, fmt.Errorf("处理第%d页产品失败: %w", pageNo, err)
			}

			// 合并结果
			result.ProcessedCount += pageResult.ProcessedCount
			result.SuccessCount += pageResult.SuccessCount
			result.FailCount += pageResult.FailCount
			result.SkippedCount += pageResult.SkippedCount
			result.Results = append(result.Results, pageResult.Results...)

			s.logger.Infof("第 %d 页处理完成: 处理=%d, 成功=%d, 失败=%d, 跳过=%d",
				pageNo, pageResult.ProcessedCount, pageResult.SuccessCount,
				pageResult.FailCount, pageResult.SkippedCount)
		}

		// 显示总体进度
		if result.TotalOfflineCount > 0 {
			progress := float64((pageNo * pageSize)) / float64(result.TotalOfflineCount) * 100
			if progress > 100 {
				progress = 100
			}
			s.logger.Infof("总体进度: %.1f%% (已处理页数: %d)", progress, pageNo)
		}

		// 检查是否已经处理完所有页面
		if len(resp.Result.SkuList) < pageSize {
			s.logger.Info("已处理完所有页面")
			break
		}

		pageNo++
	}

	s.logger.Infof("流式处理完成: 总下架数=%d, 处理数=%d, 成功=%d, 失败=%d, 跳过=%d",
		result.TotalOfflineCount, result.ProcessedCount, result.SuccessCount, result.FailCount, result.SkippedCount)

	return result, nil
}

// shouldSkipProduct 判断是否应该跳过某个产品
func (s *BulkRelistService) shouldSkipProduct(product *OfflineProductItem, conditions *SkipConditions) bool {
	if conditions == nil {
		return false
	}

	// 检查是否需要整改
	if conditions.SkipNeedRectification && product.CategoryRectificationInfo.NeedRectification {
		return true
	}

	// 检查是否被严重惩罚
	if conditions.SkipSeverelyPunished && product.PunishTags > 1 {
		return true
	}

	// 检查锁定状态 - 修正锁定状态判断逻辑
	if conditions.SkipLocked {
		// 检查是否允许上架操作
		if !product.LockInfo.CloseListingMMS.AllowOperate {
			s.logger.Debugf("商品 %s 被锁定: AllowOperate=false", product.GoodsName)
			return true
		}
	}

	// 检查库存
	if conditions.SkipNoStock && product.Stock <= 0 {
		return true
	}

	// 检查最小库存
	if conditions.MinStock > 0 && product.Stock < conditions.MinStock {
		return true
	}

	return false
}

// getSkipReason 获取跳过原因
func (s *BulkRelistService) getSkipReason(product *OfflineProductItem, conditions *SkipConditions) string {
	if conditions == nil {
		return "未知原因"
	}

	if conditions.SkipNeedRectification && product.CategoryRectificationInfo.NeedRectification {
		return "商品需要分类整改"
	}

	if conditions.SkipSeverelyPunished && product.PunishTags > 1 {
		return "商品被严重惩罚"
	}

	if conditions.SkipLocked && !product.LockInfo.CloseListingMMS.AllowOperate {
		return "商品被锁定，不允许上架操作"
	}

	if conditions.SkipNoStock && product.Stock <= 0 {
		return "商品无库存"
	}

	if conditions.MinStock > 0 && product.Stock < conditions.MinStock {
		return fmt.Sprintf("商品库存(%d)低于最小要求(%d)", product.Stock, conditions.MinStock)
	}

	return "未知原因"
}

// matchesFilter 检查产品是否匹配过滤条件
func (s *BulkRelistService) matchesFilter(product *OfflineProductItem, filter *ProductFilter) bool {
	if filter == nil {
		return true
	}

	// 检查分类
	if len(filter.IncludeCategories) > 0 {
		found := false
		for _, category := range filter.IncludeCategories {
			if slices.Contains(product.CatNameList, category) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查排除分类
	if len(filter.ExcludeCategories) > 0 {
		for _, category := range filter.ExcludeCategories {
			if slices.Contains(product.CatNameList, category) {
				return false
			}
		}
	}

	// 检查商品名称关键词
	if len(filter.NameKeywords) > 0 {
		found := false
		for _, keyword := range filter.NameKeywords {
			if len(keyword) > 0 && contains(product.GoodsName, keyword) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查库存范围
	if filter.MinStock > 0 && product.Stock < filter.MinStock {
		return false
	}

	if filter.MaxStock > 0 && product.Stock > filter.MaxStock {
		return false
	}

	// 检查价格范围
	if filter.MinPrice > 0 && product.Price < filter.MinPrice {
		return false
	}

	if filter.MaxPrice > 0 && product.Price > filter.MaxPrice {
		return false
	}

	return true
}

// contains 检查字符串是否包含子字符串（简单实现）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(substr) > 0 && findSubstring(s, substr)))
}

// findSubstring 查找子字符串
func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
