# Product Agent 标题诊断试用交接

Refs #131、#132、#134、#573。入口是当前 Console 的 1688 采集详情：
`/workbench/supply/acquisition/operation/<operation_id>`。范围是从已保存的确切商品版本
读取证据、生成标题建议、有限修复、提交人工审核，再使用现有 Review 的接受/编辑/应用。
不自动修改商品，不发布平台，不包含图片生成。

## 启动前条件

使用当前 `cmd/current-application` 私有 JSON manifest 和正常 Auth.js/ZITADEL 登录配置。
这是既有运行入口的可选组合，不是新的 launcher 或验收平台。当前仓库不随代码提交
真实密钥、企业 ID、已充值额度或可用 provider 的证明。

启用 `productAgent.enabled: true` 时，manifest 必须已经配置当前
`productAcquisitionDatabase` 和 `commercialOwnerDatabase`。Product Agent 新增配置：

| 字段 | 要求 |
| --- | --- |
| `database` | Agent 运行/审计和现有 AI credential/invocation owner 的已有数据库连接 |
| `reviewDatabase` | 与 `productAcquisitionDatabase` 相同 host/port/database，使用具备当前 Review/Catalog/SRC 权限的独立连接 |
| `assetDatabase` | 当前 Asset inventory owner 的已有数据库连接 |
| `allowedOrganizationIds` | 明确获准试用的企业 ID；没有 allowlist 不运行 |
| `steps`, `modelCalls` | 正整数，分别不超过 16、8；建议试用 12、6 |
| `tokens`, `costMicros` | 明确批准的本次运行预算，不会代替或新增 Commercial 额度 |
| `runtimeSeconds` | 1–120 秒，包含整个运行；恢复不重置预算或原始截止时间 |
| `currency`, `textPolicies` | 单一运行预算币种和按企业 ID 索引的受控标题策略；缺策略的企业不可执行 |

三个连接使用与现有 `DatabaseConfig` 相同的字段：`host`、`port`、`user`、`password`、
`database`、`maxConnections`。仅允许 loopback、每池最多 8 个连接；数据库密码只放私有
manifest。应用只打开已有池，退出/构建失败时关闭；不自动建表、升级 schema 或授予权限。

数据库必须先由获授权的安装者完成全新安装：Agent schema 在
`internal/integration/persistence/agent/schema.sql`，Review schema 在
`internal/integration/persistence/product/review/schema.sql`。继续复用现有
Catalog/SRC、Asset、`ai_client_credentials`、`ai_invocations` 和 Commercial owner schema。
Agent 运行角色需要 runs SELECT/INSERT/UPDATE、tool_calls SELECT/INSERT、现有
credential SELECT、invocation SELECT/INSERT/UPDATE；Review 连接需要既有 Review UoW
以及 Catalog/SRC 读写权限；Asset 连接只需现有 inventory 读取权限。不要使用数据库 owner
或测试 fixture 超级用户作为试用角色。已存在业务数据库不执行这里的全新安装步骤。

`textPolicies[organizationId]` 字段（只有完整 route、计量证据、定价和付费授权都具备时才启用）：

- `providerID`：真实供应商身份，与兼容协议名分离；`clientName`：当前企业组织凭据名称，例如 `default`。
- `endpoint`、`apiStyle`、`admittedRoute.modelID`：部署者预定的精确 endpoint、已准入传输协议和模型；企业成员不能从浏览器修改该策略。
- `policyVersion`：`title-review-v1`；`pricingVersion`：获准估价策略的版本标识。
- `currency`：当前策略的三字符货币；`inputMicrosPerMillion` 和
  `outputMicrosPerMillion`：每百万 token 的货币微单位估价，必须显式冻结。
- `inputWindowTokens`、`outputWindowTokens`：该 route 实际强制的输入和输出硬上界；
  `maximumOutputTokens` 和 `outputLimitField`：本次请求输出上限及供应商执行的字段。兼容
  route 仅用已证明的 `max_tokens` 或 `max_completion_tokens`。Google 原生 Interactions
  route 必须用 `max_output_tokens`，且 `outputWindowTokens=maximumOutputTokens`；可低于模型
  65,536 的最大窗口，以较小的实际硬上限报价。
- `reasoningEffort`：兼容 route 可选，当前只支持空值或 `none`；Google 原生 route 必须留空，
  另填 `thinkingLevel: "low"`。两种参数均随策略进入报价引用，不能互换。
