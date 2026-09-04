package listingkit

import (
	"strings"

	"task-processor/internal/product/catalog/canonical"
	common "task-processor/internal/publishing/common"
)

func buildWalmartPackage(req *GenerateRequest, canonical *canonical.Product) *WalmartPackage {
	if canonical == nil {
		return &WalmartPackage{ReviewNotes: []string{"canonical product is empty"}}
	}
	productType := common.LastCategory(canonical.CategoryPath)
	pkg := &WalmartPackage{
		ProductName:      common.WithBrandHint(canonical.Title, req.BrandHint),
		Brand:            common.ResolveBrand(req.BrandHint, canonical),
		ProductType:      productType,
		ShortDescription: firstNonEmpty(canonical.Description, strings.Join(canonical.SellingPoints, "; ")),
		LongDescription:  canonical.Description,
		KeyFeatures:      append([]string(nil), canonical.SellingPoints...),
		Attributes:       common.FlattenAttributes(canonical.Attributes),
		Variants:         common.BuildVariants(canonical),
		Images:           common.BuildImages(canonical),
		Metadata: map[string]string{
			"target_platform": "walmart",
			"country":         req.Country,
			"language":        req.Language,
			"status":          "draft_adapter",
			"product_type":    productType,
		},
	}
	pkg.ReviewNotes = common.CollectReviewNotes(canonical, "沃尔玛适配器目前是占位草稿，后续需要补类目、属性和 feed 导出规则")
	return pkg
}
