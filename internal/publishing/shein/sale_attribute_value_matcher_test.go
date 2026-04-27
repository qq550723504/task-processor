package shein

import (
	"context"
	"encoding/json"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type stubSaleAttributeLLM struct {
	response string
	err      error
}

func (s stubSaleAttributeLLM) CreateChatCompletion(context.Context, *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return nil, s.err
}

func (s stubSaleAttributeLLM) Generate(context.Context, string) (string, error) {
	return s.response, s.err
}

func (s stubSaleAttributeLLM) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", s.err
}

func (s stubSaleAttributeLLM) GetDefaultModel() string {
	return "test"
}

func TestBuildValueAssignmentsMatchesNormalizedNumericValues(t *testing.T) {
	index := newTemplateIndex([]sheinattribute.AttributeInfo{{
		AttributeID:       502,
		AttributeName:     "尺码",
		AttributeNameEn:   "Size",
		AttributeInputNum: 1,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 21, AttributeValue: "40", AttributeValueEn: "EUR40"},
			{AttributeValueID: 22, AttributeValue: "41", AttributeValueEn: "EUR41"},
		},
	}})

	assignments, _, notes := buildValueAssignments([]string{"40码"}, "尺码", "Size", "sku", index, nil, 0, "", nil)
	if len(notes) != 0 {
		t.Fatalf("notes = %v, want empty", notes)
	}
	assignment, ok := assignments[normalizeText("40码")]
	if !ok {
		t.Fatalf("missing normalized assignment for 40码")
	}
	if assignment.AttributeValueID == nil || *assignment.AttributeValueID != 21 {
		t.Fatalf("attribute value id = %v, want 21", assignment.AttributeValueID)
	}
	if assignment.MatchedBy != "attribute_value_normalized" && assignment.MatchedBy != "attribute_value_comparable" {
		t.Fatalf("matched by = %q, want normalized or comparable match", assignment.MatchedBy)
	}
}

func TestBuildValueAssignmentsMatchesEURSizeValues(t *testing.T) {
	index := newTemplateIndex([]sheinattribute.AttributeInfo{{
		AttributeID:       502,
		AttributeName:     "尺码",
		AttributeNameEn:   "Size",
		AttributeInputNum: 1,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 21, AttributeValue: "39", AttributeValueEn: "EUR39"},
			{AttributeValueID: 22, AttributeValue: "42", AttributeValueEn: "EUR42"},
		},
	}})

	assignments, _, notes := buildValueAssignments([]string{"39", "42"}, "尺码", "Size", "sku", index, nil, 0, "", nil)
	if len(notes) != 0 {
		t.Fatalf("notes = %v, want empty", notes)
	}
	if assignments[normalizeText("39")].AttributeValueID == nil || *assignments[normalizeText("39")].AttributeValueID != 21 {
		t.Fatalf("39 assignment = %+v, want value id 21", assignments[normalizeText("39")])
	}
	if assignments[normalizeText("42")].AttributeValueID == nil || *assignments[normalizeText("42")].AttributeValueID != 22 {
		t.Fatalf("42 assignment = %+v, want value id 22", assignments[normalizeText("42")])
	}
}

func TestBuildValueAssignmentsStripsSourceCodePrefixForColor(t *testing.T) {
	index := newTemplateIndex([]sheinattribute.AttributeInfo{{
		AttributeID:       501,
		AttributeName:     "颜色",
		AttributeNameEn:   "Color",
		AttributeType:     1,
		AttributeInputNum: 1,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 11, AttributeValue: "黑色", AttributeValueEn: "Black"},
			{AttributeValueID: 12, AttributeValue: "深灰色", AttributeValueEn: "Dark Grey"},
		},
	}})

	assignments, _, notes := buildValueAssignments([]string{"B-2601黑色"}, "颜色", "Color", "skc", index, nil, 0, "", nil)
	if len(notes) != 0 {
		t.Fatalf("notes = %v, want empty", notes)
	}
	assignment, ok := assignments[normalizeText("B-2601黑色")]
	if !ok {
		t.Fatalf("missing normalized assignment for B-2601黑色")
	}
	if assignment.AttributeValueID == nil || *assignment.AttributeValueID != 11 {
		t.Fatalf("attribute value id = %v, want 11", assignment.AttributeValueID)
	}
	if assignment.MatchedBy != "attribute_value_normalized" {
		t.Fatalf("matched by = %q, want attribute_value_normalized", assignment.MatchedBy)
	}
}

