package temu

import (
	"fmt"
	"task-processor/common/management/api"
	temuapi "task-processor/common/temu"

	"github.com/sirupsen/logrus"
)

// SyncService TEMU 产品同步服务
type SyncService struct {
	repositoryFactory func(storeID, tenantID int64) api.ProductDataAPI
	apiClient         *temuapi.APIClient
	mappingClient     api.ProductImportMappingAPI
}

// NewSyncService 创建 TEMU 同步服务
func NewSyncService(repositoryFactory func(storeID, tenantID int64) api.ProductDataAPI, temuAPIClient *temuapi.APIClient) *SyncService {
	return &SyncService{
		repositoryFactory: repositoryFactory,
		apiClient:         temuAPIClient,
		mappingClient:     nil, // 可选，如果需要查询映射关系则设置
	}
}

// SetMappingClient 设置映射客户端
func (s *SyncService) SetMappingClient(mappingClient api.ProductImportMappingAPI) {
	s.mappingClient = mappingClient
}

// SyncProducts 同步 TEMU 产品列表
func (s *SyncService) SyncProducts(tenantID, storeID int64) (int, error) {
	logrus.WithFields(logrus.Fields{
		"platform":  "TEMU",
		"tenant_id": tenantID,
		"store_id":  storeID,
	}).Info("开始同步 TEMU 产品")

	// 为当前店铺创建专用的 repository
	repository := s.repositoryFactory(storeID, tenantID)

	// 调用 TEMU 产品列表 API
	products, err := s.fetchProductList()
	if err != nil {
		return 0, fmt.Errorf("获取产品列表失败: %w", err)
	}

	logrus.WithField("count", len(products)).Info("获取到产品列表")

	// 转换并保存产品
	successCount := 0
	for _, temuProduct := range products {
		// 转换为本地 TemuProductResponse 类型，并查询映射关系
		localProduct := s.convertToLocalProductWithMapping(&temuProduct, tenantID, storeID)
		productData, err := MapToProductData(localProduct, tenantID, storeID)
		if err != nil {
			logrus.WithError(err).WithField("goods_id", temuProduct.GoodsID).Warn("映射产品数据失败")
			continue
		}

		if err := repository.CreateOrUpdate(productData); err != nil {
			logrus.WithError(err).WithField("goods_id", temuProduct.GoodsID).Error("保存产品失败")
			continue
		}

		successCount++
	}

	logrus.WithFields(logrus.Fields{
		"total":   len(products),
		"success": successCount,
		"failed":  len(products) - successCount,
	}).Info("TEMU 产品同步完成")

	return successCount, nil
}

// fetchProductList 获取 TEMU 产品列表
func (s *SyncService) fetchProductList() ([]temuapi.TemuProductResponse, error) {
	// 调用 TEMU API 获取已上架产品
	products, err := s.apiClient.ListOnShelfProducts(1, 100)
	if err != nil {
		return nil, fmt.Errorf("调用 TEMU API 失败: %w", err)
	}

	logrus.WithField("count", len(products)).Info("成功获取 TEMU 已上架产品列表")
	return products, nil
}

// SyncSingleProduct 同步单个产品
func (s *SyncService) SyncSingleProduct(tenantID, storeID int64, goodsID string) error {
	logrus.WithFields(logrus.Fields{
		"platform":  "TEMU",
		"tenant_id": tenantID,
		"store_id":  storeID,
		"goods_id":  goodsID,
	}).Info("同步单个产品")

	// 为当前店铺创建专用的 repository
	repository := s.repositoryFactory(storeID, tenantID)

	// 获取单个产品详情
	product, err := s.fetchSingleProduct(goodsID)
	if err != nil {
		return fmt.Errorf("获取产品详情失败: %w", err)
	}

	// 转换并映射，并查询映射关系
	localProduct := s.convertToLocalProductWithMapping(product, tenantID, storeID)
	productData, err := MapToProductData(localProduct, tenantID, storeID)
	if err != nil {
		return fmt.Errorf("映射产品数据失败: %w", err)
	}

	if err := repository.CreateOrUpdate(productData); err != nil {
		return fmt.Errorf("保存产品失败: %w", err)
	}

	logrus.WithField("goods_id", goodsID).Info("单个产品同步成功")
	return nil
}

// fetchSingleProduct 获取单个产品详情
func (s *SyncService) fetchSingleProduct(goodsID string) (*temuapi.TemuProductResponse, error) {
	product, err := s.apiClient.GetProduct(goodsID)
	if err != nil {
		return nil, fmt.Errorf("调用 TEMU 产品详情 API 失败: %w", err)
	}
	return product, nil
}

// GetPlatformName 获取平台名称
func (s *SyncService) GetPlatformName() string {
	return "TEMU"
}

// GetAPIClient 获取 API 客户端
func (s *SyncService) GetAPIClient() *temuapi.APIClient {
	return s.apiClient
}

// MapShelfStatus 映射上架状态（调用 mapper.go 中的实现）
func (s *SyncService) MapShelfStatus(platformStatus interface{}) int {
	status, ok := platformStatus.(int)
	if !ok {
		return api.ShelfStatusPending
	}
	return MapShelfStatus(status)
}

