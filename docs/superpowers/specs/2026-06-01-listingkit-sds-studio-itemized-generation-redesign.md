# ListingKit SDS Studio Itemized Generation Redesign

## Goal

Replace the current session-centered Shein Studio generation model with an itemized batch architecture that makes generation, recovery, review, and task creation all operate on the same durable backend truth.

The redesign must eliminate the structural conditions that allow:

- generated images to exist upstream but not be durably attached to the right product group
- succeeded generation jobs to be treated as review-ready results
- one product group's images to be reused or misapplied to a different product group during review or task creation

## Problem

The current model stores Shein Studio state primarily as one session plus two flat collections:

- `session.generation_jobs`
- `session.designs`

This creates four root problems.

### 1. The model loses the business unit of work

The real unit of work is not the session. It is one generation target group:

- one product in `per_product` mode
- one shared size group in `shared_by_size` mode

The current model has no first-class persistent record for that unit. As a result, generation, recovery, review, and task creation all have to reconstruct target ownership indirectly.

### 2. Generation success is conflated with review readiness

An upstream async job can succeed while the corresponding design results are not yet durable. The current flow allows the system to record successful jobs before it has a stable, queryable review object for the same target group.

This creates dangerous intermediate states:

- job exists but design does not
- session is still generating but some results are partially present
- review flow sees incomplete or stale design collections

### 3. Target-group identity is not the durable source of truth

Frontend review and task creation depend on target-group identity, but the current persistence model is centered on flat designs and jobs. That makes refresh, recovery, and batch execution fragile because the system must infer ownership after the fact instead of storing it as a required invariant.

### 4. Frontend and backend share orchestration responsibility

Today the backend can trigger generation, but the frontend still participates in critical result persistence and recovery behavior. This means browser lifetime, local state, and silent request failures can alter backend truth.

That boundary is wrong for a long-running, multi-item batch workflow.

## Decision

Replace the session-centered generation model with a batch-centered item model:

- `StudioBatch`
- `StudioBatchItem`
- `StudioGenerationAttempt`
- `StudioMaterializedDesign`

The backend becomes the only owner of:

- batch expansion into generation items
- upstream job tracking
- result materialization
- recovery of partially materialized results
- aggregation of batch progress
- review readiness

The frontend becomes a projection client that:

- starts batch generation
- polls batch and item status
- renders items and materialized designs
- records review approvals
- triggers task creation from approved designs

## Non-Goals

This design does not include:

- compatibility reads for the old `session.designs` or `session.generation_jobs` model
- automatic repair of already-broken historical batches
- preserving the old Studio API shape as the primary contract
- introducing a workflow engine such as Temporal for this redesign
- homepage bulk task generation changes beyond what is required to consume the new batch model

## Product Model

### StudioBatch

`StudioBatch` remains the top-level object the user recognizes as one saved Shein Studio batch.

It owns:

- shared prompt and image settings
- grouped image mode
- selected store
- selected SDS images
- grouped selection snapshot
- batch-level status and counters

It does not directly own generation jobs or reviewable designs.

### StudioBatchItem

`StudioBatchItem` is the core business unit.

One item represents one generation ownership boundary:

- exactly one SDS product selection in `per_product` mode
- exactly one size-sharing target group in `shared_by_size` mode

Each item stores:

- `batch_id`
- `target_group_key`
- `target_group_label`
- `group_mode`
- the concrete selection membership it covers
- item status
- attempt counters
- design counters
- last error summary

Every reviewable design must belong to exactly one item.

### StudioGenerationAttempt

`StudioGenerationAttempt` records one actual call to the upstream image generation system for one item.

Each attempt stores:

- `item_id`
- `attempt_no`
- `request_payload`
- `upstream_job_id`
- `status`
- `started_at`
- `finished_at`
- `error_message`
- upstream response metadata needed for debugging and recovery

Attempts are execution records, not review records.

### StudioMaterializedDesign

`StudioMaterializedDesign` is the only reviewable design object.

Each design stores:

- `item_id`
- `batch_id`
- `target_group_key`
- `target_group_label`
- `source_attempt_id`
- image url and prompt metadata
- role and role label
- product image urls
- review note
- approval state
- sort order

This record is the durable source of truth for review and task creation.

## Status Model

### Batch Status

`StudioBatch.status` is a pure aggregate over item states.

Values:

- `draft`
- `generating`
- `partially_materialized`
- `review_ready`
- `partially_failed`
- `failed`
- `tasks_created`

Rules:

- if any item is actively generating, batch is `generating`
- if at least one item is `review_ready` and at least one item is still pending or failed, batch is `partially_materialized` or `partially_failed`
- batch can only become `review_ready` when every item is review-ready

