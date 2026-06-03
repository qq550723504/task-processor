# SHEIN Tenant Generation Topics Implementation Plan

## Status

Status as of 2026-06-04: Mostly implemented.

This plan's core runtime and admin flow now exist in production code:

- Tenant topic policy persistence:
  - `internal/listingadmin/generation_topic_policy.go`
  - `internal/listingadmin/generation_topic_policy_repository.go`
- Topic runtime and sanitizer integration:
  - `internal/publishing/shein/generation_topic_runtime.go`
  - `internal/shein/submitprep/sensitive_words.go`
- Admin-facing handler surface:
  - `internal/listingkit/api/admin_generation_topic_policy_handler.go`

This document should no longer be treated as an open implementation plan.
If future work is needed here, it should start from a fresh gap-analysis against the current implementation rather than resuming these checklist steps verbatim.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add tenant-specific SHEIN generation-topic restrictions that keep prompts compact while reusing the existing sensitive-word sanitizer as the post-generation fallback.

**Architecture:** Add a new listingadmin repository for tenant-enabled generation-topic keys, a code-defined SHEIN topic registry, a compact prompt-policy builder, and a topic-lexicon overlay inside the existing `NewSensitiveWordServiceForContext(ctx)` path. Inject prompt-policy summaries only into the SHEIN title-resolution and submit-content AI prompt entry points.

**Tech Stack:** Go, GORM, existing listingadmin repository conventions, existing SHEIN publishing flow, Go tests

---

### Task 1: Add Tenant Generation Topic Policy Persistence

**Files:**
- Create: `internal/listingadmin/generation_topic_policy.go`
- Create: `internal/listingadmin/generation_topic_policy_repository.go`
- Create: `internal/listingadmin/generation_topic_policy_test.go`
- Create: `internal/listingadmin/generation_topic_policy_query_test.go`
- Modify: `internal/listingadmin/schema_migrate.go`

- [ ] **Step 1: Write the failing repository test**

```go
func TestGormGenerationTopicPolicyRepository_ListEnabledTopicsByTenantAndPlatform(t *testing.T) {
	db := openListingAdminTestDB(t)
	if err := db.AutoMigrate(&listingGenerationTopicPolicy{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewGormGenerationTopicPolicyRepository(db)
	ctx := requestContextWithUser(context.Background(), "planner")

	_, err := repo.CreateGenerationTopicPolicy(ctx, &GenerationTopicPolicy{
		TenantID: 101,
		Platform: "shein",
		TopicKey: "children",
		Status:   1,
	})
	if err != nil {
		t.Fatalf("create enabled topic: %v", err)
	}
	_, err = repo.CreateGenerationTopicPolicy(ctx, &GenerationTopicPolicy{
		TenantID: 101,
		Platform: "shein",
		TopicKey: "food",
		Status:   0,
	})
	if err != nil {
		t.Fatalf("create disabled topic: %v", err)
	}

	items, err := repo.ListEnabledTopicKeys(ctx, 101, "shein")
	if err != nil {
		t.Fatalf("list enabled topic keys: %v", err)
	}
	if diff := cmp.Diff([]string{"children"}, items); diff != "" {
		t.Fatalf("enabled topic keys mismatch (-want +got):\n%s", diff)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingadmin -run "GenerationTopicPolicy" -count=1`

Expected: FAIL with undefined `GenerationTopicPolicy`, `NewGormGenerationTopicPolicyRepository`, or missing table definitions.

- [ ] **Step 3: Add the model, query, and repository types**

