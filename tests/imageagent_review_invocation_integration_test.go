package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"task-processor/internal/ai"
	"task-processor/internal/aicapability"
	aistore "task-processor/internal/aicapability/store"
	imageworker "task-processor/internal/app/worker/imageagent"
	"task-processor/internal/authidentity"
	openai "task-processor/internal/integration/openai"
	image "task-processor/internal/product/image"
	"task-processor/internal/shared/aiidentity"
)

// Fine-grained fault tests supplement the actual HTTP/Activity combination.
// Only these transport/recorder cases construct an already-restored identity.
func review334Context(org string) context.Context {
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{HomeOrganizationID: "A", EffectiveOrganizationID: org, TenantID: org, UserID: "actor"})
	return aiidentity.WithIdentity(ctx, aiidentity.Identity{TenantID: org, UserID: "actor", BusinessTaskID: "context", AgentRunID: "run-" + org})
}
func review334Request() image.ReviewRequest {
	return image.ReviewRequest{Product: image.ProductContext{ProductKey: "product", Title: "SENSITIVE-PROMPT-SENTINEL"}, Sources: []image.Asset{{URL: "https://source.example/a.png", SourceURL: "https://origin.example/a.png", SourceAssetID: "source", Role: image.RoleSource, Width: 1200, Height: 1200, Operations: []string{"source"}}}, Candidates: []image.Candidate{{Asset: image.Asset{URL: "https://generated.example/a.png", MediaType: "image/png", SourceURL: "https://source.example/a.png", SourceAssetID: "source", Role: image.RoleScene, Width: 1200, Height: 1200, Operations: []string{"render_scene"}}}}}
}
func review334Manager(t *testing.T, db *gorm.DB, endpoint string, logger *logrus.Logger) *openai.Manager {
	t.Helper()
	credentials := openai.NewGormCredentialResolver(db)
	for _, org := range []string{"B", "C"} {
		for _, name := range []string{"default", "image_gpt_image_2"} {
			require.NoError(t, credentials.SaveCredential(context.Background(), openai.AIClientCredential{TenantID: org, UserID: "actor", ClientName: name, APIKey: "SENSITIVE-KEY-" + org, BaseURL: endpoint + "/v1", Model: "review-test", Enabled: true, TimeoutSecond: 1}))
		}
	}
	cfg := openai.NewClientConfig("SENSITIVE-STATIC", "review-test", endpoint+"/v1", 1)
	cfg.MaxRetries = 3
	cfg.RetryDelay = time.Millisecond
	cfg.Logger = openai.AdaptLogrus(logrus.NewEntry(logger))
	manager, err := openai.NewManager(&openai.ManagerConfig{Clients: map[string]*openai.ClientConfig{"default": cfg, "image_gpt_image_2": cfg}, DefaultClient: "default", Logger: cfg.Logger})
	require.NoError(t, err)
	return manager
}
func TestOrganizationReviewInvocationFailureMatrix(t *testing.T) {
	for _, name := range []string{"success", "missing_usage", "semantic_score", "semantic_reasons", "provider_503", "provider_400", "provider_401", "provider_429", "deadline", "inbound_cancel", "record_failure", "record_db_failure", "both_fail", "response_cancel", "nil_authorization", "stale_config", "stale_route", "stale_price", "forged_cost", "known_quote", "config_drift", "missing_scope", "missing_run"} {
		t.Run(name, func(t *testing.T) {
			f := newScope339Fixture(t)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				require.Equal(t, "Bearer SENSITIVE-KEY-B", r.Header.Get("Authorization"))
				if name == "deadline" {
					select {
					case <-r.Context().Done():
					case <-time.After(250 * time.Millisecond):
					}
					return
				}
				if name == "provider_503" || name == "provider_400" || name == "provider_401" || name == "provider_429" || name == "both_fail" {
					status := 503
					if name == "provider_400" {
						status = 400
					}
					if name == "provider_401" {
						status = 401
					}
					if name == "provider_429" {
						status = 429
					}
					w.WriteHeader(status)
					fmt.Fprint(w, `{"error":{"message":"SENSITIVE-ERROR-BODY-SENTINEL","type":"server_error"}}`)
					return
				}
				content := `{"score":0.9,"needs_human_review":false,"reasons":[]}`
				if name == "semantic_score" {
					content = `{"score":2,"needs_human_review":false,"reasons":[]}`
				}
				if name == "semantic_reasons" {
					content = `{"score":0.9,"needs_human_review":true,"reasons":[]}`
				}
				payload := map[string]any{"id": "safe-request-id", "model": "review-test", "choices": []any{map[string]any{"index": 0, "message": map[string]string{"role": "assistant", "content": content}, "finish_reason": "stop"}}}
				if name != "missing_usage" {
					payload["usage"] = map[string]int{"prompt_tokens": 3, "completion_tokens": 4, "total_tokens": 7}
				} else {
					payload["id"] = "bad\nrequest-id"
				}
				require.NoError(t, json.NewEncoder(w).Encode(payload))
			}))
			defer server.Close()
			var logs bytes.Buffer
			logger := logrus.New()
			logger.SetOutput(&logs)
			logger.SetLevel(logrus.DebugLevel)
			manager := review334Manager(t, f.db, server.URL, logger)
			options := imageworker.OrganizationReviewOptions{Recorder: aistore.NewGormInvocationRecorder(f.db), Logger: logger}
			if name == "known_quote" {
				options.Pricing = &imageworker.ReviewPricing{Version: "controlled-price-v1", MaximumCostMicros: 7}
			}
			capabilities, err := imageworker.BuildOrganizationImageCapabilities(manager, f.db, options)
			require.NoError(t, err)
			ctx := review334Context("B")
			req := review334Request()
			quote, err := capabilities.UsageQuoter.QuoteUsage(ctx, image.UsageQuoteRequest{Operation: "review", InputFingerprint: "input", MaximumOutputs: 1})
			require.NoError(t, err)
			require.EqualValues(t, 1, quote.MaximumModelCalls)
			require.Equal(t, name == "known_quote", quote.CostUpperBoundKnown)
			req.Authorization = &quote
			switch name {
			case "nil_authorization":
				req.Authorization = nil
			case "stale_config":
				quote.ConfigurationVersion = "old"
			case "stale_route":
				quote.RouteReference = "old"
			case "stale_price":
				quote.PricingVersion = "old"
			case "forged_cost":
				quote.CostUpperBoundKnown = true
			}
			if name == "deadline" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 80*time.Millisecond)
				defer cancel()
			}
			if name == "inbound_cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if name == "response_cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()
				require.NoError(t, f.db.Callback().Create().After("gorm:create").Register("response-cancel", func(tx *gorm.DB) {
					if tx.Statement.Table == "ai_invocations" {
						cancel()
					}
				}))
			}
			if name == "record_failure" || name == "both_fail" {
				require.NoError(t, f.db.Callback().Create().Before("gorm:create").Register("record-fault", func(tx *gorm.DB) {
					if tx.Statement.Table == "ai_invocations" {
						tx.AddError(fmt.Errorf("SENSITIVE-DB-ERROR-SENTINEL"))
					}
				}))
			}
			if name == "config_drift" {
				require.NoError(t, f.db.Model(&openai.AIClientCredential{}).Where("tenant_id = ? AND client_name = ?", "B", "default").Update("model", "changed-model").Error)
			}
			if name == "missing_scope" {
				ctx = context.Background()
			}
			if name == "missing_run" {
				id := aiidentity.FromContext(ctx)
				id.AgentRunID = ""
				ctx = aiidentity.WithIdentity(ctx, id)
			}
			if name == "record_db_failure" {
				require.NoError(t, f.db.Exec("ALTER TABLE ai_invocations ADD CONSTRAINT controlled_record_failure CHECK (false)").Error)
			}
			result, err := capabilities.Reviewer.Review(ctx, req)
			success := name == "success" || name == "missing_usage" || name == "record_failure" || name == "record_db_failure" || name == "known_quote"
			if success {
				require.NoError(t, err)
				require.Equal(t, 0.9, result.Score)
			} else {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "SENSITIVE-")
			}
			before := name == "inbound_cancel" || name == "nil_authorization" || strings.HasPrefix(name, "stale_") || name == "forged_cost" || name == "config_drift" || name == "missing_scope" || name == "missing_run"
			expectedCalls := int32(1)
			if before {
				expectedCalls = 0
			}
			require.Equal(t, expectedCalls, calls.Load())
			var rows []map[string]any
			require.NoError(t, f.db.Table("ai_invocations").Find(&rows).Error)
			if before || name == "record_failure" || name == "record_db_failure" || name == "both_fail" {
				require.Empty(t, rows)
			} else {
				require.Len(t, rows, 1)
				row := rows[0]
				require.Equal(t, "B", row["tenant_id"])
				require.Equal(t, "run-B", row["agent_run_id"])
				require.Equal(t, false, row["estimated_cost_known"])
				require.Equal(t, name != "missing_usage" && name != "provider_503" && name != "provider_400" && name != "provider_401" && name != "provider_429" && name != "deadline", row["usage_known"])
				if name == "provider_503" || name == "provider_400" || name == "provider_401" || name == "provider_429" || name == "deadline" || name == "semantic_score" || name == "semantic_reasons" {
					require.Equal(t, "failed", row["outcome"])
					if name == "semantic_score" || name == "semantic_reasons" {
						require.Equal(t, string(aicapability.ErrorInvalidProviderResponse), row["error_category"])
						require.Equal(t, "invalid_review_output", row["error_code"])
					}
					if name == "provider_400" || name == "provider_401" {
						require.Equal(t, string(aicapability.ErrorProviderRejected), row["error_category"])
					}
					if name == "provider_429" {
						require.Equal(t, string(aicapability.ErrorRateLimited), row["error_category"])
					}
				} else {
					require.Equal(t, "succeeded", row["outcome"])
				}
				if name == "missing_usage" {
					require.Equal(t, "", row["provider_request_id"])
				}
			}
			if name == "record_failure" || name == "record_db_failure" || name == "both_fail" {
				require.Contains(t, logs.String(), "image_review_record_degraded")
			} else {
				require.NotContains(t, logs.String(), "image_review_record_degraded")
			}
			data, err := json.Marshal(rows)
			require.NoError(t, err)
			require.NotContains(t, string(data), "SENSITIVE-")
			require.NotContains(t, logs.String(), "SENSITIVE-")
			t.Logf("provider HTTP=%d ledger rows=%d record degradation=%t", calls.Load(), len(rows), name == "record_failure" || name == "record_db_failure" || name == "both_fail")
		})
	}
}
func TestOrganizationReviewConcurrentRecordsAndRecorderConflict(t *testing.T) {
	f := newScope339Fixture(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"request","choices":[{"message":{"role":"assistant","content":"{\"score\":0.9,\"needs_human_review\":false,\"reasons\":[]}"}}]}`)
	}))
	defer server.Close()
	logger := logrus.New()
	var logs bytes.Buffer
	logger.SetOutput(&logs)
	manager := review334Manager(t, f.db, server.URL, logger)
	caps, err := imageworker.BuildOrganizationImageCapabilities(manager, f.db)
	require.NoError(t, err)
	var wg sync.WaitGroup
	for _, org := range []string{"B", "C"} {
		wg.Add(1)
		go func(org string) {
			defer wg.Done()
			ctx := review334Context(org)
			req := review334Request()
			quote, err := caps.UsageQuoter.QuoteUsage(ctx, image.UsageQuoteRequest{Operation: "review", InputFingerprint: org, MaximumOutputs: 1})
			require.NoError(t, err)
			req.Authorization = &quote
			_, err = caps.Reviewer.Review(ctx, req)
			require.NoError(t, err)
		}(org)
	}
	wg.Wait()
	require.EqualValues(t, 2, calls.Load())
	var rows []struct{ TenantID, AgentRunID, InvocationID string }
	require.NoError(t, f.db.Table("ai_invocations").Order("tenant_id").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.Equal(t, "run-B", rows[0].AgentRunID)
	require.Equal(t, "run-C", rows[1].AgentRunID)
	require.NotEqual(t, rows[0].InvocationID, rows[1].InvocationID)
	recorder := aistore.NewGormInvocationRecorder(f.db)
	record := aicapability.InvocationRecord{InvocationID: "fixed-id", TenantID: "B", Outcome: aicapability.InvocationSucceeded}
	require.NoError(t, recorder.RecordInvocation(context.Background(), record))
	require.Error(t, recorder.RecordInvocation(context.Background(), record))
	record.TenantID = "C"
	require.Error(t, recorder.RecordInvocation(context.Background(), record))
	var stored struct{ TenantID string }
	require.NoError(t, f.db.Table("ai_invocations").Where("invocation_id = ?", "fixed-id").Take(&stored).Error)
	require.Equal(t, "B", stored.TenantID)
}
func TestOrganizationReviewSDKRetryOverrideUsesExistingTransport(t *testing.T) {
	f := newScope339Fixture(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":{"message":"controlled retry","type":"server_error"}}`)
	}))
	defer server.Close()
	logger := logrus.New()
	manager := review334Manager(t, f.db, server.URL, logger)
	_, err := imageworker.BuildOrganizationImageCapabilities(manager, f.db)
	require.NoError(t, err)
	ctx := review334Context("B")
	route, err := manager.ResolveEffectiveClientRoute(ctx, "default")
	require.NoError(t, err)
	client, err := manager.GetClientWithRoute(ctx, "default", openai.ImageRouteSelection{CredentialReference: route.CredentialReference, ConfigurationVersion: route.ConfigurationVersion})
	require.NoError(t, err)
	retries := 1
	_, err = client.CreateChatCompletion(ctx, &ai.ChatCompletionRequest{Model: "review-test", Messages: []ai.ChatCompletionMessage{{Role: "user", Content: "controlled"}}, MaxRetries: &retries})
	require.Error(t, err)
	require.EqualValues(t, 2, calls.Load())
}

