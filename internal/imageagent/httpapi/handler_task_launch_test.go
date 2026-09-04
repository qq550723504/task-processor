package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
)

func TestLaunchTaskRunRequiresIdentity(t *testing.T) {
	handler := requireHandler(t, &stubApplication{})

	response := performLaunchRequest(t, handler, `{}`, nil)

	require.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestLaunchTaskRunRejectsInvalidPayloads(t *testing.T) {
	handler := requireHandler(t, &stubApplication{})
	identity := verifiedIdentity("tenant-a", "user-a")

	tests := []struct {
		name string
		body string
	}{
		{name: "malformed json", body: `{"business_task_id":`},
		{name: "unknown field", body: `{"business_task_id":"task-1","plan":{}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performLaunchRequest(t, handler, tt.body, identity)
			require.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
}

func TestLaunchTaskRunDecodesTaskScopedPayloadAndReturnsRunID(t *testing.T) {
	application := &stubApplication{
		launchResult: imageagent.TaskRunLaunchResult{RunID: "image-agent-launched"},
	}
	handler := requireHandler(t, application)

	response := performLaunchRequest(t, handler, `{
		"business_task_id":"task-1",
		"target_platform":"shein",
		"image_policy_context":{"country":"us","family":"default","scene_category":"shoes"},
		"source_asset_id":"source-2",
		"style_asset_ids":["style-1"]
	}`, verifiedIdentity("tenant-a", "user-a"))

	require.Equal(t, http.StatusAccepted, response.Code)
	require.JSONEq(t, `{"run_id":"image-agent-launched","status":"accepted"}`, response.Body.String())
	require.Equal(t, "tenant-a", application.launchIdentity.TenantID)
	require.Equal(t, "task-1", application.launchInput.BusinessTaskID)
	require.Equal(t, "shein", application.launchInput.TargetPlatform)
	require.Equal(t, imageagent.ImagePolicyContext{Country: "us", Family: "default", SceneCategory: "shoes"}, application.launchInput.ImagePolicyContext)
	require.Equal(t, "source-2", application.launchInput.SourceAssetID)
	require.Equal(t, []string{"style-1"}, application.launchInput.StyleAssetIDs)
}

func TestLaunchTaskRunMapsApplicationErrors(t *testing.T) {
	application := &stubApplication{launchErr: imageagent.ErrCommandBlocked}
	handler := requireHandler(t, application)

	response := performLaunchRequest(t, handler, `{"business_task_id":"task-1","target_platform":"shein"}`, verifiedIdentity("tenant-a", "user-a"))

	require.Equal(t, http.StatusConflict, response.Code)
}

func TestTaskAssetsRequiresIdentity(t *testing.T) {
	response := performRequest(t, requireHandler(t, &stubApplication{}), http.MethodGet,
		"/api/v1/image-agent/task-runs/assets?business_task_id=task-1&target_platform=shein", "", nil, nil)

	require.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestTaskAssetsDecodesQueryAndReturnsSelectableAssets(t *testing.T) {
	application := &stubApplication{
		preflight: imageagent.TaskRunAssetPreflight{
			BusinessTaskID: "task-1", TargetPlatform: "shein",
			Sources: []imageagent.AuthorizedAsset{
				{ID: "source-1", Type: imageagent.AuthorizedAssetSource, DisplayURL: "https://cdn.example.test/source-1.png", Label: "Source 1"},
				{ID: "source-2", Type: imageagent.AuthorizedAssetSource, Label: "Source 2"},
			},
			Styles: []imageagent.AuthorizedAsset{
				{ID: "style-1", Type: imageagent.AuthorizedAssetStyle, Label: "Style 1"},
			},
		},
	}
	handler := requireHandler(t, application)

	response := performRequest(t, handler, http.MethodGet,
		"/api/v1/image-agent/task-runs/assets?business_task_id=task-1&target_platform=shein", "", verifiedIdentity("tenant-a", "user-a"), nil)

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{
		"business_task_id":"task-1",
		"target_platform":"shein",
		"sources":[
			{"id":"source-1","type":"source","display_url":"https://cdn.example.test/source-1.png","label":"Source 1"},
			{"id":"source-2","type":"source","label":"Source 2"}
		],
		"styles":[{"id":"style-1","type":"style","label":"Style 1"}]
	}`, response.Body.String())
	require.Equal(t, "task-1", application.preflightInput.BusinessTaskID)
	require.Equal(t, "shein", application.preflightInput.TargetPlatform)
}

func TestTaskAssetsMapsApplicationErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{name: "blocked tenant", err: imageagent.ErrCommandBlocked, wantCode: http.StatusConflict},
		{name: "invalid request", err: imageagent.ErrValidation, wantCode: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := requireHandler(t, &stubApplication{preflightErr: tt.err})
			response := performRequest(t, handler, http.MethodGet,
				"/api/v1/image-agent/task-runs/assets?business_task_id=task-1&target_platform=shein", "", verifiedIdentity("tenant-a", "user-a"), nil)
			require.Equal(t, tt.wantCode, response.Code)
		})
	}
}

func performLaunchRequest(t *testing.T, handler *Handler, body string, identity *authidentity.AuthenticatedIdentity) *httptest.ResponseRecorder {
	t.Helper()
	return performRequest(t, handler, http.MethodPost, "/api/v1/image-agent/task-runs", body, identity, nil)
}
