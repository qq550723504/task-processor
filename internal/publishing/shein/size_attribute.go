package shein

import (
	"context"
	"strings"

	sheinpublishing "task-processor/internal/marketplace/shein/publishing"
)

const defaultSizeSaleAttributeID = 87

type SizeAttributeHeaderResolver interface {
	ResolveSizeAttributeHeaders(input SizeAttributeHeaderResolutionInput) SizeAttributeHeaderResolution
}

type SizeAttributeHeaderResolutionInput struct {
	Context            context.Context
	Headers            []string
	TemplateAttributes []sheinpublishing.SizeChartTemplateAttribute
}

type SizeAttributeHeaderResolution struct {
	AttributeIDsByHeader map[string]int
	ReviewNotes          []string
}

func applyProductSizeAttributes(pkg *Package, productSize string) bool {
	return applyProductSizeAttributesWithResolver(pkg, productSize, nil, context.Background())
}

func applyProductSizeAttributesWithResolver(pkg *Package, productSize string, resolver SizeAttributeHeaderResolver, ctx context.Context) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return false
	}
	sizeRefs := collectSizeSaleAttributeRefs(pkg)
	if len(sizeRefs) == 0 {
		pkg.DraftPayload.SizeAttributeList = nil
		return false
	}
	templateAttrs := collectSizeChartTemplateAttributes(pkg)
	headerResolution := resolveSDSSizeHeaderAttributeIDs(ctx, productSize, templateAttrs, resolver)
	attrs := sheinpublishing.BuildSizeAttributesFromProductSizeWithHeaderAttributeIDs(productSize, sizeRefs, templateAttrs, headerResolution.AttributeIDsByHeader)
	pkg.DraftPayload.SizeAttributeList = attrs
	if len(attrs) > 0 && len(headerResolution.ReviewNotes) > 0 {
		pkg.ReviewNotes = dedupeStrings(append(pkg.ReviewNotes, headerResolution.ReviewNotes...))
	}
	return len(attrs) > 0
}

func resolveSDSSizeHeaderAttributeIDs(ctx context.Context, productSize string, templateAttrs []sheinpublishing.SizeChartTemplateAttribute, resolver SizeAttributeHeaderResolver) SizeAttributeHeaderResolution {
	if resolver == nil || len(templateAttrs) == 0 {
		return SizeAttributeHeaderResolution{}
	}
	headers := sheinpublishing.UnmappedSDSSizeHeaders(productSize, templateAttrs)
	if len(headers) == 0 {
		return SizeAttributeHeaderResolution{}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	resolution := resolver.ResolveSizeAttributeHeaders(SizeAttributeHeaderResolutionInput{
		Context:            ctx,
		Headers:            headers,
		TemplateAttributes: templateAttrs,
	})
	return resolution
}

func collectSizeSaleAttributeRefs(pkg *Package) []sheinpublishing.SizeSaleAttributeRef {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return nil
	}
	sizeAttributeID := defaultSizeSaleAttributeID
	sizeSourceDimension := "size"
	if pkg.SaleAttributeResolution != nil {
		if pkg.SaleAttributeResolution.SecondaryAttributeID > 0 && sheinpublishing.IsSizeSourceDimension(pkg.SaleAttributeResolution.SecondarySourceDimension) {
			sizeAttributeID = pkg.SaleAttributeResolution.SecondaryAttributeID
		}
		if strings.TrimSpace(pkg.SaleAttributeResolution.SecondarySourceDimension) != "" {
			sizeSourceDimension = pkg.SaleAttributeResolution.SecondarySourceDimension
		}
	}
	var result []sheinpublishing.SizeSaleAttributeRef
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			for _, attr := range sku.SaleAttributes {
				if attr.AttributeID != sizeAttributeID || attr.AttributeValueID == nil || *attr.AttributeValueID <= 0 {
					continue
				}
				sizeValue := firstNonEmpty(attr.Value, lookupAttributeValue(sku.Attributes, sizeSourceDimension), lookupAttributeValue(sku.Attributes, "Size"), lookupAttributeValue(sku.Attributes, "尺码"))
				if strings.TrimSpace(sizeValue) == "" {
					continue
				}
				result = append(result, sheinpublishing.SizeSaleAttributeRef{
					SizeValue:        sizeValue,
					AttributeID:      attr.AttributeID,
					AttributeValueID: *attr.AttributeValueID,
				})
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func collectSizeChartTemplateAttributes(pkg *Package) []sheinpublishing.SizeChartTemplateAttribute {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.AttributeResolution == nil {
		return nil
	}
	if len(pkg.AttributeResolution.SizeChartAttributes) == 0 {
		return []sheinpublishing.SizeChartTemplateAttribute{}
	}
	result := make([]sheinpublishing.SizeChartTemplateAttribute, 0, len(pkg.AttributeResolution.SizeChartAttributes))
	for _, attr := range pkg.AttributeResolution.SizeChartAttributes {
		if attr.AttributeID <= 0 {
			continue
		}
		result = append(result, sheinpublishing.SizeChartTemplateAttribute{
			AttributeID:     attr.AttributeID,
			AttributeName:   attr.AttributeName,
			AttributeNameEn: attr.AttributeNameEn,
			SourceSystemIDs: append([]int(nil), attr.SourceSystemIDList...),
			SortOrder:       attr.SortOrder,
		})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
