# 硕米企业资源账本与店铺服务 Implementation Plan

**目标：** 构建企业资源 Bucket / Operation / Reservation / Event，并让 Store Activate / Renew / Reactivate 与续费期数扣减保持 PostgreSQL 单库事务原子性。

## 权威输入

实现前必须同时读取以下文档；后续 amendment 在冲突处覆盖前序内容：

1. `docs/superpowers/specs/2026-09-03-shuomi-resource-ledger-store-design.md`
2. `docs/superpowers/specs/2026-09-03-shuomi-resource-ledger-store-review-amendments.md`
3. `docs/superpowers/specs/2026-09-03-shuomi-resource-ledger-store-review-amendments-2.md`
4. `docs/superpowers/specs/2026-09-03-shuomi-resource-ledger-store-review-amendments-3.md`
5. `docs/superpowers/specs/2026-09-03-shuomi-resource-ledger-store-review-amendments-4.md`
6. `docs/superpowers/specs/2026-09-03-shuomi-resource-ledger-store-review-amendments-5.md`
7. `docs/superpowers/specs/2026-09-03-shuomi-resource-ledger-store-review-amendments-6.md`

本计划已经按 amendments 1-6 直接收敛。禁止回退到：

```text
result_reference 指向可变 Store 作为幂等 replay 结果
service_version 作为第一阶段 If-Match
只允许 reserved settlement、让 reconciliation_required 成为死状态
SQLite-only transaction retry predicate
无 lock_timeout / transaction budget 的无限等待
manual compensation 不校验 source org/resource/store
partial Store Service correction
pre-dual-write binary 的 hard-cut rollback
```

---

## Task 1：建立独立 Resource Domain + PostgreSQL transaction foundation

新增独立资源域 package，例如：

```text
internal/orgresource
```

资源域只处理：

```text
store_renewal_period
ai_point
data_row
```

不引入账号注册、ZITADEL、Onboarding 或人民币支付逻辑。

实现 PostgreSQL transaction runner，明确：

```text
request context deadline
SET LOCAL lock_timeout
SET LOCAL statement_timeout
bounded transaction timeout
max attempts
single total retry budget + jitter
```

SQLSTATE classifier：

```text
40001 serialization_failure -> retryable
40P01 deadlock_detected      -> retryable
55P03 lock_not_available     -> retryable while budget remains
```

caller context 已取消/超时不得重试。总预算耗尽返回：

```text
RESOURCE_CONCURRENCY_RETRY -> HTTP 503
```

禁止直接复用 `moderncsqlite` code 5/6 的 retry predicate。

---

## Task 2：Resource Bucket + Operation + immutable result snapshot

Schema：

```text
saas_organization_resource_buckets
PRIMARY KEY (organization_id, resource_type)

saas_organization_resource_operations
PRIMARY KEY (organization_id, operation_id)
```

Operation 至少保存：

```text
operation_type
request_fingerprint
state
failure_code
failure_http_status
immutable_result_snapshot
created_at / completed_at
```

Store lifecycle succeeded Operation 的 immutable snapshot 至少包含：

```text
organization_id
store_id
record_status
service_status
service_started_at
service_expires_at
aggregate_version
resource_type
resource_quantity_delta
resource_balance_after
```

所有 BIGINT resource quantity / delta / balance 在 HTTP/JSON snapshot 中使用十进制字符串，前端不得用 JS Number 承载。

same key + same fingerprint replay 原响应；same key + changed command/Store/quantity/version/behavior field 返回 `IDEMPOTENCY_KEY_CONFLICT` / 409。

---

## Task 3：Event / Reservation tenant-scoped schema invariants

新增：

```text
saas_organization_resource_events
saas_organization_resource_reservations
```

Event 对 Operation 使用：

```text
FK (organization_id, operation_id)
-> operations(organization_id, operation_id)
```

Event 含 `reservation_id` 时还必须：

```text
FK (organization_id, reservation_id, resource_type)
-> reservations(organization_id, reservation_id, resource_type)
```

Reservation：

```text
reservation_id
organization_id
resource_type
quantity CHECK quantity > 0
state = reserved | reconciliation_required | committed | released
reserve_operation_id
settlement_operation_id nullable
owner_type
owner_id
owner_organization_id
owner_business_scope
expires_at
reconcile_next_at
reconcile_attempts
reconcile_lease_owner
reconcile_lease_until
reconcile_epoch
```

reserve / settlement operation references 都使用 organization-scoped composite FK。

---

## Task 4：Reserve / Commit / Release + durable reconciliation

`Reserve(q)` 在创建 Operation 前验证：

```text
q > 0
q <= domain max
```

Settlement canonical fingerprint 必须绑定：