### Item Status

`StudioBatchItem.status` is the primary business truth.

Values:

- `pending`
- `generating`
- `awaiting_materialization`
- `review_ready`
- `failed`

Rules:

- `pending`: item exists but no active attempt has started
- `generating`: current attempt is in flight upstream
- `awaiting_materialization`: upstream job succeeded but the backend has not yet durably created reviewable designs
- `review_ready`: the item is stable for review and task creation
- `failed`: the current item has no valid materialized result for the latest attempt

### Attempt Status

`StudioGenerationAttempt.status` only describes one upstream execution.

Values:

- `queued`
- `running`
- `succeeded`
- `failed`

This status does not imply that reviewable designs exist.

## Invariants

The redesign should enforce these invariants in code and tests:

1. A materialized design belongs to exactly one batch item.
2. A batch item can only enter `review_ready` if at least one materialized design exists for that item.
3. A succeeded upstream attempt cannot by itself make an item review-ready.
4. Task creation must resolve target products from the owning item, never by guessing from a flat design list.
5. A design from one item can never satisfy another item's missing result.

These invariants are the direct defense against the reported "少图后串图" failure mode.

## Backend Architecture

Introduce three focused backend collaborators for the new model.

### StudioBatchLifecycleService

Responsibilities:

- create and update batch records
- expand a batch into items when generation starts
- aggregate batch counters and status
- query batch, item, and design projections for the UI

### StudioGenerationCoordinator

Responsibilities:

- start item generation attempts
- own sequential or bounded-concurrency item execution policy
- update item and attempt state transitions
- trigger materialization when upstream work completes

### StudioMaterializationService

Responsibilities:

- translate upstream responses into `StudioMaterializedDesign` records
- enforce item ownership and target-group metadata
- deduplicate repeat materialization for the same attempt
- recover items stuck in `awaiting_materialization`

This split keeps the "call upstream" concern separate from the "create durable review objects" concern.

## Execution Flow

### 1. Batch Creation

When the user saves or updates a batch, the backend persists batch-level configuration only. No generation items are created yet.

### 2. Start Generation

When the user clicks generate:

1. backend loads the saved batch snapshot
2. backend expands grouped selections into `StudioBatchItem` records
3. backend marks all items `pending`
4. backend starts generation attempts per item
5. backend marks the batch `generating`

Expansion is deterministic:

- `per_product`: one selection becomes one item
- `shared_by_size`: all selections sharing the same size-group key become one item

### 3. Run Attempt

For each item:

1. create `StudioGenerationAttempt`
2. send one upstream generation request
3. persist `upstream_job_id`
4. item enters `generating`
5. attempt enters `running`

### 4. Upstream Completion

When the upstream job completes successfully:

1. attempt becomes `succeeded`
2. item becomes `awaiting_materialization`
3. backend calls materialization using the attempt record plus the owning item

When upstream job fails:

1. attempt becomes `failed`
2. item becomes `failed`
3. batch aggregate updates accordingly

### 5. Materialization

Materialization is a separate explicit phase.

Inputs:

- owning batch
- owning item
- owning attempt
- upstream image response

Outputs:

- one or more `StudioMaterializedDesign` records

Required fields on every created design:

- `batch_id`
- `item_id`
- `source_attempt_id`
- `target_group_key`
- `target_group_label`

If materialization succeeds:

- item becomes `review_ready`

If materialization fails:

- item remains recoverable in `awaiting_materialization`
- batch cannot become `review_ready`

### 6. Recovery

Recovery is backend-owned and item-scoped.

The recovery loop searches for items in `awaiting_materialization` and retries materialization from durable attempt data and upstream job results.

It does not:

- ask the frontend to re-append designs
- rebuild ownership from local browser state
- borrow designs from any other item

### 7. Retry

Retries happen per item, not per whole batch.

Retry behavior:

1. user selects failed or incomplete items
2. backend creates a new attempt per selected item
3. old materialized designs for other items remain untouched

This prevents a 150-product batch from rerunning healthy items just because a smaller subset failed.

## Review and Approval Model

Review is item-based.

The UI consumes:

- batch summary
- ordered items
- materialized designs under each item

The user still approves concrete designs, not just items, because one item may have multiple candidate designs.

Approval records should be attached to materialized designs and queryable by item.

The UI should no longer treat one flat `selectedIds` collection as the primary source of truth.

Instead, approvals are read and written through item-owned designs.

## Task Creation Model

Task creation must resolve approved designs through their owning item.

Flow:

