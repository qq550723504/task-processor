# 硕米 Resource Ledger / Store Service 第十轮评审修订

本文件针对 PR #284 V4 最新评审继续收敛。与前序文档冲突时，以本文件和 `2026-09-03-shuomi-resource-ledger-store-plan-v5.md` 为准。

## 1. 外部历史数据 Staging 必须保留稳定 External Identity

本地 staging PK 不能作为 MigrationCredit source identity。

Staging 至少保存：

```text
source_system
source_type
source_record_id
source_version
organization_id
resource_type
quantity
payload_fingerprint
```

数据库唯一：

```text
UNIQUE(source_system, source_type, source_record_id, resource_type)
```

Importer retry：same external identity + same immutable fingerprint -> adopt/replay；same identity + changed version/org/quantity/payload -> `MIGRATION_SOURCE_CHANGED` / reconciliation，禁止创建第二 staging row。

Resource Ledger migration claim 从 external identity 派生，不能从 arbitrary staging PK 派生。

## 2. Legacy Active Store 不得猜 service expiry

旧 `workbench_stores` 没有可靠 service start/expiry 时，禁止从 create/update timestamp 推导，也禁止统一赋予 active/expired。

权威 backfill：

```text
deleted_at != NULL -> record=deleted, service=NULL
legacy deleting -> record=deleting, service=NULL
legacy disabled -> record=active, service=suspended
legacy active + VERIFIED authoritative service history/expiry -> derive active/expired
legacy active + NO authoritative service history/expiry -> record=active, service=pending_activation, started/expires=NULL
```

`pending_activation` 表示“连接/记录存在，但没有可证明的已付费服务期”。Migration 不 charge、不 credit renewal period、不发明 expiry。

Backfill report 必须统计 unknown-history -> pending_activation 数量并可审计。

## 3. Compensation Proof 必须绑定 Source Consume Event

Trusted compensation proof 最少：

```text
proof_id
organization_id
resource_type
source_consume_event_id
decision = compensate
reason/type
approved_at
issuer
```

Compensation transaction 同时锁 proof + source Event，并要求 org/resource/event/decision exact match。

数量从 source Event 派生。成功时同一事务写：

- proof-level one-time successful claim；
- source-event-level successful compensation claim；
- Bucket/Event/Operation/Audit。

因此一个 proof 不能跨多个 consume Event 重用，也不能用多个 proof 重复补同一 consume Event。

## 4. Reservation 创建必须原子绑定 Owner Attempt

Reserve transaction 必须：

```text
lock exact Owner Attempt
require owner state = not_started
verify owner organization/business scope
verify no incompatible existing reservation binding
create/adopt Reservation
persist reservation_id / resource_type / purpose on Owner Attempt
commit
```

Phase1 唯一约束至少表达：

```text
UNIQUE(owner_type, owner_attempt_id, resource_type, reservation_purpose)
```

同 attempt/purpose + same fingerprint replay；changed quantity/resource/purpose conflict。Terminal owner 不允许 late attach Reservation。

Owner Start 只有在已绑定 Reservation 且 Reservation 仍 `reserved` 时才能进入 processing。

## 5. 在线 `record_status NOT NULL` 使用 Validated Check

Phase E 不直接依赖应用 count 后做可能全表扫描的 `SET NOT NULL`。

顺序固定：

```sql
ALTER TABLE workbench_stores
  ADD CONSTRAINT workbench_stores_record_status_nn
  CHECK (record_status IS NOT NULL) NOT VALID;

ALTER TABLE workbench_stores
  VALIDATE CONSTRAINT workbench_stores_record_status_nn;

ALTER TABLE workbench_stores
  ALTER COLUMN record_status SET NOT NULL;
```

在 PostgreSQL 能利用 validated check 证明 non-null 的版本上，最后一步应为短 metadata operation。仍使用 bounded `lock_timeout` / `statement_timeout`；无法快速获取 ALTER lock 就 abort/retry later。可在成功后按需要删除冗余 check。

## 6. 第十轮验收

至少新增：

```text
same external record importer ambiguous retry -> one staging identity / one ledger claim
same external identity changed org/quantity/version -> conflict/reconciliation
legacy active without service history -> pending_activation, no invented expiry, no credit/charge
legacy active with authoritative expiry -> active/expired exact mapping
compensation proof for consume A + request consume B -> reject
same proof used twice -> reject
same consume event with two proofs -> only one successful compensation
same owner attempt fresh reserve Operation ID -> one Reservation
terminal owner late Reserve -> reject
Owner Start without bound reserved Reservation -> reject
validated NOT NULL check completes before SET NOT NULL
ALTER lock timeout -> rollout pauses/retries, writers not indefinitely blocked
```

## 7. 领域边界不变

本轮只处理 Resource Ledger / Store Service；不引入账号、ZITADEL、Onboarding 逻辑。