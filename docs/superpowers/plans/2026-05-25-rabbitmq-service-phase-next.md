# RabbitMQService Next Phase

## 目标

在不改变现有行为的前提下，决定 `RabbitMQService` 下一阶段是：

1. 继续结构精修
2. 暂停并转向下一热点

## 推荐结论

推荐先停在当前点，不立即继续做深层细拆。

原因：

- 当前已经把最重的职责混合拆开。
- 继续往下的改动会更多触及生命周期状态模型，而不只是 helper/协作者迁移。
- 这类改动更适合在一次明确的小目标里推进，而不是持续“再拆一点”。

## 若继续拆，推荐顺序

### 方向 A：生命周期状态模型收口

目标：

- 减少 `RabbitMQService` 上直接暴露的运行时状态字段。

优先项：

1. 抽 `serviceRuntimeState` 或 `serviceLifecycleState`
2. 把 `started/consumerActive/ctx/cancel/wg` 迁到集中状态对象
3. 让 guard / assignment / stop 都只通过状态对象访问生命周期字段

收益：

- service 根对象进一步瘦身
- 更容易限制锁边界

风险：

- 涉及锁语义
- 容易误改 `Stop()` / `Start()` 协同关系

### 方向 B：启动阶段模型化

目标：

- 将 `Start()` 从“顺序调用多个 helper”演进为明确阶段对象或阶段 runner。

优先项：

1. 定义 startup stages
2. 明确每个 stage 的输入/输出
3. 为 reconnect / initial startup 复用阶段逻辑

收益：

- 后续插入新阶段更自然
- reconnect 与 startup 更容易共享步骤

风险：

- 当前收益中等
- 如果没有新增阶段需求，容易过度设计

### 方向 C：queue topology/runtime model

目标：

- 显式建模“当前节点应该拥有哪些 queue handlers / queue topology”。

优先项：

1. 引入 topology snapshot
2. 让 queue initializer 与 queue handler builder 都基于该 snapshot 工作
3. 让 assignment reload 只更新 snapshot，再触发 topology refresh

收益：

- 后续支持更多平台或更多分桶策略时更稳

风险：

- 当前业务收益低于 A/B
- 需要更多建模，不适合在连续小步重构里硬推

## 更推荐的实际下一步

如果项目总体仍以“压缩热点复杂度”为主，下一步更推荐转向别的热点，而不是继续深挖 `RabbitMQService`。

候选：

1. 回到 `listingkit/httpapi`，继续推进模块注册边界
2. 盘点 `listingadmin` 里剩余聚合/语义入口
3. 对本轮 consumer 重构做一次更系统的回归验证后准备收口

## 验证建议

无论是否继续拆 `RabbitMQService`，建议在下一阶段开始前先补一轮更系统的验证：

1. `go test ./internal/app/consumer -count=1`
2. `go test ./...` 中至少补跑依赖 `consumer` 的关键包
3. 如果有 staging/本地集成环境，手动验证：
   - startup
   - disconnect/reconnect
   - dynamic store assignment reload
   - stop/shutdown

## 执行建议

如果继续：

- 一次只选一个方向
- 每次只做一层收口
- 先补测试，再搬职责

如果暂停：

- 保留当前结构为一个稳定里程碑
- 把后续方向记录到文档，不在当前分支继续累积“结构精修”改动

