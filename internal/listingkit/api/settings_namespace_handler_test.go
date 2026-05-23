package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func TestUpdateAISettingsDoesNotRequireStudioSubscription(t *testing.T) {
	t.Helper()

	svc := &stubGenerationTaskService{
		aiSettings: &listingkit.AIClientSettings{
			Scope:      "tenant",
			ClientName: "default",
			BaseURL:    "https://tenant-scope.local/v1",
			Model:      "tenant-model-v1",
			Enabled:    true,
			APIKeySet:  true,
		},
	}

	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/settings/:namespace", h.UpdateSettingsNamespace)

	body, err := json.Marshal(map[string]any{
		"scope":       "tenant",
		"client_name": "default",
		"base_url":    "https://tenant-scope.local/v1",
		"model":       "tenant-model-v1",
		"enabled":     true,
		"api_key":     "tenant-key-123",
	})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/settings/ai", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("PUT /settings/ai = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	if svc.aiSettingsReq == nil {
		t.Fatal("expected UpdateAIClientSettings to be called")
	}
	if svc.aiSettingsReq.Scope != "tenant" {
		t.Fatalf("scope = %q, want tenant", svc.aiSettingsReq.Scope)
	}
	if svc.aiSettingsReq.ClientName != "default" {
		t.Fatalf("client name = %q, want default", svc.aiSettingsReq.ClientName)
	}
}
