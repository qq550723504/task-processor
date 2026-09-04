# 硕米 Resource Ledger / Store Service：评审修订

**状态：** 权威修订  
**适用范围：** 本文覆盖 `2026-09-03-shuomi-resource-ledger-store-design.md` 与对应 Implementation Plan 中与本文冲突的内容。

本修订只处理 Resource Ledger 与 Store Service，不触碰 ZITADEL、手机号注册或 Onboarding。

---

## 1. Reservation 使用明确生命周期与恢复责任

异步 Reserve 只用于真正跨事务、需要“先占用后结算”的业务。Reservation 增加：

```text
reservation_id UUID PRIMARY KEY
organization_id
resource_type
quantity BIGINT CHECK (quantity > 0)
reserve_operation_id
settlement_operation_id NULL
status reserved|committed|released|expired
owner_type
owner_id
expires_at
created_at
updated_at
```

数据库约束：

```text
FOREIGN KEY (organization_id, reserve_operation_id)
  REFERENCES resource_operations(organization_id, operation_id)

FOREIGN KEY (organization_id, settlement_operation_id)
  REFERENCES resource_operations(organization_id, operation_id)
```

`settlement_operation_id` 为空时允许 NULL；非空时必须满足复合 FK。

Reserve 前必须验证 `quantity > 0`，数据库再用 CHECK 二次防护。

### 1.1 Abandoned Reservation Recovery

每个 Reservation 必须有明确 owner 与 expiry：

```text
owner_type + owner_id
expires_at
```

Recovery Worker 周期扫描：

```text
status = reserved AND expires_at <= now
→ 锁 Reservation
→ 再次确认 owner 不处于可继续提交的 live 状态
→ 以确定性 release operation id 执行 Release
→ reservation reserved → released
```

确定性 Recovery Operation ID：

```text
reservation-recovery:{reservation_id}
```

同一 Reservation 的 crash/restart/cancel 恢复可无限重跑但只释放一次。

对于长期运行 Job，需要 owner 在到期前续租 Reservation；续租必须受上限约束，不能无限延长无主 Reservation。

---

## 2. Store 生命周期命令使用共享 Unit of Work

Activate/Renew/Reactivate 要求 Resource 与 Store 在同一 PostgreSQL Transaction 中提交，因此不能顺序调用两个独立 Repository 的 standalone Save。

新增显式 Unit of Work 边界，例如：

```go
type StoreResourceUnitOfWork interface {
    WithinTransaction(ctx context.Context, fn func(StoreResourceTx) error) error
}

type StoreResourceTx interface {
    LockOperation(...)
    LockStore(...)
    LockBucket(...)
    AppendResourceEvent(...)
    SaveStoreService(...)
}
```

实现必须由共享 `*gorm.DB` transaction 注入 tx-bound Store/Resource repositories；禁止在事务闭包内退回独立 `db.Transaction` 或 standalone Save。

### 2.1 原子 Store 命令

```text
BEGIN
取得/校验 Operation
锁 Store
锁 renewal-period Bucket
校验状态/连接/权限/If-Match
available -= N
consumed += N
写 Resource Event
更新 Store service_started_at/service_expires_at/service_status/service_version
完成 Operation
COMMIT
```

任一写失败整事务回滚；不存在“资源已扣但 Store 未改”的可见中间态。

---

## 3. Active → Expired 的权威语义

不依赖一个容易漏跑的定时器把 `active` 行永久改成 `expired`。

权威 Effective Service Status 在读取和命令前计算：

```text
if persisted_status = active
and service_expires_at <= now
then effective_status = expired
else effective_status = persisted_status
```

所有 Service Gate、Get/List DTO、Renew/Reactivate 前置校验都使用 `effective_status`。

可选后台 materializer 可以把已过期 active 行更新为 `expired` 以方便查询，但它不是正确性的唯一来源；materializer 必须幂等、可重启。

因此：

```text
Renew: effective_status 必须 active
Reactivate: effective_status 必须 expired
```

即使 scheduler 停止，过期 Store 也不会继续被当成 active。

---

## 4. Connection Status 必须有生产权威来源

当前 `unavailableConnectionStatusProvider` 不能作为 Activate 的生产前置条件来源。

实现计划必须新增 concrete `ConnectionStatusProvider`：

```text
Store connection credential / provider binding
→ 查询或验证当前连接健康状态
→ 返回 connected|error|disconnected + observed_at
```

要求：

- 成功连接返回 connected；
- 凭据 revoked 返回 disconnected/error；
- timeout 返回 unavailable/error，不能按 connected 放行；
- 连接快照有最大 freshness，过期必须重新验证；
- 浏览器不能传 `connection_status=connected` 覆盖后端。

Activate 在进入短事务前取得 connection snapshot，事务提交前校验 Store aggregate/version 未变化且 snapshot 未过期。

---

## 5. Disabled Store 迁移为 Suspended

现有 Store Center 的 `disabled` 生命周期不能丢失。

