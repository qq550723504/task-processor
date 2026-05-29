# Task Processor Framework Phase 3B Checkpoint

## Status

`Phase 3B` is functionally complete for the intended slice.

This phase was not about building a generic bootstrap platform. The goal was narrower:

1. split `internal/listingkit/httpapi/bootstrap.go` by real responsibility seams
2. extract only the adapter-oriented bootstrap helpers that now have concrete reuse evidence
3. lock the new boundaries so app-layer and feature-layer code do not drift back together

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. ListingKit bootstrap is now split by assembly concern

The old mixed assembly hotspot in:

- [internal/listingkit/httpapi/bootstrap.go](/D:/code/task-processor/internal/listingkit/httpapi/bootstrap.go:1)

has been materially decomposed into:

- [internal/listingkit/httpapi/bootstrap_repositories.go](/D:/code/task-processor/internal/listingkit/httpapi/bootstrap_repositories.go:1)
- [internal/listingkit/httpapi/bootstrap_service_config.go](/D:/code/task-processor/internal/listingkit/httpapi/bootstrap_service_config.go:1)
- [internal/listingkit/httpapi/bootstrap_runtime.go](/D:/code/task-processor/internal/listingkit/httpapi/bootstrap_runtime.go:1)

This matters because the main `bootstrap.go` file is now primarily an orchestration entry, while the extracted files own:

- repository assembly
- service-config assembly
- runtime/module assembly

That is the right ownership split for this phase. The logic is still feature-owned, but the mixed file-level hotspot is gone.

### 2. SDS bootstrap mechanics are now adapter-oriented helpers

The SDS bootstrap path that had already become reusable now lives in:

- [internal/sds/httpbootstrap/support.go](/D:/code/task-processor/internal/sds/httpbootstrap/support.go:1)
- [internal/sds/httpbootstrap/support_test.go](/D:/code/task-processor/internal/sds/httpbootstrap/support_test.go:1)

This helper package now owns the bootstrap mechanics for:

- SDS sync-service startup
- SDS baseline remote-provider startup

The app layer still decides when to bootstrap those collaborators, but it no longer needs to own the SDS-specific setup recipe itself.

### 3. SHEIN Redis cookie-store bootstrap is now reused through `sheinlogin/bootstrap`

The Redis-backed SHEIN cookie-store setup is now expressed as a narrow helper in:

- [internal/sheinlogin/bootstrap/store_support.go](/D:/code/task-processor/internal/sheinlogin/bootstrap/store_support.go:1)
- [internal/sheinlogin/bootstrap/store_support_test.go](/D:/code/task-processor/internal/sheinlogin/bootstrap/store_support_test.go:1)

That helper now owns:

- nil-safe Redis config detection
- Redis store creation from app config
- no-op behavior when Redis config is absent

It is now consumed by:

- [internal/app/httpapi/listingkit_support.go](/D:/code/task-processor/internal/app/httpapi/listingkit_support.go:1)
- [internal/sheinlogin/bootstrap/build.go](/D:/code/task-processor/internal/sheinlogin/bootstrap/build.go:1)

This is exactly the kind of reuse `Phase 3B` was meant to allow: not a generic runtime abstraction, but a small bootstrap helper with two real callers.

### 4. App-layer ListingKit support is now more clearly limited to timing, caching, and degradation policy

After `Phase 3A` and `Phase 3B`, the remaining hotspot:

- [internal/app/httpapi/listingkit_support.go](/D:/code/task-processor/internal/app/httpapi/listingkit_support.go:1)

still owns app/runtime-local concerns such as:

- lazy initialization timing
- support-state caching
- degraded-mode logging
- closer registration

But it no longer owns:

- ListingKit repository bundle shaping
- ListingKit hook bundle shaping
- SDS bootstrap mechanics
- direct Redis store bootstrap mechanics

That is a much healthier boundary. The file is still app-owned, but its job is now operational coordination rather than feature contract definition or adapter construction logic.

### 5. Boundary tests now lock both decomposition and reuse seams

The new guardrails live in:

- [internal/listingkit/httpapi/bootstrap_test.go](/D:/code/task-processor/internal/listingkit/httpapi/bootstrap_test.go:1)
- [internal/listingkit/httpapi/phase3b_bootstrap_boundary_test.go](/D:/code/task-processor/internal/listingkit/httpapi/phase3b_bootstrap_boundary_test.go:1)
- [internal/app/httpapi/phase3a_listingkit_boundary_test.go](/D:/code/task-processor/internal/app/httpapi/phase3a_listingkit_boundary_test.go:1)
- [internal/app/httpapi/runtime_deps_test.go](/D:/code/task-processor/internal/app/httpapi/runtime_deps_test.go:1)
- [internal/sheinlogin/bootstrap/store_support_test.go](/D:/code/task-processor/internal/sheinlogin/bootstrap/store_support_test.go:1)

These tests now protect two specific regressions:

1. `bootstrap.go` regrowing into a giant mixed assembly file
2. app-layer ListingKit support regrowing direct SDS or Redis bootstrap mechanics

