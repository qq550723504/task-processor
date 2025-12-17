package shein

import (
	"encoding/json"
	"fmt"
	"task-processor/internal/common/management/api"
	shops "task-processor/internal/common/shein"
	"task-processor/internal/common/shein/api/product"

	"github.com/sirupsen/logrus"
)

// SyncService SHEIN 产品同步服务
type SyncService struct {
	repositoryFactory func(storeID, tenantID int64) api.ProductDataAPI
	mappingClient     api.ProductImportMappingAPI
}

// NewSyncService 创建 SHEIN 同步服务
func NewSyncService(repositoryFactory func(storeID, tenantID int64) api.ProductDataAPI) *SyncService {
	return &SyncService{
		repositoryFactory: repositoryFactory,
		mappingClient:     nil, // 可选，如果需要查询映射关系则设置
	}
}

// SetMappingClient 设置映射客户端
func (s *SyncService) SetMappingClient(mappingClient api.ProductImportMappingAPI) {
	s.mappingClient = mappingClient
}

// SyncProducts 同步 SHEIN 产品列表
// shopType: 0-半托管, 1-全托管, 2-自营
func (s *SyncService) SyncProducts(apiClient *shops.ShopAPIClient, tenantID, storeID int64, shopType string) (int, error) {
	logrus.WithFields(logrus.Fields{
		"platform":  "SHEIN",
		"tenant_id": tenantID,
		"store_id":  storeID,
		"shop_type": shopType,
	}).Info("开始同步 SHEIN 产品")

	// 为当前店铺创建专用的 repository
	repository := s.repositoryFactory(storeID, tenantID)

	// 调用 SHEIN 产品列表 API
	products, err := s.fetchProductList(apiClient)
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
		inventoryInfo, err := s.fetchInventoryInfo(apiClient, &sheinProduct)
		if err != nil {
			logrus.WithError(err).WithField("spu_name", sheinProduct.SpuName).Warn("获取库存信息失败")
		} else if inventoryInfo != nil {
			// 填充产品级别的库存信息（使用第一个 SKU 的库存）
			s.fillProductLevelInventory(productData, inventoryInfo)
		}

		// 根据店铺类型查询价格或成本价
		var priceMap map[string]*product.SkuPriceInfo
		var costMap map[string]*product.SkuCostInfo

		switch shopType {
		case "0":
			// 半托管店铺：查询成本价
			var err error
			costMap, err = s.fetchCostPriceInfo(apiClient, &sheinProduct)
			if err != nil {
				logrus.WithError(err).WithField("spu_name", sheinProduct.SpuName).Warn("获取成本价信息失败")
				costMap = make(map[string]*product.SkuCostInfo)
			} else {
				// 填充产品级别的成本价
				s.fillProductLevelCostPrice(productData, costMap)
			}
			priceMap = make(map[string]*product.SkuPriceInfo)
		case "2":
			// 自营店铺：查询价格
			var err error
			priceMap, err = s.fetchPriceInfo(apiClient, &sheinProduct)
			if err != nil {
				logrus.WithError(err).WithField("spu_name", sheinProduct.SpuName).Warn("获取价格信息失败")
				priceMap = make(map[string]*product.SkuPriceInfo)
			} else {
				// 填充产品级别的价格
				s.fillProductLevelPrice(productData, priceMap)
			}
			costMap = make(map[string]*product.SkuCostInfo)
		default:
			// 全托管或其他类型，暂不处理价格
			logrus.WithField("shop_type", shopType).Debug("全托管店铺暂不处理价格")
			priceMap = make(map[string]*product.SkuPriceInfo)
			costMap = make(map[string]*product.SkuCostInfo)
		}

		// 通过 SKU 查询映射表并填充 ASIN，同时填充 SKU 级别的价格/成本价/库存
		s.enrichProductWithMappingBySku(productData, &sheinProduct, tenantID, storeID, priceMap, costMap, inventoryInfo)

		if err := repository.CreateOrUpdate(productData); err != nil {
			logrus.WithError(err).WithField("spu_code", sheinProduct.SpuCode).Error("保存产品失败")
			continue
		}

		successCount++
	}

	return successCount, nil
}

