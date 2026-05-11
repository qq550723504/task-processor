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
}

func (s stubChatCompleter) CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
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
	if got := findLocalizedText(product.SKCList[0].MultiLanguageNameList, "es"); got != "white" {
		t.Fatalf("spanish skc fallback = %q, want %q", got, "white")
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
	if got := findLocalizedText(product.SKCList[0].MultiLanguageNameList, "es"); !strings.EqualFold(got, "Spanish white") {
		t.Fatalf("spanish skc = %q, want case-insensitive match for %q", got, "Spanish white")
	}
	if got := product.SKCList[0].MultiLanguageName; got.Language != "en" || got.Name != "white" {
		t.Fatalf("primary skc name = %+v, want english primary name", got)
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
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "whimsy white"},
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
