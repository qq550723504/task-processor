# 硕米手机号注册与 Onboarding Implementation Plan V4

**目标：** 在不复制 ZITADEL IAM 能力的前提下，实现手机号自助注册所需的最小 Registration Provisioning + Sumi Onboarding。本 V4 是 PR #283 的唯一执行入口。

## 权威输入

执行前读取原始 design 与 review amendments 1–10。若历史内容冲突，以本 V4 与 `2026-09-03-shuomi-phone-registration-onboarding-review-amendments-10.md` 为准。

旧 `...onboarding-plan.md`、`...plan-v2.md`、`...plan-v3.md` 仅保留历史，不再作为实施指令。

---

## 架构边界

```text
ZITADEL / Login V2
  User / Phone / OTP / Password / Factor / Session / OIDC

Registration Provisioning
  short-lived Registration Intent
  branch-neutral admission + provider headroom leases
  stable Provider IDs
  Provider Create/Get/Adopt/Finality
  stable Provider target mutation fence
  cleanup / reclaim

Sumi Onboarding
  attempt-bound Consent
  business ownership fence
  business projections
  base_payg / entitlement readiness
  Project Grant
  User Authorization
```

禁止：本地 OTP/Password、PhoneIdentity 长期目录、Decoy User、自研 OIDC Callback Runtime、Temporal 登录 Saga、plaintext ZITADEL Session Token 持久化。

---

## Task 1：ZITADEL Capability Gate

预发布必须实测 pinned / production-compatible ZITADEL：

```text
exact global E.164 login-name lookup
Organization caller-supplied ID + opaque name
Human User caller-supplied ID + Technical Email request shape
Get Organization/User exact ID
User/Organization ownership marker read/write/read-back
OTP-SMS factor read-back OR proven idempotent same-user AddOTPSMS
CreateSMSChallenge exact request + returned challenge-bearing Session
VerifySMS against returned Session
Project Grant search/create/read-back/state
User Authorization search/create/read-back/state
```

以下每一种 durable Provider Write 必须证明 provider-side idempotency、operation status、conditional mutation或 staging 验证的 bounded finality：

```text
Create Organization
Create Human User
ownership marker write/repair
Reclaim ownership mutation
Delete User / Organization
```

Stable caller-supplied ID 不是 finality 证明。任一依赖能力无法证明，self-registration feature 保持关闭。

所有 capability 做最小权限和跨租户负向测试。

---

## Task 2：Registration Schema / Stable Identity / Fences

独立 migration owner：

```text
internal/registrationprovisioning/schema/runtime.go
```

核心表：

```text
saas_registration_intents
saas_registration_admission_slots
saas_registration_provider_capacity_slots
saas_registration_attempt_leases
saas_registration_proof_attempts
saas_registration_proof_acceptances
saas_registration_provider_operations
saas_registration_provider_target_fences
saas_registration_ownership_fences
saas_account_consents
saas_onboarding_preparations
saas_policy_releases
```

Intent 在任何 Provider Create 前固定：

```text
registration_id
provider_identity_id          // local stable identity across reclaim epochs
provider_organization_id
provider_organization_name    // opaque immutable
provider_user_id
normalized_phone              // short-lived only
ownership_epoch
presented_policy_version
state
expires_at
```

Provider Operation：

```text
logical_operation_key UNIQUE
operation_id
registration_id
ownership_epoch
kind
target_type / target_id
request_fingerprint
state = prepared | inflight | outcome_unknown | succeeded | failed_definitive
owner / lease_until / epoch
```

Provider Target Fence identity **不得**包含 registration_id / ownership_epoch：

```text
UNIQUE(provider_target_type, provider_target_id)
active_logical_operation_key
active_registration_id
active_ownership_epoch
mutation_class
state
lease_owner / lease_until / fence_epoch
```

Business Ownership Fence：

```text
UNIQUE(provider_organization_id)
registration_id
state = onboarding_writable | cleanup_claimed | preserved_business
cleanup_epoch
```

---

## Task 3：`/register` Pre-lookup 双 Semaphore Admission

用固定 slot pool，而不是易漂移的 in-use counter。

Generic Admission Slot：

```text
slot_id
state = free | leased
lease_key
lease_until
```

