package catalog

import "task-processor/internal/product/catalog/canonical"

// ProjectCanonical projects an independent view for existing platform builders.
// It does not publish facts, approve images, or load external inputs.
func ProjectCanonical(snapshot ProductSnapshot) *canonical.Product {
	product := &canonical.Product{
		Title:          snapshot.Title,
		Brand:          snapshot.Brand,
		CategoryPath:   append([]string(nil), snapshot.CategoryPath...),
		Description:    snapshot.Description,
		SellingPoints:  append([]string(nil), snapshot.SellingPoints...),
		SEOKeywords:    append([]string(nil), snapshot.SEOKeywords...),
		Attributes:     canonicalAttributesFromSnapshot(snapshot.Attributes),
		Specifications: canonicalSpecificationsFromSnapshot(snapshot.Specifications),
		Variants:       canonicalVariantsFromSnapshot(snapshot.Variants),
		Images:         canonicalImagesFromSnapshot(snapshot.Images),
	}
	if snapshot.Review != nil {
		product.NeedsReview = snapshot.Review.NeedsReview
	}
	return product
}

func canonicalAttributesFromSnapshot(attributes []Attribute) map[string]canonical.Attribute {
	if len(attributes) == 0 {
		return nil
	}
	result := make(map[string]canonical.Attribute, len(attributes))
	for _, attribute := range attributes {
		result[attribute.Name] = canonical.Attribute{
			Value: attribute.Value,
			Trace: canonicalTraceFromSnapshot(attribute.Trace),
		}
	}
	return result
}

func canonicalVariantsFromSnapshot(variants []Variant) []canonical.Variant {
	if len(variants) == 0 {
		return nil
	}
	result := make([]canonical.Variant, 0, len(variants))
	for _, variant := range variants {
		result = append(result, canonical.Variant{
			SKU:        variant.SKU,
			Attributes: canonicalAttributesFromSnapshot(variant.Attributes),
			Price:      canonicalPriceFromSnapshot(variant.Price),
			Stock:      variant.Stock,
			Images:     canonicalImagesFromSnapshot(variant.Images),
			Barcode:    variant.Barcode,
			IsDefault:  variant.IsDefault,
			Trace:      canonicalTraceFromSnapshot(variant.Trace),
		})
	}
	return result
}

func canonicalImagesFromSnapshot(images []Image) []canonical.Image {
	if len(images) == 0 {
		return nil
	}
	result := make([]canonical.Image, 0, len(images))
	for _, image := range images {
		result = append(result, canonical.Image{
			URL:   image.URL,
			Role:  image.Role,
			Trace: canonicalTraceFromSnapshot(image.Trace),
		})
	}
	return result
}

func canonicalTraceFromSnapshot(trace Trace) canonical.FieldTrace {
	sources := make([]canonical.Source, 0, len(trace.Sources))
	for _, source := range trace.Sources {
		sources = append(sources, canonical.Source{Type: canonical.SourceType(source.Type), Detail: source.Detail})
	}
	return canonical.FieldTrace{
		Sources:     sources,
		Confidence:  trace.Confidence,
		IsInferred:  trace.IsInferred,
		NeedsReview: trace.NeedsReview,
	}
}

func canonicalPriceFromSnapshot(price *Price) *canonical.PriceInfo {
	if price == nil {
		return nil
	}
	return &canonical.PriceInfo{
		Currency:     price.Currency,
		Amount:       price.Amount,
		CompareAt:    price.CompareAt,
		CostPrice:    price.CostPrice,
		WholesaleMin: price.WholesaleMin,
	}
}

func canonicalSpecificationsFromSnapshot(specifications *Specifications) *canonical.ProductSpecs {
	if specifications == nil {
		return nil
	}
	technical := make(map[string]string, len(specifications.Technical))
	for key, value := range specifications.Technical {
		technical[key] = value
	}
	return &canonical.ProductSpecs{
		Dimensions: canonicalDimensionsFromSnapshot(specifications.Dimensions),
		Weight:     canonicalWeightFromSnapshot(specifications.Weight),
		Package:    canonicalPackageFromSnapshot(specifications.Package),
		Technical:  technical,
	}
}

func canonicalDimensionsFromSnapshot(dimensions *Dimensions) *canonical.Dimensions {
	if dimensions == nil {
		return nil
	}
	return &canonical.Dimensions{
		Length: dimensions.Length,
		Width:  dimensions.Width,
		Height: dimensions.Height,
		Unit:   dimensions.Unit,
	}
}

func canonicalWeightFromSnapshot(weight *Weight) *canonical.Weight {
	if weight == nil {
		return nil
	}
	return &canonical.Weight{Value: weight.Value, Unit: weight.Unit}
}

func canonicalPackageFromSnapshot(pack *PackageInfo) *canonical.PackageInfo {
	if pack == nil {
		return nil
	}
	return &canonical.PackageInfo{
		Dimensions: canonicalDimensionsFromSnapshot(pack.Dimensions),
		Weight:     canonicalWeightFromSnapshot(pack.Weight),
		Quantity:   pack.Quantity,
	}
}
