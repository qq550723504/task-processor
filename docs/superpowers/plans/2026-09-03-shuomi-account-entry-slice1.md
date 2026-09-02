# 硕米账号入口 Slice 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `task-processor` 中交付手机号验证码注册、手机号验证码登录、手机号密码登录、密码重置和邀请注册闭环，并通过 ZITADEL Login V2 Session/OIDC Auth Request 建立现有 Auth.js 会话。

**Architecture:** 继续以 ZITADEL 作为唯一身份权威。现有 `web/listingkit-ui` 承担硕米账号入口的可视页面和同源 BFF；新增 Go `internal/accountentry` 模块承担流程状态、手机号指纹、限流、邀请、ZITADEL Session/OIDC 适配和账号初始化。浏览器只持有 HttpOnly 流程 Cookie，不接触 ZITADEL 管理凭据、Session Token 或 OIDC callback URL；完成认证时由 Next.js BFF 调用 Go 服务并执行受控 `303` 跳转进入现有 Auth.js callback。

**Tech Stack:** Go 1.26、Gin/kernel module、GORM、Redis、ZITADEL Core/Login V2 v4.17.1、Next.js 16 App Router、React 19、Auth.js 5、TypeScript、Zod、Tailwind CSS 4、Vitest、Playwright。

---

## 执行前提

- 先合并或检出已批准规格：`docs/superpowers/specs/2026-09-02-shuomi-console-phase1-hard-cut-design.md`。
- 本计划只实现 Slice 1。不要顺手实现 Console Shell、账户资料、企业权益、店铺中心、企业钱包、自定义角色或认证审核。
- 所有 ZITADEL REST 合约锁定到仓库已验证的 v4.17.1。协议路径或字段与真实环境不一致时，先更新协议门槛和规格，不用猜测兼容写法。
- 每个 Task 都按 TDD 执行：先写失败测试并确认失败原因，再做最小实现，再运行目标测试和相邻回归测试。
- 每个 Task 单独提交；不要把整份计划压成一个不可审查的提交。

## 目标流程

```text
普通注册 / OTP 登录 / 密码登录
        ↓
Next.js 可视账号入口
        ↓ same-origin BFF
Go Account Entry 服务
        ↓
ZITADEL Session V2
        ↓
OIDC Auth Request CreateCallback
        ↓ callback URL 只返回给 BFF
Next.js 303 Redirect
        ↓
Auth.js /api/auth/callback/zitadel
        ↓
/workbench 或合法 returnTo
```

## 安全不变量

```text
浏览器 JavaScript 不接触：
- ZITADEL Login Client Token
- ZITADEL Provisioning Token
- ZITADEL Session Token
- OIDC callback URL / authorization code
- 明文持久化手机号
- 密码或短信验证码日志

公开响应不泄露：
- 手机号是否存在
- 账号是否设置密码
- ZITADEL User ID / Organization ID
- Provider 原始错误体
```

---

## Task 1：完成生产安全的 ZITADEL Login V2 真实设备门槛

**目的：** 先证明当前自托管 ZITADEL v4.17.1 能完成“未验证手机号 → 只发送一条 OTP SMS → 验证 Session → 事后标记手机号已验证 → 绑定 OIDC Auth Request → Auth.js 会话”的完整链路。

**Files:**
- Modify: `internal/listingkit/phoneonboardingpreflight/zitadel_client.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/zitadel_client_test.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/runner.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/runner_test.go`
- Modify: `hack/debug/listingkit-phone-onboarding-preflight/main.go`
- Create: `web/listingkit-ui/src/app/auth-entry-preflight/page.tsx`
- Create: `web/listingkit-ui/src/app/auth-entry-preflight/page.test.tsx`
- Create: `web/listingkit-ui/src/lib/server/auth-entry-preflight.ts`
- Create: `web/listingkit-ui/src/lib/server/auth-entry-preflight.test.ts`
- Modify: `web/listingkit-ui/src/components/application-frame.tsx`
- Create: `scripts/verify-shuomi-account-entry-login-v2.ps1`
- Create: `docs/verification/2026-09-03-shuomi-account-entry-login-v2.md`

### Step 1：先写失败的协议测试

- [ ] 在 `zitadel_client_test.go` 增加以下失败测试：

```go
func TestCreateTechnicalUserCanKeepPhoneUnverified(t *testing.T)
func TestMarkPhoneVerifiedUsesPrivilegedUserUpdate(t *testing.T)
func TestGetOIDCAuthRequestUsesLoginClientCredential(t *testing.T)
func TestCreateOIDCCallbackSendsSessionMaterialAndReturnsCallbackURL(t *testing.T)
func TestCreateOIDCCallbackRejectsUnexpectedCallbackOrigin(t *testing.T)
```

- [ ] 将 `TechnicalUserInput` 扩展为：

```go
type TechnicalUserInput struct {
    OrganizationID string
    Username       string
    TechnicalEmail string
    Phone          string
    PhoneVerified  bool
}
```

- [ ] 在测试服务器中断言未验证手机号请求不会发送手机验证邮件或返回验证码：

```json
{
  "phone": {
    "phone": "+8613800000000",
    "isVerified": false
  }
}
```

- [ ] 为 OIDC callback 写精确请求断言：

```http
POST /v2/oidc/auth_requests/{authRequestId}
Authorization: Bearer <login-client-token>
Content-Type: application/json
```

```json
{
  "session": {
    "sessionId": "session-1",
    "sessionToken": "session-token-2"
  }
}
```

- [ ] 运行：

```powershell
go test ./internal/listingkit/phoneonboardingpreflight -run "Test(CreateTechnicalUserCanKeepPhoneUnverified|MarkPhoneVerified|GetOIDCAuthRequest|CreateOIDCCallback)" -count=1
```

- [ ] 预期：测试失败，原因是接口和实现尚不存在。

### Step 2：实现最小协议扩展

- [ ] 在 `Client` 接口增加：

```go
GetOIDCAuthRequest(context.Context, string) (OIDCAuthRequest, error)
CreateOIDCCallback(context.Context, OIDCCallbackInput) (string, error)
MarkPhoneVerified(context.Context, string, string) error
```

- [ ] 增加类型：

```go
type OIDCAuthRequest struct {
    ID           string
    ClientID     string
    RedirectURI  string
    Organization string
}

type OIDCCallbackInput struct {
    AuthRequestID string
    SessionID     string
    SessionToken  string
    AllowedOrigin string
}
```

- [ ] 使用版本锁定端点：

```text
GET  /v2/oidc/auth_requests/{auth_request_id}
POST /v2/oidc/auth_requests/{auth_request_id}
```

- [ ] `CreateOIDCCallback` 只接受回调到 `AllowedOrigin`，且路径属于当前受支持的 Auth.js 回调集合：

```text
/api/zitadel-auth/callback
/api/auth/callback/zitadel
```

拒绝 userinfo、非 HTTPS 外部地址、其他端口、其他路径和 fragment；最终允许集合由测试锁定，不能根据 Provider 返回值动态放宽。

- [ ] Provider 错误中只保留操作名和 HTTP 状态，不读取或记录响应中的敏感内容。

### Step 3：将 Runner 改成生产安全验证顺序

- [ ] Runner 顺序固定为：

```text
创建临时 Organization
→ 创建手机号尚未验证的 Human User
→ 添加 OTP SMS Factor
→ 创建 OTP SMS Challenge
→ 人工输入收到的唯一验证码
→ 验证 Session
→ 再由特权 API 标记手机号已验证
→ 读取 OIDC Auth Request
→ CreateCallback
→ 浏览器访问 callback URL
→ 检查 Auth.js Session
→ 清理 Session 和临时 Organization
```

- [ ] Runner 输出只能包含：

```text
status
step
redacted user id suffix
redacted organization id suffix
request id
```

- [ ] Runner 禁止输出：

```text
手机号
验证码
Session ID
Session Token
OIDC callback URL
Authorization header
```

### Step 4：增加仅预发布可见的 Auth Request 捕获页

- [ ] 新增：

```text
web/listingkit-ui/src/lib/server/auth-entry-preflight.ts
web/listingkit-ui/src/lib/server/auth-entry-preflight.test.ts
```

- [ ] `auth-entry-preflight/page.tsx` 只有同时满足以下条件才渲染：

```text
SHUOMI_AUTH_PREFLIGHT_ENABLED=1
当前请求 Origin 与 SHUOMI_AUTH_PREFLIGHT_ALLOWED_ORIGIN 完全一致
允许的 Origin 不是正式生产 Origin
```

不得依赖 `NODE_ENV` 判断，因为预发布镜像也会使用 production build。

- [ ] 页面显示完整 `authRequest` 供现场操作人员复制到验证脚本；Auth Request ID 本身不得写入日志、埋点或公开 JSON API。页面同时显示末 6 位校验值，便于核对复制结果。

- [ ] 页面不显示 Session ID、Session Token、手机号、验证码或 callback URL。

- [ ] `ApplicationFrame` 将 `/auth-entry-preflight` 视为公共无壳页面；生产部署不设置上述两个预检变量，因此路由返回 `404`。

### Step 5：编写一次性验证脚本

