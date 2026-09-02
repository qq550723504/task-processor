# 硕米智能引擎第一阶段硬切设计

**日期：** 2026-09-02  
**状态：** 产品设计已确认；等待书面规格审阅后进入实施计划  
**代码库：** `qq550723504/task-processor`  
**范围：** 手机号账号入口、新 Console Shell、账户基础、企业空间总览、企业权益只读链路、店铺中心新界面与显式激活  
**设计来源：** Figma 文件 `tg48P46SSXl6TBy9lZwg63`，页面 `31:463`

## 1. 目的

本设计将已经确认的硕米后台产品口径固化为可实施的第一阶段边界，避免开发继续同时依赖旧 ListingKit 页面、新 Figma 页面和多份历史设计文档。

第一阶段采用以下原则：

1. **同一代码库、同一前端基座、两种工作区模式。** 本阶段只交付租户经营工作区；平台工作区保留架构入口但不创建空壳页面。
2. **界面直接硬切。** 新页面不搬运旧 `/listing-kits/*` 内容，不保留旧导航，不建立视觉兼容层。
3. **复用能力，不复用旧产品界面。** 继续使用 ZITADEL/Auth.js、多企业上下文、Store Center、订阅与用量基础设施；新的页面、路由、导航和交互全部按新 Console 构建。
4. **真实数据优先。** Figma 中的示例用户名、金额、店铺数、余额和认证状态不得进入生产默认数据。
5. **先交付可闭环的低风险能力。** 支付、钱包、自定义企业角色、主体认证、退款、发票和推广收益拆为后续独立项目。

本设计继承以下已实现或已确认边界：

- `2026-08-30-shuomi-workbench-store-center-zitadel-multi-org-design.md` 中关于 Home Organization、Effective Organization、组织作用域授权、企业切换和 Store Center 隔离的约束；
- 现有 `WorkbenchContextProvider` 的企业加载、切换保护、请求取消和缓存清理行为；
- 现有 Store Center 的组织隔离、创建幂等、店铺名额账本、版本冲突和审计基础；
- 现有 Subscription/Usage 中 `reserved / committed / released / reversed` 的用量事件思想。

本设计覆盖历史文档中与新 Figma 冲突的界面结构、注册字段、用户名语义、套餐等级、资源名称和店铺激活语义。

## 2. 已确认的产品决策

### 2.1 账号

```text
注册必填：手机号 + 短信验证码 + 协议确认
注册不填：展示名称、用户名、密码、邮箱、经营画像
默认登录：手机号 + 短信验证码
可选登录：用户在后台设置密码后，支持手机号 + 密码
```

Figma 中的“用户名”统一解释为**展示名称**，不作为登录账号。产品不支持“用户名 + 密码”登录。

### 2.2 资源与计费术语

```text
AI Token       → AI 点数
数据资源       → 数据额度，单位为“条”
我的权益       → 企业权益
充值中心       → 企业钱包
企业认证       → 企业空间下的企业认证
```

### 2.3 店铺

```text
绑定店铺 ≠ 激活店铺服务
绑定成功后不开始计费，也不消耗续费期数
用户显式确认“激活店铺服务”后，才消耗续费期数并开始 30 天服务周期
```

### 2.4 企业资源与成员额度

企业持有真实资源：

```text
企业续费期数余额
企业 AI 点数余额
企业数据额度余额
```

成员不拥有独立资源资产。后续企业管理员为成员配置的是**月度消费限额**：

```text
成员当月实际可用量 = min（企业当前可用余额，成员当月剩余额度）
```

成员月度限额属于后续阶段，第一阶段只展示企业总权益和真实企业余额，不实现成员额度配置。

## 3. Figma 节点基线

第一阶段使用下列节点作为视觉和信息架构依据：

| 页面 | 节点 |
|---|---|
| 注册 | `374:325`，但删除用户名和密码字段 |
| 密码登录 | `373:303` |
| 验证码登录 | `378:307` |
| 重置密码 | `1266:359` |
| Console Shell 深色基线 | `393:321` |
| Console Shell 浅色基线 | `411:323` |
| 我的账户总览 | `411:2489` |
| 账户资料总览 | `432:534` |
| 经营画像维护 | `464:540` |
| 企业空间总览 | `432:4694` |
| 套餐与权益总览 | `411:2293` |
| 套餐方案 | `431:3654` |
| 原“我的权益” | `431:4158`，页面标题改为“企业权益” |
| 用量明细 | `431:4662`，修正人民币与资源单位混用 |

