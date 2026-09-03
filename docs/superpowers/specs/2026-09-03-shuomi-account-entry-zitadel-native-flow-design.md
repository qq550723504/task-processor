# 硕米账号入口：ZITADEL Login V2 轻量定制与业务开通边界

**状态：** 已确认

**适用范围：** 第一阶段账号入口（手机号注册、手机号验证码登录、手机号密码登录、密码重置）以及直接注册后的首次业务开通。

**关联文档：**

- `docs/superpowers/specs/2026-09-02-shuomi-console-phase1-hard-cut-design.md`
- `docs/superpowers/plans/2026-09-03-shuomi-account-entry-slice1.md`

**覆盖关系：** 本文覆盖并取代旧 Console 规格和旧账号入口计划中关于 `internal/accountentry`、PhoneIdentity/HMAC Alias、Decoy User、Callback Delivery、Account Entry Operation Runtime、Temporal 登录 Saga、登录专用 Redis Flow 等设计。Console、账户、企业权益和店铺中心的其他产品决策继续有效。

---

## 1. 核心决策：认证不重造，业务开通单独收敛

账号入口以 ZITADEL 官方 Login V2 为代码基座，只做硕米需要的最小 UI、手机号注册和业务开通桥接。

```text
ZITADEL 官方 Login V2 Fork
├── Next.js Login App
├── OIDC Proxy
├── Session Cookie / Server Actions
├── Password / Reset
├── OTP SMS
├── Auth Request / CreateCallback
└── 硕米手机号注册薄适配器
        ↓
ZITADEL
├── User / Phone / Password / OTP
├── Session
├── Organization
├── Project Authorization
└── OIDC Token
        ↓
硕米业务开通 Prepare
├── Consent
├── Business User / Organization Projection
├── listingsubscription.base_payg
└── prepared 状态
        ↓
最后授予 listingkit_admin
        ↓
Auth.js OIDC Session
        ↓
Workbench Bootstrap
└── 只验证 live grant + prepared readiness，不再发放权益
```

第一阶段明确不建设：

```text
internal/accountentry 通用身份状态机
手机号本地 HMAC 盲索引 / Alias 表
登录专用 Registration Claim / Operation Runtime
共享 Decoy User / Decoy Pool
业务侧 OTP / Password 校验器
业务侧 callback generation / ACK
Account Entry Temporal Workflow
登录专用设备 Cookie + Redis 三维限流体系
```

原则：**ZITADEL 管身份，硕米只管“认证后可以获得哪些硕米业务能力”。**

---

## 2. 权威数据边界

| 数据 / 行为 | 权威来源 |
|---|---|
| User、Phone、Password、OTP Factor | ZITADEL |
| Session、AuthRequest、CreateCallback、Token | ZITADEL / Login V2 |
| Organization、Project Authorization、Project Role | ZITADEL |
| 浏览器登录状态 | 官方 Login V2 Session Cookie + Auth.js |
| 当前协议版本与接受证据 | task-processor `saas_account_consents` |
| 新企业的 `base_payg` | 现有 `internal/listingsubscription` |
| 业务用户 / 企业投影 | task-processor |
| Workbench 是否 ready | ZITADEL live grant + task-processor onboarding readiness |

`task-processor` 不保存：

```text
密码 / 密码哈希
短信验证码
ZITADEL Login Session Token
OIDC callback URL
手机号身份盲索引
```

---

## 3. Login V2 Fork 与凭据边界

采用独立 Login 域名，例如：

```text
https://login.shuomi.example
```

Fork 基线必须与当前部署 ZITADEL 版本一致，硕米差异优先集中在 `apps/login`。

### 3.1 登录运行凭据

官方 Login V2 的 Session / OIDC 运行使用独立 Login Service Account / PAT：

```text
ZITADEL_LOGIN_CLIENT_TOKEN
```

它只拥有官方 Login 所需 `IAM_LOGIN_CLIENT` 等登录权限，不允许因为手机号注册扩展而扩大成全能管理凭据。

### 3.2 注册 Provisioning 凭据

手机号自助注册需要创建 Organization / User、读取或清理自己创建的 pending 资源、授予项目角色，因此使用第二套独立凭据：

```text
ZITADEL_REGISTRATION_PROVISIONING_TOKEN
```

权限按当前 ZITADEL 版本真实 API Required Permission 配置，目标是最小化到：

```text
创建 / 查询 / 删除本注册流程拥有的 Organization / User
读取项目授权状态
创建所需 listingkit_admin 项目授权
不得读取用户密码
不得替代 Login Session Client
```