```go
type GenerationTopicPolicy struct {
	ID         int64      `json:"id"`
	TenantID   int64      `json:"tenantId"`
	Platform   string     `json:"platform"`
	TopicKey   string     `json:"topicKey"`
	Remark     string     `json:"remark,omitempty"`
	Status     int16      `json:"status"`
	CreateTime *time.Time `json:"createTime,omitempty"`
	UpdateTime *time.Time `json:"updateTime,omitempty"`
}

type GenerationTopicPolicyRepository interface {
	ListGenerationTopicPolicies(ctx context.Context, query GenerationTopicPolicyQuery) (*GenerationTopicPolicyPage, error)
	ListEnabledTopicKeys(ctx context.Context, tenantID int64, platform string) ([]string, error)
	CreateGenerationTopicPolicy(ctx context.Context, policy *GenerationTopicPolicy) (*GenerationTopicPolicy, error)
	UpdateGenerationTopicPolicy(ctx context.Context, policy *GenerationTopicPolicy) (*GenerationTopicPolicy, error)
}

type listingGenerationTopicPolicy struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID    int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	Platform    string     `gorm:"column:platform;type:varchar(32);not null;index"`
	TopicKey    string     `gorm:"column:topic_key;type:varchar(64);not null;index"`
	Remark      string     `gorm:"column:remark"`
	Status      int16      `gorm:"column:status;not null;default:0;index"`
	Creator     string     `gorm:"column:creator"`
	CreatedBy   string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime  *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater     string     `gorm:"column:updater"`
	UpdatedBy   string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime  *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted     int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingGenerationTopicPolicy) TableName() string {
	return "listing_generation_topic_policy"
}
```

- [ ] **Step 4: Add migration support**

```go
func AutoMigrateGenerationTopicPolicyRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	if err := db.AutoMigrate(&listingGenerationTopicPolicy{}); err != nil {
		return err
	}
	if err := ensureOwnerAuditColumns(db, (listingGenerationTopicPolicy{}).TableName()); err != nil {
		return err
	}
	return db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_listing_generation_topic_policy_unique
		ON listing_generation_topic_policy (tenant_id, platform, topic_key)
	`).Error
}
```

- [ ] **Step 5: Run repository tests to verify they pass**

Run: `go test ./internal/listingadmin -run "GenerationTopicPolicy" -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingadmin/generation_topic_policy.go internal/listingadmin/generation_topic_policy_repository.go internal/listingadmin/generation_topic_policy_test.go internal/listingadmin/generation_topic_policy_query_test.go internal/listingadmin/schema_migrate.go
git commit -m "feat: add tenant generation topic policy repository"
```

### Task 2: Add SHEIN Topic Registry and Prompt Policy Builder

**Files:**
- Create: `internal/publishing/shein/generation_topics.go`
- Create: `internal/publishing/shein/generation_topics_test.go`

- [ ] **Step 1: Write the failing topic summary builder tests**

```go
func TestBuildSheinGenerationPolicySummary_DeduplicatesAndPrioritizesTopics(t *testing.T) {
	summary := buildSheinGenerationPolicySummary([]string{"children", "food", "children", "knives"})
	for _, expected := range []string{
		"Do not mention children, babies, or age-specific users.",
		"Do not mention food, meals, or edible usage scenarios.",
		"Do not mention knives, blades, or sharp-tool contexts.",
	} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("summary = %q, want directive %q", summary, expected)
		}
	}
	if strings.Count(summary, "Do not mention children") != 1 {
		t.Fatalf("summary = %q, want deduplicated children directive", summary)
	}
}

