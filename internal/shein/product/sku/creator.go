package sku

import (
	"fmt"
	"math"
	"math/rand"
	"strings"

	"task-processor/internal/core/logger"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/api/product"
	sheinattr "task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/store"
	"task-processor/internal/shein/validation"
)

type SKUCreator struct {
	utils *SKUUtils
}

func NewSKUCreator() *SKUCreator {
	return &SKUCreator{utils: NewSKUUtils()}
}

func (c *SKUCreator) CreateSKU(ctx *shein.TaskContext, params shein.SKUCreationParams) (*product.SKU, error) {
	salePriceMultiplier := ctx.ProfitRule.SalePriceMultiplier
	discountPriceMultiplier := ctx.ProfitRule.DiscountPriceMultiplier
	productPrice := validation.GetProductPrice(params.ProductInfo, ctx.StoreInfo.PriceType)
	originalPrice := math.Round(productPrice*100) / 100
	salePrice := math.Round(originalPrice*salePriceMultiplier*100) / 100
	var specialPrice float64
	if discountPriceMultiplier != 0 {
		specialPrice = math.Round(originalPrice*discountPriceMultiplier*100) / 100
	}
	if originalPrice <= 0 {
		logger.GetGlobalLogger("shein/product").Infof("ASIN %s has zero price, skipping SKU", params.ASIN)
		return nil, nil
	}

	supplierSKU := ""
	if ctx.AsinSkuMap != nil {
		if sku, exists := ctx.AsinSkuMap[params.ASIN]; exists {
			supplierSKU = sku
		}
	}

	stockCount := 0
	if ctx.StoreInfo.FixedStockCount != nil {
		stockCount = *ctx.StoreInfo.FixedStockCount
	}
	if stockCount == 0 {
		stockCount = rand.Intn(1000) + 10
	}

	currency := store.GetCurrencyByRegion(ctx.Task.Region)
	quantityInfo := c.utils.BuildQuantityInfo(params)
	skuImageInfo := c.utils.BuildSKUImageInfoForMultiPiece(ctx, params)

	skuItem := &product.SKU{
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
		PriceInfoList: []product.PriceInfo{{
			SubSite:   ctx.SiteList[0].SubSiteList[0],
			BasePrice: salePrice,
			SpecialPrice: func() *float64 {
				if specialPrice != 0 && specialPrice < salePrice {
					return &specialPrice
				}
				return nil
			}(),
			Currency: currency,
		}},
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
		Extra:                    product.SkuExtra{FieldDisabledInfo: product.FieldDisabledInfo{}},
	}

	logger.GetGlobalLogger("shein/product").Debugf("created SKU: ASIN=%s price=%.2f supplierSKU=%s saleAttrs=%d", params.ASIN, salePrice, supplierSKU, len(params.SaleAttributeList))
	return skuItem, nil
}

func (c *SKUCreator) BuildSaleAttributeListForSingleVariant(ctx *shein.TaskContext, variant shein.Variant, strategy sheinattr.AttributeStrategy) []product.SaleAttribute {
	return c.BuildSaleAttributeListForSingleVariantWithRuntime(newRuntimeInput(ctx), variant, strategy)
}

func (c *SKUCreator) BuildSaleAttributeListForSingleVariantWithRuntime(runtime *RuntimeInput, variant shein.Variant, strategy sheinattr.AttributeStrategy) []product.SaleAttribute {
	var saleAttributeList []product.SaleAttribute

	var attributeTemplates []attribute.AttributeTemplate
	if runtime != nil && runtime.AttributeTemplates != nil {
		attributeTemplates = runtime.AttributeTemplates.Data
	}
	secondaryAttrName := c.utils.GetAttributeName(strategy.SecondaryAttribute.AttrID, attributeTemplates)
	var secondaryAttrValue string
	found := false

	for _, attrName := range append([]string{secondaryAttrName}, c.utils.GetAttributeNameAlternatives(strategy.SecondaryAttribute.AttrID, attributeTemplates)...) {
		for attrKey, value := range variant.Attributes {
			if strings.EqualFold(attrKey, attrName) {
				secondaryAttrValue = value
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if found {
		var valueID int
		for _, attrValue := range strategy.SecondaryAttribute.AttrValue {
			if strings.EqualFold(attrValue.Value, secondaryAttrValue) {
				valueID = attrValue.ID.Int()
				if valueID <= 0 {
					valueID = 0
				}
				break
			}
		}

		if valueID > 0 {
			saleAttributeList = append(saleAttributeList, product.SaleAttribute{
				AttributeID:        strategy.SecondaryAttribute.AttrID,
				AttributeValueID:   valueID,
				IsSPPSaleAttribute: false,
				PreFillSpec:        false,
			})
		} else {
			logger.GetGlobalLogger("shein/product").Warnf("secondary attribute value %q has no valid ID", secondaryAttrValue)
		}
	} else {
		logger.GetGlobalLogger("shein/product").Warnf("secondary attribute %q not found in variant", secondaryAttrName)
	}

	return saleAttributeList
}
