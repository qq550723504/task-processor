# 1688 单图白底生成：架构与执行交接

日期：2026-09-25；单次编辑及取消默认模型审图更新：2026-09-26。关联：[#487][issue487]；独立验收 [#473][issue473] / [#438][issue438]。

**性质：批准决定与执行边界的有界交接，不是新全局架构、IMPLEMENTATION_READY 签发或验收报告。当前产品路径为一次生成、程序校验、人工确认；运行实现尚由原 Writer 完成。**

原核对基线为 main `b47d6302b99c5229ba89825d48ea9b2823e400af`；文档交接分支基于 `ce39b52b1e458fe0657ddea91ab48ef2cde9edf8`，已随 #498 纳入。#498 已合并，仅作准备/安全修复的历史基线，不再接收后续交付；修订沿同一文件，由原 Writer 纳入后续一个主要 PR（Refs #487），不新增实现者或额外文档合并前置。本地草稿、原事实与历史测试保留；实时 HEAD/CI/进度只在当前 Issue/PR 维护。

## 1. 当前批准的用户结果

从已有 `/workbench/supply/acquisition/operation/{operation_id}` 采集详情，选择一张已入库1688来源图，直接生成通用白底主图，查看原图与结果，显式人工批准。[D1]、[D7]、[D8]

```text
exact 来源图 → 身份/权限/版本校验 → 生成预算准入与预留
→ 一次白底图像编辑 → 程序校验与候选持久化
→ 原图/结果对照预览 → 人工批准 → Product Asset 回执
```

正常无故障模型调用为 image edit=1、Extract=0、model Review=0。程序校验不调用质量评分模型；不生成主体提取中间资产，不自动重生成、二次编辑或回退旧两段流程。[D8]

白底与商品保真目标仍保留：只修改背景，保持形状、比例、颜色、材质、图案文字和配件数量，不添加装饰、不裁切主体。提示词及程序校验不是语义质量保证，最终由用户对照确认。未批准不形成 ApprovedAsset，不自动发布平台或改写 Catalog。

不要求目标平台/国家/品类，不新增顶层 AI 工作台。内部 `product/zz/default/general` 保留，zz 不是真实国家。**[D8] 明确取消本片模型评分0.70及关联Review阈值门禁，不取消商品质量目标、人工批准或既有内容安全措施。** 不全局降低其他场景阈值，不注入假Reviewer或满分来通过旧检查。

Figma精确动作节点仍只阻视觉定稿；非视觉接线与本次有界调整继续。新增用户显式重生成、自动审核配置平台、批量无人值守不在本次范围。

## 2. 决定的替代关系

| 依据 | 当前适用内容 | 被替代或不能推断的部分 |
| --- | --- | --- |
| [入口准入][S1]、[入口批准][D1] | receipt/exact Catalog、当前身份与领域owner | 不能证明默认运行链已接通 |
| [白底策略][D2] | 单图、通用白底、保真、人工批准 | 模型0.70门禁被[D8]取消；不是降低全部安全要求 |
| [单次编辑][D7] | 无独立Extract、无自动二次编辑 | 其中“一次Review”被[D8]替代 |
| [取消模型审图][D8] | 单次生成、程序校验、人工确认；真实生成计量 | 不代表代码已切换或新计费政策已批准 |
| [派发前预留][D3] | 实际第一派发前准入、幂等、原周期和UNKNOWN保护 | 不再强制为未执行Review预留quote/token |
| [Review用量失败][D4] | 原Review消费者及历史证据仍有原语义 | 不自动成为图像生成的计量合同，不为复用它继续审图 |
| [批准回执恢复][D5] | 原action/payload/不可变资产回执核实 | 不允许新action重放旧批准，不代表浏览器已验收 |
| [本地输出][D6]、[局部复核][E2] | 受限durable生成对象的本机HTTPS边界及已有证据 | 不放开任意私网URL，不签发生产验收 |

只取消明确被新决定替代的步骤和评分要求。其他领域、安全、幂等、持久化、内容安全、操作权限继续适用[AGENTS](../../AGENTS.md)、[派工规则](issue-driven-development.md)和[已有架构](../architecture/README.md)。原审批/测试保留为原流程证据，不改署新流程PASS。

## 3. Owner与代码落点

| 职责 | 当前owner / 核对入口 | 本片接入要求 |
| --- | --- | --- |
| 来源/商品版本 | Sourcing receipt / Catalog | actor/org/operation/publication/ProductKey/version/所选图精确绑定，无latest fallback |
| 身份与授权 | workbenchcontext、EffectiveMemberID、ExecutionAuthorizer | 当前企业和read/write/执行权限；不信任客户端身份，不持久化秘密 |
| 生成与恢复 | ImageAgent repository / Temporal v3 effect | 持久run/planRevision/slot/attempt和唯一生成操作 |
| 调用事实 | 现有aicapability / GormInvocationRecorder / 图像effect事实 | 记录实际白底调用，不借Review操作名、不复制账本 |
| 预算/商业计量 | 当前图像预算及商业用量owner | 匹配真实单位和获准政策，移除未执行Review的预留；见§5 |
| 文件校验/存储 | 现有Product Image/Asset及durable staging | 解码、大小/格式/尺寸、完整性、exact绑定；不是商品质量评分 |
| 人工批准 | Product Asset / immutable approval receipt | 当前授权用户显式批准，原action恢复不重复写 |
| Account/Audit | 当前权威事实投影 | 调用、用量、程序有效、人工采用、结算状态分开 |
| 装配与生命周期 | app/httpapi、app/worker/imageagent、currentapplication | 显式注入；app不拥有新计费/质量规则 |

以已合并#498的“合同定义包→实现包→注入点→消费者→guard”表为历史参考，在后续主要PR维护实际变化。核对source-based输入、adapter、organizationMainSlotExecutor/tools.ProductImageSlotExecutor、worker和批准条件；不把原图塞成已Extract的Subject，不保留本片Reviewer强依赖后注入fake结果。

移除本片未执行的extract/review操作列表、quote、预算、usage/effect成功记录和模型评分必需项；同步现有fingerprint/版本/必要状态与测试。不得将旧运行回执重新解释为新步骤成功。保留其他合法消费者的Extract/Review，不全仓删除能力、不新增双路fallback或策略插件，不扩allowlist。

## 4. 程序校验、预览与批准

复用当前图片校验与资产合同，不另造图像质量系统：返回值必须是可解码的允许格式，满足当前尺寸/大小限制，持久对象完整且绑定正确来源/原操作。坏文件、错误身份、保存失败不能成为可批准资产。

程序校验通过只表示有效候选可展示，不表示商品细节无误。界面展示原图与结果供人工对照；质量采用决定由用户作出。未执行模型审查不得显示“AI审核通过”、自动填分数或写虚假质量审计。人工不采用不触发隐藏重生成，也不抹去已发生用量。

人工批准仍核对当前权限、exact商品/候选和action。completed workflow下批准ACK丢失，按原action/payload从不可变Asset回执核实；不向已关闭Temporal Update重复写，不对不同action或失权用户盲返成功。[D5]

## 5. 计量必须来自真实生成调用

**最新用户决定优先：按 Figma，图片按张配置消耗统一 AI 点数，唯一余额 owner 为 orgresource.ai_point，替代独立图片余额。调用前预留配置所需点数、生成成功扣除（不采用仍扣），确认未生成释放，UNKNOWN保留；不扣现有AI Token，真实token只审计。服务端价格缺失即 unavailable，不设1图=1点或月度默认额度，见§9.3。下述原token-owner调查仅保留历史依据，不再要求图像消费ai_tokens或token上界作为本片准入；撤回不写成PASS。**

**当前接缝：原候选把canonical ai_tokens主要接在recordedReview。取消模型审图不能只是删除调用后留下无预算的图像生成，也不能继续为不存在的Review扣额度。** #498已移除旧预留并关闭未就绪的新生成入口；原Writer在后续主要PR继续实际生成的最小必要接线，不重建结算系统。[D8]

先明确所选生成provider实际返回的usage字段、现有图像预算覆盖的单位、商业owner支持的单位，以及相应报价/预留/结算入口。复用已有合同；确有新经济政策才报告具体决定，不预设每张价格、tokens换算或套餐规则。

- 首次且唯一编辑前仍需当前获准的权限与预算准入。没有可执行的准入方案，只阻受影响的实际开放并报告最小缺口；不要求先建设通用计费平台。
- 有可信生成用量，按获准且单位匹配的合同记录/结算；provider真实token、图片次数和货币成本不得互相冒充。没有usage不是0，不伪造Review tokens，不把“一张图片”证明成canonical ai_tokens已结算。
- 调用已发生、图片程序有效、人工采用、商业结算是不同事实。可信用量已发生但结果无效或用户未采用，仍保存调用/用量，不凭不采用自动释放实际消耗；实际收费沿获准政策，本文不发明失败收费规则。
- 已知生成成功但用量未提供，与连派发结果都不明须分别表达；沿现有owner保存真实状态。未知计量不得冒称已结算，不以缺计量为由再次生成。
- 持久调用身份绑定org/member/run/slot/attempt/quote及新流程版本；同键不同请求、单位或流程冲突。周期预留仍回原周期，不切到新周期重复扣减。
- 已生成结果/可信用量的重放只补原artifact/记录/结算，不重新派发。响应丢失、取消、DB故障、UNKNOWN不等于未发生效果；release只依据原合同可证明的无效果终局。

生成可信用量一旦成立，就进入独立的原owner记录/结算与Account/Audit回读链，不等待人工批准。图像记录和商业提交边界分开，不包成跨系统大事务，不另建Saga/恢复服务。

## 6. 失败处理与验证重点

| 事件 | 当前要求 |
| --- | --- |
| 权限/获准预算不足 | 编辑前拒绝，图像模型派发0次 |
| 正常无故障生成 | 编辑1次、Extract0次、额外模型Review0次 |
| reserve或派发响应不明 | 保留原身份/参数，按原owner核实，不换键重新生成 |
| 坏图、错绑定、保存失败 | 不显示有效可批准资产，不自动编辑修复；真实调用/用量不能抹去 |
| 用户不采用 | 不产生ApprovedAsset，不自动重生成，不推断用量为0 |
| 用量未知/结算失败 | 实际生成状态与计量状态分列，恢复原记录/结算，不虚构token或重复provider调用 |
| 周期切换、并发、重复请求 | 原预留/调用和fingerprint不变，不重计/串组织 |
| 批准ACK丢失 | 原action/payload/immutable receipt核实，当前权限依然有效 |

这张表组织已有风险的针对性验证，不要求新增通用故障平台。原Review专项测试只能证明其原消费者，不直接改名当生成计量已验证；保留未变化的有效安全和资产证据。

## 7. 环境与接续实施

受控provider仍需走final current application/worker、真实PG/Temporal及当前调用/用量owner，不以fixture-only代替独立用户验收。default/production无新增test-control route。

[D6]/[E2]的本机HTTPS输出只适用已验证durable生成对象、固定origin/路径及读取范围。禁止全局放宽ValidateSafeImageURL，来源图片/任意外部URL仍拦私网；不伪公网域名绕过SSRF，不把本机URL声称为永久公网资产。API/worker最小权限、staging/目录/写入口隔离和保留卷重启证据按变化复验。

继续原Writer，以已合并#498为基线，在后续一个主要PR消费当前Issue决定，推进尚缺的真实生成计量、安全接线和直接测试；不重做已完成且未变化的单次source输入和本片Reviewer解耦。仅新增/变化的授权、计量或恢复边界作增量高风险复核；正常独立代码评审不因取消产品模型Review而取消。未变设计/代码不重复全局审核。

保留既有本地草稿和其他任务资源。用户指定来源、合成身份、CA、实例/卷权限沿原批准；stop/restart不等于destroy。真实来源质量样例和付费provider没有额外授权，不自行扩展。Figma只阻视觉定稿，不阻已批准的非视觉调整。

## 8. #473 / #438验收交接

验收应覆盖真实业务调用，不能为获得token消耗而恢复模型Review。[D8]只取消本片的模型审图与评分门禁，不自动撤销父项其他Must或把未知计量记为完成。

若生成provider没有适用的token事实，#473的canonical ai_tokens项仍有缺口：图像预算/次数证据不能代替。原owner报告可复用的实际生成计量接缝或具体待决点；不为凑验收新增一个AI功能、虚构token或另一套结算平台。未来确要用其他已有真实业务能力提供token证据，由原协调方确认，不把它变成本次隐藏调用。

最终证据分别给出：新流程调用计数与程序校验；真实生成预算/用量及未知边界；PG/Temporal的幂等/恢复；原图/结果预览与显式人工批准；Account/Audit；准确候选CI及独立用户验收。未运行项保持NOT_RUN；取消的模型Review是需求被替代，不标PASS。

旧“三次provider/Review7tokens”及“两次provider”只属于原流程。[E1] 未变化的授权/资产等证据可复用，但不能证明新次数、新计量或新用户链。受控2×2PNG不证明商品质量，不承诺成本/延迟减半。少量获准真实样例只核对当前保真与白底目标，不新建基准平台。

接续交付留后续主要PR（Refs #487）：实际代码/测试与环境、结果及限制、正常启动/页面/保存/启停、剩余阻塞。#498不再接收提交。无新增业务Issue或文档合并前置；未授权后续PR合并、关闭Issue、部署、真实/共享数据或付费provider操作。

## 9. GRSAI gpt-image 与统一 AI 点数增量（边界已独立 IMPLEMENTATION_READY，实施中）

### 9.1 当前决定与已撤回草稿

用户最终明确 **GRSAI / gpt-image-2.5 标准 1K / 复用已有密钥**。原本节 NanoBanana async-task 草稿尚未准入、没有实现，现在撤回，不作为待办或后续前置；不新增 accepted-task phase/job JSON/轮询恢复。历史调查仍可查 Issue，但不能覆盖新选择。#500 已合并，仅作历史基线；后续唯一主要 Draft PR #503、原唯一 Writer，三份视觉草稿保全。

官方 [images/edits schema](https://qmy27nhsd9.apifox.cn/512807352e0.md)列出 gpt-image-2.5、quality=auto、multipart image 字符串示例、成功 usage 的 input_tokens/output_tokens/total_tokens。该文档不是实测，URL 示例不能证明文件上传不支持，也不能证明当前 image[] 文件字段兼容。此项作为parser适用协议的历史证据；当前选定§9.5F官方统一json/base64接口，不将未验证multipart格式列为唯一前置。**不直接把可变来源 URL 传 provider 替代已授权 exact source bytes**，不新增公网输入上传平台、协议 fallback 或本机 SSRF 例外；未经授权不付费探测。

请求 schema 无 max_tokens/max_output_tokens 或总 token 上限，示例3569不构成上限；最新§9.3决定按张配置扣统一 AI 点数，token上限不再是该准入前置，也不按该样例收费。HTTP/worker gate继续因服务端图像价格、正式准入与恢复接线尚缺而关闭，不自定配置值。

### 9.2 已准入的局部解析与最短现有路径

复用现有 source-only ProductImageAdapter → route-bound multipart client 的零重试、禁 redirect POST replay 与 V3 UNKNOWN/bundle 恢复。同步响应没有可查询 task ID，不强加异步状态；响应丢失仍 UNKNOWN，无自动重发或换键重试。

image-only decoder 同时接 JSON generation 和 multipart edit，明确解析 input/output/total，做存在性、非负、正 total、自洽与整数溢出检查，映射到现有规范化 Usage，并显式设置 ImageResponse.UsageKnown。缺失/无效不是零消耗，Known=false 且不暴露伪规范化 counters，图像响应仍可返回；不修改 chat 的 prompt/completion JSON，不猜第二 schema fallback。供应商声明/样例只用于受控 fixture，不冒充真实调用。

后续观察接缝必须位于收到 typed response 之后、下载/解码/候选校验之前；否则真实用量会随坏图或下载错误丢失。观察随下节的次数事实统一设计，不单独铺持久字段；不能把目前内存解码通过写成durable/settled PASS。

### 9.3 用户最新统一 AI 点数决定与待设计边界

用户进一步明确：**按 Figma，图片按张计量、按服务端版本化价格扣统一 AI 点数；orgresource.ai_point 是唯一余额 owner，不扣现有 ai_tokens；真实图像 token 仅审计。** 此决定替代此前独立图片余额提案，不新增 image resource、月度默认额度或1图=1点换算。调用前预留本张图片配置所需点数，provider确认生成成功扣除；后续下载/校验失败、用户不采用仍扣。确认未生成才释放；UNKNOWN保留，不换键重新生成。没有query句柄不能虚构恢复。配置缺失明确 unavailable，配置数值不阻合同与合成测试，不以旧自然月/job mirror合同代替。

因此原“为图像generation接ai_tokens结算/单独invocation observation字段/独立图片余额”的提案撤回，未实施。真实token解码仍有价值，但不能写成扣除AI Token或当前计量已完成。provider生成成功、程序有效、人工采用、AI点数提交与token审计是不同事实；不能把整个workflow失败/取消或坏图当作provider未生成的proof。

下一次最小持久边界设计复用 internal/ledger/orgresource 与 internal/integration/orgresource 的 reservation/settlement及TransactionalReservationOwnerStore。同事务要求须先解决实际DB部署：当前V3 effect在ImageAgent DB，资源在commercial owner DB，不能直接传不同DB的gorm handle冒充共享事务。图像token观察随该次调用的权威effect事实一并保存，不重复建立独立账本或先铺一套观察字段。必须保持 canonical org/member/run/slot/attempt/route/config/request fingerprint、服务端价格版本/quantity不可变、同键异量冲突、幂等、UNKNOWN held与现V3 bundle恢复；不得复活product_image_jobs_succeeded的legacy mirror。

现GormInvocationRecorder的Succeeded立即settle、Failed释放、UsageObservedFailed仅Review，不能通过借名调用达成新图片规则。现ReserveSlotProviderV3创建即provider_claimed，但资源owner的NotStarted只能表示尚未派发，不能仅凭该phase重新预留/派发。必须在同份最小设计明确独占dispatch证明、reserve后submit前崩溃、late ACK、终局effect单调保存与settle ACK恢复；恢复只补原事实，未知不重发。该新增持久边界已按§9.5取得IMPLEMENTATION_READY，正在同一PR实现；HTTP/worker gate保持关闭。

### 9.4 当前可交付与停止线

已确定且可独立验证的是image-only token decoder及现provider no-replay。§9.5F统一json/base64 adapter及exact source bytes的tools/port链已实现并有受控httptest证据；正式worker/资源及恢复装配尚未完成，不直接传可变来源URL、不live/付费探测。无已实现task恢复合同的派发不明沿原UNKNOWN，不建设异步任务状态/查询平台。

后续按张AI点数与token审计统一设计至少验证：配置缺失不派发；按固定价格原子拒超额；成功只commit原预留一次；不采用不退款；confirmed-no-generation才release；response loss/DB ACK loss/取消/restart/同身份重放均不重发或重计；异org/member/fingerprint/价格版本冲突；可信usage在下载/校验失败前可保全但不扣ai_tokens。范围沿现owner与已有测试，不另建验收工具。未执行真实provider/PG/Temporal/browser的项仍NOT_RUN。

## 9.5 本次具体方案：固定 image owner 的跨库不可变回执（IMPLEMENTATION_READY）

本节为已准入待完成实现的方案，不是已有能力/PASS。A–D/F高风险检查及E月限额增量检查均已给出IMPLEMENTATION_READY；[原检查记录](https://github.com/qq550723504/task-processor/pull/503#issuecomment-5844151392)保留原阶段范围，E后续检查由同Reviewer确认，不冒称产品验收。当前 `image-db:5437/image_agent` 与 commercial DB 是不同容器数据库，`worker/dependencies.go` 分别建连接。`orgresource.TransactionalReservationOwnerStore` 的同事务锁 owner/Bind/terminal-proof 不能直接满足；现生产也没有 ReservationAuthorizer 或已注册的 reservation owner。最小改动采用下述**仅 image_generation_v1** 的显式 receipt-based 入口，保留其他 owner 原共享事务合同。不迁库，不复制 V3 row，不新余额表、Saga、runner 或定时扫描服务。

### A. 唯一事实与固定身份

- ImageAgent V3 attempt 是生成意图、dispatch fence、provider效果和真实token审计的唯一 owner。现 effect row 增加一个受限 generation JSON fact（schema version；immutable intent；reservation binding；dispatch state；immutable outcome；settlement receipt reference），不新建通用 invocation owner。原 staging/publication phase 独立，不能用它覆盖生成效果。
- `intent_id` 固定来自 org/run/planRevision/slot/attempt（不得把变化的 quote hash 放入该主键逃避冲突）。fingerprint 包含 actor/canonical member、exact Catalog version/publication/hash、source bytes digest、prompt/policy version、route/provider/model/credential config identity（不含secret）、统一协议/1K/quality、服务端 price version 与正整数 AI point quantity、resource=ai_point。价格相同但版本不同仍冲突；原attempt不在重放时重新报价。
- 配置 owner 是现运行配置装配的版本化 image price 小合同：明确 provider/model/resolution/operation 与 points；没有配置或不匹配就是 unavailable。配置值不来自浏览器/Figma样例；本片不新增价格管理UI、月度额度、图片资源或token换算。
- 资源 owner 仍只使用现 `saas_organization_resource_operations/reservations/buckets/events/audit_logs`。资源 operation 的不可变请求摘要、receipt/evidence hash 是其扣占依据，不是第二份可变 V3 状态。generation fact 只保存资源receipt身份/摘要，不复制余额作为authority。

### B. 窄入口与现合同的精确替代

在 `internal/integration/orgresource` 显式构造 `ImageGenerationReservationExecutor`，仅注册常量 image_generation_v1 + ai_point；继续消费 ledger 的正quantity、fingerprint、余额/负债规则。它接一个**内部 owner reader**，只按身份读取 V3 已持久化 intent/不可逆 terminal proof；API不收proof/outcome，不能靠caller拼 succeeded。reader通过ImageAgent owner方法读取，其跨模块桥在 integration/app装配，不把商业gorm tx传ImageAgent DB。

窄入口为 `ReserveImageGeneration` / `ReadImageGenerationReservation` / `FinalizeImageGeneration`。普通 `ExecuteReservation`、`ExecuteSettlement` 和 TransactionalReservationOwnerStore 不放松。资源写事务内沿原 bucket/debt顺序、原event/audit/immutable result持久化；提取现账务计算/写入helper复用，禁止第二套balance算法。

所有三个写/封闭动作使用固定 org+resourceOperationID 的**现 operation row**作资源侧串行 fence：在事务内 insert-on-conflict 后 FOR UPDATE，校验固定请求fingerprint；不要跨网络/数据库持有该锁。读取不可变owner证明在锁外完成，入事务后校验proof身份和digest。可变“目前未开始”的远端快照不足以授权释放；只有已CAS成不可逆 no_generation/succeeded 的proof可终结。

`FinalizeImageGeneration` 在尚无 reservation 时同样必须封闭该 operation：只接受 owner 的不可逆 no_generation proof，写现 operation 的 terminal rejected/closed immutable receipt（不改变余额、不伪造零quantity事件）。随后晚到的 Reserve 必须返回相同closed结果而不能预留。若Reserve先提交，则Finalize释放其原reservation。这不是新恢复owner，而是关闭“cancel读到不存在后并发reserve晚提交”的孤立扣占窗口。窄reader理解本owner的closed receipt；普通 reservation replay 语义不变。

`docs/architecture/project-boundaries.md §3.10` 必须与该实现同批更新：默认同tx规则保留，仅此固定image owner以不可变intent/terminal proof + 双方各自CAS + 资源operation fence替代；禁止其他owner自动采用。现 `SettlementService` 的caller不能选择outcome原则保留；no_generation映射release，provider_succeeded映射commit，未知不能结算。此固定owner合同例外已通过本次独立IMPLEMENTATION_READY，随实现同步§3.10，不适用于其他owner。

### C. 唯一派发顺序

1. 当前 actor/org/canonical member live授权、exact source读取/bytes digest、资源消费授权、route与版本化价格核实全部成功；缺依赖在provider前拒绝。V3原slot预算仍独立，不把AI点数当images/tokens单位。
2. V3独占claim后 `PrepareGenerationIntent` 在本库CAS保存完整intent，dispatch=`prepared`。这是尚未派发的证明，**不是**原phase=provider_claimed的推断。只有该新generation fact合同的当前单main-slot使用此路径；旧记录不做兼容迁移/自动放行。
3. 调用资源 Reserve：owner reader核intent，资源事务得到不可变receipt。ACK丢失以原operation/intent查询或原键Reserve重放；不得换operationID。回读校验org/ownerAttempt/resource/quantity/fingerprint/state=reserved后，V3 CAS绑定reservation ID及receipt hash。绑定失败不调provider，重试只补原binding。
4. 再次live授权和route/config核验；若已取消/失权/配置漂移，V3 `prepared → no_generation` CAS不可逆，Finalize原资源operation，不能重发。否则 `BeginGenerationDispatch` 将prepared+exact binding CAS为`dispatch_started`，**只有CAS获胜且成功ACK的调用栈**可做一次POST。CAS ACK不明，即使回读是dispatch_started也不授权发送；保留UNKNOWN。不会发放可被新activity重复消费的持久“许可”。
5. adapter持有同intent observer，在收到typed成功payload后、任何下载/解码前，`RecordGenerationOutcome` CAS写provider_succeeded、provider request id（有则存）、结果引用摘要和可信token（有则Known=true，无则明确unknown）。本次官方wire没有usage合同，不填0。成功fact持久化后可以先Finalize commit点数，再继续下载/原bundle/staging；任何后处理失败不释放。
6. 已证明provider未调用的前置失败可写no_generation；普通HTTP错误、timeout、EOF、取消、JSON错误、running状态均不凭推测释放。当前没有已核“失败响应一定无生成”的可信映射时保守UNKNOWN。dispatch_started之后只能同一次observer的可信结果推进；late成功可由UNKNOWN单调到succeeded，但不能由no_generation翻转。对已dispatch路径不会由恢复线程制造no_generation。
7. Finalize只读不可逆proof、按原资源identity原子commit/release，V3保存其receipt引用。同键同proof幂等、异proof/quantity/quote冲突。资源ACK丢失只回读/补Finalize，不回provider。经济终局和Asset批准完全分开。

### D. Execute/Recovery/取消并发的消费规则

现 `ExecuteSlotV3` 的“claimed但非首次→只查bundle”，以及 `RecoverEffectV3` 的“providerUnknown直接block”，均在当前固定image owner分支先消费上述generation fact，不只加列。

| 持久状态/故障点 | 原attempt恢复动作；禁止项 |
| --- | --- |
| intent未提交 | 无资源预留、无provider；原请求可按现幂等合同重进 |
| prepared，Reserve/Bind响应丢失 | 原资源operation回读/重放并补绑定；自动恢复不派发。若取消/恢复选择终止，先CAS no_generation，再Finalize封闭operation，即使原Reserve晚到也不能留下扣占 |
| 原执行仍活跃 vs 恢复终止prepared | BeginDispatch与no_generation竞争同一V3 CAS；只允许一个获胜。失去CAS者绝不POST |
| dispatch_started或其ACK不明，无terminal fact | UNKNOWN、保留原预留；无新POST、无虚构query、无定时释放 |
| 成功响应已收到但V3 outcome写失败 | 同调用栈在bounded detached finalization内重试同payload；若仍失败/进程丢失则UNKNOWN held，不能据内存成功无持久proof收费/释放；不重生成 |
| provider_succeeded，settle/下载/bundle失败 | 先按原proof补原结算，再恢复现bundle/已有结果的安全下载；坏URL/坏文件仍不批准。不得让MarkBudgetUnknown或phase迁移清除success/token事实 |
| no_generation，资源尚不存在/ACK丢失 | Finalize封闭原operation或释放已存在reservation，回读确认；不得把“未读到reservation”当已安全结束 |
| 已terminal，workflow取消/用户不采用 | 只恢复对应commit/release回执，不改变provider效果，不重复生成 |

恢复继续由现 Temporal effect recovery workflow/activity负责，有界重试/原身份人工redrive沿现入口，不新scanner。为经济finalization提供只接受已存在V3 identity的内部窄恢复分支：撤权不能触发新reserve+dispatch/读图，但不应阻止既有terminal proof的后台结算/释放；调用者为显式装配的原effect owner，不是租户HTTP可自选outcome。外部GET/Approve仍实时权限。prepared终止仅作资源operation封闭，不需重新给撤权actor分配权限或资金。

### E. 已批准成员月度消费上限：增量设计 IMPLEMENTATION_READY

生产已有 `OrganizationExecutionAuthorizer` 使用 exact active grant、AuthorizationID==MemberID、组织未停用及 image_agent.write；复用它，不新IAM/permission。该permission授予operator/admin，但仍不能省略成员被管理员分配资源的约束。`accountallocation.MetricToken` 是成员token额度，不是AI点数分配；商业purchase/topup权限是购买，不偷换模型消费。Figma 1636:371/431:4158要求管理员分配给成员，统一企业余额并未批准全员无分配共享消耗。

用户已批准“补齐成员月度消费上限，按 Figma”，数值由管理员设置，企业余额不清零。该limit是使用企业AI点数的约束，不是成员钱包/余额。现 `accountallocation.QuotaReader.ReadTokenQuota`、`Service.SetTarget/Snapshot/Consume`强制权益window，DTO同时带MetricToken和WindowStart/End；抽取其中仍有效的成员校验/版本/幂等交互，但不wrap Token Service、不换label/metric、不迁移旧表或使用saas_usage计点。

**事实拆分与月份。** 在 `orgresource` owner 增加成员限额配置 row（org/member主键，monthly_limit、version、active、updated_by/at）及月计数 row（org/member/month_start主键，reserved、consumed）。配置不作为credit，不累加到bucket，不要求所有成员limit之和等于企业余额；每次调用同时满足个人remaining与企业available。未配置或limit=0即不能新消费，不填100等样例。时区复用仓库已核UTC自然月约定（`usage_ledger.go:canonicalUsagePeriodKey`的UTC YYYY-MM及商业read的UTC月[start,end)），只复用时间约定、不接其Token ledger。服务端在V3 intent首次持久化时固定month_start/end和当时配置version，进入fingerprint与原资源reservation；浏览器不能传月，跨月同key回放不改月/价格/member。

**配置命令与下限。** 现account边界新增明确AI点数 `ReadMemberAIPointLimits` / `SetMemberAIPointMonthlyLimit`，复用现管理员权限、live active canonical成员、ExpectedVersion+IdempotencyKey；服务端决定current UTC月，管理操作identity绑定首次月份，原key跨月返回原immutable结果而不再次修改。Target允许0，但不得低于current month consumed+reserved；旧月UNKNOWN的reserved不释放、不抹计数，旧月不因新配置迁移。配置持续用于后续月份，新月计数从无记录开始0（不是赠送余额）；当前月提高/降低受下限约束。成员撤权立即阻新派发，不删除配置/原月counter/reservation；恢复结算原消费仍有效。存储错误/串org/member fail closed。

**原子扣占与锁序。** 新reserve在commercial同tx：image operation fence → member config行 → 原月counter行 → enterprise bucket → 必要debt；创建counter insert-on-conflict后锁行。核intent配置version等于当前version，current month等于intent月且未过期、active、limit-(consumed+reserved)>=price、企业available>=price，才同时增加成员reserved与企业reserved并写原资源reservation/event/audit。配置version变更或月已过不对原prepared intent重报价/切月，返回明确不可派发，由原no_generation关闭；另发用户新操作才用新值。相同已提交reservation的回读不再以新version拒绝。

Finalize复用operation fence → reservation → 原月counter → bucket → debt；配置更新不锁reservation，因此无反向环。commit成员reserved-=quantity/consumed+=quantity；release仅reserved-=quantity；企业balance沿原算法同tx变动。month/member/quantity取原reservation，不取当前配置/当前月。member counter不足/事实冲突整tx失败，UNKNOWN在原月持续reserved；不能靠次月分配释放。资源receipt需包含member、原月、price/config version与intent fingerprint，V3绑定逐项核对；原terminal proof决定结果，caller不能提交charge outcome。

**持久化/范围。** 约2张commercial owner表（limit配置、月counter）及原resource reservation新增member/月/config绑定字段，限额修改复用现operation/audit不可变幂等记录；没有第二余额表。约6–9个后端文件+4–6个定向测试，成员分配页面/现BFF按Figma接真实AI点数月上限约3–5文件，总约13–20新增范围文件。加A–D/F预计总28–45文件，超过30提示线，仍是本次单图生成一个结果/主要PR，明确新增商业事务与成员限额高风险边界，不机械拆PR。实现若实际规模进一步超过生产行数阈值再报告。

**UI与验证。** 实施UI前Writer自己加载Figma skill并读取1636:371/431:4158，不以转述当验收；只当前AI点数月度栏与必要设置，明确UTC月界，原Token不冒名、店铺/数据预算不扩。生产准入须已有live image_agent.write、exact member、月limit与企业余额全满足；current HTTP gate待该链完整才开放。针对性TDD含未配置/成员不足/企业不足0provider，reserve与降limit并发，跨月原key和UNKNOWN，撤权后不新消费而原proof可结算，反向org/member篡改、版本/幂等冲突；用原PG/Temporal设施不建runner。Figma2061:362/3395:389仍非单图生成对照全链，不扩八图菜单。

### F. 当前选定的正式wire与代码落点

已核官方 [gpt-image统一接口](https://qmy27nhsd9.apifox.cn/452409160e0)：POST /v1/api/generate 支持 gpt-image-2.5、images base64、aspectRatio=1024x1024、quality=auto、replyType=json。因此选定复用现grsai exact-byte materialization与no-replay，**不走未验证image[]或双协议fallback**。仅此route显式构造这些字段，不能混用nano的imageSize或现size/response_format；返回running仅UNKNOWN，本片不加accepted-task/query状态。既有其他合法消费者默认poll/retry语义不变。成功payload要在 `downloadGeneratedImages` 前交observer，失败不能丢typed事实；provider输出下载继续原public URL SSRF限制。

- `internal/imageagent/slot_effect_v3.go`、`store/records.go`、`store/slot_effect_v3_{gorm,memory}.go`：generation fact校验、intent/binding/dispatch/outcome/receipt CAS，原row安装与权限只增必要字段，无新表owner。
- `internal/imageagent/temporal/activities_execution_v3.go`、`activities_recovery_v3.go`：消费同attempt状态、禁止replay派发、bounded finalization/原recovery接入。
- `internal/ledger/orgresource` 与 `internal/integration/orgresource`：仅固定image owner receipt合同、现operation fence/closed receipt、复用原reserve/settle账务事务；不改变其他owner的同tx接口。
- `internal/integration/grsai/client.go`、现ProductImage adapter/port：正式json字段、exact bytes、no-poll单次路径、后处理前typed outcome observer；observer错误仍UNKNOWN，不重POST。
- `internal/app/worker/imageagent/dependencies.go`、现current配置：显式price/授权/owner reader装配及fail-closed；runtime grant按实际SQL最小修改，普通启动只Verify，不操作当前保留实例。
- `docs/architecture/project-boundaries.md §3.10`：随准入后的实现更新固定owner例外；本节之外不另开设计链。

预计涉及上述5个既有子系统、两个DB和外部副作用，属架构敏感增量；A–D/F加已批准E预计28–45个范围文件（含定向测试），超30文件风险已报告并在本次高风险检查覆盖，不机械拆PR。必要TDD：价格缺失0dispatch；相同attempt/异quote冲突；两个DB真实独立的Reserve ACK/bind失败、取消vs晚Reserve、dispatch CAS并发/ACK丢失；success后坏图仍commit；UNKNOWN held；Finalize ACK丢失/restart同键不重复余额变化；失权及跨org/member拒绝；与其他owner同tx/普通provider语义回归。复用现PG/Temporal/httptest测试，不新增验收工具；无live调用/部署授权。

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
[D8]: https://github.com/qq550723504/task-processor/issues/487#issuecomment-5842639752
[E1]: https://github.com/qq550723504/task-processor/issues/487#issuecomment-5828964386
[E2]: https://github.com/qq550723504/task-processor/issues/487#issuecomment-5830323285
