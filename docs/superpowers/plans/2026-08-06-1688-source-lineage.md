# 1688 Source Lineage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task with review checkpoints.

**Goal:** Preserve normalized 1688 source identity in the existing ListingKit task request without introducing a new service, table, source model, or marketplace state owner.

**Architecture:** Add an optional `listingkit.SourceReference` to `GenerateRequest`. The existing `GenerateRequestFromSourceFacts` bridge copies identity fields from neutral `catalog.ProductFacts`; the existing 1688 command and task persistence paths then carry the reference unchanged. Source warnings, tenant checks, store checks, and SHEIN workflow ownership remain where they are.

**Tech Stack:** Go 1.26+, existing ListingKit Go packages, GORM task repository, in-memory and SQLite test fixtures, Gin HTTP adapter tests.

## Global Constraints

- Do not create `listing-ai-service`, a new deployment, a new database table, or a migration.
- Do not put raw 1688 crawler payloads or source warnings into `GenerateRequest`.
- Do not change the legacy `/api/v1/listing-kits/generate` request behavior.
- Do not change tenant identity, source/target store authorization, SHEIN preview/readiness, retry, or submission ownership.
- Keep source identity platform-neutral: `SourceKey`, `SourceType`, `SourcePlatform`, `SourceID`, and normalized `SourceURL` only.
- Use TDD: every production change below is preceded by a test that fails for the missing behavior.
- Preserve unrelated working-tree files; stage only files listed in the task being committed.

---

### Task 1: Write the failing source-lineage contract tests

**Files:**

- Modify: `internal/listingkit/product_source_bridge_test.go`
- Modify: `internal/product/sourcehandoff/a1688/command_test.go`
- Create: `internal/listingkit/store/source_reference_persistence_test.go`

**Interfaces:**

- Consumes: existing `catalog.ProductFacts`, `a1688.CreateTaskCommand`, `listingkit.GenerateRequest`, and `store.NewTaskRepository` APIs.
- Produces: failing tests that define `GenerateRequest.Source` as an optional `*listingkit.SourceReference` and verify the existing 1688 command and GORM JSON persistence paths.

- [ ] **Step 1: Add the bridge red test**

In `internal/listingkit/product_source_bridge_test.go`, extend the complete neutral-facts fixture with `SourceType: "crawler"` and assert after `GenerateRequestFromSourceFacts`:

```go
if req.Source == nil {
    t.Fatal("Source = nil, want normalized source reference")
}
if req.Source.Key != "crawler:amazon:B001" ||
    req.Source.Type != "crawler" ||
    req.Source.Platform != "amazon" ||
    req.Source.ID != "B001" ||
    req.Source.URL != "https://www.amazon.com/dp/B001" {
    t.Fatalf("Source = %+v, want normalized source identity", req.Source)
}
```

Add a second test using only `catalog.ProductFacts{Title: "Only Title"}` and assert `req.Source == nil`, while retaining the existing title-only text and empty assets/platforms assertions.

- [ ] **Step 2: Add the 1688 command red test**

In `internal/product/sourcehandoff/a1688/command_test.go`, extend `TestTaskCommandServiceCreateTaskDelegatesToListingKitCreator`:

```go
if creator.request.Source == nil {
    t.Fatal("creator request Source = nil, want 1688 source reference")
}
if creator.request.Source.Key != "crawler:1688:888" ||
    creator.request.Source.Platform != "1688" ||
    creator.request.Source.ID != "888" ||
    creator.request.Source.URL != "https://detail.1688.com/offer/888.html" {
    t.Fatalf("creator request Source = %+v, want normalized 1688 identity", creator.request.Source)
}
```

- [ ] **Step 3: Add the persistence red test**

Create `internal/listingkit/store/source_reference_persistence_test.go` in package `store_test`. Open an in-memory SQLite database with GORM, run `AutoMigrate(&listingkit.Task{})`, create a task with a `GenerateRequest` containing:

