// Package sku 提供SHEIN平台SKU策略处理功能
package sku

import (
	"fmt"
	"strings"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/api/product"
	"task-processor/internal/shein/product/variant"

	"github.com/sirupsen/logrus"
)

// SKUStrategyProcessor SKU策略处理器
type SKUStrategyProcessor struct {
	variantMatcher *variant.VariantMatcher
	creator        *SKUCreator
	utils          *SKUUtils
}

// NewSKUStrategyProcessor 创建SKU策略处理器
func NewSKUStrategyProcessor(variantMatcher *variant.VariantMatcher) *SKUStrategyProcessor {
	return &SKUStrategyProcessor{
		variantMatcher: variantMatcher,
		creator:        NewSKUCreator(),
		utils:          NewSKUUtils(),
	}
}

// BuildSingleSKU 构建单个SKU（无次要属性情况）
func (p *SKUStrategyProcessor) BuildSingleSKU(ctx *shein.TaskContext, req shein.SKUBuildRequest) ([]product.SKU, error) {
	logrus.Infof("🔨 === 开始单SKU构建流程 ===")

	// 查找对应主要属性值的变体
	logrus.Infof("🔍 步骤1: 查找匹配的变体...")
	var matchedVariant *shein.Variant
	var attributeTemplates []attribute.AttributeTemplate
	if ctx.AttributeTemplates != nil {
		attributeTemplates = ctx.AttributeTemplates.Data
	}

	// 获取属性名称及其替代形式
	primaryAttrName := p.utils.GetAttributeName(req.Strategy.PrimaryAttribute.AttrID, attributeTemplates)
	attrNameAlternatives := p.utils.GetAttributeNameAlternatives(req.Strategy.PrimaryAttribute.AttrID, attributeTemplates)

	logrus.Infof("  - 主要属性ID: %d", req.Strategy.PrimaryAttribute.AttrID)
	logrus.Infof("  - 主要属性名称: %s", primaryAttrName)
	logrus.Infof("  - 属性名称替代形式: %v", attrNameAlternatives)
	logrus.Infof("  - 目标属性值: %s", req.PrimaryAttrValue)

	// 构建完整的属性名称列表（包括主名称和替代名称）
	allAttrNames := []string{}
	if primaryAttrName != "" {
		allAttrNames = append(allAttrNames, primaryAttrName)
	}
	allAttrNames = append(allAttrNames, attrNameAlternatives...)

	// 遍历所有变体进行匹配
	for _, variant := range req.SaleAttributeData.Variants {

		// 尝试所有可能的属性名称进行匹配
		matched := false
		for _, attrName := range allAttrNames {
			for variantAttrKey, variantAttrValue := range variant.Attributes {
				// 使用忽略大小写的匹配
				if strings.EqualFold(variantAttrKey, attrName) && strings.EqualFold(variantAttrValue, req.PrimaryAttrValue) {
					matched = true
					logrus.Infof("  ✅ 找到匹配: 变体ASIN=%s, 属性=%s, 值=%s", variant.ASIN, variantAttrKey, variantAttrValue)
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
		logrus.Warnf("搜索的属性名称: %v", allAttrNames)
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

	sku, err := p.creator.CreateSKU(ctx, shein.SKUCreationParams{
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

// BuildMultipleSKUs 构建多个SKU（有次要属性情况）
func (p *SKUStrategyProcessor) BuildMultipleSKUs(ctx *shein.TaskContext, req shein.SKUBuildRequest) ([]product.SKU, error) {
	// 第一步：使用属性匹配器预先过滤出所有匹配主要属性的变体
	primaryMatchedVariants := p.variantMatcher.FindMatchingVariants(ctx,
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
	variantInfoMap := make(map[string]shein.VariantInfo) // 使用复合键: "ASIN:valueID"
	usedValueIDs := make(map[int]bool)                   // 跟踪已使用的属性值ID，防止重复

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
		secondaryMatchedVariants := p.variantMatcher.FindMatchingVariants(ctx,
			primaryMatchedVariants,
			req.Strategy.SecondaryAttribute.AttrID,
			attr.Value,
		)

		var matchedCount int
		for _, variant := range secondaryMatchedVariants {
			// 使用复合键 "ASIN:valueID" 确保同一ASIN的不同属性值都能创建SKU
			compositeKey := fmt.Sprintf("%s:%d", variant.ASIN, currentValueID)
			if _, exists := variantInfoMap[compositeKey]; !exists {
				variantInfoMap[compositeKey] = shein.VariantInfo{
					Variant:   variant,
					AttrID:    req.Strategy.SecondaryAttribute.AttrID,
					ValueID:   currentValueID,
					AttrValue: attr.Value, // 记录实际的属性值
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

	return p.buildSKUListForMultipleVariants(ctx, variantInfoMap, req)
}

// buildSKUListForMultipleVariants 为多个变体构建SKU列表
func (p *SKUStrategyProcessor) buildSKUListForMultipleVariants(ctx *shein.TaskContext, variantInfoMap map[string]shein.VariantInfo, req shein.SKUBuildRequest) ([]product.SKU, error) {
	// 结果列表
	var skuList []product.SKU
	usedAttributeValueIDs := make(map[int]bool) // 跟踪已使用的属性值ID，防止重复

	// 遍历所有变体信息
	for _, varInfo := range variantInfoMap {
		// 检查属性值ID是否重复
		if usedAttributeValueIDs[varInfo.ValueID] {
			logrus.Warnf("检测到重复的销售属性值ID: %d (ASIN: %s)，跳过该SKU以避免SHEIN平台错误",
				varInfo.ValueID, varInfo.Variant.ASIN)
			continue
		}
		usedAttributeValueIDs[varInfo.ValueID] = true

		// 从上下文中的变体数据中查找对应的产品信息
		var productInfo *model.Product
		if ctx.Variants != nil {
			for _, v := range *ctx.Variants {
				if v.Asin == varInfo.Variant.ASIN {
					productInfo = &v
					break
				}
			}
		}

		// 如果在上下文中找不到产品信息，则使用主产品信息作为备选
		if productInfo == nil {
			logrus.Warnf("在上下文中未找到ASIN %s 的产品信息，使用主产品信息", varInfo.Variant.ASIN)
			productInfo = ctx.AmazonProduct
		}

		// SHEIN规则验证：确保SKU的销售属性不与SKC的主要属性相同
		var saleAttributeList []product.SaleAttribute
		if varInfo.AttrID != req.Strategy.PrimaryAttribute.AttrID {
			saleAttributeList = []product.SaleAttribute{
				{
					AttributeID:        varInfo.AttrID,
					AttributeValueID:   varInfo.ValueID,
					IsSPPSaleAttribute: false,
					PreFillSpec:        false,
				},
			}
			logrus.Debugf("SKU添加次要销售属性: ID=%d, ValueID=%d", varInfo.AttrID, varInfo.ValueID)
		} else {
			logrus.Warnf("跳过SKU销售属性：次要属性ID(%d)与主要属性ID(%d)相同，违反SHEIN规则",
				varInfo.AttrID, req.Strategy.PrimaryAttribute.AttrID)
		}

		// 使用统一的SKU创建函数
		sku, err := p.creator.CreateSKU(ctx, shein.SKUCreationParams{
			ASIN:              varInfo.Variant.ASIN,
			ProductInfo:       productInfo,
			WarehouseCode:     req.WarehouseCode,
			SaleAttributeList: saleAttributeList,
			Variant:           varInfo.Variant,
		})
		if err != nil {
			logrus.Errorf("创建SKU失败: %v", err)
			continue
		}
		if sku == nil {
			// 价格为0的情况，跳过该SKU
			logrus.Infof("价格为0，跳过该SKU: %s", varInfo.Variant.ASIN)
			continue
		}

		logrus.Debugf("成功创建SKU: ASIN %s, 属性值ID %d", varInfo.Variant.ASIN, varInfo.ValueID)
		skuList = append(skuList, *sku)
	}

	if len(skuList) == 0 {
		logrus.Infof("无法为主要属性值 %s 创建任何 SKU，可能是由于价格问题", req.PrimaryAttrValue)
		return []product.SKU{}, nil // 返回空列表而不是错误，避免整个流程失败
	}

	logrus.Infof("成功为主要属性值 %s 创建了 %d 个 SKU，去重后避免了属性值ID重复", req.PrimaryAttrValue, len(skuList))
	return skuList, nil
}
