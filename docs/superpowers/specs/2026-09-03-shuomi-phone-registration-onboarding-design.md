# 硕米手机号注册与 Onboarding 设计

**状态：** 待评审

**范围：** 只处理 ZITADEL 官方 Login V2 之外、硕米手机号自助注册和首次业务开通所必需的最小差异。

**不在本设计范围：** Console 视觉、企业成员邀请、资源钱包/资源账本、店铺激活/续费、支付、实名认证。

---

## 1. 设计目标

第一原则：**ZITADEL 是身份权威，不在 task-processor 再实现一套 IAM。**

```text
ZITADEL 官方 Login V2
├── Password / Reset
├── Session Cookie
├── OTP SMS Challenge / Verify
├── AuthRequest
└── CreateCallback / Redirect

ZITADEL
├── User / Phone / Login Name
├── Organization
├── Project Authorization
└── OIDC Token

硕米只补：
├── 手机号自助注册临时 Registration Intent
├── 默认 Organization/User 的受控 Provisioning Adapter
├── 当前协议接受证据
├── 新企业 base_payg 初始订阅
└── listingkit_admin 最后授予
```

禁止引入：

```text
PhoneIdentity 长期映射
Phone HMAC Alias / 长期手机号盲索引
业务侧 OTP 校验器
业务侧 Password 校验器
Password Decoy User
自研 OIDC callback delivery/ACK
登录 Temporal Workflow
通用 Account Entry Operation Runtime
```

Registration Intent 是短生命周期的注册协调记录，不是身份目录；长期手机号唯一性仍由 ZITADEL canonical login name 负责。

---

## 2. 组件边界

```text
Browser
  ↓
login.shuomi.example
ZITADEL Login V2 Fork
  ├── 官方 Session/OIDC/OTP/Password 能力
  └── 调用硕米 Internal Registration API
          ↓
task-processor Registration Provisioning Adapter
  ├── Registration Intent
  ├── ZITADEL Provisioning Client
  ├── Onboarding Prepare
  └── Cleanup / Reconciliation
          ↓
ZITADEL + PostgreSQL
```

### 2.1 Login Fork 持有的凭据

Login Fork 只允许持有：

```text
ZITADEL_LOGIN_CLIENT_TOKEN
→ 官方 Login V2 所需 Session/OIDC 权限

SHUOMI_REGISTRATION_CLIENT_TOKEN
→ 只允许调用 task-processor 的 Internal Registration API
```

**Login Fork 不持有 ZITADEL Organization/User 删除、Project Authorization 等广权限管理凭据。**

### 2.2 task-processor 持有的凭据

```text
ZITADEL_REGISTRATION_PROVISIONING_TOKEN
→ 仅由后端 Provisioning Adapter 使用
→ 创建/读取自助注册所需 Organization/User
→ 写 registration metadata
→ 为固定 Sumi Project 创建目标角色 Authorization

ZITADEL_REGISTRATION_CLEANUP_TOKEN（仅在能做最小权限约束时启用）
→ 仅后台 cleanup worker 使用
```

如果 ZITADEL 当前权限模型不能证明 cleanup token 无法删除无关活跃企业，**第一阶段不自动删除 Provider Organization/User，只 quarantine 并由离线受控运维清理**。不能为了自动清理把高危删除权限放进互联网 Login App。

---

## 3. Registration Intent：必须先持久化，再调用 Provider

第一次 ZITADEL Create 之前，服务器必须先持久化 Registration Intent。

```text
saas_registration_intents
- registration_id UUID PRIMARY KEY
- normalized_phone VARCHAR NOT NULL
- provider_organization_id VARCHAR NOT NULL
- provider_user_id VARCHAR NOT NULL
- policy_version VARCHAR NOT NULL
- consent_accepted_at TIMESTAMP NOT NULL
- state VARCHAR NOT NULL
- cleanup_state VARCHAR NOT NULL
- expires_at TIMESTAMP NOT NULL
- created_at
- updated_at
```

`normalized_phone` 只存在于短生命周期 Registration Intent 中：

