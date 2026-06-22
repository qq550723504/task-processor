# ListingKit 前端重构路线图

## 文档状态

- 状态：Active
- 更新日期：2026-06-22
- 参考基线：`master@67f158d70b642a154d5c5e987447c8bf3cb543c5`
- 适用范围：`web/listingkit-ui`
- 适用对象：前端负责人、全栈研发、QA、代码审查者

## 1. 结论

ListingKit 前端需要重构，但不需要全量重写。

当前技术基础是健康的：

- Next.js 16、React 19 和严格 TypeScript；
- TanStack Query 负责服务端数据读取与缓存；
- Zod 用于部分接口运行时校验；
- Vitest 与 Testing Library 已建立测试基础；
- ESLint 已阻止新的 legacy semantic field 回流；
- Workspace 页面已开始按 data、actions、view props 和 view component 拆分。

当前主要风险集中在 SHEIN Studio 工作台、前端 API 契约、本地与远端状态所有权，以及测试守门不足。重构应采用“小步提取、行为不变、测试先行”的方式，优先降低主链路的修改成本和回归风险。

## 2. 重构目标

本轮前端重构的目标是：

```text
让页面组件只负责组合视图；
让状态、命令、持久化和服务端契约有明确所有权；
让 SHEIN Studio 的生成、审核、任务创建和恢复能力可独立测试；
让后端 DTO 变化通过稳定契约进入前端，而不是由页面猜测；
让关键前端测试成为 CI 的正式发布门禁。
```

最终判断标准：

> 新增一个 Studio 状态、生成动作或任务结果时，不需要继续扩大单个巨型组件、巨型类型文件或巨型测试文件。

## 3. 当前诊断

### 3.1 `SheinStudioWorkbench` 是前端复杂度中心

当前文件：

```text
web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx
```

该组件约 2800 行，同时承担：

- 本地草稿读取和冲突合并；
- 远端批次加载和恢复；
- Studio reducer 装配；
- baseline 检查与 warmup；
- 店铺和订阅状态；
- 设计生成和失败重试；
- task 创建和结果投影；
- 批次队列和恢复；
- 路由与页面步骤导航；
- 大量页面派生状态；
- 最终 UI 组合。

虽然部分逻辑已经提取到 hooks、actions、workspace 和 model 文件，但根组件仍拥有过多状态连接和业务编排责任。

### 3.2 Workbench 状态边界过宽

当前 reducer state 同时包含：

- 表单草稿；
- 当前选择；
- 生成状态；
- task 创建状态；
- 服务端 batch detail；
- 本地持久化信息；
- 批次队列；
- 页面消息和临时 UI 状态。

这使得一个字段变化可能触发：

```text
active group 同步
  -> local snapshot 更新
  -> remote autosave
  -> 页面派生状态重算
  -> 子组件 props 变化
```

当前 reducer 可以继续保留，但应逐步按能力拆分状态所有权，而不是继续增加新的根级字段。

### 3.3 组件和 hook 参数扩散

以下对象已经出现明显的参数扩散：

- `useSheinStudioDesignActions`；
- `SheinStudioGenerationPanel`；
- Workbench 内部的 setter 聚合；
- task creation 与 batch generation context。

当前父子组件传递的更像是“整个工作台状态”，而不是一个边界清晰的 feature model。

### 3.4 测试存在，但没有进入 CI 正式门禁

前端已经定义：

```text
npm test -> vitest run
```

当前 GitHub Actions 的 frontend job 只执行：

```text
npm ci
npm run lint
npm run typecheck
npm run build
```

尚未执行 `npm test`。

同时：

```text
shein-studio-workbench.test.tsx
```

已经接近 5000 行，并集中 mock 路由、React Query、批次、baseline、生成、任务创建和大量子组件。测试覆盖能力较强，但维护边界与生产组件一样过大。

### 3.5 API 契约仍以手写兼容为主

`src/lib/api` 下有大量手写请求模块。以 `shein-studio-batches.ts` 为例，一个文件同时承担：

- Zod schema；
- snake_case / camelCase 双字段兼容；
- DTO 到前端模型映射；
- 状态标准化；
- 请求方法；
- legacy fallback。

该方式能够兼容历史接口，但随着后端持续收缩 ListingKit facade，前端 DTO 容易与真实后端契约漂移。

### 3.6 异步任务轮询存在重复机制

`apiAsyncRequest` 与 `apiResumeAsyncJob` 分别维护相似的：

- polling；
- JSON 解析；
- timeout；
- error normalization；
- job failure handling。

