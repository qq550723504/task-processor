# 硕米账号入口 Slice 1 Implementation Plan（ZITADEL Login V2 + 窄业务开通）

> **For agentic workers:** Use `superpowers:subagent-driven-development` or `superpowers:executing-plans`. Every task is TDD-first and separately reviewable.

**Goal:** 不在 `task-processor` 重造 IAM。以与当前部署版本一致的 ZITADEL 官方 Login V2 为基座，身份认证、Session、Password、OTP、OIDC Callback 全部留在 ZITADEL；只为“手机号自助注册”和“认证后开通硕米业务”增加窄、可恢复、可审计的适配层。

**Authoritative design:**

- `docs/superpowers/specs/2026-09-03-shuomi-account-entry-zitadel-native-flow-design.md`
- `docs/superpowers/specs/2026-09-02-shuomi-console-phase1-hard-cut-design.md`（账号入口以新设计为准）

---

## 执行不变量

- PR #218 的 `zitadelsms` 与 `phoneonboardingpreflight` 是已验证资产，不复制成第二套生产登录框架。
- 不创建 `internal/accountentry` 通用身份状态机、PhoneIdentity/HMAC Alias、Password Decoy、Callback Delivery、Account Entry Temporal Workflow。
- 官方 Login V2 的 OIDC Proxy、Session Cookie、Server Actions、AuthRequest、Password/Reset、OTP、CreateCallback 保持上游结构。
- 手机号以 E.164 作为 canonical login name；身份唯一性由 ZITADEL 保证。
- `/register` 在 OTP Proof 前不得暴露手机号是 existing / pending / new。
- 新注册的 Provider Organization/User ID 在首次 ZITADEL Create 前固定；响应丢失只能 lookup/adopt 或使用同 ID 重试。
- `listingkit_admin` 是首次业务开通的最后一个效果；Consent、Business Projection、`base_payg` 未 prepared 前不得授予。
- `base_payg` 必须加入现有 `internal/listingsubscription` Catalog，并通过 `Service.ApplyPlan` 应用；禁止平行套餐表。
- Workbench Bootstrap 必须使用 live organization + admin permission，只验证 readiness，不因“已登录”发放权益。
- Login Runtime、Registration Provisioning、Sumi Onboarding 三套服务凭据独立、最小权限、独立轮换。
- 企业邀请不进入 Slice 1。

---

## Task 1：建立 ZITADEL Login V2 Fork 基线

**Primary target:** 独立 fork `zitadel/zitadel`，初始基线与当前部署 ZITADEL 版本一致（当前计划基线 `v4.17.1`），硕米改动优先集中在 `apps/login`。

**task-processor Files:**

- Create: `docs/runbooks/shuomi-zitadel-login-fork.md`
- Create: `docs/verification/2026-09-03-zitadel-login-v2-baseline.md`
- Modify: `deployments/kubernetes/zitadel/local/README.md`

### Step 1：原样构建官方 Login App

先不修改业务逻辑，验证：

```text
apps/login 从固定 upstream tag 构建
OIDC Proxy / Trusted Domain 可用
Session Cookie 可用
Password / Reset 可用
OTP SMS 页面可用
CreateCallback → 现有 Auth.js 可用
```

### Step 2：固定 upstream 同步规则

```text
upstream remote = zitadel/zitadel
每次升级先同步 upstream tag，再重放硕米小补丁
禁止复制 ZITADEL Session/OIDC Client 到 task-processor
```

### Step 3：提交

```powershell
git add docs/runbooks/shuomi-zitadel-login-fork.md `
        docs/verification/2026-09-03-zitadel-login-v2-baseline.md `
        deployments/kubernetes/zitadel/local/README.md
git commit -m "docs: define Shuomi ZITADEL Login V2 fork baseline"
```

**Stop condition:** 原版 Login V2 都不能稳定部署时，不开始手机号扩展。

---

## Task 2：锁定 ZITADEL 手机 login name、Caller-supplied ID 与查询能力

**task-processor Files:**

- Create: `docs/verification/2026-09-03-zitadel-phone-loginname.md`
- Create: `scripts/verify-zitadel-phone-loginname.ps1`

### Step 1：验证 E.164 login name

真实预发布验证：

```text
E.164 手机号可作为 login name
相同手机号并发创建最多一个可登录身份
login name 能进入 Password Flow
User 能进入 OTP SMS Session Flow
手机号修改后的 login name / Phone 一致性策略明确
```

### Step 2：验证稳定 Provider ID 合约

从固定 ZITADEL 版本的生成类型锁定：

