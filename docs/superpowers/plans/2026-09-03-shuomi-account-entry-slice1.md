# 硕米账号入口 Slice 1 Implementation Plan（ZITADEL Login V2 轻量定制）

> **For agentic workers:** Use `superpowers:subagent-driven-development` or `superpowers:executing-plans`. Every task is TDD-first and separately reviewable.

**Goal:** 不在 `task-processor` 重造 IAM。以与当前部署版本一致的 ZITADEL 官方 Login V2 为基座，只增加硕米 Figma 视觉、手机号注册/验证码入口和少量业务衔接；身份认证、Session、密码、OTP、OIDC Callback 全部继续由 ZITADEL Login V2 / ZITADEL API 负责。

**Authoritative design:**

- `docs/superpowers/specs/2026-09-03-shuomi-account-entry-zitadel-native-flow-design.md`
- `docs/superpowers/specs/2026-09-02-shuomi-console-phase1-hard-cut-design.md`（账号入口以新设计为准）

---

## 执行不变量

- PR #218 的 `zitadelsms` 与 `phoneonboardingpreflight` 是已验证能力，不重写；生产登录也不把它复制进 `internal/accountentry`。
- 第一阶段不创建 `internal/accountentry`、PhoneIdentity/HMAC Alias、Password Decoy、Callback Delivery、Account Entry Temporal Workflow。
- 官方 Login V2 的 OIDC Proxy、Session Cookie、Server Actions、AuthRequest、Password/Reset、OTP、CreateCallback 流程保持上游结构。
- 手机号以 E.164 作为硕米 canonical login name；唯一性优先由 ZITADEL Policy/用户名唯一性保证。
- `listingkit-ui` 继续使用现有 Auth.js ZITADEL Provider，不承载自定义登录状态机。
- `task-processor` 只做登录后的业务 Bootstrap：业务用户/企业投影、协议投影、`base_payg`。
- 企业邀请不进入 Slice 1，移到企业空间/成员管理。
- ZITADEL Actions v2 第一阶段可关闭，不作为登录依赖。
- 如果官方 Login V2 无法支持某个需求，先证明具体缺口，再做最小补丁；禁止“为了保险”预先复制框架。

---

## Task 1：建立 ZITADEL Login V2 Fork 基线

**Primary target:** 独立 fork `zitadel/zitadel`，只维护 `apps/login` 差异；初始基线必须与当前部署的 ZITADEL 版本一致（当前计划基线 `v4.17.1`）。

**task-processor Files:**

- Create: `docs/runbooks/shuomi-zitadel-login-fork.md`
- Create: `docs/verification/2026-09-03-zitadel-login-v2-baseline.md`
- Modify: `deployments/kubernetes/zitadel/local/README.md`

### Step 1：原样构建官方 Login App

在 fork 上先不修改任何业务逻辑，验证：

```text
apps/login 能从固定 upstream tag 构建
OIDC Proxy 可用
Session Cookie 可用
Password 登录可用
Password Reset 可用
OTP SMS 页面可用
CreateCallback → Auth.js 可用
```

### Step 2：记录 upstream 同步规则

Runbook 固定：

```text
upstream remote = zitadel/zitadel
硕米改动只允许集中在 apps/login 和必要部署文件
每次升级先 rebase/merge upstream tag，再重放硕米小补丁
禁止复制 ZITADEL Session/OIDC Client 到 task-processor
```

### Step 3：提交 task-processor 侧基线文档

```powershell
git add docs/runbooks/shuomi-zitadel-login-fork.md `
        docs/verification/2026-09-03-zitadel-login-v2-baseline.md `
        deployments/kubernetes/zitadel/local/README.md
git commit -m "docs: define Shuomi ZITADEL Login V2 fork baseline"
```

**Stop condition:** 官方 Login V2 原样无法在当前 ZITADEL 版本部署时先解决版本/部署问题，不开始自定义手机号注册。

---

## Task 2：验证手机号作为 canonical login name

**task-processor Files:**

- Create: `docs/verification/2026-09-03-zitadel-phone-loginname.md`
- Create: `scripts/verify-zitadel-phone-loginname.ps1`

**Login Fork Files:** 只修改测试/验证脚本，不改生产逻辑。

### Step 1：验证 ZITADEL Policy

真实预发布环境验证：

```text
E.164 手机号能作为 login name
同一手机号不能创建第二个可登录身份
手机号 login name 能进入 Password Flow
手机号对应 User 能进入 OTP SMS Session Flow
手机号修改后的 login name / phone 一致性策略明确
```

### Step 2：并发测试

同时提交两个相同 E.164 手机号的创建请求，要求 ZITADEL 自身只产生一个可登录身份；若不能满足，优先调整 ZITADEL Policy/创建 API 使用方式。

**禁止方案：** 为此新增 task-processor Phone HMAC Alias / Registration Claim。

### Step 3：提交验证证据

```powershell
git add docs/verification/2026-09-03-zitadel-phone-loginname.md `
        scripts/verify-zitadel-phone-loginname.ps1
