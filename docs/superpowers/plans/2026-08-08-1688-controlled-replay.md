# 1688 Controlled Replay Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a deterministic cross-boundary replay that verifies an authenticated 1688 source request reaches the existing ListingKit task-creation boundary with source lineage, normalized facts, and explainable failures.

**Architecture:** Exercise the real `sourcea1688.Handler` and `a1688.TaskCommandService` in an in-process Gin request. A local fake implements `sourcehandoff.GenerateTaskCreator`, captures the real `listingkit.GenerateRequest`, and returns a deterministic task. The replay remains test-only; it does not invoke crawling, persistence, workers, preview generation, or SHEIN submission.

**Tech Stack:** Go 1.26+, Gin, `httptest`, existing ListingKit source-handoff contracts, Go test.

## Global Constraints

- Keep real crawling, store submission, database persistence, worker queues, preview generation, and SHEIN submission out of the replay.
- Preserve the existing `POST /api/v1/product-sourcing/1688/listingkit/tasks` route and the existing `a1688.TaskCommandService` boundary.
- Use authenticated tenant/user context; never treat JSON body or headers as trusted identity.
- Keep source lineage outside canonical product facts and marketplace-specific draft payloads.
- Do not report deterministic replay success as real operator or production acceptance.
- Do not modify production code unless a replay assertion exposes a real propagation or error-reporting defect.

---

### Task 1: Add the successful 1688 HTTP-to-task replay

**Files:**
- Create: `tests/a1688_source_to_task_flow_test.go`
- Read/align with: `internal/productenrich/httpapi/sourcea1688/handler.go`
- Read/align with: `internal/product/sourcehandoff/a1688/command.go`
- Read/align with: `internal/listingkit/model_request.go`

**Interfaces:**
- Consumes: `sourcea1688.NewHandler`, `a1688.NewTaskCommandService`, `listingkit.WithAuthenticatedIdentity`, and `sourcehandoff.GenerateTaskCreator`.
- Produces: a replay test proving the actual HTTP handler and actual 1688 command service preserve source facts into a captured `*listingkit.GenerateRequest`.

- [ ] **Step 1: Write the successful replay test**

Create a Gin test router with the real handler:

```go
creator := &replayGenerateTaskCreator{}
service := a1688.NewTaskCommandService(creator, replayStoreAccessValidator{})
router := gin.New()
router.POST("/api/v1/product-sourcing/1688/listingkit/tasks", sourcea1688.NewHandler(service).CreateListingKitTask)
```

Send a JSON `sourcea1688.CreateListingKitTaskRequest` containing a fixed `Product1688` with ID `321`, a URL query string, title, category, brand, two images, one variant, price, supplier, description, source store ID `3001`, SHEIN store ID `168811`, platform `shein`, source run ID, request ID, and raw snapshot. Attach:

```go
listingkit.WithAuthenticatedIdentity(req.Context(), listingkit.AuthenticatedIdentity{
    TenantID: "tenant-1688",
    UserID:   "user-1688",
})
```

Assert HTTP 200, task ID `task-replay-321`, tenant ID `tenant-1688`, source ID `321`, source key `crawler:1688:321`, normalized product URL, and no source warnings. Assert the captured request contains trimmed tenant/user values, one normalized `shein` platform, `SheinStoreID == 168811`, the normalized product URL, title/brand/category/description/price/variant text, all expected image URLs, and a source reference with key/platform/ID/URL matching the envelope.

- [ ] **Step 2: Run the focused test**

```powershell
$env:GOWORK='off'; go test ./tests -run TestAlibaba1688HTTPReplayCreatesTaskAndPreservesSourceFacts -count=1 -v
```

Expected: the new test either fails on an actual missing assertion or exposes a test setup error. If it passes immediately, record that the repository already satisfies the behavior and continue with the remaining negative-path coverage; do not add production code solely to force a failure.

- [ ] **Step 3: Add only test-local replay helpers**

Implement in the new test file:

- `replayGenerateTaskCreator` with captured `*listingkit.GenerateRequest`, call count, and `CreateGenerateTask(context.Context, *listingkit.GenerateRequest) (*listingkit.Task, error)` returning `task-replay-321` with the request tenant and `TaskStatusPending`.
- `replayStoreAccessValidator` implementing `ValidateStoreAccess(context.Context, int64, int64, string) (listingkit.StoreAccess, error)` and allowing source store `3001` on `1688` and target store `168811` on `SHEIN`.
- `replayProduct1688(id string) *alibaba1688model.Product1688` containing the complete deterministic fixture.
- An authenticated `httptest` request helper using JSON encoding and `listingkit.WithAuthenticatedIdentity`.

Use existing production response types and normalization. Do not copy production normalization logic into helpers.

- [ ] **Step 4: Run the successful replay and focused suite**

```powershell
$env:GOWORK='off'; go test ./tests -run TestAlibaba1688HTTPReplayCreatesTaskAndPreservesSourceFacts -count=1 -v
$env:GOWORK='off'; go test ./internal/product/sourcing/... ./internal/product/sourcehandoff/... ./internal/productenrich/httpapi/sourcea1688/... ./tests/... -count=1
```

Expected: the new test and all existing focused packages pass.

- [ ] **Step 5: Commit**

```powershell
git add tests/a1688_source_to_task_flow_test.go
git commit -m "test: replay 1688 source into listingkit task"
```

### Task 2: Add missing-facts and source-error replay coverage

