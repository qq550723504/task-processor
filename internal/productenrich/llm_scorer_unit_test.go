package productenrich

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/prompt"
)

func newTestLLMScorer(llmResp string, llmErr error) *llmScorer {
	mgr := newMockLLMManager(llmResp)
	if llmErr != nil {
		mgr.def.err = llmErr
		for _, c := range mgr.clients {
			c.err = llmErr
		}
	}
	return &llmScorer{
		llmManager:     mgr,
		textClient:     "fast",
		visionClient:   "vision",
		cacheTTL:       time.Hour,
		maxRetries:     2,
		fallbackWeight: 0.3,
	}
}

// --- parseLLMScore ---

func TestParseLLMScore_ValidJSON(t *testing.T) {
	s := newTestLLMScorer("", nil)
	score, err := s.parseLLMScore(`{"score":85,"reason":"good"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 85 {
		t.Errorf("score = %.1f, want 85", score)
	}
}

func TestParseLLMScore_StripsMarkdownFence(t *testing.T) {
	s := newTestLLMScorer("", nil)
	cases := []struct {
		name  string
		input string
	}{
		{"json fence", "```json\n{\"score\":70}\n```"},
		{"plain fence", "```\n{\"score\":70}\n```"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			score, err := s.parseLLMScore(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if score != 70 {
				t.Errorf("score = %.1f, want 70", score)
			}
		})
	}
}

