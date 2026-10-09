# Account Audit 只读用量来源接线 V1

状态：`IMPLEMENTATION_READY`。执行 Issue #587；独立只读 reviewer `audit_usage_arch_review` 对 design-only HEAD `088bbe236dd01bf641407c92d96d911d48ce1e13` / 本文 blob `f7a3a8e4a0e2f78f55def362b349d5eb70700ea1` 评审，设计级 `BLOCKER=0`。实施层需证明只读角色边界和无 Agent 执行副作用、全新与保留项目初始化分支、audit-only 与 Agent 执行配置组合；三项归类 `IMPLEMENTATION_TEST`，阻本片合并，不阻设计准入。

## 当前用户结果与依据

企业管理员或获准成员在全新隔离 Account Center 安装中打开“企业空间 → 操作记录”，能按内容、时间、受影响成员定位已提交的账户与资源事件。#581 / PR #582 已交付六来源的查询与页面合同，但基础 Compose 没有 ImageAgent、ProductAgent 用量 pool；页面默认 `period=30d` 导致查询按合同返回 503，连基础账户事件也无法查看。2026-10-03 的原 22444 合成实例只读检查确认：四卡读到 5/0/0/5，列表 503；两个 Agent 均未在运行清单配置。用户随后批准准备独立、全来源的合成实例并重跑浏览器验收。

Product Authority：Issue #581、#469、用户已确认的成员筛选语义及 Figma `tg48P46SSXl6TBy9lZwg63 / 1532:525`。现行合同见 [完整历史筛选 V1](account-audit-filters-v1.md) 和 [四卡 V1](account-audit-summary-v1.md)。当前全新安装基线见 [PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08](../product/greenfield-no-legacy-migration.md)。本设计不改 UI、搜索语义或任何既有事件事实。

本轮可见结果：新项目中的 Account Audit 列表实际可读，默认近 30 天、近 7 天、全部时间、内容和目标成员筛选均可作用于真实已提交的合成账户事件；不存在 AI 调用时两个独立的、已初始化的用量账本真实为空。结果不是开启 AI 生成，也不是把缺失池解释为 0。

Out of Scope：修改原 22444 实例或迁移其历史；启用 Product/Image Agent 执行模块、provider、价格、模型、Temporal worker；生成假用量；新增统一审计平台/缓存/第二事实源；改变四卡、权限、租户、游标、搜索、时间或成员语义；真实数据、生产部署与产品最终签收。

## 事实 owner 与最小装配

两个 `ai_invocations` 账本继续分别属于 ImageAgent 与 ProductAgent。Account Audit 只读现有 `aicapability/store.GormInvocationRecorder.ListObservedUsage` 投影，不创建自己的审计行。全新项目的 `image_agent` owner DB 已在基础数据库列表中；增加独立 `product_agent` owner DB，以便未来 ProductAgent 使用同一个事实位置。两库各自安装现有 `AutoMigrateInvocationLedger` 定义的 `ai_invocations`，不引入新 DTO、同步、回填或双写。账本安装仅面向全新项目；旧实例不自动补库、不宣称其缺失历史为空。

为避免为了读历史而启用 Agent 写入/调用，当前应用清单增加**有界、可选的 Account Audit 用量读取配置**，只列出 `image` 与 `product` 两个现有 owner DB 目标。两个 pool 使用 `platform/database.OpenExistingReadOnlyContext` 和各自的只读登录角色；角色仅有对应数据库的 `CONNECT`、public schema 的 `USAGE` 与 `ai_invocations` 的 `SELECT`，无表写入、schema CREATE、序列或其他 owner 权限。应用不接收 provider key，也不因该配置挂载 Agent 路由或启动 worker。各 pool 的宿主/端口/数据库必须与本项目两个 owner 一致且相互独立；不能把同一物理表冒充两个 namespace。

`contract → implementation → injection → consumer`：

