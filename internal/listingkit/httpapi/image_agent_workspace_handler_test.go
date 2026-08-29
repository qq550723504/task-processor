package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/asset"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/listingkit"
)

func TestImageAgentWorkspacePreflightUsesOnlyRequestedTargetBundle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := requireImageAgentWorkspaceHandler(t, imageAgentWorkspaceTask(), &imageAgentWorkspaceApplicationStub{})
	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/image-agent-assets", handler.GetImageAgentAssets)

	response := performImageAgentWorkspaceRequest(router, http.MethodGet, "/api/v1/listing-kits/tasks/task-1/image-agent-assets?target_platform=%20SHEIN%20", "")

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"target_platform":"shein","source_assets":[{"id":"shein-source","label":"Shein source","display_url":"https://cdn.example.test/shein-source.png"}],"style_candidates":[{"id":"shein-style","label":"Shein style","display_url":"https://cdn.example.test/shein-style.png"}]}`, response.Body.String())
}

func TestImageAgentWorkspaceCreateBuildsSingleSourceServerOwnedRun(t *testing.T) {
	gin.SetMode(gin.TestMode)
	application := &imageAgentWorkspaceApplicationStub{}
	handler := requireImageAgentWorkspaceHandler(t, imageAgentWorkspaceTask(), application)
	ids := []string{"run", "plan", "slot", "idempotency"}
	handler.newID = func() string {
		value := ids[0]
		ids = ids[1:]
		return value
	}
	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/image-agent-runs", handler.CreateImageAgentRun)

	response := performImageAgentWorkspaceRequest(router, http.MethodPost, "/api/v1/listing-kits/tasks/task-1/image-agent-runs", `{"target_platform":"shein","source_asset_id":"shein-source","style_asset_ids":["shein-style","shein-style"]}`)

	require.Equal(t, http.StatusAccepted, response.Code)
	require.JSONEq(t, `{"run_id":"image-agent-run","status":"accepted"}`, response.Body.String())
	require.Len(t, application.starts, 1)
	start := application.starts[0]
	require.Equal(t, "image-agent-run", start.RunID)
	require.Equal(t, "task-1", start.BusinessTaskID)
	require.Equal(t, "shein", start.TargetPlatform)
	require.Equal(t, imageagent.RunModeManual, start.Mode)
	require.Equal(t, []string{"shein-source"}, start.Plan.SourceAssetIDs)
	require.Equal(t, []string{"shein-style"}, start.Plan.StyleReferenceIDs)
	require.Equal(t, imageagent.Budget{MaxImages: 1, EnabledLimits: imageagent.BudgetLimitImages}, start.Budget)
	require.Equal(t, []imageagent.Slot{{ID: "main", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"shein-source"}, StyleReferenceIDs: []string{"shein-style"}, IdempotencyKey: "image-agent-slot-main-slot", Status: imageagent.SlotStatusPending}}, start.Plan.Slots)
}

func TestImageAgentWorkspaceCreateRejectsUnknownBrowserFieldsAndUnownedSelection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	application := &imageAgentWorkspaceApplicationStub{}
	handler := requireImageAgentWorkspaceHandler(t, imageAgentWorkspaceTask(), application)
	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/image-agent-runs", handler.CreateImageAgentRun)

	unknownField := performImageAgentWorkspaceRequest(router, http.MethodPost, "/api/v1/listing-kits/tasks/task-1/image-agent-runs", `{"target_platform":"shein","source_asset_id":"shein-source","run_id":"forged"}`)
	require.Equal(t, http.StatusBadRequest, unknownField.Code)

	wrongSource := performImageAgentWorkspaceRequest(router, http.MethodPost, "/api/v1/listing-kits/tasks/task-1/image-agent-runs", `{"target_platform":"shein","source_asset_id":"amazon-source"}`)
	require.Equal(t, http.StatusBadRequest, wrongSource.Code)
	require.Empty(t, application.starts)
}

func requireImageAgentWorkspaceHandler(t *testing.T, task *listingkit.Task, application ImageAgentWorkspaceApplication) *imageAgentWorkspaceHandler {
	t.Helper()
	handler, err := NewImageAgentWorkspaceHandler(listingTaskSourceStub{task: task}, application)
	require.NoError(t, err)
	return handler
}

func imageAgentWorkspaceTask() *listingkit.Task {
	return &listingkit.Task{ID: "task-1", TenantID: "tenant-a", UserID: "user-a", Result: &listingkit.ListingKitResult{
		StandardProductSnapshot: &listingkit.StandardProductSnapshot{},
		AssetBundlesByTarget: map[string]*asset.Bundle{
			"shein": {Assets: []asset.Asset{
				{ID: "shein-source", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/shein-source.png", Labels: []string{"Shein source"}},
				{ID: "shein-style", Kind: asset.KindSceneImage, URL: "https://cdn.example.test/shein-style.png", Labels: []string{"Shein style"}},
			}},
			"amazon": {Assets: []asset.Asset{{ID: "amazon-source", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/amazon-source.png"}}},
		},
	}}
}

func performImageAgentWorkspaceRequest(router http.Handler, method, target, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request = request.WithContext(authidentity.WithAuthenticatedIdentity(request.Context(), authidentity.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

type imageAgentWorkspaceApplicationStub struct{ starts []imageagent.StartRunInput }

func (s *imageAgentWorkspaceApplicationStub) Start(_ context.Context, input imageagent.StartRunInput) error {
	s.starts = append(s.starts, input)
	return nil
}
