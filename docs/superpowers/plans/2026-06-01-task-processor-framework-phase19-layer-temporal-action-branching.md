# Task Processor Framework Phase 19 ListingKit Layer-Temporal Action Branching Ownership Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the remaining ownership hotspot in ListingKit action execution by splitting `executeLayerTemporalAction(...)` into explicit feature-local temporal action seams for branching, workflow start input assembly, and queue-only outward result shaping.

**Architecture:** Keep `executeLayerTemporalAction(...)` as the outer temporal bypass entry, but stop letting it inline every decision. Reuse the same bounded-seam pattern established in `Phase 18`: extract narrow ListingKit-local helpers/phases for standard-product temporal handling, platform-adaptation temporal handling, and shared queue-only temporal result shaping. Do **not** invent a generic temporal framework and do **not** reopen the already-stable entry / persist / refresh / projection / finalize seams.

**Tech Stack:** Go, ListingKit task generation service, standard-product temporal workflow client, platform-adaptation temporal workflow client, generation action result/audit shaping, source-boundary guardrails

**Out of Scope For This Slice:**

- reopening `ExecuteTaskGenerationAction(...)` service-entry seams from `Phase 18`
- redesigning non-temporal action business behavior
- changing generation navigation dispatch behavior
- expanding into HTTP/bootstrap/runtime changes
- inventing a repo-wide generic temporal action framework
- changing current `queue_only` outward semantics or existing audit semantics

---

## Root Cause This Slice Addresses

After `Phase 18`, the local action path below `ExecuteTaskGenerationAction(...)` is explicit and stable, but the layer-temporal bypass still remains a mixed-responsibility block:

- [internal/listingkit/task_generation_service.go:220](/D:/code/task-processor/internal/listingkit/task_generation_service.go:220)

Today `executeLayerTemporalAction(...)` still jointly owns:

1. action-key branching
2. workflow client enablement / config checks
3. platform resolution for platform-adaptation starts
4. per-action temporal start-input assembly
5. outward `GenerationActionExecutionResult` construction
6. `layer_temporal` audit shaping

The main problem is not just duplicated literals. The real ownership pressure is that one helper still decides both **whether** a temporal branch can run and **how** it shapes the queue-only outward response for that branch.

---

## File Structure

### Existing files that remain important

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:220)
  - Keeps `ExecuteTaskGenerationAction(...)`
  - Keeps the outer `executeLayerTemporalAction(...)` entry
  - Should end the phase as a thin router, not the full temporal-branch implementation

- [internal/listingkit/service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1)
  - Already owns `resolveLayerTemporalPlatform(...)`
  - May remain the home of platform-resolution helpers if no better local seam is needed

- [internal/listingkit/service_generation_actions_test.go](/D:/code/task-processor/internal/listingkit/service_generation_actions_test.go:470)
  - Already covers the current temporal action behavior
  - Should receive the behavior tests for the new temporal seams

### New files this phase should introduce

- `internal/listingkit/task_generation_action_temporal_result.go`
  - Shared queue-only outward result/audit shaping for layer-temporal actions

- `internal/listingkit/task_generation_action_temporal_standard.go`
  - Standard-product temporal branch ownership
  - Client/config checks + start-input assembly + delegation to shared result shaping

- `internal/listingkit/task_generation_action_temporal_platform.go`
  - Platform-adaptation temporal branch ownership
  - Client/config checks + platform resolution + start-input assembly + delegation to shared result shaping

- `internal/listingkit/phase19_action_layer_temporal_boundary_test.go`
  - Ownership guardrail for the split

This keeps the split narrow and responsibility-based instead of creating a larger “temporal action manager” abstraction.

---

## Target Outcome

At the end of `Phase 19`:

- `executeLayerTemporalAction(...)` remains the outer temporal short-circuit entry
- standard-product temporal behavior flows through its own explicit ListingKit-local seam
- platform-adaptation temporal behavior flows through its own explicit ListingKit-local seam
- queue-only outward result/audit shaping flows through its own explicit local seam
- current `layer_temporal` audit semantics and `queue_only` outward behavior remain unchanged
- boundary tests lock the branch ordering and prevent the mixed-responsibility block from regrowing inside `task_generation_service.go`

---

## Task 1: Extract shared queue-only temporal result shaping seam

**Files:**
- Create: `internal/listingkit/task_generation_action_temporal_result.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Test: `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Write the failing result-shaping tests**

Add focused coverage that locks:

1. standard-product temporal action still returns `InteractionMode: "queue_only"`
2. platform-adaptation temporal action still returns `InteractionMode: "queue_only"`
3. audit still reports:
   - `RequestedActionKey`
   - `ResolvedActionKey`
   - `ResolutionSource: "layer_temporal"`
   - `ExecutionPath: "queue_only"`
