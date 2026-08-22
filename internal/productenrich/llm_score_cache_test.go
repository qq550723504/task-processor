package productenrich

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/aicapability"
	"task-processor/internal/pkg/hashx"
)

func TestLLMScoreCacheGovernedNamespaceDoesNotReadLegacyEntries(t *testing.T) {
	redis := newMockRedisForCache()
	redis.store["llm_score:text:"+hashx.MD5("same input")] = `{"score":99}`
	cache := NewLLMScoreCache(redis, nil)
	identity := governedCacheTestIdentity()

	_, found := cache.GetGovernedScoreResult(context.Background(), identity)

	require.False(t, found)
}

func TestLLMScoreCacheStoresGovernedResultsOnlyUnderVersionedIdentity(t *testing.T) {
	redis := newMockRedisForCache()
	cache := NewLLMScoreCache(redis, nil)
	identity := governedCacheTestIdentity()
	expected := &CachedLLMScore{Score: 91, Prompt: &PromptObservability{PromptKey: identity.PromptKey, PromptVersion: identity.PromptVersion}}

	require.NoError(t, cache.SetGovernedScoreResult(context.Background(), identity, expected, time.Hour))
	actual, found := cache.GetGovernedScoreResult(context.Background(), identity)

	require.True(t, found)
	require.Equal(t, expected, actual)
	require.Contains(t, redis.store, identity.Key())
	require.Len(t, redis.store, 1)
}

func governedCacheTestIdentity() ScoreCacheIdentity {
	return ScoreCacheIdentity{
		Version: 1, TenantID: "tenant-a",
		Capability: aicapability.CapabilityProductEnrichText,
		Operation:  aicapability.OperationProductEnrichTextQualityScore,
		RouteMode:  aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
		ProviderID: "openai", ModelID: "gpt-4.1-mini", RoutingKey: "fast",
		PolicyVersion: "policy-v1", ConfigurationVersion: "config-v1",
		PromptKey: "productenrich.llm_scorer.text_scoring", PromptVersion: "prompt-v17", PromptScope: "product_enrich",
		BaseScore: "80", InputHash: "input-hash",
	}
}
