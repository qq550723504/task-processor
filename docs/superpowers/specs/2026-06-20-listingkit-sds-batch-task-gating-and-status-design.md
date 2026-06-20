# ListingKit SDS Batch Task Gating And Status Design

## Context

The current SDS batch flow is directionally correct:

`SDS selection -> batch -> item -> attempt -> materialized design -> ListingKit task -> workbench review -> SHEIN submission`

The durable batch graph is already the right backbone for async generation, retry, and recovery. The remaining problems are concentrated in two areas:

1. The final business gate between generated designs and task creation is still too loose.
2. Task creation state and SHEIN submission state are still mixed together in the batch UI.

This design intentionally preserves one existing business rule:

- Materialized designs default to `approved` after generation.

That default is a product requirement and is not changed here.

## Goals

- Keep `approved-by-default` for generated designs.
- Make task creation depend on batch graph data only, not `SheinStudioSession`.
- Add a single backend final gate for batch task creation.
- Keep `shared_by_size` as a UI option, but only allow shared generation/task fan-out when all selected variants are compatibility-equivalent.
- Replace title-based submission guesses with real task/submission state in batch views.
- Return per-task outcomes so partial failures do not block unrelated task creation or display.

## Non-Goals

- Rework the generation pipeline, item/attempt execution model, or async runners.
- Remove `SheinStudioSession` from the codebase entirely.
- Change the default design review status from `approved`.
- Redesign SHEIN submission readiness itself beyond exposing its real projected state to batch views.

## Current Problems

### 1. Final task gate is incomplete

The backend currently re-checks that selected design IDs exist in the batch and are `approved`, but the final task creation path does not fully re-validate:

- baseline readiness
- store validity
- selection ownership inside the current batch graph
- compatibility of grouped/shared selections

This leaves the real business gate partly in UI and partly in backend.

### 2. `SheinStudioSession` is still a required task-creation dependency

The batch graph exists, but task creation still loads `SheinStudioSession` as a hard prerequisite. This creates a dual-source-of-truth problem:

- batch page shows graph-derived state
- task creation depends on session-derived state

That can diverge as soon as either side is updated independently.

### 3. `shared_by_size` is weaker than the business requirement

Current grouped behavior still treats width/height as the main shared-design signal. But selections already carry stronger compatibility data:

- `prototype_group_id`
- `layer_id`
- `printable_width`
- `printable_height`
- `design_type`
- `template_image_url`
- `mask_image_url`

Two products sharing the same dimensions does not guarantee they can safely share one design.

### 4. Batch UI mixes task creation with submission completion

Batch status grouping currently derives `draft_saved` vs `published` from task title hints instead of real submission state. That makes `tasks_created` look more complete than it really is and misleads operators.

## Proposed Approach

Use the batch graph as the only source of truth for task creation, add an explicit compatibility fingerprint, and expose real submission-derived status in batch detail responses.

This is a focused middle-path refactor:

- broad enough to remove the risky architectural seams
- narrow enough to stay within one implementation cycle

## Design

### 1. Batch graph becomes the only task-creation source

Task creation will be rebuilt to derive all candidate tasks from:

- `StudioBatchRecord`
- `StudioBatchDetailGraph`
- `StudioBatchItemRecord`
- `StudioMaterializedDesign`
- batch selection snapshots and grouped selections

It will no longer require `loadStudioBatchTaskSession` or any mandatory `SheinStudioSession` read.

`SheinStudioSession` remains only as a legacy compatibility layer for:

- reading older saved drafts
- historical migration fallback where needed outside the main create-task path

But the create-task path itself must succeed with batch graph data alone.

### 2. Add explicit compatibility fingerprint

Introduce a shared compatibility fingerprint builder for SDS batch selections. The fingerprint must be stable and derived from the actual business-shared design surface, not just dimensions.

Fingerprint input:

- `parent_product_id`
- `prototype_group_id`
- `layer_id`
- `printable_width`
- `printable_height`
- `design_type`
- normalized `template_image_url`
- normalized `mask_image_url`

The fingerprint does not include mutable display fields like product title or color label.

This fingerprint will be used in two places:

- backend task creation final gate
- frontend grouped selection eligibility and messaging

