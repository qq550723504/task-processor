# Repository Structure

## 目标

这份文档说明仓库当前推荐的目录职责，以及后续结构收口的方向。当前重点是让正式入口、调试入口、长期工具和本地产物位置保持清晰，避免旧入口和临时产物继续污染仓库结构。

## How To Use This Document

当变更涉及以下问题时，应从 `docs/architecture/README.md` 进入后再落到这里：

1. 顶层目录里该放什么，不该放什么。
2. 正式入口、调试入口、长期维护工具应该分别落在哪。
3. 本地日志、浏览器状态、临时文件、调试二进制该放在哪个运行态目录。
4. `internal/`、`cmd/`、`hack/`、`tools/`、`internal/platforms/` 是否开始混入不该存在的源码外产物。

如果问题首先是“代码应该由哪个业务/装配边界拥有”，先看
`docs/architecture/project-boundaries.md` 和对应专项边界文档；只有当问题落到
目录职责、仓库布局或本地产物放置时，再以这份文档作为直接规则来源。

## 顶层目录约定

- `cmd/`
  - 只放正式服务或正式任务入口。
  - 当前官方入口只有：
    - `listing-control-plane`
    - `product-listing-api`
    - `shein-listing`
    - `temu-listing`
  - 不再新增临时调试可执行程序。
  - 历史爬虫、订阅、兼容 API、地址复制、一次性迁移或调试入口不得回流到 `cmd/`；确需保留时放到 `hack/`、`tools/` 或业务模块内。
  - 不放本地 `logs`、`tmp`、`__debug_bin*` 等运行态产物；这类文件统一放到仓库根 `.local/`。
- `hack/`
  - 放调试、试验、验证程序。
  - `hack/debug` 是当前受管的非生产调试入口目录。
  - `hack/k8s` 可保留 Kubernetes 相关运维验证支持。
  - 不放本地 `tmp`、`logs`、`bin` 等运行态产物；调试产物统一放到仓库根 `.local/`。
- `tools/`
  - 放长期维护的小工具模块。
  - 适合独立可复用的开发工具，不适合一次性验证程序。
  - 不放 `node_modules`、生成的 `.exe`、`result` 输出目录等本地产物；依赖和输出应保持可重建。
- `scripts/`
  - 放运维脚本、迁移脚本、部署脚本和一次性自动化脚本。
- `.local/`
  - 放本地日志、浏览器状态、临时文件、开发期二进制和其他运行态产物。
  - 默认不提交，避免继续污染仓库根目录。
  - 推荐按 `logs/`、`tmp/`、`chrome/`、`bin/`、`dev-logs/`、`playwright-cli/` 分子目录管理。

## 当前护栏

目录约定由以下测试守住：

- `TestCmdContainsOnlyOfficialEntrypoints`
- `TestCmdProductionEntrypointsDoNotImportDomainOrInfraPackages`
- `TestCmdPackagesDoNotImportAppCompatibilityLayers`
- `TestHackContainsOnlyManagedSupportAreas`
- `TestHackSupportAreasContainNoLocalArtifacts`
- `TestTrackedLocalArtifactsStayOutOfProductionEntrypoints`
- `TestProductionEntrypointsContainNoLocalArtifacts`
- `TestTrackedLocalArtifactsStayOutOfTools`
- `TestToolsContainNoLocalArtifacts`
- `TestInternalPackagesContainNoLocalArtifacts`
- `TestSDSLoginRuntimeStateStaysOutOfInternalPackages`
- `TestPlatformRegistrationPackagesStayThin`
- `TestPlatformRegistrationPackagesContainNoLocalArtifacts`

如果需要新增正式入口、调试目录或平台注册文件，应在同一变更中更新这份文档和对应 allowlist。

## internal 当前稳定边界

- `internal/app`
  - 运行装配层，负责 bootstrap、consumer、httpapi、worker、scheduler、listing runtime、listing control-plane runtime 等运行时组装与协调。
  - 不承载具体平台业务规则。
  - 其中 `internal/app/httpapi` 当前只负责共享 HTTP runtime 协调；各业务 HTTP builder 已下沉到 `internal/*/httpapi`。
  - `internal/app/runtime/listing` 和 `internal/app/runtime/listingcontrol` 是正式 runtime 入口背后的运行装配归属地。
- `internal/*`
  - 不放本地 `.local`、`logs`、`tmp` 等运行态产物；业务包、基础设施包和平台包都应保持源码可审查。
  - SDS 登录态、浏览器状态和 auth/cookie JSON 必须放在仓库根 `.local/sds/` 或其他明确忽略的运行态目录，不能放在 `internal/sdslogin/data/`。
- `internal/listingkit`
  - 产品主域兼容 facade，承接 ListingKit 的任务、工作台、审核、提交编排、多租户产品能力。
  - `internal/listingkit/httpapi` 是 ListingKit 专属 HTTP 装配、认证运行时和 AI client helper 的稳定归属地。
  - 新增平台规则、商品事实规则、可复用资产规则不应继续放入 root `internal/listingkit`。
- `internal/listing`
  - 平台中立的 Listing 子领域目标位置，当前包括 `preview`、`studio`、`submission` 等已抽出的稳定 seam。
- `internal/shein` / `internal/temu` / `internal/amazon`
  - 当前仍然存在的历史平台实现目录。
  - 本阶段不为目录一致性做大规模迁移，只在有明确所有权收益时逐步收口。
- `internal/marketplace/*`
  - 新的 marketplace 规则目标位置，当前已承接部分 SHEIN publishing/workspace 与 Amazon marketplace 结构。
- `internal/platforms`
  - 平台注册和选择层，只保留 module descriptor、文档和必要测试。
  - 不放本地 `tmp`、`logs`、`bin` 等运行态产物；这类文件统一放到 `.local/`。
- `internal/pkg`
  - 纯技术通用件目录。
  - 继续保留现状，不在本阶段重命名。

## 后续收口方向

- 平台实现逐步从历史目录收口到统一的平台边界，例如 `internal/marketplace/*` 或经过批准的目标包。
- `internal/listingkit` 继续作为产品主域 facade 和编排中心，避免回流依赖历史平台 runtime。
- `internal/app` 继续保持运行装配职责，避免混入产品或平台业务逻辑。
- 生成型依赖基线和包地图只作为当前验证证据，不作为长期结构说明；需要时重新运行脚本生成。

## 本阶段明确不做的事情

- 不迁移 `internal/listingkit`、`internal/shein`、`internal/publishing` 的正式实现目录。
- 不为目录一致性进行大规模包重命名。
- 不把历史调试或兼容入口重新放回 `cmd/`。
- 不改动业务 API、HTTP 路由、消息结构、数据库结构。
- 不重命名 `internal/pkg`，也不引入新的共享层抽象。
