package shein

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinproduct "task-processor/internal/shein/api/product"
)

func BuildPreviewProduct(pkg *Package) *sheinproduct.Product {
	if pkg == nil || pkg.RequestDraft == nil {
		return nil
	}
	productTypeID := pkg.ProductTypeID
	return &sheinproduct.Product{
		SPUName:                 pkg.RequestDraft.SpuName,
		SupplierCode:            pkg.RequestDraft.SupplierCode,
		CategoryID:              pkg.CategoryID,
		CategoryIDList:          append([]int(nil), pkg.CategoryIDList...),
		ProductTypeID:           productTypeID,
		TopCategoryID:           pkg.TopCategoryID,
		SourceSystem:            "listingkit",
		MultiLanguageNameList:   toLanguageContents(pkg.RequestDraft.MultiLanguageNameList),
		MultiLanguageDescList:   toLanguageContents(pkg.RequestDraft.MultiLanguageDescList),
		ProductAttributeList:    BuildProductAttributes(pkg),
		ImageInfo:               toImageInfo(pkg.RequestDraft.ImageInfo),
		SiteList:                toSiteInfo(pkg.RequestDraft.SiteList),
		SKCList:                 toSKCs(pkg.RequestDraft.SKCList),
		CustomAttributeRelation: append([]sheinattribute.CustomAttributeRelation(nil), pkg.CustomAttributeRelation...),
		Extra:                   sheinproduct.Extra{SwitchToSPUPic: true, UseCVTransformImage: true},
		ProductCertificateList:  []int{},
		CertificateList:         []int{},
	}
}

func BuildProductAttributes(pkg *Package) []sheinproduct.ProductAttribute {
	return toResolvedAttributes(pkg)
}

func toLanguageContents(items []LocalizedText) []sheinproduct.LanguageContent {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.LanguageContent, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		lang := strings.TrimSpace(item.Language)
		if lang == "" {
			lang = "en"
		}
		result = append(result, sheinproduct.LanguageContent{Language: lang, Name: item.Name})
	}
	return result
}

func toResolvedAttributes(pkg *Package) []sheinproduct.ProductAttribute {
	if pkg != nil && len(pkg.ResolvedAttributes) > 0 {
		result := make([]sheinproduct.ProductAttribute, 0, len(pkg.ResolvedAttributes))
		compositionCount := countResolvedCompositionAttributes(pkg.ResolvedAttributes)
		seenAttributeIDs := make(map[int]struct{}, len(pkg.ResolvedAttributes))
		for _, item := range pkg.ResolvedAttributes {
			if item.AttributeID <= 0 {
				continue
			}
			if item.AttributeType == 2 {
				// Numeric display attributes such as length/width/height are
				// submitted through SKU dimension fields. Sending them again as
				// generic product attributes causes SHEIN pre-validation type
				// mismatches for attribute_type=2 templates.
				continue
			}
			if _, exists := seenAttributeIDs[item.AttributeID]; exists {
				continue
			}
			seenAttributeIDs[item.AttributeID] = struct{}{}
			result = append(result, sheinproduct.ProductAttribute{
				AttributeID:         item.AttributeID,
				AttributeValueID:    item.AttributeValueID,
				AttributeExtraValue: resolvedAttributeExtraValue(item, compositionCount),
			})
		}
		if len(result) > 0 {
			return result
		}
	}
	if pkg == nil || pkg.RequestDraft == nil {
		return nil
	}
	return toProductAttributes(pkg.RequestDraft.ProductAttributeList)
}

func countResolvedCompositionAttributes(items []ResolvedAttribute) int {
	count := 0
	for _, item := range items {
		if item.AttributeType == 3 {
			count++
		}
	}
	return count
}

func resolvedAttributeExtraValue(item ResolvedAttribute, compositionCount int) string {
	extraValue := strings.TrimSpace(item.AttributeExtraValue)
	if item.AttributeType != 3 || extraValue != "" {
		return extraValue
	}
	if len(parseCompositionItems(item.Value)) > 0 {
		if parsed := parseResolvedCompositionPercent(item.Value); parsed != "" {
			return parsed
		}
	}
	if item.AttributeValueID != nil && compositionCount == 1 {
		return "100"
	}
	return ""
}

func parseResolvedCompositionPercent(value string) string {
	items := parseCompositionItems(value)
	if len(items) != 1 {
		return ""
	}
	return common.FormatFloat(items[0].Percent)
}

func toProductAttributes(items []common.Attribute) []sheinproduct.ProductAttribute {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.ProductAttribute, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Value) == "" {
			continue
		}
		result = append(result, sheinproduct.ProductAttribute{AttributeExtraValue: item.Value})
	}
	return result
}

