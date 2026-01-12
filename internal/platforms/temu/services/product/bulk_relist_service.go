package product

import (
	"fmt"
	"slices"
	"sync"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/api/models"
	"time"

	"github.com/sirupsen/logrus"
)

// BulkRelistService 批量重新上架服务
type BulkRelistService struct {
	apiClient  api.APIClientInterface
	offlineAPI *api.OfflineAPI
	listingAPI *api.ListingAPI
	logger     *logrus.Entry
}

// NewBulkRelistService 创建批量重新上架服务
func NewBulkRelistService(apiClient api.APIClientInterface) *BulkRelistService {
	logger := apiClient.GetLogger()

	return &BulkRelistService{
		apiClient:  apiClient,
		offlineAPI: api.NewOfflineAPI(apiClient, logger),
		listingAPI: api.NewListingAPI(apiClient, logger),
		logger:     logger,
	}
}

// RelistAllOfflineProducts 获取所有已下架产品并逐个全部上架
func (s *BulkRelistService) RelistAllOfflineProducts(options *models.BulkRelistOptions) (*models.RelistAllResult, error) {
	s.logger.Info("开始获取所有已下架产品并逐个上架")

	// 使用默认选项
	if options == nil {
		options = &models.BulkRelistOptions{
			DelayBetweenRequests: 1000, // 默认1秒延迟
			SkipConditions: &models.SkipConditions{
				SkipNeedRectification: true,
				SkipSeverelyPunished:  true,
				SkipLocked:            true,
			},
		}
	}

	result := &models.RelistAllResult{
		TotalOfflineCount: 0, // 初始不知道总数
		ProcessedCount:    0,
		SuccessCount:      0,
		FailCount:         0,
		SkippedCount:      0,
		Results:           make([]models.RelistDetailResult, 0),
	}

	pageSize := 200 // 使用最大页面大小

	if options.ProcessFirstPageOnly {
		s.logger.Info("启用循环处理第一页模式")
		return s.processFirstPageLoop(options, result, pageSize)
	}

	s.logger.Info("开始批量获取模式：先获取所有商品ID，再逐个处理")
	return s.processBatchMode(options, result, pageSize)
}

