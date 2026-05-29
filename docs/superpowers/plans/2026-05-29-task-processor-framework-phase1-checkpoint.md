# Task Processor Framework Phase 1 Checkpoint

## Status

`Phase 1` is functionally complete.

The original goal was to introduce a minimal kernel module registry and move the HTTP runtime onto registered modules instead of direct global route wiring. That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Kernel registration primitives exist

The project now has a stable module registration surface in:

- [internal/kernel/module/interfaces.go](/D:/code/task-processor/internal/kernel/module/interfaces.go:1)
- [internal/kernel/module/registry.go](/D:/code/task-processor/internal/kernel/module/registry.go:1)

This gives HTTP runtime composition a real kernel-facing abstraction instead of relying on ad hoc bootstrap contracts.

### 2. HTTP routing is module-based in production

The production HTTP path now builds route descriptors from registered modules rather than direct handler family wiring.

Key files:

- [internal/app/httpapi/http_module.go](/D:/code/task-processor/internal/app/httpapi/http_module.go:1)
- [internal/app/httpapi/http_modules.go](/D:/code/task-processor/internal/app/httpapi/http_modules.go:1)
- [internal/app/httpapi/app.go](/D:/code/task-processor/internal/app/httpapi/app.go:1)
- [internal/app/httpapi/types.go](/D:/code/task-processor/internal/app/httpapi/types.go:73)

`buildBootstrap(...)` now produces an `httpFeatureComposition`, derives route modules from that composition, and builds the server from registered modules.

### 3. Feature packages own more of their HTTP registration boundary

Several HTTP features now expose prebuilt module results instead of requiring `internal/app/httpapi` to know both:

- how to build the handler
- how to mount the routes

This is now true for:

- prompt management
- SDS catalog
- task RPC
- SHEIN login
- SDS login

That reduced app-layer knowledge and moved feature routing ownership closer to the feature packages.

### 4. Bootstrap composition is thinner and more explicit

The app-layer bootstrap no longer mixes:

- runtime dependency creation
- handler construction
- route assembly
- local task health aggregation
- random feature-specific state caching

Those responsibilities were split across:

- [internal/app/httpapi/runtime.go](/D:/code/task-processor/internal/app/httpapi/runtime.go:1)
- [internal/app/httpapi/composition_builder.go](/D:/code/task-processor/internal/app/httpapi/composition_builder.go:1)
- [internal/app/httpapi/listingkit_feature_builder.go](/D:/code/task-processor/internal/app/httpapi/listingkit_feature_builder.go:1)
- [internal/app/httpapi/listingkit_support.go](/D:/code/task-processor/internal/app/httpapi/listingkit_support.go:1)

### 5. ListingKit side-entry setup is no longer copy-pasted

The temporal worker and ListingKit-focused E2E paths now reuse a shared feature setup helper instead of hand-rolling:

- product prerequisite build
- image prerequisite build
- deps attachment
- listingkit prerequisite build

That helper lives in:

- [internal/app/httpapi/listingkit_feature_builder.go](/D:/code/task-processor/internal/app/httpapi/listingkit_feature_builder.go:1)

### 6. Unused legacy HTTP helpers are out of production scope

The old handler-based route/server helpers were first moved to tests, then trimmed further until the unused legacy helper file was removed:

- deleted: [internal/app/httpapi/server_legacy_test.go](/D:/code/task-processor/internal/app/httpapi/server_legacy_test.go:1)

Production code no longer carries those test-only compatibility wrappers.

## Acceptance Check

`Phase 1` was meant to prove three things:

1. the kernel can own module registration
2. HTTP runtime can assemble from modules
3. business service construction can stay mostly intact while registration moves

All three are now true.

More concretely:

- adding a route family no longer requires hard-coding one more direct route append path in production HTTP bootstrap
- side-entry server scenarios can be assembled from the same composition/model path as the main runtime
- test-only handler-based compatibility helpers no longer leak into production files