- [ ] `verify-shuomi-account-entry-login-v2.ps1` 使用 `Read-Host` 读取手机号、浏览器页面中显示的 `authRequest` 和短信验证码，调用现有 Go preflight 命令，并在退出时清理环境变量。

- [ ] 成功输出固定为：

```text
[PASS] one_sms_received=true
[PASS] phone_unverified_before_proof=true
[PASS] otp_sms_factor_verified=true
[PASS] phone_verified_after_proof=true
[PASS] oidc_callback_created=true
[PASS] authjs_session_created=true
[PASS] cleanup_completed=true
```

### Step 6：运行真实门槛

- [ ] 运行单元测试：

```powershell
go test ./internal/listingkit/phoneonboardingpreflight ./internal/listingkit/zitadelsms -count=1
```

- [ ] 在非生产 ZITADEL 中将测试应用的 Custom Login URL 临时指向：

```text
${LISTINGKIT_PUBLIC_BASE_URL}/auth-entry-preflight
```

- [ ] 执行：

```powershell
pwsh -File ./scripts/verify-shuomi-account-entry-login-v2.ps1
```

- [ ] 将实际版本、时间、短信条数、状态检查、Auth.js Session 检查和清理结果写入 `docs/verification/2026-09-03-shuomi-account-entry-login-v2.md`，所有标识只保留末 6 位。

- [ ] 如果任一条件不成立，停止执行后续 Task，更新设计规格并重新审阅；不得改成自建验证码数据库或绕过 ZITADEL。

### Step 7：提交

```powershell
git add internal/listingkit/phoneonboardingpreflight `
        hack/debug/listingkit-phone-onboarding-preflight `
        web/listingkit-ui/src/app/auth-entry-preflight `
        web/listingkit-ui/src/lib/server/auth-entry-preflight* `
        web/listingkit-ui/src/components/application-frame.tsx `
        scripts/verify-shuomi-account-entry-login-v2.ps1 `
        docs/verification/2026-09-03-shuomi-account-entry-login-v2.md
git commit -m "test: verify ZITADEL login v2 account entry flow"
```

---

## Task 2：增加账号入口配置与启动时失败关闭

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

### Step 1：先写 Go 配置失败测试

- [ ] 测试以下条件：

```text
AccountEntry.Enabled=false
→ 允许其他字段为空

AccountEntry.Enabled=true
→ Database 必填
→ Redis 必填
→ ListingKit.Zitadel.IssuerURL 必填
→ ListingKit.Zitadel.ProjectID 必填
→ LoginClientToken 必填
→ ProvisioningToken 必填
→ BFFToken 至少 32 字节
→ FlowAEADKey 解码后必须 32 字节
→ PhoneHMACKey 解码后至少 32 字节
→ PublicOrigin 必须是 HTTPS；仅 loopback 允许 HTTP
→ FlowTTL > ChallengeTTL
→ ResendCooldown >= 60 秒
→ MaxVerificationAttempts 在 1..10
→ 所有限流阈值为正数
```

- [ ] 运行并确认失败：

```powershell
go test ./internal/core/config -run AccountEntry -count=1
```

### Step 2：定义配置结构

```go
type AccountEntryConfig struct {
    Enabled                 bool          `yaml:"enabled"`
    SelfRegistrationEnabled bool          `yaml:"selfRegistrationEnabled"`
    PublicOrigin            string        `yaml:"publicOrigin"`
    BFFToken                string        `yaml:"bffToken"`
    FlowAEADKey             string        `yaml:"flowAEADKey"`
    PhoneHMACKey            string        `yaml:"phoneHMACKey"`
    PreviousPhoneHMACKey    string        `yaml:"previousPhoneHMACKey"`
    PhoneHMACKeyVersion     string        `yaml:"phoneHMACKeyVersion"`
    LoginClientToken        string        `yaml:"loginClientToken"`
    ProvisioningToken       string        `yaml:"provisioningToken"`
    FlowTTL                 time.Duration `yaml:"flowTTL"`
    ChallengeTTL            time.Duration `yaml:"challengeTTL"`
    ResendCooldown          time.Duration `yaml:"resendCooldown"`
    ShortSessionLifetime    time.Duration `yaml:"shortSessionLifetime"`
    RememberSessionLifetime time.Duration `yaml:"rememberSessionLifetime"`
    MaxVerificationAttempts int           `yaml:"maxVerificationAttempts"`
    PerPhoneHourly          int64         `yaml:"perPhoneHourly"`
    PerIPHourly             int64         `yaml:"perIPHourly"`
    PerIPDaily              int64         `yaml:"perIPDaily"`
}
```

- [ ] 在 `Config` 增加：

```go
AccountEntry AccountEntryConfig `yaml:"accountEntry"`
```

- [ ] 默认值固定为：

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
```

### Step 3：绑定环境变量

```text
TASK_PROCESSOR_ACCOUNT_ENTRY_ENABLED
TASK_PROCESSOR_ACCOUNT_ENTRY_SELF_REGISTRATION_ENABLED
TASK_PROCESSOR_ACCOUNT_ENTRY_PUBLIC_ORIGIN
TASK_PROCESSOR_ACCOUNT_ENTRY_BFF_TOKEN
TASK_PROCESSOR_ACCOUNT_ENTRY_FLOW_AEAD_KEY
TASK_PROCESSOR_ACCOUNT_ENTRY_PHONE_HMAC_KEY
TASK_PROCESSOR_ACCOUNT_ENTRY_PREVIOUS_PHONE_HMAC_KEY
TASK_PROCESSOR_ACCOUNT_ENTRY_PHONE_HMAC_KEY_VERSION
TASK_PROCESSOR_ACCOUNT_ENTRY_ZITADEL_LOGIN_CLIENT_TOKEN
TASK_PROCESSOR_ACCOUNT_ENTRY_ZITADEL_PROVISIONING_TOKEN
TASK_PROCESSOR_ACCOUNT_ENTRY_FLOW_TTL
TASK_PROCESSOR_ACCOUNT_ENTRY_CHALLENGE_TTL
TASK_PROCESSOR_ACCOUNT_ENTRY_RESEND_COOLDOWN
TASK_PROCESSOR_ACCOUNT_ENTRY_SHORT_SESSION_LIFETIME
TASK_PROCESSOR_ACCOUNT_ENTRY_REMEMBER_SESSION_LIFETIME
TASK_PROCESSOR_ACCOUNT_ENTRY_MAX_VERIFICATION_ATTEMPTS
TASK_PROCESSOR_ACCOUNT_ENTRY_PER_PHONE_HOURLY
TASK_PROCESSOR_ACCOUNT_ENTRY_PER_IP_HOURLY
TASK_PROCESSOR_ACCOUNT_ENTRY_PER_IP_DAILY
```

### Step 4：增加 Next.js 服务端配置读取

```ts
export type AccountEntryServerConfig = {
  serviceBaseUrl: string;
  bffToken: string;
  publicOrigin: string;
  enabled: boolean;
  selfRegistrationEnabled: boolean;
};
```

- [ ] 只读取无 `NEXT_PUBLIC_` 前缀的变量。
- [ ] 缺少 BFF Token、Service URL 或 Public Origin 时，路由返回 `503 ACCOUNT_ENTRY_NOT_CONFIGURED`。
- [ ] 模块加载时不得把密钥打印到异常消息。

### Step 5：验证

```powershell
go test ./internal/core/config -run AccountEntry -count=1
Set-Location web/listingkit-ui
pnpm test -- src/lib/server/account-entry-config.test.ts
Set-Location ../..
```

### Step 6：提交

```powershell
git add internal/core/config web/listingkit-ui/src/lib/server/account-entry-config* .env.example
git commit -m "feat: add account entry configuration"
```

---

## Task 3：建立账号入口领域状态机与持久化模型

**Files:**
- Create: `internal/accountentry/domain.go`
- Create: `internal/accountentry/domain_test.go`
- Create: `internal/accountentry/repository.go`
- Create: `internal/accountentry/gorm_repository.go`
- Create: `internal/accountentry/gorm_repository_test.go`
- Modify: `internal/workbench/schema/runtime.go`
- Modify: `internal/workbench/schema/runtime_test.go`

### Step 1：先写状态机失败测试

- [ ] 覆盖 Flow 类型：

```go
type FlowKind string

const (
    FlowRegister      FlowKind = "register"
    FlowOTPLogin      FlowKind = "otp_login"
    FlowPasswordLogin FlowKind = "password_login"
    FlowPasswordReset FlowKind = "password_reset"
    FlowInvitation    FlowKind = "invitation"
)
```

- [ ] 覆盖状态：

```go
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

- [ ] 测试允许迁移：

```text
created → challenge_sent
challenge_sent → identity_verified
identity_verified → provisioning
identity_verified → password_pending
provisioning → callback_ready
password_pending → callback_ready
callback_ready → completed
任意非终态 → failed
过期时间到达 → expired
```

- [ ] 测试拒绝：

```text
completed → 任意
failed → 任意
expired → 任意
challenge_sent → completed
callback_ready → challenge_sent
```

- [ ] 运行并确认失败：

```powershell
go test ./internal/accountentry -run Flow -count=1
```

### Step 2：定义不泄密的聚合快照

