# 硕米智能引擎工作台首期设计：店铺中心与 ZITADEL 多企业空间

**日期：** 2026-08-30  
**状态：** 已确认设计，已完成自审与实施计划；等待执行  
**范围：** 统一工作台框架、ZITADEL 多企业空间、店铺中心“我的店铺”前后端纵向切片  
**代码库：** `task-processor`

## 1. 背景

Figma 中的“硕米智能引擎”工作台包含运营驾驶舱、AI 工作台、供应市场、智能市场、工具市场、生态服务、数据服务、店铺中心、套餐与权益、我的账户等多个产品域。

当前代码库最成熟的产品能力仍是 ListingKit。它已经具备：

- Next.js / React 前端及现有 ListingKit 应用壳；
- Go 模块化单体后端；
- ZITADEL 登录、token introspection、项目角色和租户上下文；
- 店铺管理基础能力；
- 套餐、权益和用量账本；
- 多租户业务代码，以及不再由新工作台沿用的历史租户兼容逻辑。

因此首期不同时建设全部工作台菜单，也不更换技术栈。首期选择一个可独立验收的前后端纵向切片：

```text
统一工作台框架
  + ZITADEL 多企业空间
  + 店铺中心 / 我的店铺
```

其他工作台菜单在对应真实能力完成前保持关闭，不创建伪功能页面。

## 2. 对既有设计的修订

本设计修订 `2026-08-27-listingkit-product-design-v1-multi-tenant-addendum.md` 中以下假设：

- 旧约束：普通用户的 authenticated tenant 与 effective tenant 永远相同；只有平台管理员可以切换租户。
- 新约束：普通用户可以使用同一个 ZITADEL 身份访问多个获授权的企业空间，并显式切换 effective organization。

旧设计中关于租户隔离、缓存隔离、资源归属、平台管理员代管和后端强制鉴权的约束继续有效。

## 3. 目标

### 3.1 产品目标

1. 用户登录后可以看到自己有权访问的企业空间。
2. 用户可以显式切换当前企业空间。
3. 用户在不同企业空间中的角色可以不同。
4. 店铺、套餐和用量严格归属于当前有效企业空间。
5. 交付真实可用的店铺列表、创建、编辑、停用、启用和受控删除流程。
6. 新工作台使用新的路由与规范 Organization ID，不承担旧 ListingKit 页面、接口或数据兼容。

### 3.2 架构目标

1. 使用 ZITADEL Organization 作为企业空间身份，不复制一套 IAM 组织体系。
2. 保留组织作用域的角色授权，不再把所有角色压平成无组织归属的字符串数组。
3. 明确区分账号归属组织、获授权组织和当前有效组织。
4. 所有业务访问以服务端验证后的 effective organization 为边界。
5. 保持模块化单体，不拆微服务，不引入新的分布式事务设施。

## 4. 明确不做

首期不包含：

- 运营驾驶舱及指标体系；
- AI 工作台、智能体市场和工具市场；
- 供应市场、数据市场和生态服务；
- 完整企业组织管理后台；
- ZITADEL 管理控制台的完整替代品；
- 第三方店铺 OAuth 流程重写；
- 在线支付、充值和财务结算；
- 历史 ListingKit 页面、接口和业务数据迁移；
- 微服务拆分、消息队列或新的前端全局状态框架；
- 未实现菜单的静态占位页。

## 5. 核心术语

```text
ZITADEL Instance
└── Provider Organization
    └── ListingKit Project
        ├── Applications
        └── Project Roles

Customer Organization A  ← Project Grant / Role Assignments
Customer Organization B  ← Project Grant / Role Assignments
```

