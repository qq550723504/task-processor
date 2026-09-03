# 硕米账号入口 Slice 1 Implementation Plan（ZITADEL 原生身份流）

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans`. Every task is TDD-first and must be committed separately.

**Goal:** 复用已合并的手机号 OTP 能力，以 ZITADEL User／Organization／Session／OIDC 为身份主干，以 Actions v2 为可关闭的签名身份事件桥，在 `task-processor` 中补齐并发 Claim、Provider 响应丢失恢复、业务就绪、邀请、callback generation、状态恢复、BFF 和 Figma 页面。

**Authoritative designs:**

- `docs/superpowers/specs/2026-09-03-shuomi-account-entry-zitadel-native-flow-design.md`
- `docs/superpowers/specs/2026-09-03-shuomi-console-resource-store-amendments.md`
- `docs/superpowers/specs/2026-09-02-shuomi-console-phase1-hard-cut-design.md`（除被上述文档覆盖的部分外）

**Tech stack:** Go 1.26、Gin/kernel module、GORM、PostgreSQL、Redis、ZITADEL v4.17.1、Actions v2、Next.js 16、React 19、Auth.js 5、TypeScript、Zod、Vitest、Playwright。

---

## 执行不变量

- PR #218 的 `phoneonboardingpreflight`、`zitadelsms` 和真实 OTP 证据是已实现基线，不得重写。
- 验证码只交给 ZITADEL Session API；不新增验证码表，不由业务代码比较验证码。
- ZITADEL 是 User、Phone、Password、Session、Organization、Authorization 和 OIDC 的身份权威。
- Actions v2 只做签名事件通知与收敛唤醒；Actions 关闭或延迟不阻断登录主链路。
- Account Entry 第一阶段不使用 Temporal。
- 一个手机号在并发、响应丢失、重试和 Key 轮换中只能关联一个 ZITADEL User。
- ZITADEL Organization ID 和 User ID 在 Provider POST 前由硕米预分配并持久化；结果未知时 lookup/adopt，不生成新 ID。
- 新 Identity 在 `base_payg`、目标授权和业务投影完成前保持 pending，不得走普通登录。
- `listingkit_admin` 是自助注册最后一个业务访问效果。
- callback URL 在 Auth.js Session ACK 前加密保存；旧 code 已消费但浏览器没拿到 Session 时必须创建新 OIDC Auth Request generation。
- 公开错误和时序不得泄露账号存在、pending、是否设置密码或邀请状态。
- 逻辑 Operation ID 跨 HTTP 重试稳定复用；敏感字段通过服务端 Keyed HMAC 参与 Request Fingerprint。
- Go、Next.js、Kubernetes 和 Runbook 统一使用：

```text
TASK_PROCESSOR_ACCOUNT_ENTRY_ENABLED
TASK_PROCESSOR_ACCOUNT_ENTRY_SELF_REGISTRATION_ENABLED
```

- Slice 1 不渲染“记住登录状态”。

---

## Task 1：冻结并抽取 PR #218 已验证的 ZITADEL Login V2 Client

**Files:**

- Create: `internal/authruntime/zitadel/loginv2/types.go`
- Create: `internal/authruntime/zitadel/loginv2/client.go`
- Create: `internal/authruntime/zitadel/loginv2/client_test.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/zitadel_client.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/zitadel_client_test.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/runner.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/runner_test.go`
- Modify: `hack/debug/listingkit-phone-onboarding-preflight/main.go`
- Modify: `hack/debug/listingkit-phone-onboarding-preflight/main_test.go`

### Step 1：写 Characterization Tests

锁定已有请求：

```text
POST /v2/organizations
POST /v2/users/new
POST /v2/users/{id}/otp_sms
POST /v2/sessions
PATCH /v2/sessions/{id}
GET /v2/sessions/{id}?sessionToken=...
DELETE /v2/sessions/{id}
DELETE /v2/organizations/{id}
```

锁定：

```text
Provisioning Token 与 Login Client Token 分离
Provider response <= 1 MiB
错误不包含手机号、验证码、Token 或 Provider body
```

### Step 2：移动而非复制

`phoneonboardingpreflight` 只保留别名或薄适配器：

```go
type Client = loginv2.Client
type ClientConfig = loginv2.ClientConfig
type SessionMaterial = loginv2.SessionMaterial
type SessionProof = loginv2.SessionProof
var NewClient = loginv2.NewClient
```

不得存在两份 `doJSON` 或两套 Session 请求结构。

### Step 3：回归

```powershell
go test ./internal/authruntime/zitadel/loginv2 -count=1
go test ./internal/listingkit/phoneonboardingpreflight ./internal/listingkit/zitadelsms -count=1
go -C hack/debug test ./listingkit-phone-onboarding-preflight -count=1
```

### Step 4：提交

```powershell
git add internal/authruntime/zitadel/loginv2 `
        internal/listingkit/phoneonboardingpreflight `
        hack/debug/listingkit-phone-onboarding-preflight
