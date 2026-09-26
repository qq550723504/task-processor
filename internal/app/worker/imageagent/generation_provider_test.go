package imageagentworker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	openai "task-processor/internal/integration/openai"
)

type generationConfigTestResolver struct{ resolved *openai.ResolvedClientConfig }

func (r *generationConfigTestResolver) ResolveClientConfig(context.Context, string, *openai.ClientConfig) (*openai.ResolvedClientConfig, error) {
	return r.resolved, nil
}

func TestGenerationProviderFactoryUsesPinnedGRSAIProtocol(t *testing.T) {
	var pngBytes bytes.Buffer
	require.NoError(t, png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	dispatches, observed, downloads := 0, 0, 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dispatches++
		require.Equal(t, "/v1/api/generate", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "gpt-image-2.5", body["model"])
		require.Equal(t, "json", body["replyType"])
		_, _ = w.Write([]byte(`{"id":"response-1","status":"succeeded","results":[{"url":"https://output.example/image.png"}]}`))
	}))
	defer server.Close()
	resolver := &generationConfigTestResolver{resolved: &openai.ResolvedClientConfig{CacheKey: "db:1:1:image_gpt_image_2", Config: &openai.ClientConfig{APIKey: "controlled-only", APIStyle: "grsai", Model: "gpt-image-2.5", BaseURL: server.URL, Timeout: time.Second}}}
	factory := generationProviderFactory{resolver: resolver, price: config.ImageAgentGenerationConfig{PriceVersion: "test-price", PointsPerImage: 17}, profile: synchronousEditProfile{}, providerHTTPClient: server.Client(), generatedFetcher: func(context.Context, string) ([]byte, error) {
		downloads++
		require.Equal(t, 1, observed)
		return pngBytes.Bytes(), nil
	}}
	prepared, err := factory.prepare(context.Background(), func(_ context.Context, proof imageagent.GenerationSuccess) error {
		observed++
		require.Equal(t, "response-1", proof.ResponseID)
		require.False(t, proof.UsageKnown)
		return nil
	})
	require.NoError(t, err)
	require.Zero(t, dispatches)
	require.EqualValues(t, 17, prepared.Metadata.Points)
	sum := sha256.Sum256(pngBytes.Bytes())
	input := imageagent.SlotExecutionInput{RunID: "run-1", IdempotencyKey: "attempt-key", TenantID: "org-1", UserID: "user-1", PlanRevision: 1, Attempt: 1, TargetPlatform: "product", ImagePolicyContext: &imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}, Slot: imageagent.Slot{ID: "main-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key"}, AssetCatalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/image.png"}}}, ProductContext: imageagent.ProductContextRef{ProductID: "product-1", Title: "Product"}, SourceBytes: pngBytes.Bytes(), SourceDigest: hex.EncodeToString(sum[:])}
	quote, err := prepared.Executor.QuoteSlot(context.Background(), input, imageagent.BudgetPolicy{})
	require.NoError(t, err)
	require.Len(t, quote.Operations, 1)
	require.Equal(t, "grsai", quote.Operations[0].Provider)
	result, err := prepared.Executor.GenerateQuotedSlot(context.Background(), input, quote)
	require.NoError(t, err)
	require.Len(t, result.Assets, 1)
	require.Equal(t, 1, dispatches)
	require.Equal(t, 1, downloads)
	require.NoError(t, factory.revalidate(context.Background(), prepared.Metadata))
	resolver.resolved.CacheKey = "db:1:2:image_gpt_image_2"
	require.ErrorIs(t, factory.revalidate(context.Background(), prepared.Metadata), imageagent.ErrRevisionConflict)
	require.Equal(t, 1, dispatches, "route checks never submit")
}

func TestGenerationProviderFactoryMissingPriceOrWrongRouteCannotBecomeAvailable(t *testing.T) {
	for _, mode := range []string{"no_price", "empty_version", "negative_price", "wrong_model", "wrong_style", "no_credential", "plain_http", "query_secret", "wrong_path", "unbounded_timeout"} {
		t.Run(mode, func(t *testing.T) {
			cfg := &openai.ClientConfig{APIKey: "controlled-only", Model: "gpt-image-2.5", APIStyle: "grsai", BaseURL: "https://provider.example/v1", Timeout: time.Minute}
			price := config.ImageAgentGenerationConfig{PriceVersion: "p1", PointsPerImage: 17}
			switch mode {
			case "no_price":
				price.PointsPerImage = 0
			case "empty_version":
				price.PriceVersion = ""
			case "negative_price":
				price.PointsPerImage = -1
			case "wrong_model":
				cfg.Model = "nano-banana-fast"
			case "wrong_style":
				cfg.APIStyle = "openai-compatible"
			case "no_credential":
				cfg.APIKey = ""
			case "plain_http":
				cfg.BaseURL = "http://provider.example"
			case "query_secret":
				cfg.BaseURL = "https://provider.example/?key=must-not-log"
			case "wrong_path":
				cfg.BaseURL = "https://provider.example/v1/images/edits"
			case "unbounded_timeout":
				cfg.Timeout = 0
			}
			factory := generationProviderFactory{resolver: &generationConfigTestResolver{resolved: &openai.ResolvedClientConfig{Config: cfg, CacheKey: "db:1:1:profile"}}, price: price, profile: synchronousEditProfile{}}
			_, err := factory.prepare(context.Background(), func(context.Context, imageagent.GenerationSuccess) error { return nil })
			require.ErrorIs(t, err, imageagent.ErrBudgetQuoteUnavailable)
			require.NotContains(t, err.Error(), "must-not-log")
			require.NotContains(t, err.Error(), "controlled-only")
		})
	}
}
