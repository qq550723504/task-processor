# Task Processor Framework Phase 21 ListingKit Generation Action Target-Resolution Ownership Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the next residual ownership hotspot in ListingKit by moving the broad `generation action target-resolution / clone / action-key` helper cluster out of `service_generation_actions.go` and into an explicit feature-local seam.

**Architecture:** Keep `Phase 19` and `Phase 20` temporal seams intact. Do **not** reopen temporal request parsing, temporal result shaping, or service-entry orchestration. Instead, isolate the remaining `action key -> target lookup -> target clone/defaulting` flow currently embedded in `resolveAssetGenerationActionTarget(...)` and its neighboring helpers into a ListingKit-local target-resolution seam, route the existing action-execution path through it, and then lock the boundary with focused tests. This is a local ownership move, not a generic action helper framework.

**Tech Stack:** Go, ListingKit task generation action flow, action target helpers, feature-local seams, source-boundary guardrails

**Out of Scope For This Slice:**

- reopening `Phase 19` / `Phase 20` temporal seams
- broad refactors across all of `service_generation_actions.go`
- redesigning `ExecuteTaskGenerationAction(...)`
- changing outward target-resolution behavior or error messages
- inventing a repo-wide generic action helper / cloning framework
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 20`, temporal request parsing is no longer the main helper hotspot. The next residual pressure in:

- [internal/listingkit/service_generation_actions.go:17](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:17)

is the wider helper cluster around:

1. action-key selection
2. overview vs request-target resolution
3. target clone semantics
4. interaction-mode defaulting

Today those concerns still live together inside:

- [resolveAssetGenerationActionTarget(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:17)
- [collectAssetGenerationActionTargets(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:45)
- [cloneAssetGenerationActionTarget(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:57)
- [requestedAssetGenerationActionKey(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:99)

The ownership problem is no longer “where does temporal parsing live.” The next problem is “why does action target lookup / clone/default semantics still live as a broad helper cluster beside unrelated action helpers.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1)
  - Today still owns the broad target-resolution helper cluster
  - Should end the phase without owning the full target-resolution implementation

- [internal/listingkit/task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:1)
  - Existing action execution seam that consumes target resolution
  - Should end the phase depending on a feature-local target-resolution home rather than the broad helper cluster

- [internal/listingkit/service_generation_actions_test.go](/D:/code/task-processor/internal/listingkit/service_generation_actions_test.go:1)
  - Already hosts generation-action behavior tests
  - Good place for focused target-resolution behavior coverage

- [internal/listingkit/phase10_task_generation_action_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase10_task_generation_action_boundary_test.go:1)
  - Already protects broader action phase ownership
  - May need a small alignment so the execute seam points at the new local resolution home instead of the old broad helper implementation

### New files this phase should introduce

- `internal/listingkit/task_generation_action_target_resolution.go`
  - Feature-local home for action target lookup / clone/default semantics
  - Should own the implementation currently embedded in `resolveAssetGenerationActionTarget(...)` and neighboring helpers

- `internal/listingkit/phase21_action_target_resolution_boundary_test.go`
  - Guardrail ensuring target-resolution logic lives in its local home, not back in the broad helper file

This keeps the slice narrow: one target-resolution seam, one consumer update, one guardrail.

---

## Target Outcome

At the end of `Phase 21`:

- action target-resolution logic has an explicit ListingKit-local home
- the action execution seam consumes that local resolution seam
- `service_generation_actions.go` no longer owns the full target-resolution implementation
- current action-key resolution, overview/request precedence, clone semantics, and interaction-mode defaulting remain unchanged
- ownership guardrails lock the new target-resolution seam so it does not drift back into the broader helper file

---

## Task 1: Lock current action target-resolution behavior

**Files:**
- Modify: `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Add failing behavior tests for target-resolution precedence and defaults**

Add focused tests that lock the current behavior of `resolveAssetGenerationActionTarget(...)` before moving it:

1. request `ActionKey` wins over blank target action key
2. target action key is used when top-level action key is blank
3. invalid or missing action key still returns the current error surface
4. overview target resolution wins before request target when keys match
5. request target is cloned and gets default `InteractionMode` when missing
6. returned targets are defensive clones, not original pointers

Suggested test names:

```go
func TestResolveAssetGenerationActionTargetPrefersRequestedActionKey(t *testing.T)
func TestResolveAssetGenerationActionTargetResolvesOverviewBeforeRequestTarget(t *testing.T)
func TestResolveAssetGenerationActionTargetClonesAndDefaultsRequestTarget(t *testing.T)
func TestResolveAssetGenerationActionTargetReturnsCurrentErrors(t *testing.T)
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestResolveAssetGenerationActionTarget.*" -count=1
```

Expected: FAIL until the tests are aligned to the current behavior surface.

- [ ] **Step 3: Align tests until they describe current behavior exactly**

Do not change production behavior yet. If a test fails because precedence or clone/default semantics are different than expected, fix the test to match the implementation.

- [ ] **Step 4: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestResolveAssetGenerationActionTarget.*" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/service_generation_actions_test.go
git commit -m "test: lock listingkit action target resolution behavior"
```

---

## Task 2: Extract a feature-local action target-resolution seam

**Files:**
- Create: `internal/listingkit/task_generation_action_target_resolution.go`
- Modify: `internal/listingkit/service_generation_actions.go`
- Modify if needed: `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Introduce the new local resolution seam behind the current behavior**

Create a feature-local helper/seam that becomes the new home of action target resolution.

Suggested shape:

```go
type taskGenerationActionTargetResolutionPhase struct{}

func buildTaskGenerationActionTargetResolutionPhase() *taskGenerationActionTargetResolutionPhase

func (p *taskGenerationActionTargetResolutionPhase) run(
    overview *AssetGenerationOverview,
    req *ExecuteGenerationActionRequest,
) (*AssetGenerationActionTarget, string, error)
```

