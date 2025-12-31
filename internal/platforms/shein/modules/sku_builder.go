// Package modules 提供SHEIN平台SKU构建核心功能
package modules

import (
	"fmt"
	"task-processor/internal/domain/model"
	"task-processor/internal/platforms/shein/api/product"

	"github.com/sirupsen/logrus"
)

// SKUBuilder SKU构建器
type SKUBuilder struct {
	variantMatcher    *VariantMatcher
	strategyProcessor *SKUStrategyProcessor
	creator           *SKUCreator
}

// NewSKUBuilder 创建新的SKU构建器
func NewSKUBuilder(variantMatcher *VariantMatcher) *SKUBuilder {
	return &SKUBuilder{
		variantMatcher:    variantMatcher,
		strategyProcessor: NewSKUStrategyProcessor(variantMatcher),
		creator:           NewSKUCreator(),
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
		return b.strategyProcessor.BuildSingleSKU(ctx, req)
	}

	// 如果没有次要属性，只创建一个 SKU
	if req.Strategy.SecondaryAttribute.AttrID <= 0 {
		logrus.Infof("🎯 检测到无次要属性，使用单SKU构建模式")
		return b.strategyProcessor.BuildSingleSKU(ctx, req)
	}

	logrus.Infof("🎯 检测到有次要属性，使用多SKU构建模式")
	logrus.Infof("  - 次要属性值数量: %d", len(req.Strategy.SecondaryAttribute.AttrValue))

	// 打印次要属性值详情
	for i, attrValue := range req.Strategy.SecondaryAttribute.AttrValue {
		logrus.Infof("  - 次要属性值[%d]: %s (ID: %d)", i+1, attrValue.Value, attrValue.ID.Int())
	}

	return b.strategyProcessor.BuildMultipleSKUs(ctx, req)
}

// BuildSKUListForSingleVariant 为单变体构建SKU列表
func (b *SKUBuilder) BuildSKUListForSingleVariant(ctx *TaskContext, variant Variant, strategy AttributeStrategy) ([]product.SKU, error) {
	logrus.Infof("为单变体构建SKU列表: ASIN %s", variant.ASIN)

	// 根据策略构建销售属性列表
	var saleAttributeList []product.SaleAttribute

	// 如果有次要属性，需要基于变体属性创建销售属性
	if strategy.SecondaryAttribute.AttrID != 0 {
		saleAttributeList = b.creator.BuildSaleAttributeListForSingleVariant(ctx, variant, strategy)
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
	sku, err := b.creator.CreateSKU(ctx, SKUCreationParams{
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