- `admittedRoute`：通过当前 Manager 的 `ResolveTextRoute` 在该组织凭据下取得的
  `ProviderID`、`ModelID`、`CredentialReference`、`ConfigurationVersion` 非敏感元数据。
  其中兼容 route 的 Manager `ProviderID` 可能只是 `openai` 协议提示；实际供应商由策略的
  `providerID` 指明。不可猜测 version，也不能把其他组织的结果照搬。
- `boundEvidence`：已复核的该供应商精确 route 的完整 Token 计量、输出上界和无额外收费维度的依据标识；Google 原生 route 须包含独立思考量如何并入输出账本的证据；
  别家模型窗口、一般兼容说明或价格页不能自行当成该 route 已被验证的证据。
- `pointPricing`：沿用 #564 冻结积分计费，显式设置获准 `priceVersion` 和正整数
  `inputPointsPerMillionTokens` / `outputPointsPerMillionTokens`。不默认费率；预留企业积分
  和成员月限额，实际按 provider 观测输入/输出结算，UNKNOWN 保留原预留。

前置报价按该组织策略的输入/输出硬上界保守预留；剩余运行预算或现有成员额度不足
便停止，事后仅结算 provider 完整报告的实际 tokens。估价用于预算，不声称是真实账单。
未知响应/用量保持既有预留，不能通过新请求编号绕过。调整配置或凭据后必须重新核对 route。
当前只交付受控接线，未提供通用模型/计费配置平台；目标环境的 route 及证明仍需交付者配置。

首条 Google 候选为官方 Gemini Interactions API：`endpoint=https://generativelanguage.googleapis.com`、
`apiStyle=google-interactions`、`providerID=google`、`model=gemini-3.8-flash`、
`outputLimitField=max_output_tokens`、`thinkingLevel=low`。请求固定为单次非流式、
`store=false`、`background=false`，不带历史或工具。完整 usage 的输入量进入原 PromptTokens，
可见输出和思考量之和进入原 CompletionTokens；任一计数缺失、工具量未明确为零、
非文本模态、额外收费维度或超界便保持 UNKNOWN。
原 `gemini-2.5-flash` 兼容候选在当前 Google 新项目返回 404，3.8 兼容探针的完成量超过
请求 `max_tokens`；不能沿用它们作为准入证据。`admittedRoute` 仍须由目标组织凭据解析，
不能手填猜测。当前仅有合成探针和实现测试，没有产品点数费率、真实业务数据处理决定或
真实调用授权；保持执行关闭，不写入虚假的 `boundEvidence`。已创建的 Google Free tier Key
仅限合成数据，不能用来传输真实商品或企业资料。

当前应用没有挂载旧 `listingkit` AI 设置。部署者先在**关闭执行**的私有 manifest 中填入
企业 allowlist 及预定策略（首次 `admittedRoute.configurationVersion` 暂缺）；另备私有凭据输入，
用独立的 `title_credential_writer` 角色连接同一 `productAgent.database`。该角色仅获既有
`ai_client_credentials` 的 SELECT/INSERT/UPDATE 和必要序列权限；应用运行角色仍只读。
输入 JSON 包含 `action: "upsert"`、`organizationId`、`clientName`、`apiKey`、`baseURL`、
`model`、`apiStyle`、`timeoutSecond` 和 `writerDatabase`（同一 host/port/database、独立用户/密码）。
私有文件应采用与 manifest 相同的受限访问权限，不把密钥放命令行或仓库。

```powershell
go run ./cmd/product-agent-credential-provision -config C:\private\current-application.json -input C:\private\title-credential.json
```

命令只输出非敏感的组织、真实供应商及当前 route 配置版本。部署者将完整 `admittedRoute`
写入策略，补齐 `boundEvidence`、价格、点数费率和预算，经单独付费授权后才启用 runtime。
轮换凭据会产生新版本，旧报价不能发送；停用时同一命令的私有输入使用 `action: "disable"`、
组织 ID、clientName 和 writerDatabase，不携带旧 API Key。未完成任一步时仍保持执行关闭。

## 正常启动和操作

```powershell
go run ./cmd/current-application -config C:\private\current-application.json
```

