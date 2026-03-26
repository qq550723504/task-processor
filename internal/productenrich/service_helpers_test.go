package productenrich

import (
	"context"
	"strings"
	"testing"
)

// --- buildRejectionMessage ---

func TestBuildRejectionMessage_WithRequiredAndOptional(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	validation := &ValidationResult{QualityScore: 25.5}
	suggestion := &EnhancementSuggestion{
		RequiredActions:  []string{"添加图片", "补充描述"},
		OptionalActions:  []string{"添加视频"},
		EstimatedQuality: "中等质量",
	}

	msg := svc.buildRejectionMessage(validation, suggestion)

	if !strings.Contains(msg, "25.50") {
		t.Errorf("message should contain quality score, got: %s", msg)
	}
	if !strings.Contains(msg, "添加图片") {
		t.Errorf("message should contain required action, got: %s", msg)
	}
	if !strings.Contains(msg, "添加视频") {
		t.Errorf("message should contain optional action, got: %s", msg)
	}
	if !strings.Contains(msg, "中等质量") {
		t.Errorf("message should contain estimated quality, got: %s", msg)
	}
}

func TestBuildRejectionMessage_NoActions(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	validation := &ValidationResult{QualityScore: 10.0}
	suggestion := &EnhancementSuggestion{}

	msg := svc.buildRejectionMessage(validation, suggestion)

	if !strings.Contains(msg, "10.00") {
		t.Errorf("message should contain quality score, got: %s", msg)
	}
	// 无 actions 时不应包含编号列表
	if strings.Contains(msg, "1.") {
		t.Errorf("message should not contain numbered list when no actions, got: %s", msg)
	}
}

// --- validateResult ---

func TestValidateResult_NilValidator_DoesNothing(t *testing.T) {
	// 无 ResultValidator 时应静默跳过，不 panic
	svc, _ := newSvcWithSubmitter(t, nil)
	task := &Task{ID: "t1", Request: &GenerateRequest{}}
	// 不应 panic
	svc.validateResult(context.Background(), task, &ParsedInput{}, &ProductJSON{Title: "ok"})
}

func TestValidateResult_WithValidator_LogsIssues(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	svc.resultValidator = &mockResultValidator{
		result: &ResultValidation{
			IsValid: false,
			Issues: []ValidationIssue{
				{Field: "title", Severity: "warning", Message: "title too short"},
			},
		},
	}
	task := &Task{ID: "t1", Request: &GenerateRequest{}}
	// 有 issues 时应正常执行（不报错，只记录日志）
	svc.validateResult(context.Background(), task, &ParsedInput{}, &ProductJSON{})
}

func TestValidateResult_ValidatorError_ContinuesSilently(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	svc.resultValidator = &mockResultValidator{
		err: errTestValidation,
	}
	task := &Task{ID: "t1", Request: &GenerateRequest{}}
	// validator 报错时不应向上传播
	svc.validateResult(context.Background(), task, &ParsedInput{}, &ProductJSON{})
}

// --- analyzeProduct ---

func TestAnalyzeProduct_WithUnderstanding_CallsAnalyze(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	expected := &ProductAnalysis{
		Representation: &ProductRepresentation{ProductType: "Widget"},
	}
	svc.productUnderstanding = &mockProductUnderstanding{result: expected}
	task := &Task{ID: "t1", Request: &GenerateRequest{}}

	result, err := svc.analyzeProduct(context.Background(), task, &ParsedInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Representation.ProductType != "Widget" {
		t.Errorf("ProductType = %q, want Widget", result.Representation.ProductType)
	}
}

func TestAnalyzeProduct_NoUnderstanding_ReturnsSimpleAnalysis(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	// productUnderstanding 为 nil
	task := &Task{ID: "t1", Request: &GenerateRequest{}}

	result, err := svc.analyzeProduct(context.Background(), task, &ParsedInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Representation == nil {
		t.Fatal("expected non-nil Representation for simple analysis")
	}
	if result.Representation.ProductType != "Unknown Product" {
		t.Errorf("ProductType = %q, want Unknown Product", result.Representation.ProductType)
	}
}

// --- parseInput ---

func TestParseInput_WithParser_CallsParser(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	expected := &ParsedInput{
		Images: []string{"http://img.jpg"},
		Text:   "parsed text",
	}
	svc.inputParser = &mockInputParser{result: expected}
	task := &Task{ID: "t1", Request: &GenerateRequest{Text: "raw"}}

	result, err := svc.parseInput(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "parsed text" {
		t.Errorf("Text = %q, want parsed text", result.Text)
	}
}

func TestParseInput_NoParser_UsesRequestFields(t *testing.T) {
	svc, _ := newSvcWithSubmitter(t, nil)
	task := &Task{
		ID: "t1",
		Request: &GenerateRequest{
			ImageURLs: []string{"http://img.jpg"},
			Text:      "direct text",
		},
	}

	result, err := svc.parseInput(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "direct text" {
		t.Errorf("Text = %q, want direct text", result.Text)
	}
	if len(result.Images) != 1 {
		t.Errorf("Images len = %d, want 1", len(result.Images))
	}
}

// errTestValidation 测试用错误
var errTestValidation = errorf("validation error")

type stringError string

func (e stringError) Error() string { return string(e) }

func errorf(s string) error { return stringError(s) }
