## Task Processor Framework Phase 52 ListingKit Shared Retry Request Clone Aggregate Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying how the shared retry request clone helper owns its own aggregate responsibilities, while keeping outward clone behavior unchanged.

### Architecture

Keep `Phase 51` intact. Do **not** reopen the queue query clone split that is already stable, do **not** reopen the follow-up read clone splits that are already stable, and do **not** widen into a generic cloning framework or a navigation dispatch redesign.

Instead, focus on the shared retry request clone hotspot currently centered in:

- [task_generation_shared_clone.go](/D:/code-task-processor/internal/listingkit/task_generation_shared_clone.go:1)

and clarify:

- whether `cloneRetryGenerationTasksRequest(...)` should keep direct shallow-copy and slice-clone ownership together
- whether retry request clone deserves a narrower local shape home
- while keeping outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 51` shared queue query clone split
- redesigning action execute orchestration
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 51`, the shared clone helper layering is much clearer, but the retry request clone home still retains a mixed aggregate:

- top-level request shallow copy
- `TaskIDs / Slots` slice clone

The next ownership problem is no longer “who owns queue query clone.” It is “should one retry request clone seam still directly own both shallow-copy and slice-clone responsibilities.”

### Target Outcome

At the end of `Phase 52`:

- shared retry request clone aggregate ownership is clearer
- current direct consumers remain stable
- current consumer-visible clone behavior remains unchanged
- shared retry request clone guardrails lock the clarified split

### Task 1: Lock current shared retry request clone behavior

**Files:**
- Use the existing behavior home that already covers retry request clone:
  - `internal/listingkit/service_generation_actions_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - retry request clone behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit shared retry request clone behavior"
```

### Task 2: Clarify shared retry request clone owner

**Files:**
- Modify:
  - `internal/listingkit/task_generation_shared_clone.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct aggregate ownership if that is now the clearest home
   - or split out one narrower retry-request-clone home if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest" -count=1
```

4. Commit:

```bash
git add internal/listingkit/task_generation_shared_clone.go internal/listingkit/*retry*request*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit shared retry request clone aggregate ownership"
```

### Task 3: Lock shared retry request clone ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase52_shared_retry_request_clone_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - shared retry request clone stays in the final intended local place
   - queue query clone remains separate and stable
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase52_shared_retry_request_clone_boundary_test.go internal/listingkit/*retry*request*clone*.go <affected tests>
git commit -m "test: lock listingkit shared retry request clone boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest" -count=1
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
