# Task Processor Framework Phase 3A Checkpoint

## Status

`Phase 3A` is functionally complete for the intended slice.

This phase was not about rewriting ListingKit internals or inventing a new runtime framework. The goal was narrower:

1. stop app-layer code from owning ListingKit repository/hook bundle shaping
2. move ListingKit runtime-support shaping into the feature package
3. make HTTP runtime, side-entry runtime, and standalone Temporal runtime consume the same feature-owned support boundary

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. ListingKit now owns a feature-level runtime support contract

ListingKit exposes a dedicated support contract through:

- [internal/listingkit/httpapi/runtime_support.go](/D:/code/task-processor/internal/listingkit/httpapi/runtime_support.go:1)
- [internal/listingkit/httpapi/runtime_support_shein.go](/D:/code/task-processor/internal/listingkit/httpapi/runtime_support_shein.go:1)
- [internal/listingkit/httpapi/runtime_support_test.go](/D:/code/task-processor/internal/listingkit/httpapi/runtime_support_test.go:1)

That contract now owns:

- repository-builder bundle shaping
- hook-builder bundle shaping
- SHEIN runtime helper shaping
- SDS collaborator handoff

The app layer no longer needs to know ListingKit’s full repository set, hook set, or SHEIN runtime helper recipe.

### 2. ListingKit runtime builders now consume the support contract directly

Both ListingKit runtime paths now flow through the same support boundary:

- [internal/listingkit/httpapi/runtime_builder.go](/D:/code/task-processor/internal/listingkit/httpapi/runtime_builder.go:1)
- [internal/listingkit/httpapi/temporal_runtime.go](/D:/code/task-processor/internal/listingkit/httpapi/temporal_runtime.go:1)

This matters because `BuildRuntimeModule(...)` and `BuildTemporalRuntime(...)` no longer depend on app-layer code preassembling:

- `BuildServiceRepositories`
- `BuildServiceHooks`
- SDS runtime collaborators on the legacy top-level path

The runtime-input contract is now feature-owned even when app-layer code still prepares startup-time collaborators.

### 3. App-layer ListingKit support has been reduced to app-owned prerequisites

The app-owned support hotspot is now materially smaller:

- [internal/app/httpapi/listingkit_support.go](/D:/code/task-processor/internal/app/httpapi/listingkit_support.go:1)

It still prepares:

- SHEIN cookie-store bootstrap
- SDS sync-service bootstrap
- SDS baseline remote-provider bootstrap
- runtime toggles and shared collaborators

But it no longer shapes ListingKit’s feature-owned repository and hook bundles. Its job is now “prepare collaborators and state,” not “define the ListingKit runtime contract.”

### 4. Standalone Temporal entry now uses the same runtime-support handoff

The standalone ListingKit Temporal worker path now reuses the narrowed runtime input path:

- [internal/app/httpapi/listingkit_temporal_worker.go](/D:/code/task-processor/internal/app/httpapi/listingkit_temporal_worker.go:1)
- [cmd/listingkit-temporal-worker/main.go](/D:/code/task-processor/cmd/listingkit-temporal-worker/main.go:1)

This is important because the side-entry no longer bypasses the new support boundary with a direct `BuildServiceInput` assembly path.

### 5. Guardrails now lock the ownership boundary

The app-layer boundary is explicitly locked by:

- [internal/app/httpapi/phase3a_listingkit_boundary_test.go](/D:/code/task-processor/internal/app/httpapi/phase3a_listingkit_boundary_test.go:1)
- [internal/app/httpapi/runtime_deps_test.go](/D:/code/task-processor/internal/app/httpapi/runtime_deps_test.go:1)

These tests do two useful things:

- prevent app-layer repo/hook bundle shaping from returning
- verify that SDS collaborators now flow through `RuntimeSupport` instead of legacy top-level runtime fields

## Acceptance Check

`Phase 3A` was meant to prove four things:

1. ListingKit can own its runtime-support bundle shaping
2. app-layer code no longer owns ListingKit repository/hook bundle definitions
3. SDS runtime collaborators can move through the same feature-owned support contract
4. HTTP runtime, side-entry runtime, and standalone Temporal runtime still work with that ownership split

All four are now true.

More concretely:

- ListingKit owns the support contract and its bundle shaping
- app-layer support code only prepares collaborators and cached state
- SDS support no longer requires app-layer code to shape a full ListingKit runtime contract
- the standalone Temporal entry no longer uses a special legacy `BuildServiceInput` path