4. platform-adaptation result still carries `ResolvedTarget.QueueQuery.Platform`

Suggested seam shape:

```go
type taskGenerationActionTemporalResultPhase struct{}

func buildTaskGenerationActionTemporalResultPhase() *taskGenerationActionTemporalResultPhase

func (p *taskGenerationActionTemporalResultPhase) run(
	actionKey string,
	responseMode string,
	queueQuery *GenerationQueueQuery,
) *GenerationActionExecutionResult
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestExecuteTaskGenerationActionStarts(StandardProductTemporalWorkflow|PlatformAdaptTemporalWorkflow)" -count=1
```

Expected: FAIL once the tests are updated to assert the new seam behavior before implementation exists.

- [ ] **Step 3: Add the shared queue-only result seam**

Create `task_generation_action_temporal_result.go` so the seam owns:

- `queue_only` outward result construction
- `ResolvedTarget` shaping for temporal actions
- `layer_temporal` audit construction
- response-mode normalization

Important:

- keep the returned outward shape identical to current behavior
- do not place workflow start logic here
- do not move action-key routing here
- keep platform query optional so standard-product temporal can still reuse the seam

- [ ] **Step 4: Route the current temporal branches through the result seam**

Update [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:220) so both temporal branches stop inlining the outward result/audit assembly.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestExecuteTaskGenerationActionStarts(StandardProductTemporalWorkflow|PlatformAdaptTemporalWorkflow)" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_action_temporal_result.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_actions_test.go
git commit -m "refactor: extract listingkit temporal action result seam"
```

---

## Task 2: Extract standard-product temporal branch seam

**Files:**
- Create: `internal/listingkit/task_generation_action_temporal_standard.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Test: `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Write the failing standard-branch tests**

Add focused coverage that locks:

1. standard-product temporal still checks `standardWorkflow()` enablement and nil client
2. disabled/unconfigured standard workflow still returns the same error
3. successful standard branch still calls `StartStandardProduct(...)` with:
   - trimmed `TaskID`
   - non-zero `RequestedAt`
4. standard branch still delegates outward response shaping to the shared temporal result seam

Suggested seam shape:

```go
type taskGenerationActionTemporalStandardPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationActionTemporalStandardPhase(service *taskGenerationService) *taskGenerationActionTemporalStandardPhase

func (p *taskGenerationActionTemporalStandardPhase) run(
	ctx context.Context,
	taskID string,
	req *ExecuteGenerationActionRequest,
) (*GenerationActionExecutionResult, error)
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestExecuteTaskGenerationActionStartsStandardProductTemporalWorkflow|TestTaskGenerationLayerTemporalStandard.*" -count=1
```

Expected: FAIL until the standard seam exists and the tests are aligned.

- [ ] **Step 3: Add the standard-product temporal seam**

Create `task_generation_action_temporal_standard.go` so the seam owns:

- `standardWorkflow()` enablement / nil-client checks
- `StartStandardProduct(...)` start-input assembly
- delegation to the shared temporal result seam

Important:

- preserve the exact unconfigured error text
- preserve `RequestedAt: time.Now().UTC()`
- trim `taskID` exactly as today
- do not add platform query shaping here

- [ ] **Step 4: Route the standard branch through the new seam**

Update [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:223) so the standard-product branch becomes one seam handoff.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestExecuteTaskGenerationActionStartsStandardProductTemporalWorkflow|TestTaskGenerationLayerTemporalStandard.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_action_temporal_standard.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_actions_test.go
git commit -m "refactor: extract listingkit standard temporal branch seam"
```

---

## Task 3: Extract platform-adaptation temporal branch seam

**Files:**
- Create: `internal/listingkit/task_generation_action_temporal_platform.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify if needed: `internal/listingkit/service_generation_actions.go`
- Test: `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Write the failing platform-branch tests**

Add focused coverage that locks:

1. platform-adaptation temporal still checks `platformAdaptWorkflow()` enablement and nil client
2. disabled/unconfigured platform-adaptation workflow still returns the same error
3. platform resolution still defaults and normalizes through `resolveLayerTemporalPlatform(...)`
4. successful platform branch still calls `StartPlatformAdaptation(...)` with:
   - trimmed `TaskID`
   - resolved `Platform`
   - non-zero `RequestedAt`
5. platform branch still delegates outward response shaping to the shared temporal result seam with the resolved queue query

Suggested seam shape:

```go
type taskGenerationActionTemporalPlatformPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationActionTemporalPlatformPhase(service *taskGenerationService) *taskGenerationActionTemporalPlatformPhase

func (p *taskGenerationActionTemporalPlatformPhase) run(
	ctx context.Context,
	taskID string,
	req *ExecuteGenerationActionRequest,
) (*GenerationActionExecutionResult, error)
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestExecuteTaskGenerationActionStartsPlatformAdaptTemporalWorkflow|TestTaskGenerationLayerTemporalPlatform.*" -count=1
```

