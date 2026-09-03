# 硕米账号入口：ZITADEL 原生身份流与业务收敛设计

**状态：** 已确认

**适用范围：** 第一阶段账号入口（手机号注册、验证码登录、手机号密码登录、密码重置、邀请注册）

**关联文档：**

- `docs/superpowers/specs/2026-09-02-shuomi-console-phase1-hard-cut-design.md`
- `docs/superpowers/specs/2026-09-03-shuomi-console-resource-store-amendments.md`
- `docs/superpowers/plans/2026-09-03-shuomi-account-entry-slice1.md`

**覆盖关系：** 本文覆盖并取代旧 Console 规格中与账号入口执行顺序、身份持久化、并发、幂等、回调交付以及 ZITADEL／Temporal 职责有关的描述。Console、账户、企业权益和店铺中心的其他产品决策继续有效；资源与店铺激活的修订以单独 amendment 为准。

---

## 1. 决策摘要

账号入口使用 ZITADEL 自己的身份流程，不再由 `task-processor` 或 Temporal 复制一套身份工作流。

```text
ZITADEL User / Organization / Session / OIDC
→ 用户、手机号、密码、Session、Organization、授权和 OIDC 的身份权威

ZITADEL Actions v2
→ 签名身份事件通知与收敛唤醒
→ 不承载硕米业务逻辑
→ 不作为登录成功的单点依赖

task-processor + PostgreSQL
→ 手机身份盲索引、Provider 资源相关标识、注册 Claim
→ 逻辑幂等、业务就绪、邀请、callback delivery、状态恢复
→ base_payg 与应用授权等硕米业务事实

Redis
→ 手机、IP、设备短窗口限流

Auth.js
→ 登录完成后的 Web 业务会话

Temporal
→ 不进入账号入口第一阶段
→ 保留给支付、认证审核、复杂店铺授权等真正长流程
```

PR #218 已经实现并在非生产环境验证：

```text
ZITADEL HTTP SMS Provider
腾讯云短信 Relay
OTP SMS 精确事件白名单
SMS Challenge 创建
验证码验证
Session user + otpSms 因子读取
```

上述能力必须抽取复用，不得再建立第二套短信发送、验证码表或 OTP 校验器。

---

## 2. 身份与业务边界

| 数据或行为 | 权威来源 |
|---|---|
| User、手机号、密码与验证因子 | ZITADEL |
| Organization 与项目授权 | ZITADEL |
| Session、OIDC Auth Request、Callback | ZITADEL |
| 手机号到 ZITADEL User 的硕米盲索引 | PostgreSQL |
| 注册互斥、逻辑幂等、业务就绪状态 | PostgreSQL |
| `base_payg`、企业业务资料 | PostgreSQL |
| 短窗口挑战限流 | Redis |
| Web 登录会话 | Auth.js |
| 身份事件通知 | ZITADEL Actions v2 + PostgreSQL Inbox |

`task-processor` 不保存：

```text
密码或密码哈希
短信验证码
可逆的持久化手机号
ZITADEL 管理 Token
明文 Session Token
明文 callback URL
```

流程暂存的 Session Token、callback URL 和手机号只能使用版本化 AEAD 加密，并按 TTL 与 ACK 生命周期清理。

---

## 3. 为什么账号入口不使用 Temporal

ZITADEL 已原生拥有：

```text
用户与组织
OTP SMS Factor
Session Challenge 与 Proof
密码因子
OIDC Auth Request 与 Callback
Token 和 Session
```

将这些步骤再次映射成 Temporal Activity 会导致双状态机、双重超时和敏感 Session 数据进入 Workflow History。第一阶段只使用 PostgreSQL 状态、幂等执行和轻量 Reconciler 收敛跨边界业务事实。

Temporal 仅在出现以下需求时再引入：

```text
数小时或数天等待
人工审核与 Signal
支付、退款和复杂补偿
多个外部系统的长周期编排
```

---

## 4. ZITADEL Actions v2 的定位

ZITADEL v4.17.1 Actions v2 支持 Request、Response、Function、Event Execution，以及 RESTWebhook、RESTCall、RESTAsync Target。Target 支持 JSON HMAC、JWT 与 JWE Payload。

协议基准：

```text
proto/zitadel/action/v2/execution.proto@v4.17.1
proto/zitadel/action/v2/target.proto@v4.17.1
```

第一阶段规则：

