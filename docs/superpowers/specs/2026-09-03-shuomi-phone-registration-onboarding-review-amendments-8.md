# 硕米手机号注册与 Onboarding 第八轮评审修订

本文件针对 PR #283 在 `c77dd32` 上的新一轮 Code Review 结论继续收敛。与前序文档冲突时，以本文件与更新后的 `2026-09-03-shuomi-phone-registration-onboarding-plan-v2.md` 为准。

## 1. `/register` 必须在 lookup 前原子占用通用 Admission Slot

只做 `capacity available?` 检查仍存在 TOCTOU：两个并发请求都可能看到最后一个 slot，然后 existing 分支继续真实 OTP、unknown 分支在后续 allocation 失败，再次形成 existence oracle。

因此 `/register` 的第一条持久化动作改为：

```text
trusted ingress abuse checks
-> BEGIN TX
   lock registration admission capacity
   allocate one generic registration_admission_slot
   persist registration attempt/admission id
   in_use++
-> COMMIT
-> only then perform exact E.164 lookup
```

Admission Slot 在 lookup 之前不携带 existing/new 语义。

分流后：

- `new/pending Provider object`：同一 slot 转为 `unverified_provider_object` ownership；不得再申请第二个 slot；
- `existing active user`：同一 slot 继续保护本次 `/register` OTP attempt，直到 challenge 已确定创建、attempt 被取消/过期，或流程转入明确的 login continuation；随后按同一 bounded TTL/transaction 释放；
- 任意失败重放必须按 admission/registration logical key replay，不能重复 `in_use++`；
- lookup 前 allocation 失败时，不执行 lookup、不发 SMS、统一 generic unavailable。

测试：最后 1 个 slot 下 known/unknown/pending 三个并发请求，只能有获得 admission 的请求继续；未获得者都无法通过 challenge/resend 观察账号类别。

## 2. Capacity 使用三层权威：Global Provider Object + Admission Class + Verified Work Class

之前把 `unverified` 和 `verified onboarding` 作为两个互斥 hard pool，导致业务写入与 slot transfer 存在新的阻塞窗口。改为：

### 2.1 Global Provider Object High-water

所有尚未删除的自助注册 Provider User/Organization 占用同一个 global provider-object slot，直到：

- identity 被安全删除并 finality 确认；或
- identity 成为正式授权账号并从 registration pending accounting 转出。

这个 global slot 不因 class 转移而增减，防止 Provider object 数量失控。

### 2.2 Admission Class

`registration_admission/unverified` 只限制匿名 `/register` 流量。OTP proof 成功后，必须在同一事务把 slot class 从 `unverified` 转为 `verified_waiting`，即使 verified worker backlog 当前繁忙，也不得继续占匿名 admission 计数。

因此：

```text
OTP Verify accepted
-> lock proof + admission/global slot
-> state otp_verified
-> slot.class = verified_waiting
-> commit
```

这一步不创建任何业务资产，也不新增 Provider object。

### 2.3 Verified Onboarding Work Capacity

`verified_onboarding_active` 是执行 `Business Prepare` 的工作/backlog admission，不是 Provider-object 数量权威。

在第一次 non-disposable business write 之前：

```text
lock intent/global slot/verified work capacity
-> if active work capacity full:
     state = verified_waiting_capacity
     do not write Consent/projection/subscription
     release tx
-> else:
     acquire verified work lease/slot
     atomically write first non-disposable business state
     state = business_preparing
```

如果 concurrent external/business path 已经创建 paid/non-disposable artifact，则 Reconciler 只做 classification + verified work scheduling；该 identity 已是 `verified_waiting`，不会回占 anonymous admission pool，也不得 cleanup 删除。

`authorized` 后释放 verified work slot，并将 global registration Provider slot 转出 pending accounting。

## 3. Provider Operation 增加稳定 Logical Key

随机 `operation_id` 不能作为同一逻辑 Provider mutation 的幂等权威。

新增：

```text
logical_operation_key
operation_id                 // opaque instance ID，可 UUID
registration_id
ownership_epoch
kind
provider_target_id
request_fingerprint
```

数据库至少：

```text
UNIQUE (logical_operation_key)
```

推荐 logical key：

```text
provider-op:{registration_id}:{ownership_epoch}:{kind}:{provider_target_type}:{provider_target_id}
```

规则：

- same logical key + same fingerprint -> replay/adopt existing operation；
- same logical key + different fingerprint -> `PROVIDER_OPERATION_REPLAY_CONFLICT`；
- 不允许两个 worker 为同一 logical key 插入两个 prepared operation；
- Reclaim 新 ownership epoch 后允许产生新的 logical operation sequence；
- Delete/Create/marker repair 的 kind 必须固定枚举，不接受浏览器自定义。