```go
type FlowSnapshot struct {
    ID                       string
    Kind                     FlowKind
    State                    FlowState
    PhoneFingerprint         string
    PhoneKeyVersion          string
    EncryptedProviderState   []byte
    AuthRequestIDFingerprint string
    InvitationID             string
    UserID                   string
    OrganizationID           string
    AuthorizationID          string
    AgreementVersion         string
    RememberSession          bool
    VerificationAttempts     int
    LastChallengeAt          *time.Time
    ChallengeExpiresAt       *time.Time
    ExpiresAt                time.Time
    Version                  int64
    CreatedAt                time.Time
    UpdatedAt                time.Time
}
```

- [ ] `EncryptedProviderState` 中允许保存：

```text
规范化 E.164 手机号
ZITADEL Session ID
ZITADEL Session Token
OIDC callback URL（仅 callback_ready 状态，完成后立即清空）
Provider challenge metadata
```

- [ ] 以下字段不得明文持久化：

```text
手机号
短信验证码
密码
Session Token
callback URL
```

密码和验证码甚至不得进入 `EncryptedProviderState`。

### Step 3：建立数据库表

```go
type accountEntryFlowRow struct {
    ID                       string
    Kind                     string
    State                    string
    PhoneFingerprint         string
    PhoneKeyVersion          string
    EncryptedProviderState   []byte
    AuthRequestIDFingerprint string
    InvitationID             string
    UserID                   string
    OrganizationID           string
    AuthorizationID          string
    AgreementVersion         string
    RememberSession          bool
    VerificationAttempts     int
    LastChallengeAt          *time.Time
    ChallengeExpiresAt       *time.Time
    ExpiresAt                time.Time
    Version                  int64
    CreatedAt                time.Time
    UpdatedAt                time.Time
}
```

表名：

```text
account_entry_flows
```

索引：

```text
PRIMARY KEY (id)
INDEX (phone_fingerprint, state, expires_at)
UNIQUE (auth_request_id_fingerprint)
INDEX (invitation_id)
INDEX (expires_at)
```

- [ ] `Save` 使用 `WHERE id = ? AND version = ?` 乐观锁。
- [ ] 所有读取只返回未过期记录；清理任务处理历史终态。

### Step 4：Schema Runtime

- [ ] `AutoMigrate` 增加 Account Entry 表，但只有 `AccountEntry.Enabled=true` 时执行。
- [ ] `runtime_test.go` 断言开关关闭时不创建该表，开启时只增加明确列和索引。

### Step 5：验证

```powershell
go test ./internal/accountentry -run "Flow|Repository" -count=1
go test ./internal/workbench/schema -count=1
```

### Step 6：提交

```powershell
git add internal/accountentry internal/workbench/schema
git commit -m "feat: add account entry flow state"
```

---

## Task 4：实现手机号指纹、流程密文与分布式限流

**Files:**
- Create: `internal/accountentry/phone_fingerprint.go`
- Create: `internal/accountentry/phone_fingerprint_test.go`
- Create: `internal/accountentry/flow_cipher.go`
- Create: `internal/accountentry/flow_cipher_test.go`
- Create: `internal/accountentry/rate_limiter.go`
- Create: `internal/accountentry/rate_limiter_test.go`
- Modify: `internal/platform/redis/client.go`
- Modify: `internal/platform/redis/client_test.go`

### Step 1：手机号规范化与 HMAC

- [ ] 支持 E.164，默认注册界面国家码为 `+86`，服务端不接受缺少国家码的原始持久值。

```go
func NormalizePhone(countryCode, nationalNumber string) (string, error)
func PhoneFingerprint(key []byte, normalizedPhone string) string
```

- [ ] 测试：

```text
+86 + 13800000000 → +8613800000000
全角数字、空格、短横线先规范化
非法国家码、过短、过长、控制字符拒绝
Fingerprint 稳定且不可由明文直接推导
```

- [ ] HMAC 使用 `HMAC-SHA-256`，输出小写 hex。
- [ ] 读取流程同时尝试当前和上一版本 Key；新写入只使用当前 Key。

### Step 2：Provider State 加密

```go
type ProviderState struct {
    Phone            string
    SessionID        string
    SessionToken     string
    CallbackURL      string
    ChallengeCreated time.Time
}
```

- [ ] 使用 AES-256-GCM。
- [ ] AAD 固定包含：

```text
flow ID
flow kind
phone key version
```

- [ ] 每次保存重新生成随机 Nonce。
- [ ] 篡改、错 Key、错 AAD、截断密文都必须失败关闭。

### Step 3：Redis 原子限流原语

- [ ] 在 `internal/platform/redis/client.go` 增加最小接口：

```go
type WindowCounter interface {
    IncrementWindow(ctx context.Context, key string, ttl time.Duration) (int64, error)
}
```

- [ ] 使用单条 Lua Script 原子执行：

```text
INCR key
如果结果为 1，则 EXPIRE key ttl
返回当前计数
```

- [ ] 用 `miniredis` 或真实兼容 Redis 测试并发窗口、TTL、重复调用和依赖失败。

### Step 4：组合限流

```text
send challenge:
- phone fingerprint / hour
- IP HMAC / hour
- IP HMAC / day

verify challenge:
- flow / attempts
- IP HMAC / hour
```

- [ ] 原始 IP 不进入 Redis Key；使用独立 HMAC Key 派生。
- [ ] 任一 Redis 错误时，验证码发送和验证都 fail closed，返回 `DEPENDENCY_UNAVAILABLE`。
- [ ] 对外只返回通用 `TOO_MANY_REQUESTS` 和冷却秒数，不返回命中的内部维度。

### Step 5：运行

```powershell
go test ./internal/platform/redis -run Window -count=1
go test ./internal/accountentry -run "Phone|Cipher|Rate" -count=1
```

### Step 6：提交

```powershell
git add internal/accountentry internal/platform/redis
git commit -m "feat: secure account entry flow state"
```

---

## Task 5：实现版本锁定的 ZITADEL Login 与 Provisioning Client

**Files:**
- Create: `internal/accountentry/zitadel_client.go`
- Create: `internal/accountentry/zitadel_client_test.go`
- Create: `internal/accountentry/zitadel_types.go`
- Create: `internal/accountentry/provider_error.go`

### Step 1：先写失败的 Provider 合约测试

- [ ] 使用 `httptest.Server` 精确断言：

```text
Authorization credential purpose
HTTP method and path
Connect-Protocol-Version where required
request JSON
response size limit
timeout / cancellation
provider status mapping
callback URL validation
```

测试名称至少包括：

```go
func TestZitadelFindUserByPhoneDoesNotLeakProviderBody(t *testing.T)
func TestZitadelCreateOTPChallengeUsesLoginClientToken(t *testing.T)
func TestZitadelVerifyOTPReplacesSessionToken(t *testing.T)
func TestZitadelPasswordLoginUsesSessionPasswordCheck(t *testing.T)
func TestZitadelCreateCallbackRejectsUntrustedURL(t *testing.T)
func TestZitadelCreateOrganizationWithHumanAdminUsesProvisioningToken(t *testing.T)
func TestZitadelCreateAuthorizationUsesProjectID(t *testing.T)
```

- [ ] 运行并确认失败：

```powershell
go test ./internal/accountentry -run Zitadel -count=1
```

### Step 2：拆分最小权限 Client

```go
type LoginClient interface {
    FindUserByPhone(context.Context, string) (UserLookup, error)
    CreateOTPChallenge(context.Context, CreateOTPInput) (SessionMaterial, error)
    VerifyOTP(context.Context, VerifyOTPInput) (SessionProof, error)
    AuthenticatePassword(context.Context, PasswordLoginInput) (SessionProof, error)
    GetAuthRequest(context.Context, string) (AuthRequest, error)
    CreateCallback(context.Context, CallbackInput) (string, error)
}

type ProvisioningClient interface {
    CreateOrganizationWithHumanAdmin(context.Context, RegistrationIdentity) (ProvisioningResult, error)
    MarkPhoneVerified(context.Context, string, string) error
    CreateAuthorization(context.Context, AuthorizationInput) (string, error)
    SetPassword(context.Context, PasswordResetInput) error
    DeleteIncompleteOrganization(context.Context, string) error
}
```

- [ ] `LoginClient` 只接收 Login Client Token。
- [ ] `ProvisioningClient` 只接收 Provisioning Token。
- [ ] 两个 Client 都不导出底层 token 字段。

### Step 3：实现 Session 检查

OTP：

```json
{
  "checks": {
    "user": {
      "loginName": "+8613800000000"
    }
  },
  "challenges": {
    "otpSms": {
      "returnCode": false
    }
  },
  "lifetime": "300s"
}
```

验证：

```json
{
  "checks": {
    "otpSms": {
      "code": "client-supplied-code"
    }
  }
}
```

密码：

```json
{
  "checks": {
    "user": {
      "loginName": "+8613800000000"
    },
    "password": {
      "password": "client-supplied-password"
    }
  },
  "lifetime": "43200s"
}
```

- [ ] 密码和验证码只存在于请求对象生命周期中，不进入结构体 `String()`、错误或日志。

