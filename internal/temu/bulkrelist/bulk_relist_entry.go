package bulkrelist

import (
	"fmt"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/temu/api/client"
	"task-processor/internal/temu/api/inventory"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// BulkRelistEntry 批量重新上架入口
type BulkRelistEntry struct {
	managementClient *management.ClientManager
	logger           *logrus.Entry
}

// NewBulkRelistEntry 创建批量重新上架入口
func NewBulkRelistEntry(managementClient *management.ClientManager) *BulkRelistEntry {
	return &BulkRelistEntry{
		managementClient: managementClient,
		logger:           logger.GetGlobalLogger("BulkRelistEntry"),
	}
}

// ExecuteSimpleRelist 执行简单的全部重新上架
func (e *BulkRelistEntry) ExecuteSimpleRelist(storeID int64) (*RelistAllResult, error) {
	e.logger.Infof("开始执行简单的全部重新上架: storeID=%d", storeID)
	apiClient := client.NewAPIClient(storeID, e.managementClient)
	if apiClient == nil {
		return nil, fmt.Errorf("创建API客户端失败")
	}
	return NewBulkRelistService(apiClient).RelistAllOfflineProducts(nil)
}

// ExecuteCustomRelist 执行自定义配置的重新上架
func (e *BulkRelistEntry) ExecuteCustomRelist(storeID int64, options *BulkRelistOptions) (*RelistAllResult, error) {
	e.logger.Infof("开始执行自定义重新上架: storeID=%d", storeID)
	apiClient := client.NewAPIClient(storeID, e.managementClient)
	if apiClient == nil {
		return nil, fmt.Errorf("创建API客户端失败")
	}
	return NewBulkRelistService(apiClient).RelistAllOfflineProducts(options)
}

// ExecuteFilteredRelist 执行带过滤条件的重新上架
func (e *BulkRelistEntry) ExecuteFilteredRelist(storeID int64, filter *ProductFilterOptions, options *BulkRelistOptions) (*RelistAllResult, error) {
	e.logger.Infof("开始执行过滤重新上架: storeID=%d", storeID)
	apiClient := client.NewAPIClient(storeID, e.managementClient)
	if apiClient == nil {
		return nil, fmt.Errorf("创建API客户端失败")
	}
	return NewBulkRelistService(apiClient).RelistOfflineProductsWithFilter(filter, options)
}

// GetOfflineProductsPreview 获取已下架产品预览（不执行上架）
func (e *BulkRelistEntry) GetOfflineProductsPreview(storeID int64, filter *ProductFilterOptions) (*OfflineProductPreview, error) {
	e.logger.Infof("获取已下架产品预览: storeID=%d", storeID)

	apiClient := client.NewAPIClient(storeID, e.managementClient)
	if apiClient == nil {
		return nil, fmt.Errorf("创建API客户端失败")
	}

	invAPI := inventory.NewAPI(apiClient, apiClient.GetLogger())
	productFilter := NewProductFilter(apiClient.GetLogger())

	var offlineProducts []inventory.Item
	pageNo := 1
	pageSize := 200

	for {
		resp, err := invAPI.SearchOffline(pageNo, pageSize)
		if err != nil {
			return nil, fmt.Errorf("获取已下架产品失败: %w", err)
		}
		if resp == nil || len(resp.Result.SkuList) == 0 {
			break
		}
		offlineProducts = append(offlineProducts, resp.Result.SkuList...)
		if len(resp.Result.SkuList) < pageSize {
			break
		}
		pageNo++
	}

	preview := &OfflineProductPreview{
		TotalOfflineCount: len(offlineProducts),
		Products:          make([]OfflineProductSummary, 0),
	}

	goodsMap := make(map[string]*inventory.Item)
	for i, product := range offlineProducts {
		if filter == nil || productFilter.MatchesFilter(&offlineProducts[i], filter) {
			if _, exists := goodsMap[product.GoodsID]; !exists {
				goodsMap[product.GoodsID] = &offlineProducts[i]
			}
		}
	}

	for _, product := range goodsMap {
		preview.Products = append(preview.Products, OfflineProductSummary{
			GoodsID:           product.GoodsID,
			GoodsName:         product.GoodsName,
			Categories:        product.CatNameList,
			Stock:             product.Stock,
			Price:             product.Price,
			Currency:          product.Currency,
			NeedRectification: product.CategoryRectificationInfo.NeedRectification,
			PunishTags:        product.PunishTags,
			IsLocked:          !product.LockInfo.CloseListingMMS.AllowOperate,
		})
		preview.FilteredCount++
	}

	e.logger.Infof("预览完成: 总下架数=%d, 符合条件数=%d", preview.TotalOfflineCount, preview.FilteredCount)
	return preview, nil
}

// QuickRelistByCategories 快速按分类重新上架
func (e *BulkRelistEntry) QuickRelistByCategories(storeID int64, categories []string, delayMs int) (*RelistAllResult, error) {
	return e.ExecuteFilteredRelist(storeID, &ProductFilterOptions{
		IncludeCategories: categories,
	}, &BulkRelistOptions{
		DelayBetweenRequests: delayMs,
		SkipConditions:       &SkipConditions{SkipNeedRectification: true, SkipSeverelyPunished: true, SkipLocked: true},
	})
}

// QuickRelistByStock 快速按库存条件重新上架
func (e *BulkRelistEntry) QuickRelistByStock(storeID int64, minStock int, delayMs int) (*RelistAllResult, error) {
	return e.ExecuteFilteredRelist(storeID, &ProductFilterOptions{
		MinStock: minStock,
	}, &BulkRelistOptions{
		DelayBetweenRequests: delayMs,
		SkipConditions:       &SkipConditions{SkipNeedRectification: true, SkipSeverelyPunished: true, SkipLocked: true, SkipNoStock: true},
	})
}

// QuickRelistByPriceRange 快速按价格范围重新上架
func (e *BulkRelistEntry) QuickRelistByPriceRange(storeID int64, minPrice, maxPrice float64, delayMs int) (*RelistAllResult, error) {
	return e.ExecuteFilteredRelist(storeID, &ProductFilterOptions{
		MinPrice: minPrice,
		MaxPrice: maxPrice,
	}, &BulkRelistOptions{
		DelayBetweenRequests: delayMs,
		SkipConditions:       &SkipConditions{SkipNeedRectification: true, SkipSeverelyPunished: true, SkipLocked: true},
	})
}
