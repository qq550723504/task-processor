package imageagentworker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"task-processor/internal/aicapability/store"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/tools"
	openai "task-processor/internal/integration/openai"
	"task-processor/internal/listingsubscription"
)

func TestOrganizationMainControlledProviderSettlesOnlyObservedReviewTokens(t *testing.T) {
	db := admissionPostgres(t, "org-1", "member-1", 10000)
	require.NoError(t, db.Exec(`UPDATE saas_tenant_entitlements SET limits = ? WHERE tenant_id = ?`, `{"ai_tokens":1000000}`, "org-1").Error)
	require.NoError(t, db.AutoMigrate(&openai.AIClientCredential{}))
	require.NoError(t, store.AutoMigrateInvocationLedger(db))
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	var pngBody bytes.Buffer
	require.NoError(t, png.Encode(&pngBody, img))
	imageBytes := pngBody.Bytes()
	var imageCalls, reviewCalls, unexpectedCalls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/images/edits":
			imageCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(imageBytes)}}})
		case "/v1/chat/completions":
			reviewCalls.Add(1)
			_, _ = io.WriteString(w, `{"id":"controlled-review","choices":[{"message":{"role":"assistant","content":"{\"score\":0.9,\"needs_human_review\":false,\"reasons\":[]}"}}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`)
		default:
			unexpectedCalls.Add(1)
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()
	credentials := openai.NewGormCredentialResolver(db)
	for _, name := range []string{"default", "image_gpt_image_2"} {
		require.NoError(t, credentials.SaveCredential(context.Background(), openai.AIClientCredential{TenantID: "org-1", UserID: "actor-1", ClientName: name, APIKey: "controlled-local-only", BaseURL: provider.URL + "/v1", Model: "review-test", Enabled: true, TimeoutSecond: 2}))
	}
	reference := &http.Client{Transport: controlledImageReference{body: imageBytes}}
	defaultConfig := openai.NewClientConfig("unused", "review-test", provider.URL+"/v1", 2)
	imageConfig := openai.NewClientConfig("unused", "image-test", provider.URL+"/v1", 2)
	defaultConfig.ImageReferenceHTTPClient, imageConfig.ImageReferenceHTTPClient = reference, reference
	manager, err := openai.NewManager(&openai.ManagerConfig{Clients: map[string]*openai.ClientConfig{"default": defaultConfig, "image_gpt_image_2": imageConfig}, DefaultClient: "default"})
	require.NoError(t, err)
	recorder := store.NewGormInvocationRecorder(db)
	recorder.SetUsageSettler(listingsubscription.AIInvocationUsageAdapter{Repository: listingsubscription.NewGormRepository(db)})
	capabilities, err := BuildOrganizationImageCapabilities(manager, db, OrganizationReviewOptions{Recorder: recorder, Logger: logrus.New()})
	require.NoError(t, err)
	delegate := tools.NewProductImageSlotExecutor(tools.Dependencies{SubjectExtractor: capabilities.SubjectExtractor, WhiteBackgroundRenderer: capabilities.WhiteBackgroundRenderer, SceneRenderer: capabilities.SceneRenderer, Reviewer: capabilities.Reviewer, UsageQuoter: capabilities.UsageQuoter, ProfileResolver: capabilities.ProfileResolver})
	executor := organizationMainSlotExecutor{delegate: delegate, quoter: capabilities.UsageQuoter, reservation: recorder}
	input := imageagent.SlotExecutionInput{RunID: "run-1", TenantID: "org-1", UserID: "actor-1", PlanRevision: 1, Attempt: 1, IdempotencyKey: "attempt-1", TargetPlatform: "product", ImagePolicyContext: &imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}, Slot: imageagent.Slot{ID: "main-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"catalog-image-1"}, IdempotencyKey: "slot-1"}, AssetCatalog: imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product-1", Title: "Controlled product", SourceSnapshotVersion: 1}, Assets: []imageagent.AuthorizedAsset{{ID: "catalog-image-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/product.png", Width: 1200, Height: 1200}}}, ProductContext: imageagent.ProductContextRef{ProductID: "product-1", Title: "Controlled product", SourceSnapshotVersion: 1}}
	ctx := mainAdmissionContext()
	quote, err := executor.QuoteSlot(ctx, input, imageagent.BudgetPolicy{})
	require.NoError(t, err)
	require.EqualValues(t, 3, quote.Maximum.Images)
	output, err := executor.GenerateQuotedSlot(ctx, input, quote)
	require.NoError(t, err)
	require.Len(t, output.Assets, 1)
	require.Equal(t, imageBytes, output.Assets[0].Bytes)
	require.EqualValues(t, 2, imageCalls.Load())
	require.EqualValues(t, 1, reviewCalls.Load())
	require.Zero(t, unexpectedCalls.Load())
	var usage []struct {
		Quantity int64
		MemberID string
		Status   string
	}
	require.NoError(t, db.Table("saas_usage_events").Where("source_type = ?", "ai_invocation").Find(&usage).Error)
	require.Len(t, usage, 1, "Account/Audit read the canonical commercial invocation fact")
	require.EqualValues(t, 7, usage[0].Quantity)
	require.Equal(t, "member-1", usage[0].MemberID)
	require.Equal(t, "committed", usage[0].Status)
}

type controlledImageReference struct{ body []byte }

func (t controlledImageReference) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Host != "source.example" || request.URL.Scheme != "https" {
		return nil, imageagent.ErrValidation
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"image/png"}}, Body: io.NopCloser(bytes.NewReader(t.body)), Request: request}, nil
}