### 3. Keep `shared_by_size` as a UI mode, but constrain it by fingerprint

`shared_by_size` remains a selectable mode because the business still wants batch efficiency. But it no longer means "same dimensions are enough."

New rule:

- the UI can still present shared mode
- shared mode is only allowed when all selected grouped candidates resolve to the same compatibility fingerprint

Behavior:

- if all selected members match the same fingerprint, grouped mode stays available
- if any selected member differs, shared mode becomes ineligible for that set
- the UI explains that the selections cannot share one design because their printable/template surface differs

This keeps the operator-facing workflow familiar while making the underlying rule correct.

### 4. Introduce a backend final task eligibility gate

Before any ListingKit task is created, the backend will build task candidates from the batch graph and validate each candidate independently.

Each candidate must pass all of the following:

- design exists
- design belongs to the current batch
- design status is `approved`
- selection exists in the current batch graph
- selection belongs to the target item/group relationship used for this task
- `shein_store_id` is present and valid
- baseline readiness for the selection is still `ready`
- compatibility fingerprint matches the design target group and grouped mode expectation

The backend is the final source of truth. UI checks remain helpful preflight checks, but no UI path can bypass the backend gate.

### 5. Add batch-native task candidate preparation

Introduce a batch-native candidate builder that replaces session-based task request construction.

Responsibilities:

- derive the target selections for each approved design
- resolve design-to-selection matching under `per_product` or constrained shared mode
- compute `style_id`
- build ListingKit task requests
- produce created/rejected/failed per-candidate results

This candidate builder should be isolated and deterministic so it is easy to unit test without async runners.

### 6. Replace fragile `style_id` derivation

Current `style_id` truncates a normalized design ID, which risks collisions.

New rule:

- `style_id = stable_hash(batch_id + item_id + design_id + selection_id)[:10]`

Requirements:

- deterministic
- ASCII alphanumeric output
- stable across retries for the same batch/item/design/selection tuple
- different across different selections even when design IDs share prefixes

### 7. Separate task creation state from submission state

Batch detail responses will expose explicit task/submission projection state instead of relying on title hints.

Minimum task lifecycle states for batch display:

- `task_created`
- `needs_review`
- `ready_to_submit`
- `draft_saved`
- `published`
- `submit_failed`
- `unknown`

Definitions:

- `task_created`: ListingKit task exists, but no meaningful submission projection has been observed yet
- `needs_review`: task exists and still requires workbench fixes/review before submission
- `ready_to_submit`: task is ready for draft save or publish
- `draft_saved`: SHEIN draft save completed
- `published`: SHEIN publish completed
- `submit_failed`: latest submission action failed
- `unknown`: projection could not be resolved yet

`tasks_created` may remain as a batch aggregate status, but it must only mean "ListingKit task creation completed." It must not imply saved draft or published state.

### 8. Batch status groups consume real projection state

Batch status groups will be rebuilt from:

- item generation status
- created task submission projection state
- failed task creation outcomes

Grouping rules:

- generation-stage item groups remain for pending/generating/failed/review-ready
- created tasks group by explicit state, not title text
- failed task creation remains separate from submission failure

That yields operator-visible groups such as:

- `processing`
- `submittable`
- `needs_fix`
- `generation_failed`
- `task_created`
- `needs_review`
- `ready_to_submit`
- `draft_saved`
- `published`
- `submission_failed`

### 9. Partial success becomes the default result shape

Task creation and task display should tolerate mixed results across candidates.

The create-task result should include:

- `created`
- `rejected`
- `failed`

Where:

- `created` means ListingKit task created successfully
- `rejected` means the final gate intentionally blocked creation with a business reason
- `failed` means creation attempted but errored unexpectedly

Each non-created result includes:

- `design_id`
- `selection_id`
- `reason_code`
- `message`

This gives the UI enough structure to explain partial outcomes without stopping unrelated eligible tasks.

## Data Model Changes

### Backend

Add batch task projection fields to `SheinStudioCreatedTask` or a closely associated view model:

- `status`
- `submission_state`
- `last_submission_action`
- `reason_code`
- `message`
- `selection_id`
- `item_id`
- `compatibility_fingerprint`

