package publishing

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var (
	sourceDimensionLeadingScalePattern = regexp.MustCompile(`(?i)\b(eur|eu|us|uk)\s*([0-9])`)
	sourceDimensionNoisePattern        = regexp.MustCompile(`(?i)\b(eur|eu|us|uk|size)\b`)
)

// SourceDimension describes a source variant dimension used for fallback sale-attribute grouping.
type SourceDimension struct {
	Name          string
	Values        []string
	DistinctCount int
}

// SourceDimensionSelection is the source-dimension fallback selection result.
type SourceDimensionSelection struct {
	PrimarySourceDimension   string
	SecondarySourceDimension string
	Reasons                  []string
}

// SelectSourceDimensionsFallback chooses source dimensions when the live SHEIN template is unavailable.
func SelectSourceDimensionsFallback(dimensions []SourceDimension) *SourceDimensionSelection {
	if len(dimensions) == 0 {
		return nil
	}
	ranked := append([]SourceDimension(nil), dimensions...)
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if SourceDimensionPrimaryPriority(a) != SourceDimensionPrimaryPriority(b) {
			return SourceDimensionPrimaryPriority(a) > SourceDimensionPrimaryPriority(b)
		}
		if a.DistinctCount != b.DistinctCount {
			return a.DistinctCount > b.DistinctCount
		}
		return normalizeSourceDimensionText(a.Name) < normalizeSourceDimensionText(b.Name)
	})
	selection := &SourceDimensionSelection{
		PrimarySourceDimension: ranked[0].Name,
		Reasons:                []string{"SHEIN 模板未就绪，先按源销售属性维度生成最小分组计划"},
	}
	secondaryPool := append([]SourceDimension(nil), ranked[1:]...)
	sort.SliceStable(secondaryPool, func(i, j int) bool {
		a, b := secondaryPool[i], secondaryPool[j]
		if SourceDimensionSecondaryPriority(a) != SourceDimensionSecondaryPriority(b) {
			return SourceDimensionSecondaryPriority(a) > SourceDimensionSecondaryPriority(b)
		}
		if a.DistinctCount != b.DistinctCount {
			return a.DistinctCount > b.DistinctCount
		}
		return normalizeSourceDimensionText(a.Name) < normalizeSourceDimensionText(b.Name)
	})
	for _, dimension := range secondaryPool {
		if dimension.Name == selection.PrimarySourceDimension {
			continue
		}
		selection.SecondarySourceDimension = dimension.Name
		break
	}
	return selection
}

// SourceDimensionExists reports whether a source dimension name exists after normalization.
func SourceDimensionExists(dimensions []SourceDimension, name string) bool {
	name = normalizeSourceDimensionText(name)
	for _, dimension := range dimensions {
		if normalizeSourceDimensionText(dimension.Name) == name {
			return true
		}
	}
	return false
}

// SourceDimensionPrimaryPriority scores a source dimension for primary grouping.
func SourceDimensionPrimaryPriority(dimension SourceDimension) int {
	score := 0
	if dimension.DistinctCount > 1 {
		score += 4
	}
	if IsDescriptiveSourceDimension(dimension) {
		score += 6
	}
	if IsNumericScaleSourceDimension(dimension) {
		score -= 2
	}
	return score
}

// SourceDimensionSecondaryPriority scores a source dimension for secondary grouping.
func SourceDimensionSecondaryPriority(dimension SourceDimension) int {
	score := 0
	if dimension.DistinctCount > 1 {
		score += 4
	}
	if IsNumericScaleSourceDimension(dimension) {
		score += 6
	}
	return score
}

// IsDescriptiveSourceDimension reports whether a source dimension is better suited for primary grouping.
func IsDescriptiveSourceDimension(dimension SourceDimension) bool {
	name := normalizeSourceDimensionText(dimension.Name)
	switch name {
	case "颜色", "颜色分类", "color", "colour", "style", "款式", "pattern", "图案", "material", "材质":
		return true
	}
	return !IsNumericScaleSourceDimension(dimension)
}

// IsNumericScaleSourceDimension reports whether a source dimension looks like size/capacity/scale data.
func IsNumericScaleSourceDimension(dimension SourceDimension) bool {
	name := normalizeSourceDimensionText(dimension.Name)
	switch name {
	case "size", "尺码", "尺寸", "dimension", "capacity", "容量", "规格":
		return true
	}

	if len(dimension.Values) == 0 {
		return false
	}

	numericLikeCount := 0
	for _, value := range dimension.Values {
		if IsNumericLikeSourceDimensionValue(value) {
			numericLikeCount++
		}
	}
	return numericLikeCount > 0 && numericLikeCount == len(dimension.Values)
}

// IsNumericLikeSourceDimensionValue reports whether a source dimension value is number/scale-like.
func IsNumericLikeSourceDimensionValue(value string) bool {
	value = normalizeSourceDimensionValue(value)
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

func normalizeSourceDimensionText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("_", " ", "-", " ", "/", " ")
	return strings.Join(strings.Fields(replacer.Replace(value)), " ")
}

func normalizeSourceDimensionValue(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	value = strings.NewReplacer(
		"，", ",",
		"（", "(",
		"）", ")",
		"_", " ",
		"-", " ",
		"/", " ",
	).Replace(value)
	value = sourceDimensionLeadingScalePattern.ReplaceAllString(value, `$2`)
	value = sourceDimensionNoisePattern.ReplaceAllString(value, " ")
	value = trimSourceDimensionCodePrefix(value)
	return strings.Join(strings.Fields(value), " ")
}

func trimSourceDimensionCodePrefix(value string) string {
	for i, r := range value {
		if r > 127 {
			prefix := strings.TrimSpace(value[:i])
			if prefix == "" {
				return value
			}
			if isLikelySourceDimensionCodePrefix(prefix) {
				return value[i:]
			}
			return value
		}
	}
	return value
}

func isLikelySourceDimensionCodePrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	hasLetterOrDigit := false
	for _, r := range prefix {
		switch {
		case r >= 'a' && r <= 'z':
			hasLetterOrDigit = true
		case r >= '0' && r <= '9':
			hasLetterOrDigit = true
		case strings.ContainsRune(" -_./", r):
		default:
			return false
		}
	}
	return hasLetterOrDigit
}