旧 job 返回 404 后的 restart 路径还需要保证 `onJobStarted`、session id、signal 和 trace context 不丢失。

### 3.7 本地草稿与远端批次存在隐含双事实源

当前工作台同时维护：

```text
localStorage snapshot
远端 draft / batch
hydrated batch detail
当前 reducer state
```

加载时会根据时间和非空字段合并本地与远端数据。该机制有恢复价值，但状态语义尚未显式化，容易形成“本地看起来已保存，远端并未保存”或“远端更新被本地旧字段覆盖”的风险。

### 3.8 Workspace 方向正确，但仍有平台规则投影

`WorkspaceScreen` 已经按以下职责拆分：

```text
useWorkspaceData
useSheinWorkspaceActions
useWorkspaceNavigationActions
view props builders
views
```

该区域不需要整体重写。

但 `useWorkspaceData` 仍维护部分 SHEIN blocker key、操作文案和流程步骤映射。随着后端 readiness taxonomy 和 suggested action 完善，前端应逐步减少重复规则，只保留展示投影和兼容 fallback。

## 4. 非目标

本轮不做：

- 不重写整个 ListingKit UI；
- 不更换 Next.js、React Query、Vitest 或 Tailwind；
- 不为了目录整齐进行大规模移动；
- 不一次性引入 Redux、Zustand 或其他全局状态库；
- 不同时重构所有平台页面；
- 不把 UI 重构和产品视觉改版放进同一个 PR；
- 不在没有测试保护时删除 legacy draft fallback；
- 不一次性生成并替换所有 API client。

## 5. 目标前端结构

目标结构是方向，不是一次性迁移清单：

```text
web/listingkit-ui/src/
  app/
    ...                       Next.js route 与 server boundary

  features/
    shein-studio/
      api/
        contract.ts           后端 DTO / generated type adapter
        schemas.ts            必要的运行时校验
        mapper.ts             DTO -> domain model
        queries.ts            query keys 与 query functions
        mutations.ts          command / mutation functions
      model/
        types.ts              feature domain model
        state.ts              reducer / state machine
        selectors.ts          纯派生函数
        projections.ts        batch/detail -> view model
      controllers/
        use-batch-hydration-controller.ts
        use-draft-persistence-controller.ts
        use-generation-controller.ts
        use-task-creation-controller.ts
        use-batch-queue-controller.ts
      components/
        workbench-shell.tsx
        generation-panel.tsx
        review-panel.tsx
        task-results-panel.tsx
      persistence/
        local-draft-cache.ts
        conflict-policy.ts

    workspace/
      model/
      controllers/
      components/

  lib/
    api/
      client.ts               通用 HTTP transport
      async-job.ts            通用异步任务 start/poll/resume
      generated/              逐步引入的生成契约
    query/
    types/
    listingkit/
```

现有文件不要求立即全部移动到 `features/`。优先完成所有权和依赖方向，再决定物理目录迁移。

## 6. 前端边界规则

### 6.1 Page / Shell

允许：

- 读取 route params 和 search params；
- 组合 controller；
- 选择 loading / error / main view；
- 组合 feature view。

禁止新增：

- API payload 兼容；
- localStorage 解析；
- 大段 blocker taxonomy；
- task 状态机；
- 跨多个领域的字段归一。

### 6.2 Controller

拥有：

- feature use case；
- query / mutation 协调；
- 用户操作到 domain command 的转换；
- toast、navigation 和 retry 编排；
- 对 view 暴露稳定 model 和 actions。

Controller 不应直接包含大段 JSX。

### 6.3 Model

拥有：

- feature state；
- reducer；
- selector；
- projection；
- 状态转换；
- 可独立测试的业务判断。

Model 不应依赖浏览器对象、React component 或具体 HTTP transport。

### 6.4 API Adapter

拥有：

- 后端 DTO；
- Zod runtime boundary；
- snake_case 到前端 domain model 的转换；
- error normalization；
- generated type 兼容。

页面和组件不得直接读取 legacy DTO 字段。

### 6.5 Persistence

本地持久化只作为恢复缓存，不是远端 batch 的第二业务事实源。

允许：

- 保存未同步草稿；
- 浏览器刷新恢复；
- 保存最后一次已知 batch id；
- 标记本地与远端更新时间。

必须显式区分：

```text
local_only
remote_only
local_newer
remote_newer
conflict
saving
save_failed
synchronized
```

### 6.6 View Component

允许：

- 渲染；
- 局部交互；
- 受控表单事件；
- 调用语义化 action。

