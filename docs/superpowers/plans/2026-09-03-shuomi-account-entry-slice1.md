# 硕米账号入口 Slice 1 Implementation Plan（复用已实现手机号链路）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不重做已合并手机号 OTP 能力的前提下，将现有 ZITADEL 手机验证预检提升为可持久化、可恢复、可发布的硕米注册、验证码登录、手机号密码登录、密码重置与邀请注册产品流程。

**Architecture:** 已合并的 `phoneonboardingpreflight` 和 `zitadelsms` 是已验证基线，不再重新实现。先把其中已验证的 ZITADEL Session/OTP HTTP 客户端抽取到中性运行时包，再由新的 `internal/accountentry` 领域模块负责流程状态、手机号 HMAC 绑定、限流、邀请、企业与角色初始化、OIDC Auth Request 完成和补偿；`web/listingkit-ui` 只负责 Figma 页面、同源 BFF、HttpOnly 流程 Cookie 以及把受控 callback URL 以 `303` 交给现有 Auth.js。

**Tech Stack:** Go 1.26、Gin/kernel module、GORM、Redis、ZITADEL Core/Login V2 v4.17.1、Next.js 16 App Router、React 19、Auth.js 5、TypeScript、Zod、Tailwind CSS 4、Vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-09-02-shuomi-console-phase1-hard-cut-design.md`

## Global Constraints

- PR #218 已经合并并证明单次 ZITADEL SMS OTP、`user` 因子与 `otpSms` 因子可用；本计划不得重写短信 Relay、OTP 事件白名单或预检 Runner。
- 当前预检中 ZITADEL 的手机字段属于创建 Session Challenge 所需的引导状态；它本身不构成硕米授权证明。只有 `SessionProof` 同时包含匹配的 `user` 与 `otpSms` 因子后，流程才可初始化企业角色、套餐和 OIDC 回调。
- 不新增短信验证码表，不在 Go、Next.js 或数据库中比较和保存验证码；验证码只能转交 ZITADEL Session API 校验。
- ZITADEL 是用户、手机号、密码、Session、Organization 和项目角色的身份权威；`task-processor` 不保存第二套密码或密码哈希。
- 注册页面只收集手机号、短信验证码和协议确认；不要求展示名称、用户名、密码、邮箱或经营画像。
- 展示名称不作为登录账号；Slice 1 只支持手机号验证码和手机号密码登录。
- 自助注册绑定 `base_payg`；不得自动赠送 AI 点数、数据额度或店铺续费期数。
- 一个手机号指纹只映射一个 ZITADEL 用户身份；该身份以后可通过项目授权访问多个企业，不能为同一手机号重复创建多个用户。
- 浏览器 JavaScript 不得获得 ZITADEL 管理 Token、Login Client Token、Session Token、callback URL、authorization code、明文持久化手机号或邀请 Token。
- 所有公开响应对“账号存在／不存在”“是否设置密码”保持相同形状和稳定错误语义。
- 所有功能开关默认关闭；没有真实预发布验收证据时不得开放生产自助注册。
- 每个 Task 按 TDD 执行并独立提交；真实 Provider 合约不符时停止并修订计划，不添加猜测式兼容分支。

---

## 0. 已实现基线与剩余缺口

### 已经实现并合并

```text
internal/listingkit/zitadelsms
- ZITADEL HTTP SMS Provider 签名校验
- 腾讯云短信发送
- OTP SMS 精确事件白名单
- Provider 错误脱敏

internal/listingkit/phoneonboardingpreflight
- 创建临时 Organization
- 创建技术 Human User
- 添加 OTP SMS Factor
- 创建 SMS Challenge
- 验证 SMS Code
- 读取 Session user + otpSms 因子
- 删除 Session 和临时 Organization

hack/debug/listingkit-phone-onboarding-preflight
- 非生产真实设备预检命令
```

PR #218 已留下非生产证据：

```text
status=otp_verified
user_factor=true
otp_sms_factor=true
```

现有前端已经具备：

```text
Auth.js + ZITADEL Provider
/api/zitadel-auth/login
/api/zitadel-auth/callback
/api/zitadel-auth/session
/workbench 页面保护
```

### 尚未产品化

```text
持久化注册 Flow
手机号 HMAC → ZITADEL 用户绑定
生产限流与重放保护
自助注册后保留用户和企业
ORG_OWNER / listingkit_admin / base_payg 初始化
邀请链接签发与消费
OIDC Auth Request 完成并进入 Auth.js
正式 /register /login /forgot-password 页面
手机号密码登录和密码重置
部署 Secret、特性开关、回滚与发布验收
```

---

## 1. 目标流程

### 自助注册

```text
/register
→ 若无 authRequest，先进入 /api/account-entry/begin?screen=register
→ Auth.js 发起 ZITADEL authorize
→ ZITADEL Custom Login 回到 /login?authRequest=...
→ Next.js 根据 HttpOnly intent Cookie 转入 /register
→ Go 创建或恢复 inert Organization/User Flow
→ 复用已实现 ZITADEL SMS Challenge
→ 复用已实现 Session OTP 验证
→ 确认 user + otpSms 因子与 Flow 一致
→ 幂等添加 ORG_OWNER、listingkit_admin、base_payg
→ OIDC CreateCallback
→ Next.js BFF 303 到 Auth.js callback
→ /workbench 或合法 returnTo
```

### 验证码登录

```text
/login?method=otp
→ authRequest 准备完成
→ 手机号指纹解析到既有 user ID
→ ZITADEL SMS Challenge
→ ZITADEL Session OTP Proof
→ OIDC CreateCallback
→ Auth.js Session
```

### 密码登录

```text
/login?method=password
→ authRequest 准备完成
→ 手机号指纹解析到既有 user ID
→ ZITADEL Session user + password check
→ OIDC CreateCallback
→ Auth.js Session
```

### 重置密码

```text
/forgot-password
→ 手机号 + SMS OTP
→ ZITADEL Session Proof
→ 受信任服务调用 ZITADEL User API 设置新密码
→ 清除重置 Flow
→ 返回 /login?method=password
```

重置密码成功后不自动登录，以 Figma“返回登录”的产品行为为准。

---

## Task 1：抽取并复用已经验证的 ZITADEL 手机 Session Client

**Files:**
- Create: `internal/authruntime/zitadel/loginv2/client.go`
- Create: `internal/authruntime/zitadel/loginv2/types.go`
- Create: `internal/authruntime/zitadel/loginv2/client_test.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/zitadel_client.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/zitadel_client_test.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/runner.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/runner_test.go`
- Modify: `hack/debug/listingkit-phone-onboarding-preflight/main.go`
- Modify: `hack/debug/listingkit-phone-onboarding-preflight/main_test.go`

**Interfaces:**
- Consumes: PR #218 已验证的 HTTP 请求形状和双 Token 隔离。
- Produces:

```go
package loginv2

