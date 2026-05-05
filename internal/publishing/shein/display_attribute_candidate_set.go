package shein

import (
	"fmt"
	"sort"
	"strings"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

const (
	maxDisplayAttributePromptCandidates = 8
	minDisplayAttributePromptCandidates = 3
)

type scoredDisplayAttributeOption struct {
	option sheinattribute.AttributeValue
	score  int
	index  int
}

func narrowDisplayAttributeValueOptions(
	attr sheinattribute.AttributeInfo,
	sourceName string,
	sourceValue string,
	contextInputs []common.Attribute,
	limit int,
) []sheinattribute.AttributeValue {
	if limit <= 0 || len(attr.AttributeValueInfoList) <= limit {
		return append([]sheinattribute.AttributeValue(nil), attr.AttributeValueInfoList...)
	}

	queryTokens := buildDisplayAttributeCandidateQueryTokens(sourceName, sourceValue, contextInputs)
	sourceNorm := normalizeText(sourceValue)
	segments := comparableAttributeSegments(sourceValue)
	scored := make([]scoredDisplayAttributeOption, 0, len(attr.AttributeValueInfoList))
	for idx, option := range attr.AttributeValueInfoList {
		score := scoreDisplayAttributeOption(option, sourceNorm, segments, queryTokens)
		scored = append(scored, scoredDisplayAttributeOption{
			option: option,
			score:  score,
			index:  idx,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].index < scored[j].index
		}
		return scored[i].score > scored[j].score
	})

	target := limit
	if target < minDisplayAttributePromptCandidates {
		target = minDisplayAttributePromptCandidates
	}
	if target > len(scored) {
		target = len(scored)
	}

	positive := 0
	for _, item := range scored {
		if item.score > 0 {
			positive++
		}
	}
	if positive >= minDisplayAttributePromptCandidates && positive < target {
		target = positive
	}

	selected := make([]sheinattribute.AttributeValue, 0, target)
	for i := 0; i < target; i++ {
		selected = append(selected, scored[i].option)
	}
	return selected
}

func buildDisplayAttributeCandidateQueryTokens(
	sourceName string,
	sourceValue string,
	contextInputs []common.Attribute,
) []string {
	parts := []string{sourceName, sourceValue}
	for _, segment := range comparableAttributeSegments(sourceValue) {
		parts = append(parts, segment)
	}
	for _, item := range contextInputs {
		if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Value) == "" {
			continue
		}
		parts = append(parts, item.Name, item.Value)
	}
	return normalizeTextTokens(parts...)
}

func scoreDisplayAttributeOption(
	option sheinattribute.AttributeValue,
	sourceNorm string,
	sourceSegments []string,
	queryTokens []string,
) int {
	candidateTexts := []string{
		strings.TrimSpace(option.AttributeValue),
		strings.TrimSpace(option.AttributeValueEn),
	}

	score := 0
	for _, text := range candidateTexts {
		norm := normalizeText(text)
		if norm == "" {
			continue
		}
		if sourceNorm != "" && norm == sourceNorm {
			score += 1000
		}
		if sourceNorm != "" && (strings.Contains(sourceNorm, norm) || strings.Contains(norm, sourceNorm)) {
			score += 400
		}
		for _, segment := range sourceSegments {
			segmentNorm := normalizeText(segment)
			if segmentNorm == "" {
				continue
			}
			if segmentNorm == norm {
				score += 250
			}
			if strings.Contains(segmentNorm, norm) || strings.Contains(norm, segmentNorm) {
				score += 120
			}
		}
		score += 20 * tokenOverlapCount(queryTokens, normalizeTextTokens(text))
	}
	return score
}

func tokenOverlapCount(left []string, right []string) int {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, len(left))
	for _, token := range left {
		seen[token] = struct{}{}
	}
	count := 0
	for _, token := range right {
		if _, ok := seen[token]; ok {
			count++
		}
	}
	return count
}

func describeDisplayAttributeCandidates(
	attr sheinattribute.AttributeInfo,
	sourceName string,
	sourceValue string,
	contextInputs []common.Attribute,
	limit int,
) string {
	options := narrowDisplayAttributeValueOptions(attr, sourceName, sourceValue, contextInputs, limit)
	if len(options) == 0 {
		return ""
	}
	parts := make([]string, 0, len(options))
	for _, option := range options {
		label := firstNonEmpty(option.AttributeValueEn, option.AttributeValue)
		if strings.TrimSpace(label) == "" {
			label = fmt.Sprintf("attribute_value_id=%d", option.AttributeValueID)
		}
		parts = append(parts, fmt.Sprintf("%s(%d)", label, option.AttributeValueID))
	}
	return strings.Join(parts, ", ")
}

func describeDisplayAttributeEvidenceFields(inputs []common.Attribute, limit int) string {
	if len(inputs) == 0 {
		return ""
	}
	if limit <= 0 {
		limit = 8
	}
	fields := make([]string, 0, limit)
	seen := make(map[string]struct{}, len(inputs))
	for _, item := range inputs {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		if name == "" || value == "" {
			continue
		}
		key := normalizeText(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		fields = append(fields, name)
		if len(fields) >= limit {
			break
		}
	}
	return strings.Join(fields, ", ")
}