**Files:**
- Modify: `tests/a1688_source_to_task_flow_test.go`
- Read/align with: `internal/product/sourcehandoff/a1688/listingkit_task.go`
- Read/align with: `internal/product/sourcing/a1688_source_envelope.go`

**Interfaces:**
- Consumes: the Task 1 router, real command service, and test-local fake creator.
- Produces: explicit evidence that incomplete or failed 1688 source input is visible and cannot create a task.

- [ ] **Step 1: Add the missing-facts test**

Post a product with ID `322`, a valid 1688 URL, empty title, and no usable images. Use valid authenticated identity and store IDs. Assert HTTP 400, `error == "task_creation_failed"`, an error message containing `1688 source cannot create listingkit task`, a response source identity for ID `322`, at least one source warning, and creator call count `0`.

- [ ] **Step 2: Run the missing-facts test**

```powershell
$env:GOWORK='off'; go test ./tests -run TestAlibaba1688HTTPReplayRejectsMissingFacts -count=1 -v
```

Expected: the test passes against the existing validation boundary. If the response omits source identity or warnings, fix the smallest production projection defect and add a focused regression assertion before continuing.

- [ ] **Step 3: Add the source-error test**

Post a valid source identity for ID `323` with `SourceError: "controlled crawler failed"`. Assert HTTP 400, `error == "task_creation_failed"`, source ID `323`, a source warning containing the controlled error, and creator call count `0`.

- [ ] **Step 4: Run both negative-path tests and the package suite**

```powershell
$env:GOWORK='off'; go test ./tests -run 'TestAlibaba1688HTTPReplay(RejectsMissingFacts|PreservesSourceError)' -count=1 -v
$env:GOWORK='off'; go test ./internal/product/sourcing/... ./internal/product/sourcehandoff/... ./internal/productenrich/httpapi/sourcea1688/... ./tests/... -count=1
```

Expected: all replay scenarios and the focused Product Sourcing suite pass.

- [ ] **Step 5: Commit**

```powershell
git add tests/a1688_source_to_task_flow_test.go
git commit -m "test: cover 1688 replay failure evidence"
```

### Task 3: Record deterministic replay evidence and full-gate status

**Files:**
- Create: `docs/product/validation/2026-08-08-1688-controlled-replay.md`
- Modify: none unless command output requires a correction to the report

**Interfaces:**
- Consumes: the exact HEAD commit, focused replay test output, full backend test output, and build command output.
- Produces: a dated validation note that separates deterministic replay evidence from unverified runtime acceptance.

- [ ] **Step 1: Run focused validation**

```powershell
$env:GOWORK='off'; go test ./internal/product/sourcing/... ./internal/catalog/... ./internal/asset/... ./internal/product/sourcehandoff/... ./internal/productenrich/httpapi/sourcea1688/... -count=1
$env:GOWORK='off'; go test ./internal/listingkit/... ./tests/... -count=1
```

Record the exact commit SHA and results. Include the three replay scenarios and deterministic task IDs `task-replay-321`, `322` (rejected), and `323` (rejected).

- [ ] **Step 2: Run the full backend gate**

```powershell
$env:GOWORK='off'; go test ./... -count=1
```

If the command times out or fails due to the environment, record the exact failure and classify it as unresolved. Do not call it green.

- [ ] **Step 3: Run the maintained build equivalent**

```powershell
$env:CGO_ENABLED='0'; $env:GOOS='linux'; go build ./cmd/listing-control-plane ./cmd/product-listing-api ./cmd/shein-listing ./cmd/temu-listing
```

Record success or the exact failure. If GNU Make is unavailable, explicitly state that the Makefile-equivalent command was used.

- [ ] **Step 4: Write and self-review the validation note**

The note must contain the baseline commit SHA, fixture IDs and normalized source keys, tenant/user and target platform context, generated request and source-lineage assertions, successful and rejected replay outcomes, focused test commands/results, full backend/build results, and an explicit statement that real preview/readiness, real task IDs, operator acceptance, live crawler execution, and SHEIN submission remain unverified.

Run:

```powershell
rg -n "TBD|TODO|pending|unverified|not verified|not run" docs/product/validation/2026-08-08-1688-controlled-replay.md
git diff --check
```

- [ ] **Step 5: Commit**

```powershell
git add docs/product/validation/2026-08-08-1688-controlled-replay.md
git commit -m "docs: record 1688 controlled replay validation"
```

### Task 4: Final verification and handoff

**Files:**
- Verify: `tests/a1688_source_to_task_flow_test.go`
- Verify: `docs/product/validation/2026-08-08-1688-controlled-replay.md`

**Interfaces:**
- Consumes: all prior commits and current branch state.
- Produces: a clean branch with evidence-backed status and no accidental unrelated files.

- [ ] **Step 1: Run the final replay suite**

```powershell
$env:GOWORK='off'; go test ./tests -run 'TestAlibaba1688HTTPReplay' -count=1 -v
```

- [ ] **Step 2: Check diff and branch state**

```powershell
git diff --check
git diff origin/master...HEAD --stat
git status -sb
```

Expected: no whitespace errors, only replay test, validation note, design, and plan files are changed, and the working tree is clean.

- [ ] **Step 3: Report evidence boundaries**

Report separately: focused replay result, full backend result, build result, and still-unverified real preview/readiness/operator gates. Do not claim the Product Sourcing MVP is production-closed unless a real controlled runtime run supplies those missing records.
