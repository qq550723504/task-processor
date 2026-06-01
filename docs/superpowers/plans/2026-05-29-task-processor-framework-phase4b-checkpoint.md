# Task Processor Framework Phase 4B Checkpoint

## Status

`Phase 4B` is functionally complete for the intended slice.

This phase was not about redesigning ListingKit submit flows or introducing a generic runtime-context framework. The goal was narrower:

1. stop submit identity, store selection, store info lookup, API client creation, and submit-settings hydration from remaining spread across loosely related helpers
2. make that cross-cutting submit/runtime context flow through an explicit ListingKit-owned resolver seam
3. align submit execution and direct submit on the same seam
4. lock the new ownership boundary so pure settings hydration and remote store/client operations do not silently collapse back together

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Pure submit-settings hydration is now separated from remote store/client work

The merge-oriented submit settings rules now live in:

- [internal/listingkit/service_submit_settings_resolution.go](/D:/code/task-processor/internal/listingkit/service_submit_settings_resolution.go:1)

This file now owns the deterministic parts of settings shaping:

- `applySubmitSettingsProfile(...)`
- `applySubmitSettingsTaskRequest(...)`
- `applySubmitWarehouseOverride(...)`

The orchestration entry point still lives in:

- [internal/listingkit/service_submit_store_context.go](/D:/code/task-processor/internal/listingkit/service_submit_store_context.go:1)

but it no longer needs to keep all merge logic inline.

That matters because the core bug risk here was not “too many lines in one file.” The real risk was that default settings, profile overlays, request overrides, and warehouse overrides were still intertwined with runtime/store resolution behavior.

### 2. Submit/runtime context now flows through an explicit ListingKit-owned resolver seam

The new resolver seam lives in:

- [internal/listingkit/service_submit_context_resolver.go](/D:/code/task-processor/internal/listingkit/service_submit_context_resolver.go:1)

This file now owns coordinated resolution across:

- store selection
- store profile lookup
- store info lookup
- API client creation
- submit settings resolution
- warehouse resolution

The resolver is constructed through:

- `buildSubmitRuntimeContextResolver(s)`

and is now the cross-cutting seam used by the surrounding helpers.

This is the main architectural result of `Phase 4B`: ListingKit submit/runtime context is still feature-owned, but it is no longer primarily expressed as behavior scattered across several service helpers with implicit ownership.

### 3. Submit context consumers now share the same resolver-backed path

The consumer alignment landed in:

- [internal/listingkit/service_submit_wiring.go](/D:/code/task-processor/internal/listingkit/service_submit_wiring.go:1)
- [internal/listingkit/service_submit_direct.go](/D:/code/task-processor/internal/listingkit/service_submit_direct.go:1)

The wiring layer now explicitly builds submit-oriented collaborator configs through the resolver seam, including the direct-submit path via:

- `buildTaskDirectSubmissionServiceConfig(s)`

This matters because direct submit and submit execution previously had more room to drift apart in how they reached store/runtime context. After this phase, the seam is more visible and shared.

### 4. Guardrails now lock the submit/runtime context ownership split

The new boundary protections live in:

- [internal/listingkit/phase4b_submit_context_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase4b_submit_context_boundary_test.go:1)
- [internal/listingkit/service_wiring_test.go](/D:/code/task-processor/internal/listingkit/service_wiring_test.go:1)
- [internal/listingkit/service_submit_store_context_test.go](/D:/code/task-processor/internal/listingkit/service_submit_store_context_test.go:1)

These checks now explicitly protect both sides of the seam:

1. `service_submit_store_context.go` does not regrow remote client/bootstrap logic
2. `service_shein_store_client.go` does not regrow pure settings-hydration logic
3. the dedicated resolver file remains the home of cross-cutting submit/runtime context resolution
4. submit-oriented wiring continues to consume explicit builders instead of silently reshaping inline logic

## Acceptance Check

`Phase 4B` was meant to prove four things:

1. deterministic submit-settings shaping can be separated from runtime store/client work
2. cross-cutting submit/runtime context can flow through one explicit ListingKit-owned resolver seam
3. submit execution and direct submit can consume that seam without changing behavior
4. the new ownership split can be protected with narrow source and behavior guardrails

