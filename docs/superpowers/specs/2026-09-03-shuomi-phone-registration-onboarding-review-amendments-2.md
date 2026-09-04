# 硕米手机号注册与 Onboarding：第二轮评审修订

**状态：** 权威修订  
**覆盖：** 本文继续覆盖前述手机号注册设计、实施计划和第一轮评审修订中的冲突内容。

---

## 1. Authorization 只能从 `business_prepared` 推进

权威状态顺序固定为：

```text
otp_verified
→ business_preparing
→ business_prepared
→ project_grant_ready
→ authorizing
→ authorized
```

`EnsureProjectGrant/EnsureProjectAuthorization` 的正常入口状态必须是 `business_prepared`；`otp_verified/business_preparing` 不允许直接授权。Provider 调用结果未知时仅在 `project_grant_ready/authorizing` 中 reconcile，不倒退到 Prepare。

---

## 2. 过期 Intent 支持“手机号所有者 reclaim”，不永久占号

自动删除 Provider User 不安全时，不能把合法手机号永久困在 quarantined pending identity。

新增 `reclaimable` 语义：

```text
expired/quarantined pending identity
+ 无 Project Grant/User Authorization
+ 无 business_prepared/readiness
+ canonical login name 与输入手机号一致
→ 允许手机号所有者通过新的 ZITADEL OTP Session 证明所有权
→ proof 成功后，将旧 Provider User/Org 重新绑定到新的 Registration Attempt
→ 生成新的 registration_id ownership epoch
→ 旧 Intent 标记 superseded_by
```

Reclaim 只在 OTP Proof 后执行；证明前仍不可枚举 existing/pending/new。

禁止 reclaim：已有业务授权、已有有效订阅业务投影且状态不明确、Provider 属性不一致。此时进入人工修复。

这样即使自动 Delete 关闭，合法用户也可在证明手机号后继续注册，而不是永远被 canonical login name 冲突锁死。

---

## 3. Global Pending Cap 必须是数据库原子 Admission

不能使用 `COUNT(*) < limit` 后再插入的竞态检查。

新增数据库 admission semaphore，例如：

```text
saas_registration_capacity
- capacity_key PRIMARY KEY
- in_use
- max_allowed
- version
```

BeginRegistration 在同一事务：

```text
锁 capacity row
校验 in_use < max_allowed
插入 Registration Intent
in_use++
提交
```

只有 Intent 达到 Provider 已确认删除，或达到不再占用 Provider Object 的安全终态，才在同事务释放 slot。

quarantined/unresolved Provider Object 不释放 slot。

多实例并发测试必须证明 admitted unresolved intents 永远不超过 `max_allowed`。

Worker concurrency 只限制吞吐，不替代 admission cap。

---

## 4. 注册前必须支持 exact E.164 Login Name Lookup

Provider capability gate 增加 Stop Condition：

```text
exact global E.164 login-name lookup 可用
调用凭据权限足够且只读
结果能唯一映射到 User + Organization
```

实现优先复用仓库已有 `findGlobalUserByLoginName` 能力或其公共抽取，不创建第二条搜索协议。

注册入口内部顺序：

```text
exact login-name lookup
├── active user → 官方 OTP Session
├── pending/reclaimable user → pending/reclaim 流程
└── not found → stable-ID new registration
```

OTP Proof 前三类的浏览器可观察结果保持一致。

---

## 5. 所有 Provider 调用必须有 deadline + retry budget

Provisioning Worker 的 slot 不能被无限挂起连接占用。

统一 Provider Client Contract：

```text
每次 HTTP request 有 context deadline
client 有 connect/TLS/header/body timeout
整个逻辑操作有 total deadline
只对明确 retryable transport/status 做有限次数 retry
指数退避有 max delay 与 max attempts
context cancel 立即释放 worker slot
```

达到 total deadline 后状态记为 `outcome_unknown`，随后通过 Get/lookup/adopt reconcile；不得无限同步重试 Create。

测试：Provider 接受连接后永久 stall、context cancel、timeout 后 worker slot 可重新使用。

---

## 6. Human User 创建必须保留 pinned v4.17.1 Technical Email 合同

手机号注册仍需复用 PR #218 / `phoneonboardingpreflight` 已验证的 Human User request shape。

创建 User 时包括：

```text
stable provider_user_id
canonical E.164 login name / phone
opaque generated technical email
email verified = true（按当前已验证合同）
不启用邮件恢复路径
```

Technical Email 规则继续沿用：

```text
随机/opaque local part
固定受控 @phone.invalid（或当前已验证等价域）
不得包含手机号数字、手机号 hash 可逆提示、姓名等个人信息
不得向该地址发送恢复邮件
```

Capability Gate 必须验证当前 ZITADEL 版本的完整 CreateHumanUser request，而不是只验证 ID/phone 字段。

---

## 7. 实施计划追加

- Authorization state test：`business_prepared → project_grant_ready → authorizing → authorized`。
- Reclaim tests：过期 pending、自动删除关闭、合法 OTP owner 可 reclaim；攻击者无 OTP 不可 reclaim。
- Atomic admission：100+ 并发、多实例，Provider object admission 不超过 hard cap。
- Exact login-name lookup：existing/pending/new 三类；复用现有 lookup 能力。
- Provider timeout：stall/cancel/有限 retry/outcome_unknown。
- Technical Email：必填字段、无手机号数字、不形成 email recovery。

---

## 8. 更新后的核心不变量

- Provider Identity 已存在时，注册先 exact login-name lookup，不用新 ID Create 去“探测冲突”。
- Provider Object hard cap 由数据库原子 admission 保证。
- 自动删除关闭不会导致手机号所有者永久无法注册；所有权证明后可受控 reclaim。
- Project Grant/User Authorization 只能发生在 `business_prepared` 之后。
- Provider stall 不会无限占满 worker。
- 继续复用当前 ZITADEL Human User Technical Email 合同，不简化掉必填字段。