1. Actions v2 只发送身份事实和唤醒收敛，不写套餐、权益、店铺或账本业务代码。
2. 生产优先使用 `PAYLOAD_TYPE_JWT`；必须在预发布证明 `iss`、`aud`、`iat`、`exp`、`jti` 和事件标识可稳定校验。
3. 若使用 JSON HMAC，签名正文必须包含稳定事件 ID 和事件创建时间；接收端校验 `X-ZITADEL-Signature`、最大时钟偏差和事件唯一性。
4. 只有 HMAC 而没有签名时间／唯一 nonce 的模式不得启用生产 Target。
5. Inbox 对 `event_id` 或 `jti` 建唯一约束；过期但签名正确的请求仍必须拒绝。
6. Actions 允许重复、延迟或失败；登录主链路通过本地就绪检查和 ZITADEL 权威查询完成，不等待某一次 Action 成功。
7. 若预发布无法证明事件字段、签名或重投语义，Actions 降级为关闭；账号入口仍可通过同步幂等初始化与 Reconciler运行。

推荐通道：

```text
ZITADEL EventExecution
→ 签名 RESTAsync Target
→ POST /api/v1/integrations/zitadel/identity-events
→ identity_event_inbox
→ 幂等 Projector / Reconciler
```

---

## 5. Provider 资源创建必须可恢复

数据库 Claim 只能阻止并发调用，不能自动解决“ZITADEL 已创建成功但 HTTP 响应丢失”。因此所有 Provider 资源 ID 在调用前由硕米生成并持久化。

ZITADEL v4.17.1 支持：

```text
AddOrganizationRequest.organization_id
CreateUserRequest.user_id
```

规则：

```text
1. 在 PostgreSQL 中生成并保存 canonical provider_organization_id / provider_user_id
2. 再调用 ZITADEL，并显式传入这些 ID
3. 超时、断连或 5xx 都标记为 outcome_unknown，不立即发起另一个 ID 的创建
4. 恢复时先按预分配 ID Get
5. 对象存在且不可变关联匹配 → adopt
6. 对象明确不存在 → 使用同一个 ID 重试 Create
7. 对象存在但组织、用户名、技术邮箱或 flow correlation 不匹配 → quarantine
```

技术名称也必须预先确定：

```text
ZITADEL Organization technical name：全实例唯一、由 Flow／Identity 派生
业务展示名称：默认“我的工作空间”，保存在硕米业务资料中，可重复
内部用户名和技术邮箱：由 Identity ID 派生，不包含手机号
```

不得使用“响应没收到就生成新 ID 再 POST”的恢复方式。

---

## 6. 自助注册流程

ZITADEL SMS Challenge 需要已有 User ID，顺序固定为：

```text
1. 在任何手机号查询前统一检查账号入口与自助注册开关
2. 验证服务端签名设备 Cookie，执行设备、IP、手机号三维限流
3. 使用 Phone HMAC Key Ring 计算全部读取 Alias
4. 对全部 Alias 按确定顺序取得数据库 phone_registration Claim
5. 查询 PhoneIdentity
6. 若不存在，预分配并持久化 ZITADEL Organization ID、User ID、技术名称
7. 使用预分配 ID 创建或 adopt inert Organization 与 inert User
8. 此时不授予 ORG_OWNER、listingkit_admin 或套餐
9. 添加或确认 OTP SMS Factor，创建 ZITADEL SMS Challenge
10. 用户提交验证码，ZITADEL Session API 校验
11. 读取并核对 user + otpSms Session Proof
12. 创建或推进 PhoneIdentity = pending_provisioning
13. 幂等创建 base_payg 与企业业务资料
14. 幂等授予 ORG_OWNER
15. 最后授予 listingkit_admin
16. 全部完成后原子切换 PhoneIdentity = active
17. 为当前 OIDC Auth Request 创建 callback delivery generation
18. BFF 303 到 Auth.js callback
19. Auth.js Session 建立后进入 /auth-complete 并 ACK
20. ACK 后清理敏感密文并完成 Flow
```

不变量：

```text
不同 Idempotency-Key、不同实例也只能有一个手机号 Claim Owner
Provider 创建响应丢失后只允许按预分配 ID adopt 或重试
OTP Proof 前没有业务角色和套餐
pending_provisioning 不得进入普通登录
listingkit_admin 是最后一个业务访问效果
```

`SELF_REGISTRATION_ENABLED=false` 时，在读取手机号或 Binding 之前统一返回同一禁用结果；不得把已注册手机号从 `/register` 隐式转为 OTP 登录。

---

## 7. 验证码登录