### Step 4：规范化 Provider 错误

```go
type ProviderErrorCode string

const (
    ProviderInvalidCredentials ProviderErrorCode = "invalid_credentials"
    ProviderChallengeExpired   ProviderErrorCode = "challenge_expired"
    ProviderConflict           ProviderErrorCode = "conflict"
    ProviderNotFound           ProviderErrorCode = "not_found"
    ProviderRateLimited        ProviderErrorCode = "rate_limited"
    ProviderUnavailable        ProviderErrorCode = "unavailable"
)
```

- [ ] 外部错误体不透传。
- [ ] 404、409、429 和 5xx 映射为稳定内部错误。
- [ ] 密码登录中的 404 和无密码状态统一映射为 `ProviderInvalidCredentials`。

### Step 5：验证 Callback URL

- [ ] `CreateCallback` 只接受：

```text
origin == AccountEntry.PublicOrigin
path ∈ {
  /api/zitadel-auth/callback,
  /api/auth/callback/zitadel
}
query 含 code 和 state
无 fragment
无 userinfo
```

两条路径分别覆盖现有同源回调包装路由和 Auth.js 标准 Provider 回调；测试必须证明其他同源路径也会被拒绝。

### Step 6：运行

```powershell
go test ./internal/accountentry -run Zitadel -count=1
```

### Step 7：提交

```powershell
git add internal/accountentry/zitadel_*
git commit -m "feat: add account entry ZITADEL client"
```

---

## Task 6：增加 `base_payg` 初始化方案和账号初始化适配器

**Files:**
- Modify: `internal/listingsubscription/types.go`
- Modify: `internal/listingsubscription/service.go`
- Modify: `internal/listingsubscription/service_test.go`
- Create: `internal/accountentry/initializer.go`
- Create: `internal/accountentry/initializer_test.go`

### Step 1：先写失败的 `base_payg` 测试

- [ ] 增加：

```go
const PlanBasePayAsYouGo = "base_payg"
```

- [ ] `DefaultPlans()` 增加一个面向新硕米企业的单一方案：

```go
Plan{
    Code:        PlanBasePayAsYouGo,
    Name:        "基础方案 · 按需使用",
    Description: "店铺服务按期使用，AI 点数与数据额度按实际资源余额扣减",
}
```

- [ ] 首期默认模块只开通稳定已有能力；不得赠送虚构余额：

```text
store_management.store_count = 1
store_renewal_periods = 0
ai_points = 0
data_rows = 0
```

- [ ] 旧 `basic / professional / enterprise` 计划继续保留，避免破坏现有租户；新自助注册只绑定 `base_payg`。

- [ ] 运行并确认失败：

```powershell
go test ./internal/listingsubscription -run BasePay -count=1
```

### Step 2：实现幂等账号初始化器

```go
type AccountInitializer interface {
    GrantInitialRole(context.Context, string, string, string) (string, error)
    ApplyBasePlan(context.Context, string, string) error
}
```

实现：

```go
type DefaultAccountInitializer struct {
    provisioner ProvisioningClient
    subscription *listingsubscription.Service
}
```

- [ ] `GrantInitialRole` 使用：

```text
role = listingkit_admin
organization = 新创建 Organization
project = ListingKit Project
```

- [ ] `ApplyBasePlan` 使用现有 `ApplyPlan`，状态为 `active`，无试用期、无虚构到期日。
- [ ] 相同 user/organization 重放不重复创建 Authorization；Provider 返回冲突后读取现有 Authorization 或进入可修复状态。
- [ ] 订阅已存在时核对 `plan_code == base_payg`；不同 Plan 不静默覆盖。

### Step 3：邀请注册初始化规则

```text
直接注册：
- 创建默认 Organization
- 授予 listingkit_admin
- 绑定 base_payg

邀请注册：
- 不创建默认 Organization
- 不创建 base_payg
- 只消费邀请中已有的 organization + role
```

### Step 4：运行

```powershell
go test ./internal/listingsubscription -run BasePay -count=1
go test ./internal/accountentry -run Initializer -count=1
```

### Step 5：提交

```powershell
git add internal/listingsubscription internal/accountentry/initializer*
git commit -m "feat: add base pay-as-you-go onboarding"
```

---

## Task 7：实现单次邀请 Token 的签发和消费

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

### Step 1：定义邀请合约

```go
type Invitation struct {
    ID                    string
    TokenHash             string
    OrganizationID        string
    Role                  string
    PhoneFingerprint      string
    PhoneKeyVersion       string
    State                 InvitationState
    ExpiresAt             time.Time
    ConsumedByUserID      string
    ConsumedAt            *time.Time
    Version               int64
    CreatedBy             string
    CreatedAt             time.Time
}
```

状态：

```text
pending
consuming
consumed
revoked
expired
```

- [ ] 数据库只保存 `SHA-256(token)`；原始 token 仅签发时返回一次。
- [ ] Token 使用 32 字节 CSPRNG，URL-safe base64url 无填充。
- [ ] 邀请必须绑定 Organization、角色和手机号指纹，不能改为任意注册入口。
- [ ] 允许角色只包括当前固定租户角色：

```text
listingkit_viewer
listingkit_operator
listingkit_admin
```

### Step 2：签发接口只供已认证管理员

`InvitationIssuer.Issue` 输入：

```go
type IssueInvitationInput struct {
    OrganizationID  string
    ActorSubject    string
    TargetPhone     string
    Role            string
    ExpiresAt       time.Time
    IdempotencyKey  string
}
```

- [ ] 使用 Effective Organization，不信任任意 Tenant ID。
- [ ] 第一阶段不做邀请管理 UI；只提供受保护后端能力给后续成员管理使用。
- [ ] 同一 Idempotency Key 重放返回同一邀请元数据，但不再次返回原始 token。调用方必须在首次响应后立即交付链接。

### Step 3：消费并发测试

- [ ] 两个并发请求只能有一个从 `pending` 进入 `consuming`。
- [ ] 已消费、撤销、过期、手机号不匹配统一返回不可用，不泄露具体原因。
- [ ] Provisioning 失败可从 `consuming` 恢复为可重试，不允许第二个用户接管。

### Step 4：Schema

表：

```text
account_entry_invitations
```

索引：

```text
PRIMARY KEY (id)
UNIQUE (token_hash)
UNIQUE (organization_id, idempotency_key)
INDEX (phone_fingerprint, state, expires_at)
```

### Step 5：运行

```powershell
go test ./internal/accountentry -run Invitation -count=1
go test ./internal/workbench/schema -count=1
```

### Step 6：提交

```powershell
git add internal/accountentry/invitation* internal/workbench/schema
git commit -m "feat: add one-time account invitations"
```

---

## Task 8：实现注册和验证码登录 Saga

**Files:**
- Create: `internal/accountentry/service.go`
- Create: `internal/accountentry/service_test.go`
- Create: `internal/accountentry/reconciler.go`
- Create: `internal/accountentry/reconciler_test.go`
- Create: `internal/accountentry/audit.go`
- Create: `internal/accountentry/audit_test.go`

### Step 1：挑战请求测试

```go
type ChallengeRequest struct {
    Kind             FlowKind
    Phone            string
    CountryCode      string
    AuthRequestID    string
    InvitationToken  string
    AgreementVersion string
    RememberSession  bool
    ClientIP         string
    UserAgent        string
}
```

- [ ] 直接注册必须有协议版本。
- [ ] 登录不接受协议字段。
- [ ] `AuthRequestID` 必须匹配当前应用 Client ID 和允许的 Redirect URI。
- [ ] 同一个未过期 Flow 重发挑战复用 Flow ID，但遵守冷却和限流。

### Step 2：已存在与新用户分流

```text
手机号已存在：
- register 请求退化为 OTP 登录
- 不创建第二个 Organization
- 不泄露“账号已存在”

手机号不存在 + otp_login：
- 仍返回通用挑战响应
- 不创建账号
- 验证时返回通用失败

手机号不存在 + register：
- 创建未验证手机号的临时用户和 Organization
- 在短信证明前不创建 Project Authorization
- 在短信证明前不绑定 base_payg
```

- [ ] 首次创建记录保留 `provisioning` 可恢复状态。
- [ ] Organization/User 已创建但 OTP 未通过时，不拥有硕米项目角色，不能进入 Workbench。
- [ ] 过期未完成注册由 Reconciler 删除不完整 Organization；删除失败持续重试并记录脱敏审计。

### Step 3：验证 OTP

```text
读取 Flow
→ 校验 attempts / expiresAt
→ 调 ZITADEL VerifyOTP
→ 再读 Session Proof
→ 证明 user factor 与 otpSms factor 均存在
→ 证明 user ID / Organization ID 与 Flow 一致
→ 特权标记手机号已验证
→ 直接注册：创建 listingkit_admin + base_payg
→ 邀请注册：创建邀请 Role Assignment 并消费邀请
→ CreateCallback
→ 加密保存 callback URL
→ Flow = callback_ready
```

- [ ] 同一验证码重放不会重复创建 Role、Plan 或邀请消费。
- [ ] Provider 返回新 Session Token 时立即替换旧 Token；旧 Token 不再持久化。
- [ ] 验证成功后清除手机号以外的无用 Challenge metadata。

