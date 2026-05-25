# RabbitMQService Refactor Review

## 背景

`internal/app/consumer/rabbitmq_service.go` 原先同时承担了 4 类职责：

1. 基础设施装配：RabbitMQ 连接、client、consumer、initializer、registry 的创建和默认值填充。
2. 启动流水线：连接、重连回调、队列初始化、动态店铺归属同步、handler 注册、consumer 启动。
3. 后台协程控制：consumer guard、dynamic store assignment sync。
4. 业务状态同步：`ownedStores/useStoreQueues` 与 `TaskProcessorRegistry.UpdateComponents(...)` 的联动。

这会带来两个长期问题：

- 行为修改需要同时改多处相邻但不同层级的逻辑，容易漏。
- 测试虽然能覆盖结果，但不容易覆盖中间阶段的边界约束。

## 本轮已完成的结构收敛

### 1. 构造装配与运行时分离

已从 `NewRabbitMQService(...)` 中抽出：

- `applyRabbitMQServiceDefaults(...)`
- `newRabbitMQConnectionManager(...)`
- `newRabbitMQConsumer(...)`

当前效果：

- 构造层主要负责“配置规范化 + 基础设施装配”。
- 运行时行为不再和默认值填充混在一起。

### 2. 启动流水线分段

`Start()` 已拆成更清楚的阶段 helper：

- `startInfrastructure()`
- `handleReconnect()`
- `initializeQueueTopology()`
- `initializeTaskQueues()`
- `initializeCrawlerQueues()`
- `startConsumers()`

当前效果：

- 启动顺序仍保持原语义。
- 失败点已经按阶段边界变得可读，可继续为每个阶段补更细测试。

### 3. store assignment sync 独立

已新增协作者文件：

- `internal/app/consumer/store_assignment_sync.go`

承担职责：

- 是否应启用 dynamic assignment sync 的判断
- assignment state 快照
- 周期同步
- 初次同步
- owned store reload 编排

当前效果：

- `RabbitMQService` 不再自己维护一整条 assignment 轮询状态机。
- assignment 行为已经具备独立演进边界。

### 4. consumer guard 独立

已新增协作者文件：

- `internal/app/consumer/consumer_guard.go`

承担职责：

- service 健康状态快照
- 周期巡检
- 根据 `decideConsumerAction(...)` 编排 pause/resume/restart

当前效果：

- `RabbitMQService` 只保留底层动作，不再自己承担 guard 编排。

### 5. queue handler build 独立

已新增协作者文件：

- `internal/app/consumer/queue_handler_builder.go`

承担职责：

- crawler/task 平台识别
- crawler queue 注册
- shared/store queue 注册
- shein bucket queue 注册

当前效果：

- handler 组装逻辑从 service 根对象中移出。
- 现在可以单独推演队列命名和路由策略，而不牵动生命周期逻辑。

### 6. component state sync 收口

已新增：

- `internal/app/consumer/service_component_state.go`

收口方法：

- `applyComponentDependencies(...)`
- `syncProcessorRegistryComponents()`

当前效果：

- `SetComponents(...)`
- `SetStoreAssignmentProvider(...)`
- dynamic assignment reload

三处原先各自同步 `ownedStores/useStoreQueues/processorRegistry.UpdateComponents(...)` 的逻辑，现在走统一路径。

### 7. Stop 行为更收紧

`Stop()` 现在会：

1. 停 consumer
2. 停 deduplicator
3. cancel service context
4. wait for background workers
5. close connection/provider

当前效果：

- `wg` 真正和后台协程生命周期绑定。
- 降低了 guard / assignment 协程在资源关闭后继续工作的窗口。

## 当前结构现状

现在 `RabbitMQService` 更接近组合根，而不是全能对象。

已经形成的边界：

- 构造：`rabbitmq_service_factory.go`
- 启动分段：`rabbitmq_service.go`
- queue handler build：`queue_handler_builder.go`
- consumer guard：`consumer_guard.go`
- store assignment sync：`store_assignment_sync.go`
- component state sync：`service_component_state.go`

## 仍然集中的剩余职责

### 1. `RabbitMQService` 仍直接持有过多运行时字段

例如：

- `ownedStores`
- `useStoreQueues`
- `consumerActive`
- `started`
- `ctx/cancel/wg`

它们虽然已经有部分协作者消费，但生命周期状态还是集中在 service 根对象上。

### 2. `Start()` 仍是“顺序脚本”而不是“阶段对象”

虽然已经拆成 helper，但本质仍是一个 service 方法顺序调用多个私有阶段。
如果后续启动前后还要插入更多动作，比如 metrics boot、warmup gating、external readiness hooks，这条链还会继续增长。

### 3. queue topology 初始化与 handler build 仍是松耦合协作

现在已经从代码位置上拆开，但仍通过 service 上的共享状态间接协作。
如果后续引入更多平台专属 queue 策略，可能还需要更明确的 topology runtime model。

## 结论

这一轮重构已经达到一个健康停点：

- 最大块的混合职责已经拆开。
- service 根对象明显变薄。
- 行为护栏测试仍然在。

继续往下拆是可以的，但收益已经从“消除明显复杂度热点”转向“结构精修”。
下一阶段不应再无差别细拆，而应该围绕明确目标推进。

