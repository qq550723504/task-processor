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

	"task-processor/internal/prompt"
	"task-processor/internal/promptmgmt"
)

func TestPromptTemplateHandlersManageCurrentTenantPrompts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := openPromptTemplateStore(t)
	h := NewHandler(promptmgmt.NewService(store))
	router := gin.New()
	router.GET("/api/v1/listing-kits/prompts/catalog", h.ListPromptTemplateCatalog)
	router.GET("/api/v1/listing-kits/prompts/schema/:key", h.GetPromptTemplateSchema)
	router.GET("/api/v1/listing-kits/prompts", h.ListPromptTemplates)
	router.PUT("/api/v1/listing-kits/prompts", h.UpsertPromptTemplate)
	router.PATCH("/api/v1/listing-kits/prompts/:key/status", h.SetPromptTemplateStatus)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/prompts/catalog", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET catalog status = %d body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"key":"shein.content_optimizer.optimize_title_description_system"`) {
		t.Fatalf("GET catalog body missing schema key: %s", resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/prompts/schema/shein.content_optimizer.optimize_title_description_system", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET schema status = %d body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"supports_tenant_override":true`) {
		t.Fatalf("GET schema body missing tenant override: %s", resp.Body.String())
	}

	body := `{"key":"shein.content_optimizer.optimize_title_description_system","content":"Tenant prompt","version":"v1","enabled":true}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/listing-kits/prompts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-a")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("PUT status = %d body=%s", resp.Code, resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/prompts", nil)
	req.Header.Set("X-Tenant-ID", "tenant-a")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET status = %d body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"tenant_id":"tenant-a"`) || !strings.Contains(resp.Body.String(), `"content":"Tenant prompt"`) {
		t.Fatalf("GET body missing tenant prompt: %s", resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/v1/listing-kits/prompts/shein.content_optimizer.optimize_title_description_system/status", strings.NewReader(`{"enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-a")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d body=%s", resp.Code, resp.Body.String())
	}

	_, err := store.GetEnabled(req.Context(), "tenant-a", "shein.content_optimizer.optimize_title_description_system")
	if err == nil {
		t.Fatalf("disabled prompt should not be enabled")
	}
}

func TestPromptTemplateHandlersRequirePromptStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil)
	router := gin.New()
	router.GET("/api/v1/listing-kits/prompts/catalog", h.ListPromptTemplateCatalog)
	router.GET("/api/v1/listing-kits/prompts", h.ListPromptTemplates)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/prompts", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/prompts/catalog", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("catalog status = %d, want 503", resp.Code)
	}
}

func TestPromptTemplateHandlersRejectUnknownTemplateKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := openPromptTemplateStore(t)
	h := NewHandler(promptmgmt.NewService(store))
	router := gin.New()
	router.PUT("/api/v1/listing-kits/prompts", h.UpsertPromptTemplate)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/listing-kits/prompts", strings.NewReader(`{"key":"unknown.prompt.key","content":"Tenant prompt","enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-a")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("PUT unknown key status = %d body=%s", resp.Code, resp.Body.String())
	}
}

func openPromptTemplateStore(t *testing.T) *prompt.GormTenantPromptStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&prompt.TenantPromptTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return prompt.NewGormTenantPromptStore(db)
}