```text
organization_id
action = commit|release
reservation_id
resource_type
reservation_quantity
owner scope
all behavioral preconditions
```

普通 Commit/Release：

1. 认证/tenant scope；
2. 先 fingerprint-check durable settlement Operation replay；
3. 首次执行时 `SELECT ... FOR UPDATE` Reservation；
4. 只允许 `reserved` 进入普通 settlement；
5. 不同 Operation 对已 settled reservation 返回 conflict。

过期 Reservation 只触发恢复检查：

```text
owner succeeded_terminal -> Commit
owner failed/cancelled/not_started -> Release
owner processing/outcome_unknown -> reconciliation_required
```

`reconciliation_required` 的同一事务必须设置 `reconcile_next_at` 等 durable handoff 字段。

Reservation Reconciler 周期扫描：

```text
state = reconciliation_required
AND reconcile_next_at <= now
AND lease absent/expired
```

用 `FOR UPDATE SKIP LOCKED` claim lease/epoch。owner terminal 后允许 fenced `reconciliation_required -> committed|released`；仍 unknown 时更新 bounded backoff。worker crash 后 lease expiry 可接管，不依赖内存 handoff。

Owner resolver 必须硬校验：

```text
owner.organization_id == reservation.organization_id
owner business/resource scope matches reservation
```

---

## Task 5：所有资源数量输入统一正数语义

第一阶段业务 Command：

```text
Grant
MigrationCredit
AdjustCredit
AdjustDebit
Consume
Reserve
CompensateConsume
Store lifecycle consume
Store service correction refund
```

所有命令在创建 Operation 前要求：

```text
q > 0
q <= resource-specific maximum
```

客户端不能提交 signed delta；Event delta 由 command type 在服务端推导。

持久化 quantity 字段增加对应正数 CHECK。

---

## Task 6：Grant / Adjustment / Consume / Compensation + Resource Debt

禁止 generic `ReverseGrant`。

`CompensateConsume` 必须：

```text
lock source consume Event
source eligible
same organization_id
same resource_type
source not coupled_store_service
single successful compensation
```

Store lifecycle consume 标记 `coupled_store_service`，第一阶段禁止通用单边 compensation；只能走 Store + Resource 同事务 correction。

错误 Credit 已部分消费时：

```text
reclaim = min(available, correction_amount)
remaining -> resource_debt
```

Resource Debt key 至少：

```text
organization_id
resource_type
```

所有 `delta_available > 0` 路径统一调用 `ApplyPositiveAvailableDelta`：先偿还同 org/resource debt，再增加 spendable available。覆盖 Grant、AdjustCredit、Reservation Release、允许的 Compensation 等。

---

## Task 7：余额变更管理命令授权边界

普通 Store lifecycle：

```text
AuthPolicyVerifiedIdentity
OrganizationAccessPolicyLiveWrite
PermissionWorkbenchStoreLifecycle
```

管理资源命令：

```text
AdjustCredit
AdjustDebit
CompensateConsume
CorrectStoreServiceOperation
```

必须：

```text
verified identity
live effective organization == target organization
PermissionListingKitAdminWrite
mandatory audit
```

`Grant` 只允许受信任内部 Billing/Provisioning service principal，不注册普通 Workbench 浏览器 route。

`MigrationCredit` 只允许 migration runner service principal + verified source claim/snapshot。

补 viewer/operator/admin/internal-principal 正反授权测试。

---

## Task 8：Store 状态与 canonical ConnectionStatus

Store 新合同：

```text
record_status = active | deleting | deleted
service_status = pending_activation | active | expired | suspended
service_started_at
service_expires_at
aggregate version
```

第一阶段 If-Match 统一使用现有 Store aggregate `version`，不引入 `service_version`。

ConnectionStatus 继续复用 `internal/storecenter.ConnectionStatus`，internal + wire enum 固定：

```text
disconnected
connected
expired
unavailable
```

映射：

```text
no connection ref -> disconnected
healthy + usable -> connected
expired/revoked/invalid credential -> expired
timeout/network/stale/provider error -> unavailable
```

Activate 只有 `connected` 可通过。

Effective expiry 为读时权威：persisted active 且 `service_expires_at <= now` 时 effective service status = expired。Materializer 只是优化。

---

## Task 9：StoreResourceUnitOfWork + fixed lock order + audit

Activate / Renew / Reactivate / CorrectStoreServiceOperation 使用共享 PostgreSQL Unit of Work：

```text
lock/recheck Operation
lock Store
lock Bucket
lock related Reservation/source claim when applicable
mutate Resource
append Event
mutate Store
append Store Audit / Audit Outbox
persist immutable result snapshot
complete Operation
COMMIT
```

锁顺序全局固定，配合 Task 1 的 lock/statement/transaction deadlines。

Audit 至少记录：

