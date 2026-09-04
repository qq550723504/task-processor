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

### Phase1 首次 Store 体验资源

PR #281 已确认产品闭环：新**直接注册**并完成首次业务开通的 Organization，一次性获得：

```text
store_renewal_period = 1
ai_point = 0
data_row = 0
```

该 1 期是独立企业资源 Grant，不属于 `base_payg.store_count`，也不是人民币钱包余额。绑定 Store 不消费它；用户显式 Activate 时才消费 1 期并启动首个 30 天服务周期。

同一新 Organization 最多获得一次，不因 HTTP retry、Registration resume/reclaim、Reconciler 重跑或套餐变化重复赠送。资源入账权威和 trusted Provisioning/source-bound 幂等合同由 PR #284 V7 定义。

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
  one-time welcome store renewal Grant readiness
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
-> 提供短信验证码登录、密码登录、忘记密码入口（对应 capability 可用时）
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

`not_found` 分支在**任何 Provider durable write 前**，必须先用一个 PostgreSQL transaction 原子建立本地所有权与真实容量归属：

```text
classification = not_found
-> BEGIN PostgreSQL
     lock/adopt same logical registration attempt
     acquire active-phone registration claim
     preallocate stable Provider Organization/User IDs + opaque Organization name
     persist Registration Intent + immutable ownership inputs
     reserve one real pending Provider-object capacity slot
     bind slot.registration_id = Intent.registration_id
     persist Intent.pending_object_slot_id = slot.id
   COMMIT
-> Create/Get/Adopt Provider identity
-> AddOTPSMS（仅 registration-owned User）
-> CreateSMSChallenge
-> VerifySMS
-> current Consent
-> Business Bootstrap
-> Ensure base_payg / current plan entitlements ready
-> Ensure one-time welcome store_renewal_period Grant ready
-> Project Grant
-> User Authorization(listingkit_admin)
-> official Login V2 CreateCallback / Auth.js session
```

事务不允许留下“只有 slot 没有 Intent”或“只有 Intent 没有 slot”的 durable 半状态：

- crash/rollback 在 COMMIT 前 -> Intent、active-phone claim、capacity reservation 全部不存在；
- COMMIT 后、Provider call 前 crash -> RegistrationReconciler 能通过 Intent + `pending_object_slot_id` 找到并继续该 attempt；
- same logical attempt retry -> adopt exact Intent + exact slot，不重新分配容量；
- same E.164 并发 -> active-phone UNIQUE/等价 DB fence 保证最多一个 active Intent/slot；
- changed immutable payload -> conflict/fail closed。

同一手机号并发注册只允许一个 active Registration Attempt；其他请求 adopt/resume 或返回处理中，不能创建第二套 Provider Org/User。

## 3. Same-phone Registration Fence 与手机号保留

Phase1 保留最小短期并发 fence：

```text
active_phone_correlator = HMAC(K_registration_attempt, domain || normalized_E164)
```

规则：

- active/finality-hold only UNIQUE；
- 仅防同 E.164 并发或 unresolved-finality Registration Attempt，不作为账号目录；
- existing/new 由 ZITADEL exact lookup + ownership classification 决定；
- ordinary **execution** hard TTL <=72h；
- attempt terminal 且不再存在 unresolved sent mutation 后才可关闭/scrub same-phone claim；hard TTL 本身只停止执行，不自动释放同手机号排他。

### Hard TTL：先原子 `expired_fenced`，再决定是否可以 scrub

“hard TTL fenced”不是逻辑描述，必须是数据库可验证的终态过渡。达到 72h hard TTL 时，Expiry/Reconciliation owner 先执行一个 PostgreSQL transaction：

```text
lock Registration Intent
lock active-phone claim
lock current execution/work lease
lock current Provider target/operation fence metadata
require Intent 仍为可执行且 expected execution_epoch/ownership_epoch 匹配
CAS Intent -> expired_fenced
execution_epoch = execution_epoch + 1
revoke/clear current local execution lease
mark future Provider writes prohibited for this Intent
cancel/fence 尚未发送的 prepared operations
commit
```

规则：

- 每个 Provider writer 在**实际 external send 前**必须重新读取并验证 Intent 不是 `expired_fenced`、expected execution epoch、work lease 与 stable target fence；不满足即不得发送；
- worker external call 返回后、本地写结果前再次验证 epoch；旧 worker late response 若 epoch 已变化不得推进 Registration；
- 已经 `inflight/outcome_unknown` 的 Provider mutation 不能假装被本地 fence 回滚，仍由 RegistrationReconciler 按 §6 做 finality；相反 mutation继续由 stable Provider target fence 阻止；
- `expired_fenced` COMMIT **只代表旧 attempt 失去继续发送 Provider write 的资格，不代表 same-phone correlator 已可释放**；
- 如果不存在任何已经发送且尚未 definitive 的 identity-affecting Provider mutation，才可按下述 terminal/finality 规则关闭 same-phone claim；否则必须进入 `finality_hold`。

