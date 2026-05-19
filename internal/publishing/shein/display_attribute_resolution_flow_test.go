package shein

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type panicDisplayAttributeLLM struct{}

func (panicDisplayAttributeLLM) CreateChatCompletion(_ context.Context, _ *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	panic("unexpected CreateChatCompletion call")
}

func (panicDisplayAttributeLLM) Generate(_ context.Context, _ string) (string, error) {
	panic("unexpected Generate call")
}

func (panicDisplayAttributeLLM) AnalyzeImage(_ context.Context, _, _ string) (string, error) {
	panic("unexpected AnalyzeImage call")
}

func (panicDisplayAttributeLLM) GetDefaultModel() string {
	return "panic"
}

type countingDisplayAttributeLLM struct {
	calls int
}

func (l *countingDisplayAttributeLLM) CreateChatCompletion(_ context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	prompt := ""
	if req != nil && len(req.Messages) > 0 {
		prompt = req.Messages[0].Content
	}
	content, err := l.Generate(context.Background(), prompt)
	if err != nil {
		return nil, err
	}
	return &openaiclient.ChatCompletionResponse{
		Choices: []openaiclient.ChatCompletionChoice{{Message: openaiclient.ChatCompletionMessage{Role: "assistant", Content: content}}},
	}, nil
}

func (l *countingDisplayAttributeLLM) Generate(_ context.Context, prompt string) (string, error) {
	l.calls++
	switch {
	case strings.Contains(prompt, "source_index=0") && strings.Contains(prompt, "source_index=1"):
		return marshalDisplayAttributeTestJSON(map[string]any{
			"selections": []map[string]any{
				{
					"source_index": 0,
					"attribute_id": 160,
					"reasons":      []string{"matched material description to Material"},
				},
				{
					"source_index": 1,
					"attribute_id": 1000546,
					"reasons":      []string{"matched model hint to Product Model"},
				},
			},
		}), nil
	default:
		return marshalDisplayAttributeTestJSON(map[string]any{
			"attribute_id": 0,
			"reasons":      []string{"no safe template field match"},
		}), nil
	}
}

func (l *countingDisplayAttributeLLM) AnalyzeImage(_ context.Context, _, _ string) (string, error) {
	panic("unexpected AnalyzeImage call")
}

func (l *countingDisplayAttributeLLM) GetDefaultModel() string {
	return "counting"
}

type stagedDisplayAttributeLLM struct {
	totalCalls         int
	templateBatchCalls int
}

func (l *stagedDisplayAttributeLLM) CreateChatCompletion(_ context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	prompt := ""
	if req != nil && len(req.Messages) > 0 {
		prompt = req.Messages[0].Content
	}
	content, err := l.Generate(context.Background(), prompt)
	if err != nil {
		return nil, err
	}
	return &openaiclient.ChatCompletionResponse{
		Choices: []openaiclient.ChatCompletionChoice{{Message: openaiclient.ChatCompletionMessage{Role: "assistant", Content: content}}},
	}, nil
}

func (l *stagedDisplayAttributeLLM) Generate(_ context.Context, prompt string) (string, error) {
	l.totalCalls++
	switch {
	case strings.Contains(prompt, "Unresolved SHEIN template attributes:"):
		l.templateBatchCalls++
		return marshalDisplayAttributeTestJSON(map[string]any{
			"selections": []map[string]any{
				{
					"attribute_id":       160,
					"attribute_value_id": 526,
					"reasons":            []string{"polyester aligns with material"},
				},
				{
					"attribute_id":       161,
					"attribute_value_id": 701,
					"reasons":            []string{"outdoor aligns with room"},
				},
			},
		}), nil
	default:
		return marshalDisplayAttributeTestJSON(map[string]any{
			"attribute_id": 0,
			"reasons":      []string{"no safe template field match"},
		}), nil
	}
}

func (l *stagedDisplayAttributeLLM) AnalyzeImage(_ context.Context, _, _ string) (string, error) {
	panic("unexpected AnalyzeImage call")
}

func (l *stagedDisplayAttributeLLM) GetDefaultModel() string {
	return "staged"
}

func marshalDisplayAttributeTestJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal test payload: %v", err))
	}
	return string(data)
}

