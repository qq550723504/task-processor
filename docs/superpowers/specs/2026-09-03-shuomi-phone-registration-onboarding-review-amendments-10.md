# 硕米手机号注册与 Onboarding 第十轮评审修订

本文件针对 PR #283 Head `3716fe2` 的最新 Code Review / Security Review 继续收敛。与前序文档冲突时，以本文件和新的 `2026-09-03-shuomi-phone-registration-onboarding-plan-v4.md` 为准。

## 1. Provider Target Fence 必须跨 Registration / Ownership Epoch 稳定

旧 V3 把 Target Fence 唯一键放在 `registration_id + ownership_epoch` 上，无法阻止旧 ownership epoch 的 Delete 与新 epoch 的 Reclaim 并发。

改为以 **Provider 稳定目标** 为互斥权威：

```text
UNIQUE(provider_target_type, provider_target_id)
active_logical_operation_key
active_registration_id
active_ownership_epoch
mutation_class
state
lease_owner / lease_until / fence_epoch
```

`registration_id` / `ownership_epoch` 只作为当前 claim 元数据，不参与 fence identity。

因此同一个 ZITADEL User/Organization 上：

- old Delete 与 new Reclaim 共享同一个 fence；
- marker repair / reclaim / delete 等冲突 mutation 共享同一个 fence；
- lease 过期只能接管协调权，不能越过未 definitive 的旧 Provider outcome；
- 多 target mutation 必须按稳定 target key 排序取得 fence，避免死锁。

## 2. Lookup 前原子预留 Provider Headroom

仅在 lookup 前“检查 high-water”、lookup 后才为 genuinely-new 分支申请 Provider slot，仍有 last-slot TOCTOU / existence oracle。

Phase1 改为两个固定数据库 semaphore pool：

```text
registration_admission_slots
provider_pending_capacity_slots
```

每个 slot 自身是权威，不依赖易漂移的 `in_use` counter：

```text
slot_id
state = free | leased | pending_object
lease_key
lease_until
provider_identity_id nullable
```

`/register` 在手机号 lookup 前一个 PostgreSQL 事务内同时：

```text
1. acquire/reclaim one generic admission lease
2. acquire/reclaim one branch-neutral provider-headroom lease
3. persist attempt logical key -> both leases
4. commit
```

任一 lease 不可得：同事务不保留另一 lease，返回 generic unavailable，**不 lookup、不发 SMS**。

lookup/classification 后：

- generic admission lease：existing / pending / new 全部分支统一释放；
- provider headroom lease：
  - existing active -> 释放；
  - pending registration-owned -> 释放临时 headroom，继续使用该 pending identity 已有 `pending_object` slot；
  - genuinely-new -> 同事务把该 headroom lease 转为该 `provider_identity_id` 的 `pending_object` slot，再允许 Provider Create。

这样 last-slot race 不会在 lookup 后暴露分支。

## 3. Admission / Headroom Lease 必须可回收

Generic Admission 与 temporary Provider Headroom 都使用 `lease_until`。Acquire transaction 本身可原子复用 expired lease；另有低频 Admission Reaper 扫描异常长期 lease 并做幂等释放/审计。

要求：

- same logical attempt 重试 adopt 原 lease，不重复占位；
- request crash/cancel 后 lease 到期可直接被后续 acquire 回收；
- `pending_object` 不是临时 lease，不因 TTL 自动释放；只有 definitive Provider Delete 或 authorized 转出 pending accounting 才释放；
- Reaper 不得把 `pending_object` 当 expired temporary lease 回收。

## 4. Current Consent 必须先于或原子伴随第一笔 Durable Business Ownership

不能先写 business artifact / `preserved_business`，再发现 policy acceptance 已过期。

第一次 non-disposable business write 的事务固定为：

