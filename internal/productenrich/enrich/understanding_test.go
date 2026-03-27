package enrich_test

import (
	"context"
	"errors"
	"testing"

	productenrich "task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
)

func newTestUnderstanding(t *testing.T, llmResp string, llmErr error) productenrich.ProductUnderstanding {
	t.Helper()

	mgr := newMockLLMManager(llmResp)
	if llmErr != nil {
		mgr.def.err = llmErr
		for _, c := range mgr.clients {
			c.err = llmErr
		}
	}

	understanding, err := productenrichenrich.NewProductUnderstanding(mgr)
	if err != nil {
		t.Fatalf("NewProductUnderstanding() error = %v", err)
	}

	return understanding
}

func TestAnalyzeProduct_NilInput(t *testing.T) {
	p := newTestUnderstanding(t, "{}", nil)
	_, err := p.AnalyzeProduct(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil input")
	}
}

func TestAnalyzeProduct_NoImagesNoText(t *testing.T) {
	p := newTestUnderstanding(t, "{}", nil)
	result, err := p.AnalyzeProduct(context.Background(), &productenrich.ParsedInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Representation != nil {
		t.Error("expected nil Representation when no images and no text")
	}
}

func TestAnalyzeProduct_WithText_ExtractsAttributes(t *testing.T) {
	resp := `{"title":"Desk","attributes":{"material":"wood"},"selling_points":["sturdy"]}`
	p := newTestUnderstanding(t, resp, nil)

	result, err := p.AnalyzeProduct(context.Background(), &productenrich.ParsedInput{Text: "A wooden desk"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TextAttributes == nil {
		t.Fatal("expected TextAttributes to be populated")
	}
	if result.TextAttributes.Title != "Desk" {
		t.Errorf("Title = %q, want %q", result.TextAttributes.Title, "Desk")
	}
}

func TestAnalyzeProduct_MultipleImages_MergesAttributes(t *testing.T) {
	responses := []string{
		`{"color":"","material":"metal","scene":"studio","usage":"decoration"}`,
		`{"color":"silver","material":"metal","scene":"studio","usage":"decoration"}`,
	}
	p, err := productenrichenrich.NewProductUnderstanding(&rotatingMockClient{responses: responses})
	if err != nil {
		t.Fatalf("NewProductUnderstanding() error = %v", err)
	}

	result, err := p.AnalyzeProduct(context.Background(), &productenrich.ParsedInput{
		Images: []string{"img1.jpg", "img2.jpg"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ImageAttributes == nil {
		t.Fatal("expected ImageAttributes")
	}
	if result.ImageAttributes.Color != "silver" {
		t.Errorf("Color = %q, want %q", result.ImageAttributes.Color, "silver")
	}
}

func TestAnalyzeProduct_ScrapedData_MergesSellingPoints(t *testing.T) {
	responses := []string{
		`{"title":"T","attributes":{},"selling_points":["point-a"]}`,
		`{"title":"T","attributes":{},"selling_points":["point-b"]}`,
		`{"product_type":"T","attributes":{},"features":[]}`,
	}
	p, err := productenrichenrich.NewProductUnderstanding(&rotatingMockClient{responses: responses})
	if err != nil {
		t.Fatalf("NewProductUnderstanding() error = %v", err)
	}

	result, err := p.AnalyzeProduct(context.Background(), &productenrich.ParsedInput{
		Text:        "main text",
		ScrapedData: &productenrich.ScrapedData{Description: "scraped text"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TextAttributes == nil {
		t.Fatal("expected TextAttributes")
	}
	if len(result.TextAttributes.SellingPoints) != 2 {
		t.Errorf("SellingPoints len = %d, want 2", len(result.TextAttributes.SellingPoints))
	}
}

func TestAnalyzeProduct_ScrapedData_DeduplicatesSellingPoints(t *testing.T) {
	responses := []string{
		`{"title":"T","attributes":{},"selling_points":["same-point"]}`,
		`{"title":"T","attributes":{},"selling_points":["same-point"]}`,
		`{"product_type":"T","attributes":{},"features":[]}`,
	}
	p, err := productenrichenrich.NewProductUnderstanding(&rotatingMockClient{responses: responses})
	if err != nil {
		t.Fatalf("NewProductUnderstanding() error = %v", err)
	}

	result, err := p.AnalyzeProduct(context.Background(), &productenrich.ParsedInput{
		Text:        "main text",
		ScrapedData: &productenrich.ScrapedData{Description: "scraped text"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.TextAttributes.SellingPoints) != 1 {
		t.Errorf("SellingPoints len = %d, want 1 (deduped)", len(result.TextAttributes.SellingPoints))
	}
}

func TestAnalyzeProduct_ScrapedData_MergesAttributesWithNilBaseMap(t *testing.T) {
	responses := []string{
		`{"title":"T","selling_points":["point-a"]}`,
		`{"title":"T","attributes":{"material":"ABS"},"selling_points":["point-b"]}`,
		`{"product_type":"T","attributes":{"material":"ABS"},"features":["point-a","point-b"]}`,
	}
	p, err := productenrichenrich.NewProductUnderstanding(&rotatingMockClient{responses: responses})
	if err != nil {
		t.Fatalf("NewProductUnderstanding() error = %v", err)
	}

	result, err := p.AnalyzeProduct(context.Background(), &productenrich.ParsedInput{
		Text:        "main text",
		ScrapedData: &productenrich.ScrapedData{Description: "scraped text"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TextAttributes == nil {
		t.Fatal("expected TextAttributes")
	}
	if result.TextAttributes.Attributes["material"] != "ABS" {
		t.Errorf("Attributes[material] = %q, want ABS", result.TextAttributes.Attributes["material"])
	}
}

func TestAnalyzeImage_EmptyPath_ReturnsError(t *testing.T) {
	p := newTestUnderstanding(t, "{}", nil)
	_, err := p.AnalyzeImage(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty image path")
	}
}

func TestAnalyzeImage_ValidJSON_ParsesAttributes(t *testing.T) {
	resp := `{"color":"red","material":"plastic","scene":"outdoor","usage":"sports"}`
	p := newTestUnderstanding(t, resp, nil)

	attr, err := p.AnalyzeImage(context.Background(), "http://example.com/img.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attr.Color != "red" {
		t.Errorf("Color = %q, want %q", attr.Color, "red")
	}
	if attr.Material != "plastic" {
		t.Errorf("Material = %q, want %q", attr.Material, "plastic")
	}
}

func TestAnalyzeImage_InvalidJSON_FallsBackToUnknown(t *testing.T) {
	p := newTestUnderstanding(t, "not json", nil)

	attr, err := p.AnalyzeImage(context.Background(), "http://example.com/img.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attr.Color != "unknown" {
		t.Errorf("Color = %q, want %q", attr.Color, "unknown")
	}
}

func TestAnalyzeImage_LLMError_ReturnsError(t *testing.T) {
	p := newTestUnderstanding(t, "", errors.New("vision api down"))

	_, err := p.AnalyzeImage(context.Background(), "http://example.com/img.jpg")
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
}

func TestExtractTextAttributes_EmptyText_ReturnsError(t *testing.T) {
	p := newTestUnderstanding(t, "{}", nil)
	_, err := p.ExtractTextAttributes(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestExtractTextAttributes_ValidJSON_ParsesFields(t *testing.T) {
	resp := `{"title":"Lamp","attributes":{"wattage":"10W"},"selling_points":["bright","efficient"]}`
	p := newTestUnderstanding(t, resp, nil)

	attr, err := p.ExtractTextAttributes(context.Background(), "A 10W LED lamp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attr.Title != "Lamp" {
		t.Errorf("Title = %q, want %q", attr.Title, "Lamp")
	}
	if attr.Attributes["wattage"] != "10W" {
		t.Errorf("Attributes[wattage] = %q, want %q", attr.Attributes["wattage"], "10W")
	}
	if len(attr.SellingPoints) != 2 {
		t.Errorf("SellingPoints len = %d, want 2", len(attr.SellingPoints))
	}
}

func TestExtractTextAttributes_InvalidJSON_FallsBackToTruncatedTitle(t *testing.T) {
	p := newTestUnderstanding(t, "not json", nil)

	text := "A very long product description that exceeds fifty characters in total length"
	attr, err := p.ExtractTextAttributes(context.Background(), text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len([]rune(attr.Title)) > 50 {
		t.Errorf("Title too long: %d chars", len([]rune(attr.Title)))
	}
	if attr.Attributes == nil {
		t.Error("expected non-nil Attributes map")
	}
}

func TestFuseMultimodal_ValidJSON_ParsesRepresentation(t *testing.T) {
	resp := `{"product_type":"Chair","attributes":{"material":"wood"},"features":["comfortable","durable"]}`
	p := newTestUnderstanding(t, resp, nil)

	imageAttr := &productenrich.ImageAttributes{Color: "brown", Material: "wood"}
	textAttr := &productenrich.TextAttributes{Title: "Chair", SellingPoints: []string{"comfortable"}}

	rep, err := p.FuseMultimodal(context.Background(), imageAttr, textAttr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.ProductType != "Chair" {
		t.Errorf("ProductType = %q, want %q", rep.ProductType, "Chair")
	}
	if len(rep.Features) != 2 {
		t.Errorf("Features len = %d, want 2", len(rep.Features))
	}
}

func TestFuseMultimodal_InvalidJSON_FallsBackToAttributeMerge(t *testing.T) {
	p := newTestUnderstanding(t, "not json", nil)

	imageAttr := &productenrich.ImageAttributes{Color: "red", Material: "plastic"}
	textAttr := &productenrich.TextAttributes{
		Title:         "Ball",
		Attributes:    map[string]string{"size": "large"},
		SellingPoints: []string{"bouncy"},
	}

	rep, err := p.FuseMultimodal(context.Background(), imageAttr, textAttr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Attributes["color"] != "red" {
		t.Errorf("Attributes[color] = %q, want %q", rep.Attributes["color"], "red")
	}
	if rep.ProductType != "Ball" {
		t.Errorf("ProductType = %q, want %q", rep.ProductType, "Ball")
	}
	if len(rep.Features) == 0 {
		t.Error("expected features from textAttr.SellingPoints")
	}
}

func TestFuseMultimodal_NilImageAttr_UsesTextOnly(t *testing.T) {
	p := newTestUnderstanding(t, "not json", nil)

	textAttr := &productenrich.TextAttributes{
		Title:      "Table",
		Attributes: map[string]string{"legs": "4"},
	}

	rep, err := p.FuseMultimodal(context.Background(), nil, textAttr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.ProductType != "Table" {
		t.Errorf("ProductType = %q, want %q", rep.ProductType, "Table")
	}
}

func TestFuseMultimodal_LLMError_ReturnsError(t *testing.T) {
	p := newTestUnderstanding(t, "", errors.New("llm down"))

	_, err := p.FuseMultimodal(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
}

type rotatingMockClient struct {
	responses []string
	idx       int
}

func (r *rotatingMockClient) GetClient(_ string) (productenrich.LLMClient, error) {
	return r, nil
}

func (r *rotatingMockClient) GetDefaultClient() productenrich.LLMClient {
	return r
}

func (r *rotatingMockClient) Generate(_ context.Context, _ string) (string, error) {
	if r.idx >= len(r.responses) {
		return "{}", nil
	}
	resp := r.responses[r.idx]
	r.idx++
	return resp, nil
}

func (r *rotatingMockClient) AnalyzeImage(_ context.Context, _ string, _ string) (string, error) {
	if r.idx >= len(r.responses) {
		return "{}", nil
	}
	resp := r.responses[r.idx]
	r.idx++
	return resp, nil
}
