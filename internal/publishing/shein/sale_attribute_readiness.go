package shein

import (
	"fmt"
	"strings"

	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
)

// SecondarySaleAttributeRequired reports whether SKU-level sale attributes are required for submit.
func SecondarySaleAttributeRequired(pkg *Package) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return false
	}
	if !HasMultiSKUWithinSingleSKC(pkg) {
		return false
	}
	if !HasSecondarySourceVariation(pkg) {
		return false
	}
	return HasSecondaryTemplateCandidate(pkg.SaleAttributeResolution)
}

// HasMultiSKUWithinSingleSKC reports whether any SKC has multiple SKUs in draft, preview, or package state.
func HasMultiSKUWithinSingleSKC(pkg *Package) bool {
	if pkg == nil {
		return false
	}
	if pkg.DraftPayload != nil {
		for _, skc := range pkg.DraftPayload.SKCList {
			if len(skc.SKUList) > 1 {
				return true
			}
		}
	}
	if pkg.PreviewPayload != nil {
		for _, skc := range pkg.PreviewPayload.SKCList {
			if len(skc.SKUS) > 1 {
				return true
			}
		}
	}
	for _, skc := range pkg.SkcList {
		if len(skc.SKUs) > 1 {
			return true
		}
	}
	return false
}

// HasSecondarySourceVariation reports whether source data contains a secondary sale-attribute dimension.
func HasSecondarySourceVariation(pkg *Package) bool {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return false
	}
	resolution := pkg.SaleAttributeResolution
	if resolution.SecondaryAttributeID > 0 || len(resolution.SKUAttributes) > 0 || len(resolution.SKUValueAssignments) > 0 {
		return true
	}
	sourceName := strings.TrimSpace(resolution.SecondarySourceDimension)
	if sourceName == "" {
		return false
	}
	for _, dimension := range resolution.SourceDimensions {
		if SaleDimensionMatches(sourceName, dimension.Name) && dimension.DistinctCount > 1 {
			return true
		}
	}
	for _, skc := range pkg.SkcList {
		values := map[string]struct{}{}
		for _, sku := range skc.SKUs {
			value := ""
			for key, attrValue := range sku.Attributes {
				if SaleDimensionMatches(sourceName, key) {
					value = strings.TrimSpace(attrValue)
					break
				}
			}
			if value == "" {
				continue
			}
			values[value] = struct{}{}
			if len(values) > 1 {
				return true
			}
		}
	}
	return false
}

// HasSecondaryTemplateCandidate reports whether SHEIN templates expose a candidate for the secondary dimension.
func HasSecondaryTemplateCandidate(resolution *SaleAttributeResolution) bool {
	if resolution == nil {
		return false
	}
	sourceName := strings.TrimSpace(resolution.SecondarySourceDimension)
	if sourceName == "" {
		return false
	}
	for _, candidate := range resolution.Candidates {
		if candidate.AttributeID <= 0 || candidate.AttributeID == resolution.PrimaryAttributeID || candidate.SKCScope {
			continue
		}
		if SaleDimensionMatches(sourceName, candidate.SourceDimension) || SaleDimensionMatches(sourceName, candidate.Name) {
			return true
		}
	}
	for _, option := range resolution.TemplateOptions {
		if option.AttributeID <= 0 || option.AttributeID == resolution.PrimaryAttributeID || option.SKCScope {
			continue
		}
		if SaleDimensionMatches(sourceName, option.Name) || SaleDimensionMatches(sourceName, option.NameEn) {
			return true
		}
	}
	return false
}

// SaleDimensionMatches reports whether two source/template sale dimensions are equivalent.
func SaleDimensionMatches(left, right string) bool {
	return sheinmarketpub.SaleDimensionMatches(left, right)
}

// NormalizeSaleDimension normalizes common source dimension labels.
func NormalizeSaleDimension(value string) string {
	return sheinmarketpub.NormalizeSaleDimension(value)
}

