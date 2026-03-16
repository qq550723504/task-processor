package bulkrelist

import (
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/inventory"

	"github.com/sirupsen/logrus"
)

// BulkRelistService 批量重新上架服务
type BulkRelistService struct {
	apiClient      client.ClientAPI
	inventoryAPI   *inventory.API
	batchProcessor *BatchProcessor
	productFilter  *ProductFilter
	processor      *ProductProcessor
	pageLoop       *PageLoopProcessor
	logger         *logrus.Entry
}

// NewBulkRelistService 创建批量重新上架服务
func NewBulkRelistService(apiClient client.ClientAPI) *BulkRelistService {
	logger := apiClient.GetLogger()
	inventoryAPI := inventory.NewAPI(apiClient, logger)

	// 创建过滤器
	productFilter := NewProductFilter(logger)

	// 创建处理器
	processor := NewProductProcessor(inventoryAPI, productFilter, logger)

	// 创建批量处理器
	batchProcessor := NewBatchProcessor(inventoryAPI, logger)

	// 创建页面循环处理器
	pageLoop := NewPageLoopProcessor(inventoryAPI, processor, logger)

	return &BulkRelistService{
		apiClient:      apiClient,
		inventoryAPI:   inventoryAPI,
		batchProcessor: batchProcessor,
		productFilter:  productFilter,
		processor:      processor,
		pageLoop:       pageLoop,
		logger:         logger,
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
		return s.pageLoop.ProcessFirstPageLoop(options, result, pageSize)
	}

	s.logger.Info("开始批量获取模式：先获取所有商品ID，再逐个处理")
	return s.processBatchMode(options, result, pageSize)
}

// processBatchMode 批量获取模式：先获取所有商品ID，再逐个处理
func (s *BulkRelistService) processBatchMode(options *BulkRelistOptions, result *RelistAllResult, pageSize int) (*RelistAllResult, error) {
	// 第一步：获取所有下架商品的完整信息
	allProducts, totalExpected, err := s.batchProcessor.FetchAllProducts(pageSize)
	if err != nil {
		return result, err
	}

	result.TotalOfflineCount = totalExpected

	// 第二步：处理商品
	if len(allProducts) == 0 {
		s.logger.Info("没有需要处理的商品")
		return result, nil
	}

	// 去重商品（基于SkuID）
	deduplicatedProducts := s.batchProcessor.DeduplicateProducts(allProducts)

	// 分批处理商品
	batchResult, err := s.batchProcessor.ProcessInBatches(deduplicatedProducts, pageSize, options, s.processor.ProcessProducts)
	if err != nil {
		return result, err
	}

	s.logger.Infof("批量处理完成: 总商品数=%d, 处理数=%d, 成功=%d, 失败=%d, 跳过=%d",
		len(deduplicatedProducts), batchResult.ProcessedCount, batchResult.SuccessCount, batchResult.FailCount, batchResult.SkippedCount)

	return batchResult, nil
}

// RelistOfflineProductsWithFilter 根据过滤条件重新上架已下架产品（流式处理）
func (s *BulkRelistService) RelistOfflineProductsWithFilter(filter *ProductFilterOptions, options *BulkRelistOptions) (*RelistAllResult, error) {
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

	pageSize := 200

	return s.pageLoop.ProcessWithFilter(filter, options, result, pageSize, s.productFilter)
}
