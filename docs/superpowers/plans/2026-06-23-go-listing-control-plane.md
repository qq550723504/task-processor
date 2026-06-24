# Go Listing Control Plane Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the Java listing scheduling/dispatch/recovery path into Go so Java is no longer required for SHEIN listing task execution, while keeping store fairness when the deployment runs fewer pods on one server.

**Architecture:** Add a single-owner Go listing control plane that scans Postgres import tasks, computes per-store capacity from actual enabled listing stores and live worker ownership, atomically claims tasks, publishes them to RabbitMQ store queues using the existing Go message adapter, and owns stale queued/processing recovery. Worker pods remain consumers only; they do not scan the database.

**Tech Stack:** Go, GORM/Postgres, Redis, RabbitMQ, existing `internal/app/task.MessageAdapter`, existing `internal/listingadmin` repositories, Kubernetes deployment/env configuration.

---

## Closeout Snapshot 2026-06-24

This section records the implementation state after the first Go control-plane pass. The original checklist below is retained as the build plan; this snapshot is the current source for completion and rollout status.

### Completed in code

| Area | Status | Evidence |
| --- | --- | --- |
| Control-plane command and runtime | code complete | `cmd/listing-control-plane`, `internal/app/runtime/listingcontrol` |
| Config and deployment wiring | code complete | `internal/core/config/type_listing_control_plane.go`, `scripts/build-push-deploy-listing-control-plane.ps1`, `deployments/docker/Dockerfile.listing-control-plane`, `deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/listing-control-plane.yaml`; leader lock env and pod-name owner identity are explicit in the deployment. |
| Dispatch repository and RabbitMQ publisher | code complete | `internal/listingadmin` dispatch operations and `internal/listingcontrol/publisher.go` |
| Store runtime and quota handling | code complete, business capacity incomplete | `internal/listingcontrol/store_runtime.go`, `internal/listingcontrol/quota.go`; structured quota handling exists, but complete daily completed / daily limit capacity is still a P0 gap. |
| Scheduler, recovery, status endpoint | code complete | `internal/listingcontrol/scheduler.go`, `internal/listingcontrol/recovery.go`, `internal/app/runtime/listingcontrol/status.go`; `/status` and `/ready` include `leader` snapshot with owner and lease status. |

### Not yet production-closed

| Question | Current answer |
| --- | --- |
| Is Go the only scheduler owner? | Not yet documented as fully cut over. Java scheduler shutdown still needs rollout evidence. |
| Is multi-instance execution safe? | Code-level leader lease is implemented with Redis and exposed in status. Current deployment should still stay at one replica until a two-instance rollout test confirms only the leader runs recovery/dispatch. |
| Are skip/delay reasons durable business facts? | Not yet. Runtime decisions are observable through status/log summaries, but dispatch skip/delay reason persistence on tasks still needs implementation. |
| Is daily limit fully part of capacity? | Not yet. Store queue capacity and structured quota exist; daily completed / in-flight / store daily limit must be unified before production completion. |
| Is there exactly one recovery owner? | Needs rollout confirmation. The control plane has recovery coordination; old worker watchdogs must remain disabled in the control-plane deployment before claiming single ownership. |
| Were stores `976` and `1030` validated? | No repository evidence has been added yet. Add a validation report before marking rollout complete. |
| Was rollback rehearsed? | Rollback path is documented below, but no rehearsal report is present yet. |

### Design adjustments since the original plan

- The runtime command landed as `cmd/listing-control-plane`, matching the control-plane name rather than the earlier `cmd/listing-scheduler` candidate.
- Deployment files landed under `deployments/...` and `scripts/build-push-deploy-listing-control-plane.ps1`, not the earlier `deploy/...` examples.
- Status/readiness is exposed through `internal/app/runtime/listingcontrol/status.go`; it reports runtime summaries and leader identity/lease state.
- Remaining work should focus on production hardening rather than adding another scheduler shape.

## Context

The current Java scheduler scans `product_import_task`, checks store flags and Redis quota keys, sets tasks to queued, and publishes messages. That caused three production problems:

- More pods mean more duplicate scheduler loops and more database connections.
- Fewer pods reduce store coverage unless scheduling understands actual worker ownership and browser capacity.
- A permanent Redis key such as `listing:remaining:quota:<tenant>:<store> = 0` with no TTL can silently stop a store forever.

The Go worker path already has the right consumer side:

- `cmd/shein-listing/main.go` starts `internal/app/runtime/listing.Run`.
- `internal/app/consumer/task_handler.go` validates store access and claims queued messages to processing.
- `internal/app/task/message_adapter.go` defines the canonical `TaskMessage` payload.
- `internal/infra/rabbitmq/queue_config.go` supports `shein.tasks.store.{storeID}` and shared/bucket queues.
- `internal/listingadmin/import_task_repository.go` already has task listing, status update, stale queued recovery, and processing timeout recovery primitives.

