package listingkit

import (
	"fmt"
	"sort"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func sameResolvedAttributeSet(left []sheinpub.ResolvedAttribute, right []sheinpub.ResolvedAttribute) bool {
	return sameNormalizedStringSet(normalizeResolvedAttributes(left), normalizeResolvedAttributes(right))
}

func sameNormalizedStringSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	sort.Strings(left)
	sort.Strings(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func normalizeResolvedAttributes(items []sheinpub.ResolvedAttribute) []string {
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
			"%d|%d|%s|%s",
			item.AttributeID,
			valueID,
			strings.ToLower(strings.TrimSpace(item.Value)),
			strings.ToLower(strings.TrimSpace(item.AttributeExtraValue)),
		))
	}
	return normalized
}

func buildResolvedAttributeFreshnessDriftMessage(current []sheinpub.ResolvedAttribute, fresh []sheinpub.ResolvedAttribute) string {
	currentOnly, freshOnly := diffResolvedAttributes(current, fresh)
	parts := []string{"当前普通属性模板已变化，现有 resolved attributes 与在线模板结果不一致"}
	if len(currentOnly) > 0 {
		parts = append(parts, "当前任务独有: "+strings.Join(currentOnly, "; "))
	}
	if len(freshOnly) > 0 {
		parts = append(parts, "在线模板独有: "+strings.Join(freshOnly, "; "))
	}
	return strings.Join(parts, "；")
}

func diffResolvedAttributes(current []sheinpub.ResolvedAttribute, fresh []sheinpub.ResolvedAttribute) ([]string, []string) {
	currentCounts := make(map[string]int, len(current))
	for _, item := range current {
		currentCounts[formatResolvedAttributeDiffItem(item)]++
	}
	freshCounts := make(map[string]int, len(fresh))
	for _, item := range fresh {
		freshCounts[formatResolvedAttributeDiffItem(item)]++
	}

	currentOnly := make([]string, 0)
	freshOnly := make([]string, 0)
	for key, count := range currentCounts {
		diff := count - freshCounts[key]
		for i := 0; i < diff; i++ {
			currentOnly = append(currentOnly, key)
		}
	}
	for key, count := range freshCounts {
		diff := count - currentCounts[key]
		for i := 0; i < diff; i++ {
			freshOnly = append(freshOnly, key)
		}
	}
	sort.Strings(currentOnly)
	sort.Strings(freshOnly)
	return currentOnly, freshOnly
}

func buildResolvedAttributeTemplateDriftDetails(
	invalidItems []sheinpub.ResolvedAttribute,
	attributeIndex map[int]sheinattribute.AttributeInfo,
) string {
	if len(invalidItems) == 0 || len(attributeIndex) == 0 {
		return ""
	}

	currentOnly := append([]sheinpub.ResolvedAttribute(nil), invalidItems...)
	freshCandidates := make([]sheinpub.ResolvedAttribute, 0)
	seen := make(map[string]struct{})
	for _, item := range invalidItems {
		attr, ok := attributeIndex[item.AttributeID]
		if !ok {
			continue
		}
		for _, candidate := range buildResolvedAttributeTemplateCandidates(attr) {
			key := formatResolvedAttributeDiffItem(candidate)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			freshCandidates = append(freshCandidates, candidate)
		}
	}
	if len(currentOnly) == 0 && len(freshCandidates) == 0 {
		return ""
	}

	leftOnly, rightOnly := diffResolvedAttributes(currentOnly, freshCandidates)
	parts := make([]string, 0, 2)
	if len(leftOnly) > 0 {
		parts = append(parts, "当前任务独有: "+strings.Join(leftOnly, "; "))
	}
	if len(rightOnly) > 0 {
		parts = append(parts, "在线模板独有: "+strings.Join(rightOnly, "; "))
	}
	return strings.Join(parts, "；")
}

