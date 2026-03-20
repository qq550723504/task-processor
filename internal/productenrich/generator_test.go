package productenrich

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
)

// mockVariantGenerator 用于测试 GenerateJSON 中的变体生成分支
type mockVariantGenerator struct {
	specs    *ProductSpecs
	variants []ProductVariant
	specsErr error
	varErr   error
}

func (m *mockVariantGenerator) GenerateSpecs(_ context.Context, _ *ProductAnalysis) (*ProductSpecs, error) {
	return m.specs, m.specsErr
}
func (m *mockVariantGenerator) GenerateVariants(_ context.Context, _ *ProductAnalysis) ([]ProductVariant, error) {
	return m.variants, m.varErr
}
func (m *mockVariantGenerator) ExtractDimensions(_ context.Context, _ string) (*Dimensions, error) {
	return nil, nil
}
func (m *mockVariantGenerator) ExtractWeight(_ context.Context, _ string) (*Weight, error) {
	return nil, nil
}

func newTestJSONGenerator(llmResp string, llmErr error) *jsonGenerator {
	mgr := newMockLLMManager(llmResp)
	if llmErr != nil {
		mgr.def.err = llmErr
		for _, c := range mgr.clients {
			c.err = llmErr
		}
	}
	return &jsonGenerator{
		logger:     logrus.New(),
		llmManager: mgr,
	}
}

// --- GenerateJSON ---

func TestGenerateJSON_NilAnalysis(t *testing.T) {
	g := newTestJSONGenerator(`{"title":"x"}`, nil)
	_, err := g.GenerateJSON(context.Background(), nil, nil, false)
	if err == nil {
		t.Fatal("expected error for nil analysis")
	}
}