以下节点只作为后续阶段设计依据，不进入第一阶段交付：

- 个人认证 `1514:815`；
- 企业角色权限 `1536:653`；
- 成员资源与额度 `1627:539`；
- 企业操作记录 `1532:857`；
- 企业钱包 `431:5166`；
- 账单与订单 `1839:766`；
- 推广与收益相关节点。

## 4. 第一阶段范围

### 4.1 交付顺序

```text
1. 注册页
2. 登录页
3. 新 Console Shell
4. 我的账户
5. 套餐与权益
6. 店铺中心
```

### 4.2 包含

#### 账号入口

- 手机号验证码注册；
- 密码登录与验证码登录切换；
- 忘记密码与重置密码；
- 注册、登录、重置流程中的加载、错误、限流和重试状态；
- 安全校验后的 `returnTo`；
- 邀请注册与普通自助注册分流；
- 登录后建立标准 Auth.js 会话。

#### Console Shell

- 新品牌区、左侧导航、顶部栏、内容区和移动端抽屉；
- 浅色与深色语义 Token；
- 当前企业上下文、企业选择阻断状态和安全切换；
- 联系客服、通知、主题开关和用户菜单的真实入口；
- 租户工作区模式；
- 未上线模块不渲染菜单，不创建“建设中”页面。

#### 我的账户

- 账户总览；
- 账户资料总览；
- 展示名称、地区等普通资料维护；
- 手机号状态、邮箱状态和密码状态展示；
- 设置或修改登录密码；
- 经营画像维护；
- 企业空间总览；
- 真实空状态、未设置状态和无权限状态。

#### 套餐与权益

第一阶段为只读链路：

- 套餐与权益总览；
- 单一基础方案与计费规则；
- 企业权益；
- 企业 AI 点数、数据额度、店铺服务状态；
- 已开通能力；
- 用量明细查询、筛选、分页和导出；
- 资源单位和人民币成本明确分栏。

#### 店铺中心

- 按新 Figma 视觉重建店铺列表、创建、详情和编辑；
- 复用现有组织作用域 Store Center 后端；
- 绑定与激活分离；
- 店铺连接状态、服务状态和记录生命周期分离；
- 显式激活、续费和到期状态；
- 第一阶段最小企业资源账本；
- 版本冲突、幂等和企业隔离。

### 4.3 明确不包含

- 企业自定义角色和权限树；
- 成员管理完整后台；
- 成员月度消费限额；
- 企业人民币钱包；
- 在线支付、支付回调和对账；
- AI 点数或数据额度在线购买；
- 退款、发票和完整账单系统；
- 个人实名认证和企业认证提交审核；
- 推广、收益和提现；
- 运营驾驶舱、AI 工作台、供应市场、智能市场、工具市场、生态服务和数据服务页面；
- 平台工作区页面；
- 旧 ListingKit 前端页面迁移或兼容；
- 未实现菜单的静态占位页面。

## 5. 路由与导航

### 5.1 公开账号入口

```text
/register
/login?method=otp
/login?method=password
/forgot-password
```

`method` 非法时回退到 `otp`。账号入口页面共享同一个 `AuthEntryShell`。

### 5.2 租户工作区

```text
/workbench

/workbench/account
/workbench/account/profile
/workbench/account/profile/settings
/workbench/account/profile/business-profile
/workbench/account/organization

/workbench/entitlements
/workbench/entitlements/plan
/workbench/entitlements/benefits
/workbench/entitlements/usage

/workbench/stores
/workbench/stores/new
/workbench/stores/[storeId]
```

页面标题采用：

```text
/workbench/entitlements/benefits → 企业权益
```

第一阶段不创建：

```text
/workbench/entitlements/wallet
/workbench/entitlements/billing
/workbench/account/organization/roles
/workbench/account/organization/resources
/workbench/account/organization/audit
/workbench/account/profile/verification
/workbench/account/organization/verification
```

### 5.3 硬切规则

- 新导航不生成 `/listing-kits/*` 链接；
- 新页面不嵌入旧页面；
- 不建立旧路由到新路由的长期兼容映射；
- 硬切发布后，旧页面从用户可见入口移除；
- 旧后端能力只有在新页面仍依赖时才继续保留；
- 不在同一业务域长期维护两套前端。