type Client interface {
    CreateOrganization(context.Context, string) (string, error)
    CreateTechnicalUser(context.Context, TechnicalUserInput) (string, error)
    AddOTPSMS(context.Context, string) error
    CreateSMSChallenge(context.Context, string, time.Duration) (SessionMaterial, error)
    VerifySMS(context.Context, string, string) (string, error)
    GetSession(context.Context, string, string) (SessionProof, error)
    DeleteSession(context.Context, string) error
    DeleteOrganization(context.Context, string) error
}
```

- [ ] **Step 1: 写抽取前的特征测试**

在新包中复制当前 `phoneonboardingpreflight/zitadel_client_test.go` 的请求契约断言，覆盖：

```text
/v2/organizations
/v2/users/new
/v2/users/{id}/otp_sms
/v2/sessions
PATCH /v2/sessions/{id}
GET /v2/sessions/{id}?sessionToken=...
DELETE Session
DELETE Organization
Provisioning Token 与 Login Client Token 严格分离
1 MiB Provider 响应上限
错误体、手机号、验证码、Token 不进入错误文本
```

- [ ] **Step 2: 运行测试确认新包尚不存在**

```powershell
go test ./internal/authruntime/zitadel/loginv2 -count=1
```

Expected: FAIL because the package has not been created.

- [ ] **Step 3: 移动现有实现，不改变请求行为**

将当前已验证客户端移动到 `loginv2`。`phoneonboardingpreflight` 仅保留兼容别名或薄适配器：

```go
type Client = loginv2.Client
type ClientConfig = loginv2.ClientConfig
type TechnicalUserInput = loginv2.TechnicalUserInput
type SessionMaterial = loginv2.SessionMaterial
type SessionProof = loginv2.SessionProof

var NewClient = loginv2.NewClient
```

不得复制两套 `doJSON`、请求结构或错误映射。

- [ ] **Step 4: 回归现有真实预检行为**

```powershell
go test ./internal/authruntime/zitadel/loginv2 -count=1
go test ./internal/listingkit/phoneonboardingpreflight ./internal/listingkit/zitadelsms -count=1
go -C hack/debug test ./listingkit-phone-onboarding-preflight -count=1
```

Expected: PASS;不发送新的真实短信，只运行单元测试。

- [ ] **Step 5: Commit**

```powershell
git add internal/authruntime/zitadel/loginv2 `
        internal/listingkit/phoneonboardingpreflight `
        hack/debug/listingkit-phone-onboarding-preflight
