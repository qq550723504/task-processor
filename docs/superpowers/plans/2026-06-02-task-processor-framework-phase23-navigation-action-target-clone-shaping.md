## Task Processor Framework Phase 23 ListingKit Navigation Action-Target Clone Shaping Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which action-target clone semantics belong to the new local clone home and which navigation-only shaping should remain near review-navigation construction.

### Architecture

Keep `Phase 22` intact. Do **not** reopen the broad clone split, do **not** move shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)`, and do **not** redesign outward navigation behavior. Instead, isolate the duplication between:

- [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1)
- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)

so that navigation-specific action-target shaping reuses the local clone seam where possible, while keeping `NavigationTarget = nil` and review-navigation assembly logic in the navigation-local home.

### Tech Stack

Go, ListingKit action-target clone semantics, review navigation target shaping, feature-local seams, source-boundary guardrails

### Out Of Scope For This Slice

- reopening `Phase 19` / `Phase 20` / `Phase 21` / `Phase 22` behavior seams
- moving shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)`
- redesigning review navigation outward behavior
- broad cleanup across all generation review helpers
- generic clone framework or strategy layer
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 22`, broad helper ownership is clearer, but a smaller duplication hotspot remains:

- [cloneAssetGenerationActionTargetForNavigation(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:55)
- [cloneAssetGenerationActionTarget(...)](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:3)

Today, navigation-specific shaping still duplicates most of the clone work for:

1. filters
2. queue query
3. retry request
4. expected impact

The only truly navigation-specific part is that the navigation flavor must clear `NavigationTarget` before being embedded into:

- [buildGenerationReviewActionNavigationTarget(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:40)

The ownership problem is no longer “who owns action-target clone semantics in general.” `Phase 22` solved that. The next problem is “why does review navigation still keep a near-duplicate action-target clone implementation instead of reusing the new local clone seam.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1)
  - Current local home for action-target clone semantics
  - Should remain the owner of the common clone work

- [internal/listingkit/generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)
  - Current navigation-local home
  - Should end the phase owning only navigation-specific shaping

- [internal/listingkit/service_generation_actions_test.go](/D:/code/task-processor/internal/listingkit/service_generation_actions_test.go:1)
  - Existing clone behavior coverage
  - Good place to keep outward clone semantics locked

### New files this phase may introduce

- `internal/listingkit/phase23_navigation_action_target_clone_boundary_test.go`
  - Guardrail ensuring clone reuse and navigation-only shaping stay clearly split

If a new production file is needed, keep it narrow and feature-local. Do not create a general-purpose clone reuse layer.

---

## Target Outcome

At the end of `Phase 23`:

- navigation-specific action-target clone shaping is clearer
- common action-target clone semantics are reused from the `Phase 22` local clone home
- navigation-local code owns only the review-navigation-specific delta
- outward navigation behavior remains unchanged
- ownership guardrails lock the clarified reuse split

---

## Task 1: Lock current navigation action-target clone behavior

**Files:**
- Modify: `internal/listingkit/generation_review_navigation_target_test.go`
- Or modify the smallest existing test home that already covers review navigation behavior

- [ ] **Step 1: Add failing behavior tests for navigation-specific clone shaping**

Lock the current behavior of:

- `cloneAssetGenerationActionTargetForNavigation(...)`
- `buildGenerationReviewActionNavigationTarget(...)`

Behavior to lock:

1. nil input returns nil
2. common nested fields remain defensively cloned
3. returned navigation action target clears `NavigationTarget`
4. mutating the returned navigation target/action target does not mutate the original
5. navigation target identity / outward review-navigation behavior remains unchanged

- [ ] **Step 2: Run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneAssetGenerationActionTargetForNavigation.*" -count=1
```

Expected: PASS after tests are aligned to current behavior.

- [ ] **Step 3: Commit**

```bash
git add internal/listingkit/generation_review_navigation_target_test.go
git commit -m "test: lock listingkit navigation action target clone behavior"
```

---

## Task 2: Reuse the local clone seam for common action-target clone work

**Files:**
- Modify: `internal/listingkit/generation_review_navigation_target.go`
- Modify if needed: `internal/listingkit/task_generation_action_target_clone.go`
- Modify if needed: test file from Task 1

- [ ] **Step 1: Decide the minimal reuse split**

Before editing, determine which part is:

- common action-target clone work that should reuse the local clone home
- navigation-only shaping that must remain near review navigation

The move should be minimal and behavior-preserving.

- [ ] **Step 2: Implement the clarified reuse**

Refactor so that:

- navigation-specific clone shaping no longer duplicates the common clone steps
- `NavigationTarget = nil` shaping remains clearly owned by the navigation-local seam

Important:

- preserve exact behavior locked in Task 1
- do not change outward review-navigation identity behavior
- do not widen into shared queue/retry helper cleanup

- [ ] **Step 3: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneAssetGenerationActionTargetForNavigation.*|TestCloneAssetGeneration.*" -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/listingkit/generation_review_navigation_target.go internal/listingkit/task_generation_action_target_clone.go internal/listingkit/generation_review_navigation_target_test.go
git commit -m "refactor: reuse listingkit action target clone for navigation shaping"
```

---

## Task 3: Lock navigation clone ownership guardrails

**Files:**
- Create: `internal/listingkit/phase23_navigation_action_target_clone_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase22_action_target_clone_boundary_test.go`
  - `internal/listingkit/generation_review_navigation_target_test.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. common action-target clone semantics stay in the `Phase 22` local clone home
2. review navigation target keeps only the navigation-specific shaping delta
3. shared `queue/retry` clone helpers remain out of scope for this phase
4. `Phase 22` clone ownership split remains intact

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestGenerationReviewActionNavigationTarget.*|TestTaskGenerationActionTargetClone.*Boundary" -count=1
```

Expected: FAIL until guardrails match the final split.

- [ ] **Step 3: Align the boundary suite with the final clone reuse split**

Prefer the existing AST/token/source helper style and avoid fragile formatting assertions.

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestGenerationReviewActionNavigationTarget.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- clone behavior tests PASS
- navigation clone ownership boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase23_navigation_action_target_clone_boundary_test.go internal/listingkit/phase22_action_target_clone_boundary_test.go internal/listingkit/generation_review_navigation_target.go internal/listingkit/task_generation_action_target_clone.go internal/listingkit/generation_review_navigation_target_test.go
git commit -m "test: lock listingkit navigation action target clone boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 23` stays inside navigation action-target clone shaping ownership
2. the plan does not reopen `Phase 22`’s broader clone split
3. the plan does not expand into shared queue / retry clone helper cleanup
4. the plan does not introduce a generic clone framework
5. each task has a bounded write set and explicit verification command

This plan passes that check.

## Expected Verification Matrix

During execution, the implementing worker should expect to run at least these checks:

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneAssetGenerationActionTargetForNavigation.*" -count=1
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneAssetGenerationActionTargetForNavigation.*|TestCloneAssetGeneration.*" -count=1
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestGenerationReviewActionNavigationTarget.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
