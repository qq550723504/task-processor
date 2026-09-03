# 硕米账号入口：ZITADEL 原生身份流与业务收敛设计

**状态：** 已确认

**适用范围：** 第一阶段账号入口（手机号注册、验证码登录、手机号密码登录、密码重置、邀请注册）

**关联规格：**

- `docs/superpowers/specs/2026-09-02-shuomi-console-phase1-hard-cut-design.md`
- `docs/superpowers/plans/2026-09-03-shuomi-account-entry-slice1.md`

**覆盖关系：** 本文覆盖并取代上述 Console 规格中与账号入口执行顺序、持久化状态、ZITADEL／Temporal 职责有关的旧描述。Console、账户、企业权益和店铺中心的其他产品决策继续有效。

---

## 1. 决策摘要

账号入口不再设计为一套由 `task-processor` 或 Temporal 主导的通用 Saga。

最终边界固定为：

```text
ZITADEL User / Organization / Session / OIDC
→ 身份流程与身份事实权威

ZITADEL Actions v2
→ 身份事件的签名通知与最终收敛触发
→ 不承载硕米业务代码
→ 不作为业务数据库唯一性和事务的替代品

task-processor
→ 手机身份盲索引、注册占用、邀请、业务就绪状态
→ listingkit_admin、base_payg 等硕米业务初始化
→ 幂等、并发约束、回调交付、状态查询与恢复

PostgreSQL
→ 唯一约束、事务、业务 Claim、幂等执行记录

Redis
→ 短窗口限流，不作为身份或流程权威存储

Temporal
→ 不进入账号入口第一阶段
→ 保留给店铺授权、支付退款、认证审核等真正长流程
```

PR #218 已经实现并验证的以下能力必须直接复用：

```text
ZITADEL HTTP SMS Provider
腾讯云短信 Relay
OTP SMS 精确事件白名单
创建 SMS Challenge
验证短信验证码
读取 Session user + otpSms 因子
非生产真实设备验证
```

不得再实现第二套短信发送、验证码存储或 OTP 校验器。

---

## 2. 为什么不使用 Temporal 主导账号入口

账号入口的核心步骤已经由 ZITADEL 原生提供：

```text
用户与组织
OTP SMS Factor
Session Challenge
Session Proof
密码因子
OIDC Auth Request
OIDC Callback
Token 与会话
```

Temporal 不能替代这些身份语义；把每一步再映射为 Workflow Activity 会形成重复状态机，并增加：

```text
ZITADEL Session 状态与 Temporal 状态漂移
双重超时和重试策略
敏感 Session Token 进入 Workflow History 的风险
不必要的 Worker 和部署依赖
```

第一阶段只需要数据库状态和轻量收敛任务来补齐跨边界的业务初始化。后续出现数小时／数天等待、人工审核、支付回调或复杂补偿时，再单独使用 Temporal。

---

## 3. ZITADEL Actions v2 的定位

ZITADEL v4.17.1 的 Actions v2 提供：

```text
Execution Condition
├── Request
├── Response
├── Function
└── Event

Target
├── RESTWebhook
├── RESTCall
└── RESTAsync
```

Execution 可以按事件名或事件组触发有序 Target；Target 支持 `X-ZITADEL-Signature` HMAC-SHA256，也支持 JWT／JWE Payload。

协议基准：

```text
proto/zitadel/action/v2/execution.proto@v4.17.1
proto/zitadel/action/v2/target.proto@v4.17.1
```

账号入口第一阶段采用以下原则：

1. Actions v2 只向 `task-processor` 发送身份事实或触发收敛。
2. Target 接收端必须验证签名、时间窗口和请求体大小。
3. Receiver 以稳定事件 ID 或经验证的事件复合标识做幂等。
4. Actions 可能重复、延迟或失败；业务处理必须可安全重放。
5. 登录成功不只依赖某一次 Action 已投递；本地就绪检查和 Reconciler 仍是最终兜底。
6. 不在 ZITADEL Action 中编写套餐、权益、店铺或账本业务逻辑。
7. 未经真实环境验证的事件名、Payload 字段和重投行为不得写死。