## 6. 账号与身份设计

### 6.1 身份权威

ZITADEL 继续作为身份权威：

- 用户身份、手机号、密码凭据和身份验证归 ZITADEL；
- `task-processor` 不保存第二套密码或密码哈希；
- 浏览器不持有 ZITADEL 管理凭据；
- 登录完成后通过标准 OIDC/Auth.js 会话进入工作区。

### 6.2 注册

注册表单只包含：

```text
手机号
短信验证码
用户协议与隐私政策确认
注册并进入
```

服务端流程：

```text
规范化手机号
→ 发送并验证短信挑战
→ 幂等创建 ZITADEL 用户
→ 生成不可见内部技术用户名
→ 普通注册时创建默认企业空间
→ 邀请注册时加入目标企业且不额外创建空企业
→ 分配初始角色与基础权益
→ 建立会话
→ 进入合法 returnTo 或 /workbench
```

注册接口必须具备：

- IP、手机号和设备维度的发送限流；
- 验证码过期和尝试次数限制；
- 幂等注册；
- 协议版本留痕；
- 失败后的企业和角色初始化补偿；
- 日志脱敏；
- 不通过响应泄露手机号是否已注册。

### 6.3 展示名称和密码

```text
展示名称
- 用于顶部用户菜单、协作和审计展示
- 可以重复
- 不作为登录凭据

登录密码
- 注册时不要求
- 用户在“账户设置”中单独设置
- 设置后支持手机号 + 密码登录
```

### 6.4 重置密码

重置密码适用于已经设置密码的用户：

```text
手机号
→ 短信验证码
→ 新密码
→ 确认新密码
→ 返回密码登录
```

未设置密码的用户仍可直接使用验证码登录。公开错误文案不得泄露账户是否存在或是否设置过密码。

## 7. Console Shell

### 7.1 布局

桌面基线：

```text
左侧导航 240px
顶部栏 72px
内容区填充剩余空间并自然滚动
```

正式实现使用 Grid/Flex，不复制 Figma 导出的绝对定位代码。

### 7.2 组件边界

```text
ConsoleThemeProvider
└── TenantWorkspaceShell
    ├── ConsoleSidebar
    │   ├── ConsoleBrand
    │   ├── ConsoleNavigation
    │   └── ConsoleVersion
    ├── ConsoleMain
    │   ├── ConsoleTopbar
    │   ├── DelegationBanner
    │   └── ConsoleContent
    └── ConsoleMobileDrawer
```

业务页面只能渲染内容区，不允许重复实现侧边栏、顶部栏、主题或企业上下文。

### 7.3 Next.js 布局归属

`/workbench/*` 的 Shell 由 `app/workbench/layout.tsx` 承担，不再由根级 `ApplicationFrame` 根据 `pathname` 客户端选择旧壳。

建议 Provider 顺序：

```text
AuthenticatedWorkbenchBoundary
→ QueryProvider
→ ToastProvider
→ WorkbenchContextProvider
→ ConsoleThemeProvider
→ TenantWorkspaceShell
```

### 7.4 主题

- Console 使用独立 `shuomi-console-theme` 存储键；
- 默认主题为深色；
- 所有页面使用语义 Token，不在业务组件内写深浅主题分支；
- Shell 以 `393:321` 和 `411:323` 对照验收；
- 详细页面的浅色稿为像素基准；深色详细页由统一 Token 转换并通过可访问性与视觉回归测试；
- 若深色详细页未达到发布门槛，主题开关通过特性开关保持关闭，但不删除主题基础设施。

### 7.5 导航状态

- 同一时间只展开一个一级菜单；
- 同一一级菜单内只展开当前路由所需分组；
- 导航区域独立滚动；
- 当前路由自动展开父级；
- 未上线菜单不渲染，不使用可点击的灰色占位项。

## 8. 企业上下文与数据归属

### 8.1 Effective Organization

所有企业业务接口以服务端验证后的 Effective Organization 为作用域。

```text
允许操作 =
身份有效
+ 用户对当前企业有授权
+ 当前企业内角色有权限
+ 资源属于当前企业
+ 企业权益允许
+ 业务状态允许
```

浏览器传入的企业 ID、Cookie、查询参数和请求头都不是授权凭据。

### 8.2 企业切换

继续复用现有 `WorkbenchContextProvider` 行为：

