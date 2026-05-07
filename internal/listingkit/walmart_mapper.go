package listingkit

import (
	"strings"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
)

func buildWalmartPackage(req *GenerateRequest, canonical *canonical.Product, image *productimage.ImageProcessResult) *WalmartPackage {
	if canonical == nil {
		return &WalmartPackage{ReviewNotes: []string{"canonical product is empty"}}
	}
	productType := lastCategory(canonical.CategoryPath)
	pkg := &WalmartPackage{
		ProductName:      withBrandHint(canonical.Title, req),
		Brand:            resolveBrand(canonical, req),
		ProductType:      productType,
		ShortDescription: firstNonEmpty(canonical.Description, strings.Join(canonical.SellingPoints, "; ")),
		LongDescription:  canonical.Description,
		KeyFeatures:      append([]string(nil), canonical.SellingPoints...),
		Attributes:       flattenAttributes(canonical.Attributes),
		Variants:         buildPlatformVariants(canonical),
		Images:           buildPlatformImages(canonical, image),
		Metadata: map[string]string{
			"target_platform": "walmart",
			"country":         req.Country,
			"language":        req.Language,
			"status":          "draft_adapter",
			"product_type":    productType,
		},
	}
	pkg.ReviewNotes = collectReviewNotes(canonical, image, "沃尔玛适配器目前是占位草稿，后续需要补类目、属性和 feed 导出规则")
	return pkg
}