1. 现有两个 `ai_invocations` owner 和 `ListObservedUsage` 保持原查询/排序/组织条件；全新安装的 owner 初始化器仅安装原表并授予读角色。
2. 当前应用运行时从私有清单打开两个有界只读连接，验证目标 schema/SELECT 可用，构造 Account Audit 用量源映射 `{image, product}`。现有 Agent 执行配置不存在时，仅这两个读 pool 进入 Audit；Agent 执行配置存在时沿原各 Agent pool。为避免来源歧义，显式同时配置同一 namespace 的执行 pool 与审计专用 pool 必须拒绝启动，除非后续单独设计同池绑定；本轮不增加猜测或 fallback。
3. `aiUsageAuditReader.Complete()` 仍要求 image/product 均存在；`ReadFiltered`、HTTP、BFF、client、页面保持 #581 合同。`ReadSummary` 仍排除 observed-only usage。

每个 namespace 独立选择上述已批准来源。例如启用 ProductAgent、未启用 ImageAgent 时，保留 `accountAuditUsage.image` 专用只读配置，省略 `accountAuditUsage.product`，product 用量由当前 ProductAgent owner pool 提供。只打开显式声明的专用 reader；同 namespace 同时声明执行与专用 reader、错误 owner、缺少任一来源仍拒绝启动。这是既有唯一来源合同的组合接线，不开放新的权限或 provider。

读连接失败、配置缺一个、表或 SELECT 不可用时不得进入“空历史成功”。配置不完整的通用应用可以照原行为启动，但筛选返回 503；明确声明全源的隔离项目必须在启动前验证两个 pool，任一失败则当前应用不监听。数据库不可用后的请求失败按原 `DEPENDENCY_UNAVAILABLE` 或 deadline 表达，不返回已扫描的部分页。身份、组织权限仍由原 HTTP 与各 owner 查询限定；新的 pool 不接收跨组织任意搜索入口。

## 全新隔离安装与持久化边界

基础 Account Compose 在**新项目**的 bootstrap 阶段生成两个审计读角色与 ProductAgent schema owner 的私有随机凭据；业务 PostgreSQL 首次初始化创建 `product_agent` DB 和两个只读角色，已有 `image_agent` DB 保持其 owner。schema-init 通过一个有界的原账本初始化入口，对两个独立 DB 调用现有 `AutoMigrateInvocationLedger`，再授予只读权限，最后才写出包含两个只读目标的私有当前应用清单。启动期只验证并读取，不执行 DDL。原有各卷、证书、身份与数据库均按项目名隔离。旧 Account Base 项目不能用新增清单冒充“从安装之初完整”，也不在本次修复中迁移；仍用原 checkout 保留。

这些账本即使没有运行 Agent，也是本全新安装自创建之日起的完整空事实位置；不会把 5 条资源合成事件转换成 AI 用量。后续若要在该项目开启 Agent，必须明确复用对应 owner 数据库并完成该 Agent 自己的独立 schema、授权和准入；本设计不自动开放执行能力。启动、停止与删除独立：`stop` 保留卷，任何清理/删除需单独授权。

## 安全与失败验证

- 读角色和连接：用 PostgreSQL 权限探针证明两个角色可 `SELECT ai_invocations` 且不能 INSERT/UPDATE/DELETE、建表或读取另一 owner；运行时会话同时为 read-only。缺表、缺权限、错库、缺源、连接失败一律失败，不能返回 0。
- 查询：合成企业 A 已提交账户/资源事件时默认列表 200；按内容/目标成员/7d/30d/all 及既有操作人、操作类型组合在全历史中命中；企业 B 与撤权路径仍隔离；空用量账本仅贡献 0 条 observed usage。原游标和完整扫描测试复用 #581，针对新 pool 接线加必要组合测试。
- 无副作用：启动后 Agent 执行路由仍未挂载、无 provider/Temporal 调用；读取前后 owner 事实表一致。四卡仍独立读已提交成功事件。
- 交付：独立新项目、私有凭据与可访问入口，作者只记录开发和浏览器证据；用户/指定独立验证者决定产品验收。PR CI、设计复核、最终复核与运行结果分别记录；不凭绿色测试关闭 #581/#587。

Legacy decision：N/A。全新安装沿现有两个 invocation ledger owner，绝不引入旧 Task/Queue/兼容或历史数据迁移。
