package shein

import (
	"strconv"
	"strings"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func dependencyIsActive(attr sheinattribute.AttributeInfo, resolvedByID map[int]ResolvedAttribute) bool {
	if attr.CascadeAttributeID <= 0 {
		return true
	}
	parent, ok := resolvedByID[attr.CascadeAttributeID]
	if !ok || parent.AttributeID <= 0 {
		return false
	}
	if isConditionalOtherAttribute(attr, parent) {
		return false
	}
	allowed := parseCascadeValueIDs(attr.CascadeAttributeValueIDList)
	if len(allowed) == 0 {
		return true
	}
	if parent.AttributeValueID == nil || *parent.AttributeValueID <= 0 {
		return false
	}
	_, ok = allowed[*parent.AttributeValueID]
	return ok
}

func isConditionalOtherAttribute(attr sheinattribute.AttributeInfo, parent ResolvedAttribute) bool {
	name := normalizeText(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))
	if name == "" {
		return false
	}
	if !strings.HasPrefix(name, "other ") {
		return false
	}
	if values := parseCascadeValueIDs(attr.CascadeAttributeValueIDList); len(values) > 0 {
		return false
	}
	return parent.AttributeValueID != nil && *parent.AttributeValueID > 0
}

func parseCascadeValueIDs(raw *string) map[int]struct{} {
	if raw == nil {
		return nil
	}
	text := strings.TrimSpace(*raw)
	if text == "" {
		return nil
	}
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == ' ' || r == '\n' || r == '\t'
	})
	if len(fields) == 0 {
		return nil
	}
	result := make(map[int]struct{}, len(fields))
	for _, field := range fields {
		id, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || id <= 0 {
			continue
		}
		result[id] = struct{}{}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func buildDependencyPendingAttributes(
	attributes []sheinattribute.AttributeInfo,
	resolved []ResolvedAttribute,
) []common.Attribute {
	if len(attributes) == 0 {
		return nil
	}
	resolvedByID := make(map[int]ResolvedAttribute, len(resolved))
	for _, item := range resolved {
		if item.AttributeID <= 0 {
			continue
		}
		resolvedByID[item.AttributeID] = item
	}

	pending := make([]common.Attribute, 0)
	for _, attr := range attributes {
		if !isTemplateRequired(attr) {
			continue
		}
		if !dependencyIsActive(attr, resolvedByID) {
			continue
		}
		if _, ok := resolvedByID[attr.AttributeID]; ok {
			continue
		}
		pending = append(pending, common.Attribute{
			Name:  firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
			Value: "",
		})
	}
	return pending
}
