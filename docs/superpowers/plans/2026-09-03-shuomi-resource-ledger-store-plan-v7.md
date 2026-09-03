# 硕米 Resource Ledger / Store Service Implementation Plan V7

**目标：** 把企业资源余额、Reservation/Settlement、Store Activate/Renew/Reactivate、迁移与纠错收敛为 PostgreSQL 权威实现。本 V7 是 PR #284 唯一执行入口；账号/ZITADEL/Onboarding 不进入本计划。

## 权威输入

执行前读取原始 design 与 review amendments 1–12。冲突时以本 V7 与 amendment-12 为准。旧 plan / v2 / v3 / v4 / v5 / v6 仅保留历史。

## 1. PostgreSQL Unit of Work

Operation、Bucket、Event、Reservation、Debt、Store、Audit/Outbox、Correction/Migration/Source Claim 使用 transaction-bound repositories。统一 context deadline、`lock_timeout`、`statement_timeout`、transaction timeout 与 total retry budget；`40001/40P01/55P03/57014` 按 bounded classifier 处理。COMMIT response loss 按 `(organization_id, operation_id)` read-back。

## 2. Resource Core

Bucket `(organization_id, resource_type)` 唯一且 available/reserved/consumed 非负。Operation `(organization_id, operation_id)` 是幂等权威并保存 immutable response snapshot。业务 quantity `q>0` 且有 domain max；HTTP/JSON BIGINT 全部 decimal string。

Event/Reservation/Operation 使用 organization/resource scoped composite binding。

## 3. Operation Replay

身份/权限通过后，volatile validation 前先 fingerprint-check durable Operation；terminal 同 key同 fingerprint replay，changed fingerprint conflict。Ambiguous commit 仅在 authoritative Operation absence 时重新执行。

## 4. Durable Reservation Owner Contract

只有具备 durable identity、tenant/business scope、owner attempt state、recovery/finality contract、owner-specific reconciler/已有 PAY-042 或 Temporal workflow 的 owner type 才能 Reserve。processing/outcome_unknown 必须由 Owner Domain 自己收敛为 terminal proof；Reservation Reconciler 不猜结果。

## 5. Reservation Creation / Owner Binding

Reserve transaction 锁 exact Owner Attempt，要求 not_started、同 organization/business scope、无 incompatible Reservation，然后 create/adopt Reservation 并把 reservation_id/resource/purpose 绑定回 Owner。

数据库至少：

```text
UNIQUE(organization_id, owner_type, owner_attempt_id, resource_type, reservation_purpose)
```

Owner Start 与 expired-not-started recovery 共享 Owner+Reservation fence；只有一个能从 not_started/reserved 前进。`reconciliation_required` 由 durable scanner + lease/epoch 收敛，只有 terminal Owner proof 才 Commit/Release。

## 6. Resource Debt / Credit Correction

所有正向 available 回流先偿 `(organization_id,resource_type)` debt；Event 记录 gross/debt_repaid/net。

AdjustDebit 以 locked `source_credit_event_id` 为权威，quantity 从 immutable source gross credit 派生；same org/resource/type，成功 source claim 保证同 source 最多一次；已消费部分形成同 tenant/resource debt。

## 7. Authorization / Value Boundaries

Store lifecycle：VerifiedIdentity + live org + `PermissionWorkbenchStoreLifecycle`。

Tenant human `PermissionWorkbenchResourceAdmin` 第一阶段只允许 source-bound AdjustDebit；viewer/operator deny，listingkit_admin allow。

Tenant human不能 AdjustCredit/Grant/MigrationCredit/generic Compensate，也不能执行 Store Service Correction 返还价值。

Positive mint 只允许 trusted Billing/Platform-Finance/Provisioning principal + immutable approved source + source-level claim + audit。

Generic Compensation 只允许 trusted Billing/Reconciliation/Service principal + immutable proof exact 绑定 source consume Event；quantity 从 source Event 派生，proof/source 双 one-time claim。

## 8. Store Service Correction：Trusted Proof + Revocation

Store-coupled consume 只能走 paired correction，且执行者必须是 trusted Billing/Reconciliation/Support-Finance principal。

Correction proof 至少：

```text
proof_id
organization_id
store_id
source_lifecycle_operation_id
decision=correct_store_service
status=active|revoked|consumed|expired
version
issued_at / expires_at / revoked_at
issuer / reason
```

