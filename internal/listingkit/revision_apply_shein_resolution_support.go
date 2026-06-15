package listingkit

import (
	"strings"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func applySheinCategoryResolutionPatch(pkg *sheinpub.Package, patch *SheinCategoryResolutionPatch) {
	if pkg == nil || patch == nil {
		return
	}
	if pkg.CategoryResolution == nil {
		pkg.CategoryResolution = &sheinpub.CategoryResolution{}
	}
	if patch.Status != nil {
		pkg.CategoryResolution.Status = strings.TrimSpace(*patch.Status)
	}
	if patch.Source != nil {
		pkg.CategoryResolution.Source = strings.TrimSpace(*patch.Source)
	}
	if patch.QueryText != nil {
		pkg.CategoryResolution.QueryText = strings.TrimSpace(*patch.QueryText)
	}
	if patch.MatchedPath != nil {
		pkg.CategoryResolution.MatchedPath = append([]string(nil), patch.MatchedPath...)
		pkg.CategoryPath = append([]string(nil), patch.MatchedPath...)
		if len(patch.MatchedPath) > 0 {
			pkg.CategoryName = patch.MatchedPath[len(patch.MatchedPath)-1]
		}
	}
	if patch.CategoryID != nil {
		pkg.CategoryResolution.CategoryID = *patch.CategoryID
		pkg.CategoryID = *patch.CategoryID
	}
	if patch.CategoryIDList != nil {
		pkg.CategoryResolution.CategoryIDList = append([]int(nil), patch.CategoryIDList...)
		pkg.CategoryIDList = append([]int(nil), patch.CategoryIDList...)
	}
	if patch.ProductTypeID != nil {
		pkg.CategoryResolution.ProductTypeID = *patch.ProductTypeID
		productTypeID := *patch.ProductTypeID
		pkg.ProductTypeID = &productTypeID
	}
	if patch.TopCategoryID != nil {
		pkg.CategoryResolution.TopCategoryID = *patch.TopCategoryID
		pkg.TopCategoryID = *patch.TopCategoryID
	}
	if patch.ReviewNotes != nil {
		pkg.CategoryResolution.ReviewNotes = uniqueStrings(append([]string(nil), patch.ReviewNotes...))
	}
}

func applySheinAttributeResolutionPatch(pkg *sheinpub.Package, patch *SheinAttributeResolutionPatch) {
	if pkg == nil || patch == nil {
		return
	}
	if pkg.AttributeResolution == nil {
		pkg.AttributeResolution = &sheinpub.AttributeResolution{}
	}
	if patch.Status != nil {
		pkg.AttributeResolution.Status = strings.TrimSpace(*patch.Status)
	}
	if patch.Source != nil {
		pkg.AttributeResolution.Source = strings.TrimSpace(*patch.Source)
	}
	if patch.CategoryID != nil {
		pkg.AttributeResolution.CategoryID = *patch.CategoryID
	}
	if patch.TemplateCount != nil {
		pkg.AttributeResolution.TemplateCount = *patch.TemplateCount
	}
	if patch.ResolvedCount != nil {
		pkg.AttributeResolution.ResolvedCount = *patch.ResolvedCount
	}
	if patch.UnresolvedCount != nil {
		pkg.AttributeResolution.UnresolvedCount = *patch.UnresolvedCount
	}
	if patch.ResolvedAttributes != nil {
		resolved := append([]sheinpub.ResolvedAttribute(nil), patch.ResolvedAttributes...)
		pkg.AttributeResolution.ResolvedAttributes = resolved
		pkg.ResolvedAttributes = append([]sheinpub.ResolvedAttribute(nil), patch.ResolvedAttributes...)
		if pkg.DraftPayload != nil {
			pkg.DraftPayload.ResolvedAttributes = append([]sheinpub.ResolvedAttribute(nil), patch.ResolvedAttributes...)
		}
	}
	if patch.PendingAttributes != nil {
		pkg.AttributeResolution.PendingAttributes = append([]common.Attribute(nil), patch.PendingAttributes...)
	}
	if patch.PendingAttributeCandidates != nil {
		pkg.AttributeResolution.PendingAttributeCandidates = clonePendingAttributeCandidates(patch.PendingAttributeCandidates)
	}
	if patch.RecommendedAttributeCandidates != nil {
		pkg.AttributeResolution.RecommendedAttributeCandidates = clonePendingAttributeCandidates(patch.RecommendedAttributeCandidates)
	}
	if patch.ReviewNotes != nil {
		pkg.AttributeResolution.ReviewNotes = uniqueStrings(append([]string(nil), patch.ReviewNotes...))
	}
	if patch.ResolvedCount == nil && patch.ResolvedAttributes != nil {
		pkg.AttributeResolution.ResolvedCount = len(patch.ResolvedAttributes)
	}
}

func clonePendingAttributeCandidates(items []sheinpub.PendingAttributeCandidate) []sheinpub.PendingAttributeCandidate {
	if len(items) == 0 {
		return nil
	}
	result := make([]sheinpub.PendingAttributeCandidate, 0, len(items))
	for _, item := range items {
		clone := item
		clone.AttributeValueList = append([]sheinpub.AttributeValueCandidate(nil), item.AttributeValueList...)
		result = append(result, clone)
	}
	return result
}

func applySheinSaleAttributeResolutionPatch(pkg *sheinpub.Package, patch *SheinSaleAttributeResolutionPatch) {
	if pkg == nil || patch == nil {
		return
	}
	if pkg.SaleAttributeResolution == nil {
		pkg.SaleAttributeResolution = &sheinpub.SaleAttributeResolution{}
	}
	if patch.Status != nil {
		pkg.SaleAttributeResolution.Status = strings.TrimSpace(*patch.Status)
	}
	if patch.Source != nil {
		pkg.SaleAttributeResolution.Source = strings.TrimSpace(*patch.Source)
	}
	if patch.RecommendCategoryReview != nil {
		pkg.SaleAttributeResolution.RecommendCategoryReview = *patch.RecommendCategoryReview
	}
	if patch.CategoryReviewReason != nil {
		pkg.SaleAttributeResolution.CategoryReviewReason = strings.TrimSpace(*patch.CategoryReviewReason)
	}
	if patch.PrimaryAttributeID != nil {
		pkg.SaleAttributeResolution.PrimaryAttributeID = *patch.PrimaryAttributeID
	}
	if patch.SecondaryAttributeID != nil {
		pkg.SaleAttributeResolution.SecondaryAttributeID = *patch.SecondaryAttributeID
	}
	if patch.PrimarySourceDimension != nil {
		pkg.SaleAttributeResolution.PrimarySourceDimension = strings.TrimSpace(*patch.PrimarySourceDimension)
	}
	if patch.SecondarySourceDimension != nil {
		pkg.SaleAttributeResolution.SecondarySourceDimension = strings.TrimSpace(*patch.SecondarySourceDimension)
	}
	if patch.SKCAttributes != nil {
		pkg.SaleAttributeResolution.SKCAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), patch.SKCAttributes...)
	}
	if patch.SKUAttributes != nil {
		pkg.SaleAttributeResolution.SKUAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), patch.SKUAttributes...)
	}
	if patch.SKCValueAssignments != nil {
		pkg.SaleAttributeResolution.SKCValueAssignments = cloneSheinResolvedSaleAttributeMap(patch.SKCValueAssignments)
	}
	if patch.SKUValueAssignments != nil {
		pkg.SaleAttributeResolution.SKUValueAssignments = cloneSheinResolvedSaleAttributeMap(patch.SKUValueAssignments)
	}
	if patch.CustomAttributeRelation != nil {
		pkg.SaleAttributeResolution.CustomAttributeRelation = append([]sheinattribute.CustomAttributeRelation(nil), patch.CustomAttributeRelation...)
	}
	if patch.SelectionSummary != nil {
		pkg.SaleAttributeResolution.SelectionSummary = append([]string(nil), patch.SelectionSummary...)
	}
	if patch.ReviewNotes != nil {
		pkg.SaleAttributeResolution.ReviewNotes = uniqueStrings(append([]string(nil), patch.ReviewNotes...))
	}
}

func cloneSheinResolvedSaleAttributeMap(src map[string]sheinpub.ResolvedSaleAttribute) map[string]sheinpub.ResolvedSaleAttribute {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]sheinpub.ResolvedSaleAttribute, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}
