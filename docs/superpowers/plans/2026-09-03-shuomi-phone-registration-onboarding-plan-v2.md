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
9. `...-amendments-8.md`

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
AddOTPSMS user-factor enrollment
OTP-SMS factor read-back OR proven idempotent AddOTPSMS contract
CreateSMSChallenge exact request/returned challenge-bearing Session contract
VerifySMS against returned challenge-bearing Session
Project Grant search/create/read-back/state
User Authorization search/create/read-back/state
Provider Delete finality / idempotency / operation-status or bounded finality proof
```

所有 capability 都做最小权限负向测试。

`AddOTPSMS` 是 User Factor Write。若 response loss 后既不能 read-back factor state，也不能证明同 User 重放 enrollment 安全幂等，则禁止盲重试，self-registration rollout fail closed。

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
saas_registration_admissions
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

Generic Admission 在手机号 lookup 前固定：

```text
admission_id
logical_admission_key
capacity_slot_id
state
expires_at
request_fingerprint
```

Provider Operation：

```text
logical_operation_key         // 幂等权威
operation_id                  // opaque instance UUID
registration_id
ownership_epoch
kind
provider_target_type
provider_target_id
request_fingerprint
state = prepared | inflight | outcome_unknown | succeeded | failed_definitive
owner / lease_until / epoch
last_checked_at
```

数据库必须：

```text
UNIQUE(logical_admission_key)
UNIQUE(logical_operation_key)
```

同 logical key + same fingerprint 只 replay；same key + changed fingerprint 返回稳定 conflict，不能生成第二个并行 Provider mutation。

---

## Task 3：Register Atomic Admission Shield

`/register` 顺序固定：

```text
trusted ingress abuse checks
-> BEGIN TX
   lock unverified registration-admission capacity
   allocate ONE generic admission slot
   persist admission/attempt identity
   increment admission in_use once
-> COMMIT
-> if allocation failed:
     generic unavailable
     NO exact E.164 lookup
     NO OTP challenge
-> only after successful atomic admission:
     exact E.164 lookup
     server-internal split existing / pending / new
```

关键规则：

- lookup 前必须已经真实占有 slot，不能只 `check available`；
- last-slot 并发下只有成功 acquire 的请求继续 lookup；
- `new/pending` 复用同一 slot 转成 unverified Provider-object ownership，不二次申请；
- `existing active` 仍在本次 register attempt 内暂时持有同一 admission slot，直到 challenge definitive、流程取消/TTL 到期或安全转入 login continuation，再事务释放；
- admission retry 用 stable logical key，不能二次 `in_use++`；
- known/unknown/pending 在 admission 失败时同 HTTP/public code/nextAction，全部不发 SMS。

独立 `/login` 不依赖 registration admission capacity，不通过 register fallback 暴露 existence。

---

## Task 4：Capacity Authority 与 Class Transfer

Capacity 分三层：

### 4.1 Global Provider Object High-water

所有尚未安全删除或正式转出 registration pending accounting 的自助注册 Provider object，占用 global provider-object high-water。Class 转移不改变这个总数。

### 4.2 Anonymous Admission / Unverified Class

Generic register admission 与未验证 Provider object 使用 unverified class，保护匿名攻击面。

OTP proof 成功后必须在同一事务执行：

```text
lock proof + intent + global slot
-> persist otp_verified
-> slot.class = verified_waiting
-> decrement unverified admission class count
-> commit
```

因此已证明手机号所有权的 flow 不会继续占 anonymous admission pool。

### 4.3 Verified Onboarding Work Capacity

第一次 non-disposable business write 之前必须获取 verified onboarding work capacity：

```text
lock intent + verified work capacity
-> if full:
     state = verified_waiting_capacity
     write NO Consent/business projection/subscription
-> else:
     acquire verified work lease/slot
     atomically enter business_preparing + first durable business write
