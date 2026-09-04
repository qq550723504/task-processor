# 硕米企业资源账本与 Store Service 第六轮评审修订

> 本文覆盖此前 design、amendments 1-5 与 Implementation Plan 中的冲突项。本 PR 继续只处理 Resource Ledger 与 Store Service，不引入账号注册/ZITADEL/Onboarding 逻辑。

## 1. Implementation Plan 必须把完整 amendment chain 作为强制权威输入

Implementation Plan 顶部必须列出原始 design + amendments 1-6，并把被覆盖的旧指令直接改写。不能再把 `result_reference`、`service_version`、仅 `reserved` settlement、SQLite transaction retry 等旧语义留给执行者自行判断。

## 2. Store Service Correction：失败 Operation 不占 source claim，成功必须全量 correction

第一阶段不支持 partial correction。

新增成功事实：

```text
saas_store_service_correction_claims
organization_id
source_lifecycle_operation_id
correction_operation_id
corrected_quantity
created_at
UNIQUE (organization_id, source_lifecycle_operation_id)
```

规则：

1. Correction Operation 可以因版本/Store 状态等业务条件进入 `failed_terminal`；失败 Operation **不插入 correction claim**。
2. 成功事务中才锁 source lifecycle Operation + immutable snapshot，验证 source 尚无成功 claim，然后原子插入 claim。
3. `requested_refund_quantity` 必须 **等于** source immutable snapshot 的原始 `consumed_quantity`；小于或大于都返回 `CORRECTION_QUANTITY_MISMATCH`。
4. 成功 claim、Resource Events、Store 纠正、Operation succeeded、immutable result snapshot、Store Audit/Outbox 同事务提交。

未来若要 partial correction，必须另行引入 authoritative `remaining_correctable_quantity`，本阶段不实现。

## 3. Migration Credit 必须锁定/冻结 source 本身

只锁 `saas_resource_migration_claims` 不能阻止 source row 在校验后被改绑 Organization 或改 quantity。

同数据库 source：Migration transaction 必须 `SELECT ... FOR UPDATE` source row，并把 `source_version / organization_id / quantity / resource_type` 纳入 fingerprint 与条件。

不能直接锁的外部/legacy source：先生成不可变 migration snapshot row，再只从 snapshot credit；snapshot 创建必须带 source version/etag 并在源侧做条件确认。

固定顺序：

```text
lock/freeze source
-> create/lock global source claim
-> validate source fingerprint
-> create tenant Operation
-> mutate Bucket
-> append MigrationCredit Event
-> mark Operation succeeded
-> mark claim applied
-> COMMIT
```

测试 source owner/quantity 在 credit 事务中并发变化，必须导致等待、version conflict 或 snapshot conflict，不能提交 stale credit。

## 4. ConnectionStatus 保留现有 canonical enum

不得创建与现有 `internal/storecenter.ConnectionStatus` 冲突的新 enum。Canonical internal + wire enum 固定为：

```text
disconnected
connected
expired
unavailable
```

映射：

- 无 connection ref -> `disconnected`
- provider healthy + credential usable -> `connected`
- credential expired/revoked/invalid -> `expired`
- timeout/network/stale/provider error -> `unavailable`

Activate 只有 `connected` 可通过。前后端 strict schema 与 provider adapter 都使用这四值；不得把 `revoked/error` 作为新的 wire value。

## 5. 所有余额变更管理命令必须有明确 caller/permission boundary

普通 Store lifecycle：

```text
verified identity
+ live effective organization
+ PermissionWorkbenchStoreLifecycle
```

管理资源命令：

```text
AdjustCredit / AdjustDebit / CompensateConsume / CorrectStoreServiceOperation
-> verified identity
-> live effective organization == target organization
-> PermissionListingKitAdminWrite
-> mandatory audit
```

`Grant` 仅允许受信任内部 Billing/Provisioning service principal 调用，不注册普通浏览器 Workbench route；target organization 来自受信任业务输入并做存在/live 校验。

`MigrationCredit` 仅允许 migration runner service principal，且必须携带已验证 source claim/snapshot；不允许普通用户 API 调用。

所有命令补 viewer/operator/admin/internal-principal 正反授权测试。

## 6. Strict lifecycle API 固定 error code / HTTP status

第一阶段使用唯一映射：

```text
STORE_VERSION_CONFLICT              -> 409
STORE_INVALID_STATE                 -> 422
STORE_CONNECTION_NOT_CONNECTED      -> 422
STORE_CONNECTION_UNAVAILABLE        -> 503
RESOURCE_QUANTITY_INVALID           -> 422
RESOURCE_INSUFFICIENT_BALANCE       -> 409
IDEMPOTENCY_KEY_CONFLICT             -> 409
STORE_SERVICE_RESUME_REQUIRED       -> 409
RESOURCE_CONCURRENCY_RETRY          -> 503
CORRECTION_SOURCE_MISMATCH          -> 422
CORRECTION_QUANTITY_MISMATCH        -> 422
STORE_SERVICE_ALREADY_CORRECTED     -> 409
MIGRATION_OWNERSHIP_CHANGED         -> 409
```

