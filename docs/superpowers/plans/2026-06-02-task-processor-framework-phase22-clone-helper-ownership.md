# Task Processor Framework Phase 22 ListingKit Action Target Clone Helper Ownership Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the next residual ownership hotspot in ListingKit by clarifying which clone/copy semantics belong to the broad helper file and which belong closer to action-target-local seams.

**Architecture:** Keep `Phase 19`, `Phase 20`, and `Phase 21` seams intact. Do **not** reopen temporal parsing, target-resolution behavior, or service-entry orchestration. Instead, isolate the remaining `cloneAssetGenerationActionTarget(...)` helper cluster into a clearer feature-local ownership split, route existing local consumers through it where needed, and then lock the boundary with focused tests. This is a local ownership move, not a generic clone framework.

**Tech Stack:** Go, ListingKit action helpers, clone/copy semantics, feature-local seams, source-boundary guardrails

**Out of Scope For This Slice:**

- reopening `Phase 19` / `Phase 20` / `Phase 21` behavior seams
- broad refactors across all helper files
- changing outward clone / copy semantics
- redesigning target-resolution behavior
- inventing a repo-wide generic clone framework
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 21`, target-resolution behavior is no longer the main helper hotspot. The next residual pressure in:

- [internal/listingkit/service_generation_actions.go:13](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)

is the remaining clone helper cluster around:

1. action target cloning
2. nested impact cloning
3. queue query cloning
4. retry request cloning

Today those concerns still live together inside:

- [cloneAssetGenerationActionTarget(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)
- [cloneAssetGenerationActionImpact(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:25)
- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:36)
- [cloneRetryGenerationTasksRequest(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:44)

The ownership problem is no longer “where does target resolution live.” The next problem is “why does action-target-specific clone/copy semantics still live as a broad shared helper cluster.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1)
  - Today still owns the broad clone helper cluster
  - Should end the phase with clearer separation between shared and feature-local clone ownership

- [internal/listingkit/task_generation_action_target_resolution.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_resolution.go:1)
  - Existing local target-resolution home
  - Good candidate to absorb action-target-specific clone semantics if needed

- [internal/listingkit/service_generation_navigation_dispatch_helpers.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch_helpers.go:1)
  - Existing consumer of target clone helpers
  - Important reference for current shared usage surface

- [internal/listingkit/service_generation_actions_test.go](/D:/code/task-processor/internal/listingkit/service_generation_actions_test.go:1)
  - Already hosts action helper behavior tests
  - Good place for focused clone semantics coverage

### New files this phase may introduce

- `internal/listingkit/phase22_action_target_clone_boundary_test.go`
  - Guardrail ensuring clone helper ownership is clarified without changing outward copy semantics

If a new production file is needed, keep it narrowly scoped and feature-local. Do not create a generic repo-wide clone utility layer.

---

## Target Outcome

At the end of `Phase 22`:

- action-target clone ownership is clearer
- shared clone helpers that truly need to stay shared remain shared
- action-target-local clone semantics have an explicit local home if needed
- current nested clone/copy behavior remains unchanged
- ownership guardrails lock the clarified clone-helper split

---

## Task 1: Lock current clone helper behavior

**Files:**
- Modify: `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Add failing behavior tests for clone semantics**

Add focused tests that lock the current behavior of:

- `cloneAssetGenerationActionTarget(...)`
- `cloneAssetGenerationActionImpact(...)`
- `cloneGenerationQueueQuery(...)`
- `cloneRetryGenerationTasksRequest(...)`

Behavior to lock:

1. nil input returns nil
2. returned value is a distinct pointer
3. nested slices / nested action targets are defensively cloned
4. modifying the returned clone does not mutate the original
5. clone semantics remain field-for-field identical today