// processFirstPageLoop 循环处理第一页
func (s *BulkRelistService) processFirstPageLoop(options *models.BulkRelistOptions, result *models.RelistAllResult, pageSize int) (*models.RelistAllResult, error) {
	pageNo := 1
	roundCount := 0
	consecutiveNoSuccessRounds := 0 // 连续无成功上架的轮数
	maxNoSuccessRounds := 3         // 最大连续无成功轮数

	for {
		roundCount++
		s.logger.Infof("=== 第 %d 轮处理第一页 ===", roundCount)

		// 获取第一页的已下架产品
		resp, err := s.offlineAPI.GetOfflineProducts(pageNo, pageSize)
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

		// 记录本轮处理前的成功数量
		beforeSuccessCount := result.SuccessCount

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

		// 检查本轮是否有成功上架的商品
		roundSuccessCount := result.SuccessCount - beforeSuccessCount
		if roundSuccessCount == 0 {
			consecutiveNoSuccessRounds++
			s.logger.Warnf("第 %d 轮无成功上架商品，连续无成功轮数: %d/%d",
				roundCount, consecutiveNoSuccessRounds, maxNoSuccessRounds)
		} else {
			consecutiveNoSuccessRounds = 0 // 重置计数器
		}

		s.logger.Infof("第 %d 轮处理完成: 处理=%d, 成功=%d, 失败=%d, 跳过=%d",
			roundCount, pageResult.ProcessedCount, pageResult.SuccessCount,
			pageResult.FailCount, pageResult.SkippedCount)

		// 如果连续多轮都没有成功上架，停止循环
		if consecutiveNoSuccessRounds >= maxNoSuccessRounds {
			s.logger.Warnf("连续 %d 轮无成功上架商品，停止循环处理", consecutiveNoSuccessRounds)
			break
		}

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
func (s *BulkRelistService) processAllPages(options *models.BulkRelistOptions, result *models.RelistAllResult, pageSize int) (*models.RelistAllResult, error) {
	pageNo := 1

	for {
		// 获取当前页的已下架产品
		s.logger.Infof("获取第 %d 页已下架产品", pageNo)
		resp, err := s.offlineAPI.GetOfflineProducts(pageNo, pageSize)
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

// processBatchMode 批量获取模式：先获取所有商品ID，再逐个处理
func (s *BulkRelistService) processBatchMode(options *models.BulkRelistOptions, result *models.RelistAllResult, pageSize int) (*models.RelistAllResult, error) {
	s.logger.Info("开始批量获取所有下架商品信息")

	// 第一步：获取所有下架商品的完整信息
	var allProducts []models.OfflineProductItem
	pageNo := 1
	totalExpected := 0

	for {
		s.logger.Infof("获取第 %d 页商品信息", pageNo)
		resp, err := s.offlineAPI.GetOfflineProducts(pageNo, pageSize)
		if err != nil {
			s.logger.WithError(err).Errorf("获取第%d页已下架产品失败，跳过此页继续处理", pageNo)
			pageNo++
			continue // 跳过失败的页面，继续处理
		}

		if resp == nil {
			s.logger.Warnf("第 %d 页返回空响应，跳过", pageNo)
			pageNo++
			continue
		}

		if len(resp.Result.SkuList) == 0 {
			s.logger.Infof("第 %d 页没有商品，结束获取", pageNo)
			break
		}

		// 更新总数（第一次获取时）
		if pageNo == 1 {
			totalExpected = resp.Result.Total
			result.TotalOfflineCount = totalExpected
			s.logger.Infof("API显示总共有 %d 个已下架产品", totalExpected)
		}

		// 收集完整的商品信息
		beforeCount := len(allProducts)
		allProducts = append(allProducts, resp.Result.SkuList...)
		afterCount := len(allProducts)

		s.logger.Infof("第 %d 页获取到 %d 个商品信息，累计收集 %d 个商品",
			pageNo, len(resp.Result.SkuList), afterCount)

		// 强制显示前几个商品的信息用于调试
		if pageNo <= 2 && len(resp.Result.SkuList) > 0 {
			s.logger.Infof("第 %d 页前3个商品ID: %v", pageNo, func() []string {
				var ids []string
				for i, p := range resp.Result.SkuList {
					if i >= 3 {
						break
					}
					ids = append(ids, p.GoodsID)
				}
				return ids
			}())
		}

		// 检查是否有重复商品
		if afterCount-beforeCount != len(resp.Result.SkuList) {
			s.logger.Warnf("第 %d 页商品数量异常：期望增加 %d，实际增加 %d",
				pageNo, len(resp.Result.SkuList), afterCount-beforeCount)
		}

		pageNo++
	}

	s.logger.Infof("批量获取完成：API显示总数=%d，实际收集=%d，获取了 %d 页",
		totalExpected, len(allProducts), pageNo-1)

	// 如果收集的数量远少于预期，给出警告
	if totalExpected > 0 && len(allProducts) < totalExpected/2 {
		s.logger.Warnf("警告：收集的商品数量(%d)远少于API显示的总数(%d)，可能存在分页获取问题",
			len(allProducts), totalExpected)
	}

	// 第二步：处理商品
	if len(allProducts) == 0 {
		s.logger.Info("没有需要处理的商品")
		return result, nil
	}

	// 去重商品（基于SkuID）
	s.logger.Infof("开始去重处理，原始商品数量: %d", len(allProducts))
	uniqueProducts := make(map[string]models.OfflineProductItem)
	duplicateCount := 0

	for _, product := range allProducts {
		if _, exists := uniqueProducts[product.SkuID]; exists {
			duplicateCount++
			s.logger.Debugf("发现重复SKU ID: %s", product.SkuID)
		}
		uniqueProducts[product.SkuID] = product
	}

	// 转换为切片
	var deduplicatedProducts []models.OfflineProductItem
	for _, product := range uniqueProducts {
		deduplicatedProducts = append(deduplicatedProducts, product)
	}

	s.logger.Infof("去重完成：原始=%d，重复=%d，去重后=%d",
		len(allProducts), duplicateCount, len(deduplicatedProducts))

	// 分批处理商品
	batchSize := pageSize
	for i := 0; i < len(deduplicatedProducts); i += batchSize {
		end := i + batchSize
		if end > len(deduplicatedProducts) {
			end = len(deduplicatedProducts)
		}

		batchProducts := deduplicatedProducts[i:end]
		batchNo := (i / batchSize) + 1

		s.logger.Infof("处理第 %d 批商品 (%d-%d/%d)", batchNo, i+1, end, len(deduplicatedProducts))

		// 处理这一批商品
		batchResult, err := s.processPageProducts(batchProducts, options)
		if err != nil {
			s.logger.WithError(err).Errorf("处理第%d批商品失败", batchNo)
			continue
		}

		// 合并结果
		result.ProcessedCount += batchResult.ProcessedCount
		result.SuccessCount += batchResult.SuccessCount
		result.FailCount += batchResult.FailCount
		result.SkippedCount += batchResult.SkippedCount
		result.Results = append(result.Results, batchResult.Results...)

		s.logger.Infof("第 %d 批处理完成: 处理=%d, 成功=%d, 失败=%d, 跳过=%d",
			batchNo, batchResult.ProcessedCount, batchResult.SuccessCount,
			batchResult.FailCount, batchResult.SkippedCount)

		// 显示总体进度
		progress := float64(end) / float64(len(deduplicatedProducts)) * 100
		s.logger.Infof("总体进度: %.1f%% (%d/%d)", progress, end, len(deduplicatedProducts))

		// 批次间延迟
		if i+batchSize < len(deduplicatedProducts) && options.DelayBetweenRequests > 0 {
			time.Sleep(time.Duration(options.DelayBetweenRequests) * time.Millisecond)
		}
	}

	s.logger.Infof("批量处理完成: 总商品数=%d, 处理数=%d, 成功=%d, 失败=%d, 跳过=%d",
		len(deduplicatedProducts), result.ProcessedCount, result.SuccessCount, result.FailCount, result.SkippedCount)

	return result, nil
}

// processPageProducts 处理单页产品
func (s *BulkRelistService) processPageProducts(products []models.OfflineProductItem, options *models.BulkRelistOptions) (*models.RelistAllResult, error) {
	pageResult := &models.RelistAllResult{
		TotalOfflineCount: len(products),
		ProcessedCount:    0,
		SuccessCount:      0,
		FailCount:         0,
		SkippedCount:      0,
		Results:           make([]models.RelistDetailResult, 0),
	}

	// 按商品ID分组
	goodsSkuMap := make(map[string][]string)
	productInfoMap := make(map[string]*models.OfflineProductItem)

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
func (s *BulkRelistService) processPageProductsSequential(goodsSkuMap map[string][]string, productInfoMap map[string]*models.OfflineProductItem, options *models.BulkRelistOptions, result *models.RelistAllResult) (*models.RelistAllResult, error) {
	for goodsID, skuIDs := range goodsSkuMap {
		result.ProcessedCount++
		productInfo := productInfoMap[goodsID]

		detailResult := models.RelistDetailResult{
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
				resp, err := s.listingAPI.RelistProduct(goodsID, skuIDs)
				if err != nil {
					lastError = err.Error()
				} else if resp != nil && resp.Result.Result {
					success = true
					s.logger.Infof("✓ 单SKU上架成功: %s", productInfo.GoodsName)
				} else {
					// 单SKU上架也失败，打印详细信息
					lastError = "上架请求成功但结果为false"
				}
			} else {
				// 多SKU商品，先尝试全部一起上架
				s.logger.Infof("尝试批量上架多SKU商品: %s (SKU数量: %d)", productInfo.GoodsName, len(skuIDs))
				resp, err := s.listingAPI.RelistProduct(goodsID, skuIDs)
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
						singleResp, singleErr := s.listingAPI.RelistProduct(goodsID, []string{skuID})
						if singleErr != nil {
							s.logger.Warnf("SKU %s 上架失败: %v", skuID, singleErr)
							lastError = fmt.Sprintf("部分SKU上架失败: %v", singleErr)
						} else if singleResp != nil && singleResp.Result.Result {
							successCount++
							s.logger.Infof("✓ SKU %s 上架成功 (%d/%d)", skuID, i+1, len(skuIDs))
						} else {
							s.logger.Warnf("SKU %s 上架结果为false", skuID)
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

		// 添加延迟 - 只有在实际处理商品（非跳过）且不是最后一个商品时才延迟
		if !detailResult.Skipped && result.ProcessedCount < len(goodsSkuMap) && options.DelayBetweenRequests > 0 {
			time.Sleep(time.Duration(options.DelayBetweenRequests) * time.Millisecond)
		}
	}

	return result, nil
}

// processPageProductsConcurrent 并发处理单页产品
func (s *BulkRelistService) processPageProductsConcurrent(goodsSkuMap map[string][]string, productInfoMap map[string]*models.OfflineProductItem, options *models.BulkRelistOptions, result *models.RelistAllResult, maxConcurrency int) (*models.RelistAllResult, error) {
	// 创建工作任务
	type workItem struct {
		goodsID     string
		skuIDs      []string
		productInfo *models.OfflineProductItem
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
	resultChan := make(chan models.RelistDetailResult, len(goodsSkuMap))
	var wg sync.WaitGroup

	// 启动工作协程
	for i := range maxConcurrency {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for work := range workQueue {
				detailResult := models.RelistDetailResult{
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
						resp, err := s.listingAPI.RelistProduct(work.goodsID, work.skuIDs)
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
						resp, err := s.listingAPI.RelistProduct(work.goodsID, work.skuIDs)
						if err != nil {
							lastError = err.Error()
						} else if resp != nil && resp.Result.Result {
							success = true
							s.logger.Infof("✓ 批量上架成功: %s", work.productInfo.GoodsName)
						} else {
							successCount := 0
							for _, skuID := range work.skuIDs {
								singleResp, singleErr := s.listingAPI.RelistProduct(work.goodsID, []string{skuID})
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

// RelistOfflineProductsWithFilter 根据过滤条件重新上架已下架产品（流式处理）
func (s *BulkRelistService) RelistOfflineProductsWithFilter(filter *models.ProductFilter, options *models.BulkRelistOptions) (*models.RelistAllResult, error) {
	s.logger.Info("根据过滤条件重新上架已下架产品（流式处理）")

	// 使用默认选项
	if options == nil {
		options = &models.BulkRelistOptions{
			DelayBetweenRequests: 1000,
			SkipConditions: &models.SkipConditions{
				SkipNeedRectification: true,
				SkipSeverelyPunished:  true,
				SkipLocked:            true,
			},
		}
	}

	result := &models.RelistAllResult{
		TotalOfflineCount: 0,
		ProcessedCount:    0,
		SuccessCount:      0,
		FailCount:         0,
		SkippedCount:      0,
		Results:           make([]models.RelistDetailResult, 0),
	}

	pageNo := 1
	pageSize := 200

	s.logger.Info("开始流式处理：获取一页，筛选并处理一页")

	for {
		// 获取当前页的已下架产品
		s.logger.Infof("获取第 %d 页已下架产品", pageNo)
		resp, err := s.offlineAPI.GetOfflineProducts(pageNo, pageSize)
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
		var filteredProducts []models.OfflineProductItem
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
func (s *BulkRelistService) shouldSkipProduct(product *models.OfflineProductItem, conditions *models.SkipConditions) bool {
	if conditions == nil {
		return false
	}

	// 基于日志分析的核心跳过条件 - 这些是导致上架失败的关键标识

	// 1. 检查惩罚标签 - PunishTags = 1 的商品通常上架失败，PunishTags = 0 的商品通常成功
	if product.PunishTags == 1 {
		s.logger.Debugf("跳过惩罚商品: %s (PunishTags=1)", product.GoodsName)
		return true
	}

	// 2. 检查商品状态异常 - ShowSubStatus4VO = 3001 的商品通常失败，ShowSubStatus4VO = 3002 的商品通常成功
	if product.ShowSubStatus4VO == 3001 {
		s.logger.Debugf("跳过状态异常商品: %s (ShowSubStatus4VO=3001)", product.GoodsName)
		return true
	}

	// 原有的跳过条件保持不变

	// // 检查是否需要整改
	// if conditions.SkipNeedRectification && product.CategoryRectificationInfo.NeedRectification {
	// 	return true
	// }

	// 检查是否被严重惩罚 (PunishTags > 1)
	if conditions.SkipSeverelyPunished && product.PunishTags > 1 {
		return true
	}

	// 检查锁定状态 - 修正锁定状态判断逻辑
	if conditions.SkipLocked {
		// 检查是否允许上架操作
		if product.LockInfo.CloseListingMMS.AllowOperate {
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
func (s *BulkRelistService) getSkipReason(product *models.OfflineProductItem, conditions *models.SkipConditions) string {
	if conditions == nil {
		return "未知原因"
	}

	// 基于日志分析的核心跳过原因 - 优先检查这些导致上架失败的关键标识

	// 1. 惩罚标签检查 - PunishTags = 1 的商品通常失败
	if product.PunishTags == 1 {
		return "商品存在惩罚标签 (PunishTags=1)"
	}

	// 2. 状态异常检查 - ShowSubStatus4VO = 3001 的商品通常失败
	if product.ShowSubStatus4VO == 3001 {
		return "商品状态异常 (ShowSubStatus4VO=3001)"
	}

	// 原有的跳过原因检查

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
func (s *BulkRelistService) matchesFilter(product *models.OfflineProductItem, filter *models.ProductFilter) bool {
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
