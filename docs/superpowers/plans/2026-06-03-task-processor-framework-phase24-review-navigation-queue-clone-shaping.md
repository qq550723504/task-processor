## Task Processor Framework Phase 24 ListingKit Review-Navigation Queue Clone Shaping Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which `QueueQuery` clone semantics belong to the shared clone home and which review-navigation-specific shaping should remain near the review-navigation builder.

### Architecture

Keep `Phase 22` and `Phase 23` intact. Do **not** reopen the action-target clone split, do **not** redesign the outward navigation target contract, and do **not** widen into a multi-consumer rewrite of shared queue/retry clone helpers. Instead, isolate the remaining duplication between:

- [buildGenerationReviewActionNavigationTarget(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:40)
- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)

so that review-navigation builder code reuses the shared queue clone home where possible, while keeping any truly review-navigation-specific shaping local.

### Tech Stack

Go, ListingKit review navigation target shaping, queue query clone semantics, feature-local seams, source-boundary guardrails

### Out Of Scope For This Slice

- reopening `Phase 22` / `Phase 23` behavior seams
- broad cleanup of all shared queue/retry clone consumers
- redesigning navigation identity/descriptor behavior
- moving `cloneRetryGenerationTasksRequest(...)`
- generic queue-clone framework or policy layer
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 23`, review-navigation action-target clone duplication is gone, but one smaller duplication hotspot remains:

- [buildGenerationReviewActionNavigationTarget(...)](/D:/code-task-processor/internal/listingkit/generation_review_navigation_target.go:40)
- [cloneGenerationQueueQuery(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:15)

Today the review-navigation builder still performs its own queue shallow clone:

- `cloned := *target.QueueQuery`

even though the package already has a shared queue clone helper used by many other consumers.

The ownership problem is no longer “who owns action-target clone semantics.” `Phase 23` solved that. The next problem is “why does review-navigation builder still keep a local queue clone implementation instead of reusing the shared queue clone home.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/generation_review_navigation_target.go](/D:/code-task-processor/internal/listingkit/generation_review_navigation_target.go:1)
  - Current review-navigation local home
  - Should end the phase owning only truly navigation-local queue shaping

- [internal/listingkit/service_generation_actions.go](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:1)
  - Current shared home for `cloneGenerationQueueQuery(...)`
  - Should remain the owner of common queue clone semantics

- [internal/listingkit/generation_review_navigation_target_test.go](/D:/code-task-processor/internal/listingkit/generation_review_navigation_target_test.go:1)
  - Existing outward behavior coverage for review-navigation action target builder
  - Good place to lock queue clone behavior if needed

### New files this phase may introduce

- `internal/listingkit/phase24_review_navigation_queue_clone_boundary_test.go`
  - Guardrail ensuring shared queue clone reuse and local builder shaping stay clearly split

If a new production file is needed, keep it narrow and feature-local. Do not create a general queue-clone abstraction.

---

## Target Outcome

At the end of `Phase 24`:

- review-navigation builder reuses the shared queue clone home for common queue clone semantics
- review-navigation local code owns only truly navigation-local queue shaping
- outward review-navigation behavior remains unchanged
- `Phase 22` / `Phase 23` ownership splits remain intact
- queue clone ownership guardrails lock the clarified reuse split

---

## Task 1: Lock current review-navigation queue clone behavior

**Files:**
- Modify: `internal/listingkit/generation_review_navigation_target_test.go`

- [ ] **Step 1: Add failing behavior tests for builder queue clone shaping**

Lock the current behavior of:

- `buildGenerationReviewActionNavigationTarget(...)`

Behavior to lock:

1. `QueueQuery` remains a distinct cloned pointer
2. queue clone stays field-for-field equal to the source query
3. mutating returned navigation `QueueQuery` does not mutate the original
4. outward navigation identity / descriptor behavior remains unchanged

- [ ] **Step 2: Run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*" -count=1
```

Expected: PASS after tests are aligned to current behavior.

- [ ] **Step 3: Commit**

```bash
git add internal/listingkit/generation_review_navigation_target_test.go
git commit -m "test: lock listingkit review navigation queue clone behavior"
```

---

## Task 2: Reuse the shared queue clone home from the builder

**Files:**
- Modify: `internal/listingkit/generation_review_navigation_target.go`
- Modify if needed: `internal/listingkit/generation_review_navigation_target_test.go`

- [ ] **Step 1: Decide the minimal reuse split**

Before editing, determine which part is:

- common queue clone work that should reuse `cloneGenerationQueueQuery(...)`
- truly review-navigation-local shaping that must remain in the builder

The move should be minimal and behavior-preserving.

- [ ] **Step 2: Implement the clarified reuse**

Refactor so that:

- builder no longer reimplements the common queue shallow clone itself
- any review-navigation-local shaping stays near the builder

Important:

- preserve exact behavior locked in Task 1
- do not reopen action-target clone shaping
- do not widen into other `cloneGenerationQueueQuery(...)` consumers

- [ ] **Step 3: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneGenerationQueueQuery.*" -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/listingkit/generation_review_navigation_target.go internal/listingkit/generation_review_navigation_target_test.go
git commit -m "refactor: reuse listingkit queue clone for review navigation"
```

---

## Task 3: Lock review-navigation queue clone ownership guardrails

**Files:**
- Create: `internal/listingkit/phase24_review_navigation_queue_clone_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase23_navigation_action_target_clone_boundary_test.go`
  - `internal/listingkit/generation_review_navigation_target_test.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. review-navigation builder routes common queue clone work through `cloneGenerationQueueQuery(...)`
2. local builder code keeps only truly review-navigation-local queue shaping
3. `Phase 23` action-target clone reuse split remains intact
4. shared queue clone helper stays in its current shared home for this phase

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestTaskGenerationNavigation.*Boundary|TestCloneGenerationQueueQuery.*" -count=1
```

Expected: FAIL until guardrails match the final split.

- [ ] **Step 3: Align the boundary suite with the final queue clone reuse split**

Prefer the existing AST/token/source helper style and avoid fragile formatting assertions.

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- queue clone behavior tests PASS
- review-navigation queue clone ownership boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase24_review_navigation_queue_clone_boundary_test.go internal/listingkit/phase23_navigation_action_target_clone_boundary_test.go internal/listingkit/generation_review_navigation_target.go internal/listingkit/generation_review_navigation_target_test.go
git commit -m "test: lock listingkit review navigation queue clone boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 24` stays inside review-navigation queue clone shaping ownership
2. the plan does not reopen `Phase 23`’s action-target clone reuse
3. the plan does not expand into shared queue/retry helper cleanup across all consumers
4. the plan does not introduce a generic queue-clone framework
5. each task has a bounded write set and explicit verification command

This plan passes that check.

## Expected Verification Matrix

During execution, the implementing worker should expect to run at least these checks:

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*" -count=1
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