### Finality Hold：执行过期不等于释放同手机号排他

只要本 Registration 仍存在任一已经发送、但结果尚未 definitive 的 Provider identity mutation（例如 Create Organization/User、ownership mutation、Reclaim/Delete 或其它可能改变该 registration-owned Provider identity 的 sent operation），same-phone claim 必须保持为非执行态 `finality_hold` / 等价状态：

```text
Intent = expired_fenced / non-runnable
active-phone claim = finality_hold
execution lease = none
Provider sent operation = inflight | outcome_unknown | 等待 definitive read-back
```

硬规则：

- `finality_hold` 保留同一 `active_phone_correlator` 的 UNIQUE 排他，但不授予旧 attempt 任何新的 Provider send 权限；
- 新的同手机号 `/register` 即使此刻 ZITADEL exact lookup 暂时返回 `not_found`，也必须先在 §2.4 admission transaction 中命中并锁定该 existing/finality-hold claim，绑定旧 Registration ownership，返回“处理中/需恢复/repair”或进入既有 resume/reclaim 流程；**不得**据此重新预分配另一组 Provider IDs、另一条 Intent 或第二个 pending-object slot；
- stable target fence 继续保护已经知道的 Provider target；same-phone `finality_hold` 额外保护“旧 Create 可能尚未可见，因此新 attempt 尚不知道旧 target”的窗口；
- Provider finality 最终证明旧 object 已存在且 ownership exact -> 后续同手机号只能按 §2.3 `registration_owned_pending`/resume/reclaim；
- Provider finality 最终证明没有任何 Provider object/partial side effect -> 可在与 no-object terminal/capacity release 相同或等价的 fenced transaction 中关闭 same-phone claim，随后新的请求才可成为 genuinely new；
- partial object、ownership mismatch 或 finality 仍 ambiguous -> 不释放 claim，进入 repair/finality；不能为了 privacy TTL 猜测“应该不存在”。

普通可执行 attempt 的 hard execution TTL 仍为 <=72h；但若 Provider sent mutation 在该时点仍 unresolved，**same-phone HMAC fence 可以作为 correctness-only exceptional tombstone 超过 72h 保留到 definitive finality**。它不包含 plaintext phone，也不能被业务查询使用。该异常必须有独立监控/SLO；若 Provider 无法在 rollout 证明 bounded finality，或线上 finality backlog 超过安全阈值，new-phone self-registration circuit breaker fail closed，而不是释放 fence 后冒险创建重复 identity。

只有所有 sent mutation 都 definitive，且 terminal/replay 不再需要同手机号排他时，才允许关闭/scrub correlator。这样 hard TTL 能停止旧 attempt 的未来执行，同时不会让“scrub phone claim”变成迟到 Create 制造重复 Provider identity 的窗口。

### Key rotation

Phase1 **不要求零停机在线轮换**：

1. gate 新 self-registration；
2. 等待/终止旧 active attempts 到安全边界；
3. 确认旧 correlator claim 无可执行 workflow，且 unresolved-finality claim 不会在 cutover 后失去 same-phone exclusion；
4. 切换 primary key version；
5. reopen registration。

不设计 rolling old/new primary overlap、cross-key dual lookup 或 Admission Epoch 联动。存在 `finality_hold` 的旧 key claim 时，不能通过直接切 key 让其排他失效；需要 maintenance migration/dual-check 或继续 gate，直到该 claim definitive 关闭。

### Phone retention

