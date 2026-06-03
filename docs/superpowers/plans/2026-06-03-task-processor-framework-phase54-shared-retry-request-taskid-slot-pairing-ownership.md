## Task Processor Framework Phase 54 ListingKit Shared Retry Request Task-ID And Slot Clone Pairing Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying how the retry request slice clone home owns the `TaskIDs / Slots` pairing, while keeping outward clone behavior unchanged.

### Architecture

Keep `Phase 53` intact. Do **not** reopen the queue query clone split, do **not** reopen the retry request aggregate split, do **not** reopen the retry request slice entry split, and do **not** widen into a generic cloning framework or an action execute orchestration redesign.

Instead, focus on the retry request slice pairing hotspot currently centered in:

- [task_generation_retry_request_slice_clone.go](/D:/code-task-processor/internal/listingkit/task_generation_retry_request_slice_clone.go:1)

and clarify:

- whether `TaskIDs` and `Slots` should keep living together in one local slice-clone home
- whether that pairing deserves a narrower local split
- while keeping outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 51` queue query clone split
- reopening `Phase 52` retry request aggregate split
- reopening `Phase 53` retry request slice entry split
- redesigning action execute orchestration
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 53`, the shared retry clone layering is much clearer, but the retry request slice clone home still retains a mixed pairing:

- `TaskIDs` slice clone
- `Slots` slice clone

The next ownership problem is no longer “who owns retry request slice entry.” It is “should one retry request slice home still directly own multiple distinct slice-clone responsibilities.”

### Target Outcome

At the end of `Phase 54`:

- shared retry request `TaskIDs / Slots` pairing ownership is clearer
- current direct consumers remain stable
- current consumer-visible clone behavior remains unchanged
- shared retry request slice clone guardrails lock the clarified split

### Task 1: Lock current retry request slice pairing behavior

**Files:**
- Use the existing behavior home that already covers retry request clone:
  - `internal/listingkit/service_generation_actions_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - retry request slice pairing behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit shared retry request slice pairing behavior"
```

### Task 2: Clarify retry request slice pairing owner

**Files:**
- Modify:
  - `internal/listingkit/task_generation_retry_request_slice_clone.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct `TaskIDs / Slots` pairing if that is now the clearest home
   - or split out one narrower pairing home if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest" -count=1
```

4. Commit:

```bash
git add internal/listingkit/task_generation_retry_request_slice_clone.go internal/listingkit/*retry*request*taskid*slot*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit shared retry request task-id and slot clone pairing ownership"
```

### Task 3: Lock retry request slice pairing ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase54_shared_retry_request_slice_pairing_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - shared retry request slice pairing stays in the final intended local place
   - slice entry home remains separate and stable
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase54_shared_retry_request_slice_pairing_boundary_test.go internal/listingkit/*retry*request*taskid*slot*clone*.go <affected tests>
git commit -m "test: lock listingkit shared retry request slice pairing boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest" -count=1
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