- 切换前执行页面 Guard；
- 写操作进行中禁止切换；
- 取消进行中的请求；
- 切换成功或失败后清理缓存；
- 企业撤权、停用或依赖不可用时 fail closed。

所有新查询键显式包含企业 ID：

```ts
["workbench", organizationId, "account", "overview"]
["workbench", organizationId, "entitlements", "overview"]
["workbench", organizationId, "usage", filters]
["workbench", organizationId, "stores", filters]
```

### 8.3 经营画像

经营画像在页面上保持统一表单，但服务端按归属拆分：

```text
用户作用域
├── 用户角色
└── 所需服务

企业作用域
├── 店铺阶段
├── 经营平台
├── 经营站点
├── 店铺类型
└── 工厂与供应链
```

普通成员可以修改用户作用域；只有企业管理员可以修改企业作用域。经营画像只用于分类、统计、服务沟通和推荐参考，不直接触发开店、执行任务或购买资源。

### 8.4 认证位置

```text
个人认证 → 个人账户作用域
企业认证 → 当前企业空间作用域
```

第一阶段不开放认证提交页面。后续实现时，企业认证的唯一权威记录归 `organization_id`，账户页与企业空间只能读取同一记录，不复制状态。

## 9. 企业权益与用量

### 9.1 单一基础方案

当前产品只有一个计费方案：

```text
基础方案 · 按需使用
plan_code = base_payg
```

保留 Plan 模型用于版本化计费规则，而不是展示体验版、专业版和企业版。

### 9.2 四类独立概念

```text
店铺名额
- 企业最多可以创建或绑定多少家店铺
- 继续由现有 Store quota ledger 管理

店铺续费期数
- 已绑定店铺可以运行多少个 30 天周期
- 与店铺名额独立

AI 点数
- 硕米内部统一 AI 消费单位
- 不等同于第三方模型原始 Token

数据额度
- 单位为条
- 按成功的数据调用条数扣减
```

### 9.3 企业权益

企业权益页只展示企业总资源池和已开通能力：

```text
店铺服务状态
企业 AI 点数余额
企业数据额度余额
企业已开通能力
权益来源与有效期
```

企业权益不展示成员额度。权益来源使用稳定枚举：

```text
base_plan
purchase
promotion
manual_grant
```

### 9.4 用量明细

用量页必须将资源单位和人民币成本分开：

```text
资源变动：-1.28M AI 点数
资源余额：86.50M AI 点数
折算成本：¥12.80
```

数据同理：

```text
资源变动：-8,000 条
资源余额：292,000 条
折算成本：¥8.00
```

店铺续费同理：

```text
资源变动：-2 期
资源余额：110 期
店铺到期时间：2026-11-01
```

用量页不再次扣减人民币；人民币付款、钱包余额和资源使用不能混为同一余额。只有存在可信的价格版本或购买成本快照时才显示“折算成本”；没有可信成本依据时显示 `—`，不得根据当前价格反算历史成本。

### 9.5 第一阶段资源来源

第一阶段不实现企业钱包和在线购买。店铺激活所需的续费期数、企业 AI 点数和数据额度只能来自：

- 现有订阅或权益迁移形成的初始余额；
- 受控的后台人工授予；
- 明确版本的活动赠送。

当企业没有续费期数时，激活接口返回 `INSUFFICIENT_RENEWAL_PERIODS`。前端显示真实不足状态和联系客服入口，不模拟充值成功，也不跳转到尚未实现的钱包页面。

### 9.6 第一阶段最小企业资源账本

即使企业钱包延后，店铺显式激活和真实企业余额仍需要一个可核对、可并发控制的资源账本。第一阶段新增企业级资源账本，但不包含成员钱包和成员月度限额：

```text
resource_type
├── store_renewal_period
├── ai_point
└── data_row
```

建议持久化：

```text
saas_organization_resource_buckets
- organization_id
- resource_type
- available
- reserved
- consumed
- version
- updated_at

saas_organization_resource_events
- event_id
- organization_id
- resource_type
- operation_type
- quantity
- balance_after
- idempotency_key
- request_fingerprint
- business_type
- business_id
- reversal_of
- actor_user_id
- occurred_at
```

第一阶段操作类型：

```text
grant
reserve
commit
release
reverse
expire
migration_credit
```

钱包和支付后续只需要通过已支付订单向这个企业资源账本写入 `purchase_credit`，不改变现有消费语义。

