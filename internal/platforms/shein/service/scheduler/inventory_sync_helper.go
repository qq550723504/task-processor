// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
)

// extractMappingInfoFromAttributes 从Attributes JSON中提取所有映射信息和库存
func (s *inventorySyncServiceImpl) extractMappingInfoFromAttributes(attributesJSON string) []*SKUMappingData {
	if attributesJSON == "" {
		return nil
	}

	var skcList []EnrichedSkcInfo
	if err := json.Unmarshal([]byte(attributesJSON), &skcList); err != nil {
		return nil
	}

	var mappings []*SKUMappingData
	for _, skc := range skcList {
		for _, sku := range skc.SkuInfo {
			if sku.MappingInfo != nil && sku.MappingInfo.ProductId != "" {
				totalStock := 0
				if sku.UsableInventory != nil {
					totalStock = *sku.UsableInventory
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

// extractStockFromProduct 从Amazon产品中提取库存（使用公共函数）
func (s *inventorySyncServiceImpl) extractStockFromProduct(amazonProduct *model.Product) int {
	return product.GetInventory(amazonProduct)
}

// parsePrice 解析价格字符串
func (s *inventorySyncServiceImpl) parsePrice(price string) float64 {
	if price == "" {
		return 0.0
	}
	var result float64
	fmt.Sscanf(price, "%f", &result)
	return result
}

// abs 返回浮点数的绝对值
func (s *inventorySyncServiceImpl) abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// absInt 返回整数的绝对值
func (s *inventorySyncServiceImpl) absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// getStringValue 安全获取字符串指针的值
func (s *inventorySyncServiceImpl) getStringValue(str *string) string {
	if str == nil {
		return ""
	}
	return *str
}

// getFloatValue 安全获取浮点数指针的值
func (s *inventorySyncServiceImpl) getFloatValue(f *float64) float64 {
	if f == nil {
		return 0.0
	}
	return *f
}

// getStorePriceType 获取店铺配置的价格类型
func (s *inventorySyncServiceImpl) getStorePriceType(storeID int64) string {
	// 从管理系统获取店铺信息
	storeInfo, err := s.managementClient.GetStoreClient().GetStore(storeID)
	if err != nil {
		s.logger.WithError(err).WithField("store_id", storeID).Warn("获取店铺信息失败，使用默认价格类型")
		return "special" // 默认使用特价
	}

	if storeInfo.PriceType != "" {
		return storeInfo.PriceType
	}

	return "special" // 默认使用特价
}

// getStoreSiteAbbr 获取店铺配置的站点缩写
func (s *inventorySyncServiceImpl) getStoreSiteAbbr(storeID int64) (string, error) {
	// 从管理系统获取店铺信息
	storeInfo, err := s.managementClient.GetStoreClient().GetStore(storeID)
	if err != nil {
		return "", fmt.Errorf("获取店铺信息失败: %w", err)
	}

	// Region 字段存储站点信息（如：US, UK, JP 等）
	if storeInfo.Region == "" {
		return "", fmt.Errorf("店铺未配置站点信息(Region字段为空)")
	}

	// 将 Region 转换为 SHEIN 站点缩写格式（如：US -> shein-us）
	siteAbbr := fmt.Sprintf("shein-%s", strings.ToLower(storeInfo.Region))

	return siteAbbr, nil
}

// checkHasAmazonMonitorData 检查产品Attributes中是否有Amazon监控数据
func (s *inventorySyncServiceImpl) checkHasAmazonMonitorData(attributesJSON string, platformSKU string) bool {
	if attributesJSON == "" {
		return false
	}

	var skcList []EnrichedSkcInfo
	if err := json.Unmarshal([]byte(attributesJSON), &skcList); err != nil {
		s.logger.WithError(err).Debug("解析产品Attributes失败")
		return false
	}

	// 查找对应的SKU并检查是否有AmazonMonitorData
	for _, skc := range skcList {
		for _, sku := range skc.SkuInfo {
			if sku.MappingInfo != nil && s.getStringValue(sku.MappingInfo.Sku) == platformSKU {
				// 找到对应的SKU，检查是否有AmazonMonitorData
				if sku.AmazonMonitorData != nil && sku.AmazonMonitorData.LastCheckTime > 0 {
					s.logger.WithFields(map[string]interface{}{
						"platform_sku":    platformSKU,
						"asin":            sku.AmazonMonitorData.ASIN,
						"last_check_time": sku.AmazonMonitorData.LastCheckTime,
					}).Debug("找到Amazon监控数据")
					return true
				}
				return false
			}
		}
	}

	return false
}
