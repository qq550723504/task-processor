# 硕米企业资源账本与店铺服务生命周期设计

**状态：** 待评审

**范围：** 企业资源余额、异步资源 Reservation、店铺首次激活/续费/重新激活。

**不在本设计范围：** 账号注册、ZITADEL 登录、Onboarding、企业邀请、支付/退款/发票。

---

## 1. 设计原则

资源账本是企业级业务事实，与身份系统解耦。

```text
Enterprise Resource Wallet
├── store_renewal_period
├── ai_point
└── data_row
```

资源不是人民币余额；人民币钱包未来单独设计。

复用现有 `internal/listingsubscription` usage ledger 已验证的工程模式：

```text
数据库唯一幂等身份
事务内 Bucket + Event
Reserve / Commit / Release 生命周期
reversal/compensation 唯一约束
并发测试
```

但不把“套餐配额使用量”与“可购买资源余额”强行塞进同一现有表；两者语义不同。

---

## 2. Bucket：企业 + 资源类型唯一

```text
saas_organization_resource_buckets
- organization_id
- resource_type
- available BIGINT NOT NULL
- reserved BIGINT NOT NULL
- consumed BIGINT NOT NULL
- version BIGINT NOT NULL
- created_at
- updated_at

PRIMARY KEY (organization_id, resource_type)
CHECK available >= 0
CHECK reserved >= 0
CHECK consumed >= 0
```

首次创建使用：

```text
INSERT ... ON CONFLICT DO NOTHING
→ SELECT ... FOR UPDATE
```

任何余额变化必须锁定唯一 Bucket 并与对应 Operation/Event/Reservation 在同一数据库事务提交。

---

## 3. Operation：逻辑幂等权威独立于 Event

```text
saas_organization_resource_operations
- organization_id
- operation_id
- operation_type
- request_fingerprint
- state
- result_reference NULLABLE
- failure_code NULLABLE
- created_at
- completed_at NULLABLE

PRIMARY KEY (organization_id, operation_id)
```

同一个 client operation ID 可以在不同企业出现，因此所有关联都必须使用复合键 `(organization_id, operation_id)`。

规则：

```text
same key + same fingerprint + succeeded
→ replay persisted result

same key + same fingerprint + failed_terminal
→ replay persisted failure

same key + different fingerprint
→ RESOURCE_IDEMPOTENCY_CONFLICT
```

### 3.1 transient failure

数据库死锁、连接断开、内部暂时不可用等 retryable failure：整个事务回滚，不持久化 terminal failure；同 key 重试可以重新执行。

### 3.2 terminal failure

余额不足、非法状态、业务前置条件不满足等确定性业务拒绝：可以持久化：

```text
state = failed_terminal
failure_code = stable domain code
```

之后同 key 必须重放该失败；条件变化后客户端需要使用新的 Operation ID 发起新逻辑操作。

---

## 4. Event：不可变余额变化事实

```text
saas_organization_resource_events
- event_id UUID PRIMARY KEY
- organization_id
- operation_id
- resource_type
- event_type
- delta_available
- delta_reserved
- delta_consumed
- reservation_id NULLABLE
- compensation_of NULLABLE
- business_type
- business_id
- actor_user_id
- occurred_at

FOREIGN KEY (organization_id, operation_id)
  REFERENCES saas_organization_resource_operations(organization_id, operation_id)
```

不使用单列 `operation_id` 外键。

每个 Event 记录三个余额的明确 Delta，不能只存一个模糊 `quantity` 再由调用者推断方向。

---

## 5. Reservation：必须有独立结算状态

异步任务不能只依赖 Bucket aggregate `reserved`，必须有可锁定的 Reservation。

```text
saas_organization_resource_reservations
- reservation_id UUID PRIMARY KEY
- organization_id
- resource_type
- quantity BIGINT NOT NULL
- state VARCHAR NOT NULL
- business_type
- business_id
- reserve_operation_id
- settlement_operation_id NULLABLE
- created_at
- settled_at NULLABLE

state = reserved | committed | released
UNIQUE (organization_id, reserve_operation_id)
```

Commit/Release：

```text
SELECT reservation FOR UPDATE
require state = reserved
→ transition exactly once
```

因此两个不同 Commit Operation 或 Commit/Release 并发针对同一个 Reservation 时，最多一个能从 `reserved` 状态成功结算。

### 5.1 Reserve

```text
available -= q
reserved += q
consumed += 0
create Reservation(state=reserved)
```

### 5.2 Commit

```text
available += 0
reserved -= q
consumed += q
Reservation reserved → committed
```

### 5.3 Release

```text
available += q
reserved -= q
consumed += 0
Reservation reserved → released
```

Commit/Release 不创建第二份 Reservation，不通过 aggregate `reserved >= q` 证明目标 Reservation 可结算。

---

## 6. Grant 与纠错：第一阶段不做“来源 Grant 可逆”假设

企业资源是可替代的 fungible balance。Aggregate `available >= q` 不能证明某一笔特定 Grant 没被消费。

因此第一阶段：

```text
grant
→ available +q

migration_credit
→ available +q
```

**Grant/Migration Credit 不提供 source-specific generic Reverse。**

管理员纠错使用独立命令：

```text
adjust_credit  → available +q
adjust_debit   → available -q，require available >= q
```

它们是新的审计事实，不声称“撤销某一笔仍未使用的 Grant”。

未来如果支付退款要求按购买批次撤回剩余资源，再单独引入 Grant Lot / Consumption Allocation；不要在第一阶段用 aggregate balance 假装有来源归属。

---

## 7. 已消费资源的补偿

