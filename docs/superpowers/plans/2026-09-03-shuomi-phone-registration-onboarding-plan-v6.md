# 硕米手机号注册与 Onboarding Implementation Plan V6

**目标：** 在不复制 ZITADEL IAM 能力的前提下，实现手机号自助注册所需的最小 Registration Provisioning + Sumi Onboarding。本 V6 是 PR #283 的唯一执行入口。

## 权威输入

执行前读取原始 design 与 review amendments 1–12。冲突时以本 V6 与 amendment-12 为准。旧 plan / v2 / v3 / v4 / v5 仅保留历史。

## 1. 边界

```text
ZITADEL / Login V2
  User / Phone / OTP / Factor / Password / Session / OIDC

Registration Provisioning
  branch-neutral admission/safety reservation
  short-lived active-phone concurrency fence
  stable Provider IDs
  durable Provider Operation + target fence + finality
  cleanup / reclaim

Sumi Onboarding
  current Consent
  business ownership fence
  projections / base_payg / entitlements
  Project Grant / User Authorization
```

禁止本地 OTP/Password、长期 PhoneIdentity、长期 Phone HMAC Alias、Decoy User、Callback Runtime、Temporal 登录 Saga、plaintext ZITADEL Session Token。

## 2. Rollout Capability Gate

必须在 pinned ZITADEL/Login V2 staging 实测：

- exact E.164 login-name lookup 与 instance-wide uniqueness；
- caller-supplied Organization/User ID；
- opaque Organization name；
- Human User full create shape，包括 PR #218 已验证的 opaque Technical Email；
- ownership marker read/write/read-back；
- OTP SMS factor enrollment/read-back；
- CreateSMSChallenge / VerifySMS happy path与 response-loss；
- Project Grant search/create/read-back；
- User Authorization search/create/read-back。

所有 durable Provider Write 都必须进入统一 finality matrix：

```text
Create Organization
Create Human User
Ownership Marker Write/Repair
Reclaim
Delete
Create Project Grant
Create User Authorization
```

每一种 write 至少证明 provider-side idempotency、operation status、safe exact read-back/conditional mutation，或经过 staging 验证的 bounded finality；否则对应 self-registration rollout gate 不通过。

自动 destructive Delete 还必须额外证明 atomic ownership precondition（conditional/versioned delete、compare-and-delete 或等价）。无法证明时 automatic Delete=disabled，且在没有另一套经过评审的 bounded safe cleanup strategy 前 self-registration 保持 gated/off。

VerifySMS response-loss 必须证明 verified-session/factor read-back 或安全 Verify idempotency；否则只允许 `verification_outcome_unknown`。

## 3. Schema / Privacy / Stable Keys

核心表包括：Registration Intent、active-phone attempt claim、fixed admission leases、branch-neutral Provider safety reservations、proof/acceptance、Provider operations、stable target fences、business ownership fences、Consent、policy release。

### 3.1 Phone data

Raw E.164 只在内存规范化。跨请求需要时仅保存：

```text
phone_ciphertext       // versioned AEAD
phone_key_version
phone_retention_until
```

短生命周期 active-phone correlator：

```text
active_phone_correlator = HMAC(K_registration_attempt, "active-phone:v1" || normalized_E164)
correlator_key_version
UNIQUE(active_phone_correlator) WHERE active_phone_correlator IS NOT NULL
```

它只用于 Registration concurrency/admission，最长 <=72h；不允许用于长期 account lookup。

### 3.2 Admission Attempt

```text
logical_attempt_key UNIQUE
request_fingerprint    // keyed HMAC(normalized E.164 + policy + flow version + behavior fields)
lease_owner / lease_until / epoch
active_phone_correlator
```

same key + same fingerprint replay；same key + changed fingerprint -> `REGISTRATION_ATTEMPT_REPLAY_CONFLICT`。同手机号不同 logical key 必须 adopt/resume 同一个 active attempt，不得创建第二份容量或执行第二次 lookup。

### 3.3 Branch-neutral Provider Safety Reservation

```text
reservation_slot_id
logical_attempt_key
state = held | terminal_shadow | release_pending | released
admitted_at
not_before_release_at
provider_identity_id nullable
terminal_proof nullable
```

`not_before_release_at` 由版本化 rollout policy 在 admission 时固定，Phase1 建议统一 72h。existing / pending / new 不能根据 lookup branch 提前释放。

### 3.4 Provider Operation / Target Fence

```text
logical_operation_key UNIQUE
state = prepared | inflight | outcome_unknown | retry_wait | succeeded | failed_permanent
request_fingerprint
next_attempt_at / attempts / retry_after / total_deadline
```

Target fence：`UNIQUE(provider_target_type, provider_target_id)`，不依赖 registration/ownership epoch。Project Grant / User Authorization Create 也必须使用 Provider Operation + stable target fence。

### 3.5 Business Ownership Fence

```text
onboarding_writable | bootstrap_claimed | preserved_business | cleanup_claimed
```

## 4. `/register` Branch-neutral Admission + Same-phone Serialization

任何 ZITADEL E.164 lookup 之前，同一个 PostgreSQL transaction：

