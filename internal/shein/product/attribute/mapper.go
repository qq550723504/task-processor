package attribute

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/types"
	"task-processor/internal/shein/aicache"
	"task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/content"
	sheinctx "task-processor/internal/shein/context"
	sheinsize "task-processor/internal/shein/product/size"
)

type AttributeMapper struct {
	valueMatcher *AttributeValueMatcher
	processor    customAttributeValueProcessor
	resolvers    *platformValueResolverRegistry
}

type customAttributeValueProcessor interface {
	ProcessCustomAttributeValueWithRuntime(ctx *sheinctx.TaskContext, runtime *MapperRuntimeInput, attrID int, attrValue string, isRequired bool) CustomAttributeResult
}

func NewAttributeMapper() *AttributeMapper {
	return &AttributeMapper{
		valueMatcher: NewAttributeValueMatcher(),
		processor:    NewCustomAttributeProcessor(),
		resolvers:    newPlatformValueResolverRegistry(),
	}
}

func (m *AttributeMapper) MapAttributeValuesToSheinIDs(ctx *sheinctx.TaskContext, strategy *AttributeStrategy) ([]attribute.CustomAttributeRelation, error) {
	return m.MapAttributeValuesToSheinIDsWithRuntime(ctx, newMapperRuntimeInput(ctx), strategy)
}

func (m *AttributeMapper) MapAttributeValuesToSheinIDsWithRuntime(ctx *sheinctx.TaskContext, runtime *MapperRuntimeInput, strategy *AttributeStrategy) ([]attribute.CustomAttributeRelation, error) {
	if err := runtime.Validate(); err != nil {
		return nil, err
	}

	logger.GetGlobalLogger("shein/product").Info("start attribute value ID mapping")

	var allRelations []attribute.CustomAttributeRelation

	relations, err := m.mapSingleAttributeValues(ctx, runtime, &strategy.PrimaryAttribute, true)
	if err != nil {
		return nil, fmt.Errorf("failed to map primary attribute values: %w", err)
	}
	allRelations = append(allRelations, relations...)

	if strategy.SecondaryAttribute.AttrID > 0 && len(strategy.SecondaryAttribute.AttrValue) > 0 {
		relations, err := m.mapSingleAttributeValues(ctx, runtime, &strategy.SecondaryAttribute, false)
		if err != nil {
			return nil, fmt.Errorf("failed to map secondary attribute values: %w", err)
		}
		allRelations = append(allRelations, relations...)
	}

	return allRelations, nil
}

