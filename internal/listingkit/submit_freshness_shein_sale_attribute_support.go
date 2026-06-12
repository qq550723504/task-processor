package listingkit

import (
	"fmt"
	"sort"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func sameResolvedSaleAttributeSet(left []sheinpub.ResolvedSaleAttribute, right []sheinpub.ResolvedSaleAttribute) bool {
	return sameNormalizedStringSet(normalizeResolvedSaleAttributes(left), normalizeResolvedSaleAttributes(right))
}

func buildSheinFreshnessSaleTemplateOptions(info *sheinattribute.AttributeTemplateInfo) []sheinpub.SaleAttributeTemplateOption {
	if info == nil || len(info.Data) == 0 {
		return nil
	}
	template := info.Data[0]
	attributes := orderSheinFreshnessSaleScopeAttributes(filterSheinFreshnessSaleScopeAttributes(template.AttributeInfos), template.AttributeID)
	options := make([]sheinpub.SaleAttributeTemplateOption, 0, len(attributes))
	for _, attribute := range attributes {
		option := sheinpub.SaleAttributeTemplateOption{
			AttributeID: attribute.AttributeID,
			Name:        attribute.AttributeName,
			NameEn:      attribute.AttributeNameEn,
			Required:    attribute.AttributeIsShow == 1,
			Important:   attribute.AttributeLabel == 1,
		}
		if attribute.SKCScope != nil {
			option.SKCScope = *attribute.SKCScope
		}
		for _, value := range attribute.AttributeValueInfoList {
			if value.AttributeValueID <= 0 {
				continue
			}
			option.AttributeValueList = append(option.AttributeValueList, sheinpub.AttributeValueCandidate{
				AttributeValueID: value.AttributeValueID,
				Value:            value.AttributeValue,
				ValueEn:          value.AttributeValueEn,
			})
		}
		options = append(options, option)
	}
	return options
}

func filterSheinFreshnessSaleScopeAttributes(attributes []sheinattribute.AttributeInfo) []sheinattribute.AttributeInfo {
	result := make([]sheinattribute.AttributeInfo, 0, len(attributes))
	for _, attr := range attributes {
		if attr.AttributeType == 1 || (attr.SKCScope != nil && *attr.SKCScope) {
			result = append(result, attr)
			continue
		}
		switch normalizeSheinText(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)) {
		case "color", "colour", "size", "style", "pattern", "capacity", "type", "model", "set", "颜色", "颜色分类", "尺码", "尺寸", "规格", "容量", "款式", "类型", "型号", "套装":
			result = append(result, attr)
		}
	}
	return result
}

func orderSheinFreshnessSaleScopeAttributes(attributes []sheinattribute.AttributeInfo, orderedIDs []int) []sheinattribute.AttributeInfo {
	if len(attributes) == 0 {
		return nil
	}
	if len(orderedIDs) == 0 {
		ordered := append([]sheinattribute.AttributeInfo(nil), attributes...)
		sort.SliceStable(ordered, func(i, j int) bool {
			left := ordered[i].AttributeLabel == 1
			right := ordered[j].AttributeLabel == 1
			if left != right {
				return left
			}
			return false
		})
		return ordered
	}
	byID := make(map[int]sheinattribute.AttributeInfo, len(attributes))
	for _, attr := range attributes {
		byID[attr.AttributeID] = attr
	}
	ordered := make([]sheinattribute.AttributeInfo, 0, len(attributes))
	seen := make(map[int]struct{}, len(attributes))
	for _, id := range orderedIDs {
		attr, ok := byID[id]
		if !ok {
			continue
		}
		ordered = append(ordered, attr)
		seen[id] = struct{}{}
	}
	for _, attr := range attributes {
		if _, ok := seen[attr.AttributeID]; ok {
			continue
		}
		ordered = append(ordered, attr)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		left := ordered[i].AttributeLabel == 1
		right := ordered[j].AttributeLabel == 1
		if left != right {
			return left
		}
		return false
	})
	return ordered
}

func collectInvalidSaleAttributes(
	items []sheinpub.ResolvedSaleAttribute,
	options map[int]sheinpub.SaleAttributeTemplateOption,
	customRelationIDs map[int]struct{},
) []string {
	invalid := make([]string, 0)
	for _, item := range items {
		option, ok := options[item.AttributeID]
		if !ok {
			invalid = append(invalid, formatResolvedSaleAttributeDiffItem(item))
			continue
		}
		if item.AttributeValueID != nil && *item.AttributeValueID > 0 {
			if _, ok := customRelationIDs[*item.AttributeValueID]; ok {
				continue
			}
			found := false
			for _, candidate := range option.AttributeValueList {
				if candidate.AttributeValueID == *item.AttributeValueID {
					found = true
					break
				}
			}
			if !found {
				invalid = append(invalid, formatResolvedSaleAttributeDiffItem(item))
			}
		}
	}
	return invalid
}

func sheinFreshnessCustomAttributeValueIDs(relations []sheinattribute.CustomAttributeRelation) map[int]struct{} {
	if len(relations) == 0 {
		return nil
	}
	result := make(map[int]struct{}, len(relations))
	for _, relation := range relations {
		if relation.AttributeValueID <= 0 {
			continue
		}
		result[int(relation.AttributeValueID)] = struct{}{}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func formatResolvedSaleAttributeDiffItem(item sheinpub.ResolvedSaleAttribute) string {
	valueID := 0
	if item.AttributeValueID != nil {
		valueID = *item.AttributeValueID
	}
	return fmt.Sprintf(
		"%s=%s (scope=%s, attribute_id=%d, attribute_value_id=%d)",
		strings.TrimSpace(item.Name),
		strings.TrimSpace(item.Value),
		strings.TrimSpace(item.Scope),
		item.AttributeID,
		valueID,
	)
}

func normalizeResolvedSaleAttributes(items []sheinpub.ResolvedSaleAttribute) []string {
	if len(items) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		valueID := 0
		if item.AttributeValueID != nil {
			valueID = *item.AttributeValueID
		}
		normalized = append(normalized, fmt.Sprintf(
			"%s|%d|%d|%s",
			strings.ToLower(strings.TrimSpace(item.Scope)),
			item.AttributeID,
			valueID,
			strings.ToLower(strings.TrimSpace(item.Value)),
		))
	}
	return normalized
}
