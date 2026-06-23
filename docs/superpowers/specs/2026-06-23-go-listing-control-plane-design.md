# Go Listing Control Plane Migration Design

## Goal

Replace the Java `yudao-module-listing` runtime with a Go-first listing control plane. The target state is that Java is no longer required for listing operations. Go owns task scheduling, dispatch, queue routing, store runtime state, quota decisions, recovery, and worker-facing task status updates.

This design deliberately avoids copying the Java service shape. The Java implementation mixed admin APIs, scheduling, RabbitMQ publishing, quota state, store pause state, result handling, and scheduled maintenance in one module. The Go replacement should separate those responsibilities into small services with explicit ownership and observable state transitions.

## Non-Goals For Phase 1

- Rebuilding every admin CRUD endpoint in this phase.
- Changing the existing SHEIN worker execution pipeline.
- Replacing the ListingKit UI.
- Migrating unrelated product enrichment, image generation, SDS, or studio workflows.
- Preserving Java scheduler behavior where it caused hidden state or silent task starvation.

Phase 1 focuses on the task dispatch control plane because that is the part currently creating operational ambiguity between Java scheduling and Go workers.

## Current Problems

The current architecture has split ownership:

- Java scans `listing_product_import_task`, publishes RabbitMQ messages, updates tasks to Queued, and applies quota/pause rules.
- Go consumes messages, performs listing work, handles dead letters, and has started owning store assignment and recovery behavior.
- Redis contains durable business decisions such as `listing:remaining:quota:<tenant>:<store>` without TTL or provenance.
- Store assignment can be active in Go while Java still has separate route decisions.
- Scheduler skip reasons are not persisted on tasks, so operators see long-lived Pending tasks without knowing why.

The recent 1030 incident is an example: `listing:remaining:quota:246:1030 = 0` had no TTL, so Java silently skipped dispatch forever while Go queue ownership and consumers were healthy.

## Target Architecture

### 1. Listing Control Plane

The Go control plane is the only owner of runtime listing dispatch decisions:

- scans eligible tasks
- checks store dispatch readiness
- computes per-store capacity
- decides queue routing
- publishes RabbitMQ messages
- updates task lifecycle state
- recovers stale queued and processing tasks
- records why work is skipped, delayed, failed, or dispatched

It runs as a separate process from listing workers. The first command should be `cmd/listing-scheduler` or an equivalent control-plane command that does not start browser pools.

### 2. Listing Admin/Data Layer

The existing Go `internal/listingadmin` and local management provider become the data access foundation. The scheduler must use repositories and domain services directly, not call Java HTTP APIs to make internal decisions.

Admin-facing APIs can continue to evolve separately. They are not on the critical path for Phase 1 as long as the scheduler can read and write the same Postgres and Redis state.

### 3. Listing Workers

SHEIN workers remain focused on execution:

- consume store or bucket queues
- validate task ownership
- run platform-specific listing pipeline
- update task status and output data
- emit dead-letter and processing signals

Workers must not own global scheduling decisions.

## Phase 1 Scope

Phase 1 migrates the Java scheduling and dispatch runtime to Go:

- Pending and Retry task scanning.
- Fair store-level dispatch.
- Per-store capacity and daily limit enforcement.
- Optional remaining quota enforcement with TTL and provenance.
- RabbitMQ route selection.
- Task status transition from Pending/Retry to Queued.
- Publish failure rollback with explicit reason.
- Stale Queued recovery.
- Processing timeout recovery.
- Priority aging.
- Operator-visible skip reason recording.

After Phase 1 is enabled, the Java scheduled methods must be disabled:

- `TaskScheduler#scanPendingTasks`
- `TaskScheduler#checkTimeoutTasks`
- `TaskScheduler#agingTaskPriorities`
- `TaskScheduler#recoverStaleQueuedTasks`
- `ProductImportTaskTimeoutRecoveryJob`

## Core Components

### Dispatch Repository

Package candidate: `internal/listingdispatch`.

Responsibilities:

- list dispatch candidates from `listing_product_import_task`
- claim candidates safely
- update tasks to Queued
- roll back publish failures
- persist skip/delay reasons
- expose recovery queries for stale Queued and timed-out Processing

The repository should use Postgres as the state authority. A task must be claimed with a conditional update or row-locking strategy so multiple scheduler replicas cannot publish the same task.

Recommended claim pattern:

- select candidates by fair scheduling query
- attempt `UPDATE ... WHERE id = ? AND status = expected_status`
- only the scheduler that updates the row can publish it
- if RabbitMQ publish fails, restore the prior status and write reason metadata

### Dispatch Planner

Responsibilities:

- group candidates by platform and store
- enforce per-store fairness
- compute available slots
- decide whether each task should dispatch, delay, or be skipped

The planner should avoid Java's global "first N rows" bias. It should take at most a bounded number per store per cycle, weighted by configured capacity and backlog.

### Store Runtime Service

Package candidate: `internal/listingruntime` or a subpackage under `internal/listingdispatch`.

Responsibilities:

- determine whether a store is dispatchable
- read store status, auto listing flag, validity, pause state, and daily limit
- read queue mode and queue owner
- expose a structured readiness result

Readiness should return machine-readable reasons such as:

- `store_missing`
- `store_disabled`
- `auto_listing_disabled`
- `store_expired`
- `auth_pause`
- `daily_limit_reached`
- `remaining_quota_exhausted`
- `queue_owner_missing`

### Quota Service