```text
Organization 创建请求的 caller-supplied organization_id
User 创建请求的 caller-supplied user_id / organization_id
按 Organization ID / User ID 查询
按 login name 查询 User
删除本 Provisioner 创建的 pending Organization
设置 / 读取 Registration Metadata
查询 User 当前 Project Authorization
```

Characterization tests 必须覆盖：

```text
Create 成功但客户端断连 → 相同 ID Get 可 adopt
相同 ID 重试不会生成第二对象
对象存在但关联不匹配 → 明确 conflict/quarantine
```

### Step 3：禁止回退到本地身份索引

若 ZITADEL 无法满足手机号唯一性，优先调整 ZITADEL Policy/API 使用；不得新增 task-processor Phone HMAC Alias / Registration Claim。

---

## Task 3：拆分 Login Runtime 与 Registration Provisioning 凭据

**Login Fork / Deployment Files:**

- Login fork Secret / config
- Kubernetes Secret / Deployment
- Create: `docs/verification/2026-09-03-zitadel-login-credentials.md`

### Step 1：登录凭据

```text
ZITADEL_LOGIN_CLIENT_TOKEN
```

仅用于官方 Login V2 Session/OIDC 所需权限，例如当前版本的 `IAM_LOGIN_CLIENT`。

### Step 2：注册 Provisioning 凭据

```text
ZITADEL_REGISTRATION_PROVISIONING_TOKEN
```

仅用于手机号注册所需：

```text
Create/Get/Delete 自己创建的 pending Organization/User
Set/Get registration metadata
Get/Create 所需 Project Authorization
```

具体角色/permission 以固定版本 API Required Permission 为准；不得把 Login Client PAT 直接升级成全能管理 Token。

### Step 3：Sumi Onboarding 凭据

```text
SHUOMI_ONBOARDING_PROVISIONER_TOKEN
```

只允许 Login Fork 调用 task-processor：

```text
POST /api/v1/internal/onboarding/prepare
GET  /api/v1/internal/onboarding/readiness
```

### Step 4：负向权限测试

必须证明：

```text
Login Token 不能创建 Organization/User/Role
Registration Token 不能冒充浏览器 Session 完成 OIDC Login
Onboarding Token 不能访问 Store/Resource/Wallet API
浏览器请求没有任何一套服务 Token
```

---

## Task 4：在 Login Fork 实现可恢复的 Pending Registration

**Login Fork scope:** `apps/login/**`

### Step 1：BeginRegistration 不产生 Provider Side Effect

第一步 Server Action：

```text
输入手机号 + 协议 Checkbox
→ normalize E.164
→ generate registration_id
→ preallocate provider_organization_id
→ preallocate provider_user_id
→ set signed + Secure + HttpOnly short-lived Registration Intent Cookie
→ 返回验证码阶段
```

此步骤**不得调用 ZITADEL Create**。因此 Set-Cookie 响应丢失时可以安全重试。

Registration Intent 只能包含：

```text
registration_id
provider_organization_id
provider_user_id
expires_at
version / nonce
```

### Step 2：PrepareIdentity 使用稳定 ID Create / Adopt

第二步读取 Registration Intent 后才做 Provider Side Effect：

```text
Create/Get Organization(provider_organization_id)
Create/Get User(provider_user_id, provider_organization_id, E.164)
Set registration metadata = pending
Create OTP SMS Challenge
```

未知结果：

```text
超时 / 断连
→ 先 Get 同 ID
→ 匹配则 adopt
→ 明确不存在才同 ID 重试 Create
→ 不匹配进入 quarantine
```

### Step 3：Registration Metadata

至少记录：

```text
shuomi_registration_id
shuomi_registration_state = pending | active
shuomi_registration_expires_at
```

Metadata 写响应丢失时按同 User/Org ID 幂等重写。

### Step 4：丢 Cookie / 重入恢复

如果用户清 Cookie 后重新输入同手机号：

```text
服务端按 ZITADEL login name 查找 User
active → server-side existing 分支
pending + ownership marker → adopt pending
不存在 → 新建 pending
```

OTP Proof 前三种分支对浏览器保持同一页面与公开状态。

### Step 5：Registration Janitor

Login 部署增加一个窄范围 CronJob/Janitor，只处理：

```text
registration_state = pending
超过 TTL
由 Shuomi Registration Provisioner 创建
无 listingkit_admin / 其他项目授权
无 task-processor onboarding prepared
```

满足全部条件才按 Organization ID 删除整个 pending Organization；事实不确定则 quarantine，不猜测删除。

### Step 6：测试故障矩阵

