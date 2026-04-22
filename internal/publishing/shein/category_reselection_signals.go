package shein

import (
	"strings"

	"task-processor/internal/productenrich"
)

var productFamilyKeywordGroups = map[string][]string{
	"outdoor_furniture": {"camping", "outdoor", "furniture", "chair", "table", "tent", "folding", "露营", "户外", "家具", "桌", "椅", "折叠"},
	"drinkware":         {"cup", "mug", "bottle", "tumbler", "drinkware", "water bottle", "kitchen", "杯", "水杯", "保温杯", "瓶", "水壶", "厨具"},
	"apparel":           {"clothing", "apparel", "dress", "costume", "lolita", "服装", "服饰", "裙", "上衣", "裤", "洛丽塔"},
	"electronics":       {"phone", "electronics", "mobile", "smartphone", "ipad", "iphone", "手机", "电子", "数码"},
	"footwear":          {"shoe", "shoes", "sneaker", "boot", "sandals", "鞋", "运动鞋", "靴", "凉鞋"},
}

func productSignalTokens(canonical *productenrich.CanonicalProduct, current *Package) []string {
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

func familyLabelsForTokens(tokens []string) []string {
	var labels []string
	for label, keywords := range productFamilyKeywordGroups {
		if intersectsAny(tokens, keywords) {
			labels = append(labels, label)
		}
	}
	return labels
}

func productFamilyLabels(canonical *productenrich.CanonicalProduct, current *Package) []string {
	labels := make([]string, 0, 4)
	seen := map[string]struct{}{}
	merge := func(items []string) {
		for _, item := range items {
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			labels = append(labels, item)
		}
	}
	if canonical != nil && len(canonical.CategoryPath) > 0 {
		merge(familyLabelsForTokens(normalizeTextTokens(canonical.CategoryPath...)))
	}
	merge(familyLabelsForTokens(productSignalTokens(canonical, current)))
	return labels
}

func sharedFamilyLabelCount(a, b []string) int {
	set := map[string]struct{}{}
	for _, item := range a {
		set[item] = struct{}{}
	}
	count := 0
	for _, item := range b {
		if _, ok := set[item]; ok {
			count++
		}
	}
	return count
}
