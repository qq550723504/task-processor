# Internal 目标架构迁移

## 背景

仓库已经批准了模块化单体的目标架构，但当前 `internal` 目录仍然是迁移态布局，而不是稳定的包架构。当前 Windows 默认构建目标下，`internal` 包含 303 个 Go 包和 59 个一级目录。主要结构热点如下：

- `internal/listingkit` 仍是主要复杂度汇聚点。其根包包含 560 个生产文件，并直接导入 55 个内部包。
- `internal/listingadmin` 将不相关能力的模型、DTO、仓储端口、GORM 实现、处理器和管理兼容 API 混在同一个包中。
- 基础设施分散在 `internal/core`、`internal/infra`、`internal/platform` 下的目标占位目录，以及历史 marketplace 包中的具体客户端里。
- marketplace 注册职责分散在 `platform`、`platforms`、`platformbase` 和 `platformtask`；`internal/platforms/shein/module.go` 还同时承担 scheduler、prompt、watchdog 和分片装配。
- 相对默认日志路径和全局 logger 的延迟初始化，会在测试期间于各包工作目录下创建 `tmp/logs`。
- 现有护栏能阻止部分已知错误导入，但某些结构测试检查的是文件白名单，而不是真正的职责或依赖边界。

本次迁移作为一个完整的架构治理项目，在同一个治理分支上完成并最终一次合并。实施过程使用一系列可审查的连续提交，以便定位故障，并可只回退未完成的当前切片，而不撤销已经验证的底层迁移。

## 目标

将完整的 `internal` 目录收敛到已经批准的、以业务领域为先的目标架构；退休语义模糊的历史根包，并在代码与测试中落实以下目标依赖方向：

```text
cmd
  -> app

app
  -> listing / product / marketplace
  -> agent / knowledge / resourcecatalog
  -> commercial / ledger / organization
  -> platform / integration

integration
  -> 各领域的窄 contract 包
  -> shared

各领域
  -> shared

platform
  -> shared

compatibility
  -> 对应目标领域
```

图中的箭头表示允许的源码依赖。领域包不导入具体 platform 或 integration 实现；它们只声明自身消费的端口。`app` 负责选择实现和装配。integration 只有在实现端口时才允许依赖领域公开的窄 contract，不能依赖领域 service、workflow 或 handler。

只有当废弃包在仓库中不再有调用方且已被删除时，迁移才算完成。只移动文件而不修正所有权或依赖方向，不满足本设计目标。

## 非目标

- 不把模块化单体拆成多个服务。
- 不因为移动代码而顺带重写产品行为。
- 当不同 marketplace 确实需要不同能力时，不为了目录对称而强行统一。
- 不为了源码兼容而无限期保留有缺陷的内部 API。
- 不重复实现第二套依赖分析框架；扩展仓库已有的 `golangci-lint` depguard 规则和 Go 架构测试。
- 不把外部协议重设计与无关的包迁移混在同一个变更中。

## 迁移原则

1. 修正有缺陷的内部设计，并迁移仓库中的全部调用方。
2. 每次移动前先用特征测试和契约测试固定可观察行为。
3. 当外部 HTTP、队列、Temporal、配置或数据库契约必须变更时，在同一个切片中提供版本化、双读写或明确的数据迁移。禁止未经说明的破坏性切换。
4. 兼容包只能做翻译和转发，不能承担业务判断、持久化、外部客户端构造或运行时生命周期管理。
5. 每个兼容入口都必须有明确的替代入口、仓库级零引用删除条件和删除检查点。
6. 新代码只能进入目标所有者。历史包是迁移来源，不再是新职责的合法归属地。
7. 每个提交都必须保持仓库可构建，并确保适用的聚焦测试通过。

## 目标包所有权

### `internal/shared`

负责不含产品或 marketplace 业务含义的小型稳定基础能力，例如通用错误、时间工具、分页、校验原语，以及真正跨域共享的身份信封。它不能成为 `pkg`、`core` 或其他杂物目录的替代品。

### `internal/platform`