现有 PR #218 已验证“Provisioning Token 与 Login Client Token 分离”的模式；新 Login Fork 延续这一边界。

### 3.3 硕米业务开通凭据

Login Fork 调用 task-processor 的内部业务开通接口时使用第三套独立的服务到服务凭据：

```text
SHUOMI_ONBOARDING_PROVISIONER_TOKEN
```

该凭据只能调用 onboarding prepare/readiness，不允许调用店铺、资源、钱包等其他业务 API。所有 Token 均只在服务器端 Secret 中，禁止进入浏览器。

必须有负向权限测试证明三套凭据不能互相替代。

---

## 4. 手机号作为 canonical login name

硕米用户侧统一把手机号规范化成 E.164，例如：

```text
+8613800138000
```

预发布必须验证：

```text
同一 E.164 login name 在实例范围不能创建两个可登录身份
手机号 login name 能进入官方 Password Flow
手机号对应 User 能进入 OTP SMS Session Flow
手机号修改后的 login name / Phone 一致性策略明确
```

如果当前 ZITADEL Policy / API 用法不能保证唯一性，先修正 ZITADEL 配置或创建方式；不得用 task-processor HMAC Alias 表补第二套身份唯一性。

---

## 5. 手机号注册是 Login Fork 中的“薄 Provisioning Adapter”

官方 Login V2 当前注册主路径不是硕米所需的“手机号 + OTP”，所以允许在 Login Fork 中增加一个窄范围注册适配器；它不是第二套 IAM。

页面只包含：

```text
手机号
短信验证码
服务协议 + 隐私政策
[注册并进入]
```

不得要求用户名、注册密码、邮箱或经营画像。

---

## 6. Provider-native Pending Registration 生命周期

手机号 OTP Challenge 需要先有 ZITADEL User，因此新注册不可避免地会在 OTP Proof 前产生 Provider 资源。这个部分必须可恢复，但恢复逻辑只留在 Login Fork / ZITADEL 边界，不进入 task-processor 身份框架。

### 6.1 两阶段开始，先固定 ID 再做 Provider Side Effect

第一阶段注册拆成两个服务器步骤：

```text
A. BeginRegistration
   - 校验手机号格式和协议 Checkbox
   - 生成 random registration_id
   - 预分配 provider_organization_id / provider_user_id
   - 写入签名 + HttpOnly + Secure 的短期 Registration Intent Cookie
   - 返回/重定向到发送验证码阶段
   - 此步骤不调用任何 ZITADEL Create API

B. PrepareIdentityAndSendChallenge
   - 从 Registration Intent 读取稳定 IDs
   - 再调用 ZITADEL Create / lookup / adopt
   - 创建 OTP SMS Challenge
```

因此如果 A 的 HTTP 响应丢失，没有 Provider Side Effect；如果 B 的 Provider 响应丢失，重试仍使用相同 IDs。

Registration Intent 只保存：

```text
registration_id
provider_organization_id
provider_user_id
expires_at
nonce / version
```

不保存密码、验证码或 ZITADEL Session Token。

### 6.2 创建 / Adopt 规则

调用 ZITADEL Create 前必须持久拥有稳定 IDs：

```text
Create Organization(provider_organization_id)
Create User(provider_user_id, provider_organization_id, canonical_phone)
```

结果：

```text
创建成功 → 继续
超时 / 断连 / 结果未知 → 先按相同 ID Get，再决定 adopt 或同 ID 重试
相同手机号已存在 → 服务端判定 existing / pending / conflict，但 Proof 前不向浏览器暴露分支
对象存在且关键关联匹配 → adopt
对象存在但 ID / org / canonical login name 不匹配 → quarantine，不猜测覆盖
```

不得“响应没收到就生成一套新 ID 再创建”。

### 6.3 Pending 标记与清理

Login Fork 创建的新 Organization / User 使用明确的注册技术标记，例如：

```text
technical name prefix = shuomi-reg-
metadata:
  shuomi_registration_id
  shuomi_registration_state = pending
  shuomi_registration_expires_at
```

完成业务 Prepare 和 `listingkit_admin` 授权后改为：

```text
shuomi_registration_state = active
```

Login 部署包含一个窄范围 Registration Janitor（CronJob 或等价定时任务），只清理满足全部条件的过期 pending 注册：

```text
由 Shuomi Registration Provisioner 创建
超过注册 TTL
registration_state = pending
没有 listingkit_admin / 其他项目授权
没有 task-processor onboarding prepared 记录
没有已完成标记
```

