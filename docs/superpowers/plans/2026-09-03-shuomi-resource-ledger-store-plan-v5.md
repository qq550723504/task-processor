# 硕米 Resource Ledger / Store Service Implementation Plan V5

**目标：** 把企业资源余额、Reservation/Settlement、Store Activate/Renew/Reactivate、迁移与纠错收敛为 PostgreSQL 权威实现。本 V5 是 PR #284 唯一执行入口；账号/ZITADEL/Onboarding 不进入本计划。

## 权威输入

执行前读取原始 design 与 review amendments 1–10。冲突时以本 V5 与 amendment-10 为准。旧 plan / v2 / v3 / v4 仅保留历史。

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
lock exact Owner Attempt
require state=not_started
verify org/business scope
require no incompatible bound Reservation
create/adopt Reservation
bind reservation_id + resource_type + purpose onto Owner Attempt
write Reserve Operation/Event/Bucket
commit
```

数据库至少：

```text
UNIQUE(owner_type, owner_attempt_id, resource_type, reservation_purpose)
```

同 attempt/purpose + same fingerprint replay；changed quantity/resource/purpose conflict；terminal owner 不允许 late Reserve。

Owner Start 同事务锁 Owner+Reservation，要求 owner=not_started 且 bound Reservation=reserved，然后 owner->processing。Expired not-started recovery 同样锁两者，owner->cancelled_terminal/recovery_fenced，并同事务 Release。两者只能一个成功。

`reconciliation_required` 使用 durable scanner + lease/epoch；只有 Owner terminal proof 后 fenced Commit/Release。

## 6. Resource Debt

所有正向 available 回流先偿 `(organization_id,resource_type)` debt；Event 记录 gross/debt_repaid/net。覆盖 trusted credits、MigrationCredit、Reservation Release、trusted Compensation 等。

## 7. Credit Correction / AdjustDebit

Canonical command 以 `source_credit_event_id` 为权威；锁 source Event 后从 immutable gross credit quantity 派生 correction amount。same org/resource/type，成功 source claim 保证同 source 最多一次；已消费部分形成同 tenant/resource debt。

## 8. Authorization / Value Boundaries

Store lifecycle：VerifiedIdentity + live org + `PermissionWorkbenchStoreLifecycle`。

Tenant human `PermissionWorkbenchResourceAdmin` 只允许 source-bound AdjustDebit 与 paired Store Service Correction；viewer/operator deny，listingkit_admin allow。

Tenant human **不能** AdjustCredit/Grant/MigrationCredit/generic Compensate。

Positive mint 只允许 trusted Billing/Platform-Finance/Provisioning principal + immutable approved source + source-level claim + audit。

Generic Compensation 只允许 trusted Billing/Reconciliation/Service principal，且 proof 必须是 immutable trusted record：

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

Compensation tx 锁 proof+source Event，exact match org/resource/event/decision，quantity 从 source Event 派生，同时写 proof-level one-time claim + source-event-level successful claim。没有可信 proof source时不注册 generic Compensate API。Store-coupled consume 只走 paired correction。

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

固定 code/status contract，包括 `STORE_SERVICE_STATE_CORRUPT=409`、`STORE_CORRECTION_NOT_INVERTIBLE=409` 和既有 version/state mappings。BFF allowlist activate/renew/reactivate，strict body/header forwarding，更新 Zod/client/UI/contract tests。

## 13. Store Service Correction

只允许当前最近一次安全可逆 lifecycle source。锁 source immutable before+after snapshot，要求 current service fields/version == source AFTER、full quantity == source consumed、无 successful source claim。恢复 BEFORE 的业务 service fields，但 correction 自身 `aggregate_version=current+1`，版本绝不倒退。Resource correction、claim、result snapshot、Audit 同事务。

## 14. Migration Credit / Staging Identity

Phase1 MigrationCredit 的权威 source 必须位于同一 PostgreSQL 可 `FOR UPDATE` 边界。

外部历史数据先 materialize 到 local immutable staging。Staging 必须保留 external identity：

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

Importer ambiguous retry：same external identity+same fingerprint adopt；changed version/org/quantity/payload -> reconciliation，不创建第二 row。

Ledger global claim 从 external identity 派生，不从 staging PK 派生。Migration transaction锁 staging source并原子执行 claim+Operation+Bucket+Event+claim applied。

Remote API/external DB/ETag-only source 不直接入账；未来直接跨系统迁移另设计 durable source freeze/claim。

## 15. Store Hard-cut Rollout

### A Expand
新增 nullable transitional columns/index，不启用新 reads。

### B Compatibility Writer Fence
全量部署 Create/ResumeCreate/Disable/Delete + legacy `/enable` guard；旧 Pod drain/minimum writer fence 后才 backfill。Legacy disabled + service_status NULL/suspended 调 `/enable` 一律 `STORE_SERVICE_RESUME_REQUIRED`，不转 active。

### C Backfill authoritative mapping

```text
deleted_at != NULL -> record=deleted, service=NULL
legacy deleting -> record=deleting, service=NULL
legacy disabled -> record=active, service=suspended
legacy active + verified authoritative service history/expiry -> exact active/expired
legacy active + NO authoritative service history/expiry -> record=active, service=pending_activation, started/expires=NULL
```

禁止用 create/update timestamp 猜 expiry；unknown-history migration 不 charge、不 credit、不发明服务期。报告 unknown-history->pending_activation 数量。

### D Verify
`record_status` 零 NULL；active service_status 零 NULL；inactive service_status 全 NULL；enum/timestamp/legacy cross-check 全通过。

### E Online constraints
先：

```sql
ADD CONSTRAINT ... CHECK (record_status IS NOT NULL) NOT VALID;
VALIDATE CONSTRAINT ...;
```

再在 validated proof 存在时 `ALTER COLUMN record_status SET NOT NULL`；使用 bounded lock/statement timeout，无法快速取得 ALTER lock 则 abort/retry later。随后 validate conditional service/timestamp checks；可选删除冗余 non-null check。

### F Enable
最后开启新 reads + lifecycle routes。

## 16. Rollback / Acceptance

Hard cut 后仅 feature rollback，不回 pre-dual-write binary。继续运行 Reservation Reconciler、Owner Recovery、ambiguous Operation recovery、Audit/Outbox、已开始 correction/migration recovery。

关键测试强制 PostgreSQL testcontainers/harness，至少覆盖：Owner Attempt duplicate Reserve/late Reserve/Start-vs-Recovery、owner crash recovery、same external staging retry、external source identity changed conflict、legacy active unknown history->pending_activation、proof/event mismatch、proof reuse/source double compensation、online validated non-null + ALTER lock timeout、lost COMMIT、deadlock/timeout、stale If-Match、correction version monotonic。

## 完成定义

Reservation 与 exact owner attempt 原子绑定；owner uncertainty 有 durable recovery；tenant admin 不能无 trusted proof 返还 consumed；MigrationCredit 不重复 external source；legacy Store 不发明 expiry；online hard-cut 不通过长表扫描阻塞 writer；Resource/Store mutation 保持 PostgreSQL 原子一致性。