## What This Phase Did Not Try To Solve

### 1. It did not redesign ListingKit service internals

Files such as:

- [internal/listingkit/httpapi/bootstrap.go](/D:/code/task-processor/internal/listingkit/httpapi/bootstrap.go:1)
- [internal/listingkit/service.go](/D:/code/task-processor/internal/listingkit/service.go:1)

still contain significant internal assembly and service orchestration logic.

That is acceptable. `Phase 3A` was about runtime-input ownership, not deeper service decomposition.

### 2. It did not create a repo-wide runtime manifest

There is still no global “one struct for every runtime contribution” abstraction.

That is still the right call. We had real change pressure around ListingKit ownership, not enough evidence for a broad new manifest.

### 3. It did not remove all ListingKit-specific bootstrap logic from app layer

The app layer still owns bootstrap logic that is genuinely app/runtime-local:

- initializing Redis-backed cookie storage
- initializing SDS clients from app config
- keeping cached support state on `runtimeDeps`

That is acceptable because those are startup responsibilities, not feature-contract ownership.

## Residual Responsibilities Still Present

### `listingkit_support.go` still owns app-local bootstrap and caching

The file still contains:

- cookie-store lazy init
- SDS sync-service bootstrap
- SDS baseline remote-provider bootstrap
- cached support state reuse through `runtimeDeps`

This is now a much healthier responsibility set, but it is still the main app-layer ListingKit hotspot.

### `runtimeDeps` still carries ListingKit-specific support cache state

- [internal/app/httpapi/types.go](/D:/code/task-processor/internal/app/httpapi/types.go:1)
- [internal/app/httpapi/runtime.go](/D:/code/task-processor/internal/app/httpapi/runtime.go:1)

This remains acceptable for now because it is operational state, not feature contract definition.

### ListingKit runtime surfaces are still spread across several builders

ListingKit currently exposes:

- HTTP/runtime module building
- Temporal runtime building
- runtime support building

That is much better than before, but still not a single unified runtime artifact. This is not a bug; it is simply the next boundary question if reuse pressure grows.

## What Should Move To The Next Phase

If we continue, the next high-value work should not be “more tiny helper extraction” inside app/httpapi. The better next steps are:

### 1. Decide whether ListingKit runtime support should stay app-bootstrapped or move closer to adapters

If cookie-store and SDS client bootstrap start appearing in more than one feature, that is the signal to consider pushing more of this logic toward adapter-owned helpers.

Until then, avoid speculative adapter abstraction.

### 2. Revisit ListingKit internal bootstrap decomposition

If ListingKit keeps changing quickly, the next worthwhile hotspot is likely inside:

- [internal/listingkit/httpapi/bootstrap.go](/D:/code/task-processor/internal/listingkit/httpapi/bootstrap.go:1)

That is where the next real complexity concentration still lives.

### 3. Consider a broader runtime artifact only if another feature needs the same support shape

Right now ListingKit is the only feature with this exact runtime-support pressure.

Do not introduce a generic support manifest until at least one more feature needs:

- builder bundle shaping
- startup collaborator handoff
- standalone side-entry runtime reuse

## Verification Summary

The final `Phase 3A` verification that passed on this branch was:

```powershell
go test ./internal/app/httpapi -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/... -count=1
go test ./internal/app/runtime -count=1
go test ./cmd/listingkit-temporal-worker -count=1
```

Notes:

- `./cmd/listingkit-temporal-worker` currently has no test files, so this is effectively a compile/build validation.

## Current Branch Notes

The main `Phase 3A` commits are:

- `259c70c2` `docs: add framework phase3a plan`
- `e7736c82` `feat: add listingkit runtime support builder`
- `49054dbc` `refactor: narrow app-owned listingkit support shaping`
- `b5e64a6c` `refactor: move listingkit sds support shaping into feature package`
- `744adb51` `test: lock listingkit runtime ownership boundary`

## Recommendation

Mark `Phase 3A` complete.

Do not keep squeezing this slice for more symmetry. The biggest ownership bug has already been fixed:

- app-layer code no longer defines the ListingKit runtime contract

If we continue, the better next step is to choose a new hotspot based on actual change pressure, most likely:

1. deeper ListingKit bootstrap decomposition
2. adapter-oriented bootstrap reuse if another feature starts needing the same setup path
3. a new runtime contribution slice only if a second feature creates matching pressure
