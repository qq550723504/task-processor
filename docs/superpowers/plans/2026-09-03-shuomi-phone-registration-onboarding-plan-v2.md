# 硕米手机号注册与 Onboarding Implementation Plan V2

**目标：** 在不复制 ZITADEL IAM 能力的前提下，实现手机号自助注册所需的最小 Registration Provisioning + Sumi Onboarding，并把所有历史评审收敛成一个可直接执行的计划。

## 权威输入

执行本计划前必须同时读取：

1. `2026-09-03-shuomi-phone-registration-onboarding-design.md`
2. `2026-09-03-shuomi-phone-registration-onboarding-review-amendments.md`
3. `...-amendments-2.md`
4. `...-amendments-3.md`
5. `...-amendments-4.md`
6. `...-amendments-5.md`
7. `...-amendments-6.md`
8. `...-amendments-7.md`

本 V2 计划是执行入口；旧 `2026-09-03-shuomi-phone-registration-onboarding-plan.md` 仅保留历史，不再作为实施指令。

---

## 架构边界

```text
ZITADEL / Login V2
  User / Phone / OTP / Password / Session / OIDC

Registration Provisioning
  short-lived Registration Intent
  stable Provider IDs
  Provider Create/Get/Adopt/Finality
  capacity / cleanup / reclaim

Sumi Onboarding
  Consent
  business projections
  base_payg / current-plan entitlement readiness
  Project Grant
  User Authorization
```

禁止实现：本地 OTP/Password、PhoneIdentity 长期目录、Decoy User、自研 OIDC Callback Runtime、Temporal 登录 Saga。

---

## Task 1：Provider Capability Gate

预发布验证 pinned / production-compatible ZITADEL：

```text
exact global E.164 login-name lookup
Organization caller-supplied ID + required opaque name
Human User caller-supplied ID + Technical Email request shape
Get Organization/User exact ID
User/Organization ownership marker read/write + read-back
Create/Get Session
AddOTPSMS
CreateSMSChallenge
VerifySMS
Project Grant search/create/read-back/state
User Authorization search/create/read-back/state
Provider Delete finality / idempotency / operation-status or bounded finality proof
```

所有 capability 都做最小权限负向测试。

Stop condition：任一能力无法证明且设计依赖它时，self-registration feature 保持关闭。

---

## Task 2：Registration Schema 与状态

独立 migration owner：

```text
internal/registrationprovisioning/schema/runtime.go
```

核心持久化：

```text
saas_registration_intents
saas_registration_capacity
saas_registration_proof_attempts
saas_registration_proof_acceptances
saas_registration_provider_operations
saas_account_consents
saas_onboarding_preparations
saas_policy_releases
```

Intent 在任何长期 Provider Write 前固定：

```text
registration_id
normalized_phone              // 短生命周期
provider_organization_id
provider_organization_name    // stable opaque
provider_user_id
presented_policy_version
state
cleanup_state
capacity_slot_id
capacity_class
expires_at
```

Provider Operation：

```text
operation_id
registration_id
kind
provider_target_id
request_fingerprint
state
owner / lease_until / epoch
last_checked_at
```

数据库约束保证同一长期 Provider mutation 的 state / epoch / ownership 可串行化。

---

## Task 3：Register Admission Shield

`/register` 顺序固定：

```text
trusted ingress abuse checks
-> check atomic unverified_provider_capacity
-> full => generic unavailable, no login-name lookup, no OTP challenge
-> capacity available => exact E.164 lookup
-> server internal split existing / pending / new
```

known/unknown/pending 在 capacity full 时必须同 public result、同 nextAction，全部不发 SMS。

独立 `/login` 不依赖 registration provider-capacity，不通过 register fallback 暴露 existence。

---

## Task 4：Atomic Capacity Model

Provider Create 前原子占用 `unverified_provider_capacity`。

```text
lock capacity
-> allocate slot
-> insert Intent
-> in_use++
-> commit
```

规则：

- unresolved / quarantine 未确认删除前不释放；
- reclaim 转移同一 slot，不二次 +1；
- OTP verified 后若仍无 durable business ownership，继续占 unverified slot；
- 一旦 `business_prepared` 或检测到 paid/non-disposable business ownership，原子转移到 `verified_onboarding_backlog`；
- `authorized` 后释放 verified slot；
- 两个 capacity class 都有 hard limit / SLO / alert；
- capacity 变化只能由数据库事务完成，重放不可二次增减。

---

## Task 5：Provider Provisioning Adapter + Finality Fence

长期 Provider Write 全部走 durable operation：

```text
Create Organization
Create User
ownership marker repair/write
Reclaim marker mutation
Delete User/Organization
```

调用流程：

```text
persist prepared operation
-> acquire epoch/lease
-> provider call with bounded deadline
-> definite response => read-back + succeeded/failed_definitive
-> timeout/connection loss => outcome_unknown
-> reconciler resolves finality before conflicting mutation
```

Cleanup 不能越过 `prepared/inflight/outcome_unknown` 的 create/repair mutation。

Stable ID recovery：Get same ID -> exact adopt / missing marker safe repair / mismatch repair_required。

---

## Task 6：Login V2 OTP Registration Attempt

OTP 仍由 ZITADEL/Login V2 处理。

每次 attempt 绑定：

```text
proof_attempt_id
registration_id
provider_user_id
provider_session_ref_hash
challenge_created_at
proof_verified_at
proof_consumed_at
```

首次短信流程严格复用 pinned contract：

```text
Create/Get Session
-> AddOTPSMS
-> CreateSMSChallenge
-> VerifySMS
```

`AddOTPSMS` response-loss 的恢复必须先由 capability test 证明；不能盲目重复未知写入。无法 read-back/idempotent recover 时废弃 Session，建立新的 attempt-bound Session。