| 产品术语 | 身份或业务含义 |
|---|---|
| 用户 | ZITADEL `sub` 标识的自然人或服务身份 |
| 账号归属组织 | ZITADEL `resourceowner:id`，即 Home Organization |
| 企业空间 | 一个可承载硕米业务资源的 ZITADEL Customer Organization |
| 授权企业 | 用户通过 Project Grant / Role Assignment 可访问的 Organization |
| 当前企业 | 当前请求经后端验证后的 Effective Organization |
| 平台角色 | ListingKit Project 中定义的 `viewer`、`operator`、`admin` 等角色 |
| 业务租户 ID | 企业空间的 ZITADEL Organization ID |

ZITADEL 负责身份、组织和授权。店铺、商品、任务、套餐、用量和业务配置仍由业务数据库负责。

## 6. 身份与授权模型

### 6.1 可信身份结构

后端身份模型从单一 `TenantID` 演进为：

```text
AuthenticatedIdentity
├── Subject
├── HomeOrganizationID
├── OrganizationGrants[]
│   ├── OrganizationID
│   ├── ProjectID
│   └── Roles[]
└── EffectiveOrganizationID
```

概念契约：

```text
OrganizationGrant {
  organization_id
  project_id
  roles[]
  authorization_source
  authorization_expires_at
}
```

`resourceowner:id` 只表示账号归属组织。它不能在多企业模型中直接当作当前业务租户。

### 6.2 权限判定

每次业务操作必须同时满足：

```text
允许操作 =
  token 有效
  + 用户对 effective organization 存在有效项目授权
  + 该组织内的角色拥有目标权限
  + 业务资源属于 effective organization
  + 业务状态允许该操作
  + 套餐权益允许该操作
```

ZITADEL 项目角色映射为应用权限。ZITADEL 的 `ORG_OWNER` 等管理角色不自动等同于硕米业务管理员角色。

### 6.3 组织作用域角色

当前前后端角色解析器只保留角色名称，会丢失角色声明中的 Organization 信息。该模型只能安全支持单企业角色，不能用于企业切换。

首期必须保留以下差异：

```text
org-a → [listingkit_admin]
org-b → [listingkit_viewer]
```

禁止压平为：

```text
[listingkit_admin, listingkit_viewer]
```

否则用户切换到 `org-b` 后可能错误继承 `org-a` 的管理员权限。

## 7. ZITADEL 真实授权验证门槛

在修改应用身份模型前，先使用真实 ZITADEL 环境完成只读验证：

1. 创建或选取两个测试 Organization。
2. 使用同一个测试用户，对两个 Organization 分配不同 ListingKit Project 角色。
3. 完成真实 OIDC 登录。
4. 检查 ID token、userinfo 和 introspection 的角色声明。
5. 证明声明能稳定表达 `OrganizationID → Roles[]`。
6. 证明角色撤销、token 刷新和会话刷新后的变化行为。

验证结果决定授权来源：

- 如果 introspection 已完整返回组织作用域授权，则以已验证 token/introspection 声明为请求授权来源。
- 如果声明不能完整枚举获授权组织，则后端通过 ZITADEL API 查询 Project Role Assignments，并建立受 token 有效期约束的短期缓存。
- ZITADEL 不可用、声明不完整或授权归属无法证明时必须 fail closed。

不能根据文档示例猜测生产声明格式，也不能只验证角色名称存在。

## 8. 有效企业解析

### 8.1 请求处理顺序

```text
验证 bearer token / introspection
  → 提取 Subject 和 Home Organization
  → 加载组织作用域 grants
  → 读取用户请求选择的 Organization
  → 验证该 Organization 属于 grants
  → 计算该 Organization 内的角色与权限
  → 写入请求级 EffectiveOrganizationID
  → 执行业务处理
```

### 8.2 选择状态

前端可使用受保护、HttpOnly、SameSite Cookie 保存“上次选择的企业”，用于恢复用户体验。该 Cookie 只是一项偏好，不是授权凭证。

API 请求中的有效企业选择也是不可信输入。无论选择来自 Cookie、请求头、路径还是查询参数，Go 后端都必须根据已验证 grants 重新授权。

