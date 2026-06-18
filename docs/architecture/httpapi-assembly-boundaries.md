# HTTP API Assembly Boundaries

## Goal

这份说明记录当前 `ListingKit` HTTP API 的稳定装配边界，避免后续新增功能时又把业务逻辑回流到 `internal/app/httpapi`。

它回答的不是“某个业务怎么实现”，而是“HTTP 入口这一层应该由谁负责什么”。

## 当前稳定链路

当前推荐的装配链路是：

`cmd/* -> internal/app/httpapi -> internal/*/httpapi -> domain service/runtime/contracts`

对应含义：

- `cmd/*`
  - 只负责进程入口、参数和启动方式。
  - 例如 `cmd/product-listing-api`、`cmd/productenrich-api`、`cmd/listingkit-temporal-worker`。
- `internal/app/httpapi`
  - 统一 HTTP runtime 装配层。
  - 负责共享 server 启动、跨模块 route 挂载、公共依赖 bundle、以及对各业务 `httpapi` builder 的协调。
- `internal/*/httpapi`
  - 业务域自己的 HTTP builder / registrar。
  - 例如 `internal/productenrich/httpapi`、`internal/productimage/httpapi`、`internal/amazonlisting/httpapi`、`internal/listingkit/httpapi`。
- `domain service/runtime/contracts`
  - 真正的业务 service、repository、runtime contract 和平台能力。
  - 不应该再反向依赖 `internal/app/httpapi`。

## 各层职责

### `cmd/*`

应该负责：

- 选择配置文件
- 选择启动模式
- 调用共享启动入口

不应该负责：

- 手写业务装配
- 复制 HTTP 启动逻辑
- 直接拼业务依赖图

### `internal/app/httpapi`

应该负责：

- 构建共享 runtime 依赖
- 调用各业务 `BuildModule(...)`
- 统一启动 `gin` / `http.Server`
- 挂载跨业务的 route descriptor
- 协调关闭顺序和公共 closers

不应该负责：

- 某个业务域自己的 repository 规则
- 某个业务域自己的 AI client 选择策略
- 某个业务域自己的认证/授权运行时实现
- 大段平台或产品专属构建细节

一句话说，`internal/app/httpapi` 是装配协调层，不是业务实现层。

### `internal/productenrich/httpapi`

应该负责：

- `productenrich` 的 module builder
- `productenrich` 的 route registrar
- `productenrich` 自己的 task repo / pool / handler 装配

### `internal/productimage/httpapi`

应该负责：

- `productimage` 的 module builder
- 模型 provider / publisher / image pipeline 组件装配
- 具体模型 provider 和资产 publisher/S3 client 组装应放在专用 builder 文件，
  `bootstrap.go` 只负责串起 module 依赖图
- task repository / DB migration 组装应放在专用 repository builder 文件，
  避免 `bootstrap.go` 重新承担数据库细节
- image pipeline 组件选择、远程 segmenter/white-background client 组装和
  model-backed fallback resolution 应放在专用 component builder 文件
- `productimage` 自己的 handler / pool / repo 构建

### `internal/amazonlisting/httpapi`

应该负责：

- `amazonlisting` 的 module builder
- `amazonlisting` 的 route registrar
- `amazonlisting` 自己的 task repo / pool / handler 装配

### `internal/listingkit/httpapi`

应该负责：

- `ListingKit` 的 module builder 和 service builder
- `ListingKit` 的 route registrar
- `ListingKit` 专属的 ZITADEL auth runtime
- `ListingKit` 专属的 AI client routing / fallback helper
- `ListingKit` 默认 store / studio image generator 等 HTTP-facing builder helper

不应该再把这些逻辑放回 `internal/app/httpapi`。

## 当前允许的过渡点

当前仍允许少量过渡装配留在：

- `internal/app/httpapi/listingkit_support.go`

它现在承接的是：

- app 层到 `listingkit/httpapi` 的输入适配
- 部分 repo factory / legacy bridge / transitional configurator 注入

这类文件可以存在，但要求是：