Provider Pending Capacity Slot：

```text
slot_id
state = free | leased | pending_object
lease_key
lease_until
provider_identity_id nullable
```

在任何 E.164 lookup 前，一个 PostgreSQL transaction：

```text
lock/reclaim expired generic slots
lock/reclaim expired temporary provider-headroom slots
acquire one generic admission lease
acquire one branch-neutral provider-headroom lease
bind both to stable logical attempt key
commit
```

任一 acquire 失败：不保留另一 lease，generic unavailable，**NO lookup / NO SMS**。

lookup 后 internal classify：

```text
existing active
pending registration-owned
genuinely-new
```

随后一个事务：

- 三分支统一 release generic admission；
- existing active：release temporary provider headroom；
- pending：release temporary provider headroom，继续使用该 identity 原有 `pending_object` slot；
- genuinely-new：把 temporary provider headroom 原子转换成 `pending_object(provider_identity_id)`，再允许 Provider Create。

因此最后一个 Provider slot 也不会产生 known/new 可观察差异。

独立 `/login` 不依赖该 register capacity。

---

## Task 4：Lease Recovery / Capacity Lifecycle

Temporary generic admission / provider-headroom 都有 `lease_until`：

- acquire transaction 可直接 `FOR UPDATE SKIP LOCKED` 复用 expired lease；
- same logical attempt retry adopt 自己仍有效的 lease；
- 低频 Admission Reaper 扫描异常 expired leases 并审计；
- request crash/cancel 不会永久占容量。

`pending_object` 是真实 pending Provider identity 的 capacity ownership，**没有普通 TTL 自动释放**：

```text
definitive Provider Delete -> free
authorized / formal account -> transfer out of registration pending accounting
```

Reclaim 不申请第二个 pending_object slot；同一 `provider_identity_id` 沿 ownership epoch 继续持有原 slot。

Verified onboarding 使用独立短期 work lease；进入 waiting / consent_required / repair_required / authorized 时释放，恢复执行时重新 acquire。

---

## Task 5：Provider Provisioning + Stable Cross-kind Fence

所有 durable Provider Writes：

```text
Create Organization
Create User
ownership marker write/repair
Reclaim
Delete User/Organization
```

固定流程：

```text
insert/adopt Provider Operation by logical key
-> same fingerprint replay / changed fingerprint conflict
-> acquire stable target fence(target_type,target_id)
-> acquire operation lease/epoch
-> mark inflight before Provider call
-> bounded Provider call
-> definite response => read-back + terminal
-> timeout/connection loss => outcome_unknown
-> Reconciler establishes finality
-> only after definitive outcome release/advance target fence
```

同 Provider target 的旧 Delete / 新 Reclaim，即使 registration_id 和 ownership_epoch 已改变，仍共享同一 fence。

多 target operation 按 stable `(target_type,target_id)` 顺序 acquire fence，禁止循环死锁。

Stale `inflight` lease expiry 只允许进入 `outcome_unknown`，不允许 blind retry external mutation。

---

## Task 6：Login V2 OTP / Factor Flow

OTP / Factor / Session 继续属于 ZITADEL/Login V2。

### 6.1 Genuinely-new / Registration-owned Pending

只有这两类用户允许 Registration Provisioning 执行 `AddOTPSMS`：

```text
ensure AddOTPSMS factor definitive
-> proof_attempt(challenge_preparing, no session ref)
-> pinned CreateSMSChallenge
-> bind actual returned challenge-bearing Session ref/hash + generation
-> VerifySMS against exact returned Session
-> proof = verified
```

AddOTPSMS unknown outcome：只有 factor read-back 或已证明 same-user idempotent contract 才自动收敛；否则 repair_required，禁止盲重试。

Challenge response loss：计 SMS debt/budget，进入 outcome_unknown；无 credential recovery 时 cooldown 后由用户显式创建新 generation。

### 6.2 Existing Active User

绝不由 `/register` Add/Update/Reactivate factor。

在 Proof 前，浏览器结果必须 branch-neutral：

```text
“如果该号码可用于验证，请查看短信；也可以选择其他登录/找回方式。”
```

“其他方式登录/找回”对所有号码统一展示。

服务器内部可不同：

