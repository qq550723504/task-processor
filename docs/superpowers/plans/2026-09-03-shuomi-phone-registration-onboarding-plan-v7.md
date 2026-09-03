# 硕米手机号注册与 Onboarding Implementation Plan V7

**状态：IMPLEMENTATION_READY / 冻结实施基线。** 本 V7 是 PR #283 唯一执行入口。旧 design、amendments 与旧 plan 仅保留历史背景；与本 V7 冲突时一律以本 V7 为准，不再创建 V8/V9 来处理非 Blocker finding。

## 0. 产品与安全决策

硕米明确允许注册入口判断并提示手机号是否已经注册：

```text
手机号已注册 -> 明确提示“该手机号已注册，请直接登录”
手机号未注册 -> 进入手机号自助注册
```

因此 **Account Existence 不属于 Phase1 需要隐藏的安全信息**。以下内容明确不是 Phase1 验收目标：

- existing/new 必须使用相同 public response；
- Provider 容量变化必须 branch-neutral；
- SafetyAdmissionEpoch；
- cross-epoch worst-case debt；
- 通过全局容量、长期时序、key rotation 等间接现象也绝对无法推断账号存在。

仍然必须保护：手机号日志/存储安全、OTP/SMS 防滥用、账号资料、组织关系、认证因子细节、业务数据、跨租户权限、Consent 正确性与 Provider 写入一致性。

## 1. 职责边界

```text
ZITADEL / Login V2
  User / Phone / OTP / Factor / Password / Session / OIDC

Registration Provisioning
  exact phone lookup + classification
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

## 2. 注册主流程与手机号分类

### 2.1 输入手机号

1. 服务端规范化为 E.164；
2. 执行 phone / IP / session rate limit；
3. 使用受控、脱敏日志的 exact E.164 lookup 查询 ZITADEL；
4. 对命中的 Provider User 做本地 ownership / business-state classification；
5. 进入 `existing_active | registration_owned_pending | not_found | repair_required` 之一。

### 2.2 Existing active User

```text
classification = existing_active
-> 返回 accountExists=true / 等价明确业务结果
-> UI 提示“该手机号已注册，请直接登录”
-> 提供短信验证码登录、密码登录、忘记密码入口
```

要求：

- `/register` 不得对 existing active User 调用 `AddOTPSMS`、Update Factor、Reactivate Factor；
- existing User 的 OTP/Password/Recovery 全部走 ZITADEL/Login V2 原有策略；
- existence 可以公开，但不得返回姓名、邮箱、Organization、角色、Store、Factor 列表等额外资料。

### 2.3 Registration-owned pending / reclaimable User

精确 lookup 命中 User 时，不能仅因为“User 已存在”就一律送普通登录。若满足以下条件，应分类为 `registration_owned_pending`：

- 本地 immutable registration ownership 能精确关联该 Provider User/Organization；
- 尚未到 `authorized`；
- 没有与当前 ownership 冲突的 durable business ownership；
- Provider read-back 的 User ID、Organization association、canonical login、opaque Organization name/ownership marker 等与当前 ownership 证据一致。

这类用户进入 **resume/reclaim**，而不是普通 Login V2：

```text
registration_owned_pending
-> adopt/resume current active attempt，或
-> fresh reclaim proof
-> fenced ownership epoch rebind
-> OTP / Consent / Bootstrap
```

如果 Provider 属性、管理员修改、ownership marker 或本地 ownership 证据不一致，进入 `repair_required`；不得把对象当作 registration-owned 去修改 Factor 或授予业务权限。

### 2.4 Genuinely new phone

```text
classification = not_found
-> acquire short-lived same-phone registration fence
-> reserve real pending Provider-object capacity
-> persist Registration Intent + stable Provider Org/User IDs
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

## 3. Same-phone Registration Fence 与手机号保留

Phase1 保留最小短期并发 fence：

```text
active_phone_correlator = HMAC(K_registration_attempt, domain || normalized_E164)
```

规则：

- active-only UNIQUE；
- 仅防同 E.164 并发 Registration Attempt，不作为账号目录；
- existing/new 由 ZITADEL exact lookup + ownership classification 决定；
- ordinary active attempt hard TTL <=72h；
- attempt terminal 或 hard TTL fenced 后 scrub。

### Key rotation

Phase1 **不要求零停机在线轮换**：

1. gate 新 self-registration；
2. 等待/终止旧 active attempts 到安全边界；
3. 确认旧 correlator claim 无可执行 workflow；
4. 切换 primary key version；
5. reopen registration。

不设计 rolling old/new primary overlap、cross-key dual lookup 或 Admission Epoch 联动。

### Phone retention