- 只做显式注入
- 不再新增业务规则
- 不把业务 helper 再定义回 app 层

## 依赖方向规则

后续改动默认遵守下面的方向：

1. `cmd` 可以依赖 `internal/app/httpapi`
2. `internal/app/httpapi` 可以依赖业务域 `internal/*/httpapi`
3. 业务域 `internal/*/httpapi` 可以依赖本域 service / repo / runtime contract
4. 业务域不应反向依赖 `internal/app/httpapi`

如果出现“为了复用一个 helper，把业务包重新 import 回 `internal/app/httpapi`”，这通常说明边界正在退化。

## Route 装配规则

当前 route 的归属规则是：

- route descriptor 类型统一放在 `internal/httproute`
- `internal/app/httpapi/server.go` 只做 route 汇总和挂载
- 各业务域自己维护 `AppendRouteDescriptors(...)`

这意味着新增一个 ListingKit API 时，优先改：

- `internal/listingkit/httpapi/routes.go`

而不是继续把逻辑塞回 `internal/app/httpapi/server.go`。

## Module 装配规则

当前 module builder 的归属规则是：

- `productenrich` builder 在 `internal/productenrich/httpapi`
- `productimage` builder 在 `internal/productimage/httpapi`
- `amazonlisting` builder 在 `internal/amazonlisting/httpapi`
- `listingkit` builder 在 `internal/listingkit/httpapi`

`internal/app/httpapi/modules.go` 现在应该保持“薄委托”，而不是再重新长成一个集中式 God file。

`internal/app/httpapi/types.go` 应保持为 runtime state / module state 类型定义文件。
`httpFeatureComposition` 的 runtime module、route module、handler accessor 和 server
bundle 组装方法应放在 `composition_modules.go`，route handler contract / alias 应放在
`route_handler_types.go`，避免类型文件继续承载装配行为或路由契约细节。
`runtime.go` 应聚焦 `buildRuntimeDeps(...)` 的启动流程，`runtimeDeps` 的 accessor、
closer 和 module attach 方法应放在 `runtime_deps_methods.go`。
Prompt registry 初始化、tenant prompt store attach 和相关 closer 收集应放在
`runtime_prompt.go`，由 `buildPromptRuntimeDeps(...)` 对外提供 prompt runtime deps，
避免 `runtime.go` 直接承载 prompt runtime 细节或依赖 prompt adapter 类型。
ProductEnrich 的 LLM manager、mock LLM 校验、product understanding 和 input parser
初始化应放在 `runtime_productenrich.go`，让 `runtime.go` 保持顶层运行时编排。
Shared resources 的 app bootstrap 调用和 `SharedResourceOptions` 应放在
`runtime_shared_resources.go`，避免 `runtime.go` 重新承载基础设施装配选项。
OpenAI manager 创建、DB credential resolver 挂载和相关 closer 收集应放在
`runtime_openai.go`，避免 `runtime.go` 直接依赖具体 OpenAI client adapter 类型。
Image workdir 默认值和路径清洗应放在 `runtime_paths.go`，避免 `runtime.go`
重新承担路径规范化细节。
Config 文件加载和加载错误包装应放在 `runtime_config.go`，让 `runtime.go`
保持启动阶段编排，而不是直接依赖具体配置加载实现。
Worker pool 默认配置和本地 task health provider 聚合应放在
`runtime_worker_pools.go`，避免 `modules.go` 承载 worker runtime 观测和池参数细节。
SHEIN 登录账号配置、SHEIN login module 和 SDS login module bootstrap 应放在
`runtime_login_modules.go`，避免 `modules.go` 直接依赖具体登录子系统 bootstrap。

`internal/app/httpapi/adapters.go` 不应重新长成所有基础设施 adapter 的集中入口。
OpenAI manager / credential resolver 组装应放在 `adapters_openai.go`，schema migration
集合应放在 `adapters_schema_migration.go`，通用 repo / Redis / scraper adapter helper
才留在 `adapters.go`。产品、图片和 Amazon Listing 的 DB task repository adapter 工厂
应放在 `adapters_task_repositories.go`，避免 `adapters.go` 继续承载多业务 repository
细节。Tenant prompt store 的 DB/bootstrapresources 装配应放在 `adapters_prompt.go`。

