# 硕米 Console：企业资源账本与店铺激活修订

**状态：** 已确认

**覆盖范围：** 本文覆盖并取代 `2026-09-02-shuomi-console-phase1-hard-cut-design.md` 中“第一阶段最小企业资源账本”“店铺绑定与显式激活”与本文冲突的描述。

**实施边界：** 本文属于独立 Resource / Store Slice，不与账号入口 Login V2 实现耦合。账号入口 PR 可以引用这些产品不变量，但资源账本应有独立实施计划和代码评审。

---

## 1. 不重复发明第二套通用账本框架

仓库已经有 `internal/listingsubscription` 的 durable usage ledger，并已经实现：

```text
Reserve
Commit
Release
Reverse
数据库级 tenant + idempotency_key 唯一性
reversal_of 唯一性
并发事务与重放
```

企业资源钱包的余额语义与 usage ledger 不完全相同，因此不能把 `available / reserved / consumed` 强行塞入现有 Usage Bucket；但实现应复用现有 ledger 的**存储模式、幂等规则、事务写法和测试方法**，不得再建设一个 HTTP 级万能 Operation Framework。

---

## 2. 企业资源 Bucket 唯一性

资源类型：

```text
store_renewal_period
ai_point
data_row
```

持久化：

```text
saas_organization_resource_buckets
- organization_id
- resource_type
- available
- reserved
- consumed
- version
- created_at
- updated_at

PRIMARY KEY (organization_id, resource_type)
```

若保留代理主键，也必须有：

```text
UNIQUE (organization_id, resource_type)
```

首次创建：

```text
INSERT ... ON CONFLICT DO NOTHING
→ 再按 (organization_id, resource_type) FOR UPDATE 读取唯一 Bucket
```

所有余额更新必须保证：

```text
available >= 0
reserved >= 0
consumed >= 0
```

---

## 3. 逻辑 Operation 必须有独立数据库身份

单个业务命令可能产生多条 Event，例如 Reserve → Commit，因此 `event_id` 不能同时承担逻辑 Operation 幂等身份。

新增：

```text
saas_organization_resource_operations
- organization_id
- operation_id
- operation_type
- request_fingerprint
- state: executing | succeeded | failed
- result_event_id NULLABLE
- result_reference NULLABLE
- created_at
- updated_at

PRIMARY KEY (organization_id, operation_id)
```

必要规则：

```text
同 operation_id + 同 operation_type + 同 request_fingerprint
→ replay 已有结果 / 继续同一 executing operation

同 operation_id + 不同 operation_type 或不同 request_fingerprint
→ RESOURCE_IDEMPOTENCY_CONFLICT
```

每个 Grant、Reserve、Commit、Release、Reverse、Expire、MigrationCredit 命令都必须：

```text
1. 在事务内取得 / 创建 resource_operation
2. 校验 fingerprint
3. 锁定目标 Bucket
4. 执行业务状态转换
5. 写 Event
6. 更新 resource_operation.result_*
7. 标记 succeeded
8. 同事务提交
```

不得只在 Handler 中“先查一下 idempotency_key 再执行”，因为并发请求必须由数据库唯一键决定胜负。

---

## 4. 资源 Event 记录真实三余额 Delta

资源事件建议显式保存三维变化，避免只存 `quantity` 后靠 `operation_type` 猜余额变化：

```text
saas_organization_resource_events
- event_id PRIMARY KEY
- organization_id
- resource_type
- operation_id
- operation_type
- delta_available
- delta_reserved
- delta_consumed
- available_after
- reserved_after
- consumed_after
- business_type
- business_id
- reversal_of NULLABLE
- actor_user_id
- occurred_at
```

要求：

```text
FOREIGN KEY / logical reference operation_id → resource_operations
UNIQUE (reversal_of) WHERE reversal_of IS NOT NULL
```

同一个逻辑 Operation 可以产生多个 lifecycle Event，但它们必须明确关联同一个 `operation_id`。

---

## 5. 标准余额转换

