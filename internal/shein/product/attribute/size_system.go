package attribute

import (
	"regexp"
	"strings"
)

type sizeSystem string

const (
	sizeSystemUnknown sizeSystem = "unknown"
	sizeSystemUS      sizeSystem = "us"
	sizeSystemMM      sizeSystem = "mm"
	sizeSystemCN      sizeSystem = "cn"
	sizeSystemEU      sizeSystem = "eu"
	sizeSystemBR      sizeSystem = "br"
	sizeSystemAlpha   sizeSystem = "alpha"
)

var (
	sizeSystemUSPattern = regexp.MustCompile(`(?i)\bus\s*\d`)
	sizeSystemCNPattern = regexp.MustCompile(`(?i)\bcn\s*\d`)
	sizeSystemEUPattern = regexp.MustCompile(`(?i)\b(?:eu|eur)\s*\d`)
	sizeSystemBRPattern = regexp.MustCompile(`(?i)\bbr\s*\d`)
	sizeSystemMMPattern = regexp.MustCompile(`^\d+(?:\.\d+)?$`)
)

type sizeSystemBuckets map[sizeSystem][]string

func inferSourceSizeSystem(rawValue string, runtime *MapperRuntimeInput) sizeSystem {
	if system := inferSizeSystemFromValue(rawValue); system != sizeSystemUnknown {
		return system
	}
	if runtime == nil || runtime.AmazonProduct == nil || runtime.AmazonProduct.SizeChart == nil {
		return sizeSystemUnknown
	}

	if row, headers, found := findMatchingSizeChartRow(runtime.AmazonProduct.SizeChart, rawValue); found {
		for idx, header := range headers {
			if idx >= len(row) {
				continue
			}
			headerSystem := inferSizeSystemFromHeader(header)
			if headerSystem == sizeSystemUnknown {
				continue
			}
			if system := inferSizeSystemFromValue(row[idx]); system == headerSystem || system == sizeSystemUnknown {
				return headerSystem
			}
		}
	}

	if row, headers, found := findMatchingApparelSizeChartRow(runtime.AmazonProduct.SizeChart, rawValue); found {
		for idx, header := range headers {
			if idx >= len(row) {
				continue
			}
			headerSystem := inferSizeSystemFromHeader(header)
			if headerSystem == sizeSystemUnknown {
				continue
			}
			if system := inferSizeSystemFromValue(row[idx]); system == headerSystem || system == sizeSystemUnknown {
				return headerSystem
			}
		}
	}

	return sizeSystemUnknown
}

func buildSizeSystemBuckets(platformValues map[string]int) sizeSystemBuckets {
	buckets := make(sizeSystemBuckets)
	seen := make(map[string]struct{}, len(platformValues))
	for value := range platformValues {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		system := inferPlatformSizeSystemFromValue(value)
		if system == sizeSystemUnknown {
			continue
		}
		buckets[system] = append(buckets[system], value)
	}
	return buckets
}

func (b sizeSystemBuckets) isMixed() bool {
	count := 0
	for _, values := range b {
		if len(values) == 0 {
			continue
		}
		count++
		if count > 1 {
			return true
		}
	}
	return false
}

func (b sizeSystemBuckets) values(system sizeSystem) []string {
	if len(b) == 0 {
		return nil
	}
	return append([]string(nil), b[system]...)
}

func narrowPlatformValuesBySizeSystem(platformValues map[string]int, system sizeSystem) map[string]int {
	if len(platformValues) == 0 || system == sizeSystemUnknown {
		return platformValues
	}

	filtered := make(map[string]int)
	for value, id := range platformValues {
		if inferPlatformSizeSystemFromValue(value) == system {
			filtered[value] = id
		}
	}
	if len(filtered) == 0 {
		return platformValues
	}
	return filtered
}

func inferSizeSystemFromValue(value string) sizeSystem {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sizeSystemUnknown
	}
	switch {
	case sizeSystemUSPattern.MatchString(trimmed):
		return sizeSystemUS
	case sizeSystemCNPattern.MatchString(trimmed):
		return sizeSystemCN
	case sizeSystemEUPattern.MatchString(trimmed):
		return sizeSystemEU
	case sizeSystemBRPattern.MatchString(trimmed):
		return sizeSystemBR
	}

	if normalized, ok := normalizeAlphaSizeLabel(trimmed); ok && normalized != "" {
		return sizeSystemAlpha
	}

	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "wide") || strings.Contains(lower, "x-wide") || strings.Contains(lower, "x wide") {
		return sizeSystemUS
	}
	if sizeSystemMMPattern.MatchString(trimmed) {
		// Raw numeric size on the source side is ambiguous; treat it as unknown.
		return sizeSystemUnknown
	}
	return sizeSystemUnknown
}

func inferPlatformSizeSystemFromValue(value string) sizeSystem {
	if system := inferSizeSystemFromValue(value); system != sizeSystemUnknown {
		return system
	}

	trimmed := strings.TrimSpace(value)
	if !sizeSystemMMPattern.MatchString(trimmed) {
		return sizeSystemUnknown
	}
	if normalized, ok := normalizeBaseShoeSize(trimmed); ok && normalized != "" {
		return sizeSystemMM
	}
	return sizeSystemUnknown
}

func inferSizeSystemFromHeader(header string) sizeSystem {
	lower := strings.ToLower(strings.TrimSpace(header))
	switch {
	case strings.Contains(lower, "us"):
		return sizeSystemUS
	case strings.Contains(lower, "cn"):
		return sizeSystemCN
	case strings.Contains(lower, "eu") || strings.Contains(lower, "eur"):
		return sizeSystemEU
	case strings.Contains(lower, "br"):
		return sizeSystemBR
	case strings.Contains(lower, "jp") || strings.Contains(lower, "mm") || strings.Contains(lower, "cm"):
		return sizeSystemMM
	case strings.Contains(lower, "brand size"):
		return sizeSystemUnknown
	default:
		return sizeSystemUnknown
	}
}
