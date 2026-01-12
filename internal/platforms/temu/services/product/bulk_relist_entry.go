package product

import (
	"fmt"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

// BulkRelistEntry 批量重新上架入口
type BulkRelistEntry struct {
	managementClient *management.ClientManager
	logger           *logrus.Entry
}

// NewBulkRelistEntry 创建批量重新上架入口
func NewBulkRelistEntry(managementClient *management.ClientManager) *BulkRelistEntry {
	logger := logrus.WithFields(logrus.Fields{
		"component": "BulkRelistEntry",
	})

	return &BulkRelistEntry{
		managementClient: managementClient,
		logger:           logger,
	}
}

// ExecuteSimpleRelist 执行简单的全部重新上架
func (e *BulkRelistEntry) ExecuteSimpleRelist(tenantID, storeID int64) (*models.RelistAllResult, error) {
	e.logger.Infof("开始执行简单的全部重新上架: tenantID=%d, storeID=%d", tenantID, storeID)

	// 创建API客户端
	apiClient := api.NewAPIClient(tenantID, storeID, e.managementClient)
	if apiClient == nil {
		return nil, fmt.Errorf("创建API客户端失败")
	}

	// 创建批量上架服务
	service := NewBulkRelistService(apiClient)

	// 使用默认选项执行上架
	return service.RelistAllOfflineProducts(nil)
}

// ExecuteCustomRelist 执行自定义配置的重新上架
func (e *BulkRelistEntry) ExecuteCustomRelist(tenantID, storeID int64, options *models.BulkRelistOptions) (*models.RelistAllResult, error) {
	e.logger.Infof("开始执行自定义重新上架: tenantID=%d, storeID=%d", tenantID, storeID)

	// 创建API客户端
	apiClient := api.NewAPIClient(tenantID, storeID, e.managementClient)
	if apiClient == nil {
		return nil, fmt.Errorf("创建API客户端失败")
	}

	// 创建批量上架服务
	service := NewBulkRelistService(apiClient)

	// 使用自定义选项执行上架
	return service.RelistAllOfflineProducts(options)
}

// ExecuteFilteredRelist 执行带过滤条件的重新上架
func (e *BulkRelistEntry) ExecuteFilteredRelist(tenantID, storeID int64, filter *models.ProductFilter, options *models.BulkRelistOptions) (*models.RelistAllResult, error) {
	e.logger.Infof("开始执行过滤重新上架: tenantID=%d, storeID=%d", tenantID, storeID)

	// 创建API客户端
	apiClient := api.NewAPIClient(tenantID, storeID, e.managementClient)
	if apiClient == nil {
		return nil, fmt.Errorf("创建API客户端失败")
	}

	// 创建批量上架服务
	service := NewBulkRelistService(apiClient)

	// 使用过滤条件执行上架
	return service.RelistOfflineProductsWithFilter(filter, options)
}

// GetOfflineProductsPreview 获取已下架产品预览（不执行上架）
func (e *BulkRelistEntry) GetOfflineProductsPreview(tenantID, storeID int64, filter *models.ProductFilter) (*models.OfflineProductPreview, error) {
	e.logger.Infof("获取已下架产品预览: tenantID=%d, storeID=%d", tenantID, storeID)

	// 创建API客户端
	apiClient := api.NewAPIClient(tenantID, storeID, e.managementClient)
	if apiClient == nil {
		return nil, fmt.Errorf("创建API客户端失败")
	}

	// 获取所有已下架产品
	offlineAPI := api.NewOfflineAPI(apiClient, apiClient.GetLogger())

	// 分页获取所有下架产品
	var offlineProducts []models.OfflineProductItem
	pageNo := 1
	pageSize := 200

	for {
		resp, err := offlineAPI.GetOfflineProducts(pageNo, pageSize)
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

	preview := &models.OfflineProductPreview{
		TotalOfflineCount: len(offlineProducts),
		Products:          make([]models.OfflineProductSummary, 0),
	}

	// 创建服务用于过滤
	service := NewBulkRelistService(apiClient)

	// 按商品ID分组并应用过滤条件
	goodsMap := make(map[string]*models.OfflineProductItem)
	for _, product := range offlineProducts {
		if filter == nil || service.matchesFilter(&product, filter) {
			if _, exists := goodsMap[product.GoodsID]; !exists {
				goodsMap[product.GoodsID] = &product
			}
		}
	}

	// 构建预览结果
	for _, product := range goodsMap {
		summary := models.OfflineProductSummary{
			GoodsID:           product.GoodsID,
			GoodsName:         product.GoodsName,
			Categories:        product.CatNameList,
			Stock:             product.Stock,
			Price:             product.Price,
			Currency:          product.Currency,
			NeedRectification: product.CategoryRectificationInfo.NeedRectification,
			PunishTags:        product.PunishTags,
			IsLocked:          !product.LockInfo.CloseListingMMS.AllowOperate,
		}
		preview.Products = append(preview.Products, summary)
		preview.FilteredCount++
	}

	e.logger.Infof("预览完成: 总下架数=%d, 符合条件数=%d", preview.TotalOfflineCount, preview.FilteredCount)
	return preview, nil
}

// QuickRelistByCategories 快速按分类重新上架
func (e *BulkRelistEntry) QuickRelistByCategories(tenantID, storeID int64, categories []string, delayMs int) (*models.RelistAllResult, error) {
	filter := &models.ProductFilter{
		IncludeCategories: categories,
	}

	options := &models.BulkRelistOptions{
		DelayBetweenRequests: delayMs,
		SkipConditions: &models.SkipConditions{
			SkipNeedRectification: true,
			SkipSeverelyPunished:  true,
			SkipLocked:            true,
		},
	}

	return e.ExecuteFilteredRelist(tenantID, storeID, filter, options)
}

// QuickRelistByStock 快速按库存条件重新上架
func (e *BulkRelistEntry) QuickRelistByStock(tenantID, storeID int64, minStock int, delayMs int) (*models.RelistAllResult, error) {
	filter := &models.ProductFilter{
		MinStock: minStock,
	}

	options := &models.BulkRelistOptions{
		DelayBetweenRequests: delayMs,
		SkipConditions: &models.SkipConditions{
			SkipNeedRectification: true,
			SkipSeverelyPunished:  true,
			SkipLocked:            true,
			SkipNoStock:           true,
		},
	}

	return e.ExecuteFilteredRelist(tenantID, storeID, filter, options)
}

// QuickRelistByPriceRange 快速按价格范围重新上架
func (e *BulkRelistEntry) QuickRelistByPriceRange(tenantID, storeID int64, minPrice, maxPrice float64, delayMs int) (*models.RelistAllResult, error) {
	filter := &models.ProductFilter{
		MinPrice: minPrice,
		MaxPrice: maxPrice,
	}

	options := &models.BulkRelistOptions{
		DelayBetweenRequests: delayMs,
		SkipConditions: &models.SkipConditions{
			SkipNeedRectification: true,
			SkipSeverelyPunished:  true,
			SkipLocked:            true,
		},
	}

	return e.ExecuteFilteredRelist(tenantID, storeID, filter, options)
}
