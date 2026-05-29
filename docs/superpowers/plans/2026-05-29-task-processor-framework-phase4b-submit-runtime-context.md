# Task Processor Framework Phase 4B ListingKit Submit Runtime Context Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the remaining submit/runtime-context complexity in ListingKit by making submit identity, store selection, store info loading, and submit-settings resolution flow through explicit feature-owned resolver seams instead of being spread across ad hoc service helpers.

**Architecture:** Reuse the same explicit-wiring pattern already established in `Phase 4A`. Do not invent a new generic context framework. Instead, extract a small ListingKit-owned submit context resolver that coordinates existing helpers around tenant identity, store profile selection, store catalog lookup, API client creation, and submit settings/warehouse resolution. Keep business behavior unchanged and preserve existing precedence rules.

**Tech Stack:** Go, existing ListingKit service helpers, in-memory store profile repositories, existing submit/store context tests, existing submit execution service tests, CodeGraph-informed seam analysis

**Out of Scope For This Slice:**

- redesigning workflow/process execution
- changing ListingKit HTTP/bootstrap ownership
- rewriting `taskSubmissionExecutionService` behavior
- changing `sheinlogin` or store profile persistence contracts
- inventing a repo-wide runtime context abstraction

---

## Root Cause This Slice Addresses

After `Phase 4A`, collaborator wiring ownership is healthier, but one operational hotspot remains spread across several helpers:

- [internal/listingkit/service_submit_runtime_context.go](/D:/code/task-processor/internal/listingkit/service_submit_runtime_context.go:1)
- [internal/listingkit/service_submit_store_context.go](/D:/code/task-processor/internal/listingkit/service_submit_store_context.go:1)
- [internal/listingkit/service_shein_categories.go](/D:/code/task-processor/internal/listingkit/service_shein_categories.go:1)
- [internal/listingkit/service_shein_store_client.go](/D:/code/task-processor/internal/listingkit/service_shein_store_client.go:1)

Today these helpers jointly decide:

1. how submit-time tenant identity is injected into context
2. how store selection/profile resolution is read from task snapshot vs current settings
3. how store info is loaded from the catalog
4. how API clients are created and refreshed
5. how submit settings are merged from defaults, profiles, request country, and live warehouses

The problem is not just file count. The real problem is that submit/runtime context ownership is still implicit and crosses four kinds of concern:

- identity shaping
- store selection
- remote store/catalog access
- submit settings hydration

That makes future changes risky because each behavior change can leak across multiple helper files without one clear seam to test or evolve.

---

## Target Outcome

At the end of `Phase 4B`:

- submit runtime context resolution flows through an explicit ListingKit-owned resolver seam
- identity shaping, store selection, store info lookup, API client creation, and submit settings hydration are grouped more coherently
- current behavior and precedence rules remain unchanged
- direct submit and submit execution paths consume the same resolver seam
- boundary tests lock the new ownership split

---

## Task 1: Isolate pure submit-settings hydration rules

**Files:**
- Create: `internal/listingkit/service_submit_settings_resolution.go`
- Modify: `internal/listingkit/service_submit_store_context.go`
- Modify: `internal/listingkit/service_submit_store_context_test.go`

- [ ] **Step 1: Write failing tests for submit-settings precedence**

Add focused tests that lock the current precedence model without depending on live API clients:

1. base settings come from current SHEIN settings
2. store profile fields override base defaults
3. request country overrides resolved site
4. warehouse code override remains last-mile and optional
5. task snapshot still wins over current profiles when present

Extend existing tests in `service_submit_store_context_test.go` rather than creating a parallel suite.

- [ ] **Step 2: Run focused store-context tests**

Run:

```powershell
go test ./internal/listingkit -run "Test(ResolveSheinSubmitSettingsUsesStoreProfileFields|ResolveSheinSubmitSettingsPrefersTaskSnapshotOverCurrentProfiles)" -count=1
```

Expected: PASS before the move, establishing the behavior baseline.

