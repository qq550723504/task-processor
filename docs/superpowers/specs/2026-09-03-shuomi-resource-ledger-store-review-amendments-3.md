# 硕米 Resource Ledger / Store Service：第三轮评审修订

**状态：** 权威修订  
**覆盖：** 本文继续覆盖原 Resource/Store 设计、实施计划及前两轮评审修订中的冲突内容。

---

## 1. Reservation Recovery 必须先取得 Owner 的可结算事实，未知状态 Fail Closed

Reservation `expires_at` 只是“进入恢复检查”的时间，不等于“可以直接 Release”。

异步 Owner 必须有持久状态：

```text
not_started
processing
succeeded_terminal
failed_terminal
cancelled_terminal
outcome_unknown
```

Recovery Worker 锁定 Reservation 与 Owner Intent 后按以下规则处理：

```text
owner = not_started / failed_terminal / cancelled_terminal
→ 可幂等 Release

owner = succeeded_terminal
→ 必须 Commit；不能 Release

owner = processing / outcome_unknown
→ reservation = reconciliation_required
→ 保留 reserved 余额
→ 交给领域 Reconciler 查明真实结果
→ 未查明前禁止自动 Release
```

复用仓库 PAY-042 已有“processing window 不确定则阻断并 reconciliation”的模式；不以 heartbeat 过期或 worker 不在线作为“业务没有发生”的证据。

Owner 写 terminal result 与 Reservation settlement 的交接必须使用领域定义的 durable intent/fence；崩溃窗口由 Reconciler 读取 owner terminal fact 后 Commit/Release。

---

## 2. Resource Debt 拦截所有正向 `available` 回流

Debt 不能只在 Grant/Credit 时清偿。所有会产生 `delta_available > 0` 的路径必须经过唯一函数/事务规则：

```text
ApplyPositiveAvailableDelta(org, resource, q)
```

事务语义：

```text
repay = min(outstanding_debt, q)
debt -= repay
available += (q - repay)
```

适用于：

```text
Grant / MigrationCredit
Reservation Release
允许的 Consume Compensation
AdjustCredit
Store/Resource correction 中的正向返还
其他未来正向 available delta
```

因此“错误 Credit 100 → Reserve 100 → 纠错形成 debt 100 → Release 100”会先把 debt 清为 0，`available` 仍为 0，不会重新变成可消费余额。

所有正向 available 事件同时记录：

```text
gross_positive_delta
debt_repaid
net_available_delta
```

便于审计与对账。

---

## 3. Store Migration 必须优先保留 Record Lifecycle

迁移优先级固定为：

```text
1. deleted_at != NULL / 已删除事实
   → record_status = deleted

2. legacy lifecycle_status = deleting
   → record_status = deleting

3. 其他现存 Store
   → record_status = active
```

只有 `record_status=active` 后才计算 Service Status：

```text
legacy disabled → suspended
known future expiry → active
known past expiry → expired
connected + expiry unknown → pending_activation
其他 → 按安全空态/连接态处理
```

`deleting/deleted` Store 不执行 expiry/connection 推断，也不能进入 Activate/Renew/Reactivate。

Soft-deleted 行是否出现在普通 List 继续遵循 Store Repository 的删除过滤规则；迁移脚本仍必须保留其 record status 以支持审计/恢复工具。

---

## 4. Migration Source 使用全局唯一 Claim

仅用 `(organization_id, operation_id)` 不能阻止同一源记录改绑企业后再次入账。

新增：

```text
saas_resource_migration_claims
- source_table
- source_primary_key
- resource_type
- source_organization_id
- operation_id
- source_fingerprint
- state
- created_at
- updated_at

UNIQUE(source_table, source_primary_key, resource_type)
```

迁移前必须先锁/创建全局 Source Claim。

语义：

```text
同 source + 同 org + 同 fingerprint
→ replay

同 source + payload 改变
→ MIGRATION_SOURCE_CHANGED

同 source + organization 从 A 改为 B
→ MIGRATION_OWNERSHIP_CHANGED
→ 不自动给 B 再 Credit
→ 进入显式 reconciliation
```

Ownership correction 需要核对 A 的历史 credit/usage，再通过独立、审计化 correction/transfer 流程处理；不能通过换 tenant-scoped Operation Key 静默重复入账。

---

## 5. 兼容现有 `/disable` 与 `/enable` Route

第一阶段硬切后仍存在的 Store Center route 必须与新 Service Status 一致，不能只改 legacy LifecycleStatus。

### Disable

`POST /stores/:id/disable` 在共享 Store 事务中：

```text
锁 Store
要求 record_status = active
legacy lifecycle → disabled
service_status → suspended
记录 suspension_reason = operator_disabled
写 Store Audit
提交
```

