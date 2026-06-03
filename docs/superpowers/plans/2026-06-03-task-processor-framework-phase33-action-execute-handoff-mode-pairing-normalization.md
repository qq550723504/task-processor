## Task Processor Framework Phase 33 ListingKit Action Execute Handoff Mode-Pairing Normalization Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which retry/queue mirror orchestration semantics belong inside the handoff mode-pairing seam, and which should move into narrower pairing-local homes.

### Architecture

Keep `Phase 32` intact. Do **not** reopen the result-normalization / result-shape / adaptation split, do **not** redesign shared `queue/retry` clone helpers, and do **not** widen into a multi-consumer mirror-normalization framework. Instead, isolate the mixed responsibilities currently sitting together in:

- [task_generation_action_execute_request_handoff_mode_pairing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go:1)

so that mode-pairing code more clearly separates:

- retry/queue mirror orchestration
- branch invocation/result dispatch
- outward action execute behavior stability

This is a pairing-local ownership move, not a generic mirror abstraction.

### Tech Stack

Go, ListingKit action execute handoff mode pairing, retry/queue mirror orchestration, feature-local seams, source-boundary guardrails

### Out Of Scope For This Slice

- reopening `Phase 30` / `Phase 31` / `Phase 32` handoff seams
- redesigning shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)`
- broad cleanup across execute/refresh/projection/finalize phases
- generic mirror-normalization abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 32`, result normalization is clean, but the mode-pairing seam still mirrors retry/queue orchestration:

- [task_generation_action_execute_request_handoff_mode_pairing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go:1)

Today it still directly owns:

1. retry branch orchestration
2. queue branch orchestration
3. retry invocation/result dispatch ordering
4. queue invocation/result dispatch ordering

The ownership problem is no longer “who owns result normalization.” `Phase 32` solved that. The next problem is “why does one mode-pairing seam still evolve along retry/queue mirror tracks instead of routing that ownership through a clearer local home.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go:1)
  - Current mode-pairing home

- [internal/listingkit/task_generation_action_execute_request_handoff_retry.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry.go:1)
  - Current retry invocation home

- [internal/listingkit/task_generation_action_execute_request_handoff_queue.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue.go:1)
  - Current queue invocation home

- [internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go:1)
  - Current retry result home

- [internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go:1)
  - Current queue result home

- [internal/listingkit/phase31_action_execute_handoff_mode_pairing_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase31_action_execute_handoff_mode_pairing_boundary_test.go:1)
  - Existing mode-pairing boundary guardrail

### New files this phase may introduce

- `internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing_normalization.go`
  - Narrow local seam for retry/queue mirror orchestration, if justified

- `internal/listingkit/phase33_action_execute_handoff_mode_pairing_normalization_boundary_test.go`
  - Guardrail ensuring mode-pairing normalization ownership stays clear

If no new production file is truly justified, keep the implementation smaller. Do not force symmetry.

---

## Target Outcome

At the end of `Phase 33`:

- mode-pairing mirror orchestration ownership is clearer
- retry/queue mirror orchestration no longer hides inline inside the broad pairing seam if a narrower local seam helps
- branch-local invocation/result seams remain intact
- outward action execution behavior remains unchanged
- handoff mode-pairing normalization guardrails lock the clarified split

---

## Task 1: Lock current mode-pairing mirror behavior

**Files:**
- Modify: the smallest existing test home that already covers handoff behavior, currently `internal/listingkit/service_generation_retry_test.go`

- [ ] **Step 1: Add failing behavior tests for mode-pairing mirror orchestration**

Lock the current behavior of:

- `taskGenerationActionExecuteRequestHandoffModePairingPhase`

Behavior to lock:

1. retry path still dispatches retry invocation then retry result seam in the same order
2. queue path still dispatches queue invocation then queue result seam in the same order
3. outward `retryPage / queuePage / persistenceQueue` behavior remains unchanged
4. `Phase 32` result normalization behavior remains intact

- [ ] **Step 2: Run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
```

Expected: PASS after tests are aligned to current behavior.

- [ ] **Step 3: Commit**

```bash
git add <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff mode pairing normalization behavior"
```

---

## Task 2: Extract pairing-local mirror seam if justified

**Files:**
- Modify: `internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go`
- Create/modify minimal local mirror seam file(s) only if justified
- Modify tests from Task 1 if needed

- [ ] **Step 1: Decide the minimal mirror split**

Before editing, determine which parts are:

- truly local retry/queue mirror orchestration worth extracting
- branch invocation/result dispatch that should remain stable

The move should be minimal and behavior-preserving.

- [ ] **Step 2: Implement the clarified mirror split**

Refactor so that:

- retry/queue mirror orchestration no longer hides inline inside the broad pairing seam if a narrower local seam helps
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
git add internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing*.go <handoff behavior test file(s)>
git commit -m "refactor: split listingkit action execute handoff mode pairing normalization"
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

## Task 4: Lock mode-pairing normalization ownership guardrails

**Files:**
- Create: `internal/listingkit/phase33_action_execute_handoff_mode_pairing_normalization_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase31_action_execute_handoff_mode_pairing_boundary_test.go`
  - handoff behavior test file(s)

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. mode-pairing seam routes through the final local mirror-normalization seam(s)
2. branch-local invocation and result seams continue to stay outside the mirror owner
3. outward action execute behavior remains intact

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
```

Expected: FAIL until guardrails match the final split.

- [ ] **Step 3: Align the boundary suite with the final mirror split**

Prefer existing AST/token/source helper style and avoid fragile formatting assertions.

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- handoff behavior tests PASS
- handoff mode-pairing normalization boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase31_action_execute_handoff_mode_pairing_boundary_test.go internal/listingkit/phase33_action_execute_handoff_mode_pairing_normalization_boundary_test.go internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing*.go <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff mode pairing normalization boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 33` stays inside action execute handoff mode-pairing normalization ownership
2. the plan does not reopen `Phase 32` result-normalization / result-shape / adaptation split
3. the plan does not expand into shared queue/retry helper relocation
4. the plan does not introduce a generic mirror-normalization framework
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
