# ListingKit Product Design V1

**Date:** 2026-08-27  
**Status:** Approved in product-design discussion; awaiting written-spec review before implementation planning  
**Scope:** ListingKit authenticated product experience in `web/listingkit-ui`  

## 1. Purpose

ListingKit has grown from a task-oriented generation and review tool into a broader commerce operations product. The existing frontend already supports canonical products, multi-platform workspaces, SHEIN review and submission flows, queues, readiness, repair actions, revision history, store-aware operations, and platform-specific workflows. The next product-design step is not to add another feature layer; it is to reorganize those capabilities around stable business objects and operator jobs.

V1 establishes a product model and interaction architecture in which:

- users manage **products**, not task results;
- users publish **listings**, not workflow executions;
- AI presents **issues, recommendations, evidence, and actions**, not a detached chatbot;
- **tasks/workflows remain execution infrastructure** and are exposed only when operational debugging requires them;
- canonical product data and platform-specific data remain explicitly separated.

The product positioning remains:

> ListingKit is an AI commerce workspace that turns source product information into reusable product assets and platform-ready listings, while asking humans to intervene only when judgment is required.

## 2. Product principles

### 2.1 Product is the primary business object

A user should think in the following sequence:

```text
Source Product
    ↓
Canonical Product
    ↓
Platform Draft
    ↓
Listing
    ↓
Published Result
```

The execution layer remains beneath this sequence:

```text
Workflow
  ↓
Task
  ↓
Child Task
  ↓
Queue / Retry / Platform Adapter
```

A product can be regenerated many times without becoming a different product. Tasks create or modify product revisions; they do not define product identity.

### 2.2 Product language must replace execution language

User-facing language should prefer business concepts:

| Internal / engineering language | Product language |
| --- | --- |
| canonical_product | 商品 / 商品资料 |
| task result | AI 处理结果 |
| workspace | 商品工作台 / 编辑审核 |
| platform adaptation | 平台资料 |
| blocking | 必须处理 |
| warning | 建议确认 |
| run_standard_product_temporal | AI 生成商品 |
| run_platform_adapt_temporal | 生成平台资料 |
| task status / child task | 执行记录 / 高级信息 |

Engineering terms may remain visible in advanced debugging views.

### 2.3 AI is an action system, not a chat surface

AI interactions should follow this pattern:

```text
发现问题
  ↓
解释影响
  ↓
提出建议
  ↓
展示依据和置信度
  ↓
用户采用 / 保留 / 修改
  ↓
记录变更
```

The default AI UI is contextual. A generic `Ask AI` box is not a V1 requirement.

### 2.4 Human-in-the-loop should be selective

Technical failures that can recover automatically should remain silent. An issue reaches the operator only when the system cannot safely continue without a human decision or action.

```text
Technical Failure
       ↓
Can auto-recover?
  ├─ yes → retry silently
  └─ no
       ↓
Need human action?
  ├─ no → system incident / admin diagnostics
  └─ yes → Exception Center
```

### 2.5 Canonical and platform data must remain separate

Canonical product data describes the reusable product fact set. Platform draft data describes how the product is represented on a specific commerce platform. Store-specific listing data describes the actual publication instance.

```text
Canonical Product
      ↓
Platform Draft (SHEIN / TEMU / Amazon / ...)
      ↓
Listing (platform + store)
```

Edits must make the target layer explicit.

## 3. Information architecture

### 3.1 Primary navigation

```text
ListingKit

工作台

商品
  商品中心
  商品来源
  POD

上架中心

需要处理

数据

────────────────

设置
  店铺
  商品规则
  AI 设置
  自动化
  账号与订阅

平台管理      (platform/admin roles only)
```

`SHEIN`, `TEMU`, and `Amazon` are primarily filtering dimensions inside the Listing Center, not separate product silos. Platform-specific tools that cannot fit the common listing model may appear as subordinate tools, for example SHEIN campaign enrollment.

### 3.2 Role separation

**Operator** sees the daily commerce workflow:

- Workbench
- Product Center
- Listing Center
- Exception Center
- operational data

**Admin** additionally sees:

- store configuration
- rules
- prompts / AI configuration
- scheduling / automation

