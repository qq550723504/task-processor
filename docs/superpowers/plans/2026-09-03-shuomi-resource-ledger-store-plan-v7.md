# 硕米 Resource Ledger / Store Service Implementation Plan V7

**状态：IMPLEMENTATION_READY / 冻结实施基线。** 本 V7 是 PR #284 唯一执行入口；账号/ZITADEL/Onboarding 不进入本计划。旧 design、review amendments 与旧 plan 只保留历史背景；与本 V7 冲突时一律以本 V7 为准，不再创建 V8/V9 处理非 Blocker finding。

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

Tenant human不能 AdjustCredit/Grant/MigrationCredit/Generic Compensation，也不能执行 Store Service Correction 返还价值。

Positive mint 只允许 trusted Billing/Platform-Finance/Provisioning principal + immutable approved source + source-level claim + audit。

### 7.1 Phase1 Welcome Store Renewal Grant

PR #281 的 Product Decision 已锁定：新**直接注册**并完成首次业务开通的 Organization，一次性获得：

```text
resource_type = store_renewal_period
quantity = 1
```

该能力使用现有 Positive Mint 边界的一个**窄、命名、内部 Provisioning 用例**，不开放通用 tenant `Grant`。

推荐内部语义：

```text
GrantWelcomeStoreRenewalPeriod(
  organization_id,
  source_type = onboarding_welcome_store_period,
  source_identity = organization_id,
  quantity = 1
)
```

硬规则：

- caller 必须是 trusted Provisioning principal；
- `resource_type` 固定 `store_renewal_period`；
- `quantity` 固定 `1`，调用者不能传任意正数；
- immutable approved source 必须证明该 Organization 属于 PR #283 的 new-direct-registration first-business-opening；
- source identity 以 **Organization** 为稳定身份，不以 registration/reclaim attempt 为身份；
- source-level successful claim 至少唯一约束 `(source_type, source_identity, resource_type)`，保证同 Organization 该 welcome Grant 最多成功一次；
- same source + same immutable payload retry/read-back -> replay 原成功结果；
- same source changed org/resource/quantity -> conflict/fail closed；
- Operation + source claim + Bucket + Event + Audit 必须在同一 PostgreSQL transaction；
- lost COMMIT 按 Operation/source claim read-back，不重复 mint；
- 不注册 Workbench/BFF/tenant HTTP route；
- 历史 Organization 不由该命令自动补发，除非未来另有 migration source/product decision。

Event/Audit 必须能区分：

```text
reason/source = onboarding_welcome_store_period
quantity = +1
resource_type = store_renewal_period
```

这样 Phase1 首次闭环是：欢迎 Grant 先形成企业余额；用户后续显式 Activate 才真正消费该 1 期，绑定 Store 本身仍不扣资源。

### Generic Compensation Phase1 明确 Deferred

Phase1 **不实现、不注册 Generic Compensation command/API/worker**。旧 amendments 中关于 Generic `CompensateConsume` proof、revocation、approval lifecycle 的设计全部视为历史/Backlog，不属于当前 Must。

原因：当前 Phase1 需要交付 Resource Ledger、Reservation/Settlement、Store Activate/Renew/Reactivate、MigrationCredit、欢迎资源 Grant 与受控 Store Service Correction；不存在已批准的通用消费补偿业务入口或权威 proof producer。为了一个未上线能力继续扩展 approval/revocation framework 不符合当前范围。

未来若重新启用 Generic Compensation，必须单独确认 Product Decision 与 proof authority，并至少定义 source consume Event exact binding、proof current/revoked/expired semantics、proof/source one-time claim、trusted principal 与审计；在那之前任何 HTTP/BFF route 都不得注册。

## 8. Store Service Correction：Trusted Proof + Revocation + Immutable Fingerprint

Store-coupled consume 只能走 paired correction，执行者必须是 trusted Billing/Reconciliation/Support-Finance principal。

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

**Correction Operation 的 immutable fingerprint 必须包含 `proof_id + expected_proof_version`**，同时包含 organization/store/source lifecycle operation/command/quantity-derived facts/If-Match 等行为字段。same Operation key 如果把 proof A 替换为 proof B，即使版本号相同，也必须返回 replay conflict，不能把一次审批换成另一审批继续执行。

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

`suspended` 可以保留已存在的 authoritative `service_started_at/service_expires_at`，用于未来 resume 判断该 Store 曾经拥有的付费服务期；从未激活且 confirmed no history 的 suspended Store 可以保持 timestamps NULL。

异常 -> `STORE_SERVICE_STATE_CORRUPT` / 409 fail closed。

新 Create 第一次 durable insert：

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

Welcome Grant 不改变这个规则：**Grant 是资源入账，Activate 是资源消费，两者绝不合并成“绑定即自动激活”。**

## 12. Exact Error + BFF

固定 Store lifecycle code/status contract。BFF allowlist activate/renew/reactivate，strict body/header forwarding，更新 Zod/client/UI/contract tests。

