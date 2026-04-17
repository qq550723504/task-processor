package listingkit

import (
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
)

func buildSheinPreviewProduct(pkg *SheinPackage) *sheinproduct.Product {
	if pkg == nil || pkg.RequestDraft == nil {
		return nil
	}

	productTypeID := pkg.ProductTypeID
	product := &sheinproduct.Product{
		SPUName:                pkg.RequestDraft.SpuName,
		SupplierCode:           pkg.RequestDraft.SupplierCode,
		CategoryID:             pkg.CategoryID,
		CategoryIDList:         append([]int(nil), pkg.CategoryIDList...),
		ProductTypeID:          productTypeID,
		TopCategoryID:          pkg.TopCategoryID,
		SourceSystem:           "listingkit",
		MultiLanguageNameList:  toSheinLanguageContents(pkg.RequestDraft.MultiLanguageNameList),
		MultiLanguageDescList:  toSheinLanguageContents(pkg.RequestDraft.MultiLanguageDescList),
		ProductAttributeList:   toSheinResolvedAttributes(pkg),
		ImageInfo:              toSheinImageInfo(pkg.RequestDraft.ImageInfo),
		SiteList:               toSheinSiteInfo(pkg.RequestDraft.SiteList),
		SKCList:                toSheinSKCs(pkg.RequestDraft.SKCList),
		Extra:                  sheinproduct.Extra{SwitchToSPUPic: true, UseCVTransformImage: true},
		ProductCertificateList: []int{},
		CertificateList:        []int{},
	}
	return product
}

func toSheinLanguageContents(items []LocalizedText) []sheinproduct.LanguageContent {
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
		result = append(result, sheinproduct.LanguageContent{
			Language: lang,
			Name:     item.Name,
		})
	}
	return result
}

func toSheinProductAttributes(items []PlatformAttribute) []sheinproduct.ProductAttribute {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.ProductAttribute, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Value) == "" {
			continue
		}
		result = append(result, sheinproduct.ProductAttribute{
			AttributeExtraValue: item.Value,
		})
	}
	return result
}

func toSheinResolvedAttributes(pkg *SheinPackage) []sheinproduct.ProductAttribute {
	if pkg != nil && len(pkg.ResolvedAttributes) > 0 {
		result := make([]sheinproduct.ProductAttribute, 0, len(pkg.ResolvedAttributes))
		for _, item := range pkg.ResolvedAttributes {
			if item.AttributeID <= 0 {
				continue
			}
			result = append(result, sheinproduct.ProductAttribute{
				AttributeID:         item.AttributeID,
				AttributeValueID:    item.AttributeValueID,
				CVSuggestType:       "",
				AttributeExtraValue: item.AttributeExtraValue,
			})
		}
		if len(result) > 0 {
			return result
		}
	}
	if pkg == nil || pkg.RequestDraft == nil {
		return nil
	}
	return toSheinProductAttributes(pkg.RequestDraft.ProductAttributeList)
}

func toSheinImageInfo(info *SheinImageDraft) *sheinproduct.ImageInfo {
	if info == nil {
		return nil
	}
	images := make([]sheinproduct.ImageDetail, 0, 1+len(info.Gallery))
	if strings.TrimSpace(info.MainImage) != "" {
		images = append(images, sheinproduct.ImageDetail{
			ImageType:          1,
			ImageSort:          1,
			ImageURL:           info.MainImage,
			MarketingMainImage: true,
		})
	}
	for idx, imageURL := range info.Gallery {
		if strings.TrimSpace(imageURL) == "" {
			continue
		}
		images = append(images, sheinproduct.ImageDetail{
			ImageType: 2,
			ImageSort: idx + 2,
			ImageURL:  imageURL,
		})
	}
	if strings.TrimSpace(info.WhiteBg) != "" {
		images = append(images, sheinproduct.ImageDetail{
			ImageType: 2,
			ImageSort: len(images) + 1,
			ImageURL:  info.WhiteBg,
		})
	}
	if len(images) == 0 {
		return nil
	}
	return &sheinproduct.ImageInfo{
		ImageInfoList: images,
	}
}

func toSheinSiteInfo(items []PlatformSite) []sheinproduct.SiteInfo {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.SiteInfo, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.MainSite) == "" {
			continue
		}
		result = append(result, sheinproduct.SiteInfo{
			MainSite:    item.MainSite,
			SubSiteList: append([]string(nil), item.SubSites...),
		})
	}
	return result
}

func toSheinSKCs(items []SheinSKCRequestDraft) []sheinproduct.SKC {
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
			SKUS:                  toSheinSKUs(item.SKUList),
			ImageInfo:             derefSheinImageInfo(toSheinImageInfo(item.ImageInfo)),
			MultiLanguageName:     sheinproduct.LanguageContent{Language: "en", Name: item.SkcName},
			MultiLanguageNameList: toSheinLanguageContents(item.MultiLanguageNameList),
			Sort:                  item.Sort,
			SupplierCode:          &supplierCode,
			IsFirstPublish:        true,
		})
	}
	return result
}

func toSheinSKUs(items []SheinSKUDraft) []sheinproduct.SKU {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinproduct.SKU, 0, len(items))
	for _, item := range items {
		stockCount := item.StockCount
		result = append(result, sheinproduct.SKU{
			SaleAttributeList: toSheinSaleAttributes(item.SaleAttributes),
			CostInfo: &sheinproduct.CostInfo{
				CostPrice: item.CostPrice,
				Currency:  item.Currency,
			},
			Height:        item.Height,
			Length:        item.Length,
			Width:         item.Width,
			LengthUnit:    item.LengthUnit,
			Weight:        item.Weight,
			WeightUnit:    item.WeightUnit,
			StockCount:    &stockCount,
			SupplierSKU:   item.SupplierSKU,
			StopPurchase:  0,
			MallState:     1,
			PriceInfoList: toSheinPriceInfoList(item.SitePriceList),
			ImageInfo:     toSheinSKUImageInfo(item.MainImage),
			StockInfoList: toSheinStockInfoList(item.StockInfoList),
		})
	}
	return result
}

func toSheinSaleAttributes(items []SheinResolvedSaleAttribute) []sheinproduct.SaleAttribute {
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
			PreFillSpec:      false,
		})
	}
	return result
}

func toSheinPriceInfoList(items []SheinSitePrice) []sheinproduct.PriceInfo {
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
			BasePrice: parseFloatDefault(item.BasePrice),
			Currency:  item.Currency,
		})
	}
	return result
}

func toSheinStockInfoList(items []SheinStockInfo) []sheinproduct.StockInfo {
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

func toSheinSKUImageInfo(mainImage string) *sheinproduct.ImageInfo {
	mainImage = strings.TrimSpace(mainImage)
	if mainImage == "" {
		return nil
	}
	return &sheinproduct.ImageInfo{
		ImageInfoList: []sheinproduct.ImageDetail{{
			ImageType:          1,
			ImageSort:          1,
			ImageURL:           mainImage,
			MarketingMainImage: true,
		}},
	}
}

func derefSheinImageInfo(info *sheinproduct.ImageInfo) sheinproduct.ImageInfo {
	if info == nil {
		return sheinproduct.ImageInfo{}
	}
	return *info
}
