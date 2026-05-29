package shein

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/submitprep"
)

type stubChatCompleter struct {
	response *openaiclient.ChatCompletionResponse
	err      error
	lastReq  *openaiclient.ChatCompletionRequest
}

func (s stubChatCompleter) CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	s.lastReq = req
	if s.err != nil {
		return nil, s.err
	}
	return s.response, nil
}

func (s stubChatCompleter) Generate(ctx context.Context, prompt string) (string, error) {
	return "", errors.New("not implemented")
}

func (s stubChatCompleter) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	return "", errors.New("not implemented")
}

func (s stubChatCompleter) GetDefaultModel() string {
	return "test-model"
}

type recordingChatCompleter struct {
	response *openaiclient.ChatCompletionResponse
	lastReq  *openaiclient.ChatCompletionRequest
}

func (s *recordingChatCompleter) CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	s.lastReq = req
	return s.response, nil
}

func (s *recordingChatCompleter) Generate(context.Context, string) (string, error) {
	return "", errors.New("not implemented")
}

func (s *recordingChatCompleter) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", errors.New("not implemented")
}

func (s *recordingChatCompleter) GetDefaultModel() string {
	return "test-model"
}

type stubTranslateAPI struct{}

func (stubTranslateAPI) Translate(text string, from, to string) (string, error) {
	return "Spanish " + text, nil
}

func TestPrepareSubmitProductContent_FallsBackWhenAIUnavailable(t *testing.T) {
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     " Door curtain for home decor ",
		}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     " A soft curtain for bedrooms and living rooms. ",
		}},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "white"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{
				Language: "en",
				Name:     "white",
			}},
		}},
	}

	err := PrepareSubmitProductContent(context.Background(), product, "US", stubChatCompleter{err: errors.New("ai down")}, nil)
	if err != nil {
		t.Fatalf("PrepareSubmitProductContent returned error: %v", err)
	}

	if got := findLocalizedText(product.MultiLanguageNameList, "en"); got != "Door curtain for home decor" {
		t.Fatalf("english title = %q, want %q", got, "Door curtain for home decor")
	}
	if got := findLocalizedText(product.MultiLanguageDescList, "en"); got != "A soft curtain for bedrooms and living rooms." {
		t.Fatalf("english description = %q, want %q", got, "A soft curtain for bedrooms and living rooms.")
	}
	if got := findLocalizedText(product.MultiLanguageNameList, "es"); got != "Door curtain for home decor" {
		t.Fatalf("spanish title fallback = %q, want %q", got, "Door curtain for home decor")
	}
	if got := findLocalizedText(product.SKCList[0].MultiLanguageNameList, "en"); !strings.EqualFold(got, "door curtain for home decor white") {
		t.Fatalf("english skc fallback = %q, want case-insensitive match for %q", got, "door curtain for home decor white")
	}
	if got := findLocalizedText(product.SKCList[0].MultiLanguageNameList, "es"); !strings.EqualFold(got, "door curtain for home decor white") {
		t.Fatalf("spanish skc fallback = %q, want case-insensitive match for %q", got, "door curtain for home decor white")
	}
}

func TestPrepareSubmitProductContent_UsesTranslateAPIForMissingTargets(t *testing.T) {
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Door curtain for home decor",
		}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "A soft curtain for bedrooms and living rooms.",
		}},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "white"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{
				Language: "en",
				Name:     "white",
			}},
		}},
	}

	err := PrepareSubmitProductContent(context.Background(), product, "US", nil, stubTranslateAPI{})
	if err != nil {
		t.Fatalf("PrepareSubmitProductContent returned error: %v", err)
	}

	if got := findLocalizedText(product.MultiLanguageNameList, "es"); got != "Spanish Door curtain for home decor" {
		t.Fatalf("spanish title = %q, want %q", got, "Spanish Door curtain for home decor")
	}
	if got := findLocalizedText(product.MultiLanguageDescList, "es"); got != "Spanish A soft curtain for bedrooms and living rooms." {
		t.Fatalf("spanish description = %q, want %q", got, "Spanish A soft curtain for bedrooms and living rooms.")
	}
	if got := findLocalizedText(product.SKCList[0].MultiLanguageNameList, "es"); !strings.EqualFold(got, "Spanish door curtain for home decor white") {
		t.Fatalf("spanish skc = %q, want case-insensitive match for %q", got, "Spanish door curtain for home decor white")
	}
	if got := product.SKCList[0].MultiLanguageName; got.Language != "en" || !strings.EqualFold(got.Name, "door curtain for home decor white") {
		t.Fatalf("primary skc name = %+v, want english primary name", got)
	}
}

