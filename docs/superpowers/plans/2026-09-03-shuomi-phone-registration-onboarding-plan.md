# 硕米手机号注册与 Onboarding Implementation Plan

**目标：** 在不复制 ZITADEL IAM 能力的前提下，补齐手机号自助注册所需的稳定 Registration Intent、受控 Provisioning、Consent、`base_payg` 和最后角色授予。

**权威设计：** `docs/superpowers/specs/2026-09-03-shuomi-phone-registration-onboarding-design.md`

---

## Task 1：固定 ZITADEL/Login V2 能力证据

**输出：**

- `docs/verification/zitadel-registration-capabilities.md`
- 验证脚本

验证当前生产匹配版本：

```text
E.164 login name 唯一性
Organization caller-supplied ID
Human User caller-supplied ID
Get Organization/User by exact ID
User metadata
Session OTP SMS
Project Authorization
```

Stop condition：稳定 ID 或 lookup/adopt 不成立时，不进入自助注册实现。

---

## Task 2：建立短生命周期 Registration Intent

**建议 package：** `internal/registrationprovisioning`

**持久化：** 通过 `internal/workbench/schema/runtime.go` 管理 schema。

新增：

```text
saas_registration_intents
registration_id PK
normalized_phone
provider_organization_id
provider_user_id
policy_version
consent_accepted_at
state
cleanup_state
expires_at
created_at
updated_at
```

要求：

- BeginRegistration 在任何 ZITADEL Create 前提交 DB；
- Immutable fields 不允许 Update；
- 同手机号 active intent 有数据库唯一/互斥策略；
- 手机号不进入日志；
- Intent TTL/脱敏任务明确。

并发测试：两个实例、同手机号、不同请求同时 Begin，最多一个 active intent。

---

## Task 3：Registration Provisioning Adapter

后端持有 `ZITADEL_REGISTRATION_PROVISIONING_TOKEN`，Login Fork 不持有广权限 Provider Token。

实现窄接口：

```text
BeginRegistration
EnsureProviderIdentity
GetRegistrationStatus
MarkOTPVerified
PrepareBusiness
EnsureProjectAuthorization
```

Provider Create 规则：

```text
Get stable ID
→ adopt or create same ID
→ unknown outcome 先 Get
→ ownership mismatch quarantine
```

负向权限测试必须覆盖：

```text
不能通过客户端参数指定其他 org/user
不能授权非固定 Sumi Project
不能授权任意高权限角色
不能删除无关 Organization/User
```

若 Provider Token 无法表达安全删除边界，自动 Provider Delete 不实现。

---

## Task 4：Pending Object 防资源耗尽

在 Provider Create 前完成：

```text
Ingress trusted-IP rate limit
同手机号 active intent 上限
全局 active pending cap
Provisioning worker concurrency cap
```

测试大量随机手机号请求：达到 cap 后 Provider Organization/User 数量不再增长。

---

## Task 5：Login V2 Fork 手机号注册分支

Login Fork 只增加：

```text
手机号 + 协议 UI
调用 BeginRegistration
根据返回 registration_id 调 EnsureProviderIdentity
复用官方 Session/OTP SMS
OTP Proof 后调用 MarkOTPVerified/PrepareBusiness
最后继续官方 CreateCallback
```

Proof 前 existing/pending/new 使用相同 UI 状态。

不得加入：

```text
本地 OTP 表
Password 校验
callback persistence
PhoneIdentity 长期映射
```

---

## Task 6：把 `base_payg` 加入现有 listingsubscription

修改：

```text
internal/listingsubscription/types.go
internal/listingsubscription/service.go
internal/listingsubscription/service_test.go
internal/listingsubscription/gorm_repository.go / tests（按实现需要）
```

新增：

```go
PlanBasePayg = "base_payg"
```

DefaultPlans 至少包含：

```text
ModuleStoreManagement
store_count = 1
```

同时新增 subscription-authority 原子入口：

```go
EnsureInitialPlanIfAbsent(...)
```

验收：

```text
无订阅 tenant → base_payg
已有 basic/professional/enterprise → 原样保留
并发 professional + initial base_payg → professional 不被覆盖
base_payg StoreQuota → 第 1 家允许，第 2 家拒绝
```

---

## Task 7：Onboarding Prepare

新增业务表：

```text
saas_identity_users
saas_identity_organizations
saas_account_consents
saas_onboarding_preparations
```

