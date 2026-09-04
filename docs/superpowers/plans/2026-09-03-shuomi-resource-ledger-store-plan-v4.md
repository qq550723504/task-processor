# 硕米 Resource Ledger / Store Service Implementation Plan V4

**目标：** 把企业资源余额、Reservation/Settlement、Store Activate/Renew/Reactivate、迁移与纠错收敛为一套 PostgreSQL 权威实现。本 V4 是 PR #284 的唯一执行入口；账号、ZITADEL、Onboarding 不进入本计划。

## 权威输入

执行前读取原始 design 与 review amendments 1–9。若历史内容冲突，以本 V4 与 `2026-09-03-shuomi-resource-ledger-store-review-amendments-9.md` 为准。

旧 `...resource-ledger-store-plan.md`、`...plan-v2.md`、`...plan-v3.md` 仅保留历史，不再作为实施指令。

---

## Task 1：PostgreSQL Resource / Store Unit of Work

实现 transaction-bound repositories，共享一个 PostgreSQL transaction：

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

统一：request deadline、`SET LOCAL lock_timeout`、`SET LOCAL statement_timeout`、transaction timeout、max attempts、single total retry budget。

SQLSTATE：

```text
40001 -> retryable within budget
40P01 -> retryable within budget
55P03 -> retryable within budget
57014 + caller ctx active -> statement_timeout bounded retry
57014 + caller ctx cancelled -> return caller cancellation
```

budget exhausted -> `RESOURCE_CONCURRENCY_RETRY` / 503。

DB COMMIT response loss 必须进入 outcome-unknown read-back，不假设 rollback。

---

## Task 2：Resource Schema / Core Invariants

Bucket：

```text
UNIQUE(organization_id, resource_type)
available >= 0
reserved >= 0
consumed >= 0
```

Operation：

```text
UNIQUE(organization_id, operation_id)
request_fingerprint
status
immutable response snapshot
```

所有业务 quantity：`q > 0` 且不超过 domain max；客户端不提交 signed delta。

HTTP/JSON BIGINT quantity/balance/delta 统一 decimal string；JS 需要计算时显式 `BigInt()`。

Event / Reservation / Operation 之间使用 organization/resource scoped composite references，跨租户/跨资源写入由 DB 或事务硬拒绝。

---

## Task 3：Operation Replay / Ambiguous Commit

所有 mutation 在身份/权限验证后先查 durable Operation：

```text
same key + same fingerprint + succeeded/failed_terminal -> replay immutable result
same key + changed fingerprint -> conflict
absent/in-progress -> continue
```

COMMIT response loss：

```text
healthy connection read (organization_id, operation_id)
-> durable terminal -> replay
-> authoritative absent -> retry within remaining budget
```

Lifecycle / correction result snapshot 不依赖后续可变 Store state。

---

## Task 4：Reservation Owner Contract

Phase1 不允许“任意 owner_type”。只有实现 Durable Reservation Owner Contract 的 owner type 才能 Reserve：

```text
owner durable identity
organization/business scope
state = not_started | processing | outcome_unknown | succeeded_terminal | failed_terminal | cancelled_terminal
lease/epoch or equivalent fence
recovery next_at / deadline
owner-specific durable reconciler / existing workflow
terminal proof contract
```

Owner domain 负责把 crash 后的 `processing/outcome_unknown` 收敛到 terminal proof。可复用 PAY-042 recovery 或已有 Temporal workflow；不新增通用 Saga 框架。

Owner type 若无法证明 crash recovery / bounded finality，则 rollout gate 禁止它创建 Reservation。

---

## Task 5：Reservation / Settlement / Owner-start Fence

Reservation：

```text
quantity > 0
organization/resource scoped owner
reserve Operation composite FK
settlement Operation composite FK nullable
status = reserved | reconciliation_required | committed | released
expires_at
reconcile_next_at / attempts
reconcile lease owner/until/epoch
```

### Owner Start

对同 PostgreSQL owner：

```text
BEGIN
lock Owner
lock Reservation
require owner=not_started
require reservation=reserved
owner -> processing
COMMIT
```

### Recovery of expired not_started

```text
BEGIN
lock Owner
lock Reservation
require owner=not_started
require reservation still releasable
owner -> cancelled_terminal/recovery_fenced
Release Reservation + Bucket/Event/Operation
COMMIT
```

Owner 在 `cancelled_terminal/recovery_fenced` 后永远不能 Start。

无法提供等价 conditional start fence 的 owner 不进入 Phase1 Reservation API。

### processing/outcome_unknown

Reservation Reconciler 不猜 Release：调用/等待 owner-specific recovery。只有 terminal proof 后 Commit/Release。

`reconciliation_required` 由 scanner `FOR UPDATE SKIP LOCKED` + lease/epoch durable 接管；rollback 期间不得停止。

---

## Task 6：Resource Debt / Positive Available Delta

Debt key 至少：