1. derive active-phone correlator；
2. lock/adopt existing active-phone claim；若存在则内部 resume 同 attempt，不新建第二份容量；
3. create/adopt logical attempt，校验 fingerprint；
4. acquire/reclaim one generic admission execution lease；
5. acquire one branch-neutral Provider safety reservation，固定 `not_before_release_at`；
6. commit。

任一公共 capacity 不可得：不 lookup、不发 SMS、只返回 generic unavailable。

Lookup/classification 后：

- generic execution lease 可以按相同 request-stage 规则释放；
- Provider safety reservation **不允许**因 existing/pending/new 分类而早退；
- genuinely-new 若创建 pre-auth Provider identity，只把 `provider_identity_id` 绑定到已有 reservation，不改变公共占用；
- existing/pending 保持 shadow hold。

公共 UI/nextAction 在 proof 前始终 generic；不能返回 existing/pending/no_factor 等内部分类。

## 5. Provider Operation / Retry / Finality

所有 durable Provider write 固定流程：

```text
persist/adopt logical op
-> acquire stable target fence(s)
-> exact pre-read when applicable
-> inflight
-> bounded Provider call
-> succeeded / failed_permanent / retry_wait / outcome_unknown
```

- definite retryable 且 Provider 合同证明 **no side effect** -> `retry_wait`；
- timeout/connection loss/side-effect ambiguous 5xx -> `outcome_unknown`，只 finality reconcile；
- permanent semantic error -> `failed_permanent`；
- exact active existing Grant/Authorization -> adopt succeeded；different/inactive/revoked -> repair_required，不 Update/Reactivate。

### 5.1 Side-effect-free permanent Create failure

如果 Create Org/User/Grant/Auth 的 finality 已证明 no object/no side effect：

```text
operation -> failed_permanent
intent -> terminal_no_provider_object / repair_terminal as applicable
provider safety reservation -> release_pending(not_before_release_at unchanged)
```

Reaper 到统一 release time 后幂等释放。若存在任何 partial Provider side effect，则禁止按 no-side-effect 路径释放，继续 finality/cleanup/repair。

## 6. OTP / Verify

只有 genuinely-new / registration-owned pending User 可以由 Registration Provisioning 调 AddOTPSMS。Existing active User 的 `/register` 绝不能 Add/Update/Reactivate factor。

固定顺序：

```text
factor enrollment definitive
-> proof_attempt=challenge_preparing
-> CreateSMSChallenge
-> persist actual challenge-bearing Session reference/generation
-> VerifySMS
```

Task-processor 不保存 plaintext Session Token，不验证 OTP code。

VerifySMS：

- definite success -> proof verified；
- response loss + proven safe read-back/idempotency -> converge；
- response loss without contract -> `verification_outcome_unknown`，不生成 proof/Acceptance/Consent，不 blind retry；
- 必要时 cooldown 后 fresh challenge generation，旧 generation 永远作废。

Pre-proof UI 对 existing/new/pending、有/无 SMS factor 一律 generic；other-login/recovery 入口统一展示。

## 7. Proof Purpose / Reclaim

Proof immutable purpose：`reclaim` 或 `onboarding_consent`，不可跨用。

Reclaim：

```text
fresh Proof A(purpose=reclaim)
-> consume once
-> durable Reclaim Provider Operation/finality
-> ownership epoch rebind
-> reclaim_consent_required
-> fresh Proof B(purpose=onboarding_consent) + current policy Acceptance
```

Proof A 不能被 Consent 再次消费。

## 8. Current Policy / Acceptance

`saas_policy_releases` 是数据库 active policy version/epoch 单一权威。Protected tenant API 每次授权直接读取 authoritative current policy，再校验 exact current Consent；DB 不可读 fail closed，UI cache 不参与 allow。

Acceptance insert-only，绑定 proof/user/registration/purpose/policy。Same proof changed payload -> `ACCEPTANCE_REPLAY_CONFLICT`。

## 9. Consent + First Business Ownership Atomic Bootstrap

只有 Bootstrap 可以 CAS：

```text
onboarding_writable -> bootstrap_claimed
```

同一个 PostgreSQL transaction：

1. lock Intent / verified work lease / ownership fence；
2. lock current policy；
3. lock verified onboarding-consent Proof + Acceptance；
4. require exact current policy；
5. consume Proof + Acceptance once；
6. insert current Consent；
7. create first durable business ownership；
8. ownership fence -> `preserved_business`；
9. state -> business_preparing；
10. commit。

Policy stale -> zero business ownership，release work lease，进入 `consent_required`。

其他 ownership creator（paid ApplyPlan、projection、Project Grant preparation、Store/resource/order）只能在 `preserved_business` 下写；`bootstrap_claimed/cleanup_claimed` 都拒绝。Legacy existing business tenant 可 backfill preserved_business，但不得伪造 Consent。

## 10. Subscription / Entitlement / Project Authorization

`base_payg` 留在现有 `internal/listingsubscription`：`EnsureInitialPlanIfAbsent` 不覆盖 paid plan；`EnsureCurrentPlanEntitlementsReady` 对 current plan 的 plan-owned projection 做 missing/stale canonical reconcile，并保留明确 override。