推荐模式：

```text
ZITADEL EventExecution
→ RESTAsync / 签名 Target
→ /api/v1/integrations/zitadel/identity-events
→ identity_event_inbox
→ 幂等 Projector / Reconciler
```

若真实环境无法提供稳定事件 ID 或必要主体字段，Actions v2 降级为审计／唤醒信号；`task-processor` 通过 ZITADEL API读取权威状态完成收敛。

---

## 4. 权威数据所有者

| 数据 | 权威来源 |
|---|---|
| ZITADEL User ID | ZITADEL |
| 手机号及验证因子 | ZITADEL |
| 密码和密码策略 | ZITADEL |
| Session 与 OIDC Auth Request | ZITADEL |
| Organization 与项目授权 | ZITADEL |
| 手机号到 ZITADEL User 的硕米盲索引 | task-processor PostgreSQL |
| 注册占用和并发互斥 | task-processor PostgreSQL |
| base_payg、企业业务状态 | task-processor PostgreSQL |
| 页面 Flow 恢复状态 | task-processor PostgreSQL + ZITADEL Session Proof |
| 短窗口限流 | Redis |
| Auth.js 业务会话 | Next.js / Auth.js |

`task-processor` 不保存：

```text
密码或密码哈希
短信验证码
完整持久化手机号
ZITADEL 管理 Token
明文 Session Token
明文 callback URL
```

临时 Session Token 和 callback URL 只能使用版本化 AEAD 加密并按生命周期清理。

---

## 5. 自助注册顺序

ZITADEL 创建 SMS Challenge 必须先有 User ID。因此正确顺序为：

```text
1. 统一检查账号入口和自助注册开关
2. 校验可信设备 Cookie、IP 和手机号发送限流
3. 对手机号全部有效 HMAC 指纹取得数据库级注册 Claim
4. 查询 active / pending 手机身份
5. 若无身份，创建 inert Organization 和 inert User
6. 不授予 ORG_OWNER、listingkit_admin 或套餐
7. 为该 User 添加或确认 OTP SMS Factor
8. 由 ZITADEL 创建 SMS Challenge
9. 用户提交验证码，由 ZITADEL Session API 校验
10. 读取并核对 Session user + otpSms Proof
11. 手机身份进入 pending_provisioning
12. 幂等初始化 base_payg 和企业业务资料
13. 幂等授予 ORG_OWNER
14. 最后授予 listingkit_admin
15. 手机身份原子切换为 active
16. 创建 OIDC Callback
17. 加密保存可重放 callback 交付记录
18. BFF 返回 303 给 Auth.js callback
19. Auth.js Session 建立后调用 ACK
20. ACK 后清除 callback 密文并完成 Flow
```

关键不变量：

```text
同一手机号并发开始注册，只允许一个 Claim Owner 创建 Provider 资源
未完成业务初始化的身份不得进入普通登录路径
listingkit_admin 是最后一个可访问业务系统的授权效果
未收到 Auth.js Session ACK 前，callback 必须可幂等重放
```

若手机号已经存在且处于 `active`，注册页面可以将本次流程内部转换为 OTP 登录，但公开响应形状、时序等级和页面提示不得泄露账号是否存在。

---

## 6. 验证码登录

```text
1. 统一执行设备、IP、手机号限流
2. 使用所有有效 HMAC Key 计算指纹并查找身份别名
3. active → 对既有 ZITADEL User 创建 SMS Challenge
4. pending_provisioning → 返回安全的 provisioning 状态并触发收敛
5. 不存在 → 运行不可区分的假流程，不创建用户、不发送短信
6. ZITADEL 验证 user + otpSms Proof
7. 创建 OIDC Callback
8. 可重放交付并在 Auth.js Session 建立后 ACK
```

公开层不得区分：

```text
用户不存在
手机号未绑定
账号处于未完成状态
短信 Provider 没有发送
```

只有真正拥有手机号的用户能完成 OTP Proof。

