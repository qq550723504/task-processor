# 硕米手机号注册与 Onboarding Implementation Plan

**目标：** 在不复制 ZITADEL IAM 能力的前提下，补齐手机号自助注册所需的短生命周期 Registration Intent、受控 Provider Provisioning、Consent、`base_payg` 和最后业务授权。

## 权威输入

实现前必须同时读取以下文档；后续 amendment 在冲突处覆盖前序内容：

1. `docs/superpowers/specs/2026-09-03-shuomi-phone-registration-onboarding-design.md`
2. `docs/superpowers/specs/2026-09-03-shuomi-phone-registration-onboarding-review-amendments.md`
3. `docs/superpowers/specs/2026-09-03-shuomi-phone-registration-onboarding-review-amendments-2.md`
4. `docs/superpowers/specs/2026-09-03-shuomi-phone-registration-onboarding-review-amendments-3.md`
5. `docs/superpowers/specs/2026-09-03-shuomi-phone-registration-onboarding-review-amendments-4.md`
6. `docs/superpowers/specs/2026-09-03-shuomi-phone-registration-onboarding-review-amendments-5.md`

本计划已经按上述修订重新收敛。禁止回退到以下已废弃方案：

```text
Workbench-owned registration migration
pre-proof consent_accepted_at 作为 Consent 证据
PhoneIdentity / Phone HMAC Alias 长期目录
task-processor OTP/Password 校验
Decoy User
自研 OIDC Callback Runtime
登录 Temporal Workflow
会修改/重新激活既有 Project Grant 或 User Authorization 的 Ensure helper
per-instance current_policy_version 作为授权权威
```

---

## Task 1：固定 ZITADEL / Login V2 能力合同

**输出：** `docs/verification/zitadel-registration-capabilities.md` + 可重跑验证脚本。

必须验证当前 pinned / production-compatible ZITADEL：

```text
E.164 canonical login-name 唯一性
exact global login-name lookup
Organization caller-supplied ID
Human User caller-supplied ID
Get Organization/User by exact ID
完整 Create Organization 必填字段
完整 Create Human User 必填字段
Session OTP SMS challenge / verify
Project Grant search/create/read-back
User Authorization search/create/read-back
```

Organization 必须使用持久化 opaque name：`lk-<opaque-id>`，不得包含手机号。

Human User 继续复用 PR #218 已验证的 opaque verified Technical Email 合同，不含手机号数字，不建立邮件恢复路径。

E.164 lookup 优先 body-based exact lookup；若只能 query lookup，则使用专用 redacted client，错误、trace、proxy/Ingress log 不得记录 query/手机号。无法证明无明文泄露时 rollout fail closed。

**Stop condition：** stable ID、exact lookup、完整 Create contract 或安全日志边界任一不成立，不进入生产自助注册实现。

---

## Task 2：建立独立 Registration Provisioning Migration Scope

新增独立：

```text
internal/registrationprovisioning/schema/runtime.go
```

由应用 migration composition 注册独立 scope；`internal/workbench/schema/runtime.go` 不拥有 Registration / Consent / Onboarding 表。

核心表至少包括：

```text
saas_registration_intents
saas_registration_capacity
saas_registration_proof_attempts
saas_registration_proof_acceptances
saas_account_consents
saas_onboarding_preparations
saas_policy_releases
```

Registration Intent 首次 Provider Write 前固定：

```text
registration_id
normalized_phone              // 短生命周期，E.164
provider_organization_id
provider_organization_name    // opaque immutable
provider_user_id
presented_policy_version      // 非 Consent 证据
capacity_slot_id
state
cleanup_state
provider_mutation_state
provider_mutation_epoch
provider_mutation_owner
provider_mutation_lease_until
expires_at
created_at / updated_at
```

不再持久化 pre-proof `consent_accepted_at` 作为权威 Consent。

手机号不进入应用日志、trace attribute 或审计 payload；Intent 完成后按 retention 删除/脱敏。

---

## Task 3：实现数据库原子 Pending Capacity Admission

Provider Create 前：

```text
lock capacity row
-> verify in_use < max_allowed
-> allocate capacity_slot_id
-> insert Registration Intent
-> in_use++
-> COMMIT
```

容量按 unresolved Provider identity 计数，不按浏览器 attempt 计数：

- quarantined / unresolved Provider object 持续占用 slot；
- reclaim 只把旧 slot 原子转移给新 ownership epoch，不再 +1；
- `authorized` 转换与 `in_use--` 同一 PostgreSQL 事务；
- Provider 确认删除后才允许 unresolved slot 释放。

同时保留 Ingress trusted-IP 限流、同手机号 active intent 约束、Provisioning Worker 并发上限。

测试多实例 admission、repeated abandon/reclaim、authorized replay，确保 capacity 不漂移。

---

## Task 4：实现最窄 Provider Provisioning Adapter

Login Fork 不持有广权限 ZITADEL management credential；只调用 task-processor Internal Registration API。

