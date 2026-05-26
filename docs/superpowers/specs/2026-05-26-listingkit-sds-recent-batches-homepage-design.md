# ListingKit SDS Recent Batches Homepage Design

## Background

The current `/listing-kits/sds` experience has already gained:

- grouped SDS candidate selection
- grouped task creation
- multi-group recovery
- per-group prompt history

However, the user clarified a more important product truth: each “group” should be treated as an independent batch, not as multiple peer groups inside one persistent workbench draft.

That means the current `groups[]`-inside-workbench model is useful as a compatibility bridge, but it is not the long-term primary UX. The user expectation is closer to:

- open `/listing-kits/sds`
- immediately see recent independent batches
- pick one batch to continue editing
- optionally create a new batch
- optionally batch-manage several batches at once

This is a better match for the existing backend model as well, because batch persistence, batch detail pages, and task fan-out are already batch-oriented.

## Goals

- Make `/listing-kits/sds` open to a recent-batches homepage instead of assuming the workbench is the primary landing state
- Treat each saved SDS group as an independent batch in the UI model
- Let users continue editing an existing batch without reassembling its products manually
- Let users add candidate products into a chosen batch directly
- Add batch-level summaries and lightweight batch-level bulk operations
- Preserve current grouped generation, store assignment, baseline readiness, and task creation behavior

## Non-Goals

- No backend rewrite of batch/task semantics
- No change to the rule that one generated ListingKit task still maps to one SDS selection and one SHEIN listing task
- No full Kanban-style operations console in this phase
- No automatic merging of multiple existing batches into one
- No removal of legacy workbench compatibility paths in this phase

## Product Decision

The SDS homepage should become a **recent batch dashboard**, not a multi-group editor.

Recommended landing model:

- `/listing-kits/sds` shows recent batch cards first
- each card represents one independently resumable batch
- clicking a card opens the existing studio workbench in that batch context
- the workbench remains a single-batch editor

This avoids introducing two competing abstractions:

- “group”
- “batch”

Instead, the product will consistently teach users:

- a batch is the unit you create, revisit, edit, and operate on

## Alternatives Considered

### Option 1: Keep expanding `groups[]` as the homepage experience

Pros:

- lower short-term UI cost
- reuses the new group-recovery work

Cons:

- conflicts with the clarified mental model that groups are really independent batches
- makes users learn both “group” and “batch”
- makes later bulk actions awkward because the actual durable unit is still the batch

### Option 2: Homepage shows group summaries, but they are secretly batches

Pros:

- visually similar to current workbench-centered direction

Cons:

- hides the real data model behind different terminology
- creates unnecessary product ambiguity
- makes future routing and operations harder to explain

### Option 3: Recommended, recent batch homepage with batch-first editing

Pros:

- matches backend and persistence model
- matches user mental model
- makes future bulk actions straightforward
- keeps workbench focused on editing one batch at a time

Cons:

- requires a new landing section and navigation split
- needs careful compatibility handling for already-saved local grouped state

## Homepage Experience

When a user enters `/listing-kits/sds`, the page should render a `Recent Batches` dashboard above the product browser and workbench entry area.

Each batch card should show:

- batch name
- primary product name
- total grouped product count
- target store summary
- current or most recent prompt preview
- updated time
- current lifecycle summary:
  - no designs yet
  - designs generated
  - review in progress
  - tasks created

Recommended top-level actions:

- `新建批次`
- `继续最近批次`
- batch card click to open a specific batch

If there are no batches yet:

- show an empty-state message
- guide the user to create a new batch from product selection

## Batch Card Summary Model

Each batch card should derive lightweight metadata from existing saved batch content:

- `primarySelection`
- `groupedSelections.length + 1` as product count
- deduplicated store assignment summary:
  - following current store only
  - one explicit store
  - multiple stores / cross-store
- `prompt` or active prompt preview
- `designs.length`
- `createdTasks.length`
- `updatedAt`

This summary should be computed on the client from current batch payloads first, without requiring a backend summary endpoint in phase 1.

## Continue Editing Flow

Clicking a recent batch should:

- load the existing batch detail/workbench state
- hydrate the editor from that batch
- preserve current prompt, prompt history, grouped selections, design state, and store assignments

The workbench should continue behaving as a single-batch editor:

- one active primary selection context
- one active prompt editor
- one active review/task-creation path

The previously added `groups[]` support remains as a compatibility layer for old drafts, but the visible primary workflow becomes batch-first.

## Candidate Pool to Batch Flow

The current candidate pool flow requires entering the workbench and joining grouped products there. For a batch-first model, this should become more direct.

Recommended behavior:

