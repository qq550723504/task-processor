# Chat → BusinessTask 标题建议试用交接

Refs #576、#588。入口为当前 Console 的 `/workbench/ai/chat` 和 `/workbench/ai/tasks`。获准企业的成员从已保存的 1688 商品采集操作开始，在 Chat 中描述标题目标；模型只返回澄清或待确认方案。成员确认后创建 BusinessTask，调用现有 Product Agent 生成建议，最后仍须在现有 Review 中由授权人员接受、编辑和应用。当前不发布到平台，也不自动应用标题。

## 准备当前应用

复用 [`product-agent-trial.md`](product-agent-trial.md) 中的 Product Acquisition、Product Agent、Review、Asset、AI invocation、Commercial、正常登录和私有 manifest。`productAgent.enabled` 及对应企业 allowlist 必须已配置。`aiWorkbench.enabled: true` 额外要求：

- `aiWorkbench.database` 指向与 `productAgent.database` 相同的 PostgreSQL host、port 和 database，用户固定为 `ai_workbench_runtime`，密码只放私有 manifest，连接池不超过 8。
- `planningTextPolicies[organizationId]` 为获准企业单独设置 `RoutePolicy`，其 `Profile` 使用 `PromptVersion=ai-workbench-chat-plan-v1`、`OutputSchemaVersion=ai-workbench-plan-decision-v1`。规划和标题执行分别有自己的组织凭据、准入版本、输入/输出上限、点数费率与价格；不能借用对方的 route。
- 标题 `productAgent.textPolicies` 也须使用当前 `RoutePolicy`。两种文本操作都经 Eino 的受控组件、当前 AI invocation 与成员额度 owner 执行；实际 provider 的计量、价格和付费准入需要部署者核实并单独授权。缺任何一项时保持执行关闭。

全新安装时，先由数据库 owner 创建限制权限且没有 schema/table 所有权的 `ai_workbench_runtime` 角色，再使用私有、具备建表权限的连接执行：

```powershell
$env:AI_WORKBENCH_SCHEMA_DSN = '<private PostgreSQL DSN>'
go run ./cmd/ai-workbench-schema-init --runtime-role ai_workbench_runtime
Remove-Item Env:AI_WORKBENCH_SCHEMA_DSN
```

命令在一个事务中安装 `ai_workbench` schema 并授予 runtime 对五张表的限定 SELECT/INSERT/列级 UPDATE；该 runtime 不能 DELETE/TRUNCATE/DDL。应用启动只验证 schema，不自动安装或迁移。这里的连接与私有 manifest 不提交到仓库。运行前按现有 PostgreSQL 备份/恢复规程覆盖新增 schema；正常 stop/restart 保留 Conversation、PlanningCommand、Proposal、BusinessTask，销毁数据库不是停止操作。

首次配置时，先保持 `productAgent.enabled: false`、`aiWorkbench.enabled: false`，在获准企业的 title/planning 策略中留下空的 `AdmittedCredentialVersion` 和 `AdmittedEndpointIdentityDigest`。通过现有 `cmd/product-agent-credential-provision` 给规划 route 写独立的组织凭据：私有输入 `consumer: "planning"`，其他字段及独立 writer 角色要求与标题凭据相同；标题使用 `consumer: "title"`。该命令验证策略形状及输入 endpoint 后，只返回非敏感的 `credentialVersion` 与 `endpointIdentityDigest`；将两者填入对应策略，再启用服务。轮换或停用会改变准入状态，不会自动改用其他企业、成员或默认凭据。OpenAI 兼容与 Claude 原生组件均需为精确 route 核实计量与计价，不能由协议兼容性推断付费可用。

```powershell
go run ./cmd/current-application -config C:\private\current-application.json
```

Console 使用原 `LISTINGKIT_API_BASE` 指向当前 application，并保持现有 Auth.js/ZITADEL 配置。进入 `web/listingkit-ui` 后用正常 `pnpm dev` 或既有构建/启动命令。当前应用未启用 Workbench 或缺 schema/策略时，Chat/Task 请求明确不可用；前端入口本身不证明后端、provider 或付费额度已开放。规划与标题两条当前企业路由都就绪时才显示 Chat 规划可用；任一路由失配时保留已有会话读取，新消息不会触发付费规划发送。

## 使用路径与限制

1. 正常登录、选择获准企业，在 1688 采集详情获取已保存商品的 `operation_id`。Chat 新建会话后输入该 ID、目标平台与标题目标，可选当前可读的模板或 KnowledgeBase。Chat 仅保存本人/当前企业会话。
2. 查看模型回复；只有 `READY` 才显示由服务器当前 Product、AgentConfig、Knowledge 和模型准入事实形成的方案。核对商品、平台、模板、知识、模型与最大额度，再显式确认。新消息或归档会使旧方案不再可首次确认。
   若标题执行路由未就绪，或方案冻结的标题模型配置已不再匹配当前准入配置，确认按钮会禁用；可保留历史阅读，配置恢复后刷新，模型配置变更时重新提出方案。即使按钮可点，服务器仍在确认时重新验证当前事实。
3. 从任务中心打开确认生成的 BusinessTask。若执行在 Agent Claim 前中断，用“启动原任务”沿原请求继续；需要反馈时在原运行上继续。结果未知时不要创建新任务试图重发。
4. 候选通过校验后提交现有人工 Review；由有权人员接受/编辑并显式应用。任务状态来自 Product Agent 与 Review 的当前事实，不能把模型建议当成已应用结果。

会话和任务页面只显示当前账号/企业及当前页的真实数据；无权限的商品细节会隐藏。浏览器会话中保存待决消息和确认操作键，刷新后的显式重试使用原键及原载荷。关闭浏览器不取消已派发的模型调用。当前只支持已保存商品的标题建议；没有平台发布、多 Agent 编排或自动应用。真实 provider、实际浏览器登录路径和用户试用须在目标环境分别验证；隔离数据库与合成模型测试只是开发证据。
