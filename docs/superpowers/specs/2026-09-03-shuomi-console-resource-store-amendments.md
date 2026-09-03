# 硕米 Console 第一阶段：资源 Bucket 与店铺激活修订

**状态：** 已确认

**覆盖范围：** 本文覆盖并取代 `2026-09-02-shuomi-console-phase1-hard-cut-design.md` 中“第一阶段最小企业资源账本”“店铺绑定与显式激活”与本文冲突的描述；其他 Console 产品决策继续有效。

---

## 1. 企业资源 Bucket 唯一性

一个企业的一种资源只能存在一个并发权威 Bucket。

```text
resource_type
├── store_renewal_period
├── ai_point
└── data_row
```

持久化约束：

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

若技术原因保留代理主键，也必须存在：

```text
UNIQUE (organization_id, resource_type)
```

不得只依赖 `version`，因为版本号只能控制同一行，不能阻止并发首次插入两行。

Bucket 建立规则：

```text
INSERT ... ON CONFLICT DO NOTHING
→ 再按 (organization_id, resource_type) 锁定并读取唯一 Bucket
```

所有 Grant、Reserve、Commit、Release、Reverse、Expire 和 Migration Credit 都必须在这一唯一 Bucket 上执行，并与不可变资源事件处于同一数据库事务。

必须测试：

```text
两个实例并发为同一企业首次创建 Bucket
两个不同 Grant Operation 并发入账
创建与消费并发
重复 Operation ID
同 Operation ID 不同 Request Fingerprint
任何路径均只有一条 Bucket，余额不可为负
```

---

## 2. Reverse 必须有单一冲正不变量

第一阶段采用“一条原始资源事件最多有一条完整冲正事件”的简单模型，不支持部分冲正拆分。

资源事件：

```text
saas_organization_resource_events
- event_id PRIMARY KEY
- organization_id
- resource_type
- operation_type
- quantity
- balance_after
- idempotency_key
- request_fingerprint
- business_type
- business_id
- reversal_of NULLABLE
- actor_user_id
- occurred_at
```

数据库必须对非空 `reversal_of` 建唯一约束：

```text
UNIQUE (reversal_of) WHERE reversal_of IS NOT NULL
```

如果数据库/ORM 不方便创建部分唯一索引，则使用等价的专门 Reversal 表或可表达“非空唯一”的约束；不能只依赖代码先查再写。

Reverse 执行规则：

```text
锁定唯一 Bucket
→ 锁定原始 Event
→ 校验原始 Event 可被冲正
→ 尝试写入唯一 reversal_of
→ 更新 Bucket
→ 同事务提交
```

两个不同 Idempotency-Key 并发冲正同一原始 Event 时，最多一个成功；另一个返回稳定的 `RESOURCE_EVENT_ALREADY_REVERSED`，不得再次改变余额。

必须测试：

```text
相同 Operation ID 重放
不同 Operation ID 并发冲正同一 Event
数据库唯一约束竞争
冲正后再次冲正
余额与事件流水可重新核对
```

未来若需要“部分冲正”，必须单独设计累计冲正上限模型，不能直接放宽该唯一约束。

---

## 3. 店铺状态模型

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

“即将到期”仍由 `service_expires_at` 计算，不持久化为状态。

---

## 4. `activate` 只允许首次激活

`ActivateStoreService` 的必要前置条件：

```text
Store 属于 live resolved Effective Organization
record_status = active
connection_status = connected
service_status = pending_activation
企业续费期数余额 >= 1
调用者权限与成员月度限额（实现后）允许
```

以下状态必须拒绝 `activate`：

```text
activating
active
expired
suspended
```

返回稳定错误：

```text
STORE_SERVICE_ALREADY_ACTIVE
STORE_SERVICE_ACTIVATION_IN_PROGRESS
STORE_SERVICE_REACTIVATION_REQUIRED
STORE_SERVICE_SUSPENDED
```

不同 Idempotency-Key 不能绕过状态前置条件。幂等只解决同一逻辑操作重放，不等于“任何时候重复调用都成功”。

首次激活事务：

```text
锁定 Store service_version
锁定唯一 renewal-period Bucket
Reserve 1 期
service_status: pending_activation → activating
写 activation intent
Commit 1 期
service_started_at = now
service_expires_at = now + 30 天
service_status: activating → active
写资源事件、店铺服务事件和审计 Outbox
提交事务
```

若外部连接检查必须调用网络，应在进入短数据库事务前完成，并在提交时重新校验 Store version 与连接快照有效期。

---

## 5. `renew` 与 `reactivate` 是独立命令

### 5.1 使用中续费

```text
command = renew
eligible service_status = active
new_expiry = max(now, current_expiry) + N × 30 天
```

`N` 为经过服务端上限校验的正整数；重复 Operation ID 不重复扣减。

### 5.2 到期后重新激活

```text
command = reactivate
eligible service_status = expired
固定或明确选择消耗期数
service_started_at = now
service_expires_at = now + N × 30 天
service_status: expired → activating → active
```

到期状态不得调用首次 `activate`。这样审计、计费和 UI 文案能区分首次激活与恢复服务。

### 5.3 Suspended

`suspended` 不自动等同于到期。第一阶段 `activate`、`renew`、`reactivate` 均拒绝 suspended；恢复必须由单独 `resume` 策略根据停用原因决定是否消耗资源。

---

## 6. API 边界

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
Idempotency-Key
Request Fingerprint
```

`organization_id` 不从请求体读取，只能来自 live resolved Effective Organization。

---

## 7. 验收不变量

- 同一企业同一资源类型始终只有一个 Bucket。
- 并发首次建账不会产生重复 Bucket。
- 同一原始资源 Event 最多只能有一条完整 Reverse；并发不同幂等键也不能重复冲正。
- 已 active 的店铺调用 `activate` 不扣减资源，也不修改到期时间。
- `activating`、`suspended` 和 `expired` 不可走首次激活路径。
- active 店铺只通过 `renew` 延长，并保留剩余有效期。
- expired 店铺只通过 `reactivate` 恢复。
- 所有资源与 Store 状态变化在同一事务或既定恢复协议下可核对。
- 任何失败、重试、响应丢失和并发路径都不能产生负余额或缩短现有有效期。