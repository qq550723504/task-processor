## Task Processor Framework Phase 37 ListingKit Action Target Clone Aggregate Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which clone-shaping semantics belong inside the aggregate action-target clone owner, and which should remain delegated to narrower nested clone homes.

### Architecture

Keep `Phase 36` intact. Do **not** reopen the shared clone helper home move, do **not** widen into a generic cloning framework, and do **not** use this slice to redesign navigation dispatch or execution flow.

Instead, focus on the aggregate clone owner currently centered in:

- [task_generation_action_target_clone.go](/D:/code-task-processor/internal/listingkit/task_generation_action_target_clone.go:1)

and clarify:

- which parts are truly aggregate action-target shaping
- which parts should stay delegated to nested clone homes
- how to keep outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 35` request-shaping seams
- reopening `Phase 36` shared helper home move
- redesigning navigation dispatch execution flow
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 36`, helper placement is clear, but the aggregate target clone owner still knows too much at once:

- top-level target copy
- filters clone
- queue clone
- retry clone
- expected impact clone
- navigation target clone

The next ownership problem is no longer “where do shared helpers live.” It is “why does one aggregate target-clone seam still own so many nested clone-shaping decisions.”

### Target Outcome

At the end of `Phase 37`:

- action-target clone aggregate ownership is clearer
- nested clone delegation becomes more explicit if justified
- current consumer-visible clone behavior remains unchanged
- aggregate clone guardrails lock the clarified split

### Task 1: Lock current aggregate action-target clone behavior

**Files:**
- Modify the smallest existing test home that already covers clone behavior, likely:
  - `internal/listingkit/service_generation_actions_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - aggregate target clone shape
   - nested pointer/slice defensive clone behavior
   - navigation/action target consumer-visible behavior if needed
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*" -count=1
```

3. Commit:

```bash
git add <clone behavior test file(s)>
git commit -m "test: lock listingkit action target clone aggregate behavior"
```

### Task 2: Clarify aggregate action-target clone owner

**Files:**
- Modify:
  - `internal/listingkit/task_generation_action_target_clone.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep aggregate home, but make nested delegation more explicit
   - or split out one narrow nested shaping seam if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/task_generation_action_target_clone.go internal/listingkit/*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit action target clone aggregate ownership"
```

### Task 3: Lock aggregate clone ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase37_action_target_clone_aggregate_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - aggregate clone home stays in the final intended local place
   - nested clone helpers remain delegated rather than re-inlined
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase37_action_target_clone_aggregate_boundary_test.go internal/listingkit/*clone*.go <affected tests>
git commit -m "test: lock listingkit action target clone aggregate boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*" -count=1
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
