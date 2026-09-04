# 硕米 Resource Ledger / Store Service 第七轮评审修订

本文件针对 PR #284 在 `6c334a8` 上的新一轮 Code Review / Security Review 结论进行收敛。与前序文档冲突时，以本文件与 `2026-09-03-shuomi-resource-ledger-store-plan-v2.md` 为准。

## 1. AdjustDebit 必须绑定被纠正的原始 Credit

所有“纠正错误 Credit”的 AdjustDebit 不再只凭新的 Operation ID 扣余额。

必须携带并锁定：

```text
source_credit_event_id
organization_id
resource_type
original_credit_quantity
```

权威规则：

- source 必须是允许纠正的 credit 类 Event（Grant / MigrationCredit / AdjustCredit 等明确 allow-list）；
- source Event 与 correction 必须同 organization + resource；
- 第一阶段只允许一次成功 full-source credit correction；
- 成功 correction 使用独立 source claim，唯一键至少覆盖 `(organization_id, source_credit_event_id)`；
- failed_terminal 不占 successful source claim；
- 若 original credit 已被部分消费：先回收当前可用部分，剩余写同 tenant/resource 的 `resource_debt`；
- replay 同 Operation 复用 immutable result；不同 Operation 再纠正同 source 返回 already-corrected。

不能通过换 Idempotency-Key / Operation ID 重复制造 debt 或重复回收余额。

## 2. Activate / Renew / Reactivate 全部要求 Aggregate If-Match

三条 Store Service mutation 都必须把当前 Store aggregate version 作为行为性前置条件：

```text
If-Match: <aggregate version>
```

Canonical fingerprint 必须包含该 expected version。

事务内锁 Store 后再次比较：

```text
actual version != expected -> STORE_VERSION_CONFLICT
```

不能只在 Activate 要求 If-Match，而让 Renew / Reactivate 对 stale request 自动落到新状态上并再次消费 renewal periods。

## 3. Rollback 时必须保留 Correctness-critical Reconciler

Hard-cut 后 rollback 仍然是 feature rollback，不允许回到 pre-dual-write binary。

Rollback 允许停止：

```text
new user-facing activate/renew/reactivate acceptance
nonessential materializer
nonessential migration producer
```

Rollback 不允许停止：

```text
Reservation Reconciler
commit_outcome_unknown / durable Operation recovery
Resource Debt repayment invariants
mandatory Audit/Outbox delivery if correctness/audit requires
cleanup of already-started durable migration/correction work
```

只有 durable recovery backlog 已全部 settled、明确 handoff 到兼容 worker，或进入可运维 fail-closed 状态后，才允许停止对应 recovery worker。

## 4. PostgreSQL `57014 query_canceled` 必须区分 caller cancellation 与 statement timeout

UoW 已设置 `statement_timeout`，因此需要单独分类 SQLSTATE `57014`。

规则：

```text
if caller context already cancelled/deadline exceeded:
    return caller cancellation; do not retry
else if SQLSTATE == 57014 from server statement_timeout:
    classify as bounded transient concurrency/timeout failure
    retry only inside remaining total retry budget
    budget exhausted -> RESOURCE_CONCURRENCY_RETRY / 503
```

继续保留：

```text
40001 serialization_failure
40P01 deadlock_detected
55P03 lock_not_available
```

测试必须区分 server-side statement timeout 与 client context cancellation。

## 5. Store Service Correction 只允许安全可逆的最新生命周期操作

第一阶段不设计任意历史 lineage 回滚。

### 5.1 Source snapshot 必须同时保存 before + after

Lifecycle succeeded Operation 的 immutable snapshot 至少增加：

```text
source_type = activate | renew | reactivate
store_id
before_service_status
before_service_started_at
before_service_expires_at
before_aggregate_version
after_service_status
after_service_started_at
after_service_expires_at
after_aggregate_version
consumed_quantity
resource_type
```

### 5.2 Correction 可执行条件

Correction 前锁 Store + source Operation，并要求：

```text
source 是该 Store 最近一次成功的 service lifecycle mutation
当前 Store service state == source after snapshot
当前 aggregate version == source after version
source 尚无 successful correction claim
requested refund == full source consumed quantity
```

任一不满足：

```text
STORE_CORRECTION_NOT_INVERTIBLE
```