## 10. 店铺绑定与显式激活

### 10.1 为什么必须拆状态

现有 Store Center 使用单一 `LifecycleStatus`：

```text
provisioning
active
disabled
deleting
```

创建流程会在成功后自动从 `provisioning` 转为 `active`。该模型无法表达：

```text
已成功连接平台
但尚未购买或启动店铺服务
```

因此第一阶段不能继续用一个枚举同时表达连接状态、付费服务状态和记录删除状态。

### 10.2 目标状态模型

```text
ConnectionStatus
├── pending
├── connected
├── error
└── disconnected

ServiceStatus（持久化）
├── pending_activation
├── activating
├── active
├── expired
└── suspended

RecordStatus
├── active
├── deleting
└── deleted
```

“即将到期”不是持久化状态，而是根据 `service_expires_at` 与告警阈值计算出的展示状态，避免调度延迟造成状态不一致。

Store 记录增加：

```text
connection_status
service_status
service_started_at
service_expires_at
service_version
last_connection_checked_at
```

旧 `lifecycle_status` 在迁移期只用于兼容持久化读取，不能继续作为新业务的唯一状态源。硬切完成后再删除其业务职责。

### 10.3 绑定流程

```text
预留店铺名额
→ 创建 Store 记录
→ 完成平台连接或验证
→ 提交店铺名额
→ connection_status = connected
→ service_status = pending_activation
→ 不扣减续费期数
→ 不设置 service_started_at
```

绑定失败时释放名额预留；重放请求必须返回同一 Store，不得重复占用名额。

### 10.4 激活流程

```text
用户点击“激活店铺服务”
→ 展示将消耗 1 期、30 天和预计到期时间
→ 用户确认
→ 校验 Store 属于当前企业
→ 校验连接状态为 connected
→ 校验当前企业至少有 1 个续费期数
→ 幂等预留 1 期
→ service_status = activating
→ 扣减续费期数
→ service_started_at = now
→ service_expires_at = now + 30 天
→ service_status = active
→ 写入店铺服务事件和审计记录
```

激活固定消耗 1 期，客户端不能传入任意激活期数。激活失败时必须释放预留；若在资源提交后发生进程中断，恢复任务必须依据同一幂等键完成 Store 状态或执行冲正，不能出现“期数已扣但店铺仍永久待激活”。

### 10.5 续费

```text
new_expiry = max(now, current_expiry) + N × 30 天
```

提前续费不损失剩余时间。续费使用独立幂等键，重复请求不得重复扣减期数。

### 10.6 到期与停用

- 到期由可靠调度或读取时校准，将 `active` 更新为 `expired`；
- 到期只停止需要付费服务的业务执行，不删除店铺连接信息；
- 手动停用与到期分开记录；
- 重新激活已到期店铺需要再次消耗续费期数；
- 连接异常与服务到期可以同时存在，前端分别展示。

## 11. API 边界

### 11.1 账户

```http
GET   /api/workbench/account/overview
GET   /api/workbench/account/profile
PATCH /api/workbench/account/profile
GET   /api/workbench/account/business-profile
PUT   /api/workbench/account/business-profile
POST  /api/workbench/account/password/challenge
POST  /api/workbench/account/password/complete
GET   /api/workbench/organization/overview
```

手机号、邮箱和密码使用独立安全接口，不通过普通 `PATCH /profile` 修改。

### 11.2 企业权益与用量

```http
GET  /api/workbench/entitlements/overview
GET  /api/workbench/entitlements/plan
GET  /api/workbench/entitlements/benefits
GET  /api/workbench/usage/summary
GET  /api/workbench/usage/events
POST /api/workbench/usage/exports
```

### 11.3 店铺

继续复用现有店铺列表、详情、创建和编辑接口，并新增：

```http
POST /api/workbench/stores/{storeId}/activate
POST /api/workbench/stores/{storeId}/renew
POST /api/workbench/stores/{storeId}/check-connection
```

激活请求包含：

```text
idempotencyKey
expectedStoreVersion
expectedServiceVersion
```

续费请求包含：

```text
idempotencyKey
expectedStoreVersion
expectedServiceVersion
periodCount
```

所有接口从 Effective Organization 取得企业作用域，不接受任意 `organizationId` 作为授权依据。

## 12. 前端代码结构

