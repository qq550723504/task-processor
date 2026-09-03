# 硕米 Resource Ledger / Store Service 第八轮评审修订

本文件针对 PR #284 在 `92dff29` 上完成的最新 Code Review / Security Review 继续收敛。与前序文档冲突时，以本文件和 `2026-09-03-shuomi-resource-ledger-store-plan-v3.md` 为准。

## 1. Store Service Correction 必须保持 Aggregate Version 单调递增

Correction 的目标是恢复 source operation 之前的业务状态，不是回滚并发控制历史。

因此 source lifecycle Operation 的 immutable before/after snapshot 中：

```text
before_service_status / started_at / expires_at
before_aggregate_version
after_service_status / started_at / expires_at
after_aggregate_version
```

Correction 执行时：

```text
lock current Store
require current Store service fields == source after snapshot
require current aggregate_version == source after_aggregate_version
restore service status/start/expiry from source BEFORE snapshot
NEW aggregate_version = current aggregate_version + 1
```

**禁止**把 `before_aggregate_version` 写回 Store。before version 只用于 lineage/audit，不是恢复值。

Correction immutable result snapshot 必须记录新版本；任何持有旧 `If-Match` 的请求都不能因 Correction 把 version 倒退而重新合法。

## 2. 新增 Lifecycle Error 的唯一 HTTP Contract

补入共享 backend/BFF/client code-status table：

```text
STORE_SERVICE_STATE_CORRUPT          -> 409 Conflict
STORE_CORRECTION_NOT_INVERTIBLE      -> 409 Conflict
```

这两个 code 进入严格 proxy allowlist / Zod error schema / client contract tests。不得在不同层选择不同 status。

`STORE_SERVICE_STATE_CORRUPT` 表示持久化 Store service invariants 已损坏，不能安全执行生命周期命令；`STORE_CORRECTION_NOT_INVERTIBLE` 表示 source 已不是当前安全可逆的最近操作，不能自动推导纠错。

## 3. Inactive Record 的 `service_status` 使用条件空值，不制造伪 Service State

`record_status=deleting|deleted` 时 Store 已不参与服务生命周期，不应为了 `NOT NULL` 人为赋予 `expired/suspended` 等错误语义。

权威约束改为：

```text
record_status = active
=> service_status IS NOT NULL

record_status IN (deleting, deleted)
=> service_status IS NULL
```

时间约束：

```text
service_status IN (active, expired)
=> service_started_at IS NOT NULL
AND service_expires_at IS NOT NULL
AND service_expires_at > service_started_at
```

Backfill：

```text
deleted_at != NULL -> record_status=deleted, service_status=NULL
legacy deleting -> record_status=deleting, service_status=NULL
legacy disabled -> record_status=active, service_status=suspended
other active record -> infer pending_activation/active/expired
```

Phase D Verify 的“零 NULL”只针对 `record_status=active` 的 `service_status`；Phase E 使用条件 CHECK，而不是全表 `service_status SET NOT NULL`。

Read DTO 对 deleting/deleted 可返回 `serviceStatus: null`；所有 lifecycle command 已因 record_status 非 active fail closed。

## 4. Credit Correction Quantity 必须从 Locked Source Event 派生

`AdjustDebit`/Credit Correction 的金额不能由请求声明 `original_credit_quantity`。

Canonical command 第一阶段只接受：

```text
source_credit_event_id
correction operation id / idempotency key
reason / audit metadata
```

事务内锁 source Event 后从 immutable source fact 派生：

```text
gross_credit_quantity = source_event.gross_credit_quantity
```

并校验 same organization/resource、allowed credit type、未被成功 correction。

若兼容旧 internal caller 暂时仍携带 quantity，则该字段只作断言，必须 `request_quantity == source_event.gross_credit_quantity`；不一致直接 conflict，绝不能作为计算权威。

Debt/reclaim 都使用 source-derived quantity。

## 5. `AdjustCredit` 不再是 Tenant Human Command

企业资源余额代表已购买/批准资源，正向 mint 不能由普通 tenant admin 通过新 Operation ID 无限执行。

Phase1 权限重新固定：

### Tenant Human `PermissionWorkbenchResourceAdmin`

允许：

```text
AdjustDebit            // 必须 source-bound correction
Compensate             // 必须 source-bound、领域允许
CorrectStoreServiceOperation
```

不允许：

```text
AdjustCredit
Grant
MigrationCredit
```

### Trusted Financial / Provisioning Principal

正向余额创建只允许：

```text
Grant
AdjustCredit（如确实保留该命令）
```

并必须同时具有：

```text
trusted internal/service principal
immutable approved source/reference
stable source-level idempotency claim
organization/resource/quantity from approved source
mandatory audit
```

不能只有“平台管理员权限 + 手填 quantity”。如果没有已批准的 source，Phase1 不注册 AdjustCredit API，改由 Billing/Provisioning 的 Grant 流程处理。

MigrationCredit 仍只允许 migration runner + immutable migration source claim。

## 6. 第八轮验收矩阵

至少新增：

```text
Activate version 5->6, Correction -> business state restored but version becomes 7
old If-Match:5 after correction -> conflict
STORE_SERVICE_STATE_CORRUPT -> exact 409 across backend/BFF/client
STORE_CORRECTION_NOT_INVERTIBLE -> exact 409 across backend/BFF/client
deleted/deleting legacy row -> service_status NULL and migration validation succeeds
active row -> service_status NULL rejected by conditional constraint/rehydration
source credit=10 + request claims 100 -> reject; no 100-unit debt/reclaim
source credit=10 -> correction derives exactly 10 from locked Event
tenant listingkit_admin -> AdjustCredit denied/not routed
trusted financial principal + approved source -> positive credit allowed exactly once
fresh Operation IDs cannot mint repeated credit from same approved source
```

## 7. 领域边界不变

本修订只属于 Resource Ledger / Store Service：不引入 ZITADEL、账号注册、Onboarding 或支付钱包实现。