负责应用自身的运行时基础设施：配置加载、日志、可观测性、数据库启动与迁移、Redis、队列和 Temporal 运行时支持。它向 app 装配层提供与业务无关的运行时能力，不实现领域仓储，不拥有具体外部服务商客户端。

### `internal/integration`

负责出站适配器和外部系统适配器：OpenAI、图像服务商、S3、Playwright、marketplace API、crawler/browser、ZITADEL、Casbin，以及按领域组织的数据库仓储适配器。服务商 SDK、GORM 模型和具体客户端必须留在 integration 包内；领域包只看到自身定义的端口和模型。

### `internal/product`

负责 canonical catalog facts、资产、商品图片、内容增强和标准化 sourcing。它不能依赖 listing 编排、marketplace workspace、HTTP 装配或具体 integration 实现。

### `internal/marketplace/<name>`

负责各 marketplace 特有的 publishing、workspace/editor 行为、模型、校验、定价、分类、属性和 payload 策略。运行时客户端属于 integration 适配器；进程启动、scheduler 生命周期和 worker 注册属于 app。

### `internal/listing`

负责跨 marketplace 的 listing 任务生命周期、workflow、preview、export、revision、submission、studio 和 settings。它可以通过窄契约使用 product facts 和 marketplace 能力，但不能吸收 marketplace 规则。仅 listing 特有的授权映射或用量策略留在 listing；通用套餐、权益、配额和计量不属于 listing。

### `internal/agent`

负责 agent 定义、模型能力抽象、能力路由、工具调用策略，以及由 agent 业务拥有的 prompt 资产。它不能包含 OpenAI 等服务商 SDK，也不负责知识文档的存储与检索。

### `internal/knowledge`

负责知识文档、切片、检索、引用和向量检索端口。pgvector 等具体向量存储实现属于 integration；只有领域契约稳定后才引入适配器。

### `internal/resourcecatalog`

负责 agent、tool、service 和 data 等可复用资源的目录内核，包括条目、版本、发布者、可见性、审核和订阅关系。它与 Amazon、TEMU、SHEIN 等销售渠道的 `internal/marketplace` 明确分离。

### `internal/commercial`

负责通用套餐、权益、用量计量、用量账本、配额与计费协调。当前 `listingsubscription` 中可复用的订阅和 usage ledger 能力迁入此处；listing 只保留 listing 特有策略。

### `internal/ledger`

只负责真实货币或平台余额：不可变复式分录、充值、扣款和退款。它不承载调用次数、token 或任务量等 usage event。没有确认真实资金需求前，不引入 TigerBeetle，也不为了目录完整提前建设此域。

### `internal/organization`

负责组织、成员、租户归属和业务授权语义。ZITADEL 验证/管理客户端与 Casbin engine 属于 integration，HTTP middleware 和生命周期装配属于 app；迁移期租户桥接属于 compatibility。

### `internal/app`

负责 HTTP server、worker、scheduler、consumer 和 Temporal worker 的进程装配。它构造具体的 platform 和 integration 资源，将其适配到领域局部接口，注册路由和 worker，并拥有资源生命周期。它不能包含 marketplace 决策规则。

### `internal/compatibility`

只负责在有期限的切换期内保留确实面向外部的旧入口和 DTO 翻译。仓库内部兼容应通过迁移全部调用方来消除，而不是长期保留转发别名。

## 历史根包退休策略

以下通用或语义重叠的根包仅作为迁移来源，不是长期所有者：

- `internal/listingkit` 和 `internal/listingadmin`
- `internal/core`、`internal/infra`、`internal/pkg`、`internal/model` 和 `internal/domain`
- `internal/platforms`、`internal/platformbase` 和 `internal/platformtask`
- `internal/shein`、`internal/temu`、`internal/amazon` 等历史 marketplace 实现根包
- `internal/ai`、`internal/aicapability`、`internal/prompt` 和 `internal/promptmgmt` 等混合 AI 根包
- `internal/listingsubscription`、`internal/authz`、`internal/authidentity` 和 `internal/authruntime` 等混合商业或身份根包
- 职责将迁入 `internal/listing` 的其他重叠 listing 根包