现有中间件把 `resourceowner:id` 直接写入 `TenantID` 的行为需要拆分：

1. Authentication middleware 只验证身份并清除伪造身份头。
2. Effective organization resolver 验证目标 Organization。
3. 只有 resolver 成功后，业务上下文才获得 EffectiveOrganizationID。

### 8.3 默认选择

登录后按以下顺序选择默认企业：

1. 上次选择且仍有权限的企业；
2. Home Organization 且拥有 ListingKit 项目授权；
3. 唯一的可访问企业；
4. 否则要求用户选择企业。

如果用户已经没有任何获授权企业，显示明确的无企业访问状态，不回退到默认租户或公共租户。

## 9. 工作台上下文接口

### 9.1 读取上下文

```http
GET /api/workbench/context
```

响应概念结构：

```json
{
  "user": {
    "id": "user-1"
  },
  "homeOrganizationId": "org-a",
  "effectiveOrganizationId": "org-b",
  "selectionRequired": false,
  "organizations": [
    {
      "id": "org-a",
      "name": "硕米科技",
      "roles": ["listingkit_admin"]
    },
    {
      "id": "org-b",
      "name": "星海贸易",
      "roles": ["listingkit_viewer"]
    }
  ]
}
```

首次登录且存在多个可访问企业、但没有有效默认选择时，接口仍返回 `200` 和完整企业列表，并返回 `"effectiveOrganizationId": null`、`"selectionRequired": true`。这样前端可以展示选择器；在用户完成选择前，任何企业业务资源接口仍然拒绝执行。

返回角色用于界面提示和交互优化，不能替代后端鉴权。

### 9.2 切换企业

```http
PUT /api/workbench/context/effective-organization
```

请求：

```json
{
  "organizationId": "org-b"
}
```

后端验证目标企业授权后更新选择状态，并返回新的完整工作台上下文。以下情况拒绝切换：

- Organization 不在用户 grants 中；
- Organization 已停用或业务侧被冻结；
- 角色声明缺失或无法确定作用域；
- ZITADEL 授权查询失败且没有仍然有效的可验证声明。

### 9.3 首期授权供给

首期工作台只消费已经建立的 ZITADEL Organization、Project Grant 和 Role Assignment，不在工作台内新建企业、邀请成员或配置 ZITADEL 管理角色。

测试企业和成员授权通过现有 ZITADEL 管理流程或经过验证的现有管理能力预先配置。后续如果建设企业自助入驻与成员邀请，应作为独立纵向切片设计，不能把 ZITADEL 账号创建、企业成员关系和 ListingKit 项目角色分配合并成一个不可审计的操作。

## 10. 企业目录投影

业务数据库可保留最小企业目录投影：

```text
EnterpriseDirectoryEntry
├── organization_id
├── display_name
├── business_status
├── last_synced_at
└── version
```

该投影用于显示企业名称、业务冻结状态和减少目录查询，不是成员授权的最终事实来源。

禁止把店铺、套餐或业务配置写入 ZITADEL metadata。也不新增一套与 ZITADEL Organization ID 无法一一对应的企业主键。

## 11. 绿地数据边界与重置

现有历史租户、店铺和相关业务数据已明确允许废弃。新工作台不执行历史数据迁移，也不提供旧租户兼容读取。

要求：

1. ZITADEL Organization ID 是新业务边界唯一的规范租户 ID。
2. 新工作台路径不得调用 tenant bridge，也不接受历史数字租户 ID。
3. 不执行旧店铺归属回填、双写、双读或兼容转换。
4. 新数据从空状态开始，并从创建时写入非空 Organization ID。
5. 企业内唯一索引、外键和缓存键从第一版就包含规范 Organization ID。
6. 旧 ListingKit 页面和接口不作为回退路径，也没有功能对等要求。
7. 实际重置数据前，实施计划必须列出准确表、外键依赖、重置顺序和重置后的空库验证；本设计确认数据可废弃，但不在设计阶段执行删除。

