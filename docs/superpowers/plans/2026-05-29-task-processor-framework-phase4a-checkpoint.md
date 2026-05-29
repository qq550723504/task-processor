# Task Processor Framework Phase 4A Checkpoint

## Status

`Phase 4A` is functionally complete for the intended slice.

This phase was not about rewriting ListingKit business flows. The goal was narrower:

1. stop `internal/listingkit/service.go` from also being the primary home of collaborator config shaping
2. make admin, submit, and temporal collaborator wiring explicit
3. lock the new ownership seams so the service root does not silently regrow inline wiring logic

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. `service.go` is now more clearly a service root

The main service root:

- [internal/listingkit/service.go](/D:/code/task-processor/internal/listingkit/service.go:1)

still owns:

- long-lived service state
- `ServiceConfig`
- `NewService(...)`
- `newServiceWithConfig(...)`
- default application of core config

But it no longer directly owns the collaborator-group initialization bodies that were previously embedded in the same file.

Those initialization methods now live in:

- [internal/listingkit/service_collaborators.go](/D:/code/task-processor/internal/listingkit/service_collaborators.go:1)

This is a modest change on purpose. The win is not “more files”; the win is that the service root is more clearly separated from the lifecycle that hydrates collaborator groups.

### 2. Admin collaborator wiring is now explicit and local to admin-facing seams

The config shaping for admin collaborators now lives in:

- [internal/listingkit/service_admin_wiring.go](/D:/code/task-processor/internal/listingkit/service_admin_wiring.go:1)

This file now owns the builder seams for:

- `settingsAdminServiceConfig`
- `sheinAdminServiceConfig`

The corresponding `*OrDefault()` methods in:

- [internal/listingkit/settings_admin_service.go](/D:/code/task-processor/internal/listingkit/settings_admin_service.go:1)
- [internal/listingkit/shein_admin_service.go](/D:/code/task-processor/internal/listingkit/shein_admin_service.go:1)

no longer shape these config structs inline. They now consume explicit builders:

- `buildSettingsAdminServiceConfig(s)`
- `buildSheinAdminServiceConfig(s)`

That makes the ownership of admin collaborator wiring visible instead of implicit.

### 3. Submit and temporal collaborator wiring is now explicit

The config shaping for submit-oriented collaborators now lives in:

- [internal/listingkit/service_submit_wiring.go](/D:/code/task-processor/internal/listingkit/service_submit_wiring.go:1)

This file now owns the builder seams for:

- `taskSubmissionServiceConfig`
- `taskSubmissionExecutionServiceConfig`
- `taskTemporalSubmissionAdapterConfig`

The corresponding runtime helpers in:

- [internal/listingkit/service_submit.go](/D:/code/task-processor/internal/listingkit/service_submit.go:1)
- [internal/listingkit/service_submit_temporal_adapter.go](/D:/code/task-processor/internal/listingkit/service_submit_temporal_adapter.go:1)

now consume:

- `buildTaskSubmissionServiceConfig(s)`
- `buildTaskSubmissionExecutionServiceConfig(s)`
- `buildTaskTemporalSubmissionAdapterConfig(s)`

instead of inlining config shaping beside `newTaskSubmissionService(...)` and `newTaskTemporalSubmissionAdapter(...)`.

This is the main architectural result of `Phase 4A`: collaborator creation is still feature-owned, but the service root no longer hides that wiring behind scattered inline config literals.

### 4. Guardrails now lock the collaborator wiring boundary

The new tests that protect these seams live in:

- [internal/listingkit/service_wiring_test.go](/D:/code/task-processor/internal/listingkit/service_wiring_test.go:1)
- [internal/listingkit/phase4a_collaborator_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase4a_collaborator_boundary_test.go:1)
- [internal/listingkit/service_config_test.go](/D:/code/task-processor/internal/listingkit/service_config_test.go:1)

These guardrails now explicitly check that:

1. `service.go` no longer owns collaborator-group initialization bodies
2. admin collaborator files use explicit wiring builders
3. submit and temporal collaborator files use explicit wiring builders
4. the root service still owns service construction and initial lock setup

That combination is important because it protects both sides of the refactor:

- wiring stays out of the service root
- the service root still remains the place where long-lived runtime state begins

## Acceptance Check

`Phase 4A` was meant to prove four things:

1. collaborator-group initialization can move out of `service.go` without changing runtime behavior
2. admin collaborator wiring can be expressed through explicit feature-owned builders
3. submit and temporal collaborator wiring can be expressed through explicit feature-owned builders
4. the new seams can be protected with narrow source and behavior guardrails

