package shein

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

var (
	compositionPercentPattern       = regexp.MustCompile(`(?i)([^,;+\n]+?)\s*[:：]?\s*(\d+(?:\.\d+)?)\s*%`)
	compositionLeadingPercentPattern = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*%\s*([^,;+\n]+)`)
)

type compositionItem struct {
	Name    string
	Percent float64
}

func compositionAttributeNotes(attr sheinattribute.AttributeInfo, sourceValue string) []string {
	notes := []string{
		fmt.Sprintf("SHEIN 成分类属性待确认: 属性 %q 当前保留原始值 %q，后续需补成分拆解与合计校验", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), sourceValue),
	}
	items := parseCompositionItems(sourceValue)
	switch {
	case len(items) == 0:
		notes = append(notes, fmt.Sprintf("SHEIN 成分类属性缺少可计算百分比: 属性 %q 当前未识别出成分占比", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)))
	default:
		total := 0.0
		for _, item := range items {
			total += item.Percent
		}
		if total < 99.5 || total > 100.5 {
			notes = append(notes, fmt.Sprintf("SHEIN 成分类属性比例不自洽: 属性 %q 当前识别占比合计 %.1f%%，需人工校正到 100%%", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), total))
		}
	}
	if attr.CascadeAttributeID > 0 {
		notes = append(notes, fmt.Sprintf("SHEIN 成分类属性存在联动约束: 属性 %q 依赖上游属性 %d", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), attr.CascadeAttributeID))
	}
	return notes
}

func parseCompositionItems(value string) []compositionItem {
	items := collectCompositionItems(value, compositionPercentPattern, false)
	if len(items) > 0 {
		return items
	}
	return collectCompositionItems(value, compositionLeadingPercentPattern, true)
}

func collectCompositionItems(value string, pattern *regexp.Regexp, percentFirst bool) []compositionItem {
	matches := pattern.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		return nil
	}
	items := make([]compositionItem, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		var (
			nameRaw    string
			percentRaw string
		)
		if percentFirst {
			percentRaw = match[1]
			nameRaw = match[2]
		} else {
			nameRaw = match[1]
			percentRaw = match[2]
		}
		name := strings.TrimSpace(strings.Trim(nameRaw, ",;+/ "))
		percent, err := strconv.ParseFloat(strings.TrimSpace(percentRaw), 64)
		if err != nil || name == "" {
			continue
		}
		items = append(items, compositionItem{Name: name, Percent: percent})
	}
	return items
}