Backend、BFF proxy、client Zod error schema 和 contract tests 必须完全一致；不允许同 code 返回多个 status。

## 7. PostgreSQL-aware retry classifier 与有界 transaction/lock budget

新 Unit of Work 不复用 SQLite-only `moderncsqlite` retry predicate。

PostgreSQL SQLSTATE：

```text
40001 serialization_failure -> retryable
40P01 deadlock_detected      -> retryable
55P03 lock_not_available     -> retryable within total budget
```

若 caller context 已取消/超时则不重试。

每次 lifecycle/resource transaction 必须同时有：

```text
request context deadline
SET LOCAL lock_timeout
SET LOCAL statement_timeout
bounded transaction timeout
max attempt count
single total retry budget with jitter
```

预算耗尽统一返回 `RESOURCE_CONCURRENCY_RETRY` / 503；不能无限等待 Store/Bucket row lock 或耗尽连接池。

## 8. 锁与 settlement 验收必须跑 PostgreSQL testcontainers

SQLite 单测可保留做纯 repository 快速测试，但以下验收必须使用仓库 PostgreSQL testcontainers/harness：

```text
SELECT ... FOR UPDATE settlement races
Store + Bucket lock ordering/deadlock retry
partial/composite unique constraints
migration source claim/source locking
lost COMMIT response
lock_timeout / statement_timeout
40001 / 40P01 / 55P03 retry classification
correction successful-claim uniqueness
```

## 9. Create / ResumeCreate 也必须 compatibility dual-write 新状态

Phase 0 writer fence 不能只覆盖 Disable/Delete。

首次 Create 持久化 Store 时：

```text
legacy lifecycle = provisioning
record_status = active
service_status = pending_activation
service_started_at = null
service_expires_at = null
```

Create 成功完成连接配置后仍为 `pending_activation`，等待显式 Activate 消耗续费期数。

`ResumeCreate` 必须使用同一初始化/完成规则，并保持 record/service status 不缺失。

Disable/Delete 继续同步 `suspended` / `deleting|deleted`。

Backfill 前必须全量部署这套 Create/ResumeCreate/Disable/Delete dual-write，并等待旧 writer drain / minimum-writer-version fence 生效。

## 10. `reconciliation_required` 必须有 durable scanner + lease handoff

Reservation 进入 `reconciliation_required` 的同一事务必须写：

```text
reconcile_next_at
reconcile_attempts
reconcile_lease_owner
reconcile_lease_until
reconcile_epoch
```

独立 Reservation Reconciler 周期扫描：

```text
state = reconciliation_required
AND reconcile_next_at <= now
AND lease absent/expired
```

使用 `FOR UPDATE SKIP LOCKED` claim，设置 lease/epoch；owner outcome 仍 unknown 时更新 next_at + bounded backoff，terminal 时 fenced Commit/Release。

worker crash 后 lease 过期可由其他实例接管；不能依赖内存 handoff。

## 11. BFF / wire BIGINT / lifecycle action contract 保持第五轮结论

继续强制：

- `activate / renew / reactivate` 显式进入 `workbench-proxy.ts` allowlist；
- `Idempotency-Key` / `If-Match` 只按 allowlist 转发；
- BIGINT 余额、quantity、delta 在 HTTP/JSON 使用 decimal string；
- Event `reservation_id` 通过 `(organization_id,reservation_id,resource_type)` 绑定 Reservation；
- lifecycle replay 使用 immutable response snapshot；
- If-Match 第一阶段使用 Store aggregate `version`，不是 `service_version`。

## 12. 新增验收测试

```text
failed_terminal correction 后新 Operation 可再次尝试
successful correction 后任意新 Operation 都 already_corrected
refund quantity != original consumed delta -> reject
migration source row 在 claim/credit 间被修改 -> stale credit 不能提交
connection expired/unavailable strict wire contract
viewer/operator 不能 Adjust/Compensate/Correct
internal migration/billing principal 权限边界
每个 lifecycle error code/status 精确匹配
PostgreSQL 40001/40P01/55P03 bounded retry
lock_timeout 后 503 RESOURCE_CONCURRENCY_RETRY
Create/ResumeCreate 与 backfill 并发不产生空新状态
reconciliation_required transition 后进程崩溃 -> scanner 仍接管并最终 settle
```
