# 硕米手机号注册与 Onboarding 第四轮评审修订

**适用 PR：** #283

**覆盖关系：** 本文在冲突处覆盖前序手机号注册 / Onboarding 设计与评审修订。

**边界不变：** ZITADEL/Login V2 继续负责 OTP、Password、Session、OIDC；task-processor 不实现 OTP 校验器、Password 校验器、长期 PhoneIdentity、OIDC Callback Runtime 或登录 Temporal Workflow。

---

## 1. OTP Proof 必须绑定当前 Registration Attempt

仅验证 `user + otpSms` 不足以授权 Consent / Reclaim。每次需要 OTP Proof 的 Registration Attempt 必须有服务器端 `proof_attempt_id`，并绑定到本次 Login V2 创建的 ZITADEL Session / Challenge。

持久化最小信息：

```text
registration_id
proof_attempt_id
provider_user_id
provider_session_ref_hash
challenge_created_at
proof_verified_at NULLABLE
proof_consumed_at NULLABLE
state = pending | verified | consumed | expired
```

约束：

- task-processor 不接收 OTP code、Session Token；
- Login V2 在 ZITADEL Verify 成功后，通过受信 Internal Registration API 提交本次 `proof_attempt_id`、非凭据型 Session 引用和 ZITADEL factor verification time；
- 服务端要求 proof 的 Session/User 与当前 Registration Attempt 完全匹配，`verified_at >= challenge_created_at` 且未超过短 TTL；
- Consent 写入、Reclaim ownership epoch 切换必须在数据库事务中将 proof `verified -> consumed`，同一 proof 不得被第二次使用；
- 旧 Registration Attempt 的仍有效 ZITADEL Session 不能直接证明新的 attempt。

这只是对 ZITADEL 已完成认证结果做 attempt 绑定，不复制 OTP 验证逻辑。

---

## 2. Reclaim 只转移现有 Pending Capacity Slot

Reclaim 不重新申请第二个 Provider Pending Slot。

```text
old quarantined intent (capacity_slot_id = X)
  --OTP proof + row lock-->
new ownership epoch / new registration_id
  capacity_slot_id = X
old intent.capacity_slot_id = NULL
old intent.superseded_by = new_registration_id
```

整个转移在一个 PostgreSQL 事务中完成，并由唯一约束保证一个 Slot 同时只能属于一个 unresolved registration。

`authorized` 时释放 X；若新 attempt 再次 abandon，X 仍随 unresolved Provider identity 保留，不重复 +1。

必须覆盖 repeated abandon -> reclaim -> abandon -> reclaim 的 replay/concurrency 测试，确保 `capacity.in_use` 始终等于 unresolved Provider identities 数量，而不是 attempt 数量。

---

## 3. 所有 Post-OTP 状态都有 Registration Reconciler

Janitor 只负责 pre-proof cleanup；从 OTP Proof 成功开始，统一由 `RegistrationReconciler` 收敛。

权威状态与 owner：

```text
otp_verified
business_preparing
business_prepared
project_grant_ready
authorizing
  -> RegistrationReconciler

authorized
  -> terminal
```

`PrepareBusiness` / Reconciler 必须可幂等重入：

- `otp_verified`：进入本地 Business Prepare；
- `business_preparing`：读取本地 Projection / Consent / Subscription 的数据库权威结果，缺什么补什么；
- `business_prepared`：进入 create-only Project Grant；
- `project_grant_ready/authorizing`：read-back Provider Grant / Authorization 后继续；
- 每次重试有 bounded backoff、deadline 和明确 `repair_required` 终态，不能无限占用 capacity；
- 本地 Business Projection + `EnsureInitialPlanIfAbsent(base_payg)` 等数据库效果尽量在一个本地事务 / 可重入 Ensure 边界内完成。

不引入 Temporal；这是一个单领域、数据库投影驱动的轻量 reconciliation loop。

---

## 4. Reclaim 与 Provider Delete 使用互斥 Mutation Claim

仅靠最初行锁不能覆盖外部 ZITADEL Delete/Rebind 调用。

Registration Intent 增加：

```text
provider_mutation_state = none | reclaiming | deleting | outcome_unknown
provider_mutation_epoch
provider_mutation_owner
```

规则：

