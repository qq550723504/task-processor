# 硕米手机号注册与 Onboarding 第九轮评审修订

本文件针对 PR #283 在 `05216cd` 上完成的最新 Code Review / Security Review 继续收敛。与前序文档冲突时，以本文件和 `2026-09-03-shuomi-phone-registration-onboarding-plan-v3.md` 为准。

## 1. Policy Activation 第一阶段取消授权路径中的 stale cache

`saas_policy_releases` 继续作为 Current Policy Version 的数据库单一权威，但第一阶段受保护业务 API 的授权决策不再允许仅依赖 bounded TTL cache。

固定规则：

```text
protected tenant request
-> verified identity
-> live effective organization
-> authoritative DB read of active policy version / activation epoch
-> verify current consent against that exact version
-> business handler
```

允许 UI/展示侧使用非权威 cache，但任何 authorization allow decision 都必须基于当前 DB active release。若 DB 不可读则 fail closed。

未来如需为授权路径加 cache，只允许二选一：

1. 同步失效并带 activation epoch 的强一致 cache；或
2. staged policy 使用未来 `activate_at`，且 activation fence 明确晚于所有旧 cache 的最大 TTL + clock-skew，并有 mixed-version Pod 验收。

Phase1 不实现这两个优化，直接 DB authoritative read，优先正确性。

## 2. Admission Accounting 与 Provider Object Accounting 必须分离

`registration_admission_slot` 只负责 `/register` 在 lookup 前的 branch-neutral admission，不代表 Provider Object ownership。

固定顺序：

```text
atomic acquire generic admission slot before lookup
-> exact E.164 lookup
-> classify existing / pending / genuinely-new
-> same transaction release generic admission slot for ALL branches
```

分流后的 Provider Object accounting：

- existing active user：不新增 Provider Object slot；
- pending registration-owned identity：继续使用它原有的 unresolved Provider Object slot，禁止把本次 generic admission 转成第二个 Provider slot；
- genuinely new：只有该分支才申请一个新的 global Provider Object slot，再进入 Create Organization/User；
- 如果 global Provider Object high-water 已满，则 `/register` 必须在 lookup 前统一 fail closed，不能让 new 分支在 lookup 后单独失败形成 oracle。

因此 generic admission 的 acquire/release 行为对 existing/pending/new 完全一致，branch-specific accounting 只发生在内部 Provider Object authority 中。

## 3. Provider Operation 之外增加 Target-scoped Mutation Fence

`logical_operation_key` 解决同一 kind 的逻辑幂等，但不能解决 Delete 与 Reclaim 等不同 kind 之间的冲突。

新增 target-scoped fence，例如：

```text
saas_registration_provider_target_fences
registration_id
ownership_epoch
provider_target_type
provider_target_id
active_logical_operation_key
mutation_class
state
lease_owner
lease_until
epoch
UNIQUE (registration_id, ownership_epoch, provider_target_type, provider_target_id)
```

所有会改变同一 Provider target 的长期 mutation 在进入 `inflight` 前必须先取得同一个 target fence，包括：

```text
create
ownership marker write/repair
reclaim
cleanup delete
```

规则：

- 同 target 同 ownership epoch 同时最多一个 conflicting mutation 可 inflight；
- operation logical key 仍负责 same-intent replay；target fence 负责 cross-kind serialization；
- stale fence 只能按 epoch/lease fenced takeover；旧 external outcome 未 definitive 前禁止发 conflicting mutation；
- Delete 与 Reclaim 绝不能因为 kind 不同而各自取得独立并发资格。

## 4. Verified Onboarding Work Capacity 是 lease，不是长期所有权

`verified_onboarding_active` 只表示当前有 worker 正在执行可推进的业务开通步骤。

进入以下等待/终态时必须事务内释放 work capacity：

```text
authorized
consent_required
verified_waiting_capacity
PROJECT_GRANT_REPAIR_REQUIRED
AUTHORIZATION_REPAIR_REQUIRED
PROVIDER_REPAIR_REQUIRED
factor_enrollment_outcome_unknown / repair_required
其他明确 manual-repair 状态
```

身份仍保持 verified/provider ownership，不回占 anonymous admission pool。

Reconciler 在修复条件满足后重新 acquire work lease 再继续。Work lease 自身有 `lease_until / epoch` 与 crash recovery scanner，死亡 worker 不得永久占 active capacity。

