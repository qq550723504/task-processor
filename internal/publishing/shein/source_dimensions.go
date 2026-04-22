package shein

import (
	"sort"
	"strings"

	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
)

type SourceVariantDimension struct {
	Name          string   `json:"name,omitempty"`
	Values        []string `json:"values,omitempty"`
	DistinctCount int      `json:"distinct_count,omitempty"`
	SampleValue   string   `json:"sample_value,omitempty"`
}

func buildSourceVariantDimensions(canonical *productenrich.CanonicalProduct, variants []common.Variant) []SourceVariantDimension {
	if len(variants) == 0 {
		return nil
	}

	if len(canonical.VariantDimensions) > 0 {
		dimensions := make([]SourceVariantDimension, 0, len(canonical.VariantDimensions))
		for _, dimension := range canonical.VariantDimensions {
			name := strings.TrimSpace(dimension.Name)
			if name == "" {
				continue
			}
			values := uniqueDimensionValues(name, variants, dimension.Values)
			if len(values) == 0 {
				continue
			}
			dimensions = append(dimensions, SourceVariantDimension{
				Name:          name,
				Values:        values,
				DistinctCount: len(values),
				SampleValue:   values[0],
			})
		}
		if len(dimensions) > 0 {
			return dimensions
		}
	}

	keys := collectVariantAttributeKeys(variants)
	dimensions := make([]SourceVariantDimension, 0, len(keys))
	for _, key := range keys {
		values := uniqueDimensionValues(key, variants, nil)
		if len(values) == 0 {
			continue
		}
		dimensions = append(dimensions, SourceVariantDimension{
			Name:          key,
			Values:        values,
			DistinctCount: len(values),
			SampleValue:   values[0],
		})
	}
	return dimensions
}

func collectVariantAttributeKeys(variants []common.Variant) []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0)
	for _, variant := range variants {
		for key := range variant.Attributes {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func uniqueDimensionValues(name string, variants []common.Variant, preferred []string) []string {
	seen := make(map[string]struct{})
	values := make([]string, 0)
	appendValue := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := normalizeText(value)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		values = append(values, value)
	}

	for _, value := range preferred {
		appendValue(value)
	}
	for _, variant := range variants {
		appendValue(lookupAttributeValue(variant.Attributes, name))
	}
	return values
}
