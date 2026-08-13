package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
)

type stubSettingsNamespaceService struct {
	aiSettings         *listingkit.AIClientSettings
	aiSettingsByClient map[string]*listingkit.AIClientSettings
	gotAIClientNames   []string
	aiSettingsReq      *listingkit.AIClientSettings
	sheinSettings      *listingkit.SheinSettings
	sheinSettingsReq   *listingkit.SheinSettings
	err                error
}

func (s *stubSettingsNamespaceService) GetSheinSettings(context.Context) (*listingkit.SheinSettings, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.sheinSettings, nil
}

func (s *stubSettingsNamespaceService) UpdateSheinSettings(_ context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.sheinSettingsReq = req
	s.sheinSettings = req
	return req, nil
}

func (s *stubSettingsNamespaceService) GetAIClientSettings(_ context.Context, scope string, clientName string) (*listingkit.AIClientSettings, error) {
	s.gotAIClientNames = append(s.gotAIClientNames, clientName)
	if s.err != nil {
		return nil, s.err
	}
	if s.aiSettingsByClient != nil {
		return s.aiSettingsByClient[clientName], nil
	}
	if s.aiSettings != nil {
		return s.aiSettings, nil
	}
	return &listingkit.AIClientSettings{
		Scope:      scope,
		ClientName: clientName,
	}, nil
}

func (s *stubSettingsNamespaceService) UpdateAIClientSettings(_ context.Context, req *listingkit.AIClientSettings) (*listingkit.AIClientSettings, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.aiSettingsReq = req
	if s.aiSettings != nil {
		return s.aiSettings, nil
	}
	return req, nil
}