1. load approved designs
2. for each design, load owning item
3. derive the target product selection set from the item membership
4. create review tasks for those exact selections

Rules:

- `per_product`: one approved design maps to one selection
- `shared_by_size`: one approved design maps to the item's shared selection set

The task creation path is not allowed to guess ownership from prompt text, ordering, or flat design position.

## API Design

Replace the old session-centered generation endpoints with item-aware batch endpoints.

### Batch Query

`GET /studio/batches/:batch_id`

Returns:

- batch summary
- item summaries
- materialized designs grouped by item
- aggregate counters for UI display

### Start Generation

`POST /studio/batches/:batch_id/generate`

Behavior:

- expand batch into items
- start generation workflow
- return current batch snapshot

### Retry Selected Items

`POST /studio/batches/:batch_id/items/retry`

Request:

- `item_ids: string[]`

Behavior:

- create new attempts only for selected items

### Approve Designs

`POST /studio/batches/:batch_id/design-approvals`

Behavior:

- write approval state directly against materialized designs

### Create Tasks

`POST /studio/batches/:batch_id/tasks`

Behavior:

- create tasks from approved materialized designs through owning items

### Item/Attempt Detail

If needed for observability, add:

- `GET /studio/batches/:batch_id/items`
- `GET /studio/batches/:batch_id/items/:item_id/attempts`

These are operational endpoints, not primary review endpoints.

## Persistence Design

Add new persistent records using the existing repository pattern:

- `StudioBatchRecord`
- `StudioBatchItemRecord`
- `StudioGenerationAttemptRecord`
- `StudioMaterializedDesignRecord`

The old session-level generation and design collections should no longer be the source of truth for new work.

If a temporary transition is required during implementation, it must be treated as an implementation scaffold, not as the steady-state design.

## Frontend Design

The frontend workbench should be simplified around backend truth.

### Workbench Responsibilities

Keep:

- editing batch configuration
- triggering batch generation
- rendering grouped review cards
- collecting approval actions
- starting task creation

Remove:

- responsibility for design append persistence
- responsibility for reconstructing target ownership after refresh
- responsibility for deciding whether a succeeded job is review-ready

### Review UI Shape

The visible UI can remain card-based, but its view model should be:

- `batch`
- `items[]`
- `items[].designs[]`

This keeps the interaction familiar while aligning the data model with durable backend ownership.

## Observability

Add explicit logs and counters at each boundary:

- batch expanded into N items
- attempt started for item X
- upstream job id attached to attempt X
- materialization started for attempt X
- materialization succeeded with Y designs
- materialization failed for item X
- batch aggregate moved to status Z

This is required because the current failure mode is otherwise hard to distinguish from upstream image-generation failures.

## Testing

Add focused tests in five layers.

### 1. Expansion Tests

Verify:

- 150 selected products expand into the correct item count in both grouping modes
- item membership is stable and deterministic

### 2. Execution Tests

Verify:

- a succeeded upstream attempt without successful materialization does not make the item review-ready
- failed items do not block healthy item materialization from being preserved

### 3. Ownership Tests

Verify:

- every materialized design carries the correct `item_id` and `target_group_key`
- one item's design can never be selected as another item's fallback

### 4. Review and Task Tests

Verify:

- approving a design maps to the correct item
- `per_product` task creation targets exactly one product selection
- `shared_by_size` task creation expands to the expected shared selection set

### 5. Recovery Tests

Verify:

- items stuck in `awaiting_materialization` can be recovered without frontend participation
- recovery is idempotent and does not duplicate designs for the same attempt

## Migration and Cutover

This redesign intentionally avoids historical compatibility.

Cutover rule:

- only newly created or newly regenerated batches are supported by the new model
- no automatic repair is promised for already broken historical batches

Implementation may require one internal cutover step where old code paths are deleted or isolated, but the end state should expose one canonical model only.

## Risks

### 1. Scope Size

This is a large refactor because it changes model, API, persistence, and UI assumptions together.

### 2. Incomplete Cutover

The biggest implementation risk is leaving one old path still able to write flat design state. That would recreate split-brain ownership immediately.

### 3. Over-Reliance on Derived Projections

If the UI rebuilds item ownership from a flattened response instead of consuming explicit item structures, the redesign will preserve old fragility in a new codebase shape.

## Recommendation

Implement this as a full itemized generation redesign, not as a patch on top of `session.generation_jobs` plus `session.designs`.

The key architectural move is:

- generation attempts track execution
- materialized designs track reviewable output
- batch items own target-group membership

Once those three truths are separated, the reported "130 selected, fewer images returned, then images were attached to the wrong product" class of bug becomes structurally much harder to express in the system at all.
