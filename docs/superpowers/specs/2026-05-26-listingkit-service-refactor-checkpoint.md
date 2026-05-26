# ListingKit Service Refactor Checkpoint

## 背景

`internal/listingkit/service.go` 最初长期扮演了一个“总 service”：

1. 任务生命周期在这里扩张
2. generation/revision/studio/submit 等子域不断往同一对象继续挂方法
3. 后期又叠加了 settings/admin、SHEIN 管理、workflow adapter 之类的新入口

这种结构的问题不是“文件大”本身，而是边界不稳定：

- 新需求默认继续往根 `service` 上加方法
- 子域内部的依赖关系只能靠阅读散落文件来理解
- 构造测试经常只能验证最终行为，难以锁住协作者装配

## 本轮已经完成的主要收敛

### 1. 任务生命周期与提交链路已协作者化

已拆出的核心协作者包括：

- `taskLifecycleService`
- `taskSubmissionService`
- `taskSubmissionRecoveryService`
- `taskSubmissionExecutionService`
- `taskSubmissionStateService`
- `taskDirectSubmissionService`
- `taskTemporalSubmissionAdapter`

当前效果：

- submit 主链不再继续堆在根 `service`
- direct / temporal / recovery / state 的职责已经能分别落在独立协作者上

### 2. revision 子域已统一到单一协作者

已完成的 revision 收敛包括：

- `ApplyTaskRevision`
- `GetTaskRevisionHistory`
- `GetTaskRevisionHistoryDetail`
- `ValidateTaskRevision`

当前都由 [task_revision_service.go](/D:/code/task-processor/internal/listingkit/task_revision_service.go) 承接。

当前效果：

- revision 相关入口不再散落在多个 `service_revision*` 文件里
- 子域边界已经比之前清楚很多

### 3. generation 子域已基本成型

已统一进 `taskGenerationService` 的能力包括：

- task list / queue / review session / preview
- retry
- action execution
- navigation dispatch

当前效果：

- generation 从“多个 service_generation* 文件共享根对象状态”转成了单协作者编排
- generation review 的读、写、导航行为已经能在同一协作者里理解

### 4. studio 子域已拆成 session 与 media 两层

已独立的 studio 协作者包括：

- [task_studio_session_service.go](/D:/code/task-processor/internal/listingkit/task_studio_session_service.go)
- [task_studio_media_service.go](/D:/code/task-processor/internal/listingkit/task_studio_media_service.go)

当前效果：

- `Ensure/Get/Update/Batch` 这组 session 行为不再和 design/image generation 混在一起
- `GenerateStudioDesigns / GenerateStudioProductImages / sanitizeStudioImageInputURLs` 已归到 media 协作者

### 5. settings/admin 已拆成两个自然子域

已形成的 admin 协作者包括：

- [settings_admin_service.go](/D:/code/task-processor/internal/listingkit/settings_admin_service.go)
- [shein_admin_service.go](/D:/code/task-processor/internal/listingkit/shein_admin_service.go)

前者当前负责：

- store profile
- routing settings
- AI client settings
- SHEIN settings

后者当前负责：

- `PreviewSheinPrice`
- `SearchSheinCategories`
- `UpdateSheinFinalDraft`
- `GetSubmissionEvents`
- `ClearSheinResolutionCache`

当前效果：

- settings/admin 不再只是接口层概念，已经有了实际后端协作者边界
- SHEIN 管理入口不再继续零散挂在根 `service`

### 6. 装配面已能反映真实边界

`initializeCollaborators()` 现在已经分成：

- `initializeTaskCollaborators()`
- `initializeAdminCollaborators()`
- `initializeSubmitCollaborators()`
- `initializeTemporalCollaborators()`

并且 [service_wiring_test.go](/D:/code/task-processor/internal/listingkit/service_wiring_test.go) 已经持续跟上协作者初始化护栏。

## 当前结构现状

现在 `listingkit` 的根 `service` 已明显从“全能对象”转向“组合根 + facade”。

已经形成的主要协作者层次：

- task lifecycle
- generation
- revision
- studio session
- studio media
- settings admin
- shein admin
- submit / recovery / execution / state / direct / temporal

这意味着：

1. 继续新增某个子域功能时，已经有比较自然的落点
2. 根 `service` 的职责开始更像 wiring，而不是业务实现承载体

## 仍值得关注但暂时不必继续深拆的点

### 1. SHEIN 运行时辅助函数仍分散

例如：

- `resolveSheinStoreSelection(...)`
- `buildSheinAttributeAPI(...)`
- `newSheinAPIClient(...)`
- `applyDefaultSheinPricing(...)`

这些 helper 还没有完全归到更细的 domain service 里，但它们目前更多是协作者之间复用的基础能力，不是最急的复杂度热点。

### 2. workflow/process 层仍偏厚

例如：

- `ProcessListingKit(...)`
- `runStandardProductWorkflow(...)`
- `runPlatformAdaptation(...)`

这条链本身仍然很重，但和前面已经收好的 CRUD/admin/studio/generation 不同，它牵涉的执行态更复杂，继续拆需要更谨慎的验证策略。

### 3. `service_shein_categories.go` 仍承担 store selection 基础逻辑

虽然 `SearchSheinCategories(...)` 已经迁入 `sheinAdminService`，但 store selection / route match / snapshot selection 这些底层逻辑还集中在同一文件里。它现在更像“运行时 store routing 内核”，后续如果继续拆，应单独作为一个方向处理，而不是顺手继续切。

## 结论

这一轮 `listingkit` 的 service 收敛已经达到一个健康 checkpoint：

- 最大块职责已经拆出了自然协作者
- settings/admin 与 studio 两条原本最容易继续膨胀的线已经收口
- 根 `service` 现在更接近组合根，而不是继续承载业务实现

继续往下做当然还可以，但从这个点开始，收益已经明显从“拔掉复杂度热点”转向“结构精修”。

更合适的策略是：

1. 停下来做盘点与回归验证
2. 或者切去别的热点，例如前端 `web/listingkit-ui` 或 workflow/process 层
