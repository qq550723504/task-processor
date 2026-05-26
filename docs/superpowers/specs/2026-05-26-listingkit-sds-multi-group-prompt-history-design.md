# ListingKit SDS Multi-Group Auto-Load and Prompt History Design

## Background

The current `/listing-kits/sds` grouped SDS workflow persists `groupedSelections`, approved designs, and related studio draft state, but it restores that state only after the user re-enters the same active SDS selection. In practice this means:

- grouped products are not visible immediately when the user opens `/listing-kits/sds`
- the user must first reselect the matching main product to recover the prior grouped state
- there is only one implicit grouped workspace, not multiple independently resumable groups
- prompt iteration is tied to the current draft only, without an explicit per-group prompt history

This is mismatched with the operational workflow. The user expectation is to open `/listing-kits/sds`, immediately see previously assembled groups, choose one, adjust the prompt, and continue generating.

## Goals

- Automatically load previously saved SDS groups when entering `/listing-kits/sds`
- Support multiple independently resumable groups
- Preserve the current prompt for each group
- Preserve prompt history per group so users can revert to previously successful prompts
- Keep existing generation, grouped image mode, store assignment, and task creation behavior compatible

## Non-Goals

- No cross-group shared prompt history in phase 1
- No major backend task model rewrite
- No change to the underlying rule that one generated ListingKit task still maps to one SDS selection and one SHEIN task
- No attempt to merge multiple groups into one backend session automatically in phase 1

## Recommended Approach

Use an explicit `groups[]` model at the SDS workbench layer, and make group selection the top-level entry experience for `/listing-kits/sds`.

Each group becomes a first-class resumable workspace object with:

- stable group id
- human-readable group name
- primary selection
- grouped selections
- store assignment state
- generation settings
- current prompt
- prompt history
- updated timestamp

The current workbench becomes an editor for one selected group rather than the single source of truth for all grouped state.

## Alternatives Considered

### Option 1: Keep single draft and auto-restore the last active group

Pros:

- lowest implementation cost
- minimal UI change

Cons:

- still only supports one real group
- users cannot see or switch between multiple saved groups
- still creates ambiguity about which grouped state is current

### Option 2: Global prompt history shared across all groups

Pros:

- simpler storage model

Cons:

- prompt reuse becomes noisy quickly
- prompts are highly group-specific in this workflow
- users cannot reliably infer which historic prompt belonged to which product combination

### Option 3: Recommended, explicit multi-group model with per-group prompt history

Pros:

- matches the operational mental model
- supports direct resume from `/listing-kits/sds`
- isolates prompt history to the product group where it is meaningful
- gives a clean foundation for future group-level actions

Cons:

- higher UI and state migration cost than the current single-group model

## Data Model

Introduce a new client and persisted shape, conceptually:

```ts
type SDSGroupedWorkspace = {
  id: string;
  name: string;
  primarySelection: SDSProductVariantSelection;
  groupedSelections: GroupedSDSSelectionEligibility[];
  sheinStoreId: string;
  imageStrategy: SheinStudioImageStrategy;
  groupedImageMode: SheinStudioGroupedImageMode;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  renderSizeImagesWithSds: boolean;
  currentPrompt: string;
  promptHistory: SDSGroupedPromptHistoryEntry[];
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  artworkModel: SheinStudioArtworkModel;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
  updatedAt: string;
};

type SDSGroupedPromptHistoryEntry = {
  prompt: string;
  groupedImageMode: SheinStudioGroupedImageMode;
  createdAt: string;
};
```

Key rules:

- `currentPrompt` is the editable prompt for the next generation
- `promptHistory[]` stores prompts actually used for generation
- history is per group
- keep the newest 5 prompt history entries per group
- only append a new history entry when generation is triggered and the prompt differs from the latest stored prompt

## Persistence Model

Phase 1 should persist groups through the same server-backed studio draft and batch paths already used today, rather than introducing an unrelated storage channel.

Required persistence changes:

- extend session draft payloads to include `groups[]`
- extend batch payloads to include `groups[]`
- continue allowing existing single-group fields during migration
- on load, prefer `groups[]`
- if only legacy single-group fields exist, synthesize a single group during normalization

This preserves backward compatibility with already saved drafts and batches.

## Page Behavior

### Entry Experience

When the user opens `/listing-kits/sds`:

- load the saved grouped workspaces list first
- render a `Recent Groups` area even before a current main product is reselected
- if there is a most recently updated group, preselect it
- hydrate the current workbench from that selected group

This changes recovery from:

- “reselect the same product to see the group”

to:

- “open the page and pick the group to continue”

### Group Selection

The page should support:

- choosing an existing group
- creating a new group from the current SDS selection
- renaming a group
- deleting a group

Only one group is edited at a time, but multiple groups are visible and resumable.

## Prompt History UX

Each group should expose:

- current editable prompt field
- recent prompt history list for that group
- action to restore a historic prompt into the current editable field

Recommended UX:

- show a compact “recent prompts” section in the group details panel
- display the latest 5 entries
- each entry shows prompt preview, generation mode, and timestamp
- clicking an entry fills `currentPrompt` but does not generate automatically

This keeps prompt history useful without making generation irreversible or surprising.

## Compatibility with Existing Flow

The following behavior should remain unchanged after a group is selected:

- grouped image mode behavior
- same-size shared generation vs per-product generation
- grouped store assignment
- baseline readiness requirements
- grouped task creation fan-out
- review and task creation flow

Implementation should reuse the existing workbench internals by mapping the active group into the current workbench state shape whenever possible.

## Migration Strategy

Use a compatibility-first migration:

1. Add support for `groups[]` in normalized draft and batch payloads
2. When loading legacy data with `groupedSelections` but no `groups[]`, synthesize one group
3. When saving after the feature is released, write the new `groups[]` structure
4. Continue reading legacy fields for a migration window

This avoids breaking already-saved sessions and batches.

## Risks

### Risk 1: State duplication between active workbench state and selected group state

Mitigation:

- define one mapping boundary clearly
- either the selected group is the source of truth, or the workbench state is projected from it and synced back explicitly
- avoid partially updating both independently

### Risk 2: Prompt history grows noisy

Mitigation:

- cap history at 5 entries
- append only on actual generation
- deduplicate against the most recent entry

### Risk 3: Existing drafts silently fail to appear

Mitigation:

- add a legacy-to-group normalization path
- cover legacy draft restoration with tests

## Testing Strategy

Add or update coverage for:

- restoring multiple groups on `/listing-kits/sds` entry
- selecting a saved group without first reselecting its primary product
- synthesizing a group from legacy single-group drafts
- storing and restoring `currentPrompt`
- appending prompt history on generation
- restoring a historic prompt into the editable field
- preserving grouped selections, store assignments, and grouped image mode per group

## Rollout Recommendation

Ship this in two phases:

### Phase 1

- multiple saved groups
- auto-load recent groups on page entry
- per-group current prompt
- per-group prompt history
- legacy draft migration

### Phase 2

- richer group management
- group duplication
- group search/filter
- group-level batch actions

## Decision

Proceed with:

- automatic multi-group loading on `/listing-kits/sds`
- explicit `groups[]` persistence
- one active editable group at a time
- per-group prompt history, not a global history pool