**Platform admin** additionally sees:

- tenant/subscription management
- platform-wide account/system administration

Existing role-aware navigation and ZITADEL role checks should be reused rather than replaced.

## 4. Domain model

### 4.1 Product

A Product is the long-lived commerce asset.

Recommended conceptual structure:

```text
Product
├── identity
├── source references
├── canonical data
│   ├── title
│   ├── category
│   ├── images
│   ├── attributes
│   ├── variants / SKUs
│   └── description
├── revisions
├── review state
├── platform drafts
└── listings
```

V1 does not require an immediate database migration. Existing task-result-backed canonical product APIs may continue to populate the product read model while UI semantics move to Product first.

### 4.2 Platform Draft

A Platform Draft is reusable platform-specific product content before store publication.

Typical fields:

- platform title
- platform category
- platform attributes
- platform media selection
- platform SKU/variant projection
- platform validation/readiness
- platform payload version/freshness

### 4.3 Listing

A Listing is a Product + Platform + Store publication instance.

```text
Listing
├── product_id
├── platform
├── store_id
├── platform_draft_id/version
├── price / store-specific values
├── publication status
├── external listing/SPU/product id
└── submission history
```

A single platform draft may support multiple store listings when business rules allow it.

### 4.4 Task / Workflow

Task and Workflow remain execution objects. They may reference Product, Platform Draft, Listing, or a revision. They should not be the default navigation identity of user-facing assets.

Long-term routing should therefore evolve toward:

```text
/products/:productId
/products/:productId/platforms/:platform
/listings/:listingId
```

rather than depending permanently on:

```text
/canonical-products/:taskId
/listing-kits/:taskId/workspace
```

V1 may preserve current URLs while adapting page semantics.

## 5. Shared state models

### 5.1 Product lifecycle state

Expose only a compact user-facing state set:

| State | Meaning |
| --- | --- |
| 待处理 | Human input or confirmation is required |
| 处理中 | AI/background execution is in progress |
| 可上架 | Product data is sufficiently complete for one or more listings |
| 上架中 | A listing submission is running |
| 已上架 | At least one target listing is published |
| 异常 | Processing cannot continue normally and requires intervention |

Internal task/workflow states map into this presentation model.

### 5.2 Review severity

Review is a separate dimension from lifecycle state.

| Severity | Product label | Behavior |
| --- | --- | --- |
| Blocking | 必须处理 | blocks the affected action/submission |
| Warning | 建议确认 | user may continue if policy allows |
| Suggestion | 优化建议 | fully optional |

A Product may be `可上架` while still containing non-blocking warnings.

### 5.3 Listing publication state

Expose a compact platform-independent model:

| State | Meaning |
| --- | --- |
| 待准备 | Platform/store listing data is not ready |
| 待处理 | Human review/fix is needed |
| 可发布 | Listing is ready to submit |
| 发布中 | Submission is running |
| 已发布 | Platform accepted/published the listing |
| 失败 | Submission failed and cannot currently proceed |

SHEIN-specific workflow/submission/queue states should map into this model. Future TEMU/Amazon state machines should map into the same model.

## 6. Workbench / Home

### 6.1 Goal

The authenticated homepage answers:

> What needs my attention now, and what should I continue next?

It should not primarily explain what ListingKit is.

### 6.2 Structure

```text
工作台                                      [导入商品]
今天需要你关注 N 个问题

待审核 / 待修复 / 发布失败 / 运行中

继续工作                         平台状态

需要处理

最近工作
```

### 6.3 Header

Remove the large product-education hero from the authenticated daily workbench. Product education belongs on the public landing page, first-run empty states, or help/onboarding surfaces.

Primary action:

- `导入商品`

Suggested import choices:

- 1688 链接
- 图片 / 商品资料
- POD
- 批量文件

### 6.4 Operational summary

Summary tiles are work queues, not BI metrics:

- 待审核
- 待修复
- 发布失败
- 运行中

Each tile deep-links into a filtered Product/Listing/Exception view.

### 6.5 Continue Work

Reuse existing recent-task selection logic internally, but present a product-oriented card:

