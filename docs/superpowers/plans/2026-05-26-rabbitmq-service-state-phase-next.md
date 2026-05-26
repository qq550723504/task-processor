# RabbitMQService State Snapshot Next Phase

## 推荐结论

先把 `RabbitMQService` 停在当前状态，优先做阶段盘点和回归验证，而不是继续对单个文件做低收益精修。

原因：

1. 最大块职责已经在前几轮拆开。
2. 最近这几轮主要收益来自统一状态读取风格，而不是继续拆出新的大边界。
3. 再继续细拆，很容易进入“helper 更多，但模型收益有限”的阶段。

## 如果继续，推荐顺序

### 方向 1：启动/停止状态对称化

目标：

- 让 `Start()` 和 `Stop()` 的状态写入风格更对称

建议步骤：

1. 提取 `markStarted(...)`
2. 让 `Start()` 通过统一 helper 写入 `started / consumerActive / ctx`
3. 评估是否需要 `serviceStartState`

适用前提：

- 团队准备继续沿 lifecycle 模型收口

### 方向 2：owned store routing state 单独建模

目标：

- 收紧 `ownedStores / ownedBuckets / useStoreQueues` 这组 routing state

建议步骤：

1. 提取 routing state snapshot 或 value object
2. 让 `SetComponents(...)`、`SetStoreAssignmentProvider(...)`、assignment reload 共用同一套 routing state 更新入口
3. 在 `GetStats()` 中直接投影 routing state，而不是散落读取字段

适用前提：

- 未来动态分片/店铺队列策略还会继续扩展

### 方向 3：stats projection 独立

目标：

- 让 `GetStats()` 变成更明确的投影层

建议步骤：

1. 提取 `buildServiceStats(...)`
2. 分离本地 snapshot 与外部依赖查询
3. 为 metrics/exporter 预留更稳定的组装边界

适用前提：

- 后续需要把 stats 复用到别的输出面

## 不推荐的方向

### 1. 继续零散拆更多小 helper

原因：

- 当前最大的重复模式已经收平
- 继续切碎容易让阅读成本反而上升

### 2. 现在就重写为完整状态机模型

原因：

- 当前复杂度还没到必须引入完整状态机对象的程度
- 会显著扩大验证面，不适合这轮“小步、不改行为”的节奏

## 下一步建议

推荐先做下面两件事中的一个：

1. 停在这里，转去别的复杂度热点
2. 如果仍然留在 `consumer`，优先做启动/停止状态对称化，而不是继续拆 stats 之外的零散 helper