## 12. 模块边界

```text
Workspace Context
├── ZITADEL Identity / Grants
├── Enterprise Directory Projection
└── Effective Organization Resolver

Store Management Application Service
├── Workspace Context
├── Authorization Policy
├── New Store Center Domain
├── Subscription / Store Quota Ledger
└── Audit Service
```

职责：

- Workspace Context 负责当前企业和组织作用域授权。
- Store Center Domain 拥有新工作台的店铺资料、平台类型、连接状态和生命周期。
- Subscription Domain 继续拥有套餐、权益和用量。
- Workbench Shell 只消费上下文和导航配置，不承载店铺业务规则。
- 各模块通过公开接口协作，不直接查询彼此的数据表。

现有 `listingadmin.Store` / `listing_store` 使用数值型 `TenantID`、数值型店铺 ID，并包含可读的用户名与密码字段；它不满足本设计的规范 Organization ID、不可枚举凭据和绿地数据边界，因此不得作为新工作台店铺聚合或数据表复用。可以通过显式适配器复用已经验证的平台登录、授权状态探测等能力，但适配器不能把旧租户 ID 或明文凭据带入新领域。

## 13. 前端结构

### 13.1 路由

```text
/workbench/stores
/workbench/stores/new
/workbench/stores/[storeId]
```

新工作台路由是首期唯一目标入口。旧 ListingKit 路由没有并行兼容或功能对等要求，可在切换时关闭或移除。首期验收前只向内部账号开放，避免把尚未完成的工作台作为完整生产产品。

### 13.2 应用壳

```text
WorkspaceAppShell
├── OrganizationSwitcher
├── WorkbenchNavigation
├── PageHeader
└── UserMenu
```

新建中立的 `WorkspaceAppShell`。可以复用现有低层布局、样式变量和通用组件，但不保留 `ListingKitAppShell` 兼容适配器，也不复制一套长期并存的旧导航。

### 13.3 企业切换体验

- 只有一个可访问企业时可以简化切换控件，但仍显示当前企业。
- 多企业用户可以在全局壳中切换。
- 切换时清除企业相关查询、分页、勾选项、草稿和批量操作状态。
- 平台管理员代管其他企业时持续显示“正在代管”状态。
- 高风险操作的确认文案必须包含目标企业名称。

### 13.4 菜单策略

导航由功能开关、企业内权限和套餐权益共同生成：

- 未实现功能：隐藏；
- 已实现但无权限：按产品语义隐藏或禁用并说明；
- 套餐未包含：可展示升级入口，但不能进入最终必然失败的操作流程；
- 后端始终重新鉴权。

## 14. “我的店铺”功能范围

首期包括：

- 当前企业的店铺列表；
- 平台和状态筛选；
- 分页；
- 创建店铺；
- 编辑基本资料；
- 停用与重新启用；
- 受控删除；
- 真实平台连接状态；
- 套餐额度及升级提示；
- 空状态、无权限、授权失效和系统错误状态。

平台授权流程继续复用现有实现。

## 15. 店铺应用服务与一致性

### 15.1 创建流程

```text
验证请求与幂等键
  → 验证 effective organization 和 store.create 权限
  → 原子申请店铺额度
  → 调用新 Store Center 领域创建
  → 写入审计记录
  → 提交
```

优先复用现有订阅基础设施提供原子额度申请。现有通用用量账本目前只接受作业计数和存储容量指标，不能直接承载可增可减的店铺席位；实施时应在 Subscription Domain 内增加专用、幂等的店铺额度分配账本，而不是放宽既有账本的指标约束或在店铺模块复制套餐数据。店铺模块不能采用无锁的“先查数量，再插入”。

如果当前事务设施不能安全跨模块共享，则使用幂等的“额度预留 → 创建 → 确认”状态机，并提供失败释放机制。

### 15.2 额度规则

