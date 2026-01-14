// Package sync 提供SHEIN平台产品同步服务
package sync

import (
	"fmt"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/repo/client"

	"github.com/sirupsen/logrus"
)

// SyncService SHEIN产品同步服务
type SyncService struct {
	repositoryFactory func(storeID, tenantID int64) api.ProductDataAPI
	mappingClient     api.ProductImportMappingAPI
	productFetcher    *ProductFetcher
	dataEnricher      *DataEnricher
	priceManager      *PriceManager
	inventoryManager  *InventoryManager
}

// NewSyncService 创建SHEIN同步服务
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

// SyncProducts 同步SHEIN产品列表
// shopType: 0-半托管, 1-全托管, 2-自营
func (s *SyncService) SyncProducts(apiClient *client.APIClient, tenantID, storeID int64, shopType string) (int, error) {
	logrus.WithFields(logrus.Fields{
		"platform":  "SHEIN",
		"tenant_id": tenantID,
		"store_id":  storeID,
		"shop_type": shopType,
	}).Info("开始同步SHEIN产品")

	// 为当前店铺创建专用的repository
	repository := s.repositoryFactory(storeID, tenantID)

	// 调用SHEIN产品列表API
	products, err := s.productFetcher.FetchProductList(apiClient, storeID)
	if err != nil {
		return 0, fmt.Errorf("获取产品列表失败: %w", err)
	}

	logrus.WithField("count", len(products)).Info("获取到产品列表")

	// 转换并保存产品
	successCount := 0
	for _, sheinProduct := range products {
		if err := s.processProduct(&sheinProduct, apiClient, tenantID, storeID, shopType, repository); err != nil {
			logrus.WithError(err).WithField("spu_code", sheinProduct.SpuCode).Warn("处理产品失败")
			continue
		}
		successCount++
	}

	return successCount, nil
}

// processProduct 处理单个产品
func (s *SyncService) processProduct(
	sheinProduct interface{},
	apiClient *client.APIClient,
	tenantID, storeID int64,
	shopType string,
	repository api.ProductDataAPI,
) error {
	// 类型断言
	product, ok := sheinProduct.(*SheinProductResponse)
	if !ok {
		return fmt.Errorf("产品类型断言失败")
	}

	// 映射产品数据
	productData, err := MapToProductData(product, storeID)
	if err != nil {
		return fmt.Errorf("映射产品数据失败: %w", err)
	}

	// 查询SKU级别的库存信息
	inventoryInfo, err := s.inventoryManager.FetchInventoryInfo(apiClient, product)
	if err != nil {
		logrus.WithError(err).WithField("spu_name", product.SpuName).Warn("获取库存信息失败")
	} else if inventoryInfo != nil {
		// 填充产品级别的库存信息
		s.inventoryManager.FillProductLevelInventory(productData, inventoryInfo)
	}

	// 获取价格和成本价信息
	priceMap, err := s.priceManager.FetchPriceInfo(apiClient, product)
	if err != nil {
		logrus.WithError(err).WithField("spu_name", product.SpuName).Warn("获取价格信息失败")
	}

	costMap, err := s.priceManager.FetchCostPriceInfo(apiClient, product)
	if err != nil {
		logrus.WithError(err).WithField("spu_name", product.SpuName).Warn("获取成本价信息失败")
	}

	// 通过SKU查询映射表并填充ASIN，同时填充SKU级别的价格/成本价/库存
	productInventoryInfo := s.inventoryManager.ConvertToProductInventoryInfo(inventoryInfo)
	s.dataEnricher.EnrichProductWithMappingBySku(productData, product, tenantID, storeID, productInventoryInfo, priceMap, costMap)

	// 保存产品
	if err := repository.CreateOrUpdate(productData); err != nil {
		return fmt.Errorf("保存产品失败: %w", err)
	}

	return nil
}

// GetPlatformName 获取平台名称
func (s *SyncService) GetPlatformName() string {
	return "SHEIN"
}

// MapShelfStatus 映射上架状态（调用mapper.go中的实现）
func (s *SyncService) MapShelfStatus(platformStatus interface{}) int {
	status, ok := platformStatus.(string)
	if !ok {
		return api.ShelfStatusPending
	}
	return MapShelfStatus(status)
}
