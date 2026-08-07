# Task Detail Source Lineage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose persisted `source_reference` data through the task-detail API and status page so source lineage survives refreshes and historical-task navigation.

**Architecture:** Reuse the existing `Task.Request.Source` persistence boundary and add a read-only top-level field to `TaskResult`. A shared defensive-copy helper keeps task-list and task-detail projections consistent. The UI uses a dedicated persisted-source component and falls back to the existing local-draft summary only when no persisted source exists.

**Tech Stack:** Go, GORM-backed ListingKit repository, Gin API, Next.js/React, TypeScript, Zod, Vitest, Testing Library.

## Global Constraints

- Do not add a database table, migration, endpoint, filter, or source-provider integration.
- Do not accept or trust `source_reference` from public task-creation requests; only project server-persisted `Task.Request.Source`.
- Keep `source_reference` optional so legacy tasks omit the field and remain compatible.
- Keep retry/recreate behavior out of this change.
- External source links open in a new tab with `rel="noreferrer"`; do not nest them inside an existing navigation link.
- Follow TDD: each production behavior starts with a failing test and is re-run after the minimal implementation.

---

### Task 1: Add a shared source-reference read projection

**Files:**
- Create: `internal/listingkit/source_reference_projection.go`
- Modify: `internal/listingkit/model_task.go:78-90`
- Modify: `internal/listingkit/task_list_item_support.go:124-149`
- Modify: `internal/listingkit/task_result_projection.go:9-27`
- Test: `internal/listingkit/task_result_source_reference_test.go`

**Interfaces:**
- Consumes: `Task.Request.Source *SourceReference`.
- Produces: `cloneSourceReference(*SourceReference) *SourceReference` and `TaskResult.SourceReference *SourceReference`.

- [ ] **Step 1: Write the failing task-result projection tests**

Create `internal/listingkit/task_result_source_reference_test.go` with:

```go
func TestBuildTaskResultIncludesPersistedSourceReference(t *testing.T) {
    task := &Task{ID: "task-detail-source", TenantID: "tenant-a", Request: &GenerateRequest{
        Source: &SourceReference{
            Key: "crawler:1688:888", Type: "crawler", Platform: "1688", ID: "888",
            URL: "https://detail.1688.com/offer/888.html",
        },
    }}
    result := buildTaskResult(task, nil)
    if result.SourceReference == nil || result.SourceReference.ID != "888" {
        t.Fatalf("source_reference = %+v, want persisted source identity", result.SourceReference)
    }
    if result.SourceReference == task.Request.Source {
        t.Fatal("source_reference shares request pointer, want defensive copy")
    }
}

func TestBuildTaskResultOmitsLegacySourceReference(t *testing.T) {
    result := buildTaskResult(&Task{ID: "legacy-task"}, nil)
    if result.SourceReference != nil {
        t.Fatalf("source_reference = %+v, want nil for legacy task", result.SourceReference)
    }
}
```

- [ ] **Step 2: Run the tests and verify the expected red failure**

Run:

```powershell
go test ./internal/listingkit -run 'TestBuildTaskResult( IncludesPersistedSourceReference|OmitsLegacySourceReference)' -count=1
```

Expected: compilation fails because `TaskResult.SourceReference` does not yet exist.

- [ ] **Step 3: Implement the minimal shared projection and model field**

Create a nil-safe value-copy helper:

```go
func cloneSourceReference(source *SourceReference) *SourceReference {
    if source == nil {
        return nil
    }
    copy := *source
    return &copy
}
```

Add `SourceReference *SourceReference `json:"source_reference,omitempty"`` to `TaskResult`, set it in `buildTaskResultProjection` from `task.Request.Source`, and replace the task-list projection's inline field copy with `cloneSourceReference`. Keep the existing `source.Type` fallback for `TaskListItem.SourceType`.

- [ ] **Step 4: Run focused projection tests**

```powershell
go test ./internal/listingkit -run 'TestBuildTaskResult( IncludesPersistedSourceReference|OmitsLegacySourceReference)|TestBuildTaskListItem|TestTaskList' -count=1
```

Expected: all focused tests pass and legacy task JSON omits the optional field.

