package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
)

func TestReplacePlanRejectsStaleRevision(t *testing.T) {
	application := &stubApplication{replaceErr: imageagent.ErrRevisionConflict}
	handler := requireHandler(t, application)
	response := performRequest(t, handler, http.MethodPut, "/api/v1/image-agent/runs/run-1/plan",
		`{"expected_revision":0,"action_id":"replace-1","plan":{"revision":2}}`, verifiedIdentity("tenant-a", "user-a"), nil)
	require.Equal(t, http.StatusConflict, response.Code)
}

func TestGetRunDoesNotTrustRequestTenant(t *testing.T) {
	application := &stubApplication{projection: imageagent.RunProjection{Run: imageagent.Run{ID: "run-1", TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual}}}
	handler := requireHandler(t, application)
	response := performRequest(t, handler, http.MethodGet, "/api/v1/image-agent/runs/run-1?tenant_id=tenant-b", "", verifiedIdentity("tenant-a", "user-a"), nil)
	require.Equal(t, http.StatusOK, response.Code)
	require.NotContains(t, response.Body.String(), "tenant-b")
	require.Equal(t, "tenant-a", application.getIdentity.TenantID)
}

func TestHandlersRequireVerifiedIdentity(t *testing.T) {
	response := performRequest(t, requireHandler(t, &stubApplication{}), http.MethodGet, "/api/v1/image-agent/runs/run-1", "", nil, nil)
	require.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestResumeCommandRequiresIdentityAndReturnsWorkflowAcknowledgement(t *testing.T) {
	application := &stubApplication{resumeAck: imageagent.CommandAcknowledgement{RunID: "run-1", PlanRevision: 2, ActionID: "action-1", Status: imageagent.RunStatusExecuting}}
	handler := requireHandler(t, application)
	unauthorized := performRequest(t, handler, http.MethodPost, "/api/v1/image-agent/runs/run-1/commands/action-1/resume", "", nil, nil)
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)
	response := performRequest(t, handler, http.MethodPost, "/api/v1/image-agent/runs/run-1/commands/action-1/resume", "", verifiedIdentity("tenant-a", "user-a"), nil)
	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"run_id":"run-1","plan_revision":2,"action_id":"action-1","status":"executing"}`, response.Body.String())
	require.Equal(t, "run-1", application.resumeRunID)
	require.Equal(t, "action-1", application.resumeActionID)
}

func TestCreateAcceptsManualOnlyAndRejectsIdentityFields(t *testing.T) {
	validBody := `{
		"run_id":"run-1","business_task_id":"task-1","mode":"manual","idempotency_key":"run-key-1",
		"budget":{"max_images":12,"max_agent_steps":20,"max_model_calls":30,"max_repair_attempts_per_slot":2,"max_cost_micros":4000,"max_elapsed":5000000000},
		"plan":{"revision":1,"idempotency_key":"plan-key-1","source_asset_ids":["source-1"],
		"slots":[{"id":"slot-1","role":"scene","source_asset_ids":["source-1"],"idempotency_key":"slot-key-1"}]}}`
	t.Run("manual", func(t *testing.T) {
		application := &stubApplication{}
		response := performRequest(t, requireHandler(t, application), http.MethodPost, "/api/v1/image-agent/runs", validBody, verifiedIdentity("tenant-a", "user-a"), nil)
		require.Equal(t, http.StatusAccepted, response.Code)
		require.Equal(t, imageagent.RunModeManual, application.startInput.Mode)
		require.Equal(t, 12, application.startInput.Budget.MaxImages)
		require.Equal(t, 5*time.Second, application.startInput.Budget.MaxElapsed)
		require.Equal(t, "tenant-a", application.startIdentity.TenantID)
	})
	t.Run("assisted", func(t *testing.T) {
		application := &stubApplication{startErr: imageagent.ErrValidation}
		body := strings.Replace(validBody, `"mode":"manual"`, `"mode":"assisted"`, 1)
		response := performRequest(t, requireHandler(t, application), http.MethodPost, "/api/v1/image-agent/runs", body, verifiedIdentity("tenant-a", "user-a"), nil)
		require.Equal(t, http.StatusBadRequest, response.Code)
	})
	t.Run("blank business task", func(t *testing.T) {
		application := &stubApplication{startErr: imageagent.ErrValidation}
		body := strings.Replace(validBody, `"business_task_id":"task-1"`, `"business_task_id":"   "`, 1)
		response := performRequest(t, requireHandler(t, application), http.MethodPost, "/api/v1/image-agent/runs", body, verifiedIdentity("tenant-a", "user-a"), nil)
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.Equal(t, "   ", application.startInput.BusinessTaskID)
	})
	t.Run("spoofed tenant field", func(t *testing.T) {
		application := &stubApplication{}
		body := strings.Replace(validBody, `"run_id":"run-1"`, `"run_id":"run-1","tenant_id":"tenant-b"`, 1)
		response := performRequest(t, requireHandler(t, application), http.MethodPost, "/api/v1/image-agent/runs", body, verifiedIdentity("tenant-a", "user-a"), nil)
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.Empty(t, application.startInput.RunID)
	})
}

func TestGetRunUsesExplicitSnakeCaseHTTPResponseDTO(t *testing.T) {
	application := &stubApplication{projection: imageagent.RunProjection{
		Run: imageagent.Run{
			ID: "run-1", BusinessTaskID: "task-1", TenantID: "tenant-a", UserID: "user-a",
			Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-1", Status: imageagent.RunStatusBlocked,
			CurrentNode: "retry_slot", ActivePlanRevision: 2, Version: 7,
			Budget: imageagent.Budget{MaxImages: 12}, Usage: imageagent.BudgetUsage{ModelCalls: 3},
			Block: &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"},
		},
		Plan: imageagent.Plan{
			Revision: 2, ParentRevision: 1, IdempotencyKey: "plan-key-2", SourceAssetIDs: []string{"source-1"},
			Slots: []imageagent.Slot{{ID: "slot-1", Role: imageagent.SlotRoleScene, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1", Status: imageagent.SlotStatusBlocked}},
		},
		Slots: []imageagent.SlotProjection{{
			Slot:    imageagent.Slot{ID: "slot-1", Role: imageagent.SlotRoleScene, Status: imageagent.SlotStatusBlocked},
			Attempt: 2, Candidates: []imageagent.AssetCandidate{
				{AssetID: "candidate-1", URL: "https://generated.example/candidate-1.png", SourceAssetID: "source-1"},
				{AssetID: "candidate-unsafe", URL: "data:image/png;base64,AAAA", SourceAssetID: "source-1"},
			}, ErrorCode: "provider_failed",
		}},
		Actions: []imageagent.Action{imageagent.ActionRetrySlot}, LastEventID: 9,
		ProjectionVersion: 9,
		AssetCatalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, DisplayURL: "https://cdn.example.test/source.png", Label: "Source"},
			{ID: "style-1", Type: imageagent.AuthorizedAssetStyle, DisplayURL: "javascript:alert(1)", Label: "Unsafe URL omitted"},
		}},
		PendingCommand: &imageagent.PendingCommandReceipt{
			ActionID: "retry-pending", Kind: "retry_slot", Phase: "retry.persist_result", Status: "pending", PlanRevision: 2, SlotID: "slot-1",
			FailureCode: "provider_unavailable", FailureCategory: "provider", FailureMessage: "图片生成服务暂时不可用", Attempt: 2,
		},
		CommandIngress: imageagent.CommandIngress{Used: 1024, Limit: 1024, Exhausted: true, Reason: "command_capacity_exhausted"},
	}}

	response := performRequest(t, requireHandler(t, application), http.MethodGet, "/api/v1/image-agent/runs/run-1", "", verifiedIdentity("tenant-a", "user-a"), nil)

	require.Equal(t, http.StatusOK, response.Code)
	for _, field := range []string{`"business_task_id":"task-1"`, `"active_plan_revision":2`, `"source_asset_ids":["source-1"]`, `"idempotency_key":"plan-key-2"`, `"source_asset_id":"source-1"`, `"last_event_id":9`, `"projection_version":9`, `"max_images":12`, `"model_calls":3`, `"asset_catalog"`, `"action_id":"retry-pending"`, `"slot_id":"slot-1"`, `"failure_code":"provider_unavailable"`, `"failure_category":"provider"`, `"failure_message":"图片生成服务暂时不可用"`, `"command_ingress":{"used":1024,"limit":1024,"exhausted":true,"reason":"command_capacity_exhausted"}`} {
		require.Contains(t, response.Body.String(), field)
	}
	require.NotContains(t, response.Body.String(), "javascript:alert")
	require.NotContains(t, response.Body.String(), "data:image")
	require.NotContains(t, response.Body.String(), "candidate-unsafe")
	for _, forbidden := range []string{"BusinessTaskID", "ActivePlanRevision", "SourceAssetIDs", "IdempotencyKey", "LastEventID"} {
		require.NotContains(t, response.Body.String(), forbidden)
	}
}

func TestCreateRejectsValidOneMiBPrefixWithHiddenOverflow(t *testing.T) {
	prefix := `{"run_id":"run-1","mode":"manual","idempotency_key":"`
	suffix := `"}`
	padding := strings.Repeat("a", (1<<20)-len(prefix)-len(suffix))
	body := prefix + padding + suffix + `{"tenant_id":"tenant-b"}`
	application := &stubApplication{}

	response := performRequest(t, requireHandler(t, application), http.MethodPost, "/api/v1/image-agent/runs", body, verifiedIdentity("tenant-a", "user-a"), nil)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Empty(t, application.startInput.RunID)
}

