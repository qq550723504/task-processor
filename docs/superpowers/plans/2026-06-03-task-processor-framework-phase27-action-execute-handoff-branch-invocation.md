## Task Processor Framework Phase 27 ListingKit Action Execute Handoff Branch-Invocation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which branch-local invocation semantics belong inside the action execute request-handoff seam, and which should move into narrower handoff-local branch homes.

### Architecture

Keep `Phase 26` intact. Do **not** reopen the handoff/adaptation split, do **not** redesign shared `queue/retry` clone helpers, and do **not** widen into a multi-consumer invocation framework. Instead, isolate the mixed responsibilities currently sitting together in:

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

so that request-handoff code more clearly separates:

- branch selection
- branch-local downstream invocation
- shared clone-helper handoff

This is a handoff-local ownership move, not a generic invocation abstraction.

### Tech Stack

Go, ListingKit action execute request handoff, branch-local invocation, shared clone-helper handoff, feature-local seams, source-boundary guardrails

### Out Of Scope For This Slice

- reopening `Phase 25` / `Phase 26` execute and adaptation seams
- redesigning shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)`
- broad cleanup across execute/refresh/projection/finalize phases
- generic invocation abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 26`, handoff result adaptation is gone from the request-handoff seam, but the seam still mixes several responsibilities in one method:

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

Today it still directly owns:

1. `retryable` vs default branch choice
2. clone handoff into `RetryTaskGenerationTasks(...)`
3. clone handoff into `GetTaskGenerationQueue(...)`
4. downstream service invocation

The ownership problem is no longer “who adapts page results into persistenceQueue.” `Phase 26` solved that. The next problem is “why does the handoff seam still inline both branch routing and downstream invocation instead of routing through clearer handoff-local branch homes.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)
  - Current request-handoff home
  - Should end the phase as a thinner orchestration seam

- [internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)
  - Current adaptation home
  - Should remain unchanged in outward semantics

- [internal/listingkit/service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1)
  - Current shared home for queue/retry clone helpers
  - Should remain unchanged in outward semantics

- [internal/listingkit/phase26_action_execute_handoff_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase26_action_execute_handoff_boundary_test.go:1)
  - Existing handoff/adaptation ownership guardrails
  - Likely needs alignment once branch-local seams are introduced

### New files this phase may introduce

- `internal/listingkit/task_generation_action_execute_request_handoff_retry.go`
  - Narrow local seam for retry-branch clone handoff and downstream invocation

- `internal/listingkit/task_generation_action_execute_request_handoff_queue.go`
  - Narrow local seam for queue-branch clone handoff and downstream invocation

- `internal/listingkit/phase27_action_execute_handoff_branch_boundary_test.go`
  - Guardrail ensuring request-handoff branch ownership stays clear

If one shared local branch seam is enough, keep the implementation smaller. Do not force symmetry.

---

## Target Outcome

At the end of `Phase 27`:

- request-handoff branch invocation is clearer
- branch-local clone handoff and service invocation no longer hide inside the broad handoff body if narrower local seams help
- shared clone helpers remain shared
- outward action execution behavior remains unchanged
- handoff branch-invocation guardrails lock the clarified split

---

## Task 1: Lock current handoff branch-invocation behavior

**Files:**
- Modify: the smallest existing test home that already covers handoff behavior, currently `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Add failing behavior tests for handoff branch invocation**

Lock the current behavior of:

- `taskGenerationActionExecuteRequestHandoffPhase.run(...)`

Behavior to lock:

1. `retryable` branch clones `RetryRequest` before invoking the downstream service
2. default branch clones `QueueQuery` before invoking the downstream service
3. downstream mutation of the received request does not back-propagate into the original target
4. `retryable` branch continues to surface `retryPage`
5. default branch continues to surface `queuePage`
6. outward adaptation behavior from `Phase 26` remains untouched

- [ ] **Step 2: Run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
```

Expected: PASS after tests are aligned to current behavior.

- [ ] **Step 3: Commit**

```bash
git add <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff branch behavior"
```

---

## Task 2: Extract branch-local invocation seam(s)

**Files:**
- Modify: `internal/listingkit/task_generation_action_execute_request_handoff.go`
- Create/modify minimal handoff-local branch seam file(s) only if justified
- Modify tests from Task 1 if needed

- [ ] **Step 1: Decide the minimal branch-local split**

Before editing, determine which parts are:

- truly common handoff orchestration that should remain in `run(...)`
- branch-local invocation logic worth extracting

The move should be minimal and behavior-preserving.

- [ ] **Step 2: Implement the clarified branch split**

Refactor so that:

- branch-local clone handoff and downstream invocation no longer hide inline inside the broad handoff body if narrower local seams help
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
git add internal/listingkit/task_generation_action_execute_request_handoff*.go <handoff behavior test file(s)>
git commit -m "refactor: split listingkit action execute handoff branch invocation"
```

---

## Task 3: Keep top-level handoff seam orchestration-only

**Files:**
- Modify only if needed after Task 2:
  - `internal/listingkit/task_generation_action_execute_request_handoff.go`
  - branch-local seam file(s)
  - test file(s)

- [ ] **Step 1: Add/extend tests only if orchestration ownership is still mixed**

If Task 2 leaves obvious branch-local invocation and orchestration mixed together, add focused tests to lock the intended split.

- [ ] **Step 2: Narrow the top-level handoff seam only if justified**

Make `run(...)` a clearer orchestration shell only if this materially improves ownership clarity without widening the slice.

- [ ] **Step 3: Re-run focused verification**

Run the smallest command set that proves handoff behavior still holds.

- [ ] **Step 4: Commit**

Commit only if a real additional production narrowing is introduced.

---

## Task 4: Lock handoff branch-invocation ownership guardrails

**Files:**
- Create: `internal/listingkit/phase27_action_execute_handoff_branch_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase26_action_execute_handoff_boundary_test.go`
  - handoff behavior test file(s)

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. handoff seam routes branch-local invocation through the final local seam(s)
2. shared `queue/retry` clone helpers remain outside the new branch-local owner(s)
3. result adaptation continues to stay in the `Phase 26` adaptation home
4. outward action execute behavior remains intact

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
```

Expected: FAIL until guardrails match the final split.

- [ ] **Step 3: Align the boundary suite with the final branch split**

Prefer existing AST/token/source helper style and avoid fragile formatting assertions.

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- handoff behavior tests PASS
- handoff branch ownership boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase26_action_execute_handoff_boundary_test.go internal/listingkit/phase27_action_execute_handoff_branch_boundary_test.go internal/listingkit/task_generation_action_execute_request_handoff*.go <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff branch boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 27` stays inside action execute handoff branch-invocation ownership
2. the plan does not reopen `Phase 26` handoff/adaptation split
3. the plan does not expand into shared queue/retry helper relocation
4. the plan does not introduce a generic invocation framework
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