// fetchProductList 获取 SHEIN 产品列表
func (s *SyncService) fetchProductList(apiClient *shops.ShopAPIClient) ([]SheinProductResponse, error) {
	// 构建请求参数
	request := &product.ProductListRequest{
		Language:             "en",
		OnlyRecommendResell:  false,
		OnlySpmbCopyProduct:  false,
		SearchAbandonProduct: false,
		SearchIllegal:        false,
		SearchLessInventory:  false,
		//ShelfType:            "ON_SHELF", // 只获取已上架产品
		SortType: 1,
	}

	// 调用产品列表 API
	response, err := apiClient.ListProducts(1, 100, request)
	if err != nil {
		return nil, fmt.Errorf("调用产品列表 API 失败: %w", err)
	}

	// 转换为 SheinProductResponse 格式
	var products []SheinProductResponse
	for _, item := range response.Info.Data {
		// 转换 SKC 信息
		var skcInfoList []SkcInfo
		for _, skc := range item.SkcInfoList {
			var skuInfoList []SkuInfo
			for _, sku := range skc.SkuInfo {
				skuInfoList = append(skuInfoList, SkuInfo{
					SkuCode: sku.SkuCode,
				})
			}

			skcInfoList = append(skcInfoList, SkcInfo{
				SkcName:               skc.SkcName,
				SkcCode:               skc.SkcCode,
				SaleName:              skc.SaleName,
				MainImageThumbnailURL: skc.MainImageThumbnailURL,
				SupplierCode:          skc.SupplierCode,
				BusinessModel:         skc.BusinessModel,
				IsSaleAttribute:       skc.IsSaleAttribute,
				SupplierID:            skc.SupplierID,
				SkuInfo:               skuInfoList,
				MallSellStatus:        skc.MallSellStatus,
				Abandoned:             skc.Abandoned,
				TagInfoList:           skc.TagInfoList,
				ShelfFailReason:       skc.ShelfFailReason,
				HasOriginalImage:      skc.HasOriginalImage,
			})
		}

		product := SheinProductResponse{
			SpuName:          item.SpuName,
			SpuCode:          item.SpuCode,
			CategoryID:       item.CategoryID,
			BrandCode:        item.BrandCode,
			BrandName:        item.BrandName,
			ProductNameCh:    item.ProductNameCh,
			ProductNameEn:    item.ProductNameEn,
			ProductNameMulti: item.ProductNameMulti,
			SkcInfoList:      skcInfoList,
			ShelfStatus:      item.ShelfStatus,
			CreateTime:       item.CreateTime,
			PublishTime:      item.PublishTime,
			FirstShelfTime:   item.FirstShelfTime,
			ExpectShelfTime:  item.ExpectShelfTime,
			TagInfoList:      item.TagInfoList,
		}

		products = append(products, product)
	}

	logrus.WithField("count", len(products)).Info("成功获取 SHEIN 产品列表")
	return products, nil
}