首次/非终态执行必须锁 authoritative current proof row并验证：active、未过期、expected proof version、org/store/source/decision exact match。

Correction 仍只允许当前最近一次安全可逆 lifecycle source：锁 source immutable before+after snapshot，要求 current Store service fields/version == source AFTER、full source consumed quantity、无 successful source claim。恢复 BEFORE 的业务字段，但 correction 自身 `aggregate_version=current+1`。

成功时同一个 Store+Resource UoW：proof -> consumed、proof/source successful claims、Store、Resource、Operation snapshot、Event、Audit 一起提交。

审批在成功前被 revoked/version++ 后，后续首次执行或未完成 retry fail closed。已经 succeeded 的 Operation 只 replay immutable result，不因后来 proof revocation 改写历史结果。

自动 delivery-failure proof 还必须有未交付 benefit 的 immutable evidence；商业例外由平台财务/支持审批。Tenant admin 如需纠错只能提交申请，Phase1 不直接退款。

## 9. Connection Status

继续复用 `disconnected/connected/expired/unavailable`。revoked/invalid -> expired；timeout/network/stale/provider error -> unavailable。Activate 只允许 fresh connected；浏览器不可覆盖。

## 10. Store Record / Service State

RecordStatus：

```text
provisioning | active | deleting | deleted
```

`record_status` hard-cut 后全表 NOT NULL。

ServiceStatus 只对 active record 存在：

```text
pending_activation | active | expired | suspended
```

约束：

```text
record=provisioning/deleting/deleted -> service_status/start/expires MUST NULL
record=active -> service_status MUST NOT NULL
service=active/expired -> started_at/expires_at valid && expires_at > started_at
```

异常 -> `STORE_SERVICE_STATE_CORRUPT` / 409 fail closed。

新 Create 第一次 durable insert 必须写：

```text
legacy lifecycle_status=provisioning
record_status=provisioning
service_status=NULL
service_started_at=NULL
service_expires_at=NULL
```

Create/ResumeCreate 完成时原子 `provisioning -> active` + `service_status=pending_activation` 并推进 aggregate version。

Activate/Renew/Reactivate 均要求 record_status=active，因此 provisioning Store 绝不能消耗 renewal periods。

Effective expiry read-time 计算；materializer 仅优化并使用 aggregate version conditional write。

## 11. Activate / Renew / Reactivate

全部要求 active record、lifecycle permission、Idempotency-Key、If-Match aggregate version，fingerprint 包含 org/command/store/quantity/version/behavior fields；锁 Store 后重验 version。

Activate：pending_activation + fresh connected，消费1 renewal period，启动30 days。Renew：effective active，从 current expiry 延长。Reactivate：effective expired，从 now 启动。Resource/Store/Operation snapshot/Audit 同 PostgreSQL transaction。

## 12. Exact Error + BFF

固定 Store lifecycle code/status contract。BFF allowlist activate/renew/reactivate，strict body/header forwarding，更新 Zod/client/UI/contract tests。

MigrationCredit 仍仅 trusted migration runner internal path，不注册 Workbench/BFF；`MIGRATION_SOURCE_CHANGED` / `MIGRATION_OWNERSHIP_CHANGED` 是 internal typed outcome + metrics/reconciliation queue。未来若公开 admin HTTP route，注册前必须补固定 status 与 contract test。

## 13. Migration Credit / Staging

Phase1 MigrationCredit authoritative source 必须位于同一 PostgreSQL 可 `FOR UPDATE` 边界。

外部历史数据先 materialize 到 local immutable staging，保存 stable external identity：source_system/source_type/source_record_id/source_version/org/resource/quantity/payload_fingerprint，且 `(source_system,source_type,source_record_id,resource_type)` UNIQUE。

Importer ambiguous retry same identity+same fp adopt；changed version/org/quantity/payload -> reconciliation。Ledger claim 从 external identity 派生，不从 staging PK 派生。Migration transaction锁 staging source并原子执行 claim+Operation+Bucket+Event+claim applied。

## 14. Legacy Service History Resolver

Backfill 不允许直接使用“有/无历史”的二元猜测。唯一权威接口：

```text
LegacyServiceHistoryResolver.Resolve(store)
  FOUND(history, source, version)
  CONFIRMED_ABSENT(source_evidence)
  UNAVAILABLE(error, retry_after)
```

