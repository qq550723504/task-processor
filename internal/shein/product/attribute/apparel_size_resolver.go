package attribute

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"task-processor/internal/model"
)

var apparelNumericTokenPattern = regexp.MustCompile(`\d+(?:\.\d+)?`)

func buildApparelNumericSizeCandidates(rawValue string, runtime *MapperRuntimeInput) []string {
	seen := make(map[string]struct{})
	var candidates []string
	appendCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}

	for _, token := range extractNumericTokens(rawValue) {
		appendCandidate(token)
	}

	if runtime != nil && runtime.AmazonProduct != nil && runtime.AmazonProduct.SizeChart != nil {
		if row, headers, found := findMatchingApparelSizeChartRow(runtime.AmazonProduct.SizeChart, rawValue); found {
			for _, candidate := range extractApparelNumericCandidatesFromRow(row, headers) {
				appendCandidate(candidate)
			}
		}
	}

	return candidates
}

func findMatchingApparelSizeChartRow(chart *model.SizeChart, rawValue string) ([]string, []string, bool) {
	if chart == nil || len(chart.Rows) == 0 {
		return nil, nil, false
	}

	normalizedRawAlpha, rawIsAlpha := normalizeAlphaSizeLabel(rawValue)
	rawLower := strings.ToLower(strings.TrimSpace(rawValue))

	for _, row := range chart.Rows {
		for _, cell := range row {
			cellLower := strings.ToLower(strings.TrimSpace(cell))
			if cellLower == "" {
				continue
			}
			if cellLower == rawLower {
				return row, chart.Headers, true
			}
			if rawIsAlpha {
				if normalizedCell, ok := normalizeAlphaSizeLabel(cell); ok && normalizedCell == normalizedRawAlpha {
					return row, chart.Headers, true
				}
			}
		}
	}

	return nil, nil, false
}

func extractApparelNumericCandidatesFromRow(row []string, headers []string) []string {
	type rankedTokens struct {
		score  int
		tokens []string
	}

	var ranked []rankedTokens
	for idx, cell := range row {
		if len(extractNumericTokens(cell)) == 0 {
			continue
		}

		header := ""
		if idx < len(headers) {
			header = headers[idx]
		}
		score := scoreApparelNumericHeader(header)
		if score <= 0 {
			continue
		}
		ranked = append(ranked, rankedTokens{
			score:  score,
			tokens: extractNumericTokens(cell),
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	seen := make(map[string]struct{})
	var candidates []string
	for _, item := range ranked {
		for _, token := range item.tokens {
			if _, exists := seen[token]; exists {
				continue
			}
			seen[token] = struct{}{}
			candidates = append(candidates, token)
		}
	}
	return candidates
}

func scoreApparelNumericHeader(header string) int {
	lower := strings.ToLower(strings.TrimSpace(header))
	switch {
	case strings.Contains(lower, "us size"):
		return 4
	case strings.Contains(lower, "size") && !strings.Contains(lower, "bust") && !strings.Contains(lower, "waist") && !strings.Contains(lower, "hip"):
		return 3
	case strings.Contains(lower, "brand size"):
		return 2
	default:
		return 0
	}
}

func extractNumericTokens(value string) []string {
	matches := apparelNumericTokenPattern.FindAllString(value, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	tokens := make([]string, 0, len(matches))
	for _, match := range matches {
		numeric, err := strconv.ParseFloat(match, 64)
		if err != nil {
			continue
		}
		normalized := formatNumericToken(numeric)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		tokens = append(tokens, normalized)
	}
	return tokens
}

func formatNumericToken(value float64) string {
	if value == float64(int64(value)) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 1, 64), "0"), ".")
}
