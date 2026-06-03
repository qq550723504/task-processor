## Task Processor Framework Phase 25 ListingKit Action Execute Request Handoff Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which request clone and persistence-session shaping semantics belong inside the action execute phase, and which should move into more explicit execute-local handoff seams.

### Architecture

Keep `Phase 24` intact. Do **not** reopen review-navigation queue clone reuse, do **not** redesign outward action execution behavior, and do **not** widen into a multi-consumer rewrite of shared `queue/retry` clone helpers. Instead, isolate the mixed responsibilities currently sitting together in:

- [task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:1)

so that execute-phase code more clearly separates:

- request clone handoff
- branch-local service invocation
- persistence-session input shaping

This is an execute-local ownership move, not a generic request orchestration framework.

### Tech Stack

Go, ListingKit action execute phase, request clone handoff, persistence-session shaping, feature-local seams, source-boundary guardrails

### Out Of Scope For This Slice

- reopening `Phase 23` / `Phase 24` behavior seams
- redesigning shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)`
- broad cleanup across all execute/refresh/projection phases
- generic request-handoff abstraction
- HTTP/bootstrap/runtime changes

---

## Root Cause This Slice Addresses

After `Phase 24`, review-navigation queue clone duplication is gone, but the execute phase still mixes several responsibilities in one method:

- [task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:19)

Today it still directly owns:

1. `retryable` vs default branch choice
2. clone handoff into `RetryTaskGenerationTasks(...)`
3. clone handoff into `GetTaskGenerationQueue(...)`
4. retry/queue result to `GenerationWorkQueue` adaptation
5. persistence-session input shaping

The ownership problem is no longer “who owns queue clone semantics in review navigation.” `Phase 24` solved that. The next problem is “why does execute phase still inline request handoff and persistence-session shaping instead of routing through clearer execute-local seams.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:1)
  - Current execute-phase home
  - Should end the phase as a clearer orchestration seam

- [internal/listingkit/service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1)
  - Current shared home for queue/retry clone helpers
  - Should remain unchanged in outward semantics

- [internal/listingkit/phase10_task_generation_action_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase10_task_generation_action_boundary_test.go:1)
  - Existing action phase ownership guardrails
  - Likely needs alignment once execute-local seams are introduced

### New files this phase may introduce

- `internal/listingkit/task_generation_action_execute_request_handoff.go`
  - Narrow local seam for execute-phase request clone handoff and page/result adaptation

- `internal/listingkit/task_generation_action_execute_persistence.go`
  - Narrow local seam for persistence-session input shaping, if a second split is justified

- `internal/listingkit/phase25_action_execute_boundary_test.go`
  - Guardrail ensuring execute-local handoff ownership stays clear

If a second production file is not actually needed, keep the implementation smaller. Do not force symmetry.

---

## Target Outcome

At the end of `Phase 25`:

- execute-phase request handoff is clearer
- persistence-session shaping no longer hides inside the broad execute body if it can be isolated
- shared clone helpers remain shared
- outward action execution behavior remains unchanged
- execute ownership guardrails lock the clarified split

---

## Task 1: Lock current action execute request handoff behavior

**Files:**
- Modify: the smallest existing test home that already covers execute-phase behavior, likely `internal/listingkit/task_generation_action_execute_test.go` or the closest existing action execute test file

- [ ] **Step 1: Add failing behavior tests for execute request handoff**

Lock the current behavior of:

- `taskGenerationActionExecutePhase.run(...)`

Behavior to lock:

1. `retryable` branch clones `RetryRequest` before calling service
2. default branch clones `QueueQuery` before calling service
3. returned retry/queue pages continue to map into the same `taskGenerationActionExecution` fields
4. `persistenceSession` continues to be built from the corresponding queue source and original `target.QueueQuery`
5. mutating the request received by downstream stubs does not back-propagate into the original target

- [ ] **Step 2: Run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecute.*" -count=1
```

Expected: PASS after tests are aligned to current behavior.

- [ ] **Step 3: Commit**

```bash
git add <execute test file(s)>
git commit -m "test: lock listingkit action execute request handoff behavior"
```

---

## Task 2: Extract execute-local request handoff seam

**Files:**
- Modify: `internal/listingkit/task_generation_action_execute.go`
- Create/modify minimal execute-local seam file(s) only if justified
- Modify tests from Task 1 if needed

- [ ] **Step 1: Decide the minimal execute-local split**

Before editing, determine which parts are:

- common execute-local request handoff logic worth extracting
- execute-branch-local orchestration that should remain in `run(...)`

The move should be minimal and behavior-preserving.

- [ ] **Step 2: Implement the clarified handoff split**

Refactor so that:

- request clone handoff no longer hides inline inside the broad execute body if a narrower execute-local seam helps
- outward branch behavior and returned execution object remain unchanged

Important:

- preserve exact behavior locked in Task 1
- do not move shared clone helpers
- do not widen into refresh/projection/finalize cleanup

- [ ] **Step 3: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecute.*|TestCloneGenerationQueueQuery.*|TestCloneRetryGenerationTasksRequest.*" -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/listingkit/task_generation_action_execute.go internal/listingkit/task_generation_action_execute_*.go <execute test file(s)>
git commit -m "refactor: split listingkit action execute request handoff"
```

---

## Task 3: Isolate persistence-session shaping if still mixed

**Files:**
- Modify only if needed after Task 2:
  - `internal/listingkit/task_generation_action_execute.go`
  - execute-local seam file(s)
  - test file(s)

- [ ] **Step 1: Add/extend tests only if persistence shaping is still mixed**

If Task 2 leaves persistence-session shaping mixed with request handoff, add focused tests to lock its current mapping.

- [ ] **Step 2: Extract persistence-session shaping only if it is justified**

Create a second local seam only if this materially improves ownership clarity without widening the slice.

- [ ] **Step 3: Re-run focused verification**

Run the smallest command set that proves execute behavior still holds.

- [ ] **Step 4: Commit**

Commit only if a real second seam is introduced.

---

## Task 4: Lock execute-phase ownership guardrails

**Files:**
- Create: `internal/listingkit/phase25_action_execute_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase10_task_generation_action_boundary_test.go`
  - execute behavior test file(s)

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. execute phase routes request handoff through the final execute-local seam(s)
2. shared `queue/retry` clone helpers remain outside the execute-local seam owner
3. persistence-session shaping lives in the final intended execute-local home
4. `Phase 24` review-navigation queue clone reuse remains intact

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecute.*|TestTaskGenerationAction.*Boundary" -count=1
```

Expected: FAIL until guardrails match the final split.

- [ ] **Step 3: Align the boundary suite with the final execute split**

Prefer existing AST/token/source helper style and avoid fragile formatting assertions.

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecute.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- execute behavior tests PASS
- execute ownership boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase25_action_execute_boundary_test.go internal/listingkit/phase10_task_generation_action_boundary_test.go internal/listingkit/task_generation_action_execute.go internal/listingkit/task_generation_action_execute_*.go <execute test file(s)>
git commit -m "test: lock listingkit action execute boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 25` stays inside action execute request handoff ownership
2. the plan does not reopen `Phase 24` review-navigation queue clone reuse
3. the plan does not expand into shared queue/retry helper relocation
4. the plan does not introduce a generic request-handoff framework
5. each task has a bounded write set and explicit verification command

This plan passes that check.

## Expected Verification Matrix

During execution, the implementing worker should expect to run at least these checks:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecute.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecute.*|TestCloneGenerationQueueQuery.*|TestCloneRetryGenerationTasksRequest.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecute.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