- [ ] **Step 3: Extract pure settings-hydration helpers**

Create `service_submit_settings_resolution.go` and move the merge-oriented logic out of `resolveSheinSubmitSettings(...)` into focused helpers such as:

- `applySubmitSettingsProfile(settings SheinSettings, profile *ListingKitStoreProfile) SheinSettings`
- `applySubmitSettingsTaskRequest(settings SheinSettings, task *Task) SheinSettings`
- `applySubmitWarehouseOverride(settings SheinSettings, warehouseCode string) SheinSettings`

Keep `resolveSheinSubmitSettings(...)` as the orchestration point for now.

Important:

- do not change precedence
- do not introduce remote calls into the pure helpers
- keep warehouse override optional and last-mile

- [ ] **Step 4: Re-run submit settings verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(ResolveSheinSubmitSettingsUsesStoreProfileFields|ResolveSheinSubmitSettingsPrefersTaskSnapshotOverCurrentProfiles|PickSheinWarehouseCode.*)" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/service_submit_settings_resolution.go internal/listingkit/service_submit_store_context.go internal/listingkit/service_submit_store_context_test.go
git commit -m "refactor: extract listingkit submit settings resolution"
```

---

## Task 2: Introduce an explicit submit runtime context resolver

**Files:**
- Create: `internal/listingkit/service_submit_context_resolver.go`
- Modify: `internal/listingkit/service_submit_runtime_context.go`
- Modify: `internal/listingkit/service_submit_store_context.go`
- Modify: `internal/listingkit/service_shein_store_client.go`
- Modify: `internal/listingkit/service_submit_wiring.go`
- Modify: `internal/listingkit/service_wiring_test.go`

- [ ] **Step 1: Write failing boundary tests for resolver ownership**

Add narrow source/behavior tests that prove:

1. submit/runtime context helpers are built through an explicit resolver seam
2. `service_submit_store_context.go` and `service_shein_store_client.go` stop being the primary home of cross-cutting resolver assembly

Lock at least one concrete indicator such as:

- `buildSubmitRuntimeContextResolver(s)` appears in the wiring layer
- `newSheinAPIClient(...)` uses the resolver instead of directly duplicating store-info lookup logic

- [ ] **Step 2: Run focused collaborator wiring tests**

Run:

```powershell
go test ./internal/listingkit -run "Test(SubmitCollaboratorFilesUseExplicitWiringBuilders|ServiceInitializeCollaboratorGroups)" -count=1
```

Expected: PASS before the refactor, giving a baseline around current wiring.

- [ ] **Step 3: Add a ListingKit-owned resolver seam**

Create `service_submit_context_resolver.go` with a compact resolver type and builder, for example:

- `type submitRuntimeContextResolver struct { ... }`
- `func buildSubmitRuntimeContextResolver(s *service) *submitRuntimeContextResolver`

This resolver should own coordinated access to:

- tenant identity shaping
- store selection/profile lookup
- store info lookup
- API client creation
- submit settings resolution
- warehouse lookup

Important:

- reuse the existing helpers internally first
- do not change repository or API client contracts in this step
- keep the resolver feature-owned inside ListingKit

- [ ] **Step 4: Rewire service helpers to use the resolver seam**

Update:

- `withSheinSubmitTaskIdentity(...)`
- `resolveSheinSubmitSettings(...)`
- `resolveSheinWarehouseCode(...)`
- `newSheinAPIClient(...)`

so the orchestration runs through the new resolver seam instead of through several loosely related service helpers.

- [ ] **Step 5: Re-run focused submit/runtime context verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(ResolveSheinSubmitSettings.*|ResolveSheinStoreID.*|StoreProfileService.*)" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/service_submit_context_resolver.go internal/listingkit/service_submit_runtime_context.go internal/listingkit/service_submit_store_context.go internal/listingkit/service_shein_store_client.go internal/listingkit/service_submit_wiring.go internal/listingkit/service_wiring_test.go
git commit -m "refactor: introduce listingkit submit runtime context resolver"
```