MigrationCredit 仍仅 trusted migration runner internal path，不注册 Workbench/BFF；`MIGRATION_SOURCE_CHANGED` / `MIGRATION_OWNERSHIP_CHANGED` 是 internal typed outcome + metrics/reconciliation queue。未来若公开 admin HTTP route，注册前必须补固定 status 与 contract test。

Welcome Store Renewal Grant 同样仅 trusted Provisioning internal path，不注册 Workbench/BFF/tenant route；浏览器不能通过伪造 source 自助 mint。

Generic Compensation Phase1 同样没有 HTTP/BFF route。

## 13. Migration Credit / Staging

Phase1 MigrationCredit authoritative source 必须位于同一 PostgreSQL 可 `FOR UPDATE` 边界。

外部历史数据先 materialize 到 local immutable staging，保存 stable external identity：source_system/source_type/source_record_id/source_version/org/resource/quantity/payload_fingerprint，且 `(source_system,source_type,source_record_id,resource_type)` UNIQUE。

Importer ambiguous retry same identity+same fp adopt；changed version/org/quantity/payload -> reconciliation。Ledger claim 从 external identity 派生，不从 staging PK 派生。Migration transaction锁 staging source并原子执行 claim+Operation+Bucket+Event+claim applied。

## 14. Legacy Service History Resolver / Snapshot Fence

Backfill 不允许直接使用“有/无历史”的二元猜测。唯一权威接口：

```text
LegacyServiceHistoryResolver.Resolve(store)
  FOUND(history, source_identity, source_version_or_snapshot_token)
  CONFIRMED_ABSENT(source_identity, source_version_or_snapshot_token)
  UNAVAILABLE(error, retry_after)
```

Rollout manifest 必须声明 resolver 的 authoritative source/owner。若当前部署根本不存在历史系统，只能使用经过 preflight/审批的 `NoAuthoritativeHistorySource` manifest 返回 CONFIRMED_ABSENT，worker 不得自行推断。

### Snapshot/freeze 不变量

Resolver 返回的 `source_version_or_snapshot_token` 不是审计备注，而是 backfill correctness precondition。

- authoritative source 与 Store 位于同一 PostgreSQL / 可锁事务边界时，Backfill 必须锁 source row/version，并与 Store local CAS 在同一可证明不变的窗口完成；
- source 是独立但可提供 durable freeze/claim 或 immutable snapshot 时，Backfill 必须持有该 freeze/snapshot token 到 local CAS 完成；
- source 仍可写且无法提供 durable freeze/claim/immutable snapshot 时，**禁止 hard cut**，不能用一次普通读取代替并发 fence；
- 每次 local Store mutation 前必须重新验证 resolver token/version 仍 current；验证失败则 row 不变并重新 Resolve；
- Phase F 开启前必须再次验证本批次/manifest 的 source snapshot/freeze 仍有效，发现漂移则阻断 enable。

映射：

- FOUND -> exact derive authoritative history；
- CONFIRMED_ABSENT -> 没有历史事实，不 charge、不 credit、不 invented expiry；
- UNAVAILABLE/query failure/timeout -> row unchanged，retry；超过 rollout budget -> blocker，禁止 Phase F。

审计统计至少：

```text
history_found_count
history_confirmed_absent_count
history_unavailable_count
history_error_count
history_snapshot_conflict_count
```

Phase D/F 要求 unresolved unavailable/error/snapshot conflict = 0。

## 15. Store Hard-cut Rollout

### A Expand

新增 nullable transitional columns/index，不启用新 reads。RecordStatus enum 从一开始包含 provisioning。

### B Compatibility Writers

全量部署 Create/ResumeCreate/Disable/Delete + legacy `/enable` guard，并 drain pre-compatible Pod。

关键写语义：

- Create first insert -> record=provisioning, service=NULL；
- ResumeCreate/Create completion -> record=active, service=pending_activation；
- Disable -> active/suspended，并保留当前 authoritative service timestamps；
- BeginDelete -> deleting/service=NULL；
- SoftDelete -> deleted/service=NULL；
- legacy disabled + service NULL/suspended 的 `/enable` -> `STORE_SERVICE_RESUME_REQUIRED`。

### C Backfill With Row-level + History-source Concurrency Authority

逐 row/batch 使用 `SELECT ... FOR UPDATE [SKIP LOCKED]` 或 equivalent expected-version/CAS，锁后重新读取 legacy lifecycle/deleted_at/new state/version。

若 Phase B writer 已写 non-null new state，Backfill 只 validate/no-op；不得覆盖并发 writer。

对需要 history resolution 的 row，必须同时执行 §14 的 source snapshot/freeze validation。Store row lock 只能保护本地 Store writer，不能代替 authoritative history source fence。

Authoritative mapping：