```text
lock intent + verified work lease + ownership fence
-> DB read/lock active policy release (version + epoch)
-> lock current proof + acceptance
-> require acceptance matches current policy exactly
-> consume proof + acceptance once
-> insert current saas_account_consents
-> write first durable business ownership
-> ownership fence = preserved_business
-> commit
```

若 policy 已变化：

```text
no Consent write
no business artifact
no preserved_business
release verified work lease
state = consent_required
```

之后再进行 projection / base_payg / entitlement readiness。

## 5. Reclaim 使用独立 Proof；Reclaim 后重新做 Consent Proof

Phase1 不让一次 OTP Proof 同时被 Reclaim 和 Consent 两次消费。

Reclaim 固定为两阶段、两次 proof：

```text
Proof A (purpose = reclaim)
-> once-only consume for reclaim claim
-> Provider Reclaim durable operation + stable target fence
-> finality definitive
-> local ownership epoch / registration rebind
-> state = reclaim_consent_required

Proof B (fresh, purpose = onboarding_consent)
-> fresh challenge/session
-> fresh current-policy acceptance
-> Task 4 transaction consumes Proof B + acceptance
-> persist current Consent
-> first durable business ownership
```

这是 rare abandoned-registration 恢复路径，宁可多一次 OTP，也不破坏 single-use proof 不变量。

Proof A 绝不能被 Consent 使用；Proof B 绝不能被 Reclaim 使用，`purpose` 进入 proof immutable fingerprint。

## 6. Provider Delete 前必须实时重验 Ownership

破坏性 Delete 前，在拿到 stable target fence 且 business ownership fence 仍是同一 `cleanup_claimed/cleanup_epoch` 后，必须重新读取 ZITADEL：

```text
expected Provider User ID
expected Provider Organization ID / association
expected canonical login name
expected registration ownership marker / resource owner
expected immutable local ownership evidence
```

全部 exact match 才能 Delete。任一 marker 被管理员移除/修改、Organization 关系变化、login ownership 不一致、target 已被外部接管：

```text
PROVIDER_OWNERSHIP_REPAIR_REQUIRED
no Delete
no provider-slot release
manual / reconciler repair
```

## 7. Existing User 无 SMS Factor 的 Pre-proof UI 不得泄露分支

继续坚持：`/register` **绝不**给 existing active user 自动 AddOTPSMS。

但在 OTP Proof 前，浏览器也不能因为“已有账号但无可用 SMS factor”而得到不同的 redirect/nextAction。

所有输入统一展示 generic continuation，例如：

```text
“如果该号码可用于验证，请查看短信；也可以选择其他登录/找回方式。”
```

并且“其他方式登录/找回”入口对 **所有输入统一展示**。

服务器内部：

- genuinely-new / registration-owned pending：可按注册合同发送 challenge；
- existing active + usable factor：按 ZITADEL 当前 factor policy 发送/继续；
- existing active + no usable factor：不发送、不新增 factor；等待用户主动选择统一展示的官方 login/recovery 入口。

在独立身份事实被证明前，不返回 `no_factor / existing_user / recovery_required` 等可枚举状态。

## 8. 第十轮验收

至少增加：

```text
old ownership Delete vs new ownership Reclaim -> stable target fence only one inflight
last provider slot: known/unknown/pending concurrent -> pre-lookup branch-neutral reservation
request crash after admission/headroom lease -> TTL reuse without capacity leak
pending identity retry -> temporary headroom released, existing pending slot not double-counted
policy changes between OTP acceptance and business prepare -> no business artifact/preserved_business
reclaim Proof A cannot be used for Consent
fresh Proof B required after reclaim
admin changes ownership marker immediately before Delete -> no Delete
existing user with/without SMS factor -> same pre-proof public continuation and available UI actions
```

## 9. IAM 边界不变

本轮没有把 OTP/Factor/Password/Session/OIDC 搬回 task-processor。ZITADEL/Login V2 仍是认证权威；task-processor 只保存非凭据 Registration/Provisioning/Consent/Onboarding 状态。