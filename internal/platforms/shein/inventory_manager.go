// Package shein 提供SHEIN平台库存管理功能
package shein

import (
	"fmt"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/product"

	"github.com/sirupsen/logrus"
)

// InventoryManager SHEIN库存管理器
type InventoryManager struct {
	logger *logrus.Entry
}

// NewInventoryManager 创建新的库存管理器
func NewInventoryManager() *InventoryManager {
	return &InventoryManager{
		logger: logrus.WithField("component", "InventoryManager"),
	}
}

// FetchInventoryInfo 获取产品的 SKU 级别库存信息
func (m *InventoryManager) FetchInventoryInfo(apiClient *ShopAPIClient, sheinProduct *SheinProductResponse) (*LocalInventoryInfo, error) {
	// 调用库存详情查询 API
	response, err := apiClient.QueryInventory(sheinProduct.SpuName)
	if err != nil {
		return nil, fmt.Errorf("调用库存查询 API 失败: %w", err)
	}

	// 将响应转换为本地类型
	localInfo := &LocalInventoryInfo{
		SpuName:            response.Info.SpuName,
		ProductNameCh:      response.Info.ProductNameCh,
		MainImageThumbnail: response.Info.MainImageThumbnail,
		IfFbmStore:         response.Info.IfFbmStore,
		SkcInfo:            make([]LocalSkcInventory, len(response.Info.SkcInfo)),
	}

	// 转换 SKC 信息
	for i, skcInfo := range response.Info.SkcInfo {
		localInfo.SkcInfo[i] = LocalSkcInventory{
			SkcName:   skcInfo.SkcName,
			SortOrder: skcInfo.SortOrder,
			SkcCode:   skcInfo.SkcCode,
			SaleName:  skcInfo.SaleName,
			SkuInfo:   make([]LocalSkuInventory, len(skcInfo.SkuInfo)),
		}

		// 转换 SKU 信息
		for j, skuInfo := range skcInfo.SkuInfo {
			localInfo.SkcInfo[i].SkuInfo[j] = LocalSkuInventory{
				SkuCode:       skuInfo.SkuCode,
				SkuName:       skuInfo.SkuCode, // 使用 SkuCode 作为 SkuName
				InventoryInfo: make([]LocalWarehouseInventory, len(skuInfo.InventoryInfo)),
			}

			// 转换仓库库存信息
			for k, warehouseInfo := range skuInfo.InventoryInfo {
				localInfo.SkcInfo[i].SkuInfo[j].InventoryInfo[k] = LocalWarehouseInventory{
					WarehouseCode:     warehouseInfo.MerchantWarehouseCode,
					UsableInventory:   warehouseInfo.UsableInventory,
					InventoryQuantity: warehouseInfo.InventoryQuantity,
				}
			}
		}
	}

	m.logger.WithFields(logrus.Fields{
		"spu_name":  sheinProduct.SpuName,
		"skc_count": len(localInfo.SkcInfo),
	}).Debug("成功获取 SKU 级别库存信息")

	return localInfo, nil
}

// FillProductLevelInventory 填充产品级别的库存信息
func (m *InventoryManager) FillProductLevelInventory(productData *api.ProductDataDTO, inventoryInfo *LocalInventoryInfo) {
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

	m.logger.WithFields(logrus.Fields{
		"spu_name":         inventoryInfo.SpuName,
		"usable_inventory": totalUsable,
		"total_inventory":  totalInventory,
	}).Debug("填充产品级别库存信息")
}

// BuildSkuInventoryMap 构建 SKU Code 到库存信息的映射
func (m *InventoryManager) BuildSkuInventoryMap(inventoryInfo *LocalInventoryInfo) map[string]*LocalSkuInventory {
	skuInventoryMap := make(map[string]*LocalSkuInventory)
	if inventoryInfo != nil {
		for _, skcInv := range inventoryInfo.SkcInfo {
			for i := range skcInv.SkuInfo {
				skuInv := &skcInv.SkuInfo[i]
				skuInventoryMap[skuInv.SkuCode] = skuInv
			}
		}
	}
	return skuInventoryMap
}

// ConvertToProductInventoryInfo 将本地库存信息转换为产品包的库存信息
func (m *InventoryManager) ConvertToProductInventoryInfo(localInfo *LocalInventoryInfo) *product.InventoryInfo {
	if localInfo == nil {
		return nil
	}

	productInfo := &product.InventoryInfo{
		SpuName:            localInfo.SpuName,
		ProductNameCh:      localInfo.ProductNameCh,
		MainImageThumbnail: localInfo.MainImageThumbnail,
		IfFbmStore:         localInfo.IfFbmStore,
		SkcInfo:            make([]product.SkcInventory, len(localInfo.SkcInfo)),
	}

	// 转换 SKC 信息
	for i, localSkc := range localInfo.SkcInfo {
		productInfo.SkcInfo[i] = product.SkcInventory{
			SkcName:   localSkc.SkcName,
			SortOrder: localSkc.SortOrder,
			SkcCode:   localSkc.SkcCode,
			SaleName:  localSkc.SaleName,
			SkuInfo:   make([]product.SkuInventory, len(localSkc.SkuInfo)),
		}

		// 转换 SKU 信息
		for j, localSku := range localSkc.SkuInfo {
			productInfo.SkcInfo[i].SkuInfo[j] = product.SkuInventory{
				SkuCode:       localSku.SkuCode,
				InventoryInfo: make([]product.WarehouseInventory, len(localSku.InventoryInfo)),
			}

			// 转换仓库库存信息
			for k, localWarehouse := range localSku.InventoryInfo {
				productInfo.SkcInfo[i].SkuInfo[j].InventoryInfo[k] = product.WarehouseInventory{
					MerchantWarehouseCode: localWarehouse.WarehouseCode,
					UsableInventory:       localWarehouse.UsableInventory,
					InventoryQuantity:     localWarehouse.InventoryQuantity,
				}
			}
		}
	}

	return productInfo
}