```text
deleted_at != NULL -> record=deleted, service=NULL
legacy deleting -> record=deleting, service=NULL
legacy provisioning -> record=provisioning, service=NULL

legacy active + history FOUND -> record=active, exact active/expired
legacy active + history CONFIRMED_ABSENT -> record=active, service=pending_activation, timestamps=NULL
legacy active + history UNAVAILABLE/snapshot conflict -> no mutation, retry/block rollout

legacy disabled + history FOUND -> record=active, service=suspended,
                                  persist authoritative started/expires history
legacy disabled + history CONFIRMED_ABSENT -> record=active, service=suspended,
                                             started/expires=NULL
legacy disabled + history UNAVAILABLE/snapshot conflict -> no mutation, retry/block rollout
```

**disabled Store 不得绕过 LegacyServiceHistoryResolver。** 否则一个曾购买并正在 suspended 的 Store 会与“从未激活的 suspended Store”不可区分，未来 service resume 无法正确判断剩余/历史服务期。

不从 create/update timestamp 猜 expiry。

Pre-existing provisioning 可继续由现有 ResumeCreate recovery 收敛，也可作为合法 `record_status=provisioning` 保留；它们不能进入 service lifecycle routes。

### D Verify

要求：record_status 零 NULL；provisioning/deleting/deleted service fields 全 NULL；active service_status 非空；enum/timestamp/legacy cross-check 通过；所有 active/disabled legacy row 的 history resolution 有 definitive FOUND/CONFIRMED_ABSENT；resolver unavailable/error/snapshot conflict blocker=0。

对于 suspended + authoritative historical timestamps，验证 history token/freeze 与持久化值一致；suspended + CONFIRMED_ABSENT 才允许 timestamps NULL。

### E Online Constraints

先 `CHECK(record_status IS NOT NULL) NOT VALID` -> VALIDATE -> `SET NOT NULL`，使用 bounded lock/statement timeout。随后 validate：

```text
record=active <=> service_status non-null
record in provisioning/deleting/deleted => service fields null
active/expired timestamp invariants
suspended timestamps 若存在则必须 ordered/valid
```

### F Enable

最后开启新 reads + lifecycle routes。Enable 前重新验证 authoritative history source snapshot/freeze；任何 source drift 都阻断 hard cut。

## 16. Rollback / Acceptance

Hard cut 后仅 feature rollback，不回 pre-dual-write binary。继续运行 Reservation Reconciler、Owner Recovery、ambiguous Operation recovery、Audit/Outbox、correction/migration recovery、ResumeCreate recovery。

关键测试强制 PostgreSQL testcontainers/harness，至少覆盖：

```text
owner binding tenant scope
Owner Start-vs-Recovery
welcome grant new direct org -> available store_renewal_period +1
welcome grant same org/source concurrent/retry/lost-COMMIT -> exactly one credit event / +1 only
welcome grant changed quantity/resource/source payload -> conflict
welcome grant tenant/browser route absent; untrusted principal denied
welcome grant -> bind store no consume -> Activate consumes exactly 1 and starts 30d
history FOUND / CONFIRMED_ABSENT / UNAVAILABLE
history source version changes after Resolve -> local CAS rejected/re-resolve
history source changes before Phase F -> hard cut blocked
history resolver timeout leaves row unchanged
legacy disabled + FOUND history -> suspended + exact timestamps preserved
legacy disabled + CONFIRMED_ABSENT -> suspended + null timestamps
legacy disabled + UNAVAILABLE -> row unchanged / rollout blocked
trusted correction proof active/revoked/version race
correction same Operation key substitutes different proof_id -> replay conflict
succeeded correction replay after later revocation
Generic Compensation route/command absent in Phase1
new Create first provisioning insert after record_status NOT NULL
provisioning lifecycle rejects Activate/Renew/Reactivate
ResumeCreate provisioning -> active/pending_activation
Backfill vs Disable/Delete/Create/ResumeCreate concurrency
legacy provisioning preserved without invented service
lost COMMIT/deadlock/timeout/stale If-Match
```

## 17. Review Stop Rule For This Plan

本 V7 已达到 `IMPLEMENTATION_READY`。以后只有会造成跨租户/越权、资源或计费错误、welcome Grant 重复 mint/错误 mint、数据损坏/权益丢失、重复价值返还、不可恢复事务副作用、hard-cut migration 不安全、或核心 lifecycle 无法完成的 finding 才重新打开架构。

Generic Compensation 的更多 proof/revocation 设计属于 Backlog，因为 Phase1 不实现该能力；不得以此为理由创建 V8 或阻止当前开发。

## 完成定义

Reservation 与 tenant-scoped exact Owner Attempt 原子绑定；Phase1 不提供 Generic Compensation；新直接注册 Organization 的欢迎 `store_renewal_period=1` 只能由 trusted Provisioning + immutable onboarding source 按 Organization exactly-once 入账；价值返还只由受控 Store Correction trusted proof 驱动且 proof identity/revocation/idempotency 明确；Legacy history 查询失败或 source snapshot 漂移绝不被误判为“无历史”；disabled Store 的 authoritative paid-service history 不丢失；Store provisioning 是正式 RecordStatus；Resource/Store mutation 保持 PostgreSQL 原子一致性。