```text
web/listingkit-ui/src/components/auth-entry/
├── auth-entry-shell.tsx
├── register-form.tsx
├── login-form.tsx
├── password-login-form.tsx
├── otp-login-form.tsx
└── reset-password-form.tsx

web/listingkit-ui/src/components/console/
├── shell/
├── navigation/
├── organization/
├── theme/
└── shared/

web/listingkit-ui/src/components/workbench/account/
├── account-overview-page.tsx
├── account-profile-page.tsx
├── account-settings-form.tsx
├── business-profile-form.tsx
└── organization-overview-page.tsx

web/listingkit-ui/src/components/workbench/entitlements/
├── entitlements-overview-page.tsx
├── billing-plan-page.tsx
├── enterprise-benefits-page.tsx
├── usage-summary-cards.tsx
├── usage-filter-bar.tsx
└── usage-event-table.tsx

web/listingkit-ui/src/components/workbench/stores/
├── store-list-page.tsx
├── store-detail-page.tsx
├── store-connection-status.tsx
├── store-service-status.tsx
├── activate-store-dialog.tsx
└── renew-store-dialog.tsx
```

共享设计 Token 与业务组件分离。Figma 导出的 Tailwind 和绝对定位代码不得直接复制到生产代码。

## 13. 错误与状态矩阵

所有首批页面必须统一覆盖：

```text
加载中
空数据
接口失败
无权限
企业未选择
企业已停用
授权已撤销
会话过期
版本冲突
重复提交
依赖不可用
```

账号入口额外覆盖：

```text
验证码发送中
验证码倒计时
验证码错误或过期
发送过于频繁
密码错误
账号锁定或不可用
```

企业权益额外覆盖：

```text
无权益记录
资源余额为零
权益已过期
能力被暂停
```

店铺中心额外覆盖：

```text
连接中
连接成功待激活
连接失败
激活中
余额不足
服务中
即将到期
已到期
手动停用
删除中
```

错误文案不得泄露内部 Provider 响应、管理 Token、Cookie、完整手机号、身份证件或店铺凭据。

## 14. 权限

第一阶段仍使用现有粗粒度 ZITADEL 角色和后端权限映射，但新增页面必须使用稳定 Capability 名称，不把菜单文案当作权限键。

建议第一阶段能力：

```text
account.profile.read
account.profile.write
account.password.manage
organization.overview.read
entitlement.read
usage.read.self
usage.read.organization
usage.export
store.read
store.create
store.update
store.activate
store.renew
store.lifecycle
store.delete
```

用户自己的密码、绑定信息和登录设备永远属于“仅本人”能力，不进入企业角色授权。

企业自定义角色和成员月度限额在后续阶段引入；第一阶段不得在前端模拟权限树保存。

## 15. 数据迁移与发布

### 15.1 视觉硬切不等于破坏性数据迁移

- 前端路由、壳层和页面直接替换；
- 现有 Store、Subscription 和 Usage 数据尽量原位演进；
- 数据库迁移必须可重复执行并支持回滚；
- 不删除旧字段，直到新读写路径稳定并完成校验；
- 不在同一发布中同时进行钱包、认证和权限系统重构。

### 15.2 现有店铺迁移

只有在历史数据能够证明有效服务期限时，才将现有 Store 迁移为 `service_status = active`：

```text
connection_status = connected
service_status = active
service_started_at = 可证明时写入，否则保持 null
service_expires_at = 来自真实历史到期数据
migration_source = 历史数据来源
```

无法证明到期时间时，不使用迁移时间伪造服务开始或到期时间，而是迁移为：

```text
connection_status = connected
service_status = pending_activation
service_started_at = null
service_expires_at = null
```

现有 `disabled` Store 在能够证明历史服务仍有效时映射为 `service_status = suspended`；否则映射为 `pending_activation`。迁移结果必须生成逐店报告，不静默猜测。

### 15.3 特性开关

建议使用：

```text
SHUOMI_CONSOLE_ENABLED
SHUOMI_SELF_REGISTRATION_ENABLED
SHUOMI_STORE_ACTIVATION_ENABLED
SHUOMI_CONSOLE_DARK_THEME_ENABLED
```

特性开关只用于发布控制，不能成为长期双系统入口。

## 16. 测试策略

### 16.1 前端

