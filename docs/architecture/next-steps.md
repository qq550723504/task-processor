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

历史上需要重点盯的文件是：

- `internal/app/httpapi/listingkit_support.go`

该过渡文件已经退役；ListingKit runtime input shaping 已并入
`internal/app/httpapi/feature_builder_listingkit.go`。后续应重点盯：

- `internal/app/httpapi/feature_builder_listingkit.go`
- `internal/app/httpapi/runtime_support_listingkit.go`

这些文件可以承担：

- 显式依赖注入
- app 层到业务域 builder 的输入适配
- 共享 runtime prerequisite 准备

它不应该继续承接：

- ListingKit 专属认证逻辑
- ListingKit 专属 AI helper
- 新的业务规则
- 继续增厚的集中式构建逻辑

后续 code review 应把这类文件视为“高风险回流点”。

### 3. 给兼容层设定删除条件

兼容层退休状态已收口到：

- `docs/architecture/compatibility-retirement.md`

当前已退休并由测试守住的兼容路径：

- `internal/app/processor`
- `internal/app/state` Go 兼容文件

这类文件短期有价值，但不能长期双轨存在。

建议删除条件明确为：

1. 仓内零引用
2. 外部依赖确认切走
3. 下一个合适版本窗口内移除

新代码不应再引用这些兼容路径。

## Priority 2

### 4. 加强边界约束测试

当前已经有一批 import boundary 和结构测试。已落地的核心护栏不要再当成开放待办，而应该作为后续 review 的基线。

Current guard coverage:

当前 baseline 的长期入口由 `docs/architecture/README.md` 索引，正式 review 动作以
`docs/architecture/architecture-review-checklist.md` 为准。
Every next-step reference must resolve to an existing repository document.
Every guard listed in current coverage must resolve to an implemented test function.

