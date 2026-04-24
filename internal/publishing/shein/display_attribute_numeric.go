package shein

import (
	"fmt"
	"regexp"
	"strings"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

var numericTokenPattern = regexp.MustCompile(`[-+]?\d+(?:\.\d+)?`)

func numericAttributeNotes(attr sheinattribute.AttributeInfo, sourceValue string) []string {
	notes := []string{
		fmt.Sprintf("SHEIN 数值属性待确认: 属性 %q 当前保留原始值 %q，后续需按模板规则校验单位/范围", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), sourceValue),
	}
	if !containsNumericToken(sourceValue) {
		notes = append(notes, fmt.Sprintf("SHEIN 数值属性值无效: 属性 %q 当前值 %q 未识别到有效数字", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), sourceValue))
	}
	if attr.CascadeAttributeID > 0 {
		notes = append(notes, fmt.Sprintf("SHEIN 数值属性存在联动约束: 属性 %q 依赖上游属性 %d", firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), attr.CascadeAttributeID))
	}
	return notes
}

func containsNumericToken(value string) bool {
	return numericTokenPattern.FindString(strings.TrimSpace(value)) != ""
}