func TestResolveDisplayAttributesSkipsDuplicateResolvedAttributeIDs(t *testing.T) {
	t.Parallel()

	attributes := []sheinattribute.AttributeInfo{{
		AttributeID:     160,
		AttributeNameEn: "Material",
		AttributeMode:   1,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 526, AttributeValueEn: "Polyester", AttributeValue: "聚酯纤维(涤纶)"},
		},
	}}
	inputs := []common.Attribute{
		{Name: "material", Value: "Polyester"},
		{Name: "Description", Value: "Polyester"},
	}

	resolved, _, _, _, _ := resolveDisplayAttributes(attributes, newDisplayAttributeEvidencePoolFromInputs(inputs), nil)
	if len(resolved) != 1 {
		t.Fatalf("resolved attributes = %#v, want exactly 1 material attribute", resolved)
	}
	if resolved[0].AttributeID != 160 {
		t.Fatalf("attribute id = %d, want 160", resolved[0].AttributeID)
	}
}

func TestResolveDisplayAttributesTraversesTemplatesInsteadOfSourceOrder(t *testing.T) {
	t.Parallel()

	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     1000546,
			AttributeNameEn: "Product Model",
			AttributeMode:   1,
		},
		{
			AttributeID:     160,
			AttributeNameEn: "Material",
			AttributeMode:   1,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 526, AttributeValueEn: "Polyester", AttributeValue: "聚酯纤维(涤纶)"},
			},
		},
	}
	inputs := []common.Attribute{
		{Name: "Description", Value: "Polyester wall clock model MG17701061"},
		{Name: "Product Model", Value: "MG17701061"},
		{Name: "Material", Value: "Polyester"},
	}

	resolved, _, _, _, _ := resolveDisplayAttributes(attributes, newDisplayAttributeEvidencePoolFromInputs(inputs), nil)
	if len(resolved) != 2 {
		t.Fatalf("resolved attributes = %#v, want material and product model", resolved)
	}
	if resolved[0].AttributeID != 1000546 {
		t.Fatalf("first resolved attribute id = %d, want template order product model first", resolved[0].AttributeID)
	}
	if resolved[1].AttributeID != 160 {
		t.Fatalf("second resolved attribute id = %d, want material second", resolved[1].AttributeID)
	}
}

func TestResolveDisplayAttributesExactNameMatchDoesNotCallLLM(t *testing.T) {
	t.Parallel()

	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     160,
			AttributeNameEn: "Material",
			AttributeMode:   1,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 526, AttributeValueEn: "Polyester", AttributeValue: "聚酯纤维(涤纶)"},
			},
		},
	}
	inputs := []common.Attribute{
		{Name: "Material", Value: "Polyester"},
	}

	resolved, _, _, _, _ := resolveDisplayAttributes(attributes, newDisplayAttributeEvidencePoolFromInputs(inputs), panicDisplayAttributeLLM{})
	if len(resolved) != 1 {
		t.Fatalf("resolved attributes = %#v, want 1", resolved)
	}
	if resolved[0].AttributeID != 160 {
		t.Fatalf("attribute id = %d, want 160", resolved[0].AttributeID)
	}
}

func TestAssignDisplayAttributeResolutionInputsUsesExactOnly(t *testing.T) {
	t.Parallel()

	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     160,
			AttributeNameEn: "Material",
			AttributeMode:   1,
		},
		{
			AttributeID:     1000546,
			AttributeNameEn: "Product Model",
			AttributeMode:   1,
		},
	}
	inputs := []common.Attribute{
		{Name: "material description", Value: "Composite board"},
		{Name: "model hint", Value: "MG17701061"},
	}
	llm := &countingDisplayAttributeLLM{}

	assignments, _ := assignDisplayAttributeResolutionInputs(attributes, inputs, inputs, llm)
	if len(assignments) != 0 {
		t.Fatalf("assignments = %#v, want 0 because non-exact field selection should skip LLM", assignments)
	}
	if llm.calls != 0 {
		t.Fatalf("llm calls = %d, want 0", llm.calls)
	}
}