// HasBlockingPendingAttributes reports whether required or manually pending attributes still block submit.
func HasBlockingPendingAttributes(pkg *Package) bool {
	if pkg == nil || pkg.AttributeResolution == nil {
		return true
	}
	for _, candidate := range pkg.AttributeResolution.PendingAttributeCandidates {
		if candidate.Required {
			return true
		}
	}
	for _, attr := range pkg.AttributeResolution.PendingAttributes {
		if strings.TrimSpace(attr.Name) != "" || strings.TrimSpace(attr.Value) != "" {
			return true
		}
	}
	return false
}

// SaleAttributesReadyForSubmit reports whether sale attributes are complete enough for submit.
func SaleAttributesReadyForSubmit(pkg *Package) bool {
	return len(SaleAttributesReadinessFailureReasons(pkg)) == 0
}

// SaleAttributesReadinessFailureReasons returns sale-attribute submit blockers.
func SaleAttributesReadinessFailureReasons(pkg *Package) []string {
	pkg = NormalizePackageSemanticFields(pkg)
	var reasons []string
	if !SaleAttributeStatusResolved(pkg) {
		reasons = append(reasons, "sale attribute status is not resolved or primary attribute id is missing")
	}
	if SaleAttributeReviewPending(pkg) {
		reasons = append(reasons, "sale attribute category review is still pending")
	}
	if pkg == nil || pkg.DraftPayload == nil || len(pkg.DraftPayload.SKCList) == 0 {
		reasons = append(reasons, "draft payload skc_list is empty")
		return uniqueNonEmptySubmitStrings(reasons)
	}
	for _, skc := range pkg.DraftPayload.SKCList {
		if !ResolvedSaleAttributeReady(skc.SaleAttribute) {
			reasons = append(reasons, fmt.Sprintf("skc %q is missing a resolved sale attribute value id", skc.SupplierCode))
		}
		requireSKUAttributes := len(skc.SKUList) > 1 && SecondarySaleAttributeRequired(pkg)
		if !requireSKUAttributes && pkg.SaleAttributeResolution != nil {
			requireSKUAttributes = pkg.SaleAttributeResolution.SecondaryAttributeID > 0 || len(pkg.SaleAttributeResolution.SKUAttributes) > 0
		}
		if requireSKUAttributes {
			if len(skc.SKUList) == 0 {
				reasons = append(reasons, fmt.Sprintf("skc %q is missing sku_list while sku sale attributes are required", skc.SupplierCode))
				continue
			}
			for _, sku := range skc.SKUList {
				if len(sku.SaleAttributes) == 0 {
					reasons = append(reasons, fmt.Sprintf("sku %q is missing sale_attributes", sku.SupplierSKU))
					continue
				}
				for _, attr := range sku.SaleAttributes {
					if !ResolvedSaleAttributeValueReady(attr) {
						reasons = append(reasons, fmt.Sprintf(
							"sku %q has unresolved sale attribute %q (attribute_id=%d, attribute_value_id=%v)",
							sku.SupplierSKU,
							attr.Name,
							attr.AttributeID,
							attr.AttributeValueID,
						))
					}
				}
			}
		}
	}
	return uniqueNonEmptySubmitStrings(reasons)
}

// SaleAttributeReviewPending reports whether category review still blocks sale-attribute submit.
func SaleAttributeReviewPending(pkg *Package) bool {
	return pkg != nil && pkg.SaleAttributeResolution != nil && pkg.SaleAttributeResolution.RecommendCategoryReview
}

// SaleAttributeStatusResolved reports whether sale-attribute resolution has a resolved primary attribute.
func SaleAttributeStatusResolved(pkg *Package) bool {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(pkg.SaleAttributeResolution.Status), "resolved") &&
		pkg.SaleAttributeResolution.PrimaryAttributeID > 0
}

// ResolvedSaleAttributeReady reports whether a resolved sale attribute pointer has usable IDs.
func ResolvedSaleAttributeReady(attr *ResolvedSaleAttribute) bool {
	return attr != nil && ResolvedSaleAttributeValueReady(*attr)
}

// ResolvedSaleAttributeValueReady reports whether a resolved sale attribute value has usable IDs.
func ResolvedSaleAttributeValueReady(attr ResolvedSaleAttribute) bool {
	return sheinmarketpub.ResolvedSaleAttributeValueReady(attr.AttributeID, attr.AttributeValueID)
}