---

## 7. 手机号密码登录

```text
手机号盲索引解析 active User ID
→ ZITADEL Session user + password check
→ 核对 user + password 因子
→ OIDC Callback
→ Auth.js Session
```

以下情况统一返回：

```text
账号或密码不正确
```

不得区分：

```text
手机号不存在
账号尚未设置密码
密码错误
pending 身份
```

密码只在请求内存中存在，不进入数据库、日志、错误信息或 Workflow History。

---

## 8. 密码重置

```text
手机号
→ ZITADEL SMS Challenge
→ user + otpSms Proof
→ ZITADEL User API 设置新密码
→ 清理重置 Flow
→ 返回 /login?method=password
```

重置成功后不自动登录，遵循 Figma“返回登录”的产品行为。

不存在的手机号运行不可区分的假流程，不发送短信，也不泄露账号状态。

---

## 9. 邀请注册

### 已存在 active 身份

```text
验证手机号 Proof
→ 复用同一个 ZITADEL User
→ 幂等添加目标企业授权
→ 不创建默认企业
→ 不创建 base_payg
→ 不授予 ORG_OWNER，除非邀请明确且调用者有相应权限
```

### 新手机号

```text
取得手机号 Claim
→ 创建由 Invitation Flow 明确标记的 inert User
→ 创建 SMS Challenge
→ OTP Proof
→ 激活手机身份
→ 添加目标企业授权
```

新用户必须记录 `created_by_flow_id`。邀请放弃或过期时，Reconciler 只有在以下条件全部成立时才能精确删除 User：

```text
User 由该 Flow 创建
无 active 手机身份
无成功 OIDC Session
无项目授权或企业管理员角色
无其他邀请和业务引用
```

状态不确定时进入 `quarantined`，不得猜测删除目标企业或用户。

---

## 10. 手机身份与 HMAC 轮换

不能用以下唯一约束直接表达一手机号一用户：

```text
UNIQUE(phone_fingerprint, key_version)
```

因为密钥轮换后同一手机号会产生不同指纹。

采用两层模型：

```text
account_phone_identities
- identity_id
- zitadel_user_id
- home_organization_id
- state: pending_provisioning | active | quarantined
- owning_flow_id
- proof_verified_at
- activated_at

account_phone_fingerprint_aliases
- identity_id
- key_version
- fingerprint
- UNIQUE(key_version, fingerprint)
```

注册 Claim 必须同时覆盖全部读取 Key 的指纹，并按确定顺序加锁。

轮换协议：

```text
阶段 A：所有实例部署 Current + Previous 双读能力，继续旧 Key 写入
阶段 B：确认全部实例具备双读后，切换新 Key 写入
阶段 C：读取旧 Alias 时，在同一事务补写新 Alias
阶段 D：等待超过最长 Flow TTL 和滚动部署窗口
阶段 E：确认无旧 Key 未迁移记录，再移除旧 Key
```

相同手机号即使由新旧实例并发处理，也只能关联一个 `identity_id`。

---

## 11. 注册 Claim 与并发控制

`Flow.version` 只能防止同一 Flow 被并发更新，不能防止不同 Flow 创建重复 Provider 资源。

增加：

```text
account_entry_claims
- claim_type
- claim_key_hash
- owner_flow_id
- state: held | completed | released | expired
- lease_until
- version
- UNIQUE(claim_type, claim_key_hash)
```

手机号注册使用：

```text
claim_type = phone_registration
claim_key_hash = HMAC(key_version + fingerprint)
```

所有有效 Key Alias 的 Claim 在同一数据库事务中获取。只有 Claim Owner 可以创建 inert User／Organization。

必须测试：

```text
不同 Idempotency-Key 的并发注册
不同应用实例的并发注册
Claim Owner 崩溃后的租约恢复
同一请求重放
HMAC 轮换期间新旧实例并发
```

---

## 12. 幂等执行

每个写操作拥有逻辑 Operation ID，而不是每次 HTTP 重试生成新 UUID。

