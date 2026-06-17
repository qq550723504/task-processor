# Next Technical Priorities

## Goal

这份清单用于记录当前结构治理完成后的下一阶段技术优先级。

目标不是继续为了结构而重构，而是把后续 2 到 4 周里最值得投入的工程问题排清楚，避免团队重新回到边界失控或低收益重构。

## Priority 1

### 1. 明确平台边界收口策略

当前最重要的问题不是目录怎么摆，而是后续平台能力往哪里长。

正式平台边界策略已收口到：

- `docs/architecture/platform-boundary-strategy.md`

当前仓库里同时存在：

- `internal/shein`
- `internal/temu`
- `internal/amazon`
- `internal/listingkit`
- `internal/publishing/*`
- `internal/platforms/*`

后续需要明确：

- 哪些平台规则继续留在历史平台目录
- 哪些能力应该逐步收口到 `internal/publishing/*`
- 哪些能力属于 `internal/listingkit` 产品主域
- `internal/platforms/*` 是注册层、门面层，还是未来平台主入口

如果这个方向不定，后续每个功能都会把边界再次写散。

### 2. 控制过渡装配层继续膨胀

当前需要重点盯的文件是：

- `internal/app/httpapi/listingkit_support.go`

它现在作为过渡层是合理的，但只能承担：

- 显式依赖注入
- app 层到业务域 builder 的输入适配
- 还没来得及下沉的 repo factory / bridge wiring

它不应该继续承接：

- ListingKit 专属认证逻辑
- ListingKit 专属 AI helper
- 新的业务规则
- 继续增厚的集中式构建逻辑

后续 code review 应把这类文件视为“高风险回流点”。

### 3. 给兼容层设定删除条件

当前仍保留的兼容层：

- `internal/app/processor/compat.go`
- `internal/app/state/compat.go`

这类文件短期有价值，但不能长期双轨存在。

建议删除条件明确为：

1. 仓内零引用
2. 外部依赖确认切走
3. 下一个合适版本窗口内移除

新代码不应再引用这些兼容路径。

## Priority 2

### 4. 加强边界约束测试

当前已经有一批 import boundary 测试，但下一步应继续加强：

- 禁止业务域重新依赖 `internal/app/httpapi`
- 禁止新代码重新 import `internal/app/processor`
- 禁止新代码重新 import `internal/app/state`
- 禁止 route / module builder 回流到中心化装配文件

重点不是增加很多测试，而是给最容易回退的边界加“护栏”。

### 5. 收口长期有效的装配文档

当前已经补了：

- `docs/architecture/httpapi-assembly-boundaries.md`
- `docs/architecture/app-assembly-boundaries.md`
- `docs/development/repository-structure.md`

长期架构文档入口已收口到：

- `docs/architecture/README.md`

接下来要做的是控制文档数量和语义漂移，尽量把长期有效的规则收口到少数文档，而不是让大量计划文档替代正式架构说明。

### 6. 明确 Temporal 的正式边界

Temporal 现在最像下一块容易膨胀的运行时区域。

正式边界说明已收口到：

- `docs/architecture/temporal-boundaries.md`

需要尽早明确：

- 哪些链路适合进入 Temporal
- 哪些异步流程继续留在 RabbitMQ
- 哪些业务逻辑绝不迁入 workflow/activity 层
- HTTP API、service facade、workflow runtime 之间的职责边界

重点是控制编排层，不让它反向吞掉业务实现层。

## Priority 3

### 7. 盘点历史平台包的迁移成本

不是现在立刻迁，而是先盘点：

迁移成本盘点已收口到：

- `docs/architecture/historical-platform-migration-inventory.md`

- 哪些文件已经只剩 facade 作用
- 哪些文件还混着 runtime、平台规则、状态管理和组装逻辑
- 哪些子域最适合下一轮拆分

这一步的价值在于让下一次平台边界治理可预估，而不是重新大范围摸底。

### 8. 把结构治理变成 review 规则

这轮改造要长期生效，靠一次性重构不够，必须转成 review 规则。

正式 review checklist 已收口到：

- `docs/architecture/architecture-review-checklist.md`

建议以后每个相关 PR 至少检查：

1. 有没有新增反向依赖
2. 有没有把业务 helper 塞回 app 层
3. 有没有让兼容层重新变成正式入口
4. 有没有新增 undocumented assembly behavior

## Working Rule

当前阶段最重要的原则是：

- 先控制演进方向，再考虑进一步重构
- 优先阻止边界回退，而不是继续追求目录“更漂亮”
- 把结构治理成果转成约束、文档和 review 习惯

如果后续没有明确的新业务压力，结构层的默认动作应当是“守住边界”，而不是继续大规模移动代码。