### Step 4：完成 Flow

```go
type Completion struct {
    CallbackURL string
    FlowID      string
}
```

- [ ] `Complete` 只允许 `callback_ready`。
- [ ] 返回后立即将 Flow 标为 `completed` 并清空 Provider 密文。
- [ ] 两个并发 Complete 只能一个拿到 CallbackURL；第二个返回 `FLOW_ALREADY_COMPLETED`。
- [ ] Callback URL 永远不经普通浏览器 JSON 返回；只给受 BFF Token 保护的 Next.js 服务端。

### Step 5：审计

允许字段：

```text
flow id
flow kind
state transition
phone fingerprint suffix
user id suffix
organization id suffix
result
reason code
request id
timestamp
```

禁止字段：

```text
手机号
验证码
密码
Session Token
callback URL
Authorization header
Provider response body
```

### Step 6：运行

```powershell
go test ./internal/accountentry -run "Challenge|Register|OTP|Complete|Reconcile|Audit" -count=1
```

### Step 7：提交

```powershell
git add internal/accountentry/service* internal/accountentry/reconciler* internal/accountentry/audit*
git commit -m "feat: add phone registration and OTP login saga"
```

---

## Task 9：实现手机号密码登录与短信证明后的密码重置

**Files:**
- Create: `internal/accountentry/password.go`
- Create: `internal/accountentry/password_test.go`
- Modify: `internal/accountentry/service.go`
- Modify: `internal/accountentry/service_test.go`

### Step 1：密码登录

```go
type PasswordLoginRequest struct {
    Phone           string
    CountryCode     string
    Password        []byte
    AuthRequestID   string
    RememberSession bool
    ClientIP        string
    UserAgent       string
}
```

- [ ] Password 以 `[]byte` 接收；请求结束后显式覆盖切片。
- [ ] 创建 ZITADEL Session 时同时检查 user + password。
- [ ] 成功后直接 CreateCallback → callback_ready，不创建短信挑战。
- [ ] 用户不存在、未设置密码、密码错误统一返回：

```text
INVALID_CREDENTIALS
账号或密码不正确
```

- [ ] 错误响应时间增加统一的最小延迟和小幅随机抖动，避免明显账号枚举侧信道。
- [ ] 每 IP / phone fingerprint 施加独立密码登录失败限流。

### Step 2：密码重置 Challenge

```text
phone + forgot_password
→ 已存在用户：发送 OTP SMS
→ 不存在用户：执行等时假流程并返回相同 challenge_sent
```

- [ ] 不存在用户的假流程不向任意号码发短信。
- [ ] 公共响应和状态码一致。
- [ ] 仅真实用户在正确 OTP 后进入 `password_pending`。

### Step 3：设置新密码

```go
type CompletePasswordResetRequest struct {
    FlowID          string
    NewPassword     []byte
    ConfirmPassword []byte
}
```

- [ ] 只有 `password_pending` 可调用。
- [ ] 两个密码必须常量时间比较。
- [ ] 密码策略以 ZITADEL 为最终权威；前端仅做相同或更弱的提示校验。
- [ ] 设置成功后使用已验证 Session 创建 OIDC Callback，用户直接完成登录。
- [ ] 密码设置完成后覆盖内存中的密码字节并清空 Flow 中短信 Session 信息。

### Step 4：错误与审计

- [ ] Provider 的密码策略明细映射为稳定 `PASSWORD_POLICY_REJECTED`，不返回内部规则 ID。
- [ ] 记录“密码已设置/已重置”事件，不记录密码长度、强度或哈希。

### Step 5：运行

```powershell
go test ./internal/accountentry -run Password -count=1
```

### Step 6：提交

```powershell
git add internal/accountentry/password* internal/accountentry/service*
git commit -m "feat: add phone password authentication"
```

---

## Task 10：暴露受 BFF 保护的公共账号入口 HTTP API

**Files:**
- Create: `internal/accountentry/httpapi/module.go`
- Create: `internal/accountentry/httpapi/handler.go`
- Create: `internal/accountentry/httpapi/handler_test.go`
- Create: `internal/accountentry/httpapi/contract.go`
- Create: `internal/accountentry/httpapi/contract_test.go`
- Modify: `internal/httproute/descriptor.go`
- Modify: `internal/app/httpapi/server_auth.go`
- Modify: `internal/app/httpapi/server_test.go`

### Step 1：增加 BFF-only 公共策略

当前路由层只有 `public` 和 `verified_identity`。新增显式策略：

```go
const AuthPolicyTrustedBFF AuthPolicy = "trusted_bff"
```

- [ ] `AuthPolicyTrustedBFF` 不运行用户 Bearer Token 验证。
- [ ] 在常量时间比较后验证：

```http
Authorization: Bearer <ACCOUNT_ENTRY_BFF_TOKEN>
```

- [ ] 失败统一返回 `404` 或 `401`，不区分功能是否开启和 Token 是否错误。
- [ ] 账号入口路由不得接受 `X-User-ID`、`X-Tenant-ID`、`X-User-Roles` 等浏览器身份头；中间件先删除这些头。

### Step 2：定义路由

```text
POST /api/v1/account-entry/challenges
POST /api/v1/account-entry/verifications
POST /api/v1/account-entry/password-logins
POST /api/v1/account-entry/password-resets
POST /api/v1/account-entry/completions
POST /api/v1/account-entry/invitations/consume
```

受保护的邀请签发路由单独放入现有 Workbench 认证链：

```text
POST /api/v1/workbench/account-invitations
```

### Step 3：请求限制

```text
Content-Type: application/json
Body <= 16 KiB
读取超时 15 秒
拒绝未知字段
拒绝重复 JSON key
拒绝无效 UTF-8 和控制字符
```

- [ ] 所有写请求要求 `Idempotency-Key`，格式为 canonical UUID。
- [ ] 反向代理传入的 Client IP 只接受受信任来源；直接客户端伪造 `X-Forwarded-For` 不得影响限流键。

### Step 4：稳定错误 Envelope

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

允许错误码：

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

- [ ] HTTP 状态映射写测试并锁定。
- [ ] 不返回 Go error、Provider error 或数据库约束名。

### Step 5：验证路由描述符

- [ ] 测试每条公开路由：
  - `AuthPolicyTrustedBFF`
  - `OrganizationAccessPolicyNone`
  - 无业务 Permission
- [ ] 测试邀请签发：
  - `AuthPolicyVerifiedIdentity`
  - `OrganizationAccessPolicyLiveWrite`
  - 独立 `account.invitation.create` 权限

### Step 6：运行

```powershell
go test ./internal/accountentry/httpapi -count=1
go test ./internal/app/httpapi -run "TrustedBFF|AccountEntry" -count=1
```

### Step 7：提交

```powershell
git add internal/accountentry/httpapi internal/httproute internal/app/httpapi/server*
git commit -m "feat: expose trusted account entry API"
```

---

## Task 11：将 Account Entry 模块接入应用组合和生命周期

**Files:**
- Create: `internal/app/httpapi/accountentry_module.go`
- Create: `internal/app/httpapi/accountentry_module_test.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/composition_builder_test.go`
- Modify: `internal/app/httpapi/composition_modules.go`
- Modify: `internal/app/httpapi/types.go`
- Modify: `internal/app/httpapi/bootstrap.go`
- Modify: `internal/app/httpapi/bootstrap_test.go`

### Step 1：先写组合失败测试

- [ ] 开关关闭：

```text
Account Entry module == nil
不要求 Redis / Database / ZITADEL Login Client
```

- [ ] 开关开启：

```text
Database
Redis
Flow repository
Invitation repository
Rate limiter
ZITADEL Login Client
ZITADEL Provisioning Client
Subscription initializer
Reconciler
HTTP module
```

缺一不可；构造失败时 API 启动失败关闭。

### Step 2：实现 Builder

```go
type accountEntryBuildResult struct {
    module kernelmodule.Module
    closer func() error
}
```

- [ ] 复用现有 Shared Database 连接。
- [ ] 创建独立 Redis Client，并在 closer 中关闭。
- [ ] 启动一个单进程 Reconciler Tick，但数据库抢占必须保证多个 API Pod 不重复清理同一 Flow。
- [ ] Tick 周期默认 1 分钟，单次处理上限 100。

### Step 3：Composition

在 `httpFeatureComposition` 增加：

```go
accountEntryModule kernelmodule.Module
```

模块顺序：

```text
accountEntryModule
workbenchContextModule
storeCenterModule
```

Account Entry 不依赖 Workbench Context 才能处理登录；邀请签发子路由仍依赖已有 Workbench Auth 中间件。

### Step 4：启动日志

只允许：

```text
account entry enabled=true/false
self registration enabled=true/false
reconciler enabled=true/false
```

禁止打印 Origin 之外的配置值、密钥或 Provider Token。

### Step 5：运行

```powershell
go test ./internal/app/httpapi -run AccountEntry -count=1
go test ./internal/app/httpapi -run Workbench -count=1
```

### Step 6：提交

```powershell
git add internal/app/httpapi/accountentry_module* `
        internal/app/httpapi/composition* `
        internal/app/httpapi/types.go `
        internal/app/httpapi/bootstrap*
