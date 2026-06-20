# ListingKit SDS Batch Production Closure Requirements

## Status

Draft for implementation.

## Date

2026-06-20

## Related Documents

- `docs/superpowers/specs/2026-05-26-listingkit-sds-baseline-grouped-bulk-create-design.md`
- `docs/superpowers/plans/2026-05-26-listingkit-sds-baseline-grouped-bulk-create-phase1.md`
- `docs/superpowers/plans/2026-06-01-listingkit-sds-studio-itemized-generation-redesign.md`
- `docs/superpowers/specs/2026-06-20-listingkit-sds-batch-task-gating-and-status-design.md`
- `docs/superpowers/plans/2026-06-20-listingkit-sds-batch-task-gating-and-status.md`
- `docs/product/listingkit-next-execution-plan.md`

## Purpose

This requirement closes the remaining production gaps in the SDS-to-SHEIN batch flow after the itemized batch model and the first compatibility-aware task gate were introduced.

The intended business flow remains:

`SDS selection -> batch -> compatibility item -> generation attempt -> materialized design -> ListingKit task -> workbench review -> SHEIN draft/publish`

This document does not replace that flow. It defines the missing rules that must be true before the flow can be treated as production-safe.

## Current Baseline

The repository already has the following foundations:

- durable `batch -> item -> attempt -> materialized design` records
- asynchronous generation and recovery
- partial generation retries
- design approval records
- compatibility fingerprint calculation
- batch-native task request construction
- structured rejected-task result support on the backend
- ListingKit submission readiness and idempotent SHEIN submission

The remaining gaps are concentrated around fan-out correctness, durable ownership, final business gating, projection semantics, and partial-result handling.

## Product Outcome

An operator must be able to select one or more SDS products, generate and approve designs, create the expected number of ListingKit tasks, repair individual failures, and submit eligible tasks to SHEIN without:

- silently losing selected products
- creating duplicate ListingKit tasks
- using an invalid or stale SDS baseline
- sharing a design across incompatible product surfaces
- confusing task creation with SHEIN draft save or publication
- losing task-to-batch relationships after refresh
- stopping all remaining work because one candidate failed

## Business Decisions

The following decisions are fixed for this requirement.

### One ListingKit task represents one sellable product package

Each ListingKit task continues to represent:

- one SDS selection
- one resolved SHEIN store
- one approved design
- one canonical product
- one SHEIN package

Multiple SDS selections are not merged into one ListingKit task or one SHEIN listing.

### Approved design fan-out is explicit

For an item containing multiple compatible SDS selections, every approved design must fan out to every eligible selection.

The expected task count is:

`approved design count x eligible selection count`

Example:

- 3 compatible SDS selections
- 2 approved designs
- expected result: 6 task candidates

The system must never collapse these six candidates into only the first selection.

### Approved-by-default remains unchanged

The existing product decision that newly materialized designs default to `approved` is preserved by this requirement.

The UI must still make the approval state visible and allow the operator to deselect designs before task creation. This document does not change the review-default policy.

### Batch graph is the task-creation source of truth

The main task-creation path must derive its business input from the durable batch graph.

`SheinStudioSession` may remain as a compatibility source for older drafts, but it must not be required to:

- build task candidates
- persist created-task ownership
- detect duplicate task creation
- display created tasks after refresh

### Backend is the final gate

Frontend checks are advisory preflight checks. No frontend route, legacy compatibility route, or retry path may bypass the backend final gate.

### Partial success is normal

A batch operation may produce a mixture of:

- created tasks
- reused existing tasks
- business-rejected candidates
- operationally failed candidates

One rejected or failed candidate must not cancel unrelated candidates.

## Domain Definitions

### SDS selection

A stable snapshot of one selected SDS product context, including:

- parent product
- primary variant
- selected variants
- prototype group
- layer
- printable geometry
- template and mask
- target store
- baseline identity and readiness

### Compatibility fingerprint

A stable hash representing whether two selections may safely share one generated design surface.

The production fingerprint must include at least:

- parent product id
- prototype group id
- layer id
- design type
- printable width
- printable height
- normalized template image URL
- normalized mask image URL

Missing required fingerprint fields make the selection incompatible with shared generation.

### Task candidate

A deterministic tuple:

`tenant + batch + item + design + selection + store`

A candidate is evaluated independently by the final gate.

### Batch task link

A durable record connecting a batch candidate to a ListingKit task. This is the authoritative source for:

