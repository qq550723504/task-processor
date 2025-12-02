package shein

import (
	"encoding/json"
	"fmt"
)

// absInt 返回 int 的绝对值
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// abs 返回浮点数的绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// parseStock 解析库存字符串为整数
func parseStock(stock string) int {
	if stock == "" {
		return 0
	}
	var result int
	fmt.Sscanf(stock, "%d", &result)
	return result
}

// parsePrice 解析价格字符串为浮点数
func parsePrice(price string) float64 {
	if price == "" {
		return 0.0
	}
	var result float64
	fmt.Sscanf(price, "%f", &result)
	return result
}

// getStringValue 安全获取字符串指针的值
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// MappingInfo 映射信息结构
type MappingInfo struct {
	ID            int64   `json:"id"`
	SKU           string  `json:"sku"`
	Region        string  `json:"region"`
	ProductID     string  `json:"productId"`
	CostPrice     float64 `json:"costPrice"`
	StoreID       int64   `json:"storeId"`
	Platform      string  `json:"platform"`
	TenantID      int64   `json:"tenantId"`
	ParentProduct string  `json:"parentProductId"`
}

// InventoryInfo 库存信息结构
type InventoryInfo struct {
	InventoryNum    int `json:"inventory_num"`
	UsableInventory int `json:"usable_inventory"`
}

// AmazonMonitorData Amazon 监控数据结构
type AmazonMonitorData struct {
	ASIN          string  `json:"asin"`
	Price         float64 `json:"price"`
	Stock         int     `json:"stock"`
	LastCheckTime int64   `json:"last_check_time"` // Unix 时间戳（秒）
}

// SKUInfo SKU 信息结构
type SKUInfo struct {
	SKUCode           string             `json:"sku_code"`
	MappingInfo       *MappingInfo       `json:"mapping_info"`
	InventoryInfo     []InventoryInfo    `json:"inventory_info"`
	UsableInventory   int                `json:"usable_inventory"`
	AmazonMonitorData *AmazonMonitorData `json:"amazon_monitor_data,omitempty"` // Amazon 监控数据
}

// SKCInfo SKC 信息结构
type SKCInfo struct {
	SKCCode string    `json:"skc_code"`
	SKUInfo []SKUInfo `json:"sku_info"`
}

// SKUMappingData SKU 映射数据（包含映射信息和库存）
type SKUMappingData struct {
	MappingInfo *MappingInfo
	Stock       int
}

// extractMappingInfoFromAttributes 从 Attributes JSON 中提取所有映射信息和库存
func extractMappingInfoFromAttributes(attributesJSON string) []*SKUMappingData {
	if attributesJSON == "" {
		return nil
	}

	var skcList []SKCInfo
	if err := json.Unmarshal([]byte(attributesJSON), &skcList); err != nil {
		return nil
	}

	// 收集所有有效的映射信息和库存
	var mappings []*SKUMappingData
	for _, skc := range skcList {
		for _, sku := range skc.SKUInfo {
			if sku.MappingInfo != nil && sku.MappingInfo.ProductID != "" {
				// 计算总库存
				totalStock := sku.UsableInventory
				if totalStock == 0 && len(sku.InventoryInfo) > 0 {
					for _, inv := range sku.InventoryInfo {
						totalStock += inv.UsableInventory
					}
				}

				mappings = append(mappings, &SKUMappingData{
					MappingInfo: sku.MappingInfo,
					Stock:       totalStock,
				})
			}
		}
	}

	return mappings
}