git commit -m "feat: wire account entry runtime"
```

---

## Task 12：建立 Next.js 同源 BFF、流程 Cookie 和 303 完成跳转

**Files:**
- Create: `web/listingkit-ui/src/app/api/account-entry/[...path]/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/[...path]/route.test.ts`
- Create: `web/listingkit-ui/src/lib/server/account-entry-proxy.ts`
- Create: `web/listingkit-ui/src/lib/server/account-entry-proxy.test.ts`
- Create: `web/listingkit-ui/src/lib/api/account-entry.ts`
- Create: `web/listingkit-ui/src/lib/api/account-entry.test.ts`
- Modify: `web/listingkit-ui/src/lib/server/request-log.ts`

### Step 1：严格 BFF Allowlist

允许的浏览器路由：

```text
POST /api/account-entry/challenge
POST /api/account-entry/verify
POST /api/account-entry/password-login
POST /api/account-entry/password-reset
POST /api/account-entry/complete
POST /api/account-entry/invitation
```

映射到 Go API；任何其他方法、路径或额外 path segment 返回 `404`。

### Step 2：请求头和 Cookie

BFF 发送：

```text
Authorization: Bearer ACCOUNT_ENTRY_BFF_TOKEN
Idempotency-Key: browser supplied canonical UUID or server generated UUID
X-Request-ID: bounded opaque value
X-Client-IP: canonical IP
X-Client-User-Agent: bounded string
X-Account-Entry-Flow-ID: optional opaque UUID
```

- [ ] Go 服务只信任来自 BFF 的 `X-Client-IP`，因为 BFF 凭据已经验证。
- [ ] Body 最大 16 KiB，读取超时 15 秒。
- [ ] User-Agent 最多 512 字节并删除控制字符。

### Step 3：响应合约

挑战成功：

```json
{
  "state": "challenge_sent",
  "cooldownSeconds": 60,
  "expiresAt": "2026-09-03T12:00:00Z"
}
```

验证成功：

```json
{
  "state": "identity_verified",
  "next": "complete"
}
```

密码重置验证成功：

```json
{
  "state": "password_pending",
  "next": "set_password"
}
```

- [ ] Flow ID 只放在 `X-Account-Entry-Flow-ID` 响应头中，不放在 JSON。
- [ ] `/complete` 响应只提供给 Next BFF：

```json
{
  "callbackUrl": "https://app.example/api/zitadel-auth/callback?...",
  "flowCompleted": true
}
```

### Step 4：稳定错误码

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

- [ ] BFF 只透传白名单字段，其他上游 JSON 全部丢弃。
- [ ] 错误响应 `Cache-Control: no-store`。

### Step 5：Cookie 设计

```text
__Host-shuomi-account-flow
- HttpOnly
- Secure
- SameSite=Lax
- Path=/
- Max-Age <= FlowTTL
- value = opaque flow UUID

__Host-shuomi-account-intent
- HttpOnly
- Secure
- SameSite=Lax
- Path=/
- value = signed { intent, returnTo, invitationId, createdAt }
```

- [ ] 本地 HTTP 使用无 `__Host-` 前缀的开发名称；生产启动时若非 HTTPS 则失败关闭。
- [ ] Intent 使用 HMAC 签名，不能信任浏览器 query 中的 `intent`。
- [ ] Cookie 不保存手机号、Session Token、callback URL 或验证码。

### Step 6：完成跳转

- [ ] 浏览器 `POST /api/account-entry/complete` 后，BFF：
  1. 从 HttpOnly Cookie 读 Flow ID；
  2. 调用 Go `/completions`；
  3. 验证 callback URL origin + path；
  4. 清除 Flow Cookie；
  5. 返回 `303 Location: <validated callback URL>`。

- [ ] callback URL 不通过 JSON 返回浏览器。
- [ ] `fetch()` 调用 complete 时使用 `redirect: "manual"`；表单也可使用原生 POST 让浏览器跟随 303。

### Step 7：测试

```text
open redirect
header spoofing
oversized body
invalid JSON
upstream timeout
unexpected content type
upstream malformed response
callback code never appears in body or log
Cookie flags
complete replay
```

### Step 8：运行

```powershell
Set-Location web/listingkit-ui
pnpm test -- `
  "src/app/api/account-entry/[...path]/route.test.ts" `
  src/lib/server/account-entry-proxy.test.ts `
  src/lib/api/account-entry.test.ts
Set-Location ../..
```

### Step 9：提交

```powershell
git add web/listingkit-ui/src/app/api/account-entry `
        web/listingkit-ui/src/lib/server/account-entry-proxy* `
        web/listingkit-ui/src/lib/api/account-entry* `
        web/listingkit-ui/src/lib/server/request-log.ts
git commit -m "feat: add account entry browser BFF"
```

---

## Task 13：替换账号入口路由并保持 Auth.js OIDC 发起链路

**Files:**
- Modify: `web/listingkit-ui/src/app/login/page.tsx`
- Create: `web/listingkit-ui/src/app/login/page.test.tsx`
- Create: `web/listingkit-ui/src/app/register/page.tsx`
- Create: `web/listingkit-ui/src/app/register/page.test.tsx`
- Create: `web/listingkit-ui/src/app/forgot-password/page.tsx`
- Create: `web/listingkit-ui/src/app/forgot-password/page.test.tsx`
- Create: `web/listingkit-ui/src/app/join/[token]/page.tsx`
- Create: `web/listingkit-ui/src/app/join/[token]/page.test.tsx`
- Modify: `web/listingkit-ui/src/app/api/zitadel-auth/login/route.ts`
- Modify: `web/listingkit-ui/src/app/api/zitadel-auth/login/route.test.ts`
- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.ts`
- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.test.ts`
- Modify: `web/listingkit-ui/src/components/application-frame.tsx`
- Modify: `web/listingkit-ui/src/components/application-frame.test.tsx`
- Modify: `web/listingkit-ui/src/proxy.ts`
- Modify: `web/listingkit-ui/src/proxy.test.ts`

### Step 1：发起 OIDC 时设置可信 Intent

`GET /api/zitadel-auth/login` 接受：

```text
intent=login | register | invitation
returnTo=/workbench/...
invitationToken=<opaque token，仅 invitation>
```

- [ ] 服务端将 Intent 签入 `__Host-shuomi-account-intent`。
- [ ] `returnTo` 继续使用 `normalizeReturnTo`，外部 URL 回退 `/workbench`。
- [ ] Invitation Token 在 BFF 消费后只把 Invitation ID 写入 Intent；原始 token 不写 Cookie。
- [ ] 然后调用现有 `signIn("zitadel")` 发起标准 Authorization Code + PKCE。

直接页面入口：

```text
GET /login
→ 303 /api/zitadel-auth/login?intent=login

GET /register
→ 303 /api/zitadel-auth/login?intent=register