```go
Source: &listingkit.SourceReference{
    Key:      "crawler:1688:888",
    Type:     "crawler",
    Platform: "1688",
    ID:       "888",
    URL:      "https://detail.1688.com/offer/888.html",
},
```

Call `store.NewTaskRepository(db).CreateTask` and `GetTask` under the same tenant context, then assert every source-reference field survives the JSON/GORM round trip. Reuse the SQLite and repository setup pattern from `internal/listingkit/store/task_repo_test.go`.

- [ ] **Step 4: Run the red tests**

Run:

```powershell
go test ./internal/listingkit -run 'TestGenerateRequestFromSourceFacts|TestGenerateRequestFromSourceFactsKeepsEmptySourceReference' -count=1
go test ./internal/product/sourcehandoff/a1688 -run TestTaskCommandServiceCreateTaskDelegatesToListingKitCreator -count=1
go test ./internal/listingkit/store -run TestTaskRepositoryPersistsSourceReference -count=1
```

Expected: compilation or assertion failure because `GenerateRequest.Source` and `SourceReference` do not exist yet. Do not implement production code until the failure is observed.

- [ ] **Step 5: Commit the red tests**

```powershell
git add internal/listingkit/product_source_bridge_test.go internal/product/sourcehandoff/a1688/command_test.go internal/listingkit/store/source_reference_persistence_test.go
git commit -m "test: define 1688 source lineage contract"
```

### Task 2: Implement the minimal source-reference propagation

**Files:**

- Modify: `internal/listingkit/model_request.go`
- Modify: `internal/listingkit/product_source_bridge.go`

**Interfaces:**

- Consumes: the red tests from Task 1 and existing `catalog.ProductFacts` values.
- Produces: `listingkit.SourceReference` and `GenerateRequest.Source`, with no changes to existing request fields or source-warning behavior.

- [ ] **Step 1: Add the optional request model**

In `internal/listingkit/model_request.go`, add:

```go
type SourceReference struct {
    Key      string `json:"key,omitempty"`
    Type     string `json:"type,omitempty"`
    Platform string `json:"platform,omitempty"`
    ID       string `json:"id,omitempty"`
    URL      string `json:"url,omitempty"`
}
```

Add this optional field to `GenerateRequest`:

```go
Source *SourceReference `json:"source,omitempty"`
```

Keep the field pointer-based so legacy JSON omits it and manually supplied requests do not gain fabricated identity.

- [ ] **Step 2: Copy identity in the neutral bridge**

In `internal/listingkit/product_source_bridge.go`, add:

```go
func sourceReferenceFromProductFacts(product catalog.ProductFacts) *SourceReference {
    if strings.TrimSpace(product.SourceKey) == "" &&
        strings.TrimSpace(product.SourceType) == "" &&
        strings.TrimSpace(product.SourcePlatform) == "" &&
        strings.TrimSpace(product.SourceID) == "" &&
        strings.TrimSpace(product.SourceURL) == "" {
        return nil
    }
    return &SourceReference{
        Key:      strings.TrimSpace(product.SourceKey),
        Type:     strings.TrimSpace(product.SourceType),
        Platform: strings.TrimSpace(product.SourcePlatform),
        ID:       strings.TrimSpace(product.SourceID),
        URL:      strings.TrimSpace(product.SourceURL),
    }
}
```

Set `Source: sourceReferenceFromProductFacts(product)` in the existing `GenerateRequestFromSourceFacts` result. Do not alter `ProductURL`, `Text`, `ImageURLs`, `Platforms`, category, or warning mappings.

- [ ] **Step 3: Run the focused green tests**

Run:

