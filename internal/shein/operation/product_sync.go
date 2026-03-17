// Package operation 提供SHEIN平台调度器相关服务
package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/clients/management/api"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/types"
	shein_product "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/mapping"

	"github.com/sirupsen/logrus"
)

// ProductSyncService 产品同步服务接口
type ProductSyncService interface {
	// FetchProductList 获取产品列表
	FetchProductList(ctx context.Context) ([]shein_product.ProductListItem, error)

	// ConvertProducts 转换产品格式
	ConvertProducts(ctx context.Context, products []shein_product.ProductListItem, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error)

	// SaveProducts 保存产品到管理系统
	SaveProducts(ctx context.Context, productDataList []*managementapi.ProductDataDTO) (int, error)
}

// productSyncServiceImpl 产品同步服务实现
type productSyncServiceImpl struct {
	managementClient *management.ClientManager
	productAPI       shein_product.ProductAPI
	inventoryManager *shein_product.InventoryManager
	priceManager     *shein_product.PriceManager
	mappingClient    managementapi.ProductImportMappingAPI
	storeAPI         managementapi.StoreAPI
	repairService    mapping.MappingRepairService // 新增修复服务
	logger           *logrus.Entry
}

// NewProductSyncService 创建产品同步服务
func NewProductSyncService(
	managementClient *management.ClientManager,
	productAPI shein_product.ProductAPI,
	inventoryManager *shein_product.InventoryManager,
	priceManager *shein_product.PriceManager,
	mappingClient managementapi.ProductImportMappingAPI,
	storeAPI managementapi.StoreAPI,
) ProductSyncService {
	service := &productSyncServiceImpl{
		managementClient: managementClient,
		productAPI:       productAPI,
		inventoryManager: inventoryManager,
		priceManager:     priceManager,
		mappingClient:    mappingClient,
		storeAPI:         storeAPI,
		logger:           logrus.WithField("component", "ProductSyncService"),
	}

	// 创建修复服务
	service.repairService = mapping.NewMappingRepairService(
		mappingClient,
		storeAPI,
		productAPI,
		mapping.DefaultMappingRepairConfig(),
	)

	return service
}

// FetchProductList 获取产品列表
func (s *productSyncServiceImpl) FetchProductList(ctx context.Context) ([]shein_product.ProductListItem, error) {
	s.logger.Debug("开始获取产品列表")

	var allProducts []shein_product.ProductListItem
	pageNum := 1
	const pageSize = 100

	for {
		req := &shein_product.ProductListRequest{
			Language:             "en",
			OnlyRecommendResell:  false,
			OnlySpmbCopyProduct:  false,
			SearchAbandonProduct: false,
			SearchIllegal:        false,
			SearchLessInventory:  false,
			ShelfType:            "",
			SortType:             0,
		}

		response, err := s.productAPI.ListProducts(pageNum, pageSize, req)
		if err != nil {
			s.logger.Errorf("获取产品列表失败(页面%d): %v", pageNum, err)
			return nil, fmt.Errorf("获取产品列表失败: %w", err)
		}

		s.logger.Debugf("页面%d获取到%d个产品", pageNum, len(response.Info.Data))
		allProducts = append(allProducts, response.Info.Data...)

		if len(response.Info.Data) < pageSize {
			break
		}
		pageNum++
	}

	s.logger.Infof("获取产品列表完成，共%d个产品", len(allProducts))
	return allProducts, nil
}

// ConvertProducts 转换产品格式
func (s *productSyncServiceImpl) ConvertProducts(ctx context.Context, products []shein_product.ProductListItem, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error) {
	totalCount := len(products)
	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"store_id":  storeID,
		"count":     totalCount,
	}).Info("开始转换产品格式")

	productDataList := make([]*managementapi.ProductDataDTO, 0, totalCount)

	// 每处理10%或每10个产品输出一次进度
	progressInterval := totalCount / 10
	if progressInterval < 10 {
		progressInterval = 10
	}
	if progressInterval > 100 {
		progressInterval = 100
	}

	for i, sheinProduct := range products {
		productData := s.convertSingleProduct(&sheinProduct, tenantID, storeID)
		if productData != nil {
			productDataList = append(productDataList, productData)
		}

		// 输出进度日志
		currentIndex := i + 1
		if currentIndex%progressInterval == 0 || currentIndex == totalCount {
			progress := float64(currentIndex) / float64(totalCount) * 100
			s.logger.WithFields(logrus.Fields{
				"processed": currentIndex,
				"total":     totalCount,
				"progress":  fmt.Sprintf("%.1f%%", progress),
				"success":   len(productDataList),
			}).Infof("产品转换进度: %d/%d (%.1f%%)", currentIndex, totalCount, progress)
		}
	}

	s.logger.WithFields(logrus.Fields{
		"total":   totalCount,
		"success": len(productDataList),
		"failed":  totalCount - len(productDataList),
	}).Info("转换产品格式完成")
	return productDataList, nil
}

