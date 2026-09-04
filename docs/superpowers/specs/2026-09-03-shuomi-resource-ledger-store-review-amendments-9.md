# 硕米 Resource Ledger / Store Service 第九轮评审修订

本文件针对 PR #284 Head `29b3c8c` 的最新 Code Review / Security Review 继续收敛。与前序文档冲突时，以本文件和新的 `2026-09-03-shuomi-resource-ledger-store-plan-v4.md` 为准。

## 1. Phase1 Migration Credit 不支持未锁定的外部 Source

旧 V3 的“external/unlockable source -> snapshot + ETag verify”仍存在检查后、PostgreSQL credit 提交前被外部修改的窗口。

Phase1 固定：**MigrationCredit 的权威 source 必须位于同一 PostgreSQL 可锁事务边界内。**

允许：

```text
same PostgreSQL source row
-> SELECT ... FOR UPDATE
-> claim + Operation + Bucket + Event + source applied
-> one transaction
```

不允许直接：

```text
remote API row
external DB row
read-only ETag/source-version check
```

如果历史数据来自外部系统，必须先由受信 migration process 把它 materialize 成本地 immutable staging record；staging record 成为本次 MigrationCredit 的唯一 source identity，导入过程本身不属于 Resource Ledger transaction。未来若要直接跨系统迁移，必须另设计 source-side durable freeze/claim protocol。

## 2. Reservation Owner 必须有自己的 Durable Recovery Contract

Reservation Reconciler 不能替 Owner 猜测 `processing/outcome_unknown` 最终结果。

Phase1 只有实现以下合同的 owner type 才能创建 Reservation：

```text
owner durable state
owner organization/business scope
owner lease/epoch or equivalent fence
owner recovery deadline / next_at
owner-specific reconciler or existing durable workflow
terminal proof: succeeded | failed | cancelled
```

Owner domain 负责把 crash 后的 `processing/outcome_unknown` 收敛到 terminal proof；Reservation Reconciler 只消费 terminal proof 决定 Commit/Release。

如果一个 owner type 无法证明 crash recovery / bounded finality，则 Phase1 不允许它使用 Resource Reservation。

可以复用已有 PAY-042 recovery 或仓库已有 Temporal workflow；禁止为了本 PR 再造一个通用 Saga 引擎。

## 3. Owner Start 与 Recovery Release 必须共享 Fence

`not_started` 不能只读取后直接 Release，因为 owner 可能在锁释放后开始工作。

对同 PostgreSQL owner：

### Start

```text
BEGIN
lock Owner
lock Reservation
require owner=not_started
require reservation=reserved
owner -> processing
COMMIT
```

### Expired Not-started Recovery

```text
BEGIN
lock Owner
lock Reservation
require owner=not_started
require reservation=reserved/reconciliation_required as allowed
owner -> cancelled_terminal / recovery_fenced
Reservation -> released
Bucket/Event/Operation settlement
COMMIT
```

Owner Start 看到 `cancelled_terminal/recovery_fenced` 或 Reservation 已非 reserved 必须拒绝。

无法提供等价原子 start fence 的 owner type 不允许进入 Phase1 Reservation API。

## 4. Transitional Rollout 必须 Fence Legacy `/enable`

Phase A 新列为 NULL 时，旧 disabled row 仍可能被现有 `/enable` 改成 legacy active，导致 Backfill 丢失 suspension。

Phase B compatibility writer matrix 增加 `/enable`：

```text
legacy disabled + service_status IS NULL -> treat as suspended transitional row
legacy disabled + service_status=suspended -> suspended
both cases -> STORE_SERVICE_RESUME_REQUIRED
NO legacy active transition
NO resource/expiry change
```

只有未来独立 `/service/resume` 才能恢复 suspended service。

Backfill 前必须完成包含 Enable guard 的 minimum-writer-version fence 并 drain 所有旧 Pod。

## 5. `record_status` Backfill 后必须全表非空

`service_status` 继续 conditional nullable，但 `record_status` 是每一条 Store 记录的权威 record lifecycle，不能为 NULL。

Phase D 必须验证：

```text
COUNT(record_status IS NULL) = 0
active -> service_status IS NOT NULL
deleting/deleted -> service_status IS NULL
```

Phase E 在 enable 新 reads 前：

```text
VALIDATE record_status enum CHECK
SET record_status NOT NULL
VALIDATE conditional service_status CHECK
```

不对全表 `service_status SET NOT NULL`。

## 6. Generic Compensate 不再开放给 Tenant Human Admin

`CompensateConsume` 会把 consumed 资源重新变成 available，本质是 value-returning correction。仅有 same-org + ResourceAdmin + source event 不足以证明业务交付失败。

Phase1：

- `PermissionWorkbenchResourceAdmin` **不再允许** generic `Compensate`；
- tenant human admin 仅保留 source-bound `AdjustDebit` 与 paired `CorrectStoreServiceOperation`；
- generic compensation 只允许 trusted Billing/Reconciliation/Service principal；
- 必须有 immutable trusted failure/correction proof / approval reference；
- quantity 从 locked source consume Event 派生，客户端不提供返还量权威；
- source-level successful claim 保证一个 consume source 最多一次成功 compensation；
- coupled Store Service consume 仍禁止 generic compensation，只走 paired Store Service Correction。

如果没有可信 failure/approval source，Phase1 不注册 generic Compensate HTTP API。

## 7. 第九轮验收

至少新增：

```text
external source ETag-only MigrationCredit -> unsupported
local PostgreSQL migration source row changes concurrently -> FOR UPDATE serializes
owner crash in processing -> owner-specific recovery terminalizes -> Reservation settles
owner type without durable recovery -> cannot Reserve
not_started owner Start vs Recovery Release -> exactly one wins
legacy disabled+NULL new state calls /enable during backfill -> RESUME_REQUIRED, remains suspended
backfill skipped row with NULL record_status -> validation blocks enable
record_status NOT NULL after Phase E; deleted service_status remains NULL
tenant listingkit_admin calls generic Compensate -> deny
trusted reconciliation + immutable failure proof -> derive source quantity and compensate once
same source fresh Operation ID -> compensation claim prevents second refund
```

## 8. 领域边界不变

本轮只修改 Resource Ledger / Store Service；不引入账号、ZITADEL、Onboarding 逻辑。