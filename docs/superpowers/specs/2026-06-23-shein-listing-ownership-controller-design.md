# SHEIN Listing Ownership Controller Design

## Context

The SHEIN listing runtime currently mixes store ownership coordination, queue initialization, dynamic store assignment, and task execution inside the same `shein-listing` worker process. This caused store 976 to be consumed by both its dedicated pod and a shared shard after a dedicated pod was deployed.

The immediate production workaround paused the auto-shard Redis lock and manually assigned store 976 to `dedicated-store-976`. That is intentionally temporary. The first architecture phase must make store ownership explicit and enforce a single writer for Redis ownership.

## Goals

- Make Redis store ownership a single-writer responsibility.
- Allow shared shard workers to read ownership only.
- Allow a dedicated store pod to remain statically scoped to `OWNED_STORES=976`.
- Prevent stores marked `dedicatedQueueEnabled=true` from being assigned to shared shard candidates.
- Add deployable Kubernetes separation between coordinator and workers.
- Keep this phase small enough to roll out safely.

## Non-Goals

- Do not replace the shard StatefulSet with a Deployment in this phase.
- Do not introduce PgBouncer in this phase.
- Do not redesign the full scheduler.
- Do not implement browser pool lazy initialization in this phase.
- Do not change task processing semantics.

## Architecture

### Roles

Add an explicit auto-shard role:

- `coordinator`: starts `AutoShardCoordinator` and writes Redis ownership.
- `worker`: does not start `AutoShardCoordinator`; reads ownership from Redis through `RedisStoreAssignmentProvider`.
- `disabled`: neither writes nor reads dynamic ownership. Static `OWNED_STORES` still works for dedicated pods.

The existing `rabbitmq.autoShard.enabled` remains the top-level feature switch. The role controls what the process does when the feature is enabled.

Effective behavior:

| Runtime | `autoShard.enabled` | `autoShard.role` | `ownedStores` | Behavior |
|---|---:|---|---|---|
| Ownership controller | true | coordinator | empty | Writes Redis ownership only |
| Shared shard worker | true | worker | empty | Reads Redis ownership and consumes assigned store queues |
| Dedicated store worker | false | disabled | `976` | Consumes only static store 976 |

### Store Eligibility

`AutoShardCoordinator` must only assign stores that satisfy all conditions:

- platform equals the configured platform, case-insensitive
- store status is enabled
- `enableAutoListing=true`
- `dedicatedQueueEnabled` is not true

This requires the management Store API response used by workers to carry `dedicatedQueueEnabled`.

### Redis Ownership Contract

The coordinator is the only process allowed to write:

- `listing:queue:owner:{tenant_id}:{store_id}`
- `listing:queue:mode:{tenant_id}:{store_id}`
- `listing:queue:owner:node:{node_id}`

Workers only read `listing:queue:owner:node:{node_id}`.

Dedicated pods with static `ownedStores` do not need Redis ownership to consume their configured store queues. If Redis ownership is present for a dedicated store, it must point to the dedicated node. Shared coordinator must not overwrite it.

### Coordinator Deployment

Add a new Kubernetes workload for the production auto-shard coordinator:

- name: `shein-listing-ownership-controller`
- replicas: 1
- command remains `/app/listing-service`
- `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ENABLED=true`
- `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE=coordinator`
- `TASK_PROCESSOR_RABBITMQ_NODE_ROLE=task`
- `TASK_PROCESSOR_RABBITMQ_NODE_USE_STORE_QUEUES=false`

The coordinator process should register enough runtime dependencies to run `AutoShardCoordinator`, but it must not consume task queues.

Shared shard StatefulSet changes:

- keep `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ENABLED=true`
- set `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE=worker`
- keep dynamic Redis store assignment enabled
- do not start `AutoShardCoordinator`

Dedicated store 976 changes:

- keep `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ENABLED=false`
- optionally set `TASK_PROCESSOR_RABBITMQ_AUTO_SHARD_ROLE=disabled`
- keep `TASK_PROCESSOR_RABBITMQ_NODE_OWNED_STORES=976`