## Residual Responsibilities Still In `internal/app/httpapi`

These are still acceptable for `Phase 1`, but they are the main reasons not to keep digging indefinitely here.

### `buildRuntimeDeps(...)` is still a large shared runtime assembly point

- [internal/app/httpapi/runtime.go](/D:/code/task-processor/internal/app/httpapi/runtime.go:23)

This is still the place where config, prompt initialization, OpenAI manager setup, tenant prompt store wiring, and shared resources come together. That is acceptable for now because `Phase 1` was about registration flow, not full runtime extraction.

### `httpFeatureCompositionBuilder` still sequences feature construction centrally

- [internal/app/httpapi/composition_builder.go](/D:/code/task-processor/internal/app/httpapi/composition_builder.go:46)

The sequencing is much clearer than before, but it is still app-owned orchestration. That is a good `Phase 2` target once we want modules to contribute more of their own initialization contracts.

### ListingKit support remains app-local

- [internal/app/httpapi/listingkit_support.go](/D:/code/task-processor/internal/app/httpapi/listingkit_support.go:1)

This file is much cleaner now, but it still means the app layer knows ListingKit-specific support assembly. That is also a `Phase 2` candidate, not a blocker for `Phase 1`.

## What Should Move To Phase 2

These are the next high-value items. They are intentionally not part of `Phase 1` completion.

### 1. Move more feature construction behind module-owned builders

Candidates:

- product enrich
- product image
- amazon listing
- listingkit main module

Right now they still use app-local `build*Module(...)` wrappers even though the surrounding composition is much better.

### 2. Shrink `runtimeDeps`

`runtimeDeps` is healthier than before, but it is still the biggest shared mutable assembly object in `internal/app/httpapi`.

The next step is not “delete it immediately”. The next step is to split it along ownership lines:

- kernel/runtime shared deps
- feature-specific bootstrap support
- optional side-entry setup helpers

### 3. Push ListingKit bootstrap knowledge out of app layer

Even after the current cleanup, ListingKit remains the biggest feature-specific bootstrap hotspot. `Phase 2` should continue the direction of:

- module-owned service builders
- module-owned runtime support
- fewer app-local ListingKit assembly details

### 4. Start extending module registration beyond HTTP routes

`Phase 1` established route registration. `Phase 2` should begin the same normalization for:

- worker/task registration
- workflow/Temporal registration
- possibly health/ops registration

## Stop Criteria

The branch is now at a point where continuing to “just extract one more helper” inside `Phase 1` will have diminishing returns.

That is the signal to stop phase-local cleanup and switch context to `Phase 2` planning.

## Current Branch Notes

Recent `Phase 1` endgame commits include:

- `31dd0523` `refactor: extract http feature composition builder`
- `1b7d781a` `refactor: make http composition state explicit`
- `e0d0bd16` `refactor: share listingkit side-entry setup`
- `8487324a` `refactor: route side-entry servers through composition`
- `55c86d3c` `test: remove unused legacy http route helpers`
- `60066848` `test: keep http handler helpers in test scope`

There are also unrelated uncommitted workspace changes that were intentionally left untouched:

- [cmd/product-listing-api/wrappers.go](/D:/code/task-processor/cmd/product-listing-api/wrappers.go:1)
- [internal/listingkit/shein_submit_freshness.go](/D:/code/task-processor/internal/listingkit/shein_submit_freshness.go:1)
- [internal/listingkit/shein_submit_freshness_test.go](/D:/code/task-processor/internal/listingkit/shein_submit_freshness_test.go:1)

## Recommendation

Mark `Phase 1` complete and start `Phase 2` from a new checkpoint, with scope centered on:

1. feature-owned builders for `product/image/amazon/listingkit`
2. smaller runtime dependency ownership
3. worker/workflow registration surfaces that mirror the HTTP module registration path