GET /join/{token}
→ POST /api/account-entry/invitation
→ 303 /api/zitadel-auth/login?intent=invitation
```

### Step 2：ZITADEL 回到自定义 Login URL 后的路由

- [ ] `/login?authRequest=...` 读取 Intent Cookie：
  - `register` → 303 `/register?authRequest=...`
  - `login` → 保持 `/login`
  - 缺失/非法 → 默认 OTP 登录
- [ ] Password/OTP Tab 切换必须保留 `authRequest` 和合法 `returnTo`。
- [ ] 已登录用户访问 `/login` 或 `/register` 时，服务端重定向到合法 `returnTo` 或 `/workbench`。

### Step 3：更新公共页面判定

公共无壳页面：

```text
/login
/register
/forgot-password
/join/*
/auth-entry-preflight（仅测试开关）
```

- [ ] `ApplicationFrame.isPublicRoute` 支持前缀路由 `/join/`。
- [ ] `proxy.ts` 不拦截账号入口页面；只继续保护 `/workbench/*` 和发布期保留的旧页面。

### Step 4：保留 Auth.js 发起和回调

- [ ] `/api/zitadel-auth/login` 继续调用：

```ts
signIn("zitadel", {
  redirectTo: normalizeReturnTo(returnTo),
});
```

- [ ] 不新建 Credentials Provider。
- [ ] 保留现有 `/api/zitadel-auth/callback` 包装路由和 `/api/auth/callback/zitadel` Auth.js Provider 回调；不把授权码返回给客户端 JavaScript，也不新建第二套会话终结点。
- [ ] `normalizeReturnTo` 默认值从 `/` 调整为 `/workbench`，外部 origin、协议相对 URL、反斜杠和控制字符继续拒绝。

### Step 5：运行

```powershell
Set-Location web/listingkit-ui
pnpm test -- `
  src/app/login/page.test.tsx `
  src/app/register/page.test.tsx `
  src/app/forgot-password/page.test.tsx `
  "src/app/join/[token]/page.test.tsx" `
  src/app/api/zitadel-auth/login/route.test.ts `
  src/lib/server/zitadel-auth.test.ts `
  src/components/application-frame.test.tsx `
  src/proxy.test.ts
Set-Location ../..
```

### Step 6：提交

```powershell
git add web/listingkit-ui/src/app/login `
        web/listingkit-ui/src/app/register `
        web/listingkit-ui/src/app/forgot-password `
        web/listingkit-ui/src/app/join `
        web/listingkit-ui/src/app/api/zitadel-auth/login `
        web/listingkit-ui/src/lib/server/zitadel-auth* `
        web/listingkit-ui/src/components/application-frame* `
        web/listingkit-ui/src/proxy*
git commit -m "feat: route Shuomi account entry through OIDC"
```

---

## Task 14：实现 Figma 账号入口共享 Shell 和本地品牌资产

**Files:**
- Create: `web/listingkit-ui/src/components/auth-entry/auth-entry-shell.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/auth-entry-shell.test.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/auth-brand-panel.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/auth-card.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/auth-entry.css`
- Create: `web/listingkit-ui/public/shuomi/auth/shuomi-logo.png`
- Create: `web/listingkit-ui/public/shuomi/auth/collaboration-field.svg`
- Modify: `web/listingkit-ui/src/app/globals.css`

### Step 1：本地化 Figma 资产

- [ ] 从已确认节点导出并保存本地资产：

```text
374:325 注册
373:303 密码登录
378:307 验证码登录
1266:359 重置密码
```

- [ ] 不引用 `figma.com/api/mcp/asset/*` 七天临时 URL。
- [ ] Logo、协作光场分别保存语义化文件名；删除未被使用的 UUID 资产副本。
- [ ] 校验图片尺寸、透明通道和压缩；不提交字体文件。

### Step 2：建立固定深色 Auth Token

```css
.shuomi-auth-entry {
  --auth-bg: #050912;
  --auth-panel: #0b1420;
  --auth-card: rgba(12, 24, 38, 0.94);
  --auth-border: #233b52;
  --auth-text: #f5f8fb;
  --auth-muted: #8fa5ba;
  --auth-primary: #3ee7d0;
  --auth-primary-hover: #2ecab8;
  --auth-danger: #ff7185;
  --auth-focus: #6fa7ff;
}
```

- [ ] Auth 页面固定深色，不读取 Console Theme。
- [ ] Token 作用域不污染官网和旧页面。
- [ ] 中文字体优先：

```text
Noto Sans SC
Microsoft YaHei
PingFang SC
Segoe UI
sans-serif
```

### Step 3：响应式布局

```text
>= 1100px：左右双栏，品牌区约 55%，表单卡片固定最大宽度 480px
768–1099px：品牌区缩窄，表单保持可读宽度
< 768px：单列，表单优先，品牌文案精简，光场作为背景弱化
```

- [ ] 不复制 Figma 绝对定位；使用 Grid/Flex。
- [ ] 页面最小高度使用 `100dvh`。
- [ ] 320px 宽度无横向滚动。

### Step 4：组件测试

```text
Logo 有替代文本
返回官网是合法链接
标题层级唯一
Slot 正确渲染
移动端 class/结构存在
不会读取 Console Theme
```

### Step 5：视觉验收

- [ ] 在 `1440×900` 对照四个节点截图。
- [ ] 允许因删除注册用户名和密码而缩短卡片，但保持卡片宽度、视觉层级、品牌区和间距体系。
- [ ] 小字号正文不得低于 12px；不机械复制 9–11px 文字。

### Step 6：运行

```powershell
Set-Location web/listingkit-ui
pnpm test -- src/components/auth-entry/auth-entry-shell.test.tsx
pnpm exec playwright test e2e/account-entry-visual.spec.ts --project=chromium
Set-Location ../..
```

### Step 7：提交

```powershell
git add web/listingkit-ui/src/components/auth-entry `
        web/listingkit-ui/public/shuomi/auth `
        web/listingkit-ui/src/app/globals.css
git commit -m "feat: add Shuomi account entry shell"
```

---

## Task 15：实现注册、OTP 登录、密码登录和重置密码表单

**Files:**
- Create: `web/listingkit-ui/src/components/auth-entry/register-form.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/register-form.test.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/login-form.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/login-form.test.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/otp-login-form.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/password-login-form.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/reset-password-form.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/reset-password-form.test.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/phone-field.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/otp-field.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/password-field.tsx`
- Create: `web/listingkit-ui/src/components/auth-entry/auth-submit-button.tsx`
- Modify: `web/listingkit-ui/src/app/register/page.tsx`
- Modify: `web/listingkit-ui/src/app/login/page.tsx`
- Modify: `web/listingkit-ui/src/app/forgot-password/page.tsx`

### Step 1：表单 Schema

```ts
const phoneSchema = z.object({
  countryCode: z.literal("+86"),
  nationalNumber: z.string().regex(/^1[3-9]\d{9}$/),
});

const otpSchema = z.string().regex(/^\d{6}$/);
```

- [ ] 前端规则只做即时反馈，服务端仍是最终权威。
- [ ] 不在 URL、analytics 或 localStorage 中保存手机号。
- [ ] Password 不进入 React Query cache。

### Step 2：注册表单

严格只显示：

```text
手机号
短信验证码
协议确认
注册并进入
已有账号？立即登录
```

- [ ] 不渲染展示名称、用户名、密码或邮箱。
- [ ] 验证码按钮状态：

```text
获取验证码
发送中…
59 秒后重试
重新获取
```

- [ ] 协议勾选前提交禁用。
- [ ] 协议链接新窗口打开且保持当前表单状态。

### Step 3：登录表单

OTP 默认：

```text
手机号
短信验证码
登录
```

Password：

```text
手机号
登录密码
记住登录状态
忘记密码
登录
```

- [ ] Tab 使用 `?method=otp|password`，刷新和前进后退保留状态。
- [ ] Tab 切换保留手机号，但立即清除验证码和密码。
- [ ] 登录错误统一为“账号或密码不正确”或“验证码无效或已过期”。

### Step 4：密码重置

```text
手机号
短信验证码
新密码
确认新密码
确认修改
返回登录
```

- [ ] Challenge 验证成功前不显示或不启用密码字段。
- [ ] 设置成功后直接完成 OIDC 登录；失败时清除密码字段和验证码。

### Step 5：完成跳转

- [ ] 验证或密码登录成功后，使用原生隐藏表单 POST `/api/account-entry/complete`，让浏览器跟随 `303`。
- [ ] 按钮提交期间禁用，避免并发 Complete。
- [ ] complete 返回错误时恢复按钮并展示通用错误。

### Step 6：可访问性

```text
每个 input 有 label
验证码错误使用 aria-describedby
Tab 使用 tablist/tab/tabpanel
倒计时使用 aria-live=polite，但不每秒打断屏幕阅读器
密码显示切换有可访问名称
键盘可以完成全部流程
prefers-reduced-motion 时关闭光场动画
```

### Step 7：组件测试

```text
注册字段严格数量
协议门槛
验证码倒计时
重复点击保护
Tab 状态
字段清理
错误映射
原生 303 完成表单
无 localStorage/sessionStorage 写入
```

### Step 8：运行

```powershell
Set-Location web/listingkit-ui
pnpm test -- src/components/auth-entry
pnpm lint
Set-Location ../..
```

### Step 9：提交

```powershell
git add web/listingkit-ui/src/components/auth-entry `
        web/listingkit-ui/src/app/register `
        web/listingkit-ui/src/app/login `
        web/listingkit-ui/src/app/forgot-password
git commit -m "feat: implement Shuomi account entry forms"
```

---

## Task 16：配置 ZITADEL Custom Login URL 和 Kubernetes Secret

**Files:**
- Modify: `deployments/kubernetes/zitadel/local/README.md`
- Modify: `deployments/kubernetes/zitadel/local/values.yaml`
- Modify: `deployments/kubernetes/listingkit-workbench/base/listingkit-ui-auth-config.yaml`
- Modify: `deployments/kubernetes/listingkit-workbench/base/listingkit-ui-deployment.yaml`
- Modify: `deployments/kubernetes/listingkit-workbench/base/product-listing-api-deployment.yaml`
- Create: `deployments/kubernetes/listingkit-workbench/base/account-entry-secret.example.yaml`
- Create: `docs/runbooks/shuomi-account-entry.md`

### Step 1：创建 Secret 合约

`account-entry-secret.example.yaml` 只包含占位键名，不包含真实值：

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: shuomi-account-entry
  namespace: listingkit
type: Opaque
stringData:
  bff-token: ""
  flow-aead-key: ""
  phone-hmac-key: ""
  previous-phone-hmac-key: ""
  zitadel-login-client-token: ""
  zitadel-provisioning-token: ""
```

- [ ] README 给出安全生成命令：

```powershell
[Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
```

- [ ] BFF Token 采用独立 32 字节随机值，不复用 Auth.js Secret。

- [ ] `account-entry-secret.example.yaml` 仅作为操作示例保存，**不得加入 base `kustomization.yaml` 的 resources**；实际 Secret 由环境 Overlay 或外部 Secret 管理器提供。

### Step 2：配置 UI

`listingkit-ui-deployment.yaml` 增加：

```text
ACCOUNT_ENTRY_ENABLED
ACCOUNT_ENTRY_SELF_REGISTRATION_ENABLED
ACCOUNT_ENTRY_BFF_TOKEN
ACCOUNT_ENTRY_PUBLIC_ORIGIN
LISTINGKIT_SERVICE_API_BASE
```

- [ ] 所有 Secret 通过 `secretKeyRef`。
- [ ] 不设置 `NEXT_PUBLIC_ACCOUNT_ENTRY_BFF_TOKEN`。

### Step 3：配置 Go API

`product-listing-api-deployment.yaml` 增加 Task 2 的全部 `TASK_PROCESSOR_ACCOUNT_ENTRY_*` 变量。

- [ ] Redis 和 Database 使用现有 Secret/Config，不复制密码。
- [ ] 开关默认 `false`，预发布验证后才打开。

### Step 4：配置 Login V2

- [ ] 在 ZITADEL 对 ListingKit 应用设置应用级 Custom Login URL：

```text
${ACCOUNT_ENTRY_PUBLIC_ORIGIN}/login
```

- [ ] 保留 OIDC Redirect URI：

```text
${ACCOUNT_ENTRY_PUBLIC_ORIGIN}/api/zitadel-auth/callback
${ACCOUNT_ENTRY_PUBLIC_ORIGIN}/api/auth/callback/zitadel
```

以当前实际应用配置为准，删除未使用的旧 URI 需要单独变更审查。

- [ ] Login Client 只授予：

```text
session.read
session.write
session.link
```

- [ ] Provisioning Client 只授予账号创建、Organization 创建、手机号更新、Authorization 创建和密码重置所需权限；不得使用 Instance Owner 日常 Token。

### Step 5：运行手册

`shuomi-account-entry.md` 必须包含：

```text
功能开关顺序
Secret 轮换
Phone HMAC 双 Key 轮换
Flow AEAD Key 轮换
BFF Token 轮换
SMS Relay 健康检查
ZITADEL Login Client 健康检查
注册关闭但登录保留的操作
过期 Flow 手工对账
回滚步骤
敏感日志检查
```

### Step 6：静态验证

```powershell
kubectl kustomize deployments/kubernetes/listingkit-workbench/base | Out-Null
```

预期：退出码 `0`；渲染输出不包含 `kind: Secret` / `name: shuomi-account-entry` 的空示例对象，并且所有 Deployment 只通过 `secretKeyRef` 引用环境中真实创建的 Secret。

### Step 7：提交

```powershell
git add deployments/kubernetes/zitadel/local `
        deployments/kubernetes/listingkit-workbench/base `
        docs/runbooks/shuomi-account-entry.md
git commit -m "ops: configure Shuomi account entry"
```

---

## Task 17：端到端测试、真实验收和发布门槛

**Files:**
- Create: `web/listingkit-ui/e2e/account-entry.spec.ts`
- Create: `web/listingkit-ui/e2e/account-entry-accessibility.spec.ts`
- Create: `web/listingkit-ui/e2e/account-entry-visual.spec.ts`
- Create: `tests/account_entry_commercial_readiness_test.go`
- Modify: `docs/verification/2026-09-03-shuomi-account-entry-login-v2.md`
- Create: `docs/verification/2026-09-03-shuomi-account-entry-release.md`

### Step 1：Playwright 可替换 Provider 测试

- [ ] 在 E2E 模式启动受控 Fake Provider Adapter，只替换 Go Account Entry 的 `LoginClient` / `ProvisioningClient`，不替换 Next.js BFF 或 Auth.js callback 路由。

覆盖：

```text
注册字段只有手机号、验证码、协议
OTP 登录默认选中
密码登录切换
验证码倒计时
注册成功 303 到 Auth.js callback
手机号已存在的 register 不创建新组织
手机号不存在的 otp_login 不泄露状态
密码错误通用提示
密码重置成功并登录
邀请注册不创建默认企业
外部 returnTo 被拒绝
移动端无横向滚动
键盘可完成流程
```

### Step 2：安全回归

```text
BFF token 不进入浏览器 bundle
Session token 不进入响应体
callback URL 不进入 JSON 或日志
手机号不进入数据库明文列
验证码和密码不进入日志
伪造身份头被清除
重复 complete 只有一次成功
重复 register 不重复建企业或角色
Redis 失败时发送和验证均 fail closed
```

### Step 3：后端商业就绪约束测试

`tests/account_entry_commercial_readiness_test.go` 静态和行为断言：

```text
Kubernetes Secret 无硬编码值
前端不存在 NEXT_PUBLIC_* secret
账号入口公开路由全部是 trusted_bff
Provider Client 不暴露 token 字段
Account Entry 日志无敏感字段名
base_payg 不赠送 AI 点数 / 数据 / 续费期数
```

### Step 4：完整自动化验证

```powershell
go test ./internal/listingkit/phoneonboardingpreflight -count=1
go test ./internal/accountentry/... -count=1
go test ./internal/core/config -run AccountEntry -count=1
go test ./internal/app/httpapi -run "AccountEntry|TrustedBFF|Workbench" -count=1
go test ./internal/listingsubscription -run BasePay -count=1
go test ./internal/workbench/schema -count=1
go test ./tests -run AccountEntry -count=1
go test ./...

Set-Location web/listingkit-ui
pnpm test
pnpm lint
pnpm build
pnpm exec playwright test e2e/account-entry.spec.ts `
  e2e/account-entry-accessibility.spec.ts `
  e2e/account-entry-visual.spec.ts --project=chromium
Set-Location ../..

kubectl kustomize deployments/kubernetes/listingkit-workbench/base | Out-Null
```

每个命令必须记录实际退出码；不能以局部测试代替全量构建。

### Step 5：真实 ZITADEL 验收

在预发布环境使用一个全新手机号执行：

```text
1. /register 发起 OIDC Auth Request
2. 收到且只收到一条验证码
3. 注册完成进入 /workbench
4. ZITADEL 手机号为已验证
5. 只有一个默认 Organization
6. 只有一个 listingkit_admin Authorization
7. 只有一个 base_payg Subscription
8. 登出后 OTP 登录成功
9. 后台设置密码能力由 Slice 3 承接；本 Slice 先用受控测试接口设置密码后验证密码登录
10. 密码重置成功后旧密码失败、新密码成功
11. 所有临时测试 Flow 已清理
```

邀请验收使用第二个手机号：

```text
1. 管理员签发邀请
2. 邀请链接进入注册
3. OTP 证明后加入邀请企业
4. 不创建第二个默认 Organization
5. 角色与邀请一致
6. 邀请无法重复消费
```

### Step 6：发布顺序

```text
部署代码，所有开关 false
→ 配置 Secret
→ 配置 ZITADEL Custom Login URL
→ 运行 Task 1 真实门槛
→ 打开 ACCOUNT_ENTRY_ENABLED
→ 内部账号验证 OTP / 密码登录
→ 打开 ACCOUNT_ENTRY_SELF_REGISTRATION_ENABLED
→ 新手机号小流量注册
→ 观察错误率、短信量、Flow 积压和补偿队列
→ 全量开放
```

回滚：

```text
关闭 SELF_REGISTRATION
→ 保留登录
→ 必要时关闭 ACCOUNT_ENTRY_ENABLED
→ 恢复 ZITADEL 旧 Login URL
→ 不删除已创建用户、企业、角色或订阅
→ Reconciler 继续清理未完成 Flow
```

### Step 7：发布报告

`2026-09-03-shuomi-account-entry-release.md` 写入：

```text
Git SHA
ZITADEL 版本
配置版本
测试命令与退出码
真实短信条数
注册 / OTP / 密码 / 重置 / 邀请结果
敏感日志扫描结果
回滚演练结果
未解决风险
```

### Step 8：最终提交

```powershell
git add web/listingkit-ui/e2e `
        tests/account_entry_commercial_readiness_test.go `
        docs/verification/2026-09-03-shuomi-account-entry-*.md
git commit -m "test: verify Shuomi account entry release"
```

---

## 完成定义

Slice 1 只有同时满足以下条件才算完成：

- [ ] Task 1 的真实设备门槛通过，且只发送一条注册/登录 OTP 短信。
- [ ] 注册页面只要求手机号、验证码和协议。
- [ ] OTP 登录和手机号密码登录均通过现有 Auth.js 会话闭环。
- [ ] 注册时手机号在证明前保持未验证，证明后才标记已验证。
- [ ] 直接注册只创建一个默认企业、一个 `listingkit_admin` 授权和一个 `base_payg` 订阅。
- [ ] 邀请注册加入目标企业，不额外创建空企业。
- [ ] 手机号、验证码、密码、Session Token、callback URL 和管理凭据不进入浏览器 JSON、日志或明文数据库列。
- [ ] 公开 Account Entry API 只允许可信 Next.js BFF 调用。
- [ ] 注册、登录、重置和邀请支持幂等、限流、过期、并发重放和失败补偿。
- [ ] `/register`、`/login`、`/forgot-password` 在 1440×900 和 320px 宽度下可用且无横向滚动。
- [ ] Go 全量测试、前端测试、Lint、Build、Playwright、Kustomize 和商业就绪检查全部通过。
- [ ] 生产开关默认关闭；真实预发布验收后才按顺序开放。

## 下一步

Slice 1 合并并稳定运行后，再为 `Slice 2：新 Console Shell` 单独编写实施计划。