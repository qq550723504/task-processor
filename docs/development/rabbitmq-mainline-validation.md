# RabbitMQ 主链验收手册

本文档用于验证当前主链是否已经切到：

`yudao-cloud 数据库选任务 -> RabbitMQ -> shein-listing / temu-listing / amazon-listing -> 状态回写`

目标不是压满系统，而是先确认主链已经真正接通，而且不会再误走 `cmd/task` 老轮询链。

## 1. 先决条件

验收前先确认：

- `cmd/task` 没有被启动
- 至少启动一个平台消费者：
  - `cmd/shein-listing`
  - `cmd/temu-listing`
  - `cmd/amazon-listing`
- `yudao-cloud` 已启动，并且 `TaskScheduler` 已开启自动投递
- RabbitMQ 可连通

## 2. 最小检查项

### 2.1 后端侧

检查这些接口：

- `GET /admin-api/listing/task-management/health`
- `GET /admin-api/listing/task-management/metrics`
- `GET /admin-api/listing/task-management/task-processor-nodes`

通过标准：

- `taskStats.currentQueueSize` 能返回真实值
- `rabbitmq` 状态不是 `DOWN`
- `taskProcessorNodes` 能看到当前节点

### 2.2 task-processor 本地治理接口

检查这些接口：

- `GET /api/v1/management/tasks/health`
- `GET /api/v1/management/tasks/{taskId}/status`

通过标准：

- `/health` 返回 `status=ok` 或可解释的 `degraded`
- `localWorkers` 里能看到本地池状态
- 指定任务时能查到结构化状态：`statusKey/statusName/canonicalStatus`

### 2.3 consumer 运行态

检查这些接口：

- `GET /health`
- `GET /stats`

通过标准：

- `/health` 返回 RabbitMQ 就绪状态
- `/stats` 里能看到 `ServiceManager`、`rabbitmq_detail`、节点角色等信息

## 3. 推荐验收流程

### 步骤 1：确认没有 legacy polling

- 不要启动 [main.go](D:/code/task-processor/cmd/task/main.go)
- 如果有人误启，它现在必须带 `--allow-legacy-polling`
- 日志里如果出现 `legacy polling` / `legacy task fetcher`，说明仍有人在用旧链路

### 步骤 2：造一批真实任务

建议最少准备：

- SHEIN 3 条
- TEMU 3 条
- Amazon 3 条

如果要验证公平性，再准备：

- 同平台大店铺 10 条
- 同平台小店铺 2 条

### 步骤 3：观察队列进入与消费

看后端：

- `task-management/health`
- `task-management/metrics`

看消费者：

- `/health`
- `/stats`
- `/api/v1/management/tasks/health`

通过标准：

- 任务先进入 `queued`
- 平台消费者开始消费
- `currentQueueSize` 会变化
- 节点健康保持 `UP` / `WARNING` 以内

### 步骤 4：验证单任务状态流转

任选一个 `taskId`，查看：

- 后端任务详情接口
- `GET /api/v1/management/tasks/{taskId}/status`

通过标准：

- 能看到统一状态字段
- 状态从 `pending/retry -> queued -> processing -> resolved`
- `canonicalStatus` 与执行器语义一致

### 步骤 5：验证不会重复执行

观察点：

- 同一 `taskId` 不应被多个消费者同时成功执行
- `processing` 超时后允许恢复，但不应出现并发双成功
- 不应再出现“老轮询拉一次 + RabbitMQ 又消费一次”的双轨执行

## 4. 自动化检查脚本

仓库里提供了一个轻量 PowerShell 脚本：

- [validate-rabbitmq-flow.ps1](D:/code/task-processor/scripts/validate-rabbitmq-flow.ps1)

示例：

```powershell
pwsh ./scripts/validate-rabbitmq-flow.ps1 `
  -ManagementBaseUrl "http://127.0.0.1:48080" `
  -ProcessorApiBaseUrl "http://127.0.0.1:8080" `
  -ConsumerBaseUrl "http://127.0.0.1:8081" `
  -AdminToken "<admin-token>" `
  -TaskId 12345
```

脚本会检查：

- 后端任务管理健康接口
- 后端指标接口
- task-processor 节点聚合接口
- 本地 processor 健康接口
- consumer `/health` 和 `/stats`
- 可选的单任务状态接口

## 5. 通过标准

本轮主链验收建议至少满足：

- 没有人启动 `cmd/task`
- 后端能自动把 DB 任务投递到 RabbitMQ
- 平台消费者能收到并执行任务
- 任务状态流转完整
- 不出现重复消费
- `processing` 超时恢复可以解释
- 三个平台都能被同一套治理接口观测到

## 6. 下一轮再做什么

这一轮验收通过后，再进入压测：

- 多平台公平性压测
- 每日额度并发压测
- 节点宕机恢复压测
- 平台并发配额压测