git commit -m "refactor: reuse verified ZITADEL phone session client"
```

---

## Task 2：在中性 Client 上补齐 OIDC Callback 与密码能力

**Files:**
- Modify: `internal/authruntime/zitadel/loginv2/client.go`
- Modify: `internal/authruntime/zitadel/loginv2/types.go`
- Modify: `internal/authruntime/zitadel/loginv2/client_test.go`

**Interfaces:**
- Consumes: Task 1 `loginv2.Client`。
- Produces:

```go
type AccountEntryClient interface {
    Client
    GetOIDCAuthRequest(context.Context, string) (OIDCAuthRequest, error)
    CreateOIDCCallback(context.Context, OIDCCallbackInput) (string, error)
    AuthenticatePassword(context.Context, PasswordSessionInput) (SessionMaterial, SessionProof, error)
    SetPassword(context.Context, SetPasswordInput) error
    GrantOrganizationOwner(context.Context, string, string) error
    CreateProjectAuthorization(context.Context, ProjectAuthorizationInput) (string, error)
}
```

- [ ] **Step 1: 写失败的协议测试**

```go
func TestGetOIDCAuthRequestUsesLoginClientCredential(t *testing.T)
func TestCreateOIDCCallbackSendsOnlySessionMaterial(t *testing.T)
func TestCreateOIDCCallbackRejectsUnexpectedOriginPathOrFragment(t *testing.T)
func TestAuthenticatePasswordChecksUserIDAndPasswordInOneSession(t *testing.T)
func TestSetPasswordUsesProvisioningCredentialAndNeverLeaksPassword(t *testing.T)
func TestGrantOrganizationOwnerUsesExactV4171Contract(t *testing.T)
func TestCreateProjectAuthorizationUsesConfiguredProjectID(t *testing.T)
```

- [ ] **Step 2: 锁定 OIDC 请求**

```http
GET  /v2/oidc/auth_requests/{authRequestId}
POST /v2/oidc/auth_requests/{authRequestId}
```

Callback 请求体：

```json
{
  "session": {
    "sessionId": "session-1",
    "sessionToken": "session-token-2"
  }
}
```

`CreateOIDCCallback` 只接受：

```text
origin == AccountEntry.PublicOrigin
path == /api/auth/callback/zitadel 或 /api/zitadel-auth/callback
无 userinfo
无 fragment
query 同时包含 code 和 state
```

- [ ] **Step 3: 密码登录只按 user ID 检查**

本地手机号绑定先解析出 ZITADEL `userID`，然后调用 Session API：

```json
{
  "checks": {
    "user": { "userId": "user-1" },
    "password": { "password": "<request-memory-only>" }
  },
  "lifetime": "43200s"
}
```

不得通过 Provider 的“按手机号搜索用户”作为主登录路径。

- [ ] **Step 4: 设置密码使用 v4.17.1 User API**

实现必须从 v4.17.1 `UserService.UpdateUser` 或 `SetPassword` 的正式 proto 生成精确请求测试，且满足：

```text
只使用 Provisioning Token
passwordChangeRequired=false
密码只存在于调用栈内存
Provider 返回体不进入日志
404、无密码、错误密码不向公共层泄露差异
```

- [ ] **Step 5: Session Proof 是唯一认证证明**

```go
type SessionProof struct {
    UserID             string
    OrganizationID     string
    UserVerifiedAt     time.Time
    OTPSMSVerifiedAt   time.Time
    PasswordVerifiedAt time.Time
}
```

OTP 流要求 `UserVerifiedAt` 与 `OTPSMSVerifiedAt`；密码流要求 `UserVerifiedAt` 与 `PasswordVerifiedAt`。

- [ ] **Step 6: Run and commit**

```powershell
go test ./internal/authruntime/zitadel/loginv2 -count=1
git add internal/authruntime/zitadel/loginv2
git commit -m "feat: extend ZITADEL login v2 account entry client"
```

---

## Task 3：增加账号入口配置与启动失败关闭

**Files:**
- Create: `internal/core/config/type_account_entry.go`
- Create: `internal/core/config/validate_account_entry.go`
- Create: `internal/core/config/validate_account_entry_test.go`
- Modify: `internal/core/config/config.go`
- Modify: `internal/core/config/loader_builder.go`
- Modify: `internal/core/config/config_env_test.go`
- Create: `web/listingkit-ui/src/lib/server/account-entry-config.ts`
- Create: `web/listingkit-ui/src/lib/server/account-entry-config.test.ts`
- Modify: `.env.example`

**Interfaces:**
- Produces Go `config.AccountEntryConfig` and Next.js `AccountEntryServerConfig`。

- [ ] **Step 1: 写失败的 Go 配置测试**

覆盖：

```text
Enabled=false → 其他字段允许为空
Enabled=true → Database、Redis、IssuerURL、ProjectID、LoginClientToken、ProvisioningToken 必填
BFFToken 至少 32 字节
FlowAEADKey Base64 解码后恰好 32 字节
PhoneHMACKey Base64 解码后至少 32 字节
PublicOrigin 外网必须 HTTPS，loopback 可 HTTP
FlowTTL > ChallengeTTL
ResendCooldown >= 60s
MaxVerificationAttempts ∈ [1,10]
限流阈值全部为正数
```

- [ ] **Step 2: 定义配置**

```go
type AccountEntryConfig struct {
    Enabled                 bool
    SelfRegistrationEnabled bool
    PublicOrigin            string
    BFFToken                string
    FlowAEADKey             string
    PhoneHMACKey            string
    PreviousPhoneHMACKey    string
    PhoneHMACKeyVersion     string
    LoginClientToken        string
    ProvisioningToken       string
    FlowTTL                 time.Duration
    ChallengeTTL            time.Duration
    ResendCooldown          time.Duration
    ShortSessionLifetime    time.Duration
    RememberSessionLifetime time.Duration
    MaxVerificationAttempts int
    PerPhoneHourly          int64
    PerIPHourly             int64
    PerIPDaily              int64
    PasswordFailureHourly   int64
}
```

默认值：

```text
enabled=false
selfRegistrationEnabled=false
flowTTL=24h
challengeTTL=5m
resendCooldown=60s
shortSessionLifetime=12h
rememberSessionLifetime=720h
maxVerificationAttempts=5
perPhoneHourly=5
perIPHourly=20
perIPDaily=100
passwordFailureHourly=10
```

- [ ] **Step 3: 绑定环境变量并测试脱敏**

所有 Go 变量使用 `TASK_PROCESSOR_ACCOUNT_ENTRY_*`；前端只读取无 `NEXT_PUBLIC_` 前缀的：

```text
ACCOUNT_ENTRY_ENABLED
ACCOUNT_ENTRY_SELF_REGISTRATION_ENABLED
ACCOUNT_ENTRY_BFF_TOKEN
ACCOUNT_ENTRY_PUBLIC_ORIGIN
LISTINGKIT_SERVICE_API_BASE
```

缺少配置时 BFF 返回 `503 ACCOUNT_ENTRY_NOT_CONFIGURED`，错误文本不包含任何 Secret。

- [ ] **Step 4: Run and commit**

```powershell
go test ./internal/core/config -run AccountEntry -count=1
Set-Location web/listingkit-ui
pnpm test -- src/lib/server/account-entry-config.test.ts
Set-Location ../..
git add internal/core/config web/listingkit-ui/src/lib/server/account-entry-config* .env.example
git commit -m "feat: add account entry configuration"
```

---

## Task 4：建立持久化 Flow、手机号身份绑定与安全原语

**Files:**
- Create: `internal/accountentry/domain.go`
- Create: `internal/accountentry/domain_test.go`
- Create: `internal/accountentry/repository.go`
- Create: `internal/accountentry/gorm_repository.go`
- Create: `internal/accountentry/gorm_repository_test.go`
- Create: `internal/accountentry/phone_binding.go`
- Create: `internal/accountentry/phone_binding_test.go`
- Create: `internal/accountentry/phone_fingerprint.go`
- Create: `internal/accountentry/phone_fingerprint_test.go`
- Create: `internal/accountentry/flow_cipher.go`
- Create: `internal/accountentry/flow_cipher_test.go`
- Create: `internal/accountentry/rate_limiter.go`
- Create: `internal/accountentry/rate_limiter_test.go`
- Modify: `internal/platform/redis/client.go`
- Modify: `internal/platform/redis/client_test.go`
- Modify: `internal/workbench/schema/runtime.go`
- Modify: `internal/workbench/schema/runtime_test.go`

**Interfaces:**
- Produces `FlowRepository`、`PhoneBindingRepository`、`FlowCipher`、`RateLimiter`。

- [ ] **Step 1: 定义 Flow 状态机**

```go
type FlowKind string
const (
    FlowRegister      FlowKind = "register"
    FlowOTPLogin      FlowKind = "otp_login"
    FlowPasswordLogin FlowKind = "password_login"
    FlowPasswordReset FlowKind = "password_reset"
    FlowInvitation    FlowKind = "invitation"
)

