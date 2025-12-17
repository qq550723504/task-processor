package modules

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"task-processor/internal/common/amazon/model"
	"task-processor/internal/common/shein/api/attribute"
	"task-processor/internal/common/shein/api/product"

	"github.com/sirupsen/logrus"
)

// SKUBuilder SKU构建器
type SKUBuilder struct {
	variantMatcher *VariantMatcher
}

// NewSKUBuilder 创建新的SKU构建器
func NewSKUBuilder(variantMatcher *VariantMatcher) *SKUBuilder {
	return &SKUBuilder{
		variantMatcher: variantMatcher,
	}
}

// BuildSKUListWithStrategy 根据策略构建SKU列表
func (b *SKUBuilder) BuildSKUListWithStrategy(ctx *TaskContext, req SKUBuildRequest) ([]product.SKU, error) {
	logrus.Infof("🔧 === 开始SKU构建流程 ===")
	logrus.Infof("📊 SKU构建请求信息:")
	logrus.Infof("  - 主要属性ID: %d", req.Strategy.PrimaryAttribute.AttrID)
	logrus.Infof("  - 次要属性ID: %d", req.Strategy.SecondaryAttribute.AttrID)
	logrus.Infof("  - 主要属性值: %s", req.PrimaryAttrValue)
	logrus.Infof("  - 仓库代码: %s", req.WarehouseCode)
	logrus.Infof("  - 变体数量: %d", len(req.SaleAttributeData.Variants))

	// SHEIN规则验证：检查次要属性是否与主要属性相同
	if req.Strategy.SecondaryAttribute.AttrID > 0 && req.Strategy.SecondaryAttribute.AttrID == req.Strategy.PrimaryAttribute.AttrID {
		logrus.Warnf("⚠️ SHEIN规则冲突：次要属性ID(%d)与主要属性ID(%d)相同，降级为单SKU模式",
			req.Strategy.SecondaryAttribute.AttrID, req.Strategy.PrimaryAttribute.AttrID)
		return b.buildSingleSKU(ctx, req)
	}

	// 如果没有次要属性，只创建一个 SKU
	if req.Strategy.SecondaryAttribute.AttrID <= 0 {
		logrus.Infof("🎯 检测到无次要属性，使用单SKU构建模式")
		return b.buildSingleSKU(ctx, req)
	}

	logrus.Infof("🎯 检测到有次要属性，使用多SKU构建模式")
	logrus.Infof("  - 次要属性值数量: %d", len(req.Strategy.SecondaryAttribute.AttrValue))

	// 打印次要属性值详情
	for i, attrValue := range req.Strategy.SecondaryAttribute.AttrValue {
		logrus.Infof("  - 次要属性值[%d]: %s (ID: %d)", i+1, attrValue.Value, attrValue.ID.Int())
	}

	return b.buildMultipleSKUs(ctx, req)
}