## Configuration

Add to `AutoShardConfig`:

```go
Role string `yaml:"role"`
```

Accepted values:

- empty: backward-compatible default
- `coordinator`
- `worker`
- `disabled`

Defaulting:

- If `autoShard.enabled=false`, effective role is `disabled`.
- If `autoShard.enabled=true` and role is empty, effective role is `coordinator` for backward compatibility.

Validation:

- `coordinator` requires candidate nodes.
- `worker` does not require candidate nodes for writing, but can still keep the list in config for observability.
- invalid role fails config validation.

## Code Changes

### Config Layer

Modify:

- `internal/core/config/type_rabbitmq.go`
- `internal/core/config/loader_rabbitmq.go`
- `internal/core/config/config.go`
- `internal/core/config/validator_rabbitmq.go`
- related config tests

Add helpers:

- `AutoShardConfig.EffectiveRole() string`
- `AutoShardConfig.IsCoordinator() bool`
- `AutoShardConfig.IsWorker() bool`

### SHEIN Runtime Wiring

Modify `internal/platforms/shein/module.go`:

- Enable dynamic store assignment for shared workers when `UseStoreQueues=true`, no static `OwnedStores`, and role is not `coordinator`.
- Start `AutoShardCoordinator` only when role is `coordinator`.
- Do not start dynamic store assignment in coordinator-only processes.

### Store API DTO

Ensure `internal/infra/clients/management/api/store.go` includes:

```go
DedicatedQueueEnabled *bool `json:"dedicatedQueueEnabled,omitempty"`
```

Ensure local/no-auth management providers and tests preserve the field so coordinator tests match production behavior.

### AutoShardCoordinator

Keep the dedicated store exclusion in `listEligibleStores`.

Add test coverage proving:

- dedicated stores are skipped
- coordinator role starts the coordinator
- worker role does not start the coordinator
- worker role still enables dynamic store assignment

## Rollout Plan

1. Ship code with role support while leaving existing shard behavior compatible.
2. Deploy `shein-listing-ownership-controller` with role `coordinator`.
3. Roll shared shard StatefulSet with role `worker`.
4. Keep store 976 dedicated pod unchanged except optional role `disabled`.
5. Remove the temporary Redis lock `listing:queue:auto-shard:lock`.
6. Verify store 976 remains assigned only to `dedicated-store-976`.

## Verification

Automated tests:

- `go test ./internal/core/config/...`
- `go test ./internal/platforms/shein/...`
- `go test ./internal/app/consumer/...`

Production checks after deployment:

```powershell
kubectl -n task-processor get deploy shein-listing-ownership-controller
kubectl -n task-processor get sts shein-listing-shard
kubectl -n task-processor get deploy shein-listing-store-976
```

Redis checks:

```text
GET listing:queue:owner:322:976
SMEMBERS listing:queue:owner:node:dedicated-store-976
SISMEMBER listing:queue:owner:node:shein-listing-shard-10 976
```

Expected:

- `listing:queue:owner:322:976 = dedicated-store-976`, or no shared owner is written for 976.
- `listing:queue:owner:node:dedicated-store-976` contains `976`.
- no `shein-listing-shard-*` owner set contains `976`.

RabbitMQ check:

```text
shein.tasks.store.976 consumers = 1
```

## Risks

- If management Store API still omits `dedicatedQueueEnabled`, the coordinator cannot exclude dedicated stores. This must be fixed before removing the temporary Redis lock.
- If coordinator process accidentally consumes queues, it could add another worker-like runtime. Role-based wiring must prevent queue consumption.
- If workers keep auto-shard role empty in production, backward compatibility will make them coordinators. Kubernetes manifests must set `worker` explicitly before removing the temporary lock.

## Open Operational Follow-Up

After this phase is stable, separate performance work should address:

- browser pool lazy initialization
- prebuilt Playwright/fingerprint Chrome image assets
- DB pool limits for worker runtimes
- possible StatefulSet-to-Deployment migration
