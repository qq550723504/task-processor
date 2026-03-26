package productenrich

import (
	"context"
	"errors"
	"testing"
)

func newTestUnderstanding(llmResp string, llmErr error) *productUnderstanding {
	mgr := newMockLLMManager(llmResp)
	if llmErr != nil {
		mgr.def.err = llmErr
		for _, c := range mgr.clients {
			c.err = llmErr
		}
	}
	return &productUnderstanding{llmManager: mgr}
}

// --- AnalyzeProduct ---

func TestAnalyzeProduct_NilInput(t *testing.T) {
	p := newTestUnderstanding("{}", nil)
	_, err := p.AnalyzeProduct(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil input")
	}
}

func TestAnalyzeProduct_NoImagesNoText(t *testing.T) {
	p := newTestUnderstanding("{}", nil)
	result, err := p.AnalyzeProduct(context.Background(), &ParsedInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 没有图片和文本，不会调用 FuseMultimodal，Representation 应为 nil
	if result.Representation != nil {
		t.Error("expected nil Representation when no images and no text")
	}
}

func TestAnalyzeProduct_WithText_ExtractsAttributes(t *testing.T) {
	resp := `{"title":"Desk","attributes":{"material":"wood"},"selling_points":["sturdy"]}`
	p := newTestUnderstanding(resp, nil)

	result, err := p.AnalyzeProduct(context.Background(), &ParsedInput{Text: "A wooden desk"})
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
	// 第一张图片 color 为空，第二张图片补充 color
	callCount := 0
	responses := []string{
		`{"color":"","material":"metal","scene":"studio","usage":"decoration"}`,
		`{"color":"silver","material":"metal","scene":"studio","usage":"decoration"}`,
	}
	mgr := &mockLLMManager{
		clients: map[string]*mockLLMClient{},
	}
	// 用一个会轮换响应的 client
	rotatingClient := &rotatingMockClient{responses: responses}
	mgr.clients["vision"] = &mockLLMClient{}
	mgr.clients["default"] = &mockLLMClient{}
	mgr.clients["fast"] = &mockLLMClient{}
	mgr.def = &mockLLMClient{}
	_ = callCount

	// 直接用两个独立 manager 模拟两次 AnalyzeImage 调用
	// 第一次调用返回 color=""，第二次返回 color="silver"
	p := &productUnderstanding{llmManager: rotatingClient}

	result, err := p.AnalyzeProduct(context.Background(), &ParsedInput{
		Images: []string{"img1.jpg", "img2.jpg"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ImageAttributes == nil {
		t.Fatal("expected ImageAttributes")
	}
	// 第二张图片应补充了 color
	if result.ImageAttributes.Color != "silver" {
		t.Errorf("Color = %q, want %q", result.ImageAttributes.Color, "silver")
	}
}

func TestAnalyzeProduct_ScrapedData_MergesSellingPoints(t *testing.T) {
	// 主文本和 scraped 各有一个卖点，合并后应有两个（去重）
	callCount := 0
	responses := []string{
		// 第一次 ExtractTextAttributes（主文本）
		`{"title":"T","attributes":{},"selling_points":["point-a"]}`,
		// 第二次 ExtractTextAttributes（scraped）
		`{"title":"T","attributes":{},"selling_points":["point-b"]}`,
		// FuseMultimodal
		`{"product_type":"T","attributes":{},"features":[]}`,
	}
	p := &productUnderstanding{llmManager: &rotatingMockClient{responses: responses}}
	_ = callCount

	result, err := p.AnalyzeProduct(context.Background(), &ParsedInput{
		Text:        "main text",
		ScrapedData: &ScrapedData{Description: "scraped text"},
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
	p := &productUnderstanding{llmManager: &rotatingMockClient{responses: responses}}

	result, err := p.AnalyzeProduct(context.Background(), &ParsedInput{
		Text:        "main text",
		ScrapedData: &ScrapedData{Description: "scraped text"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 重复卖点应被去重
	if len(result.TextAttributes.SellingPoints) != 1 {
		t.Errorf("SellingPoints len = %d, want 1 (deduped)", len(result.TextAttributes.SellingPoints))
	}
}

// --- AnalyzeImage ---

func TestAnalyzeImage_EmptyPath_ReturnsError(t *testing.T) {
	p := newTestUnderstanding("{}", nil)
	_, err := p.AnalyzeImage(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty image path")
	}
}

func TestAnalyzeImage_ValidJSON_ParsesAttributes(t *testing.T) {
	resp := `{"color":"red","material":"plastic","scene":"outdoor","usage":"sports"}`
	p := newTestUnderstanding(resp, nil)

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
	p := newTestUnderstanding("not json", nil)

	attr, err := p.AnalyzeImage(context.Background(), "http://example.com/img.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// JSON 解析失败时应降级为 "unknown"
	if attr.Color != "unknown" {
		t.Errorf("Color = %q, want %q", attr.Color, "unknown")
	}
}

func TestAnalyzeImage_LLMError_ReturnsError(t *testing.T) {
	p := newTestUnderstanding("", errors.New("vision api down"))

	_, err := p.AnalyzeImage(context.Background(), "http://example.com/img.jpg")
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
}

// --- ExtractTextAttributes ---

func TestExtractTextAttributes_EmptyText_ReturnsError(t *testing.T) {
	p := newTestUnderstanding("{}", nil)
	_, err := p.ExtractTextAttributes(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestExtractTextAttributes_ValidJSON_ParsesFields(t *testing.T) {
	resp := `{"title":"Lamp","attributes":{"wattage":"10W"},"selling_points":["bright","efficient"]}`
	p := newTestUnderstanding(resp, nil)

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
	p := newTestUnderstanding("not json", nil)

	text := "A very long product description that exceeds fifty characters in total length"
	attr, err := p.ExtractTextAttributes(context.Background(), text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 降级时 Title 应被截断到 50 字符
	if len([]rune(attr.Title)) > 50 {
		t.Errorf("Title too long: %d chars", len([]rune(attr.Title)))
	}
	if attr.Attributes == nil {
		t.Error("expected non-nil Attributes map")
	}
}

// --- FuseMultimodal ---

func TestFuseMultimodal_ValidJSON_ParsesRepresentation(t *testing.T) {
	resp := `{"product_type":"Chair","attributes":{"material":"wood"},"features":["comfortable","durable"]}`
	p := newTestUnderstanding(resp, nil)

	imageAttr := &ImageAttributes{Color: "brown", Material: "wood"}
	textAttr := &TextAttributes{Title: "Chair", SellingPoints: []string{"comfortable"}}

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
	p := newTestUnderstanding("not json", nil)

	imageAttr := &ImageAttributes{Color: "red", Material: "plastic"}
	textAttr := &TextAttributes{
		Title:         "Ball",
		Attributes:    map[string]string{"size": "large"},
		SellingPoints: []string{"bouncy"},
	}

	rep, err := p.FuseMultimodal(context.Background(), imageAttr, textAttr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 降级时应从 imageAttr 和 textAttr 拼接属性
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
	p := newTestUnderstanding("not json", nil)

	textAttr := &TextAttributes{
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
	p := newTestUnderstanding("", errors.New("llm down"))

	_, err := p.FuseMultimodal(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
}

// --- rotatingMockClient 辅助：按顺序返回不同响应 ---

// rotatingMockClient 实现 LLMManager，按顺序返回预设响应
type rotatingMockClient struct {
	responses []string
	idx       int
}

func (r *rotatingMockClient) GetClient(_ string) (LLMClient, error) {
	return r, nil
}

func (r *rotatingMockClient) GetDefaultClient() LLMClient {
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
