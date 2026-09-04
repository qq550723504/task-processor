# 硕米登录 Phase1：ZITADEL 原生能力收敛决策

**状态：** APPROVED / IMPLEMENTATION_READY

**适用范围：** 仅替代以下冻结基线中与“现有用户登录”有关的决策；不修改手机号自助注册、Provider Provisioning、Consent/Business Bootstrap、Organization、Store Center 或 Resource Ledger 的既有范围与状态机：

- `2026-09-02-shuomi-console-phase1-hard-cut-design.md`
- `2026-09-03-shuomi-account-entry-zitadel-native-flow-design.md`

---

## 1. 决策

Phase1 登录只使用官方 ZITADEL Login V2 与现有 Auth.js ZITADEL Provider，不为现有用户登录开发或部署 Login V2 Fork。

```text
listingkit-ui /login
→ Auth.js ZITADEL Provider
→ official ZITADEL Login V2 generic entry
→ Password / Passkey / IDP
→ OTP SMS（仅在 ZITADEL Policy 要求时作为 MFA）
→ ZITADEL CreateCallback
→ Auth.js session
```

具体约束：

1. bare `/login?returnTo=...` 是 Phase1 唯一对外宣传的登录入口；
2. `returnTo` 继续使用现有 normalization/allowlist，认证成功后仍由 Auth.js 完成应用侧回跳；
3. 手机号只作为 ZITADEL 支持的 login name / user identifier；是否允许手机号识别由 ZITADEL Login Policy 决定；
4. Password、Passkey、IDP 继续由官方 Login V2 按用户与 Policy 选择；
5. OTP SMS 保持 ZITADEL 当前的二次因子语义，不提升为全局或专用的主认证因子；
6. 若产品需要无密码登录，Phase1 优先使用 ZITADEL 原生 Passkey；
7. 不新增 method handoff、跨域 entry cookie、sessionStorage bridge、自定义 authorize 参数、OTP-primary session validator 或本地 callback runtime；
8. `shuomi-zitadel-login` 不进入 Phase1 登录运行时依赖、部署或发布链路。

---

## 2. 为什么替代原登录方案

对 pinned ZITADEL `v4.17.1`（commit `a9311b8c702531832575351a663e98a2242778e5`）的实现审计确认：

- ZITADEL 创建 Login V2 URL 时只附加 `authRequest`；
- Login V2 可读取的 AuthRequest 不包含 `otp_login` / `password_login` 之类的自定义 Entry；
- Login V2 的通用 session validity 将 Password、Passkey/WebAuthn 或 IDP intent 视为主认证证明；
- OTP SMS 在现有模型中属于 MFA/second factor，不能仅通过 Entry Routing 变成无密码主登录。

原方案同时要求“短信 OTP 是主登录”“只做薄 Login V2 Fork”“不新增认证策略”和“不修改 ZITADEL Core”。这些约束在 pinned 版本下不能同时成立。

```text
Finding: advertised otp_login 无法只靠 Entry Routing 完成 OIDC happy path
Product requirement affected: 手机号短信 OTP 作为 Phase1 默认主登录
Classification: BLOCKER
Reason: 命中“核心 happy path 按当前设计无法完成”；继续实现会迫使项目自建 method handoff 和 OTP-primary 认证策略
Action: Phase1 收敛到官方 generic Login V2；OTP SMS 仅作为 ZITADEL MFA
```

这不是用 generic 路由伪装已实现专用登录，而是明确撤销 Phase1 对专用 OTP/Password Entry 的产品承诺。

---

## 3. 公开 Entry Contract

Phase1 对外合同收敛为：

```text
/login → generic_login → official ZITADEL Login V2
```

以下入口不在 Phase1 UI 中展示或宣传：

```text
/login?method=otp
/login?method=password
```

为保持已发布路由的安全兼容性，当前 allowlist 与 fail-closed 行为保留：

- bare、缺失、未知或重复 `method` 进入 `generic_login`；
- allowlisted `method=otp` 与 `method=password` 继续返回 `503 login_capability_unavailable`；
- 专用入口不得静默落到 generic 并让调用方误以为已选择目标认证方式；
- 不通过环境变量单独打开尚未实现的专用 flow。

`method=password` 不再是必要产品入口。用户提交 login name 后，官方 Login V2 已能根据用户的认证方法与 Login Policy 进入 Password Flow。

---

## 4. 能力边界

### 4.1 Phase1 Must

- generic Login V2 不因手机号产品化而关闭；
- existing email-only、手机号 login name 以及其他 ZITADEL 支持的历史身份继续使用同一入口；
- Auth.js/OIDC code exchange、token refresh 与业务 session 保持现状；
- Password、Passkey、IDP、OTP SMS MFA 的校验和策略都由 ZITADEL 持有；
- 非法 `returnTo`、外站 callback 与未实现的专用 method 继续 fail closed。

### 4.2 Phase1 Out of Scope

- 短信 OTP 作为唯一主认证；
- 手机号专用 Password Tab；
- Phone-only Password Reset；
- Login V2 Fork 的 method handoff 与专用完成策略；
- 修改 ZITADEL AuthRequest、OIDC Core、AMR/ACR 或 session linking；
- 第二个 OIDC Client、第二套 IAM 或外部 OTP IdP；
- `shuomi-zitadel-login` 的部署和运行态配置。

手机号自助注册是否仍需要独立 UI/桥接，由注册设计单独决定；不得以注册需求重新把 OTP-primary 登录隐式带回本切片。

---

## 5. 验证与上线门禁

Phase1 登录验收只要求：

1. protected route → bare `/login?returnTo=...` → generic Login V2；
2. existing email-only user 可以完成登录并建立 Auth.js session；
3. 允许 phone login name 的 ZITADEL Policy 下，手机号用户可以进入其已有的 Password/Passkey/IDP 流程；
4. 启用 OTP SMS MFA 的 ZITADEL Policy 下，MFA challenge 与验证由官方 Login V2 完成；
5. 非法 `returnTo` 回到安全默认地址；
6. `method=otp/password` 不展示且直接访问继续 fail closed。

本决策不授权部署或运行态变更。是否启用手机号识别、Passkey 或 OTP SMS MFA，必须通过 ZITADEL Policy 与 staging 验证单独完成。

---

## 6. 重新打开条件

只有以下情况之一成立时，才重新评估专用登录入口或 Login V2 Fork：

- 产品重新确认“短信 OTP 作为唯一主认证”是 Must，并接受其为架构敏感修改；
- pinned ZITADEL 后续版本原生提供可验证的 passwordless phone OTP 与 Entry 选择协议；
- 经独立设计批准引入外部 OTP IdP 或多个 OIDC Client；
- staging 证据证明现有官方能力与本文审计结论不同。

重新打开时必须先冻结认证证明、method handoff、并发标签页、AuthRequest 绑定、失败重试和 callback 完成语义，再开始实现。

---

## 7. 对现有实现的影响

PR #291 的实现继续有效：

- 保留 generic `/login`；
- 固化 method allowlist；
- 对未实现能力 fail closed；
- 统一 `returnTo` 与 Auth.js redirect 安全策略。

本决策不要求修改生产代码，也不把“已存在 fail-closed 路由”解释为将来必须开发对应能力。
