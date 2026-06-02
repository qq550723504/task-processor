package shein

import (
	"context"
	"strings"
	"testing"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit/tenantctx"
	"task-processor/internal/shein/submitprep"
)

type stubTitleAIClient struct {
	response string
}

func (s stubTitleAIClient) CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return &openaiclient.ChatCompletionResponse{
		Choices: []openaiclient.ChatCompletionChoice{{
			Message: openaiclient.ChatCompletionMessage{Content: s.response},
		}},
	}, nil
}

func (s stubTitleAIClient) Generate(ctx context.Context, prompt string) (string, error) {
	return s.response, nil
}

func (s stubTitleAIClient) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	return "", nil
}

func (s stubTitleAIClient) GetDefaultModel() string {
	return "stub"
}

func TestBuildSheinListingCopyKeepsStructuredEnglishTitle(t *testing.T) {
	canonical := &canonical.Product{
		Title: "Flannel non-slip floor mat",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Flannel Non-slip Floor Mat"},
		},
	}

	copy := buildSheinListingCopy(nil, canonical, canonical.Title, nil)
	if copy.Title != "Flannel Non-slip Floor Mat" {
		t.Fatalf("title = %q, want clean structured title", copy.Title)
	}
	if copy.TitleDiagnostics == nil || copy.TitleDiagnostics.PromptContaminated {
		t.Fatalf("title diagnostics = %+v, want uncontaminated", copy.TitleDiagnostics)
	}
}

func TestBuildSheinListingCopySanitizesPromptLikeTitleWithRules(t *testing.T) {
	canonical := &canonical.Product{
		Title: "Flannel non-slip floor mat",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Flannel non-slip floor mat - Please design an image that can be printed on my non-slip floor mat. The image should include suitable English text and graphics, and the graphics and text should have a 3D visual effect. Please ensure it does not infringe on copyright. 3000 pixels * 2"},
		},
	}

	copy := buildSheinListingCopy(nil, canonical, canonical.Title, nil)
	if copy.Title != "Flannel non-slip floor mat" {
		t.Fatalf("title = %q, want sanitized base title", copy.Title)
	}
	if copy.SKCTitleBase != "Flannel non-slip floor mat" {
		t.Fatalf("skc base title = %q, want sanitized short title", copy.SKCTitleBase)
	}
	if copy.TitleDiagnostics == nil || !copy.TitleDiagnostics.PromptContaminated || copy.TitleDiagnostics.Source != "prompt_extracted_rule" {
		t.Fatalf("title diagnostics = %+v, want prompt_extracted_rule contamination", copy.TitleDiagnostics)
	}
}

func TestBuildSheinListingCopyUsesLLMWhenRuleExtractionCannotRecover(t *testing.T) {
	canonical := &canonical.Product{
		Title: "Flannel non-slip floor mat",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Please design an image for my floor mat with floral artwork and inspirational text, 3000 pixels"},
		},
	}

	copy := buildSheinListingCopy(nil, canonical, canonical.Title, stubTitleAIClient{
		response: `{"title":"Flannel Floral Floor Mat"}`,
	})
	if copy.Title != "Flannel Floral Floor Mat" {
		t.Fatalf("title = %q, want llm extracted title", copy.Title)
	}
	if copy.TitleDiagnostics == nil || copy.TitleDiagnostics.Source != "prompt_extracted_llm" {
		t.Fatalf("title diagnostics = %+v, want prompt_extracted_llm", copy.TitleDiagnostics)
	}
}

func TestBuildSheinListingCopyFallsBackWhenLLMReturnsPromptLikeTitle(t *testing.T) {
	canonical := &canonical.Product{
		Title: "Flannel non-slip floor mat",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Please design an image for my floor mat with floral artwork and inspirational text, 3000 pixels"},
			"material":             {Value: "polyester"},
		},
	}

	copy := buildSheinListingCopy(nil, canonical, canonical.Title, stubTitleAIClient{
		response: `{"title":"Please design a floral image for this floor mat with 3D graphics"}`,
	})
	if copy.Title == "" || copy.Title == canonical.Attributes["product_english_name"].Value {
		t.Fatalf("title = %q, want non-empty fallback title", copy.Title)
	}
	if isPromptLikeTitle(copy.Title) {
		t.Fatalf("title = %q, want non prompt-like fallback", copy.Title)
	}
	if copy.TitleDiagnostics == nil || !copy.TitleDiagnostics.PromptContaminated {
		t.Fatalf("title diagnostics = %+v, want contamination note", copy.TitleDiagnostics)
	}
}

