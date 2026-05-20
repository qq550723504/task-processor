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
	pricingPolicy PricingPolicy,
) {
	if pkg == nil || canonical == nil {
		return
	}

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
	if pkg.RequestDraft == nil {
		pkg.RequestDraft = &RequestDraft{}
	}
	pkg.RequestDraft.SiteList = append([]common.Site(nil), pkg.SiteList...)

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
	pkg.RequestDraft.SupplierCode = firstSupplierCode(pkg.SkcList)
	pkg.RequestDraft.SKCList = buildRequestSKCs(groups, images, pkg.SiteList, canonical, pricingPolicy)
	ApplySaleAttributeResolution(pkg, pkg.SaleAttributeResolution)
	pkg.PreviewProduct = BuildPreviewProduct(pkg)
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