首期规则：所有未删除店铺均占用额度；停用不释放额度；受控删除成功后才释放额度。

首期套餐技术默认值为：基础版 1 家、专业版 5 家、企业版 20 家。企业级 entitlement 可以覆盖套餐默认值；缺少有效订阅或 `store_count` 限额时禁止创建，不解释为无限额度。

### 15.3 并发规则

- 创建请求支持幂等键；
- 编辑使用版本号或 `If-Match`；
- 额度占用必须并发安全；
- 企业切换不能重新解释已经开始的 mutation；
- 长任务、重试和工作流必须保存启动时的 effective organization。

## 16. 安全约束

1. `sub` 是规范用户身份，用户名和客户端身份头不能回退成为所有权依据。
2. 企业选择是输入，不是授权证明。
3. 跨企业访问不能泄露资源是否存在。
4. 第三方凭证只写不读，接口只返回脱敏状态。
5. 所有认证工作台路由必须经过 ZITADEL authentication 和 effective organization resolver。
6. 不允许认证路由使用默认租户回退。
7. 本地企业目录缓存不能赋予 ZITADEL 已撤销的权限。
8. 审计和日志不能包含 token、密钥或完整敏感资料。

## 17. 错误协议

统一错误结构：

```json
{
  "code": "ORGANIZATION_ACCESS_REVOKED",
  "message": "你已无权访问当前企业空间",
  "requestId": "req_xxx",
  "fieldErrors": []
}
```

至少包含稳定错误码：

- `AUTHENTICATION_REQUIRED`
- `ORGANIZATION_SELECTION_REQUIRED`
- `ORGANIZATION_ACCESS_DENIED`
- `ORGANIZATION_ACCESS_REVOKED`
- `ORGANIZATION_SUSPENDED`
- `PERMISSION_DENIED`
- `STORE_LIMIT_REACHED`
- `STORE_ALREADY_EXISTS`
- `STORE_VERSION_CONFLICT`
- `STORE_AUTHORIZATION_EXPIRED`
- `DEPENDENCY_UNAVAILABLE`

前端根据错误码处理，不解析后端错误文本。

## 18. 缓存与撤权

- 所有企业相关缓存键包含 EffectiveOrganizationID。
- 企业切换和所有业务写操作必须在请求时通过权威 grant verifier 实时校验，不得使用授权缓存。
- “实时校验”是指本次请求执行 introspection 或查询当前 ZITADEL Project Role Assignment；具体来源由第 7 节真实授权验证结果决定。
- 普通读取请求可以使用已经验证的组织作用域授权缓存，但缓存最长 60 秒，并且不得超过 token 或授权声明的剩余有效期。
- 授权缓存键至少包含 Subject、OrganizationID、ProjectID 和授权契约版本；缓存内容不得包含 token 明文。
- 显式切换企业时始终绕过并清除目标用户的相关授权缓存。
- ZITADEL 不可用时，切换和写操作 fail closed；读取只有在 60 秒以内且仍受有效 token 约束的已验证缓存存在时才可继续。
- 撤权对切换和写操作立即生效；已经缓存的读取权限最多延迟 60 秒失效。超过 60 秒后无法重新证明授权则 fail closed。
- 权限敏感写操作不得依赖企业目录投影或读取授权缓存。

## 19. 审计与可观测性

记录：

- 企业切换；
- 企业访问拒绝；
- 店铺创建、编辑、停用、启用和删除；
- 额度申请、确认和释放；
- 第三方授权状态变化。

审计字段至少包含：Subject、HomeOrganizationID、EffectiveOrganizationID、资源、动作、结果、时间和 RequestID。

指标至少包含：

- 工作台上下文加载失败率；
- 企业切换成功率及拒绝原因；
- 店铺创建成功率及失败原因；
- 套餐额度拒绝次数；
- 授权声明解析失败次数；
- ZITADEL 授权查询延迟和失败率。

## 20. 测试策略

### 20.1 ZITADEL 真实链路验证