// buildSingleSKU 构建单个SKU（无次要属性情况）
func (b *SKUBuilder) buildSingleSKU(ctx *TaskContext, req SKUBuildRequest) ([]product.SKU, error) {
	logrus.Infof("🔨 === 开始单SKU构建流程 ===")

	// 查找对应主要属性值的变体
	logrus.Infof("🔍 步骤1: 查找匹配的变体...")
	var matchedVariant *Variant
	primaryAttrName := b.getAttributeName(req.Strategy.PrimaryAttribute.AttrID, ctx.AttributeTemplates)
	logrus.Infof("  - 主要属性名称: %s", primaryAttrName)
	logrus.Infof("  - 目标属性值: %s", req.PrimaryAttrValue)

	for i, variant := range req.SaleAttributeData.Variants {
		logrus.Debugf("  检查变体[%d]: ASIN=%s, 价格=%.2f", i+1, variant.ASIN, variant.Price)

		// 多策略匹配
		matched := false
		attrNames := append([]string{primaryAttrName}, b.getAttributeNameAlternatives(req.Strategy.PrimaryAttribute.AttrID, ctx.AttributeTemplates)...)

		for _, attrName := range attrNames {
			for attrKey, value := range variant.Attributes {
				logrus.Debugf("    属性匹配: %s=%s vs %s", attrKey, value, attrName)
				if strings.EqualFold(attrKey, attrName) && strings.EqualFold(value, req.PrimaryAttrValue) {
					matched = true
					logrus.Infof("  ✅ 找到匹配: 变体ASIN=%s, 属性=%s, 值=%s", variant.ASIN, attrKey, value)
					break
				}
			}
			if matched {
				break
			}
		}

		if matched {
			matchedVariant = &variant
			break
		}
	}

	if matchedVariant == nil {
		logrus.Warnf("❌ 找不到主要属性值 %s 对应的变体", req.PrimaryAttrValue)
		logrus.Warnf("可用的变体属性值:")
		for i, variant := range req.SaleAttributeData.Variants {
			logrus.Warnf("  变体[%d] ASIN=%s:", i+1, variant.ASIN)
			for attrKey, value := range variant.Attributes {
				logrus.Warnf("    %s = %s", attrKey, value)
			}
		}
		return []product.SKU{}, nil
	}

	logrus.Infof("✅ 找到匹配变体: ASIN=%s, 价格=%.2f", matchedVariant.ASIN, matchedVariant.Price)

	// 处理主要属性值ID
	logrus.Infof("🔍 步骤2: 获取主要属性值ID...")
	var primaryValueID int
	for _, attrValue := range req.Strategy.PrimaryAttribute.AttrValue {
		if strings.EqualFold(attrValue.Value, req.PrimaryAttrValue) {
			primaryValueID = attrValue.ID.Int()
			logrus.Infof("  - 找到属性值ID: %s -> %d", attrValue.Value, primaryValueID)
			break
		}
	}

	// 检查主要属性值ID是否有效
	if primaryValueID <= 0 {
		logrus.Errorf("❌ 主要属性值ID无效: %s (ID: %d)", req.PrimaryAttrValue, primaryValueID)
		return []product.SKU{}, fmt.Errorf("主要属性值ID无效: %s", req.PrimaryAttrValue)
	}
	logrus.Infof("✅ 主要属性值ID有效: %d", primaryValueID)

	// 构建SKU的销售属性列表
	logrus.Infof("🔧 步骤3: 构建SKU销售属性列表...")
	var saleAttributeList []product.SaleAttribute

	// 只有在有次要属性且与主要属性不同时，才添加到SKU的销售属性中
	if req.Strategy.SecondaryAttribute.AttrID > 0 && req.Strategy.SecondaryAttribute.AttrID != req.Strategy.PrimaryAttribute.AttrID {
		logrus.Infof("  - 检测到次要属性ID=%d，但单SKU场景下不添加SKU级别的销售属性", req.Strategy.SecondaryAttribute.AttrID)
	} else {
		logrus.Infof("  - 没有有效的次要属性，SKU不添加销售属性")
	}
	logrus.Infof("  - SKU销售属性数量: %d", len(saleAttributeList))

	// 使用统一的SKU创建函数
	logrus.Infof("🏗️ 步骤4: 创建SKU...")
	var productInfo *model.Product
	if ctx.Variants != nil {
		for _, v := range *ctx.Variants {
			if v.Asin == matchedVariant.ASIN {
				productInfo = &v
				break
			}
		}
	}
	// 如果在上下文中找不到产品信息，则使用主产品信息作为备选
	if productInfo == nil {
		logrus.Warnf("在上下文中未找到ASIN %s 的产品信息，使用主产品信息", matchedVariant.ASIN)
		productInfo = ctx.AmazonProduct
	}
	sku, err := b.createSKU(ctx, SKUCreationParams{
		ASIN:              matchedVariant.ASIN,
		ProductInfo:       productInfo,
		WarehouseCode:     req.WarehouseCode,
		SaleAttributeList: saleAttributeList,
		Variant:           *matchedVariant,
	})
	if err != nil {
		logrus.Errorf("❌ 创建SKU失败: %v", err)
		return []product.SKU{}, nil
	}
	if sku == nil {
		logrus.Warnf("⚠️ SKU创建返回nil（可能是价格为0），跳过该SKU: %s", matchedVariant.ASIN)
		return []product.SKU{}, nil
	}

	logrus.Infof("🎉 成功为主要属性值 %s 创建了 1 个 SKU", req.PrimaryAttrValue)
	return []product.SKU{*sku}, nil
}