// convertSingleProduct 转换单个产品
func (s *productSyncServiceImpl) convertSingleProduct(sheinProduct *shein_product.ProductListItem, tenantID, storeID int64) *managementapi.ProductDataDTO {
	// 获取店铺信息
	storeInfo, err := s.storeAPI.GetStore(storeID)
	if err != nil {
		s.logger.WithError(err).WithField("store_id", storeID).Warn("获取店铺信息失败，使用默认处理")
	}
	productData := s.buildBaseProductData(sheinProduct, storeInfo)

	// 获取库存信息（所有店铺类型都需要）
	inventoryInfo, err := s.fetchInventoryInfo(sheinProduct.SpuName)
	if err != nil {
		s.logger.WithError(err).WithField("spu_name", sheinProduct.SpuName).Warn("获取库存信息失败")
	} else if inventoryInfo != nil {
		s.fillProductLevelInventory(productData, inventoryInfo)
	}

	var priceMap map[string]*shein_product.SkuPriceInfo
	var costMap map[string]*shein_product.SkuCostInfo

	// 根据店铺类型决定价格处理策略
	shopType := ""
	if storeInfo != nil {
		shopType = storeInfo.ShopType
	}

	switch shopType {
	case "0":
		// 半托管店铺：只查询成本价
		costMap, err = s.fetchCostPriceInfo(sheinProduct)
		if err != nil {
			s.logger.WithError(err).WithField("spu_name", sheinProduct.SpuName).Warn("获取成本价信息失败")
		}
		s.logger.WithFields(logrus.Fields{
			"spu_name":  sheinProduct.SpuName,
			"shop_type": shopType,
		}).Debug("半托管店铺，查询成本价")

	case "2":
		// 自营店铺：只查询价格
		priceMap, err = s.fetchPriceInfo(sheinProduct.SpuName)
		if err != nil {
			s.logger.WithError(err).WithField("spu_name", sheinProduct.SpuName).Warn("获取价格信息失败")
		}
		s.logger.WithFields(logrus.Fields{
			"spu_name":  sheinProduct.SpuName,
			"shop_type": shopType,
		}).Debug("自营店铺，查询价格")

	case "1":
		// 全托管店铺：暂不处理价格
		s.logger.WithFields(logrus.Fields{
			"spu_name":  sheinProduct.SpuName,
			"shop_type": shopType,
		}).Debug("全托管店铺，暂不处理价格")

	default:
		s.logger.WithFields(logrus.Fields{
			"spu_name":  sheinProduct.SpuName,
			"shop_type": shopType,
		}).Warn("未知的店铺类型，使用默认处理")
	}

	s.fillProductLevelPrice(productData, priceMap, costMap)
	s.enrichProductWithMappingBySku(productData, sheinProduct, tenantID, storeID, inventoryInfo, priceMap, costMap)

	return productData
}

