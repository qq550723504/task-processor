# RabbitMQService Refactor Checkpoint Next

## 推荐结论

把 `RabbitMQService` 停在当前 checkpoint，优先切去别的热点，除非接下来有明确需求必须继续演进 routing state 或 stats projection。

原因：

1. 高收益的职责拆分已经完成
2. 最近几刀主要是在统一状态风格，而不是继续消灭明显热点
3. 再往下做会越来越像“结构精修”，而不是继续显著降低复杂度

## 如果仍然继续留在 `consumer`

### 方向 1：routing state 单独建模

目标：

- 收紧 `ownedStores / ownedBuckets / useStoreQueues` 的更新和读取边界

建议步骤：

1. 提取 `serviceRoutingState`
2. 让 `SetComponents(...)`、`SetStoreAssignmentProvider(...)`、assignment reload 共用统一更新入口
3. 让 stats 直接读取 routing state snapshot

### 方向 2：stats projection 独立

目标：

- 让 `GetStats()` 变成更明确的投影层，而不是 service 里的拼装函数

建议步骤：

1. 提取 `buildServiceStats(...)`
2. 分离本地 snapshot 与外部依赖查询
3. 为未来 metrics/exporter 预留稳定边界

### 方向 3：启动阶段对象化

目标：

- 把 `Start()` 从顺序脚本进一步推进到阶段模型

建议步骤：

1. 抽出 `serviceStartPipeline`
2. 把 `connect / topology / handler / consumer start` 显式建模
3. 仅在后续需求确实会继续扩张启动阶段时推进

## 更推荐的下一步

当前更推荐：

1. 做一轮更大范围回归验证并准备合并
2. 或切回别的热点模块，例如 `listingkit/httpapi` 之外的新复杂度块

## 不推荐的方向

### 1. 继续零散拆更多 snapshot/helper

原因：

- 模型收益已经开始低于阅读成本

### 2. 现在就重写完整状态机

原因：

- 当前复杂度还没到必须引入完整状态机对象的程度
- 会显著扩大验证面，不符合这轮小步收敛节奏