// buildMultipleSKUs 构建多个SKU（有次要属性情况）
func (b *SKUBuilder) buildMultipleSKUs(ctx *TaskContext, req SKUBuildRequest) ([]product.SKU, error) {
	// 第一步：使用属性匹配器预先过滤出所有匹配主要属性的变体
	primaryMatchedVariants := b.variantMatcher.FindMatchingVariants(ctx,
		req.SaleAttributeData.Variants,
		req.Strategy.PrimaryAttribute.AttrID,
		req.PrimaryAttrValue,
	)

	if len(primaryMatchedVariants) == 0 {
		logrus.Infof("没有找到匹配主要属性值 %s 的变体", req.PrimaryAttrValue)
		return []product.SKU{}, nil
	}

	logrus.Debugf("找到 %d 个匹配主要属性的变体", len(primaryMatchedVariants))

	// 第二步：处理次要属性并建立复合键到属性值的映射
	processedSecondaryValues := make(map[string]bool)
	variantInfoMap := make(map[string]variantInfo) // 使用复合键: "ASIN:valueID"
	usedValueIDs := make(map[int]bool)             // 跟踪已使用的属性值ID，防止重复

	for _, attr := range req.Strategy.SecondaryAttribute.AttrValue {
		// 去重检查
		if processedSecondaryValues[attr.Value] {
			continue
		}
		processedSecondaryValues[attr.Value] = true

		// 预处理自定义属性值
		currentValueID := attr.ID.Int()

		// 检查次要属性值ID是否有效
		if currentValueID <= 0 {
			logrus.Warnf("次要属性值ID无效，跳过: %s (ID: %d)，应该在预处理阶段已经映射", attr.Value, currentValueID)
			continue
		}

		// 检查属性值ID是否重复
		if usedValueIDs[currentValueID] {
			logrus.Warnf("检测到重复的次要属性值ID: %d (属性值: %s)，跳过以避免SHEIN平台错误", currentValueID, attr.Value)
			continue
		}
		usedValueIDs[currentValueID] = true

		// 使用属性匹配器查找匹配该次要属性值的变体
		secondaryMatchedVariants := b.variantMatcher.FindMatchingVariants(ctx,
			primaryMatchedVariants,
			req.Strategy.SecondaryAttribute.AttrID,
			attr.Value,
		)

		var matchedCount int
		for _, variant := range secondaryMatchedVariants {
			// 使用复合键 "ASIN:valueID" 确保同一ASIN的不同属性值都能创建SKU
			compositeKey := fmt.Sprintf("%s:%d", variant.ASIN, currentValueID)
			if _, exists := variantInfoMap[compositeKey]; !exists {
				variantInfoMap[compositeKey] = variantInfo{
					variant:   variant,
					attrID:    req.Strategy.SecondaryAttribute.AttrID,
					valueID:   currentValueID,
					attrValue: attr.Value, // 记录实际的属性值
				}
				matchedCount++
			}
		}

		if matchedCount > 0 {
			logrus.Infof("次要属性值 %s (ID: %d) 匹配到 %d 个新变体", attr.Value, currentValueID, matchedCount)
		} else {
			logrus.Warnf("次要属性值 %s (ID: %d) 未匹配到任何新变体", attr.Value, currentValueID)
		}
	}

	return b.buildSKUListForMultipleVariants(ctx, variantInfoMap, req)
}

// buildSKUListForMultipleVariants 为多个变体构建SKU列表
func (b *SKUBuilder) buildSKUListForMultipleVariants(ctx *TaskContext, variantInfoMap map[string]variantInfo, req SKUBuildRequest) ([]product.SKU, error) {
	// 结果列表
	var skuList []product.SKU
	usedAttributeValueIDs := make(map[int]bool) // 跟踪已使用的属性值ID，防止重复

	// 遍历所有变体信息
	for _, varInfo := range variantInfoMap {
		// 检查属性值ID是否重复
		if usedAttributeValueIDs[varInfo.valueID] {
			logrus.Warnf("检测到重复的销售属性值ID: %d (ASIN: %s)，跳过该SKU以避免SHEIN平台错误",
				varInfo.valueID, varInfo.variant.ASIN)
			continue
		}
		usedAttributeValueIDs[varInfo.valueID] = true

		// 从上下文中的变体数据中查找对应的产品信息
		var productInfo *model.Product
		if ctx.Variants != nil {
			for _, v := range *ctx.Variants {
				if v.Asin == varInfo.variant.ASIN {
					productInfo = &v
					break
				}
			}
		}

		// 如果在上下文中找不到产品信息，则使用主产品信息作为备选
		if productInfo == nil {
			logrus.Warnf("在上下文中未找到ASIN %s 的产品信息，使用主产品信息", varInfo.variant.ASIN)
			productInfo = ctx.AmazonProduct
		}

		// SHEIN规则验证：确保SKU的销售属性不与SKC的主要属性相同
		var saleAttributeList []product.SaleAttribute
		if varInfo.attrID != req.Strategy.PrimaryAttribute.AttrID {
			saleAttributeList = []product.SaleAttribute{
				{
					AttributeID:        varInfo.attrID,
					AttributeValueID:   varInfo.valueID,
					IsSPPSaleAttribute: false,
					PreFillSpec:        false,
				},
			}
			logrus.Debugf("SKU添加次要销售属性: ID=%d, ValueID=%d", varInfo.attrID, varInfo.valueID)
		} else {
			logrus.Warnf("跳过SKU销售属性：次要属性ID(%d)与主要属性ID(%d)相同，违反SHEIN规则",
				varInfo.attrID, req.Strategy.PrimaryAttribute.AttrID)
		}

		// 使用统一的SKU创建函数
		sku, err := b.createSKU(ctx, SKUCreationParams{
			ASIN:              varInfo.variant.ASIN,
			ProductInfo:       productInfo,
			WarehouseCode:     req.WarehouseCode,
			SaleAttributeList: saleAttributeList,
			Variant:           varInfo.variant,
		})
		if err != nil {
			logrus.Errorf("创建SKU失败: %v", err)
			continue
		}
		if sku == nil {
			// 价格为0的情况，跳过该SKU
			logrus.Infof("价格为0，跳过该SKU: %s", varInfo.variant.ASIN)
			continue
		}

		logrus.Debugf("成功创建SKU: ASIN %s, 属性值ID %d", varInfo.variant.ASIN, varInfo.valueID)
		skuList = append(skuList, *sku)
	}

	if len(skuList) == 0 {
		logrus.Infof("无法为主要属性值 %s 创建任何 SKU，可能是由于价格问题", req.PrimaryAttrValue)
		return []product.SKU{}, nil // 返回空列表而不是错误，避免整个流程失败
	}

	logrus.Infof("成功为主要属性值 %s 创建了 %d 个 SKU，去重后避免了属性值ID重复", req.PrimaryAttrValue, len(skuList))
	return skuList, nil
}

