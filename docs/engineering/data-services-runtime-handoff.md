# Data Services v1 runtime handoff

执行 [#621](https://github.com/qq550723504/task-processor/issues/621)，主要 [PR #623](https://github.com/qq550723504/task-processor/pull/623)；设计依据为 [冻结架构](../architecture/data-services-v1.md)。本文件交接已实现消费者，不新增产品要求或运行授权。公共导航、原生权限目录、installer、composition、worker 与运行实例由 **#619** 唯一写入；本 PR 不改共享启动路径。

## 用户入口与操作

接线并获得适用运行授权后，正常登录、选择企业，通过导航进入：

- `/workbench/data/market`：选择实际配置的 Amazon 站点，输入关键词（可附类目）、原生类目节点或 ASIN/当前站点商品链接，选择字段与最多 200 条，确认最大 DATA_ROW 用量后提交。可查看任务、取消未保存项、分页读取/下载 JSON，并打开原批次“我的数据”。
- `/workbench/data/api`：创建密钥、保存仅显示一次的凭据，设置明确的到期日、能力、每日条数/月预算与可选 CIDR；编辑、禁用、撤销和读取历史。统计来自本人真实任务与原额度记录，UTC 日/月，费用区分已确认和已保存待确认。
- 市场的“提交定制需求”：最多 200 条，描述用途、规格、时间与 CSV/JSON/Excel 格式。专员线下确认报价及规格，客户查看进度，交付后打开原申请者批次。
- `/workbench/admin/data-customization`：独立平台专员入口，查看原申请者标识、评估、记录确认规格/报价、制作进度、上传已确认格式的文件交付。企业管理员角色不能代替平台权限。

浏览器请求通过本 PR 的专用 BFF，复用 server session 和当前企业 cookie。未知结果保留原请求 UUID/完整条件在当前页面会话的 `sessionStorage`，先核实原请求；密钥明文不进入存储。切换企业前必须处理当前未知请求。列表仅摘要；定制详情展示最近 100 个事件，所有事件仍由 owner 持久保存。

页面开关为服务端 `LISTINGKIT_DATA_SERVICES_ENABLED=true`，默认关闭。#619 只有完成下方注入、权限及路由接线后才开放准确导航；单独打开页面开关不能证明可用。其余环境沿当前 UI 的 `LISTINGKIT_SERVICE_API_BASE`（固定 `/api/v1`）及 `LISTINGKIT_PUBLIC_BASE_URL`，正常 session/TLS 路径。

## 安装与数据库

只用于**获授权的全新空业务安装**。三个显式 owner installer 在当前 Product 专用数据库按顺序执行：

1. `internal/integration/persistence/dataservice.InstallSchema`：credentials、commands、quota 三表。
2. `internal/integration/persistence/product/dataacquisition.InstallSchema`：jobs、items 两表。
3. `internal/integration/persistence/dataservice.InstallCustomSchema`：custom requests、events、commands 三表。

表名由各包 `Tables` / `CustomTables` 返回；依赖当前 SRC、Catalog 和 Collection owner schema 已安装。Product 侧这些八表、SRC/Catalog/Collection必须在**同一物理 Product 数据库**，用于一个 caller-owned 事务保存数据/来源/批次/额度事实；使用当前 owner 受限 pool，不交给通用业务 DB。Resource 数据库保持独立，复用原 Resource schema/事务/恢复合同。

`dataservicesapp.NewModule` 只验证 schema，不在 serving 路径执行 DDL。没有迁移、旧数据转换、第二事实源、legacy wrapper 或兼容路径。不要在已有业务库运行“修复安装”，也不要以重建/删除 volume 完成接线。

## 注入与原生权限

`internal/app/dataservices.Dependencies` 是唯一 composition 入口：

| 参数 | 现有 runtime 提供 |
| --- | --- |
| ProductDB | 当前受限 Product pool；同 DB 当前 SRC/Catalog/Collection |
| Access / Live / Specialist / Funding | 同一个 `dataserviceauth.Authorizer`，消费正式 IAM、用户 active 状态、native RoleModules 和企业 deny-only suspension |
| Provider | `amazon.New(amazon.Options{ExecutablePath, DriverDirectory, EnabledSites})` |
| Starter | `dataservicesruntime.TemporalStarter{Client: currentTemporalClient}`（`internal/app/runtime/dataservices`） |
| Charges | 接收本 module 的 `ConsumerChargeOwner`，登记到当前 Resource `ConsumerChargeService`；返回带 Lookup 的该服务 |
| TrustedProxyCIDRs | 空则用真实 socket/TLS；只有明确部署的单跳可信 TLS 代理可以配置 CIDR |

`dataserviceauth.NewAuthorizer(exactReader, activeUserReader, serviceTokenFunc, currentProjectID, nativePolicy, suspensionChecker)`：

- exactReader 复用正式 `ReadExactServiceProjectAuthorization`，核对原 subject/project/org/member grant；不能替换为缓存 membership 或本地表。
- activeUserReader 使用 `NewActiveUserClient(currentZitadelBase, normalHTTPClient)` 的官方 `/v2/users/{id}`，需 service credential 具备准确读取用户/项目授权的权限。server-only token，不保存/刷新用户 bearer。
- nativePolicy 复用正式 Casbin/RoleModules，suspensionChecker 复用当前企业 owner，任何依赖缺失均 fail closed。

公共 module catalog 消费 `dataservicesapp.ModulePermissions("data-market")` / `("data-api")`：分别为 `workbench.data-market.use`、`workbench.data-api.manage`。**不会隐式授予已有“我的数据”权限**：

| 当前操作 | 必要原生授权 |
| --- | --- |
| 市场提交/定制申请 | data-market + `workbench.collection.manage` |
| 本人 Console 任务/结果读取 | `workbench.collection.read` |
| 密钥管理/API 概览 | data-api；概览读取还需 collection.read |
| API 创建任务 | 密钥 amazon.acquire + 当前 data-api/data-market/collection.manage |
| API 读结果 | 同一密钥 amazon.result.read + 当前 data-api/collection.read |
| 专员操作 | 当前 verified `listingkit.platform_admin` + 主体 active；交付再核对原申请者准确 grant/collection.manage |

企业资金仅当前 tenant-admin，其他成员固定原 member allocation；任务创建后不切换资金来源。密钥/任务冻结原创建者及 canonical member，grant 重建不能复活旧凭据。

feature-local `internal/dataservice/httpapi` 负责框架 adapter，app module 仅注入并转发；用当前 descriptor registrar 消费 `module.BuildRoutes()`，包括：

- Console：`/api/v1/workbench/data-services`，现有 verified identity / live organization 边界。
- Specialist：`/api/v1/platform/data-customization`，current identity with verified platform roles / org-none。
- External：`/data-api/v1/amazon/jobs`，公开路由**只允许 DataKey 认证**，不注入用户 bearer、企业 cookie 或客户端身份 header。

不能用假 global identity、第二个 shared router 或绕过 descriptor 准入挂载。

## 浏览器、Temporal 与 Resource 恢复

#619 的正常接线候选使用 [原生 Data Services Compose profile](../../deployments/docker/account-compose/DATA_SERVICES.md)，显式安装于新空 Product 库；`data_services_runtime` 与原 `source_acquisition_runtime` 使用同一物理库、不同受限 pool，原 pool 不增权。当前正式 Workbench 没有企业业务停用 owner，而本特性 Authorizer 要求非空 suspensionChecker，故候选 **BLOCKED**、尚不能作为可用实例交付。此授权边界等待用户产品决定及适用设计准入；不能用永远允许的假 checker 接线。既有运行实例和数据保留。

Amazon 复用现有 Playwright Go SDK，先安装匹配 SDK 的 driver 与 Chromium，再显式配置路径。`DriverDirectory` 下必须有 `node`（Windows `node.exe`）及 `package/cli.js`；另提供真实 browser executable。`PLAYWRIGHT_NODEJS_PATH` / `PLAYWRIGHT_CLI_PATH` 如果存在必须等于这两个明确路径，否则 fail closed。serving 不下载安装，也不清空进程全局环境。

支持配置 `us uk de fr it es ca jp au mx br in ae sa`，界面/API options 只返回实际配置且本地路径就绪的站点。Ready 检查文件存在，**不是真实站点可抓取证明**。站点公开页面可能挑战或结构不支持，诚实返回失败/部分完成，不自动登录、换代理、绕过 CAPTCHA 或生成缺失数据。保留正常 DNS/公网 IP 固定、TLS 校验，匿名 context 禁用脚本/下载/子资源/ServiceWorker/WebSocket。

复用当前 namespace/client/worker 生命周期：task queue `data-services-v1`，由 `dataservicesruntime.RegisterWorker(worker, module.Runner())` 注册，再由 #619 原 runtime 启停。workflow `DataAcquisitionV1` 固定原 org/actor/job identity/hash/deadline，执行最长 30 分钟，原 memo 核对；创建重试/读取会修复原已提交未启动任务，worker 重启继续原任务，不建设新 scheduler。

Resource owner map 在创建服务时增加 `orgresource.ConsumerAmazonData` (`amazon_data_v1`) → module factory 传入的 owner，同时保留原消费者。复用 `NewGormConsumerChargeRepository(resourceDB, currentTransactionConfig)` 及现有 `RecoverDue` 循环；不能另外创建 wallet 扣费路径。每成功保存商品 1 DATA_ROW，5 分预算计量；变体不额外扣条数。先 Lookup 原 reservation 再 reserve，未知响应保留原 operation；失败/撤权先 fence，再凭原 proof 释放；已保存待确认仅结算原 reservation，不再 fetch。

## 外部 API 最小接入

在正常服务 HTTPS origin 使用：

```http
POST /data-api/v1/amazon/jobs
Authorization: DataKey <public-key-id>.<once-only-secret>
Idempotency-Key: <new-command-uuid>
Content-Type: application/json

{"query":{"site":"us","mode":"keyword","keyword":"desk lamp","limit":10,"fields":["asin","title","price","currency"]},"maximumRows":10,"maximumCostFen":50}
```

返回 **202** 和原任务状态。`GET /data-api/v1/amazon/jobs/{id}` 轮询；`GET .../{id}/results?limit=100`，返回 nextCursor 后用 cursor 续页。`GET /data-api/v1/amazon/jobs/by-command/{uuid}` 核实未知创建。新任务新 UUID；重试原 UUID/完整载荷，载荷变化冲突。其他读取和已确认普通命令返回 200。64 KiB JSON，拒绝重复/未知字段；上传/结果最多 2 MiB。当前批最多 200 条。

`DATA_UNKNOWN` 保留原请求，先查询原 UUID；403 表示原身份/权限/凭据/CIDR 不满足。IP 以 socket 为准；配置可信单跳代理后只接受一个 X-Forwarded-For IP 和 `X-Forwarded-Proto: https`，歧义链拒绝，不能全局信任任意 ClientIP。

专员上传格式为确认规格中的格式；CSV 精确表头 `title,description,brand,images,sku,currency,price,stock`；JSON 为当前 OwnProduct 数组；Excel 使用现有 Collection 导入模板及 ZIP/formula/external-link/行数 guard。交付与原申请者 SRC/Catalog/Collection 批次和回执同事务；定制不消费 DATA_ROW，也没有线上报价付款状态。

## 证据与交接边界

开发证据包括真实隔离 Product/Resource 双库保存与原 reservation 恢复、密钥/并发 quota/跨原 UTC 窗口、custom same-transaction delivery、Temporal testsuite、实际 handler（受控 IAM/provider fixture）、BFF/原命令恢复/UI 类型与构建。Figma 两个内容 frame 的受控浏览器截图用实际组件及限定 fixtures 校准；临时视觉服务器不提交、不作为交付实例。

真实 IAM 配置、实际 Amazon 请求、正式 Temporal server/worker 恢复、#619 保留运行实例与用户业务验收均需独立记录，当前本线程为 **NOT_RUN**。代码/CI/review 不能替代这些结果。#621 在实现与独立检查后进入 In Review；未获授权不 merge、deploy、close Issue、触碰真实数据或调用付费 provider。
