// Package activity 提供SHEIN平台活动报名相关服务
package activity

import (
	"context"
	"fmt"

	"task-processor/internal/listingadmin"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/shein/productsync"

	"github.com/sirupsen/logrus"
)

// ProductDataHelper 产品数据处理助手
type ProductDataHelper struct {
	productDataRepo listingadmin.ProductDataRepository
	logger          *logrus.Logger
}

// NewProductDataHelper 创建产品数据处理助手
func NewProductDataHelper(productDataRepo listingadmin.ProductDataRepository, logger *logrus.Logger) *ProductDataHelper {
	return &ProductDataHelper{
		productDataRepo: productDataRepo,
		logger:          logger,
	}
}

func (h *ProductDataHelper) listStoreProducts(storeID int64) ([]listingadmin.ProductData, error) {
	if h.productDataRepo == nil {
		return nil, fmt.Errorf("product data repository is not initialized")
	}
	shelfStatus := 2
	page, err := h.productDataRepo.ListProductData(
		context.Background(),
		listingadmin.ProductDataQuery{
			StoreID:     &storeID,
			Platform:    "SHEIN",
			ShelfStatus: &shelfStatus,
			Page:        1,
			PageSize:    10000,
		},
	)
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, nil
	}
	return page.Items, nil
}

// BuildSkcDataMap 构建SKC到EnrichedSkcInfo的映射
func (h *ProductDataHelper) BuildSkcDataMap(storeID int64) (map[string]*productsync.EnrichedSkcInfo, error) {
	allProducts, err := h.listStoreProducts(storeID)
	if err != nil {
		h.logger.WithError(err).Warn("获取店铺产品列表失败")
		return nil, err
	}

	skcDataMap := make(map[string]*productsync.EnrichedSkcInfo)
	for _, prod := range allProducts {
		attributes := string(prod.Attributes)
		if attributes == "" {
			continue
		}

		var skcList []productsync.EnrichedSkcInfo
		if err := jsonx.UnmarshalString(attributes, &skcList, ""); err != nil {
			h.logger.WithError(err).Debugf("解析产品Attributes失败: %s", prod.ProductID)
			continue
		}

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
	allProducts, err := h.listStoreProducts(storeID)
	if err != nil {
		h.logger.WithError(err).Warn("获取店铺产品列表失败")
		return nil, err
	}

	skcAttributesMap := make(map[string]string)
	for _, prod := range allProducts {
		attributes := string(prod.Attributes)
		if attributes == "" {
			continue
		}

		var skcList []productsync.EnrichedSkcInfo
		if err := jsonx.UnmarshalString(attributes, &skcList, ""); err != nil {
			h.logger.WithError(err).Debugf("解析产品Attributes失败: %s", prod.ProductID)
			continue
		}

		for i := range skcList {
			if skcList[i].SkcName != "" {
				skcAttributesMap[skcList[i].SkcName] = attributes
			}
		}
	}

	h.logger.Infof("从管理系统获取了 %d 个产品，构建了 %d 个SKC映射", len(allProducts), len(skcAttributesMap))
	return skcAttributesMap, nil
}

// ExtractAmazonPriceFromSkcData 从EnrichedSkcInfo中提取Amazon价格
func (h *ProductDataHelper) ExtractAmazonPriceFromSkcData(skcData *productsync.EnrichedSkcInfo) float64 {
	if skcData == nil {
		return 0
	}

	for _, sku := range skcData.SkuInfo {
		if sku.AmazonMonitorData != nil && sku.AmazonMonitorData.Price > 0 {
			h.logger.Debugf("产品 [%s] 找到Amazon价格: %.2f (ASIN: %s)",
				skcData.SkcCode, sku.AmazonMonitorData.Price, sku.AmazonMonitorData.ASIN)
			return sku.AmazonMonitorData.Price
		}
	}

	return 0
}

// AmazonCostBySKU returns the normalized SKU-to-Amazon-cost mapping for an SKC.
func (h *ProductDataHelper) AmazonCostBySKU(skcData *productsync.EnrichedSkcInfo) map[string]float64 {
	costs := make(map[string]float64)
	if skcData == nil {
		return costs
	}
	for _, sku := range skcData.SkuInfo {
		code := normalizePromotionSKUCode(sku.SkuCode)
		if code == "" || sku.AmazonMonitorData == nil || sku.AmazonMonitorData.Price <= 0 {
			continue
		}
		costs[code] = sku.AmazonMonitorData.Price
	}
	return costs
}

// ExtractAmazonPriceFromAttributes 从Attributes JSON字符串中提取指定SKC的Amazon价格
func (h *ProductDataHelper) ExtractAmazonPriceFromAttributes(attributesJSON string, skcCode string) float64 {
	if attributesJSON == "" {
		return 0
	}

	var skcList []productsync.EnrichedSkcInfo
	if err := jsonx.UnmarshalString(attributesJSON, &skcList, ""); err != nil {
		h.logger.WithField("skcCode", skcCode).WithError(err).Debug("解析产品Attributes失败")
		return 0
	}

	for _, skc := range skcList {
		if skc.SkcName == skcCode {
			return h.ExtractAmazonPriceFromSkcData(&skc)
		}
	}

	h.logger.WithField("skcCode", skcCode).Debug("产品未找到Amazon价格")
	return 0
}

// ExtractSkcInfoFromAttributes 从Attributes JSON字符串中提取指定SKC的完整信息
func (h *ProductDataHelper) ExtractSkcInfoFromAttributes(attributesJSON string, skcCode string) *productsync.EnrichedSkcInfo {
	if attributesJSON == "" {
		return nil
	}

	var skcList []productsync.EnrichedSkcInfo
	if err := jsonx.UnmarshalString(attributesJSON, &skcList, ""); err != nil {
		h.logger.WithField("skcCode", skcCode).WithError(err).Debug("解析产品Attributes失败")
		return nil
	}

	for _, skc := range skcList {
		if skc.SkcName == skcCode {
			return &skc
		}
	}

	h.logger.WithField("skcCode", skcCode).Debug("产品未找到SKC信息")
	return nil
}
