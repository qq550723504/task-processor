# SHEIN Topic Catalog Overrides Implementation Plan

## Status

Status as of 2026-06-04: Mostly implemented.

The main contract described here is now present in the codebase:

- Override model and repository:
  - `internal/listingadmin/generation_topic_override.go`
  - `internal/listingadmin/generation_topic_override_repository.go`
- Admin handlers and ListingKit-facing wiring:
  - `internal/listingadmin/generation_topic_override_handler.go`
  - `internal/listingadmin/generation_topic_catalog_handler.go`
  - `internal/listingkit/api/admin_generation_topic_override_handler.go`
- Runtime merge consumers:
  - `internal/publishing/shein/generation_topic_runtime.go`
  - `internal/shein/submitprep/sensitive_words.go`
- Admin UI client and page:
  - `web/listingkit-ui/src/lib/api/admin-generation-topic-overrides.ts`
  - `web/listingkit-ui/src/components/listingkit/admin/generation-topic-policy-admin-page.tsx`

This document should be treated as historical planning context, not as a still-open execution checklist.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only SHEIN topic catalog API plus tenant-level additive topic overrides, and make both prompt generation and sanitizer runtime consume the merged definitions.

**Architecture:** Keep the platform topic catalog in `internal/shein/generationtopics/registry.go` as the single default source of truth. Add a tenant-scoped override repository and admin APIs, then merge default definitions with tenant additions in one shared runtime path used by both prompt directives and sanitizer lexicons. Update the admin UI to load the catalog from the backend and edit additive overrides instead of hardcoding topic keys.

**Tech Stack:** Go, Gin, Gorm, Next.js App Router, TypeScript, Zod, Vitest, existing ListingKit admin patterns.

---

## File Structure

### Backend data and merge logic

- Modify: `internal/shein/generationtopics/registry.go`
  - Add helper APIs for catalog listing and merged-definition assembly.
- Create: `internal/listingadmin/generation_topic_override.go`
  - Tenant override domain types and DB row mapping.
- Create: `internal/listingadmin/generation_topic_override_repository.go`
  - Gorm repository, auto-migrate, list/get/create/update/status/delete.
- Create: `internal/listingadmin/generation_topic_override_handler.go`
  - Admin CRUD handler plus validation.
- Create: `internal/listingadmin/generation_topic_catalog_handler.go`
  - Read-only catalog endpoint backed by registry + optional tenant overrides.

### Backend runtime wiring

- Modify: `internal/listingkit/api/handler.go`
  - Add override handler fields.
- Modify: `internal/listingkit/api/admin_dependencies.go`
  - Wire new repositories into admin handlers.
- Create: `internal/listingkit/api/admin_generation_topic_override_handler.go`
  - ListingKit-facing admin endpoints for override CRUD and catalog read.
- Modify: `internal/listingkit/httpapi/routes.go`
  - Register new endpoints.
- Modify: `internal/listingkit/httpapi/builders.go`
  - Build DB repository for overrides.
- Modify: `internal/listingkit/httpapi/bootstrap.go`
  - Add repository builder dependency.
- Modify: `internal/listingkit/httpapi/bootstrap_repositories.go`
  - Compose built override repository into admin/runtime dependencies.
- Modify: `internal/listingkit/httpapi/bootstrap_admin_module.go`
  - Include override repository in admin module input.
- Modify: `internal/listingkit/httpapi/bootstrap_runtime.go`
  - Register override repository for generation topic runtime merge.
- Modify: `internal/publishing/shein/generation_topic_runtime.go`
  - Load merged directives from default catalog + tenant overrides.
- Modify: `internal/shein/submitprep/sensitive_words.go`
  - Load merged lexicon overlay from default catalog + tenant overrides.

### Frontend

- Create: `web/listingkit-ui/src/lib/api/admin-generation-topic-overrides.ts`
  - API client and schemas for catalog + override CRUD.
- Modify: `web/listingkit-ui/src/components/listingkit/admin/generation-topic-policy-admin-page.tsx`
  - Load topic catalog from API, render default/effective definitions, edit additive overrides.
- Create: `web/listingkit-ui/src/lib/api/admin-generation-topic-overrides.test.ts`
  - API client parsing tests.
- Modify: `web/listingkit-ui/src/components/listingkit/admin/generation-topic-policy-admin-page.test.tsx`
  - UI behavior tests for catalog loading and override editing.

