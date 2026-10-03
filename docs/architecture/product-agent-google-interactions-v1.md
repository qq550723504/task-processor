# Product Agent Google Interactions 标题路由增量合同

> Status: **IMPLEMENTATION_READY**（2026-10-03 独立架构增量复核后）。Design Basis: **Independent Architecture**。
> Execution Issue: [#573](https://github.com/qq550723504/task-processor/issues/573)；候选 PR [#586](https://github.com/qq550723504/task-processor/pull/586)。
> Product decision: 2026-10-03 用户先选择 Google 官方 Gemini、再选择 `gemini-3.8-flash`，现明确允许首条 Google 路由使用官方原生 Interactions API。该决定仅覆盖受控实现候选；真实调用、产品点数费率、预算、真实数据处理和合并另行准入。

## 用户结果与范围

已获准企业的成员在当前已保存商品的标题确认页生成有依据的建议，再由人通过既有 Product Review 接受和显式 Apply。首条 Google `gemini-3.8-flash` 路由通过官方 Interactions API 获取单次文本结果；企业仍可另行使用已经准入的 OpenAI Chat Completions 兼容路由，标题 Agent 的业务代码不绑定供应商。当前 Figma/IA、Product/Asset/Review 事实、权限和页面交互沿用 [Agent D 已批准合同](organization-agent-configuration-v1.md) 与 [当前 UI/IA Authority](../product/final-ui-ia-authority.md)。

本增量只增加一个受控的原生传输协议和它的精确准入，不建设模型目录、供应商自动切换、BYOK UI、第二账本、后台任务、工具调用或供应商恢复平台。真实付费运行仍需独立的点数费率、货币估价、额度/预算和调用授权；设计或测试通过不自动开放。全新系统不做旧数据迁移。现有 [供应商可替换合同](product-agent-text-provider-neutral-v1.md) 对组织策略、唯一 dispatch、UNKNOWN 和人工审核继续生效；其中“首条 Google 候选只走兼容协议”的结论若获用户同意由本增量替代。

## 现状、根因与复用

2026-10-03 精确 Google 兼容端点的合成探针：`gemini-2.5-flash` 在新项目返回 404；`gemini-3.8-flash` 请求 `max_tokens=64`、`reasoning_effort=low`，成功响应报告 `prompt_tokens=46`、`completion_tokens=109`、`total_tokens=155`。三项一致，但观察到的计费完成量超过请求中的 64。一次观察不证明供应商错误，也不能证明兼容参数能约束思考与可见输出的合计；该路由保持关闭。探针不含真实商品，已用完获准的两次调用，不通过测试继续探测真实服务。

[Google 官方 Interactions 文档](https://ai.google.dev/gemini-api/docs/interactions-overview) 将该 API 列为新项目推荐接口，并允许 `store=false`；[思考与上限说明](https://ai.google.dev/gemini-api/docs/thinking) 明确 `max_output_tokens` 是思考与可见输出合计的硬上界，提供 `total_input_tokens`、`total_output_tokens`、`total_thought_tokens`。[3.8 Flash 模型页](https://ai.google.dev/gemini-api/docs/models/gemini-3.8-flash) 列出输入 1,048,576、输出 65,536 token 限额。复用官方开源 [Google GenAI Go SDK](https://github.com/googleapis/go-genai) 的 Interactions 客户端，不手写原生协议；现有 Eino compose 图继续担任 Agent runtime。现有 eino-ext Gemini ChatModel 使用 GenerateContent，而非本次选择的 Interactions 合同，不能直接替代这条传输；也不能绕过原始 usage 存在性和发送后 UNKNOWN。

## Owner 与调用接线

```text
fresh organization identity -> immutable OrganizationID -> AgentTextPolicy
  -> ProductAgentDB organization-only credential + exact effective route
  -> existing quote/point reservation/claim and send-before recheck
  -> one bounded Google SDK Interactions.Create (no retry)
  -> usage -> existing AI Invocation + Commercial settlement
  -> existing Agent action validator -> Product Review -> human Apply
```

- **事实与准入 owner**：继续使用 `ProductAgentDB.ai_client_credentials` 的组织行（`UserID=""`）和现有受限部署者写入命令，不新增表或列。新增唯一 `APIStyle=google-interactions` 的受控路由；只允许 Google 官方 `https://generativelanguage.googleapis.com`、`v1beta/interactions`、`gemini-3.8-flash`，不从用户/模型输入选择 endpoint。现有 Manager 的有效配置版本继续覆盖凭据行、模型、endpoint、协议和 timeout；精确 `AdmittedRoute` 与策略的真实 `ProviderID=google`、模型、`BoundEvidence` 匹配，轮换/禁用后旧 quote 失效。其他兼容 route 的 resolver 与 SDK 行为不变。当前每企业仍至多一个已准入标题 route。当前创建的 Google 项目为 Free tier；[官方价格页](https://ai.google.dev/gemini-api/docs/pricing)标明 Free tier 内容可用于改进 Google 产品。该 Key 只允许合成数据探针，不允许把真实商品、企业资料或用户内容送入 Google；真实业务使用还需用户决定适用的数据处理方案与账号层级。
- **标题模型 owner**：复用 `AgentTextModel` 的 `Quote`、`Decide`、完整 prompt hash、额度预留与同一 invocation ID。策略显式区分传输协议，Google 专属参数为 `thinking_level=low` 和 `max_output_tokens`；数值正数且不超过该模型已证明的输出窗口。可以冻结比模型窗口更低的试用策略上界：调用前对完整输入做保守字节校验，对输出由原生硬上限约束，按这两个实际强制的界限报价和预留，不能仅填写较小数字而不执行。参数、策略版本、完整组织凭据 route、输入与价格/点数版本进入 quote 引用。既有 128 KiB 输入、256 KiB 响应、64 KiB Action、120 秒运行和单次请求 deadline/并发限制不放宽。把现有 `System`、`Prompt` 两个纯文本字段映射到单次无历史、无工具、无附件的 Interactions 请求；不发送上一轮 interaction ID，不用服务端会话。
- **当前接线的局部调整**：`AgentTextPolicy.Validate` 按 `APIStyle` 校验互斥字段：兼容 route 继续使用 `max_tokens|max_completion_tokens` 与原 `reasoning_effort` 约束，原生 Google route 只使用 `max_output_tokens`、`thinking_level=low`，不能把 2.5 的 `none` 传给 3.8。`Manager.ResolveTextRouteDetails`、组织凭据 provision 命令和配置投影必须能解析这个受控新 style 的非敏感 route；仍用同一个凭据版本算法，不复制凭据表或 route owner。`Manager.CompleteText` 在原有排队、并发控制、发送前重查及 `BeforeDispatch` 之后，根据已冻结的 style 选择原兼容 SDK 或窄 `googleInteractionsOnce`；Google 响应映射为现有 `TextCompletionResult`，`completed` 才能形成 `finish_reason=stop`，其他状态即使有文本也不得形成 Action。任何兼容调用误选原生 style 或反向误选，都在发送前失败。
- **Google SDK 传输 owner**：官方 Go SDK 只用一次非流式 `Interactions.Create`；显式 `store=false`、`background=false`、`stream=false`，不启用工具或网络搜索。官方 SDK 默认会自动重试；必须对这一调用设置 `retry.Config{Strategy:"none"}`，并以受限 HTTP client 拒绝重定向、自动连接重放、超大请求/响应和非预定 host。排队后在真正发送前重查 fresh 授权与精确 route；只有已确认网络未交接时标记未发送，交接后任何错误/超时都记 UNKNOWN、禁止另一个请求键重发。不得记录 Key、provider 原始错误、思考内容或返回的完整请求体。测试替身可使用受控 loopback endpoint，但该路径不能成为真实部署的任意 URL 准入。
- **AI/Commercial owner**：只接受完整且存在的 `total_input_tokens`、`total_output_tokens`、`total_thought_tokens`、`total_tokens`，均非负，且 `input + output + thoughts == total`。由于本次不使用工具，`total_tool_use_tokens` 必须明确报告为零，`tool_use_tokens_by_modality` 必须为空；缺失计数不能当作零。若供应商返回额外无法定价的维度或不一致计数，则保持 UNKNOWN。账本沿用两项：`PromptTokens = total_input_tokens`，`CompletionTokens = total_output_tokens + total_thought_tokens`；后者是完整可计费生成量，既有 `MaximumCompletionTokens`/point tariff 对其保守预留和结算。缓存仅按未折扣输入估算，不把缓存优惠当作必得；按次/额外费用 route 不准入。`max_output_tokens` 与模型输出窗口共同限制生成量，完整响应超出任一上界为 UNKNOWN。模型返回 `incomplete` 或非成功状态且 usage 完整时记录既有 `InvocationUsageObservedFailed`、结算已发生费用、不给标题候选；usage 缺失/无效时保留 UNKNOWN。没有新增余额、支付或恢复事实 owner。
- **模态校验**：若响应提供输入、输出或缓存的模态明细，只接受文本模态且各项合计与相应总量一致；任何图片、音频、未知模态或矛盾计数保持 UNKNOWN，不按文本费率结算。
- **消费者与可见状态**：`text.generate` 配置投影与执行前准入使用同一策略和当前组织 route。缺策略/不支持协议为 `UNAVAILABLE`，组织凭据缺失或版本不匹配为 `NEEDS_CONFIGURATION`；不以 UI 可见替代预算、授权或 provider 健康。模型动作仍过严格 JSON/证据校验，仅创建待人工审核的提案，绝不自动 Apply。

## 状态、失败与安全边界

[运行合同](product-agent-runtime-contract.md) 当前 Must 为 fresh 授权、精确商品版本、有限费用、唯一重试责任、可信校验、可追溯提案和人工 Apply；没有新增 Accepted Risk。来源与模型输出始终不可信。新增原生协议不改变 Agent Run、Invocation、Review、Product 的状态机或持久化 owner，不跨数据库建立新事务；其唯一新外部副作用是一次 Google 模型调用，处于现有 dispatch claim 和费用预留之后。请求发送前失败记录明确未发送并释放预留；发送后失联或最终记录失败保留原 invocation/预留和 UNKNOWN。Google 的 interaction ID 仅作为安全的非敏感 provider 请求引用，不用于自动查询/续跑/重发；`store=false` 禁止服务端会话保存，应用不提交隐藏思考到 Review 或 UI。

Legacy decision: **EXTRACT** 现有兼容传输的大小、deadline、错误分类与原始 usage 存在性检查到可复用窄接口；**RETIRE** 对 Google 3.8 候选的兼容 `reasoning_effort=none` 假设。不复活已淘汰的 ProductEnrich task/queue/worker，也不为旧数据做双路径迁移。

## 开工门禁与验证

用户已确认原生协议产品边界；本增量的独立复核检查了新增副作用、SDK 默认重试与禁重试配置、发送后 UNKNOWN、完整思考 token 到现有两项账本的映射、组织 route 和外部数据保留，没有可执行 finding。本合同达到 `IMPLEMENTATION_READY`，允许正式代码按本设计开始；这不是实际 Google route 的执行许可。具体 TDD：本地 SDK fixture 先证实一次请求含 `store:false`、`background:false`、`stream:false`、`max_output_tokens` 和无工具/历史；500、断连和 redirect 均零重发且发送后 UNKNOWN；缺字段/不一致/超界 usage 拒绝结算，完整思考 usage 合并进既有两项账本；incomplete 有 usage 结算但无提案；轮换、撤权、跨企业与成员凭据覆盖在发送前拒绝。运行级受控 provider 测试复用当前合成标题→Review→Apply，不触碰保留试用实例。真实 Google 执行还须目标组织 route 凭据、可复核的准确模型/输出上界/用量证据、产品点数费率、货币估价、额度/预算及单独调用授权；用户实际验收另行确认。
