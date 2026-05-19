# ListingKit Refactor Status

## Goal

The current refactor direction is:

1. `catalog` owns product facts.
2. `asset` owns reusable visual assets.
3. `publishing/*` owns platform publishing logic.
4. `workspace/*` owns client-facing workspace logic.
5. `listingkit` is being reduced to orchestration and API-facing facade behavior.

This document records the current boundary after the first large refactor phase so later changes can avoid drifting back into `listingkit`.

## Current Stable Boundaries

### `internal/catalog`

Stable responsibility:

- Canonical product facts
- Product-level normalized data for downstream publishing/workspace use

Current entry points:

- `internal/catalog/model.go`
- `internal/catalog/from_canonical.go`

### `internal/asset`

Stable responsibility:

- Reusable image/visual asset bundle
- Bridging canonical product images and `productimage` output into asset snapshots

Current entry points:

- `internal/asset/model.go`
- `internal/asset/from_productimage.go`

### `internal/publishing/shein`

Stable responsibility:

- SHEIN publishing package model
- Request draft generation
- Preview product adaptation
- Category / attribute / sale-attribute resolution
- Managed resolver access

This is the canonical home for SHEIN publishing logic. `listingkit` should not re-introduce publishing builders here.

### `internal/workspace/shein`

Stable responsibility:

- Inspection
- Status / workspace overview
- Submit readiness / checklist
- Repair center / plan / session
- Editor context, editor recommendations, effects, dirty hints, progress
- Editor revision model / skeleton / minimal revision
- Revision diff
- Restore draft / restore preview / restore detail / compare
- Revision validation payload
- Revision success payload

This is now the canonical home for SHEIN client workspace behavior.

## What `internal/listingkit` Still Owns

### Stable and expected to remain

- Task orchestration
- Workflow execution
- Repository/service facade layer
- Preview/export HTTP-oriented aggregation
- Revision/history service facade
- Cross-platform request/result shell models

Representative files:

- `internal/listingkit/workflow.go`
- `internal/listingkit/service.go`
- `internal/listingkit/processor.go`
- `internal/listingkit/preview_builder.go`
- `internal/listingkit/export_builder.go`
- `internal/listingkit/service_revision.go`
- `internal/listingkit/service_history.go`

### Acceptable facade bridges

These are still okay inside `listingkit` because they serve as package-local adapters from service/API models into `workspace/shein` or `publishing/shein`:

- `internal/listingkit/revision_workspace_bridge.go`
- `internal/listingkit/shein_workspace_types_bridge.go`
- `internal/listingkit/shein_workspace_editor_bridge.go`
- `internal/listingkit/shein_workspace_submit_bridge.go`
- `internal/listingkit/shein_workspace_repair_bridge.go`
- `internal/listingkit/shein_workspace_revision_bridge.go`
- `internal/listingkit/shein_workspace_inspection_bridge.go`
- `internal/listingkit/shein_workspace_readiness_support.go`
- `internal/listingkit/shein_repair_support.go`

These files should stay thin. If they grow domain rules, that logic should move back into `workspace/shein` or `publishing/shein`.

## Areas Considered Phase-1 Complete

The following domains are effectively moved out of `listingkit` and should be treated as complete for this phase:

- SHEIN publishing
- SHEIN workspace overview
- SHEIN submit readiness/checklist protocol
- SHEIN repair center protocol
- SHEIN editor protocol
- SHEIN revision diff / restore preview / history compare
- SHEIN revision success payload

Further cleanup here should prefer consolidation, naming cleanup, and tests over more structural movement unless a new architectural need appears.

## Areas Intentionally Left In Place

These remain in `listingkit` for now and do not need immediate migration:

- `temu_mapper.go`
- `walmart_mapper.go`
- Amazon draft integration
- Generic task/result API models
- History/revision service entrypoints

Reason:

- They are either platform-specific but not yet migrated by design, or they are true facade/orchestration code.

## Current Smells To Watch

### 1. Preview builder is still a large aggregation hub

`internal/listingkit/preview_builder.go` is now mostly doing the right thing, but it remains the highest-density aggregation file. New workspace rules should not be added here directly.

### 2. `listingkit` still contains many revision/history facade files

This is acceptable for now because these files mostly serve service/API response assembly. The key rule is:

- no new SHEIN domain rules should be added there
- only request/response composition and service glue should remain

### 3. Image processing request still uses a fixed marketplace

`internal/listingkit/workflow.go` still sends `"amazon"` in `toImageProcessRequest(...)`.

That is outside the current refactor boundary, but it remains an architectural limitation for future image asset strategy work.

## Recommended Rules For Next Changes

## Phase-2 Boundary Goal

- `internal/listingkit/workflow` becomes the implementation home for task orchestration helpers and policies.
- `internal/listingkit/workspace` becomes the implementation home for facade-level workspace composition that is not SHEIN-domain-specific.
- Root `internal/listingkit` stays as compatibility facade and service entrypoint.
- Root `internal/listingkit` must not gain new business rules that can live in `publishing/shein` or `workspace/shein`.

1. If the change is about SHEIN category/attribute/sale-attribute publishing, change `publishing/shein`.
2. If the change is about SHEIN inspection/editor/repair/revision UX, change `workspace/shein`.
3. If the change is about task flow, request normalization, persistence, preview/export aggregation, or API services, change `listingkit`.
4. Avoid creating new `shein_*` single-purpose helper files in `listingkit` unless they are clearly facade-only.
5. Prefer extending the existing grouped bridge/support files over creating another thin wrapper file.

## Suggested Next Milestone

The next meaningful milestone is not another migration sprint. It should be one of:

1. Start the same publishing/workspace split for `TEMU`.
2. Build an image-asset-oriented layer on top of `catalog + asset`.
3. Freeze the current SHEIN architecture and focus on product-facing capabilities instead of further structural churn.

## Post-SHEIN Direction

Decision: freeze the current SHEIN package split and prioritize enforcing placement rules over more directory movement.

Reason: `publishing/shein` and `workspace/shein` already exist as stable homes, while root `listingkit` still has a large compatibility surface. The highest-value next step is to stop new business rules from drifting back into the facade, not to start another broad package migration.

Non-goals: this does not start a TEMU split, rename more packages, or move stable API shell types out of `internal/listingkit`.