## Acceptance Check

`Phase 3B` was meant to prove four things:

1. ListingKit bootstrap can be decomposed without changing runtime behavior
2. SDS bootstrap reuse can move to an adapter-oriented helper without inventing a generic framework
3. SHEIN Redis bootstrap reuse has at least two real callers and can be extracted cleanly
4. the new seams can be kept stable with narrow guardrail tests

All four are now true.

More concretely:

- `bootstrap.go` is now an orchestration file instead of a mixed assembly dump
- SDS bootstrap mechanics are no longer trapped in app-layer code
- Redis cookie-store bootstrap is now shared through a narrow helper with real usage
- tests explicitly guard both the feature split and the app-layer reuse boundary

## What This Phase Did Not Try To Solve

### 1. It did not redesign ListingKit service internals

Files such as:

- [internal/listingkit/service.go](/D:/code/task-processor/internal/listingkit/service.go:1)

still contain substantial service orchestration and domain-level dependency wiring.

That is acceptable. `Phase 3B` was about bootstrap decomposition and bootstrap reuse, not service redesign.

### 2. It did not make `sheinlogin.Service` consume an externally built Redis store

The current service constructor still owns its internal Redis store creation path:

- [internal/sheinlogin/service.go](/D:/code/task-processor/internal/sheinlogin/service.go:1)

That is still the right call for now. We had enough evidence to share:

- config detection
- store bootstrap mechanics

but not enough pressure to rewrite the service constructor boundary itself.

### 3. It did not create a repo-wide bootstrap framework

There is still no generic “bootstrap manifest” or giant shared platform package for every module.

That is intentional. This phase extracted only the seams that had already stabilized:

- ListingKit bootstrap decomposition
- SDS bootstrap support
- SHEIN Redis bootstrap support

## Residual Responsibilities Still Present

### `bootstrap.go` is smaller, but ListingKit still owns a rich feature bootstrap surface

Even after decomposition, ListingKit still owns:

- service validation
- grouped builder contracts
- runtime bundle composition
- Temporal-aware runtime decisions inside the feature package

That is acceptable because those are still feature-level concerns, not app-layer leakage.

### `listingkit_support.go` still remains the main app-layer ListingKit runtime hotspot

The file is much healthier now, but it still owns:

- cached support state on `runtimeDeps`
- lazy init timing
- degrade-with-log behavior
- closer registration for app-owned resources

This is now an operational hotspot instead of an ownership bug.

### Adapter-oriented helper reuse is still intentionally narrow

We now have two reusable helper slices:

- SDS bootstrap support
- SHEIN Redis bootstrap support

That is enough evidence for these helpers, but still not enough evidence for a larger shared bootstrap layer.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “extract one more helper for symmetry.” The better next steps are:

### 1. Reassess whether ListingKit bootstrap has another real internal hotspot

Now that file-level decomposition is in place, the next useful question is whether one of these areas is changing often enough to justify deeper refactoring:

- service bundle assembly
- grouped builder validation
- handler dependency shaping

If not, leave them alone.

### 2. Wait for a second feature before broadening adapter bootstrap reuse

If another feature starts needing:

- Redis-backed bootstrap with similar config semantics
- SDS-like startup collaborators

that would justify a broader adapter-side convention.

Until then, keep the helper reuse small and explicit.

### 3. Revisit `sheinlogin` constructor boundaries only if runtime pressure appears

If future work needs to inject an externally managed Redis store into `sheinlogin.Service`, that should be treated as a separate design slice, not folded into `Phase 3B`.

Right now there is not enough evidence to pay that complexity cost.

## Verification Summary

The final `Phase 3B` verification that passed on this branch was:

```powershell
go test ./internal/listingkit/httpapi ./internal/listingkit/... -count=1
go test ./internal/app/httpapi -count=1
go test ./internal/app/runtime -count=1
go test ./internal/sheinlogin/... -count=1
go test ./cmd/listingkit-temporal-worker -count=1
```

Notes:

- `./cmd/listingkit-temporal-worker` currently has no test files, so this is effectively a compile/build validation.

## Current Branch Notes

The main `Phase 3B` commits are:

- `04e69dfb` `docs: add framework phase3b plan`
- `14045d95` `refactor: split listingkit repository assembly`
- `71b5390b` `refactor: split listingkit runtime assembly`
- `e0953848` `refactor: extract sds bootstrap support helpers`
- `a5c43f18` `refactor: extract shein redis bootstrap support`
- `717e7b73` `test: lock bootstrap decomposition and reuse boundaries`

## Recommendation

Mark `Phase 3B` complete.

Do not keep pulling tiny bootstrap helpers outward just because the new seams are visible now. The biggest architectural win of this phase is already in place:

- ListingKit bootstrap is decomposed by responsibility
- app-layer code no longer owns SDS or Redis bootstrap mechanics directly
- the remaining app-layer support logic is operational rather than feature-defining

If we continue, the better next step is to choose a new hotspot based on actual change pressure, not on residual symmetry alone.
