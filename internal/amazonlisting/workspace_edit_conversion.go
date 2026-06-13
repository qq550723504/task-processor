package amazonlisting

import (
	"strings"

	"task-processor/internal/catalog/canonical"
)

func syncDraftFromCanonical(draft *AmazonListingDraft, product *canonical.Product) {
	if draft == nil || product == nil {
		return
	}
	draft.CanonicalProduct = product
	draft.Title = product.Title
	draft.Brand = product.Brand
	draft.Description = product.Description
	draft.CategoryPath = append([]string(nil), product.CategoryPath...)
	draft.BulletPoints = append([]string(nil), product.SellingPoints...)
	draft.SearchTerms = append([]string(nil), product.SEOKeywords...)
	if draft.Attributes == nil {
		draft.Attributes = map[string]string{}
	}
	for key := range draft.Attributes {
		if key == "brand" {
			continue
		}
		delete(draft.Attributes, key)
	}
	for key, attr := range product.Attributes {
		draft.Attributes[key] = attr.Value
	}
	if draft.Brand != "" {
		draft.Attributes["brand"] = draft.Brand
	}
	if len(product.CategoryPath) > 0 {
		draft.ProductType = product.CategoryPath[len(product.CategoryPath)-1]
	} else if product.Title != "" {
		draft.ProductType = product.Title
	}
	if product.Specifications != nil {
		if product.Specifications.Dimensions != nil {
			draft.Dimensions = &AmazonDimensions{
				Length: product.Specifications.Dimensions.Length,
				Width:  product.Specifications.Dimensions.Width,
				Height: product.Specifications.Dimensions.Height,
				Unit:   product.Specifications.Dimensions.Unit,
			}
		}
		if product.Specifications.Weight != nil {
			draft.Weight = &AmazonWeight{
				Value: product.Specifications.Weight.Value,
				Unit:  product.Specifications.Weight.Unit,
			}
		}
		if product.Specifications.Package != nil {
			draft.Package = &AmazonPackageInfo{
				Quantity: product.Specifications.Package.Quantity,
			}
			if product.Specifications.Package.Dimensions != nil {
				draft.Package.Dimensions = &AmazonDimensions{
					Length: product.Specifications.Package.Dimensions.Length,
					Width:  product.Specifications.Package.Dimensions.Width,
					Height: product.Specifications.Package.Dimensions.Height,
					Unit:   product.Specifications.Package.Dimensions.Unit,
				}
			}
			if product.Specifications.Package.Weight != nil {
				draft.Package.Weight = &AmazonWeight{
					Value: product.Specifications.Package.Weight.Value,
					Unit:  product.Specifications.Package.Weight.Unit,
				}
			}
		}
	}
	if len(product.Variants) > 0 {
		draft.Variants = draft.Variants[:0]
		for _, variant := range product.Variants {
			converted := AmazonVariantDraft{
				SKU:       variant.SKU,
				Inventory: variant.Stock,
				Barcode:   variant.Barcode,
				IsDefault: variant.IsDefault,
			}
			if len(variant.Attributes) > 0 {
				converted.Attributes = make(map[string]string, len(variant.Attributes))
				for key, attr := range variant.Attributes {
					converted.Attributes[key] = attr.Value
				}
			}
			if variant.Price != nil {
				converted.Price = &AmazonMoney{
					Currency: variant.Price.Currency,
					Amount:   variant.Price.Amount,
				}
				if variant.Price.CostPrice > 0 {
					converted.CostPrice = &AmazonMoney{
						Currency: variant.Price.Currency,
						Amount:   variant.Price.CostPrice,
					}
				}
			}
			if len(variant.Images) > 0 {
				converted.MainImage = variant.Images[0].URL
			}
			draft.Variants = append(draft.Variants, converted)
		}
	}
}

func canonicalProductFromDraft(draft *AmazonListingDraft) *canonical.Product {
	if draft == nil {
		return nil
	}
	product := &canonical.Product{
		Title:         draft.Title,
		Brand:         draft.Brand,
		CategoryPath:  append([]string(nil), draft.CategoryPath...),
		Description:   draft.Description,
		SellingPoints: append([]string(nil), draft.BulletPoints...),
		SEOKeywords:   append([]string(nil), draft.SearchTerms...),
		Attributes:    map[string]canonical.Attribute{},
		FieldTraces:   map[string]canonical.FieldTrace{},
	}
	if draft.Dimensions != nil || draft.Weight != nil {
		product.Specifications = &canonical.ProductSpecs{}
		if draft.Dimensions != nil {
			product.Specifications.Dimensions = &canonical.Dimensions{
				Length: draft.Dimensions.Length,
				Width:  draft.Dimensions.Width,
				Height: draft.Dimensions.Height,
				Unit:   draft.Dimensions.Unit,
			}
		}
		if draft.Weight != nil {
			product.Specifications.Weight = &canonical.Weight{
				Value: draft.Weight.Value,
				Unit:  draft.Weight.Unit,
			}
		}
	}
	if draft.Package != nil {
		ensureCanonicalSpecifications(product)
		product.Specifications.Package = &canonical.PackageInfo{
			Quantity: draft.Package.Quantity,
		}
		if draft.Package.Dimensions != nil {
			product.Specifications.Package.Dimensions = &canonical.Dimensions{
				Length: draft.Package.Dimensions.Length,
				Width:  draft.Package.Dimensions.Width,
				Height: draft.Package.Dimensions.Height,
				Unit:   draft.Package.Dimensions.Unit,
			}
		}
		if draft.Package.Weight != nil {
			product.Specifications.Package.Weight = &canonical.Weight{
				Value: draft.Package.Weight.Value,
				Unit:  draft.Package.Weight.Unit,
			}
		}
	}
	for key, value := range draft.Attributes {
		product.Attributes[key] = canonical.Attribute{
			Value: value,
			Trace: manualFieldTrace(),
		}
	}
	if len(draft.Variants) > 0 {
		product.Variants = make([]canonical.Variant, 0, len(draft.Variants))
		for _, variant := range draft.Variants {
			converted := canonical.Variant{
				SKU:        variant.SKU,
				Attributes: map[string]canonical.Attribute{},
				Stock:      variant.Inventory,
				Barcode:    variant.Barcode,
				IsDefault:  variant.IsDefault,
				Trace:      manualFieldTrace(),
			}
			for key, value := range variant.Attributes {
				converted.Attributes[key] = canonical.Attribute{
					Value: value,
					Trace: manualFieldTrace(),
				}
			}
			if variant.Price != nil {
				converted.Price = &canonical.PriceInfo{
					Currency: variant.Price.Currency,
					Amount:   variant.Price.Amount,
				}
				if variant.CostPrice != nil {
					converted.Price.CostPrice = variant.CostPrice.Amount
				}
			}
			if strings.TrimSpace(variant.MainImage) != "" {
				converted.Images = []canonical.Image{{
					URL:   strings.TrimSpace(variant.MainImage),
					Role:  "variant",
					Trace: manualFieldTrace(),
				}}
			}
			product.Variants = append(product.Variants, converted)
		}
	}
	product.FieldTraces["title"] = manualFieldTrace()
	product.FieldTraces["brand"] = manualFieldTrace()
	product.FieldTraces["category_path"] = manualFieldTrace()
	product.FieldTraces["description"] = manualFieldTrace()
	product.FieldTraces["selling_points"] = manualFieldTrace()
	product.FieldTraces["seo_keywords"] = manualFieldTrace()
	if product.Specifications != nil {
		product.FieldTraces["specifications"] = manualFieldTrace()
	}
	product.NeedsReview = canonicalProductNeedsReview(product)
	return product
}
