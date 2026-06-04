package shein

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit/tenantctx"
)

func TestAdminGenerationTopicPolicyCreationFlowsIntoPromptAndPreviewSanitizer(t *testing.T) {
	repo := newGenerationTopicPolicyAdminRepo(t)
	createGenerationTopicPolicyViaAdmin(t, repo, "101", "children")
	createGenerationTopicPolicyViaAdmin(t, repo, "202", "meals")

	restorePromptRepo := SetGenerationTopicPolicyRepository(repo)
	defer restorePromptRepo()
	restoreSanitizerRepo := ConfigureSubmitPrepRepositories(nil, repo, nil)
	defer restoreSanitizerRepo()

	ai := &recordingChatCompleter{
		response: &openaiclient.ChatCompletionResponse{
			Choices: []openaiclient.ChatCompletionChoice{{
				Message: openaiclient.ChatCompletionMessage{
					Content: `{"title":"Door Curtain","description":"A door curtain."}`,
				},
			}},
		},
	}

	if _, _, err := optimizeSubmitContentWithAI(
		tenantctx.WithTenantID(context.Background(), "101"),
		ai,
		"Kids room curtain",
		"Decor for children bedroom",
		"",
		nil,
	); err != nil {
		t.Fatalf("optimizeSubmitContentWithAI returned error: %v", err)
	}
	if ai.lastReq == nil || len(ai.lastReq.Messages) == 0 {
		t.Fatalf("ai request = %+v, want system prompt", ai.lastReq)
	}
	systemPrompt := ai.lastReq.Messages[0].Content
	if !strings.Contains(systemPrompt, "Do not mention children, babies, or age-specific users.") {
		t.Fatalf("system prompt = %q, want children directive from admin-created policy", systemPrompt)
	}
	if strings.Contains(systemPrompt, "breakfast") || strings.Contains(systemPrompt, "meal") {
		t.Fatalf("system prompt = %q, want tenant 101 prompt to exclude tenant 202 directives", systemPrompt)
	}

	copyA := buildSheinListingCopy(tenantctx.WithTenantID(context.Background(), "101"), &canonical.Product{
		Title:       "Kids Room Curtain",
		Description: "Decor for children bedroom",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Kids Room Curtain"},
		},
	}, "Kids Room Curtain", nil)
	copyB := buildSheinListingCopy(tenantctx.WithTenantID(context.Background(), "202"), &canonical.Product{
		Title:       "Breakfast Table Curtain",
		Description: "Meal-themed decor",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Breakfast Table Curtain"},
		},
	}, "Breakfast Table Curtain", nil)

	if strings.Contains(strings.ToLower(copyA.Title), "kids") || strings.Contains(strings.ToLower(copyA.Description), "children") {
		t.Fatalf("tenant 101 copy = %+v, want children terms removed", copyA)
	}
	if strings.Contains(strings.ToLower(copyB.Title), "breakfast") || strings.Contains(strings.ToLower(copyB.Description), "meal") {
		t.Fatalf("tenant 202 copy = %+v, want meal terms removed", copyB)
	}
}