All four are now true.

More concretely:

- pure settings merge logic is isolated
- store/profile/client/settings orchestration is centralized in one resolver seam
- direct submit and execution paths are aligned through explicit wiring
- ListingKit package tests still pass without behavioral regressions

## What This Phase Did Not Try To Solve

### 1. It did not redesign submit business behavior

Files such as:

- [internal/listingkit/task_submission_execution_service.go](/D:/code/task-processor/internal/listingkit/task_submission_execution_service.go:1)
- [internal/listingkit/service_submit.go](/D:/code/task-processor/internal/listingkit/service_submit.go:1)

still represent the existing submit behavior model.

That is correct for this phase. The goal here was ownership and seam clarity, not business-process redesign.

### 2. It did not introduce a generic repo-wide context abstraction

The resolver seam remains local to ListingKit:

- [internal/listingkit/service_submit_context_resolver.go](/D:/code/task-processor/internal/listingkit/service_submit_context_resolver.go:1)

That is intentional. There still is not enough evidence that other features need the same abstraction shape.

### 3. It did not remove lazy accessor patterns

The relevant service helpers still exist and still provide the same public behavior shape.

That is acceptable. The problem was not lazy access itself; it was hidden cross-cutting ownership inside those helpers.

## Residual Responsibilities Still Present

### `service_submit_context_resolver.go` is now a real hotspot

The new resolver seam is a better ownership boundary, but it is also now the main concentration point for submit/runtime context coordination.

That is acceptable for this phase because the cross-cutting logic needed one visible home first. If future change pressure keeps landing there, the next slice should be driven by the kinds of changes inside that file, not by symmetry concerns elsewhere.

### Store/catalog lookup and submit settings are still part of one feature seam

The resolver currently coordinates both:

- remote store/client concerns
- submit settings/warehouse concerns

That is still the right tradeoff today because those responsibilities participate in the same operational decision chain. Splitting them further would only make sense if their change pressure starts diverging.

### Wiring still closes over `*service`

The resolver builder and submit config builders still work from `*service` directly.

That is acceptable for now. This phase was about making ownership explicit, not adding another intermediate abstraction layer.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “extract more helper files because the resolver exists now.” Better next steps are:

### 1. Watch whether the new resolver seam becomes behaviorally overloaded

If future changes keep landing in:

- [internal/listingkit/service_submit_context_resolver.go](/D:/code/task-processor/internal/listingkit/service_submit_context_resolver.go:1)

then the next slice should be driven by real change hotspots inside that seam, for example whether store resolution and remote client bootstrap deserve separate internal collaborators.

### 2. Reassess whether submit execution/process modeling is now the better hotspot

If the next wave of changes is more about:

- readiness checks
- retry behavior
- workflow/process orchestration

then work should move outward to execution/process boundaries rather than continuing to polish the resolver seam.

### 3. Leave this seam alone unless another concrete ownership problem appears

This layer is now in a good enough state:

- explicit resolver seam exists
- consumer alignment exists
- guardrails exist
- behavior remained stable

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 4B` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Notes:

- `./internal/listingkit/...` already covers `httpapi` and `temporal`, but the explicit second run was kept as a focused confirmation for the integration seams this phase could have affected.

## Current Branch Notes

The main `Phase 4B` commits are:

- `2f152a30` `docs: add framework phase4b plan`
- `b9fd8e59` `refactor: extract listingkit submit settings resolution`
- `2b56d7e1` `refactor: introduce listingkit submit runtime context resolver`
- `a8a816cb` `refactor: align listingkit submit context consumers`
- `6d7cf8fb` `test: lock listingkit submit runtime context boundary`

## Recommendation

Mark `Phase 4B` complete.

Do not keep working this seam just because the new resolver file is now easier to modify. The main ownership bug this phase addressed is already fixed:

- submit identity, store resolution, remote client bootstrap, and submit-settings hydration no longer primarily live as implicit behavior spread across unrelated helper files

If we continue, the better next step is to choose a new hotspot based on where behavior-level changes are actually landing, most likely around submit execution/process modeling rather than more resolver-symmetry cleanup.