All four are now true.

More concretely:

- `service.go` is no longer the place where all collaborator lifecycle details live
- admin wiring is visible through dedicated builders
- submit and temporal wiring is visible through dedicated builders
- ListingKit package tests still pass without behavior changes

## What This Phase Did Not Try To Solve

### 1. It did not redesign ListingKit service-domain structure

The service root still holds a large number of fields and collaborators:

- [internal/listingkit/service.go](/D:/code/task-processor/internal/listingkit/service.go:1)

That is acceptable. `Phase 4A` was about explicit wiring ownership, not about shrinking the domain surface by force.

### 2. It did not phase-model workflow/process execution

Files such as:

- [internal/listingkit/service_process.go](/D:/code/task-processor/internal/listingkit/service_process.go:1)
- [internal/listingkit/workflow_standard.go](/D:/code/task-processor/internal/listingkit/workflow_standard.go:1)
- [internal/listingkit/workflow_platform_adaptation.go](/D:/code/task-processor/internal/listingkit/workflow_platform_adaptation.go:1)

still represent the current workflow/process modeling approach.

That is still the right call for now. This phase deliberately stayed at the collaborator wiring layer.

### 3. It did not remove `*OrDefault()` as a pattern

The `*OrDefault()` helpers still exist and remain the lazy-init access points.

That is acceptable. The issue was not the existence of lazy accessors; it was the amount of inline config shaping hidden inside them.

## Residual Responsibilities Still Present

### `service.go` still remains the main state concentration point

The service root still holds:

- workflow clients and flags
- repositories
- submit locks
- settings state
- runtime collaborator references

That is now a clearer responsibility set than before, but it is still the main concentration point for service-wide state.

### Wiring files still close over `service` directly

The new builder files:

- [internal/listingkit/service_admin_wiring.go](/D:/code/task-processor/internal/listingkit/service_admin_wiring.go:1)
- [internal/listingkit/service_submit_wiring.go](/D:/code/task-processor/internal/listingkit/service_submit_wiring.go:1)

still take `*service` directly.

That is acceptable for this phase because the goal was to make ownership explicit, not to introduce a new intermediate abstraction.

### Business behavior still depends on the same collaborator graph

This refactor improved where collaborator config is built, but did not simplify the collaborator graph itself.

That means future refactors should be driven by actual domain-change pressure, not by the existence of builder files alone.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “extract more builders for symmetry.” The better next steps are:

### 1. Reassess whether submit/runtime context helpers are now the real hotspot

After wiring extraction, the most likely next internal pressure area is around submit/runtime context logic, for example:

- [internal/listingkit/service_submit_runtime_context.go](/D:/code/task-processor/internal/listingkit/service_submit_runtime_context.go:1)
- [internal/listingkit/service_submit_store_context.go](/D:/code/task-processor/internal/listingkit/service_submit_store_context.go:1)

If those areas continue changing rapidly, they are a better candidate than further root-service cleanup.

### 2. Consider workflow/process phase modeling only if change pressure stays in execution paths

If the next wave of changes lands in:

- generation execution
- submit execution
- workflow recovery

then a phase-model slice would make more sense than more collaborator refactoring.

### 3. Leave the wiring layer alone unless another ownership problem appears

This layer is now in a good enough state:

- explicit builders exist
- guardrails exist
- runtime behavior stayed stable

Do not keep polishing this slice unless a concrete maintenance problem reappears.

## Verification Summary

The final `Phase 4A` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Notes:

- `./internal/listingkit/...` already covers `httpapi` and `temporal`, but the explicit second run was kept as a focused confirmation for the integration seams this phase could have affected.

## Current Branch Notes

The main `Phase 4A` commits are:

- `cfbcf5d2` `docs: add framework phase4a plan`
- `e391742e` `refactor: split listingkit collaborator initialization`
- `bf2442bf` `refactor: extract listingkit admin collaborator wiring`
- `e0d2a9fa` `refactor: extract listingkit submit collaborator wiring`
- `590f10cc` `test: lock listingkit collaborator wiring boundary`

## Recommendation

Mark `Phase 4A` complete.

Do not keep working this seam just because the new structure is now easier to edit. The key ownership bug this phase addressed is already fixed:

- `service.go` is no longer silently doubling as the primary home of collaborator config shaping

If we continue, the better next step is to choose a new hotspot based on actual behavior-level change pressure, most likely around submit/runtime context or workflow/process execution, not on residual file symmetry alone.
