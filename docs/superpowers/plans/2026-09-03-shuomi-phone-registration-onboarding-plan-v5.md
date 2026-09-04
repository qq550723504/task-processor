# 硕米手机号注册与 Onboarding Implementation Plan V5

**目标：** 在不复制 ZITADEL IAM 能力的前提下，实现手机号自助注册所需的最小 Registration Provisioning + Sumi Onboarding。本 V5 是 PR #283 唯一执行入口。

## 权威输入

执行前读取原始 design 与 review amendments 1–11。冲突时以本 V5 与 amendment-11 为准。旧 plan / v2 / v3 / v4 仅保留历史。

## 1. 边界

```text
ZITADEL / Login V2
  User / Phone / OTP / Factor / Password / Session / OIDC

Registration Provisioning
  branch-neutral admission/headroom
  stable Provider IDs
  durable Provider Operation + target fence + finality
  cleanup / reclaim

Sumi Onboarding
  current Consent
  business ownership fence
  projections / base_payg / entitlements
  Project Grant / User Authorization
```

禁止本地 OTP/Password、长期 PhoneIdentity、Decoy User、Callback Runtime、Temporal 登录 Saga、plaintext ZITADEL Session Token。

## 2. Rollout Capability Gate

必须实测 exact E.164 lookup、caller-supplied Org/User ID、opaque Org name、Human User Technical Email、ownership marker、OTP factor/read-back、CreateSMSChallenge/VerifySMS、Project Grant/User Authorization。

所有 durable Provider Write（Create Org/User、marker write/repair、Reclaim、Delete）必须有 provider-side idempotency/operation-status/conditional mutation 或经过 staging 验证的 bounded finality。

**自动 destructive Delete 还必须额外证明 atomic ownership precondition**：conditional/versioned delete、compare-and-delete 或等价语义。若无法证明：自动 Provider Delete 关闭；由于 pre-auth Provider object 无法安全 bounded cleanup，self-registration rollout 保持 gated/off，直到另有经过评审的安全 cleanup strategy。

VerifySMS response-loss 也必须证明可 read-back verified Session/factor 或安全 verify idempotency；否则只能进入 `verification_outcome_unknown`。

## 3. Schema / Stable Keys

核心表包括 Intent、fixed admission slots、fixed Provider capacity slots、proof/acceptance、Provider operations、stable target fences、business ownership fences、Consent、policy release。

Intent 不保存明文手机号。跨请求需要时：

```text
phone_ciphertext       // AEAD
phone_key_version
phone_retention_until
```

Admission attempt：

```text
logical_attempt_key UNIQUE
request_fingerprint    // keyed HMAC(normalized E.164 + policy + flow version + behavior fields)
lease_owner / lease_until / epoch
```

Provider Operation：

```text
logical_operation_key UNIQUE
state = prepared | inflight | outcome_unknown | retry_wait | succeeded | failed_permanent
request_fingerprint
next_attempt_at / attempts / retry_after / total_deadline
```

Target fence：`UNIQUE(provider_target_type, provider_target_id)`，不依赖 registration/ownership epoch。

Business Ownership Fence：

```text
onboarding_writable | bootstrap_claimed | preserved_business | cleanup_claimed
```

## 4. `/register` Branch-neutral Admission

任何 lookup 前同一 PostgreSQL transaction：

1. acquire/reclaim one generic admission lease；
2. acquire/reclaim one temporary Provider-headroom lease；
3. bind both to `logical_attempt_key + request_fingerprint`；
4. commit。

same key+same fingerprint replay/adopt；changed fingerprint conflict；execution lease 保证并发 same-key 只有一个 lookup/SMS execution。

任一 capacity 不可得：不 lookup、不发 SMS、generic unavailable。

lookup 后统一释放 generic lease；existing/pending 释放 temporary headroom；genuinely-new 将 temporary headroom 原子转成 `pending_object(provider_identity_id)`。

Temporary lease 可按 `lease_until` 原子复用；`pending_object` 只有 definitive safe delete 或 authorized 转正式账号才释放。

## 5. Provider Operation / Retry / Finality

流程：persist/adopt logical op -> acquire stable target fence -> inflight -> bounded Provider call -> terminal/read-back 或 outcome_unknown。

- definite retryable且 Provider 明确证明**无 side effect** -> `retry_wait`，Reconciler 按 same logical op/fence 重试；
- timeout/connection loss/ambiguous 5xx -> `outcome_unknown`，只做 finality reconciliation；
- permanent semantic error -> `failed_permanent`。

跨 kind/ownership 的同 target mutation 共用 stable target fence。多 target 按稳定 key 排序取 fence。

## 6. OTP / Verify

New/registration-owned pending 才允许 Registration Provisioning AddOTPSMS；existing active user 不得由 `/register` 修改 factor。

Proof attempt 必须绑定 `CreateSMSChallenge` 实际返回的 challenge-bearing Session。task-processor 不保存 plaintext Session Token。

VerifySMS：

- definite success -> proof verified；
- response loss + proven read-back/idempotency -> safe convergence；
- response loss without contract -> `verification_outcome_unknown`，不写 proof/Acceptance/Consent，不 blind retry；必要时 cooldown 后 fresh challenge generation，旧 generation 作废。