```powershell
gofmt -w internal/listingkit/model_request.go internal/listingkit/product_source_bridge.go internal/listingkit/product_source_bridge_test.go internal/product/sourcehandoff/a1688/command_test.go internal/listingkit/store/source_reference_persistence_test.go
go test ./internal/listingkit -run 'TestGenerateRequestFromSourceFacts|TestGenerateRequestFromSourceFactsKeepsEmptySourceReference' -count=1
go test ./internal/product/sourcehandoff/a1688 -run TestTaskCommandServiceCreateTaskDelegatesToListingKitCreator -count=1
go test ./internal/listingkit/store -run TestTaskRepositoryPersistsSourceReference -count=1
```

Expected: all source-reference tests pass, and existing request mapping assertions remain green.

- [ ] **Step 4: Commit the implementation**

```powershell
git add internal/listingkit/model_request.go internal/listingkit/product_source_bridge.go
git commit -m "feat: preserve source identity on listingkit requests"
```

### Task 3: Verify the existing 1688 HTTP and downstream paths

**Files:**

- Modify: `internal/productenrich/httpapi/sourcea1688/handler_test.go` only if an assertion is needed to cover the new task request shape.
- Test: `internal/listingkit/product_source_bridge_test.go`
- Test: `internal/product/sourcehandoff/a1688/command_test.go`
- Test: `internal/listingkit/store/source_reference_persistence_test.go`

**Interfaces:**

- Consumes: the implemented optional source reference and existing 1688 HTTP/command contracts.
- Produces: verification evidence that the HTTP route keeps authentication/store behavior and downstream SHEIN preview/readiness ownership unchanged.

- [ ] **Step 1: Run source and adapter regression tests**

```powershell
go test ./internal/product/sourcing/... ./internal/catalog/... ./internal/asset/... ./internal/product/sourcehandoff/... ./internal/productenrich/httpapi/sourcea1688/... -count=1
```

Expected: PASS.

- [ ] **Step 2: Run ListingKit and persistence regression tests**

```powershell
go test ./internal/listingkit/... ./internal/listingkit/store/... -count=1
```

Expected: PASS. Existing preview/readiness and task repository behavior must remain unchanged.

- [ ] **Step 3: Run the controlled 1688 HTTP adapter tests**

```powershell
go test ./internal/productenrich/httpapi/sourcea1688/... -run 'TestCreateListingKitTask|TestAppendRouteDescriptors' -count=1
```

Expected: PASS, including verified identity, forged body identity rejection, store access errors, normalized source identity, and source warnings.

- [ ] **Step 4: Run the full backend gate with a bounded timeout**

```powershell
go test ./... -count=1
```

Record whether it passes, fails with a concrete package/test, or times out. The prior baseline timed out after 124 seconds; a timeout must remain explicitly classified and must not be reported as a passing full suite.

- [ ] **Step 5: Review the final diff and commit status**

```powershell
git diff --check
git status --short --branch
git log --oneline -3
```

Expected: only the design commit and scoped implementation/test commits are present on `codex/1688-listingkit-source-loop`; no unrelated files are changed.

- [ ] **Step 6: Commit any required HTTP test-only adjustment**

If Task 3 Step 3 identifies a missing assertion that is necessary to prove the new behavior, stage only `internal/productenrich/httpapi/sourcea1688/handler_test.go` and commit:

```powershell
git add internal/productenrich/httpapi/sourcea1688/handler_test.go
git commit -m "test: verify 1688 source handoff HTTP contract"
```

If the existing HTTP tests already cover the required behavior, make no additional commit.

## Final verification checklist

- [ ] Source reference tests failed before production implementation.
- [ ] `GenerateRequest.Source` is populated only from neutral source facts.
- [ ] Legacy requests without source facts keep `Source == nil`.
- [ ] GORM task request JSON round-trips the source reference.
- [ ] 1688 command tests preserve `crawler:1688:<id>` and normalized URL.
- [ ] Existing source warnings and store/tenant checks remain unchanged.
- [ ] Focused source, catalog, asset, handoff, HTTP, ListingKit, and store tests pass.
- [ ] Full-suite result is recorded as pass, concrete failure, or timeout.
- [ ] `git diff --check` is clean and unrelated working-tree changes are absent.