func TestBuildSheinListingCopyEnrichesShortStructuredTitleWithLLMAddition(t *testing.T) {
	canonical := &canonical.Product{
		Title: "Door curtain",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Door curtain"},
			"picture_request":      {Value: "Please design a door curtain print with a rock style graphic theme and dramatic lettering, 2277 x 4500px"},
		},
		Variants: []canonical.Variant{{
			Attributes: map[string]canonical.Attribute{
				"ai_style": {Value: "Please design a door curtain print with a rock style graphic theme and dramatic lettering, 2277 x 4500px"},
			},
		}},
	}

	copy := buildSheinListingCopy(nil, canonical, canonical.Title, stubTitleAIClient{
		response: `{"addition":"Rock Typography Graphic Print"}`,
	})
	if copy.Title != "Door curtain with Rock Typography Graphic Print" {
		t.Fatalf("title = %q, want enriched ecommerce title", copy.Title)
	}
	if copy.SKCTitleBase != "Door curtain with Rock Typography Graphic Print" {
		t.Fatalf("skc base title = %q, want enriched short title", copy.SKCTitleBase)
	}
	if copy.TitleDiagnostics == nil || !strings.Contains(copy.TitleDiagnostics.ResolutionNote, "llm-extracted prompt elements") {
		t.Fatalf("title diagnostics = %+v, want enrichment note", copy.TitleDiagnostics)
	}
}

func TestBuildSheinListingCopyKeepsLongStructuredTitleWithoutLLMAddition(t *testing.T) {
	canonical := &canonical.Product{
		Title: "Flannel non-slip floor mat",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Flannel Non-slip Floor Mat"},
			"picture_request":      {Value: "Please design a floral print with inspirational words, 3000 pixels"},
		},
	}

	copy := buildSheinListingCopy(nil, canonical, canonical.Title, stubTitleAIClient{
		response: `{"addition":"Floral Theme"}`,
	})
	if copy.Title != "Flannel Non-slip Floor Mat" {
		t.Fatalf("title = %q, want original long structured title", copy.Title)
	}
}

func TestBuildSheinListingCopyEnrichesRealDoorCurtainTaskTitle(t *testing.T) {
	canonical := &canonical.Product{
		Title: "Door curtain",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Door curtain"},
			"picture_request":      {Value: "2277 px * 4500 px"},
		},
		Variants: []canonical.Variant{{
			Attributes: map[string]canonical.Attribute{
				"ai_style": {Value: "帮我设计一个印在门帘上的图案，图案要有英文跟图案，元素多样，图片有3d视觉效果，摇滚风格，2277 × 4500px"},
			},
		}},
	}

	copy := buildSheinListingCopy(nil, canonical, canonical.Title, stubTitleAIClient{
		response: `{"addition":"Rock Typography Graphic Print"}`,
	})
	if copy.Title != "Door curtain with Rock Typography Graphic Print" {
		t.Fatalf("title = %q, want enriched title from real task prompt", copy.Title)
	}
	if copy.TitleDiagnostics == nil || copy.TitleDiagnostics.Source != "product_english_name" {
		t.Fatalf("title diagnostics = %+v, want structured source retained", copy.TitleDiagnostics)
	}
	if copy.SKCTitleBase != "Door curtain with Rock Typography Graphic Print" {
		t.Fatalf("skc base title = %q, want enriched skc base title", copy.SKCTitleBase)
	}
}

func TestBuildSheinListingCopyCleansSensitiveWords(t *testing.T) {
	restoreRepo := submitprep.SetSensitiveWordRepository(&stubSensitiveWordRepository{
		pages: map[int64][]listingadmin.SensitiveWord{
			101: {
				{TenantID: 101, Language: "en", Word: "amazon", Status: 1},
				{TenantID: 101, Language: "en", Word: "bpa free", Status: 1},
			},
		},
	})
	defer restoreRepo()

	canonical := &canonical.Product{
		Title:       "Amazon BPA Free Vase",
		Description: "Amazon BPA Free vase for home decor.",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Amazon BPA Free Vase"},
		},
	}

	copy := buildSheinListingCopy(tenantctx.WithTenantID(context.Background(), "101"), canonical, canonical.Title, nil)

	assertNoSensitivePhrase(t, copy.Title, "title")
	assertNoSensitivePhrase(t, copy.Description, "description")
	assertNoSensitivePhrase(t, copy.SKCTitleBase, "skc base")
}