```

如果 concurrent paid/non-disposable artifact 已存在：只做 classification + verified work scheduling；identity 保持 verified class，禁止回退到 unverified，也禁止 cleanup 删除。

`authorized` 后释放 verified work slot，并将 global registration Provider slot 转出 pending accounting。

所有 acquire/transfer/release 事务可重放，不能二次增减。

---

## Task 5：Provider Provisioning Adapter + Logical Finality Fence

长期 Provider Write 全部走 durable operation：

```text
Create Organization
Create User
ownership marker repair/write
Reclaim marker mutation
Delete User/Organization
```

Logical key 推荐：

```text
provider-op:{registration_id}:{ownership_epoch}:{kind}:{target_type}:{target_id}
```

调用流程：

```text
insert/adopt prepared operation by logical key
-> exact fingerprint replay / changed fingerprint conflict
-> acquire operation epoch/lease
-> mark inflight before Provider call
-> provider call with bounded deadline
-> definite response => read-back + succeeded/failed_definitive
-> timeout/connection loss => outcome_unknown
-> reconciler resolves finality before conflicting mutation
```

Cleanup 不能越过 `prepared/inflight/outcome_unknown` 的 create/repair mutation。

Stale recovery：

- stale `prepared` 且确认未发 Provider call：Reconciler 可按同 logical key 取得新 lease 并继续；
- `inflight` lease 未过期：禁止接管；
- stale `inflight`：先 fenced CAS 为 `outcome_unknown`，绝不直接重发，再做 Provider finality/read-back；
- stale worker 晚到结果只有 epoch 仍匹配才可写本地状态。

Stable ID recovery：Get same ID -> exact adopt / missing marker safe repair / mismatch repair_required。

---

## Task 6：Login V2 OTP Registration Attempt

OTP 仍由 ZITADEL/Login V2 处理，task-processor 不保存 OTP code 或 plaintext Session Token。

每次 attempt：

```text
proof_attempt_id
registration_id
provider_user_id
challenge_generation
provider_session_ref_hash      // CreateSMSChallenge 返回后才写
challenge_state = preparing | ready | outcome_unknown
challenge_created_at
proof_verified_at
proof_consumed_at
```

首次短信固定使用 pinned contract：

```text
1. Ensure AddOTPSMS user factor is definitively installed
2. create proof_attempt = challenge_preparing（尚未绑定 Session）
3. call pinned CreateSMSChallenge
4. CreateSMSChallenge returns challenge-bearing sessionID/sessionToken
5. persist non-secret reference/hash of THIS returned Session + generation
6. VerifySMS against this exact returned Session
7. mark proof verified
```

禁止先绑定“Session A”再由 `CreateSMSChallenge` 创建“Session B”。

### AddOTPSMS unknown outcome

`AddOTPSMS` response loss 后：

- 有 user-factor read-back：确认 installed 后继续，不重复写；
- 或 provider contract 已实测 same-user enrollment 安全幂等：按该合同恢复；
- 两者都没有：`factor_enrollment_outcome_unknown -> repair_required`，禁止换 Session 后盲目再次 AddOTPSMS。

### Challenge response loss

`CreateSMSChallenge` 成功但 response 丢失时：

```text
challenge_outcome_unknown
SMS send debt counts against limiter/global budget
no immediate automatic resend
```

若 provider 无法恢复丢失的 challenge Session credential，只能在 cooldown 后由用户显式请求新的 `challenge_generation`；旧 generation 永不作为 proof。

Challenge/resend 必须有 per-session/phone/IP limit 与 global SMS budget。

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

只有 `verified_waiting` flow 获得 verified work capacity 后，才允许第一次 non-disposable business write。

```text
verified_waiting / verified_waiting_capacity
-> BEGIN TX
   acquire verified work capacity
   state = business_preparing
   persist first non-disposable Sumi onboarding state
