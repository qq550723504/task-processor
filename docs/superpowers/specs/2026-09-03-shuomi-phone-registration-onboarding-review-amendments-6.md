# 硕米手机号注册与 Onboarding 第六轮评审修订

> 本文覆盖此前设计、amendments 1-5 与 Implementation Plan 中的冲突项。边界保持不变：本 PR 只处理手机号注册与首次 Onboarding；OTP、Password、Session、OIDC 继续由 ZITADEL/Login V2 负责，task-processor 不实现第二套身份系统。

## 1. Proof Acceptance 必须 insert-only，重放时校验完整不可变 payload

`saas_registration_proof_acceptances` 以 `proof_attempt_id` 作为一次性事实主键，但主键唯一不足以定义重放语义。

固定规则：

```text
first insert:
  proof_attempt_id
  registration_id
  provider_user_id
  policy_version
  accepted = true
  accepted_at_server
  source = login_v2_verify_action

same proof_attempt_id retry:
  SELECT existing
  compare every immutable field
  exact match -> replay existing acceptance
  any mismatch -> ACCEPTANCE_REPLAY_CONFLICT
```

禁止 `UPSERT DO UPDATE`，禁止把旧 v1 acceptance 更新成 v2。`accepted_at_server` 第一次写入后不可变化。

Consent 事务仍要求 proof + acceptance 同 attempt / registration / user / current policy，并一起一次性 consume。

## 2. Provider mutation lease takeover 只转移“协调权”，不能越过未知外部副作用

数据库 epoch 可以 fence 本地 late writer，但不能撤销已经发给 ZITADEL 的旧 Delete/Reclaim HTTP 请求。

因此 claim 过期后的 Reconciler takeover 规则改为：

```text
stale reclaiming/deleting claim
-> lock row, epoch+1, acquire reconciliation ownership
-> mark provider_mutation_state = outcome_unknown
-> only perform provider read-back / operation-status reconciliation
-> DO NOT issue the opposite mutation while old outcome is not definitive
```

若 Provider 支持可验证的 idempotency key / operation status / conditional mutation，则使用该能力确定 final outcome；否则必须证明 Provider 请求执行与可见性存在经过测试的有限 finality bound，并在该窗口结束后重复 read-back。

在旧 Delete 仍可能延迟执行的窗口内，禁止 Reclaim；在旧 Reclaim 仍可能延迟执行的窗口内，禁止 Delete。

如果 pinned ZITADEL 无法提供可证明的 mutation finality，自动 Provider Delete 不得作为生产自助注册依赖，手机号自助注册 rollout 必须 fail closed。

## 3. `consent_required` 属于 RegistrationReconciler 的正式 post-proof 状态

统一 post-proof owner 增加：

```text
otp_verified
consent_required
business_preparing
business_prepared
project_grant_ready
authorizing
```

`consent_required` 保存 `consent_required_at / consent_deadline_at`。用户可在有效 Auth Session 或重新建立合法 Login V2 Session 后接受数据库当前 policy；不复用旧 acceptance。

超过 `consent_deadline_at` 且仍未取得当前 Consent：

```text
no business authorization exists
-> transition cleanup_requested
-> bounded cleanup pipeline
-> provider deletion finality confirmed
-> release capacity slot
```

不得让 abandoned `consent_required` 永久占用 pending capacity。

## 4. `business_prepared` 前必须验证当前 Subscription entitlement projection 完整

`EnsureInitialPlanIfAbsent(base_payg)` 返回“已有 paid plan”并不代表该 plan 的 entitlement rows 已经完整；现有 `ApplyPlan` 可能在 subscription row 与 entitlement writes 之间崩溃。

在 `business_preparing -> business_prepared` 前增加 listingsubscription authority 内的幂等入口：

```text
EnsureCurrentPlanEntitlementsReady(tenantID)
```

语义：

1. 读取并锁定当前 subscription/version；
2. 根据 canonical plan 定义计算 required entitlements；
3. 只补齐缺失 projection，不改变 plan，不降级、不覆盖付费计划；
4. read-back plan/version + required entitlement rows；
5. 若并发 plan 变化，有限重试并以最新 plan 为准。

Onboarding 只有在 current plan 与 entitlement projection 一致后才可写 `business_prepared`。

测试必须覆盖：paid `ApplyPlan` 写入 subscription 后、entitlement 前崩溃；随后 Registration Prepare 能修复 projection，但不会覆盖 paid plan。

## 5. 安全的有界 Quarantine Cleanup 是手机号自助注册 Rollout Stop Condition

不再支持“自动 Provider Delete 永久关闭 + quarantined slot 永久计费”作为生产模式。该模式允许分布式攻击者最终耗尽全局 admission capacity。

生产开启手机号自助注册前必须验证完整 bounded cleanup contract：

```text
pre-authorization Provider object 有确定 TTL
cleanup credential 只能作用于本 Registration ownership
Delete/Reclaim mutation finality 可证明
cleanup worker 有 durable scan + lease + retry budget
provider absence read-back 后才 release capacity slot
最坏 cleanup latency 有明确 SLO/告警
```

如果上述任一项无法在 pinned ZITADEL / deployment boundary 中证明，手机号自助注册功能保持关闭；不能把无限期人工清理作为正常容量恢复机制。

人工 repair 仍可处理异常 ownership mismatch，但必须有独立告警与高水位保护，不能成为攻击流量的主 cleanup 路径。

## 6. 新增验收测试

```text
same proof_attempt exact acceptance replay -> success
same proof_attempt changed policy/user/registration -> ACCEPTANCE_REPLAY_CONFLICT
old delete request timeout then delayed execution -> reclaim remains fenced
mutation lease takeover -> only reconciliation until definitive outcome
consent_required browser abandonment -> deadline cleanup -> slot released
consent_required restart/resume -> current policy acceptance only
paid plan subscription row durable but entitlement write missing -> Prepare repairs projection
quarantine flood -> bounded cleanup keeps admission capacity recoverable
cleanup finality unavailable -> registration rollout stays disabled
```
