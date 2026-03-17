// Package productsync 提供 SHEIN 平台商品同步功能
package productsync

import (
	"fmt"

	"task-processor/internal/shein/api/product"
)

// fetchInventoryInfo 获取产品的SKU级别库存信息
func (s *productSyncServiceImpl) fetchInventoryInfo(spuCode string) (*product.InventoryInfo, error) {
	if s.inventoryManager == nil {
		return nil, fmt.Errorf("库存管理器未初始化")
	}

	// 调用库存查询接口
	response, err := s.inventoryManager.QueryInventory(spuCode)
	if err != nil {
		return nil, fmt.Errorf("查询库存信息失败: %w", err)
	}

	if response == nil {
		return nil, nil
	}

	return &response.Info, nil
}

// fetchPriceInfo 获取产品价格信息
func (s *productSyncServiceImpl) fetchPriceInfo(spuCode string) (map[string]*product.SkuPriceInfo, error) {
	if s.priceManager == nil {
		return nil, fmt.Errorf("价格管理器未初始化")
	}

	// 调用价格查询接口
	response, err := s.priceManager.QueryPrice(spuCode)
	if err != nil {
		return nil, fmt.Errorf("查询价格信息失败: %w", err)
	}

	// 转换为 map 格式
	priceMap := make(map[string]*product.SkuPriceInfo)
	if response != nil && len(response.Info.Data) > 0 {
		for _, skcPrice := range response.Info.Data {
			for i := range skcPrice.SkuInfoList {
				skuPrice := &skcPrice.SkuInfoList[i]
				priceMap[skuPrice.SkuCode] = skuPrice
			}
		}
	}

	return priceMap, nil
}

// fetchCostPriceInfo 获取产品成本价信息
func (s *productSyncServiceImpl) fetchCostPriceInfo(sheinProduct *product.ProductListItem) (map[string]*product.SkuCostInfo, error) {
	if s.priceManager == nil {
		return nil, fmt.Errorf("价格管理器未初始化")
	}

	// 收集所有SKC名称
	skcNameList := make([]string, 0, len(sheinProduct.SkcInfoList))
	for _, skc := range sheinProduct.SkcInfoList {
		skcNameList = append(skcNameList, skc.SkcName)
	}

	if len(skcNameList) == 0 {
		return make(map[string]*product.SkuCostInfo), nil
	}

	// 调用成本价查询接口
	response, err := s.priceManager.QueryCostPrice(sheinProduct.SpuName, skcNameList)
	if err != nil {
		return nil, fmt.Errorf("查询成本价信息失败: %w", err)
	}

	// 转换为 map 格式
	costMap := make(map[string]*product.SkuCostInfo)
	if response != nil && len(response.Info.Data) > 0 {
		for _, skcCost := range response.Info.Data {
			for i := range skcCost.SkuCostInfoList {
				skuCost := &skcCost.SkuCostInfoList[i]
				costMap[skuCost.SkuCode] = skuCost
			}
		}
	}

	return costMap, nil
}