git commit -m "refactor: reuse verified ZITADEL phone session client"
```

---

## Task 2：验证 Actions v2 的真实签名、Freshness 和 Event Contract

**Files:**

- Create: `internal/accountentry/actionspreflight/probe.go`
- Create: `internal/accountentry/actionspreflight/probe_test.go`
- Create: `hack/debug/zitadel-actions-v2-preflight/main.go`
- Create: `hack/debug/zitadel-actions-v2-preflight/main_test.go`
- Create: `scripts/verify-zitadel-actions-v2.ps1`
- Create: `docs/verification/2026-09-03-zitadel-actions-v2.md`

### Step 1：锁定 v4.17.1 协议

基于：

```text
proto/zitadel/action/v2/execution.proto@v4.17.1
proto/zitadel/action/v2/target.proto@v4.17.1
```

验证：

```text
EventExecution 可按 event 或 group 触发
RESTAsync / RESTWebhook 请求形状
PAYLOAD_TYPE_JWT 的 iss/aud/iat/exp/jti
JSON Payload 中的稳定 event_id 与 signed creation time
X-ZITADEL-Signature 对 raw body 的 HMAC-SHA256
失败与重投行为
```

### Step 2：Freshness 是硬门槛

生产模式二选一：

```text
A. JWT：验证签名、iss、aud、iat、exp、jti 和最大时钟偏差
B. JSON HMAC：验证签名正文中的 event_id、event_created_at 和最大时钟偏差
```

只有 HMAC 而没有签名时间／唯一 nonce 时，Actions 集成必须保持关闭。

### Step 3：真实非生产测试

创建一次临时 Target 和 Execution，记录：

```text
Payload 类型
签名验证结果
事件 ID 字段
事件时间字段
重复投递行为
超时行为
```

所有 ID 只保留末 6 位；不记录手机号、Token 或完整 Payload。

### Step 4：提交

```powershell
go test ./internal/accountentry/actionspreflight -count=1
go -C hack/debug test ./zitadel-actions-v2-preflight -count=1
git add internal/accountentry/actionspreflight `
        hack/debug/zitadel-actions-v2-preflight `
        scripts/verify-zitadel-actions-v2.ps1 `
        docs/verification/2026-09-03-zitadel-actions-v2.md
git commit -m "test: verify ZITADEL actions v2 contract"
```

若门槛失败：禁用 Actions，继续使用同步初始化 + Reconciler；不得猜测字段。

---

## Task 3：增加统一 Account Entry 配置与版本化 Key Ring

**Files:**

- Create: `internal/core/config/type_account_entry.go`
- Create: `internal/core/config/keyring.go`
- Create: `internal/core/config/keyring_test.go`
- Create: `internal/core/config/validate_account_entry.go`
- Create: `internal/core/config/validate_account_entry_test.go`
- Modify: `internal/core/config/config.go`
- Modify: `internal/core/config/loader_builder.go`
- Modify: `internal/core/config/config_env_test.go`
- Create: `web/listingkit-ui/src/lib/server/account-entry-config.ts`
- Create: `web/listingkit-ui/src/lib/server/account-entry-config.test.ts`
- Modify: `.env.example`

### Step 1：Canonical Flag

Go 与 Next.js 只读取：

```text
TASK_PROCESSOR_ACCOUNT_ENTRY_ENABLED
TASK_PROCESSOR_ACCOUNT_ENTRY_SELF_REGISTRATION_ENABLED
```

旧名称出现时测试要求启动失败或明确拒绝，不能静默兼容。

### Step 2：结构化 Key Ring

```go
type VersionedSecret struct {
    Version string `json:"version"`
    Value   string `json:"value"`
}

type SecretKeyRing struct {
    Current  VersionedSecret   `json:"current"`
    Previous []VersionedSecret `json:"previous"`
}
```

环境变量：

```text
TASK_PROCESSOR_ACCOUNT_ENTRY_PHONE_HMAC_KEYRING_JSON
TASK_PROCESSOR_ACCOUNT_ENTRY_FLOW_AEAD_KEYRING_JSON
TASK_PROCESSOR_ACCOUNT_ENTRY_DEVICE_SIGNING_KEYRING_JSON
TASK_PROCESSOR_ACCOUNT_ENTRY_OPERATION_FINGERPRINT_KEYRING_JSON
```

测试：

```text
多个 Previous Key
重复 Version
Current 同时出现在 Previous
非法 Base64
错误 Key 长度
无 Current
轮换后旧 Version 可读
```

### Step 3：其他配置

```go
type AccountEntryConfig struct {
    Enabled                 bool
    SelfRegistrationEnabled bool
    PublicOrigin            string
    BFFToken                string
    LoginClientToken        string
    ProvisioningToken       string
    PasswordDecoyUserID     string
    PhoneHMACKeys           SecretKeyRing
    FlowAEADKeys            SecretKeyRing
    DeviceSigningKeys       SecretKeyRing
    OperationFingerprintKeys SecretKeyRing
    FlowTTL                 time.Duration
    ChallengeTTL            time.Duration
    CallbackDeliveryGrace   time.Duration
    MaxCallbackGenerations  int
    ResendCooldown          time.Duration
    ActionMaxClockSkew      time.Duration
    PasswordResponseFloor   time.Duration
    PasswordJitterMax       time.Duration
    PerPhoneHourly          int64
    PerIPHourly             int64
    PerDeviceHourly         int64
}
```

Enabled 时 Database、Redis、ZITADEL Tokens、Decoy User、Key Rings 和 HTTPS PublicOrigin 必填；loopback 开发可 HTTP。

### Step 4：运行与提交

```powershell
go test ./internal/core/config -run "AccountEntry|KeyRing" -count=1
Set-Location web/listingkit-ui
pnpm test -- src/lib/server/account-entry-config.test.ts
Set-Location ../..
git add internal/core/config web/listingkit-ui/src/lib/server/account-entry-config* .env.example
git commit -m "feat: add versioned account entry configuration"
```

