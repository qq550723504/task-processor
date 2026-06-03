## Task Processor Framework Phase 28 ListingKit Action Execute Handoff Branch-Result Routing Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which branch-result routing semantics belong inside the action execute request-handoff seam, and which should move into narrower handoff-local result-routing homes.

### Architecture

Keep `Phase 27` intact. Do **not** reopen the handoff / branch-local invocation split, do **not** redesign shared `queue/retry` clone helpers, and do **not** widen into a multi-consumer routing framework. Instead, isolate the mixed responsibilities currently sitting together in:

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

so that request-handoff code more clearly separates:

- branch selection
- branch-local phase invocation
- branch-result routing into the `Phase 26` adaptation home

This is a handoff-local ownership move, not a generic routing abstraction.

### Tech Stack

Go, ListingKit action execute request handoff, branch-result routing, result-adaptation consumption, feature-local seams, source-boundary guardrails

### Out Of Scope For This Slice

- reopening `Phase 26` / `Phase 27` handoff and invocation seams
- redesigning shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)`
- broad cleanup across execute/refresh/projection/finalize phases
- generic routing abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 27`, branch-local invocation is gone from the request-handoff seam, but the seam still mixes several responsibilities in one method:

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

Today it still directly owns:

1. `retryable` vs default branch choice
2. branch-local phase invocation
3. `retryPage -> adaptation.fromRetryPage(...)`
4. `queuePage -> adaptation.fromQueuePage(...)`

The ownership problem is no longer “who invokes downstream retry/queue services.” `Phase 27` solved that. The next problem is “why does the handoff seam still inline both branch routing and adaptation dispatch instead of routing through clearer handoff-local branch-result homes.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)
  - Current request-handoff home
  - Should end the phase as a thinner orchestration seam

- [internal/listingkit/task_generation_action_execute_request_handoff_retry.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry.go:1)
  - Current retry-branch invocation home
  - May become the natural owner of retry result routing if justified

- [internal/listingkit/task_generation_action_execute_request_handoff_queue.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue.go:1)
  - Current queue-branch invocation home
  - May become the natural owner of queue result routing if justified

- [internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)
  - Current adaptation home
  - Should remain unchanged in outward semantics

- [internal/listingkit/phase27_action_execute_handoff_branch_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase27_action_execute_handoff_branch_boundary_test.go:1)
  - Existing branch-invocation guardrails
  - Likely needs alignment once branch-result routing moves

### New files this phase may introduce

- `internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go`
  - Narrow local seam for retry-branch result routing, if needed

- `internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go`
  - Narrow local seam for queue-branch result routing, if needed

- `internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go`
  - Guardrail ensuring request-handoff branch-result routing stays clear

If one shared local result-routing seam is enough, keep the implementation smaller. Do not force symmetry.

---

## Target Outcome

At the end of `Phase 28`:

- request-handoff branch-result routing is clearer
- branch-local phase results no longer hide adaptation dispatch inline inside the broad handoff body if narrower local seams help
- `Phase 26` adaptation home remains intact
- outward action execution behavior remains unchanged
- handoff branch-result routing guardrails lock the clarified split

---

## Task 1: Lock current handoff branch-result routing behavior

**Files:**
- Modify: the smallest existing test home that already covers handoff behavior, currently `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Add failing behavior tests for handoff branch-result routing**

Lock the current behavior of:

- `taskGenerationActionExecuteRequestHandoffPhase.run(...)`

Behavior to lock:

1. `retryable` branch still routes through retry page and adaptation to `persistenceQueue`
2. default branch still routes through queue page and adaptation to `persistenceQueue`
3. outward `retryPage` / `queuePage` surfacing remains unchanged
4. `Phase 27` branch-local invocation behavior remains intact
5. `Phase 26` adaptation semantics remain intact

- [ ] **Step 2: Run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
```

Expected: PASS after tests are aligned to current behavior.

- [ ] **Step 3: Commit**

```bash
git add <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff routing behavior"
```

---

## Task 2: Extract branch-result routing seam(s)

**Files:**
- Modify: `internal/listingkit/task_generation_action_execute_request_handoff.go`
- Modify minimal branch-local seam file(s) only if justified
- Modify tests from Task 1 if needed

- [ ] **Step 1: Decide the minimal branch-result split**

Before editing, determine which parts are:

- truly common handoff orchestration that should remain in `run(...)`
- branch-result routing logic worth extracting

The move should be minimal and behavior-preserving.

- [ ] **Step 2: Implement the clarified branch-result split**

Refactor so that:

- branch-local phase results no longer hide adaptation dispatch inline inside the broad handoff body if narrower local seams help
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
git commit -m "refactor: split listingkit action execute handoff routing"
```

---

## Task 3: Keep top-level handoff seam orchestration-only

**Files:**
- Modify only if needed after Task 2:
  - `internal/listingkit/task_generation_action_execute_request_handoff.go`
  - branch-result seam file(s)
  - test file(s)

- [ ] **Step 1: Add/extend tests only if orchestration ownership is still mixed**

If Task 2 leaves obvious branch-result routing and orchestration mixed together, add focused tests to lock the intended split.

- [ ] **Step 2: Narrow the top-level handoff seam only if justified**

Make `run(...)` a clearer orchestration shell only if this materially improves ownership clarity without widening the slice.

- [ ] **Step 3: Re-run focused verification**

Run the smallest command set that proves handoff behavior still holds.

- [ ] **Step 4: Commit**

Commit only if a real additional production narrowing is introduced.

---

## Task 4: Lock handoff branch-result routing guardrails

**Files:**
- Create: `internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase27_action_execute_handoff_branch_boundary_test.go`
  - handoff behavior test file(s)

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. handoff seam routes branch results through the final local routing seam(s)
2. branch-local invocation continues to stay outside the routing owner
3. result adaptation continues to stay in the `Phase 26` adaptation home
4. outward action execute behavior remains intact

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
```

Expected: FAIL until guardrails match the final split.

- [ ] **Step 3: Align the boundary suite with the final routing split**

Prefer existing AST/token/source helper style and avoid fragile formatting assertions.

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- handoff behavior tests PASS
- handoff routing boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase27_action_execute_handoff_branch_boundary_test.go internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go internal/listingkit/task_generation_action_execute_request_handoff*.go <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff routing boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 28` stays inside action execute handoff branch-result routing ownership
2. the plan does not reopen `Phase 27` handoff / branch-local invocation split
3. the plan does not expand into shared queue/retry helper relocation
4. the plan does not introduce a generic routing framework
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
