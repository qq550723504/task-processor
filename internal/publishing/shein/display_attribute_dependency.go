package shein

import (
	"strconv"
	"strings"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

const maxRecommendedAttributeCandidates = 12

func dependencyIsActive(attr sheinattribute.AttributeInfo, resolvedByID map[int]ResolvedAttribute) bool {
	return dependencyIsActiveWithInputs(attr, resolvedByID, nil)
}

func dependencyIsActiveWithInputs(attr sheinattribute.AttributeInfo, resolvedByID map[int]ResolvedAttribute, inputs []common.Attribute) bool {
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

func buildDependencyPendingAttributes(attributes []sheinattribute.AttributeInfo, resolved []ResolvedAttribute, inputs ...[]common.Attribute) []common.Attribute {
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
		if !dependencyIsActiveWithInputs(attr, resolvedByID, firstAttributeInputs(inputs)) {
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

func buildPendingAttributeCandidates(attributes []sheinattribute.AttributeInfo, resolved []ResolvedAttribute, inputs ...[]common.Attribute) []PendingAttributeCandidate {
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

	candidates := make([]PendingAttributeCandidate, 0)
	for _, attr := range attributes {
		if !isTemplateRequired(attr) && !isTemplateImportant(attr) {
			continue
		}
		if !dependencyIsActiveWithInputs(attr, resolvedByID, firstAttributeInputs(inputs)) {
			continue
		}
		if _, ok := resolvedByID[attr.AttributeID]; ok {
			continue
		}
		candidates = append(candidates, buildPendingAttributeCandidate(attr))
	}
	return candidates
}

func buildRecommendedAttributeCandidates(attributes []sheinattribute.AttributeInfo, resolved []ResolvedAttribute, inputs ...[]common.Attribute) []PendingAttributeCandidate {
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

	candidates := make([]PendingAttributeCandidate, 0, maxRecommendedAttributeCandidates)
	for _, attr := range attributes {
		if isTemplateRequired(attr) || isTemplateImportant(attr) {
			continue
		}
		if !dependencyIsActiveWithInputs(attr, resolvedByID, firstAttributeInputs(inputs)) {
			continue
		}
		if _, ok := resolvedByID[attr.AttributeID]; ok {
			continue
		}
		candidates = append(candidates, buildPendingAttributeCandidate(attr))
		if len(candidates) >= maxRecommendedAttributeCandidates {
			break
		}
	}
	return candidates
}

func buildPendingAttributeCandidate(attr sheinattribute.AttributeInfo) PendingAttributeCandidate {
	return PendingAttributeCandidate{
		Name:               firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		AttributeID:        attr.AttributeID,
		AttributeName:      attr.AttributeName,
		AttributeNameEn:    attr.AttributeNameEn,
		AttributeType:      attr.AttributeType,
		AttributeMode:      attr.AttributeMode,
		AttributeInputNum:  attr.AttributeInputNum,
		DataDimension:      attr.DataDimension,
		CascadeAttributeID: attr.CascadeAttributeID,
		Required:           isTemplateRequired(attr),
		Important:          isTemplateImportant(attr),
		SKCScope:           attr.SKCScope != nil && *attr.SKCScope,
		AttributeValueList: buildAttributeValueCandidates(attr.AttributeValueInfoList),
	}
}

func buildAttributeValueCandidates(values []sheinattribute.AttributeValue) []AttributeValueCandidate {
	if len(values) == 0 {
		return nil
	}
	result := make([]AttributeValueCandidate, 0, len(values))
	for _, value := range values {
		result = append(result, AttributeValueCandidate{
			AttributeValueID: value.AttributeValueID,
			Value:            value.AttributeValue,
			ValueEn:          value.AttributeValueEn,
		})
	}
	return result
}

func firstAttributeInputs(inputs [][]common.Attribute) []common.Attribute {
	if len(inputs) == 0 {
		return nil
	}
	return inputs[0]
}