---

## Task 4：建立通用 Operation Runtime 与多键 Claim

**Files:**

- Create: `internal/operationruntime/execution.go`
- Create: `internal/operationruntime/execution_test.go`
- Create: `internal/operationruntime/fingerprint.go`
- Create: `internal/operationruntime/fingerprint_test.go`
- Create: `internal/operationruntime/claim.go`
- Create: `internal/operationruntime/claim_test.go`
- Create: `internal/operationruntime/repository.go`
- Create: `internal/operationruntime/gorm_repository.go`
- Create: `internal/operationruntime/gorm_repository_test.go`
- Modify: `internal/workbench/schema/runtime.go`
- Modify: `internal/workbench/schema/runtime_test.go`

### Step 1：逻辑 Operation

```text
operation_executions
- scope_type
- scope_id
- operation_name
- operation_id
- request_fingerprint
- fingerprint_key_version
- state
- result_reference
- error_code
- lease_owner
- lease_until
- expires_at

UNIQUE(scope_type, scope_id, operation_name, operation_id)
```

规则：

```text
同 ID + 同 Fingerprint → 重放同一结果或继续同一 Operation
同 ID + 不同 Fingerprint → IDEMPOTENCY_CONFLICT
```

### Step 2：敏感字段指纹

Canonical Fingerprint 输入：

```text
非敏感字段 → canonical JSON
手机号 → identity/alias reference
OTP、当前密码、新密码 → HMAC(OperationFingerprintKeyVersion, raw secret bytes)
```

禁止存原值或无 Key Hash。重试先读取已有 Operation 的 Key Version，再计算指纹；旧 Key 保留到 Operation TTL 结束。

测试：

```text
相同 OTP 重试相同
修改 OTP 冲突
修改新密码冲突
原始秘密不在表、日志或错误
轮换期间使用 Operation 原 Version
```

### Step 3：多键 Claim

```text
operation_claims
- claim_type
- claim_key_hash
- owner_operation_id
- state
- lease_until
- version

UNIQUE(claim_type, claim_key_hash)
```

提供：

```go
AcquireManySorted(ctx, claimType, keys, owner, ttl)
CompleteMany(...)
ReleaseMany(...)
```

全部 Key 在同一事务按排序顺序获取，避免死锁。

### Step 4：真实并发测试

```text
不同 Operation ID 并发争夺同一业务 Claim
多实例并发
Lease Owner 崩溃恢复
旧 Owner 过期后不能再提交
```

### Step 5：提交

```powershell
go test ./internal/operationruntime -count=1
go test ./internal/workbench/schema -count=1
git add internal/operationruntime internal/workbench/schema
git commit -m "feat: add operation execution and claim runtime"
```

---

## Task 5：建立 Phone Identity、Flow、Callback Delivery 与 Invitation Schema

**Files:**

- Create: `internal/accountentry/domain.go`
- Create: `internal/accountentry/domain_test.go`
- Create: `internal/accountentry/repository.go`
- Create: `internal/accountentry/gorm_repository.go`
- Create: `internal/accountentry/gorm_repository_test.go`
- Create: `internal/accountentry/phone_identity.go`
- Create: `internal/accountentry/phone_identity_test.go`
- Create: `internal/accountentry/flow_cipher.go`
- Create: `internal/accountentry/flow_cipher_test.go`
- Create: `internal/accountentry/device_cookie.go`
- Create: `internal/accountentry/device_cookie_test.go`
- Create: `internal/accountentry/rate_limiter.go`
- Create: `internal/accountentry/rate_limiter_test.go`
- Modify: `internal/platform/redis/client.go`
- Modify: `internal/platform/redis/client_test.go`
- Modify: `internal/workbench/schema/runtime.go`
- Modify: `internal/workbench/schema/runtime_test.go`

### Step 1：Phone Identity 与 Alias

```text
account_phone_identities
- identity_id
- zitadel_user_id
- home_organization_id
- state: pending_provisioning | active | failed | quarantined
- owning_flow_id
- proof_verified_at
- activated_at

account_phone_fingerprint_aliases
- identity_id
- key_version
- fingerprint
- UNIQUE(key_version, fingerprint)
```

Current/Previous 全部 Alias 双读；命中旧 Alias 时同事务补写 Current Alias。

### Step 2：Flow 与预分配 Provider ID

```text
account_entry_flows
- id
- kind
- state
- provider_organization_id
- provider_user_id
- provider_resource_state
- encrypted_provider_state
- cipher_key_version
- identity_id
- invitation_id
- challenge_expires_at
- expires_at
- version
```

`provider_organization_id`、`provider_user_id` 在 ZITADEL POST 前生成并提交。

### Step 3：Callback Generation

```text
account_entry_callback_deliveries
- flow_id
- generation
- auth_request_fingerprint
- encrypted_callback_url
- cipher_key_version
- state
- delivered_at
- acknowledged_at
- expires_at
- UNIQUE(flow_id, generation)
```

状态：

```text
ready | delivering | acknowledged | consumed_unknown | superseded | expired
```

### Step 4：Invitation

```text
account_entry_invitations
- id
- token_hash
- organization_id
- role
- phone_identity_id
- state
- consuming_flow_id
- consumed_by_user_id
- expires_at
- version
- created_by
- operation_id
```

Token 只首次返回，数据库只存 SHA-256。