设数量 `q > 0`。

### 5.1 Grant / MigrationCredit

```text
available += q
reserved  += 0
consumed  += 0
```

Delta：

```text
(+q, 0, 0)
```

### 5.2 Reserve

前置：

```text
available >= q
```

转换：

```text
available -= q
reserved  += q
consumed  += 0
```

Delta：

```text
(-q, +q, 0)
```

### 5.3 Commit

前置：

```text
对应 reservation 仍处于 reserved
reserved >= q
```

转换：

```text
available += 0
reserved  -= q
consumed  += q
```

Delta：

```text
(0, -q, +q)
```

### 5.4 Release

前置：

```text
对应 reservation 仍处于 reserved
reserved >= q
```

转换：

```text
available += q
reserved  -= q
consumed  += 0
```

Delta：

```text
(+q, -q, 0)
```

### 5.5 Expire

第一阶段只允许过期**尚未 Reserve / Commit 的 available 资源**：

前置：

```text
available >= q
```

转换：

```text
available -= q
```

Delta：

```text
(-q, 0, 0)
```

已 reserved 的资源不得被 Expire；先按业务规则完成 Commit / Release。

---

## 6. Reverse 的精确允许矩阵

第一阶段采用“一条原始资源 Event 最多一条完整 Reverse”，不支持部分冲正。

### 6.1 `grant` / `migration_credit`

可 Reverse，但只能撤回**尚未被使用的余额**。

前置：

```text
source.delta = (+q, 0, 0)
current available >= q
source 尚无 reversal
```

Reverse Delta：

```text
(-q, 0, 0)
```

如果该 Grant 对应的额度已经被 Reserve / Commit，导致 `available < q`，返回：

```text
RESOURCE_REVERSAL_INSUFFICIENT_UNUSED_BALANCE
```

不得因为“原始 Grant 是 q”就把已经消费的信用重新扣成负数。

### 6.2 `reserve`

**不允许 generic Reverse。**

正确反向动作是：

```text
Release(reservation)
```

因为 Release 需要验证 reservation lifecycle，而不是单纯把 Event Delta 取反。

返回：

```text
RESOURCE_REVERSAL_USE_RELEASE
```

### 6.3 `commit`

允许作为明确的“消费冲正 / 退款式返还”进行 Reverse。

前置：

```text
source = committed consumption
source 尚无 reversal
consumed >= q
```

Reverse Delta：

```text
(+q, 0, -q)
```

结果是把已消费资源返还到 `available`，不是重新变回 `reserved`。

### 6.4 `release`

**不允许 generic Reverse。**

若业务需要再次占用资源，必须发起一个新的 `Reserve` Operation，并重新执行可用余额和权限检查。

返回：

```text
RESOURCE_REVERSAL_USE_NEW_RESERVATION
```

### 6.5 `expire`

只允许平台纠错场景 Reverse，普通租户业务 API 不开放。

前置：

```text
source = expire
source 尚无 reversal
平台纠错权限
```

Reverse Delta：

```text
(+q, 0, 0)
```

### 6.6 `reverse`

Reverse Event 本身不可再次 Reverse：

```text
RESOURCE_REVERSAL_OF_REVERSAL_FORBIDDEN
```

---

## 7. Reverse 的数据库不变量

数据库对非空 `reversal_of` 建唯一约束：

```text
UNIQUE (reversal_of) WHERE reversal_of IS NOT NULL
```

Reverse 事务：

```text
取得 resource_operation
→ 锁定 source event
→ 锁定唯一 bucket
→ 检查 source operation matrix
→ 检查 source 尚无 reversal
→ 计算明确 Delta
→ 验证三余额不会为负
→ 插入 reversal event（受 reversal_of unique 保护）
→ 更新 bucket
→ 完成 resource_operation
→ commit
```

两个不同 Operation ID 并发冲正同一 source event 时，最多一个成功；另一个稳定返回：

```text
RESOURCE_EVENT_ALREADY_REVERSED
```