// enrichProductWithMappingBySku 通过 SKU 查询映射关系并填充 ASIN，同时填充 SKU 级别的完整价格/成本价/库存信息
func (s *SyncService) enrichProductWithMappingBySku(productData *api.ProductDataDTO, sheinProduct *SheinProductResponse, tenantID, storeID int64, priceMap map[string]*product.SkuPriceInfo, costMap map[string]*product.SkuCostInfo, inventoryInfo *product.InventoryInfo) {
	if s.mappingClient == nil {
		return
	}

	// 遍历所有 SKC 和 SKU，查询映射关系并填充到 SKU 级别
	foundMapping := false
	var firstAsin string
	var firstParentAsin string
	mappingCount := 0

	// 构建 SKU Code 到库存信息的映射
	skuInventoryMap := make(map[string]*product.SkuInventory)
	if inventoryInfo != nil {
		for _, skcInv := range inventoryInfo.SkcInfo {
			for i := range skcInv.SkuInfo {
				skuInv := &skcInv.SkuInfo[i]
				skuInventoryMap[skuInv.SkuCode] = skuInv
			}
		}
	}

	// 创建增强的 SKU 数据结构
	type EnrichedSkuInfo struct {
		SkuInfo
		MappingInfo       *api.ProductImportMappingRespDTO `json:"mapping_info,omitempty"`
		SaleNameInfo      []product.SaleNameInfo           `json:"sale_name_info,omitempty"`      // 自营店铺：销售属性
		PriceInfoList     []product.SkuPriceDetail         `json:"price_info_list,omitempty"`     // 自营店铺：价格列表
		SaleAttributeList []product.SaleAttributeItem      `json:"sale_attribute_list,omitempty"` // 半托店铺：销售属性
		CostPriceInfo     *product.CostPrice               `json:"cost_price_info,omitempty"`     // 半托店铺：成本价
		InventoryInfo     []product.WarehouseInventory     `json:"inventory_info,omitempty"`      // SKU 库存信息
		UsableInventory   *int                             `json:"usable_inventory,omitempty"`    // 可用库存汇总
		InventoryQuantity *int                             `json:"inventory_quantity,omitempty"`  // 总库存汇总
	}

	// 创建增强的 SKC 数据结构
	type EnrichedSkcInfo struct {
		SkcName               string            `json:"skc_name"`
		SkcCode               string            `json:"skc_code"`
		SaleName              string            `json:"sale_name"`
		MainImageThumbnailURL string            `json:"main_image_thumbnail_url"`
		SupplierCode          string            `json:"supplier_code"`
		BusinessModel         int               `json:"business_model"`
		IsSaleAttribute       int               `json:"is_sale_attribute"`
		SupplierID            int64             `json:"supplier_id"`
		SkuInfo               []EnrichedSkuInfo `json:"sku_info"`
		MallSellStatus        int               `json:"mall_sell_status"`
		Abandoned             bool              `json:"abandoned"`
		TagInfoList           []interface{}     `json:"tag_info_list"`
		ShelfFailReason       *string           `json:"shelf_fail_reason"`
		HasOriginalImage      bool              `json:"has_original_image"`
	}

	enrichedSkcList := make([]EnrichedSkcInfo, 0, len(sheinProduct.SkcInfoList))

	for _, skc := range sheinProduct.SkcInfoList {
		enrichedSkc := EnrichedSkcInfo{
			SkcName:               skc.SkcName,
			SkcCode:               skc.SkcCode,
			SaleName:              skc.SaleName,
			MainImageThumbnailURL: skc.MainImageThumbnailURL,
			SupplierCode:          skc.SupplierCode,
			BusinessModel:         skc.BusinessModel,
			IsSaleAttribute:       skc.IsSaleAttribute,
			SupplierID:            skc.SupplierID,
			SkuInfo:               make([]EnrichedSkuInfo, 0, len(skc.SkuInfo)),
			MallSellStatus:        skc.MallSellStatus,
			Abandoned:             skc.Abandoned,
			TagInfoList:           skc.TagInfoList,
			ShelfFailReason:       skc.ShelfFailReason,
			HasOriginalImage:      skc.HasOriginalImage,
		}

		// 遍历 SKC 下的每个 SKU，为每个 SKU 查询映射关系、价格/成本价和库存
		for _, sku := range skc.SkuInfo {
			enrichedSku := EnrichedSkuInfo{
				SkuInfo:           sku,
				MappingInfo:       nil,
				SaleNameInfo:      nil,
				PriceInfoList:     nil,
				SaleAttributeList: nil,
				CostPriceInfo:     nil,
				UsableInventory:   nil,
				InventoryQuantity: nil,
			}

			// 填充 SKU 级别的完整价格信息（自营店铺）
			if skuPriceInfo, ok := priceMap[sku.SkuCode]; ok {
				enrichedSku.SaleNameInfo = skuPriceInfo.SaleNameInfo
				enrichedSku.PriceInfoList = skuPriceInfo.PriceInfoList
			}

			// 填充 SKU 级别的完整成本价信息（半托店铺）
			if skuCostInfo, ok := costMap[sku.SkuCode]; ok {
				enrichedSku.SaleAttributeList = skuCostInfo.SaleAttributeList
				enrichedSku.CostPriceInfo = &skuCostInfo.CostPriceInfo
			}

			// 填充 SKU 级别的库存信息
			if skuInv, ok := skuInventoryMap[sku.SkuCode]; ok {
				enrichedSku.InventoryInfo = skuInv.InventoryInfo

				// 计算总库存和可用库存
				totalUsable := 0
				totalInventory := 0
				for _, warehouse := range skuInv.InventoryInfo {
					totalUsable += warehouse.UsableInventory
					totalInventory += warehouse.InventoryQuantity
				}
				enrichedSku.UsableInventory = &totalUsable
				enrichedSku.InventoryQuantity = &totalInventory
			}

			// 通过平台产品ID（SkuCode）查询映射关系
			mapping, err := s.mappingClient.GetProductImportMappingByPlatformProductIdAndStore(&api.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO{
				PlatformProductId: sku.SkuCode,
				StoreId:           storeID,
			})

			if err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{
					"sku_code": sku.SkuCode,
					"store_id": storeID,
				}).Debug("查询 SKU 映射关系失败")
				enrichedSkc.SkuInfo = append(enrichedSkc.SkuInfo, enrichedSku)
				continue
			}

			if mapping == nil {
				logrus.WithFields(logrus.Fields{
					"sku_code": sku.SkuCode,
					"store_id": storeID,
				}).Debug("未找到 SKU 映射关系")
				enrichedSkc.SkuInfo = append(enrichedSkc.SkuInfo, enrichedSku)
				continue
			}

			// 将映射信息添加到 SKU 级别
			enrichedSku.MappingInfo = mapping
			enrichedSkc.SkuInfo = append(enrichedSkc.SkuInfo, enrichedSku)
			mappingCount++

			// 记录第一个找到的 ASIN 作为产品级别的 ProductID
			if !foundMapping && mapping.ProductId != "" {
				firstAsin = mapping.ProductId
				if mapping.ParentProductId != nil {
					firstParentAsin = *mapping.ParentProductId
				}
				foundMapping = true
			}
		}

		enrichedSkcList = append(enrichedSkcList, enrichedSkc)
	}

	// 使用第一个找到的 ASIN 填充产品级别的 ProductID 和 Region
	if foundMapping {
		productData.ProductID = firstAsin
		if firstParentAsin != "" {
			productData.ParentProductID = firstParentAsin
		}

		// 从映射信息中获取 Region
		if len(enrichedSkcList) > 0 {
			for _, enrichedSkc := range enrichedSkcList {
				for _, enrichedSku := range enrichedSkc.SkuInfo {
					if enrichedSku.MappingInfo != nil && enrichedSku.MappingInfo.Region != "" {
						productData.Region = enrichedSku.MappingInfo.Region
						break
					}
				}
				if productData.Region != "" {
					break
				}
			}
		}

		// 更新 Attributes，包含 SKU 级别的映射信息
		s.updateAttributesWithMappings(productData, enrichedSkcList)
	} else {
		logrus.WithFields(logrus.Fields{
			"spu_code": sheinProduct.SpuCode,
			"store_id": storeID,
		}).Debug("未找到任何 SKU 映射关系")
	}
}

