# 硕米手机号注册与 Onboarding 第七轮评审修订

本文件针对 PR #283 在 `a813344` 上的新一轮 Code Review / Security Review 结论进行收敛。与前序文档冲突时，以本文件与 `2026-09-03-shuomi-phone-registration-onboarding-plan-v2.md` 为准。

## 1. Global Pending Cap 必须在注册分流前统一生效

`/register` 不得在 global unverified-provider capacity 已满时先做手机号 lookup，再让 active/existing 分支继续真实 OTP，而 new 分支失败。否则 capacity saturation 会变成账号存在性 oracle。

权威规则：

```text
Register Begin
-> trusted ingress / abuse checks
-> read atomic unverified-provider admission availability
-> if full: every phone returns the same generic temporarily-unavailable result
   and MUST NOT perform account-existence lookup or create/send OTP challenge
-> only when capacity class admits this registration attempt may server perform exact E.164 lookup
-> existing / pending / new remain server-internal branches
```

补充：

- 这是 `/register` 的 admission shield，不得影响独立 `/login` 的正常 active-user OTP 登录能力；
- Register 页面不得在 full-cap 时自动跳到 login，从而重新暴露账号存在性；
- known/unknown/pending phone 在 cap saturated 时必须拥有同 HTTP status、同 public error code、同 UI nextAction，且不会有一方收到真实 SMS；
- 测试必须覆盖 known vs unknown vs pending phone 在 cap 满时的 challenge/resend 行为与时序类别。

## 2. Current Plan Entitlement Repair 必须修复 stale plan-owned rows

`EnsureCurrentPlanEntitlementsReady` 不能只 `insert missing`。并发 `base_payg -> professional` 时，subscription 已切到 professional、旧 `store_management=1` row 仍存在的 crash window 必须被收敛到 professional 的 canonical value。

权威规则：

```text
lock current subscription/version
-> load canonical plan catalog
-> reconcile every plan-owned entitlement row:
   missing -> insert canonical row
   stale canonical fields -> update to current plan canonical value
   exact -> no-op
-> read-back current subscription/version
-> if plan changed during reconciliation: bounded retry from latest plan
-> only after canonical projection matches current plan may business_prepared proceed
```

Plan-owned 与人工 override 必须分层：

- plan projection row 必须具有可判定 provenance（例如 `source_kind=plan_catalog` + `source_plan_code/version`，或等价既有权威字段）；
- 人工 override 不得通过直接篡改 plan-owned row 表达，应进入独立 override 层；
- 若 legacy entitlement 无法判定 provenance，先做 migration/classification，不能盲目覆盖；
- readiness 计算必须以 `current plan canonical projection + explicit override layer` 为准。

## 3. Consent expiry 不得删除已经拥有业务资产的 tenant

`consent_required` 的 bounded cleanup 只能清理仍然 disposable 的 registration。只检查“没有 User Authorization”不够。

Provider Delete / Organization Delete 前必须在同一 cleanup decision 中确认不存在任何 non-disposable business ownership：

```text
paid or base subscription
plan entitlement projection
business user/org projection
Project Grant
User Authorization
Store / resource / order / audit references
other durable tenant-owned business artifact
```

只要任一存在：

- 禁止自动删除 Provider User / Organization；
- 进入 `consent_required` / `repair_required` 的已验证业务身份路径；
- 用户完成 Current Policy re-consent 后继续 authorization；
- 不做“删身份后遗留本地 subscription”的补偿式清理。

### 3.1 Capacity 分层

Provider creation cap 的目标是限制**未验证、无业务归属**的外部 Provider object 增长，不应把已经 OTP verified 且拥有 durable business ownership 的用户永久占在同一个匿名攻击流量池中。

引入两个逻辑容量类（可以是同一表的不同 class）：

```text
unverified_provider_capacity
verified_onboarding_backlog
```

转换规则：

- Provider Create 前占用 `unverified_provider_capacity`；
- OTP verified 后仍未产生 durable business ownership，继续占该 slot；
- 一旦 `business_prepared` 或检测到已有 paid/non-disposable business artifact，事务内把 slot 从 unverified class 转移到 verified onboarding class；不得同时计两次；
- `authorized` 后释放 verified onboarding slot；
- verified onboarding 同样有独立 hard limit、SLO 和 alert，但其饱和不得改变 `/register` 对 known/unknown phone 的可观察分流。

