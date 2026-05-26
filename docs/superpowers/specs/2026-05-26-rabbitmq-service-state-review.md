# RabbitMQService State Snapshot Review

## 背景

在前一轮把 `RabbitMQService` 的大块职责拆成 factory、queue handler builder、consumer guard、store assignment sync 之后，`internal/app/consumer/rabbitmq_service.go` 里还残留了一类更细但反复出现的问题：

1. 生命周期状态读取分散
2. 停止路径依赖摘取分散
3. 统计读取和 service 锁混在一起

这些问题虽然不像最开始那样是“大对象全能职责”，但会持续放大两个维护成本：

- 同一组状态字段在多个方法中各自手写锁与条件门禁，后续容易漂移。
- 测试更容易覆盖最终行为，不容易直接约束“状态读取边界”的一致性。

## 本轮已完成的状态收敛

### 1. consumer lifecycle 状态快照

已新增：

- `consumerLifecycleState`
- `consumerLifecycleStateSnapshot()`
- `setConsumerActive(...)`

当前效果：

- `pauseConsumers(...)`
- `resumeConsumers(...)`
- `restartConsumers(...)`

这三条路径不再各自手写 `started / consumerActive / ctx` 的锁读取逻辑。

### 2. stop 状态快照

已新增：

- `serviceStopState`
- `stopStateSnapshot()`
- `markStopped()`

当前效果：

- `Stop()` 现在更接近“读取 stop state -> 执行 stop pipeline”。
- `consumer / deduplicator / cancel / connManager / provider` 这些停止依赖不再临时在方法体内拆字段。

### 3. stats 状态快照

已新增：

- `serviceStatsState`
- `statsStateSnapshot()`

当前效果：

- `GetStats()` 不再在 service 锁里同时读取 routing state 和外部连接/consumer 健康信息。
- `ownedStores / ownedBuckets` 会在快照阶段复制，避免后续意外共享底层切片。

## 当前结构现状

现在 `RabbitMQService` 里已经形成了 3 类状态读取模式：

- lifecycle snapshot
- stop snapshot
- stats snapshot

这意味着 service 根对象虽然仍保留状态字段，但状态消费方式开始统一，后续再变更字段时更容易集中处理。

## 仍然集中的剩余职责

### 1. `Start()` 仍直接改写 started/consumerActive

当前 `Start()` 仍在方法尾部直接：

- `s.started = true`
- `s.consumerActive = true`

它与已经收好的 `markStopped()` 风格还不完全对称。

### 2. owned store / queue mode 状态仍在 service 根对象中裸露

例如：

- `ownedStores`
- `ownedBuckets`
- `useStoreQueues`

虽然读取路径已经开始快照化，但它们仍主要由 service 自己直接持有和修改。

### 3. `GetStats()` 仍混合本地状态和外部依赖查询

现在本地状态已经通过快照统一，但：

- `IsConnected()`
- `HasHealthyRequiredConsumers()`
- `GetUnhealthyRequiredQueues()`
- `processorRegistry.GetStats()`

仍然在 `GetStats()` 中直接拼装。这个边界当前是合理的，但如果未来要做 metrics/exporter，可能还需要更明确的 stats projection。

## 结论

这一轮状态收敛已经达到一个健康停点：

- 最重复的状态门禁已经统一。
- `RabbitMQService` 的局部行为更接近 phase/snapshot 风格。
- 后续再动 lifecycle 或 stats 时，改动面会更集中。

继续往下做是可以的，但收益已经开始下降。
下一阶段不建议继续无差别细拆，而应围绕更明确的目标推进，例如启动状态对称化或 stats projection 边界。
