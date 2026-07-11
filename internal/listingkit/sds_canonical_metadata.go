package listingkit

import (
	"strings"

	"task-processor/internal/catalog/canonical"
	sdspod "task-processor/internal/product/sourcing/sdspod"
	sheinpub "task-processor/internal/publishing/shein"
)

func applySDSSyncMetadataToCanonical(
	product *canonical.Product,
	summary *SDSSyncSummary,
	options *SDSSyncOptions,
) bool {
	if product == nil {
		return false
	}
	return sdspod.ApplyCanonical(
		product,
		buildSDSPODCanonicalMetadata(product, summary, options),
	)
}

func buildSDSPODCanonicalMetadata(
	product *canonical.Product,
	summary *SDSSyncSummary,
	options *SDSSyncOptions,
) sdspod.CanonicalMetadata {
	metadata := sdspod.CanonicalMetadata{}
	if summary != nil {
		metadata.ProductName = summary.ProductName
		metadata.ProductSKU = summary.ProductSKU
		metadata.VariantSKU = firstSDSVariantValue(
			summary.VariantSKU, summary.VariantResults,
			func(item SDSSyncSummary) string { return item.VariantSKU })
		metadata.VariantSize = firstSDSVariantValue(
			summary.VariantSize, summary.VariantResults,
			func(item SDSSyncSummary) string { return item.VariantSize })
		metadata.VariantColor = firstSDSVariantValue(
			summary.VariantColor, summary.VariantResults,
			func(item SDSSyncSummary) string { return item.VariantColor })
		metadata.MockupURLs = append([]string(nil), summary.MockupImageURLs...)
		metadata.Variants = make([]sdspod.VariantMetadata, 0,
			len(summary.VariantResults))
		for _, item := range summary.VariantResults {
			metadata.Variants = append(metadata.Variants,
				sdspod.VariantMetadata{
					SKU:        item.VariantSKU,
					Color:      item.VariantColor,
					Status:     item.Status,
					MockupURLs: append([]string(nil), item.MockupImageURLs...),
				})
		}
	}
	if options != nil {
		if strings.TrimSpace(metadata.ProductName) == "" {
			metadata.ProductName = options.ProductName
		}
		metadata.StyleName = studioStyleName(options)
		metadata.Attributes = map[string]string{}
		for key, attr := range studioAttributes(
			options, canonical.FieldTrace{}) {
			metadata.Attributes[key] = attr.Value
		}
	}
	metadata.VariantLookup = buildSDSPODVariantLookups(product)
	return metadata
}

func firstSDSVariantValue(
	direct string,
	items []SDSSyncSummary,
	pick func(SDSSyncSummary) string,
) string {
	if value := strings.TrimSpace(direct); value != "" {
		return value
	}
	for _, item := range items {
		if value := strings.TrimSpace(pick(item)); value != "" {
			return value
		}
	}
	return ""
}

func buildSDSPODVariantLookups(
	product *canonical.Product,
) []sdspod.VariantLookup {
	if product == nil || len(product.Variants) == 0 {
		return nil
	}
	result := make([]sdspod.VariantLookup, 0, len(product.Variants))
	for i := range product.Variants {
		variant := &product.Variants[i]
		result = append(result, sdspod.VariantLookup{
			CanonicalVariantIndex: i,
			Keys: []string{
				variant.Attributes["source_sds_sku"].Value,
				sheinpub.SourceSDSSKUFromSupplierSKU(variant.SKU),
				variant.SKU,
				variant.Attributes["Color"].Value,
				variant.Attributes["color"].Value,
			},
		})
	}
	return result
}