## 兼容层规则

当前仓库里曾经有一些 app 兼容层，例如：

- `internal/app/processor/compat.go`
- `internal/app/state/compat.go`

这些兼容层的目的只是平滑迁移，不是长期双轨结构。当前退休状态记录在：

- `docs/architecture/compatibility-retirement.md`

后续原则：

- 新代码不要再引用兼容层旧路径
- 兼容层只减不增
- 等仓内和外部依赖确认切完后再删除，并用测试防止旧路径重新成为入口

## Boundary Guards

HTTP API 装配边界由以下测试守住：

- `TestDomainHTTPPackagesDoNotImportAppHTTPAPI`
- `TestBusinessDomainsDoNotImportAppHTTPAPI`
- `TestAppHTTPAPIRootListingKitHelpersStayAllowlisted`
- `TestAppHTTPAPIModuleBuildersStayAllowlisted`
- `TestAppHTTPAPIRouteDescriptorHelpersStayAllowlisted`
- `TestAppHTTPAPIListingKitSupportImportsStayAllowlisted`
- `TestAppHTTPAPIListingKitRootImportsStayAllowlisted`
- `TestAppHTTPAPIListingKitHTTPAPIImportsStayAllowlisted`
- `TestHTTPAPITypesDoesNotOwnFeatureCompositionMethods`
- `TestHTTPAPITypesDoesNotOwnRouteHandlerContracts`
- `TestHTTPAPIModulesFileDoesNotOwnWorkerRuntimeSupport`
- `TestHTTPAPIModulesFileDoesNotOwnLoginRuntimeSupport`
- `TestHTTPAPIRuntimeKeepsRuntimeDepsMethodsDedicated`
- `TestHTTPAPIRuntimeKeepsPromptRuntimeAssemblyDedicated`
- `TestHTTPAPIRuntimeKeepsProductEnrichRuntimeAssemblyDedicated`
- `TestHTTPAPIRuntimeKeepsSharedResourceAssemblyDedicated`
- `TestHTTPAPIRuntimeKeepsOpenAIRuntimeAssemblyDedicated`
- `TestHTTPAPIRuntimeKeepsPathResolutionDedicated`
- `TestHTTPAPIRuntimeKeepsConfigLoadingDedicated`
- `TestHTTPAPIAdaptersKeepOpenAIAssemblyDedicated`
- `TestHTTPAPIAdaptersKeepTaskRepositoryAssemblyDedicated`
- `TestHTTPAPIAdaptersKeepPromptStoreAssemblyDedicated`
- `TestBootstrapKeepsModelProviderAssemblyInDedicatedFile`
- `TestBootstrapKeepsAssetPublisherAssemblyInDedicatedFile`
- `TestBootstrapKeepsTaskRepositoryAssemblyInDedicatedFile`
- `TestBootstrapKeepsImagePipelineComponentAssemblyInDedicatedFile`

如果需要新增 app/httpapi 过渡 seam，应在同一变更中更新对应 allowlist、
这份文档和文档测试；否则优先把新增逻辑放到 owning `internal/*/httpapi`
包。

## 审查问题

当后续有人改 HTTP API 装配时，优先问这几个问题：

1. 这段代码是共享装配逻辑，还是某个业务域自己的实现？
2. 它应该放在 `internal/app/httpapi`，还是业务域自己的 `httpapi` 包？
3. 这次改动有没有让业务域重新依赖 app 层？
4. 新增 route / builder / auth helper 是否放回了中心包？
5. 这是在减少边界噪音，还是在制造新的隐式耦合？

## Working Rule

当前最安全的工作规则是：

- `internal/app/httpapi` 负责协调
- `internal/*/httpapi` 负责本域 HTTP 装配
- 业务 service / repo / runtime contract 负责实现

只要后续改动继续强化这条线，HTTP API 结构就不会再退回到之前的集中式装配状态。
