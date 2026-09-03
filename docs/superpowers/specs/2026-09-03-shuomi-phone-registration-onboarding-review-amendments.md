# 硕米手机号注册与 Onboarding：评审修订

**状态：** 权威修订  
**适用范围：** 本文覆盖 `2026-09-03-shuomi-phone-registration-onboarding-design.md` 与对应 Implementation Plan 中与本文冲突的内容。

原则保持不变：ZITADEL/Login V2 继续负责 OTP、Password、Session、OIDC；task-processor 只负责手机号自助注册所需的短生命周期协调、业务 Onboarding 与最后业务授权，不建立第二套 IAM。

---

## 1. Consent 必须绑定真正的手机号所有权证明

`BeginRegistration` 时可以记录用户勾选了哪个当前协议版本，但该事实 **不是最终 Consent 权威**，也不能直接写入 `saas_account_consents`。

Registration Intent 调整为：

```text
registration_id
normalized_phone
provider_organization_id
provider_user_id
presented_policy_version
state
cleanup_state
expires_at
```

OTP Proof 成功后，Login Fork 必须把当前已验证 ZITADEL Session 的安全证明引用传给 Internal Registration API。后端校验：

```text
Session Proof user == provider_user_id
Session Proof phone/user factor 对应当前 registration intent
当前 policy_version == presented_policy_version；若已变化则要求 re-consent
```

只有在上述证明成立后，才由服务器生成：

```text
consent_accepted_at = server_now
saas_account_consents(zitadel_user_id, current_policy_version, accepted_at, source_registration_id)
```

因此：知道手机号但未持有 OTP Session 的第三方不能替目标用户制造 Consent；被另一个浏览器 adopt 的 pending Intent 也不会继承攻击者预先写入的接受时间。

---

## 2. Provider Create 后的 ownership 恢复规则

优先在 Provider Create 请求本身支持的字段中写入 ownership marker；若当前 ZITADEL API 无法把 registration metadata 与 Create 原子提交，则不得因为“metadata 缺失”直接判定 ownership mismatch。

安全 adopt 规则：

```text
Provider ID 必须等于 Intent 预分配 ID
Organization/User 的 resource owner / organization 必须等于 Intent 预期值
User canonical login name 必须等于 Intent.normalized_phone
对象创建时间必须落在 Intent 合理窗口
本地 Intent 必须仍拥有该 Provider ID，且没有其他 Intent 声明相同 ID
不存在任何 project authorization / business readiness 与该 pending ownership 冲突
```

满足全部条件但 registration metadata 缺失时，可执行 `adopt_missing_marker`：先补写 metadata，再继续；任一属性不一致则 quarantine。

必须覆盖：Provider 已提交但响应丢失、Create 后进程崩溃且 metadata 尚未写入、重复恢复。

---

## 3. Registration 生命周期终点由业务授权拥有

删除无法被可靠观测的独立 `completed` 状态。

权威状态机改为：

```text
intent_created
→ provider_provisioning
→ otp_pending
→ otp_verified
→ business_preparing
→ business_prepared
→ project_grant_ready
→ authorizing
→ authorized
```

`authorized` 是 Registration Intent 的业务终态，含义是：

```text
Consent 当前且已持久化
Business Projection ready
base_payg / existing subscription ready
Sumi Project Grant ready
listingkit_admin Authorization 已创建并 read-back 验证
```

后续 `CreateCallback` / Auth.js Session 完全属于 ZITADEL/Login V2/OIDC，不再反写 Registration Intent。

手机号明文的短期 retention 从 `authorized_at` 起计算；无需等待浏览器 callback ACK。

---

## 4. Project Grant 必须先于 User Authorization

复用项目现有 `internal/zitadelprovision` 的多 Organization 模式，不再重新发明 Project Grant 逻辑。

授权顺序固定为：

```text
OTP Proof
→ Consent + Business Prepare
→ EnsureExactProjectGrant(fixed Sumi project, target organization, allowed role set)
→ read-back Project Grant
→ EnsureProjectAuthorization(provider_user_id, target organization, listingkit_admin)
→ read-back Authorization
→ Intent = authorized
```

Registration Provisioning Adapter 只能操作固定 Sumi Project；不得接受浏览器或请求体传入任意 project/role。

如果已有 Project Grant 的 role set 超出或不匹配预期，fail closed 并进入人工修复，不静默扩大权限。

---

## 5. Cleanup 与 Onboarding/Authorization 必须串行化

仅在操作开始时检查 `cleanup_state` 不够。Registration Intent 增加 `transition_version`，所有关键状态变更都在 PostgreSQL 中以行锁/CAS 串行。

### 5.1 Cleanup Claim

Janitor：

```text
BEGIN
SELECT intent FOR UPDATE
要求 state ∈ {intent_created, provider_provisioning, otp_pending}
要求 cleanup_state = none
设置 cleanup_state = cleanup_requested
transition_version++
COMMIT
```

