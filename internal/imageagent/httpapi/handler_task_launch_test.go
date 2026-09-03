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

func performLaunchRequest(t *testing.T, handler *Handler, body string, identity *authidentity.AuthenticatedIdentity) *httptest.ResponseRecorder {
	t.Helper()
	return performRequest(t, handler, http.MethodPost, "/api/v1/image-agent/task-runs", body, identity, nil)
}
