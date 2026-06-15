package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

func applySheinSKCRevisionPatches(pkg *sheinpub.Package, patches []SheinSKCRevisionPatch) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || len(patches) == 0 {
		return
	}
	for _, patch := range patches {
		if strings.TrimSpace(patch.SupplierCode) == "" {
			continue
		}
		draft := findSheinRequestSKC(pkg.DraftPayload.SKCList, patch.SupplierCode)
		pkgSKC := findSheinPackageSKC(pkg.SkcList, patch.SupplierCode)
		if draft == nil {
			continue
		}
		if patch.SkcName != nil {
			draft.SkcName = strings.TrimSpace(*patch.SkcName)
			if pkgSKC != nil {
				pkgSKC.SkcName = draft.SkcName
			}
		}
		if patch.SaleName != nil {
			draft.SaleName = strings.TrimSpace(*patch.SaleName)
			if pkgSKC != nil {
				pkgSKC.SaleName = draft.SaleName
			}
		}
		if patch.MainImageURL != nil {
			image := strings.TrimSpace(*patch.MainImageURL)
			ensureSheinImageDraft(&draft.ImageInfo)
			draft.ImageInfo.MainImage = image
			if len(draft.SKUList) > 0 && strings.TrimSpace(draft.SKUList[0].MainImage) == "" {
				draft.SKUList[0].MainImage = image
			}
			if pkgSKC != nil {
				pkgSKC.MainImageURL = image
			}
		}
		if patch.SaleAttribute != nil {
			saleAttribute := *patch.SaleAttribute
			draft.SaleAttribute = &saleAttribute
			if pkg.SaleAttributeResolution == nil {
				pkg.SaleAttributeResolution = &sheinpub.SaleAttributeResolution{}
			}
			pkg.SaleAttributeResolution.SKCAttributes = []sheinpub.ResolvedSaleAttribute{saleAttribute}
			if saleAttribute.AttributeID > 0 {
				pkg.SaleAttributeResolution.PrimaryAttributeID = saleAttribute.AttributeID
			}
		}
		applySheinSKURevisionPatches(pkg, draft, pkgSKC, patch.SKUPatches)
	}
}

func applySheinSKURevisionPatches(pkg *sheinpub.Package, draft *sheinpub.SKCRequestDraft, pkgSKC *sheinpub.SKCPackage, patches []SheinSKURevisionPatch) {
	if pkg == nil || draft == nil || len(patches) == 0 {
		return
	}
	for _, patch := range patches {
		if strings.TrimSpace(patch.SupplierSKU) == "" {
			continue
		}
		skuDraft := findSheinRequestSKU(draft.SKUList, patch.SupplierSKU)
		pkgSKU := findSheinPackageSKU(pkgSKC, patch.SupplierSKU)
		if skuDraft == nil {
			continue
		}
		if patch.Attributes != nil {
			skuDraft.Attributes = cloneMap(patch.Attributes)
			if pkgSKU != nil {
				pkgSKU.Attributes = cloneMap(patch.Attributes)
			}
		}
		if patch.BasePrice != nil {
			skuDraft.BasePrice = strings.TrimSpace(*patch.BasePrice)
		}
		if patch.CostPrice != nil {
			skuDraft.CostPrice = strings.TrimSpace(*patch.CostPrice)
		}
		if patch.Currency != nil {
			skuDraft.Currency = strings.TrimSpace(*patch.Currency)
		}
		if patch.StockCount != nil {
			skuDraft.StockCount = *patch.StockCount
			if pkgSKU != nil {
				pkgSKU.Stock = *patch.StockCount
			}
		}
		if patch.MainImage != nil {
			skuDraft.MainImage = strings.TrimSpace(*patch.MainImage)
			if pkgSKU != nil {
				pkgSKU.Image = skuDraft.MainImage
			}
		}
		if patch.Barcode != nil {
			skuDraft.Barcode = strings.TrimSpace(*patch.Barcode)
			if pkgSKU != nil {
				pkgSKU.Barcode = skuDraft.Barcode
			}
		}
		if patch.SaleAttributes != nil {
			skuDraft.SaleAttributes = append([]SheinResolvedSaleAttribute(nil), patch.SaleAttributes...)
			if pkg.SaleAttributeResolution == nil {
				pkg.SaleAttributeResolution = &sheinpub.SaleAttributeResolution{}
			}
			pkg.SaleAttributeResolution.SKUAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), patch.SaleAttributes...)
			if len(patch.SaleAttributes) > 0 && patch.SaleAttributes[0].AttributeID > 0 {
				pkg.SaleAttributeResolution.SecondaryAttributeID = patch.SaleAttributes[0].AttributeID
			}
		}
		if patch.SitePriceList != nil {
			skuDraft.SitePriceList = append([]sheinpub.SitePrice(nil), patch.SitePriceList...)
		}
		if patch.StockInfoList != nil {
			skuDraft.StockInfoList = append([]sheinpub.StockInfo(nil), patch.StockInfoList...)
		}
	}
}
