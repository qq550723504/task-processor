package shein

import (
	"strings"
	"testing"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func TestInferMissingRequiredDisplayAttributesRepairSkipsWhenTooManyPending(t *testing.T) {
	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     1,
			AttributeNameEn: "Material",
			AttributeMode:   1,
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 101, AttributeValueEn: "Wood"},
			},
		},
		{
			AttributeID:     2,
			AttributeNameEn: "Pattern",
			AttributeMode:   1,
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 201, AttributeValueEn: "Print"},
			},
		},
		{
			AttributeID:     3,
			AttributeNameEn: "Style",
			AttributeMode:   1,
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 301, AttributeValueEn: "Casual"},
			},
		},
		{
			AttributeID:     4,
			AttributeNameEn: "Occasion",
			AttributeMode:   1,
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 401, AttributeValueEn: "Daily"},
			},
		},
		{
			AttributeID:     5,
			AttributeNameEn: "Season",
			AttributeMode:   1,
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 501, AttributeValueEn: "All"},
			},
		},
		{
			AttributeID:     6,
			AttributeNameEn: "Room",
			AttributeMode:   1,
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 601, AttributeValueEn: "Outdoor"},
			},
		},
		{
			AttributeID:     7,
			AttributeNameEn: "Type",
			AttributeMode:   1,
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 701, AttributeValueEn: "Curtain Panels"},
			},
		},
	}
	inputs := []common.Attribute{
		{Name: "Title", Value: "Decorative wall clock"},
	}

	resolved, notes := inferMissingRequiredDisplayAttributesRepair(attributes, inputs, map[int]ResolvedAttribute{}, panicDisplayAttributeLLM{})
	if len(resolved) != 0 {
		t.Fatalf("resolved = %#v, want none", resolved)
	}
	joined := strings.Join(notes, " | ")
	if !strings.Contains(joined, "已跳过逐属性 repair") {
		t.Fatalf("notes = %q, want skip-repair note", joined)
	}
}
