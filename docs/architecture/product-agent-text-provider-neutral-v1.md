# Product Agent 标题文本供应商可替换合同（候选）

> Status: **IMPLEMENTATION_READY**（2026-09-30 独立增量评审后）。Design Basis: **Independent Architecture**。
> Product decision: 2026-09-30，用户要求使用 OpenAI 兼容接口，商品标题执行不锁定单一供应商。
> Execution Issue: [#573](https://github.com/qq550723504/task-processor/issues/573)。
> Baseline: `main @ 3098ada1393a3d49c3796f1e64701d749ed20c9e`。

## 1. 用户结果、范围与权威依据

当前企业在商品标题智能体已启用、已保存商品可读取时，成员能确认模板、商品、目标平台和知识选择，执行标题建议并进入现有 Human Review/显式 Apply。部署者可以为不同企业配置不同的已核准 OpenAI Chat Completions 兼容路由，也可更换某企业的路由，无需修改标题 Agent 的业务代码。路由切换后的新调用必须重新报价；已经 dispatch 或结果未知的调用不能切换供应商重发。

本增量只处理标题文本模型的供应商锁定及其调用前门禁。2026-10-03 用户选择以 Google 官方接口作为首条真实候选路由；此前的 GRSAI 候选仍保留为历史研究，不作为默认。每条实际启用的路由需要自己的凭据、配置版本、模型用量与上界依据、价格和产品点数费率。缺任一项时显示执行不可用，不形成成功状态或付费调用。

产品依据为本次用户决定、[#555](https://github.com/qq550723504/task-processor/issues/555)、[#570](https://github.com/qq550723504/task-processor/issues/570) 与 [Agent D 已批准合同](organization-agent-configuration-v1.md)。涉及入口与交互继续以 [当前 Figma/IA Authority](../product/final-ui-ia-authority.md) 为准；本增量不改页面命名、供应商选择 UI 或确认流程。继承 [Product Agent 运行合同](product-agent-runtime-contract.md) 的 Product/Knowledge/AI/Review owner、唯一 dispatch、UNKNOWN 和人工 Apply 规则。本次用户决定替代其中 §8.3 的“文本也使用 GRSAI”和 GRSAI 专属 endpoint 归属、§8.4 “首片限定 GRSAI `gemini-2.5-flash`”及 §8.5 “仅 GRSAI route”这几处供应商约束；§8.3 单次 SDK 调用、显式非流式、全字段 usage、大小/时间限制及安全错误规则继续适用于每条已准入兼容 route。其余安全和运行约束不因接口兼容而降低。

本次不建设模型目录、自动故障切换、企业自助 BYOK/任意 endpoint、第二模型账本、供应商恢复平台或新的 Agent Runtime。不为旧数据做迁移、双读/双写或兼容适配。`Legacy decision: RETIRE` 硬编码的单供应商准入判断；`EXTRACT` 现有单次传输、账本及失败行为到当前 owner。

## 2. 现状与根因

| 现有路径 | 可复用部分 | 必须调整的锁定点 |
| --- | --- | --- |
| `internal/integration/openai` Manager、组织凭据解析器与 `sashabaranov/go-openai` | OpenAI Chat Completions 单次请求、精确配置版本、发送前重查、非流式 usage 存在性、请求/响应大小及 deadline | `APIStyle` 同时承担协议与供应商提示；泛用 `openai-compatible` 被记为 `openai`，不能准确标识真实供应商。现有 `NewOrganizationCredentialResolver` 只禁止全局回退，`ResolveClientConfig` 仍优先读取成员凭据；标题调用须限定组织凭据行，并以服务端准入策略声明的供应商 ID 记账。当前应用没有挂载旧 `listingkit` 凭据设置，须补真实的部署侧写入路径。 |
| `internal/integration/agent/grsaitext` | quote→claim→额度预留→一次发送→实际 usage 结算，UNKNOWN 不重发 | `prepare` 只接受 `ProviderID=grsai`、`ModelID=gemini-2.5-flash`；输入/输出窗口常量和请求输出上限固定在当前模型。实现时将这个单一模型适配包改为供应商中性名称，不并行保留第二条执行路径。 |
| `internal/aicapability`、Commercial owner | Invocation 事实和组织/成员点数预留及结算 | 需要把准确供应商、模型、配置与价格/费率版本写入既有事实，不能以接口名冒充供应商。 |

OpenAI 官方 [Chat Completions API](https://developers.openai.com/api/reference/cli/resources/chat) 定义请求/响应协议；其 [Token 说明](https://developers.openai.com/api/docs/guides/token-counting) 解释输出上限参数涵盖生成 Token。第三方声称兼容只说明候选 wire 形状，不能替该供应商证明限额、usage 完整性、实际计费或模型窗口。现有 GRSAI 模型列表可作为该 route 的价格候选依据，不是其他 route 的价格。

### 2.1 Eino 扩展适配器复用判断

[eino-ext](https://github.com/cloudwego/eino-ext) 已提供 OpenAI、Claude、Gemini 等 ChatModel 适配器；本仓库当前只引入 `github.com/cloudwego/eino` 核心包，使用 `compose` 运行 Agent 图，文本传输已有 `sashabaranov/go-openai` SDK。当前产品决定限定 **OpenAI Chat Completions 兼容协议**，多个供应商可通过同一成熟 SDK 与不同的已准入 endpoint 接入，不需要自行实现各厂商 HTTP 客户端。实现前先核对 [eino-ext OpenAI 适配器](https://github.com/cloudwego/eino-ext/tree/main/components/model/openai) 与现有传输 seam；只有它能保持当前按组织解析/队列、原始 usage 字段存在性、请求大小/deadline、明确未发送与发送后 UNKNOWN、禁重试行为时才替换现有 SDK 调用。不能为接入 Eino 接口而丢掉这些已通过验证的门禁。

该判断有具体技术依据：[eino-ext 的 OpenAI 响应转换](https://github.com/cloudwego/eino-ext/blob/main/libs/acl/openai/chat_model.go) 将 SDK 的 `resp.Usage` 取地址后填入 Eino `ResponseMeta.Usage`；只看转换后的整数无法区分原始 JSON 缺字段与观察到零。其适配器提供自定义 `HTTPClient` 和原始响应 modifier，可在受控 seam 校验；正式选用时须以当前版本和隔离测试证实。未来若产品决定接入非 OpenAI 兼容的原生协议，优先复用对应 eino-ext 适配器，按该 route 重新完成同一准入；本增量不提前实现原生协议。

### 2.2 Google 官方直连首条候选（Reuse Existing Architecture）

[Google 官方兼容接口](https://ai.google.dev/gemini-api/docs/openai) 提供 `https://generativelanguage.googleapis.com/v1beta/openai/`，支持以 OpenAI Chat Completions 客户端直连。`gemini-2.5-flash` 的 [官方模型页](https://ai.google.dev/gemini-api/docs/models/gemini-2.5-flash) 列出输入 1,048,576、输出 65,536 token 限额；兼容接口文档还说明该模型可用 `reasoning_effort=none` 关闭思考。本路线复用现有兼容传输，不引入原生 Gemini 协议或第二个请求客户端。

标题策略可选地冻结 `ReasoningEffort`，当前只接受空值或 `none`。空值沿用既有供应商请求，`none` 由现有 SDK 写入兼容请求，并随整个策略进入 quote 引用；不同参数配置必须重新报价。Google 候选采用 `none`，因为现有账本只根据完整 `prompt_tokens` 和 `completion_tokens` 两项结算，不能把单独计费的思考 token 隐藏在 `total_tokens` 里。传输继续拒绝缺失或不一致的完整 usage，发送后的不确定结果继续 UNKNOWN。此参数不授予路由准入：Google 原生 API 的 `maxOutputTokens` 硬上界和思考计量说明不能直接证明兼容接口 `max_tokens`/`max_completion_tokens` 的映射及实际响应计量。Google 兼容层当前为 beta，须针对精确 endpoint、模型和请求参数取得实测证据；未取得时保持执行关闭。

## 3. Owner、合同与接线

```text
existing organization credential + operator-controlled text admission profiles
    -> existing Manager resolves effective route and scoped credential
    -> existing Product Agent application / governed title model
    -> existing AI invocation ledger + Commercial reservation
    -> existing Chat Completions transport (one send)
    -> existing Product Review pending proposal -> human Apply
```

- **AI credential owner 与部署写入路径**：标题 Manager 读取当前应用 `ProductAgentDB` 中既有 `ai_client_credentials` 的 `TenantID=EffectiveOrganizationID, UserID=""` 组织行，不增加字段或第二事实源。旧 `listingkit` 企业 AI 设置代码属于另一应用接线；`cmd/current-application` 没有挂载该入口，其 `CoreConfig()` 也未向旧设置注入数据库，**不得声称该 UI 可配置标题 route**。本批次复用现有 `GormCredentialResolver.SaveCredential`，补一个有界、仅部署者使用的凭据写入命令：从私有当前应用 manifest 只读取 `ProductAgentDB` 的目标 host/port/database 和 allowlist；从单独私有输入读取组织 ID、`ClientName`、API Key、BaseURL、模型、APIStyle、超时、启用位，以及一个**独立的凭据写入角色连接**。命令核对写入连接指向同一目标数据库；该角色仅有现有 `ai_client_credentials` 的 SELECT/INSERT/UPDATE 及必要序列权限，不使用或扩权 Product Agent serving 角色（其凭据权限仍为 SELECT）。只接受 manifest allowlist 内组织及部署侧预定的供应商、模型、endpoint、协议，强制 `UserID=""`，写后只回报不含密钥的组织/route 配置版本。部署者再将该版本写入完整 `AdmittedRoute` 策略并启用 runtime；首次凭据写入不依赖尚不存在的配置版本，未完成该顺序前执行保持关闭。输入文件、命令参数和日志不得暴露 Key；更新由现有表的唯一 `(tenant_id,user_id,client_name)` 行完成，并由配置版本变化使旧 quote 失效。禁用或轮换也通过同一 owner 的受控写入完成。此命令不挂载浏览器入口，不创建通用 BYOK 或任意 endpoint 服务。标题 Manager 使用专用组织行限定解析模式：即使上下文有当前 `UserID`，也只查询该组织的空 `UserID` 行，缺失或禁用即拒绝；不读取成员行，也不回退全局 Key。`NewOrganizationCredentialResolver` 现有 user-first 语义保留给其它消费者，不能直接当作此模式。凭据值不是执行许可；与部署侧准入 route 不再一致时，新调用立即拒绝。浏览器和模型不能提供本次 invocation 的 route。
- **Manager/transport owner**：`EffectiveClientRoute` 继续绑定现有协议/供应商提示、模型 ID、凭据引用和配置版本；配置版本含 endpoint、协议、模型及凭据版本，不含明文密钥。其 `ProviderID` 对 `openai-compatible` 仅是协议提示，**不能**直接当作标题 Invocation 的真实供应商身份。解析与发送前重查同一个精确 route；不同路由不能默默回退。复用 SDK，严格非流式、一次请求、禁重定向/隐式重试、保留现有大小和时间边界。对一个候选 route，先证实所用输出上限字段会被该服务接受并强制执行、成功响应含完整且一致的 `prompt_tokens`/`completion_tokens`/`total_tokens`；证据不足则不准入。
- **应用注入与标题模型 owner**：将当前单个 `ProductAgentConfig.TextPolicy` → `ProductAgentDependencies.TextPolicy` → `AgentTextModel.policy` 改为部署配置中的有界、不可变 `OrganizationID -> AgentTextPolicy` 映射，不新增数据库 owner。应用启动时校验组织 ID 唯一、与 allowlist 对应、每项策略完整、各项币种与同一应用的 `Limits.Currency`/Commercial owner 一致；缺策略的 allowlist 组织保持执行不可用。`Quote`、`Decide` 和发送前 `prepare` 每次先从 fresh identity 取得 Effective Organization，再只选该组织策略。策略声明真实 `ProviderID`，并绑定该组织 Manager 解析出的完整 `AdmittedRoute`（包括配置版本）、`BoundEvidence`、模型输入/输出硬上界、请求输出限制及其受支持的 wire 字段、币种与版本化每百万 Token 估价、版本化产品点数费率。`ProviderID` 仅由服务端策略提供，不从 APIStyle、浏览器或 endpoint 猜测；它与完整 route 及本次组织身份一起冻结进 quote 引用。数值正数、有界、计算无溢出，`request output limit <= admitted output bound`；不得从另一模型窗口或供应商价格借值。当前每组织至多一个已准入 route，不实现动态目录或随机选路。
- **AI/Commercial owner**：保留原子 dispatch claim、成员额度预留、实际 usage 结算。Invocation 的真实供应商来自已匹配的组织策略，同时记录模型、凭据引用、配置与定价版本、上界及结果。点数定价由产品明确批准，不能把供应商积分直接当成本产品点数。本期只接纳能够用完整 `prompt_tokens` 和 `completion_tokens` 两项按现有 `ModelPointTariff` 正确计算产品点数、且供应商费用上界可由这两项保守估算的 route。缓存折扣可按未折扣上界估价；按次费、额外计费维度或无法覆盖的附加费 route 不准入，留待单独产品决定与账本合同，不以推测价格开放。
- **Product/Knowledge/Review owner**：保持现有授权、快照、知识 opt-in、工具事实校验、候选提案与人工 Apply；供应商不拥有商品事实或审核决定。应用注入仍默认关闭，只有组织 allowlist、完整依赖和该 route 准入都具备才开放。
- **Agent Configuration projection**：`internal/app/httpapi/agent_configuration_module.go` 当前在组织 allowlist 且 `ResolveTextRoute` 成功后就投影 `text.generate=AVAILABLE`。改为使用与标题 `prepare` 相同的组织策略选取及完整 route 匹配准入检查；无当前 fresh 企业身份、未 allowlist、缺/无效部署策略或依赖时投影 `UNAVAILABLE`，组织凭据缺失/禁用或与策略 route 不匹配时投影 `NEEDS_CONFIGURATION`，不显示可执行，并给出安全的修复方提示。此处只预检配置，不试探实时余额或供应商健康；执行时仍重查授权、配置、额度及预算。

## 4. 状态、失败与外部副作用

供应商选择是按组织的部署配置，不是 Agent 状态机新状态。每次模型步骤在调用前以当前组织及成员身份取得该组织精确 route 和 quote；不能选取另一组织的策略。配置、价格、费率或权限变化使原 quote 失效，拒绝此次 dispatch。启动进程加载新部署策略后也只能以新策略对后续 step 重新报价，不能修改历史引用。多步运行可在下一步重新取得当前 route，但同一 invocation 一经 claim，不能通过换供应商、换模型或新请求键重试。

只有额度预留成功且发送前授权与 route 再核对成功，才允许一次网络发送。确认未发送时按现有 owner 终结并释放预留；发送后超时、响应丢失、缺 usage、usage 超界或终态持久化失败均保持 UNKNOWN/待核实，不标零费用、不再次发送。第三方返回的错误正文不进日志/UI。切换部署配置不改写历史 Invocation、Review 或商品数据；不存在跨库事务或新增恢复 worker。

## 5. 准入与验证

独立 Architecture Review 已核对本增量的供应商身份、精确配置版本、预算上界、usage、UNKNOWN 与当前产品决定。两轮完整评审发现的组织策略注入、额外计费维度、旧供应商措辞、成员凭据覆盖和能力误报均已收敛；针对修订增量的复核又指出当前应用缺凭据写入接线、部署策略状态误报及写入角色缺失，现已补入本合同。最后一次仅针对凭据写入/能力状态增量的独立复核没有可执行 finding。**本合同达到 `IMPLEMENTATION_READY`，只授权按此设计修改正式代码；每条实际供应商 route 的完整计量/上界证据、产品点数费率、调用预算和付费调用授权仍须另行具备，未具备前执行保持关闭。**其余已批准 Agent D 合同继续复用。

实现自检应覆盖部署者写入组织凭据后当前应用实际读取同一 `ProductAgentDB` 行、轮换使旧 quote 失效且不泄露 Key；两个企业各自使用不同供应商身份/模型的受控兼容服务，验证 Quote、Decide、发送前重查与 ledger 的真实路由归属及跨企业隔离；同企业成员行不能覆盖标题组织行，组织行缺失/禁用时不可落到成员行或全局配置；`text.generate` 的 `AVAILABLE`、`NEEDS_CONFIGURATION`、`UNAVAILABLE` 与可修复来源一致；配置或价格切换在 claim 前失败；生成后的完整请求 envelope 按 UTF-8 字节加 Chat framing 余量保守检查该 route 的输入上界，超限在 claim/预留前拒绝；usage 缺失、超过上界及发送后未知保持未决；不支持的按次/附加收费 route 在 claim 前拒绝；权限撤销不发送；原 GRSAI 路径不回归。仅 fixture 的第二供应商验证可替换性，不宣称其真实服务可用。实际 GRSAI 或另一供应商的付费试用还需该 route 的可信上界与计量证据、产品点数费率、调用预算和单独授权。用户试用标题→Review→Apply 的结果与实现自检/CI 分开记录。

当前状态：按合同实现的供应商可替换代码已随 PR #580 合入 main；2026-10-03 的 Google 兼容请求参数补齐仍是后续候选。没有新增 schema、付费调用或用户验收。每条实际供应商 route 仍须完成准入证据、点数费率、预算及调用授权。
