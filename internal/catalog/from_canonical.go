package catalog

import (
	"fmt"
	"sort"
	"strings"

	"task-processor/internal/catalog/canonical"
)

func BuildProduct(product *canonical.Product) *Product {
	if product == nil {
		return nil
	}

	catalogProduct := &Product{
		Title:          product.Title,
		Brand:          product.Brand,
		CategoryPath:   cloneStrings(product.CategoryPath),
		Description:    product.Description,
		SellingPoints:  cloneStrings(product.SellingPoints),
		SEOKeywords:    cloneStrings(product.SEOKeywords),
		Attributes:     buildAttributes(product.Attributes),
		Specifications: buildSpecifications(product.Specifications),
		Variants:       buildVariants(product.Variants),
		Images:         buildImages(product.Images),
		Review:         buildReviewState(product),
		Sources:        collectSources(product),
	}

	return catalogProduct
}

func buildAttributes(attrs map[string]canonical.Attribute) []Attribute {
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

func buildVariants(variants []canonical.Variant) []Variant {
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

func buildImages(images []canonical.Image) []Image {
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

func buildPrice(price *canonical.PriceInfo) *Price {
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

func buildSpecifications(specs *canonical.ProductSpecs) *Specifications {
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

func buildDimensions(dim *canonical.Dimensions) *Dimensions {
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

func buildWeight(weight *canonical.Weight) *Weight {
	if weight == nil {
		return nil
	}
	return &Weight{
		Value: weight.Value,
		Unit:  weight.Unit,
	}
}

func buildPackage(pkg *canonical.PackageInfo) *PackageInfo {
	if pkg == nil {
		return nil
	}
	return &PackageInfo{
		Dimensions: buildDimensions(pkg.Dimensions),
		Weight:     buildWeight(pkg.Weight),
		Quantity:   pkg.Quantity,
	}
}

func buildTrace(trace canonical.FieldTrace) Trace {
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

func buildReviewState(product *canonical.Product) *ReviewState {
	reasons := collectReviewReasons(product)
	return &ReviewState{
		NeedsReview: product.NeedsReview || len(reasons) > 0,
		Reasons:     reasons,
	}
}

func collectReviewReasons(product *canonical.Product) []string {
	if product == nil {
		return []string{"缺少商品事实数据"}
	}

	reasons := make([]string, 0)
	if strings.TrimSpace(product.Title) == "" {
		reasons = append(reasons, "缺少商品标题")
	}
	if strings.TrimSpace(product.Description) == "" {
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
	keys := make([]string, 0, len(product.FieldTraces))
	for key := range product.FieldTraces {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		trace := product.FieldTraces[key]
		if !trace.NeedsReview {
			continue
		}
		label := fieldLabels[key]
		if label == "" {
			label = key
		}
		reasons = append(reasons, fmt.Sprintf("%s待人工确认", label))
	}

	for key, attr := range product.Attributes {
		if attr.Trace.NeedsReview {
			reasons = append(reasons, fmt.Sprintf("属性待确认: %s", key))
		}
	}
	for _, variant := range product.Variants {
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

func collectSources(product *canonical.Product) []SourceRecord {
	if product == nil {
		return nil
	}
	items := make([]SourceRecord, 0)
	appendTraceSources := func(trace canonical.FieldTrace) {
		for _, source := range trace.Sources {
			items = append(items, SourceRecord{
				Type:   string(source.Type),
				Detail: source.Detail,
			})
		}
	}

	for _, trace := range product.FieldTraces {
		appendTraceSources(trace)
	}
	for _, attr := range product.Attributes {
		appendTraceSources(attr.Trace)
	}
	for _, image := range product.Images {
		appendTraceSources(image.Trace)
	}
	for _, variant := range product.Variants {
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