后端 Adapter 使用已持久化 stable IDs / opaque Organization name：

```text
Get exact ID
├─ exact expected object -> adopt
├─ absent -> Create same ID/name
├─ missing marker but exact expected attributes -> adopt_missing_marker + repair marker
└─ mismatch -> quarantine / repair_required
```

Provider Create response loss：始终先 Get 同 ID，不生成新 ID。

所有 Provider 调用必须有：

```text
request context deadline
HTTP client timeout
bounded retry budget
logical total deadline
```

stall/cancel 必须释放 worker slot；unknown outcome 进入 read-back reconciliation。

---

## Task 5：Login V2 Fork 手机注册 + Attempt-bound OTP Proof

Login Fork 只定制 UI 和注册分支；OTP/Password/Session/OIDC 仍由 ZITADEL/Login V2 完成。

每次 OTP registration attempt：

```text
proof_attempt_id
registration_id
provider_user_id
provider_session_ref_hash
challenge_created_at
proof_verified_at
proof_consumed_at
```

浏览器输入手机号、OTP 并勾选当前协议。权威顺序：

```text
Login V2 Server Action
1. 调 ZITADEL Verify OTP
2. Verify 成功后记录本 proof_attempt 的 acceptance
3. acceptance.policy_version 必须等于 DB CurrentPolicyVersion
4. 调 Internal Registration API 推进 Onboarding
5. 最终继续官方 CreateCallback / OIDC
```

OTP Verify 失败不得产生 acceptance。

Consent 写入时必须在同一 DB 事务中锁 proof + acceptance，并要求二者同 `proof_attempt_id / registration_id / provider_user_id / current policy`，随后一起一次性 consume。

旧 Session、旧 proof、旧 acceptance 不能跨 attempt 使用。

---

## Task 6：验证/补齐 OTP Challenge 防滥用，但不重写 OTP

预发布必须验证 ZITADEL/Login V2 或其最窄前置层具备：

```text
per-session resend cooldown
per-phone rolling limit
per-IP rolling limit
global SMS send budget / circuit breaker
verification attempt limit
```

existing-user OTP login 与 new/pending registration 使用同一安全预算原则。

若 ZITADEL 原生策略不足，只在 Login Fork / Ingress 的 Challenge 调用前补 limiter；task-processor 不读取 OTP code，不判断验证码正确性。

---

## Task 7：把 `base_payg` 纳入现有 listingsubscription 权威

在 `internal/listingsubscription` 增加：

```go
PlanBasePayg = "base_payg"
```

DefaultPlans 至少包含：

```text
ModuleStoreManagement
store_count = 1
```

新增数据库原子：

```go
EnsureInitialPlanIfAbsent(ctx, tenantID, PlanBasePayg, actorID)
```

语义：无 subscription 才初始化 `base_payg`；任何已有 basic/professional/enterprise 都不覆盖。

测试：并发付费 plan 与 initial base_payg、StoreQuota 第 1 家允许/第 2 家拒绝。

---

## Task 8：Onboarding Prepare 只处理硕米业务投影

OTP proof + acceptance 成功后：

```text
otp_verified
-> business_preparing
-> persist current Consent
-> Ensure Business User Projection
-> Ensure Business Organization Projection
-> EnsureInitialPlanIfAbsent(base_payg)
-> business_prepared
```

本地步骤使用短事务 / 可重入 Ensure，不跨 ZITADEL 持有 DB transaction。

`saas_account_consents` 以 `(zitadel_user_id, policy_version)` 版本化；新政策版本新增记录，不覆盖旧记录。

---

## Task 9：Project Grant 与 User Authorization 都必须 Non-mutating

### Project Grant

固定 Sumi Project + target Organization：

```text
absent -> Create exact allowed role set
roles exact AND state ACTIVE -> adopt/read-back
roles different -> PROJECT_GRANT_REPAIR_REQUIRED
inactive/deactivated/revoked -> PROJECT_GRANT_REPAIR_REQUIRED
```

禁止 Registration 调 Update/Reactivate Grant。

### User Authorization

固定 User + Organization + Project + `listingkit_admin`：

```text
absent -> Create
exact roles AND ACTIVE -> adopt/read-back
roles different -> AUTHORIZATION_REPAIR_REQUIRED
inactive/revoked -> AUTHORIZATION_REPAIR_REQUIRED
```

禁止 Registration 覆盖管理员并发角色修改或重新激活已撤销授权。

成功 read-back 后：

```text
business_prepared
-> project_grant_ready
-> authorizing
-> authorized
```

`authorized` 是 Registration Intent 终态；OIDC callback 不回写 Registration 状态。

---

## Task 10：RegistrationReconciler 收敛所有 Post-OTP 状态

统一 owner：

```text
otp_verified
business_preparing
business_prepared
project_grant_ready
authorizing
```

Reconciler 根据本地 DB + Provider read-back 幂等推进，使用 bounded backoff / deadline；无法自动安全收敛时进入明确 `repair_required`，不无限重试。

