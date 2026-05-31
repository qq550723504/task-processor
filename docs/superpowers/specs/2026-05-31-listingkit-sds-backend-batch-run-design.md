# ListingKit SDS Backend Batch Run Design

## Goal

Move `批量继续生成` for Shein Studio from a frontend-driven queue into a backend-owned batch runner so the execution no longer depends on browser lifetime, tab focus, or frontend step-by-step orchestration.

The result should preserve the existing single-batch Studio editing flow while making homepage bulk generation a durable backend process that users can start, leave, and inspect later.

## Problem

The current batch-open queue only helps the user move between batches. The browser still owns orchestration:

- load next batch
- trigger generation
- poll job progress
- decide when to advance

This creates three structural problems:

- page lifecycle and network interruptions can break an in-flight batch run
- observability is fragmented across browser state, proxy requests, and backend single-job logs
- the system has no durable server-side record of one whole bulk run

This is why repeated timeout fixes or proxy tuning do not fully solve the root issue. The orchestration boundary is in the wrong place.

## Decision

Introduce a new backend concept: `studio batch run`.

A batch run represents one whole bulk `继续生成` execution across multiple saved Studio batches. The backend owns:

- run lifecycle
- sequential batch progression
- failure recording
- completion summary
- progress queries
- cancellation of future items

The frontend only:

- submits selected batch ids
- polls run progress
- renders current status and failures

## Non-Goals

This design does not include:

- replacing the existing single-batch Studio generation flow
- direct interruption of an already-running OpenAI generation request
- automatic retry policies for failed batch items
- batch run support for `批量创建任务`
- a new workflow engine such as Temporal

## User Experience

### Homepage Bulk Generate

When the user selects multiple persisted Studio batches and clicks `批量继续生成`:

1. frontend sends one request to create a batch run
2. backend starts the run
3. frontend navigates to a run progress view
4. progress continues even if the page is refreshed or reopened later

### Progress View

The progress view shows:

- overall run status
- total / completed / succeeded / failed batch counts
- current batch name or id
- failed item list with error summary
- `停止后续批次` action

The cancellation behavior is explicit:

- the current batch is allowed to finish
- no later batches are started after cancellation is requested

### Single-Batch Workbench

Normal generation inside one loaded batch continues to use the existing single-batch flow. This keeps interactive editing behavior unchanged and limits the migration surface.

## Core Model

Add two new persistent records using the existing `Record + Repository + Mem/Gorm + AutoMigrate` pattern.

### StudioBatchRunRecord

Table: `listingkit_studio_batch_runs`

Fields:

- `id`
- `tenant_id`
- `user_id`
- `mode`
- `failure_policy`
- `status`
- `current_batch_id`
- `current_index`
- `total_batches`
- `completed_batches`
- `succeeded_batches`
- `failed_batches`
- `last_error`
- `cancel_requested`
- `started_at`
- `finished_at`
- `created_at`
- `updated_at`

### StudioBatchRunItemRecord

Table: `listingkit_studio_batch_run_items`

Fields:

- `id`
- `tenant_id`
- `run_id`
- `batch_id`
- `position`
- `status`
- `session_id`
- `async_job_id`
- `error_message`
- `started_at`
- `finished_at`
- `created_at`
- `updated_at`

### Status Values

Run status:

- `pending`
- `running`
- `succeeded`
- `partially_succeeded`
- `failed`
- `cancelled`

Item status:

- `pending`
- `running`
- `succeeded`
- `failed`
- `skipped`

### Default Execution Policy

Phase 1 fixes the default policy as:

- `mode = generate`
- `failure_policy = continue_on_error`

If one batch fails, the run records the failure and continues with the next batch.

## API

Add a dedicated route family for batch runs.

### `POST /studio/batch-runs`

Creates a new run from selected saved batch ids.

Request:

- `batch_ids: string[]`

Response:

- `run`
- `items`

Server validation:

- only persisted Studio batches are accepted
- duplicate batch ids are rejected or normalized
- an empty batch id list is invalid

### `GET /studio/batch-runs/:run_id`

Returns run summary and counters.

### `GET /studio/batch-runs/:run_id/items`

Returns ordered run items with per-batch execution results.

### `POST /studio/batch-runs/:run_id/cancel`

Marks the run as cancellation requested. The executor stops before starting the next pending item.

### Optional `GET /studio/batch-runs`