- product title/image
- target platform
- current business status
- blocking/warning count
- human-readable next action
- `继续处理`

Task IDs remain hidden unless advanced details are opened.

### 6.6 AI action inbox

A `需要处理` section groups unresolved human interventions, such as:

- products requiring review
- listing submission failures
- stale platform templates
- store authorization expiration

This section is the primary expression of human-in-the-loop behavior.

### 6.7 Remove duplicate navigation cards

The current Quick Tools cards (`新建任务`, `POD`, `标准商品`, `任务列表`) duplicate the global navigation and should be removed or replaced by context-specific actions.

## 7. Product Center

### 7.1 Goal

The Product Center manages long-lived product assets, not one-time AI task outputs.

The user should be able to:

- find a product quickly;
- understand lifecycle and review state;
- see platform readiness at a glance;
- perform bulk AI/platform actions;
- open the Product Workspace.

### 7.2 Table-first layout

Use a dense table/list rather than large product cards so the page remains useful at thousands of products.

Recommended columns:

```text
[select]
商品
商品状态
AI 审核
平台
更新时间
[actions]
```

The product cell carries:

- thumbnail
- title
- brand/category summary
- source label/reference

### 7.3 Work-state tabs

```text
全部
待处理
处理中
可上架
已上架
异常
```

Counts are actionable queue counts, not vanity metrics.

### 7.4 Filters

V1 filters:

- search: product title / SKU / source ID
- source
- category
- platform
- store
- updated time

Internal execution filters such as workflow ID, Temporal state, retry version, or action queue should stay in advanced/admin views.

### 7.5 Platform readiness cell

Show platform state compactly:

```text
SHEIN   ✓
TEMU    ○
Amazon  ⚠
```

Suggested icon/state semantics:

- `○` not generated
- `●` processing
- `✓` ready
- `↗` published
- `⚠` needs attention
- `✕` failed

A platform state is clickable and opens the Product Workspace focused on that platform.

### 7.6 Bulk actions

When products are selected, show a contextual action bar:

```text
已选择 N 个
[AI 检查] [生成平台资料] [上架] [更多]
```

Possible `更多` actions:

- regenerate
- change category
- tag
- export
- delete

A bulk platform action may create many backend tasks; the UI should continue to describe the business outcome rather than task count.

### 7.7 Product empty state

Do not display engineering messages such as `暂无 canonical_product`.

Suggested copy:

> 你的商品中心还是空的。导入第一个商品，ListingKit 会自动整理图片、属性、SKU 和平台所需资料。

Primary action: `导入商品`.

## 8. Product Workspace

### 8.1 Goal

The Product Workspace is the core editing, review, and platform-adaptation surface.

It replaces the mental model of a vertical stack of task/status/review panels with a coherent three-column workspace.

### 8.2 Layout

```text
Header: product identity + state + primary actions

Left rail           Main editor/work area          AI Review
──────────          ─────────────────────          ─────────
商品资料             selected product/platform       health/readiness
  概览               section                        blocking issues
  图片                                              warnings
  基础信息                                          suggestions
  SKU / 规格                                         evidence/actions
  属性
  描述

平台资料
  SHEIN
  TEMU
  Amazon

历史

Bottom sticky action bar
```

### 8.3 Header

Show:

- back to Product Center
- thumbnail/title
- brand/category/source summary
- product lifecycle state
- last saved time
- preview
- context-sensitive primary action (`上架`, `提交 SHEIN`, etc.)

Do not put Task ID, Temporal workflow status, child task state, or raw execution information in the main header.

### 8.4 Left navigation

The left rail is a product structure, not a workflow stepper.

Canonical sections:

- 概览
- 图片
- 基础信息
- SKU / 规格
- 属性
- 描述

Platform sections:

- SHEIN
- TEMU
- Amazon

Each platform may show a readiness/error indicator.

History is available at the bottom or via a drawer.

### 8.5 Canonical editor

Canonical sections edit the reusable product truth. Fields should behave like normal structured commerce editors rather than raw task-result viewers.

Each field may display:

- current value
- confidence/review state
- source/evidence on demand
- AI recommendation on demand or when an issue exists

### 8.6 Platform editor