This seam should own:

- action-key selection / fallback
- overview candidate traversal
- request-target clone + `InteractionMode` defaulting
- returned resolution source (`overview` / `request_target`)

- [ ] **Step 2: Move the target-resolution implementation into the new local file**

Refactor so the implementation lives in:

- `internal/listingkit/task_generation_action_target_resolution.go`

and the broad helper file no longer owns the full implementation.

Important:

- preserve the exact behavior locked in Task 1
- do not redesign the request shapes
- do not mix temporal parsing concerns into this new file
- keep this seam feature-local and resolution-specific

- [ ] **Step 3: Decide whether old helpers become thin aliases or are removed**

Choose the smaller change that preserves clarity:

- If external call sites still meaningfully benefit from `resolveAssetGenerationActionTarget(...)`, keep it as a thin wrapper
- If all meaningful consumers are action-local, remove the old helper and update callers directly

Whichever option you choose, the ownership goal must still hold: the full resolution implementation itself must live in the new local file, not in the broad helper file.

- [ ] **Step 4: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestResolveAssetGenerationActionTarget.*" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/task_generation_action_target_resolution.go internal/listingkit/service_generation_actions.go internal/listingkit/service_generation_actions_test.go
git commit -m "refactor: extract listingkit action target resolution seam"
```

---

## Task 3: Route the action execution seam through the new local resolution home

**Files:**
- Modify: `internal/listingkit/task_generation_action_execute.go`
- Modify if needed:
  - `internal/listingkit/service_generation_actions.go`
  - `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Add a failing seam-consumer test**

Add focused coverage that locks:

1. action execution still receives the same resolved target behavior
2. the execution seam now depends on the new local resolution seam/home rather than the broad helper implementation
3. clone/default behavior remains unchanged at the execution boundary
4. resolution source semantics remain unchanged

Suggested test names:

```go
func TestTaskGenerationActionExecutePhaseUsesLocalTargetResolutionSeam(t *testing.T)
func TestTaskGenerationActionExecutePhasePreservesResolvedTargetHandoff(t *testing.T)
```

- [ ] **Step 2: Route the execute seam through the new local resolution home**

Update:

- [internal/listingkit/task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:1)

so that the execution seam no longer depends on the old broad target-resolution implementation as its primary ownership home.

Important:

- keep the same execution branching / bootstrap / review-session behavior
- do not widen this into broader action helper cleanup

- [ ] **Step 3: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestResolveAssetGenerationActionTarget.*|TestTaskGenerationActionExecutePhase.*|TestExecuteTaskGenerationAction.*" -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/listingkit/task_generation_action_execute.go internal/listingkit/task_generation_action_target_resolution.go internal/listingkit/service_generation_actions.go internal/listingkit/service_generation_actions_test.go
git commit -m "refactor: route listingkit action execution through local target resolution"
```

---

## Task 4: Lock action target-resolution ownership guardrails

**Files:**
- Create: `internal/listingkit/phase21_action_target_resolution_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase10_task_generation_action_boundary_test.go`
  - `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. action target-resolution implementation lives in the new local file
2. the execution seam consumes that local resolution home
3. the broad helper file no longer owns the full resolution implementation
4. `Phase 10` action phase responsibilities remain unchanged

The guardrails should specifically catch regressions where:

- target-resolution logic drifts back into `service_generation_actions.go`
- execution re-inlines target resolution locally
- unrelated temporal parsing helpers get mixed into this target-resolution seam

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestResolveAssetGenerationActionTarget.*|TestTaskGenerationAction.*Boundary|TestTaskGenerationActionPhaseOwnershipBoundary" -count=1
```

Expected: FAIL until the new guardrails match the final ownership split.

- [ ] **Step 3: Align the boundary suite with the new resolution home**

Update the boundary tests so they point at the new local ownership home and keep `Phase 10`’s action execution seam expectations intact.

Important:

- prefer the existing AST/token/source helper style already used in `phase18`, `phase19`, and `phase20`
- avoid overly formatting-sensitive assertions when a lower-fragility equivalent exists
- do not broaden the suite into generic helper ownership beyond this slice

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestResolveAssetGenerationActionTarget.*|TestTaskGenerationAction.*Boundary|TestTaskGenerationActionPhaseOwnershipBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- target-resolution behavior tests PASS
- action seam boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase21_action_target_resolution_boundary_test.go internal/listingkit/phase10_task_generation_action_boundary_test.go internal/listingkit/service_generation_actions_test.go internal/listingkit/task_generation_action_target_resolution.go internal/listingkit/task_generation_action_execute.go internal/listingkit/service_generation_actions.go
git commit -m "test: lock listingkit action target resolution boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 21` stays inside action target-resolution helper ownership
2. the plan does not reopen `Phase 19` / `Phase 20` temporal seams
3. the plan does not broaden into a full `service_generation_actions.go` rewrite
4. the plan does not expand into a generic action helper framework
5. each task has a bounded write set and explicit verification command

This plan passes that check.

## Expected Verification Matrix

During execution, the implementing worker should expect to run at least these checks:

```powershell
go test ./internal/listingkit -run "TestResolveAssetGenerationActionTarget.*" -count=1
go test ./internal/listingkit -run "TestResolveAssetGenerationActionTarget.*|TestTaskGenerationActionExecutePhase.*|TestExecuteTaskGenerationAction.*" -count=1
go test ./internal/listingkit -run "TestResolveAssetGenerationActionTarget.*|TestTaskGenerationAction.*Boundary|TestTaskGenerationActionPhaseOwnershipBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
