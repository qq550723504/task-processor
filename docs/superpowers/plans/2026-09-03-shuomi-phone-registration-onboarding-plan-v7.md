# 硕米手机号注册与 Onboarding Implementation Plan V7

**目标：** 在不复制 ZITADEL IAM 能力的前提下，实现手机号自助注册所需的最小 Registration Provisioning + Sumi Onboarding。本 V7 是 PR #283 唯一执行入口。

## 权威输入

执行前读取原始 design 与 review amendments 1–13。冲突时以本 V7 与 amendment-13 为准。旧 plan / v2 / v3 / v4 / v5 / v6 仅保留历史。

## 1. 边界

```text
ZITADEL / Login V2
  User / Phone / OTP / Factor / Password / Session / OIDC

Registration Provisioning
  branch-neutral admission
  short-lived same-phone concurrency fence
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

所有 durable Provider Write（Create Org/User、marker write/repair、Reclaim、Delete、Create Project Grant、Create User Authorization）必须有 provider-side idempotency/operation-status/conditional mutation 或经过 staging 验证的 bounded finality。

自动 destructive Delete 还必须额外证明 Provider-side atomic ownership precondition；无法证明则自动 cleanup 关闭且 self-registration rollout 保持 gated/off。

VerifySMS response-loss 必须证明可 read-back verified Session/factor 或安全 verify idempotency；否则进入 `verification_outcome_unknown`，不生成 proof/Consent。

## 3. Registration Attempt / Privacy Keys

### 3.1 Same-phone Active Claim

`active_phone_correlator = HMAC(K_registration_attempt, domain || normalized E.164)` 只用于当前 Registration Attempt 并发唯一，不作为账号目录。

Active claim 必须数据库 active-only UNIQUE；同 E.164 不同 logical key 只能 adopt/resume 同一 active attempt。

正常 key rotation 使用 staged rotation：新 key 先全量分发但旧 key 继续 primary，等待旧 active claim 最长 TTL（72h）清零后才切 primary。紧急 rotation 必须先关闭 self-registration 并 fence/清理旧 active attempts，禁止有 old/new primary overlap。

### 3.2 Admission Attempt

```text
logical_attempt_key UNIQUE
attempt_payload_fingerprint
fingerprint_key_version
fingerprint_retention_until
lease_owner / lease_until / epoch
attempt_closed_at
```

fingerprint 是 keyed HMAC(normalized E.164 + policy + flow version + behavior fields)。active/replay window 内 same key+same fp replay，changed fp conflict。

terminal 后只保留短 replay window；到期 scrub fingerprint/key version，并 tombstone logical key。之后同 key统一返回 branch-neutral `REGISTRATION_ATTEMPT_CLOSED`，不得把 phone-derived HMAC 长期保留。

### 3.3 Phone Retention

Raw phone 仅内存；跨请求时只保存 versioned AEAD ciphertext。

- ordinary phone ciphertext <=24h；
- repair/quarantine ciphertext <=72h；
- active correlator 可随仍可执行 attempt 保留到 hard TTL，最多72h；
- 24h 到期先 scrub ciphertext，不得因此提前删除 active correlator；
- 72h 到期先把 attempt 原子推进 `expired_fenced`、撤销 execution lease、禁止后续 Provider write，再 scrub correlator/fingerprint。

旧 expired_fenced attempt 永不重新执行；未来用户重新输入手机号走 fresh attempt/reclaim。

## 4. Fixed Safety Admission Epoch

不再使用“某一 attempt 完成后立即返还公共 Provider slot”的可观察模型。

控制面预创建：

```text
SafetyAdmissionEpoch
  epoch_id
  started_at
  ends_at
  configured_attempt_budget
  consumed_attempts
  state=open|closed|gated