// updateAttributesWithMappings 更新 Attributes，包含 SKU 级别的映射信息
func (s *SyncService) updateAttributesWithMappings(productData *api.ProductDataDTO, enrichedSkcList interface{}) {
	// 序列化包含映射信息的 SKC 列表到 Attributes
	if attributesJSON, err := json.Marshal(enrichedSkcList); err == nil {
		productData.Attributes = string(attributesJSON)
		logrus.Debug("已更新 Attributes，包含 SKU 级别的映射信息")
	} else {
		logrus.WithError(err).Warn("序列化增强的 Attributes 失败")
	}
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

// fetchInventoryInfo 获取产品的 SKU 级别库存信息
func (s *SyncService) fetchInventoryInfo(apiClient *shops.ShopAPIClient, sheinProduct *SheinProductResponse) (*product.InventoryInfo, error) {
	// 调用库存详情查询 API
	response, err := apiClient.QueryInventory(sheinProduct.SpuName)
	if err != nil {
		return nil, fmt.Errorf("调用库存查询 API 失败: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"spu_name":  sheinProduct.SpuName,
		"skc_count": len(response.Info.SkcInfo),
	}).Debug("成功获取 SKU 级别库存信息")

	return &response.Info, nil
}

// fillProductLevelInventory 填充产品级别的库存信息（使用第一个 SKU 的库存汇总）
func (s *SyncService) fillProductLevelInventory(productData *api.ProductDataDTO, inventoryInfo *product.InventoryInfo) {
	if inventoryInfo == nil || len(inventoryInfo.SkcInfo) == 0 {
		return
	}

	// 汇总所有 SKU 的库存
	totalUsable := 0
	totalInventory := 0

	for _, skcInv := range inventoryInfo.SkcInfo {
		for _, skuInv := range skcInv.SkuInfo {
			for _, warehouse := range skuInv.InventoryInfo {
				totalUsable += warehouse.UsableInventory
				totalInventory += warehouse.InventoryQuantity
			}
		}
	}

	productData.Stock = api.FlexibleString(fmt.Sprintf("%d", totalUsable))

	logrus.WithFields(logrus.Fields{
		"spu_name":         inventoryInfo.SpuName,
		"usable_inventory": totalUsable,
		"total_inventory":  totalInventory,
	}).Debug("填充产品级别库存信息")
}

// fetchPriceInfo 获取产品价格信息，返回 SKU 级别的完整价格数据（自营店铺）
func (s *SyncService) fetchPriceInfo(apiClient *shops.ShopAPIClient, sheinProduct *SheinProductResponse) (map[string]*product.SkuPriceInfo, error) {
	// 查询价格信息
	priceResponse, err := apiClient.QueryPrice(sheinProduct.SpuName)
	if err != nil {
		return nil, fmt.Errorf("查询价格失败: %w", err)
	}

	if len(priceResponse.Info.Data) == 0 {
		return nil, fmt.Errorf("未获取到价格数据")
	}

	// 构建 SKU Code 到完整价格信息的映射
	priceMap := make(map[string]*product.SkuPriceInfo)

	for _, skcPrice := range priceResponse.Info.Data {
		for _, skuPrice := range skcPrice.SkuInfoList {
			// 保存完整的 SKU 价格信息
			skuPriceCopy := skuPrice
			priceMap[skuPrice.SkuCode] = &skuPriceCopy
		}
	}

	logrus.WithFields(logrus.Fields{
		"spu_name":    sheinProduct.SpuName,
		"price_count": len(priceMap),
	}).Debug("成功获取 SKU 级别的完整价格信息（自营店铺）")

	return priceMap, nil
}

// fetchCostPriceInfo 获取产品成本价信息，返回 SKU 级别的完整成本数据（半托店铺）
func (s *SyncService) fetchCostPriceInfo(apiClient *shops.ShopAPIClient, sheinProduct *SheinProductResponse) (map[string]*product.SkuCostInfo, error) {
	// 构建 SKC 名称列表
	var skcNameList []string
	for _, skc := range sheinProduct.SkcInfoList {
		skcNameList = append(skcNameList, skc.SkcName)
	}

	// 查询成本价信息
	costResponse, err := apiClient.QueryCostPrice(sheinProduct.SpuName, skcNameList)
	if err != nil {
		return nil, fmt.Errorf("查询成本价失败: %w", err)
	}

	if len(costResponse.Info.Data) == 0 {
		return nil, fmt.Errorf("未获取到成本价数据")
	}

	// 构建 SKU Code 到完整成本信息的映射
	costMap := make(map[string]*product.SkuCostInfo)

	for _, skcCost := range costResponse.Info.Data {
		for _, skuCost := range skcCost.SkuCostInfoList {
			// 保存完整的 SKU 成本信息
			skuCostCopy := skuCost
			costMap[skuCost.SkuCode] = &skuCostCopy
		}
	}

	logrus.WithFields(logrus.Fields{
		"spu_name":   sheinProduct.SpuName,
		"cost_count": len(costMap),
	}).Debug("成功获取 SKU 级别的完整成本价信息（半托店铺）")

	return costMap, nil
}

// fillProductLevelPrice 填充产品级别的价格（使用第一个 SKU 的价格）- 自营店铺
func (s *SyncService) fillProductLevelPrice(productData *api.ProductDataDTO, priceMap map[string]*product.SkuPriceInfo) {
	// 使用第一个有价格的 SKU 作为产品级别的价格
	for _, skuPriceInfo := range priceMap {
		if len(skuPriceInfo.PriceInfoList) > 0 {
			priceDetail := skuPriceInfo.PriceInfoList[0]
			productData.OriginalPrice = api.FlexibleString(fmt.Sprintf("%.2f", priceDetail.ShopPrice))
			productData.SpecialPrice = api.FlexibleString(fmt.Sprintf("%.2f", priceDetail.SpecialPrice))
			productData.PriceCurrency = priceDetail.Currency
			break // 只使用第一个
		}
	}
}

// fillProductLevelCostPrice 填充产品级别的成本价（使用第一个 SKU 的成本价）- 半托店铺
func (s *SyncService) fillProductLevelCostPrice(productData *api.ProductDataDTO, costMap map[string]*product.SkuCostInfo) {
	// 使用第一个有成本价的 SKU 作为产品级别的价格
	for _, skuCostInfo := range costMap {
		productData.OriginalPrice = api.FlexibleString(skuCostInfo.CostPriceInfo.CostPrice)
		productData.SpecialPrice = api.FlexibleString(skuCostInfo.CostPriceInfo.CostPrice)
		productData.PriceCurrency = skuCostInfo.CostPriceInfo.Currency
		break // 只使用第一个
	}
}
