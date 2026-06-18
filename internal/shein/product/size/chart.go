package size

import (
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/model"
)

func ResolveShoeSizeFromChart(chart *model.SizeChart, raw string) (ShoeSize, bool) {
	parsed := ParseShoeSize(raw)
	if !parsed.IsShoeSize || chart == nil || len(chart.Headers) == 0 || len(chart.Rows) == 0 {
		return ShoeSize{}, false
	}

	matchedRow, matchedCol, ok := findMatchingChartRow(chart, parsed.BaseSize)
	if !ok {
		return ShoeSize{}, false
	}

	resolved := parsed
	if matchedCol < len(chart.Headers) {
		if system := inferSystemFromHeader(chart.Headers[matchedCol]); system != SystemUnknown {
			resolved.System = system
		}
	}
	if matchedCol < len(matchedRow) {
		if base := extractBaseSize(matchedRow[matchedCol]); base != "" {
			resolved.BaseSize = base
		}
	}
	return resolved, true
}

func BuildShoeSizeCandidatesFromChart(chart *model.SizeChart, raw string) []string {
	parsed, ok := ResolveShoeSizeFromChart(chart, raw)
	if !ok {
		return nil
	}

	row, _, ok := findMatchingChartRow(chart, parsed.BaseSize)
	if !ok {
		return nil
	}

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

	appendCandidate(raw)
	if parsed.Width != WidthRegular && parsed.Width != WidthUnknown {
		appendCandidate(formatShoeCandidate(parsed.System, parsed.BaseSize, parsed.Width, false))
		appendCandidate(formatShoeCandidate(parsed.System, parsed.BaseSize, parsed.Width, true))
	}
	appendCandidate(formatBaseSizeBySystem(parsed.System, parsed.BaseSize))
	appendCandidate(parsed.BaseSize)

	for idx, cell := range row {
		if idx >= len(chart.Headers) {
			continue
		}
		base := extractBaseSize(cell)
		if base == "" {
			continue
		}
		system := inferSystemFromHeader(chart.Headers[idx])
		if parsed.Width != WidthRegular && parsed.Width != WidthUnknown {
			appendCandidate(formatShoeCandidate(system, base, parsed.Width, false))
			appendCandidate(formatShoeCandidate(system, base, parsed.Width, true))
		}
		appendCandidate(formatBaseSizeBySystem(system, base))
		appendCandidate(base)
	}

	return candidates
}

func findMatchingChartRow(chart *model.SizeChart, baseSize string) ([]string, int, bool) {
	if chart == nil || len(chart.Headers) == 0 || len(chart.Rows) == 0 {
		return nil, -1, false
	}
	for _, idx := range preferredChartColumnIndexes(chart.Headers) {
		for _, row := range chart.Rows {
			if idx >= len(row) {
				continue
			}
			if base := extractBaseSize(row[idx]); base != "" && base == baseSize {
				return row, idx, true
			}
		}
	}
	return nil, -1, false
}

func preferredChartColumnIndexes(headers []string) []int {
	type ranked struct {
		index int
		score int
	}
	rankedHeaders := make([]ranked, 0, len(headers))
	for idx, header := range headers {
		score := 0
		switch inferSystemFromHeader(header) {
		case SystemUS:
			score = 5
		case SystemUnknown:
			if strings.Contains(strings.ToLower(strings.TrimSpace(header)), "brand size") {
				score = 4
			} else if strings.Contains(strings.ToLower(strings.TrimSpace(header)), "size") {
				score = 3
			}
		default:
			score = 2
		}
		if score > 0 {
			rankedHeaders = append(rankedHeaders, ranked{index: idx, score: score})
		}
	}
	for i := 0; i < len(rankedHeaders)-1; i++ {
		for j := i + 1; j < len(rankedHeaders); j++ {
			if rankedHeaders[j].score > rankedHeaders[i].score {
				rankedHeaders[i], rankedHeaders[j] = rankedHeaders[j], rankedHeaders[i]
			}
		}
	}
	indexes := make([]int, 0, len(rankedHeaders))
	for _, item := range rankedHeaders {
		indexes = append(indexes, item.index)
	}
	if len(indexes) == 0 && len(headers) > 0 {
		return []int{0}
	}
	return indexes
}

func inferSystemFromHeader(header string) System {
	lower := strings.ToLower(strings.TrimSpace(header))
	switch {
	case strings.Contains(lower, "us"):
		return SystemUS
	case strings.Contains(lower, "uk"):
		return SystemUK
	case strings.Contains(lower, "eu") || strings.Contains(lower, "eur"):
		return SystemEU
	case strings.Contains(lower, "br"):
		return SystemBR
	case strings.Contains(lower, "cn"):
		return SystemCN
	case strings.Contains(lower, "jp") || strings.Contains(lower, "mm") || strings.Contains(lower, "cm"):
		return SystemMM
	default:
		return SystemUnknown
	}
}

func formatBaseSizeBySystem(system System, base string) string {
	switch system {
	case SystemUS:
		return "US" + base
	case SystemUK:
		return "UK" + base
	case SystemEU:
		return "EU" + base
	case SystemBR:
		return "BR" + base
	case SystemCN:
		return "CN" + base
	default:
		return base
	}
}

func formatShoeCandidate(system System, base string, width Width, compact bool) string {
	prefix := ""
	switch system {
	case SystemUS:
		prefix = "US"
	case SystemUK:
		prefix = "UK"
	case SystemEU:
		prefix = "EU"
	case SystemBR:
		prefix = "BR"
	case SystemCN:
		prefix = "CN"
	}
	switch width {
	case WidthWide:
		if compact {
			return prefix + base + "W"
		}
		return fmt.Sprintf("%s%s Wide", prefix, base)
	case WidthXWide:
		if compact {
			return prefix + base + "WW"
		}
		return fmt.Sprintf("%s%s X-Wide", prefix, base)
	case WidthNarrow:
		return fmt.Sprintf("%s%s Narrow", prefix, base)
	default:
		return formatBaseSizeBySystem(system, base)
	}
}

func normalizeNumericString(value string) string {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return strings.TrimSpace(value)
	}
	if parsed == float64(int64(parsed)) {
		return strconv.FormatInt(int64(parsed), 10)
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(parsed, 'f', 1, 64), "0"), ".")
}
