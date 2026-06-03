## Task Processor Framework Phase 61 ListingKit Action Target Filter Mutation Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying the mutation ownership inside `actionFiltersForKey(...)`, while keeping outward action-target filter behavior unchanged.

### Architecture

Keep `Phase 60` intact. Do **not** reopen the shared retry request clone layering, do **not** reopen queue query clone ownership, do **not** reopen action target impact clone layering, do **not** reopen action target filters clone layering, and do **not** widen into a generic mutation framework or an action execute orchestration redesign.

Instead, focus on the action-target filter mutation hotspot currently centered in:

- [generation_overview.go](/D:/code/task-processor/internal/listingkit/generation_overview.go:290)

and clarify:

- whether `actionFiltersForKey(...)` should keep owning all action-key-specific filter mutation rules directly
- whether it deserves a clearer local mutation split
- while keeping outward behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 51` queue query clone split
- reopening `Phase 52` through `Phase 60` clone layering work
- reopening navigation descriptor clone layering
- redesigning action execute orchestration
- moving helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 60`, the clone layering around action-target helpers is clear enough that the next real hotspot is no longer cloning. The next ownership problem is now inside action-target filter mutation:

- preview capability action specialization
- quality-grade rewriting
- retryability toggles
- execution-quality resets
- action-key specific mutation branching

All of that still lives together directly in one mutation home.

### Target Outcome

At the end of `Phase 61`:

- action target filter mutation ownership is clearer
- current direct consumers remain stable
- current consumer-visible behavior remains unchanged
- mutation guardrails lock the clarified split

### Task 1: Lock current action target filter mutation behavior

**Files:**
- Use the existing behavior home that already covers action target clone/overview behavior:
  - `internal/listingkit/service_generation_actions_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - action-target filter mutation behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget|TestResolveAssetGenerationActionTarget.*" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit action target filter mutation behavior"
```

### Task 2: Clarify action target filter mutation owner

**Files:**
- Modify:
  - `internal/listingkit/generation_overview.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct mutation rules if that is now the clearest home
   - or split out narrower local mutation homes if that materially improves ownership
2. Preserve exact outward behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget|TestResolveAssetGenerationActionTarget.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/generation_overview.go internal/listingkit/*filters*mutation*.go <affected tests>
git commit -m "refactor: clarify listingkit action target filter mutation ownership"
```

### Task 3: Lock action target filter mutation ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase61_action_target_filter_mutation_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - action target filter mutation stays in the intended local place
   - clone homes remain separate and stable
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget|TestResolveAssetGenerationActionTarget.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase61_action_target_filter_mutation_boundary_test.go internal/listingkit/*filters*mutation*.go <affected tests>
git commit -m "test: lock listingkit action target filter mutation boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget|TestResolveAssetGenerationActionTarget.*" -count=1
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget|TestResolveAssetGenerationActionTarget.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