Disable 不退还已消费 renewal periods，不修改已有 `service_expires_at`；暂停期是否顺延属于后续 Resume policy。

### Enable

第一阶段 **不得把 `/enable` 当作 Resume**。

若 Store 因新 Service Status 为 `suspended`：

```text
POST /enable
→ 返回 STORE_SERVICE_RESUME_REQUIRED
→ 不修改 legacy lifecycle / service status / expiry / balance
```

新 Console 不提供该旧入口。未来新增显式 `/resume` 时单独定义恢复原因、是否延长期限、是否消费资源。

若存在历史兼容调用方，发布前必须枚举并迁移；不能让旧 `/enable` 静默绕过 suspension policy。

---

## 6. Operation 必须保存不可变 Lifecycle Response Snapshot

Lifecycle 成功 Operation 不再只保存指向可变 Store 的 `result_reference`。

Operation 增加受限、不可变结果快照（可为列或受 schema 校验的 JSON）：

```text
result_store_id
result_record_status
result_service_status
result_service_started_at
result_service_expires_at
result_store_version
result_resource_type
result_resource_delta
result_resource_balance_after
completed_at
```

该 snapshot 在 Operation `succeeded` 时与 Store/Resource 变更同事务写入，之后不得因 Store 再次 Renew/Delete 而改变。

Response-loss retry 直接从 Operation Snapshot 重建原响应；即使 Store 后续已删除，也能稳定重放历史结果。

禁止把凭据、Provider token、请求原文等敏感数据放进 snapshot。

---

## 7. 新 Service/Record 字段必须更新 Strict Frontend Contract

无论是否采用独立 `serviceVersion`，新 Store read contract 都必须显式增加：

```text
recordStatus
serviceStatus            // effective status
serviceStartedAt
serviceExpiresAt
version                   // aggregate If-Match authority
connectionStatus
connectionObservedAt（如对 UI 有用）
```

必须修改并测试：

```text
web/listingkit-ui/src/lib/server/workbench-proxy.ts
web/listingkit-ui/src/lib/api/workbench-stores.ts
对应 Zod/strict schema tests
Store list/detail UI state mapping
Activate/Renew/Reactivate mutation response schemas
```

不得把 `expired/suspended/pending_activation` 塞进 legacy `lifecycleStatus` 字段；旧 lifecycle 与新 record/service 语义分栏直到旧字段可安全删除。

---

## 8. Store Lifecycle Audit 与业务事务同提交

共享 `StoreResourceUnitOfWork` 增加 tx-bound audit/outbox 能力：

```text
AppendStoreAudit(...)
AppendAuditOutbox(...) // 仅当存在外部审计投影
```

Activate/Renew/Reactivate/Disable（以及未来 Resume）的同一事务必须写入至少：

```text
subject/user
home organization
effective organization
store/resource id
action
result / resulting status
resource quantity/delta
request id / operation id
occurred_at
```

若 Store Audit 插入失败，整个 Resource + Store transaction 回滚。

若审计还需投影到外部系统，则同事务写 Outbox，后台异步投递；不能先提交收费/服务变更再单独 best-effort 写审计。

---

## 9. 实施计划追加

- Reservation ambiguous-owner recovery：processing/outcome_unknown 不自动 release。
- Debt repayment 覆盖 Release/Compensation/AdjustCredit 等所有正向 available 路径。
- Migration 保留 deleted/deleting/disabled 优先级。
- Global migration source claim，跨 Organization 改绑必须 conflict/reconcile。
- `/disable` 原子设置 suspended；旧 `/enable` 对 suspended 返回 Resume Required。
- Lifecycle Operation immutable response snapshot + response-loss replay after later Store mutation/delete。
- Next.js proxy/API strict schema 与 UI 状态映射升级。
- Store Audit/Outbox 与 Resource/Store 写同事务，故障注入证明审计失败会回滚业务变更。

---

## 10. 更新后的核心不变量

- Reservation 过期不是 Release 证据；Owner 结果不明确时 fail closed + reconcile。
- 任何正向 available 回流都先清偿 Resource Debt。
- Migration 永远保留 deleted/deleting/disabled 事实。
- 同一迁移源记录只能被全局 Claim 一次，改绑企业不会二次 Credit。
- 旧 disable/enable 不能绕过新 suspension 语义。
- Idempotent replay 返回原始不可变 Lifecycle Response，而不是当前 Store 状态。
- 新 Service/Record 状态是显式前后端合同，不复用 legacy lifecycle enum。
- 收费、Store 状态和强制审计在同一 Unit of Work 中原子提交。
