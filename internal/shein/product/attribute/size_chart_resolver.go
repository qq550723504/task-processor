package attribute

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"task-processor/internal/model"
	sheinsize "task-processor/internal/shein/product/size"
)

var (
	widthQualifierPattern = regexp.MustCompile(`(?i)\b(x[\s-]*wide|wide)\b`)
	sizeTokenPattern      = regexp.MustCompile(`\d+(?:\.\d+)?`)
)

type sizeChartSchema string

const (
	sizeChartSchemaUnknown sizeChartSchema = "unknown"
	sizeChartSchemaShoe    sizeChartSchema = "shoe"
)

func (m *AttributeMapper) resolvePlatformValueByDomain(domain platformValueDomain, attrID int, rawValue string, runtime *MapperRuntimeInput, platformValues map[string]int) (int, string) {
	return m.resolvers.Resolve(domain, attrID, rawValue, runtime, platformValues, m.valueMatcher)
}

func resolveShoeSizePlatformID(attrID int, rawValue string, runtime *MapperRuntimeInput, platformValues map[string]int, matcher *AttributeValueMatcher) (int, string) {
	if attrID != 87 || runtime == nil || runtime.AmazonProduct == nil || runtime.AmazonProduct.SizeChart == nil || matcher == nil {
		return 0, ""
	}
	if detectSizeChartSchema(runtime.AmazonProduct.SizeChart, rawValue) != sizeChartSchemaShoe {
		return 0, ""
	}

	baseSize, ok := normalizeBaseShoeSize(rawValue)
	if !ok {
		return 0, ""
	}
	row, headers, found := findMatchingSizeChartRow(runtime.AmazonProduct.SizeChart, baseSize)
	if !found {
		return 0, ""
	}

	candidates := sheinsize.BuildShoeSizeCandidatesFromChart(runtime.AmazonProduct.SizeChart, rawValue)
	legacyCandidates := buildShoeSizeCandidates(baseSize, row, headers, runtime.ProductTitle)
	if len(legacyCandidates) > 0 {
		seen := make(map[string]struct{}, len(candidates)+len(legacyCandidates))
		merged := make([]string, 0, len(candidates)+len(legacyCandidates))
		for _, candidate := range candidates {
			key := strings.ToLower(strings.TrimSpace(candidate))
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, candidate)
		}
		for _, candidate := range legacyCandidates {
			key := strings.ToLower(strings.TrimSpace(candidate))
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, candidate)
		}
		candidates = merged
	}
	for _, candidate := range candidates {
		if platformID := matcher.FindMatchingPlatformValue(candidate, platformValues); platformID > 0 {
			return platformID, candidate
		}
	}

	return 0, ""
}

func normalizeBaseShoeSize(rawValue string) (string, bool) {
	cleaned := widthQualifierPattern.ReplaceAllString(rawValue, "")
	token := sizeTokenPattern.FindString(cleaned)
	if token == "" {
		return "", false
	}

	value, err := strconv.ParseFloat(token, 64)
	if err != nil {
		return "", false
	}
	return formatShoeSizeNumber(value), true
}

func findMatchingSizeChartRow(chart *model.SizeChart, baseSize string) ([]string, []string, bool) {
	row, headers, _, found := findMatchingSizeChartRowWithIndex(chart, baseSize)
	return row, headers, found
}

func findMatchingSizeChartRowWithIndex(chart *model.SizeChart, baseSize string) ([]string, []string, int, bool) {
	if chart == nil || len(chart.Headers) == 0 || len(chart.Rows) == 0 {
		return nil, nil, -1, false
	}

	preferredIndexes := findPreferredSizeChartColumns(chart.Headers)
	for _, row := range chart.Rows {
		for _, idx := range preferredIndexes {
			if idx >= len(row) {
				continue
			}
			cell := strings.TrimSpace(row[idx])
			if cell == "" {
				continue
			}
			if normalized, ok := normalizeBaseShoeSize(cell); ok && normalized == baseSize {
				return row, chart.Headers, idx, true
			}
		}
	}

	return nil, nil, -1, false
}