Selecting a platform clearly changes context to platform-specific data.

Suggested platform sub-sections:

- 资料
- 图片
- 属性
- SKU
- 价格
- 预览

For SHEIN, existing category/attribute/sale-attribute/final-review capabilities should be reorganized into these sections rather than exposed as a long sequence of independent panels.

A lightweight flow indicator may remain:

```text
准备资料 → 校验 → 待提交 → 已发布
```

### 8.7 AI Review panel

The right rail remains contextual and fixed where practical.

It shows:

- product/platform readiness or health score
- count of blocking issues
- count of warnings
- count of suggestions
- issue cards sorted by severity

Issue card example:

```text
必须处理
Material 缺失

影响：SHEIN 无法提交
AI 建议：Cotton
依据：来源属性 + 商品描述
置信度：94%

[采用 Cotton] [查看依据]
```

Clicking an issue should navigate/focus the relevant editor field when possible.

### 8.8 Revision history

History should become a side drawer/timeline rather than a large page-bottom task panel.

Show product language:

- AI changed title
- operator accepted recommendation
- generated SHEIN platform data
- submitted listing

Detailed task/revision IDs remain available in advanced information.

### 8.9 Execution details

Current `TaskStatusPanel`, child retry controls, raw workflow/retry data, and execution diagnostics move under:

```text
⋯ → 查看执行记录
```

Execution history may show business steps first and engineering details second.

Example:

```text
生成标准商品      ✓
生成图片          ✓
生成 SHEIN 商品   ✓
提交 SHEIN        ✕

[重试]
[高级信息]
```

### 8.10 Sticky action bar

Canonical context:

```text
已保存
[AI 检查] [预览] [生成平台资料]
```

Platform context:

```text
SHEIN · 1 个必须处理的问题
[预览] [保存草稿] [提交 SHEIN]
```

Blocking issues disable submission with a clear explanation.

## 9. Listing Center

### 9.1 Goal

The Listing Center manages publication instances across platform + store combinations.

The user should understand:

> Which product is being sold where, and what is its current publication state?

### 9.2 Structure

```text
上架中心                                      [创建上架]

全部 | 待准备 | 可发布 | 发布中 | 已发布 | 失败

Search
平台 | 店铺 | 状态 | 类目 | 更新时间

商品 | 平台 | 店铺 | 状态 | 更新时间
```

Platform tabs/filter:

```text
全部 | SHEIN | TEMU | Amazon
```

### 9.3 Listing row

A row represents:

```text
Product + Platform + Store
```

Show:

- product identity
- platform
- store
- publication state
- relevant external ID after publication
- human-readable blocker/failure summary when applicable

### 9.4 Create Listing flow

```text
选择商品
  ↓
选择平台
  ↓
选择店铺
  ↓
AI 检查
  ↓
生成/确认平台资料
  ↓
创建并发布
```

Before confirmation, summarize the number of Listing instances that will be created.

### 9.5 Listing detail navigation

Opening a Listing should reuse the Product Workspace and focus the relevant platform/store context rather than introducing another full editing application.

## 10. Exception Center

### 10.1 Goal

The Exception Center is the human-in-the-loop queue. It contains only issues that require human attention or an explicit business decision.

### 10.2 Top-level categories

Keep user-facing categories simple:

- 商品问题
- 平台问题
- 账号问题
- 系统问题

Severity filters:

- 必须处理
- 建议确认
- 系统异常

### 10.3 Aggregate repeated issues

Do not show one failed Task per row when many products share the same root cause.

Prefer aggregation:

```text
100 个 SHEIN 商品缺少 Material

AI 可自动处理：86
需要人工确认：14

[自动修复 86 个]
[处理剩余 14 个]
```

This enables high-volume operations.

### 10.4 AI batch repair

Before applying bulk AI fixes, show a preview table:

```text
商品 | 原值 | AI 建议 | 置信度
```

Allow the operator to apply high-confidence fixes in bulk and leave ambiguous cases for manual review.

### 10.5 Manual decision flow

For ambiguous classification/attribute decisions, present ranked options with evidence and a direct decision action.

After a blocking issue is resolved, the execution layer should automatically resume when safe. Manual retry should be reserved for cases where automatic continuation is not appropriate.

