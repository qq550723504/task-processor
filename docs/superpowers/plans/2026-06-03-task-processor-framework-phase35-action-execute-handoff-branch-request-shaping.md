## Task Processor Framework Phase 35 ListingKit Action Execute Handoff Branch Request Shaping Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which request shaping semantics belong inside the retry/queue invocation seams, and which should move into narrower branch-local request homes.

### Architecture

Keep `Phase 34` intact. Do **not** reopen the result-dispatch / result-normalization / result-shape split, do **not** redesign shared clone helper definitions, and do **not** widen into a multi-consumer request framework.

Instead, isolate the mixed responsibilities currently sitting together across:

- [task_generation_action_execute_request_handoff_retry.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry.go:1)
- [task_generation_action_execute_request_handoff_queue.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue.go:1)

so that branch invocation more clearly separates:

- branch-local request shaping
- service invocation
- outward action execute behavior stability

### Out Of Scope For This Slice

- reopening `Phase 32` / `Phase 33` / `Phase 34` handoff seams
- moving shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)` definitions
- broad cleanup across execute / refresh / projection / finalize
- generic request-routing abstraction
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 34`, result-side ownership is clean, but branch invocation still mirrors retry/queue thin shells:

- [task_generation_action_execute_request_handoff_retry.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry.go:1)
- [task_generation_action_execute_request_handoff_queue.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue.go:1)

Today they still directly own:

1. branch-specific request shaping via shared clone helper
2. dispatch into the underlying service method

The next ownership problem is no longer “who owns branch-specific result routing.” `Phase 34` solved that. The next problem is “why do two branch invocation seams still hide request shaping inline instead of routing it through a clearer local home.”

### Target Outcome

At the end of `Phase 35`:

- branch request shaping ownership is clearer
- retry/queue invocation seams no longer hide inline clone handoff if a narrower local seam helps
- shared clone helper definitions remain intact
- outward action execute behavior remains unchanged
- branch-request-shaping guardrails lock the clarified split

### Task 1: Lock current branch request shaping behavior

**Files:**
- Modify: the smallest existing test home that already covers execute handoff behavior, currently `internal/listingkit/service_generation_retry_test.go`

1. Add focused behavior tests for:
   - `taskGenerationActionExecuteRequestHandoffRetryPhase`
   - `taskGenerationActionExecuteRequestHandoffQueuePhase`
2. Lock:
   - retry branch still clones `RetryRequest` before invocation
   - queue branch still clones `QueueQuery` before invocation
   - downstream mutation does not leak back into original target
   - outward page behavior remains unchanged
3. Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
```

4. Commit:

```bash
git add <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff branch request shaping behavior"
```

### Task 2: Extract branch-local request shaping seam if justified

**Files:**
- Modify:
  - `internal/listingkit/task_generation_action_execute_request_handoff_retry.go`
  - `internal/listingkit/task_generation_action_execute_request_handoff_queue.go`
- Create minimal local request-shaping seam file(s) only if justified

1. Decide the minimal split between:
   - truly local branch request shaping
   - service invocation that should remain stable
2. Refactor so retry/queue invocation seams no longer hide inline clone handoff if a narrower local seam helps
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestCloneGenerationQueueQuery.*|TestCloneRetryGenerationTasksRequest.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/task_generation_action_execute_request_handoff_* <handoff behavior test file(s)>
git commit -m "refactor: split listingkit action execute handoff branch request shaping"
```

### Task 3: Lock branch request shaping ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase35_action_execute_handoff_branch_request_shaping_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - branch request shaping stays in the final intended local home
   - service invocation remains outside the request-shaping owner when intended
   - outward action execute behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase35_action_execute_handoff_branch_request_shaping_boundary_test.go internal/listingkit/task_generation_action_execute_request_handoff_*.go <handoff behavior test file(s)>
git commit -m "test: lock listingkit action execute handoff branch request shaping boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestCloneGenerationQueueQuery.*|TestCloneRetryGenerationTasksRequest.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