func buildResolvedAttributeTemplateCandidates(attr sheinattribute.AttributeInfo) []sheinpub.ResolvedAttribute {
	if attr.AttributeID <= 0 || len(attr.AttributeValueInfoList) == 0 {
		return nil
	}

	name := strings.TrimSpace(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))
	candidates := make([]sheinpub.ResolvedAttribute, 0, len(attr.AttributeValueInfoList))
	for _, option := range attr.AttributeValueInfoList {
		if option.AttributeValueID <= 0 {
			continue
		}
		valueID := option.AttributeValueID
		candidates = append(candidates, sheinpub.ResolvedAttribute{
			Name:             name,
			Value:            strings.TrimSpace(firstNonEmpty(option.AttributeValueEn, option.AttributeValue)),
			AttributeID:      attr.AttributeID,
			AttributeValueID: &valueID,
		})
	}
	return candidates
}

func formatResolvedAttributeDiffItem(item sheinpub.ResolvedAttribute) string {
	valueID := 0
	if item.AttributeValueID != nil {
		valueID = *item.AttributeValueID
	}
	extraValue := strings.TrimSpace(item.AttributeExtraValue)
	if extraValue == "" {
		return fmt.Sprintf(
			"%s=%s (attribute_id=%d, attribute_value_id=%d)",
			strings.TrimSpace(item.Name),
			strings.TrimSpace(item.Value),
			item.AttributeID,
			valueID,
		)
	}
	return fmt.Sprintf(
		"%s=%s (attribute_id=%d, attribute_value_id=%d, extra=%s)",
		strings.TrimSpace(item.Name),
		strings.TrimSpace(item.Value),
		item.AttributeID,
		valueID,
		extraValue,
	)
}

func sheinResolvedAttributeStillLegal(
	item sheinpub.ResolvedAttribute,
	attributeIndex map[int]sheinattribute.AttributeInfo,
) bool {
	attr, ok := attributeIndex[item.AttributeID]
	if !ok {
		return false
	}
	if item.AttributeValueID != nil && *item.AttributeValueID > 0 {
		for _, option := range attr.AttributeValueInfoList {
			if option.AttributeValueID == *item.AttributeValueID {
				return true
			}
		}
		return false
	}
	if strings.TrimSpace(item.AttributeExtraValue) != "" {
		return true
	}
	return len(attr.AttributeValueInfoList) == 0
}

func sheinpubDependencyIsActive(attr sheinattribute.AttributeInfo, resolvedByID map[int]sheinpub.ResolvedAttribute) bool {
	if attr.CascadeAttributeID <= 0 {
		return true
	}
	parent, ok := resolvedByID[attr.CascadeAttributeID]
	if !ok || parent.AttributeID <= 0 {
		return false
	}
	if sheinConditionalOtherAttribute(attr, parent) {
		return false
	}
	allowed := sheinParseCascadeValueIDs(attr.CascadeAttributeValueIDList)
	if len(allowed) == 0 {
		return true
	}
	if parent.AttributeValueID == nil || *parent.AttributeValueID <= 0 {
		return false
	}
	_, ok = allowed[*parent.AttributeValueID]
	return ok
}

func sheinConditionalOtherAttribute(attr sheinattribute.AttributeInfo, parent sheinpub.ResolvedAttribute) bool {
	name := normalizeSheinText(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))
	if name == "" || !strings.HasPrefix(name, "other ") {
		return false
	}
	if values := sheinParseCascadeValueIDs(attr.CascadeAttributeValueIDList); len(values) > 0 {
		return false
	}
	return parent.AttributeValueID != nil && *parent.AttributeValueID > 0
}

func formatSheinFreshnessAttributeName(attr sheinattribute.AttributeInfo) string {
	return strings.TrimSpace(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))
}

func filterSheinFreshnessDisplayAttributes(attributes []sheinattribute.AttributeInfo) []sheinattribute.AttributeInfo {
	if len(attributes) == 0 {
		return nil
	}
	filtered := make([]sheinattribute.AttributeInfo, 0, len(attributes))
	for _, attr := range attributes {
		if attr.AttributeType == 1 || (attr.SKCScope != nil && *attr.SKCScope) {
			continue
		}
		filtered = append(filtered, attr)
	}
	return filtered
}
