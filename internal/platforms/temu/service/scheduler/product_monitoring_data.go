// Package scheduler 提供TEMU产品监控数据处理功能
package scheduler

import (
	"encoding/json"
	"fmt"
	"strconv"

	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/pkg/types"
	"task-processor/internal/platforms/temu/api/models"
)

// MonitoringData 监控数据结构
type MonitoringData struct {
	PriceInfo  PriceMonitoringInfo  `json:"price_info"`
	StockInfo  StockMonitoringInfo  `json:"stock_info"`
	StatusInfo StatusMonitoringInfo `json:"status_info"`
}

// PriceMonitoringInfo 价格监控信息
type PriceMonitoringInfo struct {
	Price         float64        `json:"price"`          // 价格
	ListPrice     string         `json:"list_price"`     // 列表价格
	SupplierPrice float64        `json:"supplier_price"` // 供应商价格
	Currency      string         `json:"currency"`       // 货币
	PriceVO       models.PriceVO `json:"price_vo"`       // 价格对象
	ListPriceVO   models.PriceVO `json:"list_price_vo"`  // 列表价格对象
}

// StockMonitoringInfo 库存监控信息
type StockMonitoringInfo struct {
	OrdinaryStock   int   `json:"ordinary_stock"`    // 普通库存
	Stock           int   `json:"stock"`             // 库存
	StockDisplayTag int   `json:"stock_display_tag"` // 库存显示标签
	StockSearchTags []any `json:"stock_search_tags"` // 库存搜索标签
}

// StatusMonitoringInfo 状态监控信息
type StatusMonitoringInfo struct {
	Status4VO             int  `json:"status4_vo"`             // 主状态
	SubStatus4VO          int  `json:"sub_status4_vo"`         // 子状态
	ShowSubStatus4VO      int  `json:"show_sub_status4_vo"`    // 显示子状态
	PersonalizationStatus int  `json:"personalization_status"` // 个性化状态
	PunishTags            int  `json:"punish_tags"`            // 惩罚标签
	LowTrafficTag         int  `json:"low_traffic_tag"`        // 低流量标签
	RestrictedTrafficTag  int  `json:"restricted_traffic_tag"` // 限制流量标签
	EasyGainsTag          int  `json:"easy_gains_tag"`         // 易获得标签
	IsBooks               bool `json:"is_books"`               // 是否为书籍
}

// buildMonitoringData 构建监控数据
func (s *productSyncServiceImpl) buildMonitoringData(temuProduct *models.SkuResponse) *MonitoringData {
	return &MonitoringData{
		PriceInfo:  s.buildPriceMonitoringInfo(temuProduct),
		StockInfo:  s.buildStockMonitoringInfo(temuProduct),
		StatusInfo: s.buildStatusMonitoringInfo(temuProduct),
	}
}

// buildPriceMonitoringInfo 构建价格监控信息
func (s *productSyncServiceImpl) buildPriceMonitoringInfo(temuProduct *models.SkuResponse) PriceMonitoringInfo {
	return PriceMonitoringInfo{
		Price:         temuProduct.Price,
		ListPrice:     temuProduct.ListPrice.Amount,
		SupplierPrice: temuProduct.SupplierPrice,
		Currency:      temuProduct.Currency,
		PriceVO:       temuProduct.PriceVO,
		ListPriceVO:   temuProduct.ListPriceVO,
	}
}

// buildStockMonitoringInfo 构建库存监控信息
func (s *productSyncServiceImpl) buildStockMonitoringInfo(temuProduct *models.SkuResponse) StockMonitoringInfo {
	return StockMonitoringInfo{
		OrdinaryStock:   temuProduct.OrdinaryStock,
		Stock:           temuProduct.Stock,
		StockDisplayTag: temuProduct.StockDisplayTag,
		StockSearchTags: temuProduct.StockSearchTags,
	}
}