Pre-proof UI 对 existing/new/pending、有/无 SMS factor 一律 generic；other-login/recovery 入口统一展示。

## 7. Proof Purpose / Reclaim

Proof immutable purpose：`reclaim` 或 `onboarding_consent`，不可跨用。

Reclaim 用 Proof A：一次 consume -> durable Reclaim operation/finality -> ownership epoch rebind -> `reclaim_consent_required`。

随后必须 fresh Proof B（`onboarding_consent`）+ current policy acceptance，才能进入业务 Bootstrap。

## 8. Current Policy / Acceptance

`saas_policy_releases` 是 DB active policy version/epoch 单一权威。Protected tenant API 每次授权直接读取 authoritative current policy，再校验 exact current Consent；DB 不可读 fail closed。

Acceptance insert-only，绑定 proof/user/registration/purpose/policy；same proof changed payload conflict。

## 9. Consent + First Business Ownership Atomic Bootstrap

只有 Bootstrap 可以：

```text
onboarding_writable -> bootstrap_claimed
```

同一个 PostgreSQL transaction：

1. lock Intent / work lease / ownership fence；
2. lock current policy；
3. lock verified onboarding-consent proof + Acceptance；
4. require exact current policy；
5. consume proof + Acceptance once；
6. insert current Consent；
7. create first durable business ownership；
8. ownership fence -> `preserved_business`；
9. state -> business_preparing；
10. commit。

Policy stale -> zero business ownership, release work lease, `consent_required`。

其他 ownership creator（paid ApplyPlan、projection、Project Grant prep、Store/resource/order）只能在 `preserved_business` 下写；`bootstrap_claimed/cleanup_claimed` 都拒绝。

Legacy existing business tenants backfill `preserved_business`，但不伪造 Consent。

## 10. Subscription / Entitlement / Authorization

`base_payg` 留在现有 `internal/listingsubscription`。`EnsureInitialPlanIfAbsent` 不覆盖 paid plan；`EnsureCurrentPlanEntitlementsReady` 修 missing + stale plan-owned projection并保留 override。

Project Grant / User Authorization：absent create，exact+ACTIVE adopt，different/inactive/revoked -> repair_required；禁止 mutating Ensure helper。

Waiting/repair_required 释放短期 verified work lease，恢复时再 acquire。

## 11. Authorized Terminal Transaction

Provider Project Grant/User Authorization read-back 成功后，本地一个 PostgreSQL transaction：

```text
lock Intent + pending_object slot + verified work lease
authorizing -> authorized
pending_object -> release / transfer out pending accounting
verified work lease -> release
scrub phone ciphertext/correlator
authorized_at = now
commit
```

Replay 不二次 release/scrub。

## 12. Phone Retention

Raw phone 仅内存规范化。跨请求只存 versioned AEAD ciphertext；operation fingerprint 使用 keyed HMAC，不建立长期 phone directory。

- authorized / definitive cleanup：立即 scrub；
- ordinary pending：目标 <=24h；
- repair/quarantine：最多72h；
- 到期即 scrub，即使 workflow 未完全收敛；
-独立 purge worker 扫描；
- future reclaim 重新输入 phone 并 fresh ZITADEL proof；
- logs/errors/traces/query 禁止 raw phone。

## 13. Cleanup

Cleanup 只允许 disposable pre-business registration。先 CAS ownership fence `onboarding_writable -> cleanup_claimed(cleanup_epoch)`；所有 ownership creator 看到 cleanup_claimed 必须 fail closed。

自动 Provider Delete **只有 capability gate 已证明 atomic destructive ownership precondition 才启用**。执行时 stable target fence + same cleanup_epoch + no business ownership + Provider conditional ownership precondition 必须同时成立；否则 `PROVIDER_OWNERSHIP_REPAIR_REQUIRED`，不 Delete、不释放 pending slot。

无 atomic delete contract 时自动 cleanup 不运行，self-registration 不进入生产 rollout。

## 14. Reconciler

负责 waiting/consent/repair/business/auth states、temporary lease recovery、Provider `prepared/inflight/outcome_unknown/retry_wait`。

`retry_wait` 只重试 definite-no-side-effect transient。`outcome_unknown` 只 finality read-back。

## 15. Credential / Legacy Boundaries

Login、Registration Provisioning、Onboarding/Project Auth、Cleanup credential 分离并最小权限；Registration credential 不能改 arbitrary existing user factor。

Legacy backfill不伪造 Consent、不覆盖 paid plan，已有业务组织分类 `preserved_business`。

## 16. Acceptance

至少覆盖：last-slot oracle、same attempt replay/conflict、temporary lease recovery、VerifySMS response loss、Provider retry_wait vs outcome_unknown、old Delete vs new Reclaim fence、Proof A/B separation、Consent+first business ownership rollback、non-bootstrap ownership denied、authorized capacity release/scrub exactly once、24h/72h phone purge、provider lacking atomic conditional delete causes self-registration rollout gate。

## 完成定义

ZITADEL 仍拥有认证核心；账号类别不通过 capacity/factor/status 暴露；Provider unknown outcome 不靠猜测恢复；Current Consent 与首笔业务 ownership 原子；destructive cleanup 只有在 Provider 原子安全条件可证明时启用。