type FlowState string
const (
    FlowCreated          FlowState = "created"
    FlowChallengeSent    FlowState = "challenge_sent"
    FlowIdentityVerified FlowState = "identity_verified"
    FlowProvisioning     FlowState = "provisioning"
    FlowPasswordPending  FlowState = "password_pending"
    FlowCallbackReady    FlowState = "callback_ready"
    FlowCompleted        FlowState = "completed"
    FlowFailed           FlowState = "failed"
    FlowExpired          FlowState = "expired"
)
```

允许迁移必须由表驱动测试锁定；终态不可恢复。

- [ ] **Step 2: Flow 表**

```text
account_entry_flows
- id
- kind
- state
- phone_fingerprint
- phone_key_version
- encrypted_provider_state
- auth_request_fingerprint
- invitation_id
- user_id
- home_organization_id
- target_organization_id
- agreement_version
- remember_session
- verification_attempts
- challenge_expires_at
- expires_at
- version
- created_at
- updated_at
```

`Save` 使用 `WHERE id=? AND version=?` 乐观锁。

- [ ] **Step 3: 手机号绑定表**

```text
account_phone_bindings
- phone_fingerprint
- phone_key_version
- zitadel_user_id
- home_organization_id
- origin
- proof_verified_at
- created_at
- updated_at
```

约束：

```text
UNIQUE(phone_fingerprint, phone_key_version)
UNIQUE(zitadel_user_id)
```

绑定代表“一条手机身份已通过 ZITADEL OTP Session Proof”，不是单纯 `phone.isVerified` 字段。

- [ ] **Step 4: HMAC 与密文**

```go
func NormalizePhone(countryCode, nationalNumber string) (string, error)
func PhoneFingerprint(key []byte, phone string) string
```

- E.164；默认 UI 国家码为 `+86`。
- HMAC-SHA-256；数据库不保存明文手机号。
- `ProviderState` 使用 AES-256-GCM；AAD 包含 Flow ID、Kind、Key Version。
- 允许加密保存 phone、Session ID、Session Token；禁止保存验证码和密码。
- 完成或过期后清空密文。

- [ ] **Step 5: Redis 原子窗口计数**

```go
type WindowCounter interface {
    IncrementWindow(context.Context, string, time.Duration) (int64, error)
}
```

Lua 原子执行 `INCR`，首次时设置 `EXPIRE`。IP 先用独立 HMAC 变换，Redis Key 不包含原始 IP 或手机号。

Redis 不可用时，挑战发送和验证 fail closed。

- [ ] **Step 6: Run and commit**

```powershell
go test ./internal/accountentry -run "Flow|Binding|Phone|Cipher|Rate" -count=1
go test ./internal/platform/redis -run Window -count=1
go test ./internal/workbench/schema -count=1
git add internal/accountentry internal/platform/redis internal/workbench/schema
git commit -m "feat: add durable account entry state"
```

---

## Task 5：增加 `base_payg` 与幂等注册初始化器

**Files:**
- Modify: `internal/listingsubscription/types.go`
- Modify: `internal/listingsubscription/service.go`
- Modify: `internal/listingsubscription/service_test.go`
- Create: `internal/accountentry/initializer.go`
- Create: `internal/accountentry/initializer_test.go`

**Interfaces:**
- Consumes: Task 2 `loginv2.AccountEntryClient`、现有 `listingsubscription.Service`。
- Produces:

```go
type AccountInitializer interface {
    EnsureOrganizationOwner(context.Context, string, string) error
    EnsureListingKitAuthorization(context.Context, string, string, string) (string, error)
    EnsureBasePayg(context.Context, string, string) error
}
```

- [ ] **Step 1: 写 `base_payg` 失败测试**

```go
const PlanBasePayAsYouGo = "base_payg"
```

默认计划：

```text
名称：基础方案 · 按需使用
store_count：1
store_renewal_periods：0
ai_points：0
data_rows：0
状态：active
无试用期、无虚构到期日
```

旧 `basic / professional / enterprise` 保留给历史租户；新自助注册只使用 `base_payg`。

- [ ] **Step 2: 初始化顺序**

```text
OTP Session Proof 已验证
→ Ensure Organization Owner
→ Ensure ListingKit project grant invariant
→ Ensure listingkit_admin authorization
→ Ensure base_payg
→ 才允许 CreateCallback
```

每一步记录独立完成标记。Provider 冲突必须读回确认，不得把 409 一律当成功。

- [ ] **Step 3: 邀请注册规则**

```text
不创建新 Organization
不创建新 base_payg
不授予 ORG_OWNER
只确保邀请指定项目角色
```

- [ ] **Step 4: Run and commit**

```powershell
go test ./internal/listingsubscription -run BasePay -count=1
go test ./internal/accountentry -run Initializer -count=1
git add internal/listingsubscription internal/accountentry/initializer*
git commit -m "feat: add base payg account initialization"
```

---

## Task 6：实现一次性手机号邀请 Token

**Files:**
- Create: `internal/accountentry/invitation.go`
- Create: `internal/accountentry/invitation_test.go`
- Create: `internal/accountentry/invitation_repository.go`
- Create: `internal/accountentry/invitation_gorm.go`
- Create: `internal/accountentry/invitation_gorm_test.go`
- Create: `internal/accountentry/invitation_issuer.go`
- Create: `internal/accountentry/invitation_issuer_test.go`
- Modify: `internal/workbench/schema/runtime.go`
- Modify: `internal/workbench/schema/runtime_test.go`

**Interfaces:**
- Produces `InvitationIssuer`、`InvitationConsumer`。

- [ ] **Step 1: 定义持久化结构**

```text
account_entry_invitations
- id
- token_hash
- organization_id
- role
- phone_fingerprint
- phone_key_version
- state
- expires_at
- consuming_by_flow_id
- consumed_by_user_id
- consumed_at
- version
- created_by
- idempotency_key
- created_at
```

原始 Token 使用 32 字节 CSPRNG、base64url 无填充，只在首次签发响应返回；数据库只存 SHA-256。

- [ ] **Step 2: 角色和作用域**

允许：

```text
listingkit_viewer
listingkit_operator
listingkit_admin
```

签发接口使用 Effective Organization，不能接受浏览器提供的任意企业作为授权依据。

- [ ] **Step 3: 并发消费**

两个并发消费只能一个进入 `consuming`。手机号不匹配、已过期、已撤销、已消费统一返回 `INVITATION_UNAVAILABLE`。

同一已绑定用户可获得目标企业的项目授权；不得为该手机号创建第二个 ZITADEL 用户。

- [ ] **Step 4: Run and commit**

```powershell
go test ./internal/accountentry -run Invitation -count=1
go test ./internal/workbench/schema -count=1
git add internal/accountentry/invitation* internal/workbench/schema
git commit -m "feat: add phone account invitations"
```

---

## Task 7：实现注册与验证码登录生产 Saga

**Files:**
- Create: `internal/accountentry/service.go`
- Create: `internal/accountentry/service_test.go`
- Create: `internal/accountentry/reconciler.go`
- Create: `internal/accountentry/reconciler_test.go`
- Create: `internal/accountentry/audit.go`
- Create: `internal/accountentry/audit_test.go`

**Interfaces:**
- Consumes: `FlowRepository`、`PhoneBindingRepository`、`RateLimiter`、`FlowCipher`、`loginv2.AccountEntryClient`、`AccountInitializer`、`InvitationConsumer`。
- Produces:

```go
type Service interface {
    StartChallenge(context.Context, ChallengeRequest) (ChallengeResult, error)
    ResendChallenge(context.Context, ResendRequest) (ChallengeResult, error)
    VerifyOTP(context.Context, VerifyOTPRequest) (VerifyResult, error)
    CompleteOIDC(context.Context, CompleteRequest) (Completion, error)
}
```

- [ ] **Step 1: StartChallenge 表驱动测试**

覆盖：

```text
已绑定手机号 + register → 作为 OTP 登录，不创建企业
已绑定手机号 + otp_login → 给既有 userID 建 Challenge
未绑定手机号 + register + 自助注册开启 → 创建 inert org/user，再建 Challenge
未绑定手机号 + otp_login → 相同公共响应，但不创建账号
邀请 Token + 已绑定手机号 → 目标企业授权 Flow
邀请 Token + 新手机号 → 在目标企业创建一个用户，不创建额外企业
协议未勾选或版本缺失 → 注册拒绝
限流、冷却、幂等重放
```

- [ ] **Step 2: 复用已验证 SMS Session 逻辑**

正式 Saga 直接调用 Task 1 抽取的：

```go
AddOTPSMS
CreateSMSChallenge
VerifySMS
GetSession
```

不得新增第二个短信 Provider 或验证码比较函数。

- [ ] **Step 3: 验证成功条件**

```text
SessionProof.UserID == Flow.UserID
SessionProof.OrganizationID == Flow.HomeOrganizationID
UserVerifiedAt 非零
OTPSMSVerifiedAt 非零
Challenge 未过期
尝试次数未超限
```

满足后才写入 `account_phone_bindings.proof_verified_at`。

- [ ] **Step 4: 注册最终化**

直接注册按 Task 5 顺序幂等执行；邀请注册只确保邀请角色。任何初始化步骤失败时 Flow 保持 `provisioning`，Reconciler 从实际 Provider 状态继续，不重新发短信，也不延长原始权益时间。

- [ ] **Step 5: OIDC 完成**

`CreateOIDCCallback` 成功后，将 callback URL 加密存入 Flow，状态改为 `callback_ready`。`CompleteOIDC` 只向通过 BFF Token 的服务端调用返回一次 callback URL，然后原子改为 `completed` 并清空密文。

- [ ] **Step 6: 清理**

未验证注册 Flow 24 小时后，仅删除可证明由该 Flow 创建且没有角色、订阅和成功 Session 的资源；不确定资源进入人工隔离状态，不能猜测删除。

- [ ] **Step 7: 审计字段**

允许：Flow ID、Kind、状态、指纹后缀、User/Org ID 后缀、结果、reason code、request ID。禁止：手机号、验证码、Session Token、callback URL、Provider body。

- [ ] **Step 8: Run and commit**

```powershell
go test ./internal/accountentry -run "Challenge|Register|OTP|Complete|Reconcile|Audit" -count=1
git add internal/accountentry/service* internal/accountentry/reconciler* internal/accountentry/audit*
git commit -m "feat: productize phone registration and otp login"
```

---

## Task 8：实现手机号密码登录和短信证明后的密码重置

**Files:**
- Create: `internal/accountentry/password.go`
- Create: `internal/accountentry/password_test.go`
- Modify: `internal/accountentry/service.go`
- Modify: `internal/accountentry/service_test.go`

**Interfaces:**
- Produces `AuthenticatePassword`、`StartPasswordReset`、`VerifyPasswordResetOTP`、`CompletePasswordReset`。

- [ ] **Step 1: 密码登录**

```text
手机号规范化 → HMAC Binding → userID
→ ZITADEL Session user + password check
→ 验证 user + password factors
→ CreateOIDCCallback
→ callback_ready
```

不存在用户、未设置密码、密码错误统一为：

```text
INVALID_CREDENTIALS
账号或密码不正确
```

密码以 `[]byte` 接收，调用完成后覆盖；日志和错误不得包含长度、内容或策略细节。

- [ ] **Step 2: 密码重置**

```text
Start → 相同公共响应
真实绑定 → ZITADEL SMS Challenge
Verify → user + otpSms Proof
Complete → SetPassword → Flow completed
→ 返回 /login?method=password
```

不存在用户不发送短信，执行等时假流程；不得泄露是否存在。

- [ ] **Step 3: 密码策略**

前端只做基础一致性检查，ZITADEL 是最终策略权威。Provider 拒绝映射为 `PASSWORD_POLICY_REJECTED`，不透传内部规则 ID。

- [ ] **Step 4: Run and commit**

```powershell
go test ./internal/accountentry -run Password -count=1
git add internal/accountentry/password* internal/accountentry/service*
git commit -m "feat: add phone password authentication"
```

---

## Task 9：暴露受 Trusted BFF 保护的 Account Entry HTTP API

**Files:**
- Create: `internal/accountentry/httpapi/module.go`
- Create: `internal/accountentry/httpapi/handler.go`
- Create: `internal/accountentry/httpapi/handler_test.go`
- Create: `internal/accountentry/httpapi/contract.go`
- Create: `internal/accountentry/httpapi/contract_test.go`
- Modify: `internal/httproute/descriptor.go`
- Modify: `internal/app/httpapi/server_auth.go`
- Modify: `internal/app/httpapi/server_test.go`

**Interfaces:**
- Produces BFF-only Routes 和已认证邀请签发 Route。

- [ ] **Step 1: 新增策略**

```go
const AuthPolicyTrustedBFF AuthPolicy = "trusted_bff"
```

以常量时间比较 `Authorization: Bearer <BFF_TOKEN>`；该策略不运行用户 Bearer 验证，并删除所有伪造的用户／企业身份头。

- [ ] **Step 2: 路由**

```text
POST /api/v1/account-entry/challenges
POST /api/v1/account-entry/challenges/resend
POST /api/v1/account-entry/verifications
POST /api/v1/account-entry/password-logins
POST /api/v1/account-entry/password-resets/challenges
POST /api/v1/account-entry/password-resets/verifications
POST /api/v1/account-entry/password-resets/completions
POST /api/v1/account-entry/oidc-completions

