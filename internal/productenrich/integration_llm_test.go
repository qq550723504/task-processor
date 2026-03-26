//go:build integration

package productenrich_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"task-processor/internal/productenrich"
)

// =============================================================================
// LLMScorer 集成测试（mock HTTP server 模拟 LLM API）
// =============================================================================

// llmScoreResponse 模拟 LLM 返回的评分 JSON
func llmScoreResponse(score float64) string {
	return fmt.Sprintf(`{"score": %.1f, "reason": "test", "strengths": [], "weaknesses": []}`, score)
}

// newMockLLMServer 创建一个 mock HTTP server，返回固定的 LLM 评分响应
func newMockLLMServer(t *testing.T, score float64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, llmScoreResponse(score))
	}))
}

// mockLLMClientForScorer 直接返回评分 JSON，不走 HTTP
type mockLLMClientForScorer struct {
	score float64
	err   error
}

func (m *mockLLMClientForScorer) Generate(_ context.Context, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return llmScoreResponse(m.score), nil
}

func (m *mockLLMClientForScorer) AnalyzeImage(_ context.Context, _ string, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return llmScoreResponse(m.score), nil
}

type mockLLMManagerForScorer struct {
	client productenrich.LLMClient
}

func (m *mockLLMManagerForScorer) GetClient(_ string) (productenrich.LLMClient, error) {
	return m.client, nil
}

func (m *mockLLMManagerForScorer) GetDefaultClient() productenrich.LLMClient {
	return m.client
}

func TestLLMScorer_Integration(t *testing.T) {
	ctx := context.Background()

	t.Run("ScoreText_no_cache", func(t *testing.T) {
		mgr := &mockLLMManagerForScorer{client: &mockLLMClientForScorer{score: 80.0}}
		scorer := productenrich.NewLLMScorer(&productenrich.LLMScorerConfig{
			LLMManager:     mgr,
			FallbackWeight: 0.3,
			MaxRetries:     1,
		})

		// baseScore=60, llmScore=80 → 60*0.7 + 80*0.3 = 42+24 = 66
		got, err := scorer.ScoreText(ctx, "高品质蓝牙耳机，支持主动降噪", 60.0)
		require.NoError(t, err)
		assert.InDelta(t, 66.0, got, 0.1)
	})

	t.Run("ScoreImage_no_cache", func(t *testing.T) {
		mgr := &mockLLMManagerForScorer{client: &mockLLMClientForScorer{score: 90.0}}
		scorer := productenrich.NewLLMScorer(&productenrich.LLMScorerConfig{
			LLMManager:     mgr,
			FallbackWeight: 0.5,
			MaxRetries:     1,
		})

		// baseScore=70, llmScore=90 → 70*0.5 + 90*0.5 = 35+45 = 80
		got, err := scorer.ScoreImage(ctx, "https://example.com/img.jpg", 70.0)
		require.NoError(t, err)
		assert.InDelta(t, 80.0, got, 0.1)
	})

	t.Run("ScoreText_with_redis_cache", func(t *testing.T) {
		suite, cleanup := setupSuite(t)
		defer cleanup()

		scoreCache := productenrich.NewLLMScoreCache(suite.redisClient, nil)
		mgr := &mockLLMManagerForScorer{client: &mockLLMClientForScorer{score: 75.0}}
		scorer := productenrich.NewLLMScorer(&productenrich.LLMScorerConfig{
			LLMManager:     mgr,
			ScoreCache:     scoreCache,
			FallbackWeight: 0.3,
			CacheTTL:       time.Minute,
			MaxRetries:     1,
		})

		text := "缓存测试文本"
		// 第一次调用：LLM 计算并写入缓存
		score1, err := scorer.ScoreText(ctx, text, 50.0)
		require.NoError(t, err)

		// 第二次调用：命中缓存，结果应相同
		score2, err := scorer.ScoreText(ctx, text, 50.0)
		require.NoError(t, err)
		assert.InDelta(t, score1, score2, 0.01, "cached score should match")
	})

	t.Run("ScoreText_empty_text_returns_base", func(t *testing.T) {
		mgr := &mockLLMManagerForScorer{client: &mockLLMClientForScorer{score: 80.0}}
		scorer := productenrich.NewLLMScorer(&productenrich.LLMScorerConfig{
			LLMManager: mgr,
			MaxRetries: 1,
		})

		got, err := scorer.ScoreText(ctx, "", 55.0)
		require.NoError(t, err)
		assert.InDelta(t, 55.0, got, 0.01, "empty text should return base score")
	})

	t.Run("ScoreText_llm_error_returns_base_score_with_error", func(t *testing.T) {
		mgr := &mockLLMManagerForScorer{
			client: &mockLLMClientForScorer{err: fmt.Errorf("LLM unavailable")},
		}
		scorer := productenrich.NewLLMScorer(&productenrich.LLMScorerConfig{
			LLMManager: mgr,
			MaxRetries: 1,
		})

		got, err := scorer.ScoreText(ctx, "some text", 60.0)
		assert.Error(t, err)
		assert.InDelta(t, 60.0, got, 0.01, "on LLM error, should return base score")
	})
}

