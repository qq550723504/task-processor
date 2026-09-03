# 硕米账号入口：ZITADEL Login V2 轻量定制设计

**状态：** 已确认

**适用范围：** 第一阶段账号入口（手机号注册、手机号验证码登录、手机号密码登录、密码重置）

**关联文档：**

- `docs/superpowers/specs/2026-09-02-shuomi-console-phase1-hard-cut-design.md`
- `docs/superpowers/plans/2026-09-03-shuomi-account-entry-slice1.md`

**覆盖关系：** 本文覆盖并取代旧 Console 规格和旧账号入口计划中关于 `internal/accountentry`、PhoneIdentity/HMAC Alias、Decoy User、Callback Delivery、Account Entry Operation Runtime、Temporal 登录 Saga、登录专用 Redis Flow 等设计。Console、账户、企业权益和店铺中心的其他产品决策继续有效。

---

## 1. 核心决策：不再在 task-processor 里重写 IAM 流程

账号入口以 ZITADEL 官方 Login V2 为代码基座，只做硕米需要的最小 UI 与手机号注册扩展。

```text
ZITADEL 官方 Login V2（独立部署）
├── Next.js Login App
├── OIDC Proxy
├── Session Cookie
├── Server Actions
├── Auth Request
├── Password / Reset
├── OTP SMS
├── MFA / Passkey（保留上游能力）
└── CreateCallback / Redirect
        ↓
ZITADEL
├── User
├── Phone / Password / OTP Factor
├── Session
├── Organization
├── Project Authorization
└── OIDC Token
        ↓
listingkit-ui
└── 现有 Auth.js ZITADEL Provider
        ↓
task-processor
├── 登录后业务用户/企业投影
├── base_payg 初始化
├── 协议接受业务投影
└── Workbench 就绪检查
```

第一阶段明确不建设：

```text
internal/accountentry 身份状态机
手机号本地 HMAC 盲索引 / Alias 表
手机号注册数据库 Claim
登录专用 Operation Runtime
共享 Decoy User / Decoy Pool
业务侧 OTP 校验器
业务侧 Password 校验器
自研 callback generation / ACK 协议
账号入口 Temporal Workflow
账号入口 Actions v2 强依赖
登录专用设备 Cookie + Redis 限流体系
```

这些能力只有在确认 ZITADEL 官方 Login V2 无法满足具体要求时，才允许以最小补丁重新评估；不能预先复制实现。

---

## 2. 为什么采用官方 Login V2

ZITADEL 官方 Login App 本身就是 Next.js 应用，使用 ZITADEL Session API 与 OIDC API，实现：

```text
Authorization Code + PKCE
AuthRequest prompt / loginHint
Session Cookie
Server Actions
OTP via SMS
OTP via Email
TOTP / Passkey
Password 登录
Password Reset
OIDC CreateCallback 与应用回跳
```

因此硕米应复用官方浏览器状态、Session Cookie、OIDC Proxy 和 Callback 流程，而不是在 `listingkit-ui` + Go BFF 中重建第二套流程。

PR #218 已经完成的 `phoneonboardingpreflight` 与 `zitadelsms` 保留为 ZITADEL 手机能力的验证工具和回归探针，不提升为第二套生产登录框架。

---

## 3. 部署边界

采用独立 Login 域名，例如：

```text
https://login.shuomi.example
```

部署内容是与当前 ZITADEL 版本匹配的官方 Login V2 fork，优先保留完整上游仓库历史，修改集中在 `apps/login`。

要求：

```text
ZITADEL 版本与 Login fork 基线版本明确固定
login 域名注册为 ZITADEL Trusted Domain
OIDC Proxy 保持官方 middleware 约定
x-zitadel-public-host / x-zitadel-instance-host 按官方要求配置
Login App Service Account 仅授予官方 Login 所需权限
```

`listingkit-ui` 不复制 Login App 的页面和 Session 管理代码。

---

## 4. 手机号作为唯一登录入口

硕米用户侧统一把手机号规范化成 E.164：

```text
+8613800138000
```

第一阶段把规范化手机号作为用户可见的登录名 / canonical login name。必须在预发布验证：

```text
同一手机号在实例范围不能创建两个可登录身份
手机号登录名能够走官方 Login V2 的 password login path
手机号能够走 ZITADEL Session + OTP SMS path
手机号变更后登录名与 ZITADEL Phone 的一致性策略明确
```