### Step 5：版本化 AEAD

密文保存 Key Version；Current 写、Previous 读，旧版本在下一次保存时重加密。测试覆盖滚动部署、重启、多个 Previous Key 和旧 Key 缺失 fail closed。

### Step 6：设备 Cookie 与三维限流

```text
shuomi_auth_device
HttpOnly
Secure
SameSite=Lax
Path=/
```

Redis 只接收 device/IP/phone HMAC。Lua 原子 `INCR + EXPIRE`；Redis 不可用时 Challenge fail closed。

### Step 7：提交

```powershell
go test ./internal/accountentry -run "Identity|Flow|Cipher|Device|Rate|Invitation" -count=1
go test ./internal/platform/redis -run Window -count=1
go test ./internal/workbench/schema -count=1
git add internal/accountentry internal/platform/redis internal/workbench/schema
git commit -m "feat: add durable account entry identities and flows"
```

---

## Task 6：扩展 ZITADEL Client，支持可恢复创建、OIDC、密码和精确清理

**Files:**

- Modify: `internal/authruntime/zitadel/loginv2/types.go`
- Modify: `internal/authruntime/zitadel/loginv2/client.go`
- Modify: `internal/authruntime/zitadel/loginv2/client_test.go`

### Step 1：Provider 预分配 ID

扩展：

```go
type CreateOrganizationInput struct {
    OrganizationID string
    TechnicalName  string
}

type CreateUserInput struct {
    UserID         string
    OrganizationID string
    Username       string
    TechnicalEmail string
    Phone          string
}
```

请求必须发送 v4.17.1 支持的：

```text
AddOrganizationRequest.organization_id
CreateUserRequest.user_id
```

新增：

```go
GetOrganizationByID(...)
GetUserByID(...)
```

### Step 2：Unknown Outcome Adopt

测试模拟：

```text
Provider 已持久化对象
→ 连接在响应前断开
→ 调用返回 outcome_unknown
→ 恢复按相同 ID Get
→ 属性匹配则 adopt
→ 不发第二个新 ID Create
```

属性不匹配返回 `ErrProviderCorrelationConflict` 并 quarantine。

### Step 3：OIDC 与新 Generation

```go
GetOIDCAuthRequest(...)
CreateOIDCCallback(...)
```

Callback Origin/Path 白名单锁定：

```text
/api/auth/callback/zitadel
/api/zitadel-auth/callback
```

每个新 Auth Request 可用同一未过期、已验证 Session 创建新的 callback generation。

### Step 4：密码

```go
AuthenticatePassword(userID, password, lifetime)
SetPassword(userID, newPassword)
```

Session Proof 增加 `PasswordVerifiedAt`。密码不进入错误或日志。

### Step 5：角色、授权与清理

```go
EnsureOrganizationOwner(...)
EnsureProjectAuthorization(...)
GetProjectAuthorization(...)
DeleteUser(userID)
DeleteSession(sessionID)
```

DeleteUser 只提供精确 ID 操作；是否允许删除由 Account Entry Service 的 ownership precondition 决定。

### Step 6：运行与提交

```powershell
go test ./internal/authruntime/zitadel/loginv2 -count=1
git add internal/authruntime/zitadel/loginv2
git commit -m "feat: extend recoverable ZITADEL account entry client"
```

---

## Task 7：增加 `base_payg` 和幂等业务就绪初始化器

**Files:**

- Modify: `internal/listingsubscription/types.go`
- Modify: `internal/listingsubscription/service.go`
- Modify: `internal/listingsubscription/service_test.go`
- Create: `internal/accountentry/initializer.go`
- Create: `internal/accountentry/initializer_test.go`

### Step 1：`base_payg`

```go
const PlanBasePayAsYouGo = "base_payg"
```

```text
名称：基础方案 · 按需使用
store_count = 1
store_renewal_periods = 0
ai_points = 0
data_rows = 0
状态 = active
无自动试用期
```

旧 plan 保留给历史租户；新自助注册只使用 `base_payg`。

### Step 2：自助注册顺序

```text
OTP Proof
→ Ensure business organization projection
→ Ensure base_payg
→ Ensure ORG_OWNER
→ Ensure listingkit_admin（最后）
→ Activate PhoneIdentity
```

每一步幂等并可 read-back。Provider 409 不自动视为成功。

### Step 3：邀请顺序

```text
OTP Proof
→ Ensure target organization authorization
→ Mark Invitation consumed + Access Projection ready（同一事务）
→ Activate new PhoneIdentity
```

邀请不创建默认企业、`base_payg` 或 ORG_OWNER。

### Step 4：提交

```powershell
go test ./internal/listingsubscription -run BasePay -count=1
go test ./internal/accountentry -run Initializer -count=1
git add internal/listingsubscription internal/accountentry/initializer*
git commit -m "feat: add idempotent account business readiness"
```

---

## Task 8：实现 Freshness-safe Actions v2 Inbox

**Files:**

- Create: `internal/accountentry/actionsreceiver/handler.go`
- Create: `internal/accountentry/actionsreceiver/handler_test.go`
- Create: `internal/accountentry/actionsreceiver/verifier.go`
- Create: `internal/accountentry/actionsreceiver/verifier_test.go`
- Create: `internal/accountentry/actionsreceiver/inbox.go`
- Create: `internal/accountentry/actionsreceiver/gorm_inbox.go`
- Create: `internal/accountentry/actionsreceiver/gorm_inbox_test.go`
- Create: `internal/accountentry/actionsreceiver/projector.go`
- Create: `internal/accountentry/actionsreceiver/projector_test.go`
- Modify: `internal/workbench/schema/runtime.go`