The exact storage location can be chosen during implementation, but the API contract must expose stable state fields and must not require inferring status from `title`.

### Frontend

Update TypeScript batch/task models to include:

- compatibility/fingerprint-aware grouped eligibility
- real task lifecycle state
- per-task reason fields for blocked/failed outcomes

The frontend should treat missing state as `unknown`, not as `draft_saved`.

## API Changes

### Batch task creation response

Expand the batch task creation response to include structured per-candidate outcomes.

Suggested shape:

```text
CreateStudioBatchTasksResult
  batch
  items
  created_tasks[]
  rejected_tasks[]
  failed_tasks[]
  status_groups
```

### Batch detail response

Expand batch detail responses so the workbench and recent-batch views can display:

- real created-task state
- explicit submission stage
- compatibility-derived grouped readiness

## Migration Strategy

### Phase 1: Additive compatibility

Ship additive fields first:

- compatibility fingerprint
- explicit task/submission state
- richer per-candidate result model

Keep legacy fields present so old callers do not break immediately.

### Phase 2: Switch task creation to batch-only

After the new candidate builder and gate are in place:

- remove mandatory session loading from batch task creation
- build requests entirely from batch graph state

### Phase 3: Flip frontend consumers

Update batch UI components to:

- read explicit task state
- stop interpreting `tasks_created` as "draft saved"
- stop using task title for publish inference
- respect shared-mode ineligibility when fingerprint mismatches

## Error Handling

### Business gate rejection

Return structured `rejected` outcomes for:

- `baseline_not_ready`
- `store_invalid`
- `selection_not_in_batch`
- `design_not_approved`
- `compatibility_mismatch`
- `design_target_mismatch`

These are expected business outcomes, not internal server errors.

### Operational failures

Return structured `failed` outcomes for:

- downstream ListingKit task creation error
- persistence error
- projection resolution error that prevents task creation result finalization

These should be retriable where safe, without discarding unrelated created tasks.

## Testing Strategy

### Backend unit tests

- compatibility fingerprint builder returns identical values for equivalent selections
- differing template/mask/layer/prototype/geometry produces different fingerprints
- batch-only task creation works without `SheinStudioSession`
- final gate rejects invalid baseline/store/design/selection combinations
- `style_id` is deterministic and collision-resistant for nearby IDs
- partial success returns created and rejected/failed outcomes together

### Backend integration tests

- batch detail surfaces explicit created-task states
- status groups derive from real projection state instead of title
- shared mode only fans out when all grouped selections share fingerprint

### Frontend tests

- grouped mode UI disables or warns on fingerprint mismatch
- batch panels render `task_created`, `draft_saved`, `published`, `submit_failed`, and `unknown` distinctly
- no component maps plain `tasks_created` to "草稿已保存"
- blocked task creation reasons are displayed from structured codes/messages

## Risks And Mitigations

### Risk: older batches lack enough data for fingerprint

Mitigation:

- compute from existing selection snapshot fields where available
- fall back to `unknown` compatibility only for legacy reads
- require full fingerprint for new create-task operations

### Risk: submission state projection lags behind task creation

Mitigation:

- explicitly support `task_created` and `unknown`
- do not overstate progress in UI while projection is pending

### Risk: removing session dependency reveals hidden coupling

Mitigation:

- isolate candidate builder behind focused tests
- migrate request construction one input field at a time from session-derived to batch-derived values
- verify all task request fields have batch-native sources before cutting over

## Open Decisions Resolved In This Spec

- Generated designs remain `approved` by default.
- Scope includes both backend gate fixes and frontend status semantics.
- `shared_by_size` remains available, but only when compatibility fingerprint matches completely.
- Batch task creation fully detaches from `SheinStudioSession` and relies on batch graph only.

## Recommendation

Implement this as one cohesive change set centered on batch-native task preparation and explicit task/submission state. The main architectural win is not any single validation rule; it is eliminating the split between:

- batch graph vs session truth
- grouped size sharing vs actual compatibility
- task creation vs submission completion semantics

Once those seams are closed, the existing SDS batch pipeline can scale with much lower operational ambiguity.
