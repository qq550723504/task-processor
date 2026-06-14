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

### `internal/marketplace/shein/publishing`

Stable responsibility:

- SHEIN publishing package model
- Request draft generation
- Preview product adaptation
- Category / attribute / sale-attribute resolution
- Managed resolver access
- Compatibility-owned thin exports for stable marketplace publishing helpers during the transition from `internal/publishing/shein`

This is the target canonical home for SHEIN publishing logic. `listingkit` should not re-introduce publishing builders here.

Current guardrail:

- `internal/marketplace/shein/publishing` must not depend on `internal/listingkit` or root runtime wiring packages.

### `internal/marketplace/shein/workspace`

Stable responsibility:

- Inspection
- Workspace overview / status overview projection
- Editor progress / dirty hints helper projection
- Status / workspace overview
- Submit readiness / checklist
- Repair center / plan / session
- Editor context, editor recommendations, effects, dirty hints, progress
- Editor revision model / skeleton / minimal revision
- Revision diff
- Restore draft / restore preview / restore detail / compare
- Revision validation payload
- Revision success payload
- Compatibility-owned thin exports for stable workspace helpers during the transition from `internal/workspace/shein`

This is the target canonical home for SHEIN client workspace behavior.

### Legacy compatibility shells

- `internal/publishing/shein`
- `internal/workspace/shein`

Current rule:

- old SHEIN packages may remain as compatibility shells,
- new stable marketplace-facing helpers should prefer landing under `internal/marketplace/shein/*`,
- old paths should get thinner over time instead of receiving fresh domain logic by default.

Current migrated helper slices:

- `publishing/pricing_policy`
- `workspace/state`
- `workspace/editor_dirty_hints`
- `workspace/editor_progress`
- `workspace/editor_recommendations`
- `workspace/editor_effects`
- `workspace/editor_context_model`
- `workspace/editor_context_builder`
- `workspace/editor_revision_model`
- `workspace/editor_revision_skeleton`
- `workspace/editor_revision_from_context`
- `workspace/editor_revision_minimal`
- `workspace/readiness`
- `workspace/readiness_guidance`
- `workspace/repair`
- `workspace/revision_diff`
- `workspace/revision_apply_changes`
- `workspace/revision_history_compare`
- `workspace/revision_restore_draft`
- `workspace/revision_restore_request`
- `workspace/revision_restore_preview`
- `workspace/history_detail`
- `workspace/revision_history_restore_detail`
- `workspace/revision_history_detail_builder`
- `workspace/revision_restore_presentation`
- `workspace/revision_field_validation`
- `workspace/revision_validation`
- `workspace/revision_success`
- `workspace/overview`
- `workspace/inspection`

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

These are still okay inside `listingkit` because they serve as package-local adapters from service/API models into `internal/marketplace/shein/workspace` or `internal/marketplace/shein/publishing`:

- `internal/listingkit/revision_workspace_bridge.go`
- `internal/listingkit/shein_workspace_types_bridge.go`
- `internal/listingkit/shein_workspace_editor_bridge.go`
- `internal/listingkit/shein_workspace_submit_bridge.go`
- `internal/listingkit/shein_workspace_repair_bridge.go`
- `internal/listingkit/shein_workspace_revision_bridge.go`
- `internal/listingkit/shein_workspace_inspection_bridge.go`
- `internal/listingkit/shein_workspace_readiness_support.go`
- `internal/listingkit/shein_repair_support.go`

These files should stay thin. If they grow domain rules, that logic should move back into `internal/marketplace/shein/workspace` or `internal/marketplace/shein/publishing`.

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

## Submission Extraction Checkpoint

`internal/listing/submission` is now a real migration target rather than an empty placeholder.

Current generic seams already moved there:

- refresh-status orchestration
- pending-task requeue orchestration
- immediate blocked-task recovery orchestration
- batch blocked-task recovery orchestration
- direct-submit phase orchestration
- prepared-payload phase orchestration (`prepare -> upload -> pre-validate`)
- remote-submit attempt orchestration (`prepare state -> execute -> shape result`)
- post-success persistence orchestration (`persist result/phase -> complete attempt -> remember -> persist success`)
- failure-record persistence orchestration (`record failed attempt/event`)

