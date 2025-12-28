// Package shein 提供SHEIN平台价格管理功能
package shein

import (
	"fmt"
	"task-processor/internal/common/management/api"
	shops "task-processor/internal/common/shein"
	"task-processor/internal/common/shein/api/product"

	"github.com/sirupsen/logrus"
)

// PriceManager SHEIN价格管理器
type PriceManager struct {
	logger *logrus.Entry
}

// NewPriceManager 创建新的价格管理器
func NewPriceManager() *PriceManager {
	return &PriceManager{
		logger: logrus.WithField("component", "PriceManager"),
	}
}

// ProcessPriceByShopType 根据店铺类型处理价格信息
func (m *PriceManager) ProcessPriceByShopType(apiClient *shops.ShopAPIClient, sheinProduct *SheinProductResponse, productData *api.ProductDataDTO, shopType string) (map[string]*product.SkuPriceInfo, map[string]*product.SkuCostInfo, error) {
	var priceMap map[string]*product.SkuPriceInfo
	var costMap map[string]*product.SkuCostInfo

	switch shopType {
	case "0":
		// 半托管店铺：查询成本价
		var err error
		costMap, err = m.FetchCostPriceInfo(apiClient, sheinProduct)
		if err != nil {
			m.logger.WithError(err).WithField("spu_name", sheinProduct.SpuName).Warn("获取成本价信息失败")
			costMap = make(map[string]*product.SkuCostInfo)
		} else {
			// 填充产品级别的成本价
			m.FillProductLevelCostPrice(productData, costMap)
		}
		priceMap = make(map[string]*product.SkuPriceInfo)

	case "2":
		// 自营店铺：查询价格
		var err error
		priceMap, err = m.FetchPriceInfo(apiClient, sheinProduct)
		if err != nil {
			m.logger.WithError(err).WithField("spu_name", sheinProduct.SpuName).Warn("获取价格信息失败")
			priceMap = make(map[string]*product.SkuPriceInfo)
		} else {
			// 填充产品级别的价格
			m.FillProductLevelPrice(productData, priceMap)
		}
		costMap = make(map[string]*product.SkuCostInfo)

	default:
		// 全托管或其他类型，暂不处理价格
		m.logger.WithField("shop_type", shopType).Debug("全托管店铺暂不处理价格")
		priceMap = make(map[string]*product.SkuPriceInfo)
		costMap = make(map[string]*product.SkuCostInfo)
	}

	return priceMap, costMap, nil
}

// FetchPriceInfo 获取产品价格信息，返回 SKU 级别的完整价格数据（自营店铺）
func (m *PriceManager) FetchPriceInfo(apiClient *shops.ShopAPIClient, sheinProduct *SheinProductResponse) (map[string]*product.SkuPriceInfo, error) {
	// 查询价格信息
	priceResponse, err := apiClient.QueryPrice(sheinProduct.SpuName)
	if err != nil {
		return nil, fmt.Errorf("查询价格失败: %w", err)
	}

	if len(priceResponse.Info.Data) == 0 {
		return nil, fmt.Errorf("未获取到价格数据")
	}

	// 构建 SKU Code 到完整价格信息的映射
	priceMap := make(map[string]*product.SkuPriceInfo)

	for _, skcPrice := range priceResponse.Info.Data {
		for _, skuPrice := range skcPrice.SkuInfoList {
			// 保存完整的 SKU 价格信息
			skuPriceCopy := skuPrice
			priceMap[skuPrice.SkuCode] = &skuPriceCopy
		}
	}

	m.logger.WithFields(logrus.Fields{
		"spu_name":    sheinProduct.SpuName,
		"price_count": len(priceMap),
	}).Debug("成功获取 SKU 级别的完整价格信息（自营店铺）")

	return priceMap, nil
}

// FetchCostPriceInfo 获取产品成本价信息，返回 SKU 级别的完整成本数据（半托店铺）
func (m *PriceManager) FetchCostPriceInfo(apiClient *shops.ShopAPIClient, sheinProduct *SheinProductResponse) (map[string]*product.SkuCostInfo, error) {
	// 构建 SKC 名称列表
	var skcNameList []string
	for _, skc := range sheinProduct.SkcInfoList {
		skcNameList = append(skcNameList, skc.SkcName)
	}

	// 查询成本价信息
	costResponse, err := apiClient.QueryCostPrice(sheinProduct.SpuName, skcNameList)
	if err != nil {
		return nil, fmt.Errorf("查询成本价失败: %w", err)
	}

	if len(costResponse.Info.Data) == 0 {
		return nil, fmt.Errorf("未获取到成本价数据")
	}

	// 构建 SKU Code 到完整成本信息的映射
	costMap := make(map[string]*product.SkuCostInfo)

	for _, skcCost := range costResponse.Info.Data {
		for _, skuCost := range skcCost.SkuCostInfoList {
			// 保存完整的 SKU 成本信息
			skuCostCopy := skuCost
			costMap[skuCost.SkuCode] = &skuCostCopy
		}
	}

	m.logger.WithFields(logrus.Fields{
		"spu_name":   sheinProduct.SpuName,
		"cost_count": len(costMap),
	}).Debug("成功获取 SKU 级别的完整成本价信息（半托店铺）")

	return costMap, nil
}

// FillProductLevelPrice 填充产品级别的价格（使用第一个 SKU 的价格）- 自营店铺
func (m *PriceManager) FillProductLevelPrice(productData *api.ProductDataDTO, priceMap map[string]*product.SkuPriceInfo) {
	// 使用第一个有价格的 SKU 作为产品级别的价格
	for _, skuPriceInfo := range priceMap {
		if len(skuPriceInfo.PriceInfoList) > 0 {
			priceDetail := skuPriceInfo.PriceInfoList[0]
			productData.OriginalPrice = api.FlexibleString(fmt.Sprintf("%.2f", priceDetail.ShopPrice))
			productData.SpecialPrice = api.FlexibleString(fmt.Sprintf("%.2f", priceDetail.SpecialPrice))
			productData.PriceCurrency = priceDetail.Currency
			break // 只使用第一个
		}
	}
}

// FillProductLevelCostPrice 填充产品级别的成本价（使用第一个 SKU 的成本价）- 半托店铺
func (m *PriceManager) FillProductLevelCostPrice(productData *api.ProductDataDTO, costMap map[string]*product.SkuCostInfo) {
	// 使用第一个有成本价的 SKU 作为产品级别的价格
	for _, skuCostInfo := range costMap {
		productData.OriginalPrice = api.FlexibleString(skuCostInfo.CostPriceInfo.CostPrice)
		productData.SpecialPrice = api.FlexibleString(skuCostInfo.CostPriceInfo.CostPrice)
		productData.PriceCurrency = skuCostInfo.CostPriceInfo.Currency
		break // 只使用第一个
	}
}