Console 的现有 `LISTINGKIT_API_BASE` 指向上述当前 application；
`PRODUCT_REVIEW_API_ORIGIN` 设为同一 application 的 origin（不含 path），以便既有标题
审核 BFF 访问同一 Review owner。`LISTINGKIT_PRODUCT_AGENT_ENABLED=true` 打开页面入口。
保留当前 Console 的 `LISTINGKIT_PUBLIC_BASE_URL`/身份配置。进入 `web/listingkit-ui` 后
使用正常 `pnpm dev`，生产构建则沿原 `pnpm build` / `pnpm start`。这里只给正常启动命令，
不授权部署或操作共享实例。

可选企业知识（#559）复用同一 manifest 中当前 Knowledge owner 的已有配置与专用池；
Console 设置 `LISTINGKIT_KNOWLEDGE_ENABLED=true` 仅显示选择入口，不授予读取或执行权限。
新空环境按 [Knowledge 启动说明](../../deployments/docker/account-compose/KNOWLEDGE.md)
准备完整当前 schema；已有 #557 保留实例继续使用匹配镜像，不能就地重建或追加 schema。
已打开标题诊断的用户在确认框主动勾选知识、查看每份实际可读版本后确认生成。
完整内容超限或不可读时明确拒绝，本次不启动；不自动截断或换库。
生成结果与标题审核单独显示知识出处和 exact revision 标识。人工编辑后出处仅属于原 AI
建议，停用/失权后隐藏名称与摘录。字段合同、开发证据和限制见
[Knowledge consumer 说明](../engineering/knowledge-context-consumption.md)。

1. 正常登录并选择获准企业，使用拥有当前操作权限及成员额度的身份。
2. 在 1688 采集页打开已成功发布并保存的商品详情，显式选择“素材查询平台”，再点击
   “生成标题建议”，检查确认框后“确认生成”。SHEIN/Temu/Amazon 仅选择对应已批准素材，不代表已评估平台发布规则；
   缺失平台不可开始，同一运行编号不得更改平台。刷新和续跑保留服务器已保存的平台。
3. 检查建议、来源证据、模型自报置信度、未解决项、已观测用量和调用记录。
4. 若状态是“需要补充说明”，在原运行截止时间内补充说明并继续。校验失败或预算停止
   不会显示可提交审核；最终通过才显示“提交人工审核”。
5. 打开标题审核，由具备原 Review 权限的管理员接受/编辑，再显式应用。Agent 不代替此操作。

运行编号会写入 `agent_key` URL 参数；刷新后点击“读取当前结果”。网络超时、运行中断或
未知结果不会自动重发模型。保留该 URL/编号供运维查询，不反复创建新运行。
关闭页面/停止等待不能取消已经发出的模型请求。后端未开放或配置缺失时页面明确显示不可用。

大份证据会使用明确标出省略字段的标题诊断视图，原始工具结果仍保存在运行记录中。
被省略的事实保持未知，模型不能据此判断“没有素材”或“发布就绪”；证据不足时需中断。
工具读取范围和当前 Product/Asset 事实不会因此变化。

运行控制/checkpoint 存在 `product_agent_runs`，安全工具摘要在 `product_agent_tool_calls`；
模型调用及用量继续由现有 AI invocation/Commercial ledger 保存。Product 和 Review
保持各自事实 owner。正常停止/重启保留数据库；不把 destroy 或删数据作为停止命令。

## 已验证与待执行

PR #502 的历史开发组合验证覆盖真实 PostgreSQL 的采集发布、显式平台下已批准素材经授权工具读取、Eino、GRSAI SDK 结构响应、
错误证据被拒绝并修复、90 tokens 记账、重复请求不增调用，以及原 Review 人工接受/Apply。
模型服务使用隔离响应，身份和额度为隔离 fixture；这不是已部署的用户实例。

2026-10-03 独立本地实例已用合成商品和受控本地兼容响应完成正常登录、标题建议、
Review 接受与显式 Apply；运行证据见 Issue #573。该实例没有 Google Key，也没有真实模型调用。
当前任一真实供应商调用以及 #47 样本集 Agent vs fixed
质量/风险/延迟/成本对照为 NOT_RUN。供应商可替换实现的准确代码 SHA、CI 和独立评审维护在 PR #580；
PR #502 的历史证据不作为新 HEAD 的直接验证。
只有完成目标环境配置、获得对应真实操作授权并实际验证后，才可宣称用户试用通过；
不得以开发测试替代 #132 的原对照验收，也不自动进入下一阶段。
