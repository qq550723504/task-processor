# ListingKit HTTPAPI Bootstrap Refactor Review

## 背景

`internal/listingkit/httpapi/bootstrap.go` 原先同时承担了多类职责：

1. repository 构造与合并
2. module service 环境准备与 service 创建
3. service runtime / module runtime 组装
4. Temporal workflow client 接线与失败回收
5. `BuildService(...)` / `BuildModule(...)` 的总编排

这会带来两个长期问题：

- 入口函数虽然还能工作，但任何新增依赖或阶段插入都会继续把 `bootstrap.go` 往“总控脚本”方向推。
- 测试虽然能覆盖最终结果，但难以锁住中间阶段的边界约束，导致后续结构调整风险高。

## 本轮已完成的结构收敛

### 1. `BuildService(...)` 主路径分阶段

当前主路径已经收成更清楚的阶段：

- `assembleRepositories(...)`
- `buildServiceRuntime(...)`

当前效果：

- `BuildService(...)` 基本只保留输入校验、closer 生命周期和阶段编排。
- repository/runtime 细节不再直接堆在入口里。

### 2. repository assembly 已按 phase 对齐

当前 repository 层已经形成 3 个大 phase：

- core
- admin
- late core

并进一步收成：

- `buildCoreTaskRepositories(...)`
- `buildCoreAsyncRepositories(...)`
- `buildAdminCatalogRepositories(...)`
- `buildAdminRuleRepositories(...)`
- `buildSubscriptionService(...)`
- `buildLateCoreRepositoryDependencies(...)`

merge 侧也同步拆开：

- `applyCoreRepositories(...)`
- `applyAdminRepositories(...)`
- `applyLateCoreRepositories(...)`

当前效果：

- build phase 和 apply phase 已经对齐。
- 后续如果引入新的子域 repo，不需要再直接往一个大函数里平铺追加。

### 3. service runtime 已独立成 assembly 阶段

当前这层已拆出：

- `buildServiceRuntimeModules(...)`
- `assembleServiceRuntime(...)`

当前效果：

- task/admin/submit/temporal 的 registrar 组装和后续 runtime bundle materialization 分离。
- `buildServiceRuntime(...)` 更像编排函数，而不是一边建模块一边消费模块。

### 4. module service environment 已分阶段

当前这层已拆出：

- `configureModuleServicePolicies(...)`
- `configureModuleServiceAuthorization(...)`

当前效果：

- policy 配置与 authorization / legacy tenant resolver 接线不再混在一个函数里。
- 后续如果再加 feature flag、auth provider 或 owner scope 变体，更容易找到明确落点。

### 5. module runtime 已分 assembly / materialization

当前这层已拆出：

- `assembleModuleRuntime(...)`
- `createModuleRuntime(...)`

当前效果：

- `processor / pool / submitter` 的 runtime 装配和 `handler / studio session handler` 的 materialization 分开。
- `BuildModule(...)` 这条路径与 `BuildService(...)` 一样，开始呈现清楚的 phase 结构。

### 6. Temporal workflow client cleanup 收口

当前这层已新增：

- `closeTemporalWorkflowClientOnError(...)`

当前效果：

- `SheinPublish / StandardProduct / PlatformAdapt` 三条 workflow client 配置路径共享同一套失败 cleanup 语义。
- 减少了重复的“失败后关闭 temporal client”分支。

## 当前结构现状

现在 `bootstrap.go` 已经从“集中式装配脚本”明显转向“分阶段编排 + 子域 phase helper”。

已经形成的主要边界：

- repository assembly
- service runtime assembly
- module service environment
- module runtime assembly
- Temporal workflow cleanup

## 仍然集中的剩余职责

### 1. `buildListingKitServiceConfig(...)` 仍然较重

它现在已经是纯装配函数，但仍一次性拼装：

- core dependencies
- asset dependencies
- shein dependencies
- workflow defaults

如果后续 `ServiceConfig` 继续扩展，这里可能再次成为新的集中点。

### 2. `BuildModule(...)` 仍然依赖 `ServiceBundle.runtime`

现在已经比之前清晰，但 module runtime 仍通过 `ServiceBundle` 的私有 runtime 负载获取 service/task repo/dependencies。
这条边界目前是合理的，但如果未来要支持更多 runtime 变体，可能需要更明确的 runtime payload type。

### 3. 测试仍主要是装配结果测试

虽然已经开始给 phase 补护栏，但当前测试更多约束“产物正确”，较少直接约束：

- closers 的阶段继承策略
- future phase insertion 的顺序假设
- config builder 的子域映射边界

## 结论

这一轮 `listingkit/httpapi/bootstrap.go` 已经达到一个健康停点：

- 最大块的集中职责已经拆开。
- phase 结构已经比较统一。
- 继续新增模块或依赖时，入口的扩张速度会明显下降。

继续往下做是可以的，但收益已经从“去掉明显热点”转向“控制未来扩张速度”的结构精修。
下一阶段不建议再无差别细拆，而应围绕明确目标推进。