建立以下授权矩阵：

| 用户 | Organization A | Organization B | 预期 |
|---|---|---|---|
| 用户 1 | admin | viewer | 切换后权限随企业改变 |
| 用户 2 | operator | 无权限 | 无法进入 B |
| 平台管理员 | delegated admin | delegated admin | 显示代管状态 |
| 已撤权用户 | 无权限 | viewer | A 的旧选择失败关闭 |

验证真实登录、token、userinfo、introspection、刷新和撤权行为。

### 20.2 单元测试

- 组织作用域角色解析；
- 默认企业选择；
- role-to-permission 策略；
- 店铺状态转换；
- 套餐额度规则；
- 错误码映射。

### 20.3 集成测试

- 企业目录同步；
- 空数据模型初始化；
- 数据重置脚本的目标约束与空库验证；
- 跨企业隔离；
- 幂等创建；
- 并发额度；
- 失败回滚或补偿；
- 并发编辑；
- 授权撤销，包括切换和写操作立即拒绝、读取缓存不超过 60 秒。

### 20.4 前端与端到端测试

- 多企业切换；
- 切换后清空缓存和选择；
- 不同企业菜单和按钮权限；
- 创建、刷新、编辑、停用和删除；
- 额度上限提示；
- 无企业、无权限、撤权和依赖失败状态；
- 旧 ListingKit 路由已关闭且新工作台不读取旧数据。

## 21. 验收标准

1. 同一用户可访问两个企业，并在两个企业中拥有不同角色。
2. 切换企业后，店铺、套餐、菜单和权限同步变化。
3. 修改 Cookie、请求头、URL 或请求体不能访问未授权企业。
4. 用户不能用 Organization A 的管理员角色操作 Organization B。
5. 用户无法判断另一企业的店铺 ID 是否存在。
6. 创建店铺真实持久化，刷新后存在。
7. 重复提交只创建一个店铺。
8. 并发创建不能突破套餐上限。
9. 并发编辑产生明确冲突，不静默覆盖。
10. 停用不释放额度，受控删除才释放。
11. 授权撤销后，企业切换和写操作立即失败关闭，读取权限在 60 秒内失效。
12. 旧 ListingKit 数据不会出现在新工作台，旧路由不承担兼容责任。

## 22. 发布与回退

1. 先在内部环境完成准确的数据重置和空库验证。
2. 通过功能开关向内部多企业测试账号开放。
3. 先验证 ZITADEL 授权结构，再启用企业切换。
4. 先启用工作台上下文，再启用店铺新页面。
5. 授权作用域异常时关闭新入口；由于不保留旧页面兼容，该操作只阻止继续使用，不提供旧产品功能回退。
6. 已创建的新规范数据不转换回历史数字租户模型。

## 23. 方案选择与取舍

### 已选择：现有技术栈上的纵向切片

优点：复用真实店铺、订阅和 ZITADEL 能力，可较早验证完整业务价值；改动边界清晰，可通过功能开关回退。

### 未选择：纯前端壳优先

原因：会产生大量空页面和临时数据，无法验证多租户和套餐边界。

### 未选择：先进行全面后端重构

原因：交付周期过长，容易提前设计尚未验证的领域，并扩大首期改动范围。

### 未选择：自建组织、成员和登录体系

原因：与现有 ZITADEL 重复，增加身份同步、撤权延迟和安全风险。

## 24. 实施前门槛

进入代码实施前必须完成：

1. 本设计规格经用户最终确认；
2. 编写独立实施计划；
3. 真实 ZITADEL 两组织、同用户、不同角色的声明验证方案写入计划；
4. 列出需要重置的店铺、租户和套餐相关表及外键依赖，并确认可复用的领域代码；
5. 明确现有认证路由是否全部经过 authruntime middleware；
6. 在独立 `codex/` 分支或 worktree 上实施；
7. 以测试驱动方式完成每个可合并切片。