// =============================================================================
// VariantGenerator 集成测试（mock LLM）
// =============================================================================

// variantLLMClient 返回预设的 JSON 响应
type variantLLMClient struct {
	specsJSON    string
	variantsJSON string
	dimJSON      string
	weightJSON   string
}

func (m *variantLLMClient) Generate(_ context.Context, prompt string) (string, error) {
	p := strings.ToLower(prompt)
	switch {
	case strings.Contains(p, "specifications") || strings.Contains(p, "dimensions") && strings.Contains(p, "weight") && strings.Contains(p, "package"):
		return m.specsJSON, nil
	case strings.Contains(p, "variants") || strings.Contains(p, "skus"):
		return m.variantsJSON, nil
	case strings.Contains(p, "dimensions"):
		return m.dimJSON, nil
	case strings.Contains(p, "weight"):
		return m.weightJSON, nil
	default:
		return "{}", nil
	}
}

func (m *variantLLMClient) AnalyzeImage(_ context.Context, _ string, _ string) (string, error) {
	return "{}", nil
}

type variantLLMManager struct {
	client productenrich.LLMClient
}

func (m *variantLLMManager) GetClient(_ string) (productenrich.LLMClient, error) {
	return m.client, nil
}

func (m *variantLLMManager) GetDefaultClient() productenrich.LLMClient {
	return m.client
}