- 统一 E.164；
- 不写应用日志、Tracing Attribute、审计 Payload；
- 数据库卷必须启用静态加密；
- Intent 到期并完成/清理后按保留策略删除或不可逆脱敏；
- 不把该列升级为长期用户目录或登录索引。

Intent 首次创建时就固定以下不可变输入：

```text
registration_id
normalized_phone
provider_organization_id
provider_user_id
policy_version
consent_accepted_at
```

后续阶段不得从浏览器重新接受另一组手机号或协议版本覆盖它们。

### 3.1 状态机

```text
intent_created
→ provider_provisioning
→ otp_pending
→ otp_verified
→ business_preparing
→ business_prepared
→ authorizing
→ authorized
→ completed

过期/异常：
intent_created/provider_provisioning/otp_pending
→ cleanup_requested
→ quarantined
→ deleted（仅安全证明成立时）
```

`cleanup_requested/quarantined` 与 `business_preparing/business_prepared/authorizing/authorized` 互斥。

---

## 4. Provider Identity 创建与恢复

### 4.1 稳定 Provider ID

实现前必须对当前部署 ZITADEL 版本做真实 API/生成类型验证：

```text
Organization 是否支持调用方提供 ID
Human User 是否支持调用方提供 ID
按 User ID / Organization ID 查询是否稳定
E.164 canonical login name 是否实例范围唯一
```

Stop condition：若当前版本不能支持稳定 ID + lookup/adopt，则先调整 ZITADEL API 用法/版本，不允许通过生成第二个 Organization/User 来“重试”。

### 4.2 Create/Get/Adopt

Provisioning Adapter 始终使用 Intent 中已持久化的 Provider IDs：

```text
Get(org_id)
├── exists + metadata/ownership 匹配 → adopt
├── not found → Create(same org_id)
└── exists 但 ownership 不匹配 → quarantine

Get(user_id)
├── exists + org/login-name/registration-id 匹配 → adopt
├── not found → Create(same user_id)
└── exists 但不匹配 → quarantine
```

Provider 响应超时或连接断开统一按 `outcome_unknown` 处理：先 Get 同一个 ID，不生成新 ID。

Provider Organization/User 创建后尽快写入：

```text
shuomi_registration_id
shuomi_registration_state=pending
shuomi_registration_expires_at
```

这些 metadata 只用于证明资源由哪个 Registration Intent 创建，不替代 task-processor Intent 状态。

---

## 5. 注册前资源耗尽保护

因为 ZITADEL OTP SMS Challenge 需要 User 已存在，所以未证明手机号前会产生 Pending Provider Object。必须在第一次 Provider Create 之前设置硬边界。

最低要求：

```text
Ingress/CDN：按可信 Client IP 限制 BeginRegistration
Registration API：同一手机号同一时刻最多一个 active intent
数据库：全局 active pending intent 上限
Provisioning Worker：最大并发 Provider Create 数
过期 Intent：进入 cleanup/quarantine，不无限累计
```

任何达到上限的请求在 Provider Create 前失败，不允许“短信稍后限流，但 Organization/User 已经无限创建”。

禁止信任浏览器自带 `X-Forwarded-For`；可信客户端 IP 必须由边缘代理覆盖。

---

## 6. `/register` 不得枚举既有账号

用户输入手机号后，服务器内部可以识别：

```text
existing active ZITADEL user
existing pending registration
new phone
```

但 OTP Proof 前用户可观察行为必须保持同类：

```text
同一验证码页面
同类状态码/文案
同类可用操作
无“该手机号已注册/正在注册”提示
```

分支只在服务器内部决定：

- existing：向已有 User 建立官方 OTP Session；
- pending：按 registration metadata/Intent adopt 后继续；
- new：按 Intent 稳定 IDs 创建 pending identity 后进入 OTP。

只有 OTP Proof 成功后才允许选择：

```text
existing → 普通登录完成
new/pending → 首次 Onboarding Prepare
```

若真实预发布测得三类时序差异可稳定枚举，只允许在 Login Fork 加有限响应时间下限；不引入 Decoy User。

---

## 7. OTP 与密码全部留在 ZITADEL

```text
手机号 OTP
→ Login V2 Session API
→ ZITADEL OTP SMS
→ ZITADEL Verify
→ Session Proof

手机号 Password / Reset
→ 官方 Login V2 Password / Reset Flow
```

