# 硕米 Resource Ledger / Store Service 第五轮评审修订

**适用 PR：** #284

**覆盖关系：** 本文在冲突处覆盖前序 Resource Ledger / Store Service 设计与评审修订。

**边界不变：** 只处理企业资源余额与 Store Service，不引入账号、ZITADEL、Onboarding 或支付域。

---

## 1. 每个 Lifecycle Source 第一阶段最多一次成功 Correction

第四轮的 `(organization_id, source_lifecycle_operation_id, correction_kind)` 唯一键过宽。第一阶段不支持一个 source 被多个 correction kind 分段纠正。

改为：

```text
UNIQUE (organization_id, source_lifecycle_operation_id)
```

`correction_kind` 仍进入 Correction Operation fingerprint 和审计，但不能绕过 source-level 唯一性。

规则：

```text
source 未纠正
-> 允许一个 correction Operation 执行

same correction Operation + same fingerprint
-> replay immutable result

任何不同 Operation / correction_kind 再引用同 source
-> STORE_SERVICE_ALREADY_CORRECTED
```

如果未来需要 partial correction，必须引入 `corrected_quantity / remaining_correctable_quantity` 的 source-authoritative 模型后单独设计，第一阶段不做。

---

## 2. Browser Proxy / Client 必须显式支持三类 Lifecycle Action

新后端路由上线前，`web/listingkit-ui` 必须同步增加：

```text
POST .../stores/{storeId}/activate
POST .../stores/{storeId}/renew
POST .../stores/{storeId}/reactivate
```

### 2.1 Workbench Proxy allowlist

`workbench-proxy.ts` 明确加入三条 action；不能靠通配或把未知 action 透传。

### 2.2 Request contract

```text
activate:
- body 按实际 contract（无业务字段则只接受空对象）
- required Idempotency-Key
- required If-Match aggregate version

renew/reactivate:
- periods: positive bounded integer
- required Idempotency-Key
- required If-Match aggregate version
```

Proxy 只转发明确允许的 header：

```text
Idempotency-Key
If-Match
content-type
内部认证/组织上下文由 BFF 自己生成
```

不得接受浏览器伪造 organization / internal authorization header。

### 2.3 Frontend client + errors

`workbench-stores.ts` 增加 `activateStore/renewStore/reactivateStore`，并为：

```text
409 idempotency conflict
409/412 version conflict
409 invalid service state
402/409 insufficient renewal period
503 connection unavailable/stale
```

建立严格错误 schema。

必须有 proxy allowlist、body validation、header forwarding、unknown action rejection 和 client contract tests。

---

## 3. `reconciliation_required` Reservation 是可收敛状态，不是死状态

Reservation state machine 更新为：

```text
reserved
├─ normal Commit   -> committed
├─ normal Release  -> released
└─ ambiguous owner -> reconciliation_required

reconciliation_required
├─ reconciler + terminal success proof  -> committed
└─ reconciler + terminal non-consume proof -> released
```

限制：

- 普通 Commit/Release API 仍不能把任意 `reconciliation_required` 当成 `reserved`；
- 只有 Reservation Reconciler 持有 `reconciliation_lease/epoch` 并锁定 Reservation + Owner Intent 后可以执行 fenced settlement；
- 必须先 read-back durable terminal Owner state；仍为 `processing/outcome_unknown` 时保持 `reconciliation_required`；
- settlement Operation 仍遵循单一 settlement uniqueness、fingerprint 和 immutable result replay；
- Reconciler Commit/Release 与普通 settlement 并发时只能一个终态成功。

新增 restart / concurrent owner-terminalization 测试。

---

## 4. Correction Target 必须由 Source Operation Snapshot 授权

Correction 请求不能只“携带 source id + target store”。执行前在共享事务中锁定 source Lifecycle Operation 及 immutable result snapshot。

必须满足：

```text
source.organization_id == request.organization_id
source.result_store_id == request.store_id
source.resource_type == store_renewal_period
source.operation_type in {activate, renew, reactivate}
source.status == succeeded
source.result_resource_delta < 0
requested_refund_quantity > 0
requested_refund_quantity <= abs(source.result_resource_delta)
```

并使用 source snapshot 的原始：

```text
service_started_at
service_expires_at
aggregate_version_after
resource_delta
```

推导可允许的 Store 侧纠正；请求不得覆盖这些 source facts。

任一 Store / resource / org / delta mismatch -> `CORRECTION_SOURCE_MISMATCH`，事务不修改 Store/Bucket/Event。

第一阶段每 source 最多一次 correction，因此 correction 成功后 source-level unique claim 同时封闭后续退款。

---

## 5. Resource BIGINT 在 HTTP/JSON 使用十进制字符串

PostgreSQL `BIGINT` 资源值不能直接作为 JavaScript Number 传输。

所有资源数量/余额类响应字段统一采用 decimal-string wire format：

```json
{
  "quantity": "1",
  "available": "9007199254740993",
  "reserved": "0",
  "consumed": "123",
  "debt": "0",
  "resourceDelta": "-1",
  "resourceBalanceAfter": "9007199254740992"
}
```

要求：

- Go DTO 负责 `int64 -> base10 string`；
- immutable Operation response snapshot 也保存/重放精确 decimal representation；
- Zod 使用 `/^-?[0-9]+$/` 或更窄的非负 schema，不用 `z.number()`；
- 前端展示可保留 string；需要运算时显式 `BigInt(value)`，不得隐式 `Number(value)`；
- 请求中的 `periods` 若产品上限明确小于 JS safe integer，可继续用受限 integer；资源余额/事件 delta 仍用 decimal string。

Contract test 必须覆盖 `2^53-1`、`2^53`、`2^53+1`。

---

## 6. Resource Event 必须绑定同 Tenant / Resource 的 Reservation

包含 `reservation_id` 的 settlement/recovery Event 必须有 reservation scope 约束。

推荐 schema：

```text
Reservation UNIQUE (organization_id, reservation_id, resource_type)

Event FK:
(organization_id, reservation_id, resource_type)
  -> Reservation(organization_id, reservation_id, resource_type)
```

如果部分 Event 没有 Reservation，`reservation_id` nullable；非空时 composite FK 必须成立。

此外 settlement transaction 必须锁该 Reservation 并验证 Event `resource_type` 与 Bucket/Reservation 相同。

测试：

```text
A 企业 Event -> B 企业 Reservation: DB reject
AI point Event -> renewal Reservation: DB reject
nonexistent Reservation: DB reject
same org/resource Reservation: allowed
```

---

## 7. Hard Cut 之后的 Rollback 是 Feature Rollback，不是 Binary Rollback

一旦新 lifecycle route 已写过 `record_status/service_status`，不得回滚到不理解这些字段的 pre-dual-write binary。

固定最低兼容版本：

```text
minimum_store_service_writer_version = 首个 dual-write + new-state-aware 版本
```

Rollback 流程：

```text
1. 关闭 activate/renew/reactivate feature flags / route admission
2. 保持 dual-write + new-state-aware read compatibility
3. 停止非必要 materializer/new background worker
4. 保留 schema、record_status/service_status、Operation/Event/Audit 数据
5. 只能部署 >= minimum compatible writer/read version 的上一稳定 build
6. 修复后重新开启 feature
```

Writer fence 必须拒绝更老 binary 写入，不能为了回滚解除 fence。

Rollback 验收至少覆盖已经发生：

```text
Activate
Renew/expiry
Disable -> suspended
BeginDelete/deleted
```

回滚后这些 Store 的 authoritative service/record state 必须仍被正确读取和保护，不能重新由 legacy lifecycle 单独决定。
