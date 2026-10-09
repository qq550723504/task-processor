# 我的供应链 / SHEIN 美国站操作与启动

当前实现对应 #605 / PR #606。用户入口为 `/workbench/data/mine` 与 `/workbench/supply/mine`，五个阶段为 `waiting`、`missing`、`ready`、`review`、`uploaded`。正式运行采用 `cmd/current-application` 与当前 Console。RUN-1 身份测试启动器不包含供应链，不能作为完整业务环境。

## 用户操作

1. 正常 ZITADEL 登录并选择当前企业。在“我的数据”添加自有商品、实际 JPEG/PNG 图片或 Excel 商品。Excel 模板从页面下载，先预览再确认保存；有 SKU 时须填写实际 currency、price、stock。
2. 将选定的完整批次加入“我的供应链”。该操作固定全部 membership，不受显示页限制；之后原批次移动不会改写已建立的供应链批次。
3. “待适配”选择批次、启用官方规则并选择已连接且服务有效的 SHEIN 美国站店铺。店铺的官方应用决定全托管、半托管或自运营类型，浏览器不能覆盖。
4. “待补全”读取实际官方字段，补齐类目、属性、价格、库存、尺寸及图片，并明确批准当前商品版本的图片。要求库存证明时，每个 SKC 最多一个公开 HTTPS JPEG/PNG/PDF 文件，类型匹配实际内容，最多3MiB。服务器在保存和发送前重新核实。
5. “已适配”可确认实际上传，也可启用已激活的标题智能体模板。执行前展示实际每件和整批预算上限。未配置或未授权会显示不可用原因。完整图片智能体的可选接线与人工选择见 [图片运行说明](product-image-agent-v1.md)；普通文件上传不要求启用 ImageAgent。
6. “待审核”展示原文、建议、证据和未解决问题。有 `listingkit.admin.write` 的当前企业管理员可接受、编辑或拒绝。本页所选最多20件可先预览后批量批准，失败或 UNKNOWN 即停止。批准与 Apply 保持两个明确动作；之后分别确认应用，使用已应用标题补全并保存新目标资料，重新核对当前版本图片批准。
7. “已上传”展示确认的原 SPU/SKC/SKU 回执。操作进度可刷新、确认原执行已启动，或取消尚未开始的商品。
8. 上传结果待核实时保留原 record/attempt，通过“查看待核实结果”输入实际 SPU，只读核实原上传。核实成功后，操作进度保留原 UNKNOWN 历史并另行显示“原执行 UNKNOWN，已核实成功”和原 SPU；刷新后从原回执读取，不再次发送。未找到、未审核、缺字段或不匹配均保留 UNKNOWN，不重新发送。图片 UNKNOWN 没有官方正向查询合同，不开放人工改成功或自动重试。

阶段数量、来源筛选与标题/SKU/批次搜索来自完整批次查询。阶段导航保留批次和店铺，所有 API 仍重新授权当前 actor/member/企业。Amazon/Walmart 不属于本次第一条实际上传开放范围。

新增、导入及批次修改在发送前保留原身份、企业、操作键和输入，重开页面后先核实原操作；未找到时只允许原键重试。浏览器无法保存原请求时不发送。后台确认撤权后停止本次批次：未开始项记为拒绝，执行中项保留为待核实，已有成功或 UNKNOWN 回执不改写；恢复权限不自动重跑。授权服务暂时故障不视为撤权。

## 运行配置

使用已获准的本地空业务环境，提供当前身份、各独立 DB owner、Temporal、正式 SHEIN 应用和存储配置。缺少配置时不能宣称可完成真实上传。本文不授权共享或生产数据操作，不自动创建环境，不包含密钥。

完整私有 JSON manifest 使用 `internal/app/runtime/currentapplication/config.go` 的既有格式，至少满足：

- `schemaVersion:1`、loopback `listen`、真实 `identity`（含当前成员授权的 `tenantDirectoryToken`）、`sourceAccountDatabase` 和当前 `commercialOwnerDatabase`。
- `productCollections:true`，`productAcquisitionDatabase` 使用 `source_acquisition_runtime` 的独立 Product DB。
- `supplyChain.assetDatabase` 使用独立 Asset DB 的 `supply_asset_runtime`，`temporalAddress` 为 literal loopback 地址，设置实际 `temporalNamespace`。同一 owner 的 Agent/Review/Asset 配置必须指向同一事实源；每个 runtime pool 最多8连接。
- 启用 `storeCenter`，使用独立 Store DB 的 `store_center_runtime`，在 `officialApplications` 配置 `self_operated`、`semi_managed`、`fully_managed` 三种实际注册应用。每项包含真实 appId、版本、API origin、正式 callback、独立私有 `appSecretFile`、`credentialKeyFile` 和 key ID。应用类型来自注册配置，不能通过站点 `store_type` 推断。
- 实际图片文件上传可选 `sourceMedia`：`enabled:true, provider:"s3", publicBase:"实际公开HTTPS地址"`，完整 `s3` 包含 bucket、region、私有 accessKeyID/secretAccessKey 和实际 endpoint（如适用）。AWS 使用 `artifactMode:"aws"`；COS 使用 `artifactMode:"cos"`、明确 endpoint 与 `cosImmutableNonVersionedBucketPolicy:true`，并实际配置不可覆盖策略。固定 key 的不可变写入与实际内容读回必须可用。

