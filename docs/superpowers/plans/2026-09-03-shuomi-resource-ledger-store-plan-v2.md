# 硕米 Resource Ledger / Store Service Implementation Plan V2

**目标：** 把企业资源余额、Reservation/Settlement、Store Activate/Renew/Reactivate、迁移与纠错收敛为一套 PostgreSQL 权威实现；账号/ZITADEL 不进入本计划。

## 权威输入

执行前必须读取：

1. `2026-09-03-shuomi-resource-ledger-store-design.md`
2. `...-review-amendments.md`
3. `...-review-amendments-2.md`
4. `...-review-amendments-3.md`
5. `...-review-amendments-4.md`
6. `...-review-amendments-5.md`
7. `...-review-amendments-6.md`
8. `...-review-amendments-7.md`

本 V2 计划是实施入口；旧 `2026-09-03-shuomi-resource-ledger-store-plan.md` 仅保留历史，不再作为执行指令。

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
Correction / Migration claim
```

统一 timeout/retry：

```text
request context deadline
SET LOCAL lock_timeout
SET LOCAL statement_timeout
bounded transaction timeout
max attempts
total retry budget
```

SQLSTATE：

```text
40001 -> retryable within budget
40P01 -> retryable within budget
55P03 -> retryable within budget
57014 -> if caller ctx active, treat server statement_timeout as bounded retryable
57014 + caller ctx cancelled/deadline -> return caller cancellation, no retry
```

budget exhausted -> stable `RESOURCE_CONCURRENCY_RETRY` / 503。

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

所有业务 quantity：

```text
q > 0
q <= domain max
```

客户端不能提交 signed delta；delta 由 Event type 内部推导。

HTTP/JSON 中 BIGINT quantity/balance/delta 使用十进制 string；前端需要计算时显式 `BigInt()`。

---

## Task 3：Operation Replay / Ambiguous Commit

所有 mutation：认证/授权后先 fingerprint-check durable Operation。

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

不能假设 connection loss 等于 rollback。

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

含 reservation_id 的 Event 必须通过 `(organization_id, reservation_id, resource_type)` 绑定真实同租户同资源 Reservation。

Settlement fingerprint：

```text
organization_id
action = commit | release
reservation_id
resource_type
reservation quantity
owner scope
behavioral preconditions
```

普通 Commit/Release 只处理 `reserved`；`reconciliation_required` 只能由持有 lease/epoch 的 Reconciler 在 terminal owner proof 后 fenced Commit/Release。

独立 scanner 使用 `FOR UPDATE SKIP LOCKED`，worker crash 后 lease expiry 可恢复。

---

## Task 5：Resource Debt

Debt key 至少：

```text
(organization_id, resource_type)
```

所有 `delta_available > 0` 路径统一先偿债：

```text
repay = min(debt, gross_positive_delta)
net_available = gross - repay
```

覆盖 Grant、MigrationCredit、AdjustCredit、Reservation Release、允许的 Compensation 等。

Event 记录 gross / debt_repaid / net。

---

## Task 6：Credit Correction / AdjustDebit

纠正错误 Credit 必须引用：

```text
source_credit_event_id
```

执行前锁 source Event，校验：

```text
same organization
same resource_type
allowed credit event type
source not successfully corrected
requested correction == full original credit quantity   // Phase1
```

成功 correction 插入 source-level successful claim：

```text
UNIQUE (organization_id, source_credit_event_id)
```

failed_terminal 不占 claim。

若 credit 已部分消费：

```text
reclaim min(available, original_credit)
remainder -> resource_debt
```

同 source 换 Operation ID 也只能成功一次。

---

## Task 7：Authorization Boundaries

普通 Store lifecycle：

```text
VerifiedIdentity
Live effective org
PermissionWorkbenchStoreLifecycle
```

Human resource management：

```text
AdjustCredit
AdjustDebit
Compensate
CorrectStoreServiceOperation
```

必须：

```text
VerifiedIdentity
Live effective org == target
PermissionWorkbenchResourceAdmin
mandatory audit
```

Phase1 tenant role map：

```text
listingkit_viewer deny
listingkit_operator deny
listingkit_admin allow
```

Grant 仅 trusted Billing/Provisioning service principal；MigrationCredit 仅 migration runner。

---

## Task 8：ConnectionStatus Provider

继续复用 canonical enum：

```text
disconnected
connected
expired
unavailable
```

映射：

```text
revoked/invalid credential -> expired
timeout/network/stale/provider failure -> unavailable
```

Activate 只允许真实、fresh `connected`。

浏览器不能覆盖 connection status。

---

## Task 9：Store Service State Model

Record：

```text
active | deleting | deleted
```

Service：

```text
pending_activation | active | expired | suspended
```

数据库与 rehydration invariant：

```text
active/expired
=> service_started_at IS NOT NULL
AND service_expires_at IS NOT NULL
AND service_expires_at > service_started_at
```

发现 `active + NULL expiry`、无序 timestamp 等 -> `STORE_SERVICE_STATE_CORRUPT`，所有消费与业务 gate fail closed。

Effective expiry 读时计算；materializer 只做优化并使用 aggregate version + conditional update。

---

## Task 10：Activate / Renew / Reactivate

三条命令共同要求：

```text
record_status = active
PermissionWorkbenchStoreLifecycle
Idempotency-Key
If-Match = expected aggregate version
canonical fingerprint includes org/command/store/quantity/version
```

事务内锁 Store 后再次比较 aggregate version。

### Activate

只允许：

```text
service_status = pending_activation
connection_status = connected
```

消费 1 个 renewal period，启动 30-day service。

### Renew

只允许 effective active；以当前 expiry 延长指定 periods。

### Reactivate

只允许 effective expired；从 now 启动新的 service period。

资源扣减、Store state、Operation snapshot、Audit 同事务。

---

## Task 11：Exact Error Contract + BFF

Backend/BFF/client 使用 amendment-6 固定的 code/status table，不允许候选 status。

BFF proxy 显式 allowlist：

```text
activate
renew
reactivate
```

严格校验 body，只转发允许的：

```text
Idempotency-Key
If-Match
```

更新 strict Zod schema、client methods、UI state 与 contract tests。

---

## Task 12：Store Service Correction（仅安全可逆最新操作）

Lifecycle succeeded Operation 保存 immutable before + after snapshot：

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
current Store == source after snapshot
current version == source after version
full refund == source consumed quantity
no successful correction claim
```