- [ ] **Step 5: Commit the backend projection**

```powershell
git add internal/listingkit/source_reference_projection.go internal/listingkit/model_task.go internal/listingkit/task_list_item_support.go internal/listingkit/task_result_projection.go internal/listingkit/task_result_source_reference_test.go
git commit -m "feat: expose source lineage in task details"
```

### Task 2: Lock the task-detail API contract

**Files:**
- Create: `internal/listingkit/task_result_api_projection_test.go`

**Interfaces:**
- Consumes: `taskLifecycleService.GetTaskResult` and the existing `TaskResult` JSON tags.
- Produces: proof that the task-detail response returns only the read-only `source_reference` projection and never the full persisted request.

- [ ] **Step 1: Add the JSON contract regression test**

Create `internal/listingkit/task_result_api_projection_test.go`. Use a task with `Request.Source`, marshal `buildTaskResult(task, nil)`, and assert that the payload contains the five normalized source fields under `source_reference` and does not contain a top-level `request` field.

- [ ] **Step 2: Run the contract regression**

```powershell
go test ./internal/listingkit -run TestTaskResultJSONContainsSourceReferenceWithoutRequest -count=1
```

Expected: PASS after Task 1, proving the read projection exposes source identity without serializing the persisted request.

- [ ] **Step 3: Verify the existing anti-forgery regression**

```powershell
go test ./internal/listingkit/api -run TestGenerateListingKitIgnoresSourceReferenceFromPublicRequest -count=1
```

Expected: PASS, confirming the detail read projection did not make client-supplied source data writable.

- [ ] **Step 4: Commit the API contract test**

```powershell
git add internal/listingkit/task_result_api_projection_test.go
git commit -m "test: lock task detail source lineage contract"
```

### Task 3: Extend the frontend task-result contract

**Files:**
- Modify: `web/listingkit-ui/src/lib/types/listingkit/tasks.ts:137-155`
- Modify: `web/listingkit-ui/src/lib/api/listingkit-response-schema.ts:47-92`
- Test: `web/listingkit-ui/src/lib/api/listingkit-response-schema.test.ts`

**Interfaces:**
- Consumes: backend `TaskResult.source_reference`.
- Produces: parsed `ListingKitTaskResult.source_reference?: ListingKitSourceReference` for the status page.

- [ ] **Step 1: Add the failing schema test**

Add a `parseTaskResultResponse` case with the normalized source reference and assert the parsed result preserves `key`, `type`, `platform`, `id`, and `url`. Add a legacy payload assertion that `source_reference` remains undefined.

- [ ] **Step 2: Run the schema test and verify the expected red failure**

From `web/listingkit-ui` run:

```powershell
npm.cmd test -- --run src/lib/api/listingkit-response-schema.test.ts
```

Expected: the new assertion fails because the task-result type/schema does not yet declare the field.

- [ ] **Step 3: Implement the type and Zod field**

Reuse the existing `ListingKitSourceReference` type. Add an optional `.object({...}).passthrough()` field to `taskResultSchema` and add `source_reference?: ListingKitSourceReference` to `ListingKitTaskResult`.

- [ ] **Step 4: Run the schema test and typecheck**

```powershell
npm.cmd test -- --run src/lib/api/listingkit-response-schema.test.ts
npm.cmd run typecheck
```

Expected: both commands pass.

- [ ] **Step 5: Commit the frontend contract**

```powershell
git add web/listingkit-ui/src/lib/types/listingkit/tasks.ts web/listingkit-ui/src/lib/api/listingkit-response-schema.ts web/listingkit-ui/src/lib/api/listingkit-response-schema.test.ts
git commit -m "feat: parse source lineage in task details"
```

### Task 4: Render persisted lineage on the task status page

**Files:**
- Create: `web/listingkit-ui/src/components/listingkit/tasks/task-persisted-source-reference.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/tasks/task-persisted-source-reference.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/tasks/task-status-screen.tsx:268`
- Test: `web/listingkit-ui/src/components/listingkit/tasks/task-status-screen.test.tsx`