### Tests

- Modify: `internal/listingadmin/generation_topic_policy_handler_test.go`
  - Reuse existing router setup patterns where helpful.
- Create: `internal/listingadmin/generation_topic_override_handler_test.go`
  - Override CRUD and validation tests.
- Create: `internal/listingadmin/generation_topic_catalog_handler_test.go`
  - Catalog response and merge tests.
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`
  - Ensure override repo is composed and not dropped.
- Modify: `internal/publishing/shein/generation_topic_runtime_test.go`
  - Prompt summary merge tests.
- Modify: `internal/publishing/shein/listing_copy_test.go`
  - Preview sanitizer merge tests.
- Modify: `internal/publishing/shein/submit_prep_test.go`
  - Submit-time merged lexicon tests.
- Modify: `internal/publishing/shein/generation_topic_policy_integration_test.go`
  - End-to-end admin override to prompt/sanitizer behavior.

## Task 1: Add failing backend tests for topic catalog and overrides

**Files:**
- Create: `internal/listingadmin/generation_topic_override_handler_test.go`
- Create: `internal/listingadmin/generation_topic_catalog_handler_test.go`
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`

- [ ] **Step 1: Write the failing catalog handler test**

```go
func TestGenerationTopicCatalogHandlerReturnsDefaultDefinitions(t *testing.T) {
	t.Parallel()

	repo := listingadmin.NewGenerationTopicOverrideMemRepository()
	handler := listingadmin.NewGenerationTopicCatalogHandler(repo)

	router := gin.New()
	router.GET("/catalog", withIdentityHeaders("101", "local-dev"), handler.ListGenerationTopicCatalog)

	req := httptest.NewRequest(http.MethodGet, "/catalog?platform=shein", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"key\":\"children\"") {
		t.Fatalf("body = %s, want children topic in catalog", resp.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingadmin -run "GenerationTopicCatalogHandlerReturnsDefaultDefinitions" -count=1`

Expected: FAIL because `GenerationTopicCatalogHandler` and/or in-memory override repo do not exist yet.

- [ ] **Step 3: Write the failing override validation test**

