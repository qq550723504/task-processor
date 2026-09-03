# 硕米手机号注册与 Onboarding Implementation Plan V3

**目标：** 在不复制 ZITADEL IAM 能力的前提下，实现手机号自助注册所需的最小 Registration Provisioning + Sumi Onboarding，并把截至 PR #283 最新评审的全部不变量收敛成唯一执行入口。

## 权威输入

执行本计划前必须读取原始 design 与 amendments 1-9。若历史文档与本 V3 冲突，以本 V3 和 `2026-09-03-shuomi-phone-registration-onboarding-review-amendments-9.md` 为准。

旧 `...onboarding-plan.md` 与 `...onboarding-plan-v2.md` 只保留历史，不再作为实施指令。

---

## 架构边界

```text
ZITADEL / Login V2
  User / Phone / OTP / Password / Factor / Session / OIDC

Registration Provisioning
  short-lived Registration Intent
  branch-neutral register admission
  stable Provider IDs
  Provider Create/Get/Adopt/Finality
  Provider target mutation fence
  capacity / cleanup / reclaim

Sumi Onboarding
  Consent
  business ownership fence
  business projections
  base_payg / current-plan entitlement readiness
  Project Grant
  User Authorization
```

禁止实现：本地 OTP/Password、PhoneIdentity 长期目录、Decoy User、自研 OIDC Callback Runtime、Temporal 登录 Saga、plaintext ZITADEL Session Token 持久化。

---

## Task 1：Provider Capability Gate

预发布必须实测 pinned / production-compatible ZITADEL 的完整能力合同：

```text
exact global E.164 login-name lookup
Organization caller-supplied ID + required opaque name
Human User caller-supplied ID + Technical Email request shape
Get Organization/User exact ID
User/Organization ownership marker read/write + read-back
AddOTPSMS user-factor enrollment
OTP-SMS factor read-back OR proven idempotent same-user AddOTPSMS contract
CreateSMSChallenge exact request + returned challenge-bearing Session contract
VerifySMS against returned challenge-bearing Session
Project Grant search/create/read-back/state
User Authorization search/create/read-back/state
```

以下**每一种 durable Provider Write** 都必须证明至少一种可靠 finality 机制：provider-side idempotency、operation status、conditional mutation，或 staging 验证过的 bounded visibility/finality window：

```text
Create Organization
Create Human User
ownership marker write/repair
Reclaim / ownership mutation
Delete User / Organization
```

Stable caller-supplied ID 只能防止创建第二个对象，不能证明 timeout 后的旧请求不会延迟提交。

所有 capability 都做最小权限与跨租户负向测试。任一设计依赖能力无法证明时，self-registration feature 保持关闭。

---

## Task 2：Registration Schema / Idempotency / Fences

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
saas_registration_provider_target_fences
saas_registration_ownership_fences
saas_account_consents
saas_onboarding_preparations
saas_policy_releases
```

Intent 在任何 durable Provider Write 前固定 stable IDs、opaque Organization name、ownership epoch、短生命周期 normalized phone、state/expiry。

Generic Admission：

```text
admission_id
logical_admission_key UNIQUE
request_fingerprint
state
expires_at
```

Provider Operation：

```text
logical_operation_key UNIQUE
operation_id                 // opaque instance id
registration_id
ownership_epoch
kind
provider_target_type
provider_target_id
request_fingerprint
state = prepared | inflight | outcome_unknown | succeeded | failed_definitive
owner / lease_until / epoch
```

同 logical key + same fingerprint 只 replay/adopt；same key + changed fingerprint -> `PROVIDER_OPERATION_REPLAY_CONFLICT`。

Provider Target Fence：

```text
UNIQUE(registration_id, ownership_epoch, provider_target_type, provider_target_id)
active_logical_operation_key
mutation_class
state
lease_owner / lease_until / epoch
```

它负责 create/marker/reclaim/delete 等**跨 kind**互斥；operation logical key 负责同 kind 逻辑幂等。

Business Ownership Fence：

```text
UNIQUE(provider_organization_id)
registration_id
state = onboarding_writable | cleanup_claimed | preserved_business
cleanup_epoch
```

它负责把 cleanup 决策与本地 durable business ownership writer 串行化。

---

## Task 3：`/register` Branch-neutral Atomic Admission

在任何手机号 lookup 前：

```text
trusted ingress abuse checks
-> check global Provider-object high-water hard stop
-> BEGIN TX
   lock generic registration admission capacity
   acquire ONE branch-neutral admission slot
   persist admission logical identity