func (m *AttributeMapper) mapSingleAttributeValues(ctx *sheinctx.TaskContext, runtime *MapperRuntimeInput, attr *ResultAttribute, isRequired bool) ([]attribute.CustomAttributeRelation, error) {
	if attr.AttrID <= 0 || len(attr.AttrValue) == 0 {
		return nil, nil
	}
	if m.valueMatcher == nil {
		m.valueMatcher = NewAttributeValueMatcher()
	}
	if m.resolvers == nil {
		m.resolvers = newPlatformValueResolverRegistry()
	}

	var relations []attribute.CustomAttributeRelation
	attrInfo := findTemplateAttributeInfo(attr.AttrID, runtime.AttributeTemplates)
	platformValues := m.valueMatcher.GetPlatformAttributeValues(attr.AttrID, runtime.AttributeTemplates)
	valueDomain := detectPlatformValueDomain(attrInfo)
	logger.GetGlobalLogger("shein/product").Infof(
		"mapping attribute values: attrID=%d domain=%s input_value_count=%d platform_candidate_count=%d platform_candidate_sample=%v",
		attr.AttrID,
		valueDomain,
		len(attr.AttrValue),
		countUniquePlatformValues(platformValues),
		samplePlatformValues(platformValues, 8),
	)

	for i := 0; i < len(attr.AttrValue); i++ {
		attrValue := &attr.AttrValue[i]
		if attrValue.ID.Int() > 0 && !shouldRemapSizeLikeAttributeValue(attrInfo) {
			continue
		}

		if platformID, resolvedValue := m.resolvePlatformValueByDomain(valueDomain, attr.AttrID, attrValue.Value, runtime, platformValues); platformID > 0 {
			logger.GetGlobalLogger("shein/product").Infof(
				"resolved platform value via domain resolver: attrID=%d domain=%s original=%q resolved=%q platformID=%d",
				attr.AttrID,
				valueDomain,
				attrValue.Value,
				resolvedValue,
				platformID,
			)
			attr.AttrValue[i].ID = types.FlexibleID(platformID)
			continue
		}

		if platformID := m.valueMatcher.FindMatchingPlatformValue(attrValue.Value, platformValues); platformID > 0 {
			attr.AttrValue[i].ID = types.FlexibleID(platformID)
			continue
		}

		if valueDomain == platformValueDomainGeneric && isSizeLikeAttribute(attrInfo) {
			if platformID, resolvedValue := resolvePlatformValueByStructuredSize(attrValue.Value, runtime, platformValues, m.valueMatcher); platformID > 0 {
				logger.GetGlobalLogger("shein/product").Infof(
					"resolved platform value via structured size resolver: attrID=%d domain=%s original=%q resolved=%q platformID=%d",
					attr.AttrID,
					valueDomain,
					attrValue.Value,
					resolvedValue,
					platformID,
				)
				attr.AttrValue[i].ID = types.FlexibleID(platformID)
				continue
			}
		}

		customAttempted := false
		customResult := CustomAttributeResult{}
		if shouldTryCustomBeforeFallbackForBeddingSize(attr.AttrID, attrValue.Value) {
			customAttempted = true
			customResult = m.processor.ProcessCustomAttributeValueWithRuntime(ctx, runtime, attr.AttrID, attrValue.Value, isRequired)
			if customResult.Success {
				attr.AttrValue[i].ID = types.FlexibleID(customResult.NewValueID)
				relations = append(relations, customResult.Relations...)
				continue
			}
			if customResult.PermissionDenied {
				if platformID, resolvedValue := resolveBeddingSizePlatformValue(attr.AttrID, attrValue.Value, platformValues, m.valueMatcher); platformID > 0 {
					categoryID := 0
					if runtime != nil {
						categoryID = runtime.CategoryID
					}
					logger.GetGlobalLogger("shein/product").Infof(
						"resolved platform value via bedding size rule after custom permission denied: attrID=%d categoryID=%d original=%q resolved=%q platformID=%d",
						attr.AttrID,
						categoryID,
						attrValue.Value,
						resolvedValue,
						platformID,
					)
					attr.AttrValue[i].ID = types.FlexibleID(platformID)
					continue
				}
				if isKnownBeddingSizeLabel(attr.AttrID, attrValue.Value) {
					categoryID := 0
					if runtime != nil {
						categoryID = runtime.CategoryID
					}
					logger.GetGlobalLogger("shein/product").Warnf(
						"skip automatic bedding size fallback after custom permission denied: attrID=%d categoryID=%d value=%q reason=no_deterministic_platform_value",
						attr.AttrID,
						categoryID,
						attrValue.Value,
					)
					continue
				}
				if shouldBlockRiskyBeddingSizeFallback(attr.AttrID, attrValue.Value) {
					categoryID := 0
					if runtime != nil {
						categoryID = runtime.CategoryID
					}
					logger.GetGlobalLogger("shein/product").Warnf(
						"skip automatic bedding size fallback after custom permission denied: attrID=%d categoryID=%d value=%q reason=requires_manual_review",
						attr.AttrID,
						categoryID,
						attrValue.Value,
					)
					continue
				}
			}
		}

		fallbackPlatformValues := m.selectFallbackPlatformValues(attrInfo, runtime, valueDomain, attrValue.Value, platformValues)
		if platformID, resolvedValue := m.resolvePlatformValueByFallback(ctx, runtime, valueDomain, attr.AttrID, attrValue.Value, fallbackPlatformValues); platformID > 0 {
			logger.GetGlobalLogger("shein/product").Infof(
				"resolved platform value via fallback resolver: attrID=%d domain=%s original=%q resolved=%q platformID=%d",
				attr.AttrID,
				valueDomain,
				attrValue.Value,
				resolvedValue,
				platformID,
			)
			attr.AttrValue[i].ID = types.FlexibleID(platformID)
			continue
		}

		m.logUnmatchedPlatformValue(attr.AttrID, attrValue.Value, valueDomain, platformValues)

		result := customResult
		if !customAttempted {
			result = m.processor.ProcessCustomAttributeValueWithRuntime(ctx, runtime, attr.AttrID, attrValue.Value, isRequired)
		}
		if !result.Success {
			if result.PermissionDenied {
				logger.GetGlobalLogger("shein/product").Warnf(
					"custom attribute values unsupported for attrID=%d, dropping unmatched value=%q",
					attr.AttrID, attrValue.Value,
				)
				attr.AttrValue = append(attr.AttrValue[:i], attr.AttrValue[i+1:]...)
				i--
				continue
			}
			logger.GetGlobalLogger("shein/product").Warnf(
				"custom attribute value mapping failed: attrID=%d value=%q required=%v shouldContinue=%v",
				attr.AttrID, attrValue.Value, isRequired, result.ShouldContinue,
			)
			if !result.ShouldContinue {
				return nil, fmt.Errorf("failed to create custom attribute value: %s", attrValue.Value)
			}
			continue
		}

		attr.AttrValue[i].ID = types.FlexibleID(result.NewValueID)
		relations = append(relations, result.Relations...)
	}

	return relations, nil
}

