// Package scheduler 提供SHEIN平台产品数据处理工具
package scheduler

import (
	"encoding/json"

	"task-processor/internal/pkg/management"

	"github.com/sirupsen/logrus"
)

// ProductDataHelper 产品数据处理助手
type ProductDataHelper struct {
	managementClient *management.ClientManager
	logger           *logrus.Logger
}

// NewProductDataHelper 创建产品数据处理助手
func NewProductDataHelper(managementClient *management.ClientManager, logger *logrus.Logger) *ProductDataHelper {
	return &ProductDataHelper{
		managementClient: managementClient,
		logger:           logger,
	}
}

// BuildSkcDataMap 构建SKC到EnrichedSkcInfo的映射
func (h *ProductDataHelper) BuildSkcDataMap(storeID int64) (map[string]*EnrichedSkcInfo, error) {
	productClient := h.managementClient.GetProductDataClient(storeID)
	shelfStatus := 2 // 2表示在售状态
	allProducts, err := productClient.ListByStore("SHEIN", 0, storeID, &shelfStatus)
	if err != nil {
		h.logger.WithError(err).Warn("获取店铺产品列表失败")
		return nil, err
	}

	// 构建 SKC -> EnrichedSkcInfo 的映射
	skcDataMap := make(map[string]*EnrichedSkcInfo)
	for _, prod := range allProducts {
		if prod.Attributes == "" {
			continue
		}

		// 解析Attributes,提取每个SKC的数据
		var skcList []EnrichedSkcInfo
		if err := json.Unmarshal([]byte(prod.Attributes), &skcList); err != nil {
			h.logger.WithError(err).Debugf("解析产品Attributes失败: %s", prod.ProductID)
			continue
		}

		// 为每个SKC建立映射
		for i := range skcList {
			if skcList[i].SkcName != "" {
				skcDataMap[skcList[i].SkcName] = &skcList[i]
			}
		}
	}

	h.logger.Infof("构建了 %d 个SKC的数据映射", len(skcDataMap))
	return skcDataMap, nil
}

// BuildSkcAttributesMap 构建SKC到Attributes字符串的映射
func (h *ProductDataHelper) BuildSkcAttributesMap(storeID int64) (map[string]string, error) {
	productClient := h.managementClient.GetProductDataClient(storeID)
	shelfStatus := 2 // 2表示在售状态
	allProducts, err := productClient.ListByStore("SHEIN", 0, storeID, &shelfStatus)
	if err != nil {
		h.logger.WithError(err).Warn("获取店铺产品列表失败")
		return nil, err
	}

	// 构建 SKC -> Attributes 的映射
	skcAttributesMap := make(map[string]string)
	for _, prod := range allProducts {
		if prod.Attributes == "" {
			continue
		}

		// 解析Attributes,提取每个SKC的数据
		var skcList []EnrichedSkcInfo
		if err := json.Unmarshal([]byte(prod.Attributes), &skcList); err != nil {
			h.logger.WithError(err).Debugf("解析产品Attributes失败: %s", prod.ProductID)
			continue
		}

		// 为每个SKC建立映射，使用SkcName作为键
		for i := range skcList {
			if skcList[i].SkcName != "" {
				skcAttributesMap[skcList[i].SkcName] = prod.Attributes
			}
		}
	}

	h.logger.Infof("从管理系统获取了 %d 个产品，构建了 %d 个SKC映射", len(allProducts), len(skcAttributesMap))
	return skcAttributesMap, nil
}

// ExtractAmazonPriceFromSkcData 从EnrichedSkcInfo中提取Amazon价格
func (h *ProductDataHelper) ExtractAmazonPriceFromSkcData(skcData *EnrichedSkcInfo) float64 {
	if skcData == nil {
		return 0
	}

	// 遍历SKU，查找Amazon监控数据
	for _, sku := range skcData.SkuInfo {
		if sku.AmazonMonitorData != nil && sku.AmazonMonitorData.Price > 0 {
			h.logger.Debugf("产品 [%s] 找到Amazon价格: %.2f (ASIN: %s)",
				skcData.SkcCode, sku.AmazonMonitorData.Price, sku.AmazonMonitorData.ASIN)
			return sku.AmazonMonitorData.Price
		}
	}

	return 0
}

// ExtractAmazonPriceFromAttributes 从Attributes JSON字符串中提取指定SKC的Amazon价格
func (h *ProductDataHelper) ExtractAmazonPriceFromAttributes(attributesJSON string, skcCode string) float64 {
	if attributesJSON == "" {
		return 0
	}

	var skcList []EnrichedSkcInfo
	if err := json.Unmarshal([]byte(attributesJSON), &skcList); err != nil {
		h.logger.WithField("skcCode", skcCode).WithError(err).Debug("解析产品Attributes失败")
		return 0
	}

	// 查找对应的SKC
	for _, skc := range skcList {
		if skc.SkcName == skcCode {
			// 使用现有的方法提取Amazon价格
			return h.ExtractAmazonPriceFromSkcData(&skc)
		}
	}

	h.logger.WithField("skcCode", skcCode).Debug("产品未找到Amazon价格")
	return 0
}

// ExtractSkcInfoFromAttributes 从Attributes JSON字符串中提取指定SKC的完整信息
func (h *ProductDataHelper) ExtractSkcInfoFromAttributes(attributesJSON string, skcCode string) *EnrichedSkcInfo {
	if attributesJSON == "" {
		return nil
	}

	var skcList []EnrichedSkcInfo
	if err := json.Unmarshal([]byte(attributesJSON), &skcList); err != nil {
		h.logger.WithField("skcCode", skcCode).WithError(err).Debug("解析产品Attributes失败")
		return nil
	}

	// 查找对应的SKC
	for _, skc := range skcList {
		if skc.SkcName == skcCode {
			return &skc
		}
	}

	h.logger.WithField("skcCode", skcCode).Debug("产品未找到SKC信息")
	return nil
}
