## Task Processor Framework Phase 32 ListingKit Action Execute Handoff Result Normalization Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which retry/queue normalization semantics belong inside the unified handoff result layer, and which should move into narrower normalization-local homes.

### Architecture

Keep `Phase 31` intact. Do **not** reopen the mode-routing / mode-pairing split, do **not** redesign shared `queue/retry` clone helpers, and do **not** widen into a multi-consumer normalization framework. Instead, isolate the mixed responsibilities currently sitting together across:

- [task_generation_action_execute_request_handoff_result_shape.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go:1)
- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)

so that unified handoff result code more clearly separates:

- outward result shape
- retry/queue normalization
- page-to-persistenceQueue mapping

This is a result-layer ownership move, not a generic normalization abstraction.

### Tech Stack

Go, ListingKit action execute handoff result shape, normalization, persistenceQueue mapping, feature-local seams, source-boundary guardrails

### Out Of Scope For This Slice

- reopening `Phase 29` / `Phase 30` / `Phase 31` handoff seams
- redesigning shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)`
- broad cleanup across execute/refresh/projection/finalize phases
- generic normalization abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 31`, mode-routing is clean, but the unified handoff result layer still mirrors retry/queue responsibilities:

- [task_generation_action_execute_request_handoff_result_shape.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go:1)
- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)

Today it still directly owns:

1. retry normalization path
2. queue normalization path
3. retry persistenceQueue derivation path
4. queue persistenceQueue derivation path

The ownership problem is no longer “who owns mode pairing.” `Phase 31` solved that. The next problem is “why does one unified result layer still evolve along retry/queue mirror tracks instead of routing that ownership through a clearer normalization home.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go:1)
  - Current unified result-shape home

- [internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)
  - Current persistenceQueue mapping home

- [internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go:1)
  - Current retry result seam

- [internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go:1)
  - Current queue result seam

- [internal/listingkit/phase30_action_execute_handoff_result_shape_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase30_action_execute_handoff_result_shape_boundary_test.go:1)
  - Existing result-shape boundary guardrail

### New files this phase may introduce

- `internal/listingkit/task_generation_action_execute_request_handoff_result_normalization.go`
  - Narrow local seam for retry/queue normalization, if justified

- `internal/listingkit/phase32_action_execute_handoff_result_normalization_boundary_test.go`
  - Guardrail ensuring result normalization ownership stays clear

If no new production file is truly justified, keep the implementation smaller. Do not force symmetry.

---

## Target Outcome

At the end of `Phase 32`:

- unified handoff result normalization ownership is clearer
- retry/queue mirror responsibilities no longer hide inline inside the broad result-shape owner if a narrower local seam helps
- persistenceQueue mapping remains intact
- outward action execution behavior remains unchanged
- handoff result-normalization guardrails lock the clarified split

---

## Task 1: Lock current unified result normalization behavior

**Files:**
- Modify: the smallest existing test home that already covers handoff behavior, currently `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Add failing behavior tests for unified result normalization**

Lock the current behavior of:

- `taskGenerationActionExecuteRequestHandoffResultShapePhase`
- `taskGenerationActionExecuteRequestHandoffResultAdaptationPhase`

Behavior to lock:

1. retry path still normalizes into the same outward handoff structure
2. queue path still normalizes into the same outward handoff structure
3. retry/queue persistenceQueue derivation remains unchanged
4. `Phase 31` routing and pairing behavior remains intact

- [ ] **Step 2: Run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
```

Expected: PASS after tests are aligned to current behavior.

- [ ] **Step 3: Commit**

```bash
git add <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff result normalization behavior"
```

---

## Task 2: Extract result normalization seam if justified

**Files:**
- Modify:
  - `internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go`
  - `internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go`
- Create/modify minimal normalization seam file(s) only if justified
- Modify tests from Task 1 if needed

- [ ] **Step 1: Decide the minimal normalization split**

Before editing, determine which parts are:

- truly unified outward result-shape ownership that should remain local
- retry/queue normalization logic worth extracting

The move should be minimal and behavior-preserving.

- [ ] **Step 2: Implement the clarified normalization split**

Refactor so that:

- retry/queue mirror responsibilities no longer hide inline inside the broad result-shape owner if a narrower local seam helps
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
git add internal/listingkit/task_generation_action_execute_request_handoff_result*.go <handoff behavior test file(s)>
git commit -m "refactor: split listingkit action execute handoff result normalization"
```

---

## Task 3: Keep persistenceQueue mapping stable

**Files:**
- Modify only if needed after Task 2:
  - `internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go`
  - test file(s)

- [ ] **Step 1: Add/extend tests only if persistenceQueue ownership is still mixed**

If Task 2 leaves obvious persistenceQueue mapping knowledge spread incorrectly, add focused tests to lock the intended split.

- [ ] **Step 2: Keep adaptation seam stable unless justified**

Do not change adaptation responsibilities unless a real ownership improvement is needed.

- [ ] **Step 3: Re-run focused verification**

Run the smallest command set that proves handoff behavior still holds.

- [ ] **Step 4: Commit**

Commit only if a real additional production narrowing is introduced.

---

## Task 4: Lock result normalization ownership guardrails

**Files:**
- Create: `internal/listingkit/phase32_action_execute_handoff_result_normalization_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase30_action_execute_handoff_result_shape_boundary_test.go`
  - handoff behavior test file(s)

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. unified result normalization ownership stays in the final intended local home
2. persistenceQueue mapping continues to stay outside the normalization owner when intended
3. outward action execute behavior remains intact

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
```

Expected: FAIL until guardrails match the final split.

- [ ] **Step 3: Align the boundary suite with the final normalization split**

Prefer existing AST/token/source helper style and avoid fragile formatting assertions.

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- handoff behavior tests PASS
- handoff result-normalization boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase30_action_execute_handoff_result_shape_boundary_test.go internal/listingkit/phase32_action_execute_handoff_result_normalization_boundary_test.go internal/listingkit/task_generation_action_execute_request_handoff_result*.go <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff result normalization boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 32` stays inside action execute handoff result normalization ownership
2. the plan does not reopen `Phase 31` mode-routing / pairing split
3. the plan does not expand into shared queue/retry helper relocation
4. the plan does not introduce a generic normalization framework
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
