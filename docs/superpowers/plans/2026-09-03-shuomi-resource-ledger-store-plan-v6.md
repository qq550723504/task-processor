# 硕米 Resource Ledger / Store Service Implementation Plan V6

**目标：** 把企业资源余额、Reservation/Settlement、Store Activate/Renew/Reactivate、迁移与纠错收敛为 PostgreSQL 权威实现。本 V6 是 PR #284 唯一执行入口；账号/ZITADEL/Onboarding 不进入本计划。

## 权威输入

执行前读取原始 design 与 review amendments 1–11。冲突时以本 V6 与 amendment-11 为准。旧 plan / v2 / v3 / v4 / v5 仅保留历史。

## 1. PostgreSQL Unit of Work

Operation、Bucket、Event、Reservation、Debt、Store、Audit/Outbox、Correction/Migration/Source Claim 使用 transaction-bound repositories。统一 context deadline、`lock_timeout`、`statement_timeout`、transaction timeout 与 total retry budget；`40001/40P01/55P03/57014` 按既定 bounded classifier 处理。COMMIT response loss 必须按 `(organization_id, operation_id)` read-back。

## 2. Resource Core

Bucket `(organization_id, resource_type)` 唯一且 available/reserved/consumed 非负。Operation `(organization_id, operation_id)` 是幂等权威并保存 immutable response snapshot。业务 quantity `q>0` 且有 domain max；HTTP/JSON BIGINT 全部 decimal string。

Event/Reservation/Operation 使用 organization/resource scoped composite binding。

## 3. Operation Replay

身份/权限通过后，volatile validation 前先 fingerprint-check durable Operation；terminal 同 key 同 fingerprint replay，changed fingerprint conflict。Ambiguous commit 只在 authoritative Operation absence 时重新执行。

## 4. Durable Reservation Owner Contract

只有具备 durable identity、tenant/business scope、owner attempt state、recovery/finality contract、owner-specific reconciler/已有 PAY-042 或 Temporal workflow 的 owner type 才能 Reserve。`processing/outcome_unknown` 必须由 Owner Domain 自己收敛为 terminal proof；Reservation Reconciler 不猜结果。不满足合同的 owner rollout gate 禁止 Reserve。

## 5. Reservation Creation / Owner Attempt Binding

Reserve transaction：

```text
lock exact Owner Attempt by organization + owner identity
require state=not_started
verify organization/business scope
require no incompatible bound Reservation
create/adopt Reservation
bind reservation_id + resource_type + purpose onto Owner Attempt
write Reserve Operation/Event/Bucket
commit
```

数据库唯一至少：

```text
UNIQUE(
  organization_id,
  owner_type,
  owner_attempt_id,
  resource_type,
  reservation_purpose
)
```

Owner/Reservation composite reference、lookup、lock 全部带 `organization_id`。同 tenant attempt/purpose + same fingerprint replay；changed quantity/resource/purpose conflict；terminal owner 不允许 late Reserve。不同企业可以合法复用 tenant-local owner_attempt_id。

Owner Start 同事务锁 Owner+Reservation，要求 owner=not_started 且 bound Reservation=reserved，然后 owner->processing。Expired not-started recovery 同样锁两者，owner->cancelled_terminal/recovery_fenced，并同事务 Release。两者只能一个成功。

`reconciliation_required` 使用 durable scanner + lease/epoch；只有 Owner terminal proof 后 fenced Commit/Release。

## 6. Resource Debt

所有正向 available 回流先偿 `(organization_id,resource_type)` debt；Event 记录 gross/debt_repaid/net。覆盖 trusted credits、MigrationCredit、Reservation Release、trusted Compensation/Correction 等。

## 7. Credit Correction / AdjustDebit

Canonical command 以 `source_credit_event_id` 为权威；锁 source Event 后从 immutable gross credit quantity 派生 correction amount。same org/resource/type，成功 source claim 保证同 source 最多一次；已消费部分形成同 tenant/resource debt。

## 8. Authorization / Value-returning Boundaries

Store lifecycle：VerifiedIdentity + live org + `PermissionWorkbenchStoreLifecycle`。

Tenant human `PermissionWorkbenchResourceAdmin`：

```text
allow: source-bound AdjustDebit
deny:  AdjustCredit / Grant / MigrationCredit / generic Compensate / Store Service Correction execution
```

viewer/operator deny，listingkit_admin 只有上述 non-value-returning correction-debit 管理能力。

Positive mint 只允许 trusted Billing/Platform-Finance/Provisioning principal + immutable approved source + source-level claim + audit。

Generic Compensation 只允许 trusted Billing/Reconciliation/Service principal，proof 必须绑定 exact source consume Event：

```text
proof_id
organization_id
resource_type
source_consume_event_id
decision=compensate
reason/type
approved_at
issuer
```

Compensation tx 锁 proof+source Event，exact match 后从 source Event 派生 quantity，并原子写 proof-level + source-event-level successful claim。没有可信 proof source时不注册 generic Compensate API。