-> COMMIT
-> allocation failed => generic unavailable, NO lookup, NO SMS
-> exact E.164 lookup
-> internal classify existing / pending / genuinely-new
-> release generic admission slot in the SAME branch-neutral manner for all three branches
```

关键点：

- generic admission 只保护 `/register` lookup/challenge 前的匿名并发，不代表 Provider Object ownership；
- existing active：不新增 Provider Object slot；
- pending registration-owned identity：继续使用它原有 unresolved Provider Object slot；不得把本次 generic admission 转成第二个 Provider slot；
- genuinely new：只在此分支申请新的 global Provider Object slot，再进入 Provider Create；
- global Provider-object high-water 若已满，必须在 lookup 前所有 `/register` 分支统一 fail closed；
- admission retry 使用 stable logical key，不二次计数。

独立 `/login` 不依赖 `/register` admission，也不通过 register fallback 暴露 existence。

---

## Task 4：Capacity Authorities

三类容量必须分开：

### 4.1 Global Provider Object High-water

统计 registration 产生、尚未安全删除或正式转出 pending accounting 的 Provider User/Organization。只有 definitive delete 或正式授权转出才释放/转出。

### 4.2 Anonymous Register Admission

只限制 `/register` 匿名并发/攻击面；lookup 后统一释放 generic admission，不携带 branch 语义。

### 4.3 Verified Onboarding Work Capacity

OTP proof 成功后身份进入 verified waiting，不再占 anonymous admission。

`verified_onboarding_active` 是**短期 worker lease**，不是身份长期容量所有权。进入第一次 non-disposable business write 前 acquire；进入以下状态必须事务内 release：

```text
authorized
consent_required
verified_waiting_capacity
PROJECT_GRANT_REPAIR_REQUIRED
AUTHORIZATION_REPAIR_REQUIRED
PROVIDER_REPAIR_REQUIRED
factor_enrollment_outcome_unknown / repair_required
任何明确 manual-repair / waiting 状态
```

修复条件满足后由 Reconciler 再 acquire lease。Lease 有 lease_until/epoch，死亡 worker 可回收。

---

## Task 5：Provider Provisioning + Cross-kind Finality Fence

Durable Provider Writes：

```text
Create Organization
Create User
ownership marker write/repair
Reclaim
Delete User/Organization
```

流程：

```text
insert/adopt Provider Operation by stable logical key
-> exact fingerprint replay / changed fingerprint conflict
-> acquire target-scoped mutation fence
-> acquire operation lease/epoch
-> mark inflight before provider call
-> bounded provider call
-> definite response => read-back + succeeded/failed_definitive
-> timeout/connection loss => outcome_unknown
-> Reconciler resolves finality
-> only after definitive outcome release/advance target fence
```

Delete 与 Reclaim、marker repair 与 Delete 等不同 kind 仍共享同一个 target fence，绝不能各自独立 inflight。

Stale recovery：

```text
prepared + no live lease + confirmed call not sent -> resume same logical op
inflight + live lease -> wait
inflight + expired lease -> fenced CAS outcome_unknown, NO resend
outcome_unknown -> finality/read-back only
```

Cleanup 对任何 non-definitive Provider operation fail closed。

---

## Task 6：Login V2 OTP Registration Attempt

OTP/Factor/Session 仍属于 ZITADEL/Login V2。

### 6.1 Genuinely new / registration-owned pending User

只有这两类用户可执行 Registration Provisioning 所需 `AddOTPSMS`：

```text
ensure AddOTPSMS factor definitive
-> create proof_attempt = challenge_preparing (no Session ref yet)
-> pinned CreateSMSChallenge
-> bind actual returned challenge-bearing Session reference/hash + generation
-> VerifySMS against that exact returned Session
-> proof verified
```

`AddOTPSMS` response loss：必须依赖 user-factor read-back 或已证明的 same-user idempotent contract；否则 `factor_enrollment_outcome_unknown / repair_required`，禁止 blind retry。

`CreateSMSChallenge` response loss：计入 SMS budget，进入 `challenge_outcome_unknown`；无 credential recovery 时只能 cooldown 后用户显式创建新 generation，旧 generation 永不作为 proof。

### 6.2 Existing active User

**禁止** `/register` 调 `AddOTPSMS`、Update/Reactivate User Factor。

- 已有可用 SMS factor：按 ZITADEL 当前 factor policy/官方 login challenge 继续；
- 没有 SMS factor：转官方 `/login` / account recovery / 当前 factor policy；Registration 不为其新增较弱 factor；
- Registration Provisioning Credential 无需且不应拥有修改任意既有用户 factor 的权限。

Challenge/resend 继续使用 per-session/phone/IP limit、verification attempt limit 和 global SMS budget。

---

## Task 7：Attempt-bound Agreement Acceptance

用户在 OTP Verify Server Action 同时提交当前协议勾选。

只有 Verify 成功后才写 insert-only Acceptance：

```text
proof_attempt_id PK
registration_id
provider_user_id
policy_version
accepted=true
accepted_at=server time
source
```

same attempt changed payload -> `ACCEPTANCE_REPLAY_CONFLICT`。

Consent 事务锁 proof + acceptance，并要求 attempt/user/registration/current policy 完全一致后一次性 consume。

---

## Task 8：Current Policy Authority——Phase1 不用授权 Cache

`saas_policy_releases` 是 active policy version 的数据库单一权威。

Phase1 受保护业务 API 每次授权：

```text
verified identity
-> live effective organization
-> authoritative DB read active policy version/epoch
-> verify user consent for THAT exact version
-> business handler
```

DB 不可读 fail closed。UI/展示可 cache，但不能用于 authorization allow decision。

未来如需授权 cache，必须实现同步 invalidation/epoch，或 staged future activation fence；不是 Phase1 必需能力。

---

## Task 9：Subscription + Entitlement Readiness

`base_payg` 继续进入现有 `internal/listingsubscription` Catalog。

```text
EnsureInitialPlanIfAbsent
EnsureCurrentPlanEntitlementsReady
```

`EnsureCurrentPlanEntitlementsReady`：

```text
lock current subscription/version
-> load canonical current plan
-> reconcile all plan-owned entitlement rows
   missing -> insert
   stale canonical -> update
   exact -> no-op