func TestGetSettingsHealthReturnsConfigurationImpact(t *testing.T) {
	t.Helper()

	svc := &stubSettingsNamespaceService{
		aiSettingsByClient: map[string]*listingkit.AIClientSettings{
			"default": {
				ClientName: "default",
				BaseURL:    "https://tenant-scope.local/v1",
				Model:      "tenant-model-v1",
				Enabled:    true,
				APIKeySet:  true,
			},
			"image_gpt_image_2": {
				ClientName: "image_gpt_image_2",
				BaseURL:    "https://tenant-scope.local/v1",
				Model:      "image-model-v1",
				Enabled:    true,
				APIKeySet:  true,
			},
		},
		sheinSettings: &listingkit.SheinSettings{
			Site:              "US",
			DefaultStock:      12,
			DefaultSubmitMode: "publish",
			Pricing: sheinpub.PricingRule{
				TargetCurrency:   "USD",
				ExchangeRate:     7.1,
				MarkupMultiplier: 1.3,
			},
		},
	}

	h, err := NewHandler(&stubHandlerCoreService{}, WithSettingsHandlerService(svc))
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/settings-health", h.GetSettingsHealth)

	req := httptest.NewRequest(http.MethodGet, "/settings-health", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /settings-health = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	var payload listingkit.SettingsHealthPage
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if payload.Status != "warning" {
		t.Fatalf("status = %q, want warning because runtime probes are unknown", payload.Status)
	}
	if len(svc.gotAIClientNames) != 2 || svc.gotAIClientNames[0] != "default" || svc.gotAIClientNames[1] != "image_gpt_image_2" {
		t.Fatalf("health requested AI clients = %#v, want [default image_gpt_image_2]", svc.gotAIClientNames)
	}
	var hasSDSUnknown bool
	for _, item := range payload.Items {
		if item.Key == "sds.session" && item.Status == "unknown" && len(item.Impact) > 0 {
			hasSDSUnknown = true
		}
	}
	if !hasSDSUnknown {
		t.Fatalf("payload items = %#v", payload.Items)
	}
}

func TestGetSheinSettingsSchemaOmitsDefaultStore(t *testing.T) {
	t.Parallel()

	h, err := NewHandler(&stubHandlerCoreService{}, WithSettingsHandlerService(&stubSettingsNamespaceService{}))
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/settings/namespaces/:namespace", h.GetSettingsNamespaceSchema)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/settings/namespaces/shein", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("GET SHEIN settings schema = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	var payload struct {
		Fields []struct {
			Key string `json:"key"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	for _, field := range payload.Fields {
		if field.Key == "default_store_id" {
			t.Fatalf("SHEIN schema fields = %#v, must not include default_store_id", payload.Fields)
		}
	}
}

func TestGetSettingsHealthReturnsUnavailableWhenSettingsServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler{}
	router := gin.New()
	router.GET("/settings-health", h.GetSettingsHealth)

	req := httptest.NewRequest(http.MethodGet, "/settings-health", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /settings-health = %d, want 503; body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "settings_service_unavailable") {
		t.Fatalf("body = %s, want settings_service_unavailable", resp.Body.String())
	}
}

func TestGetReadinessAcceptsConfigurationWarningsWhenSettingsBackendIsReachable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubHandlerCoreService{}, WithSettingsHandlerService(&stubSettingsNamespaceService{}))
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}
	router := gin.New()
	router.GET("/readyz", h.GetReadiness)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /readyz = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	if resp.Body.String() != `{"status":"ready"}` {
		t.Fatalf("body = %s, want ready response", resp.Body.String())
	}
}

func TestGetReadinessReturnsServiceUnavailableWithoutLeakingDependencyFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubHandlerCoreService{}, WithSettingsHandlerService(&stubSettingsNamespaceService{
		err: errors.New("postgres connection refused for password=secret"),
	}))
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}
	router := gin.New()
	router.GET("/readyz", h.GetReadiness)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz = %d, want 503; body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "listingkit_settings_unavailable") {
		t.Fatalf("body = %s, want listingkit_settings_unavailable", resp.Body.String())
	}
	if strings.Contains(resp.Body.String(), "password=secret") {
		t.Fatalf("body leaked dependency failure: %s", resp.Body.String())
	}
}

func TestUpdateAISettingsDoesNotRequireStudioSubscription(t *testing.T) {
	t.Helper()

	svc := &stubSettingsNamespaceService{
		aiSettings: &listingkit.AIClientSettings{
			Scope:      "tenant",
			ClientName: "default",
			BaseURL:    "https://tenant-scope.local/v1",
			Model:      "tenant-model-v1",
			Enabled:    true,
			APIKeySet:  true,
		},
	}

	h, err := NewHandler(&stubHandlerCoreService{}, WithSettingsHandlerService(svc))
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

func TestUpdateSheinSettingsIgnoresLegacyDefaultStoreID(t *testing.T) {
	t.Parallel()

	svc := &stubSettingsNamespaceService{}
	h, err := NewHandler(
		&stubHandlerCoreService{},
		WithSettingsHandlerService(svc),
		WithSubscriptionService(activeStudioOnlySubscriptionService(t)),
	)
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/settings/:namespace", h.UpdateSettingsNamespace)

	req := httptest.NewRequest(http.MethodPut, "/settings/shein", strings.NewReader(`{"default_store_id": 9, "site": "GB"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("PUT /settings/shein = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal response: %v", err)
	}
	if _, exists := payload["default_store_id"]; exists {
		t.Fatalf("response = %#v, must not expose default_store_id", payload)
	}
	if payload["site"] != "GB" {
		t.Fatalf("response site = %#v, want GB", payload["site"])
	}
	if svc.sheinSettingsReq == nil || svc.sheinSettingsReq.Site != "GB" {
		t.Fatalf("updated SHEIN settings = %+v, want Site GB", svc.sheinSettingsReq)
	}
	updatedJSON, err := json.Marshal(svc.sheinSettingsReq)
	if err != nil {
		t.Fatalf("json.Marshal updated settings: %v", err)
	}
	if bytes.Contains(updatedJSON, []byte(`"default_store_id"`)) {
		t.Fatalf("updated SHEIN settings retained a store selection: %s", updatedJSON)
	}
}
