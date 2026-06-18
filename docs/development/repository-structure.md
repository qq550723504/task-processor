# Repository Structure

## 目标

这份文档说明仓库当前推荐的目录职责，以及后续结构收口的方向。第一阶段只整理目录语义和本地产物约定，不做大规模业务包迁移。

## 顶层目录约定

- `cmd/`
  - 只放正式服务或正式任务入口。
  - 不再新增临时调试可执行程序。
  - 不放本地 `logs`、`tmp`、`__debug_bin*` 等运行态产物；这类文件统一放到仓库根 `.local/`。
- `hack/`
  - 放调试、试验、验证程序。
  - `hack/debug` 是当前受管的非生产调试入口目录。
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
- `TestHackContainsOnlyManagedSupportAreas`
- `TestHackSupportAreasContainNoLocalArtifacts`
- `TestTrackedLocalArtifactsStayOutOfProductionEntrypoints`
- `TestProductionEntrypointsContainNoLocalArtifacts`
- `TestTrackedLocalArtifactsStayOutOfTools`
- `TestToolsContainNoLocalArtifacts`
- `TestPlatformRegistrationPackagesStayThin`
- `TestPlatformRegistrationPackagesContainNoLocalArtifacts`

如果需要新增正式入口、调试目录或平台注册文件，应在同一变更中更新这份文档和对应 allowlist。

## internal 当前稳定边界

- `internal/app`
  - 运行装配层，负责 bootstrap、consumer、httpapi、worker、scheduler 等运行时组装与协调。
  - 不承载具体平台业务规则。
  - 其中 `internal/app/httpapi` 当前只负责共享 HTTP runtime 协调；各业务 HTTP builder 已下沉到 `internal/*/httpapi`。
- `internal/listingkit`
  - 产品主域，承接 ListingKit 的任务、工作台、审核、提交编排、多租户产品能力。
  - `internal/listingkit/httpapi` 是 ListingKit 专属 HTTP 装配、认证运行时和 AI client helper 的稳定归属地。
- `internal/shein` / `internal/temu` / `internal/amazon`
  - 当前仍然存在的历史平台实现目录。
  - 本阶段不迁移、不改 import，只在后续迭代中逐步收口。
- `internal/platforms`
  - 平台注册和选择层，只保留 module descriptor、文档和必要测试。
  - 不放本地 `tmp`、`logs`、`bin` 等运行态产物；这类文件统一放到 `.local/`。
- `internal/pkg`
  - 纯技术通用件目录。
  - 继续保留现状，不在本阶段重命名。

## 后续收口方向

- 平台实现逐步从历史目录收口到统一的平台边界，例如 `internal/platform/*`。
- `internal/listingkit` 继续作为产品主域中心，避免回流依赖历史平台 runtime。
- `internal/app` 继续保持运行装配职责，避免混入产品或平台业务逻辑。

## 本阶段明确不做的事情

- 不迁移 `internal/listingkit`、`internal/shein`、`internal/publishing` 的正式实现目录。
- 不改动业务 API、HTTP 路由、消息结构、数据库结构。
- 不重命名 `internal/pkg`，也不引入新的共享层抽象。
