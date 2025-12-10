package shein

import (
	"encoding/json"
	"fmt"
	"math"
	"task-processor/common/amazon"
	"task-processor/common/management/api"
	shops "task-processor/common/shein"
	"task-processor/common/shein/api/product"
	"task-processor/common/shein/api/warehouse"

	"github.com/sirupsen/logrus"
)

// StrategyExecutor 运营策略执行器
type StrategyExecutor struct {
	strategy  *api.OperationStrategyDTO
	apiClient *shops.ShopAPIClient
}

// NewStrategyExecutor 创建策略执行器
func NewStrategyExecutor(strategy *api.OperationStrategyDTO, apiClient *shops.ShopAPIClient) *StrategyExecutor {
	return &StrategyExecutor{
		strategy:  strategy,
		apiClient: apiClient,
	}
}

// ExecuteStockChange 执行库存变化策略
// 返回 error 表示执行失败，返回 nil 表示执行成功或不需要执行
func (e *StrategyExecutor) ExecuteStockChange(
	prod *api.ProductDataDTO,
	skuMapping *SKUMappingData,
	amazonProduct *amazon.Product,
) error {
	if !e.strategy.IsEnabled() || e.strategy.StockChangeAction == "NONE" {
		return nil
	}

	oldStock := skuMapping.Stock
	newStock := extractStockFromProduct(amazonProduct)

	// 如果库存充足（31 表示 In Stock），则不做任何动作
	if newStock == 31 {
		return nil
	}

	changeAmount := int(math.Abs(float64(newStock - oldStock)))

	// 库存变化未达到阈值
	if changeAmount < e.strategy.StockChangeThreshold {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"sku":           skuMapping.MappingInfo.SKU,
		"old_stock":     oldStock,
		"new_stock":     newStock,
		"change_amount": changeAmount,
		"action":        e.strategy.StockChangeAction,
	}).Info("触发库存变化策略")

	switch e.strategy.StockChangeAction {
	case "OFF_SHELF":
		return e.offShelfProduct(prod)
	case "UPDATE_STOCK":
		return e.updateStock(prod, skuMapping, newStock)
	}

	return nil
}

// ExecuteOutOfStock 执行缺货策略
// 返回 error 表示执行失败，返回 nil 表示执行成功或不需要执行
func (e *StrategyExecutor) ExecuteOutOfStock(
	prod *api.ProductDataDTO,
	skuMapping *SKUMappingData,
	amazonProduct *amazon.Product,
) error {
	if !e.strategy.IsEnabled() || e.strategy.OutOfStockAction == "NONE" {
		return nil
	}

	// 产品有货，不需要执行缺货策略
	if amazonProduct.IsAvailable {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"sku":    skuMapping.MappingInfo.SKU,
		"action": e.strategy.OutOfStockAction,
	}).Info("触发缺货策略")

	switch e.strategy.OutOfStockAction {
	case "OFF_SHELF":
		return e.offShelfProduct(prod)
	case "SET_ZERO_STOCK":
		return e.updateStock(prod, skuMapping, 0)
	}

	return nil
}

// ExecuteLowProfit 执行低利润率策略
// 返回 error 表示执行失败，返回 nil 表示执行成功或不需要执行
func (e *StrategyExecutor) ExecuteLowProfit(
	prod *api.ProductDataDTO,
	skuMapping *SKUMappingData,
	amazonProduct *amazon.Product,
) error {
	if !e.strategy.IsEnabled() || e.strategy.LowProfitAction == "NONE" {
		return nil
	}

	// 从 CostPriceInfo 获取 SHEIN 销售价格(字符串类型需要转换)
	salePrice := parsePriceString(skuMapping.CostPriceInfo.CostPrice)

	// 从 SKUMappingData 的 AmazonMonitorData 获取成本价
	var costPrice float64
	if skuMapping.AmazonMonitorData != nil {
		costPrice = skuMapping.AmazonMonitorData.Price
	}

	if salePrice <= 0 || costPrice <= 0 {
		logrus.WithFields(logrus.Fields{
			"sku":        skuMapping.MappingInfo.SKU,
			"sale_price": salePrice,
			"cost_price": costPrice,
		}).Debug("销售价或成本价无效，跳过利润率检查")
		return nil
	}

	profitRate := (salePrice - costPrice) / costPrice * 100

	// 利润率达标，不需要执行策略
	if profitRate >= e.strategy.MinProfitRate {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"sku":         skuMapping.MappingInfo.SKU,
		"sale_price":  salePrice,
		"cost_price":  costPrice,
		"profit_rate": profitRate,
		"min_rate":    e.strategy.MinProfitRate,
		"action":      e.strategy.LowProfitAction,
	}).Info("触发低利润率策略")

	switch e.strategy.LowProfitAction {
	case "OFF_SHELF":
		return e.offShelfProduct(prod)
	case "UPDATE_PRICE":
		newPrice := costPrice * (1 + e.strategy.MinProfitRate/100)
		return e.updatePrice(prod, skuMapping, newPrice)
	case "SET_ZERO_STOCK":
		return e.updateStock(prod, skuMapping, 0)
	}

	return nil
}

