package enrich

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"task-processor/internal/aicapability"
	productenrich "task-processor/internal/productenrich"
	"task-processor/internal/shared/aiidentity"
)

func TestGovernedScorerLifecycleRetriesValidatesAndRecordsExactlyOnce(t *testing.T) {
	tests := []struct {
		name string
		kind string
	}{
		{name: "text", kind: "text"},
		{name: "image", kind: "image"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("transient failure then valid response", func(t *testing.T) {
				h := newGovernedScoreLifecycleHarness(t, tt.kind, []governedScoreProviderResult{
					{err: aicapability.NewError(aicapability.ErrorRateLimited, "score", errors.New("retry later"))},
					{response: `{"score":90}`},
				})

				score, err := h.score(context.Background())
				if err != nil || score != 62 {
					t.Fatalf("score = %.1f, err = %v, want 62 and nil", score, err)
				}
				h.assertProviderCalls(t, 2)
				h.assertSingleRecord(t, aicapability.InvocationSucceeded, "", 2)
				if h.cache.writeCount != 1 {
					t.Fatalf("cache writes = %d, want 1", h.cache.writeCount)
				}
			})

			t.Run("all transient attempts fail", func(t *testing.T) {
				h := newGovernedScoreLifecycleHarness(t, tt.kind, []governedScoreProviderResult{
					{err: aicapability.NewError(aicapability.ErrorRateLimited, "score", errors.New("retry later"))},
					{err: aicapability.NewError(aicapability.ErrorProviderTimeout, "score", context.DeadlineExceeded)},
				})

				score, err := h.score(context.Background())
				if err == nil || score != 50 {
					t.Fatalf("score = %.1f, err = %v, want base score and terminal error", score, err)
				}
				h.assertProviderCalls(t, 2)
				h.assertSingleRecord(t, aicapability.InvocationFailed, aicapability.ErrorProviderTimeout, 2)
				if h.cache.writeCount != 0 {
					t.Fatalf("cache writes = %d, want 0", h.cache.writeCount)
				}
			})

			t.Run("malformed response fails without retry", func(t *testing.T) {
				const malformed = `{"reason":"missing score"}`
				h := newGovernedScoreLifecycleHarness(t, tt.kind, []governedScoreProviderResult{{response: malformed}})

				score, err := h.score(context.Background())
				if err == nil || score != 50 {
					t.Fatalf("score = %.1f, err = %v, want base score and validation error", score, err)
				}
				h.assertProviderCalls(t, 1)
				h.assertSingleRecord(t, aicapability.InvocationFailed, aicapability.ErrorInvalidProviderResponse, 1)
				record := h.recorder.snapshot()[0]
				if record.OutputHash != hashText(malformed) {
					t.Fatalf("output hash = %q, want malformed response hash", record.OutputHash)
				}
				if h.cache.writeCount != 0 {
					t.Fatalf("cache writes = %d, want 0", h.cache.writeCount)
				}
			})

			t.Run("valid first response", func(t *testing.T) {
				h := newGovernedScoreLifecycleHarness(t, tt.kind, []governedScoreProviderResult{{response: `{"score":90}`}})

				score, err := h.score(context.Background())
				if err != nil || score != 62 {
					t.Fatalf("score = %.1f, err = %v, want 62 and nil", score, err)
				}
				h.assertProviderCalls(t, 1)
				h.assertSingleRecord(t, aicapability.InvocationSucceeded, "", 1)
				if h.cache.writeCount != 1 {
					t.Fatalf("cache writes = %d, want 1", h.cache.writeCount)
				}
			})

			t.Run("context cancellation during backoff", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				h := newGovernedScoreLifecycleHarness(t, tt.kind, []governedScoreProviderResult{
					{err: aicapability.NewError(aicapability.ErrorRateLimited, "score", errors.New("retry later")), after: func() { time.AfterFunc(10*time.Millisecond, cancel) }},
					{response: `{"score":90}`},
				})

				started := time.Now()
				score, err := h.score(ctx)
				if !errors.Is(err, context.Canceled) || score != 50 {
					t.Fatalf("score = %.1f, err = %v, want base score and context cancellation", score, err)
				}
				if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
					t.Fatalf("cancellation took %v, want prompt backoff interruption", elapsed)
				}
				h.assertProviderCalls(t, 1)
				h.assertSingleRecord(t, aicapability.InvocationFailed, aicapability.ErrorProviderUnavailable, 1)
				if h.cache.writeCount != 0 {
					t.Fatalf("cache writes = %d, want 0", h.cache.writeCount)
				}
			})

			t.Run("cache hit skips provider and records hit once", func(t *testing.T) {
				h := newGovernedScoreLifecycleHarness(t, tt.kind, []governedScoreProviderResult{{response: `{"score":90}`}})
				if score, err := h.score(context.Background()); err != nil || score != 62 {
					t.Fatalf("prime score = %.1f, err = %v", score, err)
				}
				beforeCalls := h.manager.callCount()
				beforeRecords := len(h.recorder.snapshot())

				score, err := h.score(context.Background())
				if err != nil || score != 62 {
					t.Fatalf("cached score = %.1f, err = %v", score, err)
				}
				if h.manager.callCount() != beforeCalls {
					t.Fatalf("provider calls changed on cache hit: %d -> %d", beforeCalls, h.manager.callCount())
				}
				records := h.recorder.snapshot()
				if len(records) != beforeRecords+1 || records[len(records)-1].CacheStatus != aicapability.CacheStatusHit {
					t.Fatalf("records after cache hit = %+v", records)
				}
			})
		})
	}
}