### 10.6 Platform freshness

Translate template/freshness internals into business language:

> SHEIN 更新了该类目的属性规则。已有 42 个商品使用旧版本资料。AI 可以重新检查。

Primary action: `检查 42 个商品`.

### 10.7 Store authorization

Represent expired authorization as an operational blocker:

> SHEIN · US-01 需要重新授权。17 个 Listing 暂时无法继续发布。

After successful authorization, queued work should resume automatically where possible.

## 11. Task / Execution Center

Task Center remains available but is no longer a primary operator destination.

Recommended role:

- advanced operations
- administrators
- troubleshooting
- execution audit

Suggested top-level grouping:

```text
需要处理
运行中
失败
已完成
```

Rows should use business names and related product/platform/store context. Raw task IDs remain secondary metadata.

## 12. Existing frontend mapping

V1 should reuse existing frontend/data capabilities instead of rewriting the execution architecture.

### 12.1 Home

Current modules can evolve as follows:

```text
ListingKitHomeHero          → remove from authenticated daily home / reuse in onboarding
ListingKitHomeQuickTools    → remove or replace with context actions
ListingKitHomeRecentWork    → split/reuse into:
                               - Attention Summary
                               - Continue Work
                               - Platform Overview
                               - Action Inbox
                               - Recent Activity
```

Existing recent-task selection, queue summary, blocking/warning, POD, and freshness data should be reused.

### 12.2 Canonical Product pages

Current canonical product list/detail remain a low-risk starting point.

Short-term:

- retain routes such as `/listing-kits/canonical-products`;
- relabel UI as `商品中心`;
- evolve cards toward a table-first Product Center;
- keep current query APIs/read models where sufficient.

Long-term:

- introduce stable Product identity independent of Task identity;
- migrate routing/read models to Product ID.

### 12.3 Workspace

The current `WorkspaceScreen` already contains most required business capabilities. V1 should reorganize those capabilities, not discard them.

Existing capabilities to preserve/reuse include:

- workspace header/status data
- review reasons
- repair actions
- platform cards
- SHEIN flow navigation
- advanced review details
- final review
- readiness/recovery descriptors
- revision history
- child task retry
- standard product execution action
- platform adapt execution action

The primary redesign is composition and information hierarchy.

### 12.4 Task taxonomy

Existing SHEIN work/action queue taxonomy remains useful as internal routing and aggregation metadata. Product UI should map it into human concepts such as:

- generation
- review
- repair
- ready to publish
- publication failure
- authorization
- category
- attributes
- variants
- media
- pricing

## 13. Data flow

### 13.1 Product editing

```text
Source data / existing task result
        ↓
Product read model
        ↓
Product Workspace
        ↓
User edit or AI recommendation acceptance
        ↓
Revision / mutation
        ↓
Revalidation
        ↓
Updated Product + review state
```

### 13.2 Platform generation

```text
Product
  ↓
Generate Platform Draft action
  ↓
Workflow / Task execution
  ↓
Platform Draft
  ↓
Readiness validation
  ↓
ready / warning / blocking
```

### 13.3 Listing publication

```text
Platform Draft + Store
        ↓
Listing
        ↓
Submission workflow
        ↓
Platform adapter
        ↓
Platform response
        ↓
Published / Failed / Needs Attention
```

### 13.4 Exception recovery

```text
Issue detected
   ↓
auto-recover? ─ yes → retry/resume silently
   │
   no
   ↓
human action required?
   ├─ no → system/admin incident
   └─ yes → Exception Center
              ↓
        fix / decision
              ↓
        revalidation
              ↓
        automatic resume when safe
```

## 14. Error-handling product rules

1. Do not expose transient technical failures if automatic retry succeeds.
2. Do not block an entire Product when only one platform/store Listing is affected.
3. Separate Product blocking issues from Platform Draft blocking issues and Listing submission failures.
4. Every visible blocker must include a human-readable cause and a next action.
5. Raw platform/API errors should be available under technical details, not used as the primary user message.
6. Repeated failures sharing one root cause should aggregate into one actionable exception group.
7. Successful resolution should resume execution automatically whenever the operation is deterministic and safe.
8. Destructive or high-impact bulk repair actions should provide a preview before application.