## Design Rules

- Postgres is the source of truth for task lifecycle.
- Redis is runtime coordination only: locks, ownership snapshots, queue mode, and quota overrides with TTL/provenance.
- Only one Go control-plane instance performs DB scanning per platform. Workers consume RabbitMQ and do not scan.
- Dispatch fairness is store-first, not pod-first. Pod assignment is an output of live worker ownership and capacity.
- No `virtualShardCount`; capacity is derived from actual enabled stores, active worker pods, and per-pod browser/concurrency settings.
- Legacy no-TTL quota keys must not silently block a store. The new control plane reads only structured quota overrides with TTL unless a rollout flag explicitly enables legacy compatibility.

## Implementation Tasks

- [ ] 1. Add listing control-plane configuration

  Files:

  - `internal/core/config/type_listing_control_plane.go`
  - `internal/core/config/config.go`
  - `internal/core/config/defaults.go` or the existing defaults section in `loader.go`
  - `internal/core/config/config_test.go`

  Add:

  ```go
  type ListingControlPlaneConfig struct {
      Enabled bool `yaml:"enabled"`
      Platform string `yaml:"platform"`
      LeaderLockKey string `yaml:"leaderLockKey"`
      LeaderLockTTL time.Duration `yaml:"leaderLockTTL"`
      ScanInterval time.Duration `yaml:"scanInterval"`
      BatchSize int `yaml:"batchSize"`
      PerStoreBurst int `yaml:"perStoreBurst"`
      MaxQueuedPerStore int `yaml:"maxQueuedPerStore"`
      DryRun bool `yaml:"dryRun"`
      EnableLegacyQuotaKeys bool `yaml:"enableLegacyQuotaKeys"`
      QuotaKeyTTLGrace time.Duration `yaml:"quotaKeyTTLGrace"`
  }
  ```

  Bind environment variables:

  - `TASK_PROCESSOR_LISTING_CONTROL_PLANE_ENABLED`
  - `TASK_PROCESSOR_LISTING_CONTROL_PLANE_PLATFORM`
  - `TASK_PROCESSOR_LISTING_CONTROL_PLANE_SCAN_INTERVAL`
  - `TASK_PROCESSOR_LISTING_CONTROL_PLANE_BATCH_SIZE`
  - `TASK_PROCESSOR_LISTING_CONTROL_PLANE_DRY_RUN`
  - `TASK_PROCESSOR_LISTING_CONTROL_PLANE_ENABLE_LEGACY_QUOTA_KEYS`

  Defaults:

  - disabled by default
  - platform `shein`
  - scan interval `5s`
  - batch size `500`
  - per-store burst `1`
  - max queued per store derived from browser capacity unless configured
  - leader lock TTL `30s`
  - dry run `false`
  - legacy quota keys disabled

- [ ] 2. Add dispatch repository operations with atomic claims

  Files:

  - `internal/listingadmin/import_task.go`
  - `internal/listingadmin/import_task_repository.go`
  - `internal/listingadmin/import_task_dispatch_test.go`

  Extend the repository interface with:

  ```go
  type DispatchCandidateRequest struct {
      Platform string
      Limit int
      PerStoreLimit int
      ExcludedStoreIDs []int64
  }

  type DispatchClaim struct {
      TaskID int64
      PreviousStatus int16
      ProcessingNode string
      Remark string
  }

  ListDispatchCandidatesFair(ctx context.Context, req DispatchCandidateRequest) ([]ImportTask, error)
  ClaimForDispatch(ctx context.Context, claim DispatchClaim) (bool, error)
  RollbackDispatch(ctx context.Context, taskID int64, previousStatus int16, reason string) error
  CountQueuedByStore(ctx context.Context, platform string, storeIDs []int64) (map[int64]int64, error)
  ```

  SQL intent:

  ```sql
  select *
  from (
      select t.*,
             row_number() over (
                 partition by t.tenant_id, t.store_id
                 order by t.priority desc, t.update_time asc, t.id asc
             ) as rn
      from product_import_task t
      where t.deleted = 0
        and coalesce(t.target_platform, t.platform) = ?
        and t.status in (0, 2, 4)
        and t.store_id is not null
  ) ranked
  where ranked.rn <= ?
  order by ranked.rn asc, ranked.priority desc, ranked.update_time asc, ranked.id asc
  limit ?;
  ```

  Claim must be conditional:

  ```sql
  update product_import_task
  set status = 5,
      processing_node = ?,
      error_message = null,
      reason_code = null,
      update_time = now()
  where id = ?
    and status = ?;
  ```

  Tests:

  - fair query alternates stores before taking a second task from one store
  - claim succeeds only from the expected status
  - rollback restores the previous status when RabbitMQ publish fails
  - queued counts are grouped by store and do not require tenant filtering because this table is not tenant-isolated