- raw phone 只在请求内存中规范化；
- 跨请求恢复只保存 versioned AEAD ciphertext；
- **24h 是 ordinary privacy target，不是可以破坏 recovery 的无条件删除点**；
- 若仍存在可自动执行/重试且其 exact request reconstruction 需要手机号的 phone-bearing operation（例如 Create Human User 的 `prepared/retry_wait`、需要手机号输入的 registration recovery，或当前 challenge generation 的恢复），AEAD ciphertext 必须保留到该 operation terminal、被显式 fenced，或 ordinary hard execution TTL（<=72h）中的更早者；
- 到 24h 时若该 attempt 仍需要 ciphertext 才能继续，Purge owner 不得直接删除；必须先保持 recovery，或在达到 hard TTL 时执行 `expired_fenced`，撤销所有未来 phone-bearing send 资格后再 scrub；
- `outcome_unknown` mutation 若后续只允许用 stable Provider IDs/read-back 做 finality、明确禁止 blind retry，则 raw phone/ciphertext 在不再需要重构请求后可以先 scrub，但 §3 `finality_hold` 的 HMAC exclusion 必须继续保留到 definitive finality；
- repair/quarantine 中只有仍需要合法恢复输入时才可持有 ciphertext，最长不超过 72h；超过后必须要求用户重新输入手机号并重新证明，不能延长 plaintext/AEAD 恢复秘密；
- terminal 后立即或短 replay window 后 scrub；
- 日志、HTTP error、trace、proxy access log 不得记录明文 E.164 query；
- phone-derived attempt / Provider Operation fingerprint 必须有 key version 与 retention deadline，finality/replay 不再需要后 scrub 或转为不含 phone-derived stable alias 的 tombstone；
- **禁止出现“operation 仍 runnable，但其唯一必需 request material 已被 purge”的状态**，该不变量必须由状态机/测试验证。

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
- pending-object reservation 必须 DB 原子分配，并按 §2.4 与 Intent/phone claim 同事务建立 durable owner；
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
execution_epoch
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
- target mutation fence 按稳定 Provider target 隔离 conflicting mutations；
- actual external send 前必须同时验证 Provider Operation lease/epoch、stable target fence 以及 Registration `execution_epoch/state`，`expired_fenced`/stale epoch 不得发送；
- 如果 sent operation 尚未 definitive，§3 same-phone claim 不能因为 hard execution TTL 到期而释放。

### Definitive no-object Create failure 的容量终态

`failed_permanent` 不能天然等价为“继续占用 pending-object 容量”。当且仅当 Provider finality 能证明：

```text
failed Create 对应目标不存在
AND 该 mutation 没有任何 side effect
AND 当前 Registration 也不存在其它仍需该 pending-object reservation 保护的 Provider Organization/User object
```

必须在一个 PostgreSQL transaction 中：

```text
lock Intent + Provider Operation + pending_object reservation + same-phone claim
-> operation = failed_permanent
-> intent = terminal_no_provider_object / 等价终态
-> release pending_object reservation
-> release 已取得的短 work lease（如有）
-> close/scrub 可终结的 transient registration material
-> close same-phone finality_hold/claim when no other sent mutation remains unresolved
-> commit
```

同 logical replay 只能读取该终态，不能二次 release。若 Organization 已创建而 User Create 永久失败、存在 partial object、或 finality 仍不确定，则**不得**走 no-object release；继续占用 reservation 与 same-phone finality exclusion，进入 cleanup/repair/finality 收敛。

### RegistrationReconciler

`RegistrationReconciler` 是所有 Registration-owned 非终态 Provider Operation 的**唯一 durable recovery owner**。不能依赖下一次浏览器请求才能恢复。

它周期扫描并可由写入事件唤醒：

- stale `prepared`；
- lease-expired `inflight`；
- `outcome_unknown`；
- `retry_wait` 且 `next_attempt_at <= now`；
- §7 的 stale `challenge_preparing / challenge_outcome_unknown` proof attempts。

Recovery 使用 `FOR UPDATE SKIP LOCKED`/等价行锁 + lease/epoch CAS：

- `prepared` 且能证明外部 call 尚未发送 -> 在同 logical op 下继续；
- expired `inflight` -> fenced 转 `outcome_unknown`，先 read-back，禁止直接重发；
- `outcome_unknown` -> 只依据 Provider finality/read-back 收敛；
- `retry_wait` -> 到期后重新取得 lease，但发送前重新检查所有业务前置条件与 phone request material 仍存在；
- `expired_fenced` Intent 不允许新 Provider send，只允许对已发出的 operation 做 finality/read-back；
- unresolved sent operation 继续持有 §3 `finality_hold` same-phone exclusion；
- 旧 worker 的 late response 若 Provider Operation epoch、Registration execution epoch 或 challenge generation 已变化，不得写本地状态。

每个非终态最终必须进入 succeeded / failed_permanent / repair_required / 等待用户重新证明等明确状态；不能永久占用 pending capacity，也不能在 finality 未完成时释放 same-phone exclusion。

## 7. OTP / Factor Enrollment / Proof

仅 registration-owned new/pending User 可以执行 `AddOTPSMS`。

固定顺序：

