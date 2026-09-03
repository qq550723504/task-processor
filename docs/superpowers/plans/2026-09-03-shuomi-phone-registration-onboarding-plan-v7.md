# 硕米手机号注册与 Onboarding Implementation Plan V7

**状态：冻结实施基线。** 本 V7 是 PR #283 唯一执行入口，不再为了“手机号存在性绝对不可枚举”追加 Admission Epoch / cross-epoch debt / branch-neutral capacity 机制。

## 0. 产品安全决策

硕米明确允许注册入口判断并提示手机号是否已经注册。

允许：

```text
手机号已注册 -> 明确提示“该手机号已注册，请直接登录”
手机号未注册 -> 进入手机号自助注册
```

因此 **Account Existence 不属于 Phase1 需要隐藏的安全信息**。以下内容不再是验收目标：

- existing/new 必须使用相同 public response；
- Provider 容量变化必须 branch-neutral；
- SafetyAdmissionEpoch；
- cross-epoch worst-case debt；
- 通过全局容量、长期时序、key rotation 等间接现象也绝对无法推断账号存在。

仍然必须保护：手机号本身的日志/存储安全、OTP/SMS 防滥用、账号资料、组织关系、认证因子细节、业务数据、跨租户权限与 Provider 写入一致性。

## 1. 职责边界

```text
ZITADEL / Login V2
  User / Phone / OTP / Factor / Password / Session / OIDC

Registration Provisioning
  exact phone lookup
  short-lived same-phone registration fence
  stable Provider IDs
  Provider Create/Get/Adopt + finality
  bounded cleanup / reclaim

Sumi Onboarding
  current Consent
  business ownership fence
  projections / base_payg / entitlements
  Project Grant / User Authorization
```

禁止本地 OTP/Password、长期 PhoneIdentity、Decoy User、Callback Runtime、Temporal 登录 Saga、plaintext ZITADEL Session Token。

## 2. 注册主流程

### 2.1 输入手机号

1. 服务端规范化为 E.164；
2. 执行 phone / IP / session rate limit；
3. 使用受控、脱敏日志的 exact E.164 lookup 查询 ZITADEL；
4. 根据结果明确分支。

### 2.2 已注册手机号

```text
lookup = existing
-> 返回 accountExists=true / 等价明确业务结果
-> UI 提示“该手机号已注册，请直接登录”
-> 提供短信验证码登录、密码登录、忘记密码入口
```

要求：

- `/register` 不得对 existing active User 调用 `AddOTPSMS`、Update Factor、Reactivate Factor；
- existing User 的 OTP/Password/Recovery 全部走 ZITADEL/Login V2 原有策略；
- existence 可以公开，但不得返回姓名、邮箱、Organization、角色、Store、Factor 列表等额外资料。

### 2.3 未注册手机号

```text
lookup = not_found
-> acquire short-lived same-phone registration fence
-> BeginRegistration
-> stable Provider Org/User IDs
-> Create/Get/Adopt Provider identity
-> AddOTPSMS（仅 registration-owned User）
-> CreateSMSChallenge
-> VerifySMS
-> current Consent
-> Business Bootstrap
-> Project Grant
-> User Authorization(listingkit_admin)
-> official Login V2 CreateCallback / Auth.js session
```

同一手机号并发注册只允许一个 active Registration Attempt；其他请求 adopt/resume 或返回处理中，不能创建第二套 Provider Org/User。

## 3. Same-phone Registration Fence 与隐私

Phase1 保留最小短期并发 fence：

```text
active_phone_correlator = HMAC(K_registration_attempt, domain || normalized_E164)
```

用途仅限：防止同一手机号并发创建多个 Registration Attempt。

规则：

- active-only UNIQUE；
- 不作为账号目录；
- 不用于判断 existing/new；existing/new 由 ZITADEL exact lookup 决定；
- attempt terminal 或 hard TTL 后 scrub；
- ordinary active attempt hard TTL <=72h。

### Key rotation

Phase1 **不要求零停机在线轮换**。

正常 rotation 流程：

1. gate 新 self-registration；
2. 等待/终止旧 active attempts 到安全边界；
3. 确认旧 correlator claim 无可执行 workflow；
4. 切换 primary key version；
5. reopen registration。

因此不再设计 rolling old/new primary overlap、cross-key dual lookup 或 Admission Epoch 联动。

### Phone retention

- raw phone 只在请求内存中规范化；
- 跨请求恢复只保存 versioned AEAD ciphertext；
- ordinary pending ciphertext <=24h；
- repair/quarantine 最多72h；
- terminal 后立即或短 replay window 后 scrub；
- 日志、HTTP error、trace、proxy access log 不得记录明文 E.164 query。