POST /api/v1/workbench/account-invitations
```

最后一条使用 `VerifiedIdentity + LiveWrite + account.invitation.create`。

- [ ] **Step 3: 输入边界**

```text
Content-Type application/json
Body <= 16 KiB
15s 读取超时
拒绝未知字段和重复 JSON Key
所有写请求要求 canonical UUID Idempotency-Key
只信任已配置代理来源提供的 Client IP
```

- [ ] **Step 4: 错误 Envelope**

```json
{
  "error": {
    "code": "INVALID_CODE",
    "message": "验证码无效或已过期",
    "retryAfterSeconds": 0,
    "fieldErrors": {}
  }
}
```

锁定错误码：

```text
INVALID_REQUEST
ACCOUNT_ENTRY_DISABLED
SELF_REGISTRATION_DISABLED
INVALID_CODE
CHALLENGE_EXPIRED
TOO_MANY_REQUESTS
INVALID_CREDENTIALS
PASSWORD_POLICY_REJECTED
FLOW_ALREADY_COMPLETED
INVITATION_UNAVAILABLE
DEPENDENCY_UNAVAILABLE
```

- [ ] **Step 5: Run and commit**

```powershell
go test ./internal/accountentry/httpapi -count=1
go test ./internal/app/httpapi -run "TrustedBFF|AccountEntry" -count=1
git add internal/accountentry/httpapi internal/httproute internal/app/httpapi/server*
git commit -m "feat: expose trusted account entry api"
```

---

## Task 10：接入应用组合、数据库和 Redis 生命周期

**Files:**
- Create: `internal/app/httpapi/accountentry_module.go`
- Create: `internal/app/httpapi/accountentry_module_test.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/composition_builder_test.go`
- Modify: `internal/app/httpapi/composition_modules.go`
- Modify: `internal/app/httpapi/types.go`
- Modify: `internal/app/httpapi/bootstrap.go`
- Modify: `internal/app/httpapi/bootstrap_test.go`

**Interfaces:**
- Consumes: Tasks 3–9。
- Produces `accountEntryModule` 和 Reconciler 生命周期。

- [ ] **Step 1: 开关测试**

```text
Enabled=false → module nil，不要求 Account Entry Secrets
Enabled=true → Database、Redis、两个 ZITADEL Token 缺一即启动失败
```

- [ ] **Step 2: 组装顺序**

```text
open shared DB
→ migrate Account Entry tables
→ open Redis client
→ build loginv2 client
→ build repositories/cipher/limiter
→ build initializer/invitation/service
→ build httpapi module
→ start bounded reconciler
```

构造失败必须逆序关闭 DB/Redis；不能留下后台 goroutine。

- [ ] **Step 3: Module 顺序**

Account Entry 公共路由与 Workbench Context/Store Center 一起进入统一 `kernelmodule` 列表，但保持各自 AuthPolicy。

- [ ] **Step 4: Run and commit**

```powershell
go test ./internal/app/httpapi -run AccountEntry -count=1
go test ./internal/workbench/schema -count=1
git add internal/app/httpapi internal/workbench/schema
git commit -m "feat: wire account entry runtime"
```

---

## Task 11：实现 Next.js 同源 BFF、OIDC Intent 和 HttpOnly Flow Cookie

**Files:**
- Create: `web/listingkit-ui/src/lib/server/account-entry-bff.ts`
- Create: `web/listingkit-ui/src/lib/server/account-entry-bff.test.ts`
- Create: `web/listingkit-ui/src/lib/server/account-entry-cookie.ts`
- Create: `web/listingkit-ui/src/lib/server/account-entry-cookie.test.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/begin/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/challenge/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/resend/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/verify/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/password-login/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/password-reset/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/complete/route.ts`
- Modify: `web/listingkit-ui/src/app/api/zitadel-auth/login/route.ts`
- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.ts`
- Modify: `web/listingkit-ui/src/proxy.ts`