func TestBuildSheinGenerationPolicySummary_EnforcesLengthLimit(t *testing.T) {
	summary := buildSheinGenerationPolicySummary([]string{"children", "baby", "food", "meals", "knives"})
	if len(summary) > 600 {
		t.Fatalf("summary length = %d, want <= 600", len(summary))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/publishing/shein -run "GenerationPolicySummary|GenerationTopics" -count=1`

Expected: FAIL with undefined `buildSheinGenerationPolicySummary` or missing topic definitions.

- [ ] **Step 3: Add the SHEIN topic registry and builder**

```go
type GenerationTopicDefinition struct {
	Key               string
	PromptDirectives  []string
	LexiconByLanguage map[string][]string
	Priority          int
}

var sheinGenerationTopics = map[string]GenerationTopicDefinition{
	"children": {
		Key:              "children",
		Priority:         10,
		PromptDirectives: []string{"Do not mention children, babies, or age-specific users."},
		LexiconByLanguage: map[string][]string{
			"en": {"child", "children", "kid", "kids"},
			"zh": {"儿童", "小孩", "孩子"},
		},
	},
	"food": {
		Key:              "food",
		Priority:         20,
		PromptDirectives: []string{"Do not mention food, meals, or edible usage scenarios."},
		LexiconByLanguage: map[string][]string{
			"en": {"food", "snack", "edible"},
			"zh": {"食物", "食品", "零食"},
		},
	},
}

func buildSheinGenerationPolicySummary(topicKeys []string) string {
	seen := map[string]struct{}{}
	defs := make([]GenerationTopicDefinition, 0, len(topicKeys))
	for _, key := range topicKeys {
		def, ok := sheinGenerationTopics[strings.TrimSpace(key)]
		if !ok {
			continue
		}
		if _, exists := seen[def.Key]; exists {
			continue
		}
		seen[def.Key] = struct{}{}
		defs = append(defs, def)
	}
	slices.SortFunc(defs, func(a, b GenerationTopicDefinition) int {
		return cmp.Compare(a.Priority, b.Priority)
	})
	lines := make([]string, 0, len(defs))
	for _, def := range defs {
		for _, directive := range def.PromptDirectives {
			next := append(lines, directive)
			candidate := strings.Join(next, "\n")
			if len(next) > 5 || len(candidate) > 600 {
				return strings.Join(lines, "\n")
			}
			lines = next
		}
	}
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 4: Run topic-builder tests to verify they pass**

Run: `go test ./internal/publishing/shein -run "GenerationPolicySummary|GenerationTopics" -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/publishing/shein/generation_topics.go internal/publishing/shein/generation_topics_test.go
git commit -m "feat: add shein generation topic registry"
```

### Task 3: Load Tenant Topic Keys and Inject Prompt Policies

**Files:**
- Create: `internal/publishing/shein/generation_topic_runtime.go`
- Create: `internal/publishing/shein/generation_topic_runtime_test.go`
- Modify: `internal/publishing/shein/submit_prep.go`
- Modify: `internal/publishing/shein/title_resolution.go`
- Modify: `internal/listingkit/httpapi/bootstrap_runtime.go`

- [ ] **Step 1: Write the failing prompt-injection tests**

```go
func TestOptimizeSubmitContentWithAI_IncludesTenantGenerationPolicy(t *testing.T) {
	repo := &stubGenerationTopicPolicyRepository{
		items: map[int64][]listingadmin.GenerationTopicPolicy{
			101: {{TenantID: 101, Platform: "shein", TopicKey: "children", Status: 1}},
		},
	}
	restore := SetGenerationTopicPolicyRepository(repo)
	defer restore()

	ai := &captureChatCompleter{}
	ctx := tenantctx.WithTenantID(context.Background(), "101")

	_, _, _ = optimizeSubmitContentWithAI(ctx, ai, "Door Curtain", "Home decor curtain", "", nil)

	if !strings.Contains(ai.lastSystemPrompt, "Do not mention children, babies, or age-specific users.") &&
		!strings.Contains(ai.lastUserPrompt, "Do not mention children, babies, or age-specific users.") {
		t.Fatalf("captured prompts = %#v, want tenant generation policy directive", ai)
	}
}

func TestExtractPromptTitleWithLLM_IncludesTenantGenerationPolicy(t *testing.T) {
	repo := &stubGenerationTopicPolicyRepository{
		items: map[int64][]listingadmin.GenerationTopicPolicy{
			101: {{TenantID: 101, Platform: "shein", TopicKey: "food", Status: 1}},
		},
	}
	restore := SetGenerationTopicPolicyRepository(repo)
	defer restore()

	ai := &captureChatCompleter{}
	ctx := tenantctx.WithTenantID(context.Background(), "101")

	_ = extractPromptTitleWithLLM(ctx, "please design breakfast curtain art", nil, "Curtain", ai)

	if !strings.Contains(ai.lastUserPrompt, "Do not mention food, meals, or edible usage scenarios.") {
		t.Fatalf("user prompt = %q, want food restriction", ai.lastUserPrompt)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/publishing/shein -run "IncludesTenantGenerationPolicy" -count=1`

Expected: FAIL because generation-topic repository wiring and prompt injection do not exist yet.

- [ ] **Step 3: Add repository injection and runtime helper**

```go
var (
	generationTopicPolicyRepoMu sync.RWMutex
	generationTopicPolicyRepo   listingadmin.GenerationTopicPolicyRepository
)

func SetGenerationTopicPolicyRepository(repo listingadmin.GenerationTopicPolicyRepository) func() {
	generationTopicPolicyRepoMu.Lock()
	previous := generationTopicPolicyRepo
	generationTopicPolicyRepo = repo
	generationTopicPolicyRepoMu.Unlock()
	return func() {
		generationTopicPolicyRepoMu.Lock()
		generationTopicPolicyRepo = previous
		generationTopicPolicyRepoMu.Unlock()
	}
}

func buildTenantGenerationPolicySummary(ctx context.Context, platform string) string {
	repo := currentGenerationTopicPolicyRepository()
	if ctx == nil || repo == nil || strings.TrimSpace(platform) == "" {
		return ""
	}
	tenantID, ok := tenantIDFromContext(ctx)
	if !ok {
		return ""
	}
	keys, err := repo.ListEnabledTopicKeys(ctx, tenantID, platform)
	if err != nil || len(keys) == 0 {
		return ""
	}
	return buildSheinGenerationPolicySummary(keys)
}
```

- [ ] **Step 4: Inject the policy into the two prompt entry points**

```go
policy := buildTenantGenerationPolicySummary(ctx, "shein")
if policy != "" {
	systemPrompt += "\nAdditional tenant content restrictions:\n" + policy
}
```

Apply this pattern in:

- `optimizeSubmitContentWithAI(ctx, ...)`
- `extractListingTitleAdditionWithLLM(ctx, ...)`
- `extractPromptTitleWithLLM(ctx, ...)`

Update the helper signatures so the context is available:

```go
func extractPromptTitleWithLLM(ctx context.Context, promptText string, canonical *canonical.Product, fallbackTitle string, aiClient openaiclient.ChatCompleter) string
func extractListingTitleAdditionWithLLM(ctx context.Context, baseTitle string, canonical *canonical.Product, fallbackTitle string, aiClient openaiclient.ChatCompleter) string
```

- [ ] **Step 5: Wire the repository during bootstrap**

```go
submitprep.SetGenerationTopicPolicyRepository(repositories.generationTopicPolicyRepository)
shein.SetGenerationTopicPolicyRepository(repositories.generationTopicPolicyRepository)
```

- [ ] **Step 6: Run prompt-injection tests to verify they pass**

Run: `go test ./internal/publishing/shein -run "IncludesTenantGenerationPolicy" -count=1`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/publishing/shein/generation_topic_runtime.go internal/publishing/shein/generation_topic_runtime_test.go internal/publishing/shein/submit_prep.go internal/publishing/shein/title_resolution.go internal/listingkit/httpapi/bootstrap_runtime.go
git commit -m "feat: inject tenant generation topic policies into shein prompts"
```

### Task 4: Overlay Topic Lexicons into the Existing Sensitive-Word Service

**Files:**
- Modify: `internal/shein/submitprep/sensitive_words.go`
- Create: `internal/shein/submitprep/generation_topic_overlay_test.go`

- [ ] **Step 1: Write the failing overlay test**

```go
func TestNewSensitiveWordServiceForContext_OverlaysTenantGenerationTopicLexicon(t *testing.T) {
	restoreRepo := SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		items: map[int64][]listingadmin.GenerationTopicPolicy{
			101: {{TenantID: 101, Platform: "shein", TopicKey: "children", Status: 1}},
		},
	})
	defer restoreRepo()

	ctx := tenantctx.WithTenantID(context.Background(), "101")
	service := NewSensitiveWordServiceForContext(ctx)
	got := service.SanitizeDisplayTextWithContext(nil, "Kids room curtain for home decor")
	if strings.Contains(strings.ToLower(got), "kids") {
		t.Fatalf("sanitized text = %q, want children topic lexicon removed", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/shein/submitprep -run "GenerationTopicLexicon" -count=1`

Expected: FAIL because `NewSensitiveWordServiceForContext(ctx)` does not yet overlay topic lexicons.

- [ ] **Step 3: Extend the service-construction path**

```go
func overlayGenerationTopicLexicons(ctx context.Context, service *sheincontent.SensitiveWordService) {
	if ctx == nil || service == nil {
		return
	}
	repo := currentGenerationTopicPolicyRepository()
	if repo == nil {
		return
	}
	tenantID, ok := tenantIDFromContext(ctx)
	if !ok {
		return
	}
	keys, err := repo.ListEnabledTopicKeys(ctx, tenantID, "shein")
	if err != nil {
		return
	}
	for language, words := range collectSheinTopicLexicons(keys) {
		service.AddStaticSensitiveWordsByLanguage(language, words)
	}
}

func NewSensitiveWordServiceForContext(ctx context.Context) *sheincontent.SensitiveWordService {
	service := sheincontent.NewSensitiveWordServiceWithPath(sensitiveWordsConfigPath())
	overlaySensitiveWordsFromRepository(ctx, service)
	overlayGenerationTopicLexicons(ctx, service)
	return service
}
```

- [ ] **Step 4: Run submitprep tests to verify the overlay passes**

Run: `go test ./internal/shein/submitprep -run "GenerationTopicLexicon" -count=1`

Expected: PASS

- [ ] **Step 5: Run affected SHEIN publishing tests**

Run: `go test ./internal/publishing/shein -run "Sensitive|Submit|ListingCopy|GenerationPolicy" -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/shein/submitprep/sensitive_words.go internal/shein/submitprep/generation_topic_overlay_test.go
git commit -m "feat: overlay tenant generation topic lexicons into shein sanitizer"
```

### Task 5: Run Regression Coverage and Document Operational Limits

**Files:**
- Modify: `internal/publishing/shein/review_content_test.go`
- Modify: `internal/publishing/shein/submit_prep_test.go`
- Modify: `internal/publishing/shein/listing_copy_test.go`
- Modify: `docs/superpowers/specs/2026-06-02-shein-tenant-generation-topics-design.md`

- [ ] **Step 1: Add regression tests for tenant variance and fallback cleanup**

```go
func TestBuildSheinListingCopy_DifferentTenantsLoadDifferentGenerationTopicLexicons(t *testing.T) {
	restore := submitprep.SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		items: map[int64][]listingadmin.GenerationTopicPolicy{
			101: {{TenantID: 101, Platform: "shein", TopicKey: "children", Status: 1}},
			202: {{TenantID: 202, Platform: "shein", TopicKey: "food", Status: 1}},
		},
	})
	defer restore()

	copyA := buildSheinListingCopy(tenantctx.WithTenantID(context.Background(), "101"), &canonical.Product{
		Title:       "Kids Room Curtain",
		Description: "Decor for children bedroom",
	}, "Kids Room Curtain", nil)
	copyB := buildSheinListingCopy(tenantctx.WithTenantID(context.Background(), "202"), &canonical.Product{
		Title:       "Breakfast Table Curtain",
		Description: "Meal-themed decor",
	}, "Breakfast Table Curtain", nil)

	if strings.Contains(strings.ToLower(copyA.Title), "kids") {
		t.Fatalf("tenant 101 title = %q, want children term removed", copyA.Title)
	}
	if strings.Contains(strings.ToLower(copyB.Title), "breakfast") {
		t.Fatalf("tenant 202 title = %q, want meal term removed", copyB.Title)
	}
}
```

- [ ] **Step 2: Run focused regression tests**

Run: `go test ./internal/publishing/shein -run "DifferentTenantsLoadDifferentGenerationTopicLexicons|IncludesTenantGenerationPolicy|Sensitive|Submit" -count=1`

Expected: PASS

- [ ] **Step 3: Run broader package verification**

Run: `go test ./internal/publishing/shein -count=1`

Expected: PASS

Run: `go test ./internal/shein/... -run "Sensitive|Submit|Content" -count=1`

Expected: PASS

- [ ] **Step 4: Update the spec with implementation notes if behavior deviates**

```md
## Implementation Notes

- Prompt summaries are capped at five directives or 600 characters, whichever comes first.
- Topic lexicons are overlaid as static words inside `NewSensitiveWordServiceForContext(ctx)`.
- First version supports only `platform = "shein"`.
```

- [ ] **Step 5: Commit**

```bash
git add internal/publishing/shein/review_content_test.go internal/publishing/shein/submit_prep_test.go internal/publishing/shein/listing_copy_test.go docs/superpowers/specs/2026-06-02-shein-tenant-generation-topics-design.md
git commit -m "test: add shein generation topic regression coverage"
```

## Self-Review

- Spec coverage:
  - Tenant topic persistence: Task 1
  - Code-defined topic registry: Task 2
  - Prompt summary injection: Task 3
  - Sensitive-word overlay reuse: Task 4
  - Regression and operational limits: Task 5
- Placeholder scan:
  - No `TODO`, `TBD`, or unresolved task references remain.
- Type consistency:
  - `GenerationTopicPolicy`, `GenerationTopicPolicyRepository`, `buildSheinGenerationPolicySummary`, and `buildTenantGenerationPolicySummary` are used consistently across tasks.
