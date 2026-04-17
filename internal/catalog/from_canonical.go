package catalog

import (
	"fmt"
	"sort"
	"strings"

	"task-processor/internal/productenrich"
)

func BuildProduct(canonical *productenrich.CanonicalProduct) *Product {
	if canonical == nil {
		return nil
	}

	product := &Product{
		Title:          canonical.Title,
		Brand:          canonical.Brand,
		CategoryPath:   cloneStrings(canonical.CategoryPath),
		Description:    canonical.Description,
		SellingPoints:  cloneStrings(canonical.SellingPoints),
		SEOKeywords:    cloneStrings(canonical.SEOKeywords),
		Attributes:     buildAttributes(canonical.Attributes),
		Specifications: buildSpecifications(canonical.Specifications),
		Variants:       buildVariants(canonical.Variants),
		Images:         buildImages(canonical.Images),
		Review:         buildReviewState(canonical),
		Sources:        collectSources(canonical),
	}

	return product
}

func buildAttributes(attrs map[string]productenrich.CanonicalAttribute) []Attribute {
	if len(attrs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	items := make([]Attribute, 0, len(keys))
	for _, key := range keys {
		attr := attrs[key]
		items = append(items, Attribute{
			Name:  key,
			Value: attr.Value,
			Trace: buildTrace(attr.Trace),
		})
	}
	return items
}

func buildVariants(variants []productenrich.CanonicalVariant) []Variant {
	if len(variants) == 0 {
		return nil
	}
	items := make([]Variant, 0, len(variants))
	for _, variant := range variants {
		items = append(items, Variant{
			SKU:        variant.SKU,
			Attributes: buildAttributes(variant.Attributes),
			Price:      buildPrice(variant.Price),
			Stock:      variant.Stock,
			Images:     buildImages(variant.Images),
			Barcode:    variant.Barcode,
			IsDefault:  variant.IsDefault,
			Trace:      buildTrace(variant.Trace),
		})
	}
	return items
}

func buildImages(images []productenrich.CanonicalImage) []Image {
	if len(images) == 0 {
		return nil
	}
	items := make([]Image, 0, len(images))
	for _, image := range images {
		items = append(items, Image{
			URL:   image.URL,
			Role:  image.Role,
			Trace: buildTrace(image.Trace),
		})
	}
	return items
}

func buildPrice(price *productenrich.PriceInfo) *Price {
	if price == nil {
		return nil
	}
	return &Price{
		Currency:     price.Currency,
		Amount:       price.Amount,
		CompareAt:    price.CompareAt,
		CostPrice:    price.CostPrice,
		WholesaleMin: price.WholesaleMin,
	}
}

func buildSpecifications(specs *productenrich.ProductSpecs) *Specifications {
	if specs == nil {
		return nil
	}
	technical := make(map[string]string, len(specs.Technical))
	for key, value := range specs.Technical {
		technical[key] = value
	}
	return &Specifications{
		Dimensions: buildDimensions(specs.Dimensions),
		Weight:     buildWeight(specs.Weight),
		Package:    buildPackage(specs.Package),
		Technical:  technical,
	}
}

func buildDimensions(dim *productenrich.Dimensions) *Dimensions {
	if dim == nil {
		return nil
	}
	return &Dimensions{
		Length: dim.Length,
		Width:  dim.Width,
		Height: dim.Height,
		Unit:   dim.Unit,
	}
}

func buildWeight(weight *productenrich.Weight) *Weight {
	if weight == nil {
		return nil
	}
	return &Weight{
		Value: weight.Value,
		Unit:  weight.Unit,
	}
}

func buildPackage(pkg *productenrich.PackageInfo) *PackageInfo {
	if pkg == nil {
		return nil
	}
	return &PackageInfo{
		Dimensions: buildDimensions(pkg.Dimensions),
		Weight:     buildWeight(pkg.Weight),
		Quantity:   pkg.Quantity,
	}
}

func buildTrace(trace productenrich.FieldTrace) Trace {
	sources := make([]SourceRecord, 0, len(trace.Sources))
	for _, source := range trace.Sources {
		sources = append(sources, SourceRecord{
			Type:   string(source.Type),
			Detail: source.Detail,
		})
	}
	return Trace{
		Sources:     sources,
		Confidence:  trace.Confidence,
		IsInferred:  trace.IsInferred,
		NeedsReview: trace.NeedsReview,
	}
}

func buildReviewState(canonical *productenrich.CanonicalProduct) *ReviewState {
	reasons := collectReviewReasons(canonical)
	return &ReviewState{
		NeedsReview: canonical.NeedsReview || len(reasons) > 0,
		Reasons:     reasons,
	}
}

func collectReviewReasons(canonical *productenrich.CanonicalProduct) []string {
	if canonical == nil {
		return []string{"缺少商品事实数据"}
	}

	reasons := make([]string, 0)
	if strings.TrimSpace(canonical.Title) == "" {
		reasons = append(reasons, "缺少商品标题")
	}
	if strings.TrimSpace(canonical.Description) == "" {
		reasons = append(reasons, "缺少商品描述")
	}

	fieldLabels := map[string]string{
		"title":          "标题",
		"brand":          "品牌",
		"category_path":  "类目",
		"description":    "描述",
		"selling_points": "卖点",
		"seo_keywords":   "SEO关键词",
		"specifications": "规格参数",
	}
	keys := make([]string, 0, len(canonical.FieldTraces))
	for key := range canonical.FieldTraces {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		trace := canonical.FieldTraces[key]
		if !trace.NeedsReview {
			continue
		}
		label := fieldLabels[key]
		if label == "" {
			label = key
		}
		reasons = append(reasons, fmt.Sprintf("%s待人工确认", label))
	}

	for key, attr := range canonical.Attributes {
		if attr.Trace.NeedsReview {
			reasons = append(reasons, fmt.Sprintf("属性待确认: %s", key))
		}
	}
	for _, variant := range canonical.Variants {
		if variant.Trace.NeedsReview {
			reasons = append(reasons, fmt.Sprintf("变体待确认: %s", firstNonEmpty(variant.SKU, "未命名SKU")))
			continue
		}
		for key, attr := range variant.Attributes {
			if attr.Trace.NeedsReview {
				reasons = append(reasons, fmt.Sprintf("变体属性待确认: %s/%s", firstNonEmpty(variant.SKU, "未命名SKU"), key))
				break
			}
		}
	}

	return uniqueStrings(reasons)
}

func collectSources(canonical *productenrich.CanonicalProduct) []SourceRecord {
	if canonical == nil {
		return nil
	}
	items := make([]SourceRecord, 0)
	appendTraceSources := func(trace productenrich.FieldTrace) {
		for _, source := range trace.Sources {
			items = append(items, SourceRecord{
				Type:   string(source.Type),
				Detail: source.Detail,
			})
		}
	}

	for _, trace := range canonical.FieldTraces {
		appendTraceSources(trace)
	}
	for _, attr := range canonical.Attributes {
		appendTraceSources(attr.Trace)
	}
	for _, image := range canonical.Images {
		appendTraceSources(image.Trace)
	}
	for _, variant := range canonical.Variants {
		appendTraceSources(variant.Trace)
		for _, attr := range variant.Attributes {
			appendTraceSources(attr.Trace)
		}
		for _, image := range variant.Images {
			appendTraceSources(image.Trace)
		}
	}

	return uniqueSourceRecords(items)
}

func uniqueSourceRecords(items []SourceRecord) []SourceRecord {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]SourceRecord, 0, len(items))
	for _, item := range items {
		key := item.Type + "\x00" + item.Detail
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func cloneStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	return append([]string(nil), items...)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