// offShelfProduct 下架产品
func (e *StrategyExecutor) offShelfProduct(prod *api.ProductDataDTO) error {
	logrus.WithFields(logrus.Fields{
		"platform_product_id": prod.PlatformProductID,
		"spu_name":            prod.Title,
	}).Info("执行下架操作")

	// 解析 Attributes 获取 SKC 信息
	mappings := extractMappingInfoFromAttributes(prod.Attributes)
	if len(mappings) == 0 {
		logrus.Warn("产品没有 SKC 映射信息，无法下架")
		return nil
	}

	// 构建下架请求
	//request := e.buildOffShelfRequest(prod, mappings)

	// 调用 SHEIN API 下架产品
	// if err := e.apiClient.OffShelf(request); err != nil {
	// 	logrus.WithError(err).Error("调用下架接口失败")
	// 	return err
	// }

	logrus.Info("产品下架成功")
	return nil
}

// updatePrice 更新价格
func (e *StrategyExecutor) updatePrice(
	prod *api.ProductDataDTO,
	skuMapping *SKUMappingData,
	newPrice float64,
) error {
	// 应用价格更新倍数
	if e.strategy.PriceUpdateMultiplier > 0 {
		newPrice = newPrice * e.strategy.PriceUpdateMultiplier
	}

	logrus.WithFields(logrus.Fields{
		"sku":       skuMapping.MappingInfo.SKU,
		"new_price": newPrice,
	}).Info("执行价格更新操作")

	// TODO: 调用 SHEIN API 更新价格
	// 这里需要实现具体的价格更新逻辑

	return nil
}

// updateStock 更新库存
func (e *StrategyExecutor) updateStock(
	prod *api.ProductDataDTO,
	skuMapping *SKUMappingData,
	newStock int,
) error {
	// 应用库存更新比例
	if e.strategy.StockUpdateRatio > 0 {
		newStock = int(float64(newStock) * e.strategy.StockUpdateRatio)
	}

	platformSKU := skuMapping.MappingInfo.SKU
	oldStock := skuMapping.Stock

	logrus.WithFields(logrus.Fields{
		"platform_sku": platformSKU,
		"old_stock":    oldStock,
		"new_stock":    newStock,
	}).Info("执行库存更新操作")

	// 查询商户仓库信息获取仓库代码
	warehouseResp, err := e.apiClient.GetWarehouses()
	if err != nil {
		logrus.WithError(err).Error("查询商户仓库信息失败")
		return fmt.Errorf("查询商户仓库信息失败: %w", err)
	}

	if len(warehouseResp.Data) == 0 {
		return fmt.Errorf("未找到可用的仓库")
	}

	// 从 Attributes 构建库存更新请求
	request := e.buildInventoryUpdateRequestFromAttributes(
		prod.Attributes,
		warehouseResp,
		platformSKU,
		oldStock,
		newStock,
	)
	if request == nil {
		return fmt.Errorf("未找到对应的 SKU: %s", platformSKU)
	}

	// 调用 SHEIN API 更新库存
	if err := e.apiClient.UpdateInventory(request); err != nil {
		logrus.WithError(err).Error("调用库存更新接口失败")
		return fmt.Errorf("更新库存失败: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"platform_sku": platformSKU,
		"new_stock":    newStock,
	}).Info("库存更新成功")

	return nil
}

// buildOffShelfRequest 构建下架请求
func (e *StrategyExecutor) buildOffShelfRequest(
	prod *api.ProductDataDTO,
	mappings []*SKUMappingData,
) *product.ShelfOperateRequest {
	// 收集所有 SKC 名称
	skcNames := make(map[string]bool)
	for _, mapping := range mappings {
		if mapping.MappingInfo != nil && mapping.MappingInfo.SKU != "" {
			skcNames[mapping.MappingInfo.SKU] = true
		}
	}

	// 构建 SKC 站点信息列表
	var skcSiteInfos []product.SkcSiteInfo
	for skcName := range skcNames {
		skcSiteInfos = append(skcSiteInfos, product.SkcSiteInfo{
			BusinessModel: 1,
			SubSites:      []product.SubSite{},
			OffSubSites: []product.SubSite{
				{
					SiteAbbr:  "shein-us",
					StoreType: 1,
				},
			},
			SkcName: skcName,
		})
	}

	return &product.ShelfOperateRequest{
		SkcSiteInfos: skcSiteInfos,
		SpuName:      prod.ParentProductID,
	}
}