- duplicate prevention
- created-task display
- task lifecycle projection
- regeneration protection
- historical audit

## Problem Statements

### P1. Compatible selections are currently collapsed

The first compatibility-aware candidate builder selects one candidate after checking fingerprints. It does not produce one candidate per compatible selection.

This violates the one-task-per-selection business model and under-creates tasks.

### P2. Created tasks are not durable without a session

The batch graph can create a task without `SheinStudioSession`, but the created task relationship is still primarily stored on the session.

After refresh, a batch without a session can lose its created-task projection and allow duplicate task creation.

### P3. The final gate is incomplete

The final task creation path must re-check more than design approval and compatibility. It must also validate baseline readiness, store validity, selection ownership, and design-to-item ownership.

### P4. Baseline cache and business readiness are not strict enough

A cached baseline may be reused even when source validation is not ready. Credential bootstrap failures may also be interpreted as ready.

The production flow must distinguish cached data from business-ready data.

### P5. Shared generation still groups by dimensions

`shared_by_size` still builds generation items from width and height. Compatibility is checked later during task creation.

This wastes generation work and can mix incompatible reference images before the final rejection.

### P6. Task and submission states are only partially projected

Backend fields exist for explicit task state, but creation and frontend parsing do not consistently populate or consume them.

Empty state must not mean `draft_saved`, and task titles must not be used to infer publication.

### P7. Regeneration can invalidate existing ownership

An item with already-created tasks may be regenerated and its materialized designs replaced, leaving old tasks linked to designs no longer visible in the batch.

### P8. Batch submission is fail-fast

The current UI loop stops on the first failed SHEIN submission. Remaining eligible tasks are not attempted.

## Functional Requirements

## FR-1 Candidate Fan-Out

For each requested approved design:

1. Load the owning item.
2. Resolve only the selections listed by `item.selection_ids`.
3. Build one candidate for every resolved selection.
4. Compute the compatibility fingerprint for each candidate.
5. Apply group-mode rules.

### Per-product mode

- each item must resolve to exactly one selection
- one approved design produces one candidate
- zero or multiple selections is a business rejection

### Shared mode

- every selection in the item must have a complete fingerprint
- every selection fingerprint must be identical
- one approved design produces one candidate per selection
- a mismatch rejects the affected design/item set with `compatibility_mismatch`

### Acceptance

- 3 compatible selections and 2 approved designs produce 6 candidates
- 3 selections with one incompatible selection produce no silently collapsed candidate
- each result identifies `design_id`, `item_id`, and `selection_id`

## FR-2 Durable Batch Task Link

Introduce a durable batch-task association model.

Recommended record:

```text
StudioBatchTaskLink
  id
  tenant_id
  user_id
  batch_id
  item_id
  design_id
  selection_id
  compatibility_fingerprint
  shein_store_id
  listingkit_task_id
  candidate_key
  status
  reason_code
  message
  created_at
  updated_at
```

Recommended uniqueness:

```text
unique tenant_id + batch_id + item_id + design_id + selection_id
unique tenant_id + candidate_key
```

The link must be written after task creation and loaded by batch detail independently of `SheinStudioSession`.

### Acceptance

- refresh preserves all created tasks
- session-less batches still display created tasks
- retry returns the existing task instead of creating a duplicate
- legacy session task records can be projected or migrated without breaking older batches

## FR-3 Task Creation Idempotency

Build a deterministic candidate key from:

```text
tenant_id
batch_id
item_id
design_id
selection_id
shein_store_id
```

Before task creation:

1. look up the durable link
2. if a valid non-failed task exists, return it as `reused`
3. if a prior link is in a recoverable creation state, resume or reconcile it
4. only create a new task when no valid link exists

Concurrent requests for the same candidate must produce at most one ListingKit task.

## FR-4 Backend Final Gate

Every candidate must pass the following checks immediately before ListingKit task creation.

### Design gate

- design exists
- design belongs to the requested batch
- design belongs to the requested item
- design is approved
- design image URL is present

Reason codes:

```text
design_not_found
design_target_mismatch
design_not_approved
design_image_missing
```

### Selection gate

- selection exists in the batch snapshot
- selection id is listed on the owning item
- required SDS identity fields are present
- selected variants are internally compatible with the selection surface

Reason codes:

```text
selection_not_in_batch
selection_not_in_item
selection_identity_incomplete
selection_variant_incompatible
```

### Store gate

