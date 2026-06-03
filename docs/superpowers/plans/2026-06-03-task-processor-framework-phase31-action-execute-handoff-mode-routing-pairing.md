## Task Processor Framework Phase 31 ListingKit Action Execute Handoff Mode-Routing Pairing Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which branch pairing semantics belong inside the action execute handoff mode-routing seam, and which should move into narrower mode-routing-local homes.

### Architecture

Keep `Phase 30` intact. Do **not** reopen the handoff result-shape / adaptation split, do **not** redesign shared `queue/retry` clone helpers, and do **not** widen into a multi-consumer pairing framework. Instead, isolate the mixed responsibilities currently sitting together in:

- [task_generation_action_execute_request_handoff_mode_routing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go:1)

so that mode-routing code more clearly separates:

- interaction-mode selection
- branch invocation seam selection
- branch result seam pairing

This is a mode-routing-local ownership move, not a generic pairing abstraction.

### Tech Stack

Go, ListingKit action execute handoff mode routing, branch pairing, feature-local seams, source-boundary guardrails

### Out Of Scope For This Slice

- reopening `Phase 27` / `Phase 28` / `Phase 29` / `Phase 30` handoff seams
- redesigning shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)`
- broad cleanup across execute/refresh/projection/finalize phases
- generic pairing abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 30`, the handoff result layer is clean, but the mode-routing seam still mixes several responsibilities:

- [task_generation_action_execute_request_handoff_mode_routing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go:1)

Today it still directly owns:

1. `retryable` vs default mode choice
2. retry branch invocation seam selection
3. retry branch result seam pairing
4. queue branch invocation seam selection
5. queue branch result seam pairing

The ownership problem is no longer “who owns result shape.” `Phase 30` solved that. The next problem is “why does one mode-routing seam still inline both mode choice and branch invocation/result pairing instead of routing that ownership through a clearer pairing home.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go:1)
  - Current mode-routing home

- [internal/listingkit/task_generation_action_execute_request_handoff_retry.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry.go:1)
  - Current retry invocation home

- [internal/listingkit/task_generation_action_execute_request_handoff_queue.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue.go:1)
  - Current queue invocation home

- [internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go:1)
  - Current retry result home

- [internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go:1)
  - Current queue result home

- [internal/listingkit/phase29_action_execute_handoff_mode_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase29_action_execute_handoff_mode_boundary_test.go:1)
  - Existing mode-routing boundary guardrail

### New files this phase may introduce

- `internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go`
  - Narrow local seam for branch invocation/result pairing, if justified

- `internal/listingkit/phase31_action_execute_handoff_mode_pairing_boundary_test.go`
  - Guardrail ensuring mode-routing pairing ownership stays clear

If no new production file is truly justified, keep the implementation smaller. Do not force symmetry.

---

## Target Outcome

At the end of `Phase 31`:

- mode-routing pairing ownership is clearer
- branch invocation/result pairing no longer hides inline inside the broad mode-routing body if a narrower local pairing seam helps
- branch-local invocation and result seams remain intact
- outward action execution behavior remains unchanged
- handoff mode-pairing guardrails lock the clarified split

---

## Task 1: Lock current mode-routing pairing behavior

**Files:**
- Modify: the smallest existing test home that already covers handoff behavior, currently `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Add failing behavior tests for mode-routing pairing**

Lock the current behavior of:

- `taskGenerationActionExecuteRequestHandoffModeRoutingPhase.run(...)`

Behavior to lock:

1. `retryable` mode continues to pair retry invocation seam with retry result seam
2. default mode continues to pair queue invocation seam with queue result seam
3. outward `retryPage / queuePage / persistenceQueue` behavior remains unchanged
4. `Phase 30` result-shape / adaptation behavior remains intact

- [ ] **Step 2: Run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
```

Expected: PASS after tests are aligned to current behavior.

- [ ] **Step 3: Commit**

```bash
git add <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff mode pairing behavior"
```

---

## Task 2: Extract mode-routing pairing seam if justified

**Files:**
- Modify: `internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go`
- Create/modify minimal local pairing seam file(s) only if justified
- Modify tests from Task 1 if needed

- [ ] **Step 1: Decide the minimal pairing split**

Before editing, determine which parts are:

- truly local interaction-mode selection that should remain in `run(...)`
- branch invocation/result pairing logic worth extracting

The move should be minimal and behavior-preserving.

- [ ] **Step 2: Implement the clarified pairing split**

Refactor so that:

- branch invocation/result pairing no longer hides inline inside the broad mode-routing body if a narrower local seam helps
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
git add internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go internal/listingkit/task_generation_action_execute_request_handoff_mode_*.go <handoff behavior test file(s)>
git commit -m "refactor: split listingkit action execute handoff mode pairing"
```

---

## Task 3: Keep branch-local seams stable

**Files:**
- Modify only if needed after Task 2:
  - `internal/listingkit/task_generation_action_execute_request_handoff_retry.go`
  - `internal/listingkit/task_generation_action_execute_request_handoff_queue.go`
  - `internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go`
  - `internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go`
  - test file(s)

- [ ] **Step 1: Add/extend tests only if branch-local ownership is still mixed**

If Task 2 leaves obvious branch-local knowledge spread incorrectly, add focused tests to lock the intended split.

- [ ] **Step 2: Keep branch-local seams stable unless justified**

Do not change branch-local seams unless a real ownership improvement is needed.

- [ ] **Step 3: Re-run focused verification**

Run the smallest command set that proves handoff behavior still holds.

- [ ] **Step 4: Commit**

Commit only if a real additional production narrowing is introduced.

---

## Task 4: Lock mode-routing pairing ownership guardrails

**Files:**
- Create: `internal/listingkit/phase31_action_execute_handoff_mode_pairing_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase29_action_execute_handoff_mode_boundary_test.go`
  - handoff behavior test file(s)

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. mode-routing seam routes through the final local pairing seam(s)
2. branch-local invocation and result seams continue to stay outside the pairing owner
3. outward action execute behavior remains intact

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
```

Expected: FAIL until guardrails match the final split.

- [ ] **Step 3: Align the boundary suite with the final pairing split**

Prefer existing AST/token/source helper style and avoid fragile formatting assertions.

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- handoff behavior tests PASS
- handoff mode-pairing boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase29_action_execute_handoff_mode_boundary_test.go internal/listingkit/phase31_action_execute_handoff_mode_pairing_boundary_test.go internal/listingkit/task_generation_action_execute_request_handoff_mode*.go <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff mode pairing boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 31` stays inside action execute handoff mode-routing pairing ownership
2. the plan does not reopen `Phase 30` result-shape / adaptation split
3. the plan does not expand into shared queue/retry helper relocation
4. the plan does not introduce a generic pairing framework
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