func shouldRemapSizeLikeAttributeValue(attrInfo *attribute.AttributeInfo) bool {
	return isSizeLikeAttribute(attrInfo)
}

func resolveBeddingSizePlatformValue(attrID int, rawValue string, platformValues map[string]int, matcher *AttributeValueMatcher) (int, string) {
	if attrID != 87 || matcher == nil {
		return 0, ""
	}
	candidates, ok := beddingSizePlatformValueCandidates(rawValue)
	if !ok {
		return 0, ""
	}
	for _, candidate := range candidates {
		if platformID := matcher.FindMatchingPlatformValue(candidate, platformValues); platformID > 0 {
			return platformID, candidate
		}
	}
	return 0, ""
}

func beddingSizePlatformValueCandidates(rawValue string) ([]string, bool) {
	normalized := normalizeBeddingSizeLabel(rawValue)
	switch normalized {
	case "single", "twin", "twin size":
		return []string{"99cm*190cm", "99*190"}, true
	case "twin xl", "twin x long", "twin extra long", "extra long twin":
		return []string{"105cm*200cm", "105*200", "100cm*200cm", "100*200"}, true
	case "full", "double", "full size", "double size", "full double":
		return []string{"138cm*190cm", "138*190", "140cm*190cm", "140*190"}, true
	case "queen", "queen size", "standard queen":
		return []string{"152cm*203cm", "152*203"}, true
	case "king", "king size", "standard king", "eastern king":
		return []string{"193cm*203cm", "193*203"}, true
	case "california king", "cal king", "western king":
		return []string{"183cm*213cm", "183*213"}, true
	case "crib", "crib size", "toddler", "toddler bed":
		return []string{"71cm*132cm", "71*132"}, true
	default:
		return nil, false
	}
}

func shouldTryCustomBeforeFallbackForBeddingSize(attrID int, rawValue string) bool {
	if attrID != 87 {
		return false
	}
	if isKnownBeddingSizeLabel(attrID, rawValue) {
		return true
	}
	return shouldBlockRiskyBeddingSizeFallback(attrID, rawValue)
}

func isKnownBeddingSizeLabel(attrID int, rawValue string) bool {
	if attrID != 87 {
		return false
	}
	_, ok := beddingSizePlatformValueCandidates(rawValue)
	return ok
}

func shouldBlockRiskyBeddingSizeFallback(attrID int, rawValue string) bool {
	if attrID != 87 {
		return false
	}
	normalized := normalizeBeddingSizeLabel(rawValue)
	switch normalized {
	case "split king", "split california king", "split cal king", "oversized king", "oversize king", "rv queen", "short queen", "olympic queen":
		return true
	default:
		return false
	}
}

func normalizeBeddingSizeLabel(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("-", " ", "_", " ", "/", " ", "(", " ", ")", " ")
	normalized = replacer.Replace(normalized)
	return strings.Join(strings.Fields(normalized), " ")
}

