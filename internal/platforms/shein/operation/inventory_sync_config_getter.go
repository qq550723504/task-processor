// Package scheduler 提供SHEIN平台调度器相关服务
package operation

// getMinProfitRateThreshold 获取最低利润率阈值（优先从运营策略获取）
func (s *inventorySyncServiceImpl) getMinProfitRateThreshold(storeID int64) float64 {
	// 尝试从管理系统获取运营策略
	strategy, err := s.managementClient.GetOperationStrategyClient().GetOperationStrategyByStoreId(storeID)
	if err == nil && strategy != nil && strategy.IsEnabled() {
		if strategy.MinProfitRate > 0 {
			// 数据格式转换：如果值大于1，认为是百分比形式（如10表示10%），需要转换为小数形式
			if strategy.MinProfitRate > 1 {
				return strategy.MinProfitRate / 100.0
			}
			return strategy.MinProfitRate
		}
	}

	// 使用平台配置作为默认值
	if s.monitorConfig != nil && s.monitorConfig.PriceChangeThreshold > 0 {
		// 将价格变化阈值转换为利润率阈值（简单转换）
		return s.monitorConfig.PriceChangeThreshold / 100.0
	}

	return 0.15 // 默认15%利润率
}

// getStockChangeThreshold 获取库存变化阈值（优先从店铺级策略获取）
func (s *inventorySyncServiceImpl) getStockChangeThreshold(storeID int64) int {
	// 尝试从管理系统获取店铺级策略
	strategy, err := s.managementClient.GetOperationStrategyClient().GetOperationStrategyByStoreId(storeID)
	if err == nil && strategy != nil && strategy.IsEnabled() {
		if strategy.StockChangeThreshold > 0 {
			return strategy.StockChangeThreshold
		}
	}

	// 使用平台配置作为默认值
	if s.monitorConfig != nil {
		return s.monitorConfig.StockChangeThreshold
	}
	return 5 // 默认5个
}
