// Package sync 提供SHEIN平台库存管理功能
package sync

import (
	"fmt"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/product"
	"task-processor/internal/platforms/shein/model"
	"task-processor/internal/platforms/shein/repo/client"

	"github.com/sirupsen/logrus"
)

// InventoryManager 库存管理器
type InventoryManager struct {
	logger *logrus.Entry
}

// NewInventoryManager 创建新的库存管理器
func NewInventoryManager() *InventoryManager {
	return &InventoryManager{
		logger: logrus.WithField("component", "SyncInventoryManager"),
	}
}

// FetchInventoryInfo 获取产品的SKU级别库存信息
func (m *InventoryManager) FetchInventoryInfo(
	apiClient *client.APIClient,
	sheinProduct *model.SheinProductResponse,
) (*product.InventoryInfo, error) {
	// TODO: 实现库存信息获取逻辑
	// 需要调用 apiClient 的库存查询接口
	m.logger.WithField("spu_name", sheinProduct.SpuName).Debug("获取库存信息(待实现)")
	return nil, nil
}

// FillProductLevelInventory 填充产品级别的库存信息
func (m *InventoryManager) FillProductLevelInventory(
	productData *api.ProductDataDTO,
	inventoryInfo *product.InventoryInfo,
) {
	if inventoryInfo == nil || len(inventoryInfo.SkcInfo) == 0 {
		return
	}

	// 汇总所有SKU的库存
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

// ConvertToProductInventoryInfo 将库存信息转换为产品包的库存信息
func (m *InventoryManager) ConvertToProductInventoryInfo(inventoryInfo *product.InventoryInfo) *product.InventoryInfo {
	// 直接返回，因为类型已经一致
	return inventoryInfo
}
