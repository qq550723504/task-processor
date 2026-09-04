# 硕米账号入口架构边界：ZITADEL Login V2 + Auth.js

**状态：** IMPLEMENTATION_READY / 冻结职责基线

> **登录决策更新（2026-09-05）：** 本文中与现有用户登录、专用 `otp_login` / `password_login` 和 Login V2 Fork 登录职责有关的 Phase1 决策，已由 `2026-09-05-shuomi-login-phase1-zitadel-native-simplification.md` 替代。手机号注册及其 Onboarding 边界不受该补充影响。

> 本文只定义账号入口的系统职责、公开入口路由和 Capability Gate，不再定义手机号注册的数据库状态机、Provider 恢复、Consent 事务、`base_payg` 初始化或 Cleanup 算法。上述详细设计以 **PR #283 V7** 为唯一实现基线。

---

## 1. 核心决策

账号入口不在 `task-processor` 中重建 IAM。

```text
ZITADEL 官方 Login V2 Fork
├── Login UI
├── Session Cookie
├── Password / Reset
├── OTP SMS
├── AuthRequest
└── CreateCallback
        ↓
ZITADEL
├── User / Phone / Password
├── Session / OTP Factor
├── Organization
├── Project Authorization
└── OIDC Token
        ↓
listingkit-ui
└── Auth.js ZITADEL Provider
        ↓
task-processor
└── 只处理硕米业务事实
```

第一阶段只对 Login V2 做两类修改：

```text
1. 按硕米 Figma 定制视觉
2. 增加手机号自助注册的最小入口/桥接
```

**新手机号产品化不能删除现有通用 Login V2 能力。** 历史 email-only、非手机号或未来其它由 ZITADEL 支持的身份仍可通过 generic Login V2 chooser 登录。

---

## 2. ZITADEL 负责什么

ZITADEL 是身份权威：

```text
用户 ID
手机号/login name
密码与密码策略
短信 OTP 因子与验证
Session
Organization
Project Authorization
OIDC/OAuth2 Token
```

`task-processor` 不保存第二套密码、密码哈希、OTP code 或登录 Session。

手机号注册为了满足 pinned Provider schema 使用的 opaque technical email（例如 `u-<opaque-id>@phone.invalid`）也是内部技术属性，不是用户拥有的 Email Identity。

---

## 3. Login V2 Fork 负责什么

Login Fork 是独立部署的认证前端，例如：

```text
login.shuomi.example
```

负责：

- 保留官方 Login V2 generic/default 登录能力；
- 复用官方 Login V2 Session/OIDC 结构；
- 手机号验证码登录；
- 手机号密码登录（Capability Gate 通过后开放）；
- Phone-only Password Reset（Capability Gate 通过后开放）；
- 手机号自助注册 UI；
- 硕米品牌视觉；
- 调用内部 Registration/Onboarding 能力完成首次业务开通桥接。

Login Fork 不成为第二个业务后端，不存企业权益、套餐余额或店铺数据。

---

## 4. listingkit-ui / Auth.js 负责什么

`listingkit-ui` 继续使用现有 Auth.js ZITADEL Provider：

```text
OIDC authorize
code exchange
access_token / id_token / refresh_token
Token refresh
业务浏览器 Session
```

`listingkit-ui` 不复制 Login V2 页面，不接收 ZITADEL 管理凭据，不自行验证 OTP/Password。

公开 `/login`、`/register`、`/forgot-password` 只负责选择受控认证 Entry 并进入标准 Login V2 / OIDC 流程。

受保护页面未登录时现有的 bare `/login?returnTo=...` 重定向继续合法，并进入 **generic Login V2 entry**；它不能被强制解释成手机号 OTP。这样现有 email-only 等历史用户不会因为新 Console/手机号入口上线失去登录能力。

---

## 5. task-processor 负责什么

身份认证完成后，`task-processor` 只管理硕米业务：

```text
业务用户/企业投影
套餐和企业权益
协议接受证据
店铺
资源账本
业务权限与审计
```

手机号自助注册所需的短期 Registration Intent、Provider Provisioning、首次 Consent/`base_payg`/体验 Grant/角色顺序只属于“首次业务开通”差异，不升级为通用 IAM 框架；详细设计见 **PR #283 V7**。

---

## 6. 明确不建设的自研轮子

账号入口第一阶段不建设：

```text
internal/accountentry 通用身份状态机
PhoneIdentity 长期表
FingerprintAlias / Phone HMAC 长期索引
业务侧 OTP 验证器
业务侧 Password 验证器
Password Decoy User / Decoy Pool
自研 OIDC Callback Delivery / ACK
登录 Temporal Workflow
登录专用通用 Operation Runtime
```

如果 ZITADEL 官方 Login V2 在某个具体安全点不足，先证明缺口，再在 ZITADEL Policy、Ingress 或 Login Fork 最窄位置补齐。

---

## 7. 手机号产品语义

```text
canonical login name = E.164 手机号
```

目标：

```text
一个手机号对应一个 ZITADEL 可登录身份
手机号可以走 OTP 登录
设置密码后可以走 Password 登录
```

