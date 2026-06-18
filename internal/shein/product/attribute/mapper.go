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

		result := m.processor.ProcessCustomAttributeValueWithRuntime(ctx, runtime, attr.AttrID, attrValue.Value, isRequired)
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
			if platformID, resolved := m.matchFallbackResult(runtime, &cached, platformValues); platformID > 0 {
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
	if platformID, resolved := m.matchFallbackResult(runtime, result, platformValues); platformID > 0 {
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

func (m *AttributeMapper) matchFallbackResult(runtime *MapperRuntimeInput, result *PlatformValueFallbackResult, platformValues map[string]int) (int, string) {
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
	if platformID := m.valueMatcher.FindMatchingPlatformValue(result.ResolvedValue, platformValues); platformID > 0 {
		return platformID, result.ResolvedValue
	}
	return 0, ""
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
