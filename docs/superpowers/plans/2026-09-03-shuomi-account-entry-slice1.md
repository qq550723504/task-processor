# 硕米账号入口 Slice 1 Implementation Plan（ZITADEL 原生身份流）

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans`. Every task is TDD-first and must be committed separately.

**Goal:** 复用已经合并的手机号 OTP 能力，以 ZITADEL User／Organization／Session／OIDC 作为身份流程主干，以 Actions v2 作为签名身份事件桥接，并在 `task-processor` 中补齐并发 Claim、业务就绪、邀请、回调交付、状态恢复、BFF 和 Figma 页面。

**Authoritative design:**

- `docs/superpowers/specs/2026-09-03-shuomi-account-entry-zitadel-native-flow-design.md`
- `docs/superpowers/specs/2026-09-02-shuomi-console-phase1-hard-cut-design.md`（除账号入口被新设计覆盖的部分外）

**Tech stack:** Go 1.26、Gin/kernel module、GORM、PostgreSQL、Redis、ZITADEL Core/Login V2 + Actions v2 v4.17.1、Next.js 16、React 19、Auth.js 5、Zod、Vitest、Playwright。

---

## 一、执行边界

### 已实现并必须复用

PR #218 已经合并：

```text
internal/listingkit/zitadelsms
- ZITADEL HTTP SMS Provider
- 腾讯云短信 Relay
- OTP SMS 精确事件白名单
- HMAC 与错误脱敏

internal/listingkit/phoneonboardingpreflight
- 创建 Organization / Human User
- 添加 OTP SMS Factor
- 创建 SMS Challenge
- 验证短信 Code
- 读取 Session user + otpSms 因子
- 精确清理测试资源