- 组件测试：导航展开、主题、企业切换、表单状态、单位格式；
- 路由测试：公开页、受保护页、合法与非法 `returnTo`；
- React Query 测试：企业切换后缓存隔离；
- Playwright：注册、两种登录、设置密码、企业权益、店铺绑定和激活；
- 视觉回归：1440×900 浅色与深色 Shell；
- 可访问性：键盘、焦点、表单标签、错误关联、对比度和减少动画。

### 16.2 后端

- 组织隔离和跨企业拒绝；
- 激活与续费状态机；
- 企业资源账本的授予、预留、提交、释放和冲正；
- 并发激活只扣减一次；
- 版本冲突；
- 店铺名额与续费期数严格分离；
- 用量单位和人民币折算分离；
- 手机验证码限流、幂等注册和邀请注册分流；
- 敏感日志扫描。

### 16.3 集成验收

```text
注册 → 验证码登录 → 进入工作台
设置密码 → 退出 → 手机号密码登录
切换企业 → 页面数据和查询缓存同时切换
查看企业权益 → AI 点数和数据均使用正确单位
绑定店铺 → 显示“已连接待激活”且不扣期数
激活店铺 → 只扣一次期数并产生 30 天有效期
余额不足 → 激活失败且 Store 保持待激活
提前续费 → 在原到期时间基础上延长
```

## 17. 交付切片

### Slice 1：账号入口

- 注册、验证码登录、密码登录、重置密码；
- ZITADEL/Auth.js 会话闭环；
- 邀请注册分流；
- 安全限流和端到端测试。

### Slice 2：Console Shell

- 新布局和导航；
- 企业上下文；
- 主题 Token；
- 移动端基础适配；
- 删除新工作台对旧 Shell 的依赖。

### Slice 3：账户基础

- 账户总览；
- 账户资料；
- 展示名称和地区；
- 设置密码；
- 经营画像；
- 企业空间总览。

### Slice 4：企业权益只读链路

- 总览；
- 计费规则；
- 企业权益；
- 用量明细；
- 单位修正和导出。

### Slice 5：店铺中心新 UI 与显式激活

- 第一阶段最小企业资源账本；
- 新列表、详情和表单；
- 连接状态与服务状态拆分；
- 绑定不计费；
- 激活和续费；
- 数据迁移、恢复、并发、幂等和审计。

这五个 Slice 分别编写实施计划并按依赖顺序交付，不生成一个覆盖全部子系统的超大计划。书面规格批准后从 Slice 1 开始。

后续项目按独立设计进入：

```text
企业自定义角色与成员管理
成员月度消费限额
企业钱包与支付
账单、退款和发票
个人与企业认证
推广收益与提现
```

## 18. 验收底线

1. 新工作台不出现旧 `/listing-kits/*` 导航或嵌入页面。
2. 注册页不要求展示名称和密码。
3. 不支持用户名登录；展示名称不承担身份唯一性。
4. 所有“AI Token”用户文案改为“AI 点数”。
5. 数据额度统一以“条”展示。
6. “我的权益”改为“企业权益”。
7. 企业认证不位于个人认证 Tab 中。
8. 企业真实资源与成员月度限额概念不混用。
9. 企业钱包未实现前不展示伪充值入口。
10. 店铺绑定成功后保持待激活，且不消耗续费期数。
11. 激活和续费具备幂等、版本控制、故障恢复和余额不足保护。
12. 店铺名额与店铺续费期数是两个独立资源。
13. 企业切换后不残留上一企业的账户、权益、用量或店铺数据。
14. 生产页面不显示 Figma 示例数字和示例用户。
15. 所有接口在后端重新验证 Effective Organization、权限、权益和资源归属。
16. 深浅主题和响应式实现不依赖逐页复制的硬编码颜色与绝对坐标。
17. 资源余额可以通过企业资源事件逐笔核对，不允许直接无流水改余额。

## 19. 设计结论

第一阶段的目标不是一次性实现 Figma 中所有后台页面，而是建立一个可以稳定扩展的真实产品基座：

```text
手机号身份入口
+ 新 Console Shell
+ 清晰的个人与企业作用域
+ 企业权益只读模型
+ 企业级资源账本
+ 可审计的店铺绑定与显式激活
```

钱包、支付、自定义权限、成员额度和认证均有清晰预留边界，但不进入当前实施计划。这样可以在不伪造数据、不混淆账本、不维护双前端的前提下，先交付可运行、可验证、可继续扩展的新硕米后台。