- `TestBusinessDomainsDoNotImportAppHTTPAPI` 禁止业务域重新依赖 `internal/app/httpapi`
- `TestProjectBoundaryDomainsDoNotImportListingKitFacade` 禁止产品、发布、市场、平台和 infra 包重新依赖 ListingKit 根 facade
- `TestListingKitSubdomainsDoNotImportRootFacade` 禁止 ListingKit 子域包回流依赖根 facade，保持子域可以继续独立收口
- `TestListingKitRootSheinWorkspaceBridgesDoNotImportWorkspaceDomainDirectly` 禁止 ListingKit 根 SHEIN workspace bridge 直接依赖 workspace domain，保持兼容桥接层隔离
- `TestListingKitRootNonTestFilesDoNotImportWorkspaceDomainDirectly` 禁止 ListingKit 根非测试文件绕过 `internal/listingkit/workspace/shein` 兼容层直接依赖 workspace domain
- `TestListingKitSheinWorkspaceBridgeDoesNotImportLegacyWorkspaceDomain` 禁止 ListingKit SHEIN workspace bridge 回退依赖 legacy workspace domain
- `TestListingKitDoesNotImportLegacySheinRuntime` 禁止 ListingKit 根 facade 新增历史 SHEIN pipeline/publish/build runtime 依赖
- `TestListingKitDoesNotImportSheinAPIRoot` 禁止 ListingKit 根 facade 重新依赖 SHEIN API root
- `TestListingKitNonAPISheinImportsStayAllowlisted` 禁止 ListingKit 根 facade 新增未登记的非 API SHEIN 实现依赖，当前 adapter seam 必须精确登记
- `TestListingKitRootDoesNotImportManagementAPI` 禁止 ListingKit 根 facade 直接依赖 management API DTO，保持 concrete management contracts 下沉到子域或装配 adapter seam
- `TestListingKitSheinSyncLegacyPromotionImportsStayAllowlisted` 禁止 ListingKit SHEIN sync 生产核心重新依赖历史 promotion DTO/bridge，保留的 legacy 依赖只能在专用 promotion bridge adapter seam
- `TestListingKitAmazonListingImportsStayAllowlisted` 禁止 ListingKit 根 facade 新增未登记的 AmazonListing 依赖，保持当前 result/bridge seam 可审查
- `TestCatalogDoesNotDependOnProductEnrichAliases` 禁止 catalog 回退依赖 ProductEnrich 兼容别名，保持产品事实归属在 canonical/product 模块
- `TestCanonicalTypesDoNotUseProductEnrichCompatibilityAliases` 禁止 canonical 类型重新使用 ProductEnrich compatibility aliases，保持规范模型不依赖历史兼容层
- `TestSheinPipelineDoesNotImportListingKitFacade` 禁止 SHEIN pipeline 回退依赖 ListingKit facade
- `TestSheinSubmitPrepDoesNotImportListingKitTenantContext` 禁止 SHEIN submitprep 回退依赖 ListingKit tenant context
- `TestPublishingSheinSubmitPrepUsesOnlySensitiveWordAdapter` 禁止 `internal/publishing/shein/submit_prep.go` 继续通过旧 `submitprep` 复用纯语言/翻译 helper，保留的 `submitprep` 依赖只能作为敏感词和仓储 adapter seam
- `TestListingKitRootSheinHelpersStayAllowlisted` 禁止 ListingKit 根包新增未登记 SHEIN helper seam
- `TestListingKitRootServiceSubmitFilesStayAllowlisted` 禁止 ListingKit 根 service submit 文件继续扩散提交 seam
- `TestListingKitRootTaskSubmissionFilesStayAllowlisted` 禁止 ListingKit 根 task submission 文件继续扩散提交 seam
- `TestListingKitRootServiceGenerationFilesStayAllowlisted` 禁止 ListingKit 根 service generation 文件继续扩散生成 seam
- `TestListingKitRootGenerationFilesStayAllowlisted` 禁止 ListingKit 根 generation 文件继续扩散生成 seam
- `TestInternalPackagesDoNotImportAppProcessorCompatibilityLayer` 禁止新代码重新 import `internal/app/processor`
- `TestAppProcessorCompatibilityLayerIsRetired` 确认 `internal/app/processor` 兼容层保持退休状态
- `TestInternalPackagesDoNotImportAppStateCompatibilityLayer` 禁止新代码重新 import `internal/app/state`
- `TestAppStateCompatibilityLayerIsRetired` 确认 `internal/app/state` Go 兼容文件保持退休状态
- `TestInfraProductCrawlerAdapterIsRetired` 确认 `internal/infra/productcrawler` 兼容 adapter 保持退休状态
- `TestAppCrawlerFetcherCompatibilityLayerIsRetired` 确认 `internal/app/crawler/fetcher` 兼容层保持退休状态
- `TestCmdPackagesDoNotImportAppCompatibilityLayers` 禁止生产入口重新依赖已退休 app 兼容层
- `TestDomainHTTPPackagesDoNotImportAppHTTPAPI` 禁止业务域 HTTP 包反向依赖 `internal/app/httpapi`
- `TestAppHTTPAPIRootListingKitHelpersStayAllowlisted` 禁止 `internal/app/httpapi` 根目录新增 ListingKit helper，ListingKit 专属逻辑应下沉到 owning HTTPAPI 或 domain 包
- `TestAppHTTPAPIModuleBuildersStayAllowlisted` 禁止 module builder 回流到中心化装配文件
- `TestAppHTTPAPIRouteDescriptorHelpersStayAllowlisted` 禁止 route descriptor helper 回流到中心化装配文件
- `TestListingKitSupportFileStaysRetired` 禁止 `listingkit_support.go` 作为过渡桶复活
- `TestAppHTTPAPIListingKitSupportImportsStayAllowlisted` 若旧文件被恢复，会继续限制其 import 面
- `TestAppHTTPAPIListingKitRootImportsStayAllowlisted` 禁止 app/httpapi 新增未登记 ListingKit root facade 依赖
- `TestAppHTTPAPIListingKitHTTPAPIImportsStayAllowlisted` 禁止 app/httpapi 新增未登记 ListingKit HTTPAPI 依赖，保持当前 module/runtime/route/server 装配 seam 可审查
- `TestHTTPAPITypesKeepExternalClientRuntimeDepsDedicated` 禁止 `internal/app/httpapi/types.go` 吞入 concrete `openai` runtime 状态，保持外部客户端运行时依赖集中在 `runtime_shared_deps.go`
- `TestHTTPAPIAdaptersKeepOpenAIAssemblyDedicated` 禁止 `internal/app/httpapi/adapters.go` 吞入 concrete `openai` adapter 装配，保持 OpenAI 组装集中在 `adapters_openai.go`
- `TestHTTPAPIRuntimeKeepsOpenAIRuntimeAssemblyDedicated` 禁止 `internal/app/httpapi/runtime.go` 吞入 concrete `openai` runtime 装配，保持 OpenAI runtime 组装集中在 `runtime_openai.go`
- `TestHTTPAPIRuntimeKeepsSharedResourceAssemblyDedicated` 禁止 `internal/app/httpapi/runtime.go` 吞入 shared resource/bootstrap 装配，保持共享资源组装集中在 `runtime_shared_resources.go`
- `TestHTTPAPIRuntimeKeepsRuntimeDepsMethodsDedicated` 禁止 `internal/app/httpapi/runtime.go` 吞入 runtimeDeps 状态 helper，保持方法拆分集中在 `runtime_deps_methods.go`
- `TestHTTPAPIRuntimeKeepsPromptRuntimeAssemblyDedicated` 禁止 `internal/app/httpapi/runtime.go` 吞入 prompt runtime 装配，保持 prompt 运行时组装集中在 `runtime_prompt.go`
- `TestHTTPAPIRuntimeKeepsProductEnrichRuntimeAssemblyDedicated` 禁止 `internal/app/httpapi/runtime.go` 吞入 ProductEnrich runtime 装配，保持 ProductEnrich 运行时组装集中在 `runtime_productenrich.go`
- `TestHTTPAPIRuntimeKeepsPathResolutionDedicated` 禁止 `internal/app/httpapi/runtime.go` 吞入路径解析逻辑，保持路径决策集中在 `runtime_paths.go`
- `TestHTTPAPIRuntimeKeepsConfigLoadingDedicated` 禁止 `internal/app/httpapi/runtime.go` 吞入配置加载逻辑，保持配置读取集中在 `runtime_config.go`
- `TestHTTPAPIAdaptersKeepTaskRepositoryAssemblyDedicated` 禁止 `internal/app/httpapi/adapters.go` 吞入任务仓储装配，保持 repository adapter 组装集中在 `adapters_task_repositories.go`
- `TestHTTPAPIAdaptersKeepPromptStoreAssemblyDedicated` 禁止 `internal/app/httpapi/adapters.go` 吞入 prompt store 装配，保持 prompt store adapter 组装集中在 `adapters_prompt.go`
- `TestBootstrapKeepsModelProviderAssemblyInDedicatedFile` 禁止 `internal/app/httpapi/bootstrap.go` 吞入 model provider 装配，保持模型提供方组装集中在专用 bootstrap seam
- `TestBootstrapKeepsLLMScorerAssemblyInDedicatedFile` 禁止 `internal/app/httpapi/bootstrap.go` 吞入 LLM scorer 装配，保持评分器组装集中在专用 bootstrap seam
- `TestBootstrapKeepsAssetPublisherAssemblyInDedicatedFile` 禁止 `internal/app/httpapi/bootstrap.go` 吞入 asset publisher 装配，保持资源发布组装集中在专用 bootstrap seam
- `TestBootstrapKeepsTaskRepositoryAssemblyInDedicatedFile` 禁止 `internal/app/httpapi/bootstrap.go` 吞入 task repository 装配，保持任务仓储组装集中在专用 bootstrap seam
- `TestBootstrapKeepsImagePipelineComponentAssemblyInDedicatedFile` 禁止 `internal/app/httpapi/bootstrap.go` 吞入 image pipeline component 装配，保持图像流水线组件组装集中在专用 bootstrap seam
- `TestCmdContainsOnlyOfficialEntrypoints` 禁止 `cmd/` 新增临时或非正式入口，调试程序应放入受管 `hack/` 区域
- `TestCmdProductionEntrypointsDoNotImportDomainOrInfraPackages` 禁止生产入口绕过 app 装配层直接依赖业务域或 infra 包
- `TestHackContainsOnlyManagedSupportAreas` 禁止 `hack/` 继续扩散未登记的临时支持目录
- `TestHackSupportAreasContainNoLocalArtifacts` 禁止未跟踪或被忽略的本地 `tmp` / `logs` / `bin` 产物留在 `hack/` 调试支持目录
- `TestTrackedLocalArtifactsStayOutOfProductionEntrypoints` 禁止本地产物或调试文件进入生产入口目录
- `TestProductionEntrypointsContainNoLocalArtifacts` 禁止未跟踪或被忽略的本地 `logs` / `tmp` / `__debug_bin*` 产物留在 `cmd/` 生产入口目录
- `TestTrackedLocalArtifactsStayOutOfTools` 禁止本地产物或一次性调试文件进入长期维护的 `tools/` 目录
- `TestToolsContainNoLocalArtifacts` 禁止未跟踪、被忽略或输出型 `node_modules` / `.exe` / `result` 产物留在长期维护的 `tools/` 目录
- `TestInternalPackagesContainNoLocalArtifacts` 禁止未跟踪或被忽略的本地 `.local` / `logs` / `tmp` 产物留在 `internal/` 源码目录
- `TestSDSLoginRuntimeStateStaysOutOfInternalPackages` 禁止 SDS 登录态、浏览器状态和 auth/cookie JSON 回落到 `internal/sdslogin/data`，默认运行态位置应保持在 `.local/sds/`
- `TestBusinessImplementationPackagesDoNotImportGinDirectly` 禁止业务实现包新增直接 `gin` 依赖，当前历史 handler 例外必须显式登记
- `TestBusinessDomainsDoNotImportAppRuntimeAssembly` 禁止业务域新增对 `internal/app/{bootstrap,consumer,runner,runtime}` 的具体装配依赖，当前 `listingkit/httpapi` 过渡适配例外必须显式登记
- `TestPlatformModulesDoNotImportBusinessOrHTTPAssemblyPackages` 禁止 `internal/platforms/*` 新增业务域或 HTTP 装配依赖，保持平台注册层只做注册/选择/委托
- `TestPlatformModulesHistoricalImplementationImportsStayAllowlisted` 禁止 `internal/platforms/*` 新增历史平台实现依赖，当前 Amazon/SHEIN/TEMU 注册委托必须精确登记
- `TestPlatformRegistrationPackagesStayThin` 禁止 `internal/platforms/*` 新增非 module descriptor / doc / test 文件，避免平台注册层承接业务规则
- `TestPlatformRegistrationPackagesContainNoLocalArtifacts` 禁止 `internal/platforms/*` 混入本地 `tmp` / `logs` / `bin` 等运行产物，保持平台注册层只包含可审查源码
- `TestSheinPublishingDoesNotImportLegacyRuntimeOrListingKit` 禁止 `internal/publishing/shein` 重新依赖 ListingKit facade 或历史 SHEIN runtime，当前提交校验例外必须精确登记
- `TestPublishingSheinNonAPISheinImportsStayAllowlisted` 禁止 `internal/publishing/shein` 新增未登记的历史 SHEIN 实现依赖，保持 publishing seam 收口
- `TestPublishingSheinManagedAPIImportsStayAllowlisted` 禁止 `internal/publishing/sheinmanaged` 新增未登记的 concrete SHEIN API client 依赖，保持 managed runtime API 构造集中在 builder/factory seam
- `TestPublishingSheinManagedManagementImportsStayAllowlisted` 禁止 `internal/publishing/sheinmanaged` 新增未登记的 concrete `management` adapter 依赖，当前 managed publishing helper seam 只是 management 退休前过渡点
- `TestPublishingCommonUsesCanonicalPackage` 禁止 `internal/publishing/common` 回退到 ProductEnrich 兼容别名，保持 publishing shared vocabulary 依赖 canonical package
- `TestPublishingCommonDoesNotImportPlatformImplementations` 禁止 `internal/publishing/common` 依赖 SHEIN/TEMU/Amazon 实现包，避免 shared publishing vocabulary 反向承接平台规则
- `TestInfrastructurePackagesDoNotImportBusinessDomains` 禁止 `internal/infra`、`internal/integration`、`internal/platformbase`、`internal/platformtask` 反向依赖业务域
- `TestProductImageExternalClientImportsStayAllowlisted` 禁止 `internal/productimage` 新增 concrete `openai` / `nanobanana` adapter 依赖，当前 provider/runtime seam 必须精确登记
- `TestAmazonExternalClientImportsStayAllowlisted` 禁止 `internal/amazon` 新增 concrete `management` / `openai` adapter 依赖，当前 context/DTO/LLM seam 必须精确登记
- `TestSheinBridgeExternalClientImportsStayAllowlisted` 禁止 `internal/sheinbridge` 新增 concrete `management` / `openai` adapter 依赖，当前 sale-attribute runtime bridge seam 必须精确登记
- `TestSheinManagementClientImportsStayAllowlisted` 禁止 `internal/shein` 新增未登记的 concrete `management` adapter 依赖，当前库存、调度、发布、校验、活动、映射和商品 seam 必须精确登记
- `TestSheinOpenAIImportsStayAllowlisted` 禁止 `internal/shein` 新增未登记的 concrete `openai` adapter 依赖，当前类目、内容、pipeline、商品、submit-prep 和翻译 seam 必须精确登记
- `TestAppTaskManagementClientImportsStayAllowlisted` 禁止 `internal/app/task` 新增未登记的 concrete `management` adapter 依赖，当前任务源、分发、claim 和 fetcher seam 只是 management 退休前过渡点
- `TestAppRunnerManagementClientImportsStayAllowlisted` 禁止 `internal/app/runner` 新增未登记的 concrete `management` adapter 依赖，当前 scheduler、processor 和 health-check runtime seam 只是 management 退休前过渡点
- `TestAppConsumerManagementClientImportsStayAllowlisted` 禁止 `internal/app/consumer` 新增未登记的 concrete `management` adapter 依赖，当前 processor registry、RabbitMQ service、task handler、shared-resource 和 auto-shard seam 只是 management 退休前过渡点
- `TestAppBootstrapManagementClientImportsStayAllowlisted` 禁止 `internal/app/bootstrap` 新增未登记的 concrete `management` adapter 依赖，当前 app、scheduler factory、scheduler dependency 和 shared-resource assembly seam 只是 management 退休前过渡点
- `TestAppHTTPAPIManagementClientImportsStayAllowlisted` 禁止 `internal/app/httpapi` 新增未登记的 concrete `management` adapter 依赖，当前 runtime deps 和 SHEIN module test seam 只是 management 退休前过渡点
- `TestAppRuntimeListingManagementClientImportsStayAllowlisted` 禁止 `internal/app/runtime/listing` 新增未登记的 concrete `management` adapter 依赖，当前 debug task runner seam 只是 management 退休前过渡点
- `TestAppTaskStatusManagementClientImportsStayAllowlisted` 禁止 `internal/app/taskstatus` 新增未登记的 concrete `management` adapter 依赖，当前 task status service seam 只是 management 退休前过渡点
- `TestPlatformTaskManagementClientImportsStayAllowlisted` 禁止 `internal/platformtask` 新增未登记的 concrete `management` adapter 依赖，当前 product sync、inventory sync 和 auto pricing task seam 只是 management 退休前过渡点
- `TestStateManagementClientImportsStayAllowlisted` 禁止 `internal/state` 新增未登记的 concrete `management` adapter 依赖，当前 manager 和 daily-count seam 只是 management 退休前过渡点
- `TestPlatformBaseManagementClientImportsStayAllowlisted` 禁止 `internal/platformbase` 新增未登记的 concrete `management` adapter 依赖，当前 platform factory seam 只是 management 退休前过渡点
- `TestProcessorManagementClientImportsStayAllowlisted` 禁止 `internal/processor` 新增未登记的 concrete `management` adapter 依赖，当前 base processor seam 只是 management 退休前过渡点
- `TestTaskRPCAPIManagementClientImportsStayAllowlisted` 禁止 `internal/taskrpcapi` 新增未登记的 concrete `management` adapter 依赖，当前 build 和 handler seam 只是 management 退休前过渡点
- `TestSDSClientManagementClientImportsStayAllowlisted` 禁止 `internal/sds/client` 新增未登记的 concrete `management` adapter 依赖，当前 SDS auth bootstrap seam 只是 management 退休前过渡点
- `TestSheinLoginBootstrapManagementClientImportsStayAllowlisted` 禁止 `internal/sheinlogin/bootstrap` 新增未登记的 concrete `management` adapter 依赖，当前 login bootstrap seam 只是 management 退休前过渡点
- `TestSheinLoginServiceManagementClientImportsStayAllowlisted` 禁止 `internal/sheinlogin` 新增未登记的 concrete `management` adapter 依赖，当前 bootstrap 和 login service seam 只是 management 退休前过渡点
- `TestSheinLoginManagedManagementClientImportsStayAllowlisted` 禁止 `internal/sheinloginmanaged` 新增未登记的 concrete `management` adapter 依赖，当前 managed login bridge 和 account seam 只是 management 退休前过渡点
- `TestSharedPricingManagementClientImportsStayAllowlisted` 禁止 `internal/pricing` 新增未登记的 concrete `management` adapter 依赖，当前成本配置 lookup seam 只是 management 退休前过渡点
- `TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted` 禁止 `internal/app/httpapi` ProductImage 装配文件新增 concrete `openai` / `nanobanana` adapter 依赖，当前模型默认值/装配 seam 必须精确登记
- `TestPublishingSheinOpenAIImportsStayAllowlisted` 禁止 `internal/publishing/shein` 新增 concrete `openai` adapter 依赖，当前属性/类目/文案 inference seam 必须精确登记
- `TestListingKitHTTPAPIExternalClientImportsStayAllowlisted` 禁止 `internal/listingkit/httpapi` 新增 concrete `openai` / `nanobanana` adapter 依赖，当前 AI runtime/bootstrap seam 必须精确登记
- `TestListingKitHTTPAPIManagementClientImportsStayAllowlisted` 禁止 `internal/listingkit/httpapi` 新增未登记的 concrete `management` adapter 依赖，当前 SHEIN sync runtime strategy seam 只是 management 退休前过渡点
- `TestListingKitSheinSyncLegacyPromotionImportsStayAllowlisted` 禁止 `internal/listingkit/sheinsync` 新增未登记的 `management/api` DTO 或 legacy SHEIN activity bridge 依赖，当前 promotion bridge 兼容 seam 必须精确登记
- `TestListingKitRootOpenAIImportsStayAllowlisted` 禁止 `internal/listingkit` 根包新增 concrete `openai` adapter 依赖，当前 facade/settings/service/studio/task seam 必须精确登记
- `TestTEMUSyncAndPricingManagementImportsStayAllowlisted` 禁止 `internal/temu/{sync,pricing}` 新增 concrete `management` adapter 依赖，当前同步/定价 seam 必须精确登记
- `TestTEMUProductStoreAndSchedulerManagementImportsStayAllowlisted` 禁止 `internal/temu/{product,store,scheduler}` 新增 concrete `management` adapter 依赖，当前商品/店铺/调度 seam 必须精确登记
- `TestTEMURuntimeAndBridgeManagementImportsStayAllowlisted` 禁止 `internal/temu/{api/client,context,bulkrelist,filter,handlerbase,rules}` 和根包 `processor.go` 新增 concrete `management` adapter 依赖，当前 runtime/bridge seam 必须精确登记
- `TestTEMUOpenAIImportsStayAllowlisted` 禁止 `internal/temu` 新增 concrete `openai` adapter 依赖，当前 AI/image/SKU/product/pipeline seam 必须精确登记
- `TestTemporalSDKImportsStayInRuntimeAndOrchestrationAdapters` 禁止 Temporal SDK 扩散到非 runtime / orchestration adapter 区域，当前 app runtime、ListingKit temporal 和提交 activity adapter 例外必须精确登记
- `TestTemporalRuntimePackagesDoNotImportHTTPAPI` 禁止 Temporal runtime / workflow package 反向依赖 HTTP API，避免编排层吞掉请求层行为
- `TestListingPreviewPackageStaysPlatformNeutral` 禁止 `internal/listing/preview` 重新依赖 ListingKit facade 或 Amazon/SHEIN/TEMU 等平台实现包，保持 preview 抽取层平台中立

后续重点不是增加很多测试，而是给还没有被守住、且最容易回退的边界补“护栏”。新增护栏前先确认现有测试没有已经覆盖同一个风险。

### 5. 收口长期有效的装配文档

当前已经补了：

- `docs/architecture/httpapi-assembly-boundaries.md`
- `docs/architecture/app-assembly-boundaries.md`
- `docs/development/repository-structure.md`
- `docs/architecture/external-client-boundary-inventory.md`

长期架构文档入口已收口到：

- `docs/architecture/README.md`

接下来要做的是控制文档数量和语义漂移，尽量把长期有效的规则收口到少数文档，而不是让大量计划文档替代正式架构说明。

外部 client 依赖当前不适合一刀切禁止。后续应优先从 `external-client-boundary-inventory.md` 里的热点目录做窄切片：`management` 是待淘汰接口，业务数据应逐步改为仓内 database/repository 访问；`openai` / `nanobanana` 这类真实外部服务 adapter 则继续收口到本地接口或装配层。

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