### 8.1 Store Service Correction approval

Store-coupled consume 不能 generic compensate，也不能由 tenant admin 自行退款。执行主体只允许 trusted Billing/Reconciliation/Support-Finance principal，并要求 immutable source-bound proof：

```text
proof_id
organization_id
store_id
source_lifecycle_operation_id
decision=full_store_service_correction
reason_class
approved_at
issuer
evidence_reference
```

自动 `delivery_failed_before_benefit` reason 必须有不可变 failure evidence；商业例外必须由 platform finance/support approval principal 明确批准。Tenant admin 未来可提交 request，但 Phase1 不实现直接 execution。

## 9. Connection Status

继续复用 `disconnected/connected/expired/unavailable`。revoked/invalid -> expired；timeout/network/stale/provider error -> unavailable。Activate 只允许 fresh connected；浏览器不可覆盖。

## 10. Store State

Record：`active|deleting|deleted`，hard-cut 前最终 `record_status NOT NULL`。

Service 只对 active record 存在：`pending_activation|active|expired|suspended`；deleting/deleted 的 service_status 必须 NULL。active/expired 必须具有合法 started/expires 且 expires>started；异常 -> `STORE_SERVICE_STATE_CORRUPT`/409 fail closed。

Effective expiry read-time 计算；materializer 仅优化并用 aggregate version conditional write。

## 11. Activate / Renew / Reactivate

全部要求 active record、lifecycle permission、Idempotency-Key、If-Match aggregate version，fingerprint 包含 org/command/store/quantity/version/behavior fields；锁 Store 后重验 version。

Activate：pending_activation + fresh connected，消费 1 renewal period，启动 30 days。Renew：effective active，从 current expiry 延长。Reactivate：effective expired，从 now 启动。Resource/Store/Operation snapshot/Audit 同 PostgreSQL transaction。

## 12. Exact Error + BFF

Store/browser HTTP 固定 code/status contract，包括 `STORE_SERVICE_STATE_CORRUPT=409`、`STORE_CORRECTION_NOT_INVERTIBLE=409` 和既有 version/state mappings。BFF allowlist activate/renew/reactivate，strict body/header forwarding，更新 Zod/client/UI/contract tests。

MigrationCredit Phase1 **没有 tenant/browser HTTP/BFF route**。`MIGRATION_SOURCE_CHANGED` 与 `MIGRATION_OWNERSHIP_CHANGED` 是 trusted migration runner internal typed outcomes / metrics / reconciliation queue，不进入 Workbench strict error map。未来若新增 HTTP admin endpoint，注册 route 前必须给它们定义固定 status（推荐 409）并补 contract test。

## 13. Store Service Correction

Correction 的“安全可逆”与“有权退款”是两条独立 gate，必须同时通过。

事务：

1. authorize trusted Billing/Reconciliation/Support-Finance principal；
2. lock immutable correction proof；
3. lock source lifecycle Operation + immutable BEFORE/AFTER snapshot；
4. lock Store + Bucket；
5. require proof org/store/source/decision exact match；
6. require source 是当前 Store 最近一次成功 lifecycle mutation；
7. require current service fields/version == source AFTER；
8. full quantity 从 source consumed quantity 派生；
9. require no successful source/proof claim；
10. restore BEFORE 业务 service fields；
11. correction 自身 `aggregate_version=current+1`，版本不倒退；
12. Resource correction、proof claim、source claim、result snapshot、Audit 同事务。

Tenant `listingkit_admin` 直接调用固定 deny。可逆性不再被当成退款授权。

## 14. Migration Credit / Staging Identity

Phase1 MigrationCredit 的权威 source 必须位于同一 PostgreSQL 可 `FOR UPDATE` 边界。

外部历史数据先 materialize 到 local immutable staging。Staging 保留 external identity：

```text
source_system
source_type
source_record_id
source_version
organization_id
resource_type
quantity
payload_fingerprint
UNIQUE(source_system,source_type,source_record_id,resource_type)
```

Importer ambiguous retry：same external identity+same fingerprint adopt；changed version/org/quantity/payload -> `MIGRATION_SOURCE_CHANGED` internal reconciliation，不创建第二 row。

Ledger global claim 从 external identity 派生，不从 staging PK 派生。Migration transaction锁 staging source并原子执行 claim+Operation+Bucket+Event+claim applied。

Remote API/external DB/ETag-only source 不直接入账；未来直接跨系统迁移另设计 durable source freeze/claim。

## 15. Store Hard-cut Rollout

### A. Expand

新增 nullable transitional columns/index，不启用新 reads。

### B. Compatibility Writer Fence

全量部署并验证：

```text
Create
ResumeCreate
Disable
Delete
legacy /enable guard
```

Create/ResumeCreate 完成 active record 时写 `record_status=active + service_status=pending_activation`；Disable -> suspended；Delete -> deleting/deleted。Legacy disabled + service_status NULL/suspended 调 `/enable` 一律 `STORE_SERVICE_RESUME_REQUIRED`，不转 active。

