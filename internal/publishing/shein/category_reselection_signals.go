package shein

import (
	"strings"

	"task-processor/internal/catalog/canonical"
)

func productSignalTokens(canonical *canonical.Product, current *Package) []string {
	var chunks []string
	if canonical != nil {
		chunks = append(chunks, canonical.Title, canonical.Description, canonical.Brand)
		chunks = append(chunks, canonical.CategoryPath...)
		for key, attr := range canonical.Attributes {
			chunks = append(chunks, key, attr.Value)
		}
		for _, dim := range canonical.VariantDimensions {
			chunks = append(chunks, dim.Name)
			chunks = append(chunks, dim.Values...)
		}
	}
	if current != nil {
		chunks = append(chunks, current.SpuName)
		for key, value := range current.Attributes {
			chunks = append(chunks, key, value)
		}
	}
	return normalizeTextTokens(chunks...)
}

func normalizeTextTokens(parts ...string) []string {
	var tokens []string
	seen := map[string]struct{}{}
	replacer := strings.NewReplacer("&", " ", "/", " ", "-", " ", "_", " ", ">", " ", ",", " ", "，", " ", ":", " ", "：", " ", "(", " ", ")", " ")
	for _, part := range parts {
		part = normalizeCategoryToken(part)
		if part == "" {
			continue
		}
		part = replacer.Replace(part)
		for _, token := range strings.Fields(part) {
			token = strings.TrimSpace(token)
			if len(token) < 2 {
				continue
			}
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			tokens = append(tokens, token)
		}
	}
	return tokens
}
