package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/listingkit"
	"task-processor/internal/product/catalog"
)

func TestImageAgentWorkspacePreflightUsesOnlyRequestedTargetBundle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := requireImageAgentWorkspaceHandler(t, imageAgentWorkspaceTask(), &imageAgentWorkspaceApplicationStub{})
	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/image-agent-assets", handler.GetImageAgentAssets)

	response := performImageAgentWorkspaceRequest(router, http.MethodGet, "/api/v1/listing-kits/tasks/task-1/image-agent-assets?target_platform=%20SHEIN%20", "")

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"target_platform":"shein","source_assets":[{"id":"catalog-image-1","label":"Product image","display_url":"https://cdn.example.test/source.png"}],"style_candidates":[{"id":"catalog-image-2","label":"Product image","display_url":"https://cdn.example.test/style.png"}]}`, response.Body.String())
}

func TestImageAgentWorkspacePreflightReturnsEmptyStyleArrayWhenNoStyleAssetsExist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	task := imageAgentWorkspaceTask()
	task.Result.StandardProductSnapshot.CatalogProduct.Images = task.Result.StandardProductSnapshot.CatalogProduct.Images[:1]
	handler := requireImageAgentWorkspaceHandler(t, task, &imageAgentWorkspaceApplicationStub{})
	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/image-agent-assets", handler.GetImageAgentAssets)

	response := performImageAgentWorkspaceRequest(router, http.MethodGet, "/api/v1/listing-kits/tasks/task-1/image-agent-assets?target_platform=shein", "")

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"target_platform":"shein","source_assets":[{"id":"catalog-image-1","label":"Product image","display_url":"https://cdn.example.test/source.png"}],"style_candidates":[]}`, response.Body.String())
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

	response := performImageAgentWorkspaceRequest(router, http.MethodPost, "/api/v1/listing-kits/tasks/task-1/image-agent-runs", `{"target_platform":"shein","image_policy_context":{"country":"us","family":"default","scene_category":"shoes"},"source_asset_id":"catalog-image-1","style_asset_ids":["catalog-image-2","catalog-image-2"]}`)

	require.Equal(t, http.StatusAccepted, response.Code)
	require.JSONEq(t, `{"run_id":"image-agent-run","status":"accepted"}`, response.Body.String())
	require.Len(t, application.starts, 1)
	start := application.starts[0]
	require.Equal(t, "image-agent-run", start.RunID)
	require.Equal(t, "task-1", start.BusinessTaskID)
	require.Equal(t, "shein", start.TargetPlatform)
	require.Equal(t, imageagent.ImagePolicyContext{Country: "us", Family: "default", SceneCategory: "shoes"}, start.ImagePolicyContext)
	require.Equal(t, imageagent.RunModeManual, start.Mode)
	require.Equal(t, []string{"catalog-image-1"}, start.Plan.SourceAssetIDs)
	require.Equal(t, []string{"catalog-image-2"}, start.Plan.StyleReferenceIDs)
	require.Equal(t, imageagent.Budget{MaxImages: 2, EnabledLimits: imageagent.BudgetLimitImages}, start.Budget)
	require.Equal(t, []imageagent.Slot{{ID: "main", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"catalog-image-1"}, StyleReferenceIDs: []string{"catalog-image-2"}, IdempotencyKey: "image-agent-slot-main-slot", Status: imageagent.SlotStatusPending}}, start.Plan.Slots)
}

func TestImageAgentWorkspaceCreateRejectsUnknownBrowserFieldsAndUnownedSelection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	application := &imageAgentWorkspaceApplicationStub{}
	handler := requireImageAgentWorkspaceHandler(t, imageAgentWorkspaceTask(), application)
	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/image-agent-runs", handler.CreateImageAgentRun)

	unknownField := performImageAgentWorkspaceRequest(router, http.MethodPost, "/api/v1/listing-kits/tasks/task-1/image-agent-runs", `{"target_platform":"shein","source_asset_id":"catalog-image-1","run_id":"forged"}`)
	require.Equal(t, http.StatusBadRequest, unknownField.Code)

	missingPolicy := performImageAgentWorkspaceRequest(router, http.MethodPost, "/api/v1/listing-kits/tasks/task-1/image-agent-runs", `{"target_platform":"shein","source_asset_id":"catalog-image-1"}`)
	require.Equal(t, http.StatusBadRequest, missingPolicy.Code)

	wrongSource := performImageAgentWorkspaceRequest(router, http.MethodPost, "/api/v1/listing-kits/tasks/task-1/image-agent-runs", `{"target_platform":"shein","image_policy_context":{"country":"us","family":"default","scene_category":"shoes"},"source_asset_id":"catalog-image-3"}`)
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
	return &listingkit.Task{ID: "task-1", TenantID: "tenant-a", UserID: "user-a",
		Request: &listingkit.GenerateRequest{ProductKey: "product-1"},
		Result: &listingkit.ListingKitResult{StandardProductSnapshot: &listingkit.StandardProductSnapshot{CatalogProduct: &catalog.ProductSnapshot{Images: []catalog.Image{
			{URL: "https://cdn.example.test/source.png", Role: "source"},
			{URL: "https://cdn.example.test/style.png", Role: "style"},
		}}}},
	}
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