// BuildSKUListForSingleVariant 为单变体构建SKU列表
func (b *SKUBuilder) BuildSKUListForSingleVariant(ctx *TaskContext, variant Variant, strategy AttributeStrategy) ([]product.SKU, error) {
	logrus.Infof("为单变体构建SKU列表: ASIN %s", variant.ASIN)

	// 根据策略构建销售属性列表
	var saleAttributeList []product.SaleAttribute

	// 如果有次要属性，需要基于变体属性创建销售属性
	if strategy.SecondaryAttribute.AttrID != 0 {
		saleAttributeList = b.buildSaleAttributeListForSingleVariant(ctx, variant, strategy)
	}

	// 遍历变体来找到对应的productInfo
	var productInfo *model.Product
	if ctx.Variants != nil {
		for _, v := range *ctx.Variants {
			if v.Asin == variant.ASIN {
				productInfo = &v
				break
			}
		}
	}

	// 如果在变体中找不到，使用主产品信息作为备选
	if productInfo == nil {
		logrus.Warnf("在变体中未找到ASIN %s 的产品信息，使用主产品信息", variant.ASIN)
		if ctx.AmazonProduct != nil {
			productInfo = ctx.AmazonProduct
		} else {
			return nil, fmt.Errorf("未找到ASIN %s 对应的产品信息，且主产品信息也为空", variant.ASIN)
		}
	}

	// 使用统一的SKU创建函数
	sku, err := b.createSKU(ctx, SKUCreationParams{
		ASIN:              variant.ASIN,
		ProductInfo:       productInfo,
		WarehouseCode:     ctx.Warehouses.Data[0].WarehouseCode,
		SaleAttributeList: saleAttributeList,
		Variant:           variant,
	})
	if err != nil {
		return nil, err
	}
	if sku == nil {
		// 价格为0的情况，返回空列表
		return []product.SKU{}, nil
	}

	logrus.Infof("成功为单变体创建SKU，销售属性数量: %d", len(saleAttributeList))
	return []product.SKU{*sku}, nil
}

