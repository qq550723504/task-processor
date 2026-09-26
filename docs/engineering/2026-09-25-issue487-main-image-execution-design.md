# 1688 单图白底生成：架构与执行交接

日期：2026-09-25；单次编辑决定更新：2026-09-26。关联：[#487][issue487]；独立验收 [#473][issue473] / [#438][issue438]。

**性质：已有批准决定与高风险边界的有界汇编，供原实现者对照和补齐精确代码映射。不是新的全局架构、实现授权、IMPLEMENTATION_READY 签发或验收报告。**

原读取基线为 `main @ b47d6302b99c5229ba89825d48ea9b2823e400af`；本次文档修订基于 `ce39b52b1e458fe0657ddea91ab48ef2cde9edf8`。Issue 中的开发证据与 main 分开；本文件不宣称已审查未发布 worktree。原 Writer、分支、最终 HEAD、CI 和运行进展继续只在 Issue/PR 维护。本次修订交原 Writer 纳入现有主要 PR #498，不另开业务实现或文档合并前置；如原 Writer 已有同主题补充，按有效决定归并为唯一正文，不维护两个相互覆盖的合同。

## 1. 已批准的用户结果

用户从已有 `/workbench/supply/acquisition/operation/{operation_id}` 采集详情中，选择已入库的一张 1688 来源图片，生成一张通用白底商品主图，查看结果并显式人工批准。[D1]、[D2]

**当前路径为：原图直接白底编辑一次 → 原图与最终候选 Review 一次 → 显式人工批准。** 2026-09-26 用户批准此精简，[D7] 明确替代本片原 `Extract → RenderWhiteBackground → Review` 的两段编辑要求；不执行独立主体提取，不生成或伪造“已提取主体”中间资产。Review 不编辑图片。该决定尚需原 Writer 实现，文档更新不代表运行代码已切换。

质量不合格如实返回，不自动再编辑、重生成或回退两段流程；UNKNOWN/响应丢失也不授权换 invocation 重调。未来若新增用户显式重新生成或确需第二阶段处理，先明确具体需求，不把它做成本轮隐藏 fallback。[D7]

输入不要求选择目标平台或品类。内部策略为 `product/zz/default/general`；`zz` 只是无目标国家的哨兵，不能显示成真实国家。白底 QA 门槛为 0.70；不通过不能批准，通过仍不等于人工批准。现有 schema 必需时才沿用 `main_review=0.65` / `white_canvas_penalty=0.10`，二者不替代白底门槛。[D2]

不新增顶层 AI 工作台，不要求先经过标题审核或 SHEIN 诊断，不复活 Task-first ImageAgent、ProductEnrich/ProductImage 或旧 ListingKit 输入桥。生成成功不自动改写 Catalog，也不自动形成 ApprovedAsset。[D1]

UI 的精确 Figma 主图动作节点尚待确认，不假称现有节点已经批准新交互；这只约束 UI 定稿，不重新撤销上述已确认产品语义，也不阻止原 Writer 按已确定合同调整非视觉链路。[D6]、[D7]

## 2. 原始依据与适用范围

| 依据 | 已确认内容 | 不代表什么 |
| --- | --- | --- |
| [入口准入][S1]、[产品入口批准][D1] | acquisition receipt / exact Catalog 输入；现有 ImageAgent、结算、资产 owner | 所有代码已合入或默认挂载 |
| [白底策略批准][D2] | 单图、通用策略、QA 与人工批准 | 自动批准或平台发布许可 |
| [单次编辑批准][D7] | 原图一次白底编辑 + 一次 Review；无独立 Extract、无自动二次编辑 | 已完成实现、质量验证或费用/时延改善实测 |
| [首次派发前预留合同][D3] | 稳定 invocation、原窗口结算、失败与 UNKNOWN 分离；派发点现为唯一白底编辑前 | 图像预算等于成员 token 计量；必须保留 Extract |
| [Review 用量失败合同][D4] | 可信用量与业务失败分别记录；UNKNOWN 不误释放 | observer 被调用就证明已知用量 |
| [批准重放开发证据][D5] | completed workflow 下从不可变资产回执核实原 action | 新 action 可以重放旧批准或完整浏览器已验收 |
| [本地 HTTPS 输出决定][D6]、[后续局部复核][E2] | 受限本机生成图输出已在原分支有局部实现/复核证据 | 全局放开私网 URL 或最终产品已验收 |

仅 [D7] 明确替代本片的两段编辑要求，其余已批准边界保留。精确类型/API/Schema 若与原实现者当前合同不一致，先定位冲突和原 owner，只审受影响增量；不得仅凭本文较新日期覆盖其他明确批准内容。仍适用 [AGENTS](../../AGENTS.md)、[派工规则](issue-driven-development.md)、[架构索引](../architecture/README.md) 和当前领域合同。

## 3. Owner 与装配：代码依赖不是运行箭头

| 职责 | 唯一现有 owner / 核对入口 | 本次接缝 |
| --- | --- | --- |
| 来源输入与商品版本 | Sourcing acquisition receipt / Catalog | 服务端核对 actor、Organization、publication、ProductKey、exact version 与所选图片 |
| 当前身份 | workbenchcontext，含 EffectiveMemberID | HTTP 请求和 worker 派发前按现有合同确认当前授权；不持久化请求凭据 |
| run / slot / 执行恢复 | 当前 ImageAgent repository / Temporal v3 effect | 保持持久 run、planRevision、slot、attempt 身份与已有恢复责任；本片改为源图直接白底编辑 |
| 模型调用事实 | aicapability / GormInvocationRecorder | Prepared/Dispatched/可信用量与 terminal outcome 按当前窄合同处理，不记录未执行的提取步骤 |
| 成员 token 预留和结算 | listingsubscription / AIInvocationUsageAdapter / commercial usage persistence | 唯一白底编辑前预留；同 invocation 原窗口结算和幂等核实 |
| 人工资产批准 | 当前 Product Asset / immutable approval receipt | 只在显式批准且 exact binding 有效后提交 ApprovedAsset |
| 用户 Audit | 当前 Account Audit projection | 读取以上权威事实，不新建第二用量或资产账本 |
| HTTP / worker 生命周期 | internal/app/httpapi、internal/app/worker/imageagent、currentapplication | 显式构造依赖、挂载和启停，不在 app 中重写额度、QA 或批准规则 |

已知复用入口包括 `BuildOrganizationImageCapabilities`、`OrganizationReviewOptions`、`GormInvocationRecorder`、`AIInvocationUsageAdapter`；它们存在或在独立装配中使用，不自动证明最终组合已接通。[S1]

实现 PR 必须用真实路径维护小表：**合同定义包 → 实现包 → 注入点 → 实际调用者 → 适用 guard**。不要只写“放入正确目录”；不要让领域 import app、GORM、provider SDK 或 legacy DTO，也不要为了跨包调用放宽 allowlist。窄领域 port / 正常 provider adapter 不是旧系统兼容层。

### 单次编辑的有界改动

原 Writer 在 #498 内核对 `internal/product/image` 输入合同、图像 adapter、`organizationMainSlotExecutor` / `tools.ProductImageSlotExecutor`、worker 注入与操作报价。白底请求必须直接消费 exact 来源图；不能把原图塞成“已提取 Subject”绕过旧校验，也不能只删 Extract 调用而留下错误的输入约定。[D7]

同步本片的操作列表、quote/预算、usage receipt、effect、fingerprint 和提示词版本，不再预留或记录未执行的 `extract_subject`。新步骤/输入/策略须被现有 fingerprint/version 区分，不借原键核实为不同流程的成功结果。只对当前单图路径改动；仍有合法消费者的独立主体提取和其他场景能力不顺手删除，不引入通用策略插件或双路兜底。[D7]

本交接不指定新的万能接口，不把 HTTP/BFF 作为跨域业务协议，也不要求全仓搬包。原 Writer 以当前已有合法 seam 完成上述映射。

## 4. 一次操作的运行时序与持久化边界

```text
当前用户 + Effective Organization
  → 原 acquisition operation / actor 授权
  → exact Catalog snapshot 与来源图片绑定
  → 当前 image_agent 权限 + canonical member + 产品策略
  → 持久 run / planRevision / slot / attempt / 锁定 quote
  → 原 invocation 的 commercial token reservation
  → 原始来源图直接白底编辑（1 次，Extract 0 次）
  → 原图与最终候选 Review（1 次，不编辑图片）
  → 可信 Review observed usage 记录与原窗口 settlement
  → 用户查看结果 / QA / 显式人工批准
  → 当前 Product Asset immutable receipt / ApprovedAsset
  → Account consumed/remaining 与 Audit 回读
```

这是运行时序，不是把所有领域放进一个数据库事务，也不是新增一套跨域 Saga。各 owner 保留其事务，跨商业 DB、Temporal 和 provider 的恢复沿现有持久身份与回执。PR 标出本次真正改变的提交点、失败结果和恢复入口即可。

### 输入和权限

浏览器提交原 operation、所选来源图的有界选择以及现有动作字段，不提交可信 Organization、actor、member、余额、批准状态或任意 provider URL。服务端从已授权 receipt 重读 exact Catalog，不用 latest/unversioned fallback。

采集页权限 `product_sourcing.write` 不替代 `listingkit.image_agent.write/read`；资产批准继续由其现有 owner 做完整授权。请求凭据只在现有 request-local 边界内，不进入 run、fingerprint、日志或审计。worker execution authorizer 缺失不得继续 provider 派发。[S1]

### 额度与调用身份

唯一白底图像编辑前以已锁定 Review quote 上限进行幂等预留；不能等到 Review 才检查成员额度。稳定 invocation 从持久 run/planRevision/slot/attempt 与 quote 形成，最终完整候选仍参与 InputHash。同 invocation 的 member、period、quantity 不一致必须冲突。原合同中的“首次 Extract 前”只替换为当前第一派发点，其余原窗口/幂等/未知结果义务保留。[D3]、[D7]

ImageAgent v3 slot budget/effect 与 canonical member ai_tokens 不是同一个事实；不得伪造图像编辑 token，或把 Review token 结算宣称为全部图像费用已结算。未执行的主体提取不产生本片 quote/reservation/usage/effect。原实现者明确两类预算的实际覆盖范围与未知项。可信 observed token 在原 reservation window 结算，跨窗口不归入新窗口。[D3]、[D7]

### 生成、用量与批准分离

编辑要求是以输入商品为唯一主体，只改干净白底，保留形状、比例、颜色、材质、图案文字和配件数量，不添加装饰、不裁切主体。Review 比较原始来源图与最终候选，核对商品保真和白底目标。此约束不是保真保证，既有 QA 门槛与人工批准仍保留；不因减少步骤降低质量要求，不自动再次编辑补救。[D2]、[D7]

observer 已调用、response=nil、usage=0 不能单独证明可信用量。已知用量但输出解析/ValidateReview 失败，沿已准入的 Review 专用 `usage_observed_failed` 语义结算；不代表 QA 通过、生成可用或资产已批准。普通 Failed 合同不顺带改写。[D4]

用户批准必须绑定当前 exact Product/候选/审批动作。completed workflow 的相同 action 响应丢失，只能按当前授权从不可变 Product Asset 回执核实原 payload；不调用已关闭 Temporal Update，不补写，不对新 action/不同 payload 盲返成功。[D5]

## 5. 失败与恢复责任

| 事件 | 必须保留的结果 | 执行者核对的唯一恢复责任 |
| --- | --- | --- |
| 额度不足 / 授权失败 | 唯一白底编辑前拒绝，零 provider 调用 | 原授权/商业 owner；不切换身份、渠道或 invocation 绕过 |
| reserve 提交或响应不明 | 保留原 invocation 和参数，不另开操作 | 既有商业 reservation/receipt 重读或同身份核实 |
| provider 已派发，响应/用量不明 | 保留 Dispatched/UNKNOWN 与 reservation，不当确定失败释放 | 原 invocation / Temporal effect 合同；无可信证据不重调 |
| 可信用量，输出校验失败 | 幂等结算原用量，产品结果仍失败/不可批准 | 原 invocation/settlement owner；重放只补结算 |
| 图片未达质量门槛 | 如实不通过，不自动二次编辑或切回两段生成 | 原 Review/批准 owner；未通过不可写 ApprovedAsset |
| 确定未发生效果的失败 | 仅满足原 terminal rejection 条件才 release | 原商业 owner；取消/超时本身不是无效果证据 |
| settlement DB 失败 | 不把半结算当成功，不再次调用 provider | 原调用事实与商业 owner 恢复相同结算 |
| 原预留后窗口切换 | 结算仍绑定原窗口 | 原 usage owner，不转计新窗口 |
| 批准已提交但 ACK 丢失 | 原 action/payload/asset receipt 可核实 | 原 Product Asset owner；同键核实不得重复批准 |
| 重启 / 迟到结果 / 企业切换 | 持久身份不变，当前权限与 exact binding 仍校验 | 各原 owner；UI 只投影，不另建后台恢复引擎 |

表内是已记录合同的执行核对点，不是本文件声称这些测试均已通过。实现 PR 给出每项实际符号、已有或新增针对性测试及结果；不增加通用故障注入平台。

## 6. 受控环境与本地生成图片输出

controlled provider 只能进入明确的隔离 acceptance profile，必须走真实 current application/worker、Temporal、invocation、commercial settlement 和 PostgreSQL；默认/生产不新增 test-control route。不以 fixture-only 或开发者直调内部函数替代用户入口。

[D6] 只允许服务端生成图片的受限本地 HTTPS 输出。[E2] 记录了原分支的局部实现、MinIO/Traefik 负例和增量复核，不能继续笼统要求从零设计。最终单次编辑组合仍需保持受信任 durable 资产身份、exact URL、允许读取者、origin/路径限定、启动 profile、失败关闭和人工批准持久化合同；只核实际变化，不由本文签发产品验收。

禁止全局放宽 `ValidateSafeImageURL`；1688 来源图、用户输入和任意外部 URL 不因本机输出例外获得私网访问；不伪造公网域名解析到 loopback 绕过 SSRF。不复制 provider 认证或把本机 URL 当成所有部署均可用的永久公网资产。

命名卷、原回执、浏览器证据与其他任务资源保留；stop/restart 不等于 destroy。证书信任、合成凭据输入、额外身份/组织变更、真实外部调用均沿原指定环境授权，不从流程精简或文档整理推导新权限。不为证明单次编辑自行增加真实来源或开启付费模型。

## 7. 原 Writer 的连续交付顺序

1. 保留原分支/PR #498、既有实现与本地草稿；按 [D7] 调整源图输入合同、唯一白底编辑及 Review、操作预算/执行记录和直接回归，更新原有代码依赖/注入表。不重建 resolver、结算、资产系统或增加竞争 Writer。
2. 复用未变化的 Review/额度、本地输出与恢复证据；对本次改变的第一派发点、quote/fingerprint、候选输入、调用计数和最终组合做有界增量检查，不重新审核全局架构。
3. 在同一主要实现 PR 完成用户可见 BFF/UI、最终 current runtime/worker 装配、Account/Audit 投影和必要开发自检。精确 Figma 节点确定前不签 UI parity，但非视觉步骤不因此停工。
4. 交付可重复正常启动方式、页面路径、受控 provider/profile、持久化和资源归属、已实现/未开放能力。适用 CI 与最终 diff 复核后交独立 #473/#438 验收，不自行宣称产品通过。

这四步是一个用户结果的交付顺序，不是要求拆四个 Issue/PR。本文没有授权合并、关闭、部署、真实数据操作或付费调用。

## 8. 开发证据与独立验收的交接

| 层级 | 必需结果 | 不能替代的结果 |
| --- | --- | --- |
| 精确输入/规则/权限测试 | 原 receipt 和 exact version、越权、缺依赖、source-based 请求、QA、错误/重放语义 | 真实登录或浏览器用户流程 |
| 真实 PG + Temporal + 受控 provider 集成 | 正常 image edit=1、Review=1、Extract=0；quota 不足 provider=0；实际 quote/usage 无未执行提取步骤；幂等结算、批准、失败/恢复 | 真实图像质量、Figma parity、真实1688/付费provider |
| 最终 current runtime 的独立浏览器验收 | 采集详情操作、可见结果、人工批准、Account/Audit、刷新与重启 | 全部渠道/生产验收 |
| 适用最终 HEAD CI / 独立 diff 检查 | 实际候选的相关回归、依赖和接线 | 用户验收与生产签收 |

正常成功的 provider 总调用为 2；同 invocation 已执行结果重放不新增派发，质量不通过不会触发第二次编辑。[D7] 原开发报告中“三次本地 provider / Review 7 tokens”等仅是旧两段路径受控 fixture 的观察，不是单次编辑已通过、生产固定消耗或全链验收。[E1] 两段局部集成不拼接成一条已实跑浏览器证据。[D5]

真实质量用少量有代表性、已获准的商品图核对保真与白底目标，记录失败，不建设新的基准平台。未授权的真实来源、模型调用继续 NOT_RUN。不能仅凭少一个步骤宣称耗时/费用减半或质量提升，也不把受控 PNG 当作质量样例。[D7]

回报复用原 PR：Must → 代码符号/测试 → exact HEAD/环境 → PASS/FAIL/SKIP/NOT_RUN → 限制。#473 记录最终完整调用与额度链，#438 汇总产品验收；不新建平行状态文件或重复审计账本。历史有效证据注明原 SHA 和未变化范围，不倒签为当前 SHA 实跑。

## 来源

[issue487]: https://github.com/qq550723504/task-processor/issues/487
[issue473]: https://github.com/qq550723504/task-processor/issues/473
[issue438]: https://github.com/qq550723504/task-processor/issues/438
[S1]: https://github.com/qq550723504/task-processor/issues/487#issuecomment-5827255048
[D1]: https://github.com/qq550723504/task-processor/issues/487#issuecomment-5827295101
[D2]: https://github.com/qq550723504/task-processor/issues/487#issuecomment-5827434782
[D3]: https://github.com/qq550723504/task-processor/issues/487#issuecomment-5827468094
[D4]: https://github.com/qq550723504/task-processor/issues/487#issuecomment-5827608666
[D5]: https://github.com/qq550723504/task-processor/issues/487#issuecomment-5829021759
[D6]: https://github.com/qq550723504/task-processor/issues/487#issuecomment-5829308707
[D7]: https://github.com/qq550723504/task-processor/issues/487#issuecomment-5842544071
[E1]: https://github.com/qq550723504/task-processor/issues/487#issuecomment-5828964386
[E2]: https://github.com/qq550723504/task-processor/issues/487#issuecomment-5830323285