## 5. Finality Capability Gate 覆盖所有 Durable Provider Write

Capability Gate 不能只验证 Delete finality。凡 Task 5 依赖 `outcome_unknown -> definitive` 的长期 Provider 写入，都必须有可证明的 finality/idempotency/operation-status 或经过 staging 验证的 bounded visibility/finality window：

```text
Create Organization
Create Human User
ownership marker write/repair
Reclaim/ownership mutation
Delete User/Organization
```

Stable caller-supplied ID 只解决重复对象问题，不能证明一个 timeout 后“当前 Get 不存在”的 Create 永远不会稍后提交。

因此任意上述写入无法证明 finality 时：

```text
operation remains outcome_unknown
no conflicting mutation
no cleanup capacity release
self-registration rollout fail closed if this can strand normal traffic indefinitely
```

## 6. Cleanup 与所有 Business Ownership Writer 使用同一 Ownership Fence

仅在 Cleanup 开始前检查“当前没有 business artifact”不够；必须防止检查后并发 `ApplyPlan` / business projection / Store/resource/order 写入。

新增 registration-owned organization 的本地 ownership fence，例如：

```text
saas_registration_ownership_fences
provider_organization_id
registration_id
state = onboarding_writable | cleanup_claimed | preserved_business
cleanup_epoch
updated_at
UNIQUE(provider_organization_id)
```

对 registration-owned、尚未正式授权的 Organization：

- Cleanup 必须在 DB 事务中 `onboarding_writable -> cleanup_claimed`；
- `listingsubscription` initial/apply plan、Onboarding projection、Project Grant preparation 以及任何能创建 durable business ownership 的 task-processor 路径，在各自事务写入前必须锁/检查 fence；看到 `cleanup_claimed` 必须 fail closed；
- 任一 non-disposable business artifact 成功创建时，同事务或可靠 guarded transition 把 fence 变为 `preserved_business`；此后自动 Provider Delete 永久禁止；
- Cleanup 只有持有当前 `cleanup_epoch` 且再次 read-back 无业务资产时才能发 Provider Delete；
- 外部 Delete 窗口期间业务 writer 仍会看到 `cleanup_claimed`，不能在 identity 被删后落下 paid/local state。

这不是第二套 IAM，只是本地 ownership creation 与 cleanup 的并发权威。

## 7. Existing Active User 的 `/register` 不得修改 OTP-SMS Factor

`AddOTPSMS` 只允许用于本次 self-registration 创建、仍由 Registration Provisioning ownership 控制的 pending User。

分支规则：

### Genuinely new / registration-owned pending user

```text
ensure factor according to pinned registration contract
-> CreateSMSChallenge
-> VerifySMS
```

### Existing active user

禁止调用 `AddOTPSMS`、Update User Factor、Reactivate factor 等 mutation。

- 如果该用户当前已有可用 SMS factor，则按 ZITADEL 当前 factor policy 继续官方 OTP login/challenge；
- 如果没有 SMS factor，则 `/register` 不替用户新增较弱 factor，转到官方 `/login` / account recovery / 现有 factor policy；
- Registration Provisioning Credential 不需要、也不应获得修改任意既有用户 factor 的权限。

## 8. 第九轮验收矩阵

至少新增：

```text
policy V2 activate while Pod caches V1 -> protected API cannot authorize with V1 cache
lookup 后 generic admission 在 existing/pending/new 三分支统一 release
pending identity repeated /register -> 不新增 provider-object slot
provider global high-water full -> lookup 前所有 register branch uniform fail closed
Delete vs Reclaim same target -> target fence 只允许一个 inflight
marker repair vs Delete same target -> serialized
repair_required -> verified work slot released and later reacquired
Create Org/User timeout -> no cleanup/retry until finality definitive
cleanup_claimed vs concurrent ApplyPlan -> one wins; never paid state + deleted identity
existing active user without SMS factor -> /register does not AddOTPSMS
existing active user with SMS factor -> uses current ZITADEL policy, no factor mutation
```

## 9. IAM 边界不变

- OTP code、Password、Session、OIDC 仍由 ZITADEL/Login V2 负责；
- task-processor 不保存 plaintext Session Token；
- 新增的 Admission、Target Fence、Ownership Fence、Provider Operation 都只解决本地业务并发与外部副作用恢复，不成为身份认证权威。