git commit -m "test: verify phone loginname invariants in ZITADEL"
```

---

## Task 3：在 Login V2 Fork 中实现硕米手机号注册最小扩展

**Login Fork scope:**

```text
apps/login
├── 硕米注册页面
├── E.164 手机输入组件
├── 手机号注册 Server Action
├── OTP SMS 验证页面复用/适配
└── 协议 Checkbox
```

### Step 1：Figma 注册页合约

注册页只包含：

```text
手机号
短信验证码
服务协议 + 隐私政策
注册并进入
已有账号？立即登录
```

不得出现：

```text
用户名
注册密码
邮箱
经营画像
```

### Step 2：注册身份创建

优先使用 ZITADEL v4.17.1 已支持的组合/调用方 ID 能力创建默认 Organization + Human User，减少跨请求半完成窗口。

实现前必须从当前版本生成类型确认：

```text
Organization 创建请求是否可内嵌 human admin
Organization ID 是否可由调用方提供
Human User ID 是否可由调用方提供
Phone / Loginname 字段如何表达
```

只有确认的字段才进入实现。

### Step 3：OTP 继续走上游 Session 流

复用官方 Login V2 Session Cookie / Server Actions 和 ZITADEL OTP SMS API。

禁止：

```text
业务数据库验证码表
task-processor OTP Verify API
自定义 callback URL 持久化
自定义 Flow Cookie
```

### Step 4：协议接受写入 ZITADEL Metadata

OTP Proof 成功后写：

```text
shuomi_terms_version
shuomi_terms_accepted_at
```

注册请求缺少当前协议版本或未勾选时，Login App 不推进注册。

### Step 5：项目授权

OTP Proof 后、OIDC callback 前，通过 ZITADEL API 确保当前 User 获得当前默认 Organization 的 `listingkit_admin` 项目授权。

这是 ZITADEL 身份授权，不在 task-processor 建第二套角色。

### Step 6：测试

```text
新手机号注册成功
重复手机号走既有身份逻辑，不产生第二 User
并发相同手机号只产生一个可登录身份
错误验证码由 ZITADEL 拒绝
刷新/返回仍由官方 Session Cookie 恢复
OIDC callback 使用官方流程
协议未勾选无法完成注册
```

---

## Task 4：按 Figma 定制 Login V2 UI，而不是复制到 listingkit-ui

**Login Fork Files:** `apps/login/**`

### Step 1：共享视觉层

实现：

```text
硕米 Logo
左侧品牌区
AI 光场背景
登录/注册卡片
手机号输入
验证码输入
密码输入
返回官网
响应式布局
```

### Step 2：登录方式

目标 UI：

```text
手机号验证码登录
手机号密码登录
忘记密码
```

Password Flow 直接调用上游实现。

如果 Task 2 证明手机号 login name 不能可靠进入官方 Password Flow，则第一阶段隐藏密码登录 Tab，不实现 Decoy User 或自定义密码认证。

### Step 3：可访问性

```text
320px 无横向滚动
完整 label / aria-describedby
键盘可完成流程
reduced-motion 关闭非必要动画
错误不暴露敏感 Provider 内容
```

---

## Task 5：保留现有 Auth.js OIDC 集成，仅调整入口域名

**Files:**

- Modify: `web/listingkit-ui/src/auth.config.ts`（只在必要时更新 issuer/login domain 适配）
- Modify: `web/listingkit-ui/src/auth.config.test.ts`
- Modify: `web/listingkit-ui/src/app/api/zitadel-auth/login/route.ts`
- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.ts`
- Modify: `web/listingkit-ui/src/proxy.ts`

### Step 1：不改变 Token 所有权

继续由 Auth.js：

```text
OIDC authorize
code exchange
access_token / id_token / refresh_token
Token refresh
Session identity claims
```

不把 Login V2 Session Token 暴露给 listingkit-ui。

### Step 2：入口切换

`/login` 和 `/register` 最终进入 ZITADEL Login V2 fork 域名/authorize flow，而不是渲染第二套本地登录表单。

### Step 3：回归

```powershell
Set-Location web/listingkit-ui
pnpm test -- src/auth.config.test.ts src/lib/server/zitadel-auth.test.ts src/proxy.test.ts
pnpm lint
pnpm typecheck
pnpm build
Set-Location ../..
```

---

## Task 6：实现登录后 Workbench Bootstrap

**Files:**

- Create: `internal/workbenchbootstrap/domain.go`
- Create: `internal/workbenchbootstrap/service.go`
- Create: `internal/workbenchbootstrap/service_test.go`
- Create: `internal/workbenchbootstrap/repository.go`
- Create: `internal/workbenchbootstrap/gorm_repository.go`
- Create: `internal/workbenchbootstrap/gorm_repository_test.go`
- Create: `internal/workbenchbootstrap/httpapi/module.go`
- Create: `internal/workbenchbootstrap/httpapi/handler.go`
- Create: `internal/workbenchbootstrap/httpapi/handler_test.go`
- Modify: `internal/workbench/schema/runtime.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Create: `web/listingkit-ui/src/app/workbench/bootstrap/page.tsx`
- Create: `web/listingkit-ui/src/lib/api/workbench-bootstrap.ts`

### Step 1：业务投影表

建议：

```text
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
- projected_at
- UNIQUE(zitadel_user_id, policy_version)
```

### Step 2：Ensure，而不是 Saga

```go
EnsureBusinessUser(...)
EnsureBusinessOrganization(...)
EnsureCurrentConsentProjection(...)
EnsureBasePayg(...)
```

所有身份 ID 从已验证 Auth.js/ZITADEL Token Context 取得，客户端不能指定任意 user/org。

### Step 3：唯一约束

```text
一个 ZITADEL User 只有一个业务投影
一个 ZITADEL Organization 只有一个业务投影
一个企业只有一个 base_payg
一个用户同一协议版本只有一条接受记录
```

重复 Bootstrap 返回同一 ready 状态。

### Step 4：失败语义

```text
身份认证成功，但业务初始化失败
→ 保留登录 Session
→ /workbench/bootstrap 显示正在初始化/重试
→ 不进入业务 Shell
```

不回滚 ZITADEL 用户或 OIDC Session。

---

## Task 7：安全与防刷只验证上游，缺什么补什么

**Files:**

- Create: `docs/verification/2026-09-03-shuomi-login-security.md`
- Create: `scripts/verify-shuomi-login-security.ps1`
- Modify: Kubernetes/Ingress 配置（仅当验证证明需要）

### Step 1：验证上游行为

真实预发布检查：

```text
OTP 发送冷却
单 Challenge 错误尝试限制
Password 失败次数与 Lockout
Login App Origin / Host 防护
Trusted Domain
浏览器伪造 X-Forwarded-For 不影响可信客户端 IP
官方 Session Cookie 属性
CreateCallback 响应丢失/刷新/返回的用户体验
```

### Step 2：补丁原则

若某项不满足，只在最窄层补齐：

```text
优先 ZITADEL Policy
其次 Ingress / CDN
再次 Login V2 fork
最后才考虑 task-processor
```

禁止重新引入：

```text
Phone HMAC Alias
Decoy User
Account Entry Redis Device Cookie
Account Entry Operation Runtime
```

---

## Task 8：部署、回滚与最终验收

**Files:**

- Modify: `deployments/kubernetes/zitadel/local/README.md`
- Create: `docs/runbooks/shuomi-login-rollout.md`
- Create: `docs/verification/2026-09-03-shuomi-login-release.md`

### Step 1：部署契约

```text
Login fork 独立 Deployment / Host
ZITADEL Trusted Domain
官方所需 Login Service Account / PAT
listingkit-ui 继续只持有 OIDC Client 配置
```

不再需要旧计划中的：

```text
ACCOUNT_ENTRY_BFF_TOKEN
PHONE_HMAC_KEYRING
FLOW_AEAD_KEYRING
DEVICE_SIGNING_KEYRING
OPERATION_FINGERPRINT_KEYRING
PASSWORD_DECOY_USER
ACCOUNT_ENTRY_REDIS_FLOW
```

### Step 2：灰度顺序

```text
1. staging 原版 Login V2
2. staging 硕米 UI
3. staging 手机 OTP 登录
4. staging 新手机号注册
5. staging 手机 Password / Reset
6. Workbench Bootstrap
7. 真实设备全链路
8. 生产灰度
```

### Step 3：最终验证

```text
手机号注册：一个手机号只产生一个可登录身份
OTP 登录成功
Password 登录成功；若上游未满足则 UI 不展示
Password Reset 成功；若暂缓则不展示假入口
Auth.js Token refresh 正常
base_payg 重复 Bootstrap 不重复创建
协议版本有服务器端持久证据
登录安全验证全部通过
没有 task-processor 自研 OTP/Password/PhoneIdentity/Callback 状态机
```

---

## Slice 1 完成定义

- [ ] ZITADEL 官方 Login V2 fork 能独立跟踪 upstream。
- [ ] 硕米登录/注册视觉在 Login fork 中完成，不复制到 `listingkit-ui`。
- [ ] 手机号 E.164 唯一登录名由 ZITADEL 验证并约束。
- [ ] 手机 OTP 注册与登录均使用 ZITADEL Session/OTP。
- [ ] Password/Reset 使用官方流程；不能可靠支持时隐藏入口而不是自研替代。
- [ ] OIDC CreateCallback、Session Cookie 和浏览器恢复继续使用官方 Login App。
- [ ] `listingkit-ui` 继续只消费标准 OIDC/Auth.js 会话。
- [ ] `task-processor` 只做登录后幂等业务 Bootstrap。
- [ ] 企业邀请已从 Slice 1 移出。
- [ ] Actions v2 不作为登录强依赖。
- [ ] 不存在 `internal/accountentry` 第二套身份框架。

原则：**身份问题优先交给 ZITADEL；task-processor 只实现硕米业务本身。**