package shein

import (
	"strings"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
	common "task-processor/internal/publishing/common"
)

// RefreshDerivedState recomputes category-derived SHEIN data on an existing
// package without discarding manual top-level edits such as title, description,
// images, or already patched pricing fields.
func RefreshDerivedState(
	req *BuildRequest,
	canonical *canonical.Product,
	image *productimage.ImageProcessResult,
	pkg *Package,
	categoryResolver CategoryResolver,
	attributeResolver AttributeResolver,
	saleAttributeResolver SaleAttributeResolver,
	sizeAttributeHeaderResolver SizeAttributeHeaderResolver,
	pricingPolicy PricingPolicy,
) {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || canonical == nil {
		return
	}
	var previousDraftSKCs []SKCRequestDraft
	var previousPackageSKCs []SKCPackage
	if pkg.DraftPayload != nil {
		previousDraftSKCs = append(previousDraftSKCs, pkg.DraftPayload.SKCList...)
	}
	previousPackageSKCs = append(previousPackageSKCs, pkg.SkcList...)

	images := pkg.Images
	if images == nil {
		images = common.BuildImages(canonical, image)
	}
	if images == nil {
		images = &common.ImageSet{}
	}
	pkg.Images = images
	if len(pkg.SiteList) == 0 {
		pkg.SiteList = common.DefaultSites(countryOrDefault(req))
	}
	pkg.ProductAttributes = common.BuildAttributes(canonical.Attributes)
	if pkg.DraftPayload == nil {
		pkg.DraftPayload = &RequestDraft{}
	}
	pkg.DraftPayload.SiteList = append([]common.Site(nil), pkg.SiteList...)

	if attributeResolver != nil {
		pkg.AttributeResolution = attributeResolver.Resolve(req, canonical, pkg)
		ApplyAttributeResolution(pkg, pkg.AttributeResolution)
	}
	if saleAttributeResolver != nil {
		pkg.SaleAttributeResolution = saleAttributeResolver.Resolve(req, canonical, pkg)
	}
	if pkg.CategoryResolution != nil {
		pkg.CategoryResolution.SuggestedCategory = nil
	}

	variants := common.BuildVariants(canonical)
	groups := buildVariantGroups(pkg.ProductNameEn, variants, images, pkg.SaleAttributeResolution)
	pkg.SkcList = buildSKCs(groups)
	pkg.DraftPayload.SupplierCode = firstSupplierCode(pkg.SkcList)
	pkg.DraftPayload.SKCList = buildRequestSKCs(groups, images, pkg.SiteList, canonical, pricingPolicy)
	reapplyPreviousSKCImages(pkg, previousDraftSKCs, previousPackageSKCs)
	ApplySaleAttributeResolution(pkg, pkg.SaleAttributeResolution)
	applyProductSizeAttributesWithResolver(pkg, productSizeOrEmpty(req), sizeAttributeHeaderResolver, resolveBuildRequestContext(req))
	SetPreviewPayload(pkg, BuildPreviewProduct(pkg))
	NormalizePackageSemanticFields(pkg)
}

func productSizeOrEmpty(req *BuildRequest) string {
	if req == nil {
		return ""
	}
	return req.ProductSize
}

func countryOrDefault(req *BuildRequest) string {
	if req == nil {
		return "US"
	}
	country := strings.ToUpper(strings.TrimSpace(req.Country))
	if country == "" {
		return "US"
	}
	return country
}

func reapplyPreviousSKCImages(pkg *Package, previousDraftSKCs []SKCRequestDraft, previousPackageSKCs []SKCPackage) {
	if pkg == nil || pkg.DraftPayload == nil {
		return
	}
	for skcIndex := range pkg.DraftPayload.SKCList {
		draft := &pkg.DraftPayload.SKCList[skcIndex]
		mainImage, imageDraft := resolvePreviousSKCImage(draft, previousDraftSKCs, previousPackageSKCs)
		if imageDraft != nil {
			draft.ImageInfo = imageDraft
		}
		if strings.TrimSpace(mainImage) != "" {
			for skuIndex := range draft.SKUList {
				draft.SKUList[skuIndex].MainImage = mainImage
			}
			if skcIndex < len(pkg.SkcList) {
				pkg.SkcList[skcIndex].MainImageURL = mainImage
			}
		}
	}
}