func TestOptimizeSubmitContentWithAI_SendsMainImageToAI(t *testing.T) {
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Door curtain",
		}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Soft decorative curtain for bedrooms.",
		}},
		ImageInfo: &sheinproduct.ImageInfo{
			ImageInfoList: []sheinproduct.ImageDetail{
				{ImageURL: "https://example.com/main.jpg"},
				{ImageURL: "https://example.com/gallery.jpg"},
			},
		},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "white"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{
				Language: "en",
				Name:     "white",
			}},
		}},
	}
	ai := &recordingChatCompleter{
		response: &openaiclient.ChatCompletionResponse{
			Choices: []openaiclient.ChatCompletionChoice{{
				Message: openaiclient.ChatCompletionMessage{
					Content: `{"title":"Elegant Door Curtain for Bedroom Privacy and Home Decor Styling","description":"A soft decorative door curtain designed to add privacy, texture, and a warm finished look to bedrooms and living spaces."}`,
				},
			}},
		},
	}

	title, description, err := optimizeSubmitContentWithAI(
		context.Background(),
		ai,
		findLocalizedText(product.MultiLanguageNameList, "en"),
		findLocalizedText(product.MultiLanguageDescList, "en"),
		buildSubmitContentFeatures(product),
		collectSubmitContentImageURLs(product),
	)
	if err != nil {
		t.Fatalf("optimizeSubmitContentWithAI returned error: %v", err)
	}
	if title == "" || description == "" {
		t.Fatalf("optimized content = %q / %q, want non-empty", title, description)
	}
	if ai.lastReq == nil || len(ai.lastReq.Messages) < 2 {
		t.Fatalf("ai request = %+v, want multimodal user message", ai.lastReq)
	}
	parts := ai.lastReq.Messages[1].MultiContent
	if len(parts) != 2 {
		t.Fatalf("user multi-content parts = %+v, want text + main image", parts)
	}
	if parts[0].Type != "text" {
		t.Fatalf("first part type = %q, want text", parts[0].Type)
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "https://example.com/main.jpg" {
		t.Fatalf("image part = %+v, want main image only", parts[1])
	}
}

func TestPrepareSubmitProductContent_PreservesExistingContentWithoutAIRewrite(t *testing.T) {
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Envelope style pillow cover",
		}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Envelope style pillow cover designed for everyday home decor.",
		}},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "beige"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{
				Language: "en",
				Name:     "beige",
			}},
		}},
	}
	ai := &recordingChatCompleter{
		response: &openaiclient.ChatCompletionResponse{
			Choices: []openaiclient.ChatCompletionChoice{{
				Message: openaiclient.ChatCompletionMessage{
					Content: `{"title":"Unexpected rewrite","description":"Unexpected rewrite"}`,
				},
			}},
		},
	}

	if err := PrepareSubmitProductContent(context.Background(), product, "US", ai, nil); err != nil {
		t.Fatalf("PrepareSubmitProductContent returned error: %v", err)
	}
	if ai.lastReq != nil {
		t.Fatalf("ai request = %+v, want submit content to skip AI rewrite", ai.lastReq)
	}
	if got := findLocalizedText(product.MultiLanguageNameList, "en"); got != "Envelope style pillow cover" {
		t.Fatalf("english title = %q, want original reviewed content", got)
	}
	if got := findLocalizedText(product.MultiLanguageDescList, "en"); got != "Envelope style pillow cover designed for everyday home decor." {
		t.Fatalf("english description = %q, want original reviewed content", got)
	}
	if got := findLocalizedText(product.SKCList[0].MultiLanguageNameList, "en"); !strings.EqualFold(got, "envelope style pillow cover beige") {
		t.Fatalf("english skc = %q, want case-insensitive match for %q", got, "envelope style pillow cover beige")
	}
}