-> COMMIT
-> persist current Consent
-> ensure Sumi user projection
-> ensure Sumi organization projection
-> EnsureInitialPlanIfAbsent(base_payg)
-> EnsureCurrentPlanEntitlementsReady
-> read-back subscription + effective entitlements
-> business_prepared
```

Destination/work capacity 已满时：保持 `verified_waiting_capacity`，不写 Consent/business projection/subscription；不会重新占 anonymous admission pool。

任何 paid subscription / Store / resource / order / Project Grant 等 non-disposable artifact 一旦存在，就禁止 Registration cleanup 删除 Provider identity。

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

统一负责业务状态：

```text
otp_verified
verified_waiting
verified_waiting_capacity
consent_required
business_preparing
business_prepared
project_grant_ready
authorizing
```

统一负责 Provider Operation：

```text
stale prepared
stale inflight
outcome_unknown
```

使用 bounded backoff/deadline/lease/epoch。

Provider Operation recovery：

```text
prepared no live lease + call not sent -> resume same logical op
inflight live lease -> wait
inflight expired -> fenced outcome_unknown -> read-back
outcome_unknown -> finality/read-back only; no blind conflicting write
```

`consent_required`：

- disposable、无 business artifact 的 registration 超时后可进入 bounded cleanup；
- 已有任何 durable business ownership 时禁止 Provider Delete，只允许 re-consent / repair；
- verified onboarding backlog 有独立 SLO，不与 anonymous provider admission 共用同一失败域。

---

## Task 13：Cleanup / Reclaim

Cleanup 前硬条件：

```text
no prepared/inflight/outcome_unknown long-lived Provider write
no paid/base subscription requiring identity ownership
no business projection requiring preservation
no Project Grant/User Authorization
no Store/resource/order/audit ownership references
Provider finality contract definitive
```

只有 disposable pre-business registration 才允许自动 Provider Delete。

Reclaim 必须使用新的 attempt-bound OTP Proof；与 Delete 通过 durable Provider Operation logical key / epoch 互斥。

Provider absence + all previous writes definitive 后才释放 global Provider-object capacity。

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
last admission slot known/unknown/pending concurrency: lookup 前只有一个成功 acquire
same admission logical key retry 不二次 in_use++
capacity full known/unknown/pending indistinguishable
OTP verified: unverified -> verified_waiting class atomic transfer
verified work capacity full: no business artifact write, no anonymous-pool reoccupation
Create Org/User response loss and delayed success
same Provider logical key two workers -> one operation / one external mutation
same logical key different fingerprint -> conflict
stale prepared -> reconciler resumes same operation
stale inflight -> outcome_unknown -> finality read-back
Janitor vs delayed Provider Create
marker write response loss
AddOTPSMS response loss + factor read-back installed
AddOTPSMS unknown without read-back/idempotency -> fail closed
CreateSMSChallenge returned Session is the exact proof-bound Session
challenge response loss -> no uncontrolled immediate resend
OTP challenge abuse budget
same proof changed acceptance payload
paid plan apply crash leaves stale entitlement -> canonical repair
manual override preserved
business-prepared policy change -> no Provider Delete
paid subscription concurrent with consent deadline -> no Provider Delete
Project Grant/User Authorization inactive fail closed
Provider Delete/Reclaim unknown outcome
legacy user migration
```

---

## 完成定义

- 没有复制 ZITADEL OTP/Password/Session/OIDC；
- `/register` 在 lookup 前原子占用 admission，last-slot race 不形成 existence oracle；
- Provider Operation 有稳定 logical key，同一逻辑 mutation 不会并行重复；
- stale prepared/inflight/outcome_unknown 都有明确 Reconciler owner；
- OTP Factor enrollment response loss 不通过“换 Session”伪装成回滚；
- proof 绑定 `CreateSMSChallenge` 实际返回的 challenge-bearing Session；
- Provider object creation/cleanup 有 durable finality fence；
- ownership marker 能力经过真实 capability gate；
- current paid plan entitlement projection 能修 missing + stale plan-owned rows；
- business-owned tenant 永不因 consent timeout 被误删；
- unverified admission、verified onboarding work、global Provider object 都有明确容量权威、SLO 和可恢复路径。