**Interfaces:**
- Produces浏览器同源 Account Entry API 和 `303` 完成跳转。

- [ ] **Step 1: OIDC Begin**

`GET /api/account-entry/begin` 接受：

```text
screen=login|register
method=otp|password
returnTo=/workbench/...
invite=<optional one-time token>
```

流程：

```text
验证 returnTo
→ 将 screen/method/returnTo 和邀请句柄写入加密 HttpOnly intent Cookie
→ 调用现有 signIn("zitadel")
→ ZITADEL Custom Login 回到 /login?authRequest=...
```

邀请原始 Token 首次请求后必须从地址栏移除，不写 localStorage。

- [ ] **Step 2: Login 路由分流**

`/login` 无 `authRequest` 时启动 Begin；有 `authRequest` 时读取 intent Cookie：

```text
screen=register → redirect /register?authRequest=...
screen=login → 渲染对应 method
```

`authRequest` 只在服务端和表单隐藏字段中使用，不写日志或分析事件。

- [ ] **Step 3: Flow Cookie**

```text
name=shuomi_account_entry_flow
HttpOnly
Secure（loopback 开发除外）
SameSite=Lax
Path=/
Max-Age <= FlowTTL
```

Cookie 只含随机 Flow ID 和 MAC，不含手机号、Session Token、callback URL 或邀请 Token。