退休必须按能力推进，不能盲目重命名目录。只有在明确文件所有者、依赖契约、调用方和行为测试后，才允许移动文件。

## 依赖与构造规则

- 领域包定义自身消费的最小接口。
- app 装配层选择具体实现并完成注入。
- 领域之间不直接导入对方实现；跨域调用由消费领域定义局部 port，由 app 编排或注入 bridge。
- 需要被 adapter 实现的端口和稳定模型放在所属领域的聚焦 `port` 或 `contract` 包中，不能重新建立全局 `internal/ports`。
- integration 可以依赖领域的窄 `port` 或 `contract` 包来实现端口，但不能依赖领域 service、workflow、handler 或 app 类型。
- platform 不能导入任何领域包；领域包也不能导入 platform、integration 或具体 SDK 类型。
- 如果聚焦的 options 值已经足够，业务 pipeline 就不能接收完整的应用全局配置。
- 业务 pipeline 不能构造 OpenAI、数据库、队列、对象存储、浏览器或 marketplace API 客户端。
- marketplace 包通过窄依赖暴露明确的构造器或服务；不能为了实现某个接口而在方法签名中被迫导入 app 运行时类型。
- app 可以导入 marketplace 包；marketplace 包不能反向导入 app。
- 对象存储端口由消费领域定义，S3 adapter 属于 integration，bucket/client 的构造和生命周期属于 app。
- 业务权限常量和判断归所属领域或 organization；Casbin adapter 属于 integration，认证 middleware 属于 app。
- compatibility 可以向内依赖目标领域；目标领域绝不能反向依赖 compatibility。

因此，现有 platform module registry 将由 app 所有的显式装配替代。marketplace 包可以暴露 descriptor 和构造器，但 scheduler、watchdog、prompt store、分片和 worker 生命周期必须留在 app builder 中。

## 外部组件准入与时机

| 组件 | 目标归属 | 准入条件与时机 |
| --- | --- | --- |
| Goose | `platform/database/migration`，由 app 启动 | 阶段 2 以真实 migration 路径替换现有分散启动逻辑，并有升级/失败测试 |
| OpenTelemetry | `platform/observability`，由 app 装配 | 阶段 2 至少贯通一个 HTTP 或 worker 纵向链路，并验证禁用时无业务侵入 |
| OpenFeature | platform adapter + 消费领域局部 flag port | 阶段 2 贯通一个现有开关；领域不得导入 OpenFeature SDK |
| Promptfoo | 仓库根评测资产 | 独立 P0，不作为生产 Go 依赖，也不混入 platform 迁移提交 |
| MCP | `integration/mcp` | agent 工具契约和生命周期明确后的阶段 6 纵向切片 |
| pgvector | `integration/knowledge/pgvector` | knowledge 检索与向量端口明确后的阶段 6 适配器 |
| TigerBeetle | `integration/ledger/tigerbeetle` | 仅在 ledger 出现真实资金需求、记账不变量和运维方案后评估 |
| 前端 P0 | 独立前端工作流 | 可并行规划，但不作为第二阶段后端基础迁移的完成条件 |

## 迁移顺序

### 阶段 1：基线与运行时副作用

1. 先增加一个失败的文件系统级回归测试，证明包测试不能在 `internal` 下创建日志。
2. 将 logger 的库代码和测试默认行为改为只输出 stdout；只有 app 启动时才能显式选择仓库运行目录或另一个已配置的绝对运行目录下的文件路径。
3. 测试确实需要文件日志时，使用 `t.TempDir` 作为输出位置。
4. 加强仓库产物护栏，使其检查测试运行涉及的实际文件系统状态，而不是只检查 Git 已跟踪路径。

现有的忽略产物单独报告。未获得明确授权时，实施过程不得删除用户工作区数据。

### 阶段 2：shared、platform 和 integration 基础

