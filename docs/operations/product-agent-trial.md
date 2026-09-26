# Product Agent 标题诊断试用交接

Refs #131、#132、#134。入口是当前 Console 的 1688 采集详情：
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
| `textPolicy` | 下述可信配置，不接受来自请求或模型的覆盖 |

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

`textPolicy` 字段：

- `clientName`：现有 Organization AI Capability 中的文本配置名称，例如 `default`。
- `policyVersion`：`title-review-v1`；`pricingVersion`：获准估价策略的版本标识。
- `currency`：当前策略的三字符货币；`inputMicrosPerMillion` 和
  `outputMicrosPerMillion`：每百万 token 的货币微单位估价，必须显式冻结。
- `admittedRoute`：通过当前 Manager 的 `ResolveTextRoute` 在该组织/成员身份下取得的
  `ProviderID`、`ModelID`、`CredentialReference`、`ConfigurationVersion` 非敏感元数据。
  对应 `grsai` / `gemini-2.5-flash`；不可猜测 version，也不能把其他组织的结果照搬。
- `boundEvidence`：已复核的该 GRSAI route 完整计量/输入输出上界依据标识。上游 Google
  窗口说明和 GRSAI 价格页不能自行当成该 route 已被验证的证据。

前置报价保守预留整个模型窗口 1,114,112 tokens / 次；剩余运行预算或现有成员额度不足
便停止，事后仅结算 provider 完整报告的实际 tokens。估价用于预算，不声称是真实账单。
未知响应/用量保持既有预留，不能通过新请求编号绕过。调整配置或凭据后必须重新核对 route。
当前只交付受控接线，未提供通用模型/计费配置平台；目标环境的 route 及证明仍需交付者配置。

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

1. 正常登录并选择获准企业，使用拥有当前操作权限及成员额度的身份。
2. 在 1688 采集页打开已成功发布并保存的商品详情，点击“生成标题建议”。
3. 检查建议、来源证据、模型自报置信度、未解决项、已观测用量和调用记录。
4. 若状态是“需要补充说明”，在原运行截止时间内补充说明并继续。校验失败或预算停止
   不会显示可提交审核；最终通过才显示“提交人工审核”。
5. 打开标题审核，由具备原 Review 权限的管理员接受/编辑，再显式应用。Agent 不代替此操作。

运行编号会写入 `agent_key` URL 参数；刷新后点击“读取当前结果”。网络超时、运行中断或
未知结果不会自动重发模型。保留该 URL/编号供运维查询，不反复创建新运行。
关闭页面/停止等待不能取消已经发出的模型请求。后端未开放或配置缺失时页面明确显示不可用。

运行控制/checkpoint 存在 `product_agent_runs`，安全工具摘要在 `product_agent_tool_calls`；
模型调用及用量继续由现有 AI invocation/Commercial ledger 保存。Product 和 Review
保持各自事实 owner。正常停止/重启保留数据库；不把 destroy 或删数据作为停止命令。

## 已验证与待执行

开发组合验证覆盖真实 PostgreSQL 的采集发布、授权工具、Eino、GRSAI SDK 结构响应、
错误证据被拒绝并修复、90 tokens 记账、重复请求不增调用，以及原 Review 人工接受/Apply。
模型服务使用隔离响应，身份和额度为隔离 fixture；这不是已部署的用户实例。

当前真实 GRSAI 调用、浏览器完整登录后的实际使用，以及 #47 样本集 Agent vs fixed
质量/风险/延迟/成本对照为 NOT_RUN。准确代码 SHA、CI 和独立评审维护在 PR #502。
只有完成目标环境配置、获得对应真实操作授权并实际验证后，才可宣称用户试用通过；
不得以开发测试替代 #132 的原对照验收，也不自动进入下一阶段。