```text
AddOTPSMS factor definitive
-> persist proof_attempt challenge_preparing generation
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

### CreateSMSChallenge：发送前必须有 durable generation

`CreateSMSChallenge` 虽然属于 ZITADEL/Login 的短期认证边界，不强行塞入 §6 的 durable business Provider Operation 表，但它必须拥有自己的 durable、attempt-bound challenge recovery state。**禁止先发 SMS，成功返回后才第一次记录 generation。**

建议最小状态：

```text
proof_attempt_id
challenge_generation
challenge_state = challenge_preparing | challenge_ready | challenge_outcome_unknown | challenge_retry_allowed | challenge_superseded
challenge_execution_epoch
challenge_started_at
challenge_ready_at
returned_session_ref/hash   # 仅非凭据引用；绝不保存 plaintext Session Token
```

固定协议：

1. `AddOTPSMS` 已 definitive 后，先在 PostgreSQL transaction 创建/锁定当前 `proof_attempt_id`，递增或分配 `challenge_generation`，写 `challenge_preparing` + expected execution epoch，再 COMMIT；
2. 真正调用 `CreateSMSChallenge` 前重新验证 Registration 未 `expired_fenced`、当前 generation/epoch、SMS cooldown/rate limit/global budget；同一 generation 只能有一个 send owner；
3. Provider 正常返回时，在本地 transaction 再次验证 generation/epoch 仍 current，随后写入它**实际返回**的 challenge-bearing Session 非秘密 ref/hash，并转 `challenge_ready`；
4. timeout/connection loss/ambiguous response 后，写 `challenge_outcome_unknown`；**不能立即 blind resend 同 generation，也不能假定“没收到 response = 没发短信”**；
5. 若 pinned ZITADEL 能通过安全 read-back、provider-side idempotency/operation status 找回该 exact challenge/session，则 Reconciler adopt 并转 `challenge_ready`；
6. 若无法 read-back/adopt，则等待明确 cooldown。Reconciler 只把旧 generation 原子标为 `challenge_superseded / challenge_retry_allowed` 并 fence 其 execution epoch；**不得后台自动再发一条 SMS**。用户显式 resend/continue 且通过 limiter 后，才可创建 `generation+1`，并再次先持久化 `challenge_preparing` 后发送；
7. 旧 generation 的迟到 response 在写本地前必须检查 generation/epoch；一旦已 superseded，不能覆盖新 Session ref，也不能重新变成 `challenge_ready`；
8. `VerifySMS` 只接受当前 `challenge_ready` generation 绑定的 returned Session；旧/superseded generation 永远不能生成 Proof/Acceptance。

因此 crash-before-send、send-response-loss、late-response、用户 resend 都有明确状态。SMS abuse limiter 继续位于 Challenge send 前，recovery 不得绕过 cooldown/global budget。

### VerifySMS response loss

只有 Provider 能安全 read-back/idempotent reconcile 才自动收敛，否则进入 `verification_outcome_unknown`，不生成 proof/Acceptance/Consent、不 blind retry；必要时 cooldown 后新 challenge generation，旧 generation 不得变成 proof。fresh generation 同样必须走上述 pre-send durable protocol。

## 8. Consent

Acceptance 必须绑定本次 proof attempt，并在 OTP Proof 成功之后形成权威事实。`saas_policy_releases` 是 current policy 单一 DB 权威。

Consent 写入前必须验证：exact ZITADEL User、exact Registration/proof attempt、current policy version、Acceptance 与 proof 同 attempt/user/policy，并一次性消费 proof/Acceptance。

### 8.1 Bootstrap 前 Consent

初次注册在第一笔 durable business ownership 前，Consent 只能通过 §9 的原子 Bootstrap 事务建立；不能由浏览器直接指定 user/org/policy/version/accepted_at。

### 8.2 Bootstrap 后、Authorization 前的 re-consent

若 policy 在 `preserved_business` 已成立后、Project Grant/User Authorization 发送前发生变化，**不得再次执行 Bootstrap，也不得复用已消费的原 proof/Acceptance**。

Registration 进入 `consent_required`，由 Login V2 对同一 Provider User 发起新的 ownership proof：

```text
fresh proof_attempt_id
purpose = registration_reconsent
fresh ZITADEL OTP/session proof
+ current policy acceptance
```

Verify 成功后，服务器形成新的 attempt-bound Acceptance；task-processor 的 `RecordRegistrationReconsent` / 等价窄命令在一个 PostgreSQL transaction 中：

```text
lock Registration + current policy + fresh proof + fresh Acceptance
verify exact Provider User / registration ownership
verify policy still current
consume fresh proof + Acceptance exactly once
insert saas_account_consents(user_id, current_policy_version)
transition consent_required -> business_prepared / authorization_waiting 等不重写 business ownership 的可继续状态
commit
```

该路径**只新增 current Consent**：不重跑 `base_payg`、不重新创建 business projection、不重复 welcome Grant。随后 RegistrationReconciler 重新取得短 work lease，重新验证 subscription/entitlements/welcome resource readiness，并继续 §11。

若用户放弃 re-consent，由该 post-business Registration/Consent recovery owner 保持可恢复状态；因为 `preserved_business` 已存在，绝不再走 destructive pre-business identity cleanup。

### 8.3 Authorized 用户的 Current Consent

Registration 到 `authorized` 后，ZITADEL `listingkit_admin` 是长期业务角色，但**不能绕过当前协议版本**。所有受保护 tenant business request 必须在 VerifiedIdentity + live effective Organization 之后、业务 Handler 之前执行 `CurrentConsentPolicy`：

```text
read DB current active policy version/epoch
-> require exact Consent(zitadel_user_id, current_policy_version)
-> missing/stale => CONSENT_REQUIRED / 受控 re-consent response
-> DB authority unavailable => fail closed
```

登录/退出、re-consent、必要的 consent/status/recovery 路由显式豁免，避免死锁；其它仍可达的 legacy/new tenant business API 都必须受同一 gate 约束，而不是只拦 Workbench 页面。

已授权用户完成 re-consent 时，使用现有 authenticated Auth.js/ZITADEL identity + live Organization 上下文，服务器读取 current policy 并记录该 subject 的 Acceptance/Consent；请求体不得覆盖 subject/organization/current policy version。若特定政策要求 re-auth，再委托 Provider-native re-auth，不在 task-processor 新建密码/OTP 系统。

Current Consent Gate **不通过删除、Update 或 Reactivate ZITADEL 长期角色来模拟**；Consent 恢复后同一合法 authorization 自动重新获得业务可用性。

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

首次体验资源 Grant **不需要和这笔 Bootstrap transaction 强行合并成跨 package 巨型事务**；它是 Bootstrap 成功后、授权前必须收敛的独立幂等 readiness effect。这样 #283 只负责业务顺序，资源写入仍由 #284 的 Resource authority 完成。

## 10. Subscription + Welcome Resource Readiness

### 10.1 Subscription

`base_payg` 继续复用现有 `internal/listingsubscription`：

- `EnsureInitialPlanIfAbsent` 只在确实无 subscription 时赋 `base_payg`；
- 不覆盖 paid plan；
- existing paid plan 必须 canonical reconcile plan-owned entitlement projection；
- initial assignment 使用 DB conditional insert / locked authority，不能 check-then-unconditional-upsert。

### 10.2 One-time Welcome Store Renewal Grant

对本次确认为**新直接注册 Organization** 的首次业务开通，Onboarding 必须在 `preserved_business` 后确保以下资源事实 ready：

```text
resource_type = store_renewal_period
quantity = 1
source_type = onboarding_welcome_store_period
source_identity = organization_id
```

规则：

- 调用 PR #284 Resource authority 的窄 trusted Provisioning path；
- quantity 固定为 1，浏览器/tenant human 不可传入或提高数量；
- source identity 按 Organization 唯一，不按可变化的 registration/reclaim attempt 唯一；
- same Organization 同 source replay -> 返回原成功结果，不再次 +1；
- source 已成功后 Registration resume/reclaim/Reconciler 重跑只做 read-back；
- 历史/既有 Organization、existing active User 普通登录不触发该 Grant；
- paid plan race 不取消或重复该一次性 welcome Grant，它与套餐 code 独立。

如果 Resource authority 尚未就绪或 Grant outcome 未能权威确认：

```text
-> 不授予 listingkit_admin
-> RegistrationReconciler 保持 welcome_resource_pending / 等价 readiness 状态
-> 按同一 source identity 重试/read-back
```

不允许因为 welcome Grant 暂时失败而回退删除已经 `preserved_business` 的 Provider identity。

## 11. Project Grant / User Authorization

`listingkit_admin` 始终是最后一项业务访问效果。

进入 Project Grant / User Authorization 前必须同时确认：

```text
current Consent ready
current subscription/entitlements ready
one-time welcome store_renewal_period Grant ready（仅 new direct registration）
```

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

fresh Consent 必须通过 §8.2 的明确 re-consent transaction 建立，而不是假设原 Bootstrap/proof 可以重放。完成后 Reconciler 才可在同 target fence 下重新取得执行资格；`listingkit_admin` 绝不能凭已知 stale Consent 延迟授予。

### Policy activation 与远端 Provider send 的 Phase1 一致性边界

Phase1 **不承诺 PostgreSQL policy publication 与远端 ZITADEL Create 的跨系统可串行化原子性**。上述“send 前 current Consent DB check”是最后一个本地授权决策点；若新的 policy activation 在这个 DB check 之后、ZITADEL 接受 Create 之前提交，这被定义为“policy 在 authorization decision 之后变化”，而不是要求建设一个跨 PostgreSQL/ZITADEL 的全局分布式锁或回滚 Provider role。

为了保证这个边界不会变成业务授权绕过：

1. Provider Grant/User Authorization exact read-back 成功后，**在本地 `authorized` 终态提交前再次读取 DB current active policy/version**；
2. 若仍与刚才授权决策使用的 policy version 相同且 exact Consent current -> 才允许进入 §12 `authorized`；
3. 若已变化 -> Provider authorization 可以作为已存在的身份侧事实保留，但 Registration 进入 `authorization_present_consent_required` / 等价 `consent_required` 状态，释放短 work lease，不完成 Auth.js 业务进入；
4. 用户通过 §8.2 fresh re-consent 后，Reconciler exact read-back 现有 ACTIVE authorization，确认不需要再次 Create，再进入 authorized terminal；
5. 无论 Registration 状态如何，§8.3 `CurrentConsentPolicy` 是每个受保护 tenant business request 的持续权威，因此 stale Consent 永远不能借已经存在的 ZITADEL role 调用业务 API。

这个合同接受“policy 可以在远端角色创建的瞬间之后变化”，但**不接受 stale Consent 获得任何业务访问**。如果未来合规要求“policy switch 后连 Provider role 都不得短暂存在”，必须另行引入 policy-publication/send serialization；它不是 Phase1 当前一致性目标。

## 12. Authorized Terminal / Pending Capacity Release

Provider Grant + Authorization exact read-back 成功后，先执行 §11 的**post-send current-policy recheck**。只有 exact current Consent 仍成立时，本地终态才允许进入下面的 PostgreSQL transaction：

```text
lock Intent
lock pending_object reservation
lock verified work lease
lock/read current policy version
verify exact current Consent
verify expected ownership/fence/version
-> authorizing -> authorized
-> release/transfer pending Provider-object slot out of registration pending accounting
-> release verified work lease
-> close Registration Attempt
-> close same-phone claim when no unresolved sent Provider mutation remains
-> scrub transient phone material when replay/finality no longer needs it
-> authorized_at = now
-> commit
```

若 post-send policy 已变化，则不执行 `authorized`/pending-slot terminal transaction；进入 §11 的 `authorization_present_consent_required`，fresh re-consent 后再 exact-adopt 现有 authorization，并在当前 policy 下完成本事务。

pending-object release/transfer 与 `authorized` 不得拆开；crash/replay 不能泄漏或二次释放容量。成功账号继续存在于 ZITADEL，但不再计入“未完成注册 Provider object”高水位。

如果仍存在较早 sent Provider mutation 的 unresolved finality，`authorized` 业务终态与 pending accounting 可以按其真实业务合同完成，但 same-phone HMAC exclusion 不得提前释放；由 §3 `finality_hold` 独立保持到全部相关 sent mutation definitive。通常正常成功路径不应存在这种异常重叠；若出现必须审计并收敛。

`authorized` 之后的 policy 变化由 §8.3 `CurrentConsentPolicy` 持续执行；不能因为 Registration Intent 已终态就绕过新 policy Consent。

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

一旦 `preserved_business` 已成立，包括 welcome resource readiness 正在等待/已完成，都不得再走 destructive pre-business cleanup。资源 Grant 失败只能 repair/retry，不能通过删除 Identity “回滚”。

自动 destructive Provider Delete 还必须满足 pinned ZITADEL 已证明的 atomic/conditional ownership precondition 与 finality；否则 automatic Delete 关闭，new self-registration rollout 保持 gated/off，直到存在经过验证的 bounded cleanup strategy。

### Cleanup 成功后的 Pending Capacity 终态

取得 `cleanup_claimed` **不等于**可以释放 pending-object reservation。只有 RegistrationReconciler/cleanup owner 已对本 Registration 保护的全部 disposable Provider User/Organization 完成 Delete，并通过 Provider finality/read-back **确认目标全部不存在** 后，才能释放容量。

最终收敛必须在一个 PostgreSQL transaction 中完成：

```text
lock Registration Intent
lock cleanup claim / cleanup_epoch
lock pending_object reservation
lock relevant terminal Provider Delete Operations
lock same-phone claim
verify fence = cleanup_claimed(expected cleanup_epoch)
verify every protected disposable Provider target = definitively absent
verify no durable business ownership was created
verify no other sent identity mutation remains unresolved
-> cleanup_state = cleaned / 等价 terminal
-> Registration = terminal_cleaned / expired_cleaned
-> release pending_object reservation exactly once
-> release cleanup/work lease
-> close active-phone/finality-hold registration fence
-> scrub transient phone material when replay/finality no longer needs it
-> commit
```

规则：

- Delete response loss 后若 read-back 最终确认 absent，走同一终态事务，不能额外释放第二次；
- 任一 Provider target 仍存在、partial delete、finality ambiguous、ownership mismatch、其它 sent identity mutation 未 definitive 或 business ownership 出现时，**不得释放 pending capacity/same-phone finality exclusion**；继续由 RegistrationReconciler 做 finality/repair；
- 同 cleanup logical replay 只能读取 `terminal_cleaned`，不得重复 decrement/release reservation；
- capacity release 与 cleanup terminal transition 不得拆成两个 commit，否则 crash 会造成永久泄漏或双释放。

Reclaim 使用 fresh ownership proof；reclaim proof 与 onboarding-consent proof 分离。Reclaim 前同样 exact read-back Provider ownership，并与 Delete 共享 stable target fence；不允许 stale Delete 与新 Reclaim 并行 inflight。若旧 attempt 仍处于 §3 `finality_hold`，新的同手机号请求只能绑定旧 ownership 等待/收敛 finality，不能分配新 Provider IDs 后“绕过” reclaim fence。

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
- Challenge 创建是否支持安全 read-back/idempotency；若不支持，必须验证 §7 durable generation + cooldown + supersede/fresh-generation fallback；
- Project Grant / User Authorization exact read-back；
- durable Provider Write finality；
- Provider ownership attributes/marker read-back；
- destructive cleanup conditional ownership safety；
- credential negative permission matrix。

业务 readiness 还必须实测：

- `base_payg`/current plan entitlement reconcile；
- PR #284 welcome resource Grant path 对同 Organization source identity 的 exactly-once/replay；
- welcome Grant 不暴露 tenant/browser positive-mint API。

任一关键 Provider finality / Factor recovery / destructive cleanup 条件无法证明时，只关闭 **new phone self-registration**；Login V2、existing user login、Console 其他业务不因此停止开发。

## 16. Acceptance Tests

至少覆盖：

```text
existing active phone -> accountExists=true / 登录提示，不调用 AddOTPSMS
registration-owned pending phone -> resume/reclaim，不误送 ordinary login
provider ownership mismatch -> repair_required，不 adopt/不改 factor/不授权
new phone -> one Registration / one Provider identity
not_found atomic owner transaction crash-before-commit -> no Intent/no phone claim/no pending slot
not_found atomic owner transaction crash-after-commit-before-Provider-call -> Reconciler finds exact Intent+slot
same new E.164 concurrent requests -> one active Registration Attempt + one pending slot
hard TTL -> expired_fenced + execution_epoch fence before any same-phone claim release
expired_fenced vs leased Provider worker -> stale worker cannot new-send or advance local state; inflight only finality/read-back
old Create outcome_unknown at hard TTL -> same-phone claim enters finality_hold and remains unique; same-phone retry cannot allocate new IDs/Intent/slot even when Provider lookup temporarily not_found
finality_hold + delayed old Create appears -> exact old ownership resume/reclaim; never second Provider identity
finality_hold + definitive no-object -> terminal transaction releases pending capacity + same-phone claim exactly once, then fresh registration may start
key rotation -> maintenance gate 后切换；finality_hold claim 不因 key cutover 失去排他
rate limit / SMS cooldown / global SMS budget
Create Org/User response loss -> exact ownership read-back/adopt，不 blind retry
Create definitive no-side-effect/no-object failure -> terminal_no_provider_object + pending capacity atomic release；partial object 不释放
AddOTPSMS response loss -> factor read-back/idempotency，否则 outcome_unknown/repair
same logical Provider op changed payload -> conflict
stale prepared/inflight/outcome_unknown/retry_wait -> RegistrationReconciler durable recovery
phone-bearing retry_wait >24h but <72h -> required AEAD ciphertext retained and exact retry succeeds; purge cannot strand runnable operation
phone-bearing workflow reaches hard TTL -> expired_fenced first, future sends disabled, then ciphertext scrub when no remaining recovery call needs it
outcome_unknown needs only stable-ID read-back -> ciphertext may scrub but same-phone finality_hold remains until definitive
CreateSMSChallenge crash before pre-send transaction commit -> no challenge send/no generation
CreateSMSChallenge crash after challenge_preparing commit before send -> Reconciler safely resumes same unsent generation under lease
CreateSMSChallenge response loss after SMS sent -> challenge_outcome_unknown, no blind resend
challenge outcome cannot read-back -> cooldown then old generation superseded; explicit resend creates generation+1 only after pre-send persistence
late response from superseded challenge generation -> cannot overwrite current Session ref or produce proof
VerifySMS only current challenge_ready generation -> superseded/old generation rejected
VerifySMS response loss -> 无证明则 outcome_unknown，不写 Consent
current policy changes before Bootstrap -> zero business write
current policy changes after Bootstrap before Grant/Auth -> consent_required + fresh registration_reconsent proof/Acceptance；不重跑 Bootstrap
current policy changes while Grant/Auth retry_wait -> 下一次 send 前拦截，不授予 listingkit_admin
policy activates after final pre-send DB check but before Provider Create commits -> post-send policy recheck prevents local authorized/business entry; authorization_present_consent_required until fresh Consent
post-Bootstrap reconsent -> 只新增 current Consent，不重复 base_payg/business projection/welcome Grant
Cleanup vs Bootstrap race -> 只有 cleanup_claimed 或 preserved_business 一方成功
Cleanup Delete success + all protected Provider targets absent + no unresolved sent mutation -> terminal_cleaned + pending capacity + same-phone claim 同 tx exactly-once release
Cleanup Delete response loss -> read-back absent 后同一 terminal tx release once；ambiguous/partial delete 不释放
Cleanup terminal replay -> 不二次释放 capacity；Reclaim/Bootstrap 不能越过 cleanup target fence
new direct org welcome grant -> exactly +1 store_renewal_period
welcome grant response/retry/resume/reclaim -> same org source replay，不重复 +1
existing org / ordinary login -> 不触发 welcome grant
welcome grant unavailable -> 不授权，进入 readiness retry；不 destructive cleanup preserved business
successful authorization -> post-send current-policy recheck + consent + plan + welcome resource ready, authorized + pending-object release + work-lease release
policy changes after authorized -> protected tenant API 返回 CONSENT_REQUIRED，不能凭旧 Consent 继续业务访问
authorized user authenticated reconsent -> 写 current Consent 后业务访问恢复，不修改长期 ZITADEL role
paid plan race -> 不覆盖 paid plan，entitlements ready，welcome grant 仍最多一次
Grant/Auth absent create，exact active adopt，different/revoked repair
unsafe automatic Delete capability -> self-registration remains off
phone ciphertext / phone-derived fingerprints retention 后 scrub；不得存在 runnable operation 丢失必需 request material
```

## 17. 开发停止条件

本设计不以“自动评审找不到任何 P1/P2”为开工条件。真正阻塞实现的是会导致：跨租户/越权、重复 Provider identity/授权、错误 Consent 导致**实际 tenant business access**、paid plan 覆盖、welcome resource 重复 mint/漏发导致 Phase1 首次 Store 闭环失效、不可恢复 Provider side effect、身份删除与业务资产不一致、pending capacity 永久泄漏导致核心注册不可用、或核心注册流程无法完成的问题。

纯 Account Enumeration、branch-neutral capacity、cross-epoch side-channel、零停机 correlator key rotation，以及“要求 PostgreSQL policy activation 与远端 ZITADEL Create 全局线性化”不属于 Phase1 blocking requirement。Phase1 对 Consent 的强保证是：final local decision 前检查 current policy、Provider result 后再检查一次、且所有 protected tenant business request 始终由 DB `CurrentConsentPolicy` fail-closed 控制。

## 完成定义

ZITADEL 继续拥有认证核心；已注册 active 手机号可明确提示并转登录；registration-owned pending identity 可以安全 resume/reclaim；未注册手机号只产生一条可恢复 Registration；active-phone claim + Intent + pending reservation 在第一笔 Provider write 前同一事务建立 durable ownership；hard TTL 先原子 `expired_fenced`/execution epoch fence 停止旧执行，**same-phone claim 在任何 sent mutation 未 definitive 时继续进入 finality_hold，不能因 TTL scrub 后重新创建第二套 Provider identity**；Provider/Factor unknown outcome 不靠猜测；Provider adopt 必须验证 ownership；所有非终态 Provider operation 有唯一 durable recovery owner；phone ciphertext 的 privacy purge 不能删除仍 runnable recovery 所必需的 request material；CreateSMSChallenge 在发送前已有 durable generation，response loss 不 blind resend，旧 generation 被 supersede 后不能产生 Proof；definitive no-object Create failure 与 successful definitive Cleanup 都会在正确终态 transaction 中 exactly-once 释放真实 pending capacity 与可释放的 same-phone exclusion；Current Consent 在每次延迟授权 send 前、Provider authorization read-back 后、以及 authorized 后每个受保护 tenant business request 上都持续 fail-closed；policy activation 与远端 Provider send 不要求跨系统全局线性化，但 race 后不会获得业务访问；Bootstrap 后 policy 变化有独立 fresh re-consent 路径且不重复 business ownership；Cleanup 与 Business Bootstrap 有互斥 fence；新直接注册 Organization 的一次性 `store_renewal_period=1` 在授权前按 Organization source identity 幂等 ready；authorized 与 pending capacity release 同事务；`listingkit_admin` 最后授予；没有 Admission Epoch / cross-epoch anti-enumeration 复杂度。