- a positive store id resolves from selection or batch
- the store belongs to the current tenant
- the store is enabled and usable for SHEIN submission

Reason codes:

```text
store_missing
store_invalid
store_not_available
```

### Baseline gate

- baseline key is derived from the current selection
- baseline cache payload exists and is valid
- validation status is `ready`
- baseline schema version is supported

Reason codes:

```text
baseline_missing
baseline_not_ready
baseline_invalid
baseline_stale
baseline_check_unavailable
```

### Compatibility gate

- fingerprint is complete
- shared-mode item selections have the same fingerprint
- design target key matches its item/group

Reason codes:

```text
compatibility_incomplete
compatibility_mismatch
design_target_mismatch
```

Business rejections must be returned as structured `rejected_tasks`; they are not internal server errors.

## FR-5 Strict Baseline Reuse

The standard ListingKit workflow may reuse an SDS baseline only when:

```text
cache status is usable
canonical payload is valid
validation status is ready
schema version is supported
```

A cached-but-blocked baseline must not be hydrated into the production workflow.

Credential/bootstrap errors must never be converted into `ready`. They must resolve to `unknown`, `blocked`, or `failed` with a stable reason code.

The final task gate must call the same readiness policy used by the workflow. Readiness policy must not be duplicated with different semantics.

## FR-6 Compatibility-Aware Generation Grouping

`shared_by_size` remains the UI label for backward compatibility, but backend item expansion must use compatibility fingerprint as the actual group key.

Recommended group key:

```text
compat:<fingerprint>
```

The display label may continue to show printable dimensions.

Rules:

- complete, equal fingerprints may share one generation item
- different fingerprints must become separate items
- incomplete fingerprints must fall back to per-product items or block generation with a clear reason
- task creation still re-checks compatibility as defense in depth

The frontend grouping helper must use the same normalized compatibility fields or consume a backend-provided fingerprint.

## FR-7 Structured Task Creation Result

The task creation response must expose:

```text
batch
items
created_tasks[]
reused_tasks[]
rejected_tasks[]
failed_tasks[]
status_groups
```

Each created or reused task includes:

```text
id
title
design_id
item_id
selection_id
compatibility_fingerprint
status
submission_state
last_submission_action
reason_code
message
```

Each rejected or failed result includes:

```text
design_id
item_id
selection_id
reason_code
message
```

The frontend must parse and display all categories.

## FR-8 Explicit Task Lifecycle Projection

Batch views must use real task and submission state.

Supported states:

```text
task_created
needs_review
ready_to_submit
draft_saved
published
submit_failed
unknown
```

Rules:

- newly created ListingKit task starts as `task_created`
- `needs_review` and `ready_to_submit` derive from real task result/readiness
- `draft_saved`, `published`, and `submit_failed` derive from real submission state
- missing state is `unknown` or `task_created`, never `draft_saved`
- title text must not determine state

Task creation failure and SHEIN submission failure remain separate concepts.

## FR-9 Regeneration Protection

An item with durable task links must not destructively replace designs without an explicit revision policy.

Production-safe phase-one rule:

- block ordinary retry/regeneration when the item has created task links
- show `tasks_already_created` and direct the operator to the existing tasks
- allow an explicit `create_new_revision` action later, preserving old designs and links

No existing ListingKit task may become orphaned from its historical design record.

## FR-10 Partial Batch Submission

Batch save-draft and publish actions must attempt every eligible task independently.

Return/display:

```text
succeeded
failed
skipped
```

A failure for one task must not stop subsequent tasks.

Only failed tasks should be retried. Successful tasks must not be submitted again.

## FR-11 Operator Preview Before Creation

Before creating tasks, the UI must display:

- approved design count
- eligible selection count
- estimated task candidate count
- blocked candidate count and reasons
- target store distribution

Example:

```text
2 approved designs x 3 eligible products = 6 ListingKit tasks
1 product blocked: baseline not ready
```

The displayed estimate is advisory. The backend result remains authoritative.

## FR-12 Observability

Every task candidate operation must log:

```text
tenant_id
batch_id
item_id
design_id
selection_id
candidate_key
listingkit_task_id
result_type
reason_code
```

Required counters:

```text
studio_batch_task_candidate_total
studio_batch_task_created_total
studio_batch_task_reused_total
studio_batch_task_rejected_total
studio_batch_task_failed_total
studio_batch_task_duplicate_prevented_total
```

## Non-Functional Requirements

### Consistency