## 4. OTP SMS Factor Enrollment 是 pinned Login V2 合同的一部分

PR #218 已验证的 pinned v4.17.1 顺序包含：

```text
Create/Get Session
-> AddOTPSMS
-> CreateSMSChallenge
-> VerifySMS
```

因此 capability gate 必须验证 `AddOTPSMS`，不能只验证 challenge/verify。

约束：

- 优先复用 `internal/listingkit/phoneonboardingpreflight` 已验证的 request shape，不另造 OTP 实现；
- `AddOTPSMS` 必须发生在首次 challenge 前；
- task-processor 不接收 OTP code；
- 必须实测 `AddOTPSMS` 的 retry / duplicate / response-loss 行为。

如果 `AddOTPSMS` 成功后响应丢失且 provider 不提供可安全 read-back/idempotency：

- 不允许盲目重复同一未知写入；
- 废弃该 Login V2 Session，创建新的 attempt-bound Session 后重新 enrollment；
- 新 Session 仍受 SMS challenge rate-limit / global budget 约束；
- 若该恢复语义无法被 pinned ZITADEL 实测证明，则 self-registration rollout fail closed。

## 5. 所有长生命周期 Provider Write 与 Cleanup 使用同一 finality fence

之前 mutation claim 只覆盖 Reclaim/Delete 不够。以下影响长期 User/Organization ownership 的 Provider 写入都必须进入同一 durable Provider Operation / finality protocol：

```text
Create Organization
Create Human User
write/repair ownership metadata marker
Reclaim ownership metadata mutation
Delete User / Organization
```

每个操作至少持久化：

```text
operation_id
registration_id
kind
provider_target_id
state = prepared | inflight | outcome_unknown | succeeded | failed_definitive
owner
lease_until
epoch
request_fingerprint
last_checked_at
```

核心不变量：

- Cleanup 只有在该 Registration 没有任何 `prepared/inflight/outcome_unknown` Provider Write 时才能获得 deleting claim；
- response loss 后必须先按 stable target ID / provider operation status / verified finality contract read-back；
- delayed Create outcome 未 definitive 时，Janitor 不得因为当前 Get=absent 就释放 capacity；
- stale worker lease takeover 只转移协调权，不能取消已经发出的 provider mutation；
- Provider absence + no pending write finality proof 同时成立后，才能释放 unresolved capacity。

Login V2 的短生命周期 Session/OTP challenge 不进入这个长期 Provider object ledger；它们继续属于 attempt-bound Login V2 flow。

## 6. Ownership Marker API 是 rollout capability gate，不是假定能力

现有 recovery / reclaim / cleanup 设计依赖 Provider-side ownership marker 时，必须在实现前实测：

```text
read Organization marker
write Organization marker
read User marker
write User marker
narrow provisioning credential permissions
same-value retry/idempotency behavior
response-loss 后 read-back
cross-tenant negative permission test
```

若 pinned ZITADEL / 当前 credential 不能满足这些要求，不得继续依赖“marker repair”作为自动恢复条件。此时必须重新设计为只依赖 stable caller-supplied IDs + 本地 immutable ownership evidence，并同步删掉所有 marker-dependent reclaim/cleanup 前置条件；在设计重写前 self-registration 保持关闭。

## 7. 第七轮实现验收矩阵

至少新增：

```text
cap 满：known / unknown / pending phone 全部不 lookup、不发 challenge、相同 public result
base_payg entitlement row 存在但 paid plan 已切换：repair stale value 到 paid canonical
legacy/manual override 不被 plan projection repair 误覆盖
business_prepared + policy change：不得 Provider Delete
paid plan 并发写入 + consent deadline：不得删除 tenant
unverified slot -> verified onboarding slot 转移只发生一次
AddOTPSMS -> Challenge 顺序
AddOTPSMS response loss 的 verified recovery path
Create Organization delayed success vs Janitor cleanup
Create User delayed success vs capacity release
marker write response loss / read-back
marker API permission negative tests
```

## 8. 边界保持不变

这些修订不改变产品架构：

- ZITADEL/Login V2 继续拥有 OTP、Password、Session、OIDC；
- task-processor 不保存或验证 OTP code；
- Registration Provisioning 只解决手机号自助注册的稳定 Provider object 创建与业务 Onboarding；
- 不新增 PhoneIdentity 长期目录、Decoy User、Callback Runtime 或 Temporal 登录工作流。