func TestParseLLMScore_ScoreOutOfRange(t *testing.T) {
	s := newTestLLMScorer("", nil)
	cases := []struct {
		name  string
		input string
	}{
		{"negative", `{"score":-1}`},
		{"over 100", `{"score":101}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.parseLLMScore(tc.input)
			if err == nil {
				t.Fatal("expected error for out-of-range score")
			}
		})
	}
}

func TestParseLLMScore_InvalidJSON_ReturnsError(t *testing.T) {
	s := newTestLLMScorer("", nil)
	_, err := s.parseLLMScore("not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseLLMScore_TruncatedJSON_ExtractsScore(t *testing.T) {
	s := newTestLLMScorer("", nil)
	score, err := s.parseLLMScore("{\n  \"score\": 88,\n  \"reason\": \"图片清晰度高")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 88 {
		t.Errorf("score = %.1f, want 88", score)
	}
}

func TestParseLLMScore_TruncatedJSONWithoutScore_ReturnsError(t *testing.T) {
	s := newTestLLMScorer("", nil)
	_, err := s.parseLLMScore("{\n  \"reason\": \"图片清晰度高")
	if err == nil {
		t.Fatal("expected error when truncated JSON does not contain score")
	}
}

// --- combineScores ---

func TestCombineScores_WeightedAverage(t *testing.T) {
	s := &llmScorer{fallbackWeight: 0.3}
	// base=80, llm=100 → 80*0.7 + 100*0.3 = 86
	got := s.combineScores(80, 100)
	want := 86.0
	if got != want {
		t.Errorf("combineScores = %.1f, want %.1f", got, want)
	}
}

func TestCombineScores_ClampedToZero(t *testing.T) {
	s := &llmScorer{fallbackWeight: 0.5}
	got := s.combineScores(-50, -50)
	if got != 0 {
		t.Errorf("combineScores = %.1f, want 0 (clamped)", got)
	}
}

func TestCombineScores_ClampedTo100(t *testing.T) {
	s := &llmScorer{fallbackWeight: 0.5}
	got := s.combineScores(200, 200)
	if got != 100 {
		t.Errorf("combineScores = %.1f, want 100 (clamped)", got)
	}
}

// --- retryLLMCall ---

func TestRetryLLMCall_SuccessOnFirstAttempt(t *testing.T) {
	s := newTestLLMScorer("", nil)
	calls := 0
	resp, err := s.retryLLMCall(context.Background(), 3, func() (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("resp = %q, want %q", resp, "ok")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestRetryLLMCall_RetriesOnFailure(t *testing.T) {
	s := newTestLLMScorer("", nil)
	calls := 0
	// maxRetries=2 避免等待过长（每次重试等待 i+1 秒）
	_, err := s.retryLLMCall(context.Background(), 2, func() (string, error) {
		calls++
		return "", errors.New("transient")
	})
	if err == nil {
		t.Fatal("expected error after all retries")
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2 (maxRetries)", calls)
	}
}

func TestRetryLLMCall_ContextCancellation(t *testing.T) {
	s := newTestLLMScorer("", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := s.retryLLMCall(ctx, 3, func() (string, error) {
		return "", errors.New("fail")
	})
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}

// --- ScoreText / ScoreImage 空输入返回 baseScore ---

func TestScoreText_EmptyText_ReturnsBaseScore(t *testing.T) {
	s := newTestLLMScorer(`{"score":90}`, nil)
	score, err := s.ScoreText(context.Background(), "", 55.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 55.0 {
		t.Errorf("score = %.1f, want 55.0 (base score)", score)
	}
}

func TestScoreImage_EmptyURL_ReturnsBaseScore(t *testing.T) {
	s := newTestLLMScorer(`{"score":90}`, nil)
	score, err := s.ScoreImage(context.Background(), "", 42.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 42.0 {
		t.Errorf("score = %.1f, want 42.0 (base score)", score)
	}
}

// --- scoreWithCache nil cache 路径 ---

func TestScoreWithCache_NilCache_CallsLLMDirectly(t *testing.T) {
	s := newTestLLMScorer(`{"score":80}`, nil)
	s.scoreCache = nil // 确保 cache 为 nil

	score, err := s.ScoreText(context.Background(), "some text", 50.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 无缓存时直接调用 LLM，结果应为 combineScores(50, 80)
	want := s.combineScores(50, 80)
	if score != want {
		t.Errorf("score = %.1f, want %.1f", score, want)
	}
}

// --- mockLLMScoreCache 用于测试缓存命中路径 ---

type mockLLMScoreCache struct {
	textScores   map[string]float64
	imageScores  map[string]float64
	textResults  map[string]*CachedLLMScore
	imageResults map[string]*CachedLLMScore
}

func newMockLLMScoreCache() *mockLLMScoreCache {
	return &mockLLMScoreCache{
		textScores:   make(map[string]float64),
		imageScores:  make(map[string]float64),
		textResults:  make(map[string]*CachedLLMScore),
		imageResults: make(map[string]*CachedLLMScore),
	}
}

func (m *mockLLMScoreCache) GetTextScore(_ context.Context, text string) (float64, bool) {
	if result, ok := m.textResults[text]; ok && result != nil {
		return result.Score, true
	}
	v, ok := m.textScores[text]
	return v, ok
}
func (m *mockLLMScoreCache) SetTextScore(_ context.Context, text string, score float64, _ time.Duration) error {
	m.textScores[text] = score
	return nil
}
func (m *mockLLMScoreCache) GetImageScore(_ context.Context, url string) (float64, bool) {
	if result, ok := m.imageResults[url]; ok && result != nil {
		return result.Score, true
	}
	v, ok := m.imageScores[url]
	return v, ok
}
func (m *mockLLMScoreCache) SetImageScore(_ context.Context, url string, score float64, _ time.Duration) error {
	m.imageScores[url] = score
	return nil
}
func (m *mockLLMScoreCache) GetTextScoreResult(_ context.Context, text string) (*CachedLLMScore, bool) {
	if result, ok := m.textResults[text]; ok {
		if result == nil {
			return nil, false
		}
		cloned := &CachedLLMScore{Score: result.Score}
		if result.Prompt != nil {
			cloned.Prompt = result.Prompt.Clone()
		}
		return cloned, true
	}
	v, ok := m.textScores[text]
	if !ok {
		return nil, false
	}
	return &CachedLLMScore{Score: v}, true
}
func (m *mockLLMScoreCache) SetTextScoreResult(_ context.Context, text string, result *CachedLLMScore, _ time.Duration) error {
	if result == nil {
		m.textResults[text] = nil
		return nil
	}
	cloned := &CachedLLMScore{Score: result.Score}
	if result.Prompt != nil {
		cloned.Prompt = result.Prompt.Clone()
	}
	m.textResults[text] = cloned
	return nil
}
func (m *mockLLMScoreCache) GetImageScoreResult(_ context.Context, url string) (*CachedLLMScore, bool) {
	if result, ok := m.imageResults[url]; ok {
		if result == nil {
			return nil, false
		}
		cloned := &CachedLLMScore{Score: result.Score}
		if result.Prompt != nil {
			cloned.Prompt = result.Prompt.Clone()
		}
		return cloned, true
	}
	v, ok := m.imageScores[url]
	if !ok {
		return nil, false
	}
	return &CachedLLMScore{Score: v}, true
}
func (m *mockLLMScoreCache) SetImageScoreResult(_ context.Context, url string, result *CachedLLMScore, _ time.Duration) error {
	if result == nil {
		m.imageResults[url] = nil
		return nil
	}
	cloned := &CachedLLMScore{Score: result.Score}
	if result.Prompt != nil {
		cloned.Prompt = result.Prompt.Clone()
	}
	m.imageResults[url] = cloned
	return nil
}

func TestScoreText_CacheHit_SkipsLLM(t *testing.T) {
	// LLM 返回错误，但缓存命中时不应调用 LLM
	s := newTestLLMScorer("", errors.New("should not be called"))
	cache := newMockLLMScoreCache()
	cache.textScores["hello"] = 90.0
	s.scoreCache = cache

	score, err := s.ScoreText(context.Background(), "hello", 50.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := s.combineScores(50, 90)
	if score != want {
		t.Errorf("score = %.1f, want %.1f (from cache)", score, want)
	}
}

func TestScoreText_CacheMiss_StoresResult(t *testing.T) {
	s := newTestLLMScorer(`{"score":75}`, nil)
	cache := newMockLLMScoreCache()
	s.scoreCache = cache

	_, err := s.ScoreText(context.Background(), "new text", 50.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 调用后应写入带 provenance 的 typed cache
	result, ok := cache.textResults["new text"]
	if !ok || result == nil {
		t.Fatal("expected score result to be cached after LLM call")
	}
	if result.Score != 75 {
		t.Fatalf("cached score = %.1f, want 75", result.Score)
	}
}

type scorerPromptRegistryStub struct {
	templates map[string]string
}

func (s *scorerPromptRegistryStub) Get(key string, fallback string) string {
	if value, ok := s.templates[key]; ok {
		return value
	}
	return fallback
}

func (s *scorerPromptRegistryStub) Render(key string, vars map[string]any, fallback string) (string, error) {
	if value, ok := s.templates[key]; ok {
		switch key {
		case prompt.KProductEnrichLlmScorerTextScoring:
			if text, ok := vars["Text"].(string); ok {
				value = replacePromptToken(value, "{{.Text}}", text)
			}
		case prompt.KProductEnrichLlmScorerImageScoring:
			// no-op, only checking source selection in tests
		}
		return value, nil
	}
	return fallback, nil
}

func (s *scorerPromptRegistryStub) Keys() []string {
	keys := make([]string, 0, len(s.templates))
	for key := range s.templates {
		keys = append(keys, key)
	}
	return keys
}

func TestScoreTextResult_RegistryHitCarriesPromptMetadata(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &scorerPromptRegistryStub{
		templates: map[string]string{
			prompt.KProductEnrichLlmScorerTextScoring: "Registry text scoring prompt {{.Text}}",
		},
	}
	t.Cleanup(func() { prompt.GlobalRegistry = previous })

	s := newTestLLMScorer(`{"score":88}`, nil)

	result, err := s.scoreTextResult(context.Background(), "sample text", 60)
	if err != nil {
		t.Fatalf("scoreTextResult() error = %v", err)
	}
	if result.Prompt == nil {
		t.Fatal("expected prompt metadata")
	}
	if result.Prompt.PromptKey != prompt.KProductEnrichLlmScorerTextScoring {
		t.Fatalf("PromptKey = %q", result.Prompt.PromptKey)
	}
	if result.Prompt.PromptSource != "registry" {
		t.Fatalf("PromptSource = %q", result.Prompt.PromptSource)
	}
	if result.Prompt.PromptVersion != "default" {
		t.Fatalf("PromptVersion = %q", result.Prompt.PromptVersion)
	}
}

func TestScoreTextResult_RegistryMissFallsBackToPromptMetadata(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &scorerPromptRegistryStub{templates: map[string]string{}}
	t.Cleanup(func() { prompt.GlobalRegistry = previous })

	s := newTestLLMScorer(`{"score":88}`, nil)

	result, err := s.scoreTextResult(context.Background(), "sample text", 60)
	if err != nil {
		t.Fatalf("scoreTextResult() error = %v", err)
	}
	if result.Prompt == nil {
		t.Fatal("expected prompt metadata")
	}
	if result.Prompt.PromptKey != prompt.KProductEnrichLlmScorerTextScoring {
		t.Fatalf("PromptKey = %q", result.Prompt.PromptKey)
	}
	if result.Prompt.PromptSource != "fallback" {
		t.Fatalf("PromptSource = %q", result.Prompt.PromptSource)
	}
	if result.Prompt.PromptVersion != "default" {
		t.Fatalf("PromptVersion = %q", result.Prompt.PromptVersion)
	}
}

func TestScoreImageResult_WhitespaceRegistryRenderFallsBack(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &scorerPromptRegistryStub{
		templates: map[string]string{
			prompt.KProductEnrichLlmScorerImageScoring: "   ",
		},
	}
	t.Cleanup(func() { prompt.GlobalRegistry = previous })

	s := newTestLLMScorer(`{"score":92}`, nil)

	result, err := s.scoreImageResult(context.Background(), "https://example.com/image.jpg", 70)
	if err != nil {
		t.Fatalf("scoreImageResult() error = %v", err)
	}
	if result.Prompt == nil {
		t.Fatal("expected prompt metadata")
	}
	if result.Prompt.PromptKey != prompt.KProductEnrichLlmScorerImageScoring {
		t.Fatalf("PromptKey = %q", result.Prompt.PromptKey)
	}
	if result.Prompt.PromptSource != "fallback" {
		t.Fatalf("PromptSource = %q", result.Prompt.PromptSource)
	}
}

func TestScoreTextResult_CacheHitDoesNotFabricatePromptMetadata(t *testing.T) {
	s := newTestLLMScorer("", errors.New("should not be called"))
	cache := newMockLLMScoreCache()
	cache.textScores["cached text"] = 90
	s.scoreCache = cache

	result, err := s.scoreTextResult(context.Background(), "cached text", 50)
	if err != nil {
		t.Fatalf("scoreTextResult() error = %v", err)
	}
	if result.Prompt != nil {
		t.Fatalf("Prompt = %#v, want nil for cache hit without stored provenance", result.Prompt)
	}
}

func TestScoreTextResult_CacheHitPreservesPromptMetadataWhenAvailable(t *testing.T) {
	s := newTestLLMScorer("", errors.New("should not be called"))
	cache := newMockLLMScoreCache()
	cache.textResults["cached text"] = &CachedLLMScore{
		Score: 90,
		Prompt: &PromptObservability{
			PromptRef:     prompt.KProductEnrichLlmScorerTextScoring,
			PromptKey:     prompt.KProductEnrichLlmScorerTextScoring,
			PromptSource:  "registry",
			PromptVersion: "default",
		},
	}
	s.scoreCache = cache

	result, err := s.scoreTextResult(context.Background(), "cached text", 50)
	if err != nil {
		t.Fatalf("scoreTextResult() error = %v", err)
	}
	if result.Prompt == nil {
		t.Fatal("expected prompt metadata on cache hit")
	}
	if result.Prompt.PromptKey != prompt.KProductEnrichLlmScorerTextScoring {
		t.Fatalf("PromptKey = %q", result.Prompt.PromptKey)
	}
}

func replacePromptToken(value string, token string, replacement string) string {
	for {
		idx := promptTokenIndex(value, token)
		if idx < 0 {
			return value
		}
		value = value[:idx] + replacement + value[idx+len(token):]
	}
}

func promptTokenIndex(value string, needle string) int {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