```text
1. 设备、IP、手机号限流
2. Key Ring 双读 Phone Alias
3. active → 对既有 User 创建 SMS Challenge
4. pending_provisioning → 返回安全 provisioning 状态并触发收敛
5. 不存在 → 执行不可区分的假流程，不创建用户、不发送短信
6. ZITADEL 校验 user + otpSms Proof
7. 创建 callback delivery generation
8. Auth.js Session ACK 后完成
```

公开层不区分用户不存在、手机号未绑定、账号未完成或短信未发送。只有真正拥有手机号的用户能够完成 Proof。

---

## 8. 手机号密码登录与反枚举

正常路径：

```text
手机号 Alias → active User ID
→ ZITADEL Session user + password check
→ user + password Proof
→ OIDC callback delivery
→ Auth.js Session
```

以下情况统一为 `INVALID_CREDENTIALS`：

```text
手机号不存在
Identity 非 active
用户未设置密码
密码错误
```

仅统一文案不足以防止时序枚举。每一次密码登录必须执行同级工作：

```text
active 且有密码 → 对真实 User 执行一次 ZITADEL password check
未知／pending／passwordless → 对专用无业务权限 Decoy User 执行一次同类 ZITADEL password check
所有路径 → 应用预发布标定的最小响应时间 + 小幅随机抖动
所有返回的临时 Session → 精确清理
```

Decoy User：

```text
无项目授权、无企业角色、不能进入 OIDC callback
ID 仅在 Secret 中配置
随机密码不向外部公开
即使意外校验成功也丢弃并删除 Session
```

预发布必须用统计区间测试真实、未知、pending、passwordless 和错误密码的延迟类别，不能只断言错误文本相同。

密码按原始字节处理，不 trim、不 Unicode normalize，并在调用完成后尽力覆盖内存。

---

## 9. 密码重置

```text
手机号
→ 真实绑定时创建 ZITADEL SMS Challenge
→ user + otpSms Proof
→ ZITADEL User API 设置新密码
→ 完成重置 Flow
→ 返回 /login?method=password
```

未知手机号执行不可区分的假流程。重置成功后不自动登录，遵循 Figma 返回密码登录的产品行为。

---

## 10. 邀请注册

### 10.1 已存在 active 身份

```text
验证手机号 Proof
→ 复用同一个 ZITADEL User
→ 幂等授予当前有效企业的目标项目角色
→ 原子标记 Invitation consumed
→ 不创建默认企业或 base_payg
```

### 10.2 新手机号

```text
取得 Phone Claim
→ 预分配并创建/adopt Invitation Flow 所有的 inert User
→ SMS Challenge 与 OTP Proof
→ PhoneIdentity 保持 pending_provisioning
→ 幂等授予目标企业项目角色
→ 原子标记 Invitation consumed 和本地 Access Projection ready
→ 最后切换 PhoneIdentity = active
→ OIDC callback delivery
```

Identity 不能在目标企业授权完成前变为 active。

邀请签发和消费的企业作用域：

```text
POST /api/v1/workbench/account-invitations
AuthPolicy = VerifiedIdentity
OrganizationAccessPolicy = LiveWrite
organization_id 只取 live resolved Effective Organization
请求体不接受 organizationId；若出现未知字段直接拒绝
```

必须测试 A 企业管理员无法为 B 企业签发或消费邀请。

邀请放弃清理：

```text
User 由当前 Flow 创建
Identity 仍非 active
没有项目授权、企业管理员角色、成功 Session、其他邀请或业务引用
→ 可按精确 User ID 删除

任一事实不确定
→ quarantine，禁止猜测删除
```

---

## 11. Phone Identity、Alias 与轮换

```text
account_phone_identities
- identity_id
- zitadel_user_id
- home_organization_id
- state: pending_provisioning | active | failed | quarantined
- owning_flow_id
- proof_verified_at
- activated_at

account_phone_fingerprint_aliases
- identity_id
- key_version
- fingerprint
- UNIQUE(key_version, fingerprint)
```

不能只依赖 `UNIQUE(fingerprint, key_version)`；同一手机号在不同 Key 下会产生多个值。

注册时对 Current 和全部 Previous Key 计算 Alias，并在同一事务按排序结果获取 Claim。轮换协议：

```text
A. 全部实例先部署新旧双读，旧 Key 继续写
B. 确认全部实例支持双读后切换 Current 写 Key
C. 命中旧 Alias 时同事务补写新 Alias
D. 等待超过最长 Flow、Operation 保留期和滚动部署窗口
E. 确认无旧引用后移除 Previous Key
```

新旧实例并发时仍只能关联一个 Identity。

---

## 12. Flow、Claim 与 Binding 状态