唯一性事实以 ZITADEL 为准，不在 task-processor 建长期手机号身份目录。

**Account Existence 可以公开。** `/register` 可以明确提示“该手机号已注册，请直接登录”；不再为了隐藏 existing/new 构建 Decoy、branch-neutral capacity 或 timing indistinguishability 机制。

仍禁止未授权披露：姓名、真实邮箱、Organization、角色、Store、Factor 列表及其他账户/业务资料。

自助注册的并发、稳定 Provider ID、Pending 生命周期和 Provider Object 清理问题见 **PR #283 V7**。

---

## 8. 注册 UI 契约

注册页只包含：

```text
手机号
短信验证码
服务协议 + 隐私政策
注册并进入
已有账号？立即登录
```

不包含：

```text
用户名
展示名称
注册密码
邮箱
经营画像
```

Figma 注册基线：`374:325`，字段按上述产品口径裁剪。

手机号查询命中 existing active User 时，可以直接显示：

```text
该手机号已注册，请直接登录
```

并提供 generic Login V2、OTP 登录、Password 登录（若 capability enabled）和找回入口（若 capability enabled）。

---

## 9. 公开 Entry Routing Contract

公开入口固定为：

```text
/login
/register
/login?method=otp
/login?method=password
/forgot-password
```

服务器维护 allowlist 映射：

```text
/login                   -> generic_login
/register                -> registration
/login?method=otp        -> otp_login
/login?method=password   -> password_login
/forgot-password         -> password_reset
```

其中 `generic_login` 是官方 Login V2 默认/chooser 入口，必须保留 ZITADEL 当前允许的既有登录方式。它承担 protected-route 默认 redirect，不受手机号专用 capability gate 影响。

实现可以使用受控 authorize 参数或独立 Login Fork URL，但必须满足：

- bare `/login?returnTo=...` 进入 generic upstream Login V2，不强制 OTP；
- `listingkit-ui` 不丢弃明确的 `method` 后把 OTP/Password/Reset 全部落到 generic；
- 只有上述 allowlist entry 可以进入 Login Fork；
- 浏览器不能指定任意 action/handler/redirect URL；
- `returnTo` 仍走现有安全 normalization/allowlist；
- password/reset capability 关闭时，不渲染对应专用入口，直接请求也不会误入一个看似可用但实际无法完成的 flow；
- contract tests 必须逐一证明 5 个公开 URL 选择了正确 Entry；
- 必须覆盖 existing email-only historical user：protected route -> bare `/login?returnTo=...` -> generic Login V2 -> Auth.js authenticated session。

---

## 10. OTP → OIDC Capability Gate

手机号验证码登录是 Phase1 默认手机号登录方式，但**不能仅因为 Session API/OTP preflight 成功就视为完整登录已确认**。

上线前必须在 pinned ZITADEL/Login V2 staging 环境记录证据，证明：

```text
E.164 User
-> OTP SMS Factor/Challenge
-> VerifySMS
-> verified ZITADEL Session
-> OIDC Auth Request
-> CreateCallback
-> Auth.js code exchange
-> listingkit-ui authenticated session
```

还必须覆盖刷新/失败/错误 OTP 与正常 retry 的真实行为。

若该完整链路没有通过：

- 对应 OTP login / new-phone self-registration 保持 feature gated；
- generic Login V2 与现有非手机号身份登录不因此关闭；
- Console 其他不依赖新自注册的实现不因此停止；
- 不允许在 task-processor 自研另一套 OTP→OIDC 桥接来绕过 Gate。

---

## 11. Password Login Capability Gate

手机号密码登录只有在当前 pinned ZITADEL/Login V2 证明 E.164 canonical login 可可靠进入 Password Flow 后才开放。

未通过时：

```text
不展示手机号密码 Tab
/login?method=password -> feature unavailable / 安全回 generic 或 OTP
不新增业务侧 Password Flow
```

这不意味着删除 generic Login V2 中现有用户可能已经可用的其它 Provider-native 登录方式。

通过时仍完全使用 ZITADEL Password Policy、lockout 与 Session/OIDC。

---

## 12. Password Reset / Recovery Capability Gate

直接注册只要求手机号，不要求用户真实邮箱；technical `@phone.invalid` 地址不能用于 Recovery。

因此手机号产品中的“忘记密码/重置密码”只有在 staging 明确证明 **Phone-only / SMS ownership proof 可以完成 ZITADEL Password Reset** 时才开放。

Gate 必须证明：

- 不依赖 technical email 接收邮件；
- 手机号所有权验证后可重置当前 User 的密码；
- Reset 完成后旧/新 Session 行为符合 ZITADEL Policy；
- 错误手机号/错误验证码不会暴露其他账户资料；
- task-processor 不生成/保存 Reset Token。

若 Provider 当前能力不满足，第一阶段隐藏手机号专用 Forgot Password/Reset UI，而不是把 technical email 暴露给用户或新建自研 Reset Token 系统。Generic Login V2 自己对其它既有身份提供的合法 recovery 能力不因本 Gate 被删除。

---

## 13. Authenticated Password Management