// convertToLocalProductWithMapping 将 common/temu 的 TemuProductResponse 转换为 platforms/temu 的 TemuProductResponse
// 并通过 SKU 查询映射关系，填充 ASIN 等信息
func (s *SyncService) convertToLocalProductWithMapping(apiProduct *temuapi.TemuProductResponse, tenantID, storeID int64) *TemuProductResponse {
	if apiProduct == nil {
		return nil
	}

	localProduct := &TemuProductResponse{
		ListingCommitID:       apiProduct.ListingCommitID,
		GoodsCommitID:         apiProduct.GoodsCommitID,
		GoodsID:               apiProduct.GoodsID,
		GoodsName:             apiProduct.GoodsName,
		SpecName:              apiProduct.SpecName,
		ThumbURL:              apiProduct.ThumbURL,
		SkuPreviewURL:         apiProduct.SkuPreviewURL,
		MallID:                apiProduct.MallID,
		OutGoodsSn:            apiProduct.OutGoodsSn,
		Status4Vo:             apiProduct.Status4Vo,
		SubStatus4Vo:          apiProduct.SubStatus4Vo,
		ClosedTypeList:        apiProduct.ClosedTypeList,
		Currency:              apiProduct.Currency,
		MarketPrice:           apiProduct.MarketPrice,
		MarketPriceVo:         PriceVO(apiProduct.MarketPriceVo),
		ListPrice:             PriceVO(apiProduct.ListPrice),
		ListPriceVo:           PriceVO(apiProduct.ListPriceVo),
		OutSkuSnList:          apiProduct.OutSkuSnList,
		SkuIDList:             apiProduct.SkuIDList,
		Price:                 apiProduct.Price,
		PriceVo:               PriceVO(apiProduct.PriceVo),
		Quantity:              apiProduct.Quantity,
		VariationsCount:       apiProduct.VariationsCount,
		CrtTime:               apiProduct.CrtTime,
		StatusUpdateTime:      apiProduct.StatusUpdateTime,
		SupplierPrice:         apiProduct.SupplierPrice,
		GoodsAllowSiteList:    apiProduct.GoodsAllowSiteList,
		CatType:               apiProduct.CatType,
		CatID:                 apiProduct.CatID,
		CatNameList:           apiProduct.CatNameList,
		MultiSiteGoods:        apiProduct.MultiSiteGoods,
		ShowSubStatus4Vo:      apiProduct.ShowSubStatus4Vo,
		PersonalizationStatus: apiProduct.PersonalizationStatus,
		PunishTags:            apiProduct.PunishTags,
		StockDisplayTag:       apiProduct.StockDisplayTag,
		LowTrafficTag:         apiProduct.LowTrafficTag,
		RestrictedTrafficTag:  apiProduct.RestrictedTrafficTag,
		OrdinaryStock:         apiProduct.OrdinaryStock,
		ShippingMode:          apiProduct.ShippingMode,
		EasyGainsTag:          apiProduct.EasyGainsTag,
		IsBooks:               apiProduct.IsBooks,
	}

	// 如果有映射客户端，查询 SKU 映射关系并填充 ASIN
	if s.mappingClient != nil && len(apiProduct.OutSkuSnList) > 0 {
		skuToAsinMap := s.enrichProductWithMappings(localProduct, tenantID, storeID)

		// 将 SKU -> ASIN 映射存储到产品的扩展字段中
		// 这样在 mapper 中可以使用这些信息
		if len(skuToAsinMap) > 0 {
			// 可以将映射信息存储到 localProduct 的某个字段
			// 或者通过其他方式传递给 mapper
			logrus.WithField("mappings_count", len(skuToAsinMap)).Debug("已获取 SKU 映射关系")
		}
	}

	return localProduct
}

// enrichProductWithMappings 通过查询映射关系丰富产品信息
func (s *SyncService) enrichProductWithMappings(product *TemuProductResponse, tenantID, storeID int64) map[string]string {
	// 创建 SKU -> ASIN 的映射
	skuToAsinMap := make(map[string]string)

	// 遍历所有 SKU，查询映射关系
	for _, outSkuSn := range product.OutSkuSnList {
		if outSkuSn == "" {
			continue
		}

		// 通过 SKU 查询映射关系
		mapping, err := s.mappingClient.GetProductImportMappingBySku(&api.ProductImportMappingGetBySkuReqDTO{
			Sku:     outSkuSn,
			StoreId: storeID,
		})

		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"sku":      outSkuSn,
				"store_id": storeID,
			}).Debug("查询 SKU 映射关系失败")
			continue
		}

		if mapping == nil {
			logrus.WithFields(logrus.Fields{
				"sku":      outSkuSn,
				"store_id": storeID,
			}).Debug("未找到 SKU 映射关系")
			continue
		}

		// 保存 SKU -> ASIN 映射
		if mapping.ProductId != "" {
			skuToAsinMap[outSkuSn] = mapping.ProductId
		}

		logrus.WithFields(logrus.Fields{
			"sku":                 outSkuSn,
			"asin":                mapping.ProductId,
			"parent_asin":         getStringValue(mapping.ParentProductId),
			"platform_product_id": getStringValue(mapping.PlatformProductId),
			"cost_price":          mapping.CostPrice,
		}).Info("找到 SKU 映射关系")
	}

	return skuToAsinMap
}

// stringPtr 返回字符串指针
func stringPtr(s string) *string {
	return &s
}

// getStringValue 安全获取字符串指针的值
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