- raw phone 只在请求内存中规范化；
- 跨请求恢复只保存 versioned AEAD ciphertext；
- ordinary pending ciphertext <=24h；
- repair/quarantine 最多72h；
- terminal 后立即或短 replay window 后 scrub；
- 日志、HTTP error、trace、proxy access log 不得记录明文 E.164 query；
- phone-derived attempt / Provider Operation fingerprint 必须有 key version 与 retention deadline，finality/replay 不再需要后 scrub 或转为不含 phone-derived stable alias 的 tombstone。

## 4. 真实容量保护

删除 `SafetyAdmissionEpoch`、branch-neutral Provider headroom、cross-epoch debt。

```text
phone lookup + classification
   ├─ existing_active -> 登录，不占新 Provider object 容量
   ├─ registration_owned_pending -> 复用已有 pending-object accounting
   └─ not_found -> 原子预留真实 pending Provider-object capacity
```

允许不同分支产生不同真实容量结果，因为账号存在性已经由产品明确公开。

仍要求：

- global pending Provider object high-water；
- pending-object reservation 必须 DB 原子分配；
- per-phone / per-IP / per-session 注册限流；
- SMS challenge/resend cooldown 与全局 SMS budget；
- Provider worker bounded concurrency；
- Provider request context deadline / client timeout / bounded retry；
- 达到安全水位时拒绝新的未注册手机号创建 Provider identity。

同 logical attempt replay 必须先识别已有 attempt，再决定是否需要容量；response-loss retry 不得重复占 pending-object slot。

