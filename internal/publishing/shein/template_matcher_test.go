package shein

import (
	"testing"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

func TestTemplateIndexMatchDoesNotUseChineseAliasesForSize(t *testing.T) {
	index := newTemplateIndex([]sheinattribute.AttributeInfo{
		{
			AttributeID:     87,
			AttributeName:   "尺寸",
			AttributeNameEn: "Size",
			AttributeType:   1,
		},
	})

	match := index.Match("尺码", "39")
	if match.AttributeID != 0 {
		t.Fatalf("expected no alias match, got attribute %d", match.AttributeID)
	}
}
