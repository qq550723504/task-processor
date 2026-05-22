# ListingKit Child Task Retry Design

## Goal

为 `listingkit` 提供统一的 `child task` 定向重跑能力，让调用方可以只重试失败或可恢复的子任务，而不必整单重新入队。

第一阶段覆盖当前已经存在于 `result.child_tasks` / `result.workflow_stages` 中、并且业务上适合单独重跑的 SDS 类子任务：

- `sds_catalog_product`
- `sds_design_sync`

后续其他 `kind` 应复用同一套接口和分发框架接入，而不是继续新增专用接口。

## Current Context

当前 `listingkit` 有两类“重试”能力，但都不能直接满足需求：

1. `POST /api/v1/listing-kits/tasks/:task_id/generation-tasks/retry`
   只面向资产生成子任务，服务入口是 `RetryTaskGenerationTasks`。
2. `POST /api/v1/management/tasks/:task_id/retry`
   面向管理端 import task，不是 `listingkit` 的 `child_tasks`。

而 `listingkit` 里的 `child_tasks` 目前只是主任务运行时写回 `ListingKitResult` 的阶段状态，不是独立调度实体：

- `internal/listingkit/workflow_standard.go`
- `internal/listingkit/workflow_sds_sync.go`

这意味着“重跑 child task”本质上不是切换一个已有任务状态，而是：

1. 读取当前主任务与已有 `result`
2. 找到该 `kind` 的重跑器
3. 在不重建整单的前提下，只执行对应子流程
4. 把 `result` 的对应片段、`child_tasks`、`workflow_stages`、`workflow_issues` 回写

## Proposed API

新增统一接口：

`POST /api/v1/listing-kits/tasks/:task_id/child-tasks/retry`

请求体：

```json
{
  "kind": "sds_design_sync"
}
```

预留但第一阶段不启用的字段：

```json
{
  "kind": "sds_design_sync",
  "options": {}
}
```

返回值使用最新的 task 结果页，而不是只返回动作确认：

```json
{
  "task_id": "48aacce8-9694-45a3-86f6-9021724d2528",
  "status": "needs_review",
  "result": {
    "child_tasks": [],
    "workflow_stages": []
  }
}
```

原因：

- 调用方通常下一步就要刷新状态面板
- 现有 `GetTaskResult` 结果模型已经足够表达重跑后的最新状态
- 避免前端额外发一次查询请求才能知道是否重跑成功

## Retry Semantics

### Allowed states

第一阶段允许在这些主任务状态下触发 child retry：

- `needs_review`
- `failed`
- `completed`

其中：

- `needs_review`
  是最常见场景，子任务失败但主任务结果仍可展示
- `failed`
  允许在整单失败后尝试恢复某个可独立补跑的子阶段
- `completed`
  允许对已完成任务做定向补跑，例如需要重新生成 SDS 结果

不允许：

- `pending`
- `processing`

理由是主流程仍在运行时再触发 child retry，会和现有状态写回互相覆盖。

### Allowed child task states

第一阶段仅允许以下 child 状态发起重跑：

- `failed`
- `completed`

其中：

- `failed`
  是显式恢复
- `completed`
  允许用户主动重新生成

不支持 `processing`，避免并发执行同一阶段。

### Unsupported kinds

如果 `kind` 不存在于当前任务结果中，或当前版本尚未注册对应重跑器，返回明确错误：

- `child_task_not_found`
- `child_task_not_retryable`

不要静默 fallback 到“整单重跑”。

## Service Design

新增统一服务入口，建议命名：

- `RetryTaskChildTask(ctx, taskID string, req *RetryChildTaskRequest) (*TaskResult, error)`

核心职责：

1. 读取任务
2. 校验主任务状态
3. 从 `result.child_tasks` 中定位对应 `kind`
4. 根据 `kind` 分发到对应重跑器
5. 把重跑结果写回任务
6. 返回最新 `TaskResult`

### Request model

新增模型：

```go
type RetryChildTaskRequest struct {
    Kind string         `json:"kind"`
    Options map[string]any `json:"options,omitempty"`
}
```

第一阶段 `Options` 仅保留扩展位，不做业务解释。

### Dispatcher

新增 child task retry dispatcher，而不是在主 service 里写 `switch` 到处散开。

建议结构：

```go
type childTaskRetryHandler interface {
    Kind() string
    Retry(ctx context.Context, task *Task, result *ListingKitResult) error
}
```

service 内部维护按 `kind` 注册的 handler map。

好处：

- 扩展新 `kind` 时只新增 handler
- 每个 child retry 的依赖和状态回写边界更清晰
- 避免把所有分支塞进一个超大的 service 文件

## First-phase Handlers

### `sds_catalog_product`

重跑器复用现有标准商品 SDS 建链逻辑，而不是重新发明流程。

目标：