func TestVariantGenerator_Integration(t *testing.T) {
	ctx := context.Background()

	specsJSON := `{
		"dimensions": {"length": 20.0, "width": 10.0, "height": 5.0, "unit": "cm"},
		"weight": {"value": 0.5, "unit": "kg"}
	}`

	variantsJSON := `[
		{"sku": "BT-001-BLACK", "attributes": {"color": "黑色"}, "price": {"currency": "CNY", "amount": 299.0}, "stock": 100, "is_default": true},
		{"sku": "BT-001-WHITE", "attributes": {"color": "白色"}, "price": {"currency": "CNY", "amount": 299.0}, "stock": 50, "is_default": false}
	]`

	dimJSON := `{"length": 15.0, "width": 8.0, "height": 3.0, "unit": "cm"}`
	weightJSON := `{"value": 0.3, "unit": "kg"}`

	client := &variantLLMClient{
		specsJSON:    specsJSON,
		variantsJSON: variantsJSON,
		dimJSON:      dimJSON,
		weightJSON:   weightJSON,
	}
	mgr := &variantLLMManager{client: client}

	gen, err := productenrich.NewVariantGenerator(mgr)
	require.NoError(t, err)

	analysis := &productenrich.ProductAnalysis{
		Representation: &productenrich.ProductRepresentation{
			ProductType: "electronics",
			Attributes:  map[string]string{"name": "蓝牙耳机", "category": "电子产品"},
			Features:    []string{"主动降噪", "30小时续航"},
		},
	}

	t.Run("GenerateSpecs", func(t *testing.T) {
		specs, err := gen.GenerateSpecs(ctx, analysis)
		require.NoError(t, err)
		require.NotNil(t, specs)
		assert.NotNil(t, specs.Dimensions)
		assert.InDelta(t, 20.0, specs.Dimensions.Length, 0.01)
		assert.Equal(t, "cm", specs.Dimensions.Unit)
		assert.NotNil(t, specs.Weight)
		assert.InDelta(t, 0.5, specs.Weight.Value, 0.01)
	})

	t.Run("GenerateVariants", func(t *testing.T) {
		variants, err := gen.GenerateVariants(ctx, analysis)
		require.NoError(t, err)
		require.Len(t, variants, 2)

		// 验证默认变体
		hasDefault := false
		for _, v := range variants {
			if v.IsDefault {
				hasDefault = true
				assert.Equal(t, "BT-001-BLACK", v.SKU)
			}
		}
		assert.True(t, hasDefault, "should have a default variant")
	})

	t.Run("ExtractDimensions", func(t *testing.T) {
		dims, err := gen.ExtractDimensions(ctx, "产品尺寸：长15cm 宽8cm 高3cm")
		require.NoError(t, err)
		require.NotNil(t, dims)
		assert.InDelta(t, 15.0, dims.Length, 0.01)
		assert.Equal(t, "cm", dims.Unit)
	})

	t.Run("ExtractWeight", func(t *testing.T) {
		w, err := gen.ExtractWeight(ctx, "产品重量：300g")
		require.NoError(t, err)
		require.NotNil(t, w)
		assert.InDelta(t, 0.3, w.Value, 0.01)
		assert.Equal(t, "kg", w.Unit)
	})

	t.Run("GenerateSpecs_nil_analysis_returns_error", func(t *testing.T) {
		_, err := gen.GenerateSpecs(ctx, nil)
		assert.Error(t, err)
	})

	t.Run("GenerateVariants_nil_analysis_returns_error", func(t *testing.T) {
		_, err := gen.GenerateVariants(ctx, nil)
		assert.Error(t, err)
	})

	t.Run("ExtractDimensions_empty_text_returns_error", func(t *testing.T) {
		_, err := gen.ExtractDimensions(ctx, "")
		assert.Error(t, err)
	})

	t.Run("ExtractWeight_empty_text_returns_error", func(t *testing.T) {
		_, err := gen.ExtractWeight(ctx, "")
		assert.Error(t, err)
	})

	t.Run("NewVariantGenerator_nil_manager_returns_error", func(t *testing.T) {
		_, err := productenrich.NewVariantGenerator(nil)
		assert.Error(t, err)
	})
}

// =============================================================================
// 辅助：验证 variantsJSON 中的 price 字段
// =============================================================================

func TestVariantGenerator_Integration_VariantPrice(t *testing.T) {
	ctx := context.Background()

	variantsJSON := `[{"sku": "P-001", "attributes": {"color": "红色"}, "price": {"currency": "CNY", "amount": 199.0}, "stock": 10, "is_default": true}]`
	client := &variantLLMClient{variantsJSON: variantsJSON}
	mgr := &variantLLMManager{client: client}

	gen, err := productenrich.NewVariantGenerator(mgr)
	require.NoError(t, err)

	analysis := &productenrich.ProductAnalysis{
		Representation: &productenrich.ProductRepresentation{
			ProductType: "electronics",
			Attributes:  map[string]string{"name": "测试产品"},
		},
	}

	variants, err := gen.GenerateVariants(ctx, analysis)
	require.NoError(t, err)
	require.Len(t, variants, 1)

	// 验证价格货币为 CNY
	priceBytes, err := json.Marshal(variants[0].Price)
	require.NoError(t, err)
	assert.Contains(t, string(priceBytes), "CNY")
}