## 15. UX and visual direction

Recommended direction:

- professional operations console;
- table-first where volume matters;
- product workspace inspired by modern structured editors rather than BI dashboards;
- compact status language;
- restrained use of color reserved for state/severity;
- no decorative AI gradients or chat-first framing;
- authenticated pages should feel like a workbench, not a marketing landing page.

The visual system should continue using the existing component primitives and design tokens where possible.

## 16. Accessibility and interaction requirements

- Status cannot rely on color alone; labels/icons must carry meaning.
- All issue/action controls must be keyboard reachable.
- Disabled submission controls must explain the blocking reason.
- Tables must retain useful mobile/narrow-screen behavior through responsive column reduction or row detail expansion.
- Bulk selection must clearly communicate current selection scope.
- Loading/error/empty states must use business language.

## 17. Testing strategy

Implementation should retain the repository's existing TDD approach.

### 17.1 Model/mapping tests

Test mapping from internal execution state to:

- Product lifecycle state
- review severity
- platform readiness
- Listing publication state
- exception category/aggregation

### 17.2 Component tests

Cover:

- Workbench summary links and Continue Work behavior
- Product Center filtering, states, platform links, bulk selection/actions
- Product Workspace section navigation and platform context
- AI issue navigation to affected fields
- blocking submission behavior
- Listing Center filters/status display
- Exception Center aggregation and batch repair preview

### 17.3 Regression coverage

Preserve existing coverage for:

- canonical product mapping
- workspace routing
- SHEIN review/submit behavior
- task/work queue taxonomy
- retry/recovery logic

### 17.4 End-to-end journeys

At minimum, validate these flows:

1. Import source → Product appears → AI processing → Product ready.
2. Product → generate SHEIN draft → resolve warning/blocker → create Listing → submit → published.
3. Bulk products → generate platform data → aggregated issue → batch AI fix → resume.
4. Listing submission fails → Exception Center → repair → resubmit → published.
5. Store authorization expires → affected Listings pause → reauthorize → automatic resume.

## 18. Migration strategy

The redesign should be incremental.

### Phase 1 — Information architecture and language

- reorganize sidebar navigation;
- relabel Standard Product as Product Center;
- hide execution language from primary UI;
- preserve routes/APIs.

### Phase 2 — Workbench and Product Center

- replace authenticated Hero/Quick Tools structure;
- introduce operational workbench;
- redesign canonical product list into table-first Product Center;
- introduce product-facing state mapping.

### Phase 3 — Product Workspace composition

- reorganize current Workspace into header + product rail + main editor + AI Review + sticky actions;
- move TaskStatus/Revision/technical details into drawers/advanced views;
- preserve existing SHEIN actions and routing underneath.

### Phase 4 — Listing Center and Exception Center

- create platform/store Listing read model;
- unify publication state presentation;
- project queue/blocker/failure data into actionable exception groups.

### Phase 5 — Stable Product identity

When backend/domain changes are justified:

- introduce stable Product ID independent of Task;
- attach tasks/revisions/platform drafts/listings to Product;
- migrate routes from task-based identity to product/listing identity.

This phase is intentionally deferred so product UX can improve before a high-risk data migration.

## 19. Non-goals for V1

V1 does not require:

- replacing Temporal or the existing task execution architecture;
- rewriting all current APIs;
- removing task/debug pages;
- implementing a generic AI chat assistant;
- building a full BI/GMV/order dashboard;
- migrating all URLs to Product IDs immediately;
- redesigning every admin screen;
- giving every platform a separate application shell.

## 20. Success criteria

The V1 redesign is successful when an operator can answer the following without understanding Task/Temporal internals:

1. What products do I have?
2. Which products need my attention?
3. What is AI doing now?
4. Which products are ready for each platform?
5. What is published to which store?
6. Why did a listing fail?
7. What can AI fix automatically?
8. What decision does the system need from me?
9. What should I do next?

The architecture is successful when backend workflow/task complexity can continue to grow without forcing corresponding complexity into the operator's primary navigation and mental model.
