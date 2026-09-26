# 1688 单图白底生成：架构与执行交接

日期：2026-09-25；单次编辑及取消默认模型审图更新：2026-09-26。关联：[#487][issue487]；独立验收 [#473][issue473] / [#438][issue438]。

**性质：批准决定与执行边界的有界交接，不是新全局架构、IMPLEMENTATION_READY 签发或验收报告。当前产品路径为一次生成、程序校验、人工确认；运行实现尚由原 Writer 完成。**

原核对基线为 main `b47d6302b99c5229ba89825d48ea9b2823e400af`；文档交接分支基于 `ce39b52b1e458fe0657ddea91ab48ef2cde9edf8`。修订沿同一文件、同一文档分支交原 Writer 归并进主要 PR #498，不向业务分支直接写入、不新增实现者或额外文档合并前置。本地草稿、原事实与历史测试保留；实时 HEAD/CI/进度只在原 Issue/PR 维护。

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

沿#498已有“合同定义包→实现包→注入点→消费者→guard”表修改实际变化。核对source-based输入、adapter、organizationMainSlotExecutor/tools.ProductImageSlotExecutor、worker和批准条件；不把原图塞成已Extract的Subject，不保留本片Reviewer强依赖后注入fake结果。

移除本片未执行的extract/review操作列表、quote、预算、usage/effect成功记录和模型评分必需项；同步现有fingerprint/版本/必要状态与测试。不得将旧运行回执重新解释为新步骤成功。保留其他合法消费者的Extract/Review，不全仓删除能力、不新增双路fallback或策略插件，不扩allowlist。

## 4. 程序校验、预览与批准

复用当前图片校验与资产合同，不另造图像质量系统：返回值必须是可解码的允许格式，满足当前尺寸/大小限制，持久对象完整且绑定正确来源/原操作。坏文件、错误身份、保存失败不能成为可批准资产。

程序校验通过只表示有效候选可展示，不表示商品细节无误。界面展示原图与结果供人工对照；质量采用决定由用户作出。未执行模型审查不得显示“AI审核通过”、自动填分数或写虚假质量审计。人工不采用不触发隐藏重生成，也不抹去已发生用量。

人工批准仍核对当前权限、exact商品/候选和action。completed workflow下批准ACK丢失，按原action/payload从不可变Asset回执核实；不向已关闭Temporal Update重复写，不对不同action或失权用户盲返成功。[D5]

## 5. 计量必须来自真实生成调用

**当前接缝：原候选把canonical ai_tokens主要接在recordedReview。取消模型审图不能只是删除调用后留下无预算的图像生成，也不能继续为不存在的Review扣额度。** 原Writer在#498内做最小必要解耦，不重建结算系统。[D8]

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

继续原Writer和#498，先消费最新决定、归并本文，再成批调整单次source输入、Reviewer依赖、真实生成计量和直接测试。仅新增/变化的授权、计量或恢复边界作增量高风险复核；正常独立代码评审不因取消产品模型Review而取消。未变设计/代码不重复全局审核。

保留既有本地草稿和其他任务资源。用户指定来源、合成身份、CA、实例/卷权限沿原批准；stop/restart不等于destroy。真实来源质量样例和付费provider没有额外授权，不自行扩展。Figma只阻视觉定稿，不阻已批准的非视觉调整。

## 8. #473 / #438验收交接

验收应覆盖真实业务调用，不能为获得token消耗而恢复模型Review。[D8]只取消本片的模型审图与评分门禁，不自动撤销父项其他Must或把未知计量记为完成。

若生成provider没有适用的token事实，#473的canonical ai_tokens项仍有缺口：图像预算/次数证据不能代替。原owner报告可复用的实际生成计量接缝或具体待决点；不为凑验收新增一个AI功能、虚构token或另一套结算平台。未来确要用其他已有真实业务能力提供token证据，由原协调方确认，不把它变成本次隐藏调用。

最终证据分别给出：新流程调用计数与程序校验；真实生成预算/用量及未知边界；PG/Temporal的幂等/恢复；原图/结果预览与显式人工批准；Account/Audit；准确候选CI及独立用户验收。未运行项保持NOT_RUN；取消的模型Review是需求被替代，不标PASS。

旧“三次provider/Review7tokens”及“两次provider”只属于原流程。[E1] 未变化的授权/资产等证据可复用，但不能证明新次数、新计量或新用户链。受控2×2PNG不证明商品质量，不承诺成本/延迟减半。少量获准真实样例只核对当前保真与白底目标，不新建基准平台。

交付留原PR：实际代码/测试与环境、结果及限制、正常启动/页面/保存/启停、剩余阻塞。无新增业务Issue或文档合并前置；未授权合并、关闭Issue、部署、真实/共享数据或付费provider操作。

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