否则 `STORE_CORRECTION_NOT_INVERTIBLE`。

安全执行时：

```text
restore exact source before snapshot
full resource correction
successful source claim
immutable correction result
Audit/Outbox
```

不对有后续 Renew/Reactivate 的历史 source 自动“减 30 天”。

---

## Task 13：Migration Credit

Global source claim：

```text
UNIQUE (source_table, source_primary_key, resource_type)
```

Migration tx 必须锁/freeze source：

```text
same DB -> SELECT ... FOR UPDATE
external/unlockable -> immutable snapshot + version/etag conditional verification
```

同一 transaction：

```text
source lock/snapshot verify
claim create/lock
Operation
Bucket credit
Event
Operation succeeded
claim applied
```

source org 变化 -> explicit reconciliation，不能给新 org 再 Credit。

---

## Task 14：Store Schema Hard-cut Rollout

### Phase A Expand

新增 nullable/transitional `record_status / service_status / started_at / expires_at` 等列和 index，不启用新逻辑。

### Phase B Dual-write

全量部署兼容 writer：

```text
Create
ResumeCreate
Disable
Delete
```

同步写新状态；等待旧 Pod drain / minimum writer version fence。

### Phase C Backfill

优先：

```text
deleted_at != null -> deleted
legacy deleting -> deleting
legacy disabled -> suspended
其余 active record 再推 service state
```

### Phase D Verify

零 NULL、零 invalid enum、零 timestamp invariant violation、legacy/new cross-check。

### Phase E Validate Constraints

staged CHECK/VALIDATE/SET NOT NULL。

### Phase F Enable

最后开启新 read contract + Activate/Renew/Reactivate。

---

## Task 15：Rollback

Hard-cut 后只能 feature rollback，不回 pre-dual-write binary。

停止新 user-facing lifecycle acceptance / nonessential workers，但必须继续：

```text
Reservation Reconciler
ambiguous Operation recovery
mandatory correctness/audit outbox
already-started durable correction/migration recovery
```

直到 durable backlog settled 或 handoff 到兼容 recovery worker。

---

## Task 16：PostgreSQL Acceptance Tests

关键并发/锁/约束测试强制 PostgreSQL testcontainers/harness；SQLite 仅用于无生产锁语义的快速 repository tests。

至少覆盖：

```text
40001 / 40P01 / 55P03 retry
57014 server statement timeout
caller context cancellation
lost COMMIT response
same reservation double settlement
reconciliation_required crash/restart
cross-tenant owner/event/source rejection
same source credit double correction
stale If-Match Renew/Reactivate
Store+Resource atomic rollback
active + NULL expiry fail closed
Correct Activate/Renew/Reactivate exact inverse
later lifecycle makes old correction NOT_INVERTIBLE
operator resource-admin denial
schema expand/backfill/validate on populated DB
rollback while reconciliation backlog exists
```

---

## 完成定义

- Resource ledger 不出现负余额、跨租户 source、重复 source correction；
- Store service mutation 与资源扣减、Operation snapshot、Audit 原子；
- 所有三条 lifecycle 命令都具备 Idempotency + If-Match；
- PostgreSQL timeout/deadlock/serialization 行为有明确 bounded recovery；
- Store correction 只做安全可逆最新操作；
- hard-cut schema 和 rollback 不留下旧 writer / dead reconciler 窗口；
- `listingkit_operator` 不能执行企业资源管理命令。
