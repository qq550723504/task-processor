# Task Processor Framework Phase 3 Scope Recommendation

## Recommendation

`Phase 3` should focus on **ListingKit runtime input ownership**, not additional Temporal abstraction work.

The highest-value next hotspot is:

- moving app-owned ListingKit runtime input assembly out of [internal/app/httpapi/listingkit_support.go](/D:/code/task-processor/internal/app/httpapi/listingkit_support.go:1)
- reducing how much `internal/app/httpapi` still knows about ListingKit-specific repositories, hooks, SDS support, SHEIN cookie wiring, and runtime toggles

In short:

- `Phase 2C` already made Temporal startup modular enough for now
- `Phase 3` should make ListingKit runtime construction more feature-owned

## Why This Is The Right Next Step

After `Phase 2A`, `Phase 2B`, and `Phase 2C`, the biggest remaining asymmetry is no longer:

- route registration
- worker pool registration
- Temporal worker startup registration

Those three are now on the module-registration path.

The biggest remaining asymmetry is that the app layer still prepares a large ListingKit-specific runtime input object and several ListingKit-specific support builders before handing control back to the feature package.

That means the app layer still knows too much about:

- ListingKit repository builder sets
- ListingKit hook builder sets
- SDS sync service bootstrap
- SDS baseline provider bootstrap
- SHEIN cookie-store bootstrap
- in-process Temporal worker enablement as a ListingKit runtime input concern

This is the next root-cause hotspot because it keeps runtime ownership split across:

- app-layer input assembly
- feature-owned service construction
- adapter/runtime helpers

The code is cleaner than before, but the responsibility line is still not where we ultimately want it.

## Current Hotspot

The main hotspot is:

- [internal/app/httpapi/listingkit_support.go](/D:/code/task-processor/internal/app/httpapi/listingkit_support.go:1)

This file currently does several different jobs:

1. builds ListingKit runtime inputs
2. builds ListingKit repository-builder bundles
3. builds ListingKit hook-builder bundles
4. bootstraps ListingKit-specific SDS support
5. bootstraps ListingKit-specific SHEIN cookie support
6. decides runtime flags such as in-process Temporal enablement

That is not “bad code” in isolation. It was a reasonable staging layer during `Phase 1` and `Phase 2`.

But as a long-term boundary, it means the app layer still owns too much of ListingKit’s runtime contract.

## Candidate Phase 3 Directions

There are three realistic directions from the current branch state.

### Option 1: ListingKit runtime input ownership

Move more of the logic in `internal/app/httpapi/listingkit_support.go` into feature-owned builders inside `internal/listingkit/httpapi`.

This would likely mean:

- feature-owned builders for repository and hook bundles
- feature-owned construction of SDS support collaborators
- feature-owned construction of SHEIN runtime support collaborators
- a thinner app-layer handoff that mostly passes shared runtime dependencies

**Pros**

- directly reduces the biggest remaining app-layer feature leak
- keeps work aligned with the existing feature-owned builder direction
- improves future maintainability of ListingKit runtime surfaces
- likely reduces future changes in `internal/app/httpapi`

**Cons**

- touches a large feature with many collaborators
- needs careful staging to avoid breaking existing ListingKit tests

**Recommendation:** `Yes`

This is the best `Phase 3` target.

### Option 2: Temporal adapter generalization

Generalize [internal/app/runtime/temporal_runtime.go](/D:/code/task-processor/internal/app/runtime/temporal_runtime.go:1) into a broader adapter-owned Temporal runtime helper.

This would mean:

- more generic naming and startup conventions
- broader worker/client helper abstractions
- possibly a more reusable Temporal runtime surface for future modules

**Pros**

- looks architecturally neat
- may help if more modules adopt Temporal soon

**Cons**

- currently speculative
- only ListingKit uses this real production path today
- easy to over-abstract before a second user exists

**Recommendation:** `Not yet`

This should wait for a second real Temporal consumer.

### Option 3: Unified runtime manifest across all contribution types

Introduce a broader feature runtime manifest that combines:

- routes
- worker pools
- Temporal workers
- maybe future health or scheduler contributions

**Pros**

- could make runtime composition more uniform
- could simplify future multi-surface feature registration

**Cons**

- currently higher abstraction than the codebase needs
- risks creating a large “manifest model” before multiple modules need it
- likely to increase design surface without immediate delivery value

**Recommendation:** `Defer`

This should wait for more than one feature needing the same cross-surface runtime shape.

## Why Not Continue Temporal Work Immediately

`Phase 2C` already solved the real Temporal root cause:

- standalone app entrypoints no longer directly start ListingKit Temporal internals
- ListingKit now owns its Temporal runtime contribution
- the kernel registry can collect Temporal starters

What remains in Temporal is mostly:

- adapter specialization
- naming policy
- possible future reuse

Those are all second-order concerns right now.

By contrast, ListingKit runtime input ownership is still a first-order concern because it affects present-day composition boundaries every time ListingKit runtime dependencies evolve.

## Suggested Phase 3 Goal

The concrete `Phase 3` goal should be:

> Move ListingKit runtime support assembly behind feature-owned builders so `internal/app/httpapi` no longer owns ListingKit-specific repository bundles, hook bundles, SDS support bootstrap, and SHEIN runtime support bootstrap.

That is specific enough to implement incrementally and broad enough to remove the largest remaining app-layer feature leak.

## Suggested Phase 3 Success Criteria

`Phase 3` should be considered successful when:

1. `internal/app/httpapi/listingkit_support.go` is materially smaller or removed
2. ListingKit runtime input assembly is primarily feature-owned
3. app-layer code no longer needs to know ListingKit-specific repository-builder and hook-builder sets
4. SDS support and SHEIN runtime support for ListingKit are built through feature-owned or adapter-owned seams rather than app-local helper chains
5. existing HTTP, worker-pool, and Temporal runtime behavior remains compatible

## Suggested Non-Goals For Phase 3

To keep the next slice disciplined, `Phase 3` should explicitly avoid:

- building a generic repo-wide workflow engine
- redesigning ListingKit service contracts
- introducing a universal runtime manifest
- moving all ListingKit collaborators into new directories in one pass
- rewriting `internal/app/runtime/temporal_runtime.go` unless another real consumer appears

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/app/httpapi/listingkit_support.go](/D:/code/task-processor/internal/app/httpapi/listingkit_support.go:1)
- [internal/listingkit/httpapi/runtime_builder.go](/D:/code/task-processor/internal/listingkit/httpapi/runtime_builder.go:1)
- [internal/listingkit/httpapi/bootstrap.go](/D:/code/task-processor/internal/listingkit/httpapi/bootstrap.go:1)
- [internal/listingkit/httpapi/temporal_runtime.go](/D:/code/task-processor/internal/listingkit/httpapi/temporal_runtime.go:1)

The design pressure should be:

- thinner app-layer handoff
- thicker feature-owned runtime builder boundary
- no speculative generic runtime framework

## Recommendation Summary

Proceed to `Phase 3`, but scope it narrowly:

- choose **ListingKit runtime input ownership** as the next hotspot
- defer broader Temporal adapterization until there is another real Temporal consumer
- defer a unified runtime manifest until multiple features need the same contribution categories

That keeps the work aligned with actual complexity in the codebase rather than architectural symmetry for its own sake.