- existing + usable SMS factor：按 ZITADEL 当前 factor policy；
- existing + no usable factor：不发 SMS、不改 factor；等待用户主动走统一展示的官方 login/recovery；
- new/pending：按注册 challenge 流程。

Proof 前不返回 `existing_user / no_factor / recovery_required` 等状态。

Challenge/resend 保留 per-session/phone/IP limit、verification attempt limit、global SMS budget。

---

## Task 7：Proof Purpose + Agreement Acceptance

Proof 增加 immutable purpose：

```text
onboarding_consent
reclaim
```

不可跨 purpose 使用。

普通新注册的 `onboarding_consent` proof：用户在 OTP Verify Server Action 同时提交当前协议勾选；Verify 成功后才写 insert-only Acceptance：

```text
proof_attempt_id PK
registration_id
provider_user_id
purpose
policy_version
accepted=true
accepted_at=server time
```

same proof changed immutable payload -> `ACCEPTANCE_REPLAY_CONFLICT`。

---

## Task 8：Current Policy Authority

`saas_policy_releases` 是 active policy version/epoch 的 DB 单一权威。

Phase1 protected tenant API 每次授权：

```text
VerifiedIdentity
-> live effective org
-> authoritative DB read current policy version/epoch
-> exact current Consent
-> business handler
```

DB 不可读 fail closed。UI cache 不参与授权 allow。

---

## Task 9：Consent-before-Business Atomic Boundary

**第一次 non-disposable business ownership 必须与 Current Consent 同事务。**

执行前 acquire verified work lease。事务：

```text
lock intent
lock ownership fence; require onboarding_writable
lock DB active policy release
lock onboarding_consent proof + acceptance
require proof verified / unconsumed
require acceptance == exact active policy
consume proof + acceptance once
insert current saas_account_consents
write first durable Sumi business ownership
ownership fence -> preserved_business
state -> business_preparing
commit
```

若 policy 已变化：

```text
no consent write
no business artifact
no preserved_business
release work lease
state = consent_required
```

这之后才允许 projection / subscription / Project Grant 等后续业务写。

---

## Task 10：Subscription + Entitlement Readiness

`base_payg` 继续属于现有 `internal/listingsubscription` Catalog。

新增/复用：

```text
EnsureInitialPlanIfAbsent
EnsureCurrentPlanEntitlementsReady
```

Entitlement readiness：lock current subscription/version，按 canonical current plan 修 missing + stale plan-owned rows，保留 explicit override，read-back effective entitlements。已有 paid plan 永不被 base_payg 覆盖。

所有能为 registration-owned Organization 创建 durable ownership 的写路径都必须先检查 Business Ownership Fence；`cleanup_claimed` 时 fail closed。

---

## Task 11：Business Prepare / Project Authorization

Consent-before-business transaction 完成后：

```text
ensure Sumi user projection
ensure Sumi org projection
EnsureInitialPlanIfAbsent(base_payg)
EnsureCurrentPlanEntitlementsReady
read-back plan + entitlements
business_prepared
```

Project Grant：absent create；exact+ACTIVE adopt；different/inactive/revoked -> `PROJECT_GRANT_REPAIR_REQUIRED`。

User Authorization：absent create listingkit_admin；exact+ACTIVE adopt；different/inactive/revoked -> `AUTHORIZATION_REPAIR_REQUIRED`。

禁止调用会 Update/Reactivate 现有 Provider auth 对象的高层 Ensure helper。

进入 repair/waiting 状态先释放 verified work lease；修复后 Reconciler 再 acquire。

最终 authorized 后释放 verified work lease，并把 pending_object slot 转出 registration pending accounting。

---

## Task 12：Reclaim 两 Proof 模型

Phase1 对 abandoned pending identity 采用更清晰的两 Proof 流程。

### Proof A — Reclaim

```text
fresh OTP proof, purpose=reclaim
-> lock/consume Proof A once
-> create durable Provider Reclaim operation
-> stable target fence
-> Provider mutation + finality reconciliation
-> definitive success
-> local ownership_epoch++ / registration rebind
-> state = reclaim_consent_required
```

Proof A 只能用于 Reclaim，不能写 Consent。

### Proof B — Current Consent

用户重新获取 fresh OTP challenge：

