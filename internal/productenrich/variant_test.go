package productenrich

import (
	"context"
	"errors"
	"testing"
)

func newTestVariantGenerator(llmResp string, llmErr error) *variantGenerator {
	mgr := newMockLLMManager(llmResp)
	if llmErr != nil {
		mgr.def.err = llmErr
		for _, c := range mgr.clients {
			c.err = llmErr
		}
	}
	return &variantGenerator{llmManager: mgr}
}

// --- GenerateSpecs ---

func TestGenerateSpecs_NilAnalysis_ReturnsError(t *testing.T) {
	v := newTestVariantGenerator("{}", nil)
	_, err := v.GenerateSpecs(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil analysis")
	}
}

func TestGenerateSpecs_ValidJSON_ParsesSpecs(t *testing.T) {
	resp := `{"dimensions":{"length":10,"width":5,"height":3,"unit":"cm"},"weight":{"value":0.5,"unit":"kg"}}`
	v := newTestVariantGenerator(resp, nil)

	specs, err := v.GenerateSpecs(context.Background(), &ProductAnalysis{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if specs == nil {
		t.Fatal("expected non-nil specs")
	}
	if specs.Dimensions == nil {
		t.Error("expected Dimensions to be populated")
	}
	if specs.Dimensions.Length != 10 {
		t.Errorf("Length = %.1f, want 10", specs.Dimensions.Length)
	}
}

func TestGenerateSpecs_InvalidJSON_ReturnsNil(t *testing.T) {
	// JSON 解析失败时应返回 nil specs（不是 error）
	v := newTestVariantGenerator("not json", nil)

	specs, err := v.GenerateSpecs(context.Background(), &ProductAnalysis{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if specs != nil {
		t.Error("expected nil specs when JSON parse fails")
	}
}

func TestGenerateSpecs_LLMError_ReturnsError(t *testing.T) {
	v := newTestVariantGenerator("", errors.New("llm down"))

	_, err := v.GenerateSpecs(context.Background(), &ProductAnalysis{})
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
}

// --- GenerateVariants ---

func TestGenerateVariants_NilAnalysis_ReturnsError(t *testing.T) {
	v := newTestVariantGenerator("[]", nil)
	_, err := v.GenerateVariants(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil analysis")
	}
}

func TestGenerateVariants_ValidJSON_ParsesVariants(t *testing.T) {
	resp := `[{"sku":"PROD-RED","attributes":{"color":"red"},"stock":100,"is_default":true}]`
	v := newTestVariantGenerator(resp, nil)

	variants, err := v.GenerateVariants(context.Background(), &ProductAnalysis{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(variants) != 1 {
		t.Fatalf("variants len = %d, want 1", len(variants))
	}
	if variants[0].SKU != "PROD-RED" {
		t.Errorf("SKU = %q, want PROD-RED", variants[0].SKU)
	}
	if !variants[0].IsDefault {
		t.Error("expected IsDefault = true")
	}
}

func TestGenerateVariants_InvalidJSON_ReturnsDefaultVariant(t *testing.T) {
	// JSON 解析失败时应返回一个默认变体
	v := newTestVariantGenerator("not json", nil)

	variants, err := v.GenerateVariants(context.Background(), &ProductAnalysis{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(variants) == 0 {
		t.Fatal("expected at least one default variant on parse failure")
	}
	if variants[0].SKU != "DEFAULT-001" {
		t.Errorf("SKU = %q, want DEFAULT-001", variants[0].SKU)
	}
}

func TestGenerateVariants_NoDefaultSet_FirstBecomesDefault(t *testing.T) {
	// 返回的变体都没有 is_default=true，应自动将第一个设为 default
	resp := `[{"sku":"A","attributes":{},"is_default":false},{"sku":"B","attributes":{},"is_default":false}]`
	v := newTestVariantGenerator(resp, nil)

	variants, err := v.GenerateVariants(context.Background(), &ProductAnalysis{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !variants[0].IsDefault {
		t.Error("expected first variant to be set as default")
	}
}

func TestGenerateVariants_LLMError_ReturnsError(t *testing.T) {
	v := newTestVariantGenerator("", errors.New("llm down"))

	_, err := v.GenerateVariants(context.Background(), &ProductAnalysis{})
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
}

// --- ExtractDimensions ---

func TestExtractDimensions_EmptyText_ReturnsError(t *testing.T) {
	v := newTestVariantGenerator("{}", nil)
	_, err := v.ExtractDimensions(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestExtractDimensions_ValidJSON_ParsesDimensions(t *testing.T) {
	resp := `{"length":20,"width":10,"height":5,"unit":"cm"}`
	v := newTestVariantGenerator(resp, nil)

	dims, err := v.ExtractDimensions(context.Background(), "20x10x5 cm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dims == nil {
		t.Fatal("expected non-nil dimensions")
	}
	if dims.Length != 20 {
		t.Errorf("Length = %.1f, want 20", dims.Length)
	}
}

func TestExtractDimensions_NullResponse_ReturnsEmptyDimensions(t *testing.T) {
	// LLM 返回 "null" 时应返回零值 Dimensions（不报错）
	v := newTestVariantGenerator("null", nil)

	dims, err := v.ExtractDimensions(context.Background(), "no dimensions here")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// null 响应时 extractWithLLM 不写入 dest，返回零值
	if dims == nil {
		t.Fatal("expected non-nil dimensions (zero value)")
	}
}

// --- ExtractWeight ---

func TestExtractWeight_EmptyText_ReturnsError(t *testing.T) {
	v := newTestVariantGenerator("{}", nil)
	_, err := v.ExtractWeight(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestExtractWeight_ValidJSON_ParsesWeight(t *testing.T) {
	resp := `{"value":1.5,"unit":"kg"}`
	v := newTestVariantGenerator(resp, nil)

	w, err := v.ExtractWeight(context.Background(), "1.5kg product")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil weight")
	}
	if w.Value != 1.5 {
		t.Errorf("Value = %.1f, want 1.5", w.Value)
	}
	if w.Unit != "kg" {
		t.Errorf("Unit = %q, want kg", w.Unit)
	}
}

func TestExtractWeight_InvalidJSON_ReturnsZeroWeight(t *testing.T) {
	v := newTestVariantGenerator("not json", nil)

	w, err := v.ExtractWeight(context.Background(), "some text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil weight (zero value)")
	}
}

// --- NewVariantGenerator ---

func TestNewVariantGenerator_NilLLM_ReturnsError(t *testing.T) {
	_, err := NewVariantGenerator(nil)
	if err == nil {
		t.Fatal("expected error for nil LLM manager")
	}
}

func TestNewVariantGenerator_ValidLLM_Succeeds(t *testing.T) {
	mgr := newMockLLMManager("{}")
	_, err := NewVariantGenerator(mgr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