不允许修改历史 source 后再跨越后续 Renew/Reactivate。

### 5.3 Exact inverse

满足可逆条件时，同一 StoreResourceUnitOfWork 内：

```text
Store service state := source before snapshot
Resource consumed correction := full source consumed quantity
successful correction claim insert
Correction immutable result snapshot
Store audit/outbox
```

因此：

- Correct Activate -> 恢复到 Activate 前的 pending_activation 状态及原时间字段；
- Correct Renew -> 恢复 Renew 前的 expiry/status/timestamps；
- Correct Reactivate -> 恢复 Reactivate 前的 expired/service snapshot；
- 若 source 之后已有其他成功 lifecycle mutation，则不自动推导“减 30 天”，直接拒绝自动 correction。

## 6. Store Status Schema 必须 Expand -> Dual-write -> Backfill -> Validate

禁止在已有 `workbench_stores` 数据上第一步就加入强制 NOT NULL 新状态列。

固定 rollout：

### Phase A - Expand

新增 nullable / transitional columns 与必要 index，不启用新 lifecycle reads/writes。

### Phase B - Compatibility Dual-write

全量部署 Create / ResumeCreate / Disable / Delete 等 legacy writer 的新状态同步，旧 Pod drain / minimum writer fence 生效。

### Phase C - Backfill

按固定优先级填充 `record_status / service_status / started_at / expires_at`，可重跑，记录 mismatch。

### Phase D - Verify

必须证明：

```text
no required status NULL
no invalid enum
no active/expired timestamp invariant violation
legacy/new state cross-check 通过
```

### Phase E - Validate Constraints

PostgreSQL 使用可安全上线的 staged constraint：先 CHECK/NOT VALID（适用处）-> VALIDATE -> 最终 SET NOT NULL/正式 enum constraint。

### Phase F - Enable

最后才开启新 read contract 与 lifecycle mutation feature。

## 7. Active / Expired Store 时间不变量 Fail Closed

数据库与 rehydration 都必须保证：

```text
service_status in (active, expired)
=> service_started_at IS NOT NULL
AND service_expires_at IS NOT NULL
AND service_expires_at > service_started_at
```

`pending_activation` 不得伪造 expiry。

若运行时读到：

```text
active + NULL expiry
expired + NULL expiry
expires_at <= started_at
```

则：

- effective service status 不得继续当 active；
- 所有 consume / renew / reactivate / business service gate fail closed；
- 返回稳定 `STORE_SERVICE_STATE_CORRUPT` 并进入 repair/audit；
- 不能通过 materializer 自动猜一个 expiry。

## 8. Human Resource Management 使用独立 Admin Permission

`PermissionListingKitAdminWrite` 当前也授予 `listingkit_operator`，因此不能用来保护 AdjustCredit / AdjustDebit / Compensate / Store Service Correction。

新增稳定 capability，例如：

```text
PermissionWorkbenchResourceAdmin
```

Phase1 tenant-mode 映射：

```text
listingkit_viewer   -> deny
listingkit_operator -> deny
listingkit_admin    -> allow
```

平台管理员如需操作，必须走明确 platform context / platform permission，不通过 tenant body 任意指定目标企业。

Human management route 仍要求：

```text
VerifiedIdentity
Live effective organization == target
PermissionWorkbenchResourceAdmin
Idempotency / concurrency checks
mandatory audit
```

Grant 继续仅 trusted Billing/Provisioning service principal；MigrationCredit 继续仅 migration runner。

## 9. 第七轮验收矩阵

至少新增：

```text
同 source credit 不同 correction Operation 只能成功一次
credit 已消费后 correction 只产生一次 debt
Renew missing/stale If-Match -> conflict before consume
Reactivate stale If-Match -> conflict before consume
rollback 时 reconciliation_required reservation 继续收敛
SQLSTATE 57014 server timeout -> bounded retry -> 503
caller context cancelled + 57014 -> 不重试
Correct Activate exact inverse
Correct Renew exact inverse
Correct Reactivate exact inverse
source 后已有 later lifecycle mutation -> NOT_INVERTIBLE
expand schema with populated legacy rows succeeds
backfill 后才 validate NOT NULL/check
active + NULL expiry -> fail closed
operator 调 AdjustDebit/Correction -> deny
listingkit_admin same-org -> allow
```