func TestApplySubmitContent_TruncatesTitleToSheinLimit(t *testing.T) {
	t.Parallel()

	title := strings.Repeat("A", sheinSubmitTitleMaxLength+20)
	description := strings.Repeat("B", sheinSubmitDescriptionMaxLength+50)
	product := &sheinproduct.Product{
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "white"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{
				Language: "en",
				Name:     "white",
			}},
		}},
	}

	applySubmitContent(product, title, description)

	gotTitle := findLocalizedText(product.MultiLanguageNameList, "en")
	if len(gotTitle) != sheinSubmitTitleMaxLength {
		t.Fatalf("title length = %d, want %d", len(gotTitle), sheinSubmitTitleMaxLength)
	}
	gotDescription := findLocalizedText(product.MultiLanguageDescList, "en")
	if len(gotDescription) != sheinSubmitDescriptionMaxLength {
		t.Fatalf("description length = %d, want %d", len(gotDescription), sheinSubmitDescriptionMaxLength)
	}
	gotSKCTitle := findLocalizedText(product.SKCList[0].MultiLanguageNameList, "en")
	if len(gotSKCTitle) > sheinSubmitTitleMaxLength {
		t.Fatalf("skc title length = %d, want <= %d", len(gotSKCTitle), sheinSubmitTitleMaxLength)
	}
}

func TestBuildSubmitSnapshot_CapturesFinalPayloadFields(t *testing.T) {
	supplierCode := "SKC-1"
	product := &sheinproduct.Product{
		SPUName:      "SPU-123",
		SupplierCode: "SUP-001",
		ImageInfo: &sheinproduct.ImageInfo{
			ImageInfoList: []sheinproduct.ImageDetail{
				{ImageURL: "https://example.com/1.jpg"},
				{ImageURL: "https://example.com/2.jpg"},
			},
		},
		MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Door curtain"}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{Language: "en", Name: "Soft curtain"}},
		SKCList: []sheinproduct.SKC{{
			SupplierCode:      &supplierCode,
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "white"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{
				{Language: "en", Name: "white"},
				{Language: "es", Name: "blanco"},
			},
		}},
	}

	snapshot := BuildSubmitSnapshot(product)
	if snapshot == nil {
		t.Fatal("BuildSubmitSnapshot returned nil")
	}
	if snapshot.SPUName != "SPU-123" || snapshot.SupplierCode != "SUP-001" {
		t.Fatalf("snapshot header = %+v", snapshot)
	}
	if snapshot.ImageCount != 2 {
		t.Fatalf("image count = %d, want 2", snapshot.ImageCount)
	}
	if len(snapshot.SKCList) != 1 {
		t.Fatalf("skc snapshot count = %d, want 1", len(snapshot.SKCList))
	}
	if snapshot.SKCList[0].SupplierCode != "SKC-1" || snapshot.SKCList[0].PrimaryName != "white" {
		t.Fatalf("skc snapshot = %+v", snapshot.SKCList[0])
	}
}

func TestRetrySensitiveWordCleanup_RemovesFlaggedWord(t *testing.T) {
	restore := overrideSensitiveWordsConfigForTest(t)
	defer restore()

	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Whimsy Door Curtain"}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{Language: "en", Name: "Whimsy curtain for home decor"}},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName:     sheinproduct.LanguageContent{Language: "en", Name: "whimsy white"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "whimsy white"}},
		}},
	}

	if !RetrySensitiveWordCleanup(product, []string{"敏感词：whimsy"}) {
		t.Fatal("expected sensitive-word retry cleanup to return true")
	}
	if strings.Contains(strings.ToLower(findLocalizedText(product.MultiLanguageNameList, "en")), "whimsy") {
		t.Fatalf("english title still contains whimsy: %+v", product.MultiLanguageNameList)
	}
	if strings.Contains(strings.ToLower(findLocalizedText(product.MultiLanguageDescList, "en")), "whimsy") {
		t.Fatalf("english description still contains whimsy: %+v", product.MultiLanguageDescList)
	}
	if strings.Contains(strings.ToLower(findLocalizedText(product.SKCList[0].MultiLanguageNameList, "en")), "whimsy") {
		t.Fatalf("english skc still contains whimsy: %+v", product.SKCList[0].MultiLanguageNameList)
	}
}

func findLocalizedText(items []sheinproduct.LanguageContent, language string) string {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Language), language) {
			return strings.TrimSpace(item.Name)
		}
	}
	return ""
}

func overrideSensitiveWordsConfigForTest(t *testing.T) func() {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	sourcePath := filepath.Join(filepath.Dir(file), "..", "..", "..", "data", "sensitive_words_shein.json")
	bytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read sensitive words config: %v", err)
	}
	tempPath := filepath.Join(t.TempDir(), "sensitive_words_shein.json")
	if err := os.WriteFile(tempPath, bytes, 0o600); err != nil {
		t.Fatalf("write temp sensitive words config: %v", err)
	}
	return submitprep.SetSensitiveWordsConfigPathForTesting(tempPath)
}