// buildBaseProductData 构建基础产品数据
func (s *productSyncServiceImpl) buildBaseProductData(sheinProduct *shein_product.ProductListItem, storeInfo *api.StoreRespDTO) *managementapi.ProductDataDTO {
	var publishTime *time.Time
	if sheinProduct.PublishTime != "" {
		if t, err := s.parseTime(sheinProduct.PublishTime); err == nil {
			publishTime = t
		}
	}

	var shelfTime *time.Time
	if sheinProduct.FirstShelfTime != "" {
		if t, err := s.parseTime(sheinProduct.FirstShelfTime); err == nil {
			shelfTime = t
		}
	}

	mainImageURL := ""
	if len(sheinProduct.SkcInfoList) > 0 {
		mainImageURL = sheinProduct.SkcInfoList[0].MainImageThumbnailURL
	}

	imageURLs := make([]string, 0, len(sheinProduct.SkcInfoList))
	for _, skc := range sheinProduct.SkcInfoList {
		if skc.MainImageThumbnailURL != "" {
			imageURLs = append(imageURLs, skc.MainImageThumbnailURL)
		}
	}
	imageURLsJSON, _ := json.Marshal(imageURLs)

	platformStatusJSON, _ := json.Marshal(map[string]any{
		"shelf_status": sheinProduct.ShelfStatus,
	})

	return &managementapi.ProductDataDTO{
		TenantID:          storeInfo.TenantID,
		StoreID:           storeInfo.ID,
		Region:            storeInfo.Region,
		Platform:          "SHEIN",
		PlatformProductID: sheinProduct.SpuCode,
		Title:             sheinProduct.ProductNameMulti,
		CategoryID:        sheinProduct.CategoryID,
		Brand:             sheinProduct.BrandName,
		MainImageURL:      mainImageURL,
		ImageURLs:         string(imageURLsJSON),
		PlatformStatus:    string(platformStatusJSON),
		ShelfStatus:       s.mapShelfStatus(sheinProduct.ShelfStatus),
		PublishTime:       types.ToFlexibleTime(publishTime),
		ShelfTime:         types.ToFlexibleTime(shelfTime),
	}
}

// mapShelfStatus 映射上架状态
func (s *productSyncServiceImpl) mapShelfStatus(status string) int {
	switch status {
	case "ON_SHELF":
		return 2
	case "OFF_SHELF":
		return 3
	default:
		return 0
	}
}

// parseTime 解析时间字符串
func (s *productSyncServiceImpl) parseTime(timeStr string) (*time.Time, error) {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("无法解析时间: %s", timeStr)
}

// SaveProducts 保存产品到管理系统
func (s *productSyncServiceImpl) SaveProducts(ctx context.Context, productDataList []*managementapi.ProductDataDTO) (int, error) {
	totalCount := len(productDataList)
	s.logger.WithField("count", totalCount).Info("开始保存产品")

	if totalCount == 0 {
		s.logger.Info("没有产品需要保存")
		return 0, nil
	}

	firstProduct := productDataList[0]
	productDataAPI := s.managementClient.GetProductDataClient(firstProduct.StoreID)

	// 转换为批量请求格式
	products := make([]managementapi.ProductDataItemDTO, 0, totalCount)
	for _, productData := range productDataList {
		item := managementapi.ProductDataItemDTO{
			PlatformProductID:  productData.PlatformProductID,
			ProductName:        productData.Title,
			ProductSku:         productData.ProductID,
			ProductPrice:       productData.OriginalPrice,
			ProductStock:       productData.Stock, // 默认库存，需要根据实际情况设置
			ProductCategory:    productData.Category,
			ProductImage:       productData.MainImageURL,
			ProductDescription: productData.Description,
			ShelfStatus:        &productData.ShelfStatus,
			PublishTime:        productData.PublishTime,
			ShelfTime:          productData.ShelfTime,
			Brand:              productData.Brand,
			CategoryID:         &productData.CategoryID,
			SpecialPrice:       productData.SpecialPrice,
			PriceCurrency:      productData.PriceCurrency,
			ImageUrls:          productData.ImageURLs,
			Attributes:         productData.Attributes,
			PlatformStatus:     productData.PlatformStatus,
			PlatformData:       productData.PlatformData,
			ParentProductID:    productData.ParentProductID,
			CreateTime:         productData.CreateTime,
			UpdateTime:         productData.UpdateTime,
		}
		products = append(products, item)
	}

	// 构建批量请求
	batchReq := &managementapi.ProductDataBatchSaveReqDTO{
		Platform: firstProduct.Platform,
		TenantID: firstProduct.TenantID,
		Region:   firstProduct.Region,
		StoreID:  firstProduct.StoreID,
		Products: products,
	}

	// 执行批量保存
	successCount, err := productDataAPI.BatchCreateOrUpdate(batchReq)
	if err != nil {
		s.logger.WithError(err).Error("批量保存产品失败")
		return 0, fmt.Errorf("批量保存产品失败: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"total":   totalCount,
		"success": successCount,
	}).Info("批量保存产品完成")

	return successCount, nil
}