```text
BeginRegistration 响应丢失
Organization Create 成功但响应丢失
User Create 成功但响应丢失
Metadata 写成功但响应丢失
用户中途关闭浏览器
Cookie 丢失后相同手机号重入
Janitor 不删除 prepared / active / 有角色的账号
```

---

## Task 5：实现手机号 OTP 注册 UI 与反枚举行为

**Login Fork scope:** `apps/login/**`

### Step 1：Figma 合约

注册页只包含：

```text
手机号
短信验证码
服务协议 + 隐私政策
注册并进入
已有账号？立即登录
```

不得出现用户名、注册密码、邮箱、经营画像。

### Step 2：Proof 前 existing / pending / new 不可观察

服务器内部可以分支，但公开流程固定：

```text
提交手机号
→ 同一验证码页面
→ 同类成功提示 / 错误格式
```

路径：

```text
existing active → 给已有 User 发 OTP Login Challenge
existing pending → adopt 后发/恢复 OTP Challenge
new → 创建 pending 后发 OTP Challenge
```

只有 OTP Proof 成功后才选择 existing login 或 first-time provisioning。

### Step 3：时序测试

对 existing / pending / new 做多轮统计测试，检查：

```text
页面
状态码
可用操作
错误文案
响应时间分布
```

若新用户创建带来明显可利用的时序差异，只在 Login Fork 增加有限响应时间下限；禁止 Decoy User。

### Step 4：OTP 继续走上游

验证码发送、验证、Session Proof、尝试次数和 Challenge 生命周期继续由 ZITADEL / Login V2 管理。

---

## Task 6：把 Consent 与 base_payg 放进窄 Onboarding Prepare

**Files:**

- Create: `internal/workbenchbootstrap/domain.go`
- Create: `internal/workbenchbootstrap/service.go`
- Create: `internal/workbenchbootstrap/service_test.go`
- Create: `internal/workbenchbootstrap/repository.go`
- Create: `internal/workbenchbootstrap/gorm_repository.go`
- Create: `internal/workbenchbootstrap/gorm_repository_test.go`
- Create: `internal/workbenchbootstrap/internalhttp/module.go`
- Create: `internal/workbenchbootstrap/internalhttp/handler.go`
- Create: `internal/workbenchbootstrap/internalhttp/handler_test.go`
- Modify: `internal/workbench/schema/runtime.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/httproute/descriptor.go`
- Modify: `internal/httproute/descriptor_test.go`
- Modify: `internal/listingsubscription/types.go`
- Modify: `internal/listingsubscription/service.go`
- Modify: `internal/listingsubscription/service_test.go`
- Modify: `internal/listingsubscription/gorm_repository_test.go`

### Step 1：把 base_payg 加入现有 Catalog

新增：

```go
const PlanBasePayg = "base_payg"
```

并把“基础方案 · 按需使用”加入 `DefaultPlans()`，让现有 `Service.ApplyPlan` 正式支持。

要求：

```text
basic / professional / enterprise 不删除、不自动迁移
新直接注册且无 subscription 的 org → ApplyPlan(base_payg)
已有 subscription → 不覆盖，返回 existing_subscription / migration_required
```

测试 `NewService()` 能 seed base_payg，`ApplyPlan(base_payg)` 成功，历史套餐仍保持原样。

### Step 2：业务准备表

```text
saas_onboarding_preparations
- zitadel_user_id
- zitadel_organization_id
- policy_version
- request_fingerprint
- state: preparing | prepared | failed
- prepared_at
- updated_at
PRIMARY KEY (zitadel_user_id, zitadel_organization_id)

saas_identity_users
- zitadel_user_id PRIMARY KEY
- created_at

saas_identity_organizations
- zitadel_organization_id PRIMARY KEY
- created_at

saas_account_consents
- zitadel_user_id
- policy_version
- accepted_at
- source
- created_at
UNIQUE (zitadel_user_id, policy_version)
```

### Step 3：Internal Service Auth

新增窄范围 Service Auth Policy，例如：

```text
AuthPolicyOnboardingProvisioner
```

只接受 `SHUOMI_ONBOARDING_PROVISIONER_TOKEN`；不接受浏览器 Auth.js Cookie、用户传入 Tenant Header 或任意 organization override。

请求的 user/org 来源于 Login Fork 服务端完成 OTP Proof 的 ZITADEL Session。

### Step 4：Prepare 收敛顺序

`PrepareBusinessAccess`：