- candidate creation and link persistence must be transactionally safe where practical
- no successful task may be omitted from the durable association record
- duplicate prevention must work across concurrent HTTP requests and worker retries

### Tenant isolation

All batch-task links, baseline lookups, store checks, and task lookups must be tenant-scoped.

### Backward compatibility

- older session-backed batches remain readable
- legacy style IDs remain detectable for existing-task reuse
- new task creation uses scoped style IDs and durable links
- API fields are additive during migration

### Performance

- final gate checks should be batched where possible
- baseline readiness should avoid one remote call per candidate when candidates share one baseline identity
- task projection should avoid unbounded N+1 requests

## State Semantics

### Batch status

`tasks_created` means only that at least one ListingKit task has been created and task creation has completed for the current request.

It does not mean:

- SHEIN draft saved
- SHEIN product published
- every candidate succeeded

### Candidate result

```text
created  -> a new ListingKit task was created
reused   -> an existing valid ListingKit task was returned
rejected -> expected business gate prevented creation
failed   -> an unexpected operational error occurred
```

### Submission result

```text
draft_saved   -> remote draft save completed
published     -> remote publish completed
submit_failed -> latest remote submission action failed
```

## Acceptance Scenarios

### Scenario A: Compatible fan-out

Given:

- one shared item
- three compatible selections
- two approved designs

When task creation runs,

Then:

- six candidates are evaluated
- six created or reused outcomes are returned
- six durable links exist
- refreshing the batch shows all six tasks

### Scenario B: Compatibility mismatch

Given:

- two same-size selections
- different template or mask fingerprints

When generation is prepared,

Then:

- they are split into separate compatibility items or blocked before shared generation

When task creation is called against an older mixed item,

Then:

- it returns `compatibility_mismatch`
- no selection is silently substituted by the first selection

### Scenario C: Duplicate request

Given a successful candidate link,

When the same request is submitted twice concurrently,

Then:

- one ListingKit task exists
- one response is `created`
- the other is `reused` or equivalent

### Scenario D: Session-less batch

Given a valid batch graph with no legacy session,

When tasks are created and the page is refreshed,

Then:

- created tasks remain visible
- duplicate detection still works

### Scenario E: Baseline blocked

Given a cached baseline whose validation status is blocked,

When task creation runs,

Then:

- candidate is rejected with `baseline_not_ready`
- no ListingKit task is created
- the standard workflow does not reuse that baseline as production-ready data

### Scenario F: Partial creation

Given four candidates where one baseline is blocked and one task creator call fails,

When creation runs,

Then:

- two eligible candidates are created
- one is rejected
- one is failed
- all four outcomes are returned

### Scenario G: Regeneration after task creation

Given an item with existing task links,

When normal retry is requested,

Then:

- retry is blocked with `tasks_already_created`
- historical designs and links remain intact

### Scenario H: Partial submission

Given five publish-eligible tasks where the second fails,

When batch publish runs,

Then:

- tasks three through five are still attempted
- the UI reports four succeeded and one failed
- retry targets only the failed task

## Migration Strategy

### Phase 1: Add durable links and additive API fields

- create the link table/repository
- write links for new task creation
- read links first and session records as fallback
- expose rejected/reused results and explicit states

### Phase 2: Backfill and reconcile

- backfill links from legacy session `CreatedTasks`
- validate referenced ListingKit tasks still exist
- mark unresolved legacy records explicitly

### Phase 3: Switch projections

- batch detail reads durable links as authoritative
- frontend consumes explicit task lifecycle fields
- remove title-based state inference

### Phase 4: Retire compatibility dependence

- remove mandatory session task ownership
- keep session only for older draft migration paths

## Out of Scope

- merging multiple SDS products into one SHEIN listing
- redesigning the entire ListingKit submission state machine
- changing the approved-by-default product rule
- introducing a new marketplace
- replacing the full Studio UI
- automatic destructive migration of old failed tasks

## Release Gate

This requirement is complete only when all of the following are demonstrated:

```text
compatible candidate fan-out is correct
session-less task ownership survives refresh
concurrent duplicate creation is prevented
backend baseline/store/ownership gates are enforced
shared generation uses compatibility-aware grouping
frontend displays created/reused/rejected/failed outcomes
explicit task and submission states are shown
regeneration cannot orphan existing tasks
batch submission continues after individual failures
one real SDS -> ListingKit -> SHEIN draft flow passes
one controlled failure and recovery flow passes
```
