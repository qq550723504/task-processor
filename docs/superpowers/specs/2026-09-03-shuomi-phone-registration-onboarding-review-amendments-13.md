# 硕米手机号注册与 Onboarding 第十三轮评审修订（历史）

> **状态：SUPERSEDED / HISTORICAL ONLY**
>
> 本文件记录 PR #283 V6→V7 期间针对旧 anti-enumeration 目标的评审背景，不再是实施权威。唯一执行入口是：
>
> `docs/superpowers/plans/2026-09-03-shuomi-phone-registration-onboarding-plan-v7.md`
>
> 与 V7 冲突时必须忽略本文件，不得从本文件恢复已经取消的机制或 acceptance tests。

## 1. 仍被 V7 保留的历史结论

以下问题仍有工程价值，但其最终合同已经直接写入 V7：

- active-phone correlator 只用于短生命周期并发防重；
- Phase1 key rotation 使用 maintenance gate，不要求零停机 rolling rotation；
- phone ciphertext、phone-derived request fingerprint 与 Provider Operation fingerprint 都有 bounded retention；
- 旧 attempt 在 hard TTL 后必须先 fenced/terminal，再清除 concurrency correlator；
- ZITADEL/Login V2 继续拥有 OTP、Factor、Password、Session、OIDC。

这些规则只能按 V7 当前文本实施，不再把本文件当作第二份规范。

## 2. 已明确撤销：SafetyAdmissionEpoch / Branch-neutral Capacity

本文件旧版本曾要求：

- fixed `SafetyAdmissionEpoch`；
- existing/pending/new 在公共容量上不可区分；
- 单条 attempt 不得影响当前/下一 epoch 可用 quota；
- control-plane worst-case epoch budget；
- cross-epoch capacity side-channel 防护。

**这些要求全部撤销。**

2026-09-03 产品决策明确允许注册入口提示手机号是否已经注册，Account Existence 不再属于 Phase1 需要隐藏的安全信息。因此为了隐藏 existing/new 而引入的 branch-neutral capacity、SafetyAdmissionEpoch、cross-epoch debt 与相关 control-plane 发布机制均不属于当前产品合同。

Phase1 采用真实容量保护：

```text
existing_active -> 不占新的 Provider-object capacity
registration_owned_pending -> 复用已有 pending-object accounting
not_found -> 原子申请真实 pending Provider-object capacity
```

容量结果可以因分支不同而不同；真正需要保护的是重复 Provider identity、容量泄漏、不可恢复外部副作用与注册可用性，而不是账号存在性侧信道。

## 3. 已撤销的验收项

以下历史测试不得再作为 blocker 或重新写回 V7：

```text
within one Safety Admission Epoch existing/new/pending all consume exactly one non-refundable quota
victim attempt outcome cannot increase current-epoch available quota
next epoch budget published only by control plane/worst-case sizing
cross-epoch admission remains account-existence indistinguishable
```

如果 Reviewer 再提出仅用于隐藏账号存在性的 branch-neutral / cross-epoch finding，应依据当前 Product Decision 分类为 `NOT_APPLICABLE` 或 Accepted Risk，而不是增加新架构。

## 4. 当前实现边界

真正仍需阻塞实现的问题以 V7 为准，包括但不限于：Provider ownership adopt 校验、registration-owned pending reclaim、AddOTPSMS unknown outcome、Provider Operation durable recovery、Consent freshness、Business Ownership Fence、Cleanup finality 与 authorized/pending-capacity 原子释放。

本文件不再新增或修改实施要求。