package skc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/aicache"
	"task-processor/internal/shein/api/product"
	sheintranslate "task-processor/internal/shein/api/translate"
	sheinclient "task-processor/internal/shein/client"
	"task-processor/internal/shein/namelimit"

	"github.com/imroc/req/v3"
)

func TestOptimizeMultiLanguageContentUsesConfiguredEnglishLimit(t *testing.T) {
	t.Parallel()

	cache := aicache.New(nil)
	overlong := strings.Repeat("A", 50)
	cache.Set(aicache.TypeSKCTranslate, aicache.HashKey(overlong), []string{overlong})

	handler := &SKCTranslationHandler{runtime: &SKCRuntimeInput{
		AICache:          cache,
		NameLengthLimits: namelimit.Limits{"en": 25},
	}}
	items := []product.LanguageContent{{
		Language: "en",
		Name:     overlong,
	}}

	handler.optimizeMultiLanguageContent(context.Background(), &items, "source title")

	if len(items[0].Name) != 25 {
		t.Fatalf("english title length = %d, want 25", len(items[0].Name))
	}
}

func TestCreateSKCUsesTaskTargetLanguages(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"0","msg":"OK","info":{"data":[{"translated_text":"bonjourlong","code":0}]}}`))
	}))
	defer server.Close()

	cache := aicache.New(nil)
	cache.Set(aicache.TypeSKCTranslate, aicache.HashKey("source title"), []string{"source title"})
	handler := &SKCTranslationHandler{runtime: &SKCRuntimeInput{
		Region:           "JP",
		AmazonProduct:    &model.Product{Title: "source title"},
		TranslateAPI:     sheintranslate.NewClient(sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C())),
		AICache:          cache,
		TargetLanguages:  []string{"en", "fr"},
		NameLengthLimits: namelimit.Limits{"fr": 5},
	}}

	got := handler.CreateSKC(context.Background(), shein.SKCCreationParams{})
	languages := make([]string, 0, len(got.MultiLanguageNameList))
	for _, item := range got.MultiLanguageNameList {
		languages = append(languages, item.Language)
		if item.Language == "fr" && item.Name != "bonjo" {
			t.Fatalf("french name = %q, want %q", item.Name, "bonjo")
		}
	}
	if !reflect.DeepEqual(languages, []string{"en", "fr"}) {
		t.Fatalf("languages = %#v, want [en fr]", languages)
	}
}

func TestApplyNameLimitsCountsChineseCharacters(t *testing.T) {
	t.Parallel()

	handler := &SKCTranslationHandler{runtime: &SKCRuntimeInput{
		NameLengthLimits: namelimit.Limits{"zh-cn": 3},
	}}
	items := []product.LanguageContent{{Language: "zh-cn", Name: "一二三四五"}}

	handler.applyNameLimits(items)

	if items[0].Name != "一二三" {
		t.Fatalf("chinese name = %q, want %q", items[0].Name, "一二三")
	}
}

type promptCapturingCompleter struct{ systemPrompt string }

func (c *promptCapturingCompleter) CreateChatCompletion(_ context.Context, req *openaiClient.ChatCompletionRequest) (*openaiClient.ChatCompletionResponse, error) {
	c.systemPrompt = req.Messages[0].Content
	return &openaiClient.ChatCompletionResponse{Choices: []openaiClient.ChatCompletionChoice{{
		Message: openaiClient.ChatCompletionMessage{Content: `{"optimized_titles":["valid optimized title"]}`},
	}}}, nil
}
func (*promptCapturingCompleter) Generate(context.Context, string) (string, error) { return "", nil }
func (*promptCapturingCompleter) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", nil
}
func (*promptCapturingCompleter) GetDefaultModel() string { return "test" }

func TestBatchOptimizeEnglishContentUsesConfiguredLimitInPrompt(t *testing.T) {
	t.Parallel()

	client := &promptCapturingCompleter{}
	handler := &SKCTranslationHandler{
		runtime:      &SKCRuntimeInput{NameLengthLimits: namelimit.Limits{"en": 25}},
		openaiClient: client,
	}
	if _, err := handler.batchOptimizeEnglishContent(context.Background(), []string{"source title"}, "source title"); err != nil {
		t.Fatalf("batchOptimizeEnglishContent() error = %v", err)
	}
	if !strings.Contains(client.systemPrompt, "10-25 characters") {
		t.Fatalf("system prompt does not contain dynamic limit: %q", client.systemPrompt)
	}
}
