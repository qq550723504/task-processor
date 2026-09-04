# 硕米 Resource Ledger / Store Service Implementation Plan V3

**目标：** 把企业资源余额、Reservation/Settlement、Store Activate/Renew/Reactivate、迁移与纠错收敛为一套 PostgreSQL 权威实现；账号、ZITADEL 与 Onboarding 不进入本计划。

## 权威输入

执行前必须读取原始 design 与 review amendments 1-8。若历史文档与本 V3 冲突，以本 V3 和 `2026-09-03-shuomi-resource-ledger-store-review-amendments-8.md` 为准。

旧 `...resource-ledger-store-plan.md` 与 `...plan-v2.md` 只保留历史，不再作为实施指令。

---

## Task 1：PostgreSQL Resource/Store Unit of Work

实现显式 transaction-bound repositories，共享同一 PostgreSQL transaction：

```text
Operation
Bucket
Event
Reservation
Debt
Store
Store Audit / Audit Outbox
Correction / Migration / Source Claims
```

统一 timeout/retry：request context deadline、`SET LOCAL lock_timeout`、`SET LOCAL statement_timeout`、bounded transaction timeout、max attempts、single total retry budget。

SQLSTATE：

```text
40001 -> retryable within budget
40P01 -> retryable within budget
55P03 -> retryable within budget
57014 + caller ctx active -> server statement_timeout, bounded retry
57014 + caller ctx cancelled/deadline -> return caller cancellation, no retry
```

budget exhausted -> `RESOURCE_CONCURRENCY_RETRY` / 503。

---

## Task 2：Resource Schema / Core Invariants

Bucket：

```text
UNIQUE (organization_id, resource_type)
available >= 0
reserved >= 0
consumed >= 0
```

Operation：

```text
PK/UNIQUE authority = (organization_id, operation_id)
request_fingerprint
status
immutable response snapshot
```

所有业务 quantity：`q > 0` 且不超过 domain max；客户端不能提交 signed delta。

HTTP/JSON 中 BIGINT quantity/balance/delta 使用十进制 string；前端计算时显式 `BigInt()`。

---

## Task 3：Operation Replay / Ambiguous Commit

所有 mutation 在认证授权后先 fingerprint-check durable Operation：

```text
succeeded / failed_terminal -> replay immutable result
absent / in-progress -> continue
```

DB COMMIT response loss：

```text
commit_outcome_unknown
-> healthy connection read (org, operation_id)
-> durable terminal -> replay
-> authoritative absent -> retry within budget
```

连接丢失不能直接视为 rollback。

---

## Task 4：Reservation / Settlement / Reconciler

Reservation：

```text
quantity > 0
organization/resource scoped owner
reserve operation composite FK
nullable settlement operation composite FK
status = reserved | reconciliation_required | committed | released
expires_at
reconcile_next_at / attempts
reconcile lease owner/until/epoch
```

含 `reservation_id` 的 Event 必须通过 `(organization_id, reservation_id, resource_type)` 绑定真实同租户同资源 Reservation。

Settlement fingerprint 固定包含 org/action/reservation/resource/quantity/owner scope/behavioral preconditions。

普通 Commit/Release 只处理 reserved；`reconciliation_required` 只能由持有 lease/epoch 的 Reconciler 在 terminal owner proof 后 fenced Commit/Release。

独立 scanner 使用 `FOR UPDATE SKIP LOCKED`；rollback 期间 correctness-critical Reconciler 必须继续运行。

---

## Task 5：Resource Debt

Debt 至少按 `(organization_id, resource_type)` 隔离。

所有 `delta_available > 0` 路径统一先偿债：

```text
repay = min(debt, gross_positive_delta)
net_available = gross - repay
```

覆盖 Grant、可信 AdjustCredit、MigrationCredit、Reservation Release、允许的 Compensation 等。Event 记录 gross/debt_repaid/net。

---

## Task 6：Credit Correction / AdjustDebit

Canonical correction command 只接收：

```text
source_credit_event_id
idempotency/operation identity
reason / audit metadata
```

事务内锁 immutable source Event，校验：

```text
same organization
same resource_type
allowed credit type
source not successfully corrected
```

