package enrich_test

import (
	"context"
	"errors"
	"testing"

	productenrich "task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"

	"github.com/sirupsen/logrus"
)

type mockVariantGenerator struct {
	specs    *productenrich.ProductSpecs
	variants []productenrich.ProductVariant
	specsErr error
	varErr   error
}

func (m *mockVariantGenerator) GenerateSpecs(_ context.Context, _ *productenrich.ProductAnalysis) (*productenrich.ProductSpecs, error) {
	return m.specs, m.specsErr
}

func (m *mockVariantGenerator) GenerateVariants(_ context.Context, _ *productenrich.ProductAnalysis) ([]productenrich.ProductVariant, error) {
	return m.variants, m.varErr
}

func (m *mockVariantGenerator) ExtractDimensions(_ context.Context, _ string) (*productenrich.Dimensions, error) {
	return nil, nil
}

func (m *mockVariantGenerator) ExtractWeight(_ context.Context, _ string) (*productenrich.Weight, error) {
	return nil, nil
}

func newTestJSONGenerator(t *testing.T, llmResp string, llmErr error) productenrich.JSONGenerator {
	t.Helper()

	mgr := newMockLLMManager(llmResp)
	if llmErr != nil {
		mgr.def.err = llmErr
		for _, c := range mgr.clients {
			c.err = llmErr
		}
	}

	generator, err := productenrichenrich.NewJSONGenerator(logrus.New(), mgr)
	if err != nil {
		t.Fatalf("NewJSONGenerator() error = %v", err)
	}

	return generator
}

func TestGenerateJSON_NilAnalysis(t *testing.T) {
	g := newTestJSONGenerator(t, `{"title":"x"}`, nil)
	_, err := g.GenerateJSON(context.Background(), nil, nil, false)
	if err == nil {
		t.Fatal("expected error for nil analysis")
	}
}

func TestGenerateJSON_FullStrategy_WithVariants(t *testing.T) {
	resp := `{"title":"Widget","category":["Electronics"],"selling_points":["fast"],"seo_keywords":["widget"],"description":"A widget."}`
	g := newTestJSONGenerator(t, resp, nil)

	specs := &productenrich.ProductSpecs{Technical: map[string]string{"color": "red"}}
	variants := []productenrich.ProductVariant{{SKU: "W-RED", IsDefault: true}}
	vg := &mockVariantGenerator{specs: specs, variants: variants}

	result, err := g.GenerateJSON(context.Background(), &productenrich.ProductAnalysis{}, vg, false)
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
	g := newTestJSONGenerator(t, resp, nil)

	specs := &productenrich.ProductSpecs{Technical: map[string]string{"size": "M"}}
	variants := []productenrich.ProductVariant{{SKU: "W-M"}}
	vg := &mockVariantGenerator{specs: specs, variants: variants}

	result, err := g.GenerateJSON(context.Background(), &productenrich.ProductAnalysis{}, vg, true)
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
	g := newTestJSONGenerator(t, resp, nil)

	result, err := g.GenerateJSON(context.Background(), &productenrich.ProductAnalysis{}, nil, false)
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
	g := newTestJSONGenerator(t, "", errors.New("llm down"))

	analysis := &productenrich.ProductAnalysis{
		Representation: &productenrich.ProductRepresentation{
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
	if result.Description == "" {
		t.Error("expected fallback path to populate description")
	}
}

func TestGenerateJSON_SpecsError_ContinuesGracefully(t *testing.T) {
	resp := `{"title":"Widget","category":["Electronics"],"selling_points":["fast"],"seo_keywords":["widget"],"description":"A widget."}`
	g := newTestJSONGenerator(t, resp, nil)
	vg := &mockVariantGenerator{specsErr: errors.New("specs failed")}

	result, err := g.GenerateJSON(context.Background(), &productenrich.ProductAnalysis{}, vg, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result even when specs fail")
	}
}

func TestGenerateJSON_StripsMarkdownCodeBlock(t *testing.T) {
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
			g := newTestJSONGenerator(t, tc.resp, nil)
			result, err := g.GenerateJSON(context.Background(), &productenrich.ProductAnalysis{}, nil, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Title != "T" {
				t.Errorf("Title = %q, want %q", result.Title, "T")
			}
		})
	}
}

func TestGenerateJSON_InvalidJSON_FallsBack(t *testing.T) {
	g := newTestJSONGenerator(t, "not json", nil)
	analysis := &productenrich.ProductAnalysis{
		Representation: &productenrich.ProductRepresentation{
			ProductType: "Lamp",
			Features:    []string{"bright", "energy-saving"},
			Attributes:  map[string]string{"wattage": "10W"},
		},
	}

	result, err := g.GenerateJSON(context.Background(), analysis, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

func TestGenerateJSON_FallbackUsesTextAttributesWhenRepresentationMissing(t *testing.T) {
	g := newTestJSONGenerator(t, "not json", nil)
	analysis := &productenrich.ProductAnalysis{
		TextAttributes: &productenrich.TextAttributes{
			Title:         "Chair",
			SellingPoints: []string{"comfortable"},
		},
	}

	result, err := g.GenerateJSON(context.Background(), analysis, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Chair" {
		t.Errorf("Title = %q, want %q", result.Title, "Chair")
	}
	if len(result.SellingPoints) != 1 {
		t.Errorf("SellingPoints len = %d, want 1", len(result.SellingPoints))
	}
	if result.Description == "" {
		t.Error("expected description to be populated from fallback")
	}
}

func TestGenerateJSON_FallbackUsesDefaultTitleForEmptyAnalysis(t *testing.T) {
	g := newTestJSONGenerator(t, "not json", nil)
	result, err := g.GenerateJSON(context.Background(), &productenrich.ProductAnalysis{}, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Product" {
		t.Errorf("Title = %q, want %q", result.Title, "Product")
	}
	if len(result.Category) == 0 {
		t.Error("expected default category")
	}
}

func TestNewJSONGenerator_Validation(t *testing.T) {
	if _, err := productenrichenrich.NewJSONGenerator(nil, newMockLLMManager("{}")); err == nil {
		t.Error("expected error for nil logger")
	}
	if _, err := productenrichenrich.NewJSONGenerator(logrus.New(), nil); err == nil {
		t.Error("expected error for nil llm manager")
	}
}