### Step 1：验证签名与时间

JWT：

```text
签名、iss、aud、iat、exp、jti
```

JSON HMAC：

```text
X-ZITADEL-Signature
raw body
signed event_id
signed event_created_at
abs(now - event_created_at) <= ActionMaxClockSkew
```

签名正确但过期必须拒绝。

### Step 2：Inbox

```text
account_identity_event_inbox
- event_id / jti
- event_type
- subject_id
- organization_id
- occurred_at
- received_at
- state
- attempts
- next_attempt_at
- payload_digest
- UNIQUE(event_id)
```

重复事件返回成功但不重复执行。乱序事件必须通过读取 ZITADEL 当前事实收敛，不根据事件顺序盲目覆盖。

### Step 3：非关键路径

Actions 失败不能阻止同步注册。Projector 只更新投影或唤醒 Reconciler；本地初始化服务仍核对业务就绪。

### Step 4：提交

```powershell
go test ./internal/accountentry/actionsreceiver -count=1
go test ./internal/workbench/schema -count=1
git add internal/accountentry/actionsreceiver internal/workbench/schema
git commit -m "feat: add fresh ZITADEL identity event inbox"
```

---

## Task 9：实现 Account Entry Service、状态恢复与轻量 Reconciler

**Files:**

- Create: `internal/accountentry/service.go`
- Create: `internal/accountentry/service_test.go`
- Create: `internal/accountentry/status.go`
- Create: `internal/accountentry/status_test.go`
- Create: `internal/accountentry/password.go`
- Create: `internal/accountentry/password_test.go`
- Create: `internal/accountentry/callback.go`
- Create: `internal/accountentry/callback_test.go`
- Create: `internal/accountentry/reconciler.go`
- Create: `internal/accountentry/reconciler_test.go`
- Create: `internal/accountentry/audit.go`
- Create: `internal/accountentry/audit_test.go`

### Step 1：注册前统一 Gate

`SELF_REGISTRATION=false` 时，在手机号 Normalize、Alias 查询或 Challenge 发送前统一拒绝；已存在和未知手机号行为一致。

### Step 2：并发注册与 Provider Recovery

```text
计算所有 Alias
→ AcquireManySorted phone_registration Claims
→ 持久化 Provider IDs
→ Create/adopt Organization
→ Create/adopt User
→ Challenge
```

测试两个不同 Operation ID、两个服务实例，以及 Provider 创建成功但响应丢失。

### Step 3：OTP Proof 与 Pending Identity

Proof 必须满足：

```text
SessionProof.UserID == Flow.ProviderUserID
SessionProof.OrganizationID == Flow.ProviderOrganizationID（自助注册）
UserVerifiedAt 非零
OTPSMSVerifiedAt 非零
Challenge 未过期
```

Proof 后 Identity 先进入 pending；只有 Task 7 完成后 active。

### Step 4：邀请

- 已有 active Identity：验证 Proof 后复用 User，Grant + consume 完成。
- 新 Identity：Grant + consume 完成前保持 pending。
- 清理 abandoned invited User 前重新读取角色、授权、Session 和引用；不确定则 quarantine。

### Step 5：密码登录等时工作

```text
active Identity → 对真实 User 调一次 ZITADEL password check
未知或 pending → 对 PasswordDecoyUserID 调一次同类 password check
全部路径 → 清理临时 Session → 应用 response floor + jitter
```

统一 `INVALID_CREDENTIALS`。测试延迟分布类别，不使用单次毫秒断言。

### Step 6：密码重置

真实 Identity 使用 OTP Proof 后 SetPassword；未知 Identity 使用假流程。成功后返回密码登录，不自动建立 Session。

### Step 7：Callback Delivery 与 Fresh Auth Request

首次：

```text
CreateCallback → generation 1 ready → 303 → /auth-complete ACK
```

响应丢失：

```text
code 未消费 → 重放当前 generation
无 ACK 超过 grace 或收到 invalid_grant → generation 标记 consumed_unknown/superseded
→ BFF 启动新的 Auth.js signIn
→ 新 authRequest 绑定同一 Flow
→ 若 ZITADEL Session 未过期，CreateCallback generation N+1
→ 若已过期，nextAction=reauthenticate
```

限制 `MaxCallbackGenerations`，超过后要求重新认证。

### Step 8：Flow Status

```go
type NextAction string
const (
    EnterPhone       NextAction = "enter_phone"
    EnterOTP         NextAction = "enter_otp"
    Provisioning     NextAction = "provisioning"
    EnterNewPassword NextAction = "enter_new_password"
    RedirectReady    NextAction = "redirect_ready"
    RestartOIDC      NextAction = "restart_oidc"
    Reauthenticate   NextAction = "reauthenticate"
    Completed        NextAction = "completed"
    Expired          NextAction = "expired"
)
```

只返回 nextAction、retryAfter、canResend、canRetry。

### Step 9：Reconciler

轮询：

```text
Provider outcome_unknown
pending_provisioning
Callback delivering 无 ACK
邀请 consuming
过期 Flow 和 abandoned User
Actions Inbox pending
```

恢复先读取 ZITADEL／PostgreSQL事实，再 adopt、继续或 quarantine；不生成新 Provider ID。

### Step 10：提交