“我的账户 → 设置/修改密码”是**用户自己的 IAM 安全动作**，不由企业 RBAC 控制，但也不能通过业务后端的广权限管理 Token 直接实现。

Phase1 只有在验证存在 Provider-native self-service / re-auth 路径后才开放：

```text
read password configured status
set initial password
change password after re-auth
```

未通过 Gate 时：

- 不伪造“已设置/未设置”状态；
- 不注册新的 task-processor 管理 API；
- 账户页隐藏动作或显示 capability unavailable。

---

## 14. Technical Email UI Mapping

`u-<opaque-id>@phone.invalid` 或等价 technical email：

```text
Provider internal technical field
!= user-owned email
```

所有 Account DTO/UI projection 必须将它视为：

```text
email = unset
emailVerified = false / not_applicable for user-facing purposes
```

即使 ZITADEL 为满足 User schema 把 technical email 标记为 verified，也不能：

- 在账户页显示该地址；
- 显示“邮箱已验证”；
- 提供“向该邮箱发送验证码/找回链接”；
- 把它作为通知地址。

以后用户主动绑定真实邮箱时，再以真实 User-owned email 替换用户侧状态。

---

## 15. 安全边界

第一阶段必须真实验证：

```text
OTP 发送冷却
OTP 错误尝试上限
Password lockout（若手机号 Password capability enabled）
Origin / Host / Trusted Domain
Session Cookie 属性
可信代理链
浏览器伪造 X-Forwarded-* 不改变安全判断
```

**不再要求手机号账户存在性不可枚举。** 注册入口允许明确返回 existing/new。安全目标是防 SMS 滥用、Account Takeover、Factor 未授权修改、跨租户访问和账户资料泄露，而不是隐藏“这个手机号有没有账号”。

安全补丁优先级：

```text
ZITADEL Policy
→ Ingress/CDN
→ Login V2 Fork
→ 最后才考虑业务后端
```

---

## 16. 企业和角色

ZITADEL Project Role 继续作为第一阶段粗粒度身份授权：

```text
listingkit_viewer
listingkit_operator
listingkit_admin
platform_admin
```

登录成功不等于可以访问任意企业。Workbench 敏感操作仍要求 live Effective Organization 和对应权限。

首次注册什么时候授予 `listingkit_admin`、如何保证 Consent/套餐/首次体验资源先完成，见 **PR #283 V7**。

---

## 17. Actions v2

ZITADEL Actions v2 第一阶段不是登录强依赖。

可以后续用于：

```text
User / Organization / Role 变化
→ 唤醒 task-processor 投影/缓存收敛
```

不用于：

```text
OTP 校验
Password 判定
维持浏览器 Login Flow
成为 base_payg 唯一初始化事务
```

---

## 18. 企业邀请

企业邀请不进入账号入口 Slice 1。

归属：

```text
企业空间
→ 成员管理
```

后续单独定义邀请 Token、角色白名单、LiveWrite Organization 和权限升级边界。

---

## 19. PR 分拆关系

```text
PR #281
→ 账号职责/Capability Gate、Console 产品契约、Figma

PR #283 V7
→ 手机号自助注册 + Registration Intent + Provider Provisioning + Onboarding

PR #284 V7
→ 企业资源账本 + Store Service
```

本文与 #283 冲突时，手机号注册/Onboarding 实现细节以 #283 V7 为准；#281 的已确认 Product Decision（例如允许 account existence disclosure、保留 generic existing-user login）必须同步到下游实现，历史 anti-enumeration 假设不再恢复。

---

## 20. Review Stop Rule

本文件只接受以下类型的 BLOCKER：

- advertised auth entry 无法实际进入目标 Login V2 flow；
- 新手机号入口让既有可登录身份失去 generic login path；
- mandatory OTP→OIDC 链路无法完成；
- 页面承诺 Password/Reset/Change 但没有经过验证的 Provider authority；
- technical internal identity 被误当成用户真实资料；
- 跨租户/Account Takeover/未授权 Factor mutation 风险。

Account Enumeration、时序等价、Decoy User、branch-neutral capacity 不属于当前 Phase1 blocker。

---

## 21. 完成定义

#281 在账号入口方面只要求：

- ZITADEL/Login V2/Auth.js/task-processor 职责边界明确；
- 注册/登录页面字段与 Figma 产品口径稳定；
- Account Existence 允许公开，但账户/组织/业务资料仍私密；
- bare `/login` 保留 generic Login V2，existing email-only 等历史身份不会因手机号产品化被锁死；
- `/login`、`/register`、OTP、Password、Reset 有明确 allowlisted Entry Routing；
- OTP→OIDC 有 staging Capability Gate；
- Password Login、Phone-only Reset、Authenticated Password Management 未验证前不展示为手机号产品可用功能；
- technical email 永不作为真实用户邮箱展示或恢复通道；
- 不再计划自研密码、OTP、Session、Callback 或长期手机号身份系统；
- 手机号注册的复杂可靠性问题已移动到 #283 V7 单独实现。

**原则：能由 ZITADEL 提供的身份能力不重复实现；新增手机号能力不能破坏既有身份登录；未验证的 IAM 能力宁可 feature-gate，也不通过业务后端补一套新的身份系统。**
