# ListingKit Semantic Field Cleanup Inventory

## 目的

这份清单用于回答一个非常具体的问题：

现在仓库里还保留旧字段名的地方，哪些是必须保留的兼容面，哪些只是可以继续清理的残余点。

这里的“旧字段名”主要指：

- `sds_sync`
- `request_draft`
- `preview_product`
- `submission`
- `final_draft`

## 结论先行

当前剩余引用大致分成两类：

1. **必须保留兼容**
   这些地方要么是 JSON 协议、要么是 readiness key、要么是历史工作台/editor path，短期不应该硬删。

2. **可以继续清理**
   这些地方本质上只是错误文案、提示语、前端 helper 接口名或内部说明，后续可以逐步替换成新语义。

## A 类：必须保留兼容

### 1. JSON 输出兼容字段

这些字段是当前对外协议双轨兼容的基础，短期必须保留：

- [D:\code\task-processor\internal\publishing\shein\model.go](D:/code/task-processor/internal/publishing/shein/model.go:47)
- [D:\code\task-processor\internal\listingkit\model_result.go](D:/code/task-processor/internal/listingkit/model_result.go:40)
- [D:\code\task-processor\internal\listingkit\preview_model.go](D:/code/task-processor/internal/listingkit/preview_model.go:140)
- [D:\code\task-processor\internal\listingkit\export_model.go](D:/code/task-processor/internal/listingkit/export_model.go:69)
- [D:\code\task-processor\web\listingkit-ui\src\lib\types\listingkit\tasks.ts](D:/code/task-processor/web/listingkit-ui/src/lib/types/listingkit/tasks.ts:91)
- [D:\code\task-processor\web\listingkit-ui\src\lib\types\listingkit\shein.ts](D:/code/task-processor/web/listingkit-ui/src/lib/types/listingkit/shein.ts:392)

原因：

- 外部调用方还在迁移窗口中
- 历史任务 JSON 仍可能只带旧字段
- 预览、导出、任务详情都还需要兼容旧数据反序列化

### 2. 语义归一化 helper 的 fallback 逻辑

这些地方虽然提到了旧字段，但它们正是兼容策略本身，不应该清理掉：

- [D:\code\task-processor\internal\publishing\shein\semantic_fields.go](D:/code/task-processor/internal/publishing/shein/semantic_fields.go:7)
- [D:\code\task-processor\internal\listingkit\semantic_fields.go](D:/code/task-processor/internal/listingkit/semantic_fields.go:3)
- [D:\code\task-processor\internal\listingkit\preview_export_semantic_fields.go](D:/code/task-processor/internal/listingkit/preview_export_semantic_fields.go:3)
- [D:\code\task-processor\web\listingkit-ui\src\lib\listingkit\semantic-fields.ts](D:/code/task-processor/web/listingkit-ui/src/lib/listingkit/semantic-fields.ts:1)

原因：

- 这些文件就是“新字段优先、旧字段兜底”的实现边界

### 3. readiness key / workspace action key / revision path

这些旧名字现在不只是字段名，它们已经变成了 UI 和工作流约定 key，短期不能直接改：

- [D:\code\task-processor\internal\listingkit\shein_submit_readiness.go](D:/code/task-processor/internal/listingkit/shein_submit_readiness.go:103)
- [D:\code\task-processor\internal\listingkit\task_contract.go](D:/code/task-processor/internal/listingkit/task_contract.go:76)
- [D:\code\task-processor\internal\listingkit\task_state_support.go](D:/code/task-processor/internal/listingkit/task_state_support.go:112)
- [D:\code\task-processor\internal\workspace\shein\readiness_guidance.go](D:/code/task-processor/internal/workspace/shein/readiness_guidance.go:85)
- [D:\code\task-processor\internal\workspace\shein\readiness.go](D:/code/task-processor/internal/workspace/shein/readiness.go:191)
- [D:\code\task-processor\internal\workspace\shein\editor_effects.go](D:/code/task-processor/internal/workspace/shein/editor_effects.go:8)
- [D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein\shein-workspace-actions.ts](D:/code/task-processor/web/listingkit-ui/src/components/listingkit/shein/shein-workspace-actions.ts:52)
- [D:\code\task-processor\web\listingkit-ui\src\components\listingkit\workspace\use-workspace-data.ts](D:/code/task-processor/web/listingkit-ui/src/components/listingkit/workspace/use-workspace-data.ts:179)

原因：

- 当前 `request_draft` / `preview_product` 在这里表示“编辑区块 key / 阻断项 key / 导航焦点 key”
- 它们不再单纯等于后端 struct 字段名
- 如果要改，需要单独做一轮 workspace key 协议迁移

### 4. submission 领域 package / route / 概念名

这些地方出现 `submission`，大多不是旧字段，而是业务领域术语，应保留：

- [D:\code\task-processor\internal\listingkit\submission\state.go](D:/code/task-processor/internal/listingkit/submission/state.go:1)
- [D:\code\task-processor\internal\listingkit\shein_submit_state.go](D:/code/task-processor/internal/listingkit/shein_submit_state.go:1)
- [D:\code\task-processor\internal\listingkit\httpapi\routes.go](D:/code/task-processor/internal/listingkit/httpapi/routes.go:403)
- [D:\code\task-processor\web\listingkit-ui\src\lib\shein-studio\shein-submission-display.ts](D:/code/task-processor/web/listingkit-ui/src/lib/shein-studio/shein-submission-display.ts:1)

原因：

- 这里的 `submission` 指的是“提交流程/提交状态”领域，不是旧 JSON 字段名本身

## B 类：可以继续清理