```

一个 epoch 内所有 accepted `/register` attempt 都消耗 1 个 quota，existing/pending/new/success/failure 一律**不返还本 epoch quota**。

Acquire 顺序：

1. lock current open epoch；
2. ensure consumed < configured budget；
3. acquire/create active-phone claim；
4. persist/adopt logical attempt；
5. `consumed_attempts++`（same logical attempt replay 不二次增加）；
6. commit；
7. 才允许 exact E.164 Provider lookup。

容量不足时所有分支都在 lookup 前 generic unavailable。

Epoch budget 必须以 worst-case sizing 固定：假设本 epoch 所有 accepted attempt 都 genuinely-new，并可能占用完整 bounded finality/cleanup window。单条 attempt 的 authorized/cleanup/finality 不能改变当前 epoch 的可用 quota。

Provider unresolved inventory / cleanup SLO 只决定 **下一 epoch** 是否开放及固定 budget；不得因单条 victim outcome 在当前 epoch 即时释放/增加公开容量。若安全水位失守，next epoch `gated`，人工/控制面恢复后才能开放。

## 5. Provider Operation / Finality

Provider Operation：

```text
logical_operation_key UNIQUE
state=prepared|inflight|outcome_unknown|retry_wait|succeeded|failed_permanent
request_fingerprint
next_attempt_at / attempts / retry_after / total_deadline
```

Target fence：`UNIQUE(provider_target_type, provider_target_id)`，跨 kind/ownership 共用。

- definite retryable + Provider 证明无 side effect -> `retry_wait`；
- timeout/connection loss/ambiguous 5xx -> `outcome_unknown`，只 read-back/finality；
- semantic permanent failure -> `failed_permanent`。

Create Org/User/Project Grant/User Authorization/marker/Reclaim/Delete 都遵循同一 finality 协议。

Create side-effect-free failed_permanent 可以结束 workflow，但不会“返还本 epoch quota”；只更新内部 provider inventory bookkeeping。Partial/unknown side effect 必须继续由 Reconciler 收敛。

## 6. OTP / Proof

New/registration-owned pending 才允许 AddOTPSMS；existing active user `/register` 不改 factor。

Proof attempt 必须绑定 CreateSMSChallenge 实际返回的 challenge-bearing Session。task-processor 不持久化 plaintext Session Token。

VerifySMS response loss：有 proven read-back/idempotency才收敛；否则 `verification_outcome_unknown`，不写 proof/Acceptance/Consent，不 blind retry。

Pre-proof UI 对 existing/new/pending、有/无 SMS factor 全部 generic；其他登录/找回入口统一展示。

## 7. Reclaim

Proof purpose 不可跨用。

Proof A `reclaim` -> Reclaim finality -> ownership epoch rebind -> `reclaim_consent_required`。

然后 fresh Proof B `onboarding_consent` + current policy Acceptance -> 才能业务 Bootstrap。

## 8. Current Consent

`saas_policy_releases` 是 DB active policy/epoch 单一权威；protected tenant API 每次读取当前权威版本并校验 exact Consent，DB 不可读 fail closed。

Acceptance insert-only，绑定 proof/user/registration/purpose/policy；same proof changed payload conflict。

## 9. Consent + First Business Ownership Atomic Bootstrap

Business Ownership Fence：

```text
onboarding_writable | bootstrap_claimed | preserved_business | cleanup_claimed
```

只有 Bootstrap 可 `onboarding_writable -> bootstrap_claimed`。

同一个 PostgreSQL transaction：lock Intent/work lease/fence/current policy/proof+Acceptance -> validate current policy -> consume proof/Acceptance -> insert Consent -> create first durable business ownership -> fence `preserved_business` -> business_preparing -> commit。

Policy stale -> zero business write、释放 work lease、进入 consent_required。

其他 paid ApplyPlan/projection/Project Grant prep/Store/resource/order 只能在 preserved_business 下写。

## 10. Subscription / Authorization

`base_payg` 继续属于现有 `internal/listingsubscription`；`EnsureInitialPlanIfAbsent` 不覆盖 paid plan；`EnsureCurrentPlanEntitlementsReady` canonical reconcile plan-owned projection。

Project Grant / User Authorization：absent create，exact+ACTIVE adopt，different/inactive/revoked repair_required；禁止 mutating Ensure helper。Create 本身必须走 Provider finality Operation。

## 11. Authorized Terminal

Project Grant + User Authorization read-back 成功后，本地一个 PostgreSQL transaction：

```text
authorizing -> authorized
release verified work lease
scrub phone ciphertext
authorized_at=now
close active attempt when no further registration action is allowed
```

注意：authorized 不返还当前 Safety Admission Epoch quota；epoch quota 只按时间/控制面换代。

关闭 attempt 后保留短 terminal replay window，随后 scrub phone-derived fingerprint/correlator。

## 12. Cleanup

Cleanup 只允许 disposable pre-business registration；先 CAS ownership fence `onboarding_writable -> cleanup_claimed`。

自动 Provider Delete 只有 capability gate 已证明 atomic destructive ownership precondition 才启用；否则 self-registration 不进入生产 rollout。

Cleanup 成功不会返还当前 epoch quota，只更新内部 provider inventory；下一 epoch budget 由控制面基于 worst-case + inventory watermark 决定。

## 13. Registration / Safety Reconciler

负责：Provider prepared/inflight/outcome_unknown/retry_wait、proof/waiting/repair、active attempt hard TTL、fingerprint replay expiry、phone ciphertext purge、epoch closure/inventory health。

24h ciphertext purge与 active attempt closure分离；不得先删 correlator再让旧 attempt继续可执行。

## 14. Epoch Control Plane

开启下一 epoch 前必须：

- 读取真实 unresolved Provider inventory；
- 验证 cleanup/finality SLO；
- 保留 configured safety margin；
- 按 worst-case every-attempt-new 计算固定 budget；
- 一次性 publish epoch，不在 epoch 中途因单条 branch 修改 budget。

若任何 rollout capability（特别是 bounded safe destructive cleanup）不满足，发布 `gated` epoch而不是继续注册。

## 15. Credential / Legacy

Login、Registration Provisioning、Onboarding/Project Auth、Cleanup credential 分离并最小权限；Registration credential 不得改 arbitrary existing user factor。

Legacy business tenant backfill preserved_business，但不伪造 Consent、不覆盖 paid plan。

## 16. Acceptance

至少覆盖：

```text
same E.164 different logical keys -> one active attempt
staged key rotation with old active claim -> no duplicate attempt
emergency key rotation -> self-registration gated until old claims fenced
24h phone ciphertext purge -> active correlator remains
72h hard TTL -> attempt expired_fenced before correlator scrub
terminal replay window expires -> phone-derived fingerprint scrubbed
same epoch existing/pending/new all consume one non-refundable quota
victim success/failure/branch does not change current epoch remaining quota
side-effect-free permanent Create failure does not leak capacity within epoch
next epoch budget only control-plane publish
Provider inventory unhealthy -> next epoch gated
Project Grant/Auth Create response loss -> same Provider finality protocol
```

## 完成定义

ZITADEL 继续拥有认证核心；同手机号并发不会因 key rotation/retention 失效；phone-derived aliases 都有明确短 retention；公共注册容量按固定 epoch 而不是单条账号分支变化；Provider unknown outcome 不靠猜测；Current Consent 与首笔业务 ownership 原子；unsafe cleanup 时 self-registration fail closed。