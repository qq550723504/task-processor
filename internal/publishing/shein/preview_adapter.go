package shein

import (
	"regexp"
	"strconv"
	"strings"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinproduct "task-processor/internal/shein/api/product"
)

var (
	quantityPrefixPattern = regexp.MustCompile(`(?i)\b(\d+)\s*[- ]?\s*(piece|pieces|piece\(s\)|pc|pcs|pack|packs|set|sets|pair|pairs)\b`)
	quantityOfPattern     = regexp.MustCompile(`(?i)\b(pack|set|bundle|collection)\s+of\s+(\d+)\b`)
)

func BuildPreviewProduct(pkg *Package) *sheinproduct.Product {
	NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return nil
	}
	productTypeID := pkg.ProductTypeID
	return &sheinproduct.Product{
		SPUName:                 pkg.DraftPayload.SpuName,
		SupplierCode:            pkg.DraftPayload.SupplierCode,
		CategoryID:              pkg.CategoryID,
		CategoryIDList:          append([]int(nil), pkg.CategoryIDList...),
		ProductTypeID:           productTypeID,
		TopCategoryID:           pkg.TopCategoryID,
		SourceSystem:            "listingkit",
		MultiLanguageNameList:   toLanguageContents(pkg.DraftPayload.MultiLanguageNameList),
		MultiLanguageDescList:   toLanguageContents(pkg.DraftPayload.MultiLanguageDescList),
		ProductAttributeList:    BuildProductAttributes(pkg),
		ImageInfo:               toImageInfo(pkg.DraftPayload.ImageInfo),
		SiteList:                toSiteInfo(pkg.DraftPayload.SiteList),
		SKCList:                 toSKCs(pkg.DraftPayload.SKCList),
		CustomAttributeRelation: append([]sheinattribute.CustomAttributeRelation(nil), pkg.CustomAttributeRelation...),
		Extra:                   sheinproduct.Extra{SwitchToSPUPic: true, UseCVTransformImage: true},
		ProductCertificateList:  []int{},
		CertificateList:         []int{},
	}
}

func BuildProductAttributes(pkg *Package) []sheinproduct.ProductAttribute {
	NormalizePackageSemanticFields(pkg)
	return toResolvedAttributes(pkg)
}

func ProductAttributesReadyForSubmit(attrs []sheinproduct.ProductAttribute) bool {
	if len(attrs) == 0 {
		return false
	}
	for _, attr := range attrs {
		if attr.AttributeID <= 0 {
			return false
		}
		if attr.AttributeValueID == nil && strings.TrimSpace(attr.AttributeExtraValue) == "" {
			return false
		}
	}
	return true
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
		seenAttributeKeys := make(map[string]struct{}, len(pkg.ResolvedAttributes))
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
			extraValue := resolvedAttributeExtraValue(pkg, item, compositionCount)
			dedupKey := resolvedAttributeDedupKey(item, extraValue)
			if _, exists := seenAttributeKeys[dedupKey]; exists {
				continue
			}
			seenAttributeKeys[dedupKey] = struct{}{}
			result = append(result, sheinproduct.ProductAttribute{
				AttributeID:         item.AttributeID,
				AttributeValueID:    item.AttributeValueID,
				AttributeExtraValue: extraValue,
			})
		}
		if len(result) > 0 {
			return result
		}
	}
	NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return nil
	}
	return toProductAttributes(pkg.DraftPayload.ProductAttributeList)
}

func resolvedAttributeDedupKey(item ResolvedAttribute, extraValue string) string {
	valueID := 0
	if item.AttributeValueID != nil {
		valueID = *item.AttributeValueID
	}
	return strconv.Itoa(item.AttributeID) + ":" + strconv.Itoa(valueID) + ":" + extraValue
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

func resolvedAttributeExtraValue(pkg *Package, item ResolvedAttribute, compositionCount int) string {
	extraValue := strings.TrimSpace(item.AttributeExtraValue)
	if extraValue != "" {
		return extraValue
	}
	if inferred := inferSupplementalAttributeExtraValue(pkg, item); inferred != "" {
		return inferred
	}
	if item.AttributeType != 3 {
		return ""
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

func inferSupplementalAttributeExtraValue(pkg *Package, item ResolvedAttribute) string {
	if numeric := firstNumericToken(item.Value); numeric != "" {
		return numeric
	}
	if !attributeNeedsSupplementalExtraValue(item) {
		return ""
	}
	if inferred := inferQuantityAttributeExtraValue(pkg, item); inferred != "" {
		return inferred
	}
	return ""
}

func attributeNeedsSupplementalExtraValue(item ResolvedAttribute) bool {
	if item.AttributeValueID == nil {
		return false
	}
	if strings.TrimSpace(item.AttributeExtraValue) != "" {
		return false
	}
	if item.AttributeID == 1000411 {
		return true
	}
	name := normalizeText(item.Name)
	value := normalizeText(item.Value)
	return strings.Contains(name, "quantity") ||
		strings.Contains(name, "count") ||
		strings.Contains(value, "piece") ||
		strings.Contains(value, "pack") ||
		strings.Contains(value, "set")
}

func inferQuantityAttributeExtraValue(pkg *Package, item ResolvedAttribute) string {
	if item.AttributeID != 1000411 && !strings.Contains(normalizeText(item.Name), "quantity") {
		return ""
	}
	for _, candidate := range collectSupplementalAttributeTextCandidates(pkg, item) {
		if value := inferQuantityFromText(candidate); value != "" {
			return value
		}
	}
	return "1"
}

func collectSupplementalAttributeTextCandidates(pkg *Package, item ResolvedAttribute) []string {
	candidates := make([]string, 0, 16)
	appendCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			candidates = append(candidates, value)
		}
	}
	appendCandidate(item.Value)
	if pkg == nil {
		return candidates
	}
	appendCandidate(pkg.ProductNameEn)
	appendCandidate(pkg.ProductNameMulti)
	appendCandidate(pkg.SpuName)
	appendCandidate(pkg.Description)
	for _, attr := range pkg.ProductAttributes {
		appendCandidate(attr.Name)
		appendCandidate(attr.Value)
	}
	for _, value := range pkg.Attributes {
		appendCandidate(value)
	}
	if pkg.DraftPayload != nil {
		appendCandidate(pkg.DraftPayload.SpuName)
		for _, text := range pkg.DraftPayload.MultiLanguageNameList {
			appendCandidate(text.Name)
		}
		for _, text := range pkg.DraftPayload.MultiLanguageDescList {
			appendCandidate(text.Name)
		}
		for _, attr := range pkg.DraftPayload.ProductAttributeList {
			appendCandidate(attr.Name)
			appendCandidate(attr.Value)
		}
	}
	return candidates
}

func inferQuantityFromText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if matches := quantityPrefixPattern.FindStringSubmatch(value); len(matches) > 1 {
		return matches[1]
	}
	if matches := quantityOfPattern.FindStringSubmatch(value); len(matches) > 2 {
		return matches[2]
	}
	return ""
}

func firstNumericToken(value string) string {
	if match := numericTokenPattern.FindString(strings.TrimSpace(value)); match != "" {
		if parsed, err := strconv.ParseFloat(match, 64); err == nil && parsed > 0 {
			return common.FormatFloat(parsed)
		}
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
