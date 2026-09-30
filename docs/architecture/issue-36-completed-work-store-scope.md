# #36 已完成本地资料的店铺归属投影

> 状态：**REVIEW_CANDIDATE，尚未 IMPLEMENTATION_READY**（2026-09-30）。本文件是正式业务代码修改前的 Independent Architecture Design Basis；独立评审及 Issue 明确放行前只允许调查与已授权的 test-only 工作。

## 用户结果与依据

有当前企业 Listing 读取权限的用户在 AI 工作台 → 任务中心 → 已完成，能查看一条已提交的 SHEIN 本地资料准备记录所属的真实店铺 ID、当时选择的准备动作、来源商品、时间和原诊断入口。用户不会因 `action=publish` 误以为资料已向 SHEIN 提交或发布。工作范围总览不再把店铺关联记录一律称为“通用业务”。

产品决定以 [#36 的 2026-09-30 更新](https://github.com/qq550723504/task-processor/issues/36) 为准；[#340](https://github.com/qq550723504/task-processor/issues/340) 的 `projection_version: "1"` / `work_scope: "general"` 是当时无店铺字段的历史投影合同，不能用来抹去当前 source 的归属事实。[Store Center 当前应用](store-center-current-application-v1.md)、[当前无 Legacy 迁移产品决定](../product/greenfield-no-legacy-migration.md) 和 [Hard-Cut 规则](../refactoring/legacy-hard-cut-policy.md) 继续适用。Figma file `tg48P46SSXl6TBy9lZwg63` 的当前页面 `31:463`、任务中心 `427:3005`、工作范围 `560:398` 和店铺示例 `560:438` 仅规定入口/信息呈现：其中“通用业务”“一号店”、数量、进度都是示例，不是事实源，也不覆盖领域合同。

两条真实业务流程必须分开：Product 标题 Proposal → 人工 Review → **独立** Apply → Catalog 新版本，事实 owner 是 Product Review/Catalog；Store 绑定的 SHEIN DRAFT-S1 输入 → **本地** Listing record 与离线诊断，之后的远端平台提交是另一项尚未由本页面执行的操作。它们共享任务中心导航，但前者不消费后者的完成记录作为审核前置，后者也不代表标题审核成功。现有浏览器脚本在进入标题审核检查前先检查 completed-work，因其 502 中断；这是验证脚本的顺序和投影缺陷，不能据此推断 Review/Apply 内核失败或建立产品依赖。

本次只修复当前 Listing collection → completed-work BFF/DTO/client → 已完成列表、详情与任务中心范围文案。保留 #36 标题审核和 Apply 现有合同，不重建 Product/Listing 状态机；不增加店铺筛选/统计、店铺名称查询或变更、通用 Task/Agent 表、平台发布、真实数据操作、共享部署。

## 事实 owner、调用链及不变量

| 层 | 已存在的事实或职责 | 本次改变 |
| --- | --- | --- |
| Store Center | 当前店铺名称、生命周期和连接状态的唯一 owner；Store read 需独立 `workbench.store.read` | 不读取、不缓存名称或状态；不修改 Store |
| Listing Record | 创建时验证当前企业的 canonical `store_id` 与 SHEIN 平台，持久化不可变 Input；`action` 只允许 `save_draft` / `publish`；本地创建与离线诊断均不执行平台提交 | 不改 Go 创建、SQL、状态或集合；把既有 metadata 如实传递 |
| Go collection | `GET /api/v1/listing/shein-records` 返回已提交的 `record_id/product_key/snapshot_version/store_id/country/language/action/created_at`；服务校验 Listing read，SQL 按 organization/owner 限定，管理员保留原组织范围和 cursor anchor 检查 | 不扩大读取范围或权限 |
| 同源 BFF | Auth.js/session → 受控 Listing upstream → 严格解析 → 只读投影；原组织断言、header/origin/redirect、错误及撤权 cookie、15 秒总 deadline 和 128 KiB 输入/输出上限 | 将当前真实 source 字段加入严格 schema，投影 v2 |
| Task Center | 已完成页消费 typed client、行/详情/诊断链接；共享范围卡目前写死“通用业务” | 按每条记录显示 Store ID/动作，范围卡陈述当前企业与逐条归属 |

`store_id` 是 Listing record 创建时经过 Store reference 校验并随不可变 Input 提交的**历史归属 ID**。列表读取不重新断言店铺现在存在、启用、连接或归当前名称；因此 UI 文案为“记录所属店铺 ID”，不称“当前已授权/已连接店铺”。即使 Store 后续删除或改名，也不改写记录事实。Listing read 只授予该 Listing metadata 的现有可见性，不能推导 Store read；本次不用 Store 名称。若未来要求名称，需在单独设计中经 Store owner 的 `workbench.store.read`、当前企业/撤权检查后读取，并明确无权、删除、改名和延迟结果的处理，不能用假名或通过 Listing DB 旁读。

创建并持久化 record 是 `completion_basis=local_record_committed` 的唯一依据；`action` 表示生成/验证本地资料所用的动作目标，不是提交状态、远端回执或诊断结果。`publish` 仅显示为“本地发布准备”；`save_draft` 仅显示为“本地草稿准备”。两者都附“未向平台提交/发布；诊断与实际发布是独立步骤”，不推断通过诊断或可发布。当前已完成入口及结果链接均只读，不加新的外部副作用。

## v2 消费合同和页面映射

源 schema 仍用严格对象，必须存在并校验 canonical、非 nil、小写 UUID `store_id` 和 `action ∈ {save_draft,publish}`；不能用宽松 `passthrough` 吞字段，也不能在缺失/未知动作时猜为 `general`。Go record 的 `Input.Validate` 和 collection repository 已做相同核心验证；BFF 再验证 HTTP 信任边界。无效 200 source 保持 `502 INVALID_UPSTREAM_RESPONSE`，不渲染空列表。

同源 `GET /api/workbench/completed-work?source=listing-local-preparation&limit=…&cursor=…` 的成功形状改为 **`projection_version: "2"`**；`coverage: "listing-local-preparation-only"`、分页、source/result 身份和固定诊断 URL 均不变。每个 item 在旧有字段之外必含 `store_id`、原始 `action` 和 **`work_scope: "store"`**。`work_scope` 指记录级 Store 归属，不是一个已实现的全局 Store 筛选器，也不表示当前登录用户有 Store Center 的读取权限。空页仍有 version/coverage，不为零行捏造 Store。示意（仅形状，ID/商品/时间不是演示业务事实）：

```json
{
  "projection_version": "2",
  "coverage": "listing-local-preparation-only",
  "items": [{
    "source_type": "listing-local-preparation",
    "source_kind": "shein-local-record",
    "title": "准备商品上架资料",
    "summary": "本地资料已创建；诊断和发布是后续独立操作",
    "platform": "SHEIN",
    "work_scope": "store",
    "store_id": "11111111-1111-4111-8111-111111111111",
    "action": "publish",
    "completion_basis": "local_record_committed",
    "source_record_id": "22222222-2222-4222-8222-222222222222",
    "product_key": "synthetic-product",
    "snapshot_version": "1",
    "country": "US",
    "language": "en",
    "created_at": "2026-09-30T00:00:00Z",
    "result": {"kind":"shein-diagnostic","href":"/workbench/shein-records/22222222-2222-4222-8222-222222222222/diagnostic"}
  }],
  "next_cursor": null
}
```

`result.href` 继续由经验证的 `source_record_id` 在 BFF 固定生成并严格绑定；不接受 source 提供的任意 URL。Next.js 的 BFF 和 typed client/UI 在同一前端候选中一起切换 v2，不提供 v1 双读/降级/兼容 adapter。Go source 现已提供字段，无 Go 协议版本或数据迁移。`web/listingkit-ui/src/lib/server/completed-work.md` 与 #340 历史正文需标注 v1 已被本设计替代，避免旧样例再次被当作当前授权合同；保留历史交付证据。

列表行明确显示“记录所属店铺 ID：…”、“本地草稿准备/本地发布准备”及“本地资料已创建”；详情重复准确归属、原始动作含义与未远端提交提示，保留来源商品、快照版本、时间和原诊断跳转。共享“工作范围”卡不再声称“通用业务”或装作可点击店铺筛选；显示“当前企业”，说明“已完成记录按条目标注所属店铺；店铺筛选暂未接入”。这也适用于待确认标题流程的共享外壳，不声称该流程有 Store 绑定。Figma `560:398` 的卡片位置/信息层级和 `560:438` 的条目范围表达可沿用，但不得复制假店名、计数、状态或发布进度。

## 权限、失败与回滚

- 保留 `PermissionListingKitAdminRead`、当前 session/expected organization、Go 组织/owner SQL scope、管理员原组织内范围及 cursor anchor。点击诊断仍重新授权；切企业、角色/访问撤销或过期时清除旧页面数据与选择，晚到结果不得跨范围显示。
- 上游非 200 原样传回（含撤权 `Set-Cookie`、状态和 request ID）；无效 payload、缺/错 `store_id/action`、过大或无效 JSON 均显式失败。请求取消、总 deadline、响应上限和 no-store 不退化；不可将错误解释成“当前企业无记录”。
- 本次无写入、事务、幂等键、消息、provider 调用、远端 side effect、恢复 worker 或新持久化事实。多次读取只投影相同可见的已提交记录；现有分页次序不变。
- 前端同一部署切换 v2；若候选需回退，回退前端整体版本而不是在新消费者里保留 v1/general fallback。Go 当前集合无需回滚或数据处理。

## Legacy、实现验证与准入停止点

Legacy decision: **RETIRE** 旧 `general-only` 投影假设、相关旧 DTO/fixture 文案；**EXTRACT** 其中仍正确的已提交本地记录含义、受控只读 BFF/诊断链接、分页与权限行为到当前 v2。Current owner：Listing Record 保持 record/历史 Store ID；Store Center 保持实时店铺名称/状态；completed-work 仅作无事实写入的消费投影。Cutover/deletion condition：v2 BFF、typed client、UI、fixture 与说明在同一候选切换后，不再由当前路径使用 v1/general 测试预期或 fallback。不得重新引入旧 Task-first 产品模型、双事实源或旧数据迁移。

实现者先用现有测试为真实 Go `store_id/action` 200 响应与错误范围/文案写 RED，确认旧 BFF 的 502 和旧错误展示，再作最小修复；不创建新 runner 或专项验收平台。相关检验包含：严格解析/缺失与未知动作拒绝、v2 DTO 与固定诊断 URL、`save_draft`/`publish` 不宣称远端成功、空页/跨页/同店与多店、列表与详情同 ID、共享卡/待确认不假设 Store、非 200/撤权 cookie/取消/超时/跨企业与迟到响应。使用现有隔离 PostgreSQL/合成账号 fixture 与 `product-title-review-ui.mjs`、`title-review-ui-regression.mjs` 两条浏览器脚本检验真实入口和标题审核独立路径；真实 IAM、平台、共享环境和产品试用未运行则保持 `NOT_RUN`。开发自检/CI 不能签发用户验收。

独立 Reviewer 先读 #36 当前决定和上述两个独立领域流程，核对 v2 是否准确表达 Listing 历史归属且不越过 Store read，确认无项目定义的 BLOCKER 后，在**本文件与 #36 均显式记录 `IMPLEMENTATION_READY`**。该门槛前不得改生产 DTO/BFF/UI。最终候选再按准确 HEAD 检查真实 diff 和已运行路径；本文件不批准合并、部署、关 Issue 或真实数据操作。
