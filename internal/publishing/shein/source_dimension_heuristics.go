package shein

import (
	"strconv"
	"strings"
	"unicode"
)

func sourceDimensionPrimaryPriority(dimension SourceVariantDimension) int {
	score := 0
	if dimension.DistinctCount > 1 {
		score += 4
	}
	if isDescriptiveSourceDimension(dimension) {
		score += 6
	}
	if isNumericScaleSourceDimension(dimension) {
		score -= 2
	}
	return score
}

func sourceDimensionSecondaryPriority(dimension SourceVariantDimension) int {
	score := 0
	if dimension.DistinctCount > 1 {
		score += 4
	}
	if isNumericScaleSourceDimension(dimension) {
		score += 6
	}
	return score
}

func isDescriptiveSourceDimension(dimension SourceVariantDimension) bool {
	name := normalizeText(dimension.Name)
	switch name {
	case "颜色", "颜色分类", "color", "colour", "style", "款式", "pattern", "图案", "material", "材质":
		return true
	}
	return !isNumericScaleSourceDimension(dimension)
}

func isNumericScaleSourceDimension(dimension SourceVariantDimension) bool {
	name := normalizeText(dimension.Name)
	switch name {
	case "size", "尺码", "尺寸", "dimension", "capacity", "容量", "规格":
		return true
	}

	if len(dimension.Values) == 0 {
		return false
	}

	numericLikeCount := 0
	for _, value := range dimension.Values {
		if isNumericLikeDimensionValue(value) {
			numericLikeCount++
		}
	}
	return numericLikeCount > 0 && numericLikeCount == len(dimension.Values)
}

func isNumericLikeDimensionValue(value string) bool {
	value = normalizeSaleAttributeValue(value)
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}

	compact := strings.NewReplacer("eu", "", "us", "", "uk", "", "eur", "", "cm", "", "mm", "", "ml", "", "l", "", "g", "", "kg", "", "码", "", "号", "").Replace(strings.ToLower(value))
	compact = strings.Join(strings.Fields(compact), "")
	if compact == "" {
		return false
	}
	if _, err := strconv.ParseFloat(compact, 64); err == nil {
		return true
	}

	hasDigit := false
	for _, r := range compact {
		if unicode.IsDigit(r) {
			hasDigit = true
			continue
		}
		if r != '.' && r != '-' && r != '/' && r != '_' {
			return false
		}
	}
	return hasDigit
}
