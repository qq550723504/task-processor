package imageagentworker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/ai"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/tools"
	"task-processor/internal/integration/grsai"
	openai "task-processor/internal/integration/openai"
	"task-processor/internal/marketplace/imagepolicy"
	productimage "task-processor/internal/product/image"
)

func TestSourceOnlyGRSAIAdapterRetainsProvenanceButSendsOnlyExactBytes(t *testing.T) {
	for _, mode := range []string{"success", "download_failure", "observer_failure"} {
		t.Run(mode, func(t *testing.T) {
			var encoded bytes.Buffer
			require.NoError(t, png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))))
			submits, observations, downloads := 0, 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				submits++
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/v1/api/generate", r.URL.Path)
				var body struct {
					Images []string `json:"images"`
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Equal(t, []string{"data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())}, body.Images)
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "generation-1", "status": "succeeded", "results": []map[string]string{{"url": "https://output.example/result.png"}}})
			}))
			defer server.Close()
			client := grsai.NewClient(grsai.Config{Model: "gpt-image-2.5", SubmitURL: server.URL, HTTPClient: server.Client()})
			provider, err := grsai.NewSynchronousProductImageAdapter(grsai.ProductImageAdapterConfig{
				Client: client, ImageModel: "gpt-image-2.5", RouteReference: "route-1", CredentialReference: "credential-1", ConfigurationVersion: "config-1",
				Build: func(bound ai.RouteBoundImageGenerator) (grsai.ProductImageProvider, error) {
					return openai.NewProductImageAdapter(openai.ProductImageAdapterConfig{ImageClient: bound, Provider: "grsai", ImageModel: "gpt-image-2.5", RouteReference: "route-1", CredentialReference: "credential-1", ConfigurationVersion: "config-1", PricingVersion: "test-only-unpriced", Prompts: openai.DefaultProductImagePrompts(), MaximumSceneOutputs: 1, GeneratedImageFetcher: func(context.Context, string) ([]byte, error) {
						downloads++
						require.Equal(t, 1, observations)
						if mode == "download_failure" {
							return nil, errors.New("download failed")
						}
						return encoded.Bytes(), nil
					}})
				},
			}, func(context.Context, grsai.GenerationObservation) error {
				observations++
				if mode == "observer_failure" {
					return errors.New("DB failed")
				}
				return nil
			})
			require.NoError(t, err)
			capability, err := productimage.NewWhiteBackgroundCapability(provider)
			require.NoError(t, err)
			executor := tools.NewProductImageSlotExecutor(tools.Dependencies{WhiteBackgroundRenderer: capability, ProfileResolver: synchronousEditProfile{}})
			digest := sha256.Sum256(encoded.Bytes())
			input := imageagent.SlotExecutionInput{
				RunID: "run-1", TenantID: "org-1", UserID: "user-1", TargetPlatform: "product", PlanRevision: 1, Attempt: 1, IdempotencyKey: "attempt-1",
				ImagePolicyContext: &imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"},
				Slot:               imageagent.Slot{ID: "main-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"catalog-image-1"}, IdempotencyKey: "slot-1"},
				AssetCatalog:       imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "catalog-image-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/original.png", SourceURL: "https://source.example/original.png", Width: 2, Height: 2}}},
				ProductContext:     imageagent.ProductContextRef{ProductID: "product-1", Title: "Product"}, SourceBytes: encoded.Bytes(), SourceDigest: hex.EncodeToString(digest[:]),
			}
			result, err := executor.GenerateSlot(context.Background(), input)
			require.Equal(t, 1, submits, "render error: %v", err)
			require.Equal(t, 1, observations)
			if mode != "success" {
				require.Error(t, err)
				require.Empty(t, result.Assets)
			} else {
				require.NoError(t, err)
				require.Equal(t, "https://source.example/original.png", result.Assets[0].SourceURL)
				require.Equal(t, "catalog-image-1", result.SourceAssetID)
			}
			if mode == "observer_failure" {
				require.Zero(t, downloads)
			} else {
				require.Equal(t, 1, downloads)
			}
		})
	}
}

type synchronousEditProfile struct{}

func (synchronousEditProfile) Resolve(input imagepolicy.ProfileInput) (imagepolicy.ProductImageProfile, error) {
	return imagepolicy.ProductImageProfile{Key: imagepolicy.PolicyKey(input), PolicyVersion: "test-v1"}, nil
}