func TestCommandErrorMapping(t *testing.T) {
	tests := []struct {
		name, method, target, body string
		setup                      func(*stubApplication)
		status                     int
	}{
		{name: "validation", method: http.MethodPost, target: "/api/v1/image-agent/runs/run-1/cancel", body: `{}`, setup: func(app *stubApplication) { app.cancelErr = imageagent.ErrValidation }, status: http.StatusBadRequest},
		{name: "blocked approval", method: http.MethodPost, target: "/api/v1/image-agent/runs/run-1/results/approve", body: `{"plan_revision":1,"action_id":"approve-1"}`, setup: func(app *stubApplication) { app.approveErr = imageagent.ErrCommandBlocked }, status: http.StatusConflict},
		{name: "missing run", method: http.MethodPost, target: "/api/v1/image-agent/runs/run-1/slots/slot-1/retry", body: `{"plan_revision":1,"action_id":"retry-1"}`, setup: func(app *stubApplication) { app.retryErr = imageagent.ErrRunNotFound }, status: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			application := &stubApplication{}
			tt.setup(application)
			response := performRequest(t, requireHandler(t, application), tt.method, tt.target, tt.body, verifiedIdentity("tenant-a", "user-a"), nil)
			require.Equal(t, tt.status, response.Code)
		})
	}
}