### 5.2 Onboarding Prepare

Prepare：

```text
BEGIN
SELECT intent FOR UPDATE
要求 cleanup_state = none
要求 state = otp_verified
设置 state = business_preparing
COMMIT
执行本地业务事务
再次锁 Intent
只有 cleanup_state = none 才能写 business_prepared
```

由于 Cleanup Claim 只允许 pre-OTP 状态，因此 OTP 已验证后 Janitor 不再竞争业务开通。

### 5.3 外部 Project Grant / Authorization 边界

外部调用前先原子把状态推进到 `project_grant_ready/authorizing`；Janitor 不允许 claim 这些状态。Provider 调用结果未知时只由 Provisioning Reconciler 继续，不由 Cleanup Worker 删除。

**结论：** cleanup 与 onboarding 不是“查两个系统后抢时间”，而是由本地 Intent 状态所有权决定谁有资格继续。

---

## 6. Quarantine 仍计入 Provider 资源上限

Provider Object 限额不能只统计 active pending Intent。

定义：

```text
unresolved_provider_objects =
  provider_provisioning
+ otp_pending
+ cleanup_requested
+ quarantined
```

只要 Provider Organization/User 尚未被确认删除，就继续计入：

```text
全局 unresolved provider object high-water cap
按 IP / 时间窗口的新建上限
Provisioning Worker 最大并发
```

达到 high-water cap 后 fail closed，停止创建新的 Provider Organization/User，并要求运维先清理 quarantine backlog。

只有 Provider Delete read-back 确认对象不存在后才能释放该容量。

---

## 7. Policy 版本必须在授权前再次校验

`presented_policy_version` 只代表用户开始注册时看到的版本。

在 OTP Proof 后创建 Consent 前，以及 Project Authorization 前，都必须比较服务器当前版本：

```text
intent.presented_policy_version == CurrentPolicyVersion
```

若不相等：

```text
state = consent_required
→ Login Fork 展示当前协议
→ 用户确认
→ 绑定已验证 Session 生成新的 server accepted_at
→ 写当前版本 Consent
→ 回到 business_preparing / business_prepared
```

旧版本 Consent 保留审计，不覆盖、不伪造。

---

## 8. Registration schema 独立迁移边界

Registration/Onboarding 表不再挂到 `internal/workbench/schema/runtime.go`。

新增独立 migration owner：

```text
internal/registrationprovisioning/schema/runtime.go
```

应用 migration composition 显式注册 `registrationprovisioning` scope。

该 scope 负责：

```text
saas_registration_intents
saas_account_consents
saas_onboarding_preparations / readiness projection
相关索引、约束与版本升级
```

Workbench migration 不创建、修改或回滚 Registration 表；Registration rollout 也不依赖 Workbench scope 被选择。

---

## 9. 实施计划追加任务

### Task A：Session-bound Consent

- 测试第三方仅知道手机号、未完成 OTP 时不能产生 Consent。
- 测试 pending Intent 被另一个浏览器 resume 后，Consent 的 `accepted_at` 来自该浏览器完成 OTP 后的服务器时间。
- 测试 policy 在 OTP 等待期间升级，旧版本不能授予角色。

### Task B：Provider adopt missing marker

- Create 成功响应丢失。
- Create 成功后 metadata 写入前崩溃。
- exact attributes 匹配时补 marker 并 adopt。
- 任意属性不匹配 quarantine。

### Task C：Project Grant + Authorization

- 复用 `internal/zitadelprovision`。
- Ensure Project Grant → read-back → Ensure User Authorization → read-back。
- 任意 project/role 注入拒绝。

### Task D：Cleanup serialization

- Cleanup 与 OTP Verify 并发。
- Cleanup 与 Business Prepare 并发。
- Cleanup 与 Project Authorization 并发。
- 验证每个状态只有一个合法 owner。

### Task E：Provider high-water cap

- quarantine 对象继续占用配额。
- 每个 TTL 周期持续随机手机号攻击不能无限增长 Provider Object。
- 删除 read-back 成功才释放容量。

### Task F：独立 migration scope

- Workbench-only migration 不触碰 Registration 表。
- Registration-only migration 可独立前滚/回滚。

---

## 10. 更新后的完成定义

- Consent 只能在 OTP 所有权证明后由服务器持久化。
- Registration Intent 的终态为 `authorized`，不依赖 OIDC callback ACK。
- Project Grant 必须先于 `listingkit_admin` User Authorization，并复用现有 provisioning 能力。
- Cleanup 与 Onboarding/Authorization 通过本地状态锁/CAS 串行，不发生“检查后删除”的跨系统竞态。
- quarantine Provider Object 在确认删除前始终计入资源上限。
- 当前 Policy Version 在 Consent 和 Role Grant 前都重新校验。
- Registration schema 拥有独立 migration scope。
- 不新增 PhoneIdentity、OTP/Password 校验器、自研 OIDC Callback Runtime 或登录 Temporal Workflow。