- [ ] 3. Add store runtime and quota services

  Files:

  - `internal/listingcontrol/store_runtime.go`
  - `internal/listingcontrol/quota.go`
  - `internal/listingcontrol/quota_test.go`

  Responsibilities:

  - Load enabled auto-listing stores for the platform from the existing store repository/API path.
  - Read queue ownership keys:
    - `listing:queue:mode:<tenantID>:<storeID>`
    - `listing:queue:owner:<tenantID>:<storeID>`
  - Treat a store as dispatchable only when it is enabled, auto-listing enabled, not paused, owned by a live worker, and has capacity.
  - Read only structured quota keys by default:
    - `listing:remaining:quota:v2:<tenantID>:<storeID>`
    - value JSON: `{"remaining":12,"source":"manual","updatedAt":"...","expiresAt":"..."}`
  - Ignore legacy no-TTL quota keys unless `EnableLegacyQuotaKeys=true`.
  - When a quota blocks dispatch, record the skip reason as `quota_exhausted` with key name, ttl, and source in logs/metrics.

  Capacity formula:

  ```text
  store_capacity = min(
    maxQueuedPerStore,
    max(1, floor(owner_browser_pool_size / owned_store_count_for_owner))
  )
  ```

  This means scaling from 32 pods to fewer pods still gives every active store a fair slot, and larger pods can raise browser pool/concurrency to match their assigned store count.

- [ ] 4. Add RabbitMQ dispatch publisher

  Files:

  - `internal/listingcontrol/publisher.go`
  - `internal/listingcontrol/publisher_test.go`

  Use `internal/app/task.MessageAdapter.TaskToMessage` for the payload, then publish through an AMQP channel to the exact store queue returned by `rabbitmq.GetStoreQueueName("shein", storeID)`.

  Message requirements:

  - `ContentType: application/json`
  - `DeliveryMode: 2`
  - `MessageId: strconv.FormatInt(task.ID, 10)`
  - `Type: "task"`
  - `Priority: adapter.CalculatePriority(task.Priority)`

  The body is the JSON `TaskMessage`, not a new schema. The consumer already maps delivery bodies into `rabbitmq.Message.Payload` and then `MessageAdapter.MessageToTask`.

  Tests:

  - payload contains `taskId`, `tenantId`, `storeId`, `sourcePlatform`, `targetPlatform`, `productId`, `status`
  - queue name for store 976 is `shein.tasks.store.976`
  - publish error is surfaced so the scheduler can roll back the DB status

- [ ] 5. Build the scheduler engine

  Files:

  - `internal/listingcontrol/scheduler.go`
  - `internal/listingcontrol/decision.go`
  - `internal/listingcontrol/scheduler_test.go`

  Main loop:

  ```text
  acquire leader lock
  load enabled stores and worker ownership
  load queued counts per store
  list fair candidates
  for each candidate:
    evaluate store readiness and quota
    skip with visible reason when blocked
    claim task to queued
    publish to RabbitMQ
    rollback claim if publish fails
  emit summary
  renew/release leader lock
  sleep scan interval
  ```

  Decision result:

  ```go
  type DispatchDecision struct {
      TaskID int64
      TenantID int64
      StoreID int64
      Action string
      Queue string
      Reason string
      OwnerNode string
      Capacity int
      Queued int64
  }
  ```

  Tests:

  - store 976 with owner and capacity dispatches to store queue
  - disabled store is skipped with `store_disabled`
  - store without owner is skipped with `no_live_owner`
  - quota exhausted is skipped without changing task status
  - publish failure rolls task back to prior status
  - dry run logs decisions but does not claim or publish

- [ ] 6. Merge recovery jobs into the control plane

  Files:

  - `internal/listingcontrol/recovery.go`
  - `internal/listingcontrol/recovery_test.go`
  - existing `internal/app/consumer/processing_timeout_watchdog.go`
  - existing `internal/app/consumer/stale_queued_watchdog.go`

  Use the existing repository methods first:

  - `RecoverTimedOutProcessingTasks`
  - `RecoverStaleQueuedTasks`

  Add a control-plane recovery coordinator that runs before each dispatch scan or on its own interval. Keep old watchdogs available behind config during rollout, but disable them in the new control-plane deployment to avoid double recovery.

  Tests:

  - stale queued tasks are returned to pending retry/pending with a clear error message
  - timed out processing tasks are recovered with retry rules preserved
  - disabled recovery config skips repository calls

