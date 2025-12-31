// Package shein 提供SHEIN平台的库存更新功能
package shein

import (
	"fmt"
	shops "task-processor/internal/common/shein"
	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// StockUpdater 库存更新器
type StockUpdater struct {
	strategy       *api.OperationStrategyDTO
	apiClient      *shops.ShopAPIClient
	requestBuilder *StrategyRequestBuilder
}

// NewStockUpdater 创建库存更新器
func NewStockUpdater(strategy *api.OperationStrategyDTO, apiClient *shops.ShopAPIClient) *StockUpdater {
	return &StockUpdater{
		strategy:       strategy,
		apiClient:      apiClient,
		requestBuilder: NewStrategyRequestBuilder(),
	}
}

// UpdateStock 更新库存
func (u *StockUpdater) UpdateStock(
	prod *api.ProductDataDTO,
	skuMapping *SKUMappingData,
	newStock int,
) error {
	// 应用库存更新比例
	if u.strategy.StockUpdateRatio > 0 {
		newStock = int(float64(newStock) * u.strategy.StockUpdateRatio)
	}

	platformSKU := skuMapping.MappingInfo.SKU
	oldStock := skuMapping.Stock

	logrus.WithFields(logrus.Fields{
		"platform_sku": platformSKU,
		"old_stock":    oldStock,
		"new_stock":    newStock,
	}).Info("执行库存更新操作")

	// 查询商户仓库信息获取仓库代码
	warehouseResp, err := u.apiClient.GetWarehouses()
	if err != nil {
		logrus.WithError(err).Error("查询商户仓库信息失败")
		return fmt.Errorf("查询商户仓库信息失败: %w", err)
	}

	if len(warehouseResp.Data) == 0 {
		return fmt.Errorf("未找到可用的仓库")
	}

	// 从 Attributes 构建库存更新请求
	request := u.requestBuilder.BuildInventoryUpdateRequestFromAttributes(
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
	if err := u.apiClient.UpdateInventory(request); err != nil {
		logrus.WithError(err).Error("调用库存更新接口失败")
		return fmt.Errorf("更新库存失败: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"platform_sku": platformSKU,
		"new_stock":    newStock,
	}).Info("库存更新成功")

	return nil
}
