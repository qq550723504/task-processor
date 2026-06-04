# Shein Studio Workbench Persistence Cleanup Design

## Context

The current Shein Studio workbench keeps two different concerns in the same state and persistence model:

- itemized batch truth, represented by `itemizedBatchDetail`
- draft and page interaction state, represented by `savedBatch`, `draft`, and workbench-local state

This overlap leaves transitional flat fields such as `designs`, `selectedIds`, `createdTasks`, and `generationJobs` acting as alternate owners for data that should now come from itemized batch detail. The result is unclear ownership during hydration, draft restore, and recent batch recovery.

The workbench already contains partial compatibility projection logic such as `projectItemizedBatchCompatibilityFields(detail)`, which shows the intended direction. This cleanup formalizes that direction and moves legacy flat fields behind a single adapter boundary.

## Goal

Clean up Shein Studio workbench persistence so that:

- `itemizedBatchDetail` is the only structural source of truth for batch results
- persisted draft and saved batch data keep only view-oriented state and user interaction inputs
- legacy flat fields remain readable for existing local data, but only through a compatibility adapter
- hydration and draft restore follow one deterministic merge order across the workbench

## Non-Goals

- no backend contract redesign for itemized batch detail
- no removal of all flat workbench fields in a single pass if the page still needs projected compatibility values
- no unrelated refactor outside Shein Studio workbench persistence and hydration flows

## Recommended Approach

Use option 2: clean up page state and persistence shape together, while keeping a compatibility adapter in `shein-studio-batches.ts`.

This approach gives the workbench a correct owner model without risking existing local drafts, recent batch flows, or hydration paths that still encounter old stored data.

## Ownership Model

The new model should be split into two persistent layers and one compatibility-only layer.

### 1. Itemized truth

`itemizedBatchDetail` owns:

- real batch structure
- generated designs and review outcomes
- approved selections and final result state
- any result data that should remain authoritative after hydration

### 2. Persisted view state

Persisted workbench draft and saved batch view data own:

- `selection`
- `prompt`
- `styleCount`
- `variationIntensity`
- `productImageCount`
- `productImagePrompt`
- `productImagePrompts`
- `artworkModel`
- `transparentBackground`
- `sheinStoreId`
- `imageStrategy`
- `groupedImageMode`
- `selectedSdsImages`
- `groupedSelections`
- `renderSizeImagesWithSds`
- grouped workspace view information
- `persistedUpdatedAt`
- `galleryRatioCheck`
- a small amount of metadata such as `draftUpdatedAt`, `generationError`, or `batchStatus` if still needed for restore and messaging

### 3. Legacy compatibility snapshot

Legacy flat fields from previously stored data remain readable, but they are no longer authoritative:

- `designs`
- `selectedIds`
- `createdTasks`
- `generationJobs`

These values should exist only as fallback snapshot data when there is not yet a hydrated `itemizedBatchDetail`.

## Type Design

The current `SheinStudioDraft = Omit<SheinStudioSavedBatch, ...>` relationship should be replaced with explicit persistence types so old flat result fields are not inherited by default.

Recommended type split:

- `SheinStudioPersistedBatchView`
- `SheinStudioPersistedDraft`
- `SheinStudioLegacyCompatibilitySnapshot`

Guidelines:

- `SheinStudioPersistedBatchView` contains only view state and page restore fields
- `SheinStudioPersistedDraft` extends the persisted view shape with draft-specific metadata
- `SheinStudioLegacyCompatibilitySnapshot` is only used by storage decode and hydration fallback
- workbench runtime state can still expose projected flat fields for rendering, but those fields must come from projection logic instead of persistence ownership

## Grouped Workspace Cleanup

`groups` must be included in the cleanup. Leaving grouped workspace untouched would preserve a second hidden owner path because grouped workspace still carries result-like fields such as:

- `designs`
- `selectedIds`
- `createdTasks`

Grouped workspace should be reduced to a view-only shape containing:

- group identity
- prompt and prompt history
- grouped SDS selections
- generation configuration
- any minimal UI restore data needed for the page

Grouped result state should come from `itemizedBatchDetail` projection or, before hydration completes, from the legacy compatibility snapshot.