```text
(organization_id, resource_type)
```

任何 `delta_available > 0` 先偿 debt：

```text
repay = min(debt, gross_positive_delta)
net_available = gross - repay
```

覆盖 trusted Grant/Credit、MigrationCredit、Reservation Release、trusted Compensation 等。Event 记录 gross/debt_repaid/net。

---

## Task 7：Credit Correction / AdjustDebit

Canonical AdjustDebit 只接：

```text
source_credit_event_id
operation identity
reason / audit metadata
```

事务锁 immutable source Event，校验 same org/resource、allowed credit type、source 未成功纠正。

Correction quantity 从 source Event 派生：

```text
original_credit = source_event.gross_credit_quantity
```

请求值不能成为数量权威。

成功 source claim：

```text
UNIQUE(organization_id, source_credit_event_id)
```

failed_terminal 不占 claim。已消费部分：回收 `min(available, original_credit)`，余量写同 tenant/resource debt。

---

## Task 8：Authorization / Value-returning Boundaries

Store lifecycle：

```text
VerifiedIdentity
Live effective org
PermissionWorkbenchStoreLifecycle
```

Tenant Human ResourceAdmin：

```text
VerifiedIdentity
Live effective org == target
PermissionWorkbenchResourceAdmin
mandatory audit
```

Phase1 role：viewer deny、operator deny、listingkit_admin allow。

但 Human ResourceAdmin 只允许：

```text
source-bound AdjustDebit
CorrectStoreServiceOperation
```

**不允许 human tenant user：**

```text
AdjustCredit
Grant
MigrationCredit
generic CompensateConsume
```

正向 mint 只允许 trusted Billing/Platform-Finance/Provisioning principal + immutable approved source + source-level claim + audit。

Generic `CompensateConsume` 只允许 trusted Billing/Reconciliation/Service principal，必须同时有：

```text
immutable failure/correction proof or approved correction reference
locked source consume Event
derived quantity from source Event
source-level successful compensation claim
mandatory audit
```

没有可信 proof/approval source时，不注册 generic Compensate HTTP API。

Store-coupled consume 永远不能走 generic compensate，只走 paired Store Service Correction。

MigrationCredit 只允许 trusted migration runner。

---

## Task 9：ConnectionStatus Provider

继续复用：

```text
disconnected
connected
expired
unavailable
```

revoked/invalid -> expired；timeout/network/stale/provider error -> unavailable。Activate 只允许 fresh connected。浏览器不能覆盖 connection status。

---

## Task 10：Store Record / Service State

Record：

```text
active | deleting | deleted
```

Active Record Service：

```text
pending_activation | active | expired | suspended
```

约束：

```text
record_status IS NOT NULL
record_status=active -> service_status IS NOT NULL
record_status IN (deleting,deleted) -> service_status IS NULL
service_status IN (active,expired)
  -> started_at/expires_at non-null AND expires_at > started_at
```

读到 active+NULL service、active/expired timestamp corrupt 等 -> `STORE_SERVICE_STATE_CORRUPT` / 409，所有 lifecycle/business gate fail closed。

Effective expiry read-time 计算；materializer 只优化，必须用 expected aggregate version + conditional update。

---

## Task 11：Activate / Renew / Reactivate

共同要求：

```text
record_status=active
PermissionWorkbenchStoreLifecycle
Idempotency-Key
If-Match expected aggregate version
fingerprint = org + command + store + quantity + expected version + behavior fields
```

事务锁 Store 后再次比较 version。

- Activate：pending_activation + fresh connected；消费 1 renewal period，start 30-day service。
- Renew：effective active；从 current expiry 延长 periods。
- Reactivate：effective expired；从 now 启动新 period。

资源、Store state、Operation snapshot、Audit 同一 PostgreSQL transaction。

---

## Task 12：Exact Error Contract + BFF

Backend/BFF/client 使用一套固定 code/status table，包括：

```text
STORE_SERVICE_STATE_CORRUPT       -> 409
STORE_CORRECTION_NOT_INVERTIBLE   -> 409
STORE_VERSION_CONFLICT            -> existing fixed mapping
STORE_INVALID_STATE               -> existing fixed mapping
```

BFF 显式 allowlist activate/renew/reactivate，strict body，仅转发 `Idempotency-Key` / `If-Match` 等允许 header。更新 strict Zod/client/UI/contract tests。

---

## Task 13：Store Service Correction

Lifecycle succeeded Operation 保存 immutable before+after snapshot：source type、store、before/after service fields/version、resource type、consumed quantity。

只允许纠正当前 Store 最近一次成功 lifecycle mutation：

```text
source is latest lifecycle mutation
current service fields == source AFTER
current aggregate version == source AFTER version
full correction quantity == source consumed quantity
no successful source correction claim
```

否则 `STORE_CORRECTION_NOT_INVERTIBLE` / 409。

安全执行：