```powershell
go test ./internal/accountentry -run "Register|OTP|Password|Invitation|Callback|Status|Reconcile" -count=1
git add internal/accountentry
git commit -m "feat: productize ZITADEL-native account entry"
```

---

## Task 10：暴露 HTTP API，并将邀请绑定 Live Organization

**Files:**

- Create: `internal/accountentry/httpapi/module.go`
- Create: `internal/accountentry/httpapi/handler.go`
- Create: `internal/accountentry/httpapi/handler_test.go`
- Create: `internal/accountentry/httpapi/contract.go`
- Create: `internal/accountentry/httpapi/contract_test.go`
- Modify: `internal/httproute/descriptor.go`
- Modify: `internal/app/httpapi/server_auth.go`
- Modify: `internal/app/httpapi/server_test.go`
- Modify: `internal/authz/listingkit.go`
- Modify: `internal/authz/listingkit_test.go`

### Step 1：Trusted BFF

```go
const AuthPolicyTrustedBFF AuthPolicy = "trusted_bff"
```

常量时间验证 BFF Token，并删除客户端伪造的 User/Org Header。

### Step 2：BFF-only Routes

```text
GET  /api/v1/account-entry/status
POST /api/v1/account-entry/challenges
POST /api/v1/account-entry/challenges/resend
POST /api/v1/account-entry/verifications
POST /api/v1/account-entry/password-logins
POST /api/v1/account-entry/password-resets/challenges
POST /api/v1/account-entry/password-resets/verifications
POST /api/v1/account-entry/password-resets/completions
POST /api/v1/account-entry/oidc-completions
POST /api/v1/account-entry/oidc-acknowledgements
POST /api/v1/integrations/zitadel/identity-events
```

Actions Route 使用自身 JWT/HMAC Freshness 验证，不接受用户 Bearer。

### Step 3：邀请 Route

```text
POST /api/v1/workbench/account-invitations
AuthPolicyVerifiedIdentity
OrganizationAccessPolicyLiveWrite
PermissionAccountInvitationCreate
```

请求体不含 `organizationId`；即使提交未知字段也拒绝。服务端只使用 live resolved Effective Organization。

权限：

```text
listingkit_admin → allow
platform_admin → 仅受控代管上下文 allow
listingkit_operator / viewer → deny
```

必须有 A 企业管理员提交 B 企业 ID／Cookie／Header 的拒绝测试。

### Step 4：输入和幂等

```text
JSON <= 16 KiB
拒绝未知字段和重复 JSON Key
写请求要求 Operation ID
同 ID 不同 Fingerprint → 409 IDEMPOTENCY_CONFLICT
```

### Step 5：提交

```powershell
go test ./internal/accountentry/httpapi -count=1
go test ./internal/authz -run Invitation -count=1
go test ./internal/app/httpapi -run "TrustedBFF|AccountEntry|Invitation" -count=1
git add internal/accountentry/httpapi internal/httproute internal/authz internal/app/httpapi
git commit -m "feat: expose scoped account entry api"
```

---

## Task 11：接入应用组合、数据库、Redis 与 Reconciler 生命周期

**Files:**

- Create: `internal/app/httpapi/accountentry_module.go`
- Create: `internal/app/httpapi/accountentry_module_test.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/composition_builder_test.go`
- Modify: `internal/app/httpapi/composition_modules.go`
- Modify: `internal/app/httpapi/types.go`
- Modify: `internal/app/httpapi/bootstrap.go`
- Modify: `internal/app/httpapi/bootstrap_test.go`

### Step 1：启动门槛

```text
Enabled=false → module nil，不要求 Secret
Enabled=true → DB、Redis、ZITADEL Tokens、Decoy User、全部 Key Ring 缺一即失败
```

### Step 2：组装

```text
open shared DB
→ migrate operation/account tables
→ open Redis
→ build loginv2 client
→ build Operation Runtime / repositories / key rings / limiter
→ build initializer / service / actions receiver
→ build HTTP module
→ start bounded lightweight reconciler
```

不创建 Account Entry Temporal Worker。

### Step 3：关闭顺序

Reconciler → Redis → DB；构造失败逆序清理，不留 goroutine。

### Step 4：提交

```powershell
go test ./internal/app/httpapi -run AccountEntry -count=1
go test ./internal/workbench/schema -count=1
git add internal/app/httpapi internal/workbench/schema
git commit -m "feat: wire ZITADEL-native account entry runtime"
```

---

## Task 12：实现 Next.js BFF、Flow Cookie 与 OIDC Generation Recovery

**Files:**

