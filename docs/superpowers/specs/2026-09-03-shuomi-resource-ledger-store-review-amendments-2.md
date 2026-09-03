# 硕米 Resource Ledger / Store Service：第二轮评审修订

**状态：** 权威修订  
**覆盖：** 本文继续覆盖原 Resource/Store 设计、实施计划及第一轮评审修订中的冲突内容。

---

## 1. Compensation source 必须同企业、同资源

任何 Compensation 都必须锁定并验证 source consume event：

```text
source.organization_id == compensation.organization_id
source.resource_type == compensation.resource_type
source.operation_type / business_type 在允许补偿集合内
source 尚未被 compensation
```

数据库优先使用可表达的复合引用，例如：

```text
(source_organization_id, source_event_id, source_resource_type)
```

若 event_id 全局唯一，仍必须在事务内锁 source event 并校验 tenant/resource；不能只依赖 `UNIQUE(compensation_of)`。

跨企业或跨资源 Compensation 一律拒绝，并增加数据库/服务层负向测试。

---

## 2. Store Service Consume Event 第一阶段禁止独立 Compensation

Activate/Renew/Reactivate 的 Consume 与 Store entitlement 是同一事务的一个业务事实。

第一阶段把这些 consume event 标记为：

```text
compensation_policy = coupled_store_service
```

禁止调用通用 `CompensateConsume` 单独返还 renewal periods。

如需纠错，必须走专门 `CorrectStoreServiceOperation`：

```text
锁原 Lifecycle Operation
锁 Store
锁 renewal Bucket
计算 Store side correction
同时修正 service_expires_at/service_status 与资源余额
写 paired correction events
同一事务提交
```

如果不能安全推导 Store 侧修正，fail closed，人工修复；不能只返资源而保留免费服务。

---

## 3. Renew / Reactivate 也要求 RecordStatus=active

三条生命周期命令统一要求：

```text
record_status = active
```

因此：

```text
Activate: record active + effective service pending_activation
Renew: record active + effective service active
Reactivate: record active + effective service expired
```

`deleting/deleted` 一律拒绝，不得消耗 renewal periods。

---

## 4. Lifecycle Replay 必须早于 volatile Connection Validation

请求顺序调整为：

```text
1. Auth + Live Organization + Permission
2. canonical fingerprint
3. 只读 Operation replay lookup
   - succeeded → 直接重放
   - failed_terminal → 直接重放稳定失败
   - same key + different fingerprint → conflict
   - absent/in_progress → 继续
4. volatile connection validation（仅新执行需要）
5. 进入事务再次锁/校验 Operation，处理竞争
6. 执行业务事务
```

因此，已成功 Operation 的重试不会因为 Connection Provider 临时宕机而改变历史结果。

事务内仍必须重新校验 Operation，避免两个首次请求同时通过只读 lookup。

---

## 5. Lifecycle Request Fingerprint 必须包含命令与目标

Canonical fingerprint 最少包含：

```text
organization_id
operation_type = activate|renew|reactivate
store_id
quantity / periods
If-Match aggregate version
所有影响行为的规范化请求字段
```

不得只 hash HTTP body。

同一 Idempotency-Key：

- 换 Store → conflict；
- activate 改 renew → conflict；
- quantity 改变 → conflict；
- If-Match 改变 → conflict；
- 完全相同 → replay。

---

## 6. Settlement Replay 必须早于 Reservation state check

Commit/Release 顺序：

```text
1. Auth/tenant scope
2. canonical fingerprint
3. 查 settlement Operation
   - succeeded/failed_terminal → replay
   - same key different fingerprint → conflict
   - absent → 继续
4. 事务内锁 Operation + Reservation
5. 只有首次 settlement 才要求 reservation.status = reserved
6. committed/released by another operation → already-settled conflict
```

因此 response loss 后同一 settlement Operation 可以稳定重放；不同 Operation 仍不能二次结算。

---

## 7. Ambiguous COMMIT 必须 read-after-unknown

数据库连接断开不能直接假定事务已回滚。

所有写命令在 COMMIT 返回 connection loss / ambiguous outcome 时进入：

```text
commit_outcome_unknown
→ 使用健康连接按 (organization_id, operation_id) 重读 Operation
   ├── succeeded/failed_terminal → replay persisted outcome
   ├── executing/unknown → bounded reconcile/retry read
   └── authoritative absent → 才允许重新执行
```

不能在未重读 Operation 的情况下直接重跑资源/Store 写。

测试必须故障注入“数据库已 commit，但客户端丢失 COMMIT response”。

---

## 8. Erroneous Credit 已被消费时记录 Residual Debt

`AdjustDebit` 只允许扣除当前可用余额，不能把 Bucket 扣成负数。

若错误 Credit 数量 `q` 已部分消费：

```text
recoverable_now = min(available, q)
residual_debt = q - recoverable_now
```

处理：

```text
available -= recoverable_now
记录 correction event
记录 saas_organization_resource_debts(resource_type, remaining_debt, source_correction_id)
```

后续新 Grant/Credit 到账时，必须先按策略清偿 debt，再增加 spendable available；直到 debt=0。

第一阶段不允许为了纠错制造负 available，也不声称单次 AdjustDebit 可以撤销已消费额度。

若该 Credit 与支付退款等资金业务相关，进入后续财务切片，不在本资源切片自行退款。

---

## 9. 实施计划追加

- Compensation tenant/resource mismatch tests。
- Store lifecycle consume 禁止 generic compensation；专门 correction 需要 Store+Resource 同事务。
- Renew/Reactivate `record_status=active` 测试。
- Operation replay 在 Connection Provider 故障前返回历史结果。
- Fingerprint 跨 command/Store/quantity/version 冲突测试。
- Commit/Release response-loss replay。
- Lost COMMIT response read-after-unknown。
- Erroneous Credit 已消费后的 residual debt 与后续 credit 自动清偿测试。

---

## 10. 更新后的核心不变量

- Compensation 不得跨 tenant/resource。
- Store lifecycle resource consumption 与 Store entitlement 不能被单边补偿。
- deleting/deleted Store 不能 Renew/Reactivate。
- 已完成 Operation 的 replay 不依赖外部 Connection Provider 当前健康状态。
- Idempotency fingerprint 明确绑定 command + target + quantity + concurrency version。
- Settlement response loss 可由同一 Operation 稳定 replay。
- COMMIT 结果不明先重读 Operation，再决定是否重试。
- 已消费的错误 Credit 通过 residual debt 收敛，不产生负余额。