type governedScoreLifecycleHarness struct {
	kind     string
	scorer   productenrich.LLMScorer
	manager  *governedScoreSequenceManager
	recorder *governedScoreLifecycleRecorder
	cache    *governedScoreLifecycleCache
}

func newGovernedScoreLifecycleHarness(t *testing.T, kind string, results []governedScoreProviderResult) *governedScoreLifecycleHarness {
	t.Helper()
	manager := &governedScoreSequenceManager{results: append([]governedScoreProviderResult(nil), results...)}
	recorder := &governedScoreLifecycleRecorder{}
	cache := &governedScoreLifecycleCache{governed: map[string]*productenrich.CachedLLMScore{}}
	config := &productenrich.LLMScorerConfig{ScoreCache: cache, MaxRetries: 2, FallbackWeight: 0.3}
	if kind == "text" {
		generator, err := NewGovernedTextGenerator(manager, GovernedTextGeneratorConfig{
			Planner: governedScoreLifecyclePlanner{}, LegacyRouteMetadata: governedScoreLegacyMetadata{}, Recorder: recorder,
			Capability: aicapability.CapabilityProductEnrichText, Operation: aicapability.OperationProductEnrichTextQualityScore,
			RequiredFeature: aicapability.FeatureTextGenerate,
		})
		if err != nil {
			t.Fatalf("NewGovernedTextGenerator: %v", err)
		}
		config.TextGenerator = generator
	} else {
		analyzer, err := NewGovernedImageAnalyzer(manager, GovernedImageAnalyzerConfig{
			Planner: governedScoreLifecyclePlanner{}, LegacyRouteMetadata: governedScoreLegacyMetadata{}, Recorder: recorder,
			Capability: aicapability.CapabilityProductEnrichVision, Operation: aicapability.OperationProductEnrichVisionQualityScore,
			RequiredFeature: aicapability.FeatureVisionAnalyze,
		})
		if err != nil {
			t.Fatalf("NewGovernedImageAnalyzer: %v", err)
		}
		config.ImageAnalyzer = analyzer
	}
	return &governedScoreLifecycleHarness{kind: kind, scorer: productenrich.NewLLMScorer(config), manager: manager, recorder: recorder, cache: cache}
}

func (h *governedScoreLifecycleHarness) score(ctx context.Context) (float64, error) {
	ctx = aiidentity.WithIdentity(ctx, aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	if h.kind == "text" {
		return h.scorer.ScoreText(ctx, "product text", 50)
	}
	return h.scorer.ScoreImage(ctx, "https://example.test/product.jpg", 50)
}

func (h *governedScoreLifecycleHarness) assertProviderCalls(t *testing.T, want int) {
	t.Helper()
	if got := h.manager.callCount(); got != want {
		t.Fatalf("provider calls = %d, want %d", got, want)
	}
}

func (h *governedScoreLifecycleHarness) assertSingleRecord(t *testing.T, outcome aicapability.InvocationOutcome, category aicapability.ErrorCategory, attempt int) {
	t.Helper()
	records := h.recorder.snapshot()
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1: %+v", len(records), records)
	}
	record := records[0]
	if record.Outcome != outcome || record.ErrorCategory != category || record.Attempt != attempt || record.CacheStatus != aicapability.CacheStatusMiss {
		t.Fatalf("record = %+v", record)
	}
}

type governedScoreProviderResult struct {
	response string
	err      error
	after    func()
}

type governedScoreSequenceManager struct {
	mu      sync.Mutex
	results []governedScoreProviderResult
	calls   int
}