Not required in phase 1 unless the UI needs a recent-runs list.

## Backend Execution Design

### Main Split

Introduce three focused collaborators:

- `StudioBatchRunService`
- `StudioBatchRunExecutor`
- `StudioBatchRunCoordinator`

Responsibilities:

- service: create, query, list items, cancel
- executor: run one batch run sequentially
- coordinator: launch and recover unfinished runs

### Important Boundary

The executor must not call the HTTP route `/studio/async-jobs`.

Instead, extract the shared single-batch generation logic behind an internal service-level entry point that both of these callers use:

- HTTP `StartStudioAsyncJob`
- backend `StudioBatchRunExecutor`

This avoids:

- HTTP self-calls
- duplicated serialization logic
- split error handling paths
- a fake backend orchestration layer that still depends on request plumbing

### Execution Flow

For each run item in order:

1. load saved Studio batch
2. mark run and item as `running`
3. resolve the target session or batch context
4. execute single-batch Studio design generation through the shared internal service
5. persist item result:
   - `session_id`
   - `async_job_id` if applicable
   - `status`
   - `error_message`
6. update aggregate run counters
7. move to next item unless cancellation was requested

At the end:

- all succeeded -> `succeeded`
- some succeeded and some failed -> `partially_succeeded`
- none succeeded and at least one failed -> `failed`
- cancellation requested before remaining work starts -> `cancelled`

## Recovery Semantics

The system must support process restart without losing the run.

Phase 1 recovery behavior:

- on startup, coordinator scans `pending` and `running` runs
- any run without an active in-process executor is resumed
- the executor resumes from durable run/item state, not frontend state

This design assumes run/item persistence is the source of truth.

## Logging and Observability

The existing `batchRunId` trace context remains useful and should be aligned with the new run id model.

Logging requirements:

- every run-level log includes `run_id`
- every item-level log includes `run_id`, `batch_id`, `position`
- generation sub-steps continue to include `session_id`, `async_job_id`, and timing
- final run summary logs include success/failure counters

This keeps one consistent log spine across:

- browser progress view
- Next proxy
- Go backend executor

## Frontend Changes

Replace the homepage bulk-generate behavior:

- remove frontend ownership of advancing from batch to batch
- keep selection UI
- replace queue execution with run submission

The frontend will:

- call `POST /studio/batch-runs`
- store or route with `run_id`
- poll `GET /studio/batch-runs/:run_id`
- render failed items from `GET /studio/batch-runs/:run_id/items`

The old recent-batch queue state can remain temporarily for UI selection and navigation, but it is no longer the execution engine for bulk generation.

## Compatibility Strategy

Keep single-batch Studio generation unchanged in phase 1.

This produces a safer migration:

- homepage bulk generation uses backend batch runs
- workbench single-batch generation continues to use current interactive generation

That avoids mixing a large orchestration rewrite with existing editing behavior.

## Storage and Indexing Notes

Recommended indexes:

- run table:
  - `(tenant_id, created_at desc)`
  - `(tenant_id, status)`
- item table:
  - `(run_id, position)` unique
  - `(tenant_id, batch_id)`
  - `(tenant_id, status)`

The item table should remain normalized. Phase 1 should not persist full batch snapshots inside run records.

## Testing

Required backend coverage:

- create run with ordered items
- reject empty or invalid batch lists
- executor processes items sequentially
- one item failure still advances the run
- final status becomes `partially_succeeded` when mixed
- cancellation prevents new items from starting
- restart recovery resumes unfinished runs
- repository scope enforcement matches tenant and owner rules

Required frontend coverage:

- homepage bulk generate calls new batch-run API
- progress view renders run counters and current item
- refresh/reopen can resume progress by run id
- cancellation updates UI state correctly

## Rollout Plan

### Phase 1

- add models, repositories, auto-migrate
- add run APIs
- add executor/coordinator
- wire homepage bulk generate to batch runs

### Phase 2

- add progress UI polish
- add recent-run discoverability if needed
- improve failure summaries

### Phase 3

- remove obsolete frontend batch execution logic once the new path is stable

## Recommendation

Implement backend batch runs inside the existing ListingKit service first, reusing current Studio session, batch, and single-job generation capabilities.

This is the smallest architecture change that actually fixes the root problem:

- the browser stops being the orchestrator
- one bulk run becomes durable server state
- failures become queryable and observable
- future retry and cancellation policies can be layered on top without moving the boundary again
