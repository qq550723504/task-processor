# 硕米 Resource Ledger / Store Service 第四轮评审修订

**适用 PR：** #284

**覆盖关系：** 本文在冲突处覆盖前序 Resource Ledger / Store Service 设计与评审修订。

**边界不变：** 本文只处理企业资源余额与 Store Service；不引入账号注册、ZITADEL、Onboarding 或支付能力。

---

## 1. 所有资源 Command 的业务 Quantity 必须正数且有上界

不能只对 Reservation 做 `q > 0`。

所有以业务 quantity 为输入的命令统一在创建 Operation 前调用领域校验：

```text
Grant
MigrationCredit
AdjustCredit
AdjustDebit
Consume
Reserve
Commit/Release（quantity 来自 Reservation，不允许客户端覆盖）
Store renewal-period Consume
```

要求：

```text
q > 0
q <= configured_domain_max
```

数据库中持久化业务 quantity 的列均增加 `CHECK (quantity > 0)`；事件的正负 delta 由 event type 内部推导，客户端不得提交 signed delta。

测试覆盖每个 sibling command 的 zero、negative、overflow/超上界输入。

---

## 2. Settlement Fingerprint 必须绑定动作和 Reservation

Commit/Release 的 canonical fingerprint 固定包含：

```text
organization_id
operation_type = reservation_commit | reservation_release
reservation_id
resource_type
reservation_quantity
owner_type + owner_id（如存在）
任何行为性 precondition
```

同一个 operation key：

- 从 Release 改 Commit -> conflict；
- 从 Reservation A 改 Reservation B -> conflict；
- quantity / owner scope 变化 -> conflict。

先 fingerprint-check durable settlement Operation，再判断 Reservation 当前状态。

---

## 3. Resource Debt 必须 Tenant + Resource Scoped

Debt 记录至少使用：

```text
organization_id
resource_type
source_correction_id
amount_remaining
created_at
updated_at
```

数据库约束要求 source correction 与 debt 的 `organization_id + resource_type` 一致；`ApplyPositiveAvailableDelta` 必须在锁定对应企业资源 Bucket + Debt 后，只偿还同一 `(organization_id, resource_type)` 的 debt。

任何跨企业/跨资源 debt association 由数据库或事务内硬校验拒绝。

---

## 4. Migration Source Claim 与 Credit 必须一个事务提交

`saas_resource_migration_claims` 不是独立先提交的 admission 表。

同一个 PostgreSQL 事务内：

```text
1. lock/create global source claim
2. validate stable source fingerprint + organization ownership
3. lock/create tenant Operation
4. mutate Bucket
5. append MigrationCredit Event
6. mark Operation succeeded
7. mark Source Claim applied + operation reference
8. commit
```

事务失败全部回滚。

可选状态：

```text
pending | applied | reconciliation_required
```

但 `pending` 与 Credit Operation 必须由同一事务创建；重启时若 claim=applied，必须 read-back Operation；若 ownership changed，进入 reconciliation，不能静默补第二笔 Credit。

---

## 5. Reservation Owner 必须与 Reservation Tenant/Business Scope 一致

Reservation owner reference 不只是 `(owner_type, owner_id)`。

至少保存/验证：

```text
organization_id
owner_type
owner_id
owner_business_type / operation scope
```

Recovery 在决定 Commit/Release 前锁定 Owner Intent，并要求：

```text
owner.organization_id == reservation.organization_id
owner business/resource purpose matches reservation purpose
```

能用数据库复合 FK 的 Owner 类型优先使用复合 FK；不能统一 FK 的 polymorphic owner 必须通过 transaction-scoped owner resolver 做 tenant/business identity 校验。

跨企业 owner 永远不能驱动另一企业 Reservation settlement。

---

## 6. Store Service Migration 必须先 Fence Legacy Writers

不能在旧 Pod 仍只写 `lifecycle_status` 时直接 backfill `service_status/record_status`。

固定 rollout：

```text
Phase 0
部署 Compatibility Dual-Write：
- disable 同时写 legacy disabled + service suspended
- delete begin 同时写 legacy deleting + record deleting
- soft delete 同时写 deleted_at + record deleted
- 不启用新 Activate/Renew/Reactivate

Phase 1
等待所有不支持 Dual-Write 的旧 Pod drain / 被版本 fence 拒绝写

Phase 2
执行 service/record backfill

Phase 3
开启新 read contract + lifecycle routes
```