```text
restore service business fields from BEFORE
DO NOT restore before aggregate version
new aggregate_version = locked current + 1
full resource correction
successful source claim
immutable result with new version
Audit/Outbox
```

paired Store correction 可由 tenant ResourceAdmin 执行，因为资源返还与 Store benefit rollback 原子绑定；换 fresh Operation ID 仍不能重复纠正同 source。

---

## Task 14：Migration Credit — Phase1 仅本地可锁 Source

Global source claim：

```text
UNIQUE(source_table, source_primary_key, resource_type)
```

Phase1 权威 source 必须在同一 PostgreSQL 可锁事务边界：

```text
SELECT source ... FOR UPDATE
-> validate immutable source identity/org/resource/quantity/version
-> claim create/lock
-> Operation
-> Bucket credit
-> Event
-> Operation succeeded
-> claim applied
-> COMMIT
```

**禁止**直接用 remote API / external DB / ETag-only check 做 MigrationCredit source。

外部历史数据必须先由 trusted migration process materialize 到本地 immutable staging table；Resource Ledger 只把该 staging row 当 source。未来直接跨系统迁移另行设计 source-side durable freeze/claim protocol。

source org 变化 -> explicit reconciliation，不给新 org 自动二次 Credit。

---

## Task 15：Store Schema Hard-cut Rollout

### Phase A — Expand

新增 nullable transitional `record_status/service_status/started_at/expires_at` 等列/index；不启用新 lifecycle reads。

### Phase B — Compatibility Writer Fence

全量部署并验证：

```text
Create
ResumeCreate
Disable
Delete
Enable guard
```

Create/ResumeCreate 初始化新状态；Disable -> suspended；Delete -> deleting/deleted。

Legacy `/enable` 在 transitional row 上：

```text
legacy disabled + service_status NULL -> STORE_SERVICE_RESUME_REQUIRED
legacy disabled + service_status suspended -> STORE_SERVICE_RESUME_REQUIRED
NO legacy active transition
```

只有 future `/service/resume` 才恢复 suspended service。

完成 minimum-writer-version fence并 drain 旧 Pod 后才 backfill。

### Phase C — Backfill

```text
deleted_at != null -> record=deleted, service=NULL
legacy deleting -> record=deleting, service=NULL
legacy disabled -> record=active, service=suspended
other active -> infer pending_activation/active/expired
```

### Phase D — Verify

```text
COUNT(record_status IS NULL) = 0
active rows service_status zero NULL
inactive rows service_status all NULL
zero invalid enum
zero active/expired timestamp corruption
legacy/new cross-check
```

### Phase E — Validate

```text
VALIDATE record_status enum CHECK
SET record_status NOT NULL
VALIDATE conditional service_status/timestamp CHECKs
```

不对全表 `service_status SET NOT NULL`。

### Phase F — Enable

最后开启新 read contract + Activate/Renew/Reactivate。

---

## Task 16：Rollback / PostgreSQL Acceptance

Hard-cut 后只能 feature rollback，不回 pre-dual-write binary。

停止新 user-facing lifecycle acceptance / nonessential workers；继续：Reservation Reconciler、Owner Recovery、ambiguous Operation recovery、mandatory audit/outbox、already-started durable correction/migration recovery，直到 backlog settled/handoff。

关键验收强制 PostgreSQL testcontainers/harness，至少：

```text
40001/40P01/55P03/57014 bounded retry
lost COMMIT response
owner crash processing/outcome_unknown -> owner recovery terminalizes
owner type without durable recovery -> Reserve denied
not_started owner Start vs Recovery Release -> exactly one wins
same Reservation double settlement
cross-tenant owner/event/source rejection
source credit correction quantity derived from Event
same source double correction denied
tenant listingkit_admin generic Compensate denied
trusted compensation without failure proof denied
trusted compensation with immutable proof derives source quantity and succeeds once
external ETag-only MigrationCredit denied
local migration source concurrent update serialized by FOR UPDATE
legacy disabled+NULL state /enable during Phase B -> RESUME_REQUIRED
zero NULL record_status required before enable
record_status NOT NULL after Phase E
inactive service_status NULL allowed
stale If-Match lifecycle denied
Correction aggregate version monotonic
rollback with owner/reconciliation backlog running
```

---

## 完成定义

- Resource ledger 不出现负余额、跨租户 source、重复 correction/compensation；
- Reservation 只能绑定有 durable recovery/finality 的 owner，Start/Release 有共享 fence；
- tenant admin 不能无 trusted proof 把 consumed 资源返还为 available；
- MigrationCredit Phase1 没有跨系统 check-then-credit 窗口；
- Store lifecycle 与资源、Operation snapshot、Audit 原子；
- record_status 在 hard cut 前全表完整且非空，service_status 仅按 record lifecycle 条件存在；
- rollout/rollback 不留下 legacy enable、旧 writer 或 dead reconciler 窗口。