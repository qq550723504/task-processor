# 硕米手机号注册与 Onboarding 第十二轮评审修订

本文件针对 PR #283 V5 最新 Code/Security Review 继续收敛。与前序文档冲突时，以本文件和 `2026-09-03-shuomi-phone-registration-onboarding-plan-v6.md` 为准。

## 1. Provider Safety Reservation 必须在所有 lookup 分支保持同一可观测生命周期

V5 在 lookup 后 `existing/pending -> release`、`new -> pending_object`，仍会把账号类别编码到后续可用容量中。Phase1 不再允许 branch-specific early release/convert 影响 `/register` admission。

新的模型：

```text
registration_admission_lease     // 短期执行 bulkhead
provider_safety_reservation      // branch-neutral fixed hold
```

所有 `/register` attempt 在 E.164 lookup **之前**原子取得两者。`provider_safety_reservation` 的 `not_before_release_at` 由 rollout policy 在 admission 时固定，例如统一 72h；existing / pending / genuinely-new 都保持同样 reservation footprint，lookup/classification/OTP success 不允许提前释放。

Genuinely-new 若创建了 pre-auth Provider object，该 object 绑定到现有 safety reservation，但 reservation 的公共占用语义不改变；existing/pending 只是 shadow hold。`authorized` 也只把 reservation 标成 `terminal_shadow/release_pending`，不得立即把公共 headroom 返还。

到 `not_before_release_at` 后，仅当满足以下任一 definitive terminal proof 时才释放：

- existing/no-create branch 已 terminal；
- new branch 已 authorized，pre-auth object 不再属于 unresolved inventory；
- safe cleanup 已 definitive delete；
- Provider Create `failed_permanent` 且 finality 证明**没有任何 Provider object/side effect**。

若时间到但仍有 unresolved Provider side effect，reservation 不释放，同时 self-registration circuit breaker fail closed；不能以 branch-specific release 继续对外提供可探测容量。

因此一个 victim probe 不再通过“下一次是否能拿到最后 Provider slot”推断 victim 是 existing 还是 new。

## 2. 同一手机号只能有一个 Active Registration Attempt

`logical_attempt_key` 只解决 same-key replay，不能阻止攻击者给同一手机号生成多个 key。

新增短生命周期 correlator：

```text
active_phone_correlator = HMAC(K_registration_attempt, "active-phone:v1" || normalized_E164)
correlator_key_version
```

数据库使用 active-only uniqueness，例如：

```text
UNIQUE(active_phone_correlator) WHERE active_phone_correlator IS NOT NULL
```

规则：

- correlator 在任何 Provider lookup 前建立；
- 同手机号不同 logical key 只能 adopt/resume 已有 active attempt，不能再拿第二组 admission/safety reservation；
- public response/nextAction 仍保持 generic，不返回“已有注册尝试/已有账号”等 branch 信息；
- correlator 只用于短期并发与 admission fencing，不用于查找长期账号，不是 PhoneIdentity/Phone HMAC Alias；
- authorized、definitive cleanup 或 retention deadline 时 scrub；最长跟随 phone repair/quarantine retention（<=72h）。

## 3. Side-effect-free Permanent Create Failure 也必须有 bounded capacity release

Provider Create Organization/User 的 permanent semantic rejection 若 finality 已证明 no object/no side effect，不得留下永久 safety reservation。

事务语义：

```text
Provider Operation -> failed_permanent_no_side_effect
Intent -> terminal_no_provider_object
provider safety reservation -> release_pending(not_before_release_at)
scrub provider create claim / transient phone per retention policy
```

到统一 branch-neutral `not_before_release_at` 后由 Reconciler/Reaper 幂等 release。这样既不泄露 branch，也不会永久吞掉 capacity。

如果 permanent failure 发生在部分 Provider side effect 已成功之后，则不能走 no-side-effect release；必须继续 finality/cleanup/repair gate。

## 4. Project Grant / User Authorization Create 纳入 Provider Operation Finality

Create Project Grant 与 Create User Authorization 都是 durable ZITADEL write，不能继续使用“absent -> unkeyed POST -> timeout 后再查/重试”的隐式流程。

Provider durable write 集合扩展为：

```text
Create Organization
Create Human User
Ownership Marker Write/Repair
Reclaim
Delete
Create Project Grant
Create User Authorization
```

Project Grant logical target 至少稳定绑定 fixed project + organization；User Authorization target 稳定绑定 fixed project/grant + organization + user。

每个 Create 都必须：

1. durable logical operation + immutable fingerprint；
2. stable target fence；
3. absent/exact-active read-back；
4. bounded Provider call；
5. timeout/ambiguous response -> `outcome_unknown`；
6. 只有 proven no-side-effect transient -> `retry_wait`；
7. exact active -> adopt succeeded；different/inactive/revoked -> repair_required，不 Update/Reactivate。

如果 pinned ZITADEL 对 Grant/Authorization Create 无法提供安全 read-back/finality/idempotency，则对应自助注册 rollout gate 不通过。

## 5. `authorized` 不再提前释放 Branch-neutral Safety Reservation

第十一轮“authorized + pending capacity release 同事务”被本轮 branch-neutral 要求细化：

```text
same tx:
  authorizing -> authorized
  unresolved Provider inventory -> account-owned / no longer pre-auth
  provider_safety_reservation -> terminal_shadow(release_pending, not_before_release_at unchanged)
  verified work lease -> release
  transient phone/correlator -> scrub
```

Safety reservation 到统一时间后由 Reaper 释放。这样 terminal crash 仍可恢复，同时不把成功时间/账号类别编码成 public headroom。

## 6. 第十二轮验收

至少新增：

```text
same E.164 + two different logical keys concurrently -> one active attempt / one lookup execution
same E.164 active attempt retry -> adopt, no second safety reservation
existing/new/pending after classification -> identical provider-safety occupancy until fixed release time
victim probe + controlled canary at last slot -> canary admission does not reveal victim branch
new Create permanent no-side-effect -> terminal + bounded release at uniform not-before time
partial Create side effect + later failure -> no unsafe release
Create Project Grant response loss -> outcome_unknown -> exact read-back/adopt, no duplicate mutation
Create User Authorization response loss -> outcome_unknown -> exact read-back/adopt, no duplicate mutation
authorized before hold expiry -> terminal_shadow still occupies safety reservation until same release time
retention expiry -> phone correlator scrubbed, no long-lived account directory remains
```

## 7. IAM 边界不变

ZITADEL/Login V2 继续拥有 OTP、Factor、Password、Session、OIDC。task-processor 的 HMAC correlator 只是一条 <=72h 的 Registration concurrency fence，不成为身份系统或长期手机号索引。