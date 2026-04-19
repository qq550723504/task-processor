package listingkit

import (
	"strings"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func buildTemuPackage(req *GenerateRequest, canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult) *TemuPackage {
	if canonical == nil {
		return &TemuPackage{ReviewNotes: []string{"canonical product is empty"}}
	}

	images := buildPlatformImages(canonical, image)
	variants := buildPlatformVariants(canonical)
	pkg := &TemuPackage{
		GoodsName:        withBrandHint(canonical.Title, req),
		CategoryPath:     append([]string(nil), canonical.CategoryPath...),
		ShortDescription: canonical.Description,
		BulletPoints:     append([]string(nil), canonical.SellingPoints...),
		Attributes:       flattenAttributes(canonical.Attributes),
		SkcList:          buildTemuSKCs(variants, images),
		BatchSkuInfo:     buildTemuBatchSKUInfo(variants, canonical),
		Images:           images,
		Metadata: map[string]string{
			"target_platform": "temu",
			"country":         req.Country,
			"language":        req.Language,
			"goods_type":      "normal",
			"category_name":   lastCategory(canonical.CategoryPath),
		},
		CategoryDisclaimer: nil,
	}
	if strings.TrimSpace(req.TargetCategoryHint) != "" {
		pkg.Metadata["target_category_hint"] = req.TargetCategoryHint
	}
	pkg.ReviewNotes = collectReviewNotes(canonical, image, "TEMU 资料包已贴近 goods_basic/skc_list 结构，但类目 ID、属性 ID、承诺/扩展字段仍需接 TEMU 模板规则")
	return pkg
}

func buildTemuSKCs(variants []PlatformVariant, images *PlatformImageSet) []TemuSKCPackage {
	if len(variants) == 0 {
		return nil
	}
	result := make([]TemuSKCPackage, 0, len(variants))
	for idx, variant := range variants {
		colorValue := firstNonEmpty(variant.Attributes["color"], variant.Attributes["colour"], variant.Attributes["style"], variant.SKU)
		specs := make([]TemuSpecPackage, 0, len(variant.Attributes))
		for key, value := range variant.Attributes {
			specs = append(specs, TemuSpecPackage{Name: key, Value: value})
		}
		colorImageURL := firstNonEmpty(variant.Image, images.MainImage)
		result = append(result, TemuSKCPackage{
			Priority:        idx + 1,
			ColorImageURL:   colorImageURL,
			Spec:            append([]TemuSpecPackage{{Name: "variation", Value: colorValue}}, specs...),
			CarouselGallery: append([]string(nil), images.Gallery...),
			SKUs:            []PlatformVariant{variant},
		})
	}
	return result
}

func buildTemuBatchSKUInfo(variants []PlatformVariant, canonical *productenrich.CanonicalProduct) *TemuBatchSKUInfo {
	if len(variants) == 0 {
		return nil
	}
	base := variants[0]
	info := &TemuBatchSKUInfo{
		Currency: firstNonEmpty(basePriceCurrency(base), "USD"),
		Quantity: "999",
		OutSkuSN: base.SKU,
	}
	if base.Price != nil {
		info.Price = formatFloat(base.Price.Amount)
		info.CostPrice = formatFloat(base.Price.CostPrice)
	}
	if canonical != nil && canonical.Specifications != nil {
		if canonical.Specifications.Weight != nil {
			info.Weight = formatFloat(canonical.Specifications.Weight.Value)
		}
		if canonical.Specifications.Dimensions != nil {
			info.Length = formatFloat(canonical.Specifications.Dimensions.Length)
			info.Width = formatFloat(canonical.Specifications.Dimensions.Width)
			info.Height = formatFloat(canonical.Specifications.Dimensions.Height)
		}
	}
	return info
}

func basePriceCurrency(variant PlatformVariant) string {
	if variant.Price == nil {
		return ""
	}
	return variant.Price.Currency
}
