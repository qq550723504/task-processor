package shein

import (
	"testing"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

func TestTemplateIndexMatchUsesChineseAliasesForSize(t *testing.T) {
	index := newTemplateIndex([]sheinattribute.AttributeInfo{
		{
			AttributeID:     87,
			AttributeName:   "尺寸",
			AttributeNameEn: "Size",
			AttributeType:   1,
		},
	})

	match := index.Match("尺码", "39")
	if match.AttributeID != 87 {
		t.Fatalf("expected attribute 87, got %d", match.AttributeID)
	}
	if match.Name != "Size" {
		t.Fatalf("expected matched name Size, got %q", match.Name)
	}
}