func (*governedScoreSequenceManager) GetClient(string) (productenrich.LLMClient, error) {
	return nil, errors.New("legacy client lookup not expected")
}

func (*governedScoreSequenceManager) GetDefaultClient() productenrich.LLMClient { return nil }

func (m *governedScoreSequenceManager) GetClientWithRoute(context.Context, string, productenrich.LLMClientRoute) (productenrich.LLMClient, error) {
	return governedScoreSequenceClient{manager: m}, nil
}

func (m *governedScoreSequenceManager) next() (string, error) {
	m.mu.Lock()
	index := m.calls
	m.calls++
	var result governedScoreProviderResult
	if index < len(m.results) {
		result = m.results[index]
	} else {
		result.err = errors.New("unexpected extra provider call")
	}
	m.mu.Unlock()
	if result.after != nil {
		result.after()
	}
	return result.response, result.err
}

func (m *governedScoreSequenceManager) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

type governedScoreSequenceClient struct{ manager *governedScoreSequenceManager }

func (c governedScoreSequenceClient) Generate(context.Context, string) (string, error) {
	return c.manager.next()
}

func (c governedScoreSequenceClient) AnalyzeImage(context.Context, string, string) (string, error) {
	return c.manager.next()
}

type governedScoreLifecyclePlanner struct{}

func (governedScoreLifecyclePlanner) Plan(_ context.Context, request aicapability.RouteRequest) (aicapability.ExecutionPlan, error) {
	return aicapability.ExecutionPlan{
		Mode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
		Decision: aicapability.RouteDecision{
			Capability: request.Capability, Operation: request.Operation,
			ProviderID: "openai", ModelID: "score-model", RoutingKey: "scorer", CredentialReference: "scorer",
			PolicyVersion: "policy-v1", ConfigurationVersion: "config-v1",
		},
	}, nil
}

type governedScoreLegacyMetadata struct{}

func (governedScoreLegacyMetadata) ResolveLegacyRoute(context.Context, string) (aicapability.RouteDecision, error) {
	return aicapability.RouteDecision{}, errors.New("legacy metadata not expected")
}

type governedScoreLifecycleRecorder struct {
	mu      sync.Mutex
	records []aicapability.InvocationRecord
}

func (r *governedScoreLifecycleRecorder) RecordInvocation(_ context.Context, record aicapability.InvocationRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record)
	return nil
}

func (r *governedScoreLifecycleRecorder) snapshot() []aicapability.InvocationRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]aicapability.InvocationRecord(nil), r.records...)
}

type governedScoreLifecycleCache struct {
	governed   map[string]*productenrich.CachedLLMScore
	writeCount int
}

func (c *governedScoreLifecycleCache) GetGovernedScoreResult(_ context.Context, identity productenrich.ScoreCacheIdentity) (*productenrich.CachedLLMScore, bool) {
	result, ok := c.governed[identity.Key()]
	return result, ok
}

func (c *governedScoreLifecycleCache) SetGovernedScoreResult(_ context.Context, identity productenrich.ScoreCacheIdentity, result *productenrich.CachedLLMScore, _ time.Duration) error {
	c.writeCount++
	c.governed[identity.Key()] = result
	return nil
}

func (*governedScoreLifecycleCache) GetTextScore(context.Context, string) (float64, bool) {
	return 0, false
}
func (*governedScoreLifecycleCache) GetTextScoreResult(context.Context, string) (*productenrich.CachedLLMScore, bool) {
	return nil, false
}
func (*governedScoreLifecycleCache) SetTextScore(context.Context, string, float64, time.Duration) error {
	return nil
}
func (*governedScoreLifecycleCache) SetTextScoreResult(context.Context, string, *productenrich.CachedLLMScore, time.Duration) error {
	return nil
}
func (*governedScoreLifecycleCache) GetImageScore(context.Context, string) (float64, bool) {
	return 0, false
}
func (*governedScoreLifecycleCache) GetImageScoreResult(context.Context, string) (*productenrich.CachedLLMScore, bool) {
	return nil, false
}
func (*governedScoreLifecycleCache) SetImageScore(context.Context, string, float64, time.Duration) error {
	return nil
}
func (*governedScoreLifecycleCache) SetImageScoreResult(context.Context, string, *productenrich.CachedLLMScore, time.Duration) error {
	return nil
}

var _ productenrich.RoutedLLMManager = (*governedScoreSequenceManager)(nil)
var _ productenrich.LLMScoreCache = (*governedScoreLifecycleCache)(nil)
