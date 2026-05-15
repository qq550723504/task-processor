package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/listingkit"
	"task-processor/internal/prompt"
)

func TestPromptSettingsHandlersManageCurrentTenantPrompts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := openPromptSettingsStore(t)
	h, err := NewHandler(&stubGenerationTaskService{}, WithTenantPromptStore(store))
	if err != nil {
		t.Fatalf("NewHandler error = %v", err)
	}
	router := gin.New()
	router.GET("/api/v1/listing-kits/settings/prompts", h.ListPromptSettings)
	router.PUT("/api/v1/listing-kits/settings/prompts", h.UpsertPromptSetting)
	router.PATCH("/api/v1/listing-kits/settings/prompts/:key/status", h.SetPromptSettingStatus)

	body := `{"key":"shein.content_optimizer.optimize_title_description_system","content":"Tenant prompt","version":"v1","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/listing-kits/settings/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-a")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("PUT status = %d body=%s", resp.Code, resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/settings/prompts", nil)
	req.Header.Set("X-Tenant-ID", "tenant-a")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET status = %d body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"tenant_id":"tenant-a"`) || !strings.Contains(resp.Body.String(), `"content":"Tenant prompt"`) {
		t.Fatalf("GET body missing tenant prompt: %s", resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/v1/listing-kits/settings/prompts/shein.content_optimizer.optimize_title_description_system/status", strings.NewReader(`{"enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-a")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d body=%s", resp.Code, resp.Body.String())
	}

	_, err = store.GetEnabled(req.Context(), "tenant-a", "shein.content_optimizer.optimize_title_description_system")
	if err == nil {
		t.Fatalf("disabled prompt should not be enabled")
	}
}

func TestPromptSettingsHandlersRequirePromptStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubGenerationTaskService{})
	if err != nil {
		t.Fatalf("NewHandler error = %v", err)
	}
	router := gin.New()
	router.GET("/api/v1/listing-kits/settings/prompts", h.ListPromptSettings)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/settings/prompts", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.Code)
	}
}

func openPromptSettingsStore(t *testing.T) *prompt.GormTenantPromptStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&prompt.TenantPromptTemplate{}, &listingkit.Task{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return prompt.NewGormTenantPromptStore(db)
}