- from the candidate pool or selection summary, allow:
  - `加入当前批次`
  - `加入其他批次`
  - `新建批次并加入`

For phase 1, “加入其他批次” can be a compact picker listing recent batches by name.

Rules:

- only `baseline ready` products can be added
- batch compatibility checks still apply where required by existing generation behavior
- adding a product to a batch updates that batch’s grouped selections and save timestamp

## Batch Management Actions

The homepage should support lightweight batch-level management actions.

### Single-batch actions

- continue editing
- rename
- duplicate
- delete or archive

### Multi-batch actions

Phase 1 bulk actions should be intentionally narrow:

- bulk change target store
- bulk continue generation later is out of scope for this phase
- bulk create SHEIN tasks is allowed only if all selected batches are already in a review-ready state

Recommended phase-1 bulk action set:

- select multiple batches
- bulk assign or override store
- bulk archive or delete

Bulk generation and bulk publish should be deferred until batch readiness semantics are clearer.

## Store Assignment UX

Store summaries should be visible directly in the homepage card.

Examples:

- `跟随当前店铺`
- `SHEIN US 1`
- `2 个店铺`
- `跨店铺分发`

If a batch contains mixed per-product store overrides, the card should clearly indicate that.

Bulk store updates should support:

- set all selected batches to follow current store
- set all selected batches to one specific explicit store

This action should update only the batch-level and grouped selection store assignments relevant to those batches.

## Prompt Presentation

The homepage should show only a short prompt preview per batch.

Prompt editing and prompt-history restoration remain inside the batch editor.

That keeps the homepage scannable while preserving the batch-level prompt workflow already implemented.

## Routing and Navigation

Recommended routing model:

- `/listing-kits/sds`
  - recent batch dashboard + product browser entry
- `/listing-kits/sds?batch=<id>` or existing batch detail route
  - opens a chosen batch in editor context

If an existing dedicated batch detail route already exists and works well, reuse it instead of inventing a new editor route. The homepage should be an entry layer, not a replacement for existing batch editing routes.

## Compatibility Strategy

This phase should preserve compatibility with:

- legacy single-group drafts
- current `groups[]`-based recovery state
- existing saved session batches

Compatibility rules:

1. If a saved batch already exists, homepage cards come from persisted batches first
2. If only local grouped recovery state exists, it may be surfaced as a recoverable draft card, visually distinguished from persisted batches
3. Existing workbench `groups[]` state remains readable, but new homepage interactions should prefer explicit batch persistence

This lets the system transition cleanly from “draft-oriented grouped recovery” to “batch-first homepage” without dropping user work.

## Opened Batch Editor Behavior

Once a batch is opened:

- current grouped candidate and generation flow should still work
- prompt history remains per batch
- adding/removing grouped products updates only that batch
- save/restore remains unchanged from the editor point of view

In other words, the homepage changes discovery and management, while the editor keeps current behavior.

## Risks

### Risk 1: Two overlapping sources of truth

The current local draft/group recovery and saved batch list may disagree.

Mitigation:

- define explicit precedence
- prefer persisted batch records for homepage cards
- show local-only recovery as a separate recoverable draft state, not as a peer persisted batch

### Risk 2: Batch list becomes visually noisy

Batch cards may show too much metadata and become hard to scan.

Mitigation:

- keep summary fields short
- push detailed prompt history and per-product specifics into the editor
- cap homepage metadata to a few high-signal attributes

### Risk 3: Bulk operations on partially ready batches become confusing

Not every batch is in a state where the same action is valid.

Mitigation:

- phase 1 bulk actions should be limited to safe metadata operations
- readiness-sensitive actions should remain single-batch until the rules are explicit

## Testing Strategy

Add coverage for:

- recent batches homepage rendering
- empty-state rendering when no saved batches exist
- homepage card summary correctness
- opening a saved batch from the homepage
- candidate pool add-to-batch flow
- batch rename / delete / duplicate interactions
- bulk store assignment over selected batches
- compatibility behavior for legacy grouped recovery and persisted batch coexistence

## Rollout Recommendation

Implement in two phases.

### Phase 2A

- recent batches homepage
- batch summary cards
- continue editing from card
- new batch entry
- candidate pool add-to-batch

### Phase 2B

- batch rename / duplicate / delete
- multi-select batch cards
- bulk store assignment
- optional local-only draft recovery card treatment

## Decision

Proceed with a batch-first SDS homepage:

- `/listing-kits/sds` becomes a recent-batches dashboard
- the workbench becomes the single-batch editor
- candidate products can be added directly into a chosen batch
- batch-level summaries and lightweight bulk operations live on the homepage
