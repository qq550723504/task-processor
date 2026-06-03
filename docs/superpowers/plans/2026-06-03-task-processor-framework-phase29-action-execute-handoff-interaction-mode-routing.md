## Task Processor Framework Phase 29 ListingKit Action Execute Handoff Interaction-Mode Routing Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which interaction-mode routing semantics belong inside the action execute request-handoff seam, and which should move into narrower handoff-local routing homes.

### Architecture

Keep `Phase 28` intact. Do **not** reopen the handoff / branch-result routing split, do **not** redesign shared `queue/retry` clone helpers, and do **not** widen into a multi-consumer routing framework. Instead, isolate the mixed responsibilities currently sitting together in:

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

so that request-handoff code more clearly separates:

- interaction-mode selection
- local seam dispatch
- outward handoff behavior stability

This is a handoff-local ownership move, not a generic routing abstraction.

### Tech Stack

Go, ListingKit action execute request handoff, interaction-mode routing, local seam dispatch, feature-local seams, source-boundary guardrails

### Out Of Scope For This Slice

- reopening `Phase 27` / `Phase 28` handoff and routing seams
- redesigning shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)`
- broad cleanup across execute/refresh/projection/finalize phases
- generic routing abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 28`, branch-result routing is gone from the request-handoff seam, but the seam still mixes several responsibilities in one method:

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

Today it still directly owns:

1. `retryable` vs default mode choice
2. retry local seam dispatch
3. queue local seam dispatch

The ownership problem is no longer “who routes branch results into adaptation.” `Phase 28` solved that. The next problem is “why does the handoff seam still inline both mode selection and local seam dispatch instead of routing through a clearer handoff-local mode-routing home.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)
  - Current request-handoff home
  - Should end the phase as a thinner orchestration seam

- [internal/listingkit/task_generation_action_execute_request_handoff_retry.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry.go:1)
  - Current retry invocation home

- [internal/listingkit/task_generation_action_execute_request_handoff_queue.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue.go:1)
  - Current queue invocation home

- [internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go:1)
  - Current retry result-routing home

- [internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go:1)
  - Current queue result-routing home

- [internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go:1)
  - Existing routing guardrails
  - Likely needs alignment once interaction-mode routing moves

### New files this phase may introduce

- `internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go`
  - Narrow local seam for interaction-mode routing, if justified

- `internal/listingkit/phase29_action_execute_handoff_mode_boundary_test.go`
  - Guardrail ensuring interaction-mode routing ownership stays clear

If no new production file is truly justified, keep the implementation smaller. Do not force symmetry.

---

## Target Outcome

At the end of `Phase 29`:

- request-handoff interaction-mode routing is clearer
- local retry / queue seam dispatch no longer hides inline inside the broad handoff body if a narrower local routing seam helps
- `Phase 27/28` local seams remain intact
- outward action execution behavior remains unchanged
- handoff interaction-mode routing guardrails lock the clarified split

---

## Task 1: Lock current handoff interaction-mode routing behavior

**Files:**
- Modify: the smallest existing test home that already covers handoff behavior, currently `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Add failing behavior tests for handoff interaction-mode routing**

Lock the current behavior of:

- `taskGenerationActionExecuteRequestHandoffPhase.run(...)`

Behavior to lock:

1. `retryable` mode continues to dispatch through retry local seams
2. default mode continues to dispatch through queue local seams
3. outward `retryPage` / `queuePage` surfacing remains unchanged
4. `Phase 27` invocation behavior remains intact
5. `Phase 28` result-routing behavior remains intact

- [ ] **Step 2: Run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
```

Expected: PASS after tests are aligned to current behavior.

- [ ] **Step 3: Commit**

```bash
git add <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff mode behavior"
```

---

## Task 2: Extract interaction-mode routing seam if justified

**Files:**
- Modify: `internal/listingkit/task_generation_action_execute_request_handoff.go`
- Create/modify minimal local routing seam file(s) only if justified
- Modify tests from Task 1 if needed

- [ ] **Step 1: Decide the minimal mode-routing split**

Before editing, determine which parts are:

- truly common handoff orchestration that should remain in `run(...)`
- mode-routing logic worth extracting

The move should be minimal and behavior-preserving.

- [ ] **Step 2: Implement the clarified mode-routing split**

Refactor so that:

- local retry / queue seam dispatch no longer hides inline inside the broad handoff body if a narrower local routing seam helps
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
git commit -m "refactor: split listingkit action execute handoff mode routing"
```

---

## Task 3: Keep top-level handoff seam orchestration-only

**Files:**
- Modify only if needed after Task 2:
  - `internal/listingkit/task_generation_action_execute_request_handoff.go`
  - mode-routing seam file(s)
  - test file(s)

- [ ] **Step 1: Add/extend tests only if orchestration ownership is still mixed**

If Task 2 leaves obvious mode-routing and orchestration mixed together, add focused tests to lock the intended split.

- [ ] **Step 2: Narrow the top-level handoff seam only if justified**

Make `run(...)` a clearer orchestration shell only if this materially improves ownership clarity without widening the slice.

- [ ] **Step 3: Re-run focused verification**

Run the smallest command set that proves handoff behavior still holds.

- [ ] **Step 4: Commit**

Commit only if a real additional production narrowing is introduced.

---

## Task 4: Lock handoff interaction-mode routing guardrails

**Files:**
- Create: `internal/listingkit/phase29_action_execute_handoff_mode_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go`
  - handoff behavior test file(s)

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. handoff seam routes through the final local mode-routing seam(s)
2. branch-local invocation and branch-result routing continue to stay outside the mode-routing owner
3. outward action execute behavior remains intact

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
```

Expected: FAIL until guardrails match the final split.

- [ ] **Step 3: Align the boundary suite with the final mode-routing split**

Prefer existing AST/token/source helper style and avoid fragile formatting assertions.

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- handoff behavior tests PASS
- handoff mode-routing boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go internal/listingkit/phase29_action_execute_handoff_mode_boundary_test.go internal/listingkit/task_generation_action_execute_request_handoff*.go <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff mode boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 29` stays inside action execute handoff interaction-mode routing ownership
2. the plan does not reopen `Phase 28` handoff / branch-result routing split
3. the plan does not expand into shared queue/retry helper relocation
4. the plan does not introduce a generic mode-routing framework
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
