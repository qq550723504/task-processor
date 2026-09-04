# 硕米手机号注册与 Onboarding：第三轮评审修订

**状态：** 权威修订  
**覆盖：** 本文继续覆盖前述手机号注册设计、实施计划及两轮评审修订中的冲突内容。

---

## 1. `authorized` 原子释放 Pending Provider Capacity

Registration Capacity 只约束“尚未完成业务归属的 Provider Object”，不是限制平台累计注册用户数。

因此 `authorized` 是明确的 capacity-releasing terminal state。

在同一个 PostgreSQL 事务内完成：

```text
SELECT registration_intent FOR UPDATE
SELECT registration_capacity FOR UPDATE
要求 intent.state = authorizing
要求 Project Grant / User Authorization read-back 已成功
intent.state = authorized
intent.authorized_at = now
capacity.in_use -= 1
COMMIT
```

只有以下状态继续占用 pending capacity：

```text
intent_created
provider_provisioning
otp_pending
cleanup_requested
quarantined
以及其他仍对应 unresolved Provider Object 的非终态
```

`authorized` Provider User/Organization 是合法业务身份，不计入 pending/unresolved cap；它们由正常企业/用户生命周期管理。

容量释放必须与 `authorized` 状态变更原子提交，避免重复释放；重放 `authorized` transition 不得再次 decrement。

---

## 2. Re-consent 必须成为受保护业务 API 的授权门，而非只拦 Workbench 页面

Policy 升级后，仅阻止 `/workbench` 页面不足以阻止旧 Session 直接调用后端 API。

新增业务级 `CurrentConsentPolicy`（名称可按现有 `httproute.Descriptor` 风格实现），在身份认证和 live organization resolution 后、业务 Handler 前校验：

```text
current_policy_version
saas_account_consents(user_id, current_policy_version)
```

适用范围：

```text
所有租户业务写 API
所有会读取企业敏感/付费业务数据的 Workbench API
仍保留可达性的旧 ListingKit tenant business API
```

明确豁免：

```text
public/login/OIDC routes
/logout/session termination
当前协议读取
re-consent 提交接口
必要的 account recovery/security routes
health/readiness
```

缺少当前版本 Consent 时返回稳定的：

```text
CONSENT_REQUIRED
```

并引导前端到 re-consent 流程。

不通过临时删除 ZITADEL `listingkit_admin` 来实现，因为 Policy Consent 是硕米业务规则，不应改写身份 Provider 的长期角色来充当短期协议 Gate。

必须测试：

- policy 升级后旧 Auth.js Session 直接调用 tenant write API 被拒绝；
- 完成当前版本 re-consent 后同一用户恢复访问；
- re-consent endpoint 本身不被 Consent Gate 死锁；
- cross-org/live-write 规则仍独立生效。

---

## 3. Registration Project Grant 使用 non-mutating exact-match/create-only 路径

不能直接调用现有会在角色集合不一致时执行 `updateProjectGrant` 的高层 helper。

Registration 专用行为固定为：

```text
查 fixed Sumi Project → target Organization 的 Project Grant
├── 不存在
│   → Create Project Grant，role set 使用注册允许的固定集合
│   → read-back exact match
├── 已存在且 role set exact match
│   → adopt，不写入
└── 已存在但 role set 不一致
    → fail closed: PROJECT_GRANT_REPAIR_REQUIRED
    → 不 Update、不增删角色
```

实现应复用 `internal/zitadelprovision` 已有的低层 search/create/read-back 能力，但抽取新的 non-mutating helper，例如：

```text
EnsureProjectGrantExactCreateOnly
```

禁止注册流程调用会调整已有组织 Project Grant role set 的 `updateProjectGrant` 路径。

必须测试：

- absent → create；
- exact → no-op/adopt；
- superset/subset/different role set → fail closed 且 Provider 无 mutation；
- 并发 create 时唯一结果可 adopt；
- 不能影响已有组织其他 User Authorization 可用角色。

---

## 4. 更新后的完成定义

- 成功注册进入 `authorized` 时原子释放 pending capacity，平台不会因累计正常注册数耗尽 pending cap。
- quarantined/unresolved Provider Object 仍持续占用 pending capacity，直到被安全解决。
- 当前协议版本是后端业务 API 的实际授权前置条件，不只是前端页面 Gate。
- Re-consent 是 task-processor 业务授权规则，不通过修改 ZITADEL 长期角色模拟。
- Registration Project Grant 只允许 create-only/exact-adopt；已有 Grant 角色集合不匹配时绝不自动 update。
- ZITADEL 继续负责身份、OTP、Password、Session、OIDC；本修订没有增加第二套 IAM。