Provider Operation 的 fingerprint 若包含 phone-derived material，也必须带 key version / retention deadline；finality/replay 不再需要后按同样 <=72h 边界 scrub 或转成不含手机号的 terminal tombstone。

## 4. 容量与防滥用

删除 `SafetyAdmissionEpoch`、branch-neutral Provider headroom、cross-epoch debt。

Phase1 使用普通、真实的容量保护：

```text
phone lookup
   ├─ existing -> 登录，不占新 Provider object 容量
   └─ not_found -> 检查/预留真实 pending Provider object capacity
```

允许 existing/new 因真实资源需求产生不同容量结果，因为账号存在性已经由产品明确公开。

仍要求：

- global pending Provider object high-water；
- per-phone / per-IP / per-session 注册限流；
- SMS challenge/resend cooldown 与全局 SMS budget；
- Provider worker bounded concurrency；
- Provider request context deadline / client timeout / bounded retry；
- 达到安全水位时拒绝新的未注册手机号创建 Provider identity。

同 logical attempt replay 必须先识别已有 attempt，再决定是否需要新容量；不得因一次客户端 response loss 重复占用 Provider object slot。

## 5. Registration Intent

Provider 第一次 durable write 前必须持久化最小 Registration Intent：

```text
registration_id
provider_organization_id
provider_organization_name
provider_user_id
phone_ciphertext / key_version / retention_until
state
ownership_epoch
expires_at
cleanup_state
```

`provider_organization_name` 使用稳定 opaque name，不含手机号。

Browser cookie 只保存 opaque registration reference，不是唯一持久化权威。

## 6. Provider Operation / Finality

所有 durable Provider Write 必须使用稳定 logical operation identity：

```text
logical_operation_key UNIQUE
state = prepared | inflight | outcome_unknown | retry_wait | succeeded | failed_permanent
request_fingerprint
lease_owner / lease_until / epoch
next_attempt_at / attempts / total_deadline
```

覆盖：

- Create Organization；
- Create Human User；
- ownership marker write/repair；
- Reclaim；
- Delete；
- Create Project Grant；
- Create User Authorization。

规则：

- same logical key + same immutable payload -> replay/adopt；
- changed payload -> conflict；
- definite retryable 且 Provider 能证明无副作用 -> retry_wait；
- timeout/connection loss/ambiguous 5xx -> outcome_unknown，只做 read-back/finality；
- semantic permanent failure -> failed_permanent；
- 不因网络超时 blind retry Create/Delete/Grant/Auth。

Target mutation fence 继续按稳定 Provider target 隔离 conflicting mutations。

## 7. OTP / Proof

仅 registration-owned new/pending User 可以执行 `AddOTPSMS`。

固定顺序：

```text
AddOTPSMS definitive
-> CreateSMSChallenge
-> 持久化其实际返回的 challenge-bearing Session 非凭据引用
-> VerifySMS
-> proof verified
```

task-processor 不持久化 OTP code / plaintext Session Token。

VerifySMS response loss：只有 Provider 能安全 read-back/idempotent reconcile 才自动收敛，否则进入 `verification_outcome_unknown`，不生成 Consent、不 blind retry。

## 8. Consent

Acceptance 必须绑定本次 proof attempt，并在 OTP Proof 成功之后形成权威事实。

`saas_policy_releases` 是 current policy 单一 DB 权威。

Consent 写入前必须验证：

- exact ZITADEL User；
- exact Registration / proof attempt；
- current policy version；
- Acceptance 与 proof 同 attempt/user/policy；
- proof/Acceptance 一次性消费。

## 9. Consent + First Business Ownership Atomic Bootstrap

Business Ownership Fence：

```text
onboarding_writable | bootstrap_claimed | preserved_business | cleanup_claimed
```

只有 Bootstrap 可从 `onboarding_writable -> bootstrap_claimed`。

同一 PostgreSQL transaction：

```text
lock Intent / work lease / fence / current policy / proof / Acceptance
-> validate current Consent
-> consume proof + Acceptance
-> insert Consent
-> create first durable business ownership
-> fence = preserved_business
-> commit
```

Policy stale -> zero business write，进入 `consent_required`。

## 10. Subscription

`base_payg` 继续复用现有 `internal/listingsubscription`：

- `EnsureInitialPlanIfAbsent` 只在确实无 subscription 时赋 `base_payg`；
- 不覆盖 paid plan；
- existing paid plan 必须 canonical reconcile plan-owned entitlement projection；
- initial assignment 必须使用 DB conditional insert / locked authority，不能 check-then-unconditional-upsert。