如果当前 ZITADEL Policy 无法在实例范围保证手机号登录名唯一，则先修正 ZITADEL Policy/配置；不以 task-processor HMAC Alias 表补第二套唯一性。

---

## 5. 自助手机号注册：只在 Login V2 fork 内做最小扩展

官方 Login V2 当前注册主路径以 Email/Password、Passkey 为主，因此硕米只增加一个手机号注册分支，仍然复用上游 Session/OIDC 结构。

目标页面：

```text
手机号
短信验证码
同意服务协议与隐私政策
[注册并进入]
```

正确流程：

```text
1. Login App 规范化手机号为 E.164
2. 使用 ZITADEL 自身登录名唯一性判断已有身份
3. 新身份注册时创建默认 ZITADEL Organization + Human User
4. User 的 canonical login name 使用手机号
5. 不在 task-processor 创建 PhoneIdentity / Binding
6. 使用 ZITADEL OTP SMS / Session API 发送并验证验证码
7. OTP Proof 成功后，使用 ZITADEL API 确保该 User 拥有当前企业的 listingkit_admin 项目授权
8. 在 ZITADEL User Metadata 记录本次接受的硕米协议版本与接受时间
9. 按官方 Login V2 流程调用 CreateCallback 并回到 Auth.js
10. 登录后由 task-processor 幂等初始化业务投影和 base_payg
```

创建 Organization/User 时优先使用 ZITADEL v4.17.1 已支持的调用方 ID / 组合创建能力，减少跨请求部分成功窗口；具体 API 形状必须由实现任务基于当前部署版本生成类型验证，不能在业务层再造恢复协议。

如果 ZITADEL 返回“用户/登录名已存在”，Login App 转入既有用户登录流程；唯一性事实以 ZITADEL 为准。

---

## 6. 手机号验证码登录

验证码登录完全属于 Login App + ZITADEL：

```text
手机号
→ ZITADEL 查找登录身份
→ Session
→ OTP SMS Challenge
→ ZITADEL Verify
→ Session Proof
→ CreateCallback
→ Auth.js
```

`task-processor` 不参与：

```text
手机号查 User
验证码生成
验证码比较
OTP attempt 状态
Session Token
Callback URL
```

不存在用户、错误验证码、Challenge 过期、尝试次数和锁定等行为优先继承 ZITADEL / Login V2 的上游策略。预发布必须验证实际用户可观察行为；如果上游保护低于硕米最低安全要求，补丁应放在 Login fork 或 Ingress，不放进业务 API。

---

## 7. 手机号密码登录与密码重置

手机号密码登录复用官方 Login V2 Password Flow：

```text
canonical login name = E.164 手机号
→ 官方 Login App password path
→ ZITADEL password check / lockout policy
→ Session
→ OIDC
```

不实现业务侧：

```text
PasswordDecoyUser
密码存在性缓存
密码时序补偿
密码哈希
密码重置 Token
```

密码重置同样使用官方 Login V2 / ZITADEL User API 已有流程。

如果预发布确认官方 Password Flow 不能以手机号 login name 工作，则 Slice 1 暂时只开放验证码登录；不得为了保留一个 Tab 而新增自研密码认证框架。

---

## 8. 防刷、锁定、CSRF 与浏览器安全

优先顺序：

```text
ZITADEL Login / Lockout / OTP Policy
→ Login App 官方 Server Action / Session Cookie 安全模型
→ Ingress / CDN 粗粒度 IP 速率限制
→ 必要时 Login fork 的窄范围补丁
```

不在 `task-processor` 重复建设手机号/IP/设备三维登录限流。

必须在预发布验证：

```text
OTP 发送冷却
单 Challenge 错误尝试上限或等效 ZITADEL 限制
Password 错误尝试与 lockout
跨 Origin POST / Server Action 防护
Trusted Domain / Host 校验
同站点恶意子域不能驱动认证写操作
代理链不能由浏览器伪造客户端 IP
```

若某一项上游没有满足要求，只针对该缺口加补丁，不复制整套 Account Entry Runtime。

---

## 9. 协议接受证据

注册协议不是只做前端 Checkbox。

Login App 在 OTP Proof 成功后写入 ZITADEL User Metadata：

```text
shuomi_terms_version
shuomi_terms_accepted_at
```

Workbench 登录后 Bootstrap 再把该事实幂等投影到 task-processor：

