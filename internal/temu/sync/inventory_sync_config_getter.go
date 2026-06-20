// package sync 提供TEMU平台调度器相关服务
package sync

// getMinProfitRateThreshold 获取最低利润率阈值（优先从运营策略获取）
func (s *inventorySyncServiceImpl) getMinProfitRateThreshold(storeID int64) float64 {
	// 尝试从运行时获取运营策略
	if s.runtime == nil {
		return 0
	}
	strategy, err := s.runtime.GetRuntimeOperationStrategy(storeID)
	if err == nil && strategy != nil && strategy.IsEnabled() {
		if strategy.MinProfitRate > 0 {
			// 数据格式转换：如果值大于1，认为是百分比形式（如10表示10%），需要转换为小数形式
			if strategy.MinProfitRate > 1 {
				return strategy.MinProfitRate / 100.0
			}
			return strategy.MinProfitRate
		}
	}

	return 0
}

// getStockChangeThreshold 获取库存变化阈值（优先从店铺级策略获取）
func (s *inventorySyncServiceImpl) getStockChangeThreshold(storeID int64) int {
	// 尝试从运行时获取店铺级策略
	if s.runtime == nil {
		return 5
	}
	strategy, err := s.runtime.GetRuntimeOperationStrategy(storeID)
	if err == nil && strategy != nil && strategy.IsEnabled() {
		if strategy.StockChangeThreshold > 0 {
			return strategy.StockChangeThreshold
		}
	}

	return 5 // 默认5个
}
