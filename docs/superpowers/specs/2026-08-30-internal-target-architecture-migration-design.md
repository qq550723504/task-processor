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
  -> listing / product / marketplace
  -> platform / integration
  -> shared
```

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

负责应用自身的基础设施：配置加载、日志、指标、数据库启动、Redis、队列、Temporal 运行时支持和对象存储。它向 app 装配层提供基础设施能力，不向业务域提供业务服务。

### `internal/integration`

负责应用外部系统的适配器：OpenAI、图像服务商、S3、Playwright、marketplace API，以及 crawler/browser 适配器。服务商 SDK 类型和具体客户端必须留在 integration 包内。

### `internal/product`

负责 canonical catalog facts、资产、商品图片、内容增强和标准化 sourcing。它不能依赖 listing 编排、marketplace workspace、HTTP 装配或具体 integration 实现。

### `internal/marketplace/<name>`

负责各 marketplace 特有的 publishing、workspace/editor 行为、模型、校验、定价、分类、属性和 payload 策略。运行时客户端属于 integration 适配器；进程启动、scheduler 生命周期和 worker 注册属于 app。

### `internal/listing`

负责跨 marketplace 的 listing 任务生命周期、workflow、preview、export、revision、submission、studio、settings 和 subscription。它可以通过窄契约使用 product facts 和 marketplace 能力，但不能吸收 marketplace 规则。

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
- 职责将迁入 `internal/listing` 的其他重叠 listing 根包

退休必须按能力推进，不能盲目重命名目录。只有在明确文件所有者、依赖契约、调用方和行为测试后，才允许移动文件。

## 依赖与构造规则

- 领域包定义自身消费的最小接口。
- app 装配层选择具体实现并完成注入。
- 如果聚焦的 options 值已经足够，业务 pipeline 就不能接收完整的应用全局配置。
- 业务 pipeline 不能构造 OpenAI、数据库、队列、对象存储、浏览器或 marketplace API 客户端。
- marketplace 包通过窄依赖暴露明确的构造器或服务；不能为了实现某个接口而在方法签名中被迫导入 app 运行时类型。
- app 可以导入 marketplace 包；marketplace 包不能反向导入 app。
- platform 和 integration 包不能导入 listing、product 或 marketplace 的业务实现。
- compatibility 可以向内依赖目标领域；目标领域绝不能反向依赖 compatibility。

因此，现有 platform module registry 将由 app 所有的显式装配替代。marketplace 包可以暴露 descriptor 和构造器，但 scheduler、watchdog、prompt store、分片和 worker 生命周期必须留在 app builder 中。

## 迁移顺序

### 阶段 1：基线与运行时副作用

1. 先增加一个失败的文件系统级回归测试，证明包测试不能在 `internal` 下创建日志。
2. 将 logger 的库代码和测试默认行为改为只输出 stdout；只有 app 启动时才能显式选择仓库运行目录或另一个已配置的绝对运行目录下的文件路径。
3. 测试确实需要文件日志时，使用 `t.TempDir` 作为输出位置。
4. 加强仓库产物护栏，使其检查测试运行涉及的实际文件系统状态，而不是只检查 Git 已跟踪路径。

现有的忽略产物单独报告。未获得明确授权时，实施过程不得删除用户工作区数据。

### 阶段 2：shared、platform 和 integration 基础

1. 盘点当前 `core`、`infra` 和技术型 `pkg` 能力。
2. 将配置、日志、指标、数据库、Redis、队列、Temporal 和对象存储运行时所有权迁入 `internal/platform`。
3. 将具体服务商客户端和 crawler 适配器迁入 `internal/integration`。
4. 先更新 app 的构造和生命周期所有权，再迁移业务调用方。
5. 增加 depguard 规则，阻止业务包导入具体 adapter 包。

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
2. 将 ListingKit 的 task、workflow、preview、export、revision、submission、studio、settings 和 subscription 职责迁入 `internal/listing`。
3. 切换期间只保留确实面向外部的兼容 adapter。
4. 生产代码和测试代码引用均归零后，删除 `listingadmin` 和 `listingkit` 根包。

### 阶段 6：app 装配与入口切换

1. 用 app 所有的 builder 替换 `internal/platforms/*` 注册；builder 通过窄依赖调用 marketplace 构造器。
2. 切换 HTTP、consumer、worker、scheduler、Temporal 和命令入口。
3. 删除 platform 注册包和剩余历史装配根包。

### 阶段 7：仓库收口

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
- 退休根包和仓库内部兼容路径已经删除。
- app 是唯一的运行时装配所有者。
- 业务包不构造或暴露具体基础设施客户端。
- 外部契约保持不变，或已经具有经过测试的迁移路径。
- 源码目录中的测试不再产生运行态产物。
- 目标依赖方向已由 depguard 和聚焦语义测试强制执行。
- 默认构建、领域聚焦测试、架构测试、lint 和仓库级测试命令全部成功完成；若存在只能在特定环境运行的测试排除项，必须提供明确证据和说明。
- 稳定架构文档描述的是最终结构，而不是迁移状态。