- Create: `web/listingkit-ui/src/lib/server/account-entry-bff.ts`
- Create: `web/listingkit-ui/src/lib/server/account-entry-bff.test.ts`
- Create: `web/listingkit-ui/src/lib/server/account-entry-cookie.ts`
- Create: `web/listingkit-ui/src/lib/server/account-entry-cookie.test.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/begin/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/status/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/challenge/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/resend/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/verify/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/password-login/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/password-reset/route.ts`
- Create: `web/listingkit-ui/src/app/api/account-entry/complete/route.ts`
- Create: `web/listingkit-ui/src/app/auth-complete/page.tsx`
- Create: `web/listingkit-ui/src/app/auth-complete/page.test.tsx`
- Modify: `web/listingkit-ui/src/app/api/zitadel-auth/login/route.ts`
- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.ts`
- Modify: `web/listingkit-ui/src/proxy.ts`

### Step 1：Begin Intent

```text
screen=login|register
method=otp|password
returnTo=合法站内路径
invite=一次性 Token（可选）
```

Intent 与 Operation sequences 保存于加密 HttpOnly Cookie；邀请原始 Token 从地址栏移除，不进 storage。

### Step 2：稳定 Operation ID

BFF 从 Intent/Flow 读取并复用：

```text
flow_id + operation_kind + sequence
```

HTTP 重试不递增 sequence；用户修改输入并主动重试才创建下一 sequence。

### Step 3：Flow/Device Cookie

```text
shuomi_account_entry_flow
shuomi_auth_device
HttpOnly
Secure
SameSite=Lax
Path=/
```

Cookie 不含手机号、Session Token、callback URL 或邀请 Token。

### Step 4：Status 驱动恢复

页面刷新只调用 Status；不通过重新 POST Challenge/Verify 猜测状态。

### Step 5：Callback 交付

`complete` 从 Go 取得当前 generation 的 callback URL并 303。`/auth-complete`：

```text
读取 Auth.js Session
验证 subject 与 Flow 预期一致
POST ACK
清除 Flow Cookie
跳转 returnTo
```

### Step 6：Code 已消费但 Session 丢失

Status 返回 `restart_oidc` 时：

```text
BFF 使用同一 Flow 启动新的 signIn("zitadel")
→ 保存新 authRequest generation
→ Custom Login 使用保留的已验证 ZITADEL Session 创建新 callback
```

若 Session 过期，返回 `reauthenticate`。

### Step 7：提交

```powershell
Set-Location web/listingkit-ui
pnpm test -- src/lib/server/account-entry src/app/api/account-entry src/app/auth-complete
pnpm typecheck
Set-Location ../..
git add web/listingkit-ui/src/lib/server/account-entry* `
        web/listingkit-ui/src/app/api/account-entry `
        web/listingkit-ui/src/app/auth-complete `
        web/listingkit-ui/src/app/api/zitadel-auth/login/route.ts `
        web/listingkit-ui/src/lib/server/zitadel-auth.ts `
        web/listingkit-ui/src/proxy.ts
git commit -m "feat: add recoverable account entry bff"
```

---

## Task 13：按 Figma 实现注册、登录和重置密码页面

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

### Step 1：注册字段

严格只有：

```text
手机号
短信验证码
协议确认
注册并进入
```

不得出现用户名、注册密码或邮箱。

### Step 2：登录

```text
验证码登录：手机号 + 验证码
密码登录：手机号 + 密码
```

Slice 1 不显示“记住登录状态”。

### Step 3：重置密码

```text
手机号 + 验证码 + 新密码 + 确认密码
成功后返回密码登录
```

### Step 4：Status UI

支持：

```text
enter_phone
enter_otp
provisioning
enter_new_password
redirect_ready
restart_oidc
reauthenticate
expired
```

`provisioning` 和 `restart_oidc` 不重复发送短信或重复提交业务初始化。

### Step 5：可访问性和响应式

```text
真实 label / radio / checkbox
aria-describedby
克制的 aria-live
键盘全流程
prefers-reduced-motion
1440×900 与 320px 无横向滚动
```

### Step 6：提交

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

## Task 14：部署 Secret、Actions Target、Decoy User 与 Feature Flags

**Files:**

- Modify: `deployments/kubernetes/zitadel/local/README.md`
- Modify: `deployments/kubernetes/zitadel/local/values.yaml`
- Modify: `deployments/kubernetes/listingkit-workbench/base/listingkit-ui-auth-config.yaml`
- Modify: `deployments/kubernetes/listingkit-workbench/base/listingkit-ui-deployment.yaml`
- Modify: `deployments/kubernetes/listingkit-workbench/base/product-listing-api-deployment.yaml`
- Create: `deployments/kubernetes/listingkit-workbench/base/account-entry-secret.example.yaml`
- Create: `docs/runbooks/shuomi-account-entry.md`
- Create: `tests/account_entry_commercial_readiness_test.go`

### Step 1：Key Ring Secret Contract

```yaml
stringData:
  bff-token: ""
  zitadel-login-client-token: ""
  zitadel-provisioning-token: ""
  zitadel-password-decoy-user-id: ""
  phone-hmac-keyring.json: '{"current":{"version":"","value":""},"previous":[]}'
  flow-aead-keyring.json: '{"current":{"version":"","value":""},"previous":[]}'
  device-signing-keyring.json: '{"current":{"version":"","value":""},"previous":[]}'
  operation-fingerprint-keyring.json: '{"current":{"version":"","value":""},"previous":[]}'
  actions-signing-key: ""
```

示例 Secret 不进入 base resources；真实 Secret 由 Overlay 或 Secret Manager 注入。

### Step 2：Canonical Env

UI 与 API 都使用：

```text
TASK_PROCESSOR_ACCOUNT_ENTRY_ENABLED
TASK_PROCESSOR_ACCOUNT_ENTRY_SELF_REGISTRATION_ENABLED
```

部署、Loader、Runbook 和 rollout tests 断言完全一致。

### Step 3：Decoy User

Runbook 创建一个：

```text
无项目授权
无企业管理角色
仅用于密码等时检查
不可进入 OIDC callback
```

并验证任何临时 Session 都被清理。

### Step 4：Actions

只有 Task 2 门槛通过才创建 Target/Execution。优先 JWT；JSON HMAC 必须含签名事件时间和 ID。Receiver endpoint、audience、时钟偏差和 Key 轮换写入 Runbook。