// buildSaleAttributeListForSingleVariant 为单变体构建销售属性列表
func (b *SKUBuilder) buildSaleAttributeListForSingleVariant(ctx *TaskContext, variant Variant, strategy AttributeStrategy) []product.SaleAttribute {
	var saleAttributeList []product.SaleAttribute

	// 尝试从变体属性中获取次要属性值
	secondaryAttrName := b.getAttributeName(strategy.SecondaryAttribute.AttrID, ctx.AttributeTemplates)
	var secondaryAttrValue string
	found := false

	// 多策略匹配次要属性
	for _, attrName := range append([]string{secondaryAttrName}, b.getAttributeNameAlternatives(strategy.SecondaryAttribute.AttrID, ctx.AttributeTemplates)...) {
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

// createSKU 统一的SKU创建函数
func (b *SKUBuilder) createSKU(ctx *TaskContext, params SKUCreationParams) (*product.SKU, error) {
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
	quantityInfo := b.buildQuantityInfo(params)

	skuImageInfo := b.buildSKUImageInfoForMultiPiece(ctx, params)
	logrus.Infof("检测到多件商品 ASIN %s，已处理SKU图片", params.ASIN)

	// 7. 创建SKU结构体
	sku := &product.SKU{
		SaleAttributeList: func() []product.SaleAttribute {
			if params.SaleAttributeList == nil {
				return []product.SaleAttribute{}
			}
			return params.SaleAttributeList
		}(),
		CostInfo: &product.CostInfo{
			CostPrice: b.formatPriceByCurrency(salePrice, currency),
			Currency:  currency,
		},
		StockInfoList: b.buildStockInfoList(ctx, stockCount, params.WarehouseCode),
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
		Length:                   params.Variant.Length,
		Width:                    params.Variant.Width,
		Height:                   params.Variant.Height,
		Weight:                   b.parseWeight(params.Variant.Weight),
		LengthUnit:               params.Variant.LengthUnit,
		CompetingCostPriceImages: []any{},
		WeightUnit:               "g",
		StopPurchase:             1,
		MallState:                1,
		QuantityInfo:             quantityInfo,
		ImageInfo:                skuImageInfo, // 添加SKU图片信息
		Extra: product.SkuExtra{
			FieldDisabledInfo: product.FieldDisabledInfo{},
		},
	}

	logrus.Debugf("成功创建SKU: ASIN %s, 价格 %.2f, SKU代码 %s, 销售属性数量 %d",
		params.ASIN, salePrice, supplierSKU, len(params.SaleAttributeList))

	return sku, nil
}

// Helper methods

// getAttributeName 获取属性名称
func (b *SKUBuilder) getAttributeName(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) string {
	if attributeTemplates != nil && len(attributeTemplates.Data) > 0 {
		for _, data := range attributeTemplates.Data {
			for i := range data.AttributeInfos {
				if data.AttributeInfos[i].AttributeID == attrID {
					attrInfo := &data.AttributeInfos[i]
					if attrInfo.AttributeName != "" {
						return attrInfo.AttributeName
					}
					if attrInfo.AttributeNameEn != "" {
						return attrInfo.AttributeNameEn
					}
				}
			}
		}
	}
	return ""
}

// getAttributeNameAlternatives 获取属性名的替代形式
func (b *SKUBuilder) getAttributeNameAlternatives(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) []string {
	alternatives := []string{}

	if attributeTemplates != nil && len(attributeTemplates.Data) > 0 {
		for _, data := range attributeTemplates.Data {
			for i := range data.AttributeInfos {
				if data.AttributeInfos[i].AttributeID == attrID {
					attrInfo := &data.AttributeInfos[i]
					if attrInfo.AttributeName != "" {
						alternatives = append(alternatives, attrInfo.AttributeName)
					}
					if attrInfo.AttributeNameEn != "" && attrInfo.AttributeNameEn != attrInfo.AttributeName {
						alternatives = append(alternatives, attrInfo.AttributeNameEn)
					}
				}
			}
		}
	}

	return alternatives
}

// buildQuantityInfo 构建SKU数量信息
func (b *SKUBuilder) buildQuantityInfo(params SKUCreationParams) *product.QuantityInfo {
	// 默认配置：单品，件为单位，数量为1
	quantityType := 1 // 默认单品
	quantityUnit := 1 // 默认件为单位
	quantity := 1

	// 根据产品属性或变体信息判断数量类型和数量
	if params.Variant.QuantityType > 0 {
		// 使用变体提供的数量类型
		quantityType = params.Variant.QuantityType
		logrus.Debugf("使用变体数量类型: %d", quantityType)
	}

	// 根据数量类型设置数量和单位
	switch quantityType {
	case 2: // 同款多件
		quantity = max(params.Variant.Quantity, 2)
		quantityUnit = 1 // 件为单位
		logrus.Debugf("同款多件，数量设置为: %d, 单位: 件", quantity)
	case 4: // 混合套装
		quantity = max(params.Variant.Quantity, 2)
		quantityUnit = 2 // 套为单位 (根据SHEIN规则，混合套装必须使用套为单位)
		logrus.Debugf("混合套装，数量设置为: %d, 单位: 套", quantity)
	case 1: // 单件
		quantity = 1
		quantityUnit = 1 // 件为单位
		logrus.Debugf("单品，数量强制设置为: 1, 单位: 件")
	case 3: // 单套
		quantity = 1
		quantityUnit = 2 // 套为单位
		logrus.Debugf("单套���数量强制设置为: 1, 单位: 套")
	default:
		// 未知类型，默认为单品，数量必须为1
		quantity = 1
		quantityType = 1
		quantityUnit = 1
		logrus.Debugf("未知数量类型，默认为单品，数量设置为: 1, 单位: 件")
	}

	// 最终验证：单品（类型1）或单套（类型3）时数量必须为1
	if (quantityType == 1 || quantityType == 3) && quantity != 1 {
		logrus.Warnf("⚠️ SHEIN规则验证：单品/单套数量必须为1。类型: %d, 原数量: %d, 已强制修正为1", quantityType, quantity)
		quantity = 1
	}

	// SHEIN规则验证：混合套装时单位必须为套
	if quantityType == 4 && quantityUnit != 2 {
		logrus.Warnf("⚠️ SHEIN规则验证：混合套装单位必须为套。类型: %d, 原单位: %d, 已强制修正为2(套)", quantityType, quantityUnit)
		quantityUnit = 2
	}

	// 最终安全检查：确保单品数量为1
	if quantityType == 1 && quantity != 1 {
		logrus.Errorf("❌ 严重错误：单品数量不为1，强制修正。类型: %d, 数量: %d -> 1", quantityType, quantity)
		quantity = 1
	}

	unitName := "件"
	if quantityUnit == 2 {
		unitName = "套"
	}

	logrus.Infof("✅ SKU数量信息构建完成 - 类型: %d, 数量: %d, 单位: %d(%s)", quantityType, quantity, quantityUnit, unitName)

	return &product.QuantityInfo{
		Quantity:     &quantity,
		QuantityType: &quantityType,
		QuantityUnit: &quantityUnit,
	}
}

// buildSKUImageInfoForMultiPiece 为多件商品构建SKU图片信息
func (b *SKUBuilder) buildSKUImageInfoForMultiPiece(ctx *TaskContext, params SKUCreationParams) *product.ImageInfo {
	logrus.Infof("🖼️ 开始为多件商品构建SKU图片，ASIN: %s", params.ASIN)

	// 初始化空的图片信息
	imageInfo := &product.ImageInfo{
		ImageInfoList:         []product.ImageDetail{},
		OriginalImageInfoList: &[]interface{}{},
	}

	// 查找对应变体的图片
	var variantImages []string
	var imageSource string

	// 通过ASIN查找对应的变体图片
	if ctx.Variants != nil {
		for _, variant := range *ctx.Variants {
			if variant.Asin == params.ASIN && len(variant.Images) > 0 {
				variantImages = variant.Images
				imageSource = "变体图片"
				logrus.Infof("✅ 找到变体图片，ASIN: %s, 图片数量: %d", params.ASIN, len(variantImages))
				break
			}
		}
	}

	// 如果没找到变体图片，使用主产品图片
	if len(variantImages) == 0 && ctx.AmazonProduct != nil && len(ctx.AmazonProduct.Images) > 0 {
		variantImages = ctx.AmazonProduct.Images
		imageSource = "主产品图片"
		logrus.Infof("⚠️ 未找到变体图片，使用主产品图片，ASIN: %s, 图片数量: %d", params.ASIN, len(variantImages))
	}

	// 如果仍然没有图片，这是一个严重问题
	if len(variantImages) == 0 {
		logrus.Errorf("❌ 严重错误：多件商品必须有SKU图片，但未找到任何图片，ASIN: %s", params.ASIN)
		logrus.Errorf("   - 变体数据: %v", ctx.Variants != nil)
		if ctx.Variants != nil {
			logrus.Errorf("   - 变体数量: %d", len(*ctx.Variants))
		}
		logrus.Errorf("   - 主产品数据: %v", ctx.AmazonProduct != nil)
		if ctx.AmazonProduct != nil {
			logrus.Errorf("   - 主产品图片数量: %d", len(ctx.AmazonProduct.Images))
		}
		return imageInfo
	}

	// 增强的图片上传逻辑：支持重试和多图片处理
	uploadSuccess := b.uploadSKUImagesWithRetry(ctx, params, variantImages, imageSource, imageInfo)

	if !uploadSuccess {
		logrus.Errorf("❌ 所有图片上传都失败了，多件商品SKU将缺少必需的图片，ASIN: %s", params.ASIN)
	} else {
		logrus.Infof("🎉 多件商品SKU图片构建完成，ASIN: %s, 图片数量: %d", params.ASIN, len(imageInfo.ImageInfoList))
	}

	return imageInfo
}

// uploadSKUImagesWithRetry 带重试机制的SKU图片上传
func (b *SKUBuilder) uploadSKUImagesWithRetry(ctx *TaskContext, params SKUCreationParams, variantImages []string, imageSource string, imageInfo *product.ImageInfo) bool {
	const maxRetries = 3
	const maxImages = 1 // 只保留一张图片

	uploadedCount := 0

	// 遍历可用图片，只上传一张图片
	for i, imageURL := range variantImages {
		if imageURL == "" || uploadedCount >= maxImages {
			continue
		}

		// 对每张图片进行重试上传
		uploaded := b.uploadSingleImageWithRetry(ctx, params, imageURL, imageSource, i+1, maxRetries)
		if uploaded != "" {
			// 创建SKU图片信息
			// 重要：SKU图片的排序必须从1开始，第一张SKU图片排序为1
			skuImageSort := 1 // 排序编号固定为1
			imageDetail := product.ImageDetail{
				ImageURL:             uploaded,
				ImageType:            1,            // SKU图片类型
				ImageSort:            skuImageSort, // SKU图片排序固定为1
				AISStatus:            0,
				MarketingMainImage:   false,
				PSTypes:              []string{},
				SizeImgFlag:          false,
				TransformCVSizeImage: false,
			}
			imageInfo.ImageInfoList = append(imageInfo.ImageInfoList, imageDetail)
			uploadedCount++
			logrus.Infof("✅ 成功上传第%d张SKU图片，ASIN: %s, ImageSort: %d, URL: %s", uploadedCount, params.ASIN, skuImageSort, uploaded)

			// 对于多件商品，至少需要1张图片
			if uploadedCount >= 1 {
				logrus.Infof("✅ 已成功上传%d张SKU图片，满足多件商品要求，ASIN: %s", uploadedCount, params.ASIN)
				break // 只上传一张图片
			}
		}
	}

	// 验证SKU图片排序
	if uploadedCount > 0 {
		if err := b.validateSKUImageSorting(imageInfo); err != nil {
			logrus.Errorf("❌ SKU图片排序验证失败: %v", err)
			// 不返回错误，而是尝试修复
			b.fixSKUImageSorting(imageInfo)
		}
	}

	return uploadedCount > 0
}

// validateSKUImageSorting 验证SKU图片排序的正确性
func (b *SKUBuilder) validateSKUImageSorting(imageInfo *product.ImageInfo) error {
	if len(imageInfo.ImageInfoList) == 0 {
		return nil
	}

	// 检查第一张SKU图片的排序是否为1
	firstImage := imageInfo.ImageInfoList[0]
	if firstImage.ImageType == 1 && firstImage.ImageSort != 1 {
		return fmt.Errorf("第一张SKU图片排序应为1，当前为: %d", firstImage.ImageSort)
	}

	// 如果只有一张图片，不需要检查排序连续性
	if len(imageInfo.ImageInfoList) == 1 {
		logrus.Infof("✅ SKU图片排序验证通过，共%d张图片", len(imageInfo.ImageInfoList))
		return nil
	}

	// 检查排序连续性（仅在有多张图片时）
	for i, img := range imageInfo.ImageInfoList {
		expectedSort := i + 1
		if img.ImageSort != expectedSort {
			return fmt.Errorf("SKU图片排序不连续，第%d张图片期望排序%d，实际%d", i+1, expectedSort, img.ImageSort)
		}
	}

	logrus.Infof("✅ SKU图片排序验证通过，共%d张图片", len(imageInfo.ImageInfoList))
	return nil
}

// fixSKUImageSorting 修复SKU图片排序
func (b *SKUBuilder) fixSKUImageSorting(imageInfo *product.ImageInfo) {
	logrus.Infof("🔧 开始修复SKU图片排序...")

	// 如果只有一张图片，确保其排序为1
	if len(imageInfo.ImageInfoList) == 1 {
		if imageInfo.ImageInfoList[0].ImageSort != 1 {
			oldSort := imageInfo.ImageInfoList[0].ImageSort
			imageInfo.ImageInfoList[0].ImageSort = 1
			logrus.Infof("✅ 修复SKU图片排序：唯一图片 %d -> %d", oldSort, 1)
		}
		return
	}

	// 多张图片的情况，保持原有逻辑
	for i := range imageInfo.ImageInfoList {
		correctSort := i + 1
		if imageInfo.ImageInfoList[i].ImageSort != correctSort {
			oldSort := imageInfo.ImageInfoList[i].ImageSort
			imageInfo.ImageInfoList[i].ImageSort = correctSort
			logrus.Infof("✅ 修复SKU图片排序：第%d张图片 %d -> %d", i+1, oldSort, correctSort)
		}
	}

	logrus.Infof("🎉 SKU图片排序修复完成")
}

// uploadSingleImageWithRetry 单张图片重试上传
func (b *SKUBuilder) uploadSingleImageWithRetry(ctx *TaskContext, params SKUCreationParams, imageURL, imageSource string, imageIndex, maxRetries int) string {
	for retry := 1; retry <= maxRetries; retry++ {
		logrus.Infof("🔄 尝试上传第%d张%s作为SKU图片 (重试%d/%d): %s", imageIndex, imageSource, retry, maxRetries, imageURL)

		// 验证图片URL格式
		if !b.isValidImageURL(imageURL) {
			logrus.Warnf("⚠️ 无效的图片URL格式，跳过: %s", imageURL)
			break
		}

		uploadedURL, err := ctx.ShopClient.DownloadAndUploadImage(imageURL)
		if err != nil {
			logrus.Warnf("⚠️ 上传第%d张SKU图片失败 (重试%d/%d)，ASIN: %s, 错误: %v", imageIndex, retry, maxRetries, params.ASIN, err)

			// 如果是最后一次重试，记录详细错误
			if retry == maxRetries {
				logrus.Errorf("❌ 第%d张SKU图片上传彻底失败，已重试%d次，ASIN: %s, URL: %s, 最终错误: %v",
					imageIndex, maxRetries, params.ASIN, imageURL, err)
			}
			continue
		}

		if uploadedURL != "" {
			// 验证上传结果
			if b.isValidImageURL(uploadedURL) {
				return uploadedURL
			} else {
				logrus.Warnf("⚠️ 上传返回的URL格式无效，重试: %s", uploadedURL)
			}
		}
	}

	return ""
}

// isValidImageURL 验证图片URL格式
func (b *SKUBuilder) isValidImageURL(url string) bool {
	if url == "" {
		return false
	}

	// 基本URL格式检查
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return false
	}

	// 检查是否包含常见图片扩展名或图片服务域名
	lowerURL := strings.ToLower(url)
	imageExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp"}
	imageServices := []string{"amazonaws.com", "cloudfront.net", "ssl-images-amazon.com", "media-amazon.com"}

	// 检查扩展名
	for _, ext := range imageExtensions {
		if strings.Contains(lowerURL, ext) {
			return true
		}
	}

	// 检查图片服务域名
	for _, service := range imageServices {
		if strings.Contains(lowerURL, service) {
			return true
		}
	}

	return false
}

// parseWeight 解析重量字符串为浮点数
func (b *SKUBuilder) parseWeight(weightStr string) float64 {
	if weightStr == "" {
		return 149
	}

	// 移除常见的重量单位
	weightStr = strings.TrimSpace(weightStr)
	weightStr = strings.ToLower(weightStr)

	// 移除单位后缀
	suffixes := []string{"kg", "g", "lb", "oz", "pounds", "grams", "kilograms", "ounces"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(weightStr, suffix) {
			weightStr = strings.TrimSuffix(weightStr, suffix)
			weightStr = strings.TrimSpace(weightStr)
			break
		}
	}

	// 尝试解析为浮点数
	if weight, err := strconv.ParseFloat(weightStr, 64); err == nil {
		return weight
	}

	return 149
}

// buildStockInfoList 构建所有仓库的库存信息
func (b *SKUBuilder) buildStockInfoList(ctx *TaskContext, stockCount int, primaryWarehouseCode string) []product.StockInfo {
	var stockInfoList []product.StockInfo

	// 如果上下文中有仓库信息，为所有仓库设置库存
	if ctx.Warehouses != nil && len(ctx.Warehouses.Data) > 0 {
		logrus.Infof("🏪 为所有仓库设置库存，仓库数量: %d", len(ctx.Warehouses.Data))

		for i, warehouse := range ctx.Warehouses.Data {
			stockInfo := product.StockInfo{
				InventoryNum:          stockCount,
				MerchantWarehouseCode: warehouse.WarehouseCode,
			}
			stockInfoList = append(stockInfoList, stockInfo)
			logrus.Debugf("  - 仓库[%d]: %s, 库存: %d", i+1, warehouse.WarehouseCode, stockCount)
		}
	} else {
		// 如果没有仓库信息，使用传入的主要仓库代码
		logrus.Warnf("⚠️ 上下文中没有仓库信息，使用主要仓库代码: %s", primaryWarehouseCode)
		stockInfoList = []product.StockInfo{
			{
				InventoryNum:          stockCount,
				MerchantWarehouseCode: primaryWarehouseCode,
			},
		}
	}

	logrus.Infof("✅ 库存信息构建完成，共设置 %d 个仓库", len(stockInfoList))
	return stockInfoList
}

// formatPriceByCurrency 根据货币类型格式化价格
func (b *SKUBuilder) formatPriceByCurrency(price float64, currency string) string {
	switch currency {
	case "JPY":
		// 日元不需要小数位
		return fmt.Sprintf("%.0f", price)
	case "KRW":
		// 韩元也不需要小数位
		return fmt.Sprintf("%.0f", price)
	default:
		// 其他货币使用两位小数
		return fmt.Sprintf("%.2f", price)
	}
}