func TestResolveDisplayAttributesCallsValueBatchLLMOnce(t *testing.T) {
	t.Parallel()

	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     160,
			AttributeNameEn: "Material",
			AttributeMode:   1,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 526, AttributeValueEn: "Polyester", AttributeValue: "聚酯纤维(涤纶)"},
			},
		},
		{
			AttributeID:     161,
			AttributeNameEn: "Room",
			AttributeMode:   1,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 701, AttributeValueEn: "Outdoor", AttributeValue: "户外"},
			},
		},
	}
	inputs := []common.Attribute{
		{Name: "Material", Value: "Poly blend"},
		{Name: "Room", Value: "outside patio"},
	}
	llm := &stagedDisplayAttributeLLM{}

	resolved, _, _, _, _ := resolveDisplayAttributes(attributes, newDisplayAttributeEvidencePoolFromInputs(inputs), llm)
	if len(resolved) != 2 {
		t.Fatalf("resolved attributes = %#v, want 2", resolved)
	}
	if llm.templateBatchCalls != 1 {
		t.Fatalf("template batch calls = %d, want 1", llm.templateBatchCalls)
	}
	if llm.totalCalls != 1 {
		t.Fatalf("total llm calls = %d, want exactly 1 template decision call", llm.totalCalls)
	}
}

func TestResolveDisplayAttributesFallsBackToRequiredRepairWhenBatchReturnsEmpty(t *testing.T) {
	t.Parallel()

	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     160,
			AttributeNameEn: "Material",
			AttributeMode:   1,
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 526, AttributeValueEn: "Polyester", AttributeValue: "聚酯纤维(涤纶)"},
				{AttributeValueID: 527, AttributeValueEn: "Cotton", AttributeValue: "棉"},
			},
		},
	}
	inputs := []common.Attribute{
		{Name: "Material", Value: "涤纶"},
		{Name: "Description", Value: "Polyester door curtain"},
	}
	llm := &scriptedAttributeLLM{
		responses: []string{
			`{"selections":[]}`,
			`{"attribute_value_id":526,"reasons":["polyester is directly supported by source evidence"]}`,
		},
	}

	resolved, pending, _, _, notes := resolveDisplayAttributes(attributes, newDisplayAttributeEvidencePoolFromInputs(inputs), llm)
	if len(resolved) != 1 {
		t.Fatalf("resolved = %#v, want 1", resolved)
	}
	if resolved[0].AttributeID != 160 {
		t.Fatalf("attribute id = %d, want 160", resolved[0].AttributeID)
	}
	if resolved[0].AttributeValueID == nil || *resolved[0].AttributeValueID != 526 {
		t.Fatalf("attribute value id = %#v, want 526", resolved[0].AttributeValueID)
	}
	if resolved[0].MatchedBy != "llm_attribute_inference" {
		t.Fatalf("matched by = %q, want llm_attribute_inference", resolved[0].MatchedBy)
	}
	if len(pending) != 0 {
		t.Fatalf("pending = %#v, want none", pending)
	}
	if len(llm.prompts) != 2 {
		t.Fatalf("llm prompt count = %d, want 2", len(llm.prompts))
	}
	if strings.Contains(strings.Join(notes, "\n"), "SHEIN 必填展示属性缺失") {
		t.Fatalf("notes = %#v, want no missing-required note after fallback inference", notes)
	}
}

func TestResolveDisplayAttributesAddsCandidateDiagnosticsWhenUnresolved(t *testing.T) {
	t.Parallel()

	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     160,
			AttributeNameEn: "Material",
			AttributeMode:   1,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 526, AttributeValueEn: "Polyester"},
				{AttributeValueID: 527, AttributeValueEn: "Cotton"},
				{AttributeValueID: 528, AttributeValueEn: "Linen"},
			},
		},
	}
	inputs := []common.Attribute{
		{Name: "Material", Value: "Composite board"},
		{Name: "Description", Value: "Polyester fabric wall clock"},
	}
	llm := &captureAttributeLLM{
		responses: []string{
			`{"selections":[]}`,
		},
	}

	_, _, _, _, notes := resolveDisplayAttributes(attributes, newDisplayAttributeEvidencePoolFromInputs(inputs), llm)
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "SHEIN 普通属性候选诊断") {
		t.Fatalf("notes = %#v, want candidate diagnostics note", notes)
	}
	if !strings.Contains(joined, "Polyester(526)") {
		t.Fatalf("notes = %#v, want narrowed candidate detail", notes)
	}
}