func (m *AttributeMapper) selectFallbackPlatformValues(attrInfo *attribute.AttributeInfo, runtime *MapperRuntimeInput, valueDomain platformValueDomain, rawValue string, platformValues map[string]int) map[string]int {
	if attrInfo == nil || valueDomain != platformValueDomainGeneric {
		return platformValues
	}
	if attrInfo.AttributeType == 0 {
		return platformValues
	}

	buckets := buildSizeSystemBuckets(platformValues)
	if !buckets.isMixed() {
		return platformValues
	}

	sourceSystem := inferSourceSizeSystem(rawValue, runtime)
	if sourceSystem == sizeSystemUnknown {
		preferredSystems := preferredTargetSizeSystems(runtime)
		filtered, preferredSystem := narrowPlatformValuesByPreferredSizeSystems(platformValues, preferredSystems)
		if preferredSystem != sizeSystemUnknown && len(filtered) != len(platformValues) {
			logger.GetGlobalLogger("shein/product").Infof(
				"size-system narrowing applied by target site preference: attrID=%d value=%q preferred_system=%s preferred_systems=%s region=%q original_candidates=%d narrowed_candidates=%d",
				attrInfo.AttributeID,
				rawValue,
				preferredSystem,
				formatSizeSystemsForLog(preferredSystems),
				runtime.Region,
				countUniquePlatformValues(platformValues),
				countUniquePlatformValues(filtered),
			)
			return filtered
		}

		logger.GetGlobalLogger("shein/product").Infof(
			"size-system narrowing skipped: attrID=%d value=%q reason=unknown_source_system mixed_systems=true preferred_systems=%s region=%q",
			attrInfo.AttributeID,
			rawValue,
			formatSizeSystemsForLog(preferredSystems),
			runtime.Region,
		)
		return platformValues
	}

	filtered := narrowPlatformValuesBySizeSystem(platformValues, sourceSystem)
	if len(filtered) == len(platformValues) {
		return platformValues
	}

	logger.GetGlobalLogger("shein/product").Infof(
		"size-system narrowing applied: attrID=%d value=%q source_system=%s original_candidates=%d narrowed_candidates=%d",
		attrInfo.AttributeID,
		rawValue,
		sourceSystem,
		countUniquePlatformValues(platformValues),
		countUniquePlatformValues(filtered),
	)
	return filtered
}

func (m *AttributeMapper) resolvePlatformValueByFallback(taskCtx *sheinctx.TaskContext, runtime *MapperRuntimeInput, domain platformValueDomain, attrID int, rawValue string, platformValues map[string]int) (int, string) {
	if runtime == nil {
		return 0, ""
	}
	cacheKey := runtime.buildFallbackCacheKey(domain, attrID, rawValue, platformValues)
	if runtime.FallbackCache != nil {
		var cached PlatformValueFallbackResult
		if runtime.FallbackCache.Get(aicache.TypeAttrValueFallback, cacheKey, &cached) {
			logger.GetGlobalLogger("shein/product").Infof(
				"platform value fallback cache hit: attrID=%d domain=%s value=%q resolved=%q confidence=%.2f",
				attrID, domain, rawValue, cached.ResolvedValue, cached.Confidence,
			)
			if platformID, resolved := m.matchFallbackResult(runtime, &cached, rawValue, platformValues); platformID > 0 {
				return platformID, resolved
			}
		}
	}
	if runtime.FallbackValueResolver == nil {
		return 0, ""
	}

	req := runtime.buildFallbackRequest(attrID, domain, rawValue, platformValues)
	if req == nil {
		return 0, ""
	}

	callCtx := context.Background()
	if taskCtx != nil && taskCtx.Context != nil {
		callCtx = taskCtx.Context
	}
	result, err := runtime.FallbackValueResolver.ResolvePlatformValue(callCtx, req)
	if err != nil {
		logger.GetGlobalLogger("shein/product").Warnf(
			"platform value fallback resolver failed: attrID=%d domain=%s value=%q err=%v",
			attrID, domain, rawValue, err,
		)
		return 0, ""
	}
	if result == nil {
		return 0, ""
	}
	if platformID, resolved := m.matchFallbackResult(runtime, result, rawValue, platformValues); platformID > 0 {
		if runtime.FallbackCache != nil {
			runtime.FallbackCache.Set(aicache.TypeAttrValueFallback, cacheKey, result)
			logger.GetGlobalLogger("shein/product").Infof(
				"platform value fallback cache store: attrID=%d domain=%s value=%q resolved=%q confidence=%.2f",
				attrID, domain, rawValue, result.ResolvedValue, result.Confidence,
			)
		}
		return platformID, resolved
	}
	logger.GetGlobalLogger("shein/product").Warnf(
		"platform value fallback rejected: attrID=%d domain=%s value=%q resolved=%q confidence=%.2f reason=%q",
		attrID, domain, rawValue, result.ResolvedValue, result.Confidence, result.Reason,
	)
	return 0, ""
}