Rollout manifest 必须声明 resolver 的 authoritative source/owner。若当前部署根本不存在历史系统，只能使用经过 preflight/审批的 `NoAuthoritativeHistorySource` manifest 返回 CONFIRMED_ABSENT，worker 不得自行推断。

映射：

- FOUND -> exact derive active/expired；
- CONFIRMED_ABSENT -> pending_activation，无 charge/credit/ invented expiry；
- UNAVAILABLE/query failure/timeout -> row unchanged，retry；超过 rollout budget -> blocker，禁止 Phase F。

审计统计至少：

```text
history_found_count
history_confirmed_absent_count
history_unavailable_count
history_error_count
```

Phase D/F 要求 unresolved unavailable/error = 0。

## 15. Store Hard-cut Rollout

### A Expand

新增 nullable transitional columns/index，不启用新 reads。RecordStatus enum 从一开始包含 provisioning。

### B Compatibility Writers

全量部署 Create/ResumeCreate/Disable/Delete + legacy `/enable` guard，并 drain pre-compatible Pod。

关键写语义：

- Create first insert -> record=provisioning, service=NULL；
- ResumeCreate/Create completion -> record=active, service=pending_activation；
- Disable -> active/suspended；
- BeginDelete -> deleting/service=NULL；
- SoftDelete -> deleted/service=NULL；
- legacy disabled + service NULL/suspended 的 `/enable` -> `STORE_SERVICE_RESUME_REQUIRED`。

### C Backfill With Row-level Concurrency Authority

逐 row/batch 使用 `SELECT ... FOR UPDATE [SKIP LOCKED]` 或 equivalent expected-version/CAS，锁后重新读取 legacy lifecycle/deleted_at/new state/version。

若 Phase B writer 已写 non-null new state，Backfill 只 validate/no-op；不得覆盖并发 writer。

Authoritative mapping：

```text
deleted_at != NULL -> record=deleted, service=NULL
legacy deleting -> record=deleting, service=NULL
legacy provisioning -> record=provisioning, service=NULL
legacy disabled -> record=active, service=suspended
legacy active + history FOUND -> exact active/expired
legacy active + history CONFIRMED_ABSENT -> active/pending_activation
history UNAVAILABLE -> no mutation, retry/block rollout
```

不从 create/update timestamp 猜 expiry。

Pre-existing provisioning 可继续由现有 ResumeCreate recovery 收敛，但无需为了模型正确性强制全部消失；它们作为合法 `record_status=provisioning` 保留，且不能进入 service lifecycle routes。

### D Verify

要求：record_status 零 NULL；provisioning/deleting/deleted service fields 全 NULL；active service_status 非空；enum/timestamp/legacy cross-check 通过；history resolver unavailable/error blocker=0。

### E Online Constraints

先 `CHECK(record_status IS NOT NULL) NOT VALID` -> VALIDATE -> `SET NOT NULL`，使用 bounded lock/statement timeout。随后 validate：

```text
record=active <=> service_status non-null
record in provisioning/deleting/deleted => service fields null
active/expired timestamp invariants
```

### F Enable

最后开启新 reads + lifecycle routes。

## 16. Rollback / Acceptance

Hard cut 后仅 feature rollback，不回 pre-dual-write binary。继续运行 Reservation Reconciler、Owner Recovery、ambiguous Operation recovery、Audit/Outbox、correction/migration recovery、ResumeCreate recovery。

关键测试强制 PostgreSQL testcontainers/harness，至少覆盖：

```text
owner binding tenant scope
Owner Start-vs-Recovery
history FOUND / CONFIRMED_ABSENT / UNAVAILABLE
history resolver timeout leaves row unchanged
trusted correction proof active/revoked/version race
succeeded correction replay after later revocation
new Create first provisioning insert after record_status NOT NULL
provisioning lifecycle rejects Activate/Renew/Reactivate
ResumeCreate provisioning -> active/pending_activation
Backfill vs Disable/Delete/Create/ResumeCreate concurrency
legacy provisioning preserved without invented service
lost COMMIT/deadlock/timeout/stale If-Match
```

## 完成定义

Reservation 与 tenant-scoped exact Owner Attempt 原子绑定；价值返还只由 trusted proof 驱动且撤销语义明确；Legacy history 查询失败绝不被误判为“无历史”；Store provisioning 是正式 RecordStatus，可跨 hard cut 安全恢复；Resource/Store mutation 保持 PostgreSQL 原子一致性。