## Merge and Hydration Rules

All workbench entry points must use a single merge precedence:

1. `itemizedBatchDetail` projection
2. legacy compatibility snapshot
3. persisted view defaults

This precedence must be centralized in workbench model logic rather than reimplemented in multiple hooks or actions.

### `applyBatch(savedBatch)`

`applyBatch` should:

- apply persisted view state to the page
- keep any legacy compatibility data only as pre-hydration fallback
- trigger hydration for the batch detail

`applyBatch` should not treat `savedBatch` as a complete workbench truth payload.

### `applyHydratedBatch({ savedBatch, detail })`

Hydrated detail is the only source allowed to overwrite result-state fields such as:

- `designs`
- `selectedIds`
- `createdTasks`
- other reviewed or generated result data

After hydration succeeds, the workbench must stop trusting legacy flat fields from the saved batch.

### `load draft`

Draft restore needs two supported modes:

- draft-only restore with no active hydrated batch: allow `persisted view + legacy compatibility snapshot`
- active batch restore that will hydrate detail: render initially from `persisted view + legacy snapshot`, then switch to `detail projection + persisted view` once hydration finishes

This avoids both blank initial renders and stale flat result state surviving after detail hydration.

## `generationJobs` Handling

`generationJobs` should not remain a long-term owner in the new persistence shape.

Recommended rule:

- while no hydrated detail exists, old stored `generationJobs` may be used as compatibility fallback for UI continuity
- new persisted draft data should stop treating `generationJobs` as durable truth
- any ongoing loading, retry, or warning UI that still depends on jobs should either be modeled as transient UI state or deliberately projected from current runtime state

This keeps process residue out of the new persistence model without breaking in-progress user flows.

## Implementation Sequence

Implement in this order:

1. Define new persistence and compatibility types
2. Add storage adapters so `shein-studio-batches.ts` can read both old and new stored shapes
3. Change draft write paths to emit only the new persisted view shape
4. Update hydration and batch apply flows to use centralized merge precedence
5. Remove direct draft ownership of result fields from workbench draft contracts
6. Clean up remaining page-level dependencies on old flat persistence owners

This sequence keeps backward compatibility in place before any write-path cleanup begins.

## Testing Requirements

At minimum, cover:

1. old local draft data loads successfully through the adapter
2. new draft writes do not persist `designs`, `selectedIds`, `createdTasks`, or `generationJobs`
3. `applyBatch` can still render historical results before detail hydration completes
4. hydrated detail overrides legacy fallback result fields
5. recent batch summary and batch list data still render correctly after shape cleanup
6. grouped workspace restore no longer reintroduces group-owned result truth

## Risks and Mitigations

### Hidden group-owned truth

Risk:
`groups` may continue to rehydrate `designs`, `selectedIds`, or `createdTasks` indirectly.

Mitigation:
include grouped workspace in the same type and adapter cleanup, not as follow-up work.

### Merge order regression

Risk:
draft fallback logic may overwrite hydrated detail if merge logic remains distributed across hooks and actions.

Mitigation:
centralize precedence in workbench model helpers and make all entry points call them.

### Recent batch summary regression

Risk:
batch list and recent batch UI may lose display fields if old flat fields are deleted without replacement summary projection.

Mitigation:
explicitly preserve list-facing summary data and test recent batch rendering after migration.

### `generationJobs` UI gaps

Risk:
warning, loading, or retry messaging may disappear if `generationJobs` is removed too aggressively.

Mitigation:
keep legacy job data as compatibility fallback first, then narrow any remaining UI dependencies intentionally.

## Open Source / Reuse Consideration

This cleanup is an internal ownership and persistence-boundary correction inside an existing feature. It should continue to reuse the current itemized batch model and compatibility projection helpers already present in the codebase rather than inventing a parallel state system. No external open source replacement is needed for this specific work because the risk is in local ownership drift, not in missing infrastructure.

## Expected Outcome

After this cleanup:

- new persisted data is view-oriented and structurally smaller
- old local data still restores safely through one adapter
- hydrated batch detail becomes the clear result-state owner
- the workbench no longer treats legacy flat fields as first-class truth sources