## 11. Project Grant / User Authorization

`listingkit_admin` 始终是最后一项业务访问效果。

在准备 Project Grant / User Authorization **之前必须重新读取 current active policy，并确认 exact current Consent 仍有效**。如果 policy 在 Bootstrap 后发生变化：

```text
-> 不创建 Grant / Authorization
-> release short work lease
-> consent_required
```

Project Grant / User Authorization 规则：

- absent -> create；
- exact roles + ACTIVE -> adopt；
- different / inactive / revoked -> repair_required；
- 禁止 Update/Reactivate 管理员已经修改或撤销的授权；
- Create response loss 走同一 Provider finality protocol。

## 12. Authorized Terminal

Provider Grant + Authorization read-back 成功后，本地事务：

```text
authorizing -> authorized
release verified work lease
close Registration Attempt
scrub phone ciphertext when replay window no longer needs it
authorized_at = now
```

随后 Login V2 使用官方 OIDC CreateCallback 完成应用登录；task-processor 不自建 callback acknowledgement runtime。

## 13. Cleanup / Reclaim

Cleanup 只针对 disposable pre-business Registration。

自动 destructive Provider Delete 只有 pinned ZITADEL 能证明安全 ownership precondition / finality 时启用；否则 self-registration 生产 rollout 保持 gated/off，直到存在经过验证的 bounded cleanup strategy。

Reclaim 使用 fresh ownership proof；reclaim proof 与 onboarding-consent proof 分离，避免一个 proof 被两个业务动作重复消费。

## 14. SMS 与公共接口防滥用

允许账号存在性公开 **不等于取消安全控制**。

必须保留：

- phone/IP/session rate limit；
- challenge resend cooldown；
- verification attempt limit；
- global SMS budget / circuit breaker；
- generic provider unavailable/error，不暴露内部 token/credential/provider path；
- existing user 不允许通过 `/register` 增加或恢复认证因子。

## 15. Rollout Capability Gate

上线前必须实测 pinned ZITADEL 的：

- exact E.164 global lookup；
- caller-supplied Org/User IDs；
- opaque Organization Name；
- Human User Technical Email request shape；
- OTP factor enrollment/read-back；
- CreateSMSChallenge / VerifySMS；
- Project Grant / User Authorization exact read-back；
- durable Provider Write finality；
- destructive cleanup safety；
- credential negative permission matrix。

任一关键 Provider finality / destructive cleanup 条件无法证明时，只关闭 **new phone self-registration**；Login V2、existing user login、Console 其他业务不因此停止开发。

## 16. Acceptance Tests

至少覆盖：

```text
existing phone -> 明确 accountExists=true / 登录提示，不调用 AddOTPSMS
new phone -> 进入 Registration
same new E.164 concurrent requests -> one active Registration Attempt / one Provider identity
registration key rotation -> maintenance gate期间不接受新 attempt，切换后无重复 active attempt
rate limit / SMS cooldown / global SMS budget
Create Org/User response loss -> read-back/adopt，不 blind retry
same logical Provider op changed payload -> conflict
VerifySMS response loss -> 无证明则 outcome_unknown，不写 Consent
current policy changes before Bootstrap -> zero business write
current policy changes after Bootstrap but before Grant/Auth -> consent_required，不授予 listingkit_admin
paid plan race -> 不覆盖 paid plan，entitlements ready
Grant/Auth absent create，exact active adopt，different/revoked repair
unsafe automatic Delete capability -> self-registration feature remains off
phone ciphertext / phone-derived fingerprints 在 retention deadline 后被 scrub
```

## 17. 开发停止条件

本设计不再以“自动评审找不到任何 P1/P2”为开工条件。

阻塞实现的 finding 仅包括：

- 会导致跨租户/越权；
- 会重复创建 Provider identity / 重复授予业务权限；
- 会错误覆盖 paid plan；
- 会产生不可恢复的 Provider side effect；
- 会错误写 Consent；
- 会造成身份删除与业务资产不一致；
- 会使核心注册流程无法完成。

纯 Account Enumeration、branch-neutral capacity、cross-epoch side-channel、零停机 correlator key rotation 不再属于 Phase1 blocking requirement。

## 完成定义

ZITADEL 继续拥有认证核心；已注册手机号可明确提示并转登录；未注册手机号只产生一条可恢复 Registration；同手机号并发不重复创建 Provider identity；Provider unknown outcome 不靠猜测；Current Consent 与业务 ownership 顺序正确；`listingkit_admin` 最后授予；SMS/Provider 有实际防滥用与容量保护；没有 Admission Epoch / cross-epoch anti-enumeration 复杂度。