建议派生方式：

```text
HMAC(operation-key,
  flow_id + operation_name + operation_sequence)
```

例如：

```text
challenge:1
resend:2
verify:1
provision:1
password-reset-complete:1
oidc-deliver:1
invitation-consume:1
```

持久化：

```text
operation_executions
- scope_type
- scope_id
- operation_name
- idempotency_key
- request_fingerprint
- state
- result_reference
- error_code
- expires_at
- UNIQUE(scope_type, scope_id, operation_name, idempotency_key)
```

规则：

```text
同 Key + 同 Fingerprint → 返回已记录结果
同 Key + 不同 Fingerprint → IDEMPOTENCY_CONFLICT
处理中 → 返回 processing / Retry-After，不并行执行
成功响应丢失 → 重试可取得同一结果
```

敏感结果只保存加密引用，不写普通响应缓存。

---

## 13. OIDC Callback 交付

ZITADEL OIDC `CreateCallback` 对一个 Auth Request 只能完成一次。因此不能在第一次向 BFF 返回 URL 时立即删除。

状态：

```text
callback_ready
→ callback_delivering
→ callback_acknowledged
→ completed
```

记录：

```text
account_entry_callback_deliveries
- flow_id
- auth_request_fingerprint
- encrypted_callback_url
- cipher_key_version
- expires_at
- delivery_count
- last_delivered_at
- acknowledged_at
- version
```

行为：

```text
Go → BFF 响应丢失 → 相同 Operation ID 重放同一 callback
BFF → 浏览器 303 丢失 → 浏览器状态恢复后再次取得同一 callback
Auth.js callback 成功 → /auth-complete 服务器确认 Session
→ ACK callback delivery
→ 清除密文
```

callback URL 永不进入浏览器 JSON、日志、埋点或普通幂等结果表。

---

## 14. Flow 状态恢复

提供只读状态接口，仅返回下一项安全 UI 动作：

```text
enter_phone
enter_otp
provisioning
enter_password
enter_new_password
redirect_ready
completed
expired
quarantined
```

不返回：

```text
手机号
User ID
Organization ID
Session ID
Session Token
callback URL
邀请 Token
Provider 原始错误
```

浏览器刷新、响应丢失或重启后先读取状态，不通过重复写请求猜测当前阶段。

---

## 15. 设备、IP 与手机号限流

设备身份采用服务端生成的随机 Cookie：

```text
shuomi_auth_device
HttpOnly
Secure
SameSite=Lax
签名并带 Key Version
```

Redis 只保存设备 ID、IP 和手机号的 HMAC，不保存原始值。

挑战发送至少限制：

```text
per-phone
per-IP
per-device
手机号 + IP 组合
手机号 + device 组合
```

设备 Cookie 被清除时，手机号和 IP 限流仍然生效。签名失败时生成新设备身份，但记录风险计数，不接受客户端自报设备 ID。

---

## 16. 自助注册关闭时的枚举防护

当：

```text
SELF_REGISTRATION_ENABLED=false
```

注册入口必须在查询手机号身份之前统一处理。

所有手机号得到相同状态：

```text
当前暂未开放新用户注册，请使用登录入口或联系管理员。
```

不得将已存在手机号从注册页自动转成 OTP 登录，否则会泄露账号存在性。

验证码登录入口仍执行不可区分的真实／假流程。

---

## 17. Pending Binding 与业务就绪

手机身份状态：

```text
pending_provisioning
active
quarantined
```

OTP Proof 后先进入 `pending_provisioning`。只有以下事实全部核对后才能原子转为 `active`：

```text
ZITADEL User 存在且与 Proof 一致
目标 Organization 存在
base_payg 已初始化
ORG_OWNER 符合自助注册规则
listingkit_admin 已授予
本地企业业务映射已完成
```

普通登录只接受 `active`。`pending_provisioning` 请求进入状态恢复和 Reconciler，不绕过未完成步骤。

---

## 18. Actions v2 Identity Event Inbox

