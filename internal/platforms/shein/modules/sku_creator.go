// Package modules 提供SHEIN平台SKU创建功能
package modules

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"task-processor/internal/platforms/shein/api/attribute"
	"task-processor/internal/platforms/shein/api/product"

	"github.com/sirupsen/logrus"
)

// SKUCreator SKU创建器
type SKUCreator struct {
	utils *SKUUtils
}

// NewSKUCreator 创建SKU创建器
func NewSKUCreator() *SKUCreator {
	return &SKUCreator{
		utils: NewSKUUtils(),
	}
}

// CreateSKU 统一的SKU创建函数
func (c *SKUCreator) CreateSKU(ctx *TaskContext, params SKUCreationParams) (*product.SKU, error) {
	// 1. 价格计算
	salePriceMultiplier := ctx.ProfitRule.SalePriceMultiplier
	discountPriceMultiplier := ctx.ProfitRule.DiscountPriceMultiplier
	productPrice := GetProductPrice(params.ProductInfo, ctx.StoreInfo.PriceType)
	originalPrice := math.Round((productPrice)*100) / 100
	salePrice := math.Round(originalPrice*salePriceMultiplier*100) / 100
	var specialPrice float64
	if discountPriceMultiplier != 0 {
		specialPrice = math.Round(originalPrice*discountPriceMultiplier*100) / 100
	}

	// 2. 价格验证
	if originalPrice <= 0 {
		logrus.Infof("ASIN %s 价格为0，跳过创建SKU", params.ASIN)
		return nil, nil // 返回nil表示跳过，不是错误
	}

	// 3. 生成SKU代码 - 从AsinSkuMap中获取
	supplierSKU := ""
	if ctx.AsinSkuMap != nil {
		if sku, exists := ctx.AsinSkuMap[params.ASIN]; exists {
			supplierSKU = sku
		}
	}

	// 4. 库存数量设置
	stockCount := 0
	if ctx.StoreInfo.FixedStockCount != nil {
		stockCount = *ctx.StoreInfo.FixedStockCount
	}
	if stockCount == 0 {
		stockCount = rand.Intn(1000) + 10
	}

	currency := GetCurrencyByRegion(ctx.Task.Region)

	// 5. 构建数量信息
	quantityInfo := c.utils.BuildQuantityInfo(params)

	// 6. 构建SKU图片信息
	skuImageInfo := c.utils.BuildSKUImageInfoForMultiPiece(ctx, params)
	if skuImageInfo != nil {
		logrus.Infof("检测到多件商品 ASIN %s，已处理SKU图片", params.ASIN)
	}

	// 7. 创建SKU结构体
	sku := &product.SKU{
		SaleAttributeList: func() []product.SaleAttribute {
			if params.SaleAttributeList == nil {
				return []product.SaleAttribute{}
			}
			return params.SaleAttributeList
		}(),
		CostInfo: &product.CostInfo{
			CostPrice: fmt.Sprintf("%.2f", c.utils.FormatPriceByCurrency(salePrice, currency)),
			Currency:  currency,
		},
		StockInfoList: c.utils.BuildStockInfoList(ctx, stockCount, params.WarehouseCode),
		PriceInfoList: []product.PriceInfo{
			{
				SubSite:   ctx.SiteList[0].SubSiteList[0],
				BasePrice: salePrice,
				SpecialPrice: func() *float64 {
					if specialPrice != 0 && specialPrice < salePrice {
						return &specialPrice
					}
					return nil
				}(),
				Currency: currency,
			},
		},
		SupplierSKU:              supplierSKU,
		Length:                   params.Variant.Length.String(),
		Width:                    params.Variant.Width.String(),
		Height:                   params.Variant.Height.String(),
		Weight:                   c.utils.ParseWeight(params.Variant.Weight.String()),
		LengthUnit:               params.Variant.LengthUnit,
		CompetingCostPriceImages: []any{},
		WeightUnit:               "g",
		StopPurchase:             1,
		MallState:                1,
		QuantityInfo:             quantityInfo,
		ImageInfo:                skuImageInfo,
		Extra: product.SkuExtra{
			FieldDisabledInfo: product.FieldDisabledInfo{},
		},
	}

	logrus.Debugf("成功创建SKU: ASIN %s, 价格 %.2f, SKU代码 %s, 销售属性数量 %d",
		params.ASIN, salePrice, supplierSKU, len(params.SaleAttributeList))

	return sku, nil
}

// BuildSaleAttributeListForSingleVariant 为单变体构建销售属性列表
func (c *SKUCreator) BuildSaleAttributeListForSingleVariant(ctx *TaskContext, variant Variant, strategy AttributeStrategy) []product.SaleAttribute {
	var saleAttributeList []product.SaleAttribute

	// 尝试从变体属性中获取次要属性值
	var attributeTemplates []attribute.AttributeTemplate
	if ctx.AttributeTemplates != nil {
		attributeTemplates = ctx.AttributeTemplates.Data
	}
	secondaryAttrName := c.utils.GetAttributeName(strategy.SecondaryAttribute.AttrID, attributeTemplates)
	var secondaryAttrValue string
	found := false

	// 多策略匹配次要属性
	for _, attrName := range append([]string{secondaryAttrName}, c.utils.GetAttributeNameAlternatives(strategy.SecondaryAttribute.AttrID, attributeTemplates)...) {
		for attrKey, value := range variant.Attributes {
			if strings.EqualFold(attrKey, attrName) {
				secondaryAttrValue = value
				found = true
				logrus.Debugf("找到次要属性值: %s = %s", attrName, value)
				break
			}
		}
		if found {
			break
		}
	}

	if found {
		// 查找对应的属性值ID
		var valueID int
		for _, attrValue := range strategy.SecondaryAttribute.AttrValue {
			if strings.EqualFold(attrValue.Value, secondaryAttrValue) {
				valueID = attrValue.ID.Int()

				// 检查次要属性值ID是否有效
				if valueID <= 0 {
					logrus.Warnf("次要属性值ID无效，跳过: %s (ID: %d)，应该在预处理阶段已经映射", attrValue.Value, valueID)
					valueID = 0 // 标记为无效
				}

				logrus.Debugf("匹配到次要属性值ID: %d", valueID)
				break
			}
		}

		// 如果找到匹配且有效的值，添加到销售属性列表
		if valueID > 0 {
			logrus.Debugf("为单变体添加次要销售属性: 属性ID=%d, 属性值ID=%d, 属性值=%s",
				strategy.SecondaryAttribute.AttrID, valueID, secondaryAttrValue)

			saleAttributeList = append(saleAttributeList, product.SaleAttribute{
				AttributeID:        strategy.SecondaryAttribute.AttrID,
				AttributeValueID:   valueID,
				IsSPPSaleAttribute: false,
				PreFillSpec:        false,
			})
		} else {
			logrus.Warnf("次要属性值 '%s' 未找到有效的ID", secondaryAttrValue)
		}
	} else {
		logrus.Warnf("变体中未找到次要属性 '%s'", secondaryAttrName)
	}

	return saleAttributeList
}