Expected: FAIL until the platform seam exists and the tests are aligned.

- [ ] **Step 3: Add the platform-adaptation temporal seam**

Create `task_generation_action_temporal_platform.go` so the seam owns:

- `platformAdaptWorkflow()` enablement / nil-client checks
- platform resolution via `resolveLayerTemporalPlatform(...)`
- `StartPlatformAdaptation(...)` start-input assembly
- delegation to the shared temporal result seam with a resolved queue query

Important:

- preserve the exact unconfigured error text
- preserve current `"shein"` defaulting behavior from `resolveLayerTemporalPlatform(...)`
- keep platform normalization behavior in one place
- do not duplicate platform resolution logic inline if an existing helper can be reused cleanly

- [ ] **Step 4: Route the platform-adaptation branch through the new seam**

Update [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:250) so the platform branch becomes one seam handoff.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestExecuteTaskGenerationActionStartsPlatformAdaptTemporalWorkflow|TestTaskGenerationLayerTemporalPlatform.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_action_temporal_platform.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_actions.go internal/listingkit/service_generation_actions_test.go
git commit -m "refactor: extract listingkit platform temporal branch seam"
```

---

## Task 4: Thin the temporal bypass entry and lock ownership guardrails

**Files:**
- Modify: `internal/listingkit/task_generation_service.go`
- Create: `internal/listingkit/phase19_action_layer_temporal_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/service_generation_actions_test.go`
  - `internal/listingkit/phase18_action_service_entry_boundary_test.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. `ExecuteTaskGenerationAction(...)` still checks `executeLayerTemporalAction(...)` before the local action path
2. `executeLayerTemporalAction(...)` only does:
   - requested action-key routing
   - delegation to standard/platform temporal seams
   - default `handled=false` fallback
3. `task_generation_service.go` no longer directly owns:
   - standard temporal start-input assembly
   - platform temporal start-input assembly
   - queue-only outward temporal result shaping
4. the shared temporal result seam continues to be the only home of `layer_temporal` outward result/audit shaping

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationLayerTemporal.*Boundary|TestTaskGenerationActionPhaseOwnershipBoundary" -count=1
```

Expected: FAIL until the guardrails are aligned with the new split.

- [ ] **Step 3: Thin `executeLayerTemporalAction(...)` into a router**

Update [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:220) so the helper now mainly coordinates:

1. action-key detection
2. branch routing to standard/product temporal seams
3. `handled=false` for non-layer-temporal actions

Important:

- keep existing action keys:
  - `assetGenerationActionRunStandardProductTemporal`
  - `assetGenerationActionRunPlatformAdaptTemporal`
- keep branch ordering stable
- keep non-temporal fallback unchanged

- [ ] **Step 4: Re-run boundary and behavior verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationLayerTemporal.*|TestExecuteTaskGenerationActionStarts(StandardProductTemporalWorkflow|PlatformAdaptTemporalWorkflow)|TestTaskGenerationActionPhaseOwnershipBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- all layer-temporal behavior tests PASS
- ownership boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/task_generation_service.go internal/listingkit/phase19_action_layer_temporal_boundary_test.go internal/listingkit/service_generation_actions_test.go internal/listingkit/phase18_action_service_entry_boundary_test.go
git commit -m "test: lock listingkit layer temporal action boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 19` stays inside ListingKit action temporal bypass ownership
2. the plan does not reopen stable `Phase 18` service-entry seams
3. the plan does not broaden into generic temporal framework work
4. the plan directly addresses the four scoped pressures:
   - action-key branching
   - workflow enablement/config checks
   - start-input assembly
   - outward result/audit shaping
5. each task has a bounded write set and explicit verification command

This plan passes that check.

## Expected Verification Matrix

During execution, the implementing worker should expect to run at least these checks:

```powershell
go test ./internal/listingkit -run "TestExecuteTaskGenerationActionStarts(StandardProductTemporalWorkflow|PlatformAdaptTemporalWorkflow)" -count=1
go test ./internal/listingkit -run "TestTaskGenerationLayerTemporal.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationLayerTemporal.*Boundary|TestTaskGenerationActionPhaseOwnershipBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

If a worker broadens beyond those checks, it should explain why.

## Execution Handoff

Plan complete and saved to [2026-06-01-task-processor-framework-phase19-layer-temporal-action-branching.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-01-task-processor-framework-phase19-layer-temporal-action-branching.md:1).

Two execution options:

1. `Subagent-Driven` (recommended) - I dispatch a fresh subagent per task, review between tasks, and keep only the minimum number of agents open.
2. `Inline Execution` - I execute the tasks in this session with checkpoints.
