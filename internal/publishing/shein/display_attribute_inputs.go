package shein

import (
	"strings"

	common "task-processor/internal/publishing/common"
)

func buildDerivedAttributeInputs(pkg *Package) []common.Attribute {
	return buildDerivedAttributeInputsFromEvidence(buildDisplayAttributeEvidencePool(pkg))
}

func findAttributeValue(items []common.Attribute, candidates ...string) string {
	for _, candidate := range candidates {
		for _, item := range items {
			if matchesAttributeName(item.Name, candidate) && strings.TrimSpace(item.Value) != "" {
				return strings.TrimSpace(item.Value)
			}
		}
	}
	return ""
}

func findAttributeValueInMap(items map[string]string, candidates ...string) string {
	for _, candidate := range candidates {
		for key, value := range items {
			if matchesAttributeName(key, candidate) && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func matchesAttributeName(actual string, candidate string) bool {
	return normalizeText(actual) == normalizeText(candidate)
}

func dedupeAttributeInputs(items []common.Attribute) []common.Attribute {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]common.Attribute, 0, len(items))
	for _, item := range items {
		key := normalizeText(item.Name) + "\x00" + strings.TrimSpace(item.Value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}
