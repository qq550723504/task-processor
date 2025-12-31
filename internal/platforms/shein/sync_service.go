// Package shein 提供SHEIN平台产品同步服务
package shein

import (
	"fmt"
	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// SyncService SHEIN 产品同步服务
type SyncService struct {
	repositoryFactory func(storeID, tenantID int64) api.ProductDataAPI
	mappingClient     api.ProductImportMappingAPI
	productFetcher    *ProductFetcher
	dataEnricher      *DataEnricher
	priceManager      *PriceManager
	inventoryManager  *InventoryManager
}

// NewSyncService 创建 SHEIN 同步服务
func NewSyncService(repositoryFactory func(storeID, tenantID int64) api.ProductDataAPI) *SyncService {
	return &SyncService{
		repositoryFactory: repositoryFactory,
		mappingClient:     nil, // 可选，如果需要查询映射关系则设置
		productFetcher:    NewProductFetcher(),
		dataEnricher:      NewDataEnricher(),
		priceManager:      NewPriceManager(),
		inventoryManager:  NewInventoryManager(),
	}
}

// SetMappingClient 设置映射客户端
func (s *SyncService) SetMappingClient(mappingClient api.ProductImportMappingAPI) {
	s.mappingClient = mappingClient
	s.dataEnricher.SetMappingClient(mappingClient)
}

// SyncProducts 同步 SHEIN 产品列表
// shopType: 0-半托管, 1-全托管, 2-自营
func (s *SyncService) SyncProducts(apiClient *ShopAPIClient, tenantID, storeID int64, shopType string) (int, error) {
	logrus.WithFields(logrus.Fields{
		"platform":  "SHEIN",
		"tenant_id": tenantID,
		"store_id":  storeID,
		"shop_type": shopType,
	}).Info("开始同步 SHEIN 产品")

	// 为当前店铺创建专用的 repository
	repository := s.repositoryFactory(storeID, tenantID)

	// 调用 SHEIN 产品列表 API
	products, err := s.productFetcher.FetchProductList(apiClient)
	if err != nil {
		return 0, fmt.Errorf("获取产品列表失败: %w", err)
	}

	logrus.WithField("count", len(products)).Info("获取到产品列表")

	// 转换并保存产品
	successCount := 0
	for _, sheinProduct := range products {
		productData, err := MapToProductData(&sheinProduct, tenantID, storeID)
		if err != nil {
			logrus.WithError(err).WithField("spu_code", sheinProduct.SpuCode).Warn("映射产品数据失败")
			continue
		}

		// 查询 SKU 级别的库存信息
		inventoryInfo, err := s.inventoryManager.FetchInventoryInfo(apiClient, &sheinProduct)
		if err != nil {
			logrus.WithError(err).WithField("spu_name", sheinProduct.SpuName).Warn("获取库存信息失败")
		} else if inventoryInfo != nil {
			// 填充产品级别的库存信息（使用第一个 SKU 的库存）
			s.inventoryManager.FillProductLevelInventory(productData, inventoryInfo)
		}

		// 根据店铺类型处理价格信息
		priceMap, costMap, err := s.priceManager.ProcessPriceByShopType(apiClient, &sheinProduct, productData, shopType)
		if err != nil {
			logrus.WithError(err).WithField("spu_name", sheinProduct.SpuName).Warn("处理价格信息失败")
		}

		// 通过 SKU 查询映射表并填充 ASIN，同时填充 SKU 级别的价格/成本价/库存
		productInventoryInfo := s.inventoryManager.ConvertToProductInventoryInfo(inventoryInfo)
		s.dataEnricher.EnrichProductWithMappingBySku(productData, &sheinProduct, tenantID, storeID, productInventoryInfo, priceMap, costMap)

		if err := repository.CreateOrUpdate(productData); err != nil {
			logrus.WithError(err).WithField("spu_code", sheinProduct.SpuCode).Error("保存产品失败")
			continue
		}

		successCount++
	}

	return successCount, nil
}

// GetPlatformName 获取平台名称
func (s *SyncService) GetPlatformName() string {
	return "SHEIN"
}

// MapShelfStatus 映射上架状态（调用 mapper.go 中的实现）
func (s *SyncService) MapShelfStatus(platformStatus interface{}) int {
	status, ok := platformStatus.(string)
	if !ok {
		return api.ShelfStatusPending
	}
	return MapShelfStatus(status)
}