`task-processor` 不接收：

```text
OTP code
password
password reset secret
ZITADEL session token（浏览器认证用途）
OIDC callback URL
```

如果当前 ZITADEL/Login V2 的 OTP 错误尝试上限、Lockout、Origin/Host 等安全能力低于最低要求，只在 ZITADEL Policy、Ingress 或 Login Fork 的最窄位置补齐。

---

## 8. Consent 与业务 Onboarding：角色授权必须最后发生

新手机号 OTP Proof 成功后执行：

```text
1. Registration Intent: otp_verified
2. task-processor Onboarding Prepare
3. 持久化当前协议 Consent
4. Ensure Business User Projection
5. Ensure Business Organization Projection
6. EnsureInitialPlanIfAbsent(base_payg)
7. readiness = business_prepared
8. 再创建/adopt listingkit_admin Project Authorization
9. Intent = authorized
10. 官方 Login V2 CreateCallback
11. Auth.js Session
```

**任何 `listingkit_admin` 授权都不得早于当前协议接受证据和 base_payg readiness。**

### 8.1 Consent 权威

```text
saas_account_consents
- zitadel_user_id
- policy_version
- accepted_at
- source_registration_id NULLABLE
- created_at
PRIMARY KEY (zitadel_user_id, policy_version)
```

首次注册的 `policy_version`、`accepted_at` 必须来自服务器端 Registration Intent，不接受 callback/browser 重新提交。

政策版本升级不修改旧 Registration Preparation：

```text
新 policy version
→ 新增一条 saas_account_consents
→ Workbench Consent Gate 检查当前版本
```

因此不会因为旧 Preparation fingerprint 导致用户永久 `repair_required`。

---

## 9. base_payg 必须复用现有 listingsubscription 权威

不得建立第二套套餐表。

现有 Catalog 只有 `basic/professional/enterprise`，因此实现必须在 `internal/listingsubscription` 中正式增加：

```text
PlanBasePayg = "base_payg"
Name = "基础方案 · 按需使用"
Active = true
```

最小 Module Bundle：

```text
ModuleStoreManagement
  store_count = 1
```

续费期数、AI 点数、数据条数属于后续企业资源钱包，不伪装成 `store_count` 或 import_tasks；第一阶段初始余额均为 0。

必须用现有 StoreQuota resolver 验证：`base_payg` 企业可以绑定第 1 家店，不能绑定第 2 家店。

### 9.1 不能“先查再 ApplyPlan”

新增 subscription-authority 原子入口，例如：

```go
EnsureInitialPlanIfAbsent(ctx, tenantID, PlanBasePayg, actorID)
```

数据库语义：

```text
tenant 无 subscription
→ 原子插入 base_payg + entitlements

tenant 已有任何 subscription
→ 返回 existing，不覆盖
```

不得：

```text
GetTenantSubscription
→ 空
→ 普通 ApplyPlan(base_payg)
```

因为并发的 professional/basic 赋值可能在两步之间被覆盖。

历史 `basic/professional/enterprise` 不自动迁移。

---

## 10. Workbench 不是发权益入口，只是 Readiness Gate

Workbench 路由要求：

```text
AuthPolicyVerifiedIdentity
OrganizationAccessPolicyLiveWrite
PermissionWorkbenchBootstrap
```

第一阶段 `PermissionWorkbenchBootstrap` 只映射给允许的 ZITADEL 项目角色（默认 `listingkit_admin`）。

Organization 必须来自 live resolved Effective Organization，不接受 body/query/header 中的任意 org ID。

Workbench Bootstrap 只做：

```text
读取 live authorization
读取本地 Business Projection
读取 subscription readiness
读取当前 consent 状态
返回 ready / consent_required / repair_required
```

它**不创建 base_payg、不授予 listingkit_admin、不创建 Organization**。

---

## 11. 现有用户迁移与灰度

不能在 `saas_onboarding_preparations` 为空时直接强制 Readiness Gate，否则历史用户会全部被锁死。

上线顺序：