禁止：

- 自行拼接 API 请求；
- 自行推断后端状态；
- 自行维护跨步骤业务事实；
- 直接处理 legacy DTO。

## 7. 重构工作流

## Stream A：测试与 CI 守门

目标：先建立所有后续重构的安全网。

需要完成：

```text
frontend CI 增加 npm test；
记录 lint / typecheck / test / build baseline；
识别不稳定测试；
建立关键 Studio 主链路测试清单；
把 giant workbench test 按能力拆分。
```

测试层级：

```text
纯函数测试
  -> reducer / selector / mapper / conflict policy

controller 测试
  -> query/mutation 协调、错误和重试

component 测试
  -> 关键交互和可见状态

少量 integration 测试
  -> hydrate -> generate -> review -> create tasks
```

退出条件：

- frontend CI 正式执行 `npm test`；
- 主链路重构 PR 无法绕过测试；
- 单个测试文件不再继续无限增长。

## Stream B：异步任务客户端收口

目标：通用异步任务机制只保留一套。

目标 API：

```ts
startAsyncJob(input)
pollAsyncJob(jobId, options)
resumeAsyncJob(jobId, options)
resumeOrRestartAsyncJob(input, options)
```

必须保留：

- AbortSignal；
- timeout；
- retry policy；
- session id；
- trace context；
- `onJobStarted`；
- resumed job 404 后安全 restart；
- backend error payload。

退出条件：

- `apiAsyncRequest` 与 `apiResumeAsyncJob` 共用 polling core；
- restart 不丢失调用上下文；
- polling core 有独立测试。

## Stream C：Workbench Shell 收缩

目标：`SheinStudioWorkbench` 只保留页面编排。

建议先提取：

```text
useBatchHydrationController
useDraftPersistenceController
useBatchQueueController
```

再提取：

```text
useGenerationController
useTaskCreationController
```

最终 shell 只负责：

```text
读取 props
初始化 feature controllers
选择页面 mode
把 view model 和 actions 传给 view
```

退出条件：

- shell 不再直接处理 localStorage；
- shell 不再拼接 generation / task creation API；
- shell 不再拥有几十个 setter；
- 关键行为测试迁到 controller/model 层。

## Stream D：状态所有权与持久化

目标：区分草稿、服务端事实和临时 UI 状态。

建议状态切片：

```text
draft
  prompt
  image settings
  store
  selections

generation
  status
  jobs
  designs
  warnings

taskCreation
  status
  created/reused/rejected/failed

batch
  active batch
  hydrated detail
  baseline status

queue
  mode
  selected ids
  cursor
  resume state

ui
  active step
  temporary dialogs
  local messages
```

迁移原则：

```text
服务端返回的 batch/item/design/task 是远端事实；
本地 reducer 可持有投影，但不能成为新事实源；
localStorage 只缓存未同步草稿和恢复上下文；
所有 merge 必须由纯 conflict policy 完成。
```

退出条件：

- local/remote merge 有明确纯函数和测试；
- UI message 不再污染持久化 domain state；
- task 创建结果只从 durable batch detail 或创建响应投影。

## Stream E：组件契约收口

目标：减少 props drilling 和 setter 扩散。

推荐模式：

```ts
type GenerationFormModel = { ... };
type GenerationStatusModel = { ... };
type GenerationActions = { ... };

<GenerationPanel
  form={form}
  status={status}
  actions={actions}
/>
```

要求：

- 子组件接收稳定 feature model；
- action 名称表达业务意图，而不是 `setField`；
- view component 不知道 reducer 字段名；
- 避免用 Context 隐藏无边界的全局状态。

退出条件：

- GenerationPanel 不再接收大量独立 setter；
- DesignActions 不再接收完整 workbench state；
- view props builder 可独立测试。

## Stream F：API 契约与类型

目标：前端不再猜测后端响应结构。

分阶段执行：

```text
1. 先为 Studio batch/detail/task creation 固化 OpenAPI 或稳定 schema；
2. 生成原始 DTO type；
3. 保留 mapper 转换为前端 domain model；
4. 删除重复手写 DTO type；
5. 再扩展到 submission、workspace 和 settings。
```

推荐边界：

```text
generated DTO
  -> runtime schema（只在不可信边界）
  -> mapper
  -> domain model
  -> view model
```

禁止：

- component 直接使用 generated DTO；
- 在多个文件重复兼容 snake_case/camelCase；
- generated type 与 domain model 混为一体；
- 为了生成类型一次性修改所有接口。

退出条件：