func TestOrganizationReviewHTTPActivityRecordFailureAndCancellation(t *testing.T) {
	for _, mode := range []string{"record_failure", "response_cancel", "known_cost", "unknown_cost"} {
		t.Run(mode, func(t *testing.T) {
			f := newScope339Fixture(t)
			server := f.server(t)
			body := scope339Body
			if mode == "known_cost" || mode == "unknown_cost" {
				body = strings.Replace(body, `"budget":{`, `"budget":{"max_cost_micros":100,`, 1)
			}
			require.Equal(t, 202, scope339Request(t, server, "POST", scope339Path, "actor", "B", body))
			f.verifyActivity(t, f.temporal.input(t), mode)
		})
	}
}

func TestOrganizationReviewRejectsMissingRecorderBeforeProvider(t *testing.T) {
	f := newScope339Fixture(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	logger := logrus.New()
	manager := review334Manager(t, f.db, server.URL, logger)
	_, err := imageworker.BuildOrganizationImageCapabilities(manager, f.db, imageworker.OrganizationReviewOptions{Logger: logger})
	require.Error(t, err)
	_, err = imageworker.BuildOrganizationImageCapabilities(manager, f.db, imageworker.OrganizationReviewOptions{Recorder: aistore.NewGormInvocationRecorder(f.db)})
	require.Error(t, err)
	require.Zero(t, calls.Load())
}