```text
account_identity_event_inbox
- delivery_id
- event_id / event_fingerprint
- event_type
- aggregate_type
- aggregate_id
- raw_body_digest
- received_at
- state: received | applied | ignored | failed | quarantined
- attempts
- next_attempt_at
- last_error_code
```

Receiver：

```text
验证 X-ZITADEL-Signature
验证时间窗口或 JWT/JWE 声明
限制请求体大小
拒绝重复 JSON Key
计算 raw body digest
原子插入 Inbox
快速返回 2xx
后台 Projector 幂等处理
```

Actions 事件只用于收敛和审计，不直接授予业务访问。收到 `user.human.added` 也不能绕过 OTP Proof 或手机号 Claim。

---

## 19. 邀请权限

邀请签发使用现有授权器的正式 Capability：

```text
account.invitation.create
```

映射：

```text
listingkit_admin  → allow
platform_admin    → 仅受控代管上下文 allow
listingkit_operator → deny
listingkit_viewer → deny
```

必须修改 `internal/authz` 的权限常量、策略和正反测试。前端隐藏按钮不能替代后端授权。

---

## 20. “记住登录状态”第一阶段处理

第一阶段不展示“记住登录状态”复选框。

原因：当前 Auth.js 会话生命周期是全局配置，若不完成逐次登录的 Cookie／Session Guard 设计，复选框不会产生真实效果。

后续单独设计时必须同时定义：

```text
ZITADEL Session Lifetime
Auth.js Session Lifetime
浏览器 Session Cookie 与持久 Cookie
退出登录和会话撤销
无 Guard 时强制重新认证
```

不得先显示一个无效选项。

---

## 21. 轻量 Reconciler

账号入口第一阶段使用数据库扫描型 Reconciler，不使用 Temporal。

职责：

```text
恢复 pending_provisioning
核对 Actions Event Inbox
重试幂等业务初始化
清理过期 Session 和 Callback
安全删除由 Flow 独占的 abandoned User／Organization
将状态不确定的资源标记 quarantined
```

不负责：

```text
重新发送短信
伪造 OTP Proof
自动删除来源不明的 ZITADEL 资源
重置用户密码
跳过 callback ACK
```

Reconciler 使用租约、退避、最大尝试次数和可观测指标；同一 Flow 同时只能由一个实例处理。

---

## 22. 何时再使用 Temporal

满足以下任一条件时，为独立领域引入 Temporal Workflow：

```text
需要等待数小时或数天
需要人工审核后继续
跨多个第三方系统并存在复杂补偿
支付、退款、发票和对账
店铺授权与平台回调
企业／个人认证审核
```

账号入口第一阶段不满足这些条件，因此不进入 Temporal。

---

## 23. 验收不变量

```text
同一手机号在并发、重试和 HMAC 轮换中只产生一个 ZITADEL User
不同 Idempotency-Key 的并发注册仍只创建一个 Provider 身份
验证码永远由 ZITADEL 校验
pending 身份不能进入普通登录
注册开关关闭时不能枚举账号
邀请放弃不会永久遗留无主 User
callback 响应丢失后仍能恢复交付
callback ACK 后才清理密文
浏览器刷新可以通过只读状态接口恢复
Actions 重复、延迟或丢失不会产生重复业务初始化
Actions Receiver 验证签名并幂等入箱
第一阶段不依赖 Temporal
第一阶段不展示无效的“记住登录状态”
```

---

## 24. 发布顺序

```text
1. 合并文档和协议门槛
2. 上线 Actions v2 Receiver，但不配置 Execution
3. 验证签名、Payload 和事件标识
4. 配置非阻断 RESTAsync Event Execution
5. 上线手机号 Claim、Identity Alias、Flow Status 和 Reconciler
6. 开放既有用户 OTP／密码登录
7. 在预发布开放自助注册
8. 验证并发、响应丢失、重启、密钥轮换和邀请清理
9. 再开放生产自助注册
```

回滚 Actions Execution 或账号入口开关时，不删除已经完成的用户、企业、角色、套餐或有效手机身份。
