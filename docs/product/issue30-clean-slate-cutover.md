# #30 / #307：不保留旧业务数据的 1688 Hard-Cut

Product Decision：**PD-ISSUE30-CLEAN-SLATE-2026-09-05**。
状态：**DECISION_RECORDED / WAITING_FOR_ENVIRONMENT_SCOPE**。
来源：用户在 P2-R 任务中的明确决定（2026-09-05）：

> “旧商品／资产／任务数据不需要保留”。

Refs [#307](https://github.com/qq550723504/task-processor/issues/307)；关联 #30、#304、#301/#303。该产品决定立即替代本任务此前的历史保留执行要求；文档PR记录该决定，不授权真实清理、停机、部署或接线。#307/#30保持开放。

## 决策边界

**不再要求保留或迁移到新系统：**旧 ProductSnapshot/publication/version、旧 ApprovedAsset与旧业务资产引用、旧任务及旧任务结果。取消跨切换重试返回旧publication/version、多对一映射、历史版本合并、旧receipt寻址/保留，以及为此新增Catalog身份返回合同、alias/resolver或迁移器的工作。此前P2-C未提交草稿撤下，不进入本PR。

**默认保护：**Organization/IAM、成员权限、店铺、源账号、browser profile及现有登录态、凭据、套餐、权益、计费及其账本/未决结算证据、实际订单、平台远端状态。共享存储中的其他组织/平台/业务数据亦受保护。旧业务资产与仍服务订单/远端商品的共享文件不可仅凭名字归入清理范围。

“不保留”表示新系统无需携带或读取这些旧业务状态，**不是立即物理删除的要求，更不是全库、整个bucket、Redis或profile目录全删授权**。旧资产停用和物理文件回收分开；无法证明归属的对象不删，也不为此重开历史迁移设计。保护类别优先：混在旧任务行内的计费/外部effect证据，先由对应owner明确保管和终止责任，不随整行盲删。

旧分析仍然是其原假设下的有效证据。#307的固定基线 `50dcc43865b2941c4c3c1d8453add28a0c44cdc9`、身份矩阵与v1→v2 characterization保留为**历史分析 / 被本Product Decision替代的执行方向**，不改称分析错误。其跨切换历史保持断言不再是当前Must或实施blocker。

新系统仍必须满足：Catalog唯一Product事实权威；当前算法不变；新系统内相同显式幂等键同payload/有效lineage重放、不同payload冲突；内容寻址内容变化可形成新键；租户隔离、当前account/store授权和撤销检查；新history在新系统内保持不可变；warnings/lineage在新导入中保留；source images仅为候选，显式批准后才进入ApprovedAsset；旧入口/执行/retry/迟到结果不能给新系统写入或产生新的提交、上传、计费。

## 最小方案与证据范围

推荐**白名单式定向清理获批准的业务行 + 旧执行隔离**，让新路径在该范围内看不到任何旧商品、资产、任务状态；物理对象清理默认延期。

已检查现有配置：main `1b82592e1179b681ed9f6962479468dec5990ef7` 的 [runtime.go](../../internal/app/httpapi/runtime.go) 以 `cfg.Database` 构建Catalog DB；[runtime_support_listingkit.go](../../internal/app/httpapi/runtime_support_listingkit.go) 用同一handle读取ApprovedAsset，[feature_module_builders.go](../../internal/app/httpapi/feature_module_builders.go)也消费Database配置。尚未证明有独立业务范围配置可安全切到空库，不能直接改整个数据库配置而使受保护账号/店铺等消失；因此本轮不推荐“换整库”，不新增通用namespace框架。

以下仅为**需要环境owner确认的最小清单，不是可直接执行的删除清单**。只收集owner、组织/平台、记录ID/关联ID、数量、状态、存储位置及活动执行标识，不再采集重建旧publication所需完整payload/lineage，不解决历史映射。

| 边界 | 真实仓库入口 / 待确认的范围 | 处置要求 |
|---|---|---|
| Catalog | `product_snapshot_heads`、`product_snapshot_versions`；[model](../../internal/integration/persistence/product/catalog/model.go) | tenant字段的真实Organization归属、source平台与全部匹配ProductKey白名单；无平台列时用最小来源标识/owner证据，不能以tenant alone或hash前缀猜范围 |
| ApprovedAsset | `product_approved_assets`、`product_approved_inventory_heads`、`product_approved_inventory_version_heads`、`product_approval_receipts`；[model](../../internal/integration/persistence/product/asset/model.go) | 关联的tenant/product/target/action/asset ID范围一起核对；同action跨范围或包含受保护资产时不能整action清理。批准receipt与金融receipt不是同类 |
| 旧业务任务 | `listing_kit_tasks`；[表名](../../internal/listingkit/task_table_name.go)、[模型](../../internal/listingkit/model_task.go) | 只列本次组织/平台/来源关联task ID及结果/重试引用；表中的计费/外部状态先按保护清单处理，不清空共享表 |
| 图片执行 | [ImageAgent records](../../internal/imageagent/store/records.go)中的 `image_agent_v2_runs/plans/slots/attempts/events/asset_catalog/asset_catalog_manifests/projection_snapshots/projection_commits/slot_external_effects` 和 `image_agent_v3_slot_external_effects` | 仅核对与白名单旧任务/资产相关run；这些共享表不整体纳入删除。先停止执行/回调，external effect/计费证据默认保护；无相关记录就不扩展盘点 |
| 1688任务/缓存 | shared crawler task/result及 `crawler:1688:task-result:tenant:<id>:<task>`、已知可能存在的unscoped keys；证据见[#303 source-account设计](https://github.com/qq550723504/task-processor/blob/e4a3d57d8a6693d3e90349b11caa02d0b7dd0c27/docs/superpowers/specs/2026-09-05-source-account-organization-cutover.md) | 只确认实际backend、匹配task IDs、运行实例与TTL/重试；不清整个Redis，不动共享Amazon通道；不能以等待TTL到期替代worker隔离 |
| durable通道/writer | [ListingKit queue](../../internal/listingkit/temporal/task_queue.go)默认`listingkit-shein-submit-publish`；[ImageAgent queues](../../internal/imageagent/temporal/types.go)的`image-agent-manual`、`image-agent-manual-v3`及配置覆盖；1688本地queue/worker见#303设计 | 环境提供实际namespace/queue、workflow/run IDs、API/worker部署和调度来源；名称只是定位入口，不证明所有消息都可删。共享queue不能整体purge/停掉无关任务 |
| 对象与引用 | [artifact.go](../../internal/imageagent/artifact.go)的`image-agent/public`、[durable.go](../../internal/imageagent/objectstore/durable.go)的`image-agent/staging`；实际bucket/key由匹配asset/effect提供 | 只核对归属明确的完整object keys及共享引用；不删除整个前缀/bucket，不下载旧图重建资产。先阻断旧引用读取与旧执行写入，再考虑另行批准的物理回收 |
| 保护边界 | `source_account`及profile引用；Organization/IAM、成员/店铺/套餐/权益/计费/订单由各owner指定实际存储 | 不用表名前缀猜删除类别；只提供保护清单和操作前后不变性证据，不读秘密、cookies或profile正文 |

实际表存在与否、记录数量、部署版本、共享情况均未知。本轮未连接DSN、Redis、browser、对象存储或部署环境。

## 七步有界切换计划（全部真实操作待另行批准）

1. **确认范围。** 环境负责人提交环境身份、已部署API/worker SHA或镜像、组织/平台、Catalog/asset/task及所有受影响保护数据的owner、实际schema/backend、匹配ID白名单/数量、共享引用和执行ID、清理窗口、操作人。保护对象和数据范围不能混淆；未知项阻断该范围执行。
2. **停止旧写入。** 关闭旧route的新写入口，停止/隔离旧writer、worker、调度和自动retry，确认旧进程无法因重启/扩容重新拿到新写入权限或消费新工作。优先复用既有部署隔离、queue配置和身份权限；共享资源只能按已批准实例/任务范围操作。仅把HTTP入口关掉不够。
3. **终止旧任务、拒绝迟到结果。** 使用既有任务/workflow取消或终止能力，停止对应重试/dispatch；必要时停止持有旧执行的进程并限制其本地提交、对象上传和计费出口。新路径只接受新协议、已登记的新run/当前有效状态的结果，拒绝旧task ID、旧协议、已终止/不存在执行及跨Organization结果，不因收到回调自动创建新记录。新run不复用旧执行身份，旧请求不得自动转发为新导入。若现有边界不足以区分/拒绝，列为后续有界切换PR的实现前提，不能假装仅清数据就完成隔离。
4. **清空获批业务范围。** DBA/各owner依据固定白名单制定并审查定向操作；同DB关联行优先共享事务，先校验范围/预期数量/保护条件，按实际外键依赖处理引用、head、业务行，失败回滚。跨DB/cache/workflow边界没有共享事务时保持旧新写入冻结，逐项确认后才开放；中断后先核对实际结果，不blind retry。禁止TRUNCATE共享表、FLUSHALL或删bucket/profile。新路径不可读取旧缓存/投影/旧ApprovedAsset；物理文件可保持停用不删。
5. **验证边界。** 目标范围旧商品/资产/任务/缓存不可由新路径读取；迟到结果、旧消息重投、worker重启都不能写新Catalog、资产、上传、提交或新增计费。检查保护数据和无关范围未变。缺少任一边界证据则继续冻结，不能恢复旧writer作为fallback。
6. **新路径受控验收。** 单独获准接线/运行后，新导入→新Catalog snapshot→source warning/lineage→显式资产批准→readiness。验证新系统同key重放、不一致冲突、内容新键、跨租户拒绝、account disabled/deleted/revoked和store访问拒绝；未批准source image时readiness应明确未就绪，不能自动批准。无需重放旧publication/version。新import是用户明确发起的新业务，不是旧任务自动重试。
7. **退休旧route/handoff。** 在单独批准的切换PR中删除/停用旧 `/api/v1/product-sourcing/1688/listingkit/tasks` route及handoff/wiring，启用absence guards；以本决策下的验收替换临时gate的解除条件。该docs-only PR不接route、不删门禁、不修改#304/#303代码。

已经发出的外部调用不会随任务记录消失而撤销。先列仅与活动旧执行相关的request/effect/计费关联ID与“未dispatch/在途/已受理/未知”结果；能取消的由既有owner取消，不能取消的阻止本地重投、后续提交和结果导入。保护实际订单/远端平台状态，不擅自删除/撤销远端商品或退费。计费owner用既有幂等/结算能力防止重复扣费，不能删账本令其重扣；已发生或无法阻止的provider费用需环境owner确认处置，不假称成本归零。不能证明旧执行出口被控制时不得启用新写入；不要求旧任务业务成功，也不建立历史任务恢复框架。

执行前必须有定向操作的失败/中止方案；保护数据的备份与恢复证据由环境负责人确认，不能借备份把旧业务恢复到新路径。开放新写后发生问题先停止新入口并保全新数据，禁止自动重启旧worker或恢复旧全库覆盖新数据。代码合并、接线批准、真实清理/切换执行分别授权。

## 两条线的要求差异

| Owner | 取消 | 保留 / 改为 |
|---|---|---|
| #30/#304 Product Sourcing / Catalog | 旧publication迁移、跨cutover原版本replay、历史映射/receipt/stream合并；P2-C返回合同改造 | 新系统内当前算法/幂等/冲突/tenant/HTTP receipt正确；旧执行排除、空业务范围、新导入与显式资产批准/readiness。原characterization标历史证据，不能继续当历史保留blocker；prepared临时gate仍待独立切换验收，不能直接删除 |
| #301/#303 Source Account | “旧1688任务/结果必须迁移或续跑”及仅为此设计的历史引用搬迁 | Organization ownership、disabled/deleted/access/revocation、跨组织隔离、旧job拒绝、新任务结果协议正确；保留现有browser ProfileRef、实际profile路径及登录态/凭据，不创建空profile替代现有登录态。账号ownership迁移不是被取消的商品迁移 |

此差异交由各owner调整后续设计/验收，不代改#303/#304分支、源码或冻结的独立实现。#301对本范围之外的durable assets保护要求不被这份决定取消。#30/#307仍需新的真实cutover证据后才讨论关闭。

## 当前停止点

**DECISION_RECORDED；最小计划已记录，执行状态 WAITING_FOR_ENVIRONMENT_SCOPE。**
仍缺：目标环境与组织/平台、各存储/保护数据owner、实际表/记录/对象/缓存白名单、共享资源分界、活动workflow/worker与外部请求状态、profile保护范围、停写与迟到结果拒绝能力证据、定向操作与新路径接线的独立批准。

后续只先收齐这些范围信息并审查有界操作单，不再重建旧publication。没有执行删除、停机、迁移、部署、接线或真实环境查询；不新增CLI、Saga、namespace框架或兼容层。

Legacy decision: RETIRE（旧业务数据和执行路径）；EXTRACT（新系统仍需要的确定性identity、幂等/冲突、授权、显式资产批准行为）。
Current owner: Product Sourcing / Catalog / Asset；Application协调切换；Source Account/Organization owner保护账号与profile。
Cutover/deletion condition: 范围确认、保护清单、旧执行隔离、新空业务状态与新路径受控验收，真实操作分别批准；旧文件回收可独立延期。