Project Grant / User Authorization：

```text
absent -> Create through Provider Operation/finality
exact roles + ACTIVE -> adopt succeeded
different/inactive/revoked -> repair_required
```

禁止调用会 Update/Reactivate 的高层 Ensure helper。Grant/Auth timeout/response-loss 必须进入 `outcome_unknown` 并 exact read-back；不能直接再 POST。

Waiting/repair_required 释放短期 verified work lease，恢复后再 acquire。

## 11. Authorized Terminal Transaction

Project Grant + User Authorization finality/read-back 均 succeeded 后，本地一个 PostgreSQL transaction：

```text
lock Intent + Provider safety reservation + verified work lease
authorizing -> authorized
unresolved Provider inventory -> account-owned / no longer pre-auth
Provider safety reservation -> terminal_shadow/release_pending
  // not_before_release_at 不变，不能按 branch/success 早退
verified work lease -> release
scrub phone ciphertext + active_phone_correlator
authorized_at = now
commit
```

Replay 不二次 release/scrub。Safety reservation 只在统一时间由 Reaper 释放。

## 12. Phone Retention

- raw phone 只在内存；
-跨请求只存 versioned AEAD ciphertext；
- active-phone correlator 是短期 HMAC concurrency fence，不是 identity index；
- authorized / definitive cleanup：立即 scrub phone ciphertext/correlator；
- ordinary pending：目标 <=24h；
- repair/quarantine：最多72h；
- retention 到期即 scrub，即使 workflow 未完全收敛；
- future reclaim 重新输入 phone + fresh ZITADEL proof；
- logs/errors/traces/query 禁止 raw phone。

## 13. Cleanup

Cleanup 只允许 disposable pre-business registration。先 CAS ownership fence `onboarding_writable -> cleanup_claimed(cleanup_epoch)`；所有 ownership creator 看到 cleanup_claimed 必须 fail closed。

自动 Provider Delete 只有 capability gate 已证明 atomic destructive ownership precondition 才启用。执行时 stable target fence + same cleanup_epoch + no business ownership + Provider conditional ownership precondition 同时成立；否则 repair_required，不 Delete。

Delete finality succeeded 后把 Provider safety reservation 标成 `release_pending`，但仍等统一 `not_before_release_at`；不按 cleanup branch 提前归还公共 headroom。

无 atomic delete contract 时自动 cleanup 不运行，self-registration 不进入生产 rollout。

## 14. Registration Reconciler / Safety Reaper

Reconciler 负责：

- waiting/consent/business/auth/repair states；
- stale attempt execution lease；
- Provider `prepared/inflight/outcome_unknown/retry_wait`；
- Grant/Auth finality；
- side-effect-free permanent Create terminalization。

Safety Reaper 负责 branch-neutral reservation：

```text
now < not_before_release_at -> never release
now >= not_before_release_at AND terminal proof safe -> release exactly once
now >= not_before_release_at AND unresolved Provider side effect -> keep held + open global self-registration circuit breaker
```

`retry_wait` 只重试 definite-no-side-effect transient；`outcome_unknown` 只做 finality read-back。

## 15. Credential / Legacy Boundaries

Login、Registration Provisioning、Onboarding/Project Authorization、Cleanup credential 分离并最小权限。Registration credential 不能改 arbitrary existing user factor。

Legacy backfill不伪造 Consent、不覆盖 paid plan；已有业务组织分类 `preserved_business`。

## 16. Acceptance

至少覆盖：

```text
same E.164 + two distinct logical keys concurrently -> one active attempt / one lookup execution
same E.164 retry -> adopt same attempt, no second safety reservation
same logical key changed phone/policy -> conflict
existing/new/pending classification -> same Provider safety occupancy until fixed release time
last-slot victim probe + controlled canary -> no branch-dependent capacity signal
Create Org/User permanent no-side-effect -> bounded release at uniform not-before time
partial Create side effect -> no unsafe release
Create Project Grant response loss -> outcome_unknown -> exact read-back/adopt
Create User Authorization response loss -> outcome_unknown -> exact read-back/adopt
VerifySMS response loss without safe contract -> verification_outcome_unknown
Proof A/B separation
Consent + first business ownership rollback together
non-bootstrap ownership before preserved_business -> denied
authorized before safety hold expiry -> terminal_shadow remains held until same time
safe cleanup before safety hold expiry -> release_pending but remains held until same time
hold expiry + unresolved Provider outcome -> self-registration circuit breaker opens / reservation not released
24h/72h phone data purge + active-phone correlator scrub
provider lacking atomic conditional Delete -> self-registration rollout blocked
```

## 完成定义

ZITADEL 仍拥有认证核心；同手机号只能有一个短期 active registration attempt；账号类别不通过 factor/status/capacity 变化暴露；所有 durable Provider Create（包括 Grant/Auth）都有 finality owner；Current Consent 与首笔业务 ownership 原子；destructive cleanup 只有 Provider 原子安全条件可证明时启用。