`sourceMedia` 独立消费，没有 ImageAgent fallback。未配置时文件上传保持不可用，已有 HTTPS 图片来源、商品和 Excel 入口可以使用。上传会生成公开商品图片链接，界面先提示用户；凭据和私有 owner 留在服务器。所有私有文件遵循当前 ACL 校验，不提交仓库。

可选标题智能体复用 `productAgent`、Organization credential/template owner、真实 model tariff 和账户计量。需要 enabled、独立 Agent DB、同一个 Product DB 的 Review pool、同一个 Asset DB、企业 allowlist、合法 `textPolicies`/调用上限、已激活模板和有效资源/积分。配置准入与实际付费调用分开，默认不开付费调用。未配置时基础适配和上传仍可使用。

## 初始化与正常启动

仅对已获准、明确命名的全新空 Product DB，预置 runtime role 后使用现有工具。私有初始化 JSON 包含 `schemaVersion:1, collections:true, supplyChain:true` 和维护 owner 的 loopback database 配置。维护凭据只用于显式初始化，不供服务使用。

```powershell
# 目录：当前供应链 worktree 根目录。替换成已准备的私有文件。
go run ./cmd/product-acquisition-init -config C:\private\product-init.json -confirm-empty-database product_supply
# 独立 Asset DB，预置 supply_asset_runtime；该开关仅首次空库安装使用。
go run ./cmd/supply-asset-init -dsn-file C:\private\asset-owner-dsn.txt -confirm-database product_assets -install-empty-schema
# Store 使用当前维护 owner 的私有配置。
go run ./cmd/store-center-schema-init -config C:\private\store-owner.json
```

Source Account、Commercial 和可选 Agent 按各自当前 owner 的既有初始化路径准备。服务启动只验证已有 schema/grant，不迁移旧数据；不要将上述命令用于未授权环境或非空旧数据。

```powershell
# 独立终端保持后端运行，manifest 包含全部实际配置。
go run ./cmd/current-application -config C:\private\supply-current-application.json
```

Console 配置正常 Auth.js/ZITADEL 登录（包括私有 AUTH_SECRET 和既有 client/callback 设置），为实际后端端口设置：

```powershell
Set-Location web/listingkit-ui
$env:LISTINGKIT_PRODUCT_COLLECTIONS_ENABLED='true'
$env:LISTINGKIT_SUPPLY_CHAIN_ENABLED='true'
$env:LISTINGKIT_SERVICE_API_BASE='http://127.0.0.1:8085/api/v1'
$env:LISTINGKIT_PUBLIC_BASE_URL='http://127.0.0.1:3000'
npm.cmd run build
npm.cmd run start -- --hostname 127.0.0.1 --port 3000
```

启动后正常登录，访问 `http://127.0.0.1:3000/workbench/data/mine` 和 `http://127.0.0.1:3000/workbench/supply/mine`。地址以启动成功、配置一致为前提；本次未启动包含正式 SHEIN、存储和付费 provider 配置的长期实例。

停止 Go/Next 使用 Ctrl+C 或 current-application 既有私有 `-shutdown-file`。重启复用同一 manifest、DB 和 Temporal namespace，不再次初始化。来源、批次、目标、Review 和执行回执保存在 Product DB，批准图片在 Asset DB，店铺与加密连接在 Store DB，运行进度使用同一 Product owner 和 Temporal。停止进程与销毁数据分开。

## 证据与限制

开发验证涵盖真实 PostgreSQL 完整 membership、目标 pin、回滚和 UNKNOWN 原操作恢复，实际 SDK 签名/不可覆盖对象读回，三类应用的官方 DTO 校验，现有 Agent owner 的受控 provider 执行与 Review.Apply，以及 BFF/前端交互。布局检查使用隔离合成只读数据，没有实际发布或付费请求，检查状态已清理。

真实 SHEIN 上传、真实存储配置、付费 Agent 和用户试用均为 NOT_RUN。自检、CI 与代码评审不等于产品验收。最终候选、CI、独立检查及本批次授权记录在 PR #606；合并、部署、Issue 关闭和真实外部操作分别按用户授权执行。