### Step 5：Rollout

```text
1. TASK_PROCESSOR_ACCOUNT_ENTRY_ENABLED=true
2. TASK_PROCESSOR_ACCOUNT_ENTRY_SELF_REGISTRATION_ENABLED=false
3. 验证既有用户 OTP/密码登录
4. 预发布开启 Self Registration
5. 完成新注册、邀请、callback generation、恢复矩阵
6. 再开放生产
```

### Step 6：提交

```powershell
go test ./tests -run AccountEntry -count=1
kubectl kustomize deployments/kubernetes/listingkit-workbench/overlays/staging > $null
kubectl kustomize deployments/kubernetes/listingkit-workbench/overlays/prod > $null
git add deployments/kubernetes docs/runbooks/shuomi-account-entry.md tests/account_entry_commercial_readiness_test.go
git commit -m "ops: configure ZITADEL-native account entry"
```

---

## Task 15：全量故障矩阵、真实预发布验收与发布报告

**Files:**

- Create: `web/listingkit-ui/e2e/account-entry.spec.ts`
- Modify: `web/listingkit-ui/e2e/accessibility.spec.ts`
- Create: `scripts/verify-shuomi-account-entry-release.ps1`
- Create: `docs/verification/2026-09-03-shuomi-account-entry-release.md`

### Step 1：自动化故障矩阵

必须覆盖：

```text
不同 Operation ID 并发注册同一手机号
Organization 创建成功但响应丢失
User 创建成功但响应丢失
Claim Owner 崩溃后 lookup/adopt
OTP Proof 后、base_payg 前崩溃
ORG_OWNER 后、listingkit_admin 前崩溃
新邀请 Identity 在 Grant 前保持 pending
邀请 Grant 成功、consume 响应丢失
跨企业邀请签发拒绝
callback 303 丢失且 code 未消费
Auth.js 已消费 code，但 Set-Cookie/redirect 丢失
新 Auth Request generation 恢复
ZITADEL Session 过期后 reauthenticate
Actions 签名正确但过期
Actions 重复、乱序、延迟
Phone/AEAD/Device/Fingerprint Key Ring 轮换
同 Operation ID 修改 OTP
同 Operation ID 修改新密码
未知、pending、真实用户的密码登录时序类别
注册关闭时已注册和未知手机号一致
页面刷新只走 Status，不重复写操作
```

### Step 2：全量回归

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

### Step 3：真实预发布验收

不重复证明 PR #218 的短信基础，只验证产品化增量：

```text
既有手机号 OTP → Auth.js Session
既有手机号密码 → Auth.js Session
新手机号 → 单条 OTP → 单一 User/Org → base_payg → ORG_OWNER → listingkit_admin
重复注册 → 不增加 User/Org
Provider 响应丢失模拟 → 相同 ID adopt
邀请注册 → 目标授权完成后 Identity 才 active
callback code 消费后响应丢失 → 新 generation 成功登录
密码重置 → 新密码可登录
Actions 重复与过期请求处理正确
敏感日志扫描通过
```

### Step 4：发布报告

记录 Git SHA、ZITADEL 版本、Actions 配置版本、Key Ring Version、测试命令及退出码、对象数量核对、短信条数、故障恢复结果、回滚演练和剩余风险。所有主体 ID 只保留末 6 位。

### Step 5：提交

```powershell
git add web/listingkit-ui/e2e `
        scripts/verify-shuomi-account-entry-release.ps1 `
        docs/verification/2026-09-03-shuomi-account-entry-release.md
git commit -m "test: verify ZITADEL-native account entry release"
```

---

## 完成定义

- [ ] PR #218 已验证的 OTP/SMS Client 被抽取复用，没有第二套验证码系统。
- [ ] ZITADEL 是身份主流程，Actions v2 仅作为 Freshness-safe 身份事件桥。
- [ ] Actions 关闭、重复或延迟不影响同步登录正确性。
- [ ] Organization/User 使用预分配 Provider ID；响应丢失后 lookup/adopt。
- [ ] 同一手机号在并发和 Key 轮换中只有一个 Identity/User。
- [ ] 新自助注册在 base_payg、ORG_OWNER、listingkit_admin 完成前保持 pending。
- [ ] 新邀请 Identity 在目标授权与 Invitation consume 完成前保持 pending。
- [ ] 邀请签发只使用 live resolved Effective Organization。
- [ ] 密码登录对未知和 pending 身份执行同类 ZITADEL Decoy Check，并通过时序测试。
- [ ] Request Fingerprint 对 OTP/密码使用服务端 Keyed HMAC。
- [ ] Phone、AEAD、Device、Fingerprint Secret 使用带 Version 的 Key Ring，并支持多个 Previous Key。
- [ ] callback 在 code 未消费时可重放，在 code 已消费且浏览器 Session 丢失时可生成新 Auth Request。
- [ ] Flow Status 支持刷新、重启和响应丢失恢复。
- [ ] Canonical Feature Flag 名称在所有层一致。
- [ ] Slice 1 不显示无效的“记住登录状态”。
- [ ] 手机号、验证码、密码、Session Token、callback URL 和管理凭据不进入日志、公开 JSON 或明文数据库列。
- [ ] Go、前端、Playwright、Kustomize 和商业就绪检查全部通过。
- [ ] 生产开关默认关闭，真实预发布验收后才开放。
