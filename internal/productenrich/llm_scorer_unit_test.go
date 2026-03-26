package productenrich

import (
	"context"
	"errors"
	"testing"
	"time"
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
	textScores  map[string]float64
	imageScores map[string]float64
}

func newMockLLMScoreCache() *mockLLMScoreCache {
	return &mockLLMScoreCache{
		textScores:  make(map[string]float64),
		imageScores: make(map[string]float64),
	}
}

func (m *mockLLMScoreCache) GetTextScore(_ context.Context, text string) (float64, bool) {
	v, ok := m.textScores[text]
	return v, ok
}
func (m *mockLLMScoreCache) SetTextScore(_ context.Context, text string, score float64, _ time.Duration) error {
	m.textScores[text] = score
	return nil
}
func (m *mockLLMScoreCache) GetImageScore(_ context.Context, url string) (float64, bool) {
	v, ok := m.imageScores[url]
	return v, ok
}
func (m *mockLLMScoreCache) SetImageScore(_ context.Context, url string, score float64, _ time.Duration) error {
	m.imageScores[url] = score
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
	// 调用后应写入缓存
	if _, ok := cache.textScores["new text"]; !ok {
		t.Error("expected score to be cached after LLM call")
	}
}