**Correction quantity 永远从 locked source Event 派生**：

```text
original_credit = source_event.gross_credit_quantity
```

请求不得成为 original quantity 权威。若临时兼容旧 caller 的 quantity 字段，必须 exact equal source Event，否则 conflict。

成功 correction 插入 source-level successful claim：

```text
UNIQUE (organization_id, source_credit_event_id)
```

failed_terminal 不占 claim。同 source 换 Operation ID 也最多成功一次。

若 credit 已部分消费：回收 `min(available, original_credit)`，余量形成同租户 resource_debt。

---

## Task 7：Authorization / Mint Boundaries

普通 Store lifecycle：

```text
VerifiedIdentity
Live effective org
PermissionWorkbenchStoreLifecycle
```

Tenant Human Resource Admin：

```text
VerifiedIdentity
Live effective org == target
PermissionWorkbenchResourceAdmin
mandatory audit
```

Phase1 tenant role map：viewer deny、operator deny、listingkit_admin allow。

但 `PermissionWorkbenchResourceAdmin` **只允许 source-bound correction 类命令**：

```text
AdjustDebit
Compensate
CorrectStoreServiceOperation
```

它**不允许**：

```text
AdjustCredit
Grant
MigrationCredit
```

正向资源 mint 只允许 trusted Billing/Platform-Finance/Provisioning service principal，且必须来自 immutable approved source/reference + stable source-level idempotency claim + mandatory audit。

若 Phase1 没有可信 approved source，**不注册 AdjustCredit API**，统一由 Billing/Provisioning Grant 流程入账。

MigrationCredit 只允许 migration runner + immutable migration source claim。

---

## Task 8：ConnectionStatus Provider

Canonical enum 保持：

```text
disconnected
connected
expired
unavailable
```

revoked/invalid credential -> expired；timeout/network/stale/provider failure -> unavailable。Activate 只允许真实 fresh connected；浏览器不能覆盖。

---

## Task 9：Store Record/Service State + Conditional Nullability

Record：

```text
active | deleting | deleted
```

Active Record 的 Service：

```text
pending_activation | active | expired | suspended
```

条件约束：

```text
record_status = active
=> service_status IS NOT NULL

record_status IN (deleting, deleted)
=> service_status IS NULL

service_status IN (active, expired)
=> service_started_at IS NOT NULL
AND service_expires_at IS NOT NULL
AND service_expires_at > service_started_at
```

删除中/已删除记录不制造伪 service enum。Read DTO 对非 active record 允许 `serviceStatus: null`。

发现 active+NULL service status、active/expired+NULL expiry、无序 timestamp 等 -> `STORE_SERVICE_STATE_CORRUPT` / 409，所有 consume/lifecycle/business gate fail closed。

Effective expiry 读时计算；materializer 只做优化并使用 aggregate version + conditional update。

---

## Task 10：Activate / Renew / Reactivate

三条命令共同要求：

```text
record_status = active
PermissionWorkbenchStoreLifecycle
Idempotency-Key
If-Match = expected aggregate version
fingerprint includes org/command/store/quantity/version
```

事务内锁 Store 后再次比较 aggregate version。

Activate：pending_activation + fresh connected，消费 1 renewal period，启动 30-day service。

Renew：只允许 effective active；从当前 expiry 延长 periods。

Reactivate：只允许 effective expired；从 now 启动新 period。

资源扣减、Store state、Operation snapshot、Audit 同事务。

---

## Task 11：Exact Error Contract + BFF

Backend/BFF/client 使用一套固定 code/status table，不允许候选 status。

必须包含：

```text
STORE_SERVICE_STATE_CORRUPT       -> 409
STORE_CORRECTION_NOT_INVERTIBLE   -> 409
```

并保持已有 `STORE_VERSION_CONFLICT`、`STORE_INVALID_STATE` 等既定映射不漂移。

BFF proxy 显式 allowlist activate/renew/reactivate，严格 body，且只转发允许的 `Idempotency-Key` / `If-Match`。更新 strict Zod schema、client methods、UI state、error contract tests。

---

## Task 12：Store Service Correction——只恢复业务字段，Version 单调

Lifecycle succeeded Operation 保存 immutable before+after snapshot：

