# 硕米手机号注册与 Onboarding 第五轮评审修订

**适用 PR：** #283

**覆盖关系：** 本文在冲突处覆盖前序手机号注册 / Onboarding 设计与评审修订。

**边界不变：** ZITADEL/Login V2 继续负责 OTP、Password、Session、OIDC；task-processor 不接收/验证 OTP code 或 Password，也不实现第二套 IAM。

---

## 1. Agreement Acceptance 必须与本次 Proof Attempt 同一次用户动作绑定

`presented_policy_version` 只表示页面展示了哪个版本，不能作为 Consent 证据。

注册 UI 可以继续在手机号/验证码页面展示协议勾选框，但权威接受事件只能在**本次 OTP Verify 的 Login V2 Server Action** 中产生：

```text
用户输入 OTP + 勾选当前协议
        ↓
Login V2 Server Action
        ├─ ZITADEL Verify OTP
        └─ Verify 成功后，使用同一 proof_attempt_id 记录 acceptance
                ↓
task-processor Internal Registration API
```

服务端持久化：

```text
saas_registration_proof_acceptances
- proof_attempt_id PRIMARY KEY
- registration_id
- provider_user_id
- policy_version
- accepted_at   // server time after successful proof
- consumed_at NULLABLE
```

硬约束：

- OTP Verify 失败 -> 不得生成 acceptance；
- acceptance.policy_version 必须等于提交瞬间 DB `CurrentPolicyVersion`；
- Consent transaction 同时锁定 proof + acceptance：二者必须属于同一 `proof_attempt_id / registration_id / provider_user_id`；
- proof 与 acceptance 一起一次性消费；
- 旧 attempt 的 acceptance 不能被新 attempt 使用；
- pre-proof intent 中不再保存权威 `consent_accepted_at`；最多保存非权威的 `presented_policy_version`。

这样不会把攻击者在 victim OTP proof 之前做出的勾选转换成 victim Consent，同时不要求 task-processor 处理 OTP 本身。

---

## 2. Provider Mutation Claim 必须有 Lease + Epoch Takeover

`provider_mutation_state` 不能由死亡 worker 永久占有。

增加：

```text
provider_mutation_state
provider_mutation_epoch BIGINT
provider_mutation_owner
provider_mutation_lease_until
provider_mutation_last_heartbeat_at
```

获取规则：

```text
none
-> CAS to reclaiming/deleting, epoch+1, owner, lease_until

active claim + lease valid
-> 其他 worker fail closed

active claim + lease expired
-> RegistrationReconciler 锁行
-> CAS epoch+1 / new owner
-> 先 Provider read-back
-> 再决定继续、完成、quarantine 或 outcome_unknown
```

旧 worker 的 Provider 返回晚到时必须比较 epoch；epoch 已变化则不得再修改本地状态。

Provider 调用本身继续使用短 deadline，lease 必须大于单次 provider-call deadline 并支持受控 heartbeat；不能通过无限续租隐藏卡死 worker。

测试至少覆盖：

```text
crash before Provider call
crash during Provider call
old worker late response after lease takeover
reclaiming stale claim takeover
deleting stale claim takeover
```

---

## 3. Provider Organization Name 使用持久化 Opaque Contract

当前 pinned ZITADEL Create Organization 需要非空 name，因此 Registration Intent 在第一次 Provider Create 前同时固定：

```text
provider_organization_id
provider_organization_name
```

名称使用不含手机号/邮箱/业务敏感信息的 opaque convention，例如：

```text
lk-<opaque-id>
```

`opaque-id` 从 registration/provider organization ID 做稳定编码或首次生成后持久化；重试、lookup/adopt 永远复用同一个 name，不临时再生成。

禁止：

```text
+8613...
phone-...
手机号后四位
technical email local-part
```

Create/Get/Adopt capability test 必须验证：

- Organization Create request 的完整必填字段；
- stable ID + stable opaque name；
- response loss 后 Get 同 ID 时 name 完全匹配；
- name mismatch -> quarantine/repair，不静默改名。

---

## 4. Project Grant Exact-adopt 必须同时要求 ACTIVE

Registration 的 non-mutating Project Grant contract 修正为：

```text
Grant absent
-> Create exact fixed Sumi Project Grant

roles exact AND state ACTIVE
-> adopt/read-back

roles different
-> PROJECT_GRANT_REPAIR_REQUIRED

state inactive/deactivated/revoked
-> PROJECT_GRANT_REPAIR_REQUIRED
```

Registration/Onboarding 不得为了继续注册而调用 Update/Reactivate Project Grant。

这与 User Authorization 的 create-only/exact-active-adopt 规则一致，保留管理员显式撤销/停用的意图。

---

## 5. 实施计划必须直接采用全部修订

`docs/superpowers/plans/2026-09-03-shuomi-phone-registration-onboarding-plan.md` 本轮同步重写关键任务和权威输入。

执行实现时必须读取：

```text
原始 design
review-amendments 1
review-amendments 2
review-amendments 3
review-amendments 4
review-amendments 5
```

后续 amendment 在冲突处覆盖前序内容；Implementation Plan 不得继续执行已被废弃的 Workbench-owned migration、pre-proof consent、旧授权入口或 per-instance policy 配置。

---

## 6. 本轮新增验收矩阵

```text
agreement unchecked -> OTP proof succeeds but no Consent/authorization
agreement checked + OTP verify fails -> no acceptance record
proof A + acceptance B -> reject
old acceptance replay in new attempt -> reject
mutation owner crashes before/during provider call -> reconciler lease takeover
late old worker cannot overwrite newer mutation epoch
organization name is non-empty, deterministic/immutable and contains no phone digits
inactive exact-role Project Grant -> repair_required, never reactivate
implementation plan references and follows amendments 1-5
```