func TestInferMissingDisplayAttributeTextCandidatesAddsEvidenceDiagnostics(t *testing.T) {
	t.Parallel()

	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     1000546,
			AttributeNameEn: "Product Model",
			AttributeMode:   1,
			AttributeStatus: 3,
		},
	}
	inputs := []common.Attribute{
		{Name: "Title", Value: "Wall clock"},
		{Name: "Description", Value: "Composite board decorative clock"},
	}
	llm := &captureAttributeLLM{
		responses: []string{
			`{"value":"","reasons":["no explicit model found"]}`,
		},
	}

	_, notes := inferMissingDisplayAttributeTextCandidates(attributes, inputs, map[int]ResolvedAttribute{}, llm)
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "SHEIN 普通属性文本诊断") {
		t.Fatalf("notes = %#v, want text diagnostics note", notes)
	}
	if !strings.Contains(joined, "Title, Description") {
		t.Fatalf("notes = %#v, want evidence field summary", notes)
	}
}

func TestInferDisplayAttributesTemplateBatchAddsCandidateDiagnostics(t *testing.T) {
	t.Parallel()

	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     160,
			AttributeNameEn: "Material",
			AttributeMode:   1,
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 526, AttributeValueEn: "Polyester"},
				{AttributeValueID: 527, AttributeValueEn: "Cotton"},
			},
		},
	}
	inputs := []common.Attribute{
		{Name: "Description", Value: "Polyester wall clock"},
	}
	llm := &captureAttributeLLM{
		responses: []string{
			`{"selections":[]}`,
		},
	}

	_, notes := inferDisplayAttributesTemplateBatch(attributes, inputs, map[int]ResolvedAttribute{}, llm)
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "SHEIN 普通属性候选诊断") {
		t.Fatalf("notes = %#v, want batch candidate diagnostics", notes)
	}
	if !strings.Contains(joined, "Polyester(526)") {
		t.Fatalf("notes = %#v, want narrowed candidate detail", notes)
	}
}

func TestInferDisplayAttributesTemplateBatchAddsTextDiagnostics(t *testing.T) {
	t.Parallel()

	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     1000546,
			AttributeNameEn: "Product Model",
			AttributeMode:   1,
			AttributeStatus: 3,
		},
	}
	inputs := []common.Attribute{
		{Name: "Title", Value: "Wall clock"},
		{Name: "Description", Value: "Composite board decorative clock"},
	}
	llm := &captureAttributeLLM{
		responses: []string{
			`{"selections":[]}`,
		},
	}

	_, notes := inferDisplayAttributesTemplateBatch(attributes, inputs, map[int]ResolvedAttribute{}, llm)
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "SHEIN 普通属性文本诊断") {
		t.Fatalf("notes = %#v, want text diagnostics note", notes)
	}
	if !strings.Contains(joined, "Title, Description") {
		t.Fatalf("notes = %#v, want evidence field summary", notes)
	}
}

func TestInferDisplayAttributesTemplateBatchToleratesTrailingMalformedJSON(t *testing.T) {
	t.Parallel()

	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     1001519,
			AttributeNameEn: "Product Features",
			AttributeMode:   1,
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 8790846, AttributeValueEn: "Printing"},
			},
		},
		{
			AttributeID:     77,
			AttributeNameEn: "Season",
			AttributeMode:   1,
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 1601, AttributeValueEn: "All"},
			},
		},
	}
	inputs := []common.Attribute{
		{Name: "production_process", Value: "UV打印"},
		{Name: "design_area", Value: "单面印制"},
	}
	llm := &captureAttributeLLM{
		responses: []string{
			`{"selections":[{"attribute_id":1001519,"attribute_value_id":8790846,"reasons":["printing is directly supported by the source evidence"]}]}{"attribute_id":77,"attribute_value_id":1601,"reasons":["all-season is broad and safe"]}`,
		},
	}

	resolved, notes := inferDisplayAttributesTemplateBatch(attributes, inputs, map[int]ResolvedAttribute{}, llm)
	if len(resolved) != 1 {
		t.Fatalf("resolved = %#v, want first valid selection recovered", resolved)
	}
	if resolved[0].AttributeID != 1001519 {
		t.Fatalf("attribute id = %d, want 1001519", resolved[0].AttributeID)
	}
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "printing is directly supported") {
		t.Fatalf("notes = %#v, want recovered reasons", notes)
	}
}
