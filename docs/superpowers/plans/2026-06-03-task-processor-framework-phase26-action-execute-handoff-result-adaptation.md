## Task Processor Framework Phase 26 ListingKit Action Execute Handoff Result-Adaptation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which result-adaptation semantics belong inside the action execute handoff seam, and which should move into a narrower execute-local adaptation seam.

### Architecture

Keep `Phase 25` intact. Do **not** reopen execute top-level orchestration, do **not** redesign outward action execution behavior, and do **not** widen into a multi-consumer rewrite of shared `queue/retry` clone helpers. Instead, isolate the mixed responsibilities currently sitting together in:

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

so that the handoff seam more clearly separates:

- branch-local service invocation
- page/result adaptation into `persistenceQueue`

This is a handoff-local ownership move, not a generic adaptation framework.

### Tech Stack

Go, ListingKit action execute handoff seam, page/result adaptation, feature-local seams, source-boundary guardrails

### Out Of Scope For This Slice

- reopening `Phase 24` / `Phase 25` behavior seams
- redesigning shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)`
- broad cleanup across execute/refresh/projection phases
- generic result-adaptation abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 25`, execute top-level orchestration is already thinner, but the new handoff seam still mixes two responsibilities:

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

Today it still directly owns:

1. `retryable` vs default branch choice
2. service invocation through shared clone helper handoff
3. page/result adaptation into `persistenceQueue`

The ownership problem is no longer “who owns request clone handoff in execute phase.” `Phase 25` solved that. The next problem is “why does the handoff seam still inline result adaptation instead of routing through a clearer local adaptation home.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)
  - Current handoff seam
  - Should end the phase either thinner or explicitly split

- [internal/listingkit/task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:1)
  - Current execute top-level orchestration
  - Should stay unchanged in outward behavior

- [internal/listingkit/phase25_action_execute_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase25_action_execute_boundary_test.go:1)
  - Existing execute-phase ownership guardrail
  - Likely needs alignment once result-adaptation ownership is clarified

### New files this phase may introduce

- `internal/listingkit/task_generation_action_execute_result_adaptation.go`
  - Narrow local seam for page/result -> `persistenceQueue` adaptation if justified

- `internal/listingkit/phase26_action_execute_handoff_boundary_test.go`
  - Guardrail ensuring invocation vs result-adaptation ownership stays clear

If a new production file is not actually justified, keep the implementation smaller. Do not force symmetry.

---

## Target Outcome

At the end of `Phase 26`:

- handoff seam invocation vs result-adaptation ownership is clearer
- outward action execution behavior remains unchanged
- shared clone helpers remain shared
- execute top-level orchestration remains stable
- handoff ownership guardrails lock the clarified split

---

## Task 1: Lock current handoff result-adaptation behavior

**Files:**
- Modify: the smallest existing execute/handoff behavior test home, likely `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Add failing behavior tests for handoff result adaptation**

Lock the current behavior of:

- `taskGenerationActionExecuteRequestHandoffPhase.run(...)`

Behavior to lock:

1. `retryable` branch returns `retryPage` and derives `persistenceQueue` from `generationWorkQueueFromRetryPage(...)`
2. default branch returns `queuePage` and derives `persistenceQueue` from `generationWorkQueueFromPage(...)`
3. `persistenceQueue` stays field-for-field aligned with the page-derived queue for each branch
4. adaptation result does not accidentally alias the page pointers in ways current behavior forbids

- [ ] **Step 2: Run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
```

Expected: PASS after tests are aligned to current behavior.

- [ ] **Step 3: Commit**

```bash
git add <execute/handoff test file(s)>
git commit -m "test: lock listingkit action execute handoff adaptation behavior"
```

---

## Task 2: Extract a narrower result-adaptation seam if justified

**Files:**
- Modify: `internal/listingkit/task_generation_action_execute_request_handoff.go`
- Create/modify a minimal adaptation seam file only if justified
- Modify tests from Task 1 if needed

- [ ] **Step 1: Decide the minimal handoff-local split**

Before editing, determine which parts are:

- branch-local invocation that should remain in handoff seam
- result adaptation logic worth extracting into a narrower local home

The move should be minimal and behavior-preserving.

- [ ] **Step 2: Implement the clarified split**

Refactor so that:

- service invocation stays with the handoff seam
- page/result adaptation no longer hides inline there if a narrower local seam helps

Important:

- preserve exact behavior locked in Task 1
- do not move shared clone helpers
- do not widen back into execute top-level or refresh/projection

- [ ] **Step 3: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationActionExecute.*" -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/listingkit/task_generation_action_execute_request_handoff.go internal/listingkit/task_generation_action_execute_result_adaptation.go <execute/handoff test file(s)>
git commit -m "refactor: split listingkit action execute handoff adaptation"
```

---

## Task 3: Lock handoff ownership guardrails

**Files:**
- Create: `internal/listingkit/phase26_action_execute_handoff_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase25_action_execute_boundary_test.go`
  - behavior test file(s)

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. handoff seam keeps branch-local invocation
2. result adaptation lives in the final intended local home
3. shared clone helpers remain outside the adaptation home
4. `Phase 25` execute top-level / handoff split remains intact

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
```

Expected: FAIL until guardrails match the final split.

- [ ] **Step 3: Align the boundary suite with the final handoff split**

Prefer existing AST/token/source helper style and avoid fragile formatting assertions.

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- handoff behavior tests PASS
- handoff ownership boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase26_action_execute_handoff_boundary_test.go internal/listingkit/phase25_action_execute_boundary_test.go internal/listingkit/task_generation_action_execute_request_handoff.go internal/listingkit/task_generation_action_execute_result_adaptation.go <execute/handoff test file(s)>
git commit -m "test: lock listingkit action execute handoff boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 26` stays inside action execute handoff result-adaptation ownership
2. the plan does not reopen `Phase 25` execute top-level / handoff split
3. the plan does not expand into shared queue/retry helper relocation
4. the plan does not introduce a generic result-adaptation framework
5. each task has a bounded write set and explicit verification command

This plan passes that check.

## Expected Verification Matrix

During execution, the implementing worker should expect to run at least these checks:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationActionExecute.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
