// Package syncsvc 提供TEMU平台调度器相关服务
package syncsvc

import (
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/internal/domain/model"
	productpkg "task-processor/internal/domain/product"
	"task-processor/internal/pkg/jsonx"

	"github.com/sirupsen/logrus"
)

// extractMappingInfoFromAttributes 从Attributes JSON中提取所有映射信息和库存
func (s *inventorySyncServiceImpl) extractMappingInfoFromAttributes(attributesJSON string) []*TemuMappingData {
	if attributesJSON == "" {
		s.logger.Debug("产品Attributes为空")
		return nil
	}

	var mappingList []*TemuMappingData
	if err := jsonx.UnmarshalString(attributesJSON, &mappingList, "解析产品Attributes JSON失败"); err != nil {
		s.logger.WithError(err).WithField("attributes_length", len(attributesJSON)).Error(err.Error())
		return nil
	}

	return mappingList
}

// extractStockFromProduct 从Amazon产品中提取库存（使用公共函数）
func (s *inventorySyncServiceImpl) extractStockFromProduct(amazonProduct *model.Product) int {
	return productpkg.GetInventory(amazonProduct)
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

	// 将 Region 转换为 TEMU 站点缩写格式（如：US -> temu-us）
	siteAbbr := fmt.Sprintf("temu-%s", strings.ToLower(storeInfo.Region))

	return siteAbbr, nil
}

// checkHasAmazonMonitorData 检查产品Attributes中是否有Amazon监控数据
func (s *inventorySyncServiceImpl) checkHasAmazonMonitorData(attributesJSON string, platformSKU string) bool {
	if attributesJSON == "" {
		return false
	}

	var skuList []TemuSkuInfo
	if err := jsonx.UnmarshalString(attributesJSON, &skuList, "解析产品Attributes失败"); err != nil {
		s.logger.WithError(err).Debug(err.Error())
		return false
	}

	// 查找对应的SKU并检查是否有映射信息
	for _, sku := range skuList {
		if s.getStringValue(sku.MappingInfo.Sku) == platformSKU {
			// 找到对应的SKU，TEMU结构中直接检查映射信息是否存在
			s.logger.WithFields(map[string]any{
				"platform_sku": platformSKU,
				"asin":         sku.MappingInfo.ProductId,
			}).Debug("找到SKU映射信息")
			return true
		}
	}

	return false
}

// getAmazonMonitorLastCheckTime 获取Amazon监控数据的最后检查时间
// TEMU结构中没有LastCheckTime字段，始终返回0表示需要检查
func (s *inventorySyncServiceImpl) getAmazonMonitorLastCheckTime(_ string, _ string) int64 {
	return 0
}

// validateAttributesStructure 验证 Attributes JSON 结构
func (s *inventorySyncServiceImpl) validateAttributesStructure(attributesJSON string) error {
	if attributesJSON == "" {
		return fmt.Errorf("attributes JSON 为空")
	}

	// 尝试解析为通用 JSON 结构
	var genericData any
	if err := json.Unmarshal([]byte(attributesJSON), &genericData); err != nil {
		return fmt.Errorf("JSON 格式无效: %w", err)
	}

	// 尝试解析为 SKUMappingData 数组
	var skuList []TemuSkuInfo
	if err := jsonx.UnmarshalString(attributesJSON, &skuList, "无法解析为 SKUMappingData 数组"); err != nil {
		return err
	}

	return nil
}

// enableDebugLogging 启用调试日志（用于问题排查）
func (s *inventorySyncServiceImpl) enableDebugLogging() {
	logrus.SetLevel(logrus.DebugLevel)
	s.logger.Debug("已启用 Debug 级别日志")
}