1. 盘点 `core`、`infra`、技术型 `pkg`，同时标记 AI、身份、授权和订阅根包中的混合职责；本阶段只迁移基础能力，不借机建设未来业务功能。
2. 先记录旧根包文件数、依赖数和禁止新增引用的基线，并增加单向收敛护栏。
3. 将真正稳定、无业务语义的原语迁入 `internal/shared`；禁止以迁移为由形成新的通用杂物包。
4. 将配置、日志、可观测性、数据库启动与迁移、Redis、队列和 Temporal 运行时所有权迁入 `internal/platform`。
5. 将现有具体服务商客户端、S3、crawler 和持久化 adapter 迁入 `internal/integration`；先不引入 MCP 或 pgvector。
6. 以真实纵向切片接入 Goose、OpenTelemetry 和 OpenFeature：必须有 app 装配、配置、运行路径和测试，不能只增加未使用依赖。
7. 先更新 app 的构造和生命周期所有权，再迁移业务调用方；随后增加 depguard 和语义测试，阻止业务包导入具体 adapter 或 SDK。
8. 证明旧入口零引用后删除对应旧实现；Promptfoo 评测和前端 P0 分开实施，不混入后端基础设施提交。

### 阶段 3：product 领域

1. 将 catalog、asset、product image、enrichment 和 sourcing 收敛到 `internal/product`。
2. 在 product 边界引入或保留局部 port，并迁移调用方。
3. 零引用扫描通过后，删除面向 product 的别名和通用历史模型。

### 阶段 4：marketplace 领域

1. 先迁移稳定的策略与 payload 模型，再迁移运行时耦合较重的 pipeline。
2. 将 marketplace API adapter 分离到 integration 包。
3. 用注入的局部接口和聚焦 options 值替换 pipeline 中的客户端构造。
4. 按能力将历史 SHEIN、TEMU、Amazon 和 Walmart 行为迁入对应 marketplace 所有者。
5. 运行时装配留在 app，不能进入 marketplace 注册文件。

### 阶段 5：listing 领域

1. 将 `listingadmin` 的能力拆入实际的 listing、product 或 marketplace 所有者；仓储 port 和实现分别迁移。
2. 将 ListingKit 的 task、workflow、preview、export、revision、submission、studio 和 settings 职责迁入 `internal/listing`。
3. 切换期间只保留确实面向外部的兼容 adapter。
4. 生产代码和测试代码引用均归零后，删除 `listingadmin` 和 `listingkit` 根包。

### 阶段 6：agent、knowledge 和 resourcecatalog

1. 先从现有 `ai`、`aicapability`、`prompt` 和 `promptmgmt` 中提取 provider-neutral 的 agent 契约、能力与 prompt 所有权。
2. 明确 knowledge 的文档、切片、检索、引用和向量端口后，再实现 pgvector adapter；没有稳定契约时不提前选型固化。
3. 为 agent、tool、service 和 data 建立独立 resourcecatalog 内核，不复用销售渠道 marketplace 模型。
4. 领域契约与工具生命周期明确后，再用一个可测试纵向切片接入 MCP；禁止只添加客户端依赖或空目录。

### 阶段 7：commercial、ledger 和 organization

1. 将 `listingsubscription` 拆分为通用 commercial 能力和 listing 特有策略；usage ledger 归 commercial。
2. 将身份信封、组织成员、租户归属和业务权限语义收敛到 organization 或所属领域。
3. 将 ZITADEL、Casbin 和领域仓储具体实现迁入 integration，租户兼容桥迁入 compatibility。
4. 只有真实资金余额、充值、扣款或退款需求被确认后才建设 ledger；usage ledger 不因此迁入 ledger，也不提前引入 TigerBeetle。

### 阶段 8：app 装配与入口切换

1. 用 app 所有的 builder 替换 `internal/platforms/*` 注册；builder 通过窄依赖调用 marketplace 构造器。
2. 切换 HTTP、consumer、worker、scheduler、Temporal 和命令入口。
3. 删除 platform 注册包和剩余历史装配根包。

### 阶段 9：仓库收口

1. 删除退休目录、别名、DTO 和临时兼容代码。
2. 更新稳定架构文档及其文档测试。
3. 将迁移期 allowlist 转为永久的目标方向 deny 规则。
4. 运行仓库级验证，并记录无法在当前环境完成的环境依赖测试。

