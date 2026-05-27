package listingkit

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func ensureSheinImageDraft(info **sheinpub.ImageDraft) {
	if info == nil || *info != nil {
		return
	}
	*info = &sheinpub.ImageDraft{}
}

func findSheinRequestSKC(items []sheinpub.SKCRequestDraft, supplierCode string) *sheinpub.SKCRequestDraft {
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].SupplierCode), strings.TrimSpace(supplierCode)) {
			return &items[i]
		}
	}
	return nil
}

func findSheinPackageSKC(items []sheinpub.SKCPackage, supplierCode string) *sheinpub.SKCPackage {
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].SupplierCode), strings.TrimSpace(supplierCode)) {
			return &items[i]
		}
	}
	return nil
}

func findSheinRequestSKU(items []sheinpub.SKUDraft, supplierSKU string) *sheinpub.SKUDraft {
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].SupplierSKU), strings.TrimSpace(supplierSKU)) {
			return &items[i]
		}
	}
	return nil
}

func findSheinPackageSKU(skc *sheinpub.SKCPackage, supplierSKU string) *common.Variant {
	if skc == nil {
		return nil
	}
	for i := range skc.SKUs {
		if strings.EqualFold(strings.TrimSpace(skc.SKUs[i].SKU), strings.TrimSpace(supplierSKU)) {
			return &skc.SKUs[i]
		}
	}
	return nil
}

func ensureSheinRequestDraft(pkg *sheinpub.Package) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload != nil {
		return
	}
	pkg.DraftPayload = &sheinpub.RequestDraft{}
	sheinpub.NormalizePackageSemanticFields(pkg)
}

func syncSheinDraftFromPackage(pkg *sheinpub.Package) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return
	}
	if strings.TrimSpace(pkg.SpuName) != "" {
		pkg.DraftPayload.SpuName = pkg.SpuName
	}
	if pkg.Images != nil {
		pkg.DraftPayload.ImageInfo = sheinpub.BuildImageDraft(pkg.Images)
	}
	if pkg.ProductAttributes != nil {
		pkg.DraftPayload.ProductAttributeList = append([]common.Attribute(nil), pkg.ProductAttributes...)
	}
	if pkg.ResolvedAttributes != nil {
		pkg.DraftPayload.ResolvedAttributes = append([]sheinpub.ResolvedAttribute(nil), pkg.ResolvedAttributes...)
	}
	if strings.TrimSpace(pkg.Description) != "" {
		updateLocalizedTexts(&pkg.DraftPayload.MultiLanguageDescList, pkg.Description)
	}
	name := firstNonEmpty(pkg.ProductNameEn, pkg.SpuName)
	if strings.TrimSpace(name) != "" {
		updateLocalizedTexts(&pkg.DraftPayload.MultiLanguageNameList, name)
	}
	sheinpub.NormalizePackageSemanticFields(pkg)
}

func updateLocalizedTexts(items *[]sheinpub.LocalizedText, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if items == nil {
		return
	}
	if len(*items) == 0 {
		*items = []sheinpub.LocalizedText{
			{Language: "en", Name: value},
		}
		return
	}
	for i := range *items {
		(*items)[i].Name = value
	}
}