需要返还已完成消费时，使用显式 compensation，而不是 Reverse Grant。

```text
compensate_consume
→ reference source consume event
→ available +q
→ consumed -q
```

数据库要求：

```text
UNIQUE(compensation_of) WHERE compensation_of IS NOT NULL
```

第一阶段只允许完整补偿一条 eligible consume event，不支持部分补偿。

Reserve 失败/取消优先 `Release`；尚未 Commit 的 Reservation 不走 compensation。

---

## 8. 同步 Consume：店铺服务不需要 Reserve/Commit 两阶段

店铺激活/续费/重新激活在同一 PostgreSQL 内更新 Store 和资源 Bucket，不需要为了“形式统一”制造 Reserve→Commit。

在一个短事务中：

```text
lock Operation
lock Store row/version
lock renewal-period Bucket
validate state + available
available -= N
consumed += N
write consume Event
update Store service fields
complete Operation
COMMIT
```

外部网络校验必须在事务前完成；提交时重新检查 Store version/连接快照有效性。

这样不会出现“资源已扣但 Store 未更新”或“Store 已更新但资源没扣”的跨事务窗口。

---

## 9. Store 状态模型

```text
ConnectionStatus
pending | connected | error | disconnected

ServiceStatus
pending_activation | activating | active | expired | suspended

RecordStatus
active | deleting | deleted
```

“即将到期”由 `service_expires_at` 计算，不落持久状态。

---

## 10. 首次 Activate

只允许：

```text
record_status = active
connection_status = connected
service_status = pending_activation
renewal-period available >= 1
```

拒绝：

```text
activating
active
expired
suspended
```

同一事务：

```text
consume 1 store_renewal_period
service_started_at = now
service_expires_at = now + 30 days
service_status = active
```

不同 Operation ID 的第二次 activate 仍然因状态前置条件失败，不再次扣资源。

---

## 11. Renew

只允许：

```text
service_status = active
N > 0 且 <= server max
renewal-period available >= N
```

同一事务：

```text
consume N store_renewal_period
new_expiry = max(now, current_expiry) + N * 30 days
service_expires_at = new_expiry
```

必须保证：

- 资源扣减与 expiry 更新同事务；
- same Operation replay 不重复扣减；
- current expiry 不会被缩短。

---

## 12. Reactivate

只允许：

```text
service_status = expired
N > 0 且 <= server max
renewal-period available >= N
```

同一事务：

```text
consume N store_renewal_period
service_started_at = now
service_expires_at = now + N * 30 days
service_status = active
```

Expired 不允许调用首次 activate。

---

## 13. Suspended

`suspended` 不等于 expired。

第一阶段：

```text
activate → reject
renew → reject
reactivate → reject
```

恢复由后续独立 `resume` 策略定义，避免把风控/人工停用与正常到期混为一谈。

---

## 14. API 幂等与租户边界

```http
POST /api/v1/workbench/stores/{store_id}/activate
POST /api/v1/workbench/stores/{store_id}/renew
POST /api/v1/workbench/stores/{store_id}/reactivate
```

要求：

```text
AuthPolicyVerifiedIdentity
OrganizationAccessPolicyLiveWrite
If-Match / service_version
Idempotency-Key
canonical Request Fingerprint
```

Organization 只来自 live resolved Effective Organization。

资源 Operation PK 使用 `(organization_id, operation_id)`，所有 Event/Reservation 同样绑定 organization。

---

## 15. 现有 listingsubscription usage ledger 的复用边界

直接复用的工程模式：

```text
unique tenant/idempotency identity
transaction retry
Reserve/Commit/Release state transition
reversal/compensation unique source reference
outbox/reconciliation pattern（需要时）
```

不直接复用的业务含义：

```text
subscription quota used
≠
enterprise purchased resource available balance
```

因此新资源域可以有独立表/包，但实现和测试应参考现有 `usage_ledger_gorm.go`，而不是再发明全局 HTTP Operation Framework。

---

## 16. 迁移规则

现有 Store：

```text
已有可信 service expiry
→ 迁移为 active + 原 expiry

只有连接记录、没有可信服务期
→ connected + pending_activation
```

不得凭当前日期伪造历史 expiry。

资源余额迁移使用 `migration_credit`，不可 source-reverse；发现错误时使用 audited adjustment。

---

## 17. 关键验收

必须覆盖：

```text
并发首次 Bucket 创建
same Operation/same fingerprint replay
same Operation/different fingerprint conflict
terminal failure replay
transient failure rollback + retry
两个 Commit 结算同一 Reservation
Commit vs Release 并发
Grant A 消费后 Grant B 入账，不能再“Reverse Grant A”拿走 B
consume compensation 只能一次
activate 双击/不同 Operation 并发
active renew 并发
expired reactivate 并发
renew/reactivate 资源不足不改 Store
资源扣减后事务失败回滚 Store 和 Bucket
Store version 冲突不扣资源
跨企业复用相同 operation_id 不串账
Event composite FK 拒绝跨企业 Operation
```

---

## 18. 完成定义

- 一个企业一种资源只有一个 Bucket。
- Operation 是逻辑幂等权威，Event 不是。
- Event 用 `(organization_id, operation_id)` 复合外键。
- 异步 Reserve 有独立 Reservation 状态，Commit/Release 只能结算一次。
- 第一阶段不声称 aggregate balance 能证明特定 Grant 未使用。
- Grant 不做 generic source reverse；纠错使用 adjustment。
- 已消费资源补偿有唯一 `compensation_of`。
- Activate/Renew/Reactivate 的资源扣减与 Store 更新在同一 DB 事务。
- 不把资源钱包与 ZITADEL/账号注册耦合。