```go
func TestGenerationTopicOverrideHandlerRejectsUnknownTopicKey(t *testing.T) {
	t.Parallel()

	repo := listingadmin.NewGenerationTopicOverrideMemRepository()
	handler := listingadmin.NewGenerationTopicOverrideHandler(repo)

	router := gin.New()
	router.POST("/overrides", withIdentityHeaders("101", "local-dev"), handler.CreateGenerationTopicOverride)

	req := httptest.NewRequest(http.MethodPost, "/overrides", strings.NewReader(`{
		"platform":"shein",
		"topic_key":"unknown-topic",
		"additional_prompt_directives":["Avoid this term"]
	}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", resp.Code, resp.Body.String())
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/listingadmin -run "GenerationTopicOverrideHandlerRejectsUnknownTopicKey" -count=1`

Expected: FAIL because override handler/repository do not exist yet.

- [ ] **Step 5: Write the failing bootstrap composition test**

```go
func TestBuildAdminRepositoriesIncludesGenerationTopicOverrideRepository(t *testing.T) {
	t.Parallel()

	input := buildSuccessfulServiceInputFixture()
	input.Repositories.Admin.GenerationTopicOverride = func(*config.Config, *logrus.Logger) (listingadmin.GenerationTopicOverrideRepository, []func() error, error) {
		return &listingadmin.GormGenerationTopicOverrideRepository{}, nil, nil
	}
	closers := &closerStack{}

	adminRepos, err := buildAdminRepositories(input, closers)
	if err != nil {
		t.Fatalf("buildAdminRepositories: %v", err)
	}
	if adminRepos.generationTopicOverrideRepository == nil {
		t.Fatal("expected generation topic override repository")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/listingkit/httpapi -run "BuildAdminRepositoriesIncludesGenerationTopicOverrideRepository" -count=1`

Expected: FAIL because the new repository field and builder hook do not exist yet.

- [ ] **Step 7: Commit**

```bash
git add internal/listingadmin/generation_topic_override_handler_test.go internal/listingadmin/generation_topic_catalog_handler_test.go internal/listingkit/httpapi/bootstrap_test.go
git commit -m "test: add failing tests for shein topic catalog overrides"
```

## Task 2: Implement override domain model and repository

**Files:**
- Create: `internal/listingadmin/generation_topic_override.go`
- Create: `internal/listingadmin/generation_topic_override_repository.go`
- Create: `internal/listingadmin/generation_topic_override_handler.go`
- Create: `internal/listingadmin/generation_topic_catalog_handler.go`

- [ ] **Step 1: Add minimal domain types and row mapping**

```go
type GenerationTopicOverride struct {
	ID                          int64               `json:"id"`
	TenantID                    int64               `json:"tenant_id"`
	Platform                    string              `json:"platform"`
	TopicKey                    string              `json:"topic_key"`
	AdditionalPromptDirectives  []string            `json:"additional_prompt_directives"`
	AdditionalLexiconByLanguage map[string][]string `json:"additional_lexicon_by_language"`
	Status                      int16               `json:"status"`
	Remark                      string              `json:"remark"`
}

type listingGenerationTopicOverride struct {
	ID                             int64  `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID                       int64  `gorm:"column:tenant_id"`
	Platform                       string `gorm:"column:platform"`
	TopicKey                       string `gorm:"column:topic_key"`
	AdditionalPromptDirectivesJSON string `gorm:"column:additional_prompt_directives_json"`
	AdditionalLexiconJSON          string `gorm:"column:additional_lexicon_json"`
	Status                         int16  `gorm:"column:status"`
	Remark                         string `gorm:"column:remark"`
	BaseAuditModel
}
```

- [ ] **Step 2: Run listingadmin tests to verify compile still fails at repository methods**

Run: `go test ./internal/listingadmin -run "GenerationTopic(Catalog|Override)" -count=1`

Expected: FAIL because repository and handlers are still incomplete.

- [ ] **Step 3: Implement repository interface and auto-migrate**

```go
type GenerationTopicOverrideRepository interface {
	ListGenerationTopicOverrides(ctx context.Context, query GenerationTopicOverrideQuery) (*GenerationTopicOverridePage, error)
	GetGenerationTopicOverride(ctx context.Context, tenantID, id int64) (*GenerationTopicOverride, error)
	GetGenerationTopicOverrideByTopicKey(ctx context.Context, tenantID int64, platform string, topicKey string) (*GenerationTopicOverride, error)
	CreateGenerationTopicOverride(ctx context.Context, item *GenerationTopicOverride) (*GenerationTopicOverride, error)
	UpdateGenerationTopicOverride(ctx context.Context, item *GenerationTopicOverride) (*GenerationTopicOverride, error)
	UpdateGenerationTopicOverrideStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*GenerationTopicOverride, error)
	DeleteGenerationTopicOverride(ctx context.Context, tenantID, id int64) error
}
```

- [ ] **Step 4: Implement catalog and override handlers with topic-key validation against registry**

```go
func validateGenerationTopicOverride(item *GenerationTopicOverride) error {
	if item.TenantID <= 0 {
		return errors.New("tenant id is required")
	}
	if generationtopics.NormalizeKey(item.Platform) != "shein" {
		return errors.New("platform must be shein")
	}
	if _, unknown := generationtopics.ResolveSheinTopicKeys([]string{item.TopicKey}); len(unknown) > 0 {
		return errors.New("topic key must exist in shein topic catalog")
	}
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/listingadmin -run "GenerationTopic(Catalog|Override)" -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingadmin/generation_topic_override.go internal/listingadmin/generation_topic_override_repository.go internal/listingadmin/generation_topic_override_handler.go internal/listingadmin/generation_topic_catalog_handler.go internal/listingadmin/generation_topic_override_handler_test.go internal/listingadmin/generation_topic_catalog_handler_test.go
git commit -m "feat: add shein generation topic override admin backend"
```

## Task 3: Wire override repository into ListingKit runtime and routes

**Files:**
- Modify: `internal/listingkit/api/handler.go`
- Modify: `internal/listingkit/api/admin_dependencies.go`
- Create: `internal/listingkit/api/admin_generation_topic_override_handler.go`
- Modify: `internal/listingkit/httpapi/routes.go`
- Modify: `internal/listingkit/httpapi/builders.go`
- Modify: `internal/listingkit/httpapi/bootstrap.go`
- Modify: `internal/listingkit/httpapi/bootstrap_repositories.go`
- Modify: `internal/listingkit/httpapi/bootstrap_admin_module.go`
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`

- [ ] **Step 1: Add failing route test for catalog endpoint**

```go
func TestListingKitRoutesRegisterGenerationTopicCatalogAndOverrides(t *testing.T) {
	t.Parallel()

	handler := &stubRouteHandler{}
	routes := AppendRouteDescriptors(nil, handler)
	paths := collectRoutePaths(routes)

	if !containsRoute(paths, http.MethodGet, "/api/v1/listing-kits/admin/generation-topic-catalog") {
		t.Fatal("expected generation topic catalog route")
	}
	if !containsRoute(paths, http.MethodPost, "/api/v1/listing-kits/admin/generation-topic-overrides") {
		t.Fatal("expected generation topic override create route")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit/httpapi -run "ListingKitRoutesRegisterGenerationTopicCatalogAndOverrides|BuildAdminRepositoriesIncludesGenerationTopicOverrideRepository" -count=1`

Expected: FAIL because route descriptors and dependency structs are missing.

- [ ] **Step 3: Implement handler fields, dependency structs, route methods, and repository builder**

```go
type catalogAdminHandlers struct {
	// existing handlers...
	generationTopicOverrideHandler *listingadmin.GenerationTopicOverrideHandler
	generationTopicCatalogHandler  *listingadmin.GenerationTopicCatalogHandler
}
```

```go
func BuildListingAdminGenerationTopicOverrideRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.GenerationTopicOverrideRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminGenerationTopicOverrideRepository, func(logger *logrus.Logger) (listingadmin.GenerationTopicOverrideRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit generation topic override admin API disabled")
		return nil, nil, nil
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/listingkit/httpapi -run "ListingKitRoutesRegisterGenerationTopicCatalogAndOverrides|BuildAdminRepositoriesIncludesGenerationTopicOverrideRepository|Build" -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/api/handler.go internal/listingkit/api/admin_dependencies.go internal/listingkit/api/admin_generation_topic_override_handler.go internal/listingkit/httpapi/routes.go internal/listingkit/httpapi/builders.go internal/listingkit/httpapi/bootstrap.go internal/listingkit/httpapi/bootstrap_repositories.go internal/listingkit/httpapi/bootstrap_admin_module.go internal/listingkit/httpapi/bootstrap_test.go
git commit -m "feat: wire shein topic catalog override routes"
```

## Task 4: Merge tenant overrides into generation topic runtime

**Files:**
- Modify: `internal/shein/generationtopics/registry.go`
- Modify: `internal/publishing/shein/generation_topic_runtime.go`
- Modify: `internal/shein/submitprep/sensitive_words.go`
- Modify: `internal/publishing/shein/generation_topic_runtime_test.go`
- Modify: `internal/publishing/shein/listing_copy_test.go`
- Modify: `internal/publishing/shein/submit_prep_test.go`
- Modify: `internal/publishing/shein/generation_topic_policy_integration_test.go`

- [ ] **Step 1: Write the failing runtime merge test**

```go
func TestBuildTenantGenerationPolicySummaryIncludesOverrideDirectives(t *testing.T) {
	topicRepo := &stubGenerationTopicPolicyRepository{tenantTopicKeys: map[int64][]string{101: {"children"}}}
	overrideRepo := &stubGenerationTopicOverrideRepository{items: map[string]listingadmin.GenerationTopicOverride{
		"101:shein:children": {
			TenantID: 101,
			Platform: "shein",
			TopicKey: "children",
			AdditionalPromptDirectives: []string{"Avoid toddler-focused positioning."},
			Status: 1,
		},
	}}
	SetGenerationTopicPolicyRepository(topicRepo)
	SetGenerationTopicOverrideRepository(overrideRepo)

	summary := BuildTenantGenerationPolicySummary(contextWithTenantID(101), "shein")

	if !strings.Contains(summary, "Avoid toddler-focused positioning.") {
		t.Fatalf("summary = %q, want tenant override directive", summary)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/publishing/shein -run "BuildTenantGenerationPolicySummaryIncludesOverrideDirectives" -count=1`

Expected: FAIL because override repository and merge logic are not connected yet.

- [ ] **Step 3: Implement shared merged-definition helpers in generationtopics**

```go
func MergeDefinitionWithOverride(def Definition, override *listingadmin.GenerationTopicOverride) Definition {
	merged := cloneDefinition(def)
	if override == nil || override.Status != 1 {
		return merged
	}
	merged.PromptDirectives = mergeStringLists(merged.PromptDirectives, override.AdditionalPromptDirectives)
	merged.LexiconByLanguage = mergeLexiconMaps(merged.LexiconByLanguage, override.AdditionalLexiconByLanguage)
	return merged
}
```

- [ ] **Step 4: Connect prompt runtime and sanitizer overlay to merged definitions**

```go
func loadMergedTenantTopicDefinitions(ctx context.Context, tenantID int64, platform string, topicKeys []string) ([]generationtopics.Definition, error) {
	defs, _ := generationtopics.ResolveSheinTopicKeys(topicKeys)
	for i := range defs {
		override, err := generationTopicOverrideRepository.GetGenerationTopicOverrideByTopicKey(ctx, tenantID, platform, defs[i].Key)
		if err != nil && !errors.Is(err, listingadmin.ErrGenerationTopicOverrideNotFound) {
			return defs, err
		}
		defs[i] = generationtopics.MergeDefinitionWithOverride(defs[i], override)
	}
	return defs, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/publishing/shein -run "GenerationPolicy|DifferentTenants|AdminGenerationTopicPolicyCreationFlowsIntoPromptAndPreviewSanitizer|BuildTenantGenerationPolicySummaryIncludesOverrideDirectives" -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/shein/generationtopics/registry.go internal/publishing/shein/generation_topic_runtime.go internal/shein/submitprep/sensitive_words.go internal/publishing/shein/generation_topic_runtime_test.go internal/publishing/shein/listing_copy_test.go internal/publishing/shein/submit_prep_test.go internal/publishing/shein/generation_topic_policy_integration_test.go
git commit -m "feat: merge tenant overrides into shein topic runtime"
```

## Task 5: Add frontend catalog API client and page integration

**Files:**
- Create: `web/listingkit-ui/src/lib/api/admin-generation-topic-overrides.ts`
- Create: `web/listingkit-ui/src/lib/api/admin-generation-topic-overrides.test.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/admin/generation-topic-policy-admin-page.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/admin/generation-topic-policy-admin-page.test.tsx`

- [ ] **Step 1: Write the failing API client parsing test**

```ts
it("parses generation topic catalog responses", async () => {
  const response = new Response(JSON.stringify({
    items: [{
      key: "children",
      promptDirectives: ["Do not mention children, babies, or age-specific users."],
      lexiconByLanguage: { en: ["child"], zh: ["儿童"] },
      tenantOverride: {
        additionalPromptDirectives: ["Avoid toddler-focused positioning."],
        additionalLexiconByLanguage: { en: ["toddler"] }
      },
      effectiveDefinition: {
        promptDirectives: ["Do not mention children, babies, or age-specific users.", "Avoid toddler-focused positioning."],
        lexiconByLanguage: { en: ["child", "toddler"], zh: ["儿童"] }
      }
    }]
  }))

  const result = await parseGenerationTopicCatalogResponse(response)
  expect(result.items[0].effectiveDefinition.lexiconByLanguage.en).toContain("toddler")
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm --prefix web/listingkit-ui test -- --run admin-generation-topic-overrides.test.ts`

Expected: FAIL because the client and schema do not exist yet.

- [ ] **Step 3: Write the failing page test for dynamic catalog loading**

```tsx
it("renders topic catalog definitions from the API", async () => {
  vi.spyOn(topicCatalogApi, "getListingGenerationTopicCatalog").mockResolvedValue({
    items: [{
      key: "children",
      promptDirectives: ["Do not mention children, babies, or age-specific users."],
      lexiconByLanguage: { en: ["child", "children"], zh: ["儿童"] },
      tenantOverride: null,
      effectiveDefinition: {
        promptDirectives: ["Do not mention children, babies, or age-specific users."],
        lexiconByLanguage: { en: ["child", "children"], zh: ["儿童"] },
      },
    }],
  })

  render(<GenerationTopicPolicyAdminPage />)

  expect(await screen.findByText("children")).toBeInTheDocument()
  expect(screen.getByText("child, children")).toBeInTheDocument()
})
```

- [ ] **Step 4: Run test to verify it fails**

Run: `npm --prefix web/listingkit-ui test -- --run generation-topic-policy-admin-page.test.tsx`

Expected: FAIL because the page still uses hardcoded topic-key options and has no catalog section.

- [ ] **Step 5: Implement client, schemas, and page rendering**

```ts
export const generationTopicCatalogItemSchema = z.object({
  key: z.string(),
  priority: z.number().optional(),
  promptDirectives: z.array(z.string()),
  lexiconByLanguage: z.record(z.string(), z.array(z.string())),
  tenantOverride: z.object({
    id: z.number().optional(),
    status: z.number().optional(),
    remark: z.string().optional(),
    additionalPromptDirectives: z.array(z.string()).default([]),
    additionalLexiconByLanguage: z.record(z.string(), z.array(z.string())).default({}),
  }).nullable(),
  effectiveDefinition: z.object({
    promptDirectives: z.array(z.string()),
    lexiconByLanguage: z.record(z.string(), z.array(z.string())),
  }),
})
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `npm --prefix web/listingkit-ui test -- --run admin-generation-topic-overrides.test.ts generation-topic-policy-admin-page.test.tsx`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add web/listingkit-ui/src/lib/api/admin-generation-topic-overrides.ts web/listingkit-ui/src/lib/api/admin-generation-topic-overrides.test.ts web/listingkit-ui/src/components/listingkit/admin/generation-topic-policy-admin-page.tsx web/listingkit-ui/src/components/listingkit/admin/generation-topic-policy-admin-page.test.tsx
git commit -m "feat: show shein topic catalog and tenant overrides in admin ui"
```

## Task 6: Full verification and cleanup

**Files:**
- Modify: `docs/superpowers/specs/2026-06-02-shein-topic-catalog-overrides-design.md` if implementation notes are needed

- [ ] **Step 1: Format Go code**

Run:

```bash
gofmt -w internal/listingadmin/generation_topic_override.go internal/listingadmin/generation_topic_override_repository.go internal/listingadmin/generation_topic_override_handler.go internal/listingadmin/generation_topic_catalog_handler.go internal/listingkit/api/admin_generation_topic_override_handler.go internal/listingkit/api/admin_dependencies.go internal/listingkit/api/handler.go internal/listingkit/httpapi/routes.go internal/listingkit/httpapi/builders.go internal/listingkit/httpapi/bootstrap.go internal/listingkit/httpapi/bootstrap_repositories.go internal/listingkit/httpapi/bootstrap_admin_module.go internal/shein/generationtopics/registry.go internal/publishing/shein/generation_topic_runtime.go internal/shein/submitprep/sensitive_words.go
```

Expected: no output

- [ ] **Step 2: Run backend verification**

Run:

```bash
go test ./internal/listingadmin -run "GenerationTopic" -count=1
go test ./internal/listingkit/httpapi -run "Build|Route|GenerationTopic" -count=1
go test ./internal/app/httpapi -run "RegisterRoutes|BuildRegisteredRoutes" -count=1
go test ./internal/publishing/shein -run "GenerationPolicy|DifferentTenants|Prompt|Sensitive|Submit|Integration" -count=1
go test ./internal/shein/... -run "Sensitive|Submit|Content" -count=1
```

Expected: PASS for all commands

- [ ] **Step 3: Run frontend verification**

Run:

```bash
npm --prefix web/listingkit-ui test -- --run admin-generation-topic-overrides.test.ts generation-topic-policy-admin-page.test.tsx
```

Expected: PASS

- [ ] **Step 4: Smoke-test the local page**

Run:

```bash
powershell -ExecutionPolicy Bypass -File scripts/start-listingkit-local-api.ps1
```

Then verify in browser:

- `/listing-kits/admin/generation-topic-policies` loads catalog rows
- topic selector comes from API
- effective definition shows merged tenant additions

Expected: page renders without `503`, and catalog rows are visible.

- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/specs/2026-06-02-shein-topic-catalog-overrides-design.md
git commit -m "docs: update shein topic catalog overrides notes"
```

## Self-Review

- Spec coverage:
  - read-only catalog API is covered in Tasks 1, 2, 3, 5
  - tenant override CRUD is covered in Tasks 1, 2, 3
  - merged runtime behavior is covered in Task 4
  - admin UI catalog visibility and editing is covered in Task 5
- Placeholder scan:
  - no `TBD`, `TODO`, or implicit “write tests later” placeholders remain
- Type consistency:
  - plan consistently uses `GenerationTopicOverrideRepository`, `GenerationTopicCatalogHandler`, `additionalPromptDirectives`, and `additionalLexiconByLanguage`