func TestAdminGenerationTopicOverrideFlowsIntoPromptAndPreviewSanitizer(t *testing.T) {
	policyRepo, overrideRepo := newGenerationTopicAdminRepos(t)
	createGenerationTopicPolicyViaAdmin(t, policyRepo, "101", "children")
	createGenerationTopicOverrideViaAdmin(t, overrideRepo, "101", `{
		"platform":"shein",
		"topicKey":"children",
		"additionalPromptDirectives":["Avoid toddler-focused positioning."],
		"additionalLexiconByLanguage":{"en":["toddler"]},
		"status":1
	}`)

	restorePromptPolicyRepo := SetGenerationTopicPolicyRepository(policyRepo)
	defer restorePromptPolicyRepo()
	restorePromptOverrideRepo := SetGenerationTopicOverrideRepository(overrideRepo)
	defer restorePromptOverrideRepo()
	restoreSanitizerRepos := ConfigureSubmitPrepRepositories(nil, policyRepo, overrideRepo)
	defer restoreSanitizerRepos()

	ai := &recordingChatCompleter{
		response: &openaiclient.ChatCompletionResponse{
			Choices: []openaiclient.ChatCompletionChoice{{
				Message: openaiclient.ChatCompletionMessage{
					Content: `{"title":"Door Curtain","description":"A door curtain."}`,
				},
			}},
		},
	}

	if _, _, err := optimizeSubmitContentWithAI(
		tenantctx.WithTenantID(context.Background(), "101"),
		ai,
		"Toddler room curtain",
		"Decor for toddler bedroom",
		"",
		nil,
	); err != nil {
		t.Fatalf("optimizeSubmitContentWithAI returned error: %v", err)
	}
	if ai.lastReq == nil || len(ai.lastReq.Messages) == 0 {
		t.Fatalf("ai request = %+v, want system prompt", ai.lastReq)
	}
	systemPrompt := ai.lastReq.Messages[0].Content
	if !strings.Contains(systemPrompt, "Do not mention children, babies, or age-specific users.") {
		t.Fatalf("system prompt = %q, want default directive retained", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "Avoid toddler-focused positioning.") {
		t.Fatalf("system prompt = %q, want override directive included", systemPrompt)
	}

	copy := buildSheinListingCopy(tenantctx.WithTenantID(context.Background(), "101"), &canonical.Product{
		Title:       "Toddler Room Curtain",
		Description: "Toddler decor for children bedroom",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Toddler Room Curtain"},
		},
	}, "Toddler Room Curtain", nil)

	if strings.Contains(strings.ToLower(copy.Title), "toddler") {
		t.Fatalf("tenant copy title = %q, want override lexicon removed", copy.Title)
	}
	if strings.Contains(strings.ToLower(copy.Description), "toddler") {
		t.Fatalf("tenant copy description = %q, want override lexicon removed", copy.Description)
	}
}

func newGenerationTopicPolicyAdminRepo(t *testing.T) listingadmin.GenerationTopicPolicyRepository {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := listingadmin.AutoMigrateGenerationTopicPolicyRepository(db); err != nil {
		t.Fatalf("migrate generation topic policy: %v", err)
	}
	return listingadmin.NewGormGenerationTopicPolicyRepository(db)
}

func newGenerationTopicAdminRepos(t *testing.T) (listingadmin.GenerationTopicPolicyRepository, listingadmin.GenerationTopicOverrideRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := listingadmin.AutoMigrateGenerationTopicPolicyRepository(db); err != nil {
		t.Fatalf("migrate generation topic policy: %v", err)
	}
	if err := listingadmin.AutoMigrateGenerationTopicOverrideRepository(db); err != nil {
		t.Fatalf("migrate generation topic override: %v", err)
	}
	return listingadmin.NewGormGenerationTopicPolicyRepository(db), listingadmin.NewGormGenerationTopicOverrideRepository(db)
}

func createGenerationTopicPolicyViaAdmin(t *testing.T, repo listingadmin.GenerationTopicPolicyRepository, tenantID string, topicKey string) {
	t.Helper()
	handler := listingadmin.NewGenerationTopicPolicyHandler(repo)
	engine := gin.New()
	engine.POST("/generation-topic-policies", handler.CreateGenerationTopicPolicy)

	body := bytes.NewBufferString(`{"platform":"shein","topicKey":"` + topicKey + `","status":1}`)
	req := httptest.NewRequest(http.MethodPost, "/generation-topic-policies", body)
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-User-ID", "user-"+tenantID)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /generation-topic-policies = %d, body=%s", resp.Code, resp.Body.String())
	}

	var created listingadmin.GenerationTopicPolicy
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.TopicKey != topicKey || created.Platform != "shein" {
		t.Fatalf("created policy = %+v, want topicKey=%q platform=shein", created, topicKey)
	}
}

func createGenerationTopicOverrideViaAdmin(t *testing.T, repo listingadmin.GenerationTopicOverrideRepository, tenantID string, payload string) {
	t.Helper()
	handler := listingadmin.NewGenerationTopicOverrideHandler(repo)
	engine := gin.New()
	engine.POST("/generation-topic-overrides", handler.CreateGenerationTopicOverride)

	req := httptest.NewRequest(http.MethodPost, "/generation-topic-overrides", bytes.NewBufferString(payload))
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-User-ID", "user-"+tenantID)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /generation-topic-overrides = %d, body=%s", resp.Code, resp.Body.String())
	}
}
