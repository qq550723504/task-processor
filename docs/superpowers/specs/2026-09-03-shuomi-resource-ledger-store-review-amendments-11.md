# 硕米 Resource Ledger / Store Service 第十一轮评审修订

本文件针对 PR #284 V5 最新 Code/Security Review 继续收敛。与前序文档冲突时，以本文件和 `2026-09-03-shuomi-resource-ledger-store-plan-v6.md` 为准。

## 1. Store Service Correction 不再允许 Tenant Human 直接执行返还

V5 允许同组织 `listingkit_admin` 执行 paired Store Service Correction，仍可形成经济价值循环：先 Activate/Renew 获得服务，临近到期再 Correction 返还完整 renewal period，然后重新消费。

Phase1 权限改为：

```text
Tenant human PermissionWorkbenchResourceAdmin
  allow: source-bound AdjustDebit
  deny:  Store Service Correction execution

Trusted Billing / Reconciliation / Support-Finance principal
  allow: Store Service Correction with immutable approval/proof
```

Tenant admin 未来可以有“提交纠错申请”能力，但本 PR 不实现直接 value-returning execution。

Store Service Correction proof 至少：

```text
proof_id
organization_id
store_id
source_lifecycle_operation_id
decision = full_store_service_correction
reason_class
approved_at
issuer
evidence_reference
```

自动型 `reason_class=delivery_failed_before_benefit` 必须有 immutable delivery-failure evidence；商业例外只能由明确 platform finance/support approval principal 签发并审计。普通 tenant user 不能自签 proof。

Correction transaction 同时锁 proof、source lifecycle Operation、Store、Bucket，并 exact match org/store/source/decision；quantity 继续从 source immutable consumed quantity 派生。成功同事务写 proof-level claim + source-level successful correction claim + Resource/Event/Store/Audit。

因此“latest source + current AFTER snapshot”只证明可逆性，不再被当成退款授权。

## 2. Reservation Owner Attempt 唯一性必须包含 Organization

Owner Attempt ID 允许 tenant-local，因此唯一约束改为：

```text
UNIQUE(
  organization_id,
  owner_type,
  owner_attempt_id,
  resource_type,
  reservation_purpose
)
```

Owner/Reservation composite binding 与所有 lookup/lock 也必须包含 `organization_id`，避免企业 A 的 tenant-local attempt ID 阻塞企业 B。

## 3. Store Backfill 必须与 Compatibility Writer 原子串行

Phase C 不允许先读后无条件 UPDATE。

每个 batch/row 使用 PostgreSQL concurrency authority：

```text
BEGIN
SELECT target row ... FOR UPDATE [SKIP LOCKED]
re-read legacy lifecycle + deleted_at + new record/service fields + aggregate version
if new-state already written by Phase B compatibility writer:
    validate compatible and NO-OP
else:
    derive mapping from locked current row
    UPDATE only this locked/expected version row
COMMIT
```

可等价使用 `UPDATE ... WHERE record_status IS NULL AND version=:expected AND legacy_predicate... RETURNING ...` CAS，但必须在失败后重新读取，不能覆盖 Phase B writer 已写的 `suspended/deleting/deleted/pending_activation`。

至少覆盖 Backfill vs Disable、Delete、Create/ResumeCreate、legacy Enable guard 并发；任意竞争顺序最终都不能把 suspended/deleting/deleted 覆盖回 pending_activation/active。

## 4. Legacy `provisioning` Store 必须先 Drain/Reconcile，不能直接 Backfill

现有 Store aggregate 的 `provisioning` 是 durable interrupted-Create state。Phase B 全量兼容 writer 部署后、Phase C backfill 前新增 **Provisioning Drain Gate**：

1. 扫描所有 legacy `lifecycle_status=provisioning`；
2. 优先复用现有 `ResumeCreate`/Create recovery；
3. 成功完成的 row 由 Phase B compatibility writer 写入 `record_status=active + service_status=pending_activation`；
4. 可确定删除/失败终态按既有 record lifecycle 处理；
5. 无法自动收敛的 provisioning row 标记 rollout repair blocker，禁止硬切。

Phase C backfill 本身跳过 unresolved provisioning。Phase D 要求：

```text
COUNT(legacy lifecycle_status = provisioning) = 0
```

或等价“zero unresolved provisioning blockers”。不得把 interrupted Create 直接猜成 active service。

## 5. `MIGRATION_SOURCE_CHANGED` Phase1 是 Internal-only Outcome

Phase1 MigrationCredit 只允许 trusted migration runner，**不注册 tenant/browser HTTP/BFF route**。

因此：

```text
MIGRATION_SOURCE_CHANGED
MIGRATION_OWNERSHIP_CHANGED
```

属于 migration runner typed outcome / metrics / reconciliation queue，不进入 Store BFF strict HTTP error map。

若未来增加任何 HTTP admin endpoint，注册 route 前必须把这两个 code 加入共享 one-code/one-status contract；推荐固定 409，但本 PR 不提前暴露不存在的 HTTP API。

Contract test 要证明当前 Workbench/BFF allowlist 无 MigrationCredit route。

## 6. 第十一轮验收

至少新增：

```text
tenant listingkit_admin executes Store Service Correction -> denied
trusted correction without source-bound proof -> denied
trusted proof for source A used against source B -> denied
trusted proof reused -> denied
trusted approved correction -> Store+Resource+Audit atomically succeeds once
org A/B same tenant-local owner_attempt_id -> both can Reserve independently
Backfill lock first, concurrent Disable waits then writes suspended -> final suspended
Disable writes first, Backfill sees non-null new state -> NO-OP suspended
Backfill vs Delete -> never resurrect active service
legacy provisioning row -> ResumeCreate/reconcile before Phase C
unresolved provisioning blocker -> Phase F hard cut denied
MIGRATION_SOURCE_CHANGED never crosses tenant/browser BFF in Phase1
```

## 7. 领域边界不变

本轮只处理 Resource Ledger / Store Service；不引入账号、ZITADEL、Onboarding 逻辑。