```text
saas_account_consents
- zitadel_user_id
- policy_version
- accepted_at
- projected_at
- UNIQUE(zitadel_user_id, policy_version)
```

没有当前有效协议版本时，WorkBench 进入协议确认 Gate，不进入业务页面。

这样不需要把业务协议接受事务塞进 OIDC callback 前的跨系统 Saga，同时仍保留可审计证据。

---

## 10. 登录后业务 Bootstrap

业务初始化从登录流程中移出。

Auth.js 已经拿到 ZITADEL Token 后：

```text
/workbench/bootstrap
→ 从已验证 Token 读取 user_id / organization_id / roles
→ EnsureBusinessUser
→ EnsureBusinessOrganization
→ EnsureCurrentConsentProjection
→ EnsureBasePayg
→ 返回 ready
```

数据库唯一约束是最终一致性边界：

```text
UNIQUE(zitadel_user_id)
UNIQUE(zitadel_organization_id)
UNIQUE(organization_id, plan_code)
UNIQUE(zitadel_user_id, policy_version)
```

Bootstrap 要求幂等，但它是普通业务初始化，不是 IAM 工作流：

```text
重复调用 → 返回同一 ready 状态
部分失败 → 下次请求继续 Ensure
初始化失败 → 显示“正在初始化工作空间 / 重试”
身份 Session 本身仍有效
```

`listingkit_admin` 等 ZITADEL 项目授权必须在 OIDC Token 产生前由 Login App/ZITADEL 完成；`base_payg` 等硕米业务数据放到登录后 Bootstrap。

---

## 11. ZITADEL Actions v2

Actions v2 第一阶段是可选增强，不是账号入口依赖。

可以用于：

```text
User / Organization / Role 变化
→ 唤醒 task-processor 刷新业务投影或缓存
```

不用于：

```text
发送验证码
决定密码是否正确
维持浏览器登录 Flow
阻塞每一次登录
创建 base_payg 的唯一事务入口
```

如果 Actions v2 的签名、事件 ID、Freshness 在当前版本没有完成预发布证明，则保持关闭，不影响账号入口上线。

---

## 12. 企业邀请不进入 Slice 1

企业邀请属于：

```text
企业空间
→ 成员管理
```

从账号入口 Slice 1 移出。

后续实现时再定义：

```text
可邀请角色白名单
LiveWrite Effective Organization
邀请 Token
已有用户加入企业
新用户接受邀请
权限升级上限
```

第一阶段不为邀请功能引入额外身份状态机。

---

## 13. 第一阶段明确保留与删除

### 保留

```text
PR #218 SMS Provider 与预检能力
现有 Auth.js ZITADEL Provider
ZITADEL User / Organization / Project Role
ZITADEL Session / OIDC
官方 Login V2 Session Cookie / Server Actions / Callback
手机号注册最小扩展
登录后 base_payg Bootstrap
协议接受证据
```

### 删除或延期

```text
PhoneIdentity / FingerprintAlias
Phone HMAC Key Ring
Operation ID Key Ring
Flow AEAD Key Ring
Account Entry Business Claim
Password Decoy
Callback Delivery Generation
Account Entry Actions Inbox（可选后续）
Account Entry Temporal
企业邀请
成员权限
企业钱包
资源账本和店铺激活实现
```

资源账本、店铺激活仍按各自独立设计/切片实施，不与身份登录代码耦合。

---

## 14. 上线门槛

第一阶段只有以下条件同时成立才允许生产启用：

```text
官方 Login V2 fork 能从当前 ZITADEL 版本干净构建和部署
手机号 OTP 注册真实验证通过
同手机号并发注册由 ZITADEL 唯一性拒绝重复身份
手机号 OTP 登录通过
手机号 Password 登录通过；若不支持则该入口隐藏
Password Reset 通过；若不支持则保持官方可用入口或暂缓
OIDC → Auth.js → Workbench 闭环通过
登录后 Bootstrap 重放不会创建重复业务数据
协议版本有服务端持久证据
安全行为（OTP 尝试、lockout、Origin、Host、代理链）有真实验证结果
无 task-processor 自建验证码、密码、PhoneIdentity 或 callback 流程
```

原则固定为：

> 能由 ZITADEL 官方 Login V2、Session/OIDC、Organization/Project Role 解决的身份问题，不在 task-processor 再造一遍；硕米只实现真正的业务差异。