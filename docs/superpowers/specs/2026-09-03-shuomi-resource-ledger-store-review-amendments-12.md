# 硕米 Resource Ledger / Store Service 第十二轮评审修订

本文件针对 PR #284 V6 最新 Code Review 的 2 个 P1 + 1 个 P2 继续收敛。与前序文档冲突时，以本文件和 `2026-09-03-shuomi-resource-ledger-store-plan-v7.md` 为准。

## 1. Legacy Service History 必须是三态权威解析，不得把查询失败当“无历史”

Backfill 新增明确接口：

```text
LegacyServiceHistoryResolver.Resolve(store)
  -> FOUND(authoritative started_at/expires_at/source/version)
  -> CONFIRMED_ABSENT(source_evidence)
  -> UNAVAILABLE(error/retry_after)
```

规则：

- 只有 `FOUND` 才能 exact derive active/expired；
- 只有 `CONFIRMED_ABSENT` 才能迁移为 `pending_activation`；
- timeout/network/query error/source unavailable/schema mismatch 一律 `UNAVAILABLE`，row 保持原状，Backfill retry，超过 rollout budget 则成为 blocker；
- 不允许把 `nil/error`、空响应、超时或读失败解释为 `CONFIRMED_ABSENT`。

Resolver 的 authoritative source/owner 必须在 rollout manifest 中显式声明。若部署环境根本没有任何历史系统，则必须使用一个经过 preflight 确认的 `NoAuthoritativeHistorySource` manifest 作为“该环境确实不存在历史来源”的权威证据，不能由 worker 临时猜测。

恢复 V5 的审计统计并扩展：

```text
history_found_count
history_confirmed_absent_count
history_unavailable_count
history_error_count
```

Phase D/F 只有 `history_unavailable_count=0` 且没有 unresolved resolver error 才能继续 hard cut。

## 2. Store Service Correction Proof 在成功消费前必须可撤销并实时校验

Trusted correction proof 不是签发即永久有效。Phase1 固定为“成功消费前可撤销，成功消费后不可重复使用”。

Proof 至少：

```text
proof_id
organization_id
store_id
source_lifecycle_operation_id
decision=correct_store_service
status = active | revoked | consumed | expired
version
issued_at
expires_at
revoked_at
issuer
reason/type
```

Correction transaction 必须锁 authoritative current proof row，并要求：

- status=`active`；
- `now < expires_at`；
- org/store/source/decision exact match；
- proof version 与命令 fingerprint 中的 expected proof version 一致；
- source lifecycle Operation 仍满足最近一次安全可逆等全部条件；
- source/proof successful claim 都不存在。

成功 correction 的同一事务内：

```text
proof active -> consumed
insert proof-level successful claim
insert source-level successful claim
Store + Resource + Operation + Event + Audit commit
```

如果审批在首次成功前被撤销：`status=revoked, version++`；后续首次执行/非终态 retry 必须 fail closed。已经 succeeded 的 Operation response-loss replay 仍从 immutable Operation result 重放，不重新解释后来发生的 proof revocation。

## 3. `provisioning` 必须成为正式 RecordStatus，而不是迁移异常

RecordStatus 扩展为：

```text
provisioning | active | deleting | deleted
```

ServiceStatus 只对 `record_status=active` 有意义：

```text
record=provisioning/deleting/deleted -> service_status MUST NULL
record=active -> service_status MUST NOT NULL
```

新 Create 第一次 durable insert 必须原子写：

```text
legacy lifecycle_status = provisioning
record_status = provisioning
service_status = NULL
service_started_at = NULL
service_expires_at = NULL
```

因此 Phase E `record_status NOT NULL` 后，新 Create 仍可合法插入，不依赖默认值，也不会把未完成 Store 伪装成 active。

Create/ResumeCreate 完成连接/原有创建事务后，原子转换：

```text
record_status: provisioning -> active
service_status: NULL -> pending_activation
```

并推进 aggregate version。失败/删除继续走既有 deleting/deleted 状态。

Activate/Renew/Reactivate 始终要求 `record_status=active`，因此 provisioning Store 不能消耗 renewal periods。

## 4. 旧 Provisioning Rows 不再要求“先全部消失”才能建模

前序 V6 的 Provisioning Drain Gate 仍可作为上线前清理优化，但 V7 不再依赖“zero provisioning rows”作为数据模型正确性的前提。

Backfill 对 legacy provisioning 精确保留：

```text
legacy provisioning -> record=provisioning, service=NULL
```

已有 `ResumeCreate` 继续拥有其恢复责任。Phase D 允许合法 provisioning rows，但要求：

- record_status=provisioning；
- service fields 全 NULL；
- legacy lifecycle 仍与 provisioning 一致；
- 不进入 Store service lifecycle routes。

可以在 rollout 前尽量 drain，无法自动收敛的 provisioning row 不再被错误映射为 active/pending，但应持续纳入 operational repair metrics。

## 5. 第十二轮验收

至少新增：

```text
history FOUND -> exact active/expired
history CONFIRMED_ABSENT -> pending_activation
history timeout/error -> row unchanged + retry/blocker, never pending_activation
rollout manifest has no source -> only explicit NoAuthoritativeHistorySource may yield confirmed absent
history found/absent/unavailable counts auditable
correction proof active current version -> eligible
proof revoked before correction -> reject/no refund
proof version changed during retry -> reject first execution
succeeded correction then proof revoked -> same Operation replay original result only
new Create first insert after NOT NULL cutover -> record=provisioning/service=NULL succeeds
provisioning Store -> Activate/Renew/Reactivate reject
ResumeCreate completion -> provisioning -> active/pending_activation atomically
legacy provisioning backfill -> preserved provisioning, no invented service entitlement
```

## 6. 领域边界不变

本轮只处理 Resource Ledger / Store Service；不引入账号、ZITADEL、Onboarding 逻辑。