func TestSSEReplaysMonotonicVersionedProjectionEventsAfterLastEventID(t *testing.T) {
	application := &stubApplication{
		projection: imageagent.RunProjection{Run: imageagent.Run{ID: "run-1"}, LastEventID: 3},
		events: []imageagent.RunEvent{
			{TenantID: "tenant-a", RunID: "run-1", Type: "run.updated", Cursor: 2, ProjectionVersion: 2, Payload: json.RawMessage(`{"status":"executing"}`)},
			{TenantID: "tenant-a", RunID: "run-1", Type: "slot.result.persisted", Cursor: 3, ProjectionVersion: 3, Payload: json.RawMessage(`{"slot_id":"slot-1"}`)},
		},
	}
	handler := requireHandler(t, application)
	handler.pollInterval = time.Millisecond
	handler.heartbeatInterval = time.Hour
	response := performRequest(t, handler, http.MethodGet, "/api/v1/image-agent/runs/run-1/events", "", verifiedIdentity("tenant-a", "user-a"), func(request *http.Request) *http.Request {
		ctx, cancel := context.WithCancel(request.Context())
		application.afterList = cancel
		request.Header.Set("Last-Event-ID", "1")
		return request.WithContext(ctx)
	})
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, response.Body.String(), "id: 2\n")
	require.Contains(t, response.Body.String(), "id: 3\n")
	require.Contains(t, response.Body.String(), `"schema_version":"image-agent.projection.v1"`)
	require.NotContains(t, response.Body.String(), "tenant-a")
}

