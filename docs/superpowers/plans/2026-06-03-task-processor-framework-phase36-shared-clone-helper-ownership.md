## Task Processor Framework Phase 36 ListingKit Shared Queue/Retry Clone Helper Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying where shared `queue/retry` clone helper semantics should live, now that action execute handoff and navigation-related consumers both depend on them.

### Architecture

Keep `Phase 35` intact. Do **not** reopen the local request-shaping / result-dispatch splits, do **not** widen into a generic cloning framework, and do **not** use this slice to redesign navigation or service execution flows.

Instead, focus on the shared clone helper home currently centered in:

- [service_generation_actions.go](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:1)

and clarify:

- which clone semantics are truly shared
- which consumers should continue to call the shared home
- how to keep shared helper ownership explicit without broadening into unrelated files

### Out Of Scope For This Slice

- reopening `Phase 32` / `Phase 33` / `Phase 34` / `Phase 35` local seams
- redesigning navigation dispatch execution or action projection
- moving non-clone helpers out of `service_generation_actions.go`
- introducing a generic cloning utility package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 35`, branch-local request shaping is clean, but shared cloning semantics still sit in a broad helper home:

- `cloneGenerationQueueQuery(...)`
- `cloneRetryGenerationTasksRequest(...)`

These are now shared by multiple feature paths, which makes the current home a real ownership seam rather than a harmless leftover.

### Target Outcome

At the end of `Phase 36`:

- shared queue/retry clone helper ownership is clearer
- current consumers keep the same outward behavior
- clone helper home no longer feels incidental or misplaced
- clone-helper ownership guardrails lock the clarified split

### Task 1: Lock current shared clone helper behavior

**Files:**
- Modify the smallest existing test home that already covers clone behavior, likely:
  - `internal/listingkit/service_generation_actions_test.go`
  - and only the minimum additional consumer test home if needed

1. Add focused tests that lock:
   - `cloneGenerationQueueQuery(...)`
   - `cloneRetryGenerationTasksRequest(...)`
   - current consumer-visible behavior for action execute / navigation callers if needed
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*" -count=1
```

3. Commit:

```bash
git add <clone helper behavior test file(s)>
git commit -m "test: lock listingkit shared clone helper behavior"
```

### Task 2: Clarify shared clone helper home

**Files:**
- Modify:
  - `internal/listingkit/service_generation_actions.go`
- Create or move into a narrower shared local home only if justified
- Update only the minimum direct consumers if needed

1. Decide the minimal ownership move:
   - either keep helpers where they are but clarify the shared seam explicitly
   - or move them into a narrower feature-local shared home if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*|TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/service_generation_actions.go internal/listingkit/*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit shared clone helper ownership"
```

### Task 3: Lock shared clone helper ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase36_shared_clone_helper_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - shared clone helper home stays in the final intended local place
   - direct consumers keep calling the shared seam rather than redefining clone logic
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase36_shared_clone_helper_boundary_test.go internal/listingkit/*clone*.go <affected tests>
git commit -m "test: lock listingkit shared clone helper boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*|TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
