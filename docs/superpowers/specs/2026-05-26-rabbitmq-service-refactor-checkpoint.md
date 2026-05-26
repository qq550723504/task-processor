# RabbitMQService Refactor Checkpoint

## 背景

`internal/app/consumer/rabbitmq_service.go` 最初同时承载了 3 层复杂度：

1. 基础设施装配：连接、client、consumer、initializer、registry 创建
2. 运行时编排：启动、停止、重连、handler 注册、assignment 同步、consumer guard
3. 生命周期状态：`started`、`consumerActive`、`ctx/cancel`、`ownedStores/useStoreQueues`、stats 输出

这类“组合根 + 运行时状态机 + 业务辅助状态”糅合在一起的 service，会在两个方向上持续恶化：

- 任何一类新需求都会继续往同一个根对象里堆
- 测试往往只能兜最终结果，不容易锁住中间阶段边界

## 本轮已经完成的主要收敛

### 1. 构造层与运行时分离

已从 `NewRabbitMQService(...)` 收出：

- `applyRabbitMQServiceDefaults(...)`
- `newRabbitMQConnectionManager(...)`
- `newRabbitMQConsumer(...)`

当前效果：

- 构造装配和后续运行时行为不再混在一起
- `NewRabbitMQService(...)` 更接近真正的组合根

### 2. 启动流水线分阶段

`Start()` 已分成明确阶段：

- `startInfrastructure()`
- `initializeQueueTopology()`
- `initializeTaskQueues()`
- `initializeCrawlerQueues()`
- `startConsumers()`

并且最近又补上了：

- `prepareStartState(...)`
- `markStarted()`

当前效果：

- 启动状态准备与最终状态写入开始对称化
- 启动方法本身更接近“阶段编排”

### 3. 后台协作者已独立

已独立的协作者包括：

- [store_assignment_sync.go](/D:/code/task-processor/internal/app/consumer/store_assignment_sync.go)
- [consumer_guard.go](/D:/code/task-processor/internal/app/consumer/consumer_guard.go)
- [queue_handler_builder.go](/D:/code/task-processor/internal/app/consumer/queue_handler_builder.go)
- [service_component_state.go](/D:/code/task-processor/internal/app/consumer/service_component_state.go)

当前效果：

- `RabbitMQService` 不再自己维护完整 assignment 状态机
- consumer guard 不再嵌在 service 根对象里
- queue handler 组装与生命周期逻辑解耦
- component state 同步有了统一入口

### 4. 停止路径已阶段化

`Stop()` 近期又收出：

- `serviceStopState`
- `stopStateSnapshot()`
- `markStopped()`

当前效果：

- `Stop()` 现在更接近“读 stop state -> 执行 stop pipeline”
- 停止依赖摘取不再散在方法体里

### 5. 状态读取已统一成 snapshot 风格

已形成的状态快照包括：

- `consumerLifecycleState`
- `serviceStopState`
- `serviceStatsState`

对应 helper：

- `consumerLifecycleStateSnapshot()`
- `stopStateSnapshot()`
- `statsStateSnapshot()`

当前效果：

- `pauseConsumers(...)`
- `resumeConsumers(...)`
- `restartConsumers(...)`
- `Stop()`
- `GetStats()`

都开始共用统一的状态读取风格，而不是各自手写锁与字段组合

## 当前结构现状

现在 `RabbitMQService` 已经从“全能服务”明显转向“组合根 + 协作者 + 状态快照”。

已经形成的边界：

- factory helpers
- startup phases
- queue handler builder
- store assignment sync
- consumer guard
- component state sync
- lifecycle snapshots
- stop snapshot
- stats snapshot

## 仍然值得关注但暂时不必继续细拆的点

### 1. routing state 仍裸露在 service 根对象上

例如：

- `ownedStores`
- `ownedBuckets`
- `useStoreQueues`

现在它们的读取开始快照化，但更新入口仍主要挂在 service 自身。

### 2. `GetStats()` 仍是投影拼装点

当前它已经有本地 snapshot，但仍直接拼：

- 连接状态
- consumer 健康状态
- processor registry 统计

这没有错，但如果未来要做 metrics/exporter，可能需要更明确的 projection 层。

### 3. `Start()` 仍是顺序脚本

虽然启动阶段已经更清楚，但它仍然是 service 方法顺序串联多个 phase helper，而不是独立的启动阶段对象。

## 结论

这一轮 `consumer` 收敛已经达到一个健康 checkpoint：

- 最大块职责已经拆掉
- 重复状态门禁已经统一
- start/stop/stats/lifecycle 的状态边界开始对称化

继续往下做当然可以，但收益已经明显下降。
从这个点开始，更合适的策略不是继续把同一个文件切得更碎，而是：

1. 停下来做盘点与回归验证
2. 或者切去别的复杂度热点，拿更高收益的结构改进