```text
Phase A：部署 schema 和只读 readiness，Gate 关闭
Phase B：对历史 Organization 执行 reconciliation/backfill
Phase C：验证 live ZITADEL grant + existing subscription
Phase D：标记 legacy_ready
Phase E：开启 Workbench Readiness Gate
```

历史用户 Consent：

- 不伪造过去的协议接受记录；
- 初始可标记 `legacy_consent_unknown`，不因为新表为空立即锁死；
- 产品决定需要重新确认时，进入独立 Consent Gate；
- 接受后写当前 `saas_account_consents`；
- 新注册用户从第一天起必须有当前 Consent 才能获得 `listingkit_admin`。

Backfill 必须可重跑，且不会改变已有套餐。

---

## 12. Cleanup / Janitor：先 Claim，再隔离，再删除

Janitor 不能“查询两个系统后立刻 Delete”，因为查询和删除之间可能刚好完成 Onboarding。

本地状态机：

```text
expired pending intent
→ 原子 CAS: cleanup_state none → cleanup_requested
→ 进入 quarantine grace period
```

所有 Onboarding/Authorization 操作开始前必须检查：

```text
cleanup_state == none
```

如果已 `cleanup_requested/quarantined`，不得继续开通。

Janitor 在 grace period 后再次核验：

```text
本地 preparation 未 prepared
authorization 未创建
intent 仍 expired
registration metadata 仍属于同一 registration_id
```

只有全部成立且 cleanup credential 权限边界经测试证明安全，才允许 Provider Delete。

任何状态不确定：

```text
→ quarantined
→ 不自动删除
```

若 Provider 删除权限无法限制到自助注册资源，则自动删除功能关闭。

---

## 13. 服务凭据轮换

`SHUOMI_REGISTRATION_CLIENT_TOKEN` 不使用单值硬切。

Phase 1 使用简单 current/previous overlap：

```text
receiver accepts: current + previous
caller sends: current
```

轮换：

```text
1. receiver 加入 new，同时保留 old
2. caller 切 new
3. 确认所有 Login Fork Pod 已切换
4. receiver 删除 old
```

旧 token 保留时间不得短于最大滚动部署时间 + 请求重试窗口。

ZITADEL Provisioning Token 独立轮换；不得与 Login Session Token 共用。

---

## 14. 关键故障与并发验收

必须覆盖：

```text
Intent DB commit 后进程崩溃、Provider 尚未调用
Organization Create 成功但响应丢失
Organization 成功、User 尚未创建时进程崩溃
User Create 成功但 metadata 尚未写时进程崩溃
浏览器清 Cookie 后用同手机号重新进入并 adopt 原 Intent/Provider 资源
同手机号并发 BeginRegistration
全局 pending cap 到达后不再产生 Provider Create
OTP Proof 成功、Consent 写入前失败
Consent/Projection 成功、base_payg 失败
base_payg 成功、role grant 失败
role grant 成功、Intent 状态写回失败
Janitor 与 Onboarding 并发
Janitor 与 role grant 并发
历史用户 Gate 开启前 backfill
professional 与 base_payg 初始化并发，professional 不被覆盖
policy_version 升级后重新同意
Registration client token 滚动轮换不中断
```

---

## 15. 完成定义

- 身份、OTP、Password、Session、OIDC 仍由 ZITADEL/Login V2 负责。
- task-processor 只保存短期 Registration Intent 和长期硕米业务事实。
- Provider ID 在第一次 Create 前已服务端持久化。
- 手机号、协议版本与接受时间绑定在同一个 Intent，不允许浏览器二次替换。
- 未证明手机号不能无限创建 Provider Object。
- existing/pending/new 在 OTP Proof 前不可枚举。
- Consent 和 base_payg 成功后才允许授予 `listingkit_admin`。
- `base_payg` 使用现有 listingsubscription Catalog/Entitlement/StoreQuota 权威。
- 初始套餐使用数据库原子 apply-if-absent，不覆盖历史套餐。
- 历史用户有明确 backfill/grace 路径，不被新 Gate 锁死。
- Cleanup 与 Onboarding 有本地 Claim/Quarantine 序列化；无法安全删除时 fail closed 为 quarantine。
- 不新增第二套 IAM、手机号长期目录、密码系统或 OIDC 工作流。