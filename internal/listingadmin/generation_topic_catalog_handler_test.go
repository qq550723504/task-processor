package listingadmin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerationTopicCatalogHandlerReturnsDefaultDefinitions(t *testing.T) {
	t.Parallel()

	router := newGenerationTopicCatalogTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/generation-topic-catalog?platform=shein", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /generation-topic-catalog = %d, body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"key":"children"`) {
		t.Fatalf("body = %s, want children topic in catalog", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `Do not mention children, babies, or age-specific users.`) {
		t.Fatalf("body = %s, want default children directive in catalog", resp.Body.String())
	}
}

func newGenerationTopicCatalogTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	router := newStoreTestRouter(t)
	if err := router.db.AutoMigrate(&listingGenerationTopicOverride{}); err != nil {
		t.Fatalf("migrate listing_generation_topic_override: %v", err)
	}
	repo := NewGormGenerationTopicOverrideRepository(router.db)
	handler := NewGenerationTopicCatalogHandler(repo)
	router.engine.GET("/generation-topic-catalog", handler.ListGenerationTopicCatalog)
	return router
}