- [ ] 7. Add a standalone command for the control plane

  Files:

  - `cmd/listing-control-plane/main.go`
  - `internal/app/runtime/listingcontrol/options.go`
  - `internal/app/runtime/listingcontrol/runtime.go`
  - `internal/app/runtime/listingcontrol/runtime_test.go`

  Runtime requirements:

  - Load the same config path handling as `internal/app/runtime/listing`.
  - Initialize DB, Redis, RabbitMQ, and store/runtime dependencies once.
  - Start health endpoint.
  - Start one scheduler for platform `shein`.
  - Exit if `listingControlPlane.enabled=false` unless `--force` is provided.

  This command is the replacement for Java listing scheduler pods. It should be deployed as `replicas: 1` first; multiple replicas are allowed later because leader lock protects scanning.

- [ ] 8. Add Kubernetes/deployment wiring

  Files:

  - `deploy/build-push-deploy-listing.ps1` or the current Go deployment script
  - `deploy/k8s/listing-control-plane.yaml`
  - `deploy/k8s/shein-listing.yaml` if worker env changes are needed

  Deployment shape:

  - `listing-control-plane`: 1 replica, low browser resources, DB/RabbitMQ/Redis access, scheduler enabled.
  - `shein-listing` workers: no DB scan loop, consume assigned store queues, browser pool/concurrency sized for actual owned stores.

  Environment:

  ```text
  TASK_PROCESSOR_LISTING_CONTROL_PLANE_ENABLED=true
  TASK_PROCESSOR_LISTING_CONTROL_PLANE_PLATFORM=shein
  TASK_PROCESSOR_LISTING_CONTROL_PLANE_DRY_RUN=true
  TASK_PROCESSOR_RABBITMQ_STALE_QUEUED_WATCHDOG_ENABLED=false
  TASK_PROCESSOR_RABBITMQ_PROCESSING_TIMEOUT_WATCHDOG_ENABLED=false
  ```

  Rollout:

  - Start in dry run and compare decisions against pending task counts.
  - Switch dry run off.
  - Disable Java listing scheduler after Go has dispatched successfully.
  - Keep worker pods reduced, but increase `TASK_PROCESSOR_BROWSER_POOL_SIZE` and `TASK_PROCESSOR_WORKER_CONCURRENCY` based on owned store count.

- [ ] 9. Add observability and operations queries

  Files:

  - `internal/listingcontrol/metrics.go`
  - `internal/listingcontrol/status.go`
  - health/status endpoint wiring in `internal/app/runtime/listingcontrol`

  Expose:

  - last scan time
  - scanned candidates
  - dispatched count
  - skipped count by reason
  - recovered stale queued count
  - recovered processing timeout count
  - per-store queued count/capacity
  - Redis quota blocks with TTL/source

  This directly answers production questions such as “976 专用 pod 是否正常”, “还有几个没更新完”, and “最新失败原因是什么”.

- [ ] 10. Verification and rollout commands

  Local verification:

  ```powershell
  go test ./internal/listingadmin ./internal/listingcontrol ./internal/app/runtime/listingcontrol
  go test ./internal/app/task ./internal/infra/rabbitmq
  go build ./cmd/listing-control-plane
  go build ./cmd/shein-listing
  ```

  Dry-run production checks:

  ```powershell
  kubectl -n yudao-cloud logs deploy/listing-control-plane --tail=200
  kubectl -n yudao-cloud exec deploy/redis-master -- redis-cli -a "<password>" -n 9 TTL listing:remaining:quota:246:1030
  kubectl -n yudao-cloud get pods -l app=shein-listing -o wide
  ```

  Database checks:

  ```sql
  select store_id, status, count(*)
  from product_import_task
  where coalesce(target_platform, platform) = 'shein'
    and store_id in (976, 1030)
    and deleted = 0
  group by store_id, status
  order by store_id, status;
  ```

  RabbitMQ checks:

  ```powershell
  kubectl -n yudao-cloud exec deploy/rabbitmq -- rabbitmqctl list_queues name messages_ready messages_unacknowledged consumers | Select-String "shein.tasks.store.976|shein.tasks.store.1030"
  ```

## Rollback

- Set `TASK_PROCESSOR_LISTING_CONTROL_PLANE_ENABLED=false`.
- Scale `listing-control-plane` to 0.
- Re-enable the Java scheduler only if Go has not claimed queued tasks successfully.
- Run stale queued recovery once if RabbitMQ publish failed after DB claims.

## Definition of Done

- Go control plane dispatches SHEIN pending/retry/crawled tasks to store queues.
- Store 976 and 1030 can be explained from Go status output without querying Java logs.
- Legacy permanent quota key cannot silently block a store by default.
- Reduced worker pod count still preserves store-level fairness.
- Java listing scheduler can be turned off without stopping task dispatch/recovery.