- Studio 核心接口有单一 DTO 定义；
- mapper 有 contract tests；
- legacy alias 只存在于兼容 adapter。

## Stream G：Workspace 前端规则收口

目标：前端消费后端 readiness taxonomy 和 suggested action，不重复拥有平台规则。

执行方向：

```text
优先使用后端返回的：
  blocker key
  taxonomy
  title / summary
  suggested action
  navigation target

前端仅负责：
  展示顺序
  UI tone
  route projection
  legacy fallback
```

退出条件：

- 新的 SHEIN blocker 不需要在多个前端 switch 中重复登记；
- fallback 映射集中在纯函数文件；
- WorkspaceScreen 保持薄编排层。

## 8. 分阶段路线

## Phase 0：基线和安全网

交付：

- CI 增加 `npm test`；
- 记录前端测试时长和失败 baseline；
- 为 async polling restart 增加测试；
- 为 local/remote conflict policy 建立测试夹具；
- 标记 giant component/test 文件基线。

退出条件：

```text
npm run lint
npm run typecheck
npm test
npm run build
```

全部进入 frontend CI。

## Phase 1：低风险基础提取

交付：

- 抽取 async-job polling core；
- 抽取 local draft cache；
- 抽取 local/remote conflict policy；
- 抽取稳定 view model builders；
- 不改变页面结构和用户行为。

退出条件：

- 生产组件只调用新 facade；
- 老实现无调用后删除；
- 行为测试保持通过。

## Phase 2：Workbench Controller 化

交付：

- hydration controller；
- persistence controller；
- queue controller；
- generation controller；
- task creation controller；
- workbench shell 收缩。

执行规则：

- 每个 PR 只提取一个 controller；
- 不同时重命名大量文件；
- 不在提取 PR 中修改产品行为；
- 先委托新实现，再删除旧代码。

退出条件：

- `SheinStudioWorkbench` 主要是组合代码；
- controller 可独立测试；
- 子组件不再依赖根 reducer 的字段名。

## Phase 3：API Contract

交付：

- Studio batch/detail/task creation 契约；
- generated DTO 或稳定 contract schema；
- mapper 和 mapper tests；
- legacy alias adapter；
- 手写 API 类型开始减少。

退出条件：

- 后端 DTO 修改需要同步 contract；
- 前端不再在页面内猜测字段；
- CI 可发现关键契约漂移。

## Phase 4：Workspace 和跨平台边界

交付：

- readiness/action projection 收口；
- SHEIN 平台 fallback 集中；
- Workspace data model 与 view model 分离；
- 为后续 TEMU/Amazon 复用通用 workspace contract 做准备。

退出条件：

- WorkspaceScreen 保持薄层；
- 新平台规则不进入通用组件；
- 通用 view 与平台 adapter 边界清晰。

## 9. 多 Agent 并行方案

可以并行，但每个 agent 必须拥有独立文件边界和分支。

推荐第一轮：

```text
Agent A
  Stream A
  CI npm test + test baseline

Agent B
  Stream B
  async-job polling core + tests

Agent C
  Stream F
  Studio batch API contract inventory，不改 Workbench
```

第一轮合并后：

```text
Agent A
  hydration controller

Agent B
  persistence + conflict policy

Agent C
  queue controller
```

最后再并行：

```text
Agent A
  generation controller

Agent B
  task creation controller

Agent C
  giant test 拆分与 integration fixtures
```

冲突控制：

- 同一时刻只允许一个 agent 修改 `shein-studio-workbench.tsx` 主体；
- 其他 agent 先新增目标模块和测试，由 owner PR 完成接线；
- 每合一个 controller PR，其他分支同步最新 `master`；
- 不允许多个 agent 同时改 giant workbench test；
- API contract agent 不直接修改 UI 行为。

## 10. PR 规则

每个前端重构 PR 必须说明：

```text
Problem
Ownership before
Ownership after
Behavior change: yes/no
State source of truth
Compatibility path
Tests
Rollback
Follow-up deletion
```

并满足：

1. 一个 PR 只处理一个所有权问题；
2. 纯提取 PR 不修改产品行为；
3. 不同时做依赖升级；
4. 新增 controller 必须有测试；
5. 新增 mapper 必须有 contract fixture；
6. 删除 legacy fallback 前必须提供真实数据迁移证据；
7. 新增 blocker/action 优先来自后端 contract；
8. 更新本路线图的执行记录。

## 11. 停止条件

以下重构不应执行：