func findPreferredSizeChartColumns(headers []string) []int {
	type rankedHeader struct {
		index int
		score int
	}

	ranked := make([]rankedHeader, 0, len(headers))
	for idx, header := range headers {
		lower := strings.ToLower(header)
		score := 0
		switch {
		case strings.Contains(lower, "us size"):
			score = 4
		case strings.Contains(lower, "brand size"):
			score = 3
		case strings.Contains(lower, "size"):
			score = 2
		}
		if score > 0 {
			ranked = append(ranked, rankedHeader{index: idx, score: score})
		}
	}

	if len(ranked) == 0 {
		return []int{0}
	}

	for i := 0; i < len(ranked)-1; i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].score > ranked[i].score {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}

	indexes := make([]int, 0, len(ranked))
	for _, item := range ranked {
		indexes = append(indexes, item.index)
	}
	return indexes
}

func buildShoeSizeCandidates(baseSize string, row []string, headers []string, productTitle string) []string {
	seen := make(map[string]struct{})
	var candidates []string

	appendCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if numeric, err := strconv.ParseFloat(value, 64); err == nil {
			value = formatShoeSizeNumber(numeric)
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}

	appendCandidate(baseSize)

	for idx, cell := range row {
		appendCandidate(cell)
		if idx < len(headers) && isMetricSizeHeader(headers[idx]) {
			appendCandidate(cell)
		}
	}

	if inferredMetric, ok := inferShoeMetricSize(baseSize, productTitle); ok {
		appendCandidate(inferredMetric)
	}

	return candidates
}

func isMetricSizeHeader(header string) bool {
	lower := strings.ToLower(header)
	return strings.Contains(lower, "eu") || strings.Contains(lower, "cn") || strings.Contains(lower, "jp")
}

func inferShoeMetricSize(baseSize string, productTitle string) (string, bool) {
	sizeValue, err := strconv.ParseFloat(baseSize, 64)
	if err != nil {
		return "", false
	}

	title := strings.ToLower(productTitle)
	if strings.Contains(title, "women") || strings.Contains(title, "woman") {
		mm := 220 + (sizeValue-5.0)*10
		return fmt.Sprintf("%.0f", math.Round(mm)), true
	}

	return "", false
}

func formatShoeSizeNumber(value float64) string {
	if math.Mod(value, 1) == 0 {
		return fmt.Sprintf("%.0f", value)
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".")
}

func detectSizeChartSchema(chart *model.SizeChart, rawValue string) sizeChartSchema {
	if chart == nil || len(chart.Headers) == 0 {
		return sizeChartSchemaUnknown
	}
	if _, ok := normalizeBaseShoeSize(rawValue); !ok {
		return sizeChartSchemaUnknown
	}

	headerScore := scoreShoeSizeChartHeaders(chart.Headers)
	if headerScore < 4 {
		return sizeChartSchemaUnknown
	}
	if hasNonShoeMeasurementHeaders(chart.Headers) {
		return sizeChartSchemaUnknown
	}

	return sizeChartSchemaShoe
}

func findNormalizedAlphaSizeFromChart(chart *model.SizeChart, rawValue string) (string, bool) {
	if chart == nil || len(chart.Rows) == 0 {
		return "", false
	}
	normalizedRaw, ok := normalizeAlphaSizeLabel(rawValue)
	if !ok {
		return "", false
	}

	for _, row := range chart.Rows {
		for _, cell := range row {
			normalizedCell, ok := normalizeAlphaSizeLabel(cell)
			if ok && normalizedCell == normalizedRaw {
				return normalizedCell, true
			}
		}
	}
	return normalizedRaw, true
}

func scoreShoeSizeChartHeaders(headers []string) int {
	score := 0
	for _, header := range headers {
		lower := strings.ToLower(strings.TrimSpace(header))
		matched := false
		for _, rule := range shoeSizeHeaderScores {
			if strings.Contains(lower, rule.keyword) {
				score += rule.score
				matched = true
				break
			}
		}
		if !matched && lower == "size" {
			score++
		}
	}
	return score
}

func hasNonShoeMeasurementHeaders(headers []string) bool {
	for _, header := range headers {
		lower := strings.ToLower(strings.TrimSpace(header))
		for _, keyword := range apparelMeasurementHeaderKeywords {
			if strings.Contains(lower, keyword) {
				return true
			}
		}
	}
	return false
}
