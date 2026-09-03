# 硕米企业资源账本与店铺服务 Implementation Plan

**目标：** 构建企业资源 Bucket/Operation/Reservation/Event，并让店铺 Activate/Renew/Reactivate 与续费期数扣减保持单库事务原子性。

**权威设计：** `docs/superpowers/specs/2026-09-03-shuomi-resource-ledger-store-design.md`

---

## Task 1：审计现有 usage ledger，可复用模式不复用错误语义

阅读并锁定：

```text
internal/listingsubscription/usage_ledger_gorm.go
internal/listingsubscription/gorm_repository.go
internal/listingsubscription/gorm_usage_ledger_test.go
internal/storecenter/**
```

输出设计对照：

```text
复用：unique idempotency、transaction retry、state transition、source unique compensation
不复用：subscription quota bucket 作为企业可购买余额
```

---

## Task 2：Resource Bucket + Operation schema

新增独立资源域 package（命名按 repo 审核后确定，例如 `internal/orgresource`）。

Schema：

```text
saas_organization_resource_buckets
PRIMARY KEY (organization_id, resource_type)

saas_organization_resource_operations
PRIMARY KEY (organization_id, operation_id)
```

Operation 保存：

```text
operation_type
request_fingerprint
state
result_reference
failure_code
```

测试：

```text
same org/same operation conflict
cross-org same operation allowed
same fingerprint replay
changed fingerprint conflict
terminal failure replay
transient rollback has no terminal row
```

---

## Task 3：Event composite FK

新增：

```text
saas_organization_resource_events
```

事件必须包含：

```text
organization_id
operation_id
delta_available
delta_reserved
delta_consumed
```

数据库 FK：

```text
(organization_id, operation_id)
→ resource_operations(organization_id, operation_id)
```

负向测试：A 企业 Event 不能指向 B 企业相同 operation_id。

---

## Task 4：Reservation settlement authority

新增：

```text
saas_organization_resource_reservations
reservation_id PK
state reserved|committed|released
```

实现：

```go
Reserve(...)
Commit(reservationID,...)
Release(reservationID,...)
```

Commit/Release 必须 `SELECT ... FOR UPDATE` Reservation 并只允许 `reserved` 转换一次。

并发测试：

```text
Commit vs Commit
Commit vs Release
Release vs Release
```

第二个 settlement 不得消费其他 Reservation 的 aggregate reserved balance。

---

## Task 5：Grant / Adjustment / Consume / Compensation

第一阶段命令：

```text
Grant
MigrationCredit
AdjustCredit
AdjustDebit
Consume
CompensateConsume
```

禁止：

```text
Generic ReverseGrant
```

原因：aggregate available 无法证明具体 Grant lot 尚未使用。

`CompensateConsume` 必须：

```text
source consume eligible
UNIQUE(compensation_of)
完整补偿一次
```

---

## Task 6：Operation 失败语义

定义：

```text
validation/auth error → Operation 前返回
business terminal failure → persisted failed_terminal + failure_code
transient DB/internal failure → transaction rollback, retry same key allowed
```

same key 对 terminal failure 重放同一稳定错误；条件变化需要新的 Operation ID。

---

## Task 7：Store Service 状态模型

扩展/校准 Store：

```text
ConnectionStatus
ServiceStatus
RecordStatus
service_started_at
service_expires_at
service_version
```

迁移：

```text
known expiry → active
unknown expiry but connected → pending_activation
```

不伪造历史服务期。

---

## Task 8：Activate 原子消费

`ActivateStoreService`：

```text
Auth LiveWrite
If-Match service_version
state = pending_activation
connection = connected
available renewal periods >= 1
```

同一 DB transaction：

```text
lock Operation
lock Store
lock Bucket
Consume 1
write Resource Event
update Store active + expiry
complete Operation
commit
```

不使用跨事务 Reserve→Commit。

---

## Task 9：Renew 原子消费

`RenewStoreService(N)`：

```text
state = active
N bounded positive
```

同一事务：

```text
Consume N
new_expiry = max(now,current_expiry)+N*30d
```

测试：same key replay、不同 key 并发、余额不足、expiry 不缩短。

---

## Task 10：Reactivate 原子消费

`ReactivateStoreService(N)`：

```text
state = expired
```

同一事务：

```text
Consume N
started_at = now
expiry = now + N*30d
state = active
```

Expired 调首次 Activate 必须拒绝。

---

## Task 11：API 与租户隔离

路由：

```text
POST /api/v1/workbench/stores/:store_id/activate
POST /api/v1/workbench/stores/:store_id/renew
POST /api/v1/workbench/stores/:store_id/reactivate
```

必须：

```text
AuthPolicyVerifiedIdentity
OrganizationAccessPolicyLiveWrite
Idempotency-Key
Request Fingerprint
If-Match
```

organization 只能来自 live effective org。

---

## Task 12：Migration Credit

历史资源导入：

```text
migration_credit → available +q
```

Migration Credit 不 source reverse。错误迁移通过 audited AdjustDebit/AdjustCredit 修正。

---

## Task 13：故障与并发测试

覆盖：

```text
Bucket 首次并发创建
Operation 幂等竞争
Event composite FK
Reservation double settlement
Commit/Release race
Consume compensation duplicate
Store version conflict
Activate double click
Renew concurrent
Reactivate concurrent
DB commit failure rollback Store + Resource
跨 org 同 operation_id
```

使用真实 DB 并发测试，不只 Mock。

---

## 完成定义

- [ ] Bucket `(organization_id, resource_type)` 唯一。
- [ ] Operation `(organization_id, operation_id)` 是幂等权威。
- [ ] Event 使用复合 FK。
- [ ] Reservation 有可锁定 settlement state。
- [ ] 不存在 aggregate balance 推断 Grant 未使用的 source reverse。
- [ ] Compensation 一次且可审计。
- [ ] Activate/Renew/Reactivate 资源扣减与 Store 更新同事务。
- [ ] 资源域与账号/ZITADEL 完全解耦。