旧 Pod drain / minimum compatible writer fence 完成后，进入 B.5。

### B.5. Provisioning Drain Gate

现有 durable `lifecycle_status=provisioning` 不允许直接 Backfill。

1. 扫描所有 legacy provisioning rows；
2. 复用现有 `ResumeCreate` / Create recovery；
3. 成功完成后由 Phase B compatibility writer 落 `active + pending_activation`；
4. 可确定 delete/failure terminal 走既有 record lifecycle；
5. 无法自动收敛者进入 rollout repair blocker。

进入 Phase C 前必须 `COUNT(unresolved legacy provisioning)=0`。不得把 interrupted Create 猜成 active service。

### C. Atomic Backfill authoritative mapping

Backfill 逐 row/batch 使用 PostgreSQL row lock 或等价 CAS，绝不先读后无条件覆盖：

```text
BEGIN
SELECT row FOR UPDATE [SKIP LOCKED]
re-read legacy lifecycle + deleted_at + record/service + aggregate version
if new record/service state already written by Phase B writer:
    validate compatible, NO-OP
else:
    derive from locked current row
    UPDATE locked/expected-version row only
COMMIT
```

等价 CAS 需要 `record_status IS NULL + expected version + legacy predicate` 并在 0 rows affected 后重新读取。

权威 mapping：

```text
deleted_at != NULL -> record=deleted, service=NULL
legacy deleting -> record=deleting, service=NULL
legacy disabled -> record=active, service=suspended
legacy active + verified authoritative service history/expiry -> exact active/expired
legacy active + NO authoritative service history/expiry -> record=active, service=pending_activation, started/expires=NULL
```

禁止用 create/update timestamp 猜 expiry；unknown-history migration 不 charge、不 credit。Unresolved provisioning 在本阶段应为 0，否则 halt。

并发要求：

- Disable/Delete 先写：Backfill 看到 non-null new state -> NO-OP；
- Backfill 先锁：compatible writer 等待，随后在 backfill commit 后继续写正确的新状态；
- 任何 interleaving 都不能把 suspended/deleting/deleted 覆盖回 pending_activation/active。

### D. Verify

```text
COUNT(unresolved legacy provisioning)=0
COUNT(record_status IS NULL)=0
active service_status zero NULL
inactive service_status all NULL
enum/timestamp/legacy cross-check all pass
```

### E. Online constraints

先：

```sql
ALTER TABLE workbench_stores
  ADD CONSTRAINT workbench_stores_record_status_nn
  CHECK (record_status IS NOT NULL) NOT VALID;

ALTER TABLE workbench_stores
  VALIDATE CONSTRAINT workbench_stores_record_status_nn;
```

再在 validated proof 存在时 `ALTER COLUMN record_status SET NOT NULL`；使用 bounded lock/statement timeout，无法快速取得 ALTER lock 则 abort/retry later。随后 validate conditional service/timestamp checks；可选删除冗余 non-null check。

### F. Enable

最后开启新 reads + lifecycle routes。

## 16. Rollback / Acceptance

Hard cut 后仅 feature rollback，不回 pre-dual-write binary。继续运行 Reservation Reconciler、Owner Recovery、ambiguous Operation recovery、Audit/Outbox、已开始 correction/migration recovery。

关键测试强制 PostgreSQL testcontainers/harness，至少覆盖：

```text
org A/B same tenant-local owner_attempt_id -> independent Reservations
same owner attempt fresh Reserve Operation ID -> one Reservation
Owner Start vs not-started Recovery -> exactly one wins
owner crash processing/outcome_unknown -> owner-domain recovery terminalizes
tenant listingkit_admin Store Service Correction -> denied
trusted correction without immutable source-bound proof -> denied
trusted proof for source A used against source B -> denied
trusted proof reuse / source double correction -> denied
trusted approved correction -> Store+Resource+Audit succeeds once
same external staging retry -> one identity / one ledger claim
MIGRATION_SOURCE_CHANGED stays internal; no Workbench/BFF route
legacy provisioning -> ResumeCreate/reconcile before backfill
unresolved provisioning -> hard cut blocked
Backfill vs Disable/Delete/Create/ResumeCreate -> no state overwrite
legacy active unknown history -> pending_activation, no invented expiry
validated NOT NULL + ALTER lock timeout
lost COMMIT / deadlock / statement timeout
stale If-Match
correction aggregate version monotonic
```

## 完成定义

Reservation 与 exact tenant-scoped Owner Attempt 原子绑定；owner uncertainty 有 durable recovery；tenant user 不能自行返还已消费服务价值；所有 Store Service refund 必须有 trusted source-bound approval/proof；MigrationCredit 不重复 external source；legacy provisioning/active rows 不被猜测；Backfill 与 live compatibility writer 可串行；Resource/Store mutation 保持 PostgreSQL 原子一致性。