Preparation 不用 `(user, org)` + immutable policy fingerprint 锁死未来 re-consent；建议以 `registration_id` 为初始注册操作身份，Consent 单独按 `(user, policy_version)` 版本化。

执行顺序：

```text
OTP verified
→ Consent
→ User Projection
→ Organization Projection
→ EnsureInitialPlanIfAbsent(base_payg)
→ preparation = prepared
```

任何一步可安全重放；只有 `prepared` 才允许进入 Role Grant。

---

## Task 8：`listingkit_admin` 最后授予

Project Authorization 只在：

```text
Intent = otp_verified/business_preparing
Preparation = prepared
cleanup_state = none
```

时执行。

采用 exact user/org/project/role from server-side intent/config；不接受浏览器覆盖。

Provider 响应未知：按 Authorization 查询/adopt，不创建不同目标授权。

成功后 Intent `authorizing → authorized`，再由 Login V2 继续官方 OIDC callback。

---

## Task 9：Workbench Readiness Gate

新增：

```text
PermissionWorkbenchBootstrap
OrganizationAccessPolicyLiveWrite
```

Bootstrap：

```text
确认 live effective org + live allowed role
检查本地 projection/subscription/consent
返回 ready / consent_required / repair_required
```

Bootstrap 不再创建套餐、Organization 或角色。

---

## Task 10：Legacy Backfill

Gate 默认关闭。

实现可重跑 reconciliation：

```text
扫描现有 Organization/Subscription
核对 live ZITADEL role
写 legacy readiness projection
不修改现有 plan
不伪造 consent
```

启用顺序：schema → backfill → 抽样核验 → Gate。

历史 consent 缺失使用 `legacy_consent_unknown`，后续独立 Consent Gate 收集，不因新表为空直接锁死。

---

## Task 11：Policy Re-consent

当前政策版本由服务器配置发布。

缺少当前 `(user, policy_version)` consent 时：

```text
Auth Session 保持有效
Workbench → consent_required
用户确认 → server write current consent
返回 Workbench
```

不能更新/覆盖旧 consent；不能复用初次 Registration Preparation fingerprint 阻止新政策版本。

---

## Task 12：Cleanup / Quarantine

Janitor 先在本地 CAS：

```text
cleanup_state: none → cleanup_requested
```

Onboarding/Authorization 所有写操作必须 fail closed 检查 cleanup_state。

等待 grace period 后再复核 Provider grant + 本地 preparation + metadata。

不确定状态进入 `quarantined`。

只有 cleanup token 权限边界经真实负向测试证明安全时才自动 Delete；否则生成运维清理任务，不自动删除。

---

## Task 13：内部服务凭据轮换

`SHUOMI_REGISTRATION_CLIENT_TOKEN` receiver 支持 current/previous overlap。

测试滚动顺序：

```text
receiver(new+old)
→ caller(new)
→ old caller drain
→ receiver(new only)
```

ZITADEL Login Token 与 Provisioning Token 分开轮换。

---

## Task 14：安全与故障矩阵

至少覆盖：

```text
DB Intent 已提交、Provider 未调用时崩溃
Org 已创建、User 未创建时崩溃
Provider 成功响应丢失
浏览器 Cookie 丢失后重新输入手机号
同手机号并发注册
随机手机号资源耗尽攻击
OTP verified 后每个业务步骤失败
base_payg 与付费套餐并发赋值
role grant 成功、状态写回失败
Janitor vs Prepare/Authorization 并发
历史用户迁移
policy version 升级
内部 token 滚动轮换
```

---

## 完成定义

- [ ] 没有 task-processor OTP/Password/OIDC 实现。
- [ ] Stable Provider IDs 在 Provider Write 前已服务端持久化。
- [ ] 手机号/协议事实绑定到 Registration Intent。
- [ ] Pending Provider Object 有硬资源上限。
- [ ] Proof 前不可枚举 existing/pending/new。
- [ ] `base_payg` 属于现有 listingsubscription，并有 `store_count=1`。
- [ ] Initial Plan 是 apply-if-absent，不覆盖历史 plan。
- [ ] Consent + plan readiness 早于 `listingkit_admin`。
- [ ] Workbench 只做 Live Authorization + Readiness Gate。
- [ ] Legacy 用户不会因新表为空被锁死。
- [ ] Cleanup 有 Claim/Quarantine，不跨系统裸检查后直接删除。
- [ ] 广权限 Provider Credential 不存在于互联网 Login App。