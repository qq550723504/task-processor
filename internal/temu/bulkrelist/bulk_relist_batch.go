package bulkrelist

import (
	"time"

	"task-processor/internal/temu/api/inventory"

	"github.com/sirupsen/logrus"
)

// OfflineAPI 离线API接口
type OfflineAPI interface {
	SearchOffline(pageNo, pageSize int) (*inventory.SearchResponse, error)
}

// BatchProcessor 批量获取处理器
type BatchProcessor struct {
	offlineAPI OfflineAPI
	logger     *logrus.Entry
}

// NewBatchProcessor 创建批量获取处理器
func NewBatchProcessor(offlineAPI OfflineAPI, logger *logrus.Entry) *BatchProcessor {
	return &BatchProcessor{
		offlineAPI: offlineAPI,
		logger:     logger,
	}
}

// FetchAllProducts 批量获取所有下架商品信息
func (bp *BatchProcessor) FetchAllProducts(pageSize int) ([]inventory.Item, int, error) {
	bp.logger.Info("开始批量获取所有下架商品信息")

	var allProducts []inventory.Item
	pageNo := 1
	totalExpected := 0

	for {
		bp.logger.Infof("获取第 %d 页商品信息", pageNo)
		resp, err := bp.offlineAPI.SearchOffline(pageNo, pageSize)
		if err != nil {
			bp.logger.WithError(err).Errorf("获取第%d页已下架产品失败，跳过此页继续处理", pageNo)
			pageNo++
			continue
		}

		if resp == nil {
			bp.logger.Warnf("第 %d 页返回空响应，跳过", pageNo)
			pageNo++
			continue
		}

		if len(resp.Result.SkuList) == 0 {
			bp.logger.Infof("第 %d 页没有商品，结束获取", pageNo)
			break
		}

		// 更新总数（第一次获取时）
		if pageNo == 1 {
			totalExpected = resp.Result.Total
			bp.logger.Infof("API显示总共有 %d 个已下架产品", totalExpected)
		}

		// 收集完整的商品信息
		beforeCount := len(allProducts)
		allProducts = append(allProducts, resp.Result.SkuList...)
		afterCount := len(allProducts)

		bp.logger.Infof("第 %d 页获取到 %d 个商品信息，累计收集 %d 个商品",
			pageNo, len(resp.Result.SkuList), afterCount)

		// 强制显示前几个商品的信息用于调试
		if pageNo <= 2 && len(resp.Result.SkuList) > 0 {
			bp.logger.Infof("第 %d 页前3个商品ID: %v", pageNo, func() []string {
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
			bp.logger.Warnf("第 %d 页商品数量异常：期望增加 %d，实际增加 %d",
				pageNo, len(resp.Result.SkuList), afterCount-beforeCount)
		}

		pageNo++
	}

	bp.logger.Infof("批量获取完成：API显示总数=%d，实际收集=%d，获取了 %d 页",
		totalExpected, len(allProducts), pageNo-1)

	// 如果收集的数量远少于预期，给出警告
	if totalExpected > 0 && len(allProducts) < totalExpected/2 {
		bp.logger.Warnf("警告：收集的商品数量(%d)远少于API显示的总数(%d)，可能存在分页获取问题",
			len(allProducts), totalExpected)
	}

	return allProducts, totalExpected, nil
}

// DeduplicateProducts 去重商品（基于SkuID）
func (bp *BatchProcessor) DeduplicateProducts(products []inventory.Item) []inventory.Item {
	bp.logger.Infof("开始去重处理，原始商品数量: %d", len(products))
	uniqueProducts := make(map[string]inventory.Item)
	duplicateCount := 0

	for _, product := range products {
		if _, exists := uniqueProducts[product.SkuID]; exists {
			duplicateCount++
			bp.logger.Debugf("发现重复SKU ID: %s", product.SkuID)
		}
		uniqueProducts[product.SkuID] = product
	}

	// 转换为切片
	deduplicatedProducts := make([]inventory.Item, 0, len(uniqueProducts))
	for _, product := range uniqueProducts {
		deduplicatedProducts = append(deduplicatedProducts, product)
	}

	bp.logger.Infof("去重完成：原始=%d，重复=%d，去重后=%d",
		len(products), duplicateCount, len(deduplicatedProducts))

	return deduplicatedProducts
}

// ProcessInBatches 分批处理商品
func (bp *BatchProcessor) ProcessInBatches(
	products []inventory.Item,
	batchSize int,
	options *BulkRelistOptions,
	processor func([]inventory.Item, *BulkRelistOptions) (*RelistAllResult, error),
) (*RelistAllResult, error) {
	result := &RelistAllResult{
		TotalOfflineCount: 0,
		ProcessedCount:    0,
		SuccessCount:      0,
		FailCount:         0,
		SkippedCount:      0,
		Results:           make([]RelistDetailResult, 0),
	}

	for i := 0; i < len(products); i += batchSize {
		end := min(i+batchSize, len(products))
		batchProducts := products[i:end]
		batchNo := (i / batchSize) + 1

		bp.logger.Infof("处理第 %d 批商品 (%d-%d/%d)", batchNo, i+1, end, len(products))

		// 处理这一批商品
		batchResult, err := processor(batchProducts, options)
		if err != nil {
			bp.logger.WithError(err).Errorf("处理第%d批商品失败", batchNo)
			continue
		}

		// 合并结果
		result.ProcessedCount += batchResult.ProcessedCount
		result.SuccessCount += batchResult.SuccessCount
		result.FailCount += batchResult.FailCount
		result.SkippedCount += batchResult.SkippedCount
		result.Results = append(result.Results, batchResult.Results...)

		bp.logger.Infof("第 %d 批处理完成: 处理=%d, 成功=%d, 失败=%d, 跳过=%d",
			batchNo, batchResult.ProcessedCount, batchResult.SuccessCount,
			batchResult.FailCount, batchResult.SkippedCount)

		// 显示总体进度
		progress := float64(end) / float64(len(products)) * 100
		bp.logger.Infof("总体进度: %.1f%% (%d/%d)", progress, end, len(products))

		// 批次间延迟
		if i+batchSize < len(products) && options.DelayBetweenRequests > 0 {
			time.Sleep(time.Duration(options.DelayBetweenRequests) * time.Millisecond)
		}
	}

	return result, nil
}