- [ ] **Step 4: BFF 转发**

每个 Route Handler：

```text
读取并验证 Origin
构建服务端 Idempotency-Key
使用 ACCOUNT_ENTRY_BFF_TOKEN 调 Go API
限制上游响应 <= 64 KiB
仅转发白名单字段
不转发 Set-Cookie、Location、Provider headers
```

- [ ] **Step 5: Complete**

`POST /api/account-entry/complete` 从 Go 获取一次性 callback URL，验证 Origin/Path 后直接 `303`；callback URL 不进入 JSON 或客户端状态。

- [ ] **Step 6: Run and commit**

```powershell
Set-Location web/listingkit-ui
pnpm test -- src/lib/server/account-entry src/app/api/account-entry
pnpm typecheck
Set-Location ../..
git add web/listingkit-ui/src/lib/server/account-entry* `
        web/listingkit-ui/src/app/api/account-entry `
        web/listingkit-ui/src/app/api/zitadel-auth/login/route.ts `
        web/listingkit-ui/src/lib/server/zitadel-auth.ts `
        web/listingkit-ui/src/proxy.ts
git commit -m "feat: add account entry bff"
```

---

## Task 12：按 Figma 实现注册、登录与重置密码页面

**Files:**
- Create: `web/listingkit-ui/src/components/auth-entry/auth-entry-shell.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/auth-brand-panel.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/auth-card.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/phone-field.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/otp-field.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/password-field.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/register-form.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/login-form.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/reset-password-form.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/auth-entry.test.tsx`
- Create: `web/listingkit-ui/src/app/register/page.tsx`
- Replace: `web/listingkit-ui/src/app/login/page.tsx`
- Create: `web/listingkit-ui/src/app/forgot-password/page.tsx`
- Modify: `web/listingkit-ui/src/components/application-frame.tsx`
- Modify: `web/listingkit-ui/src/app/globals.css`

**Interfaces:**
- Consumes: Task 11 same-origin routes。
- Produces Figma nodes `374:325`、`373:303`、`378:307`、`1266:359` 的产品化页面。

- [ ] **Step 1: 写 UI 合约测试**

注册页严格只有：

```text
手机号码
短信验证码
协议复选框
注册并进入
已有账号？立即登录
```

断言不存在：

```text
用户名输入框
注册密码输入框
邮箱输入框
```

登录：

```text
密码登录：手机号 + 密码 + 记住登录状态 + 忘记密码
验证码登录：手机号 + 验证码
```

重置：

```text
手机号 + 验证码 + 新密码 + 确认新密码
成功后返回密码登录
```

- [ ] **Step 2: 共享 Shell**

桌面双栏，移动端单列；品牌、返回官网、AI 光场和卡片统一复用。禁止复制 Figma 绝对定位代码。

- [ ] **Step 3: 表单行为**

```text
OTP 60 秒显示倒计时
真正冷却由服务端决定
重复提交禁用
失败后清除验证码和密码
returnTo 在服务器白名单内传递
不使用 localStorage/sessionStorage
```

- [ ] **Step 4: 可访问性**

```text
每个 input 有 label
错误通过 aria-describedby 关联
登录方式使用 tablist/tab/tabpanel
倒计时使用克制的 aria-live=polite
密码可见切换有明确名称
键盘可完成全部流程
prefers-reduced-motion 时关闭光场动画
320px 无横向滚动
```

- [ ] **Step 5: Run and commit**

```powershell
Set-Location web/listingkit-ui
pnpm test -- src/components/auth-entry src/app/login src/app/register src/app/forgot-password
pnpm lint
pnpm typecheck
pnpm build
Set-Location ../..
git add web/listingkit-ui/src/components/auth-entry `
        web/listingkit-ui/src/app/login `
        web/listingkit-ui/src/app/register `
        web/listingkit-ui/src/app/forgot-password `
        web/listingkit-ui/src/components/application-frame.tsx `
        web/listingkit-ui/src/app/globals.css
git commit -m "feat: implement Shuomi account entry pages"
```

---

## Task 13：配置 ZITADEL Custom Login、Secret 和发布开关

**Files:**
- Modify: `deployments/kubernetes/zitadel/local/README.md`
- Modify: `deployments/kubernetes/zitadel/local/values.yaml`
- Modify: `deployments/kubernetes/listingkit-workbench/base/listingkit-ui-auth-config.yaml`
- Modify: `deployments/kubernetes/listingkit-workbench/base/listingkit-ui-deployment.yaml`
- Modify: `deployments/kubernetes/listingkit-workbench/base/product-listing-api-deployment.yaml`
- Create: `deployments/kubernetes/listingkit-workbench/base/account-entry-secret.example.yaml`
- Create: `docs/runbooks/shuomi-account-entry.md`
- Create: `tests/account_entry_commercial_readiness_test.go`

- [ ] **Step 1: Secret 合约**

```yaml
stringData:
  bff-token: ""
  flow-aead-key: ""
  phone-hmac-key: ""
  previous-phone-hmac-key: ""
  zitadel-login-client-token: ""
  zitadel-provisioning-token: ""