-> preserve explicit override layer
-> retry on subscription/version race
-> read-back effective entitlements
```

legacy provenance 不明确先 classification；已有 paid plan 永不被 base_payg 覆盖。

**所有能为 registration-owned Organization 创建 durable business ownership 的 task-processor 写路径**，包括 initial/apply plan，必须先锁/校验 Business Ownership Fence，不得越过 `cleanup_claimed`。

---

## Task 10：Business Prepare + Ownership Fence

OTP proof verified 后进入 verified waiting。第一次 non-disposable write 前：

```text
BEGIN TX
  acquire verified work lease
  lock ownership fence
  require state = onboarding_writable
  state = business_preparing
  persist first durable onboarding/business ownership
  ownership fence -> preserved_business
COMMIT
```

之后再幂等执行：

```text
persist current Consent
ensure Sumi user/org projection
EnsureInitialPlanIfAbsent(base_payg)
EnsureCurrentPlanEntitlementsReady
read-back plan + entitlements
business_prepared
```

若 work capacity 满：保持 verified waiting，不写 non-disposable business artifact。

任一 paid subscription、Store/resource/order、Project Grant 等外部/并发 durable ownership 已存在，Reconciler 必须把 ownership fence 归类为 `preserved_business`，自动 Provider Delete 永久禁止。

---

## Task 11：Non-mutating Project Grant + User Authorization

Project Grant：

```text
absent -> Create exact allowed role set
exact roles + ACTIVE -> adopt
roles different/inactive/revoked -> PROJECT_GRANT_REPAIR_REQUIRED
```

User Authorization：

```text
absent -> Create listingkit_admin
exact roles + ACTIVE -> adopt
roles different/inactive/revoked -> AUTHORIZATION_REPAIR_REQUIRED
```

Registration 不调用会 Update/Reactivate 现有对象的高层 Ensure helper。

进入任何 repair_required 前先释放 verified work lease；修复后再由 Reconciler acquire。

---

## Task 12：RegistrationReconciler

负责业务状态：

```text
otp_verified
verified_waiting
verified_waiting_capacity
consent_required
business_preparing
business_prepared
project_grant_ready
authorizing
all *_repair_required / waiting states (for lease release + safe resumption)
```

负责 Provider Operation：

```text
stale prepared
stale inflight
outcome_unknown
```

负责 stale verified work lease 回收。

Reconciler 永远不能用“lease 已过期”推断外部副作用未发生；Provider mutation 必须通过 finality contract 收敛。

---

## Task 13：Cleanup / Reclaim

Cleanup 只允许 disposable pre-business registration。

第一步必须在 DB 事务中：

```text
lock ownership fence
require onboarding_writable
recheck no durable business ownership
ownership fence -> cleanup_claimed with cleanup_epoch
commit
```

随后所有 task-processor business ownership writer 看到 `cleanup_claimed` 都必须 fail closed。

Provider Delete 前再次验证：

```text
ownership fence still cleanup_claimed + same epoch
no durable business artifact
no non-definitive Provider operation
target mutation fence available
Provider finality contract available
```

外部 Delete 期间 ownership fence 持续阻止 ApplyPlan/projection/Grant preparation 等新写入。

任一 non-disposable artifact 出现 -> `preserved_business`，禁止自动 Provider Delete。

Reclaim 使用新 attempt-bound OTP proof，并与 Delete/marker write 共享 target-scoped mutation fence。

Provider absence definitive 后才释放 Provider-object accounting。

---

## Task 14：Legacy Backfill

Readiness Gate 默认关闭。Backfill existing ZITADEL membership/roles、subscription/entitlements、business projections、legacy consent status；不伪造 consent、不覆盖 paid plan。

历史业务 tenant 直接分类为 `preserved_business`，不进入 registration cleanup。

---

## Task 15：Credential Boundaries

分开：

```text
Login Credential
Registration Provisioning Credential
Sumi Onboarding / Project Authorization Credential
Cleanup Credential
```

每个 credential 最小权限、current/previous rotation overlap、跨租户负向测试。

Registration Provisioning Credential 不拥有任意既有 active user factor mutation 权限。

---

## Task 16：故障、安全与并发验收

至少覆盖：

```text
policy activation while stale UI cache exists -> protected API reads current DB version
last generic admission slot known/unknown/pending concurrency
lookup 后 generic admission 三分支统一 release
pending repeated /register -> no duplicate provider-object accounting
global provider high-water full -> pre-lookup uniform register failure
same Provider logical key -> one operation
Delete vs Reclaim same target -> one target fence winner
marker repair vs Delete serialized
stale prepared/inflight/outcome_unknown recovery
Create Org/User delayed success finality
repair_required releases verified work lease
cleanup_claimed vs concurrent ApplyPlan/business projection
existing active user without SMS factor -> no AddOTPSMS
existing active user with SMS factor -> no factor mutation
new/pending AddOTPSMS response loss recovery/fail-closed
challenge-bearing Session binding
paid plan entitlement missing/stale projection repair
business-owned tenant consent timeout -> no Provider Delete
```

---

## 完成定义

- 没有复制 ZITADEL OTP/Password/Factor/Session/OIDC；
- `/register` admission 与 Provider Object accounting 不形成 branch oracle；
- 所有 durable Provider Write 都有可证明 finality；
- cross-kind Provider mutation 有 target-scoped fence；
- cleanup 与 business ownership creation 通过本地 ownership fence 串行；
- existing active user 的 factor 不被 `/register` 擅自修改；
- verified work capacity 在 repair/waiting 路径不会泄漏；
- Current Policy 的授权决策不会使用 stale cache。
