package attribute

import (
	"strings"

	"task-processor/internal/core/logger"
	sheinsize "task-processor/internal/shein/product/size"
)

type structuredSizeMatch struct {
	platformID int
	value      string
	score      int
}

func resolvePlatformValueByStructuredSize(rawValue string, runtime *MapperRuntimeInput, platformValues map[string]int, matcher *AttributeValueMatcher) (int, string) {
	if matcher == nil || len(platformValues) == 0 {
		return 0, ""
	}

	source, ok := resolveStructuredSourceShoeSize(rawValue, runtime)
	if !ok {
		logger.GetGlobalLogger("shein/product").Infof(
			"structured size resolver skipped: value=%q reason=source_not_parsed",
			rawValue,
		)
		return 0, ""
	}
	logger.GetGlobalLogger("shein/product").Infof(
		"structured size resolver parsed source: value=%q system=%s base=%s width=%s",
		rawValue,
		source.System,
		source.BaseSize,
		source.Width,
	)

	matches := collectStructuredSizeMatches(source, platformValues, matcher)
	if len(matches) == 0 {
		logger.GetGlobalLogger("shein/product").Infof(
			"structured size resolver found no candidate: value=%q system=%s base=%s width=%s candidate_count=%d",
			rawValue,
			source.System,
			source.BaseSize,
			source.Width,
			countUniquePlatformValues(platformValues),
		)
		return 0, ""
	}

	best := matches[0]
	if len(matches) > 1 && matches[1].score == best.score {
		logger.GetGlobalLogger("shein/product").Warnf(
			"structured size resolver ambiguous: value=%q top_candidates=%v score=%d",
			rawValue,
			sampleStructuredSizeMatches(matches, 3),
			best.score,
		)
		return 0, ""
	}

	logger.GetGlobalLogger("shein/product").Infof(
		"resolved platform value via structured size resolver: value=%q resolved=%q platformID=%d system=%s width=%s score=%d",
		rawValue,
		best.value,
		best.platformID,
		source.System,
		source.Width,
		best.score,
	)
	return best.platformID, best.value
}

func resolveStructuredSourceShoeSize(rawValue string, runtime *MapperRuntimeInput) (sheinsize.ShoeSize, bool) {
	if runtime != nil && runtime.AmazonProduct != nil && runtime.AmazonProduct.SizeChart != nil {
		if resolved, ok := sheinsize.ResolveShoeSizeFromChart(runtime.AmazonProduct.SizeChart, rawValue); ok {
			return resolved, true
		}
	}
	parsed := sheinsize.ParseShoeSize(rawValue)
	if !parsed.IsShoeSize {
		return sheinsize.ShoeSize{}, false
	}
	return parsed, true
}

func collectStructuredSizeMatches(source sheinsize.ShoeSize, platformValues map[string]int, matcher *AttributeValueMatcher) []structuredSizeMatch {
	seen := make(map[string]struct{}, len(platformValues))
	matches := make([]structuredSizeMatch, 0, len(platformValues))

	for platformValue := range platformValues {
		normalized := strings.ToLower(strings.TrimSpace(platformValue))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}

		score, ok := scoreStructuredShoeSizeCandidate(source, platformValue)
		if !ok {
			continue
		}

		platformID := matcher.FindMatchingPlatformValue(platformValue, platformValues)
		if platformID <= 0 {
			continue
		}
		matches = append(matches, structuredSizeMatch{
			platformID: platformID,
			value:      platformValue,
			score:      score,
		})
	}

	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].score > matches[i].score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	return matches
}

func scoreStructuredShoeSizeCandidate(source sheinsize.ShoeSize, candidateValue string) (int, bool) {
	candidate := sheinsize.ParseShoeSize(candidateValue)
	if !candidate.IsShoeSize {
		return 0, false
	}
	if source.BaseSize == "" || candidate.BaseSize == "" || source.BaseSize != candidate.BaseSize {
		return 0, false
	}

	if source.System != sheinsize.SystemUnknown &&
		candidate.System != sheinsize.SystemUnknown &&
		source.System != candidate.System {
		return 0, false
	}

	if source.Width != sheinsize.WidthUnknown && source.Width != sheinsize.WidthRegular {
		if candidate.Width == sheinsize.WidthUnknown || candidate.Width == sheinsize.WidthRegular {
			return 0, false
		}
	}

	score := 100
	if source.System != sheinsize.SystemUnknown {
		if candidate.System == source.System {
			score += 20
		} else if candidate.System == sheinsize.SystemUnknown {
			score += 5
		}
	}

	switch {
	case source.Width == candidate.Width:
		score += 20
	case sheinsize.AreShoeSizesFuzzyCompatible(source.Raw, candidateValue):
		score += 10
	case source.Width == sheinsize.WidthUnknown || source.Width == sheinsize.WidthRegular:
		if candidate.Width == sheinsize.WidthUnknown || candidate.Width == sheinsize.WidthRegular {
			score += 5
		}
	default:
		return 0, false
	}

	return score, true
}

func sampleStructuredSizeMatches(matches []structuredSizeMatch, limit int) []string {
	if len(matches) == 0 || limit <= 0 {
		return nil
	}

	if len(matches) < limit {
		limit = len(matches)
	}

	values := make([]string, 0, limit)
	for _, match := range matches[:limit] {
		values = append(values, match.value)
	}
	return values
}
