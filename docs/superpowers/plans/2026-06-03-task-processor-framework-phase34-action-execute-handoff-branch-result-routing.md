## Task Processor Framework Phase 34 ListingKit Action Execute Handoff Branch-Specific Result Routing Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which branch-specific result routing semantics belong inside the retry/queue result seams, and which should move into narrower dispatch-local homes.

### Architecture

Keep `Phase 33` intact. Do **not** reopen the mode-pairing / normalization split, do **not** redesign shared `queue/retry` clone helpers, and do **not** widen into a multi-consumer branch-result-routing framework. Instead, isolate the mixed responsibilities currently sitting together across:

- [task_generation_action_execute_request_handoff_retry_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go:1)
- [task_generation_action_execute_request_handoff_queue_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go:1)

so that branch-specific result routing more clearly separates:

- branch-specific page input
- unified normalization dispatch
- outward action execute behavior stability

This is a result-dispatch ownership move, not a generic branch-result-routing abstraction.

### Tech Stack

Go, ListingKit action execute handoff branch-specific result routing, result dispatch, feature-local seams, source-boundary guardrails

### Out Of Scope For This Slice

- reopening `Phase 31` / `Phase 32` / `Phase 33` handoff seams
- redesigning shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)`
- broad cleanup across execute/refresh/projection/finalize phases
- generic branch-result-routing abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 33`, mode-pairing is clean, but branch-specific result routing still mirrors retry/queue thin shells:

- [task_generation_action_execute_request_handoff_retry_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go:1)
- [task_generation_action_execute_request_handoff_queue_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go:1)

Today they still directly own:

1. branch-specific page input
2. dispatch into unified result-normalization / result-shape layer

The ownership problem is no longer “who owns mode-pairing normalization.” `Phase 33` solved that. The next problem is “why do two branch-specific result seams still exist as mirrored thin shells instead of routing that ownership through a clearer dispatch home.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go:1)
  - Current retry result home

- [internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go:1)
  - Current queue result home

- [internal/listingkit/task_generation_action_execute_request_handoff_result_normalization.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_normalization.go:1)
  - Current unified normalization home

- [internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go:1)
  - Current outward result-shape home

- [internal/listingkit/phase32_action_execute_handoff_result_normalization_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase32_action_execute_handoff_result_normalization_boundary_test.go:1)
  - Existing result-normalization boundary guardrail

### New files this phase may introduce

- `internal/listingkit/task_generation_action_execute_request_handoff_result_dispatch.go`
  - Narrow local seam for branch-specific result dispatch, if justified

- `internal/listingkit/phase34_action_execute_handoff_branch_result_routing_boundary_test.go`
  - Guardrail ensuring branch-specific result routing ownership stays clear

If no new production file is truly justified, keep the implementation smaller. Do not force symmetry.

---

## Target Outcome

At the end of `Phase 34`:

- branch-specific result routing ownership is clearer
- retry/queue thin-shell dispatch no longer hides inline inside two mirrored result owners if a narrower local seam helps
- unified normalization / result-shape layers remain intact
- outward action execution behavior remains unchanged
- branch-result-routing guardrails lock the clarified split

---

## Task 1: Lock current branch-specific result routing behavior

**Files:**
- Modify: the smallest existing test home that already covers handoff behavior, currently `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Add failing behavior tests for branch-specific result routing**

Lock the current behavior of:

- `taskGenerationActionExecuteRequestHandoffRetryResultPhase`
- `taskGenerationActionExecuteRequestHandoffQueueResultPhase`

Behavior to lock:

1. retry result seam still routes retry page into the same outward handoff result
2. queue result seam still routes queue page into the same outward handoff result
3. `Phase 32` unified normalization / result-shape behavior remains intact
4. outward `retryPage / queuePage / persistenceQueue` behavior remains unchanged

- [ ] **Step 2: Run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
```

Expected: PASS after tests are aligned to current behavior.

- [ ] **Step 3: Commit**

```bash
git add <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff branch result routing behavior"
```

---

## Task 2: Extract branch-specific result dispatch seam if justified

**Files:**
- Modify:
  - `internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go`
  - `internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go`
- Create/modify minimal local dispatch seam file(s) only if justified
- Modify tests from Task 1 if needed

- [ ] **Step 1: Decide the minimal dispatch split**

Before editing, determine which parts are:

- truly local branch-specific dispatch worth extracting
- unified normalization / shape dispatch that should remain stable

The move should be minimal and behavior-preserving.

- [ ] **Step 2: Implement the clarified dispatch split**

Refactor so that:

- retry/queue thin-shell dispatch no longer hides inline inside mirrored result owners if a narrower local seam helps
- outward branch behavior and returned handoff object remain unchanged

Important:

- preserve exact behavior locked in Task 1
- do not move shared clone helpers
- do not widen into execute/refresh/projection/finalize cleanup

- [ ] **Step 3: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestCloneGenerationQueueQuery.*|TestCloneRetryGenerationTasksRequest.*" -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/listingkit/task_generation_action_execute_request_handoff_*result*.go <handoff behavior test file(s)>
git commit -m "refactor: split listingkit action execute handoff branch result routing"
```

---

## Task 3: Keep unified normalization / result-shape stable

**Files:**
- Modify only if needed after Task 2:
  - `internal/listingkit/task_generation_action_execute_request_handoff_result_normalization.go`
  - `internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go`
  - test file(s)

- [ ] **Step 1: Add/extend tests only if unified result ownership is still mixed**

If Task 2 leaves obvious unified result knowledge spread incorrectly, add focused tests to lock the intended split.

- [ ] **Step 2: Keep unified layers stable unless justified**

Do not change unified normalization / result-shape seams unless a real ownership improvement is needed.

- [ ] **Step 3: Re-run focused verification**

Run the smallest command set that proves handoff behavior still holds.

- [ ] **Step 4: Commit**

Commit only if a real additional production narrowing is introduced.

---

## Task 4: Lock branch-specific result routing ownership guardrails

**Files:**
- Create: `internal/listingkit/phase34_action_execute_handoff_branch_result_routing_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase32_action_execute_handoff_result_normalization_boundary_test.go`
  - handoff behavior test file(s)

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. branch-specific result routing ownership stays in the final intended local home
2. unified normalization / result-shape layers continue to stay outside the branch-specific routing owner when intended
3. outward action execute behavior remains intact

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
```

Expected: FAIL until guardrails match the final split.

- [ ] **Step 3: Align the boundary suite with the final dispatch split**

Prefer existing AST/token/source helper style and avoid fragile formatting assertions.

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- handoff behavior tests PASS
- handoff branch-result-routing boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase32_action_execute_handoff_result_normalization_boundary_test.go internal/listingkit/phase34_action_execute_handoff_branch_result_routing_boundary_test.go internal/listingkit/task_generation_action_execute_request_handoff_*result*.go <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff branch result routing boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 34` stays inside action execute handoff branch-specific result routing ownership
2. the plan does not reopen `Phase 33` mode-pairing / normalization split
3. the plan does not expand into shared queue/retry helper relocation
4. the plan does not introduce a generic branch-result-routing framework
5. each task has a bounded write set and explicit verification command

This plan passes that check.

## Expected Verification Matrix

During execution, the implementing worker should expect to run at least these checks:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestCloneGenerationQueueQuery.*|TestCloneRetryGenerationTasksRequest.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