```

示例 Secret 不加入 base `kustomization.yaml`；真实值由环境 Overlay 或 Secret Manager 注入。

- [ ] **Step 2: Custom Login URL**

预发布 ZITADEL 应用指向：

```text
https://<staging-origin>/login
```

回滚时恢复旧 Login URL；不删除已经完成注册的用户、企业、角色和订阅。

- [ ] **Step 3: 开关顺序**

```text
1. ACCOUNT_ENTRY_ENABLED=true, SELF_REGISTRATION=false
2. 验证既有用户 OTP/密码登录
3. SELF_REGISTRATION=true 仅预发布
4. 完成真实新注册、邀请和补偿验收
5. 再开放生产
```

- [ ] **Step 4: 商业就绪静态检查**

测试扫描：

```text
无 NEXT_PUBLIC_* Secret
无硬编码 Token/手机号/验证码
示例 Secret 未进入 base resources
Account Entry 关闭时原有 OIDC 登录仍可回滚使用
公开路由均为 TrustedBFF
```

- [ ] **Step 5: Run and commit**

```powershell
go test ./tests -run AccountEntry -count=1
kubectl kustomize deployments/kubernetes/listingkit-workbench/overlays/staging > $null
kubectl kustomize deployments/kubernetes/listingkit-workbench/overlays/prod > $null
git add deployments/kubernetes docs/runbooks/shuomi-account-entry.md tests/account_entry_commercial_readiness_test.go
git commit -m "ops: configure Shuomi account entry"
```

---

## Task 14：全量自动化、真实预发布验收与发布报告

**Files:**
- Create: `web/listingkit-ui/e2e/account-entry.spec.ts`
- Modify: `web/listingkit-ui/e2e/accessibility.spec.ts`
- Create: `scripts/verify-shuomi-account-entry-release.ps1`
- Create: `docs/verification/2026-09-03-shuomi-account-entry-release.md`

- [ ] **Step 1: 自动化 E2E**

使用可控 Provider Stub 覆盖：

```text
/register 字段与协议门槛
OTP 登录
密码登录
密码重置后返回密码登录
合法和非法 returnTo
验证码错误、过期、限流
重复提交与 Flow replay
邀请注册不创建额外企业
callback URL 不出现在浏览器 JSON
Cookie 为 HttpOnly/Secure/SameSite
320px 和 1440×900
axe 可访问性
```

- [ ] **Step 2: 全量回归**

```powershell
go test ./... -count=1
go test ./tests -count=1
Set-Location web/listingkit-ui
pnpm test
pnpm lint
pnpm typecheck
pnpm build
pnpm test:e2e
pnpm test:a11y
Set-Location ../..
kubectl kustomize deployments/kubernetes/listingkit-workbench/overlays/staging > $null
kubectl kustomize deployments/kubernetes/listingkit-workbench/overlays/prod > $null
git diff --check
```

- [ ] **Step 3: 真实预发布验收**

复用 PR #218 已证明的短信基础，不再重新证明“ZITADEL 能发 OTP”。本次真实验收只验证新增产品化闭环：

```text
已有手机号 OTP 登录 → Auth.js Session
已有密码手机号登录 → Auth.js Session
新手机号注册 → 仅一条 OTP → 一个用户 → 一个默认企业 → ORG_OWNER → listingkit_admin → base_payg
同一手机号重复注册 → 不新增用户或企业
邀请注册 → 加入目标企业 → 不创建默认企业
密码重置 → 新密码可登录
日志与响应无手机号、验证码、密码、Session Token、callback URL
失败中断后 Reconciler 能恢复或安全隔离
```

- [ ] **Step 4: 发布报告**

记录：

```text
Git SHA
ZITADEL 版本
配置版本
全部命令和退出码
真实短信条数
注册/OTP/密码/重置/邀请结果
创建对象数量核对
敏感日志扫描
回滚演练
剩余风险
```

所有 ID 仅保留末 6 位，不记录手机号和 callback URL。

- [ ] **Step 5: Commit**

```powershell
git add web/listingkit-ui/e2e `
        scripts/verify-shuomi-account-entry-release.ps1 `
        docs/verification/2026-09-03-shuomi-account-entry-release.md
git commit -m "test: verify Shuomi account entry release"
```

---

## 完成定义

Slice 1 只有同时满足以下条件才算完成：

- [ ] PR #218 的手机号 OTP Client 被抽取复用，没有第二套短信或 Session HTTP 实现。
- [ ] 现有 `phoneonboardingpreflight` 和真实验证证据继续有效。
- [ ] 注册页面只要求手机号、验证码和协议。
- [ ] OTP 登录和手机号密码登录均通过同一个 Auth.js 会话闭环。
- [ ] ZITADEL 手机引导状态不被当作授权证明；只有 Session `user + otpSms` Proof 可推进注册。
- [ ] 直接注册幂等创建一个用户、一个默认企业、ORG_OWNER、`listingkit_admin` 和 `base_payg`。
- [ ] 邀请注册加入目标企业，不创建额外用户或默认企业。
- [ ] 密码重置完成后返回密码登录，业务数据库不保存密码或哈希。
- [ ] 手机号、验证码、密码、Session Token、callback URL 和管理凭据不进入浏览器 JSON、日志或明文数据库列。
- [ ] 公开 API 只允许可信 Next.js BFF 调用。
- [ ] 注册、登录、重置和邀请支持幂等、限流、过期、并发重放和失败补偿。
- [ ] `/register`、`/login`、`/forgot-password` 在 1440×900 和 320px 宽度下可用。
- [ ] Go、前端、Playwright、Kustomize 和商业就绪检查全部通过。
- [ ] 生产开关默认关闭；真实预发布产品化闭环验收后才开放。

## 与上一版计划的差异

```text
删除：重新实现手机号 OTP、重新证明短信事件、重新编写 preflight Client
保留：PR #218 已合并能力和非生产证据
新增：把已验证 Client 抽到中性包供生产复用
修正：不要求将 ZITADEL 手机字段先设为未验证再二次标记
修正：Session user + otpSms Proof 才是业务授权证明
调整：真实设备验收移到最终发布门槛，只验证产品化增量
```

Slice 1 稳定合并后，再单独编写 `Slice 2：新 Console Shell` 实施计划。