```text
读取/锁定 onboarding preparation
同 user/org + 不同不可变 fingerprint → ONBOARDING_PREPARE_CONFLICT
EnsureBusinessUser
EnsureBusinessOrganization
EnsureConsent(policy_version, accepted_at)
检查当前 subscription
  无 subscription → listingsubscription.ApplyPlan(base_payg)
  已 base_payg → replay
  已其他历史 plan → 不覆盖，返回 migration_required
确认所有前置事实
state = prepared
```

`ApplyPlan` 本身是 Upsert 型 API；如果进程在套餐写完但 `prepared` 标记前崩溃，重试再次 Apply 同一 `base_payg` 并最终收敛，不创建平行套餐记录。

### Step 5：Readiness

```text
GET /api/v1/internal/onboarding/readiness
```

返回安全服务器事实：

```text
prepared
policy_version
plan_code
```

Login Fork 在任何 Prepare 响应丢失后先查 readiness，再决定是否继续 Project Authorization。

### Step 6：Consent 必须早于 Role

API 级测试：

```text
未写当前 consent → prepared=false
ApplyPlan 失败 → prepared=false
prepared=false 时 Login Fork 不允许授予 listingkit_admin
同一 consent 重放只有一条记录
```

---

## Task 7：最后授予 listingkit_admin，再走官方 OIDC Callback

**Login Fork scope:** `apps/login/**`

### Step 1：OTP Proof 后调用 Prepare

新 / pending 注册：

```text
OTP Proof
→ POST task-processor onboarding/prepare
→ GET readiness 确认 prepared
```

existing active 用户的普通登录不执行新企业 `base_payg` 初始化，也不覆盖历史 Subscription。

### Step 2：Project Authorization 是最后一个业务访问效果

只有 readiness 明确 `prepared=true` 后：

```text
Get current Project Authorization
已有 listingkit_admin → adopt
没有 → Create Authorization(listingkit_admin)
响应未知 → 先 Get 再同语义重试
```

授权成功后把 registration metadata 置为 active。

### Step 3：官方 OIDC 完成

继续使用 Login V2：

```text
CreateCallback
→ Redirect
→ Auth.js code exchange
```

不创建业务侧 callback delivery / ACK。

### Step 4：失败语义

```text
Prepare 失败 → 保持已认证 ZITADEL Session，但不授予业务角色，显示“正在准备工作空间 / 重试”
Role grant 失败 → prepared 事实保留，下次按 live authorization 查询后继续
CreateCallback 失败 → 交给 Login V2 官方流程恢复
```

---

## Task 8：保留 Auth.js OIDC，只增加 Workbench Live Authorization Gate

**Files:**

