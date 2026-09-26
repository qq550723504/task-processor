# Product Agent 最小运行合同与接入依赖

依据：[#131](https://github.com/qq550723504/task-processor/issues/131)、
[#132](https://github.com/qq550723504/task-processor/issues/132)、
[#134](https://github.com/qq550723504/task-processor/issues/134)、
[产品战略](../product/ai-commerce-agent-platform-strategy.md)、
[目标架构](project-target-architecture.md) 与
[全新系统基线](../product/greenfield-no-legacy-migration.md)。

状态：合同候选，待独立边界检查。本文不是运行时实现、产品验收或上线批准。
用户在 2026-09-26 批准先收敛 #131 与实际接入依赖，再交付 #132。
Issue/主要 PR 记录当前进度、准确代码版本、检查结果与批准；本文件只维护合同。

## 1. 用户结果与阶段边界

当前组织的商品运营选择已保存商品，查看事实/来源/素材与缺失输入，获得有证据的
补全建议，校验后交给现有人工审核。Agent 不接受/应用自己的提案，不发布商品。
完整路径为 `load -> diagnose -> choose tools -> propose -> validate -> repair <= 2 -> human review`。

#131 先交付框架无关合同及 fake model/tool 的可执行验证；不是一个独立用户产品。
这些工作与 #132 保持一个主要开发分支和 PR，在合同检查后连续实现。当前阶段不
为合同先行单独申请合并，不把文档完成算作 #131 的可执行验收通过。

首个真实审核消费者仅支持标题：接入时先让标题走现有 Product Review；其它缺失项
保留为 unresolved。这是完整 PoC 的一个实施步骤，不撤销 #132 的原验收，也不把
图片、类目/属性规则或跨字段人工审核记为已交付。扩大字段须先具备对应 owner 合同。
现有 #47 共享样本用于对照，缺少 Agent 标注的部分留 NOT_RUN，不建新的验收平台。

Must：真实调用上下文、fresh 授权、精确商品版本、有限执行/费用、唯一重试责任、
可信校验、提案可追溯、人工 Apply。Should：复用现有 Go runtime 和 UI。
不做：自动 Apply/publish、图片生成、平台写工具、Multi-Agent、通用工作流编辑器、
新 IAM/Registry/BusinessTask 平台、旧数据映射、部署或真实付费调用。
没有新增 Accepted Risk；来源与模型均是不可信数据，不能修改上述 Must。

## 2. 现有能力与具体缺口

| 当前能力及代码依据 | 可复用内容 | 本批实际缺口 |
| --- | --- | --- |
| [Commerce Tool](../../internal/commercetool/invocation_contracts.go) / [Registry](../../internal/commercetool/registry.go) | AgentDefinition、精确 ToolRef、allowlist、审计、trace、错误分类 | 真实 AgentRun/step 调用者与注入；不重建 Registry |
| [canonicalinspect](../../internal/product/catalog/tools/canonicalinspect/definition.go)、[sourceevidenceinspect](../../internal/product/sourcing/tools/sourceevidenceinspect/definition.go)、[assetinspect](../../internal/product/asset/tools/assetinspect/definition.go)、[readinessinspect](../../internal/listing/readiness/tools/readinessinspect/definition.go) | 精确版本下事实、来源、安全素材投影与输入诊断；已有 fresh org 组合 | 没有生产 Agent 调用方；source-evidence 不返回原始文本/任意 URL；不能当模型已获得完整来源正文 |
| [AI Capability](../../internal/aicapability/invocation.go) / [路由](../../internal/aicapability/routing.go) | 当前组织策略、路由、调用记录与 usage/cost unknown 语义 | 文本决策/提案调用尚无经本批确认的受治理执行入口；ImageAgent Review 证据不能代替文本链 |
| [ProductEnrichmentAdapter](../../internal/integration/openai/product_enrichment_adapter.go) | 有界 prompt/严格候选 JSON，调用窄 TextInvoker | TextInvoker 仅返回字符串/错误，没有单次调用的账本引用、可信 usage/cost 与 dispatch 状态；不能直接充当 Agent 模型门禁 |
| [Enrichment Proposer](../../internal/product/enrichment/proposer.go) | 当前 Candidate、字段/证据/策略校验 | Propose 将生成和校验放在同一入口；需要抽取现有纯校验供修复后重验，不能为验证而再调模型 |
| [Readiness Executor](../../internal/listing/readiness/tools/readinessinspect/executor.go) | 已保存 Product 版本与 ApprovedAsset 的输入检查 | 不接收 proposed patch；不能用原版本 ready 证明补丁有效，且 marketplace rules 仍 not_evaluated |
| [Product Review](../../internal/product/review/service.go)、[现有 UI](../engineering/product-title-review-ui.md) | 标题 pending/accept/edit/reject、精确版本、显式 Apply、原子 Catalog 发布/receipt | Create 会调用自己的 Proposer，尚无接收既有 Agent 提案的入口；默认 currentapplication 也未挂载该独立应用 |
| [采集入口](../engineering/src2b-public-acquisition.md) / [当前 ImageAgent 绑定](../../internal/app/httpapi/imageagent_acquisition_catalog.go) | 真实 acquisition operation 与 Catalog 发布回执绑定，可作为首个 UI 消费场景 | 新诊断操作必须读取授权后的已发布 operation，不接入旧 ListingTask 或伪造 BusinessTask |

这些是接线前必须补齐的接口，不是为理论场景扩建通用平台的依据。

## 3. Owner 与调用合同

### 3.1 运行输入

应用在 verified request/Effective Organization 上构造 RunRequest：

- 固定的 `AgentDefinition{ID, Version, AllowedTools}`，复用 Commerce Tool 定义；
- `RunID`、trace、调用方真实 context reference（kind、ID），由应用生成/解析；
- 精确 `ProductKey + CatalogVersion + CatalogPublicationID`；版本是非零十进制字符串；
- 当前策略/prompt/工具版本、hard limits 与 feature flag/组织 allowlist；
- 来源/素材仅通过工具拿到授权投影，模型不能提交完整快照作为事实输入。

首个可接入场景是既有 acquisition operation：应用从 owner 读取其已发布回执，
证明它属于当前组织/允许的操作者，并取得上述精确绑定。没有回执则拒绝。
`CallMetadata.BusinessTaskID` 保留现有传输字段名，填这个真实 context ID，语义明确
为 correlation，不将其解释成新建 BusinessTask 或授权凭据；与当前组织 ImageAgent
使用 operation ID 的做法一致。通用商品入口没有合法 context 时保持未开放，
不得生成一个无 owner 的假 task ID 绕过 Registry 的必填检查。

浏览器/模型不得设置 org/user/member/roles、AgentRunID、allowlist、预算、模型路由或
审核状态。组织身份沿当前 authidentity/workbenchcontext 传递；模型 session 内的
字符串不是认证依据。每个工具调用、模型 dispatch、resume 和结果读取重新授权。
重放/缓存不绕过授权；组织、操作者或绑定变化不能继承旧运行结果。

### 3.2 执行适配层

`internal/agent` 只拥有运行合同、预算/停止语义和结构化结果，不导入 provider SDK、
GORM、marketplace client、retired Task/ListingKit owner。框架适配放在 Integration，
应用只做组装。具体 Product 规则仍在 enrichment/readiness/review 内。

只开放以下接口职责；具体 Go DTO 在实现时依据本合同确定：

| 接口职责 | 唯一责任方 | 必须返回/保证 |
| --- | --- | --- |
| Run / Resume / Interrupt | Agent runtime + 一个框架适配 | 运行状态、step、剩余预算、stop reason；不拥有业务生命周期 |
| 调用已绑定工具 | 既有 BoundToolSet / Invoker | 原 Result.Output、AIInvocationID、AuditStatus 与错误均保留 |
| 有界模型决策/文本生成 | AI Capability 治理的 Integration adapter | 冻结的 invocation ID、配置/prompt 引用、已知/未知 usage/cost、dispatch outcome；Agent 不选 provider、不处理 key |
| 校验候选 | Product enrichment 的纯校验 | 精确 base/policy/candidate hash、字段证据、拒绝/未解决项；不调模型、不写 Catalog |
| 接收审核提案 | Product Review owner | 幂等 pending proposal 引用；复用原 owner/事务/校验，不重跑生成、不直接 Apply |

模型选择的工具名/参数只是建议。适配层在 Registry 前核对 run 的精确资源绑定、
allowlist 和剩余预算，Registry/领域入口仍执行各自 fresh 授权与 schema 校验。
固定字段不能由模型改成另一个商品、版本或平台；任何建议修改都拒绝。
任何 `write/publish` 工具不进入首版；propose 的外部模型调用仍受费用与未知结果门禁，
“无业务写入”不等于“没有外部成本”。

### 3.3 结构化结果

返回 `RunID/context ref/base binding/agent version`、ProposedProductPatch、字段证据引用、
字段 confidence、unresolved issues、校验报告、调用引用与 stop reason。
confidence 必须标出来源/是否已知；不把 enrichment.Quality 或任意固定数值当模型置信度。
结果默认 `HumanReviewRequired=true`，不提供 Agent 可设的 approved/applied/publish-ready。
校验报告绑定 exact base + candidate hash + rule/policy version；candidate 修改使原校验失效。
禁止 raw provider body、secret、任意外链、隐藏推理内容进入 trace/UI。

## 4. 状态、预算与恢复

### 4.1 状态和前置条件

| 前态/事件 | 后态 | 持久效果和责任 |
| --- | --- | --- |
| 未运行，身份/绑定/策略/预算完整 | running | 运行调用方冻结 request fingerprint；不是业务 task 创建 |
| running，工具/模型 step 完成且结果可用 | running | 保存有限 step 结果/引用、消耗与 next step；不复制权威商品事实 |
| running，请求人工补充信息 | interrupted | 仅在完整 checkpoint 已确认保存且无未决 dispatch 时可返回可恢复状态 |
| interrupted，有效 exact revision 的显式 resume | running | fresh 授权、原绑定/版本、原剩余预算和原 deadline；同一 revision 至多一个执行者 |
| running，校验完成或 repair 达两次 | human_review_required | 保存有效候选/诊断；校验失败保持不可应用，不因停止循环变成成功 |
| running，取消/耗尽/依赖失败/结果未知 | stopped | 保留明确 stop reason 和已发生调用引用，不自动重新运行模型 |
| human_review_required/stopped | 终态 | Agent 不自动恢复；人工审核由 Product Review 独立接管 |

“达到人工审核”是 Agent 推理终态；等待 accept/edit/Apply 不是运行中的 Agent step。
人工补充信息的 interrupted 与业务提案审批不同，不建立另一套审批状态机。
目录版本/策略/定义变化使 checkpoint 失效，返回冲突；不自动换成 latest/rebase。

### 4.2 Hard limits

steps、model calls、tokens、elapsed runtime、estimated cost 都是服务端正数上限；
repair 上限为两次，执行串行，不加并行 scheduler。模型不能修改限制。
每次外部调用前预留该次最坏 tokens/cost，并计入所有决策、生成和 repair 调用。
不能获得可信上界、币种不符、用量/费用未知时停止新增付费 dispatch；不把 unknown 当 0。
商用额度/扣费仍由现有 Commercial/AI Capability owner 负责，Agent 只限制自己的运行。

模型返回后只用受治理 adapter 的 observed usage 结算；不能用模型 JSON 自报 tokens。
如果实际消耗违反上界，记录异常并停止；不能声称事后检查能撤销已发生的费用。
文本 adapter 必须证明输入计数和 provider 最大输出约束才能开放；不能借用图片计数、
Review 的 token ceiling 或新造价格换算。数值随实际模型策略配置，不在合同中发明价格。

每个 step 的 deadline 是运行绝对 deadline、工具/模型 timeout 与调用方 deadline 的最小值。
人工等待不延长原运行 deadline，耗尽后只能由用户另行发起新运行。
请求取消向下传播，返回后再次检查；不允许另起失联 goroutine 绕过 deadline。
输入/单次输出保留现有工具限额；run 聚合 transcript/checkpoint 上限为 2 MiB，单模型
输出沿现有文本 adapter 的 64 KiB，超限即停止，不静默截断关键证据或清零计数。
CPU/存储边界属实现测试；fake tests 使用低限额，不引入真实模型费用。

### 4.3 幂等、失败与唯一恢复责任

运行身份为 `(org, actor, context kind/ID, idempotency key)`，fingerprint 包含精确商品/来源
绑定、agent/tool/policy/prompt 版本及 limits。同键不同 fingerprint 冲突；同键同请求
返回既有状态，不能再次开始执行。call ID 由 run + step 固定生成，与 invocation ID 关联。
repair 是得到确定性拒绝后的新 step，不是重试丢失响应的同一次模型调用。

| 失败边界 | 行为 |
| --- | --- |
| 工具读取失败/取消 | 首版无自动重试；报告 tool_error/cancelled；不进入假成功或 fallback 到 legacy |
| Tool AuditStatus=record_failed | 保留状态与有用诊断，停止后续步骤并标记 audit_unavailable；不重调工具补审计 |
| 模型 dispatch 前记录/预算预留失败 | 不 dispatch，停止 dependency_unavailable；不存在待推测的外部效果 |
| 模型已 dispatch，响应丢失/超时/结果记录失败 | 标记 model_outcome_unknown，保留 invocation ID，不重试、不换 provider、不释放未知消耗为零 |
| checkpoint 保存失败/响应丢失 | 不暴露可 resume 的成功；读取唯一记录确认状态；不得以缺记录推断模型未调用 |
| 并发 Start/Resume | 唯一 run 身份 + expected checkpoint revision/CAS；失败者只读取，不能 dispatch |
| 重启/取消后发现 in-flight 调用 | 由 AI Capability 的既有 invocation 事实判定；未决则停止，不能从旧 checkpoint 重发 |
| 输出交付/提案保存响应丢失 | 用原 run/result hash/review key 查询同一 owner；不重新生成或 Apply |

Temporal/调用方拥有 run 级调度，首版不得设置会重放完整有费用循环的自动 retry。
框架的 failover/重试必须关闭或由 AI Capability 的单一策略控制；不得叠加 SDK 默认重试。
当前 ImageAgent 的 Review 专用 replay envelope 不能直接用于文本模型结果恢复。
在文本调用的可恢复结果合同/记录尚未完成前，只运行 fake 模型，不开放真实 dispatch。

持久化：运行调用方与框架 checkpoint adapter 是同一运行状态 owner，使用同一事务/CAS
保存运行控制字段与 checkpoint；不以独立 KV + 第二 run 表做双写同步。框架 blob 只是
内部推理状态，不能被浏览器提交/替换，不能持有 token/key 或当作当前授权缓存。
AI invocation 和 Product Review 的持久事务保持各自 owner；上述未知结果停止规则负责
跨边界，不为本批新建 Saga/outbox/reconciler。自动恢复任务不在本次范围。

stop reason 至少覆盖：budget_steps/model_calls/tokens/runtime/cost、repair_limit、
cancelled、unauthorized、binding_conflict、invalid_model_output、tool_error、
audit_unavailable、dependency_unavailable、model_outcome_unknown、checkpoint_conflict。
不透传原始 SQL/provider 错误；面向 UI 区分“未执行”“已停止”“结果待核实”。

## 5. 复用框架，不自建通用引擎

沿原设计的 Go 优先候选，首个适配验证使用 Eino；SDK DTO 留在 Integration。
[Eino ADK](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/) 已提供 Agent 抽象；
[Runner](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/agent_extension/) 提供
运行及 checkpoint/interrupt/resume 接口。它的 checkpoint store 本身不证明本项目的
组织授权、并发 CAS、费用或 provider 幂等，上述门禁仍是应用/owner 合同。
这是基于现有 Go 单体和所需能力的适配候选判断，不是已完成兼容验证。

依赖引入前固定具体版本并检查 Go 版本/许可证/当前 API；用 #131 要求的 fake model/tool
直接验证所需能力，不搭建额外评测服务。若无法满足合同，报告具体缺口后再比较原候选
[Google ADK Go](https://adk.dev/get-started/go/)；不同时安装多个框架，也不自写通用图引擎。
领域合同不出现 Eino/LangGraph/ADK DTO，不增 Python/Node Agent 服务。

## 6. 实施顺序与解锁条件

1. 本合同及真实缺口做一次独立边界检查。只对新状态/恢复、身份和有费用调用边界审查。
2. #131 在同一主要 PR 落最小合同/单框架适配，fake model/tool 验证完整循环与全部预算停止。
   此时不开放 HTTP、数据库初始化或真实 provider；恢复/幂等使用测试替身不能宣称持久化完成。
3. #130/#134 的当前消费者增量：受治理文本调用；抽取 enrichment 纯校验；受控 propose 工具。
   在原 owner 内补齐，不把图像、平台工具全集作为标题首片的前置，也不撤销父 Issue 原目标。
4. #132 接入既有采集结果页和 Product Review，补齐当前组织 runtime 组装与持久执行合同；
   只在现有准入、隔离验证和明确真实调用授权满足后开放。用户能看到入口和不确定项，
   不能把标题建议、input readiness 或 fixture 当平台可发布/产品验收完成。

| 缺口 | 责任 / 解除条件 | 阻塞层级 |
| --- | --- | --- |
| 文本模型治理、可观测预算与未知结果防重复 | #130 / AI Capability：当前文本调用有真实 adapter、前置边界、调用结果/用量与不重放证据 | 真实模型接线；不阻塞 #131 fake 合同实现 |
| proposed patch 校验及窄 propose tool | #134 / enrichment：无模型副作用的相同规则校验与精确证据绑定 | 补全/repair 功能实现与该功能合并 |
| Agent 结果进入既有审核 | #132 消费 #36 / review：幂等接收可信内部结果、fresh 授权、校验、原 UoW；未修改部分复用现有测试 | 人工审核闭环；不阻塞只读工具 |
| 当前调用上下文与运行状态持久化 | #131/#132：真实 operation 绑定、run 唯一性、checkpoint revision、restart/unknown 证据 | HTTP/worker 开放；禁止假 task ID 或伪持久化 |
| 同样本的 Agent/fixed 对照 | #132 消费 #47：复用已有核心样本，补必要标注与实际测量 | PoC 结论/下一阶段；不是只读工具已合入证据 |

缺口是事实/实施义务，不自动授权另起多个 Issue、多个 PR 或修改相邻活跃交付。
若真实实现需要新增资金规则、provider 行为或扩大产品字段，提交具体最小决定再实施。

## 7. 验证与交付

TDD 使用现有测试工具，不另建 runner/人工验收 session 平台。

- #131：fake model + fake tools 完整走一遍读取/动态选择/提案/拒绝/两次修复/人工审核；
  每个预算维度单独耗尽，断言耗尽后零新增调用；未知用量与不同币种拒绝后续 dispatch。
- 权限/绑定：跨组织、撤权、修改 product/version、write 工具、重复 call、并发 resume、
  checkpoint/定义/策略冲突均不能继续；审计失败保留原状态，不能覆盖 Result。
- 恢复：真实接线增量再验证 start/restart/CAS 与模型已 dispatch 后丢响应；同一次付费
  调用至多由原调用 owner 判定/恢复，不以单元替身通过冒充持久执行保证。
- Product：坏字段/不存在证据/patch 变化重验失败、不重调模型的纯验证；readiness 不当
  patch validator；人工审核仍必须 accept/edit/revalidate/显式 Apply。
- 产品交接：当前入口、启动步骤、实际保存方式、可用/未开放功能；保留 #132 对照指标。

当前文档不附执行 PASS。fake 验证、真实 PG、真实 IAM/provider、UI 试用、产品验收分别
记录在 PR，缺少实际执行保留 NOT_RUN。独立 Reviewer 检查真实差异，最终用户验收仍由
用户或其指定验证者完成；作者不得因代码评审或 CI 绿色自行宣布可上线。

Legacy decision: RETIRE（设计约束，不在本文件中删除代码）。
Reusable behavior: 当前 Catalog/Source/Asset/enrichment/review 与 #133 的合格合同。
Current owner: 各原领域 owner；Agent 仅推理运行状态。
Cutover/deletion condition: 新消费者从开始即不依赖旧 Task/tenantbridge；无双协议或旧数据迁移。
