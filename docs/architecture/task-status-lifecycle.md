# Task Status Lifecycle

本文档描述当前 `task-processor` 中导入任务的状态定义、流转约束，以及 `app/task`、`SHEIN`、`TEMU` 三条主链路如何使用这些规则。

## 1. 统一状态定义

公共任务状态定义位于 [internal/model/task_status.go](D:/code/task-processor/internal/model/task_status.go)。

当前主要状态：

- `pending`: 待处理
- `processing`: 处理中
- `pending_retry`: 待重试
- `published`: 已上架
- `draft`: 草稿
- `paused`: 已暂停
- `terminated`: 已终止

补充状态：

- `crawled`
- `crawl_failed`
- `queued`
- `republishing`
- `cancelled`
- `resumed`
- `resuming`

统一能力：

- `ParseTaskStatus(code int16)`
- `IsValid()`
- `IsTerminal()`
- `CanTransitionTo(target)`
- `ValidateTaskStatusTransition(from, to)`
- `ValidateTaskStatusTransitionCode(fromCode, to)`

## 2. 统一流转规则

当前代码里显式维护的核心流转：

- `pending -> processing`
- `pending_retry -> processing`
- `processing -> pending_retry`
- `processing -> published`
- `processing -> draft`
- `processing -> paused`
- `processing -> terminated`
- `processing -> cancelled`
- `published -> republishing`
- `published -> paused`
- `paused -> resumed`
- `resumed -> resuming`
- `resuming -> published`
- `resuming -> paused`
- `resuming -> terminated`

说明：

- 这套规则由公共模型维护，不允许业务模块各自手写隐式状态表。
- 业务里如果需要从 API 返回的状态码进入下一状态，优先使用 `ValidateTaskStatusTransitionCode(...)`。
- 成功态和失败态更新，优先通过统一状态更新服务 [internal/app/taskstatus/service.go](D:/code/task-processor/internal/app/taskstatus/service.go) 发往管理端。

## 3. 获取链路

`app/task` 中获取和分发任务的职责已经拆开：

- [internal/app/task/task_source.go](D:/code/task-processor/internal/app/task/task_source.go)
  负责从管理端获取 `pending` / `pending_retry` 任务
- [internal/app/task/task_claim_service.go](D:/code/task-processor/internal/app/task/task_claim_service.go)
  负责 claim 抢占任务，并将状态推进到 `processing`
- [internal/app/task/task_dispatch_guard.go](D:/code/task-processor/internal/app/task/task_dispatch_guard.go)
  负责店铺信息检查和暂停态检查
- [internal/app/task/task_dispatcher.go](D:/code/task-processor/internal/app/task/task_dispatcher.go)
  负责把任务分发到具体平台 submitter
- [internal/app/task/dispatcher.go](D:/code/task-processor/internal/app/task/dispatcher.go)
  只负责编排和统计

claim 约束：

- 已在本地 `processingTasks` 中的任务不会重复抢占
- 只有满足 `current_status -> processing` 的任务才允许被 claim
- 非法状态不会被推进为 `processing`

## 4. 状态更新服务

管理端任务状态更新统一经由：

- [internal/app/taskstatus/service.go](D:/code/task-processor/internal/app/taskstatus/service.go)

职责：

- 统一构造 `ProductImportTaskUpdateReqDTO`
- 统一重试
- 统一同步/异步更新
- 统一校验状态流转

当前已经接入的链路：

- `app/task` 抢占任务到 `processing`
- `TEMU` 处理完成、草稿、待重试、暂停、终止
- `SHEIN` pipeline 错误处理
- `SHEIN publish` 成功后 `published` / `draft`

## 5. SHEIN 生命周期

### 5.1 错误定义与导出

SHEIN 错误定义分层如下：

- [internal/shein/sherr](D:/code/task-processor/internal/shein/sherr)
  真正的错误类型与分类规则
- [internal/shein/error_exports.go](D:/code/task-processor/internal/shein/error_exports.go)
  对外导出入口，保持上层调用稳定

`sherr` 已按职责拆分：

- `cookie_errors.go`
- `retryable_errors.go`
- `filtered_errors.go`
- `auth_errors.go`
- `classification.go`

### 5.2 错误路由

SHEIN 任务处理入口位于：

- [internal/shein/pipeline/task.go](D:/code/task-processor/internal/shein/pipeline/task.go)

错误路由位于：

- [internal/shein/pipeline/task_error_router.go](D:/code/task-processor/internal/shein/pipeline/task_error_router.go)

当前规则：

- `CookieLoadError` 走普通失败处理
- 认证过期错误走 `HandleAuthenticationExpired`
- 其余错误走 `HandleTaskFailure`

### 5.3 错误处理器

错误处理器位于：

- [internal/shein/pipeline/task_error_handler.go](D:/code/task-processor/internal/shein/pipeline/task_error_handler.go)

当前规则：

- `FilteredError`
  只记录为业务过滤，不写 `terminated`，不计失败指标
- 不可重试错误
  写 `terminated`
- 可重试错误且未达上限
  写 `pending_retry`
- 可重试错误达到上限
  写 `terminated`
- `MaxRetries <= 0`
  回退默认值 `3`
- 认证过期
  走店铺暂停、Cookie 清理、任务待重试

### 5.4 Publish 成功态回写

`publish` 成功后的状态通知统一经由：

- [internal/shein/publish/task_status_notifier.go](D:/code/task-processor/internal/shein/publish/task_status_notifier.go)

当前场景：

- 发品成功后写 `published`
- 保存草稿后写 `draft`

## 6. TEMU 生命周期

TEMU 状态出口位于：

- [internal/temu/task_handler.go](D:/code/task-processor/internal/temu/task_handler.go)

当前将业务结果映射为公共状态：

- `completed -> published`
- `draft -> draft`
- `pending_retry -> pending_retry`
- `terminated -> terminated`
- `paused -> paused`

更新时统一经过公共状态更新服务，并校验：

- `processing -> target_status`

## 7. 当前约定

- 新增任务状态时，必须先改公共模型，再改业务模块
- 新增任务状态出口时，优先复用 `taskstatus.Service`
- 新增平台错误分类时，优先拆成“错误定义 / 错误路由 / 错误处理器”三层
- 业务过滤不应当被统计为真正失败
- 认证过期应尽量统一路由为“暂停店铺 + 任务待重试”

## 8. 后续可继续收口的方向

- 把状态流转表改成表驱动配置或更显式的 transition map 文档
- 为 `SHEIN publish` 增加 notifier 层测试
- 为 `TEMU` 补状态映射和状态出口测试
- 在 README 中补一条指向本文档的链接