hack/debug/listingkit-phone-onboarding-preflight
- 非生产真实设备验证命令
```

禁止：

```text
重新实现短信发送
建立短信验证码表
在 task-processor 中比较验证码
复制第二套 ZITADEL Session HTTP Client
把手机号字段 isVerified 当作 OTP Proof
```

### 第一阶段不做

```text
Temporal Account Entry Workflow
企业钱包、在线支付、退款、发票
企业自定义角色
成员月度消费额度
个人或企业实名认证
推广收益与提现
用户名密码登录
“记住登录状态”复选框
```

“记住登录状态”在 Auth.js 与 ZITADEL Session 生命周期没有完成逐次登录语义前不展示，避免无效产品承诺。

### 身份与业务边界

```text
ZITADEL：User、Phone、Password、OTP、Session、Organization、OIDC、项目授权
Actions v2：签名身份事件通知，不承载业务逻辑
PostgreSQL：手机号盲索引、Claim、幂等、业务就绪、邀请、callback 交付
Redis：短窗口限流
Auth.js：浏览器业务会话
Temporal：本 Slice 不使用
```

---

## 二、完成后的主要流程

### 自助注册

```text
/register
→ 统一检查 SELF_REGISTRATION
→ 设备 / IP / 手机号限流
→ 对全部有效手机号 HMAC Alias 取得注册 Claim
→ 创建 inert Organization + inert User（无角色、无套餐）
→ 复用 ZITADEL OTP SMS Challenge
→ 复用 ZITADEL Session OTP Proof
→ pending_provisioning 手机身份
→ Ensure base_payg
→ Ensure ORG_OWNER
→ 最后 Ensure listingkit_admin
→ 手机身份 active
→ ZITADEL OIDC CreateCallback
→ callback 加密可重放
→ Next.js 303 到 Auth.js callback
→ /auth-complete 确认 Session 并 ACK
→ /workbench 或合法 returnTo
```

### OTP 登录

```text
手机号 Blind Index → active User ID
→ ZITADEL SMS Challenge
→ user + otpSms Proof
→ OIDC callback 可重放交付
→ Auth.js Session ACK
```

### 密码登录

```text
手机号 Blind Index → active User ID
→ ZITADEL Session user + password check
→ OIDC callback 可重放交付
→ Auth.js Session ACK
```

### 密码重置

```text
手机号 + SMS OTP Proof
→ ZITADEL User API 设置新密码
→ 返回 /login?method=password
```

### 邀请注册

```text
已有 active 身份 → 复用同一个 User，增加目标企业授权
新手机号 → Claim → inert User → OTP Proof → 目标企业授权
不创建默认企业，不创建 base_payg，不默认授予 ORG_OWNER
```

---

## Task 0：重新校准现有实现与计划基线

**Files:**

- Read: `internal/listingkit/phoneonboardingpreflight/*`
- Read: `internal/listingkit/zitadelsms/*`
- Read: `web/listingkit-ui/src/auth.config.ts`
- Read: `web/listingkit-ui/src/auth.ts`
- Read: `web/listingkit-ui/src/app/api/zitadel-auth/*`
- Read: `internal/authz/listingkit.go`
- Create: `docs/verification/2026-09-03-account-entry-existing-baseline.md`

- [ ] 记录 PR #218 已覆盖的请求、测试和真实验证证据。
- [ ] 确认现有 OTP Challenge、Verify、Session Proof 的精确函数签名。
- [ ] 确认 Auth.js 回调路径、Session 策略、Cookie 名称与登出路径。
- [ ] 确认当前 `listingkit_admin`、`operator`、`viewer` 的授权映射。
- [ ] 搜索并证明仓库中不存在第二套业务短信验证码存储。
- [ ] 文档只记录脱敏 ID 后缀，不记录手机号、验证码、Session Token 或 callback URL。

**Run:**

```powershell
go test ./internal/listingkit/phoneonboardingpreflight ./internal/listingkit/zitadelsms -count=1
Set-Location web/listingkit-ui
pnpm test -- src/auth.config.test.ts src/lib/server/zitadel-auth.test.ts
Set-Location ../..
```

**Commit:**

```powershell
git add docs/verification/2026-09-03-account-entry-existing-baseline.md
git commit -m "docs: verify existing phone onboarding baseline"
```

---

## Task 1：完成 ZITADEL Actions v2 真实能力门槛

**Purpose:** 先验证当前自托管 v4.17.1 的 Actions v2 Target、Execution、签名、事件 Payload 和重复投递行为；不把未经验证的事件名或字段写入生产代码。

**Files:**

- Create: `internal/authruntime/zitadel/actionsv2/client.go`
- Create: `internal/authruntime/zitadel/actionsv2/client_test.go`
- Create: `internal/authruntime/zitadel/actionsv2/signature.go`
- Create: `internal/authruntime/zitadel/actionsv2/signature_test.go`
- Create: `hack/debug/zitadel-actions-v2-preflight/main.go`
- Create: `hack/debug/zitadel-actions-v2-preflight/main_test.go`
- Create: `scripts/verify-zitadel-actions-v2.ps1`
- Create: `docs/verification/2026-09-03-zitadel-actions-v2.md`

### Step 1：锁定协议来源

- [ ] 以 v4.17.1 下列 proto 为唯一协议来源：

```text
proto/zitadel/action/v2/action_service.proto
proto/zitadel/action/v2/execution.proto
proto/zitadel/action/v2/target.proto
```

- [ ] 测试 `EventExecution`、有序 Target、RESTAsync／RESTWebhook、`X-ZITADEL-Signature` HMAC-SHA256。

### Step 2：创建非生产 Target 和 Execution

- [ ] Target 指向一次性本地或预发布接收器。
- [ ] 使用独立签名 Key，不复用 SMS、BFF 或 Auth.js Secret。
- [ ] 先测试 proto 示例中的 `user.human.added`；其他 event name 必须从实际 Event/Audit 输出确认。
- [ ] 记录真实 Payload 是否包含：

```text
稳定 event ID
事件类型
aggregate / resource ID
organization ID
时间戳或序列
```

### Step 3：验证投递语义

- [ ] 证明正常投递、目标超时、5xx、重复投递和 Execution 禁用后的行为。
- [ ] 若无稳定事件 ID，记录 Actions 仅作为唤醒／审计信号，业务 Projector 使用事件复合指纹和 ZITADEL 回读。
- [ ] Actions 不进入登录阻断路径；门槛失败只阻止 Actions 收敛功能，不否定已验证的 OTP 登录能力。

**Run:**

```powershell
go test ./internal/authruntime/zitadel/actionsv2 -count=1
go -C hack/debug test ./zitadel-actions-v2-preflight -count=1
pwsh -File ./scripts/verify-zitadel-actions-v2.ps1
```

**Commit:**

```powershell
git add internal/authruntime/zitadel/actionsv2 `
        hack/debug/zitadel-actions-v2-preflight `
        scripts/verify-zitadel-actions-v2.ps1 `
        docs/verification/2026-09-03-zitadel-actions-v2.md
git commit -m "test: verify ZITADEL actions v2 identity events"
```

---

## Task 2：抽取已验证的 ZITADEL Login V2 Client

**Files:**

- Create: `internal/authruntime/zitadel/loginv2/client.go`
- Create: `internal/authruntime/zitadel/loginv2/types.go`
- Create: `internal/authruntime/zitadel/loginv2/client_test.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/zitadel_client.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/zitadel_client_test.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/runner.go`
- Modify: `internal/listingkit/phoneonboardingpreflight/runner_test.go`
- Modify: `hack/debug/listingkit-phone-onboarding-preflight/*`

### Step 1：先复制契约测试，不复制实现

覆盖：

```text
CreateOrganization
CreateTechnicalUser
AddOTPSMS
CreateSMSChallenge
VerifySMS
GetSession
DeleteSession
DeleteOrganization
Provisioning / Login Client Token 隔离
Provider 错误脱敏与响应大小上限
```

### Step 2：移动实现

`phoneonboardingpreflight` 只保留别名或薄适配器：

```go
type Client = loginv2.Client
type ClientConfig = loginv2.ClientConfig
type SessionProof = loginv2.SessionProof
var NewClient = loginv2.NewClient
```

禁止保留两套 `doJSON`。

### Step 3：回归

```powershell
go test ./internal/authruntime/zitadel/loginv2 -count=1
go test ./internal/listingkit/phoneonboardingpreflight ./internal/listingkit/zitadelsms -count=1
go -C hack/debug test ./listingkit-phone-onboarding-preflight -count=1
```

**Commit:**

```powershell
git add internal/authruntime/zitadel/loginv2 `
        internal/listingkit/phoneonboardingpreflight `
        hack/debug/listingkit-phone-onboarding-preflight
git commit -m "refactor: reuse verified ZITADEL login client"
```

---

## Task 3：建立通用 Operation Runtime 最小基础

**Purpose:** 抽取会被账号入口和后续店铺激活复用的本地幂等／Claim 原语，但不自行实现 Workflow Engine。

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

### Step 1：Operation Execution

```text
operation_executions
- scope_type
- scope_id
- operation_name
- idempotency_key
- request_fingerprint
- state: executing | succeeded | retryable_failed | permanent_failed
- result_reference
- error_code
- lease_owner
- lease_until
- expires_at
- version
- UNIQUE(scope_type, scope_id, operation_name, idempotency_key)
```

规则：

```text
同 Key + 同 Fingerprint → 等待或重放同一结果
同 Key + 不同 Fingerprint → IDEMPOTENCY_CONFLICT
响应丢失 → 重试取得已存结果
租约过期 → 允许受控恢复
```

### Step 2：Business Claim

```text
operation_claims
- claim_type
- claim_key_hash
- owner_operation_id
- state
- lease_until
- version
- UNIQUE(claim_type, claim_key_hash)
```

支持：

```go
AcquireManySorted
Renew
Complete
Release
RecoverExpired
```

多 Claim 按规范化 Key 排序后在一个事务中获取，避免 HMAC 多 Alias 死锁。

### Step 3：请求指纹

- [ ] 只对 Schema 校验后的 Canonical Command 计算 SHA-256/HMAC。
- [ ] 密码、验证码、Session Token、callback URL、完整手机号不得进入可观察指纹输入或日志。
- [ ] 操作 ID由 `flow + operation + sequence` 稳定派生，BFF 重试不能生成新逻辑 ID。

**Run:**

```powershell
go test ./internal/operationruntime -count=1
go test ./internal/workbench/schema -count=1
```

**Commit:**

```powershell
git add internal/operationruntime internal/workbench/schema
git commit -m "feat: add operation idempotency and claim runtime"
```

---

## Task 4：增加账号入口配置、Key Ring 与设备身份

**Files:**

- Create: `internal/core/config/type_account_entry.go`
- Create: `internal/core/config/validate_account_entry.go`
- Create: `internal/core/config/validate_account_entry_test.go`
- Modify: `internal/core/config/config.go`
- Modify: `internal/core/config/loader_builder.go`
- Modify: `internal/core/config/config_env_test.go`
- Create: `internal/accountentry/keyring.go`
- Create: `internal/accountentry/keyring_test.go`
- Create: `internal/accountentry/device.go`
- Create: `internal/accountentry/device_test.go`
- Create: `web/listingkit-ui/src/lib/server/account-entry-config.ts`
- Create: `web/listingkit-ui/src/lib/server/account-entry-config.test.ts`
- Modify: `.env.example`

### Step 1：配置模型

```go
type VersionedSecret struct {
    Version string
    Value   string
}

type AccountEntryConfig struct {
    Enabled                 bool
    SelfRegistrationEnabled bool
    PublicOrigin            string
    BFFToken                string

    PhoneHMACCurrent  VersionedSecret
    PhoneHMACPrevious []VersionedSecret
    FlowAEADCurrent   VersionedSecret
    FlowAEADPrevious  []VersionedSecret
    DeviceHMACCurrent VersionedSecret
    DeviceHMACPrevious []VersionedSecret

    LoginClientToken  string
    ProvisioningToken string
    ActionsSigningKey string

    FlowTTL                 time.Duration
    ChallengeTTL            time.Duration
    ResendCooldown          time.Duration
    MaxVerificationAttempts int

    PerPhoneHourly        int64
    PerIPHourly           int64
    PerIPDaily            int64
    PerDeviceHourly       int64
    PerDeviceDaily        int64
    PerPhoneIPHourly      int64
    PerPhoneDeviceHourly  int64
}
```

### Step 2：Key Ring 规则

- [ ] 当前写 Key 只能有一个。
- [ ] 所有 Previous Key 均可读。
- [ ] 密钥版本唯一且非空。
- [ ] AEAD Key 解码后恰好 32 字节。
- [ ] HMAC Key 解码后至少 32 字节。
- [ ] 密文记录保存 `cipher_key_version`。
- [ ] 读取旧密文后下一次写入自动升级到 Current Key。
- [ ] 提供轮换 Runbook，旧 Key 移除必须晚于最大 Flow TTL 和滚动部署窗口。

### Step 3：设备 Cookie

```text
shuomi_auth_device
HttpOnly
Secure（loopback 开发除外）
SameSite=Lax
Path=/
随机 128 bit 以上
服务端签名并带 key version
```

客户端不能自报设备 ID；签名失败时替换 Cookie 并累积风险计数。

### Step 4：注册禁用枚举防护

`SelfRegistrationEnabled=false` 时，在任何手机号 Binding 查询前统一返回同一注册关闭状态。

**Run:**

```powershell
go test ./internal/core/config -run AccountEntry -count=1
go test ./internal/accountentry -run "KeyRing|Device" -count=1
Set-Location web/listingkit-ui
pnpm test -- src/lib/server/account-entry-config.test.ts
Set-Location ../..
```

**Commit:**

```powershell
git add internal/core/config internal/accountentry/keyring* internal/accountentry/device* `
        web/listingkit-ui/src/lib/server/account-entry-config* .env.example
git commit -m "feat: add account entry key rings and device identity"
```

---

## Task 5：建立 Phone Identity、Alias、Attempt、Callback 与 Invitation 数据模型

**Files:**

- Create: `internal/accountentry/domain.go`
- Create: `internal/accountentry/domain_test.go`
- Create: `internal/accountentry/repository.go`
- Create: `internal/accountentry/gorm_repository.go`
- Create: `internal/accountentry/gorm_repository_test.go`
- Create: `internal/accountentry/cipher.go`
- Create: `internal/accountentry/cipher_test.go`
- Modify: `internal/workbench/schema/runtime.go`
- Modify: `internal/workbench/schema/runtime_test.go`

### Step 1：Phone Identity 与 Alias

```text
account_phone_identities
- identity_id
- zitadel_user_id
- home_organization_id
- state: pending_provisioning | active | quarantined
- owning_flow_id
- proof_verified_at
- activated_at
- version

account_phone_fingerprint_aliases
- identity_id
- key_version
- fingerprint
- UNIQUE(key_version, fingerprint)
- UNIQUE(identity_id, key_version)
```

同一 `zitadel_user_id` 只能有一个 Identity。

### Step 2：Attempt

```text
account_entry_attempts
- id
- kind: register | otp_login | password_login | password_reset | invitation
- state
- identity_id
- phone_claim_operation_id
- encrypted_provider_state
- cipher_key_version
- auth_request_fingerprint
- invitation_id
- agreement_version
- verification_attempts
- operation_sequence_json
- challenge_expires_at
- expires_at
- version
```

Provider State 可包含手机号、Session ID、Session Token，但必须 AEAD；不得包含验证码或密码。

### Step 3：Callback Delivery

```text
account_entry_callback_deliveries
- flow_id
- encrypted_callback_url
- cipher_key_version
- expires_at
- delivery_count
- last_delivered_at
- acknowledged_at
- version
```

第一次返回 callback 不能删除密文。

### Step 4：Invitation

```text
account_entry_invitations
- id
- token_hash
- organization_id
- role
- phone_identity_constraint
- state
- expires_at
- consuming_flow_id
- consumed_by_user_id
- version
- created_by
- operation_id
```

原始 Token 只返回一次；数据库只存 SHA-256。

### Step 5：HMAC 轮换并发测试

- [ ] Current + Previous 双读。
- [ ] 读取旧 Alias 时同事务补写新 Alias。
- [ ] 新旧实例并发注册同一手机号只产生一个 `identity_id`。
- [ ] 多 Alias Claim 同事务获取。

**Run:**

```powershell
go test ./internal/accountentry -run "Identity|Alias|Attempt|Callback|Invitation|Cipher|Rotation" -count=1
go test ./internal/workbench/schema -count=1
```

**Commit:**

```powershell
git add internal/accountentry internal/workbench/schema
git commit -m "feat: add durable account identity state"
```

---

## Task 6：补齐 ZITADEL OIDC、密码、授权和精确清理 Client

**Files:**

- Modify: `internal/authruntime/zitadel/loginv2/client.go`
- Modify: `internal/authruntime/zitadel/loginv2/types.go`
- Modify: `internal/authruntime/zitadel/loginv2/client_test.go`

### Step 1：OIDC

```go
GetOIDCAuthRequest(context.Context, string) (OIDCAuthRequest, error)
CreateOIDCCallback(context.Context, OIDCCallbackInput) (string, error)
```

端点锁定：

```text
GET  /v2/oidc/auth_requests/{id}
POST /v2/oidc/auth_requests/{id}
```

Callback URL 只允许配置 Origin，以及：

```text
/api/auth/callback/zitadel
/api/zitadel-auth/callback
```

无 userinfo、无 fragment，必须含 `code` 与 `state`。

### Step 2：密码

```go
AuthenticatePassword(context.Context, PasswordSessionInput) (SessionMaterial, SessionProof, error)
SetPassword(context.Context, SetPasswordInput) error
```

密码登录按已解析的 User ID做 `user + password` Session Check；不依赖按手机号搜索 Provider。

### Step 3：组织与项目授权

```go
EnsureOrganizationOwner(context.Context, organizationID, userID string) error
EnsureProjectAuthorization(context.Context, input ProjectAuthorizationInput) (string, error)
GetProjectAuthorization(...)
```

409 必须读回并核对目标主体、组织、项目和角色，不可一律视为成功。

### Step 4：邀请废弃 User 清理

```go
DeleteUser(context.Context, userID string) error
GetUser(context.Context, userID string) (UserSnapshot, error)
```

删除前由领域服务核对 `created_by_flow_id`、无 active Identity、无角色、无成功 Session、无其他引用。Client 只执行精确 User ID删除。

### Step 5：Session Proof

```go
type SessionProof struct {
    UserID             string
    OrganizationID     string
    UserVerifiedAt     time.Time
    OTPSMSVerifiedAt   time.Time
    PasswordVerifiedAt time.Time
}
```

**Run:**

```powershell
go test ./internal/authruntime/zitadel/loginv2 -count=1
```

**Commit:**

```powershell
git add internal/authruntime/zitadel/loginv2
git commit -m "feat: extend ZITADEL account entry client"
```

---

## Task 7：增加 Actions v2 Identity Event Inbox 与 Projector

**Files:**

- Create: `internal/accountentry/identityevent/event.go`
- Create: `internal/accountentry/identityevent/repository.go`
- Create: `internal/accountentry/identityevent/gorm_repository.go`
- Create: `internal/accountentry/identityevent/receiver.go`
- Create: `internal/accountentry/identityevent/receiver_test.go`
- Create: `internal/accountentry/identityevent/projector.go`
- Create: `internal/accountentry/identityevent/projector_test.go`
- Create: `internal/accountentry/identityevent/httpapi/module.go`
- Create: `internal/accountentry/identityevent/httpapi/handler.go`
- Create: `internal/accountentry/identityevent/httpapi/handler_test.go`
- Modify: `internal/workbench/schema/runtime.go`
- Modify: `internal/workbench/schema/runtime_test.go`

### Step 1：Inbox

```text
account_identity_event_inbox
- delivery_id
- event_id / event_fingerprint
- event_type
- aggregate_type
- aggregate_id
- raw_body_digest
- state
- attempts
- next_attempt_at
- last_error_code
- received_at
- UNIQUE(event_id) 或验证后的复合唯一键
```

### Step 2：Receiver

```text
POST /api/v1/integrations/zitadel/identity-events
```

- [ ] 验证 `X-ZITADEL-Signature`，常量时间比较。
- [ ] 请求体 <= 64 KiB，拒绝重复 JSON Key。
- [ ] 原子写 Inbox 后快速返回。
- [ ] 不信任 Payload 中的角色或业务就绪状态，必要时回读 ZITADEL。
- [ ] 不记录完整原始 Payload；只存受控字段和 digest。

### Step 3：Projector

Actions 事件可以：

```text
唤醒 pending Identity 收敛
记录 User / Org / Authorization 事实快照
发现授权撤销
补偿漏掉的本地映射
```

不能：

```text
绕过 OTP Proof 激活身份
单独创建 base_payg
仅凭事件 Payload 授予业务权限
```

**Run:**

```powershell
go test ./internal/accountentry/identityevent/... -count=1
go test ./internal/workbench/schema -count=1
```

**Commit:**

```powershell
git add internal/accountentry/identityevent internal/workbench/schema
git commit -m "feat: add ZITADEL identity event inbox"
```

---

## Task 8：实现幂等 Business Readiness 初始化器

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

默认：

```text
名称：基础方案 · 按需使用
store_count：1
store_renewal_periods：0
ai_points：0
data_rows：0
状态：active
无试用期、无伪造到期日
```

旧套餐保留给历史租户；新注册只用 `base_payg`。

### Step 2：初始化顺序

```text
OTP Proof
→ Ensure local organization projection
→ Ensure base_payg
→ Ensure ORG_OWNER
→ Ensure listingkit_admin（最后）
→ Activate Phone Identity
```

每一步有稳定 Operation ID 和可验证结果。`listingkit_admin` 授予后若前序不完整，Identity 不得 active，Reconciler 必须收敛或隔离。

### Step 3：邀请规则

```text
不创建默认 Organization
不创建 base_payg
不默认授予 ORG_OWNER
只确保邀请角色
```

**Run:**

```powershell
go test ./internal/listingsubscription -run BasePay -count=1
go test ./internal/accountentry -run Initializer -count=1
```

**Commit:**

```powershell
git add internal/listingsubscription internal/accountentry/initializer*
git commit -m "feat: add idempotent account business readiness"
```

---

## Task 9：实现 Account Entry Service、状态恢复和轻量 Reconciler

**Files:**

- Create: `internal/accountentry/service.go`
- Create: `internal/accountentry/service_test.go`
- Create: `internal/accountentry/status.go`
- Create: `internal/accountentry/status_test.go`
- Create: `internal/accountentry/reconciler.go`
- Create: `internal/accountentry/reconciler_test.go`
- Create: `internal/accountentry/audit.go`
- Create: `internal/accountentry/audit_test.go`

### Step 1：Service Contract

```go
type Service interface {
    StartChallenge(context.Context, ChallengeRequest) (ChallengeResult, error)
    ResendChallenge(context.Context, ResendRequest) (ChallengeResult, error)
    VerifyOTP(context.Context, VerifyOTPRequest) (VerifyResult, error)
    AuthenticatePassword(context.Context, PasswordRequest) (VerifyResult, error)
    CompletePasswordReset(context.Context, PasswordResetRequest) error
    PrepareOIDCCallback(context.Context, PrepareCallbackRequest) error
    DeliverOIDCCallback(context.Context, DeliverCallbackRequest) (CallbackDelivery, error)
    AcknowledgeOIDCCallback(context.Context, CallbackAckRequest) error
    Status(context.Context, StatusRequest) (SafeFlowStatus, error)
}
```

### Step 2：并发注册

不同 Idempotency-Key 同时注册同一手机号：

```text
计算 Current + Previous 全部 Alias
→ AcquireManySorted phone_registration Claims
→ 只有 Owner 创建 inert Provider 资源
→ 其他请求复用 owning Flow 的安全状态
```

必须有真实数据库并发测试，不只使用内存 Stub。

### Step 3：注册关闭

`SelfRegistration=false` 在 Binding 查询前统一拒绝；已有和未知手机号行为相同。

### Step 4：Pending Binding

OTP Proof 后：

```text
Identity pending_provisioning
→ Business Readiness
→ active
```

登录遇到 pending 时不走普通 callback；返回 `provisioning` 状态并触发 Reconciler。

### Step 5：假流程和枚举防护

OTP 登录／重置对不存在手机号：

```text
相同响应结构
相近时间等级
不创建 User
不发送短信
不可完成 Proof
```

### Step 6：Callback 可重放与 ACK

```text
CreateCallback 只调用一次
callback 密文保留到 ACK 或过期
同一 Deliver Operation 重放同一 URL
/auth-complete 确认 Auth.js Session 后 ACK
ACK 后清除密文并 completed
```

### Step 7：Safe Status

仅返回：

```text
enter_phone
enter_otp
provisioning
enter_password
enter_new_password
redirect_ready
completed
expired
quarantined
```

页面刷新不得通过重发 Challenge 推断状态。

### Step 8：邀请废弃清理

Reconciler 对 Flow 创建的 User执行所有权检查后 DeleteUser；失败重试，状态不确定转 quarantined。

### Step 9：Reconciler

数据库租约扫描：

```text
pending_provisioning
callback_ready / delivering
过期 Attempt
过期 Claim
Identity Event Inbox 重试
abandoned invited User
```

不使用 Temporal，不重发短信，不伪造 Proof。

**Run:**

```powershell
go test ./internal/accountentry -run "Service|Register|OTP|Password|Status|Callback|Reconcile|Concurrent|Enumeration" -count=1
```

**Commit:**

```powershell
git add internal/accountentry/service* internal/accountentry/status* `
        internal/accountentry/reconciler* internal/accountentry/audit*
git commit -m "feat: implement ZITADEL-native account entry service"
```

---

## Task 10：增加 Trusted BFF API 和邀请权限

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

- [ ] 常量时间校验独立 BFF Token。
- [ ] 删除伪造用户／企业 Header。
- [ ] 不运行浏览器 Bearer 身份验证。

### Step 2：公开给 BFF 的路由

```text
GET  /api/v1/account-entry/status
POST /api/v1/account-entry/challenges
POST /api/v1/account-entry/challenges/resend
POST /api/v1/account-entry/verifications
POST /api/v1/account-entry/password-logins
POST /api/v1/account-entry/password-resets/challenges
POST /api/v1/account-entry/password-resets/verifications
POST /api/v1/account-entry/password-resets/completions
POST /api/v1/account-entry/oidc-callbacks/prepare
POST /api/v1/account-entry/oidc-callbacks/deliver
POST /api/v1/account-entry/oidc-callbacks/ack
```

### Step 3：邀请签发路由

```text
POST /api/v1/workbench/account-invitations
```

正式权限：

```go
const PermissionAccountInvitationCreate = "account.invitation.create"
```

映射：

```text
listingkit_admin → allow
platform_admin → 仅受控代管上下文 allow
operator / viewer → deny
```

增加正反测试。

### Step 4：幂等协议

BFF 传递由 Flow 派生的逻辑 Operation ID；Go 同时验证 Request Fingerprint。相同 Key 不同载荷返回 `409 IDEMPOTENCY_CONFLICT`。

### Step 5：输入和错误边界

```text
JSON <= 16 KiB
读取超时 15s
拒绝未知字段和重复 Key
标准错误 Envelope
不暴露 Provider Body 或主体存在性
```

**Run:**

```powershell
go test ./internal/accountentry/httpapi -count=1
go test ./internal/authz -run Invitation -count=1
go test ./internal/app/httpapi -run "TrustedBFF|AccountEntry" -count=1
```

**Commit:**

```powershell
git add internal/accountentry/httpapi internal/httproute `
        internal/app/httpapi/server* internal/authz
git commit -m "feat: expose account entry and invitation APIs"
```

---

## Task 11：组装运行时和生命周期

**Files:**

- Create: `internal/app/httpapi/accountentry_module.go`
- Create: `internal/app/httpapi/accountentry_module_test.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/composition_builder_test.go`
- Modify: `internal/app/httpapi/composition_modules.go`
- Modify: `internal/app/httpapi/types.go`
- Modify: `internal/app/httpapi/bootstrap.go`
- Modify: `internal/app/httpapi/bootstrap_test.go`

- [ ] `Enabled=false` 时不构造任何 Account Entry 依赖。
- [ ] `Enabled=true` 时 Database、Redis、ZITADEL Token、Key Ring 和 BFF Token 缺失即启动失败。
- [ ] 组装：DB → schema → Redis → loginv2 → operationruntime → accountentry → identity event inbox → Reconciler → HTTP module。
- [ ] 构造失败逆序关闭资源；不遗留 goroutine。
- [ ] Reconciler 使用有界并发、租约和退避。
- [ ] 不增加 Temporal Client 依赖。

**Run:**

```powershell
go test ./internal/app/httpapi -run AccountEntry -count=1
go test ./internal/workbench/schema -count=1
```

**Commit:**

```powershell
git add internal/app/httpapi internal/workbench/schema
git commit -m "feat: wire account entry runtime"
```

---

## Task 12：实现 Next.js BFF、Flow 恢复和 callback ACK

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
- Create: `web/listingkit-ui/src/app/auth-complete/route.test.ts`
- Modify: `web/listingkit-ui/src/app/api/zitadel-auth/login/route.ts`
- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.ts`
- Modify: `web/listingkit-ui/src/proxy.ts`

### Step 1：Intent 和 Flow Cookie

```text
shuomi_account_entry_intent
shuomi_account_entry_flow
shuomi_auth_device
```

全部 HttpOnly、Secure、SameSite=Lax。Flow Cookie 只含随机 ID 与 MAC，不含手机号、邀请 Token、Session 或 callback。

### Step 2：逻辑 Operation ID

Operation Sequence 保存在服务端 Attempt；BFF 从 Status/Mutation Response 取得当前操作句柄并在浏览器重试中复用。禁止每次 Route Handler 生成新的业务幂等键。

### Step 3：Status-first 恢复

页面加载和刷新先调用 `/api/account-entry/status`，根据 `nextAction` 渲染，不自动重发验证码或重复 Provisioning。

### Step 4：Complete 与 ACK

```text
POST /api/account-entry/complete
→ 取得可重放 callback
→ 验证 Origin / Path / code / state
→ 303
→ Auth.js callback
→ /auth-complete
→ serverAuth 确认 Session subject
→ POST callback ACK
→ 清理 Flow Cookie
→ redirect returnTo
```

Go/BFF 或 BFF/浏览器响应丢失时，Status 仍返回 `redirect_ready`，可重试相同 callback。

### Step 5：安全转发

- [ ] 精确 Origin 校验。
- [ ] 上游响应 <= 64 KiB。
- [ ] 只转发白名单字段。
- [ ] 不转发 `Set-Cookie`、Provider `Location`、内部 ID或原始错误。

**Run:**

```powershell
Set-Location web/listingkit-ui
pnpm test -- src/lib/server/account-entry src/app/api/account-entry src/app/auth-complete
pnpm typecheck
Set-Location ../..
```

**Commit:**

```powershell
git add web/listingkit-ui/src/lib/server/account-entry* `
        web/listingkit-ui/src/app/api/account-entry `
        web/listingkit-ui/src/app/auth-complete `
        web/listingkit-ui/src/app/api/zitadel-auth/login/route.ts `
        web/listingkit-ui/src/lib/server/zitadel-auth.ts `
        web/listingkit-ui/src/proxy.ts
git commit -m "feat: add recoverable account entry bff"
```

---

## Task 13：按 Figma 实现注册、登录与重置页面

**Files:**

- Create: `web/listingkit-ui/src/components/auth-entry/*`
- Create: `web/listingkit-ui/src/app/register/page.tsx`
- Replace: `web/listingkit-ui/src/app/login/page.tsx`
- Create: `web/listingkit-ui/src/app/forgot-password/page.tsx`
- Modify: `web/listingkit-ui/src/components/application-frame.tsx`
- Modify: `web/listingkit-ui/src/app/globals.css`

### UI Contract

注册只包含：

```text
手机号码
短信验证码
协议确认
注册并进入
已有账号？立即登录
```

不存在：

```text
用户名
注册密码
邮箱
经营画像
```

登录：

```text
验证码登录：手机号 + 验证码
密码登录：手机号 + 密码
```

第一阶段不展示“记住登录状态”。

重置：

```text
手机号 + 验证码 + 新密码 + 确认密码
成功后返回密码登录
```

### 状态恢复

页面根据 Status API支持：

```text
手机号输入
验证码输入
初始化中
设置新密码
准备跳转
已过期
隔离错误
```

刷新不会重新发送短信。

### 可访问性

```text
输入框有 label
错误有 aria-describedby
登录方式为 tablist/tab/tabpanel
倒计时克制使用 aria-live
prefers-reduced-motion 关闭光场动画
1440×900 与 320px 均无横向滚动
```

**Run:**

```powershell
Set-Location web/listingkit-ui
pnpm test -- src/components/auth-entry src/app/login src/app/register src/app/forgot-password
pnpm lint
pnpm typecheck
pnpm build
Set-Location ../..
```

**Commit:**

```powershell
git add web/listingkit-ui/src/components/auth-entry `
        web/listingkit-ui/src/app/login `
        web/listingkit-ui/src/app/register `
        web/listingkit-ui/src/app/forgot-password `
        web/listingkit-ui/src/components/application-frame.tsx `
        web/listingkit-ui/src/app/globals.css
git commit -m "feat: implement Shuomi account entry pages"
```

---

## Task 14：配置 Actions v2、Kubernetes Secret 和发布开关

**Files:**

- Modify: `deployments/kubernetes/zitadel/local/README.md`
- Modify: `deployments/kubernetes/zitadel/local/values.yaml`
- Modify: `deployments/kubernetes/listingkit-workbench/base/listingkit-ui-auth-config.yaml`
- Modify: `deployments/kubernetes/listingkit-workbench/base/listingkit-ui-deployment.yaml`
- Modify: `deployments/kubernetes/listingkit-workbench/base/product-listing-api-deployment.yaml`
- Create: `deployments/kubernetes/listingkit-workbench/base/account-entry-secret.example.yaml`
- Create: `docs/runbooks/shuomi-account-entry.md`
- Create: `docs/runbooks/zitadel-actions-v2-identity-events.md`
- Create: `tests/account_entry_commercial_readiness_test.go`

### Secret Contract

```yaml
stringData:
  bff-token: ""
  phone-hmac-current: ""
  phone-hmac-previous: ""
  flow-aead-current: ""
  flow-aead-previous: ""
  device-hmac-current: ""
  device-hmac-previous: ""
  zitadel-login-client-token: ""
  zitadel-provisioning-token: ""
  zitadel-actions-signing-key: ""
```

示例 Secret 不加入 base resources；真实 Secret 由 Overlay 或 Secret Manager 提供。

### Actions 发布顺序

```text
1. Receiver 上线但不配置 Execution
2. 非生产创建 Target
3. 验证签名和 Payload
4. 配置 RESTAsync Event Execution
5. 观察重复／失败行为
6. 再启用 Projector
```

Actions 不作为自助注册开关。

### Account Entry 开关

```text
1. ACCOUNT_ENTRY_ENABLED=true, SELF_REGISTRATION=false
2. 验证既有用户 OTP／密码登录
3. 预发布 SELF_REGISTRATION=true
4. 完成并发、重试、轮换和清理验收
5. 再开放生产
```

**Run:**

```powershell
go test ./tests -run AccountEntry -count=1
kubectl kustomize deployments/kubernetes/listingkit-workbench/overlays/staging > $null
kubectl kustomize deployments/kubernetes/listingkit-workbench/overlays/prod > $null
```

**Commit:**

```powershell
git add deployments/kubernetes docs/runbooks tests/account_entry_commercial_readiness_test.go
git commit -m "ops: configure ZITADEL-native account entry"
```

---

## Task 15：全量验证、故障注入与发布报告

**Files:**

- Create: `web/listingkit-ui/e2e/account-entry.spec.ts`
- Modify: `web/listingkit-ui/e2e/accessibility.spec.ts`
- Create: `scripts/verify-shuomi-account-entry-release.ps1`
- Create: `docs/verification/2026-09-03-shuomi-account-entry-release.md`

### 自动化故障矩阵

```text
同手机号、不同 Idempotency-Key 并发注册
新旧 HMAC Key 实例并发
Claim Owner 在 Provider 创建前／后崩溃
OTP Proof 后、base_payg 前崩溃
base_payg 后、listingkit_admin 前崩溃
callback 创建后 Go→BFF 响应丢失
BFF 303 响应丢失
Auth.js 成功、ACK 响应丢失
页面刷新和浏览器重启
Actions Event 重复、延迟、5xx 和签名错误
邀请 User 创建后放弃
DeleteUser 失败和服务重启
SELF_REGISTRATION=false 下已存在／未知手机号一致
相同 Operation ID + 不同 Payload 冲突
```

### 产品 E2E

```text
OTP 登录
手机号密码登录
手机号注册
重复注册不新增 User/Org
密码重置
邀请注册不创建默认企业
Status-first 页面恢复
callback URL 不进入浏览器 JSON
无“记住登录状态”无效选项
320px 和 1440×900
axe
```

### 全量命令

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

### 真实预发布验收

复用 PR #218 的短信基础，只验证产品化增量：

```text
已有手机号 OTP 登录 → Auth.js Session
已有手机号密码登录 → Auth.js Session
新手机号 → 一条 OTP → 一个 User → 一个 Org → base_payg → ORG_OWNER → listingkit_admin
并发重复注册 → 无重复 Provider 资源
邀请新用户放弃 → 精确清理或 quarantined
callback 响应丢失 → 可恢复
Actions 重复投递 → 无重复业务效果
密钥轮换演练 → active Flow 和手机号唯一性保持
```

报告只记录脱敏 ID 后缀，不记录手机号、验证码、密码、Session Token 或 callback URL。

**Commit:**

```powershell
git add web/listingkit-ui/e2e scripts/verify-shuomi-account-entry-release.ps1 `
        docs/verification/2026-09-03-shuomi-account-entry-release.md
git commit -m "test: verify ZITADEL-native account entry release"
```

---

## 三、完成定义

- [ ] PR #218 的 OTP 实现被复用，没有第二套短信或 Session Client。
- [ ] ZITADEL User／Session／OIDC 是身份流程权威。
- [ ] Actions v2 使用签名 Target 和幂等 Inbox，只负责收敛，不承载业务代码。
- [ ] Account Entry 第一阶段不使用 Temporal。
- [ ] 不同请求、不同 Idempotency-Key 的并发注册仍只产生一个 User／Organization。
- [ ] HMAC Key 轮换保持一手机号一 Identity。
- [ ] AEAD Key 轮换不破坏 active Flow。
- [ ] OTP Proof 之前没有业务角色和套餐。
- [ ] Pending Identity 不能普通登录。
- [ ] `listingkit_admin` 是自助注册初始化最后的业务访问效果。
- [ ] 注册关闭时不能枚举账号。
- [ ] 邀请废弃 User 可精确清理或进入 quarantined。
- [ ] 邀请权限已加入现有 Authorizer 并有正反测试。
- [ ] 逻辑 Operation ID 可跨浏览器／代理重试复用，并校验 Fingerprint。
- [ ] callback 可重放，Auth.js Session ACK 后才清理。
- [ ] Flow Status 支持刷新、重启和响应丢失恢复。
- [ ] 设备、IP、手机号及组合限流均生效。
- [ ] 第一阶段不展示无效的“记住登录状态”。
- [ ] 注册页只有手机号、验证码和协议。
- [ ] OTP／密码登录进入同一种 Auth.js 会话。
- [ ] 全量 Go、前端、Playwright、Kustomize 和商业就绪检查通过。
- [ ] 生产自助注册默认关闭，真实预发布验收后才开放。

## 四、相对上一版计划的修订

```text
采用：ZITADEL 原生身份流 + Actions v2 事件收敛
取消：Account Entry Temporal Workflow 设想
增加：PostgreSQL Operation Runtime 最小幂等／Claim 原语
增加：多 HMAC Alias 和版本化 AEAD Key Ring
增加：pending / active Phone Identity
增加：callback 重放 + Auth.js ACK
增加：只读 Flow Status
增加：设备限流
增加：邀请 User 精确删除
增加：account.invitation.create 授权映射
修正：inert Organization/User 必须在 SMS Challenge 前创建
修正：注册禁用必须在 Binding 查询前统一处理
移除：第一阶段“记住登录状态”复选框
```
