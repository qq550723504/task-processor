# Repository Structure

## 目标

这份文档说明仓库当前推荐的目录职责，以及后续结构收口的方向。当前重点是让正式入口、调试入口、长期工具和本地产物位置保持清晰，避免旧入口和临时产物继续污染仓库结构。

## How To Use This Document

先从下面的[当前入口地图](#current-entrypoint-map)判断运行路径，再从[新代码落点](#current-owner-landing)进入已有 owner；无需先遍历历史计划。目录/命令完整清单只维护在本文件，领域合同与完整退休规则仍分别属于 [Project Boundaries](../architecture/project-boundaries.md)、[Module Target Mapping](../refactoring/module-target-mapping.md) 和 [Legacy Register](../refactoring/legacy-register.md)。本地图是导航，不新建架构权威或依赖准入。

当变更涉及以下问题时，应从 `docs/architecture/README.md` 进入后再落到这里：

1. 顶层目录里该放什么，不该放什么。
2. 正式入口、调试入口、长期维护工具应该分别落在哪。
3. 本地日志、浏览器状态、临时文件、调试二进制该放在哪个运行态目录。
4. `internal/`、`cmd/`、`hack/`、`tools/`、`internal/platforms/` 是否开始混入不该存在的源码外产物。

如果问题首先是“代码应该由哪个业务/装配边界拥有”，先看
`docs/architecture/project-boundaries.md` 和对应专项边界文档；只有当问题落到
目录职责、仓库布局或本地产物放置时，再以这份文档作为直接规则来源。

<a id="current-entrypoint-map"></a>

## 当前入口地图

本节及命令清单静态核对于 `main @ eb9e019686564b976c1a1a9828eeddb454adabac`（2026-09-25）。只确认源码路径、装配关系和命令分类，不声称执行过当前基线测试、已部署或通过产品/生产验收。[Current Refactoring Status](../refactoring/current-refactoring-status.md) 中较早基线的成熟度记录不能代替本节的入口核对；本节也不重签那些历史验收。

| 要做什么 | 从哪里进入 | 当前边界 |
| --- | --- | --- |
| 当前账户、组织、来源账号及按配置开放的业务模块 | [`cmd/current-application/main.go`](../../cmd/current-application/main.go) → [`internal/app/runtime/currentapplication`](../../internal/app/runtime/currentapplication) → [`current_application.go`](../../internal/app/httpapi/current_application.go) | 独立应用装配；`NewCurrentApplicationWithOptions` 不调用旧默认 feature composition。采集、Browser capture、成员、推广和商业模块的启用取决于配置、显式 options 和依赖，不能把路由声明当作全部默认开放。 |
| 排查保留的 ListingKit API / 旧来源 handoff | [`cmd/product-listing-api/main.go`](../../cmd/product-listing-api/main.go)、[`internal/pkg/httpapicmd`](../../internal/pkg/httpapicmd)、[`composition_builder.go`](../../internal/app/httpapi/composition_builder.go) | 旧装配仍建立 ListingKit，并在对应条件下接入 `sourceaccount` 和 `compatibility/listingkit/sourcehandoff/a1688`；这条路径尚未退休，不是新业务的默认落点。 |
| 平台执行、control-plane 或图片 worker | 本文件下方的受维护命令及其运行装配归属 | 保留已有单一执行/重试 owner；不是多个独立新产品，也不代表平台成熟度相同。 |
| 修改当前 Console / BFF | [`web/listingkit-ui`](../../web/listingkit-ui) | 复用当前页面、Shell 与身份边界；目录仍叫 ListingKit 不等于其中所有页面都应退休。最终导航服从 UI Authority，不由后端目录名决定。 |
| 初始化、预检或恢复 | 本文件下方的运维入口和明确维护脚本 | 命令存在不是执行许可。当前空库初始化与历史 migration/preflight 必须区分；不得为新系统引入历史迁移前置。 |

当前入口示例（仅说明正常命令形态，不授权连接真实依赖）：

```text
go run ./cmd/current-application -config /absolute/private/current-application.json
```

`-config` 是必需的私有 JSON manifest；当前应用并不使用旧 API 的默认 YAML 配置。该命令不会替代显式 schema 初始化，也不是无依赖 demo。不要通过启动旧入口、回退旧表或增加 wrapper 来填补当前模块尚未开放的能力。实际参数和可选模块以该入口及其 runtime 配置为准。

<a id="current-owner-landing"></a>

## 新代码落点

以下是现有归属的阅读入口，不是新的全仓迁移计划；使用具体 API 前仍须满足现有合同与精确依赖 guard。完整 file/package 退休归属只维护在 [Mapping](../refactoring/module-target-mapping.md) / [Register](../refactoring/legacy-register.md)。

| 变更内容 | 当前阅读入口 | 不应做什么 |
| --- | --- | --- |
| 应用配置、依赖组装、生命周期 | `internal/app/runtime/currentapplication`、`internal/app/httpapi` | 不把业务规则写入 app；不把旧默认装配接入当前应用。 |
| 来源证据、标准商品、资产、增强 | `internal/product/sourcing`、`catalog`、`asset`、`enrichment`；采集协调见 `internal/app/productsourcing` | 不把商品事实放回 root ListingKit，不恢复 ProductEnrich/ProductImage 旧 task 系统。 |
| 平台中立上架与平台规则 | `internal/listing/*`、`internal/marketplace/*` | 不再向历史 `listingkit` / `publishing` / `workspace` 壳层新增长期职责，不复制提交状态机。 |
| 当前身份、组织与来源账号 | 既有 `authidentity` / `authruntime` / `workbenchcontext`、`internal/organization`、`internal/sourceaccountregistry` | 不新建 IAM，不把来源账号当组织成员，不新增 `sourceaccount` / `tenantbridge` 旧消费者。 |
| 商业订单、资金、组织资源 | `internal/commercial/billing`、`internal/ledger/money`、`internal/ledger/orgresource` | 保持报价/订单、资金事实、资源余额各自归属；模块存在不代表微信/支付宝渠道已实现或可用。 |
| AI 模型治理与受控工具 | `internal/aicapability`、`internal/commercetool`、具体领域的 Tool adapter | 不把 provider SDK 或数据库访问直接放进 Agent/Tool 合同，不另建业务重试 owner。 |
| 基础运行机制与外部适配 | `internal/platform/*`、`internal/integration/*` | 不仅凭名字向 `core` / `kernel` / `infra` / `pkg` 增加宽泛职责；按已有精确能力复用，不为统一名称搬目录。 |

### 旧路径如何退出

沿当前用户交付路径执行已有的 **EXTRACT → 切换当前调用方 → RETIRE**：先明确仍需保留的行为及当前 owner，再用适用测试确认当前消费者不依赖旧 owner，最后按该任务范围移除旧调用/实现。新路径已存在，不代表旧路径已删除；改目录名也不能证明依赖已断开。

Marketplace/Listing 归 #29，来源闭环与旧 1688 handoff 归 #30，数字租户依赖归 #301，现有自动边界归 #300。退出条件和剩余消费者记录在对应 Issue / Register，不在这里复制实时数量。保持有效的业务、安全、权限、幂等测试；不新增 compatibility/fallback/双读双写，不执行旧数据迁移，不把全仓清理变成当前业务交付前置。本地图本身不授权代码删除或环境操作。

## 顶层目录约定

CURRENT STATE：命令清单核对于上述 `main @ eb9e019686564b976c1a1a9828eeddb454adabac`，
与 `TestCmdContainsOnlyOfficialEntrypoints` 及实际受维护路径一致。README 和 Code Guide
只引用这里；受维护不等于已部署或通过生产验收。

- `cmd/`
  - 只放受维护的产品运行入口或有明确所有者的运维入口。
  - 当前六个产品运行入口为：
    - `current-application`
    - `image-agent-temporal-worker`
    - `listing-control-plane`
    - `product-listing-api`
    - `shein-listing`
    - `temu-listing`
  - 当前二十个运维入口为：
    - `account-acceptance-fixture`
    - `1688-batch-import`
    - `1688-local-agent`
    - `commercial-owner-schema-migrate`
    - `fingerprint-browser-installer`
    - `listing-scheduler`
    - `listingkit-identity-preflight`
    - `listingkit-owner-scope-dry-run`
    - `listingkit-owner-scope-exceptions`
    - `listingkit-schema-migrate`
    - `playwright-installer`
    - `product-listing-api-schema-migrate`
    - `product-acquisition-init`
    - `shein-import-platform-recovery`
    - `shein-login-worker`
    - `store-service-history-migrate`
    - `source-account-ownership-preflight`
    - `source-account-registry-schema-init`
    - `organization-membership-schema-init`
    - `referral-schema-init`
  - `image-agent-temporal-worker` 的构建归属为 `deployments/docker/Dockerfile.product-listing-api`，运行装配归 `internal/app/worker/imageagent`。
  - `1688-local-agent` 的维护入口为 `scripts/1688-local-agent-acceptance.ps1`；归 1688 source runtime。列入清单不授权连接真实账号或执行该脚本。
  - `1688-batch-import` 的维护入口为 `scripts/1688-batch-import.ps1`；属 #398 路线 B 的执行器本地队列切片 S1，只驱动本地队列中的单条商品并回读终态。已确认的 actor/组织必须由调用方显式提供，不从浏览器会话推断；退出码 3 表示结果未知，只允许人工核实，不允许重跑。列入清单不授权对真实 1688 账号执行批量采集。
  - 每个运维入口必须由 `.github/`、`deployments/` 或 `scripts/` 中的构建、部署或脚本引用明确其维护所有者；未归类或同时归类为两类的入口不允许保留在 `cmd/`。
  - `shein-import-platform-recovery` 由 `scripts/shein-import-platform-recovery.ps1` 运行；脚本默认 dry-run，只有同时提供 `-Execute` 和 dry-run 返回的 `-ConfirmFingerprint` 才会请求写入。
  - `store-service-history-migrate` 由 `scripts/store-service-history-migrate.ps1` 运行；脚本默认只读 `verify`，只有显式选择 `backfill` 才会写入一个有界批次，只有显式选择 `constraints` 且 Phase D 重验通过才会执行 PostgreSQL staged constraints。
  - 不再新增临时调试可执行程序。
  - `source-account-ownership-preflight` 由 `scripts/source-account-ownership-preflight.ps1` 维护，是历史 Source Account 迁移的只读运维预检，不是 #301 当前新系统开发前置；两个数据库连接从环境注入，不执行 backfill 或生产 cutover。运行说明见 `docs/operations/source-account-ownership-preflight.md`。保留历史运维工具不恢复已取消的迁移授权。
  - `source-account-registry-schema-init` 由 `scripts/source-account-registry-local-acceptance.ps1` 维护，只初始化 #368 当前 Source Account registry 的空库 schema；不读取、迁移或兼容旧 Source Account 数据。
  - `organization-membership-schema-init` 由 `deployments/docker/account-compose` 维护，只初始化本账户中心实例的组织成员回执 schema；不读取、迁移或兼容旧成员数据。
  - `commercial-owner-schema-migrate` 由 `scripts/commercial-owner-schema-migrate.ps1` 维护；分别针对私有 schema-owner manifests 初始化 canonical money 数据库的钱包/结算 schema，以及 commercial-owner 数据库的订单与 orgresource schema，不自动切换数据库或操作生产数据。
  - `account-acceptance-fixture` 由 `deployments/docker/account-compose` 的 `acceptance` profile 维护，只通过官方 ZITADEL API 创建并回读隔离验收组织与角色授权，写入项目私有的脱敏 manifest；不提供生产路由、不写业务事实表。
  - `product-acquisition-init` 由 `scripts/product-acquisition-init.ps1` 显式委托，要求私有配置路径和精确空库名称确认；只初始化 #398 当前 Product 采集的五张表及 runtime grants，不创建数据库或角色，不自动执行，不迁移旧数据。准入见 #398 评论 5643032970。
  - `referral-schema-init` 由 `scripts/referral-schema-init.ps1` 维护，通过显式 DSN 文件初始化当前推广注册空库 schema；执行范围和验证要求见 [推广注册运维说明](../engineering/referral-registration.md)。
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
  - 日志库和测试默认只输出到 stdout；只有 app 运行装配可以通过显式配置启用文件日志。
  - 仓库内受维护的相对日志路径统一位于 `.local/logs/`；测试文件输出使用 `t.TempDir()`。

## 当前护栏

目录约定由以下测试守住：

- `TestCmdContainsOnlyOfficialEntrypoints`
- `TestCmdProductionEntrypointsDoNotImportDomainOrInfraPackages`
- `TestInternalCmdEntrypointsDoNotImportDomainOrInfraPackages`
- `depguard: cmd_domain_dependencies`
- `TestCmdPackagesDoNotImportAppCompatibilityLayers`
- `depguard: cmd_legacy_app_compatibility`
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
- `depguard: commercetool_boundaries`

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
  - `TestInternalPackagesContainNoLocalArtifacts` 检查实际文件系统，包括 Git 忽略路径；`.gitignore` 不能作为在源码包下保留运行态文件的依据。
- `internal/listingkit`
  - CURRENT STATE：仍承接旧任务、工作台、审核、提交与 HTTP 能力；按 Legacy Register 执行 EXTRACT → RETIRE，不是长期产品 facade。
  - `internal/listingkit/httpapi` 保留 ListingKit 专属 HTTP／角色适配 helper；当前 Organization 路由鉴权由 `internal/app/httpapi/server_auth.go` 装配，身份与组织解析归既有 authidentity/authruntime/workbenchcontext owner。
  - 新增平台规则、商品事实规则、可复用资产规则不应继续放入 root `internal/listingkit`。
- `internal/commercetool`
  - 拥有框架中立的 Tool Definition、Schema、Registry、Agent Allowlist、Invocation Policy 和 Tool Audit Port。
  - 生产代码不得依赖 Framework、Transport、Workflow、Persistence、Provider SDK、Marketplace Client 或 domain implementation packages；领域能力由装配层通过窄 Service/Query ports 注入 Executor。
  - `depguard: commercetool_boundaries` 对该目录全部生产 Go 文件使用 strict allowlist，禁止通过宽泛 `internal` 或 SDK 父命名空间绕过边界。
- `internal/listing`
  - 平台中立的 Listing 子领域目标位置，当前包括 `preview`、`studio`、`submission` 等已抽出的稳定 seam。
  - `internal/listing/task` 拥有 task-scoped resource resolution 与 tenant-admin checker 窄合同；现有 ListingKit persistence adapter 可以实现该 Port，但调用方不能依赖 legacy Task DTO 或全局默认权限实例。
- `internal/product/catalog/tools/canonicalinspect`
  - `product.canonical.inspect` 是 Phase 2A 的只读 B0 Tool Adapter：通过 `internal/listing/task` 解析授权资源，只读取权威 `ProductSnapshot`，并输出严格、去除任意 authority metadata 的投影。
  - 该包不拥有 task、repository、write/publish 能力、HTTP/worker/runtime；其 8 MiB Catalog 输入边界由只读 persistence adapter 通过数据库侧条件投影执行，不是共享 Catalog 的全局限制；Phase 2B 之前没有用户可访问的 Agent Runtime。
- `internal/integration/commercetoolauth`
  - 只负责把 `authidentity.AuthenticatedIdentity` 和现有 Casbin owner 适配到 Commerce Tool contracts，不复制 IAM policy。
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

### Phase 2 runtime foundation closure

Historical Phase 2 closure evidence (not a new dependency permission) establishes three runtime ownership roots:

- `internal/app` owns lifecycle and provider registration, migration execution,
  final HTTP instrumentation, and ordered shutdown.
- `internal/platform` owns the application runtime mechanisms: config loading,
  logging, database and Goose, Redis, RabbitMQ, worker pool, Temporal dial,
  feature flags, and tracing.
- `internal/integration` owns external adapters: OpenAI, Gemini, GRSAI, S3, and
  remote image HTTP.

This is a runtime-foundation closure, not the end of the business migration.
At this close, only nine target domain roots satisfy the final dependency rule:
`internal/listing`, `internal/product`, `internal/marketplace`,
`internal/agent`, `internal/knowledge`, `internal/resourcecatalog`,
`internal/commercial`, `internal/ledger`, and `internal/organization`.
Historical business roots remain under non-growth ceilings until their owning
product, marketplace, listing, agent, or organization phase removes concrete
runtime dependencies.

- 平台实现逐步从历史目录收口到统一的平台边界，例如 `internal/marketplace/*` 或经过批准的目标包。
- `internal/listingkit` 按 [Legacy Register](../refactoring/legacy-register.md) 抽取有效行为到当前 owner，调用方切换后退休旧路径；不作为永久 facade。
- `internal/app` 继续保持运行装配职责，避免混入产品或平台业务逻辑。
- 生成型依赖基线和包地图只作为当前验证证据，不作为长期结构说明；需要时重新运行脚本生成。

## 本阶段明确不做的事情

- 不迁移 `internal/listingkit`、`internal/shein`、`internal/publishing` 的正式实现目录。
- 不为目录一致性进行大规模包重命名。
- 不把历史调试或兼容入口重新放回 `cmd/`。
- 不改动业务 API、HTTP 路由、消息结构、数据库结构。
- 不重命名 `internal/pkg`，也不引入新的共享层抽象。