迁移规则优先级：

```text
existing disabled
→ service_status = suspended
→ 保留 disabled/suspension reason

否则 known future expiry
→ effective active

否则 known past expiry
→ effective expired

否则 connected + expiry unknown
→ pending_activation
```

`suspended` 明确拒绝：

```text
activate
renew
reactivate
```

必须通过独立 resume 策略恢复，不能因为迁移后连接仍是 connected 就重新激活。

---

## 6. API 权限与并发版本合同

Lifecycle Routes 必须同时要求：

```text
AuthPolicyVerifiedIdentity
OrganizationAccessPolicyLiveWrite
PermissionWorkbenchStoreLifecycle
If-Match
Idempotency-Key / Operation ID
```

权限矩阵沿用 Store Center 既有行为：viewer 禁止；operator/admin 是否允许以现有 `PermissionWorkbenchStoreLifecycle` policy 为准并通过测试锁定。

### 6.1 If-Match 统一使用可见版本

第一阶段不再要求客户端猜一个未暴露的 `service_version`。

优先选择：**使用现有 Store aggregate `version` 作为 If-Match 权威。**

原因：当前 Store list/get 已暴露 aggregate version，生命周期更新本身也会更新 aggregate version。这样无需引入第二个前端不可见并发版本。

如果实现阶段证明必须单独维护 service_version，则必须先：

```text
Get/List DTO 暴露 serviceVersion
Mutation Response 返回新 serviceVersion
前端 strict schema 接收 serviceVersion
```

在此之前，路由合同统一写 aggregate version。

---

## 7. Migration Credit 必须 restart-idempotent

迁移额度的 Operation ID 由稳定源记录派生，而不是 batch/run UUID：

```text
migration-credit:{source_table}:{source_primary_key}:{resource_type}
```

Request Fingerprint 至少包含：

```text
organization_id
resource_type
quantity
source_table
source_primary_key
source_version / canonical source payload hash
```

语义：

- partial run 重启 → 同 Operation ID、同 fingerprint，重放不重复入账；
- 同源记录被修改 → 同 Operation ID、不同 fingerprint，返回 migration conflict，要求显式人工/新迁移版本处理；
- 不允许通过新的随机 Operation ID 给同一源记录重复 Credit。

---

## 8. Resource Operation 与 Reservation 完整外键

Operation 主键保持：

```text
PRIMARY KEY (organization_id, operation_id)
```

Event：

```text
FOREIGN KEY (organization_id, operation_id)
```

Reservation：

```text
FOREIGN KEY (organization_id, reserve_operation_id)
FOREIGN KEY (organization_id, settlement_operation_id)
```

其中 nullable settlement FK 使用数据库支持的标准 nullable composite FK；当 settlement_operation_id 非空时必须匹配同一 organization。

必须测试 A 企业 Reservation/Event 无法引用 B 企业同名 operation_id。

---

## 9. 实施计划追加任务

### Task A：Reservation Recovery

- `CHECK(quantity > 0)`。
- owner/expiry/lease。
- crash after Reserve。
- cancel after Reserve。
- worker restart sweep。
- deterministic recovery release operation。
- Commit 与 Recovery Release 并发最多一个 settlement 成功。

### Task B：Shared Unit of Work

- 新增 tx-bound Store/Resource repository。
- 在故障注入下证明 resource update 与 Store update 同事务。
- crash/DB error 不产生半完成结果。

### Task C：Effective Expiry

- read-time effective status。
- scheduler/materializer 完全停止时仍能 Reactivate 已过期 Store。
- Renew 不得作用于过期 Store。

### Task D：Connection Provider

- successful / revoked / timeout / stale snapshot。
- 替换 production `unavailableConnectionStatusProvider`。

### Task E：Migration

- disabled → suspended。
- unknown expiry → pending_activation。
- deterministic migration credit replay。

### Task F：HTTP Contract

- PermissionWorkbenchStoreLifecycle。
- aggregate `version` If-Match。
- viewer/operator/admin 权限测试。
- response loss + replay tests。

---

## 10. 更新后的完成定义

- Reservation quantity 永远为正，并且每个 Reservation 有 owner、expiry 与单一 settlement。
- Abandoned Reservation 可被幂等 Recovery，不会永久冻结企业余额。
- Reservation/Event 对 Operation 的引用均使用 organization-scoped composite FK。
- Activate/Renew/Reactivate 的 Resource 和 Store 修改由同一个 Unit of Work/数据库事务拥有。
- Effective expiry 不依赖 scheduler 才正确。
- disabled Store 迁移为 suspended，不能直接进入激活路径。
- Activate 有真实 Connection Status Authority。
- Lifecycle Routes 有明确 `PermissionWorkbenchStoreLifecycle`。
- If-Match 使用客户端可获得的版本合同。
- Migration Credit 重启不会重复入账。
- 本切片不引入 ZITADEL/账号注册/Onboarding 依赖。
