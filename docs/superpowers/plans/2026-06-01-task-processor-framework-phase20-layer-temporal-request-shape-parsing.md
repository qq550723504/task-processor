# Task Processor Framework Phase 20 ListingKit Layer-Temporal Request-Shape Parsing Ownership Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the next residual ownership hotspot in ListingKit by moving `layer-temporal` request-shape parsing out of the broader action-helper cluster and into an explicit feature-local temporal parsing seam.

**Architecture:** Keep the `Phase 19` temporal execution seams intact. Do **not** reopen `standard` / `platform` / `result` execution ownership. Instead, isolate the temporal-specific request traversal currently embedded in `resolveLayerTemporalPlatform(...)` into a ListingKit-local parsing seam, route the platform temporal execution path through it, and then lock the new boundary with focused tests. This is a local ownership move, not a generic request-parsing framework.

**Tech Stack:** Go, ListingKit task generation action flow, temporal action seams, request-shape traversal helpers, source-boundary guardrails

**Out of Scope For This Slice:**

- reopening `Phase 19` temporal execution seams
- broad refactors across all of `service_generation_actions.go`
- redesigning `resolveAssetGenerationActionTarget(...)`
- inventing a repo-wide request-parsing framework
- HTTP/bootstrap/runtime changes
- changes to temporal outward behavior, workflow start semantics, or platform defaulting/normalization behavior

---

## Root Cause This Slice Addresses

After `Phase 19`, temporal execution ownership is explicit, but one temporal-specific request traversal hotspot still lives inside the broader action-helper file:

- [internal/listingkit/service_generation_actions.go:13](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)

`resolveLayerTemporalPlatform(...)` currently walks multiple request-shape layers:

1. `Target.QueueQuery`
2. `Target.NavigationTarget.QueueQuery`
3. `Target.NavigationTarget.SessionQuery`
4. `Target.NavigationTarget.PreviewQuery`
5. `Target.NavigationTarget.ActionTarget.QueueQuery`
6. `Target.NavigationTarget.Descriptor.FollowUpReads[*].Query`
7. nested `ActionTarget`

That logic is clearly temporal-specific, but it still lives beside broader helpers such as:

- [resolveAssetGenerationActionTarget(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:56)
- [requestedAssetGenerationActionKey(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:147)
- [cloneAssetGenerationActionTarget(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:105)

The ownership problem is no longer “how do temporal branches execute.” The next problem is “why is temporal request parsing still mixed into a wider action-helper home.”

---

## File Structure

### Existing files that remain important

- [internal/listingkit/service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1)
  - Today still owns `resolveLayerTemporalPlatform(...)`
  - Should end the phase without owning the temporal-specific traversal implementation

- [internal/listingkit/task_generation_action_temporal_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_platform.go:1)
  - Already owns platform temporal execution start-input assembly
  - Should end the phase consuming a feature-local temporal request-parsing helper/seam, not the broader generic helper file

- [internal/listingkit/service_generation_actions_test.go](/D:/code/task-processor/internal/listingkit/service_generation_actions_test.go:1)
  - Already hosts many generation-action helper and temporal branch tests
  - Good place for focused behavior tests around request-shape traversal

- [internal/listingkit/phase19_action_layer_temporal_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase19_action_layer_temporal_boundary_test.go:1)
  - Already protects `Phase 19` temporal seam ownership
  - May need a small update so the platform seam’s required parsing dependency points at the new local helper instead of the old broad helper

### New files this phase should introduce

- `internal/listingkit/task_generation_action_temporal_request_platform.go`
  - Feature-local home for layer-temporal platform request parsing
  - Should own the traversal/defaulting/normalization behavior currently embedded in `resolveLayerTemporalPlatform(...)`

- `internal/listingkit/phase20_action_temporal_request_boundary_test.go`
  - Guardrail ensuring temporal request parsing lives in its local home, not back in the broader helper file

This keeps the slice narrow: one temporal-specific parsing seam, one consumer update, one guardrail.

---

## Target Outcome

At the end of `Phase 20`:

- `layer-temporal` request-shape parsing has an explicit ListingKit-local home
- the platform temporal execution seam consumes that local parsing seam
- `service_generation_actions.go` no longer owns the temporal-specific traversal implementation
- existing platform defaulting (`"shein"`) and normalization behavior remain unchanged
- ownership guardrails lock the new parsing seam so it does not drift back into the broader helper file

---

## Task 1: Lock current layer-temporal request-shape traversal behavior

**Files:**
- Modify: `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Add failing behavior tests for traversal precedence and defaults**

Add focused tests that lock the existing behavior of `resolveLayerTemporalPlatform(...)` before moving it:

1. `Target.QueueQuery.Platform` wins when present
2. `NavigationTarget.QueueQuery.Platform` is used when root queue query is absent
3. `NavigationTarget.SessionQuery.Platform` is used when queue query is absent
4. `NavigationTarget.PreviewQuery.Platform` is used when earlier shapes are absent
5. `Descriptor.FollowUpReads[*].Query.Platform` is used when earlier shapes are absent
6. nested `ActionTarget` is traversed
7. empty / missing input still defaults to `"shein"`
8. normalization still trims and lowercases values

Suggested test names:

```go
func TestResolveLayerTemporalPlatformPrefersTargetQueueQuery(t *testing.T)
func TestResolveLayerTemporalPlatformTraversesNavigationQueries(t *testing.T)
func TestResolveLayerTemporalPlatformTraversesFollowUpReadsAndNestedActionTarget(t *testing.T)
func TestResolveLayerTemporalPlatformDefaultsToShein(t *testing.T)
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestResolveLayerTemporalPlatform.*" -count=1
```

Expected: FAIL until the tests are written/aligned to the current behavior surface.

- [ ] **Step 3: Align tests until they describe current behavior exactly**

Do not change production behavior yet. If a test fails because the expected precedence is wrong, fix the test to match the current implementation. The point of this task is to freeze the existing semantics before moving ownership.

- [ ] **Step 4: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestResolveLayerTemporalPlatform.*" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/service_generation_actions_test.go
git commit -m "test: lock layer temporal platform parsing behavior"
```

---

## Task 2: Extract a feature-local temporal request-parsing seam

**Files:**
- Create: `internal/listingkit/task_generation_action_temporal_request_platform.go`
- Modify: `internal/listingkit/service_generation_actions.go`
- Modify if needed: `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Introduce the new local parsing seam behind the current behavior**

Create a feature-local helper/seam that becomes the new home of temporal request-shape parsing.

Suggested shape:

```go
type taskGenerationActionTemporalPlatformRequestPhase struct{}

func buildTaskGenerationActionTemporalPlatformRequestPhase() *taskGenerationActionTemporalPlatformRequestPhase

func (p *taskGenerationActionTemporalPlatformRequestPhase) run(req *ExecuteGenerationActionRequest) string
```

This seam should own:

- traversal across the current request-shape layers
- trimming and lowercasing
- nested action-target recursion if still needed
- defaulting to `"shein"`

- [ ] **Step 2: Move the traversal logic into the new local file**

Refactor so the traversal implementation lives in:

- `internal/listingkit/task_generation_action_temporal_request_platform.go`

and the broader helper file no longer owns that implementation.

Important:

- preserve the exact behavior locked in Task 1
- do not redesign the request shapes
- do not mix broader target-resolution concerns into this new file
- keep this seam temporal-specific and feature-local

- [ ] **Step 3: Decide whether `resolveLayerTemporalPlatform(...)` becomes a thin wrapper or is fully removed**

Choose the smaller change that preserves clarity:

- If external call sites still meaningfully benefit from the old helper name, make it a thin wrapper that delegates to the new local seam
- If the only remaining consumers are temporal-local, remove the old helper and update callers directly

Whichever option you choose, the ownership goal must still hold: the traversal implementation itself must live in the new local file, not in the broad helper file.

- [ ] **Step 4: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestResolveLayerTemporalPlatform.*" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/task_generation_action_temporal_request_platform.go internal/listingkit/service_generation_actions.go internal/listingkit/service_generation_actions_test.go
git commit -m "refactor: extract listingkit temporal platform request seam"
```

---

## Task 3: Route the platform temporal execution seam through the new local parsing home

**Files:**
- Modify: `internal/listingkit/task_generation_action_temporal_platform.go`
- Modify if needed:
  - `internal/listingkit/service_generation_actions.go`
  - `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Add a failing seam-consumer test**

Add focused coverage that locks:

1. platform temporal execution still uses the same parsing/defaulting behavior
2. the platform seam now depends on the new local parsing seam/home rather than the broad helper implementation
3. `StartPlatformAdaptation(...)` still receives the parsed platform
4. the shared temporal result seam still receives `&GenerationQueueQuery{Platform: platform}`

Suggested test names:

```go
func TestTaskGenerationLayerTemporalPlatformPhaseUsesLocalRequestParsingSeam(t *testing.T)
func TestTaskGenerationLayerTemporalPlatformPhasePreservesParsedPlatformHandoff(t *testing.T)
```

- [ ] **Step 2: Route the platform seam through the new local parsing home**

Update:

- [internal/listingkit/task_generation_action_temporal_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_platform.go:1)

so that the platform seam no longer depends on the old broad traversal implementation as its primary ownership home.

Important:

- keep the same workflow enablement/client checks
- keep the same start-input semantics
- keep the same shared result handoff
- do not widen this into broader action helper cleanup

- [ ] **Step 3: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestResolveLayerTemporalPlatform.*|TestTaskGenerationLayerTemporalPlatform.*|TestExecuteTaskGenerationActionStartsPlatformAdaptTemporalWorkflow" -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/listingkit/task_generation_action_temporal_platform.go internal/listingkit/task_generation_action_temporal_request_platform.go internal/listingkit/service_generation_actions.go internal/listingkit/service_generation_actions_test.go
git commit -m "refactor: route listingkit platform temporal seam through local request parsing"
```

---

## Task 4: Lock temporal request-parsing ownership guardrails

**Files:**
- Create: `internal/listingkit/phase20_action_temporal_request_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/phase19_action_layer_temporal_boundary_test.go`
  - `internal/listingkit/service_generation_actions_test.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. temporal request-shape traversal implementation lives in the new local file
2. the platform temporal seam consumes that local parsing home
3. the broad helper file no longer owns the traversal implementation
4. `Phase 19` temporal execution seams remain unchanged in responsibility

The guardrails should specifically catch regressions where:

- traversal logic drifts back into `service_generation_actions.go`
- platform execution re-inlines parsing locally
- unrelated action-target resolution helpers get mixed into this temporal parsing seam

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestResolveLayerTemporalPlatform.*|TestTaskGenerationLayerTemporal.*Boundary|TestTaskGenerationActionPhaseOwnershipBoundary" -count=1
```

Expected: FAIL until the new guardrails match the final ownership split.

- [ ] **Step 3: Align the boundary suite with the new parsing home**

Update the boundary tests so they point at the new local ownership home and keep `Phase 19`’s execution seam expectations intact.

Important:

- prefer the existing AST/token/source helper style already used in `phase18` and `phase19`
- avoid overly formatting-sensitive assertions when a lower-fragility equivalent exists
- do not broaden the suite into generic helper ownership beyond this slice

- [ ] **Step 4: Re-run verification**

Run:

```powershell
go test ./internal/listingkit -run "TestResolveLayerTemporalPlatform.*|TestTaskGenerationLayerTemporal.*|TestTaskGenerationActionPhaseOwnershipBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected:

- request-parsing behavior tests PASS
- temporal seam boundary tests PASS
- downstream HTTP / temporal packages PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase20_action_temporal_request_boundary_test.go internal/listingkit/phase19_action_layer_temporal_boundary_test.go internal/listingkit/service_generation_actions_test.go internal/listingkit/task_generation_action_temporal_request_platform.go internal/listingkit/task_generation_action_temporal_platform.go internal/listingkit/service_generation_actions.go
git commit -m "test: lock listingkit temporal request parsing boundaries"
```

---

## Self-Review Checklist

Before execution starts, verify the plan against the scope:

1. `Phase 20` stays inside temporal request-shape parsing ownership
2. the plan does not reopen `Phase 19` temporal execution seams
3. the plan does not broaden into a full `service_generation_actions.go` rewrite
4. the plan does not expand into a generic request-parsing framework
5. each task has a bounded write set and explicit verification command

This plan passes that check.

## Expected Verification Matrix

During execution, the implementing worker should expect to run at least these checks:

```powershell
go test ./internal/listingkit -run "TestResolveLayerTemporalPlatform.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationLayerTemporalPlatform.*|TestExecuteTaskGenerationActionStartsPlatformAdaptTemporalWorkflow" -count=1
go test ./internal/listingkit -run "TestTaskGenerationLayerTemporal.*Boundary|TestTaskGenerationActionPhaseOwnershipBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

If a worker broadens beyond those checks, it should explain why.

## Execution Handoff

Plan complete and saved to [2026-06-01-task-processor-framework-phase20-layer-temporal-request-shape-parsing.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-01-task-processor-framework-phase20-layer-temporal-request-shape-parsing.md:1).

Two execution options:

1. `Subagent-Driven` (recommended) - I dispatch a fresh subagent per task, review between tasks, and keep only the minimum number of agents open.
2. `Inline Execution` - I execute the tasks in this session with checkpoints.
