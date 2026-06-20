// Package productsync 提供 SHEIN 平台商品同步功能
package productsync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/pkg/types"
	shein_product "task-processor/internal/shein/api/product"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// ProductSyncService 产品同步服务接口
type ProductSyncService interface {
	// FetchProductList 获取产品列表
	FetchProductList(ctx context.Context) ([]shein_product.ProductListItem, error)

	// ConvertProducts 转换产品格式
	ConvertProducts(ctx context.Context, products []shein_product.ProductListItem, tenantID, storeID int64) ([]*ProductSnapshot, error)

	// SaveProducts 保存产品到管理系统
	SaveProducts(ctx context.Context, productDataList []*ProductSnapshot) (int, error)
}

// productSyncServiceImpl 产品同步服务实现
type productSyncServiceImpl struct {
	productAPI       shein_product.ProductAPI
	inventoryManager *shein_product.InventoryManager
	priceManager     *shein_product.PriceManager
	storeService     listingruntime.StoreService
	storeRepo        productSyncStoreFinder
	mappingRepo      productSyncMappingFinder
	productDataRepo  listingadmin.ProductDataRepository
	logger           *logrus.Entry
}

type productSyncStoreFinder interface {
	GetStore(ctx context.Context, tenantID, id int64) (*listingadmin.Store, error)
}

type productSyncMappingFinder interface {
	FindLatest(ctx context.Context, query listingadmin.ProductImportMappingQuery) (*listingadmin.ProductImportMapping, error)
}

// NewProductSyncService 创建产品同步服务
func NewProductSyncService(
	productAPI shein_product.ProductAPI,
	inventoryManager *shein_product.InventoryManager,
	priceManager *shein_product.PriceManager,
	storeService listingruntime.StoreService,
	storeRepo productSyncStoreFinder,
	mappingRepo productSyncMappingFinder,
	productDataRepo listingadmin.ProductDataRepository,
) ProductSyncService {
	service := &productSyncServiceImpl{
		productAPI:       productAPI,
		inventoryManager: inventoryManager,
		priceManager:     priceManager,
		storeService:     storeService,
		storeRepo:        storeRepo,
		mappingRepo:      mappingRepo,
		productDataRepo:  productDataRepo,
		logger:           logger.GetGlobalLogger("ProductSyncService"),
	}

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
func (s *productSyncServiceImpl) ConvertProducts(ctx context.Context, products []shein_product.ProductListItem, tenantID, storeID int64) ([]*ProductSnapshot, error) {
	totalCount := len(products)
	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"store_id":  storeID,
		"count":     totalCount,
	}).Info("开始转换产品格式")

	productDataList := make([]*ProductSnapshot, 0, totalCount)

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
func (s *productSyncServiceImpl) convertSingleProduct(sheinProduct *shein_product.ProductListItem, tenantID, storeID int64) *ProductSnapshot {
	// 获取店铺信息
	storeInfo, err := s.getStoreInfo(context.Background(), tenantID, storeID)
	if err != nil {
		s.logger.WithError(err).WithField("store_id", storeID).Warn("获取店铺信息失败，使用默认处理")
	}
	productData := s.buildBaseProductData(sheinProduct, defaultSheinStoreInfo(storeInfo, tenantID, storeID))

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
func (s *productSyncServiceImpl) buildBaseProductData(sheinProduct *shein_product.ProductListItem, storeInfo *listingruntime.StoreInfo) *ProductSnapshot {
	storeInfo = defaultSheinStoreInfo(storeInfo, 0, 0)

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

	return &ProductSnapshot{
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

func (s *productSyncServiceImpl) getStoreInfo(ctx context.Context, tenantID, storeID int64) (*listingruntime.StoreInfo, error) {
	if s.storeRepo != nil && tenantID > 0 {
		store, err := s.storeRepo.GetStore(ctx, tenantID, storeID)
		if err == nil && store != nil {
			return sheinRuntimeStoreInfoFromListingStore(store), nil
		}
		if err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"tenant_id": tenantID,
				"store_id":  storeID,
				"path":      "repository",
			}).Warn("从本地仓储获取店铺失败，回退 runtime store service")
		}
	}
	if s.storeService == nil {
		return nil, nil
	}
	return s.storeService.GetStore(storeID)
}

func defaultSheinStoreInfo(storeInfo *listingruntime.StoreInfo, tenantID, storeID int64) *listingruntime.StoreInfo {
	if storeInfo != nil {
		if storeInfo.TenantID == 0 {
			storeInfo.TenantID = tenantID
		}
		if storeInfo.ID == 0 {
			storeInfo.ID = storeID
		}
		return storeInfo
	}
	return &listingruntime.StoreInfo{
		ID:       storeID,
		TenantID: tenantID,
		Platform: "SHEIN",
	}
}

func sheinRuntimeStoreInfoFromListingStore(store *listingadmin.Store) *listingruntime.StoreInfo {
	if store == nil {
		return nil
	}
	return &listingruntime.StoreInfo{
		ID:                       store.ID,
		TenantID:                 store.TenantID,
		StoreID:                  store.StoreID,
		Username:                 store.Username,
		Name:                     store.Name,
		ShopType:                 store.ShopType,
		Region:                   store.Region,
		Platform:                 store.Platform,
		LoginURL:                 store.LoginURL,
		Proxy:                    store.Proxy,
		DailyLimit:               store.DailyLimit,
		DailyLimitType:           store.DailyLimitType,
		PriceType:                store.PriceType,
		EnableDraft:              store.EnableDraft,
		EnableAutoListing:        store.EnableAutoListing,
		FixedStockCount:          store.FixedStockCount,
		SkuGenerateStrategy:      store.SKUGenerateStrategy,
		Prefix:                   store.Prefix,
		Suffix:                   store.Suffix,
		EnableBrandAuthorization: store.EnableBrandAuthorization,
		AuthorizedBrandCode:      store.AuthorizedBrandCode,
		AuthorizedBrandName:      store.AuthorizedBrandName,
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
func (s *productSyncServiceImpl) SaveProducts(ctx context.Context, productDataList []*ProductSnapshot) (int, error) {
	totalCount := len(productDataList)
	s.logger.WithField("count", totalCount).Info("开始保存产品")

	if totalCount == 0 {
		s.logger.Info("没有产品需要保存")
		return 0, nil
	}

	if s.productDataRepo == nil {
		return 0, fmt.Errorf("product data repository is not initialized")
	}

	items := make([]listingadmin.ProductData, 0, totalCount)
	for _, productData := range productDataList {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		items = append(items, sheinProductDataFromSnapshot(productData))
	}
	successCount, err := s.productDataRepo.UpsertProductDataBatch(ctx, items)
	if err != nil {
		s.logger.WithError(err).Error("通过产品数据仓储批量保存SHEIN产品失败")
		return 0, fmt.Errorf("通过产品数据仓储批量保存SHEIN产品失败: %w", err)
	}
	s.logger.WithFields(logrus.Fields{
		"total":   totalCount,
		"success": successCount,
		"path":    "repository",
	}).Info("批量保存产品完成")
	return successCount, nil
}
