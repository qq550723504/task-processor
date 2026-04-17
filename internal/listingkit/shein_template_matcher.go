package listingkit

import (
	"strings"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

type sheinTemplateIndex struct {
	attributes []sheinattribute.AttributeInfo
}

func newSheinTemplateIndex(attributes []sheinattribute.AttributeInfo) *sheinTemplateIndex {
	return &sheinTemplateIndex{attributes: append([]sheinattribute.AttributeInfo(nil), attributes...)}
}

func (i *sheinTemplateIndex) Match(name, value string) SheinResolvedAttribute {
	nameNorm := normalizeSheinText(name)
	valueNorm := normalizeSheinText(value)
	for _, attr := range i.attributes {
		if !matchesSheinAttributeName(attr, nameNorm) {
			continue
		}
		resolved := SheinResolvedAttribute{
			Name:        name,
			Value:       value,
			AttributeID: attr.AttributeID,
			MatchedBy:   "attribute_name",
			Required:    isSheinTemplateRequired(attr),
			SKCScope:    attr.SKCScope != nil && *attr.SKCScope,
		}
		if valueID := matchSheinAttributeValue(attr, valueNorm); valueID != nil {
			resolved.AttributeValueID = valueID
			resolved.MatchedBy = "attribute_name+value"
		} else {
			resolved.AttributeExtraValue = value
		}
		return resolved
	}
	return SheinResolvedAttribute{}
}

func matchesSheinAttributeName(attr sheinattribute.AttributeInfo, normalized string) bool {
	if normalized == "" {
		return false
	}
	candidates := []string{
		attr.AttributeName,
		attr.AttributeNameEn,
	}
	for _, alias := range sheinAttributeAliases(normalized) {
		candidates = append(candidates, alias)
	}
	for _, candidate := range candidates {
		candidateNorm := normalizeSheinText(candidate)
		if candidateNorm == "" {
			continue
		}
		if candidateNorm == normalized {
			return true
		}
	}
	return false
}

func matchSheinAttributeValue(attr sheinattribute.AttributeInfo, normalized string) *int {
	if normalized == "" {
		return nil
	}
	for _, candidate := range attr.AttributeValueInfoList {
		for _, raw := range []string{candidate.AttributeValue, candidate.AttributeValueEn} {
			if normalizeSheinText(raw) == normalized {
				valueID := candidate.AttributeValueID
				return &valueID
			}
		}
	}
	return nil
}

func normalizeSheinText(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	replacer := strings.NewReplacer("_", "", "-", "", " ", "", "/", "", "&", "and")
	return replacer.Replace(raw)
}

func sheinAttributeAliases(normalized string) []string {
	switch normalized {
	case "colour":
		return []string{"color"}
	case "color":
		return []string{"colour"}
	case "material":
		return []string{"fabric"}
	case "fabric":
		return []string{"material"}
	default:
		return nil
	}
}

func isSheinTemplateRequired(attribute sheinattribute.AttributeInfo) bool {
	switch {
	case len(attribute.AttributeRemarkList) > 0:
		return true
	case attribute.AttributeLabel == 1:
		return true
	case attribute.AttributeStatus == 3:
		return true
	default:
		return false
	}
}
