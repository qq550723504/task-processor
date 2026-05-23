package shein

import (
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func cloneCategoryResolutionWithCacheNote(resolution *CategoryResolution) *CategoryResolution {
	clone := cloneCategoryResolution(resolution)
	if clone != nil {
		if clone.Cache != nil {
			clone.Cache.HitSource = ResolutionCacheHitSourceMemoryCache
			clone.Cache.Status = "hit"
		}
		clone.ReviewNotes = append(clone.ReviewNotes, "SHEIN 类目缓存命中: 已复用同一底版商品的人工/历史类目解析结果")
	}
	return clone
}

func cloneAttributeResolutionWithCacheNote(resolution *AttributeResolution) *AttributeResolution {
	clone := cloneAttributeResolution(resolution)
	if clone != nil {
		if clone.Cache != nil {
			clone.Cache.HitSource = ResolutionCacheHitSourceMemoryCache
			clone.Cache.Status = "hit"
		}
		clone.ReviewNotes = append(clone.ReviewNotes, "SHEIN 普通属性缓存命中: 已复用同一底版商品的人工/历史属性解析结果")
	}
	return clone
}

func cloneSaleAttributeResolutionWithCacheNote(resolution *SaleAttributeResolution) *SaleAttributeResolution {
	clone := cloneSaleAttributeResolution(resolution)
	if clone != nil {
		if clone.Cache != nil {
			clone.Cache.HitSource = ResolutionCacheHitSourceMemoryCache
			clone.Cache.Status = "hit"
		}
		clone.ReviewNotes = append(clone.ReviewNotes, "SHEIN 销售属性缓存命中: 已复用同一底版商品的人工/历史销售属性解析结果")
	}
	return clone
}

func cloneCategoryResolution(resolution *CategoryResolution) *CategoryResolution {
	if resolution == nil {
		return nil
	}
	clone := *resolution
	clone.MatchedPath = append([]string(nil), resolution.MatchedPath...)
	clone.CategoryIDList = append([]int(nil), resolution.CategoryIDList...)
	clone.ReviewNotes = append([]string(nil), resolution.ReviewNotes...)
	clone.Cache = cloneResolutionCacheInfo(resolution.Cache)
	clone.SuggestedCategory = cloneCategorySuggestion(resolution.SuggestedCategory)
	if resolution.SemanticValidation != nil {
		semantic := *resolution.SemanticValidation
		semantic.ComparedPath = append([]string(nil), resolution.SemanticValidation.ComparedPath...)
		clone.SemanticValidation = &semantic
	}
	return &clone
}

func cloneCategorySuggestion(suggestion *CategorySuggestion) *CategorySuggestion {
	if suggestion == nil {
		return nil
	}
	clone := *suggestion
	clone.MatchedPath = append([]string(nil), suggestion.MatchedPath...)
	clone.CategoryIDList = append([]int(nil), suggestion.CategoryIDList...)
	return &clone
}

func cloneAttributeResolution(resolution *AttributeResolution) *AttributeResolution {
	if resolution == nil {
		return nil
	}
	clone := *resolution
	clone.ResolvedAttributes = append([]ResolvedAttribute(nil), resolution.ResolvedAttributes...)
	clone.PendingAttributes = append([]common.Attribute(nil), resolution.PendingAttributes...)
	clone.PendingAttributeCandidates = clonePendingAttributeCandidates(resolution.PendingAttributeCandidates)
	clone.RecommendedAttributeCandidates = clonePendingAttributeCandidates(resolution.RecommendedAttributeCandidates)
	clone.ReviewNotes = append([]string(nil), resolution.ReviewNotes...)
	clone.Cache = cloneResolutionCacheInfo(resolution.Cache)
	return &clone
}

func clonePendingAttributeCandidates(items []PendingAttributeCandidate) []PendingAttributeCandidate {
	if len(items) == 0 {
		return nil
	}
	result := make([]PendingAttributeCandidate, 0, len(items))
	for _, item := range items {
		clone := item
		clone.AttributeValueList = append([]AttributeValueCandidate(nil), item.AttributeValueList...)
		result = append(result, clone)
	}
	return result
}

func cloneSaleAttributeResolution(resolution *SaleAttributeResolution) *SaleAttributeResolution {
	if resolution == nil {
		return nil
	}
	clone := *resolution
	clone.SourceDimensions = cloneSourceVariantDimensions(resolution.SourceDimensions)
	clone.TemplateOptions = cloneSaleAttributeTemplateOptions(resolution.TemplateOptions)
	clone.SKCAttributes = append([]ResolvedSaleAttribute(nil), resolution.SKCAttributes...)
	clone.SKUAttributes = append([]ResolvedSaleAttribute(nil), resolution.SKUAttributes...)
	clone.Candidates = cloneSaleAttributeCandidateInfos(resolution.Candidates)
	clone.SelectionSummary = append([]string(nil), resolution.SelectionSummary...)
	clone.ReviewNotes = append([]string(nil), resolution.ReviewNotes...)
	clone.CustomAttributeRelation = append([]sheinattribute.CustomAttributeRelation(nil), resolution.CustomAttributeRelation...)
	clone.Cache = cloneResolutionCacheInfo(resolution.Cache)
	clone.SKCValueAssignments = cloneResolvedSaleAttributeMap(resolution.SKCValueAssignments)
	clone.SKUValueAssignments = cloneResolvedSaleAttributeMap(resolution.SKUValueAssignments)
	clone.skcAssignments = cloneResolvedSaleAttributeMap(resolution.skcAssignments)
	clone.skuAssignments = cloneResolvedSaleAttributeSliceMap(resolution.skuAssignments)
	clone.skcValueAssignments = cloneResolvedSaleAttributeMap(resolution.skcValueAssignments)
	clone.skuValueAssignments = cloneResolvedSaleAttributeMap(resolution.skuValueAssignments)
	return &clone
}

func cloneResolutionCacheInfo(info *ResolutionCacheInfo) *ResolutionCacheInfo {
	if info == nil {
		return nil
	}
	clone := *info
	if info.UpdatedAt != nil {
		updatedAt := *info.UpdatedAt
		clone.UpdatedAt = &updatedAt
	}
	return &clone
}

func CloneResolutionCacheInfo(info *ResolutionCacheInfo) *ResolutionCacheInfo {
	return cloneResolutionCacheInfo(info)
}

func cloneSourceVariantDimensions(items []SourceVariantDimension) []SourceVariantDimension {
	if len(items) == 0 {
		return nil
	}
	out := make([]SourceVariantDimension, 0, len(items))
	for _, item := range items {
		item.Values = append([]string(nil), item.Values...)
		out = append(out, item)
	}
	return out
}

func cloneSaleAttributeCandidateInfos(items []SaleAttributeCandidateInfo) []SaleAttributeCandidateInfo {
	if len(items) == 0 {
		return nil
	}
	out := make([]SaleAttributeCandidateInfo, 0, len(items))
	for _, item := range items {
		item.Reasons = append([]string(nil), item.Reasons...)
		out = append(out, item)
	}
	return out
}

func cloneSaleAttributeTemplateOptions(items []SaleAttributeTemplateOption) []SaleAttributeTemplateOption {
	if len(items) == 0 {
		return nil
	}
	out := make([]SaleAttributeTemplateOption, 0, len(items))
	for _, item := range items {
		item.AttributeValueList = append([]AttributeValueCandidate(nil), item.AttributeValueList...)
		out = append(out, item)
	}
	return out
}

func cloneResolvedSaleAttributeMap(input map[string]ResolvedSaleAttribute) map[string]ResolvedSaleAttribute {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]ResolvedSaleAttribute, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneResolvedSaleAttributeSliceMap(input map[string][]ResolvedSaleAttribute) map[string][]ResolvedSaleAttribute {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string][]ResolvedSaleAttribute, len(input))
	for key, value := range input {
		out[key] = append([]ResolvedSaleAttribute(nil), value...)
	}
	return out
}