满足条件时按稳定 Organization ID 删除整个 pending Organization；任一事实不确定则保留并记录 quarantine，不做猜测删除。

这套 lifecycle 仅服务于“官方 Login V2 不提供手机号自助注册”这一具体扩展，不抽成通用 Account Entry Framework。

---

## 7. `/register` 在手机号所有权证明前不得泄露账号存在性

服务端可以知道手机号对应：

```text
existing active user
existing pending registration
new user
```

但浏览器在 OTP Proof 前看到的流程必须相同：

```text
提交手机号
→ 同一“验证码已发送 / 请输入验证码”页面
→ 同类错误和状态
```

处理方式：

```text
existing active → 对已有 User 发送 OTP Login Challenge
existing pending → adopt pending User 后发送 / 恢复 OTP Challenge
new → 创建 pending User 后发送 OTP Challenge
```

只有手机号 OTP Proof 成功后，服务器才选择：

```text
existing active → 继续正常登录
new / pending → 继续首次业务开通
```

必须测试新手机号、已有手机号、pending 手机号在 Proof 前的页面、状态码、可用操作和响应时间分布没有可利用的存在性差异。若创建新身份带来明显时序差异，可在 Login Fork 增加有上限的响应时间下限；不得重新引入 Decoy User。

---

## 8. OTP Proof 后先准备业务，再授予项目访问

`listingkit_admin` 是新用户获得硕米业务访问的**最后一个效果**。

正确顺序：

```text
ZITADEL OTP Proof
        ↓
Login Fork 调用 task-processor Internal Onboarding Prepare
        ↓
持久化当前协议接受证据
        ↓
Ensure Business User / Organization Projection
        ↓
通过现有 listingsubscription.ApplyPlan 确保 base_payg
        ↓
标记 onboarding = prepared
        ↓
Login Fork 查询确认 prepared
        ↓
最后创建 / adopt ZITADEL listingkit_admin Project Authorization
        ↓
标记 registration_state = active
        ↓
官方 CreateCallback
        ↓
Auth.js Session
```

**禁止：** 先授予 `listingkit_admin`，再尝试写协议或套餐。

### 8.1 Internal Onboarding Prepare

Login Fork 使用 `SHUOMI_ONBOARDING_PROVISIONER_TOKEN` 调用：

```http
POST /api/v1/internal/onboarding/prepare
GET  /api/v1/internal/onboarding/readiness
```

请求中的 `user_id / organization_id` 只能来自 Login Fork 服务器端已完成 OTP Proof 的 ZITADEL Session，上述接口不暴露给浏览器。

持久化模型：

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