func TestBuildSheinListingCopyLoadsTenantSensitiveWordsFromRepository(t *testing.T) {
	restoreRepo := submitprep.SetSensitiveWordRepository(&stubSensitiveWordRepository{
		pages: map[int64][]listingadmin.SensitiveWord{
			101: {{
				TenantID: 101,
				Language: "en",
				Word:     "whimsy",
				Status:   1,
			}},
		},
	})
	defer restoreRepo()

	runtimeCtx := tenantctx.WithTenantID(context.Background(), "101")
	canonical := &canonical.Product{
		Title:       "Whimsy Door Curtain",
		Description: "Whimsy curtain for home decor.",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Whimsy Door Curtain"},
		},
	}

	copy := buildSheinListingCopy(runtimeCtx, canonical, canonical.Title, nil)

	if strings.Contains(strings.ToLower(copy.Title), "whimsy") {
		t.Fatalf("title = %q, want tenant sensitive word removed", copy.Title)
	}
	if strings.Contains(strings.ToLower(copy.Description), "whimsy") {
		t.Fatalf("description = %q, want tenant sensitive word removed", copy.Description)
	}
	if strings.Contains(strings.ToLower(copy.SKCTitleBase), "whimsy") {
		t.Fatalf("skc base = %q, want tenant sensitive word removed", copy.SKCTitleBase)
	}
}

func TestDifferentTenantsLoadDifferentGenerationTopicLexicons(t *testing.T) {
	restoreRepo := submitprep.SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		keys: map[int64][]string{
			101: {"children"},
			202: {"meals"},
		},
	})
	defer restoreRepo()

	copyA := buildSheinListingCopy(tenantctx.WithTenantID(context.Background(), "101"), &canonical.Product{
		Title:       "Kids Room Curtain",
		Description: "Decor for children bedroom",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Kids Room Curtain"},
		},
	}, "Kids Room Curtain", nil)
	copyB := buildSheinListingCopy(tenantctx.WithTenantID(context.Background(), "202"), &canonical.Product{
		Title:       "Breakfast Table Curtain",
		Description: "Meal-themed decor",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Breakfast Table Curtain"},
		},
	}, "Breakfast Table Curtain", nil)

	if strings.Contains(strings.ToLower(copyA.Title), "kids") || strings.Contains(strings.ToLower(copyA.Description), "children") {
		t.Fatalf("tenant 101 copy = %+v, want children terms removed", copyA)
	}
	if strings.Contains(strings.ToLower(copyB.Title), "breakfast") || strings.Contains(strings.ToLower(copyB.Description), "meal") {
		t.Fatalf("tenant 202 copy = %+v, want meal terms removed", copyB)
	}
}

func TestBuildSheinListingCopyLoadsTenantGenerationTopicOverrideLexicon(t *testing.T) {
	restorePolicyRepo := submitprep.SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		keys: map[int64][]string{
			101: {"children"},
		},
	})
	defer restorePolicyRepo()
	restoreOverrideRepo := submitprep.SetGenerationTopicOverrideRepository(&stubGenerationTopicOverrideRepository{
		items: map[string]listingadmin.GenerationTopicOverride{
			overrideRepoKey(101, "shein", "children"): {
				TenantID: 101,
				Platform: "shein",
				TopicKey: "children",
				AdditionalLexiconByLanguage: map[string][]string{
					"en": {"toddler"},
				},
				Status: 1,
			},
		},
	})
	defer restoreOverrideRepo()

	copy := buildSheinListingCopy(tenantctx.WithTenantID(context.Background(), "101"), &canonical.Product{
		Title:       "Toddler Room Curtain",
		Description: "Toddler-themed decor for a kids room",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Toddler Room Curtain"},
		},
	}, "Toddler Room Curtain", nil)

	if strings.Contains(strings.ToLower(copy.Title), "toddler") {
		t.Fatalf("title = %q, want override lexicon removed", copy.Title)
	}
	if strings.Contains(strings.ToLower(copy.Description), "toddler") {
		t.Fatalf("description = %q, want override lexicon removed", copy.Description)
	}
}

func assertNoSensitivePhrase(t *testing.T, value string, field string) {
	t.Helper()
	normalized := strings.ToLower(value)
	for _, phrase := range []string{"amazon", "bpa free"} {
		if strings.Contains(normalized, phrase) {
			t.Fatalf("%s = %q, want %q removed", field, value, phrase)
		}
	}
}
