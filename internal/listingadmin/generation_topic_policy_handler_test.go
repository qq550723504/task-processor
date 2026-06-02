package listingadmin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestGenerationTopicPolicyHandlerListsPoliciesWithinRequestTenant(t *testing.T) {
	t.Parallel()

	router := newGenerationTopicPolicyTestRouter(t)
	seedGenerationTopicPolicy(t, router.db, listingGenerationTopicPolicy{
		TenantID: 101,
		Platform: "shein",
		TopicKey: "children",
		Status:   1,
		Remark:   "manual",
	})
	seedGenerationTopicPolicy(t, router.db, listingGenerationTopicPolicy{
		TenantID: 202,
		Platform: "shein",
		TopicKey: "food",
		Status:   1,
	})

	req := httptest.NewRequest(http.MethodGet, "/generation-topic-policies?page=1&page_size=20&platform=shein", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /generation-topic-policies = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page GenerationTopicPolicyPage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one policy", page)
	}
	if page.Items[0].TopicKey != "children" || page.Items[0].TenantID != 101 {
		t.Fatalf("items = %+v, want tenant 101 policy only", page.Items)
	}
}

func TestGenerationTopicPolicyHandlerCreatesPolicyWithRequestTenant(t *testing.T) {
	t.Parallel()

	router := newGenerationTopicPolicyTestRouter(t)
	body := bytes.NewBufferString(`{
		"platform":"shein",
		"topicKey":"children",
		"status":1,
		"remark":"manual"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/generation-topic-policies", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /generation-topic-policies = %d, body=%s", resp.Code, resp.Body.String())
	}
	var created GenerationTopicPolicy
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID == 0 || created.TenantID != 303 || created.TopicKey != "children" || created.Platform != "shein" {
		t.Fatalf("created = %+v, want tenant scoped generation topic policy", created)
	}
}

func TestGenerationTopicPolicyHandlerSoftDeletesWithinTenant(t *testing.T) {
	t.Parallel()

	router := newGenerationTopicPolicyTestRouter(t)
	policy := seedGenerationTopicPolicy(t, router.db, listingGenerationTopicPolicy{
		TenantID: 505,
		Platform: "shein",
		TopicKey: "children",
		Status:   1,
	})

	req := httptest.NewRequest(http.MethodDelete, "/generation-topic-policies/1", nil)
	req.Header.Set("X-Tenant-ID", "505")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /generation-topic-policies/1 = %d, body=%s", resp.Code, resp.Body.String())
	}
	var row listingGenerationTopicPolicy
	if err := router.db.Table("listing_generation_topic_policy").Where("id = ?", policy.ID).Take(&row).Error; err != nil {
		t.Fatalf("load deleted row: %v", err)
	}
	if row.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", row.Deleted)
	}
}

func TestGenerationTopicPolicyHandlerRejectsMissingPlatform(t *testing.T) {
	t.Parallel()

	router := newGenerationTopicPolicyTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/generation-topic-policies", strings.NewReader(`{"topicKey":"children"}`))
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("POST /generation-topic-policies missing platform = %d, body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"invalid_generation_topic_policy"`) {
		t.Fatalf("response body = %s, want invalid_generation_topic_policy", resp.Body.String())
	}
}

func newGenerationTopicPolicyTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	router := newStoreTestRouter(t)
	if err := router.db.AutoMigrate(&listingGenerationTopicPolicy{}); err != nil {
		t.Fatalf("migrate listing_generation_topic_policy: %v", err)
	}
	repo := NewGormGenerationTopicPolicyRepository(router.db)
	handler := NewGenerationTopicPolicyHandler(repo)
	router.engine.GET("/generation-topic-policies", handler.ListGenerationTopicPolicies)
	router.engine.POST("/generation-topic-policies", handler.CreateGenerationTopicPolicy)
	router.engine.DELETE("/generation-topic-policies/:id", handler.DeleteGenerationTopicPolicy)
	return router
}

func seedGenerationTopicPolicy(t *testing.T, db *gorm.DB, policy listingGenerationTopicPolicy) listingGenerationTopicPolicy {
	t.Helper()
	if err := db.Table("listing_generation_topic_policy").Create(&policy).Error; err != nil {
		t.Fatalf("seed generation topic policy: %v", err)
	}
	return policy
}