## 5. Registration Intent / Ownership

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
pending_object_slot_id
```

`provider_organization_name` 使用稳定 opaque name，不含手机号。Browser cookie 只保存 opaque registration reference，不是唯一持久化权威。

Provider Create response loss 后的 Get/Adopt 必须重新验证当前对象仍归本 Registration：至少 exact match Provider User ID、Organization ID/association、canonical login、opaque Organization name、ownership marker（若 Provider 能力存在）以及本地 immutable ownership evidence。任一 mismatch -> `PROVIDER_OWNERSHIP_REPAIR_REQUIRED`，不 adopt、不修改 Factor、不继续授权。

若 marker 写入与 Create 不能原子，只有 stable IDs + exact immutable attributes + 本地唯一 ownership 证据能证明对象就是本次 Create 的结果时，才允许 `adopt_missing_marker` 后补 marker；不能仅凭“ID 查到了”就 adopt。

## 6. Provider Operation / Finality 与唯一恢复责任方

所有 durable Provider Write 使用稳定 logical operation identity：

```text
logical_operation_key UNIQUE
state = prepared | inflight | outcome_unknown | retry_wait | succeeded | failed_permanent
request_fingerprint
lease_owner / lease_until / epoch
next_attempt_at / attempts / total_deadline
```

覆盖：Create Organization、Create Human User、ownership marker write/repair、Reclaim、Delete、Create Project Grant、Create User Authorization。

规则：

- same logical key + same immutable payload -> replay/adopt；
- changed payload -> conflict；
- definite retryable 且 Provider 能证明无副作用 -> retry_wait；
- timeout/connection loss/ambiguous 5xx -> outcome_unknown，只做 read-back/finality；
- semantic permanent failure -> failed_permanent；
- 不因网络超时 blind retry Create/Delete/Grant/Auth；
- target mutation fence 按稳定 Provider target 隔离 conflicting mutations。

### RegistrationReconciler

`RegistrationReconciler` 是所有 Registration-owned 非终态 Provider Operation 的**唯一 durable recovery owner**。不能依赖下一次浏览器请求才能恢复。

它周期扫描并可由写入事件唤醒：

- stale `prepared`；
- lease-expired `inflight`；
- `outcome_unknown`；
- `retry_wait` 且 `next_attempt_at <= now`。

Recovery 使用 `FOR UPDATE SKIP LOCKED`/等价行锁 + lease/epoch CAS：

- `prepared` 且能证明外部 call 尚未发送 -> 在同 logical op 下继续；
- expired `inflight` -> fenced 转 `outcome_unknown`，先 read-back，禁止直接重发；
- `outcome_unknown` -> 只依据 Provider finality/read-back 收敛；
- `retry_wait` -> 到期后重新取得 lease，但发送前重新检查所有业务前置条件；
- 旧 worker 的 late response 若 epoch 已变化，不得写本地状态。

每个非终态最终必须进入 succeeded / failed_permanent / repair_required / 等待用户重新证明等明确状态；不能永久占用 pending capacity。

## 7. OTP / Factor Enrollment / Proof

仅 registration-owned new/pending User 可以执行 `AddOTPSMS`。

固定顺序：

```text
AddOTPSMS factor definitive
-> CreateSMSChallenge
-> 持久化其实际返回的 challenge-bearing Session 非凭据引用
-> VerifySMS
-> proof verified
```

task-processor 不持久化 OTP code / plaintext Session Token。

### AddOTPSMS response loss

`AddOTPSMS` 是 User Factor durable mutation，不能因为换 Session 就假定回滚。执行前记录 attempt-bound factor enrollment operation/state；response loss 后：

- Provider 支持 user-factor read-back -> read-back 确认已安装后继续；
- 或 pinned Provider 已证明 same-user enrollment 幂等 -> 在该合同下安全 replay；
- 两者都无法证明 -> `factor_enrollment_outcome_unknown / repair_required`，禁止 blind retry。

该非终态同样由 `RegistrationReconciler` 接管；existing active User 永远不进入此写路径。

### VerifySMS response loss

只有 Provider 能安全 read-back/idempotent reconcile 才自动收敛，否则进入 `verification_outcome_unknown`，不生成 proof/Acceptance/Consent、不 blind retry；必要时 cooldown 后新 challenge generation，旧 generation 不得变成 proof。

## 8. Consent

Acceptance 必须绑定本次 proof attempt，并在 OTP Proof 成功之后形成权威事实。`saas_policy_releases` 是 current policy 单一 DB 权威。

Consent 写入前必须验证：exact ZITADEL User、exact Registration/proof attempt、current policy version、Acceptance 与 proof 同 attempt/user/policy，并一次性消费 proof/Acceptance。

## 9. Consent + First Business Ownership Atomic Bootstrap

Business Ownership Fence：

```text
onboarding_writable | bootstrap_claimed | preserved_business | cleanup_claimed
```

只有 Bootstrap 可从 `onboarding_writable -> bootstrap_claimed`。

同一 PostgreSQL transaction：

```text
lock Intent / work lease / fence / current policy / proof / Acceptance
-> validate current policy acceptance
-> consume proof + Acceptance
-> insert Consent
-> create first durable business ownership
-> fence = preserved_business
-> commit
```

Policy stale -> zero business write，进入 `consent_required`。除 Bootstrap 外，paid ApplyPlan、projection、Project Grant preparation、Store/resource/order 等 ownership creator 只有 `preserved_business` 才可写。

## 10. Subscription

`base_payg` 继续复用现有 `internal/listingsubscription`：

- `EnsureInitialPlanIfAbsent` 只在确实无 subscription 时赋 `base_payg`；
- 不覆盖 paid plan；
- existing paid plan 必须 canonical reconcile plan-owned entitlement projection；
- initial assignment 使用 DB conditional insert / locked authority，不能 check-then-unconditional-upsert。

## 11. Project Grant / User Authorization

`listingkit_admin` 始终是最后一项业务访问效果。

Project Grant / User Authorization：absent -> create；exact roles + ACTIVE -> adopt；different/inactive/revoked -> repair_required；禁止 Update/Reactivate 管理员修改或撤销的授权；Create response loss 走 Provider finality protocol。

### 每一次外部 send/adopt 前重新验证 Consent

不是只在 Operation 第一次 prepare 时检查。**每一次实际 Create send、retry_wait 恢复 send、outcome_unknown 后准备 adopt，以及 User Authorization send 前**，都必须重新读取 DB current active policy，并确认 exact current Consent 仍有效。

若 policy 在 Bootstrap 后或 retry_wait 期间变化：

```text
-> 不发送/不 adopt Grant 或 Authorization
-> release short work lease
-> Registration -> consent_required
-> 原 operation 不得在没有 fresh current Consent 时再次变为 runnable
```

fresh Consent 完成后，Reconciler 可在同 target fence 下重新取得执行资格；`listingkit_admin` 绝不能凭旧 policy Consent 延迟授予。

## 12. Authorized Terminal / Pending Capacity Release

Provider Grant + Authorization exact read-back 成功后，本地终态必须是**一个 PostgreSQL transaction**：

```text
lock Intent
lock pending_object reservation
lock verified work lease
verify expected ownership/fence/version
-> authorizing -> authorized
-> release/transfer pending Provider-object slot out of registration pending accounting
-> release verified work lease
-> close Registration Attempt
-> scrub transient phone material when replay window no longer needs it
-> authorized_at = now
-> commit
```

pending-object release/transfer 与 `authorized` 不得拆开；crash/replay 不能泄漏或二次释放容量。成功账号继续存在于 ZITADEL，但不再计入“未完成注册 Provider object”高水位。

随后 Login V2 使用官方 OIDC CreateCallback 完成应用登录；task-processor 不自建 callback acknowledgement runtime。

## 13. Cleanup / Reclaim

Cleanup 只针对 disposable pre-business Registration。

### Cleanup 必须先取得 Business Ownership Fence

真正开始任何 destructive cleanup 前，同一 PostgreSQL transaction 必须：

```text
lock Registration Intent + Business Ownership Fence
require fence = onboarding_writable
CAS onboarding_writable -> cleanup_claimed(cleanup_epoch)
confirm no durable business ownership
commit claim
```

之后整个 Provider Delete/finality 期间保持 `cleanup_claimed`。Bootstrap/ApplyPlan/projection/Grant preparation/Store/resource/order 等 ownership writer 看到 `cleanup_claimed` 必须 fail closed；因此 Cleanup 与第一笔 business ownership 不可能交叉提交。

自动 destructive Provider Delete 还必须满足 pinned ZITADEL 已证明的 atomic/conditional ownership precondition 与 finality；否则 automatic Delete 关闭，new self-registration rollout 保持 gated/off，直到存在经过验证的 bounded cleanup strategy。

Reclaim 使用 fresh ownership proof；reclaim proof 与 onboarding-consent proof 分离。Reclaim 前同样 exact read-back Provider ownership，并与 Delete 共享 stable target fence；不允许 stale Delete 与新 Reclaim 并行 inflight。

## 14. SMS 与公共接口防滥用

允许账号存在性公开 **不等于取消安全控制**。必须保留 phone/IP/session rate limit、challenge resend cooldown、verification attempt limit、global SMS budget/circuit breaker、generic provider unavailable/error，以及 existing user 不能通过 `/register` 增加或恢复认证因子。

## 15. Rollout Capability Gate

上线前实测 pinned ZITADEL：

- exact E.164 global lookup 与 active/pending ownership classification 所需 read-back；
- caller-supplied Org/User IDs；
- opaque Organization Name；
- Human User Technical Email request shape；
- OTP factor enrollment **read-back 或 proven idempotency**；
- CreateSMSChallenge / VerifySMS 与 response-loss recovery；
- Project Grant / User Authorization exact read-back；
- durable Provider Write finality；
- Provider ownership attributes/marker read-back；
- destructive cleanup conditional ownership safety；
- credential negative permission matrix。

任一关键 Provider finality / Factor recovery / destructive cleanup 条件无法证明时，只关闭 **new phone self-registration**；Login V2、existing user login、Console 其他业务不因此停止开发。

## 16. Acceptance Tests

至少覆盖：

```text
existing active phone -> accountExists=true / 登录提示，不调用 AddOTPSMS
registration-owned pending phone -> resume/reclaim，不误送 ordinary login
provider ownership mismatch -> repair_required，不 adopt/不改 factor/不授权
new phone -> one Registration / one Provider identity
same new E.164 concurrent requests -> one active Registration Attempt
key rotation -> maintenance gate 后切换，无重复 active attempt
rate limit / SMS cooldown / global SMS budget
Create Org/User response loss -> exact ownership read-back/adopt，不 blind retry
AddOTPSMS response loss -> factor read-back/idempotency，否则 outcome_unknown/repair
same logical Provider op changed payload -> conflict
stale prepared/inflight/outcome_unknown/retry_wait -> RegistrationReconciler durable recovery
VerifySMS response loss -> 无证明则 outcome_unknown，不写 Consent
current policy changes before Bootstrap -> zero business write
current policy changes after Bootstrap before Grant/Auth -> consent_required
current policy changes while Grant/Auth retry_wait -> 下一次 send 前拦截，不授予 listingkit_admin
Cleanup vs Bootstrap race -> 只有 cleanup_claimed 或 preserved_business 一方成功
successful authorization -> authorized + pending-object release + work-lease release 同事务
paid plan race -> 不覆盖 paid plan，entitlements ready
Grant/Auth absent create，exact active adopt，different/revoked repair
unsafe automatic Delete capability -> self-registration remains off
phone ciphertext / phone-derived fingerprints retention 后 scrub
```

## 17. 开发停止条件

本设计不以“自动评审找不到任何 P1/P2”为开工条件。真正阻塞实现的是会导致：跨租户/越权、重复 Provider identity/授权、错误 Consent、paid plan 覆盖、不可恢复 Provider side effect、身份删除与业务资产不一致、pending capacity 永久泄漏导致核心注册不可用、或核心注册流程无法完成的问题。

纯 Account Enumeration、branch-neutral capacity、cross-epoch side-channel、零停机 correlator key rotation 不属于 Phase1 blocking requirement。

## 完成定义

ZITADEL 继续拥有认证核心；已注册 active 手机号可明确提示并转登录；registration-owned pending identity 可以安全 resume/reclaim；未注册手机号只产生一条可恢复 Registration；Provider/Factor unknown outcome 不靠猜测；Provider adopt 必须验证 ownership；所有非终态 Provider operation 有唯一 durable recovery owner；Current Consent 在每次延迟授权 send 前仍有效；Cleanup 与 Business Bootstrap 有互斥 fence；authorized 与 pending capacity release 同事务；`listingkit_admin` 最后授予；没有 Admission Epoch / cross-epoch anti-enumeration 复杂度。