### 1. 错误提示和阻断文案还写着旧字段名

这些地方可以后续逐步换成新语义描述：

- [D:\code\task-processor\internal\listingkit\service_submit.go](D:/code/task-processor/internal/listingkit/service_submit.go:71)
- [D:\code\task-processor\internal\listingkit\service_submit_temporal_adapter.go](D:/code/task-processor/internal/listingkit/service_submit_temporal_adapter.go:88)
- [D:\code\task-processor\internal\listingkit\task_temporal_submission_adapter.go](D:/code/task-processor/internal/listingkit/task_temporal_submission_adapter.go:79)
- [D:\code\task-processor\internal\listingkit\shein_submit_images.go](D:/code/task-processor/internal/listingkit/shein_submit_images.go:132)

建议：

- `shein preview_product is not available`
  可改成
  `shein preview payload is not available`
- `SHEIN preview_product has no image_info URLs to submit`
  可改成
  `SHEIN preview payload has no image_info URLs to submit`

### 2. 内部校验 field path 仍使用旧路径名

这些路径更多是调试/前端定位信息，后续可以评估是否升级：

- [D:\code\task-processor\internal\listingkit\shein_build_validation.go](D:/code/task-processor/internal/listingkit/shein_build_validation.go:50)
- [D:\code\task-processor\internal\listingkit\shein_submit_readiness.go](D:/code/task-processor/internal/listingkit/shein_submit_readiness.go:76)

注意：

- 这里不能直接替换
- 因为前端 repair center、readiness 高亮、editor focus 可能还依赖这些路径字符串
- 这类要和 workspace key 一起迁

### 3. workspace/editor 文案描述仍引用旧名字

这些主要是面向用户或开发者的文案，不影响协议本身，但可以优化：

- [D:\code\task-processor\internal\workspace\shein\repair.go](D:/code/task-processor/internal/workspace/shein/repair.go:481)
- [D:\code\task-processor\internal\workspace\shein\readiness_guidance.go](D:/code/task-processor/internal/workspace/shein/readiness_guidance.go:90)
- [D:\code\task-processor\web\listingkit-ui\src\lib\shein-studio\shein-customer-issues.ts](D:/code/task-processor/web/listingkit-ui/src/lib/shein-studio/shein-customer-issues.ts:88)

建议：

- 对用户展示层可继续保留 `preview_product` 一段时间
- 对开发者文案、日志、注释，优先切成 `preview payload`

### 4. 前端个别 helper / prop 名还叫 submission

这类更多是局部变量和 prop 名，还不算问题，但如果要彻底统一，可以继续收口：

- [D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein\shein-submit-readiness-panel.tsx](D:/code/task-processor/web/listingkit-ui/src/components/listingkit/shein/shein-submit-readiness-panel.tsx:47)
- [D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein\shein-submit-readiness-submission-sections.tsx](D:/code/task-processor/web/listingkit-ui/src/components/listingkit/shein/shein-submit-readiness-submission-sections.tsx:113)
- [D:\code\task-processor\web\listingkit-ui\src\components\listingkit\shein\shein-submit-readiness-helpers.ts](D:/code/task-processor/web/listingkit-ui/src/components/listingkit/shein/shein-submit-readiness-helpers.ts:79)

说明：

- 这里的 `submission` 变量已经在消费 `submission_state`
- 只是变量名没进一步改成 `submissionState`

### 5. task list 的 `sds_sync_status`

这个字段目前还是列表筛选/摘要字段，不是完整 SDS 结果结构：

- [D:\code\task-processor\internal\listingkit\model_task.go](D:/code/task-processor/internal/listingkit/model_task.go:83)
- [D:\code\task-processor\web\listingkit-ui\src\lib\api\task-list-schema.ts](D:/code/task-processor/web/listingkit-ui/src/lib/api/task-list-schema.ts:16)
- [D:\code\task-processor\web\listingkit-ui\src\components\listingkit\tasks\task-list-page-sections.tsx](D:/code/task-processor/web/listingkit-ui/src/components/listingkit/tasks/task-list-page-sections.tsx:608)

建议：

- 如果后面要统一命名，可以新增 `sds_design_status`
- 但这不是当前主协议的阻塞项

## 不建议现在处理的点

### 1. workspace key 协议整体更名

例如把：

- `request_draft`
- `preview_product`

改成：

- `draft_payload`
- `preview_payload`

这会连带影响：

- repair center
- readiness guidance
- editor focus
- revision validation
- 前端 action key 映射

这已经不是“字段清理”，而是一轮新的 workspace key 协议迁移。

### 2. 直接删除 JSON 兼容字段

当前还不满足删除条件。

应继续保留的字段：

- `sds_sync`
- `request_draft`
- `preview_product`
- `submission`
- `final_draft`

直到确认：

1. 前端不再依赖旧字段
2. 外部调用方不再依赖旧字段
3. 历史任务恢复路径稳定
4. 导出/审计消费方已迁移

## 推荐下一步

如果继续做，我建议按下面顺序：

1. 清理 B1 类错误文案和日志文案
2. 只在前端内部把局部变量名逐步改成 `submissionState / previewPayload`
3. 暂时不要动 workspace key
4. 等调用方迁移稳定后，再单独规划“workspace key 语义升级”

## 一句话判断

现在仓库里看到的很多旧名字，并不都代表技术债。

真正应该继续清理的，主要是：

- 旧字段名出现在错误文案
- 旧字段名出现在局部变量和辅助说明
- 旧字段名出现在新代码示例之外的残余提示

而下面这些应继续保留：

- JSON 兼容字段
- 兼容归一化 helper
- readiness/workspace key
- submission 领域名词
