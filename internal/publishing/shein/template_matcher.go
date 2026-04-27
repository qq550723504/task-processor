package shein

import (
	"strings"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

type templateIndex struct {
	attributes []sheinattribute.AttributeInfo
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func newTemplateIndex(attributes []sheinattribute.AttributeInfo) *templateIndex {
	return &templateIndex{attributes: append([]sheinattribute.AttributeInfo(nil), attributes...)}
}

func (i *templateIndex) FindAttribute(name string) *sheinattribute.AttributeInfo {
	name = normalizeText(name)
	for _, attr := range i.attributes {
		if !matchesAnyName(name, collectAttributeNames(attr)) {
			continue
		}
		attrCopy := attr
		return &attrCopy
	}
	return nil
}

func (i *templateIndex) Match(name, value string) ResolvedAttribute {
	name = normalizeText(name)
	value = strings.TrimSpace(value)
	attr := i.FindAttribute(name)
	if attr == nil {
		return ResolvedAttribute{Name: strings.TrimSpace(name), Value: value}
	}
	match := ResolvedAttribute{
		Name:                firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		Value:               value,
		AttributeID:         attr.AttributeID,
		AttributeExtraValue: value,
		MatchedBy:           "attribute_name",
		Required:            isTemplateRequired(*attr),
		Important:           isTemplateImportant(*attr),
		SKCScope:            attr.SKCScope != nil && *attr.SKCScope,
	}
	for _, option := range attr.AttributeValueInfoList {
		if normalizeText(firstNonEmpty(option.AttributeValueEn, option.AttributeValue)) == normalizeText(value) {
			valueID := option.AttributeValueID
			match.AttributeValueID = &valueID
			match.AttributeExtraValue = ""
			match.MatchedBy = "attribute_value"
			break
		}
	}
	return match
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("_", " ", "-", " ", "/", " ")
	return strings.Join(strings.Fields(replacer.Replace(value)), " ")
}

func matchesAnyName(name string, candidates []string) bool {
	name = normalizeText(name)
	for _, candidate := range candidates {
		if normalizeText(candidate) == name {
			return true
		}
	}
	return false
}

func collectAttributeNames(attr sheinattribute.AttributeInfo) []string {
	return []string{attr.AttributeName, attr.AttributeNameEn}
}

func isTemplateRequired(attr sheinattribute.AttributeInfo) bool {
	return len(attr.AttributeRemarkList) > 0 || attr.AttributeStatus == 3
}

func isTemplateImportant(attr sheinattribute.AttributeInfo) bool {
	return attr.AttributeLabel > 0
}