func toImageInfo(info *ImageDraft) *sheinproduct.ImageInfo {
	if info == nil {
		return nil
	}
	images := make([]sheinproduct.ImageDetail, 0, 1+len(info.Gallery))
	if strings.TrimSpace(info.MainImage) != "" {
		images = append(images, sheinproduct.ImageDetail{ImageType: 1, ImageSort: 1, ImageURL: info.MainImage, MarketingMainImage: true})
	}
	for idx, imageURL := range info.Gallery {
		if strings.TrimSpace(imageURL) == "" {
			continue
		}
		images = append(images, sheinproduct.ImageDetail{ImageType: 2, ImageSort: idx + 2, ImageURL: imageURL})
	}
	if strings.TrimSpace(info.WhiteBg) != "" {
		images = append(images, sheinproduct.ImageDetail{ImageType: 2, ImageSort: len(images) + 1, ImageURL: info.WhiteBg})
	}
	if len(images) == 0 {
		return nil
	}
	return &sheinproduct.ImageInfo{ImageInfoList: images}
}

func toSiteInfo(items []common.Site) []sheinproduct.SiteInfo {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.SiteInfo, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.MainSite) == "" {
			continue
		}
		result = append(result, sheinproduct.SiteInfo{MainSite: item.MainSite, SubSiteList: append([]string(nil), item.SubSites...)})
	}
	return result
}

func toSKCs(items []SKCRequestDraft) []sheinproduct.SKC {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.SKC, 0, len(items))
	for _, item := range items {
		supplierCode := item.SupplierCode
		saleAttribute := sheinproduct.SaleAttribute{PreFillSpec: true}
		if item.SaleAttribute != nil && item.SaleAttribute.AttributeID > 0 && item.SaleAttribute.AttributeValueID != nil {
			saleAttribute = sheinproduct.SaleAttribute{
				AttributeID:      item.SaleAttribute.AttributeID,
				AttributeValueID: *item.SaleAttribute.AttributeValueID,
				PreFillSpec:      true,
			}
		}
		result = append(result, sheinproduct.SKC{
			SaleAttribute:         saleAttribute,
			SKUS:                  toSKUs(item.SKUList),
			ImageInfo:             derefImageInfo(toImageInfo(item.ImageInfo)),
			MultiLanguageName:     sheinproduct.LanguageContent{Language: "en", Name: item.SkcName},
			MultiLanguageNameList: toLanguageContents(item.MultiLanguageNameList),
			Sort:                  item.Sort,
			SupplierCode:          &supplierCode,
			IsFirstPublish:        true,
		})
	}
	return result
}

func toSKUs(items []SKUDraft) []sheinproduct.SKU {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.SKU, 0, len(items))
	for _, item := range items {
		stockCount := item.StockCount
		result = append(result, sheinproduct.SKU{
			SaleAttributeList: toSaleAttributes(item.SaleAttributes),
			CostInfo:          &sheinproduct.CostInfo{CostPrice: item.CostPrice, Currency: item.Currency},
			Height:            item.Height,
			Length:            item.Length,
			Width:             item.Width,
			LengthUnit:        item.LengthUnit,
			Weight:            item.Weight,
			WeightUnit:        item.WeightUnit,
			StockCount:        &stockCount,
			SupplierSKU:       item.SupplierSKU,
			StopPurchase:      0,
			MallState:         1,
			PriceInfoList:     toPriceInfoList(item.SitePriceList),
			ImageInfo:         toSKUImageInfo(item.MainImage),
			StockInfoList:     toStockInfoList(item.StockInfoList),
			PackageType:       3,
		})
	}
	return result
}

func toSaleAttributes(items []ResolvedSaleAttribute) []sheinproduct.SaleAttribute {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.SaleAttribute, 0, len(items))
	for _, item := range items {
		if item.AttributeID <= 0 || item.AttributeValueID == nil {
			continue
		}
		result = append(result, sheinproduct.SaleAttribute{
			AttributeID:      item.AttributeID,
			AttributeValueID: *item.AttributeValueID,
		})
	}
	return result
}

func toPriceInfoList(items []SitePrice) []sheinproduct.PriceInfo {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.PriceInfo, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.BasePrice) == "" {
			continue
		}
		result = append(result, sheinproduct.PriceInfo{
			SubSite:   item.SubSite,
			BasePrice: common.ParseFloatDefault(item.BasePrice),
			Currency:  item.Currency,
		})
	}
	return result
}

func toStockInfoList(items []StockInfo) []sheinproduct.StockInfo {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.StockInfo, 0, len(items))
	for _, item := range items {
		result = append(result, sheinproduct.StockInfo{
			InventoryNum:          item.InventoryNum,
			MerchantWarehouseCode: item.WarehouseCode,
		})
	}
	return result
}

func toSKUImageInfo(image string) *sheinproduct.ImageInfo {
	if strings.TrimSpace(image) == "" {
		return nil
	}
	return &sheinproduct.ImageInfo{
		ImageInfoList: []sheinproduct.ImageDetail{{
			ImageType: 1,
			ImageSort: 1,
			ImageURL:  image,
		}},
	}
}

func derefImageInfo(info *sheinproduct.ImageInfo) sheinproduct.ImageInfo {
	if info == nil {
		return sheinproduct.ImageInfo{}
	}
	return *info
}