saas_account_consents
- zitadel_user_id
- policy_version
- accepted_at
- source = phone_registration
- created_at
UNIQUE (zitadel_user_id, policy_version)
```

Prepare 使用短数据库事务和业务唯一约束，不引入通用 Operation Runtime：

```text
同 user/org + 同 policy/fingerprint → 幂等继续或返回 prepared
同 user/org + 不同不可变注册事实 → ONBOARDING_PREPARE_CONFLICT
```

Provider / 网络响应丢失时，Login Fork 调用 readiness；`prepared=true` 就继续授权，不重复产生业务权益。

### 8.2 Consent 权威来源

协议接受证据直接写入 task-processor；Workbench 不再从浏览器参数或未定义的 ZITADEL Metadata reader 推断 Consent。

Consent 必须在项目授权之前持久化成功。Prepare 失败时不授予 `listingkit_admin`。

---

## 9. `base_payg` 必须复用现有 Subscription Authority

新方案不得建立平行套餐表。

在现有 `internal/listingsubscription` 中新增：

```text
PlanBasePayg = "base_payg"
展示名 = 基础方案 · 按需使用
状态 = active
```

并加入 `DefaultPlans()` / 现有计划 Catalog，使 `Service.ApplyPlan` 能正式接受该 Plan。

历史计划：

```text
basic
professional
enterprise
```

继续保留，不能因为新增 `base_payg` 自动迁移或覆盖历史租户。

直接注册的新 Organization：

```text
无历史 subscription → ApplyPlan(base_payg)
已有 subscription → 不覆盖，进入一致性检查 / 人工迁移策略
```

`base_payg` 的产品约束继续是：基础按需使用、无伪造 Trial、无虚假到期；店铺数量与后续资源钱包按对应领域设计执行。

---

## 10. 手机号验证码登录、密码登录与密码重置

### 10.1 OTP 登录

验证码登录完全属于 Login App + ZITADEL：

```text
手机号
→ ZITADEL User / Session
→ OTP SMS Challenge
→ Verify
→ Session Proof
→ CreateCallback
→ Auth.js
```

`task-processor` 不参与手机号查 User、验证码生成/比较、OTP attempt、Session Token 或 Callback URL。

### 10.2 Password / Reset

手机号密码登录和密码重置优先复用官方 Login V2 Password Flow / ZITADEL Password Policy。

如果预发布确认 E.164 login name 不能可靠进入官方 Password Flow，Slice 1 隐藏密码登录 Tab；不得为了保留 UI Tab 引入 Decoy User、自定义密码校验或业务侧密码重置 Token。

---

## 11. 防刷、锁定、CSRF 与代理边界

优先顺序：

```text
ZITADEL Policy
→ 官方 Login V2 Server Action / Session Cookie
→ Ingress / CDN
→ Login Fork 窄范围补丁
```

预发布必须验证：

```text
OTP 发送冷却
单 Challenge 错误尝试上限或等效限制
Password 错误次数与 lockout
Origin / Host / Trusted Domain 防护
同站点恶意子域不能驱动认证写操作
浏览器伪造 X-Forwarded-* 不改变可信客户端 IP
Registration Intent Cookie 的 Secure / HttpOnly / SameSite / TTL
Registration Janitor 只能删除自己拥有且仍 pending 的资源
```

若上游不满足，只补具体缺口；禁止重新引入 task-processor 登录限流框架。

---

## 12. Workbench Bootstrap 不再发放权益

OIDC / Auth.js 完成后进入：

```text
/workbench/bootstrap
```

它的职责是：

```text
1. 通过 AuthPolicyVerifiedIdentity 验证身份
2. 使用 OrganizationAccessPolicyLiveWrite 解析实时 Effective Organization
3. 通过稳定 permission 要求该主体当前拥有 listingkit_admin（platform 管理代管另走受控语义）
4. organization_id 只取 live resolved organization，不接受 body / query 覆盖
5. 查询 saas_onboarding_preparations = prepared
6. 查询当前 Subscription / Consent / Business Projection 是否一致
7. 返回 ready 或明确 repair_required
```

Bootstrap **不得**因为“用户已经登录”就创建 `base_payg` 或授予新的企业权益。

前端 `proxy.ts` 的角色判断只是 UX 优化，后端 LiveWrite + Permission 是授权权威。

---

## 13. ZITADEL Actions v2

Actions v2 第一阶段是可选增强，不是登录或业务开通依赖。

可以用于：

```text
User / Organization / Role 变化
→ 唤醒 task-processor 刷新业务投影或缓存
```

不用于发送验证码、判断密码、维持浏览器 Flow、发放 `base_payg` 或绕过 Onboarding Prepare。

---

## 14. 企业邀请不进入 Slice 1

企业邀请属于：

```text
企业空间 → 成员管理
```

后续单独定义可邀请角色白名单、LiveWrite Effective Organization、邀请 Token、已有用户加入企业、新用户接受邀请和权限升级上限。

---

## 15. 上线门槛

只有以下条件同时成立才允许生产启用：

```text
官方 Login V2 fork 能从固定 ZITADEL 版本干净构建和升级
Login Runtime / Registration Provisioning / Sumi Onboarding 三套凭据隔离并通过负向测试
E.164 login name 唯一性和并发注册真实验证通过
Provider Create 响应丢失可按稳定 ID lookup/adopt
pending 注册有 TTL、resume 和 ownership-safe cleanup
已有 / 新 / pending 手机号在 OTP Proof 前不可枚举
OTP Proof 前没有 listingkit_admin
Consent + base_payg + business projection 全部 prepared 后才授予 listingkit_admin
base_payg 通过现有 listingsubscription Catalog / ApplyPlan 实现且不覆盖历史套餐
Auth.js → Workbench Bootstrap 使用 live org + admin permission
OTP / Password / Origin / Host / Proxy 安全行为真实验证通过
没有 task-processor 自建 PhoneIdentity、验证码、密码或 callback 状态机
```

固定原则：

> 能由 ZITADEL 官方 Login V2、Session/OIDC、Organization/Project Role 解决的身份问题，不在 task-processor 再造一遍；无法由 ZITADEL 完成的“硕米业务开通”只实现一条窄、可审计、可重放的 Provisioning 边界。