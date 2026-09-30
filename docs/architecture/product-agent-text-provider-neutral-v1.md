# Product Agent 标题文本供应商可替换合同（候选）

> Status: **DRAFT / NOT IMPLEMENTATION_READY**。Design Basis: **Independent Architecture**。
> Product decision: 2026-09-30，用户要求使用 OpenAI 兼容接口，商品标题执行不锁定单一供应商。
> Execution Issue: [#573](https://github.com/qq550723504/task-processor/issues/573)。
> Baseline: `main @ 3098ada1393a3d49c3796f1e64701d749ed20c9e`。

## 1. 用户结果、范围与权威依据

当前企业在商品标题智能体已启用、已保存商品可读取时，成员能确认模板、商品、目标平台和知识选择，执行标题建议并进入现有 Human Review/显式 Apply。部署者可以为不同企业配置不同的已核准 OpenAI Chat Completions 兼容路由，也可更换某企业的路由，无需修改标题 Agent 的业务代码。路由切换后的新调用必须重新报价；已经 dispatch 或结果未知的调用不能切换供应商重发。

本增量只处理标题文本模型的供应商锁定及其调用前门禁。首个隔离试用仍可选 GRSAI，但它不是代码中的唯一允许供应商。每条实际启用的路由需要自己的凭据、配置版本、模型用量与上界依据、价格和产品点数费率。缺任一项时显示执行不可用，不形成成功状态或付费调用。

产品依据为本次用户决定、[#555](https://github.com/qq550723504/task-processor/issues/555)、[#570](https://github.com/qq550723504/task-processor/issues/570) 与 [Agent D 已批准合同](organization-agent-configuration-v1.md)。涉及入口与交互继续以 [当前 Figma/IA Authority](../product/final-ui-ia-authority.md) 为准；本增量不改页面命名、供应商选择 UI 或确认流程。继承 [Product Agent 运行合同](product-agent-runtime-contract.md) 的 Product/Knowledge/AI/Review owner、唯一 dispatch、UNKNOWN 和人工 Apply 规则。本次用户决定替代其中 §8.4 “首片限定 GRSAI `gemini-2.5-flash`”及 §8.5 “仅 GRSAI route”这两处供应商约束；其余安全和运行约束不因接口兼容而降低。

本次不建设模型目录、自动故障切换、企业自助 BYOK/任意 endpoint、第二模型账本、供应商恢复平台或新的 Agent Runtime。不为旧数据做迁移、双读/双写或兼容适配。`Legacy decision: RETIRE` 硬编码的单供应商准入判断；`EXTRACT` 现有单次传输、账本及失败行为到当前 owner。

## 2. 现状与根因

| 现有路径 | 可复用部分 | 必须调整的锁定点 |
| --- | --- | --- |
| `internal/integration/openai` Manager、组织凭据解析器与 `sashabaranov/go-openai` | OpenAI Chat Completions 单次请求、精确配置版本、发送前重查、非流式 usage 存在性、请求/响应大小及 deadline | `APIStyle` 同时承担协议与供应商身份；泛用 `openai-compatible` 被记为 `openai`，不能准确标识真实供应商。 |
| `internal/integration/agent/grsaitext` | quote→claim→额度预留→一次发送→实际 usage 结算，UNKNOWN 不重发 | `prepare` 只接受 `ProviderID=grsai`、`ModelID=gemini-2.5-flash`；输入/输出窗口常量和请求输出上限固定在当前模型。实现时将这个单一模型适配包改为供应商中性名称，不并行保留第二条执行路径。 |
| `internal/aicapability`、Commercial owner | Invocation 事实和组织/成员点数预留及结算 | 需要把准确供应商、模型、配置与价格/费率版本写入既有事实，不能以接口名冒充供应商。 |

OpenAI 官方 [Chat Completions API](https://developers.openai.com/api/reference/cli/resources/chat) 定义请求/响应协议；其 [Token 说明](https://developers.openai.com/api/docs/guides/token-counting) 解释输出上限参数涵盖生成 Token。第三方声称兼容只说明候选 wire 形状，不能替该供应商证明限额、usage 完整性、实际计费或模型窗口。现有 GRSAI 模型列表可作为该 route 的价格候选依据，不是其他 route 的价格。

## 3. Owner、合同与接线

```text
operator-controlled organization credential + exact text admission policy
    -> existing Manager resolves effective route and scoped credential
    -> existing Product Agent application / governed title model
    -> existing AI invocation ledger + Commercial reservation
    -> existing Chat Completions transport (one send)
    -> existing Product Review pending proposal -> human Apply
```

- **AI credential owner**：在现有组织凭据中明确记录供应商身份 `ProviderID`，与协议 `APIStyle=openai-compatible` 分离。标题执行要求非空、规范化，且与部署策略批准的精确路由相同；不能由浏览器、模型或 `BaseURL` 主机名猜测。不另建供应商注册表。当前静态/非标题消费者可沿用其原合同。本增量不增加第二凭据 owner。新增列的空安装 schema、写入和读取归现有 credential store；标题准入不接受缺少身份的记录，不做旧记录回填。
- **Manager/transport owner**：`EffectiveClientRoute` 继续绑定供应商 ID、模型 ID、凭据引用和配置版本。配置版本含供应商 ID、endpoint、协议、模型及凭据版本，不含明文密钥。解析与发送前重查同一个精确 route；不同路由不能默默回退。复用 SDK，严格非流式、一次请求、禁重定向/隐式重试、保留现有大小和时间边界。对一个候选 route，先证实所用输出上限字段会被该服务接受并强制执行、成功响应含完整且一致的 `prompt_tokens`/`completion_tokens`/`total_tokens`；证据不足则不准入。
- **标题模型 owner**：部署策略按当前授权的组织选择至多一个 `AdmittedRoute`，连同该 route 的 `BoundEvidence`、模型输入/输出硬上界、请求输出限制及其 wire 字段、币种与版本化每百万 Token 估价、版本化产品点数费率。无该组织策略则拒绝。各值与精确 route 一同冻结进 quote 引用；`Quote`、`Decide` 和发送前检查一致。数值正数、有界、计算无溢出，`request output limit <= admitted output bound`；不得从另一模型窗口或供应商价格借值。只有实际计费维度能由该策略覆盖的 route 可准入；若供应商另按缓存、推理、请求或其它维度计费，须有相应上界与估价依据，不能仅按两个 Token 数低估。当前一次执行选择一个已准入 route，不实现动态目录或随机选路。
- **AI/Commercial owner**：保留原子 dispatch claim、成员额度预留、实际 usage 结算。Invocation 记录真实供应商、模型、凭据引用、配置与定价版本、上界及结果。点数定价由产品明确批准，不能把供应商积分直接当成本产品点数。
- **Product/Knowledge/Review owner**：保持现有授权、快照、知识 opt-in、工具事实校验、候选提案与人工 Apply；供应商不拥有商品事实或审核决定。应用注入仍默认关闭，只有组织 allowlist、完整依赖和该 route 准入都具备才开放。

## 4. 状态、失败与外部副作用

供应商选择是按组织的部署配置，不是 Agent 状态机新状态。每次模型步骤在调用前以当前组织及成员身份取得该组织精确 route 和 quote；不能选取另一组织的策略。配置、价格、费率或权限变化使原 quote 失效，拒绝此次 dispatch。多步运行可在下一步重新取得当前 route，但同一 invocation 一经 claim，不能通过换供应商、换模型或新请求键重试。

只有额度预留成功且发送前授权与 route 再核对成功，才允许一次网络发送。确认未发送时按现有 owner 终结并释放预留；发送后超时、响应丢失、缺 usage、usage 超界或终态持久化失败均保持 UNKNOWN/待核实，不标零费用、不再次发送。第三方返回的错误正文不进日志/UI。切换部署配置不改写历史 Invocation、Review 或商品数据；不存在跨库事务或新增恢复 worker。

## 5. 准入与验证

独立 Architecture Review 需先核对本增量的供应商身份、精确配置版本、预算上界、usage、UNKNOWN 与当前产品决定；明确 **IMPLEMENTATION_READY** 前，#573 不修改正式标题执行代码。评审只覆盖本次改变的边界，至多两轮正常审查；其余已批准 Agent D 合同继续复用。

实现自检应覆盖两个企业各自使用不同供应商身份/模型的受控兼容服务，验证 quote/dispatch/ledger 的真实路由归属及跨企业隔离；配置或价格切换在 claim 前失败；usage 缺失、超过上界及发送后未知保持未决；组织凭据缺失和权限撤销不发送；原 GRSAI 路径不回归。仅 fixture 的第二供应商验证可替换性，不宣称其真实服务可用。实际 GRSAI 或另一供应商的付费试用还需该 route 的可信上界与计量证据、产品点数费率、调用预算和单独授权。用户试用标题→Review→Apply 的结果与实现自检/CI 分开记录。

当前状态：设计候选；没有新增生产代码、schema、付费调用或用户验收。