- Janitor 要 Delete 前，事务内 CAS `none -> deleting`；
- Reclaim 要开始前，事务内 CAS `none -> reclaiming`；
- 二者互斥；看到对方 claim 必须 fail closed；
- 外部调用前后都 read-back claim epoch；
- Provider response loss -> `outcome_unknown`，由 Reconciler 根据 Provider 实际对象/ownership read-back 收敛；
- 只有 claim owner 才能完成/释放 mutation claim。

这样手机号所有者不会在旧 Janitor 已准备删除 Provider 对象时完成 Reclaim，然后立刻被旧 Delete 删除。

---

## 5. E.164 Lookup 不得把手机号暴露到错误或遥测

不能直接复用当前会把 `loginName` 拼进错误 path 的高层 helper。

实现优先级：

1. 当前 ZITADEL 版本若有 body-based exact login-name lookup，使用该接口；
2. 若只能使用 query lookup，则抽取专用 registration lookup client：
   - 返回错误只包含 operation/status/request-id，不包含 URL/query；
   - HTTP client tracing 禁止记录 query string；
   - 代理/Ingress 到 ZITADEL 的 access log 对该路径关闭 query logging 或做强制 redaction；
   - 应用日志/metrics/traces 只记录不可逆 request correlation，不记录 E.164；
3. 若部署环境不能证明 query 不会进入标准 proxy/client 日志，则手机号注册 rollout fail closed。

仍可复用已有 lookup 的 API 语义和底层认证，不复用其泄露明文手机号的错误包装。

---

## 6. OTP Challenge 发送限流由 ZITADEL/Login 边界负责

不在 task-processor 重写 OTP，但必须验证短信发送防滥用。

Rollout Stop Condition：预发布必须证明 ZITADEL/Login V2 已对 Challenge/Create/Resend 提供满足要求的：

```text
per-session cooldown
per-phone rolling limit
per-IP rolling limit
global SMS send budget / circuit breaker
verification attempt limit
```

若 ZITADEL 原生策略不足，只能在 Login Fork / Ingress 的 Challenge 调用前增加最窄限流；task-processor 不读取 OTP code，也不做验证码正确性判断。

Existing-user 登录分支同样受上述 SMS limiter 约束，不能通过“不占 pending capacity”绕过短信预算。

---

## 7. User Authorization 也必须 Create-only / Exact-adopt

Registration 不调用会 Update/Reactivate 既有 Authorization 的高层 `EnsureProjectAuthorization`。

固定 Sumi Project + target Organization + target User 下：

```text
authorization absent
-> Create exact listingkit_admin authorization

exact roles + active
-> adopt/read-back

roles different
-> AUTHORIZATION_REPAIR_REQUIRED

inactive / revoked
-> AUTHORIZATION_REPAIR_REQUIRED
```

禁止 Registration Reconciler：

- 修改管理员并发调整后的角色集合；
- 自动重新激活已被管理员 revoke 的授权。

只复用 `internal/zitadelprovision` 的低层 search/create/read-back 能力。

---

## 8. Current Policy Version 使用数据库单一权威

`current_policy_version` 不再仅靠每实例环境变量作为业务授权依据。

增加数据库权威，例如：

```text
saas_policy_releases
- policy_code
- policy_version
- state: staged | active | retired
- activate_at
- version
PRIMARY KEY(policy_code, policy_version)
UNIQUE(policy_code) WHERE state='active'
```

业务 CurrentConsentPolicy 每次以数据库 active policy 为最终权威；允许短 TTL cache，但 cache 过期且数据库不可读时 fail closed，不回退到旧环境变量。

激活新版本采用单一数据库事务/CAS；混合版本 API 实例在 rolling deploy 期间也会读取同一个 active version，因此不能由旧 Pod 绕过 re-consent。

测试必须覆盖：旧/新应用实例并存、cache stale、policy activation concurrent requests。

---

## 9. 本轮新增验收矩阵

至少补以下测试：

```text
old verified session cannot prove a new registration attempt
proof is consumed once; replay rejected
reclaim transfers capacity slot; repeated reclaim cycles do not leak slots
crash in otp_verified/business_preparing/business_prepared is reconciled
reclaim vs deleting claim: exactly one wins
provider delete response loss enters outcome_unknown reconciliation
E.164 lookup errors/traces/proxy logs contain no phone
challenge resend hits per-session/phone/IP/global limiter
revoked/different user authorization is not mutated
mixed old/new API instances enforce one DB active policy version
```
