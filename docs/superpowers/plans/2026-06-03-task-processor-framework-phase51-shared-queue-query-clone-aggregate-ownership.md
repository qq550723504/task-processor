## Task Processor Framework Phase 51 ListingKit Shared Queue Query Clone Aggregate Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying how the shared queue query clone helper owns its own aggregate responsibilities separately from adjacent shared clone helpers, while keeping outward clone behavior unchanged.

### Architecture

Keep `Phase 50` intact. Do **not** reopen the shared helper home move from `Phase 36`, do **not** reopen the follow-up read clone splits that are already stable, and do **not** widen into a generic cloning framework or a navigation dispatch redesign.

Instead, focus on the shared queue query clone hotspot currently centered in:

- [task_generation_shared_clone.go](/D:/code-task-processor/internal/listingkit/task_generation_shared_clone.go:1)

and clarify:

- whether `cloneGenerationQueueQuery(...)` should keep living as a direct sibling of retry request clone in the same shared file
- whether queue query clone deserves a narrower local aggregate split or local home
- while keeping outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 50` follow-up read item clone aggregate split
- redesigning navigation dispatch execution flow
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 50`, the follow-up read chain is much clearer, but the shared helper home still retains a mixed aggregate:

- queue query clone
- retry request clone

The next ownership problem is no longer “who owns follow-up read clone.” It is “should one shared clone home still directly own multiple distinct clone aggregates with different consumer surfaces.”

### Target Outcome

At the end of `Phase 51`:

- shared queue query clone aggregate ownership is clearer
- retry request clone remains stable
- current consumer-visible clone behavior remains unchanged
- shared queue query clone guardrails lock the clarified split

### Task 1: Lock current shared queue query clone behavior

**Files:**
- Use the existing behavior home that already covers queue query clone:
  - `internal/listingkit/service_generation_actions_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - queue query clone behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit shared queue query clone behavior"
```

### Task 2: Clarify shared queue query clone owner

**Files:**
- Modify:
  - `internal/listingkit/task_generation_shared_clone.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct aggregate ownership if that is now the clearest home
   - or split out one narrower queue-query-clone home if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/task_generation_shared_clone.go internal/listingkit/*queue*query*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit shared queue query clone aggregate ownership"
```

### Task 3: Lock shared queue query clone ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase51_shared_queue_query_clone_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - shared queue query clone stays in the final intended local place
   - retry request clone remains separate and stable
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase51_shared_queue_query_clone_boundary_test.go internal/listingkit/*queue*query*clone*.go <affected tests>
git commit -m "test: lock listingkit shared queue query clone boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
