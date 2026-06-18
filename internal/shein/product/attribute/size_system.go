package attribute

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	sheinsize "task-processor/internal/shein/product/size"
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

	if row, headers, matchedIndex, found := findMatchingSizeChartRowWithIndex(runtime.AmazonProduct.SizeChart, rawValue); found {
		if matchedIndex >= 0 && matchedIndex < len(headers) && matchedIndex < len(row) {
			headerSystem := inferSizeSystemFromHeader(headers[matchedIndex])
			if headerSystem != sizeSystemUnknown {
				if system := inferSizeSystemFromValue(row[matchedIndex]); system == headerSystem || system == sizeSystemUnknown {
					return headerSystem
				}
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

func narrowPlatformValuesByPreferredSizeSystems(platformValues map[string]int, systems []sizeSystem) (map[string]int, sizeSystem) {
	if len(platformValues) == 0 || len(systems) == 0 {
		return platformValues, sizeSystemUnknown
	}

	for _, system := range systems {
		if system == sizeSystemUnknown {
			continue
		}
		filtered := narrowPlatformValuesBySizeSystem(platformValues, system)
		if len(filtered) == 0 || len(filtered) == len(platformValues) {
			continue
		}
		return filtered, system
	}

	return platformValues, sizeSystemUnknown
}

func preferredTargetSizeSystems(runtime *MapperRuntimeInput) []sizeSystem {
	if runtime == nil {
		return nil
	}

	for _, site := range runtime.SiteList {
		for _, subSite := range site.SubSiteList {
			if systems := preferredSizeSystemsBySite(subSite); len(systems) > 0 {
				return systems
			}
		}
	}

	return preferredSizeSystemsByRegion(runtime.Region)
}

func preferredSizeSystemsBySite(site string) []sizeSystem {
	normalized := strings.ToLower(strings.TrimSpace(site))
	if normalized == "" {
		return nil
	}
	if strings.HasPrefix(normalized, "shein-") {
		normalized = strings.TrimPrefix(normalized, "shein-")
	}
	return preferredSizeSystemsByRegion(normalized)
}

func preferredSizeSystemsByRegion(region string) []sizeSystem {
	switch strings.ToUpper(strings.TrimSpace(region)) {
	case "US", "CA", "MX":
		return []sizeSystem{sizeSystemUS, sizeSystemMM, sizeSystemEU}
	case "UK":
		return []sizeSystem{sizeSystemEU, sizeSystemUS, sizeSystemMM}
	case "FR", "DE", "IT", "ES":
		return []sizeSystem{sizeSystemEU, sizeSystemMM, sizeSystemUS}
	case "JP":
		return []sizeSystem{sizeSystemMM, sizeSystemUS, sizeSystemEU}
	case "BR":
		return []sizeSystem{sizeSystemBR, sizeSystemUS, sizeSystemMM}
	case "CN":
		return []sizeSystem{sizeSystemCN, sizeSystemMM, sizeSystemEU}
	case "AU":
		return []sizeSystem{sizeSystemUS, sizeSystemEU, sizeSystemMM}
	case "SA", "AE":
		return []sizeSystem{sizeSystemEU, sizeSystemUS, sizeSystemMM}
	default:
		return nil
	}
}

func formatSizeSystemsForLog(systems []sizeSystem) string {
	if len(systems) == 0 {
		return ""
	}
	parts := make([]string, 0, len(systems))
	for _, system := range systems {
		if system == sizeSystemUnknown {
			continue
		}
		parts = append(parts, string(system))
	}
	if len(parts) == 0 {
		return ""
	}
	slices.Sort(parts)
	return fmt.Sprintf("[%s]", strings.Join(parts, ","))
}

func inferSizeSystemFromValue(value string) sizeSystem {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sizeSystemUnknown
	}
	if parsed := sheinsize.ParseShoeSize(trimmed); parsed.IsShoeSize {
		switch parsed.System {
		case sheinsize.SystemUS:
			return sizeSystemUS
		case sheinsize.SystemCN:
			return sizeSystemCN
		case sheinsize.SystemEU:
			return sizeSystemEU
		case sheinsize.SystemBR:
			return sizeSystemBR
		case sheinsize.SystemMM:
			return sizeSystemMM
		}
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
