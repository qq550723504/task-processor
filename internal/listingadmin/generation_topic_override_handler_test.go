package listingadmin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerationTopicOverrideHandlerRejectsUnknownTopicKey(t *testing.T) {
	t.Parallel()

	router := newGenerationTopicOverrideTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/generation-topic-overrides", strings.NewReader(`{
		"platform":"shein",
		"topicKey":"unknown-topic",
		"additionalPromptDirectives":["Avoid this term"]
	}`))
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("POST /generation-topic-overrides = %d, body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"invalid_generation_topic_override"`) {
		t.Fatalf("body = %s, want invalid_generation_topic_override", resp.Body.String())
	}
}

func newGenerationTopicOverrideTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	router := newStoreTestRouter(t)
	if err := router.db.AutoMigrate(&listingGenerationTopicOverride{}); err != nil {
		t.Fatalf("migrate listing_generation_topic_override: %v", err)
	}
	repo := NewGormGenerationTopicOverrideRepository(router.db)
	handler := NewGenerationTopicOverrideHandler(repo)
	router.engine.POST("/generation-topic-overrides", handler.CreateGenerationTopicOverride)
	return router
}