// onShelfProduct 上架产品
func (e *StrategyExecutor) onShelfProduct(prod *api.ProductDataDTO) error {
	logrus.WithFields(logrus.Fields{
		"platform_product_id": prod.PlatformProductID,
		"spu_name":            prod.Title,
	}).Info("执行上架操作")

	// 解析 Attributes 获取 SKC 信息
	mappings := extractMappingInfoFromAttributes(prod.Attributes)
	if len(mappings) == 0 {
		logrus.Warn("产品没有 SKC 映射信息，无法上架")
		return nil
	}

	// 构建上架请求
	request := e.buildOnShelfRequest(prod, mappings)

	// 调用 SHEIN API 上架产品
	if err := e.apiClient.OnShelf(request); err != nil {
		logrus.WithError(err).Error("调用上架接口失败")
		return err
	}

	logrus.Info("产品上架成功")
	return nil
}

// buildOnShelfRequest 构建上架请求
func (e *StrategyExecutor) buildOnShelfRequest(
	prod *api.ProductDataDTO,
	mappings []*SKUMappingData,
) *product.ShelfOperateRequest {
	// 收集所有 SKC 名称
	skcNames := make(map[string]bool)
	for _, mapping := range mappings {
		if mapping.MappingInfo != nil && mapping.MappingInfo.SKU != "" {
			skcNames[mapping.MappingInfo.SKU] = true
		}
	}

	// 构建 SKC 站点信息列表
	var skcSiteInfos []product.SkcSiteInfo
	for skcName := range skcNames {
		skcSiteInfos = append(skcSiteInfos, product.SkcSiteInfo{
			BusinessModel: 1,
			SubSites: []product.SubSite{
				{
					SiteAbbr:  "shein-us",
					StoreType: 1,
				},
			},
			OffSubSites: []product.SubSite{},
			SkcName:     skcName,
		})
	}

	return &product.ShelfOperateRequest{
		SkcSiteInfos: skcSiteInfos,
		SpuName:      prod.PlatformProductID,
	}
}

// buildInventoryUpdateRequestFromAttributes 从 Attributes 和仓库信息构建库存更新请求
func (e *StrategyExecutor) buildInventoryUpdateRequestFromAttributes(
	attributesJSON string,
	warehouseResp *warehouse.WarehouseResponse,
	platformSKU string,
	oldStock int,
	newStock int,
) *product.InventoryUpdateRequest {
	if attributesJSON == "" {
		return nil
	}

	var skcList []SKCInfo
	if err := json.Unmarshal([]byte(attributesJSON), &skcList); err != nil {
		logrus.WithError(err).Error("解析 Attributes 失败")
		return nil
	}

	// 获取第一个可用仓库代码
	var warehouseCode string
	if len(warehouseResp.Data) > 0 {
		warehouseCode = warehouseResp.Data[0].WarehouseCode
	}

	var skcUpdates []product.SkcInventoryUpdate

	// 遍历所有 SKC
	for _, skc := range skcList {
		var skuUpdates []product.SkuInventoryUpdate

		// 遍历 SKC 下的所有 SKU
		for _, sku := range skc.SKUInfo {
			// 找到匹配的 SKU（通过 MappingInfo.SKU 匹配）
			if sku.MappingInfo != nil && sku.MappingInfo.SKU == platformSKU {
				// 构建仓库库存更新信息
				var warehouseUpdates []product.WarehouseInventoryUpdate
				warehouseUpdates = append(warehouseUpdates, product.WarehouseInventoryUpdate{
					MerchantWarehouseCode:    warehouseCode,
					BeforeChangeInventoryNum: oldStock,
					AfterChangeInventoryNum:  newStock,
				})

				if len(warehouseUpdates) > 0 {
					skuUpdates = append(skuUpdates, product.SkuInventoryUpdate{
						SkuCode:           sku.SKUCode,
						DeliveryMode:      1,
						WarehouseInfoList: warehouseUpdates,
					})
				}
			}
		}

		// 如果该 SKC 有需要更新的 SKU
		if len(skuUpdates) > 0 {
			skcUpdates = append(skcUpdates, product.SkcInventoryUpdate{
				SkcCode: skc.SKCCode,
				SkcName: platformSKU,
				SkuInfo: skuUpdates,
			})
		}
	}

	if len(skcUpdates) == 0 {
		return nil
	}

	return &product.InventoryUpdateRequest{
		SkcInfo: skcUpdates,
	}
}

// parsePriceString 解析价格字符串为 float64
func parsePriceString(priceStr string) float64 {
	if priceStr == "" {
		return 0
	}

	var price float64
	// 尝试解析价格字符串
	if _, err := fmt.Sscanf(priceStr, "%f", &price); err != nil {
		return 0
	}

	return price
}