func TestSSEEventDTORejectsHostileDurablePayloadFields(t *testing.T) {
	application := &stubApplication{
		projection: imageagent.RunProjection{Run: imageagent.Run{ID: "run-1"}, LastEventID: 1},
		events: []imageagent.RunEvent{{
			TenantID: "tenant-a", RunID: "run-1", Type: "slot.result.persisted", Cursor: 1, ProjectionVersion: 1,
			Payload: json.RawMessage(`{"tenant_id":"tenant-secret","user_id":"user-secret","idempotency_key":"action-secret","internal_metadata":{"token":"raw-secret"}}`),
		}},
	}
	handler := requireHandler(t, application)
	handler.pollInterval = time.Millisecond
	handler.heartbeatInterval = time.Hour
	response := performRequest(t, handler, http.MethodGet, "/api/v1/image-agent/runs/run-1/events", "", verifiedIdentity("tenant-a", "user-a"), func(request *http.Request) *http.Request {
		ctx, cancel := context.WithCancel(request.Context())
		application.afterList = cancel
		return request.WithContext(ctx)
	})

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"type":"slot.result.persisted"`)
	require.Contains(t, response.Body.String(), `"projection_version":1`)
	for _, forbidden := range []string{"payload", "tenant-secret", "user-secret", "action-secret", "raw-secret", "internal_metadata"} {
		require.NotContains(t, response.Body.String(), forbidden)
	}
}

func TestSSECursorInputFailsClosed(t *testing.T) {
	for _, tt := range []struct{ name, cursor string }{{"not numeric", "abc"}, {"negative", "-1"}, {"ahead of snapshot", "4"}} {
		t.Run(tt.name, func(t *testing.T) {
			application := &stubApplication{projection: imageagent.RunProjection{Run: imageagent.Run{ID: "run-1"}, LastEventID: 3}}
			response := performRequest(t, requireHandler(t, application), http.MethodGet, "/api/v1/image-agent/runs/run-1/events", "", verifiedIdentity("tenant-a", "user-a"), func(request *http.Request) *http.Request {
				request.Header.Set("Last-Event-ID", tt.cursor)
				return request
			})
			require.Equal(t, http.StatusBadRequest, response.Code)
			require.Zero(t, application.listCalls)
		})
	}
}

func TestSSERetrogradeRepositoryEventDisconnectsWithoutEmission(t *testing.T) {
	application := &stubApplication{
		projection:                   imageagent.RunProjection{Run: imageagent.Run{ID: "run-1"}, LastEventID: 2},
		events:                       []imageagent.RunEvent{{TenantID: "tenant-a", RunID: "run-1", Type: "retrograde", Cursor: 1, ProjectionVersion: 2, Payload: json.RawMessage(`{}`)}},
		returnEventsWithoutFiltering: true,
	}
	handler := requireHandler(t, application)
	response := performRequest(t, handler, http.MethodGet, "/api/v1/image-agent/runs/run-1/events", "", verifiedIdentity("tenant-a", "user-a"), func(request *http.Request) *http.Request {
		ctx, cancel := context.WithCancel(request.Context())
		application.afterList = cancel
		request.Header.Set("Last-Event-ID", "1")
		return request.WithContext(ctx)
	})
	require.Equal(t, http.StatusOK, response.Code)
	require.NotContains(t, response.Body.String(), "id:")
}

func TestSSEEmitsHeartbeatAndReturnsPromptlyOnDisconnect(t *testing.T) {
	application := &stubApplication{projection: imageagent.RunProjection{Run: imageagent.Run{ID: "run-1"}}}
	handler := requireHandler(t, application)
	handler.pollInterval = time.Hour
	handler.heartbeatInterval = 2 * time.Millisecond
	started := time.Now()
	response := performRequest(t, handler, http.MethodGet, "/api/v1/image-agent/runs/run-1/events", "", verifiedIdentity("tenant-a", "user-a"), func(request *http.Request) *http.Request {
		ctx, cancel := context.WithCancel(request.Context())
		time.AfterFunc(8*time.Millisecond, cancel)
		return request.WithContext(ctx)
	})
	require.Less(t, time.Since(started), 200*time.Millisecond)
	require.Contains(t, response.Body.String(), ": heartbeat\n\n")
}

func requireHandler(t *testing.T, application Application) *Handler {
	t.Helper()
	handler, err := NewHandler(application)
	require.NoError(t, err)
	return handler
}

func performRequest(t *testing.T, handler *Handler, method, target, body string, identity *authidentity.AuthenticatedIdentity, mutate func(*http.Request) *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/image-agent/runs", handler.Create)
	router.GET("/api/v1/image-agent/runs/:run_id", handler.Get)
	router.PUT("/api/v1/image-agent/runs/:run_id/plan", handler.ReplacePlan)
	router.POST("/api/v1/image-agent/runs/:run_id/slots/:slot_id/retry", handler.RetrySlot)
	router.POST("/api/v1/image-agent/runs/:run_id/results/approve", handler.ApproveResults)
	router.POST("/api/v1/image-agent/runs/:run_id/cancel", handler.Cancel)
	router.POST("/api/v1/image-agent/runs/:run_id/commands/:action_id/resume", handler.Resume)
	router.GET("/api/v1/image-agent/runs/:run_id/events", handler.Events)
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if identity != nil {
		request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), *identity))
	}
	if mutate != nil {
		request = mutate(request)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func verifiedIdentity(tenantID, userID string) *authidentity.AuthenticatedIdentity {
	return &authidentity.AuthenticatedIdentity{TenantID: tenantID, UserID: userID}
}

type stubApplication struct {
	mu                           sync.Mutex
	projection                   imageagent.RunProjection
	events                       []imageagent.RunEvent
	returnEventsWithoutFiltering bool
	afterList                    func()
	listCalls                    int
	getIdentity                  authidentity.AuthenticatedIdentity
	startIdentity                authidentity.AuthenticatedIdentity
	startInput                   imageagent.StartRunInput
	startErr, replaceErr         error
	retryErr, approveErr         error
	cancelErr                    error
	resumeAck                    imageagent.CommandAcknowledgement
	resumeErr                    error
	resumeRunID                  string
	resumeActionID               string
}

func (s *stubApplication) Start(ctx context.Context, input imageagent.StartRunInput) error {
	s.startIdentity, _ = authidentity.AuthenticatedIdentityFromContext(ctx)
	s.startInput = input
	return s.startErr
}
func (s *stubApplication) Get(ctx context.Context, _ string) (imageagent.RunProjection, error) {
	s.getIdentity, _ = authidentity.AuthenticatedIdentityFromContext(ctx)
	return s.projection, nil
}
func (s *stubApplication) ReplacePlan(context.Context, string, int64, imageagent.Plan, string) error {
	return s.replaceErr
}
func (s *stubApplication) RetrySlot(context.Context, string, string, int64, string) error {
	return s.retryErr
}
func (s *stubApplication) ApproveResults(context.Context, string, int64, string, string) error {
	return s.approveErr
}
func (s *stubApplication) Cancel(context.Context, string, int64, string) error { return s.cancelErr }
func (s *stubApplication) Resume(_ context.Context, runID, actionID string) (imageagent.CommandAcknowledgement, error) {
	s.resumeRunID, s.resumeActionID = runID, actionID
	return s.resumeAck, s.resumeErr
}
func (s *stubApplication) ListEvents(_ context.Context, _ string, after int64, _ int) ([]imageagent.RunEvent, error) {
	s.mu.Lock()
	s.listCalls++
	events := append([]imageagent.RunEvent(nil), s.events...)
	withoutFiltering, afterList := s.returnEventsWithoutFiltering, s.afterList
	s.afterList = nil
	s.mu.Unlock()
	if afterList != nil {
		defer afterList()
	}
	if withoutFiltering {
		return events, nil
	}
	filtered := events[:0]
	for _, event := range events {
		if event.Cursor > after {
			filtered = append(filtered, event)
		}
	}
	return filtered, nil
}
