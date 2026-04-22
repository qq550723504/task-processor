package shein

import (
	"context"
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

	assignments, notes := buildValueAssignments([]string{"40码"}, "尺码", "Size", "sku", index, nil)
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
	if assignment.MatchedBy != "attribute_value_normalized" {
		t.Fatalf("matched by = %q, want attribute_value_normalized", assignment.MatchedBy)
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

	assignments, notes := buildValueAssignments([]string{"39", "42"}, "尺码", "Size", "sku", index, nil)
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

	assignments, notes := buildValueAssignments([]string{"B-2601黑色"}, "颜色", "Color", "skc", index, nil)
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

	assignments, notes := buildValueAssignments(
		[]string{"B-2601黑灰色"},
		"颜色",
		"Color",
		"skc",
		index,
		stubSaleAttributeLLM{response: `{"attribute_value_id":11,"reasons":["closest semantic match"]}`},
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

	assignments, notes := buildValueAssignments([]string{"未知色"}, "颜色", "Color", "skc", index, nil)
	if assignments != nil {
		t.Fatalf("assignments = %+v, want nil", assignments)
	}
	if len(notes) != 1 {
		t.Fatalf("notes len = %d, want 1", len(notes))
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
