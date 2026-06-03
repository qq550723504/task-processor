## Task Processor Framework Phase 53 ListingKit Shared Retry Request Slice Clone Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying how the retry request shape home owns its slice-clone responsibilities, while keeping outward clone behavior unchanged.

### Architecture

Keep `Phase 52` intact. Do **not** reopen the queue query clone split, do **not** reopen the retry request aggregate split that is already stable, and do **not** widen into a generic cloning framework or an action execute orchestration redesign.

Instead, focus on the retry request slice-clone hotspot currently centered in:

- [task_generation_retry_request_clone_shape.go](/D:/code-task-processor/internal/listingkit/task_generation_retry_request_clone_shape.go:1)

and clarify:

- whether `TaskIDs` and `Slots` slice clone should keep living together in one local shape home
- whether that pairing deserves a narrower local split
- while keeping outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 51` queue query clone split
- reopening `Phase 52` retry request aggregate split
- redesigning action execute orchestration
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 52`, the shared retry clone layering is much clearer, but the retry request shape home still retains a mixed slice-clone pairing:

- `TaskIDs` slice clone
- `Slots` slice clone

The next ownership problem is no longer “who owns retry request aggregate copy.” It is “should one retry request shape seam still directly own multiple distinct slice-clone responsibilities.”

### Target Outcome

At the end of `Phase 53`:

- shared retry request slice clone ownership is clearer
- current direct consumers remain stable
- current consumer-visible clone behavior remains unchanged
- shared retry request slice clone guardrails lock the clarified split

### Task 1: Lock current shared retry request slice clone behavior

**Files:**
- Use the existing behavior home that already covers retry request clone:
  - `internal/listingkit/service_generation_actions_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - retry request slice clone behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit shared retry request slice clone behavior"
```

### Task 2: Clarify shared retry request slice clone owner

**Files:**
- Modify:
  - `internal/listingkit/task_generation_retry_request_clone_shape.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct slice pairing if that is now the clearest home
   - or split out one narrower retry-request-slice home if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest" -count=1
```

4. Commit:

```bash
git add internal/listingkit/task_generation_retry_request_clone_shape.go internal/listingkit/*retry*request*slice*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit shared retry request slice clone ownership"
```

### Task 3: Lock shared retry request slice clone ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase53_shared_retry_request_slice_clone_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - shared retry request slice clone stays in the final intended local place
   - aggregate copy home remains separate and stable
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase53_shared_retry_request_slice_clone_boundary_test.go internal/listingkit/*retry*request*slice*clone*.go <affected tests>
git commit -m "test: lock listingkit shared retry request slice clone boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest" -count=1
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