## 测试与验证策略

每个能力迁移都遵循以下顺序：

1. 增加或定位当前可观察行为的特征测试。
2. 为有缺陷的边界增加失败的架构测试或契约测试。
3. 引入目标所有者和窄契约。
4. 先迁移生产调用方，再迁移测试调用方。
5. 证明旧 import 或 constructor 在仓库中达到零引用。
6. 删除旧实现；若存在外部兼容要求，则将旧路径缩减为只做翻译的 adapter。
7. 运行聚焦包测试、反向依赖调用方测试、架构测试、`git diff --check`，以及适用的 lint/build 命令。

架构测试还必须覆盖：领域包不导入 platform、integration 或服务商 SDK；platform 不导入领域；integration 只依赖领域窄 contract；commercial usage ledger 与 money ledger 不能互相冒充。

HTTP、RabbitMQ、Temporal、配置和数据库边界使用与包位置无关的契约测试。只有完整的仓库级测试命令成功结束后，才能报告全量测试通过。

现有 depguard 规则仍是导入方向的主要执行机制。只有在 depguard 无法表达语义约束时，才使用 Go AST 测试，例如禁止特定构造调用或兼容声明。

## 单向收敛护栏

开始抽取前，记录最大历史包的生产文件数和内部依赖基线。迁移期间：

- `internal/listingkit` 和 `internal/listingadmin` 的生产文件数只能减少。
- 它们的直接内部依赖集合只能缩小。
- 不允许任何新包导入计划退休的根包。
- 没有明确的外部契约理由时，不允许向 compatibility 包新增导出声明。

这些是迁移期的单向收敛约束，不是永久、武断的包大小阈值。对应历史包删除后，护栏随之移除。

## 提交与回退模型

- 每个提交代表一个保持行为不变的能力迁移，或一个明确的契约迁移。
- 依赖基础提交先于领域提交；领域提交先于 app 切换提交。
- 当前切片失败时可以独立回退，已经验证的底层提交继续保留。
- 临时 adapter 必须在本治理项目内通过命名清晰的提交引入和删除，不能成为无限期并行的第二套架构。
- 所有阶段达到完成条件后，治理分支最终一次合并。

## 风险与防护

### 巨型包中的隐藏行为

文件名不能证明所有权。每次迁移都必须从调用方、状态变化和行为测试出发，而不能只按目录分类。

### import cycle 压力

接口由消费方持有，具体构造由 app 完成。如果移动产生 import cycle，应重新设计边界，而不是把共享 DTO 塞入通用包来绕过循环。

### 外部协议破坏

协议变更必须与无关移动隔离，并具有明确的版本化或迁移测试。只有达到切换条件后，才能删除旧版本。

### 长期分支漂移

提交保持小而聚焦；每个阶段开始前明确目标所有权；治理分支按团队正常流程定期 rebase 或合并主线。架构护栏用于阻止迁移期间新增历史依赖。

### 结构测试造成的错误安全感

文件白名单不能作为高内聚的证明。护栏必须检查依赖方向、禁止的构造行为、退休包零引用和运行时文件系统副作用。

## 完成标准

满足以下全部条件后，本治理项目才算完成：

- 目标包拥有本设计描述的全部生产行为。
- agent、knowledge、resourcecatalog、commercial、ledger 和 organization 的边界已按实际能力落地；没有业务需求的未来域允许保持未实现，但不得继续把对应职责塞回旧包。
- 退休根包和仓库内部兼容路径已经删除。
- app 是唯一的运行时装配所有者。
- 业务包不构造或暴露具体基础设施客户端。
- 外部契约保持不变，或已经具有经过测试的迁移路径。
- 源码目录中的测试不再产生运行态产物。
- 目标依赖方向已由 depguard 和聚焦语义测试强制执行。
- 默认构建、领域聚焦测试、架构测试、lint 和仓库级测试命令全部成功完成；若存在只能在特定环境运行的测试排除项，必须提供明确证据和说明。
- 稳定架构文档描述的是最终结构，而不是迁移状态。