func (m *AttributeMapper) matchFallbackResult(runtime *MapperRuntimeInput, result *PlatformValueFallbackResult, rawValue string, platformValues map[string]int) (int, string) {
	if result == nil || result.ResolvedValue == "" {
		return 0, ""
	}
	minConfidence := 0.8
	if runtime != nil && runtime.FallbackMinConfidence > 0 {
		minConfidence = runtime.FallbackMinConfidence
	}
	if result.Confidence > 0 && result.Confidence < minConfidence {
		logger.GetGlobalLogger("shein/product").Infof(
			"platform value fallback below confidence threshold: resolved=%q confidence=%.2f min_confidence=%.2f",
			result.ResolvedValue, result.Confidence, minConfidence,
		)
		return 0, ""
	}
	if !fallbackShoeSizeSemanticsCompatible(runtime, rawValue, result.ResolvedValue) {
		logger.GetGlobalLogger("shein/product").Infof(
			"platform value fallback rejected due to shoe-size semantic mismatch: source=%q resolved=%q",
			rawValue, result.ResolvedValue,
		)
		return 0, ""
	}
	if platformID := m.valueMatcher.FindMatchingPlatformValue(result.ResolvedValue, platformValues); platformID > 0 {
		return platformID, result.ResolvedValue
	}
	return 0, ""
}

func fallbackShoeSizeSemanticsCompatible(runtime *MapperRuntimeInput, rawValue, resolvedValue string) bool {
	source, ok := resolveStructuredSourceShoeSize(rawValue, runtime)
	if !ok {
		return true
	}
	resolved := sheinsize.ParseShoeSize(resolvedValue)
	if !resolved.IsShoeSize {
		return true
	}
	if source.BaseSize != "" && resolved.BaseSize != "" && source.BaseSize != resolved.BaseSize {
		return false
	}
	if source.System != sheinsize.SystemUnknown &&
		resolved.System != sheinsize.SystemUnknown &&
		source.System != resolved.System {
		return false
	}
	if source.Width == sheinsize.WidthWide || source.Width == sheinsize.WidthXWide {
		if resolved.Width == sheinsize.WidthUnknown || resolved.Width == sheinsize.WidthRegular {
			return false
		}
		if !sheinsize.AreShoeSizesFuzzyCompatible(rawValue, resolvedValue) {
			return false
		}
	}
	return true
}

func (m *AttributeMapper) logUnmatchedPlatformValue(attrID int, rawValue string, valueDomain platformValueDomain, platformValues map[string]int) {
	if valueDomain == platformValueDomainNumericSizeLike {
		logger.GetGlobalLogger("shein/product").Warnf(
			"platform attribute value unmatched in numeric size domain: attrID=%d value=%q sanitized=%q normalized=%q platform_candidate_count=%d platform_candidate_sample=%v",
			attrID,
			rawValue,
			content.SanitizeForSheinAttribute(rawValue),
			normalizeAttributeValueForLog(rawValue),
			countUniquePlatformValues(platformValues),
			samplePlatformValues(platformValues, 8),
		)
		return
	}

	logger.GetGlobalLogger("shein/product").Warnf(
		"platform attribute value unmatched: attrID=%d value=%q sanitized=%q normalized=%q platform_candidate_count=%d platform_candidate_sample=%v",
		attrID,
		rawValue,
		content.SanitizeForSheinAttribute(rawValue),
		normalizeAttributeValueForLog(rawValue),
		countUniquePlatformValues(platformValues),
		samplePlatformValues(platformValues, 8),
	)
}

func normalizeAttributeValueForLog(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func countUniquePlatformValues(platformValues map[string]int) int {
	return len(samplePlatformValues(platformValues, len(platformValues)))
}

func samplePlatformValues(platformValues map[string]int, limit int) []string {
	if len(platformValues) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(platformValues))
	values := make([]string, 0, len(platformValues))
	for platformValue := range platformValues {
		normalized := strings.ToLower(strings.TrimSpace(platformValue))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		values = append(values, platformValue)
	}

	sort.Strings(values)
	if len(values) > limit {
		return values[:limit]
	}
	return values
}

func stablePlatformValues(platformValues map[string]int, limit int) []string {
	if len(platformValues) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(platformValues))
	values := make([]string, 0, len(platformValues))
	for platformValue := range platformValues {
		normalized := strings.ToLower(strings.TrimSpace(platformValue))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		values = append(values, normalized)
	}

	sort.Strings(values)
	if len(values) > limit {
		return values[:limit]
	}
	return values
}