- 只是为了减少文件行数；
- 拆分后所有模块仍共享同一个无边界 state；
- 用 Context 隐藏 props，而没有明确 ownership；
- 为未来假设引入复杂状态机框架；
- 同时改变生成、任务创建和持久化行为；
- 没有测试就删除 local draft recovery；
- 一次性替换全部 API client；
- 视觉改版掩盖业务行为变化。

出现以下信号时，应先切边界再继续加功能：

- 新功能需要向 Workbench 增加多个 state 和 setter；
- 子组件 props 再次显著增长；
- 同一后端字段在多个 mapper 重复兼容；
- localStorage 与远端 batch 状态难以解释；
- 一个测试需要 mock 十几个无关模块；
- 一个小改动必须同时修改 Workbench、hooks、actions、types 和 giant test。

## 12. 度量

### 12.1 结构指标

- `shein-studio-workbench.tsx` 行数；
- Workbench 根级 state 字段数；
- 最大组件 props 数；
- 最大测试文件行数；
- `src/lib/types/shein-studio.ts` 行数；
- 手写 DTO / mapper 数量；
- legacy alias 出现位置数量。

### 12.2 质量指标

- frontend CI 测试通过率；
- flaky test 数量；
- Studio 主链路回归数量；
- local/remote draft 冲突数量；
- 404 resume/restart 失败数量；
- 空错误和 unknown 状态数量。

### 12.3 工程效率指标

- 新 Studio 能力平均触碰文件数；
- 修改一个生成动作所需测试启动成本；
- 后端 DTO 变化到前端编译失败的发现时间；
- Workbench PR 平均冲突数；
- 单个 PR 的 mock 数量。

## 13. 近期执行顺序

```text
1. frontend CI 增加 npm test
2. 固化 lint/typecheck/test/build baseline
3. 抽取 async-job polling core
4. 修复 resume 404 restart 上下文传递
5. 抽取 local draft cache
6. 抽取 local/remote conflict policy
7. 拆 hydration controller
8. 拆 persistence controller
9. 拆 queue controller
10. 拆 generation controller
11. 拆 task creation controller
12. 收缩 GenerationPanel props
13. 拆分 giant workbench test
14. 固化 Studio batch API contract
15. 拆分 shein-studio types
16. 收口 Workspace readiness/action projection
```

## 14. 第一批建议 PR

### PR 1：前端测试进入 CI

```text
范围：.github/workflows/ci.yml
行为变化：无产品行为变化
验证：npm test
回滚：删除单个 CI step
```

### PR 2：统一 async job polling

```text
范围：src/lib/api/client.ts + 新 async-job 模块
行为变化：应保持一致；补齐 restart 上下文
验证：poll success/failure/timeout/404 restart/abort
回滚：保留旧 facade 委托路径
```

### PR 3：本地草稿 cache 与 conflict policy

```text
范围：shein-studio-workbench-hooks.ts + 新 persistence 模块
行为变化：无；先保持当前 merge 规则
验证：local only/remote only/local newer/remote newer/invalid snapshot
回滚：旧 hook facade
```

### PR 4：Hydration Controller

```text
范围：Workbench load/hydrate/dedicated batch 逻辑
行为变化：无
验证：初次加载、刷新、batch 不存在、local snapshot、tenant batch
回滚：controller facade 委托
```

### PR 5：Queue Controller

```text
范围：批次队列选择、游标、恢复和退出
行为变化：无
验证：start/advance/skip/resume/exit
回滚：旧 handler facade
```

## 15. 完成定义

前端重构不是以“所有文件都进入 features 目录”为完成标准，而是以以下结果为准：

```text
SheinStudioWorkbench 主要负责组合 controller 和 view；
生成、任务创建、队列和持久化可以独立测试；
本地草稿与远端 batch 的事实源和冲突规则可解释；
通用异步任务轮询只有一套实现；
Studio 核心 API 有稳定契约和 mapper；
前端测试是 CI 正式门禁；
Workspace 优先消费后端 readiness/action contract；
新增能力不再扩大巨型组件、巨型类型和巨型测试。
```

## 16. 相关文档

- `docs/refactoring/listingkit-refactoring-roadmap.md`
- `docs/refactoring/project-wide-refactoring-plan.md`
- `docs/architecture/project-boundaries.md`
- `docs/product/listingkit-project-goals.md`
- `docs/superpowers/plans/2026-06-04-shein-studio-workbench-persistence-cleanup.md`
- `docs/superpowers/plans/2026-06-20-listingkit-sds-batch-production-closure.md`
- `docs/development/listingkit-semantic-field-cleanup-inventory.md`