---

## Task 3: Align submit execution and direct submit on the new resolver seam

**Files:**
- Modify: `internal/listingkit/service_submit_direct.go`
- Modify: `internal/listingkit/task_submission_execution_service.go`
- Modify: `internal/listingkit/service_submit_wiring.go`
- Modify: `internal/listingkit/task_submission_execution_service_test.go`
- Modify: `internal/listingkit/service_submit_test.go`

- [ ] **Step 1: Write failing tests for shared submit context consumption**

Add tests that prove:

1. direct submit still applies the same resolved submit settings after image upload
2. submit execution still resolves store ID and runtime identity through the same context seam
3. no caller grows its own ad hoc submit-context logic

Prefer extending existing tests in:

- `task_submission_execution_service_test.go`
- `service_submit_test.go`

instead of adding a brand-new end-to-end harness.

- [ ] **Step 2: Run focused submit execution tests**

Run:

```powershell
go test ./internal/listingkit -run "Test(SubmitTask|TaskSubmissionExecution|SheinSubmit)" -count=1
```

Expected: PASS before the rewire.

- [ ] **Step 3: Rewire submit execution and direct submit to the shared resolver seam**

Update the relevant config builders and consumers so that:

- `taskSubmissionExecutionService` gets store-resolution and settings-resolution through the resolver-backed seam
- direct submit preparation uses the same resolved settings path as the execution service

Keep current behavior unchanged:

- image upload still happens before final pre-validation
- request identity still derives from task tenant/user
- store selection precedence still stays the same

- [ ] **Step 4: Re-run focused submit behavior verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(SubmitTask|TaskSubmissionExecution|ResolveSheinSubmitSettings)" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/service_submit_direct.go internal/listingkit/task_submission_execution_service.go internal/listingkit/service_submit_wiring.go internal/listingkit/task_submission_execution_service_test.go internal/listingkit/service_submit_test.go
git commit -m "refactor: align listingkit submit context consumers"
```

---

## Task 4: Lock the submit/runtime context ownership boundary

**Files:**
- Create: `internal/listingkit/phase4b_submit_context_boundary_test.go`
- Modify: `internal/listingkit/service_wiring_test.go`
- Modify: `internal/listingkit/service_submit_store_context_test.go`

- [ ] **Step 1: Add boundary guardrails**

Lock two things:

1. submit/runtime context ownership stays behind the explicit resolver seam
2. pure settings-hydration helpers remain separate from remote store/catalog operations

Suggested checks:

- `service_submit_store_context.go` should not regrow API client creation logic
- `service_shein_store_client.go` should not regrow settings-hydration logic
- resolver builder ownership stays in the wiring layer / dedicated resolver file

- [ ] **Step 2: Run full ListingKit verification**

Run:

```powershell
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/listingkit/phase4b_submit_context_boundary_test.go internal/listingkit/service_wiring_test.go internal/listingkit/service_submit_store_context_test.go
git commit -m "test: lock listingkit submit runtime context boundary"
```

---

## Self-Review

### Spec coverage

This plan intentionally covers one bounded hotspot:

- submit identity shaping
- store/profile selection
- store info / API client context
- submit settings hydration
- boundary tests

It does not mix in workflow/process modeling or bootstrap work.

### Reuse check

This plan reuses the same explicit builder/resolver pattern already proven in `Phase 4A`.

It does not invent a generic repo-wide context abstraction. The target seam is feature-owned and local to ListingKit.

### Root-cause check

The problem being addressed is crossed ownership between:

- identity
- store resolution
- remote catalog/client access
- settings hydration

The plan therefore focuses on:

- pure helper extraction where logic is deterministic
- one explicit resolver seam where orchestration is inherently stateful
- reusing that seam across submit consumers

### Scope discipline

This is a bounded slice:

- no workflow rewrite
- no service-root cleanup round two
- no speculative abstraction outside ListingKit

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-29-task-processor-framework-phase4b-submit-runtime-context.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
