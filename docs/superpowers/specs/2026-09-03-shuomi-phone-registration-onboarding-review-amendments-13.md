# 硕米手机号注册与 Onboarding 第十三轮评审修订

本文件针对 PR #283 V6 最新 Code Review 的 4 个 P2 继续收敛。与前序文档冲突时，以本文件和 `2026-09-03-shuomi-phone-registration-onboarding-plan-v7.md` 为准。

## 1. Active Phone Correlator 密钥轮换必须保持跨版本唯一

`active_phone_correlator` 仍只用于短生命周期并发 fence，不升级为长期 PhoneIdentity。

普通轮换禁止“旧 active claim 未过期时直接切新 primary key”。固定为 staged rotation：

1. 先把 next key 分发到所有实例；
2. 旧 key 继续作为唯一写入 primary；
3. 等待最长 active correlator TTL（72h）并确认旧版本 active claim 为 0；
4. 才把 next key 提升为 primary；
5. 旧 key 再保留一个短 read-only retirement window 后销毁。

因此正常轮换期间数据库不会同时存在同一手机号由两个 primary key 产生的 active claim。

紧急密钥泄露时不得直接双写/切换绕过唯一性；应先全局关闭 self-registration，终止或人工迁移仍存活的 active attempts，确认旧 claim 已不可执行，再启用新 key。

验收必须覆盖：rolling deploy、old-key active claim、new-key activation、emergency rotation，确保同 E.164 不能因 key version 变化创建第二个 active attempt。

## 2. Phone-bearing Request Fingerprint 也是短生命周期数据

Admission `request_fingerprint` 中含手机号派生 HMAC，因此不能在 Intent 历史记录里永久保留。

新增生命周期：

```text
attempt_payload_fingerprint
fingerprint_key_version
fingerprint_retention_until
attempt_closed_at
```

规则：

- active / retry / response-loss replay window 内保留 fingerprint；
- terminal 后只保留一个很短的 terminal replay window；
- replay window 到期后必须 scrub `attempt_payload_fingerprint` 与 key version；
- `logical_attempt_key` 进入 closed/tombstoned 状态，后续同 key 请求只返回 branch-neutral `REGISTRATION_ATTEMPT_CLOSED`，不再依赖手机号 fingerprint 比对；
- terminal immutable result 可以保留，但不得包含 raw phone 或 phone-derived stable alias。

因此 idempotency 只覆盖需要的活动/重放窗口，不把手机号 HMAC 变成长期可关联标识。

## 3. Raw Phone Retention 与 Active Concurrency Fence Retention 分离

24h 是 raw/recoverable phone ciphertext 的目标保留期，不是 active correlator 的强制删除时间。

规则改为：

```text
phone_ciphertext <= 24h ordinary pending
active_phone_correlator <= active-attempt hard TTL (max 72h)
```

如果 attempt 在 24h 时仍未完成：

- 立即 scrub `phone_ciphertext`；
- 但只要 attempt 仍可执行，就继续保留 active correlator；
- 到 72h hard TTL 前必须先把 attempt 原子推进到 `expired_fenced` / terminal repair 状态，撤销 execution lease，并禁止新的 Provider write；
- 只有确认旧 attempt 已不可再次执行后，才 scrub active correlator/fingerprint。

新 attempt 若之后重新输入同手机号，可以重新开始 branch-neutral admission；旧 `expired_fenced` attempt 不能恢复执行。若 lookup 命中旧 pending Provider identity，则只能走新的 reclaim/finality 规则，不能让旧 attempt 复活。

## 4. 公共 Registration Capacity 改为固定 Epoch Budget，不按分支/单条结果回收

V6 的 per-attempt fixed hold 仍可能在 hold 到期后因 branch/finality 不同产生容量侧信道。V7 删除“单条 attempt 到时释放后立刻增加公共可用容量”的语义。

公共 `/register` admission 使用 **fixed Safety Admission Epoch**：

```text
epoch_id
epoch_started_at
epoch_ends_at
configured_attempt_budget
consumed_attempts
```

一个 epoch 内：

- 每个被接受的 attempt 永久消耗该 epoch 一个 quota；
- existing / pending / new / success / failure 都不返还 quota；
- 单条 attempt 的 cleanup/authorized/finality 不改变本 epoch 外部可观察的剩余 quota；
- epoch 结束前禁止基于某个用户分支“腾回一个名额”。

新 epoch 的 budget 是预先固定、worst-case sizing 的控制面配置：必须按“本 epoch 所有 attempt 都可能 genuinely-new，并在最大安全 cleanup/finality 窗口内占用 Provider object”计算，保留足够 Provider headroom + safety margin。

若 Provider cleanup/finality SLO 失效或 unresolved inventory 超过安全水位：

```text
self-registration rollout/next epoch = gated off
```

但不能在当前 epoch 因某个 attempt 的 branch 立即增减公共 quota。恢复也只在下一个明确 epoch/control-plane activation 生效。

这把账号存在性从“某个 victim 是否释放 slot”中解耦；容量变化只发生在固定 epoch 边界，而不是单条注册结果边界。

## 5. Provider Inventory 仍需独立真实高水位

Epoch budget 不是 Provider inventory 的替代品。内部仍持续统计：

- unresolved Create/marker/Grant/Auth/Delete/Reclaim operations；
- pending Provider User/Organization；
- repair/quarantine inventory。

但 inventory 只能影响 **下一 epoch 是否开启及其固定 budget**，不能在当前 epoch 对外暴露 per-attempt branch-specific capacity delta。

如果自动 destructive cleanup 的 atomic ownership precondition 仍不能在 pinned ZITADEL 证明，则 self-registration 继续 rollout gated/off，不因为引入 epoch budget 而放宽这一条件。

## 6. 第十三轮验收

至少新增：

```text
old key active claim + staged key rotation -> same phone cannot open second attempt
emergency key rotation -> self-registration off until old active claims fenced
terminal attempt fingerprint replay window expires -> phone-derived fingerprint scrubbed
24h raw phone purge while active attempt -> correlator remains, duplicate phone still blocked
72h hard TTL -> attempt first expired_fenced, then correlator scrub
same phone after old expired_fenced -> new attempt allowed, old attempt cannot resume
within one Safety Admission Epoch existing/new/pending all consume exactly one non-refundable quota
victim attempt outcome cannot increase current-epoch available quota
next epoch budget published only by control plane/worst-case sizing
unresolved provider inventory over watermark -> next epoch gated off, not per-attempt immediate slot behavior
```

## 7. IAM 边界不变

ZITADEL/Login V2 继续拥有 OTP、Factor、Password、Session、OIDC；本轮只修 Registration admission/privacy/finality，不引入长期 PhoneIdentity 或本地 OTP。