func resolvePreviousSKCImage(current *SKCRequestDraft, previousDraftSKCs []SKCRequestDraft, previousPackageSKCs []SKCPackage) (string, *ImageDraft) {
	if current == nil {
		return "", nil
	}
	if matchedSKUImage := matchPreviousSKUImage(current, previousDraftSKCs); strings.TrimSpace(matchedSKUImage) != "" {
		return matchedSKUImage, &ImageDraft{MainImage: matchedSKUImage}
	}
	for _, previous := range previousDraftSKCs {
		if sameTrimmed(previous.SupplierCode, current.SupplierCode) {
			return requestSKCMainImage(&previous), cloneImageDraft(previous.ImageInfo)
		}
	}
	for _, previous := range previousPackageSKCs {
		if sameTrimmed(previous.SupplierCode, current.SupplierCode) {
			mainImage := strings.TrimSpace(previous.MainImageURL)
			if mainImage != "" {
				return mainImage, &ImageDraft{MainImage: mainImage}
			}
		}
	}
	return "", nil
}

func matchPreviousSKUImage(current *SKCRequestDraft, previousDraftSKCs []SKCRequestDraft) string {
	for _, candidate := range previousImageSKUCandidates(current) {
		if candidate == "" {
			continue
		}
		for _, previousSKC := range previousDraftSKCs {
			for _, previousSKU := range previousSKC.SKUList {
				for _, previousCandidate := range previousImageSKUCandidatesFromSKU(previousSKU) {
					if previousCandidate == "" {
						continue
					}
					if sameTrimmed(candidate, previousCandidate) && strings.TrimSpace(previousSKU.MainImage) != "" {
						return strings.TrimSpace(previousSKU.MainImage)
					}
				}
			}
		}
	}
	return ""
}

func previousImageSKUCandidates(current *SKCRequestDraft) []string {
	if current == nil {
		return nil
	}
	values := []string{current.SupplierCode, trimVariantSuffix(current.SupplierCode)}
	for _, sku := range current.SKUList {
		values = append(values, previousImageSKUCandidatesFromSKU(sku)...)
	}
	return values
}

func previousImageSKUCandidatesFromSKU(sku SKUDraft) []string {
	return []string{
		strings.TrimSpace(sku.Attributes["source_sds_sku"]),
		strings.TrimSpace(sku.SupplierSKU),
		trimVariantSuffix(sku.SupplierSKU),
	}
}

func trimVariantSuffix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if index := strings.LastIndex(value, "-"); index > 0 {
		return strings.TrimSpace(value[:index])
	}
	return value
}

func sameTrimmed(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func cloneImageDraft(info *ImageDraft) *ImageDraft {
	if info == nil {
		return nil
	}
	return &ImageDraft{
		MainImage: strings.TrimSpace(info.MainImage),
		Gallery:   append([]string(nil), info.Gallery...),
		WhiteBg:   strings.TrimSpace(info.WhiteBg),
		Source:    append([]string(nil), info.Source...),
	}
}

func requestSKCMainImage(skc *SKCRequestDraft) string {
	if skc == nil {
		return ""
	}
	if skc.ImageInfo != nil && strings.TrimSpace(skc.ImageInfo.MainImage) != "" {
		return strings.TrimSpace(skc.ImageInfo.MainImage)
	}
	for _, sku := range skc.SKUList {
		if strings.TrimSpace(sku.MainImage) != "" {
			return strings.TrimSpace(sku.MainImage)
		}
	}
	return ""
}