Challenge / resend 必须有 per-session / phone / IP limit 与 global SMS budget。

---

## Task 7：Attempt-bound Agreement Acceptance

用户在 OTP Verify Server Action 同时提交当前协议勾选。

只有 Verify 成功后才写 acceptance：

```text
proof_attempt_id PK
registration_id
provider_user_id
policy_version
accepted=true
accepted_at=server time
source
```

Acceptance insert-only；same attempt changed payload -> `ACCEPTANCE_REPLAY_CONFLICT`。

Consent 事务锁 proof + acceptance，并要求 attempt/user/registration/current policy 完全一致后一次 consume。

---

## Task 8：Current Policy Authority

`saas_policy_releases` 是 active policy version 的数据库单一权威。

业务 API 的 CurrentConsentPolicy：

```text
verified identity
-> live effective organization
-> DB active policy (bounded cache)
-> current consent
-> business handler
```

cache 过期且 DB 不可读时 fail closed。

---

## Task 9：Subscription + Entitlement Readiness

`base_payg` 进入现有 `internal/listingsubscription` Catalog。

新增：

```text
EnsureInitialPlanIfAbsent
EnsureCurrentPlanEntitlementsReady
```

`EnsureCurrentPlanEntitlementsReady` 必须：

```text
lock current subscription/version
-> load canonical current plan
-> reconcile every plan-owned entitlement row
   missing -> insert
   stale canonical value -> update
   exact -> no-op
-> preserve explicit override layer
-> bounded retry if subscription/version changed
-> read-back current plan + effective required entitlements
```

Plan-owned row 必须具有 provenance；legacy provenance 不明确先 migration/classification。

已有 paid plan 永不被 base_payg 覆盖。

---

## Task 10：Business Prepare

```text
otp_verified
-> business_preparing
-> persist current Consent
-> ensure Sumi user projection
-> ensure Sumi organization projection
-> EnsureInitialPlanIfAbsent(base_payg)
-> EnsureCurrentPlanEntitlementsReady
-> read-back subscription + effective entitlements
-> business_prepared
```

任何 paid subscription / Store / resource / order / Project Grant 等 non-disposable artifact 一旦存在，就禁止 Registration cleanup 删除 Provider identity。

`business_prepared` 后把 capacity 从 unverified class 转到 verified onboarding class。

---

## Task 11：Non-mutating Project Grant + User Authorization

Project Grant：

```text
absent -> Create exact allowed role set
roles exact AND ACTIVE -> adopt
roles different/inactive/revoked -> PROJECT_GRANT_REPAIR_REQUIRED
```

User Authorization：

```text
absent -> Create listingkit_admin
exact AND ACTIVE -> adopt
roles different/inactive/revoked -> AUTHORIZATION_REPAIR_REQUIRED
```

禁止使用会 Update/Reactivate 现有对象的高层 Ensure helper。

成功后：

```text
business_prepared
-> project_grant_ready
-> authorizing
-> authorized
```

---

## Task 12：RegistrationReconciler

统一负责：

```text
otp_verified
consent_required
business_preparing
business_prepared
project_grant_ready
authorizing
provider operation outcome_unknown
```

使用 bounded backoff/deadline/lease/epoch。

`consent_required`：

- disposable、无 business artifact 的 registration 超时后可进入 bounded cleanup；
- 已有任何 durable business ownership 时禁止 Provider Delete，只允许 re-consent / repair；
- verified onboarding backlog 有独立 SLO，不与 anonymous provider-cap 共用同一失败域。

---

## Task 13：Cleanup / Reclaim

Cleanup 前硬条件：

```text
no in-flight/unknown long-lived Provider write
no paid/base subscription requiring identity ownership
no business projection requiring preservation
no Project Grant/User Authorization
no Store/resource/order/audit ownership references
Provider finality contract definitive
```

只有 disposable pre-business registration 才允许自动 Provider Delete。

Reclaim 必须使用新的 attempt-bound OTP Proof；与 Delete 通过 durable Provider Operation / epoch 互斥。

Provider absence + all previous writes definitive 后才释放 capacity。

---

## Task 14：Legacy Backfill

新 readiness gate 默认关闭。

Backfill：

```text
existing ZITADEL membership/role
existing subscription/entitlements
business projections
legacy consent status
```

不伪造 consent，不覆盖 paid plan。

---

## Task 15：Internal Credential Boundaries

分开：

```text
Login Credential
Registration Provisioning Credential
Sumi Onboarding / Project Authorization Credential
Cleanup Credential
```

所有 credential 最小权限、current/previous overlap rotation、负向跨租户测试。

---

## Task 16：故障与安全验收

至少覆盖：

```text
capacity full known/unknown/pending indistinguishable
Create Org/User response loss and delayed success
Janitor vs delayed Provider Create
marker write response loss
AddOTPSMS response loss
OTP challenge abuse budget
same proof changed acceptance payload
paid plan apply crash leaves stale entitlement -> canonical repair
manual override preserved
business_prepared policy change -> no Provider Delete
paid subscription concurrent with consent deadline -> no Provider Delete
capacity class transfer replay
Project Grant/User Authorization inactive fail closed
Provider Delete/Reclaim unknown outcome
legacy user migration
```

---

## 完成定义

- 没有复制 ZITADEL OTP/Password/Session/OIDC；
- `/register` capacity saturation 不形成 existence oracle；
- Provider object creation/cleanup 有 durable finality fence；
- OTP factor enrollment 顺序与 pinned contract 一致；
- ownership marker 能力经过真实 capability gate；
- current paid plan entitlement projection 能修 missing + stale plan-owned rows；
- business-owned tenant 永不因 consent timeout 被误删；
- 所有 pending capacity 都有明确 owner、上限、SLO 和可恢复路径。