```text
purpose=onboarding_consent
+ current policy acceptance
```

随后走 Task 9，Proof B + Acceptance 与 Current Consent + 第一笔 business ownership 原子提交。

Rare reclaim 多一次 OTP，换取明确的 single-use proof、不跨外部 Provider transaction 伪装原子性。

---

## Task 13：RegistrationReconciler

负责：

```text
verified waiting / capacity waiting
consent_required / reclaim_consent_required
business_preparing / business_prepared
project_grant_ready / authorizing
all repair_required states
stale verified work leases
stale prepared/inflight/outcome_unknown Provider operations
```

另外运行 Admission Reaper/lease recovery，处理 expired temporary admission/headroom lease；绝不自动回收 `pending_object`。

Reconciler 不能从 lease expiry 推断 Provider side effect 未发生，必须遵守 Provider finality contract。

---

## Task 14：Cleanup / Destructive Delete

Cleanup 只允许 disposable pre-business registration。

先 DB transaction：

```text
lock ownership fence
require onboarding_writable
recheck no durable business ownership
ownership fence -> cleanup_claimed(cleanup_epoch)
commit
```

所有 business ownership writer 看到 `cleanup_claimed` 必须 fail closed。

准备 Delete 时：

1. acquire stable Provider target fence；
2. 确认 ownership fence 仍是同 cleanup_epoch；
3. 确认无 non-definitive Provider operation / durable business artifact；
4. **实时读取 ZITADEL 并 exact revalidate ownership**：Provider User ID、Org ID/association、canonical login、ownership marker/resource owner、本地 immutable ownership evidence；
5. 任一 mismatch -> `PROVIDER_OWNERSHIP_REPAIR_REQUIRED`，不 Delete、不释放 provider slot；
6. exact match 后才执行 durable Delete + finality；
7. Provider absence definitive 后释放 `pending_object` slot。

外部 Delete 期间 ownership fence 持续阻止新业务 ownership。

---

## Task 15：Legacy Backfill / Credential Boundaries

Legacy readiness gate 默认关闭。Backfill existing ZITADEL membership/roles、subscription/entitlements、business projection、legacy consent status；不伪造 consent，不覆盖 paid plan。已有业务 tenant 直接分类为 `preserved_business`。

Credentials 分开：

```text
Login Credential
Registration Provisioning Credential
Sumi Onboarding / Project Authorization Credential
Cleanup Credential
```

Registration Provisioning Credential 不得修改任意 existing active user factor；所有 credential 最小权限、轮换 overlap、跨租户负向测试。

---

## Task 16：Acceptance Matrix

至少覆盖：

```text
last provider slot known/new/pending race -> lookup 前 branch-neutral headroom reservation
request crash after generic/headroom acquire -> expired lease reused
same logical attempt retry -> no duplicate lease
pending retry -> temporary headroom released; original pending_object retained once
old ownership Delete vs new ownership Reclaim -> stable target fence one inflight
provider Create/Delete timeout -> no opposite mutation before finality
existing user usable/no SMS factor -> same pre-proof UI/nextAction
existing user no factor -> no AddOTPSMS
policy changes after OTP before Business Prepare -> zero durable business ownership
Consent + first business ownership same transaction rollback
Proof A cannot write Consent
Proof B required after successful reclaim
reclaim repeated -> ownership epoch changes, provider slot not duplicated
admin changes ownership marker/login/org before Delete -> fail closed, no Delete
business writer vs cleanup_claimed -> business write rejected
paid plan race -> base_payg never overwrites paid, entitlement projection repaired
repair_required/work-lease crash -> lease recovered
```

---

## 完成定义

- ZITADEL 仍拥有 OTP/Factor/Password/Session/OIDC；
- `/register` 不通过 capacity、factor 状态或 fallback 暴露账号类别；
- Provider headroom、temporary admission、pending Provider object 各有明确且不泄漏的容量语义；
- 同 Provider target 的跨 ownership/kind mutation 严格串行；
- Current Consent 永远早于或原子伴随第一笔 durable business ownership；
- destructive Delete 前实时证明 Provider identity 仍属于该 Registration；
- 所有外部 unknown outcome 都有 finality owner，不靠猜测恢复。