func TestBuildValueAssignmentsUsesLastSegmentForCompositeColor(t *testing.T) {
	index := newTemplateIndex([]sheinattribute.AttributeInfo{{
		AttributeID:       501,
		AttributeName:     "颜色",
		AttributeNameEn:   "Color",
		AttributeType:     1,
		AttributeInputNum: 1,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 11, AttributeValue: "深蓝", AttributeValueEn: "Navy Blue"},
			{AttributeValueID: 12, AttributeValue: "黑色", AttributeValueEn: "Black"},
		},
	}})

	assignments, _, notes := buildValueAssignments([]string{"牛津布/防水防晒/深蓝"}, "颜色", "Color", "skc", index, nil, 0, "", nil)
	if len(notes) != 0 {
		t.Fatalf("notes = %v, want empty", notes)
	}
	assignment, ok := assignments[normalizeText("牛津布/防水防晒/深蓝")]
	if !ok {
		t.Fatalf("missing normalized assignment for composite color")
	}
	if assignment.AttributeValueID == nil || *assignment.AttributeValueID != 11 {
		t.Fatalf("attribute value id = %v, want 11", assignment.AttributeValueID)
	}
}

func TestBuildValueAssignmentsStripsWeightAnnotationForSize(t *testing.T) {
	index := newTemplateIndex([]sheinattribute.AttributeInfo{{
		AttributeID:       502,
		AttributeName:     "尺码",
		AttributeNameEn:   "Size",
		AttributeInputNum: 1,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 21, AttributeValue: "150x100x10cm", AttributeValueEn: "150x100x10cm"},
			{AttributeValueID: 22, AttributeValue: "120x100x10cm", AttributeValueEn: "120x100x10cm"},
		},
	}})

	assignments, _, notes := buildValueAssignments([]string{"150*100*10CM（4kg）"}, "尺寸", "Size", "sku", index, nil, 0, "", nil)
	if len(notes) != 0 {
		t.Fatalf("notes = %v, want empty", notes)
	}
	assignment, ok := assignments[normalizeText("150*100*10CM（4kg）")]
	if !ok {
		t.Fatalf("missing normalized assignment for weighted size")
	}
	if assignment.AttributeValueID == nil || *assignment.AttributeValueID != 21 {
		t.Fatalf("attribute value id = %v, want 21", assignment.AttributeValueID)
	}
}

func TestBuildValueAssignmentsMatchesSegmentedCandidateWithoutExactWholeValueMatch(t *testing.T) {
	index := newTemplateIndex([]sheinattribute.AttributeInfo{{
		AttributeID:       501,
		AttributeName:     "颜色",
		AttributeNameEn:   "Color",
		AttributeType:     1,
		AttributeInputNum: 1,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 11, AttributeValue: "防水深蓝", AttributeValueEn: "Waterproof Navy Blue"},
			{AttributeValueID: 12, AttributeValue: "防水黑色", AttributeValueEn: "Waterproof Black"},
		},
	}})

	assignments, _, notes := buildValueAssignments([]string{"牛津布/防水深蓝"}, "颜色", "Color", "skc", index, nil, 0, "", nil)
	if len(notes) != 0 {
		t.Fatalf("notes = %v, want empty", notes)
	}
	assignment, ok := assignments[normalizeText("牛津布/防水深蓝")]
	if !ok {
		t.Fatalf("missing normalized assignment for segmented candidate")
	}
	if assignment.AttributeValueID == nil || *assignment.AttributeValueID != 11 {
		t.Fatalf("attribute value id = %v, want 11", assignment.AttributeValueID)
	}
	if assignment.MatchedBy != "attribute_value_comparable" && assignment.MatchedBy != "attribute_value_normalized" {
		t.Fatalf("matched by = %q, want comparable-style match", assignment.MatchedBy)
	}
}