```text
account_entry_flows
- id
- kind
- state
- provider_organization_id
- provider_user_id
- provider_resource_state
- encrypted_provider_state
- cipher_key_version
- identity_id
- invitation_id
- challenge_expires_at
- expires_at
- version

account_entry_claims
- claim_type
- claim_key_hash
- owner_flow_id
- state: held | completed | released | expired
- lease_until
- version
- UNIQUE(claim_type, claim_key_hash)
```

`Flow.version` 只保护同一 Flow；跨 Flow 唯一性由 Claim 和 Alias 唯一约束保证。

Claim Owner 崩溃后，新 Owner 必须先执行 provider lookup/adopt，不允许直接创建新 ID。

普通 OTP／密码登录只允许 `PhoneIdentity.active`。`pending_provisioning` 只可通过原 Flow Status 与 Reconciler 继续。

---

## 13. 逻辑幂等与敏感字段指纹

每个用户动作拥有持久化逻辑 Operation ID：

```text
operation_id = HMAC(operation-id-key,
  flow_id + operation_kind + operation_sequence)
```

同一次 HTTP 重试复用同一个 Operation ID；用户修改输入并再次主动提交时递增 sequence。

```text
operation_executions
- scope_type
- scope_id
- operation_name
- operation_id
- request_fingerprint
- fingerprint_key_version
- state
- result_reference
- error_code
- expires_at
- UNIQUE(scope_type, scope_id, operation_name, operation_id)
```

Request Fingerprint：

```text
非敏感字段 → canonical encoding 后参与 SHA-256
手机号 → Phone Identity/Alias ID，不使用明文
OTP、当前密码、新密码 → 使用专用 OperationFingerprint Key 做 HMAC-SHA-256
```

绝对禁止保存敏感原值或无密钥哈希。重试时先读取已有 Operation 的 `fingerprint_key_version`，再使用该版本重新计算；旧 Fingerprint Key 必须保留到所有 Operation 过期。

规则：

```text
同 Operation ID + 同 Fingerprint → 返回或继续同一结果
同 Operation ID + 不同 Fingerprint → IDEMPOTENCY_CONFLICT
```

---

## 14. 版本化 Key Ring 部署协议

Phone HMAC、Flow AEAD、Device Signing 和 Operation Fingerprint 都使用结构化 Key Ring，不使用一个 current 字符串加一个 previous 字符串。

Secret JSON：

```json
{
  "current": {
    "version": "2026-09-a",
    "value": "<base64>"
  },
  "previous": [
    {
      "version": "2026-08-b",
      "value": "<base64>"
    }
  ]
}
```

Canonical 环境变量：

```text
TASK_PROCESSOR_ACCOUNT_ENTRY_PHONE_HMAC_KEYRING_JSON
TASK_PROCESSOR_ACCOUNT_ENTRY_FLOW_AEAD_KEYRING_JSON
TASK_PROCESSOR_ACCOUNT_ENTRY_DEVICE_SIGNING_KEYRING_JSON
TASK_PROCESSOR_ACCOUNT_ENTRY_OPERATION_FINGERPRINT_KEYRING_JSON
```

每个版本唯一；Current 不得同时出现在 Previous；所有 Key 长度和编码启动时 fail closed。部署测试必须覆盖多个 Previous Key、滚动部署和旧密文重写。

---

## 15. Callback Delivery 与 Auth.js 响应丢失

ZITADEL OIDC CreateCallback 生成一次性 authorization code。需要区分两类丢失：

### 15.1 Code 尚未被 Auth.js 消费

```text
callback_ready
→ callback_delivering
→ 重放同一加密 callback URL
```

### 15.2 Auth.js 已兑换 code，但 Set-Cookie／redirect 未到达浏览器

旧 callback 已不可再次使用。若在交付宽限期内没有收到已认证 ACK：

```text
旧 delivery → consumed_unknown / superseded
保留已经验证的 ZITADEL Session Proof 与 Session Material
BFF 启动新的 Auth.js signIn，获得新的 OIDC Auth Request
服务端用同一已验证 ZITADEL Session 为新 Auth Request 创建新的 callback generation
浏览器完成新 callback
```

若 ZITADEL Session 已过期，则返回 `reauthenticate`，要求重新 OTP 或密码验证；不得重新创建用户、企业、角色或套餐。

```text
account_entry_callback_deliveries
- flow_id
- generation
- auth_request_fingerprint
- encrypted_callback_url
- cipher_key_version
- state: ready | delivering | acknowledged | consumed_unknown | superseded | expired
- delivered_at
- acknowledged_at
- expires_at
- UNIQUE(flow_id, generation)
```

