# Task Processor Framework Phase 2C Checkpoint

## Status

`Phase 2C` is functionally complete for the intended slice.

The original goal was not to build a generic workflow framework. The goal was narrower and more practical:

1. let the kernel registry collect Temporal runtime contributions
2. let ListingKit own its Temporal runtime contribution
3. stop standalone app-layer entrypoints from directly assembling and starting the ListingKit Temporal worker

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Kernel module registry now supports Temporal worker starters

The kernel registry no longer only collects:

- HTTP routes
- named worker pools
- workflow handler names

It now also collects named Temporal worker starters through:

- [internal/kernel/module/interfaces.go](/D:/code/task-processor/internal/kernel/module/interfaces.go:1)
- [internal/kernel/module/registry.go](/D:/code/task-processor/internal/kernel/module/registry.go:1)
- [internal/kernel/module/registry_test.go](/D:/code/task-processor/internal/kernel/module/registry_test.go:1)

This matters because runtime assembly is no longer forced to special-case Temporal startup outside the registry path.

### 2. ListingKit now exposes a feature-owned Temporal runtime contribution

ListingKit can now produce a dedicated Temporal runtime result through:

- [internal/listingkit/httpapi/temporal_runtime.go](/D:/code/task-processor/internal/listingkit/httpapi/temporal_runtime.go:1)
- [internal/listingkit/httpapi/bootstrap_temporal_module.go](/D:/code/task-processor/internal/listingkit/httpapi/bootstrap_temporal_module.go:1)
- [internal/listingkit/httpapi/temporal_runtime_test.go](/D:/code/task-processor/internal/listingkit/httpapi/temporal_runtime_test.go:1)

This runtime contribution now owns three things together:

- the worker-capable service boundary
- the workflow-name registration surface
- the Temporal worker starter closure

That is the main architectural win of this phase. App-layer code no longer has to understand the internal `BuildService(...) -> StartListingKitSheinPublishTemporalWorker(...)` chain in order to run ListingKit’s Temporal runtime.

### 3. Standalone Temporal startup now goes through a registry-backed runtime bundle

The standalone ListingKit Temporal worker path now builds and starts a bundle from registered modules instead of directly calling ListingKit-specific startup functions.

Key files:

- [internal/app/runtime/temporal_bundle.go](/D:/code/task-processor/internal/app/runtime/temporal_bundle.go:1)
- [internal/app/runtime/temporal_bundle_test.go](/D:/code/task-processor/internal/app/runtime/temporal_bundle_test.go:1)
- [internal/app/httpapi/listingkit_temporal_worker.go](/D:/code/task-processor/internal/app/httpapi/listingkit_temporal_worker.go:1)

The app-layer entrypoint still chooses the runtime mode, but it no longer owns the Temporal worker startup recipe itself.

### 4. A guardrail now prevents the old direct startup path from returning

The old anti-pattern is now explicitly locked by:

- [internal/app/httpapi/phase2c_temporal_boundary_test.go](/D:/code/task-processor/internal/app/httpapi/phase2c_temporal_boundary_test.go:1)

This is intentionally narrow. It does not try to freeze all future Temporal evolution. It only prevents the specific regression where app-layer code directly reintroduces:

- `BuildService(...)`
- `StartListingKitSheinPublishTemporalWorker(...)`

into the standalone entrypoint.

## Acceptance Check

`Phase 2C` was meant to prove three things:

1. Temporal worker starters can be registered through the kernel path
2. ListingKit can own its own Temporal runtime contribution
3. standalone Temporal startup can consume a registry-built runtime bundle instead of direct feature bootstrap internals

All three are now true.

More concretely:

- the kernel now has a runtime contribution path for Temporal starters
- ListingKit’s Temporal registration surface is feature-owned
- the standalone `cmd/listingkit-temporal-worker` path is no longer coupled to app-layer direct Temporal startup wiring

## What This Phase Did Not Try To Solve

This is important because `Phase 2C` was intentionally scoped.

### 1. It did not create a generic repo-wide workflow runtime framework

There is still no general-purpose “workflow platform” abstraction for every module in the repository.

That is acceptable and intentional.

The current codebase only had one real production-grade Temporal runtime path worth normalizing right now: ListingKit’s.