// buildStatusMonitoringInfo 构建状态监控信息
func (s *productSyncServiceImpl) buildStatusMonitoringInfo(temuProduct *models.SkuResponse) StatusMonitoringInfo {
	return StatusMonitoringInfo{
		Status4VO:             temuProduct.Status4VO,
		SubStatus4VO:          temuProduct.SubStatus4VO,
		ShowSubStatus4VO:      temuProduct.ShowSubStatus4VO,
		PersonalizationStatus: temuProduct.PersonalizationStatus,
		PunishTags:            temuProduct.PunishTags,
		LowTrafficTag:         temuProduct.LowTrafficTag,
		RestrictedTrafficTag:  temuProduct.RestrictedTrafficTag,
		EasyGainsTag:          temuProduct.EasyGainsTag,
		IsBooks:               temuProduct.IsBooks,
	}
}

// enrichProductWithMonitoringData 使用监控数据丰富产品信息
func (s *productSyncServiceImpl) enrichProductWithMonitoringData(productData *managementapi.ProductDataDTO, temuProduct *models.SkuResponse) {
	monitoringData := s.buildMonitoringData(temuProduct)

	// 将监控数据序列化并存储到平台数据中
	_, err := json.Marshal(monitoringData)
	if err != nil {
		s.logger.WithError(err).Warn("序列化监控数据失败")
		return
	}

	// 更新平台数据，包含监控信息
	var platformData map[string]any
	if productData.PlatformData != "" {
		if err := json.Unmarshal([]byte(productData.PlatformData), &platformData); err != nil {
			platformData = make(map[string]any)
		}
	} else {
		platformData = make(map[string]any)
	}

	platformData["monitoring_data"] = monitoringData
	platformData["last_monitoring_update"] = types.FlexibleTime{}

	updatedPlatformDataJSON, _ := json.Marshal(platformData)
	productData.PlatformData = string(updatedPlatformDataJSON)
}

// extractPriceForComparison 提取用于价格比较的标准价格
func (s *productSyncServiceImpl) extractPriceForComparison(temuProduct *models.SkuResponse) (float64, string, error) {
	// 使用Price字段
	if temuProduct.Price > 0 {
		return temuProduct.Price, temuProduct.Currency, nil
	}

	// 使用价格对象
	if temuProduct.PriceVO.Amount != "" {
		price, err := strconv.ParseFloat(temuProduct.PriceVO.Amount, 64)
		if err == nil {
			return price, temuProduct.PriceVO.Currency, nil
		}
	}

	// 使用列表价格
	if temuProduct.ListPriceVO.Amount != "" {
		price, err := strconv.ParseFloat(temuProduct.ListPriceVO.Amount, 64)
		if err == nil {
			return price, temuProduct.ListPriceVO.Currency, nil
		}
	}

	return 0, "", fmt.Errorf("无法提取有效价格")
}

// extractStockForComparison 提取用于库存比较的标准库存
func (s *productSyncServiceImpl) extractStockForComparison(temuProduct *models.SkuResponse) int {
	// 优先使用普通库存
	if temuProduct.OrdinaryStock > 0 {
		return temuProduct.OrdinaryStock
	}

	// 使用库存字段
	if temuProduct.Stock > 0 {
		return temuProduct.Stock
	}

	return 0
}

// isSignificantPriceChange 判断是否为显著的价格变化
func (s *productSyncServiceImpl) isSignificantPriceChange(oldPrice, newPrice float64, threshold float64) bool {
	if oldPrice == 0 {
		return newPrice > 0
	}

	changePercent := ((newPrice - oldPrice) / oldPrice) * 100
	return changePercent > threshold || changePercent < -threshold
}

// isSignificantStockChange 判断是否为显著的库存变化
func (s *productSyncServiceImpl) isSignificantStockChange(oldStock, newStock int, threshold int) bool {
	diff := newStock - oldStock
	return diff > threshold || diff < -threshold
}
