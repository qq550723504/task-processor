## Task Processor Framework Phase 55 ListingKit Shared Retry Request Task-ID And Slot Clone Final Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying the final ownership split between retry request `TaskIDs` clone and `Slots` clone, while keeping outward clone behavior unchanged.

### Architecture

Keep `Phase 54` intact. Do **not** reopen the queue query clone split, do **not** reopen the retry request aggregate split, do **not** reopen the retry request slice entry split, do **not** reopen the retry request pairing split, and do **not** widen into a generic cloning framework or an action execute orchestration redesign.

Instead, focus on the retry request final slice-clone hotspot currently centered in:

- [task_generation_retry_request_taskid_slot_clone_pairing.go](/D:/code-task-processor/internal/listingkit/task_generation_retry_request_taskid_slot_clone_pairing.go:1)

and clarify:

- whether `TaskIDs` and `Slots` should keep living together in one final local home
- whether each deserves its own narrower final home
- while keeping outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 51` queue query clone split
- reopening `Phase 52` retry request aggregate split
- reopening `Phase 53` retry request slice entry split
- reopening `Phase 54` retry request pairing split
- redesigning action execute orchestration
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 54`, the shared retry clone layering is much clearer, but the retry request final pairing home still retains a mixed clone responsibility:

- `TaskIDs` slice clone
- `Slots` slice clone

The next ownership problem is no longer “who owns retry request slice entry or pairing.” It is “should one final local home still directly own two distinct slice-clone responsibilities.”

### Target Outcome

At the end of `Phase 55`:

- shared retry request final clone ownership is clearer
- current direct consumers remain stable
- current consumer-visible clone behavior remains unchanged
- shared retry request final guardrails lock the clarified split

### Task 1: Lock current retry request final pairing behavior

**Files:**
- Use the existing behavior home that already covers retry request clone:
  - `internal/listingkit/service_generation_actions_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - retry request final clone behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit shared retry request final clone behavior"
```

### Task 2: Clarify retry request final clone owner

**Files:**
- Modify:
  - `internal/listingkit/task_generation_retry_request_taskid_slot_clone_pairing.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct `TaskIDs / Slots` final pairing if that is now the clearest home
   - or split out narrower final homes if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest" -count=1
```

4. Commit:

```bash
git add internal/listingkit/task_generation_retry_request_taskid_slot_clone_pairing.go internal/listingkit/*retry*request*taskid*clone*.go internal/listingkit/*retry*request*slot*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit shared retry request task-id and slot final ownership"
```

### Task 3: Lock retry request final clone ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase55_shared_retry_request_final_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - shared retry request final clone stays in the final intended local place
   - pairing home remains separate and stable if retained
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase55_shared_retry_request_final_boundary_test.go internal/listingkit/*retry*request*taskid*clone*.go internal/listingkit/*retry*request*slot*clone*.go <affected tests>
git commit -m "test: lock listingkit shared retry request final clone boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest" -count=1
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