- Modify: `web/listingkit-ui/src/auth.config.ts`（只做 login domain / scope 必要适配）
- Modify: `web/listingkit-ui/src/auth.config.test.ts`
- Modify: `web/listingkit-ui/src/app/api/zitadel-auth/login/route.ts`
- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.ts`
- Modify: `web/listingkit-ui/src/proxy.ts`
- Create: `internal/workbenchbootstrap/httpapi/module.go`
- Create: `internal/workbenchbootstrap/httpapi/handler.go`
- Create: `internal/workbenchbootstrap/httpapi/handler_test.go`
- Modify: `internal/authz/listingkit.go`
- Modify: `internal/authz/listingkit_test.go`
- Create: `web/listingkit-ui/src/app/workbench/bootstrap/page.tsx`
- Create: `web/listingkit-ui/src/lib/api/workbench-bootstrap.ts`

### Step 1：Token 所有权不变

Auth.js 继续负责 authorize、code exchange、access/id/refresh token 和 refresh。

### Step 2：Bootstrap 后端 Route 是授权权威

Route 固定：

```text
AuthPolicy = VerifiedIdentity
OrganizationAccessPolicy = LiveWrite
Permission = workbench.bootstrap（第一阶段映射 listingkit_admin）
```

要求：

```text
organization_id 只来自 live resolved Effective Organization
body/query/header 中的任意 org override 都忽略或拒绝
live grant 没有 listingkit_admin → 403，不执行任何 Ensure/Benefit
```

`platform_admin` 只有在现有受控代管上下文中才能通过，不产生隐式跨租户权限。

### Step 3：Bootstrap 只验证 readiness

```text
查询 onboarding prepared
查询 current consent
查询 current subscription
Ensure 允许的无权益业务投影修复
返回 ready / repair_required
```

**不得**在这里因为“用户已登录”直接 `ApplyPlan(base_payg)`。

### Step 4：Proxy 只是 UX

前端 proxy 可以根据 token roles 做早期导航，但后端 LiveWrite + Permission 才是最终授权边界。

---

## Task 9：按 Figma 定制 Login V2 UI

**Login Fork Files:** `apps/login/**`

实现：

```text
硕米 Logo
左侧品牌区 / AI 光场背景
手机号验证码登录
手机号密码登录
忘记密码
手机号注册
返回官网
响应式布局
```

Password Flow 直接调用上游。如果 Task 2 证明手机号 login name 不能可靠进入官方 Password Flow，第一阶段隐藏密码登录 Tab，不实现 Decoy User。

可访问性：

```text
320px 无横向滚动
完整 label / aria-describedby
键盘可完成流程
reduced-motion
错误不暴露 Provider body/token
```

---

## Task 10：安全、防刷和故障只验证上游 / 边缘具体缺口

**Files:**

- Create: `docs/verification/2026-09-03-shuomi-login-security.md`
- Create: `scripts/verify-shuomi-login-security.ps1`
- Modify: Kubernetes / Ingress（仅验证证明需要）

真实预发布验证：

```text
OTP 发送冷却
单 Challenge 错误尝试限制
Password Lockout
Origin / Host / Trusted Domain
同站点恶意子域
可信代理链和 X-Forwarded-* 清理
Registration Intent Cookie 属性 / TTL / 篡改拒绝
CreateCallback 刷新/返回/断连体验
Registration Janitor ownership-safe cleanup
```

补丁优先级：

```text
ZITADEL Policy
→ Ingress / CDN
→ Login V2 Fork
→ task-processor 仅业务边界
```

禁止重新引入 Phone HMAC Alias、Decoy User、Account Entry Redis Device Cookie、Account Entry Operation Runtime。

---

## Task 11：部署、灰度与最终验收

**Files:**

- Modify: `deployments/kubernetes/zitadel/local/README.md`
- Create: `docs/runbooks/shuomi-login-rollout.md`
- Create: `docs/verification/2026-09-03-shuomi-login-release.md`

### Step 1：Secret 契约

```text
ZITADEL_LOGIN_CLIENT_TOKEN
ZITADEL_REGISTRATION_PROVISIONING_TOKEN
SHUOMI_ONBOARDING_PROVISIONER_TOKEN
```

三套独立轮换，不再需要旧账号入口：

```text
PHONE_HMAC_KEYRING
FLOW_AEAD_KEYRING
DEVICE_SIGNING_KEYRING
OPERATION_FINGERPRINT_KEYRING
PASSWORD_DECOY_USER
```

### Step 2：灰度顺序

```text
1. staging 原版 Login V2
2. 凭据隔离验证
3. E.164 / stable ID / lookup-adopt 预检
4. pending registration + Janitor
5. OTP existing/new 反枚举
6. onboarding prepare + base_payg
7. listingkit_admin 最后授权
8. Auth.js / live Bootstrap
9. Figma UI
10. 真实设备全链路
11. 生产灰度
```

### Step 3：最终验收

```text
相同手机号并发/响应丢失不产生第二可登录身份或第二默认企业
过期 pending 注册可以 ownership-safe 清理
已有 / pending / 新手机号在 Proof 前不可枚举
Consent、base_payg、Business Projection 未 prepared 前绝无 listingkit_admin
base_payg 由现有 listingsubscription ApplyPlan 提供，历史 plan 不被覆盖
Role grant 响应丢失可 live lookup/adopt
Workbench Bootstrap 无 live admin grant 时不能创建任何权益
OTP / Password / Origin / Host / Proxy 安全验证通过
没有 task-processor 自研 OTP/Password/PhoneIdentity/Callback 状态机
```

---

## Slice 1 完成定义

- [ ] Login V2 fork 可独立跟踪 upstream。
- [ ] Login / Registration / Onboarding 三类凭据最小权限隔离。
- [ ] E.164 login name 与 caller-supplied IDs 真实验证。
- [ ] Pending Registration 有 stable ID、resume、TTL、Janitor。
- [ ] Proof 前 existing / pending / new 不可枚举。
- [ ] OTP / Password / Callback 继续使用 ZITADEL 官方流程。
- [ ] `PlanBasePayg` 已进入现有 `listingsubscription` Catalog。
- [ ] Consent + base_payg + business projection 先 prepared，`listingkit_admin` 最后授予。
- [ ] Workbench Bootstrap 使用 LiveWrite + admin permission，只验证 readiness。
- [ ] 企业邀请已移出 Slice 1。
- [ ] 不存在 `internal/accountentry` 第二套 IAM。

原则：**身份问题优先交给 ZITADEL；硕米只实现 Provider 不提供的手机号注册薄适配，以及认证后的业务开通。**