---

## 8. 店铺状态模型

```text
ConnectionStatus
├── pending
├── connected
├── error
└── disconnected

ServiceStatus
├── pending_activation
├── activating
├── active
├── expired
└── suspended

RecordStatus
├── active
├── deleting
└── deleted
```

“即将到期”由 `service_expires_at` 计算，不持久化为独立状态。

---

## 9. `activate` 只允许首次激活

必要前置：

```text
Store 属于 live resolved Effective Organization
record_status = active
connection_status = connected
service_status = pending_activation
企业 store_renewal_period available >= 1
调用者权限 / 成员限额允许
```

以下状态拒绝 `activate`：

```text
activating
active
expired
suspended
```

稳定错误：

```text
STORE_SERVICE_ALREADY_ACTIVE
STORE_SERVICE_ACTIVATION_IN_PROGRESS
STORE_SERVICE_REACTIVATION_REQUIRED
STORE_SERVICE_SUSPENDED
```

首次激活：

```text
同一 Store Operation ID 建立幂等身份
锁 Store service_version
Reserve 1 个 renewal period
pending_activation → activating
写 activation intent
Commit reservation
service_started_at = now
service_expires_at = now + 30 days
activating → active
写 Store Event / Resource Event / Audit Outbox
提交
```

已 active 的 Store 即使换一个 Idempotency-Key 也不能再次 `activate`。

---

## 10. `renew` 与 `reactivate`

### 10.1 Renew

```text
eligible = active
new_expiry = max(now, current_expiry) + N × 30 days
```

`N` 为服务端校验的正整数；重复 Operation ID 不重复扣减。

### 10.2 Reactivate

```text
eligible = expired
service_started_at = now
service_expires_at = now + N × 30 days
expired → activating → active
```

expired 不得走首次 `activate`。

### 10.3 Suspended

`suspended` 不是 expired。第一阶段 `activate / renew / reactivate` 均拒绝 suspended；恢复由未来独立 `resume` 策略定义。

---

## 11. API 边界

```http
POST /api/v1/workbench/stores/{store_id}/activate
POST /api/v1/workbench/stores/{store_id}/renew
POST /api/v1/workbench/stores/{store_id}/reactivate
```

三条路由都要求：

```text
AuthPolicyVerifiedIdentity
OrganizationAccessPolicyLiveWrite
If-Match / service_version
Idempotency-Key / Operation ID
Request Fingerprint
```

`organization_id` 只能来自 live resolved Effective Organization，不从请求体读取。

---

## 12. 必须测试的并发与幂等场景

```text
两个实例并发首次创建同一 Bucket
同 Operation ID 同 fingerprint 并发执行
同 Operation ID 不同 fingerprint
不同 Grant 并发入账
Reserve 与 Expire 并发
Reserve / Commit / Release lifecycle 并发
两个不同 Reverse Operation 并发冲正同一 source
Grant 已部分/全部消费后尝试 Reverse
Commit Reverse 后余额准确返还 available
Release 不允许 generic Reverse
已 active Store 不允许第二次 activate
active Renew 不损失原剩余有效期
expired 只走 Reactivate
```

---

## 13. 验收不变量

- 同一企业同一资源类型始终只有一个 Bucket。
- 每个逻辑 Resource Operation 都有数据库唯一身份和 Request Fingerprint。
- Event ID 不替代 Operation ID。
- 同一 source event 最多一条完整 Reverse。
- Reverse 必须遵循明确 source-operation matrix，不能简单“quantity 取负”。
- 任何余额路径都不能产生负数。
- 已消费的 Grant 不能通过冲正凭空制造余额。
- Reserve 的正常反向动作是 Release；Release 的反向动作是新 Reserve。
- 已 active 的 Store 调用 `activate` 不扣资源、不改 expiry。
- active 只通过 `renew` 延长，expired 只通过 `reactivate` 恢复。
- 所有资源和 Store 状态变化都能通过 Operation + Event + Bucket 重放核对。