不引入 Temporal。

Provider mutation claim：

```text
state = none | reclaiming | deleting | outcome_unknown
epoch
owner
lease_until
last_heartbeat_at
```

claim lease 过期后，Reconciler 可锁行、epoch+1 接管；旧 worker 晚到响应若 epoch 已变化不得写状态。

---

## Task 11：Cleanup / Reclaim 与 Provider Delete 串行

Pre-proof Janitor 负责 cleanup；post-proof 由 RegistrationReconciler 负责。

Reclaim 与 Delete 使用互斥 mutation claim：

```text
none -> reclaiming
none -> deleting
```

不能同时成功。

自动 Provider Delete 只有在 cleanup credential 的跨租户负向权限测试证明安全时启用；否则 quarantine + 受控运维清理。

过期 pending identity 的合法手机号所有者可用**新的 attempt-bound ZITADEL OTP Proof** reclaim；不得依赖旧 proof/session。

---

## Task 12：Current Policy Version 使用数据库单一权威

`saas_policy_releases` 管理 `staged | active | retired`；同 policy code 只能一个 active version。

CurrentConsentPolicy：

```text
身份认证
-> live organization resolution
-> 读取 DB active policy（允许短 TTL cache）
-> 检查用户 current consent
-> 再进入 tenant business handler
```

cache 过期且 DB 不可读时 fail closed，不回退到旧环境变量。

Re-consent / login / logout / recovery 路由显式豁免 Consent Gate，避免死锁。

该 Gate 必须覆盖直接业务 API，不能只保护 Workbench 页面入口。

---

## Task 13：Legacy Backfill 与 Readiness Gate

Gate 默认关闭：

```text
部署 schema/readiness
-> 可重跑历史 Organization/Subscription reconciliation
-> 核对 live ZITADEL authorization
-> 标记 legacy readiness
-> 抽样核验
-> 开启 Gate
```

不改变历史 plan，不伪造历史 Consent；`legacy_consent_unknown` 后续通过独立 CurrentConsentPolicy 收集。

---

## Task 14：内部服务凭据和权限负向测试

Login Fork 只持有窄 `SHUOMI_REGISTRATION_CLIENT_TOKEN`；receiver 支持 current/previous overlap，按：

```text
receiver(new+old)
-> caller(new)
-> old caller drain
-> receiver(new only)
```

ZITADEL Login credential、Registration Provisioning credential、Cleanup credential 分开。

Provider 权限负向测试覆盖：

```text
无关 Organization/User 不可被修改/删除
客户端不能指定任意 Project/Role
Registration 不得 Update/Reactivate 已存在 Grant/Authorization
Cleanup credential 不得删除活跃无关企业
```

---

## Task 15：故障 / 并发 / 隐私验收矩阵

至少覆盖：

```text
DB Intent commit 后、Provider call 前崩溃
Org 创建成功但响应丢失
User 创建成功但 marker 未补写
同手机号多实例并发注册
pending capacity 多实例并发 admission
OTP Challenge resend abuse / global SMS budget
old proof/session/acceptance 跨 attempt replay
agreement unchecked / OTP fail 不生成 Consent
proof + acceptance 不同 attempt 拒绝
post-OTP 每个状态 crash 后 Reconciler 收敛
mutation claim owner crash + lease takeover + old worker late response
reclaim vs delete race
repeated abandon/reclaim 不泄漏 capacity slot
E.164 不出现在 error/log/trace/proxy query
opaque Organization name 不含手机号且 response-loss adopt 一致
base_payg vs paid plan 并发
inactive/mismatched Project Grant 不被修改/激活
inactive/mismatched User Authorization 不被修改/激活
mixed-version API Pod 使用同一 DB active policy
legacy backfill 可重跑
内部 token rolling rotation
```

---

## 完成定义

- [ ] task-processor 没有 OTP/Password/OIDC 认证实现。
- [ ] Provider Write 前 stable ID + opaque Organization name 已服务端持久化。
- [ ] exact E.164 lookup 的 error/log/trace/proxy 无手机号明文。
- [ ] OTP Proof 与 Agreement Acceptance 同一 registration attempt 绑定并一次消费。
- [ ] Pending Provider Object 有数据库原子容量上限；reclaim 转移 slot，authorized 释放 slot。
- [ ] 所有 post-OTP 状态都有 RegistrationReconciler owner。
- [ ] Provider mutation claim 可 lease takeover，不会因 worker crash 永久卡死。
- [ ] `base_payg` 属于现有 listingsubscription，initial plan 不覆盖历史 plan。
- [ ] Project Grant / User Authorization 均为 create-only / exact-active-adopt。
- [ ] Current Policy Version 使用数据库单一权威，业务 API 强制 CurrentConsentPolicy。
- [ ] Registration schema 有独立 migration scope，Workbench-only rollout 不修改它。
- [ ] 广权限 Provider credential 不存在于互联网 Login Fork。