Current rule:

- `internal/listing/submission` should own generic flow structure,
- `internal/listingkit` should keep compatibility DTO mapping, repository bridging, SHEIN readiness gates, failure-return semantics, refresh-specific completion logic, and ListingKit-specific retryable-block persistence behavior.

## Studio Extraction Checkpoint

`internal/listing/studio` is no longer just a placeholder package.

Current generic seams already moved there:

- studio batch-run service skeleton (`create/get/list/cancel`)
- studio batch-detail read skeleton (`read graph -> fallback -> ensure graph -> project detail`)
- studio batch review skeleton (`ensure batch -> replace reviews -> reload detail`)
- studio batch-draft read/delete skeleton (`gallery/list/get/delete`)
- studio session ensure/get skeleton (`ensure/get`)
- studio session async-job sync skeleton (`sync async job -> persist session state`)
- studio session generation-metadata patch skeleton (`status/job/error` metadata-only updates)
- studio session review/task-metadata patch skeleton (`approved_design_ids/created_tasks` metadata-only updates)
- studio session general-metadata patch skeleton (`load session -> apply listingkit adapter patch -> persist`)
- studio batch-run completion skeleton (`cancel unfinished items -> count item statuses -> resolve final status`)

Current rule:

- `internal/listing/studio` should own stable studio service flow and internal DTOs,
- `internal/listingkit` should keep API shell models, repository implementations, mixed-field session field assignment adapters, upsert-heavy write logic, batch execution/task-creation orchestration, batch graph materialization details, start-run wiring, concrete batch-run executor loops, and error translation until more studio services settle.

Current guardrail:

- `internal/listing/studio` must not depend on `internal/listingkit`, SHEIN marketplace/workspace/publishing packages, or root runtime/integration wiring.

## Preview Extraction Checkpoint

`internal/listing/preview` is already an active read-model/service seam rather than a new target to recreate.

Current rule:

- `internal/listing/preview` owns generic preview read orchestration and preview model helpers,
- `internal/listingkit` keeps platform-specific preview aggregation and decoration adapters,
- `internal/listing/preview` must not depend back on `internal/listingkit` or SHEIN marketplace/workspace compatibility packages.

Current guardrail:

- ListingKit preview service construction is pinned to `previewdomain.TaskPreviewService`.
- The preview package has a boundary guard against `listingkit`, `marketplace/shein`, `publishing/shein`, and `workspace/shein` imports.

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
- Root `internal/listingkit` must not gain new business rules that can live in `internal/marketplace/shein/publishing` or `internal/marketplace/shein/workspace`.

1. If the change is about SHEIN category/attribute/sale-attribute publishing, change `internal/marketplace/shein/publishing`.
2. If the change is about SHEIN inspection/editor/repair/revision UX, change `internal/marketplace/shein/workspace`.
3. If the change is about task flow, request normalization, persistence, preview/export aggregation, or API services, change `listingkit`.
4. Avoid creating new `shein_*` single-purpose helper files in `listingkit` unless they are clearly facade-only.
5. Prefer extending the existing grouped bridge/support files over creating another thin wrapper file.

## Suggested Next Milestone

The next meaningful milestone is not another migration sprint. It should be one of:

1. Continue with another low-risk `listing/studio` service seam before touching batch-generation orchestration.
2. Start the same publishing/workspace split for `TEMU`.
3. Build an image-asset-oriented layer on top of `catalog + asset`.

## Post-SHEIN Direction

Decision: freeze the current SHEIN package split and prioritize enforcing placement rules over more directory movement.

Reason: `internal/marketplace/shein/publishing` and `internal/marketplace/shein/workspace` already exist as stable homes, while root `listingkit` still has a large compatibility surface. The highest-value next step is to stop new business rules from drifting back into the facade, not to start another broad package migration.

Non-goals: this does not start a TEMU split, rename more packages, or move stable API shell types out of `internal/listingkit`.

Current exception: `internal/publishing/shein` remains a legacy compatibility/model package and is still allowed where existing submission and model flows depend on it. The guardrail applies to new canonical marketplace publishing logic flowing back into root `listingkit` or runtime assembly, not to completing a broad SHEIN model migration in this slice.