可通过 deployment generation / minimum writer version fence 拒绝旧实例写入；不得依赖“rolling deployment 应该很快”。

测试覆盖：backfill 与 legacy disable/delete 并发窗口。

---

## 7. Store Service Correction 对 Source 必须幂等且唯一

`CorrectStoreServiceOperation` 自身也是一个 Operation，并必须绑定原 lifecycle source。

第一阶段规则：每个 source lifecycle operation 最多一个成功 correction：

```text
UNIQUE (organization_id, source_lifecycle_operation_id, correction_kind)
```

Correction Operation fingerprint 至少包括：

```text
organization_id
source_lifecycle_operation_id
store_id
correction_kind
periods / expiry correction parameters
expected aggregate version
```

并发/重试：

- same correction key + same fingerprint -> replay immutable correction result；
- different key 纠正同 source -> 唯一约束/锁判定 already_corrected；
- correction 不能重复返还同一 consumed periods。

---

## 8. Existing Delete Path 同步 record_status

现有 Delete Service 也属于 hard-cut compatibility 范围。

```text
BeginDelete
-> 同事务/同并发权威写 legacy lifecycle=deleting + record_status=deleting

SoftDelete success
-> 写 deleted_at + record_status=deleted
```

删除中断时保持 `record_status=deleting`，因此 Activate/Renew/Reactivate 均 fail closed；恢复 Delete 完成后进入 deleted。

Delete compatibility 写必须在 Phase 0 Dual-Write 中先上线，再执行 backfill。

---

## 9. Service Resume 使用独立 Route Namespace

不得复用现有 Store Create Resume 路由。

预留未来服务恢复路由：

```text
POST /api/workbench/stores/{storeId}/service/resume
```

要求：

```text
AuthPolicyVerifiedIdentity
OrganizationAccessPolicyLiveWrite
PermissionWorkbenchStoreLifecycle
```

现有 `/resume` / ResumeCreate 保持 Store 创建恢复语义，不承担 service suspension resume。

第一阶段如果不实现 service resume，则该路由仅保留在设计中，不注册空 handler。

---

## 10. Expiry Materializer 必须使用与 Lifecycle 相同的并发 Authority

Materializer 不是正确性来源，且不能覆盖刚完成的 Reactivate/Renew。

更新必须为锁定或原子条件更新：

```text
WHERE record_status = 'active'
  AND persisted_service_status = 'active'
  AND service_expires_at <= now()
  AND version = expected_version
```

成功时推进 aggregate version；若条件不成立/版本已变，直接 skip 并重新读取，不写 stale expired。

测试：Materializer select old expiry -> Reactivate writes future expiry/version -> Materializer stale update 必须 0 rows affected。

---

## 11. Store Service Correction 必须进入强制事务审计

`CorrectStoreServiceOperation` 与 Activate/Renew/Reactivate/Disable 一样属于强制 Store Audit。

共享 `StoreResourceUnitOfWork` 同事务提交：

```text
Correction Operation
Resource Event(s)
Store expiry/status correction
immutable correction response snapshot
Store Audit / Audit Outbox
```

审计至少记录：subject、home/effective org、store、source lifecycle operation、correction kind、resource delta、result、request/operation id、time。

Audit 写失败 -> 整个 Correction 回滚。

---

## 12. 本轮新增验收矩阵

```text
Grant/Adjust/Consume zero/negative/too-large rejected before Operation
same settlement key cannot switch Commit<->Release or reservation
resource debt cannot cross organization/resource
migration claim crash cannot strand uncredited applied source
migration source ownership change requires reconciliation
reservation owner cross-tenant association rejected
old writer is fenced before backfill; dual-write window tested
same lifecycle source cannot be corrected twice
BeginDelete/SoftDelete synchronize record_status
service resume route does not collide with ResumeCreate and requires lifecycle permission
stale expiry materializer cannot overwrite Reactivate/Renew
correction audit failure rolls back Store + Resource + Operation
```
