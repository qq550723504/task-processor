## Task Processor Framework Phase 30 ListingKit Action Execute Handoff Result-Shape / Adaptation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which unified handoff result-shape semantics belong inside the action execute handoff DTO, and which should move into narrower adaptation-local homes.

### Architecture

Keep `Phase 29` intact. Do **not** reopen the handoff entry / mode-routing split, do **not** redesign shared `queue/retry` clone helpers, and do **not** widen into a multi-consumer result-shape framework. Instead, isolate the mixed responsibilities currently sitting together across:

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)
- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)

so that handoff result ownership more clearly separates:

- outward DTO shape
- page-to-persistenceQueue adaptation
- branch-specific page knowledge

This is a handoff-result ownership move, not a generic DTO redesign.

### Tech Stack

Go, ListingKit action execute handoff result DTO, result adaptation, persistenceQueue shaping, feature-local seams, source-boundary guardrails

### Out Of Scope For This Slice

- reopening `Phase 27` / `Phase 28` / `Phase 29` handoff routing seams
- redesigning shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)`
- broad cleanup across execute/refresh/projection/finalize phases
- generic result-shape abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 29`, top-level handoff entry is clean, but the handoff result layer still mixes several responsibilities:

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:9)
- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)

Today it still directly owns:

1. `retryPage` shape
2. `queuePage` shape
3. shared `persistenceQueue` shape
4. unified outward handoff DTO construction

The ownership problem is no longer “who owns mode routing.” `Phase 29` solved that. The next problem is “why does one unified handoff result layer still know both page variants plus shared persistenceQueue derivation instead of routing that ownership through a clearer result-shape home.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)
  - Current handoff DTO home

- [internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)
  - Current adaptation home

- [internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go:1)
  - Current retry result-routing home

- [internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go:1)
  - Current queue result-routing home

- [internal/listingkit/phase26_action_execute_handoff_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase26_action_execute_handoff_boundary_test.go:1)
  - Existing adaptation boundary guardrail

- [internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go:1)
  - Existing result-routing guardrail

### New files this phase may introduce

- `internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go`
  - Narrow local seam for unified handoff result-shape ownership, if justified

- `internal/listingkit/phase30_action_execute_handoff_result_shape_boundary_test.go`
  - Guardrail ensuring unified handoff result-shape ownership stays clear

If no new production file is truly justified, keep the implementation smaller. Do not force symmetry.

---

## Target Outcome

At the end of `Phase 30`:

- unified handoff result-shape ownership is clearer
- adaptation no longer implicitly owns more outward DTO shape than it needs to
- branch-specific result seams remain intact
- outward action execution behavior remains unchanged
- handoff result-shape guardrails lock the clarified split

---

## Task 1: Lock current handoff result-shape behavior

**Files:**
- Modify: the smallest existing test home that already covers handoff behavior, currently `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Add failing behavior tests for unified handoff result shape**

Lock the current behavior of:

- `taskGenerationActionExecuteRequestHandoffPhase.run(...)`
- `taskGenerationActionExecuteRequestHandoffResultAdaptationPhase`

Behavior to lock:

1. retry path still surfaces `retryPage` and derived `persistenceQueue`
2. queue path still surfaces `queuePage` and derived `persistenceQueue`
3. unified outward handoff object shape remains unchanged
4. `Phase 27/28/29` routing and seam ownership remain intact

- [ ] **Step 2: Run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
```

Expected: PASS after tests are aligned to current behavior.

- [ ] **Step 3: Commit**

```bash
git add <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff result shape behavior"
```

---

## Task 2: Clarify result-shape ownership if justified

**Files:**
- Modify:
  - `internal/listingkit/task_generation_action_execute_request_handoff.go`
  - `internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go`
- Create/modify minimal result-shape seam file(s) only if justified
- Modify tests from Task 1 if needed

- [ ] **Step 1: Decide the minimal result-shape split**

Before editing, determine which parts are:

- truly unified outward handoff DTO ownership
- adaptation-local shape knowledge that should remain local

The move should be minimal and behavior-preserving.

- [ ] **Step 2: Implement the clarified result-shape split**

Refactor so that:

- adaptation no longer implicitly owns broader outward DTO shape than necessary if a narrower local result-shape home helps
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
git commit -m "refactor: clarify listingkit action execute handoff result shape ownership"
```

---

## Task 3: Keep branch-specific result seams stable

**Files:**
- Modify only if needed after Task 2:
  - `internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go`
  - `internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go`
  - test file(s)

- [ ] **Step 1: Add/extend tests only if branch-specific result ownership is still mixed**

If Task 2 leaves obvious branch-specific shape knowledge spread incorrectly, add focused tests to lock the intended split.

- [ ] **Step 2: Narrow branch-specific result seams only if justified**

Keep retry/queue result seams stable unless a real ownership improvement is needed.

- [ ] **Step 3: Re-run focused verification**

Run the smallest command set that proves handoff behavior still holds.

- [ ] **Step 4: Commit**

Commit only if a real additional production narrowing is introduced.

---

## Task 4: Lock handoff result-shape ownership guardrails

**Files:**
- Create: `internal/listingkit/phase30_action_execute_handoff_result_shape_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase26_action_execute_handoff_boundary_test.go`
  - `internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go`
  - handoff behavior test file(s)

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. unified handoff result-shape ownership stays in the final intended local home
2. adaptation continues to own page-to-persistenceQueue mapping only
3. branch-specific result seams continue to stay outside the unified shape owner
4. outward action execute behavior remains intact

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
```

Expected: FAIL until guardrails match the final split.

- [ ] **Step 3: Align the boundary suite with the final result-shape split**

Prefer existing AST/token/source helper style and avoid fragile formatting assertions.

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- handoff behavior tests PASS
- handoff result-shape boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase26_action_execute_handoff_boundary_test.go internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go internal/listingkit/phase30_action_execute_handoff_result_shape_boundary_test.go internal/listingkit/task_generation_action_execute_request_handoff*.go <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff result shape boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 30` stays inside action execute handoff result-shape / adaptation ownership
2. the plan does not reopen `Phase 29` handoff entry / mode-routing split
3. the plan does not expand into shared queue/retry helper relocation
4. the plan does not introduce a generic result-shape framework
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