func TestBuildValueAssignmentsUsesLLMFallback(t *testing.T) {
	index := newTemplateIndex([]sheinattribute.AttributeInfo{{
		AttributeID:       501,
		AttributeName:     "颜色",
		AttributeNameEn:   "Color",
		AttributeType:     1,
		AttributeInputNum: 1,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 11, AttributeValue: "黑色", AttributeValueEn: "Black"},
			{AttributeValueID: 12, AttributeValue: "白色", AttributeValueEn: "White"},
		},
	}})

	assignments, _, notes := buildValueAssignments(
		[]string{"B-2601黑灰色"},
		"颜色",
		"Color",
		"skc",
		index,
		nil,
		0,
		"",
		stubSaleAttributeLLM{response: `{"matches":[{"source_value":"B-2601黑灰色","attribute_value_id":11,"reasons":["closest semantic match"]}]}`},
	)
	if len(notes) != 1 || notes[0] != "closest semantic match" {
		t.Fatalf("notes = %v, want [closest semantic match]", notes)
	}
	assignment, ok := assignments[normalizeText("B-2601黑灰色")]
	if !ok {
		t.Fatalf("missing llm assignment for B-2601黑灰色")
	}
	if assignment.AttributeValueID == nil || *assignment.AttributeValueID != 11 {
		t.Fatalf("attribute value id = %v, want 11", assignment.AttributeValueID)
	}
	if assignment.MatchedBy != "llm_attribute_value" {
		t.Fatalf("matched by = %q, want llm_attribute_value", assignment.MatchedBy)
	}
}

func TestBuildValueAssignmentsUsesTemplateBoundLLMForUnseenColorValue(t *testing.T) {
	index := newTemplateIndex([]sheinattribute.AttributeInfo{{
		AttributeID:       501,
		AttributeName:     "颜色",
		AttributeNameEn:   "Color",
		AttributeType:     1,
		AttributeInputNum: 1,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 11, AttributeValue: "Dusty Blue", AttributeValueEn: "Dusty Blue"},
			{AttributeValueID: 12, AttributeValue: "Black", AttributeValueEn: "Black"},
		},
	}})

	assignments, _, notes := buildValueAssignments(
		[]string{"雾霾蓝"},
		"颜色",
		"Color",
		"skc",
		index,
		nil,
		0,
		"",
		stubSaleAttributeLLM{response: `{"matches":[{"source_value":"雾霾蓝","attribute_value_id":11,"reasons":["matched against existing template value"]}]}`},
	)
	if len(notes) != 1 || notes[0] != "matched against existing template value" {
		t.Fatalf("notes = %v, want [matched against existing template value]", notes)
	}
	assignment, ok := assignments[normalizeText("雾霾蓝")]
	if !ok {
		t.Fatalf("missing llm assignment for 雾霾蓝")
	}
	if assignment.AttributeValueID == nil || *assignment.AttributeValueID != 11 {
		t.Fatalf("attribute value id = %v, want 11", assignment.AttributeValueID)
	}
	if assignment.MatchedBy != "llm_attribute_value" {
		t.Fatalf("matched by = %q, want llm_attribute_value", assignment.MatchedBy)
	}
}

func TestBuildValueAssignmentsReturnsReviewNoteWhenNoMatch(t *testing.T) {
	index := newTemplateIndex([]sheinattribute.AttributeInfo{{
		AttributeID:       501,
		AttributeName:     "颜色",
		AttributeNameEn:   "Color",
		AttributeType:     1,
		AttributeInputNum: 1,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 11, AttributeValue: "红色", AttributeValueEn: "Red"},
			{AttributeValueID: 12, AttributeValue: "蓝色", AttributeValueEn: "Blue"},
		},
	}})

	assignments, _, notes := buildValueAssignments([]string{"未知色"}, "颜色", "Color", "skc", index, nil, 0, "", nil)
	if assignments != nil {
		t.Fatalf("assignments = %+v, want nil", assignments)
	}
	if len(notes) != 1 {
		t.Fatalf("notes len = %d, want 1", len(notes))
	}
}