`/auth-complete` 只有在 Auth.js Session 的 subject 与 Flow 预期 User 匹配时才 ACK。ACK 前不清除可恢复的验证状态。

---

## 16. Flow Status

浏览器通过 HttpOnly Flow Cookie 调用只读状态接口恢复页面：

```text
GET /api/v1/account-entry/status
GET /api/account-entry/status
```

只返回：

```text
nextAction:
- enter_phone
- enter_otp
- provisioning
- enter_new_password
- redirect_ready
- restart_oidc
- reauthenticate
- completed
- expired

retryAfterSeconds
canResend
canRetry
```

不得返回手机号、User ID、Organization ID、Session ID、Session Token、callback URL 或邀请 Token。

---

## 17. 设备、IP 与手机号限流

设备标识使用服务端生成并签名的随机 HttpOnly Cookie：

```text
name = shuomi_auth_device
HttpOnly
Secure
SameSite = Lax
Path = /
```

Redis Key 只包含设备、IP 和手机号的 HMAC 摘要。挑战发送必须同时通过三维限流；设备 Cookie 无效时重新签发，但手机号和 IP 限制继续生效。

Key 轮换遵循版本化 Device Signing Key Ring；测试覆盖签名失败、Cookie 重放、轮换和清除 Cookie。

---

## 18. Feature Flag

第一阶段统一使用同一组 Canonical 环境变量，Go、Next.js、Kubernetes 和 Runbook 不使用别名：

```text
TASK_PROCESSOR_ACCOUNT_ENTRY_ENABLED
TASK_PROCESSOR_ACCOUNT_ENTRY_SELF_REGISTRATION_ENABLED
```

关闭自助注册时，在任何手机号查找之前统一拦截。旧 `SELF_REGISTRATION`、`SELF_REGISTRATION_ENABLED`、`SHUOMI_SELF_REGISTRATION_ENABLED` 配置若出现应启动失败或明确告警，不能静默读取不同含义。

---

## 19. “记住登录状态”

Slice 1 不展示“记住登录状态”，Flow 不保存 `remember_session`。

只有当以下三层的短期与持久化语义可以端到端验证后，才另行实现：

```text
ZITADEL Session lifetime
Auth.js Session cookie lifetime
浏览器关闭后的 Workbench 访问保护
```

禁止先显示一个对最终 Session 没有真实影响的复选框。

---

## 20. 最终安全与故障矩阵

必须覆盖：

```text
不同 Operation ID 并发注册同一手机号
ZITADEL Organization/User 创建成功但响应丢失
Provider 创建后进程崩溃与 adopt
OTP Proof 后、base_payg 前崩溃
ORG_OWNER 后、listingkit_admin 前崩溃
邀请 Grant 成功但 Invitation consume 响应丢失
Callback 303 丢失且 code 未消费
Auth.js 已消费 code 但浏览器未收到 Session Cookie
Actions 签名正确但过期
Actions 重复、乱序和延迟
Phone HMAC / AEAD / Device / Fingerprint Key 轮换
同 Operation ID 修改 OTP 或密码
未知手机号与真实手机号的密码登录时序类别
跨企业邀请签发尝试
```

恢复原则：

```text
先读取 PostgreSQL 与 ZITADEL 权威事实
能证明成功 → adopt 并继续
能证明不存在 → 使用同一预分配 ID／Operation 重试
事实不确定或不匹配 → quarantine
绝不通过生成新 Provider ID、跳过初始化或放宽权限“修复”
```

---

## 21. 第一阶段完成定义

- 已实现的 OTP/SMS Client 被复用，没有第二套验证码系统。
- ZITADEL 是身份流程权威，Actions v2 仅作为签名事件桥与收敛唤醒。
- Provider Organization/User 使用预分配 ID，响应丢失可 lookup/adopt。
- 一个手机号在并发、重试和 Key 轮换中只对应一个 User。
- 新身份在套餐与目标授权完成前保持 pending。
- 密码登录对未知、pending 和 passwordless 身份执行同级 Provider 工作。
- callback 支持旧 code 未消费重放，也支持 code 已消费后的新 Auth Request generation。
- Actions Receiver 拒绝过期但签名正确的请求。
- 所有 Key Ring 在 Secret 中包含明确版本并支持多个 Previous Key。
- 自助注册开关只有一套 Canonical 名称。
- 注册、登录、重置、邀请支持刷新、响应丢失、重启和并发恢复。
- 手机号、验证码、密码、Session Token、callback URL 和管理凭据不进入日志、公开 JSON 或明文数据库列。
- 生产开关默认关闭，真实预发布验收通过后才开放。