- 刷新 `result.StandardProductSnapshot` 或相关 SDS 产物
- 刷新 `child_tasks[kind=sds_catalog_product]`
- 追加新的 `workflow_stage`

不要求它自动联动触发 `sds_design_sync`。

原因：

- 用户明确点的是某个 child task
- 自动级联会让重跑边界变模糊
- 如果需要连带执行，应由上层显式再重试 `sds_design_sync`

### `sds_design_sync`

重跑器复用现有 `workflow_sds_sync.go` 中 SDS 设计同步逻辑。

目标：

- 重新执行远端 SDS 渲染/同步
- 刷新 `result.SDSSync`
- 刷新 `child_tasks[kind=sds_design_sync]`
- 追加新的 `workflow_stage`
- 刷新与该阶段相关的 `workflow_issues`

其中 `workflow_stages` 应保留历史记录追加，不覆盖旧条目。现有 recorder 已经有这种能力，新的 retry 执行应继续复用该模式。

## Result Mutation Rules

child retry 不是整单重算，因此必须明确只允许修改哪些片段。

### Shared mutations

所有 child retry 都可以修改：

- `result.child_tasks`
- `result.workflow_stages`
- `result.workflow_issues`
- `result.summary.warning_count`
- `result.review_reasons`
- 顶层 `task.error`

### `sds_catalog_product`

允许修改：

- `result.standard_product_snapshot`
- `result.warnings` 中与 SDS 建链相关的条目

不应修改：

- 图片资产结果
- 其他平台草稿
- `result.SDSSync`

### `sds_design_sync`

允许修改：

- `result.SDSSync`
- `result.warnings` 中与 SDS 渲染相关的条目

不应修改：

- 规范商品缓存
- 其他非 SDS 的平台结果

## Status Reconciliation

child retry 执行完成后，需要重新收敛主任务状态。

规则：

1. 如果重跑器返回错误：
   - 对应 child task 标记为 `failed`
   - 主任务状态保持原状，除非当前是 `completed`，则回退到 `needs_review`
2. 如果重跑成功但结果仍有 warning / review blocker：
   - 主任务状态为 `needs_review`
3. 如果重跑成功且不再需要 review：
   - 主任务状态为 `completed`

不要在 child retry 成功后直接标记主任务 `processing` 或 `pending`。

## Error Model

新增明确错误语义，供 handler 转成稳定 HTTP 响应：

- `ErrChildTaskRetryInvalidRequest`
- `ErrChildTaskNotFound`
- `ErrChildTaskNotRetryable`
- `ErrChildTaskRetryConflict`

建议映射：

- `400`
  请求体错误、空 `kind`
- `404`
  task 不存在或 child task 不存在
- `409`
  task / child 当前状态不允许重跑
- `500`
  实际重跑执行失败

## HTTP Handler Design

在 `listingkit` handler 增加新入口，例如：

- `RetryTaskChildTask(c *gin.Context)`

路由新增到：

- `internal/listingkit/httpapi/routes.go`

路径：

- `POST /api/v1/listing-kits/tasks/:task_id/child-tasks/retry`

handler 行为：

1. bind JSON
2. 调用 service
3. 返回最新 task 结果
4. 错误时输出稳定 code/message

## UI Expectations

虽然第一阶段可以先只做后端，但接口语义要为前端预留稳定接法。

前端可按下面方式判断是否展示按钮：

- `child.status in ["failed", "completed"]`
- `child.kind` 在后端已支持名单中
- 主任务状态不在 `pending/processing`

按钮文案统一为：

- `重试子任务`

失败提示直接显示后端返回的稳定错误 message。

## Testing Strategy

测试分三层：

### Service tests

覆盖：

- `kind` 为空
- task 不存在
- child task 不存在
- child task 状态不允许
- `sds_catalog_product` retry 成功
- `sds_design_sync` retry 成功
- retry 后主任务状态从 `needs_review` 收敛到 `completed`
- retry 失败后主任务状态保持或回退到 `needs_review`

### Handler tests

覆盖：

- 路由注册
- JSON bind
- 404 / 409 / 500 映射
- 成功时返回最新 task result

### Regression tests

针对 `workflow_stages` / `child_tasks`：

- 重跑后应追加新的 stage，而不是覆写历史 stage
- 重跑后对应 child status 和 error 应更新

## Non-goals

第一阶段不做这些事情：

- 不把 `child_tasks` 持久化成独立数据库表
- 不为每个 `kind` 新建专用 HTTP 接口
- 不支持正在 `processing` 的 child 强制抢占重跑
- 不自动级联“重试 A 顺便重试 B”
- 不对前端 UI 做强依赖改造

## Recommended Implementation Order

1. 定义 request / error / service interface
2. 补 handler 和 route
3. 搭 child retry dispatcher
4. 接 `sds_design_sync`
5. 接 `sds_catalog_product`
6. 补状态收敛与回归测试