```text
source type
store id
before status/start/expiry/version
after status/start/expiry/version
resource type
consumed quantity
```

自动 Correction 只允许：

```text
source 是当前 Store 最近一次成功 lifecycle mutation
current service fields == source after snapshot
current aggregate version == source after version
full refund == source consumed quantity
no successful correction claim
```

否则 `STORE_CORRECTION_NOT_INVERTIBLE` / 409。

安全执行：

```text
restore status/start/expiry from source BEFORE snapshot
DO NOT restore before_aggregate_version
new aggregate_version = locked current version + 1
full resource correction
successful source claim
immutable correction result with NEW version
Audit/Outbox
```

Correction 自身是一个新 mutation，所以版本只能前进，不能倒退。

---

## Task 13：Migration Credit

Global source claim：

```text
UNIQUE (source_table, source_primary_key, resource_type)
```

Migration tx 锁/freeze source；same DB 用 `SELECT ... FOR UPDATE`，不可锁 source 用 immutable snapshot + version/etag conditional verification。

同一 transaction：source verify -> claim -> Operation -> Bucket credit -> Event -> Operation succeeded -> claim applied。

source org 变化 -> explicit reconciliation，不能重复给新 org Credit。

---

## Task 14：Store Schema Hard-cut Rollout

### Phase A Expand

新增 nullable/transitional `record_status / service_status / started_at / expires_at` 等列和 indexes，不启用新逻辑。

### Phase B Dual-write

全量部署 Create/ResumeCreate/Disable/Delete compatibility dual-write；等待旧 Pod drain / minimum writer fence。

### Phase C Backfill

```text
deleted_at != null -> record_status=deleted, service_status=NULL
legacy deleting -> record_status=deleting, service_status=NULL
legacy disabled -> record_status=active, service_status=suspended
other active records -> infer pending_activation/active/expired
```

### Phase D Verify

```text
record_status=active rows: service_status zero NULL
inactive rows: service_status must be NULL
zero invalid enum
zero active/expired timestamp invariant violation
legacy/new cross-check
```

### Phase E Validate Constraints

使用 staged conditional CHECK/VALIDATE；**不对全表 service_status SET NOT NULL**。只通过条件约束保证 active record 必须非空。

### Phase F Enable

最后开启新 read contract + lifecycle routes。

---

## Task 15：Rollback

Hard-cut 后只能 feature rollback，不回 pre-dual-write binary。

停止新 user-facing lifecycle acceptance / nonessential workers，但继续：Reservation Reconciler、ambiguous Operation recovery、mandatory audit/outbox、already-started durable correction/migration recovery，直到 backlog settled 或安全 handoff。

---

## Task 16：PostgreSQL Acceptance Tests

关键并发/锁/约束测试强制 PostgreSQL testcontainers/harness。

至少覆盖：

```text
40001/40P01/55P03/57014 bounded retry
lost COMMIT response
same reservation double settlement
reconciliation_required crash/restart
cross-tenant owner/event/source rejection
source credit quantity derived from locked Event
same source double correction
listingkit_admin cannot AdjustCredit
trusted finance + approved source positive mint exactly once
stale If-Match Renew/Reactivate
Store+Resource atomic rollback
active service status NULL fail closed
inactive record service status NULL allowed
Correction business state inverse + aggregate version monotonic
old If-Match after Correction rejected
new error codes exact 409 across backend/BFF/client
schema expand/backfill/conditional validate on populated DB
rollback while reconciliation backlog exists
```

---

## 完成定义

- Resource ledger 不出现负余额、跨租户 source、重复 source correction；
- 正向资源 mint 只能来自可信批准来源，tenant admin 不能凭新 Operation ID 造余额；
- Store service mutation 与资源、Operation snapshot、Audit 原子；
- 所有 lifecycle mutation 有 Idempotency + If-Match；
- PostgreSQL timeout/deadlock/serialization 有 bounded recovery；
- Store correction 只做安全可逆最新操作，且 aggregate version 单调增加；
- deleting/deleted tombstone 不被强迫成伪 Service 状态；
- hard-cut rollout/rollback 不留下旧 writer 或 dead reconciler 窗口。