func TestGenerateJSON_FullStrategy_WithVariants(t *testing.T) {
	resp := `{"title":"Widget","category":["Electronics"],"selling_points":["fast"],"seo_keywords":["widget"],"description":"A widget."}`
	g := newTestJSONGenerator(resp, nil)

	specs := &ProductSpecs{Technical: map[string]string{"color": "red"}}
	variants := []ProductVariant{{SKU: "W-RED", IsDefault: true}}
	vg := &mockVariantGenerator{specs: specs, variants: variants}

	result, err := g.GenerateJSON(context.Background(), &ProductAnalysis{}, vg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Specifications == nil {
		t.Error("expected specifications to be populated")
	}
	if len(result.Variants) == 0 {
		t.Error("expected variants to be populated (skipVariants=false)")
	}
}

func TestGenerateJSON_BasicStrategy_SkipVariants(t *testing.T) {
	resp := `{"title":"Widget","category":["Electronics"],"selling_points":["fast"],"seo_keywords":["widget"],"description":"A widget."}`
	g := newTestJSONGenerator(resp, nil)

	specs := &ProductSpecs{Technical: map[string]string{"size": "M"}}
	variants := []ProductVariant{{SKU: "W-M"}}
	vg := &mockVariantGenerator{specs: specs, variants: variants}

	result, err := g.GenerateJSON(context.Background(), &ProductAnalysis{}, vg, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Specifications == nil {
		t.Error("expected specifications to be populated")
	}
	if len(result.Variants) != 0 {
		t.Error("expected variants to be empty (skipVariants=true)")
	}
}

func TestGenerateJSON_MinimalStrategy_NoVariantGen(t *testing.T) {
	resp := `{"title":"Widget","category":["Electronics"],"selling_points":["fast"],"seo_keywords":["widget"],"description":"A widget."}`
	g := newTestJSONGenerator(resp, nil)

	result, err := g.GenerateJSON(context.Background(), &ProductAnalysis{}, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Specifications != nil {
		t.Error("expected no specifications when variantGen is nil")
	}
	if len(result.Variants) != 0 {
		t.Error("expected no variants when variantGen is nil")
	}
}

func TestGenerateJSON_LLMFail_FallsBackToAnalysis(t *testing.T) {
	g := newTestJSONGenerator("", errors.New("llm down"))

	analysis := &ProductAnalysis{
		Representation: &ProductRepresentation{
			ProductType: "Gadget",
			Features:    []string{"durable"},
			Attributes:  map[string]string{"color": "blue"},
		},
	}
	result, err := g.GenerateJSON(context.Background(), analysis, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Gadget" {
		t.Errorf("Title = %q, want %q", result.Title, "Gadget")
	}
}

func TestGenerateJSON_SpecsError_ContinuesGracefully(t *testing.T) {
	resp := `{"title":"Widget","category":["Electronics"],"selling_points":["fast"],"seo_keywords":["widget"],"description":"A widget."}`
	g := newTestJSONGenerator(resp, nil)
	vg := &mockVariantGenerator{specsErr: errors.New("specs failed")}

	result, err := g.GenerateJSON(context.Background(), &ProductAnalysis{}, vg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result even when specs fail")
	}
}

// --- generateWithLLM markdown 清理 ---

func TestGenerateWithLLM_StripsMarkdownCodeBlock(t *testing.T) {
	cases := []struct {
		name string
		resp string
	}{
		{"json fence", "```json\n{\"title\":\"T\",\"category\":[],\"selling_points\":[],\"seo_keywords\":[],\"description\":\"d\"}\n```"},
		{"plain fence", "```\n{\"title\":\"T\",\"category\":[],\"selling_points\":[],\"seo_keywords\":[],\"description\":\"d\"}\n```"},
		{"no fence", "{\"title\":\"T\",\"category\":[],\"selling_points\":[],\"seo_keywords\":[],\"description\":\"d\"}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestJSONGenerator(tc.resp, nil)
			result, err := g.generateWithLLM(context.Background(), &ProductAnalysis{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Title != "T" {
				t.Errorf("Title = %q, want %q", result.Title, "T")
			}
		})
	}
}

func TestGenerateWithLLM_InvalidJSON_ReturnsError(t *testing.T) {
	g := newTestJSONGenerator("not json", nil)
	_, err := g.generateWithLLM(context.Background(), &ProductAnalysis{})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

// --- fallbackFromAnalysis ---

func TestFallbackFromAnalysis_RepresentationFields(t *testing.T) {
	g := newTestJSONGenerator("", nil)
	analysis := &ProductAnalysis{
		Representation: &ProductRepresentation{
			ProductType: "Lamp",
			Features:    []string{"bright", "energy-saving"},
			Attributes:  map[string]string{"wattage": "10W"},
		},
	}
	result := g.fallbackFromAnalysis(analysis)
	if result.Title != "Lamp" {
		t.Errorf("Title = %q, want %q", result.Title, "Lamp")
	}
	if len(result.SellingPoints) != 2 {
		t.Errorf("SellingPoints len = %d, want 2", len(result.SellingPoints))
	}
	if result.Attributes["wattage"] != "10W" {
		t.Errorf("Attributes[wattage] = %q, want %q", result.Attributes["wattage"], "10W")
	}
}

func TestFallbackFromAnalysis_TextAttributesFallback(t *testing.T) {
	g := newTestJSONGenerator("", nil)
	// Representation 为空，应从 TextAttributes 取 Title 和 SellingPoints
	analysis := &ProductAnalysis{
		TextAttributes: &TextAttributes{
			Title:         "Chair",
			SellingPoints: []string{"comfortable"},
		},
	}
	result := g.fallbackFromAnalysis(analysis)
	if result.Title != "Chair" {
		t.Errorf("Title = %q, want %q", result.Title, "Chair")
	}
	if len(result.SellingPoints) != 1 {
		t.Errorf("SellingPoints len = %d, want 1", len(result.SellingPoints))
	}
}

func TestFallbackFromAnalysis_EmptyAnalysis_DefaultTitle(t *testing.T) {
	g := newTestJSONGenerator("", nil)
	result := g.fallbackFromAnalysis(&ProductAnalysis{})
	if result.Title != "Product" {
		t.Errorf("Title = %q, want %q", result.Title, "Product")
	}
	if len(result.Category) == 0 {
		t.Error("expected default category")
	}
}