- [ ] **Step 2: Run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestCloneGeneration.*" -count=1
```

Expected: PASS after tests are aligned to current behavior.

- [ ] **Step 3: Commit**

```bash
git add internal/listingkit/service_generation_actions_test.go
git commit -m "test: lock listingkit action target clone behavior"
```

---

## Task 2: Clarify clone-helper ownership split

**Files:**
- Modify: `internal/listingkit/service_generation_actions.go`
- Modify if needed: `internal/listingkit/task_generation_action_target_resolution.go`
- Modify if needed: `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Decide the minimal ownership move**

Before editing, determine which parts are:

- genuinely shared clone helpers that should remain in the broad helper home
- action-target-local clone semantics that should move closer to target-resolution ownership

The move should be minimal and behavior-preserving.

- [ ] **Step 2: Implement the clarified split**

Refactor so that:

- target-local clone semantics no longer hide inside the broad helper cluster if they are only local now
- truly shared clone helpers remain available to current multi-consumer paths

Important:

- preserve the exact clone semantics locked in Task 1
- do not redesign target-resolution behavior
- do not widen into unrelated helper cleanup

- [ ] **Step 3: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestCloneGeneration.*|TestResolveAssetGenerationActionTarget.*" -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/listingkit/service_generation_actions.go internal/listingkit/task_generation_action_target_resolution.go internal/listingkit/service_generation_actions_test.go
git commit -m "refactor: clarify listingkit action target clone ownership"
```

---

## Task 3: Align local consumers with the clarified clone home

**Files:**
- Modify consumers only if needed, for example:
  - `internal/listingkit/task_generation_action_target_resolution.go`
  - `internal/listingkit/service_generation_navigation_dispatch_helpers.go`
  - `internal/listingkit/generation_resolved_action_summary*.go`

- [ ] **Step 1: Add failing seam-consumer test if ownership moved**

Only if Task 2 changes production ownership for a consumer path, add focused tests that lock the new consumer seam.

- [ ] **Step 2: Route consumer to clarified clone home**

Update only the directly affected consumer paths.

Important:

- do not broaden into all clone-helper consumers if they are unaffected
- keep write scope minimal

- [ ] **Step 3: Re-run focused verification**

Run the smallest command set that proves the affected consumer path still behaves correctly.

- [ ] **Step 4: Commit**

Commit only if a real consumer route changed.

---

## Task 4: Lock clone-helper ownership guardrails

**Files:**
- Create: `internal/listingkit/phase22_action_target_clone_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase21_action_target_resolution_boundary_test.go`
  - `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. the clarified clone-helper split
2. target-local clone semantics stay near the local seam when moved
3. shared clone helpers stay in the shared home when intentionally shared
4. `Phase 21` target-resolution behavior ownership remains intact

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestCloneGeneration.*|TestTaskGenerationActionTargetResolution.*Boundary" -count=1
```

Expected: FAIL until guardrails match the final ownership split.

- [ ] **Step 3: Align the boundary suite with the final clone-helper split**

Prefer existing AST/token/source helper style and avoid overly formatting-sensitive assertions.

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestCloneGeneration.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- clone behavior tests PASS
- clone ownership boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase22_action_target_clone_boundary_test.go internal/listingkit/phase21_action_target_resolution_boundary_test.go internal/listingkit/service_generation_actions_test.go internal/listingkit/service_generation_actions.go internal/listingkit/task_generation_action_target_resolution.go
git commit -m "test: lock listingkit action target clone boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 22` stays inside clone helper ownership
2. the plan does not reopen `Phase 19` / `Phase 20` / `Phase 21` behavior seams
3. the plan does not broaden into a full helper rewrite
4. the plan does not expand into a generic clone framework
5. each task has a bounded write set and explicit verification command

This plan passes that check.

## Expected Verification Matrix

During execution, the implementing worker should expect to run at least these checks:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestCloneGeneration.*" -count=1
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestCloneGeneration.*|TestResolveAssetGenerationActionTarget.*" -count=1
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestCloneGeneration.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
