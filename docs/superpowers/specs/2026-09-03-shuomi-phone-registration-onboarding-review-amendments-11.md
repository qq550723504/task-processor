# 硕米手机号注册与 Onboarding 第十一轮评审修订

本文件针对 PR #283 V4 最新评审继续收敛。与前序文档冲突时，以本文件和 `2026-09-03-shuomi-phone-registration-onboarding-plan-v5.md` 为准。

## 1. 自动 Provider Delete 必须具备原子破坏性前置条件

“read ownership -> Delete”仍存在 Provider 外部管理员在两步之间改变 ownership 的 TOCTOU。Phase1 **不得假定** ZITADEL 支持 conditional/versioned delete。

自动 destructive cleanup 的 rollout gate 必须实测并证明至少一种原子语义：

- conditional/versioned delete；
- compare-and-delete；
- 与 Delete 绑定的 immutable version/etag precondition；
- 或等价的 Provider-side atomic ownership precondition。

若 pinned ZITADEL 无法证明该能力：

```text
automatic Provider Delete = disabled
self-registration = rollout gated/off
```

除非另有经过评审的 bounded safe cleanup strategy。不能用“刚刚 read-back 过”代替原子 destructive precondition。

## 2. Admission Attempt 也必须有稳定逻辑幂等

新增：

```text
logical_attempt_key UNIQUE
request_fingerprint
lease_owner / lease_until / epoch
```

`request_fingerprint` 使用已有 Operation Fingerprint Key 对以下 canonical tuple 做 keyed HMAC：

```text
normalized E.164
presented policy version
flow version
behavior-changing registration fields
```

这只是 operation idempotency，不建立长期 PhoneIdentity 目录。

规则：same key + same fingerprint replay/adopt；same key + changed fingerprint -> `REGISTRATION_ATTEMPT_REPLAY_CONFLICT`；并发 same-key 只能一个 execution lease 执行 lookup/SMS。

## 3. VerifySMS response-loss 也必须 fail closed

Capability Gate 必须实测 VerifySMS response-loss 语义。自动恢复只允许：

1. Login V2 安全保留本次 pre-verify Session credential，并可 read-back Session/factor 已验证状态；或
2. Provider 提供 Verify 的安全幂等/operation-status 合同。

否则：

```text
verification_outcome_unknown
-> no proof verified
-> no Acceptance / Consent
-> no business progression
-> no blind Verify retry
```

若旧 credential 无法恢复，则 cooldown 后创建 fresh challenge generation / fresh Login Session；旧 generation 永不能作为 proof。无法获得安全 fresh proof 时 rollout fail closed。task-processor 仍不验证 OTP code。

## 4. `authorized` 与 capacity release / phone scrub 同一 PostgreSQL 事务

Project Grant + User Authorization Provider read-back 已成功后，本地终态必须一个事务完成：

```text
lock Intent
lock pending_object capacity slot
lock verified work lease
authorizing -> authorized
pending Provider slot -> release / transfer out pending accounting
verified work lease -> release
transient phone ciphertext/correlator -> scrub
authorized_at = now
commit
```

重复执行不可二次 release/scrub。

## 5. 所有 Durable Business Ownership Creator 都必须经过同一 Fence

Business Ownership Fence 状态固定：

```text
onboarding_writable
bootstrap_claimed
preserved_business
cleanup_claimed
```

只有 Task 9 Bootstrap 可以 CAS `onboarding_writable -> bootstrap_claimed`，并在**同一 PostgreSQL 事务**里验证 current Consent、创建第一笔 durable business ownership、转 `preserved_business`。

其他 ownership creator（paid ApplyPlan、business projection、Project Grant preparation、Store/resource/order 等）只能在 `preserved_business` 下写；不能仅检查“不是 cleanup”。

Legacy/business tenant backfill 为 `preserved_business`，不伪造 Consent；用户访问仍由 CurrentConsentPolicy 单独 gate。

## 6. Definitive transient Provider failure 使用 `retry_wait`

Provider Operation 状态统一：

```text
prepared
inflight
outcome_unknown
retry_wait
succeeded
failed_permanent
```

只有 Provider 合同能**确定没有副作用发生**且错误明确 retryable（例如经过验证的 429/503）时，才进入 `retry_wait`，记录：

```text
next_attempt_at
attempts
retry_after
total_deadline
```

始终复用同 logical operation + stable target fence，由 RegistrationReconciler 重试。

网络 timeout、connection loss、side-effect 不确定 5xx -> `outcome_unknown`，禁止 blind retry。语义永久错误 -> `failed_permanent`。

## 7. E.164 只做短期加密保留

手机号规范化在内存完成。若跨请求恢复必须持久化，保存：

```text
phone_ciphertext      // AEAD
phone_key_version
phone_retention_until
phone_operation_fingerprint // keyed HMAC，可选
```

禁止持久化明文 `normalized_phone`。

- authorized 终态事务立即 scrub；
- definitive cleanup 立即 scrub；
-普通 pending 目标 <=24h；
- repair/quarantine 异常最多 72h，之后即使 workflow 未收敛也 scrub；
- 独立 purge worker 负责 retention；
- future reclaim 重新输入手机号并做 fresh ZITADEL proof；
- raw phone 不进入 logs/errors/traces/query。

## 8. 第十一轮验收

至少覆盖：

```text
provider lacks atomic conditional delete -> auto cleanup off / self-registration rollout blocked
same logical attempt same fingerprint -> one lookup/SMS execution
same logical attempt changed phone/policy -> conflict
VerifySMS response lost + no read-back/idempotency -> verification_outcome_unknown
VerifySMS response lost + proven read-back verified -> safe convergence
authorized commit crash/retry -> capacity release + phone scrub exactly once
paid ApplyPlan before preserved_business -> denied
bootstrap Consent + first business ownership rollback together
Provider 429 definitive-no-side-effect -> retry_wait same logical op
ambiguous 5xx -> outcome_unknown, no blind retry
pending phone ciphertext auto-purged at retention deadline
```

## 9. IAM 边界不变

ZITADEL/Login V2 继续拥有 OTP、Factor、Password、Session、OIDC；task-processor 只保存非凭据 Registration/Provisioning/Consent/Onboarding 状态。