## 4. RegistrationReconciler 必须接管 stale `prepared` / `inflight`

统一 owner 集合新增 Provider Operation 状态恢复：

```text
prepared
inflight
outcome_unknown
```

规则：

- `prepared` 且没有活跃 lease/尚未发出 Provider call：Reconciler 可按同 logical key 获取新 lease，继续原 operation；
- `inflight` lease 未过期：其他 worker 不接管；
- `inflight` lease 过期：不得直接重发。先 fenced CAS 为 `outcome_unknown`，再做 Provider finality/read-back；
- `outcome_unknown`：只有 finality/read-back 收敛后才能进入 succeeded/failed_definitive 或允许相反 mutation；
- Cleanup 对上述三个非 definitive 状态全部 fail closed。

必须测试 crash-before-call、crash-after-inflight-before-call、call sent + worker crash、stale worker late response。

## 5. `AddOTPSMS` 是 User Factor Write；新 Session 不能回滚未知 enrollment

`AddOTPSMS` 写入的是 `/v2/users/{userID}/otp_sms`，其副作用属于 User，不属于 Login Session。因此“response loss -> 换新 Session -> 再 AddOTPSMS”不是安全恢复。

Capability gate 必须证明至少一种：

1. Provider 提供 user OTP-SMS factor read-back，可在 timeout 后确认 factor 是否已安装；或
2. `AddOTPSMS` 对同一 User 已被 pinned/staging 实测为安全幂等，并有明确响应/错误合同。

只有满足其一才允许自动恢复。

否则：

```text
AddOTPSMS timeout/connection loss
-> factor_enrollment_outcome_unknown
-> 禁止盲目再次 AddOTPSMS
-> flow = repair_required / rollout fail closed
```

新建 Login Session 只用于 challenge/session 丢失恢复，不能被当成 User Factor rollback。

## 6. Proof 必须绑定 `CreateSMSChallenge` 实际返回的 Challenge-bearing Session

Pinned `phoneonboardingpreflight` 的 `CreateSMSChallenge` 自身通过 Session V2 创建 challenge-bearing Session，并返回该 Session ID/token。因此不能先把 proof attempt 绑定到一个“预创建 Session A”，随后调用 helper 又得到 Session B。

固定顺序改为：

```text
ensure AddOTPSMS factor definitive
-> create proof_attempt in challenge_preparing state（尚无 provider_session_ref）
-> CreateSMSChallenge using pinned request shape
-> provider returns challenge-bearing sessionID/sessionToken
-> persist hash/reference of THIS returned session under same proof_attempt
-> state challenge_ready
-> VerifySMS against this exact returned Session
-> proof verified
```

安全约束：

- task-processor 不持久化 plaintext sessionToken；Login V2 按其现有 credential handling 保存/持有必要 token；本地只保存 opaque/non-secret reference/hash 与 challenge generation；
- `CreateSMSChallenge` response loss 时，SMS 发送计入 rate-limit/global budget，attempt 进入 `challenge_outcome_unknown`；
- 如果 provider 不能恢复丢失的 challenge session credential，不立即自动 resend；用户只能在 cooldown 后显式请求新 challenge generation；旧 generation 永不作为 proof；
- 每个 Verify 必须匹配当前 `proof_attempt_id + challenge_generation + returned session reference`。

## 7. 第八轮验收矩阵

至少新增：

```text
last admission slot：known/unknown 并发，lookup 前只有一个获得原子 slot
same admission retry 不二次 in_use++
OTP verified 后 slot class 原子 unverified -> verified_waiting
verified worker capacity full：不写任何 business artifact，也不重新占 anonymous admission
paid artifact concurrent 出现：identity 保持 verified class，禁止 cleanup
same Provider logical key 两 worker -> 只一个 prepared row / 一个 external mutation
same logical key different fingerprint -> conflict
stale prepared -> reconciler resume
stale inflight -> outcome_unknown -> read-back，禁止 blind retry
AddOTPSMS timeout + factor read-back installed -> continue without duplicate write
AddOTPSMS timeout 无 read-back/idempotency proof -> fail closed
CreateSMSChallenge 返回 Session B -> proof 必须绑定 B
challenge response loss -> no immediate uncontrolled resend
```

## 8. IAM 边界不变

这些修订没有把 OTP/Session/Password/OIDC 搬回 task-processor：

- OTP factor、challenge、Verify、Session 继续由 ZITADEL/Login V2 负责；
- task-processor 只保存 Registration/Provider operation 的非凭据状态与业务 Onboarding 投影；
- 不新增本地 OTP code、password hash、Session Token 持久化或第二套 IAM。