```text
subject
home organization
effective organization
store/resource
action/result
quantity
request/operation id
resulting Store status/expiry
```

Audit 写入失败整个业务事务回滚。

DB COMMIT response 丢失进入 `commit_outcome_unknown`：使用健康连接按 `(organization_id,operation_id)` 重读；durable completed 则 replay，只有 authoritative absence 才允许重新执行。

---

## Task 10：Activate / Renew / Reactivate lifecycle commands

### Activate

前置：

```text
record_status = active
effective service_status = pending_activation
connection_status = connected
available store_renewal_period >= 1
If-Match aggregate version
```

同事务 Consume 1，写：

```text
service_status = active
service_started_at = now
service_expires_at = now + 30d
aggregate version++
```

### Renew(N)

前置：

```text
record_status = active
effective service_status = active
N > 0 and bounded
```

同事务 Consume N：

```text
new_expiry = max(now,current_expiry) + N*30d
aggregate version++
```

### Reactivate(N)

前置：

```text
record_status = active
effective service_status = expired
N > 0 and bounded
```

同事务 Consume N：

```text
service_status = active
service_started_at = now
service_expires_at = now + N*30d
aggregate version++
```

`deleting/deleted/suspended` 都不得消耗 renewal periods。

Durable Operation replay 必须发生在 volatile ConnectionStatusProvider 检查之前。

---

## Task 11：Strict API / BFF / Client contract

Backend routes：

```text
POST /api/v1/workbench/stores/:store_id/activate
POST /api/v1/workbench/stores/:store_id/renew
POST /api/v1/workbench/stores/:store_id/reactivate
```

BFF `workbench-proxy.ts` 必须显式加入三项 allowlist，增加严格 body schema、client methods、error schema，并只转发 allowlisted：

```text
Idempotency-Key
If-Match
```

新 Store read/mutation contract 显式包含：

```text
recordStatus
serviceStatus
serviceStartedAt
serviceExpiresAt
version
connectionStatus
```

不把新 service enum 塞回 legacy lifecycleStatus。

BIGINT resource balance/quantity/delta 使用 decimal string + Zod string；需要计算时显式 `BigInt()`。

固定 error code/status：

```text
STORE_VERSION_CONFLICT              409
STORE_INVALID_STATE                 422
STORE_CONNECTION_NOT_CONNECTED      422
STORE_CONNECTION_UNAVAILABLE        503
RESOURCE_QUANTITY_INVALID           422
RESOURCE_INSUFFICIENT_BALANCE       409
IDEMPOTENCY_KEY_CONFLICT             409
STORE_SERVICE_RESUME_REQUIRED       409
RESOURCE_CONCURRENCY_RETRY          503
CORRECTION_SOURCE_MISMATCH          422
CORRECTION_QUANTITY_MISMATCH        422
STORE_SERVICE_ALREADY_CORRECTED     409
MIGRATION_OWNERSHIP_CHANGED         409
```

Backend/BFF/client tests 必须逐项一致，不允许同 code 多 status。

---

## Task 12：Store Service Correction 只支持“成功一次 + 全量纠正”

新增：

```text
saas_store_service_correction_claims
UNIQUE (organization_id, source_lifecycle_operation_id)
```

Correction Operation 的 failed_terminal 不插入 claim，因此条件修复后允许用新 Operation ID 再尝试。

成功前锁 source lifecycle Operation + immutable snapshot，并验证：

```text
source organization == request effective org
source store_id == target store
source resource_type == store_renewal_period
source state == succeeded
source is coupled_store_service
requested_refund_quantity == source original consumed_quantity
no successful correction claim exists
```

第一阶段 partial correction 禁止；quantity 不等于原 consume delta 返回 `CORRECTION_QUANTITY_MISMATCH`。

成功 claim、Store correction、Resource refund/debt handling、Events、Audit、snapshot、Operation succeeded 同事务提交。

---

## Task 13：Migration Credit 使用 source lock/snapshot + global claim

Global claim：

```text
saas_resource_migration_claims
UNIQUE (source_table, source_primary_key, resource_type)
```

稳定 Operation ID：

```text
migration-credit:{source_table}:{source_primary_key}:{resource_type}
```

同数据库 source：事务内先 `SELECT ... FOR UPDATE` source row；外部/不可锁 source：先建立 immutable migration snapshot + source version/etag 条件确认。

同一事务：

```text
lock/freeze source
create/lock global claim
validate source fingerprint + organization
create/replay tenant Operation
mutate Bucket
append MigrationCredit Event
mark Operation succeeded
mark claim applied
COMMIT
```

source 改绑 Organization 返回 `MIGRATION_OWNERSHIP_CHANGED`，不能给新 Organization 再 credit。

partial-run restart 同 source/fingerprint replay，不重复入账。