Responsibilities:

- compute daily completed count
- compute in-flight count
- enforce store daily limit
- optionally enforce external remaining quota

External remaining quota must not be a permanent Redis integer. If retained, it should be stored as structured state:

```json
{
  "quota": 300,
  "source": "operator",
  "updated_at": "2026-06-23T14:50:00+08:00",
  "expires_at": "2026-06-24T00:00:00+08:00",
  "reason": "manual daily campaign cap"
}
```

The Go service should ignore expired quota state and record a warning metric. A quota of zero should produce a visible delayed reason, not silent Pending.

### Queue Router

Responsibilities:

- resolve route from task platform, store ID, and queue mode
- ensure store-dedicated queues exist when needed
- support existing bucket and shared queue modes where still required
- publish messages through RabbitMQ

Routing decisions should be made in one Go package, used by both scheduler and operational tooling. Redis queue mode and owner state should be read through typed helpers rather than scattered key formatting.

### Recovery Services

Responsibilities:

- recover Queued tasks older than a configured threshold when RabbitMQ has no corresponding work or the task has not been acknowledged
- recover Processing tasks whose lease expired
- preserve detailed reason fields
- avoid overwriting terminal task states

Existing Go dead-letter, processing timeout, and stale queued watchdog work should be reused and folded into the control plane.

## Data Model And State

Postgres remains the source of truth for task lifecycle state.

The task table should carry enough reason metadata for operators:

- `status`
- `reason_code`
- `stage`
- `error_message`
- `remark`
- `update_time`
- `updater`

Phase 1 can use existing columns. If richer audit history is needed, add a separate append-only task event table in a later phase.

Redis is allowed for:

- distributed locks
- queue ownership
- pause state with TTL
- daily counters with TTL
- short-lived external quota state with TTL

Redis must not hold permanent business state that blocks dispatch without auditability.

## Dispatch Flow

1. Scheduler tick starts under a distributed lock or leader election.
2. Repository loads candidates for platforms enabled in the scheduler.
3. Planner groups candidates by store and computes store readiness.
4. Planner chooses tasks up to per-store available slots.
5. Repository claims each selected task with a conditional status update.
6. Queue router publishes RabbitMQ message.
7. If publish succeeds, task remains Queued with dispatch metadata.
8. If publish fails, repository restores prior status and writes `MQ_PUBLISH_FAILED`.
9. If a task is delayed by quota or pause, repository records a non-terminal delay reason without changing it to failure.

## Fairness Model

The scheduler should dispatch across stores, not across raw rows. A single large backlog store must not starve smaller stores.

Recommended algorithm:

- build active store candidates from Pending and Retry tasks
- compute store capacity from daily limit, in-flight count, remaining quota, and configured max dispatch per cycle
- perform round-robin or weighted round-robin across stores
- enforce a small per-cycle cap per store
- prefer higher task priority inside each store

This complements the Go store ownership controller, which already distributes stores across worker pods.

## Observability

The scheduler should emit:

- tasks scanned
- tasks claimed
- tasks published
- publish failures
- delayed tasks by reason
- stores skipped by reason
- quota exhausted stores
- queue routing mode counts
- recovery counts
- scheduler tick duration

Logs should include `tenant_id`, `store_id`, `task_id`, `reason_code`, and route information. Routine logs should be INFO only for cycle summaries and WARN/ERROR for actionable failures.

## Rollout Plan

1. Build Go scheduler in passive dry-run mode.
   - It computes dispatch decisions but does not claim or publish.
   - Compare decisions against Java scheduler behavior for several cycles.

2. Enable Go scheduler for one store.
   - Use a store allowlist, starting with a non-critical or manually selected store.
   - Disable Java dispatch for that store if possible, or ensure Java is globally paused during the test window.

3. Enable Go scheduler for SHEIN stores.
   - Turn off Java `TaskScheduler` methods.
   - Keep Java admin APIs online during this phase.

4. Migrate remaining Java runtime jobs.
   - archive
   - store validity
   - node health
   - notifications if still needed

5. Remove Java listing service from production.
   - Confirm no remaining callers depend on Java-only RPC endpoints.
   - Remove deployment and build pipeline.

## Rollback Plan

Rollback must be simple:

- stop Go scheduler deployment
- re-enable Java scheduler
- leave workers unchanged
- recover Queued/Processing tasks with existing watchdogs

Because Postgres remains the task state authority and RabbitMQ message schema does not change in Phase 1, rollback does not require data migration.

## Testing Strategy

Unit tests:

- candidate selection and fair scheduling
- claim concurrency
- quota and daily limit decisions
- queue route resolution
- publish rollback
- skip reason persistence

Integration tests:

- Postgres-backed repository lifecycle
- RabbitMQ publish path with queue declaration
- Redis quota state expiration
- stale queued and processing timeout recovery

Production verification:

- run dry-run and compare counts
- enable one store
- verify Pending moves to Queued
- verify workers consume expected queue
- verify success, draft, failure, and dead-letter status transitions
- verify no Java scheduler logs after cutover

## Open Decisions

- Whether to create a new append-only `listing_task_event` table in Phase 1 or defer it.
- Whether external remaining quota should be operator-controlled only or synced from another system.
- Whether the scheduler should live in the existing `shein-listing` image with a different command or in a separate image.

Recommended defaults:

- defer task event table until Phase 2
- treat remaining quota as operator-controlled, TTL-bound state
- use the existing Go image with a separate command to keep build and deployment simple