func TestBuildValueAssignmentsCreatesCustomSaleAttributeValueAfterValidation(t *testing.T) {
	index := newTemplateIndex([]sheinattribute.AttributeInfo{{
		AttributeID:       501,
		AttributeName:     "颜色",
		AttributeNameEn:   "Color",
		AttributeType:     1,
		AttributeInputNum: 1,
	}})

	api := stubAttributeAPI{
		validateCustom: func(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error) {
			resp := &sheinattribute.ValidateAttributeResponse{}
			resp.Data.AttributeID = attributeID
			resp.Data.PreAttributeValueID = 3001
			resp.Data.AttributeValueNameMultis = []struct {
				Language                string `json:"language"`
				AttributeValueNameMulti string `json:"attribute_value_name_multi"`
				WarningType             int    `json:"warning_type"`
			}{
				{Language: "en", AttributeValueNameMulti: "Cream Beige"},
			}
			return resp, nil
		},
		addCustom: func(req *sheinattribute.AddCustomAttributeValueRequest) (*sheinattribute.AddCustomAttributeValueResponse, error) {
			resp := &sheinattribute.AddCustomAttributeValueResponse{}
			resp.Info.Data.CustomAttributeRelation = []sheinattribute.CustomAttributeRelation{{
				PreAttributeValueID: 3001,
				AttributeValueID:    9001,
			}}
			return resp, nil
		},
	}

	assignments, relations, notes := buildValueAssignments([]string{"米驼"}, "颜色", "Color", "skc", index, api, 12143, "Bench Cushion", nil)
	assignment, ok := assignments[normalizeText("米驼")]
	if !ok {
		t.Fatalf("missing custom assignment for 米驼")
	}
	if assignment.AttributeValueID == nil || *assignment.AttributeValueID != 9001 {
		t.Fatalf("attribute value id = %v, want 9001", assignment.AttributeValueID)
	}
	if assignment.MatchedBy != "custom_attribute_value" {
		t.Fatalf("matched by = %q, want custom_attribute_value", assignment.MatchedBy)
	}
	if len(relations) != 1 || relations[0].AttributeValueID != 9001 {
		t.Fatalf("relations = %+v, want created custom relation", relations)
	}
	if len(notes) == 0 {
		t.Fatalf("notes = %v, want custom creation note", notes)
	}
}

func TestApplySaleAttributeResolutionDoesNotFallbackToAttributeOnlyPlaceholder(t *testing.T) {
	pkg := &Package{
		SkcList: []SKCPackage{
			{
				SupplierCode: "SCRAPED-SET",
				Attributes:   map[string]string{"颜色": "一桌四椅套装"},
				SKUs:         []common.Variant{{SKU: "SCRAPED-SET"}},
			},
		},
		RequestDraft: &RequestDraft{
			SKCList: []SKCRequestDraft{{
				SupplierCode: "SCRAPED-SET",
				SKUList:      []SKUDraft{{SupplierSKU: "SCRAPED-SET"}},
			}},
		},
	}
	resolution := &SaleAttributeResolution{
		PrimarySourceDimension: "颜色",
		SKCAttributes: []ResolvedSaleAttribute{{
			Scope:       "skc",
			Name:        "Color",
			AttributeID: 27,
			MatchedBy:   "attribute_name",
		}},
	}

	ApplySaleAttributeResolution(pkg, resolution)

	if pkg.RequestDraft.SKCList[0].SaleAttribute != nil {
		t.Fatalf("expected no placeholder SKC sale attribute, got %+v", pkg.RequestDraft.SKCList[0].SaleAttribute)
	}
}

func TestApplySaleAttributeResolutionUsesSerializedValueAssignments(t *testing.T) {
	valueID := 9001
	resolution := &SaleAttributeResolution{
		PrimarySourceDimension: "ai_style",
		SKCAttributes: []ResolvedSaleAttribute{{
			Scope:       "skc",
			Name:        "Style Type",
			Value:       "Style A",
			AttributeID: 1001184,
			MatchedBy:   "custom_attribute_value",
		}},
		SKCValueAssignments: map[string]ResolvedSaleAttribute{
			normalizeText("Style A"): {
				Scope:            "skc",
				Name:             "Style Type",
				Value:            "Style A",
				AttributeID:      1001184,
				AttributeValueID: &valueID,
				MatchedBy:        "custom_attribute_value",
			},
		},
	}
	data, err := json.Marshal(resolution)
	if err != nil {
		t.Fatalf("marshal resolution: %v", err)
	}
	var decoded SaleAttributeResolution
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal resolution: %v", err)
	}
	pkg := &Package{
		SkcList: []SKCPackage{{
			SupplierCode: "SKC-1",
			Attributes:   map[string]string{"ai_style": "Style A"},
		}},
		RequestDraft: &RequestDraft{SKCList: []SKCRequestDraft{{
			SupplierCode: "SKC-1",
			SKUList:      []SKUDraft{{SupplierSKU: "SKU-1"}},
		}}},
	}

	ApplySaleAttributeResolution(pkg, &decoded)

	got := pkg.RequestDraft.SKCList[0].SaleAttribute
	if got == nil || got.AttributeValueID == nil || *got.AttributeValueID != 9001 {
		t.Fatalf("skc sale attribute = %+v, want serialized value id 9001", got)
	}
}