### 2. It did not remove ListingKit-owned Temporal knowledge from ListingKit

Workflow names such as:

- `PublishWorkflow`
- `StandardProductWorkflow`
- `PlatformAdaptWorkflow`

still belong to ListingKit’s feature package. That is the correct ownership for now.

This phase moved startup and registration boundaries. It did not try to make ListingKit’s workflow contracts generic.

### 3. It did not replace `internal/app/runtime/temporal_runtime.go`

That file still owns the concrete Temporal SDK dial/start mechanics:

- client dialing
- env-based enablement
- worker startup
- worker shutdown

That is still acceptable because it behaves like an adapter/runtime helper, not app-layer business composition.

## Residual Responsibilities Still Present

These are not blockers for `Phase 2C` completion, but they are the main candidates for future work.

### `internal/app/httpapi/listingkit_support.go` still assembles ListingKit runtime inputs

- [internal/app/httpapi/listingkit_support.go](/D:/code/task-processor/internal/app/httpapi/listingkit_support.go:1)

This file still knows how to prepare the ListingKit runtime build input, including whether the in-process Temporal worker should be enabled.

That is acceptable for now because it is runtime-input assembly, not direct Temporal worker startup.

### `internal/app/runtime/temporal_runtime.go` is still ListingKit-specific

- [internal/app/runtime/temporal_runtime.go](/D:/code/task-processor/internal/app/runtime/temporal_runtime.go:1)

The runtime helper still has ListingKit-specific env names and startup functions.

This is a reasonable `Phase 3` candidate if another real Temporal runtime appears and needs the same path. Right now there is not enough evidence to justify generalizing it further.

### ListingKit HTTP/runtime build results are still split across multiple outputs

ListingKit now exposes:

- its HTTP/runtime module
- its Temporal runtime result
- its service bundle path

That is better than before, but it still means ListingKit’s runtime surfaces are not yet expressed as one fully unified feature runtime manifest.

This is a possible future improvement, but not necessary for the current framework milestone.

## What Should Move To Phase 3

If we continue, the highest-value next steps are not “more helper extraction.” They are boundary decisions driven by actual reuse pressure.

### 1. Decide whether Temporal runtime should stay ListingKit-specific or become adapter-owned

If another module adopts Temporal in a similar way, that will justify:

- shared Temporal runtime bundle conventions
- shared naming and enablement policy
- possibly an adapter-owned Temporal runtime registration helper

Until then, avoid speculative abstraction.

### 2. Revisit runtime input ownership for ListingKit

If `listingkit_support.go` keeps growing, the next step should be to push more of that runtime-input assembly back into ListingKit-owned builders instead of adding more app-local support helpers.

### 3. Consider a broader runtime-manifest shape only when multiple modules need it

Right now we have three meaningful contribution types:

- routes
- worker pools
- Temporal worker starters

Do not rush to introduce a large “everything runtime manifest” unless at least one more feature needs the same contribution categories.

## Verification Summary

The final `Phase 2C` verification that passed on this branch was:

```powershell
go test ./internal/kernel/module -count=1
go test ./internal/app/runtime ./internal/app/httpapi -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
go test ./cmd/listingkit-temporal-worker -count=1
```

Notes:

- `./cmd/listingkit-temporal-worker` currently has no test files, so this is effectively a compile/build validation.

## Current Branch Notes

The main `Phase 2C` commits are:

- `761f86cd` `docs: add framework phase2c temporal runtime plan`
- `c7ac0ca2` `feat: add temporal worker registration to module registry`
- `e0164323` `feat: add listingkit temporal runtime module`
- `c7070fd9` `refactor: build temporal runtime bundle from registered modules`
- `af3deb05` `test: lock temporal runtime registration boundary`

## Recommendation

Mark this `Phase 2C` slice complete.

Do not keep digging in Temporal runtime abstraction unless one of these becomes true:

1. another feature needs the same Temporal runtime registration path
2. ListingKit runtime input assembly starts growing back into app-layer complexity
3. adapter boundaries around Temporal become operationally painful

If none of those happens, the better next step is to pivot to a `Phase 3` checkpoint and choose a new hotspot based on actual change pressure instead of abstract symmetry.