**Interfaces:**
- Consumes: `ListingKitSourceReference` from `ListingKitTaskResult.source_reference`.
- Produces: a plain card with source summary and an optional safe external link.

- [ ] **Step 1: Write the failing component test**

Render a reference with `platform: "1688"`, `id: "888"`, and a URL, then assert:

```tsx
expect(screen.getByText("来源 1688 · 888")).toBeInTheDocument();
expect(screen.getByRole("link", { name: "查看来源" })).toHaveAttribute(
    "href",
    "https://detail.1688.com/offer/888.html",
);
expect(screen.getByRole("link", { name: "查看来源" })).toHaveAttribute("target", "_blank");
expect(screen.getByRole("link", { name: "查看来源" })).toHaveAttribute("rel", "noreferrer");
```

Add an empty-reference case that renders nothing.

- [ ] **Step 2: Run the component test and verify the expected red failure**

```powershell
npm.cmd test -- --run src/components/listingkit/tasks/task-persisted-source-reference.test.tsx
```

Expected: the test fails because the component does not exist.

- [ ] **Step 3: Implement the dedicated persisted-source component**

Render a `Card` with heading `任务来源`, a summary assembled from non-empty `platform` and `id` values, and `查看来源` only when `url` is non-empty. Do not wrap the card in a `Link`.

- [ ] **Step 4: Integrate persisted-first fallback behavior**

In `TaskStatusScreen`, render the persisted-source component when `task.source_reference` contains at least one identity field. Render the existing `TaskSourceSummary` only when `task.source_reference` is absent, preserving local-draft behavior for legacy tasks.

Add status-screen tests for persisted source without local draft, legacy/local-draft fallback, and no invalid nested link.

- [ ] **Step 5: Run focused frontend tests**

```powershell
npm.cmd test -- --run src/components/listingkit/tasks/task-persisted-source-reference.test.tsx src/components/listingkit/tasks/task-status-screen.test.tsx
```

Expected: all new and existing status-screen tests pass.

- [ ] **Step 6: Commit the status-page UI**

```powershell
git add web/listingkit-ui/src/components/listingkit/tasks/task-persisted-source-reference.tsx web/listingkit-ui/src/components/listingkit/tasks/task-persisted-source-reference.test.tsx web/listingkit-ui/src/components/listingkit/tasks/task-status-screen.tsx web/listingkit-ui/src/components/listingkit/tasks/task-status-screen.test.tsx
git commit -m "feat: show persisted source on task status"
```

### Task 5: Regression verification and handoff

**Files:**
- Modify: none unless a test exposes an implementation defect.

- [ ] **Step 1: Run focused backend and frontend suites**

```powershell
go test ./internal/listingkit/... ./internal/product/sourcehandoff/a1688/... ./internal/productenrich/httpapi/sourcea1688/... -count=1
Set-Location web/listingkit-ui
npm.cmd test -- --run src/lib/api/listingkit-response-schema.test.ts src/components/listingkit/tasks/task-persisted-source-reference.test.tsx src/components/listingkit/tasks/task-status-screen.test.tsx
npm.cmd run typecheck
npm.cmd run lint -- src/lib/api/listingkit-response-schema.ts src/components/listingkit/tasks/task-persisted-source-reference.tsx src/components/listingkit/tasks/task-status-screen.tsx
Set-Location ../..
```

Expected: all focused tests pass, typecheck exits 0, and lint reports no errors.

- [ ] **Step 2: Run full repository checks**

```powershell
go test ./... -count=1
Set-Location web/listingkit-ui
npm.cmd test -- --maxWorkers=4
npm.cmd run typecheck
npm.cmd run lint
Set-Location ../..
```

Expected: Go and Vitest suites pass; lint has 0 errors. Existing unrelated lint warnings may remain and must be reported rather than folded into this feature.

- [ ] **Step 3: Inspect the final diff and worktree**

```powershell
git diff --check
git status -sb
git log --oneline -8
```

Expected: only the planned commits are present and the worktree is clean.

- [ ] **Step 4: Commit or publish only after verification**

Do not merge or push automatically. After checks pass, use the finishing workflow to present the three integration options for `codex/task-detail-source-lineage`.