---

## Task 14：Store hard-cut compatibility writer fence

Phase 0 先全量部署 dual-write compatibility，不只 Disable/Delete。

### Create / ResumeCreate

首次 Create 持久化：

```text
legacy lifecycle = provisioning
record_status = active
service_status = pending_activation
service_started_at = null
service_expires_at = null
```

Create/ResumeCreate 完成连接配置后仍 `pending_activation`，等待显式 Activate 消耗续费期数。

### Disable

同事务：

```text
legacy disabled
service_status = suspended
```

### Delete

```text
BeginDelete -> legacy deleting + record_status=deleting
SoftDelete  -> deleted_at + record_status=deleted
```

旧 `/enable` 对 suspended 返回 `STORE_SERVICE_RESUME_REQUIRED`，不承担 service resume。

未来服务恢复使用独立：

```text
POST /api/workbench/stores/{storeId}/service/resume
PermissionWorkbenchStoreLifecycle
```

不复用已有 ResumeCreate route。

固定 rollout：

```text
1. deploy schema + dual-write compatible writer/read
2. wait all pre-dual-write pods drained OR minimum-writer-version fence active
3. backfill record/service state
4. verify
5. enable new read contract/lifecycle feature
```

Hard cut 后 rollback 只能做 feature rollback：关闭新 routes/workers，但保留 schema、dual-write、new-state-aware reads、writer fence；不得部署 pre-dual-write binary。

---

## Task 15：Effective expiry Materializer 只做优化

Materializer update 必须包含：

```text
record_status = active
persisted service_status = active
service_expires_at <= now
expected aggregate version
```

0 rows affected 说明并发 Renew/Reactivate/other lifecycle 已推进，重新读取即可，禁止 stale materializer 覆盖新 expiry。

---

## Task 16：PostgreSQL acceptance / fault-injection suite

SQLite 单测可保留做纯 repository 快测，但以下必须运行 PostgreSQL testcontainers/harness：

```text
Bucket 首次并发创建
Operation idempotency race
Event composite FK
Reservation Commit/Release double settlement
reconciliation_required durable scan + lease takeover
owner cross-tenant/resource rejection
Store + Bucket SELECT FOR UPDATE ordering
40001 serialization retry
40P01 deadlock retry
55P03 lock timeout retry
lock_timeout / statement_timeout budget exhaustion -> 503
lost COMMIT response replay
Resource Debt intercept all positive available paths
successful correction claim uniqueness
failed correction does not block later retry
partial correction rejected
migration source row/snapshot concurrency
migration global claim restart-idempotency
Create/ResumeCreate vs backfill race
Disable/Delete vs backfill race
Materializer vs Reactivate race
ConnectionStatus connected/expired/unavailable strict contract
BFF activate/renew/reactivate allowlist + header forwarding
BIGINT 2^53-1 / 2^53 / 2^53+1 lossless wire tests
admin resource command permission denial tests
Audit write failure rolls back Store + Resource + Operation
feature rollback after Activate/Suspend/Expiry/Delete
```

---

## 完成定义

- [ ] Bucket `(organization_id, resource_type)` 唯一，Resource Debt 同 org/resource 隔离。
- [ ] Operation `(organization_id, operation_id)` 是幂等权威并保存 immutable replay snapshot。
- [ ] Event/Reservation/Operation 引用均 tenant-scoped；reservation Event 绑定同 org/resource Reservation。
- [ ] 所有 quantity 输入在 Operation 前正数校验，客户端不能传 signed delta。
- [ ] Reservation `reserved/reconciliation_required` 都有可重启 settlement owner，unknown outcome fail closed。
- [ ] PostgreSQL lock/transaction wait 有 deadline、SQLSTATE retry classifier 和总预算。
- [ ] 高风险 concurrency/fault tests 使用 PostgreSQL testcontainers，不以 SQLite 结果替代。
- [ ] Store ConnectionStatus wire contract 保留 disconnected/connected/expired/unavailable。
- [ ] Activate/Renew/Reactivate 资源扣减、Store 更新、Audit、Operation snapshot 同事务。
- [ ] Correction failed attempt 不占 successful source claim；成功只允许一次且全量 correction。
- [ ] Migration Credit 锁/冻结 source，与 global claim + credit 同事务。
- [ ] 管理资源命令有明确 internal/admin permission boundary。
- [ ] Strict backend/BFF/client error code/status 与 BIGINT wire contract 完全一致。
- [ ] Create/ResumeCreate/Disable/Delete 在 hard cut 前全部 dual-write 新状态。
- [ ] hard cut 后只允许 feature rollback，不部署 pre-dual-write writer。
- [ ] Resource Ledger / Store Service 与账号注册/ZITADEL/Onboarding 保持解耦。
