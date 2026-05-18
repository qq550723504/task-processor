# Temporal 工作流编排评估

## 结论

对当前项目来说，如果后续要把 SHEIN 提交、远端确认、失败恢复、人工重试、跨节点续跑做成真正的长流程能力，优先方案应当是 Temporal，而不是继续在 RabbitMQ request-reply 和本地状态表之上累积自研编排逻辑。

这个结论不意味着“现在立刻把所有流程迁过去”。更合理的做法是：

1. 保留当前已落地的进程内 SHEIN submit 状态机。
2. 把 Temporal 作为第二阶段 durable workflow 底座。
3. 从一个小而完整的长流程 PoC 切入，而不是一次性迁全链路。

## 为什么是 Temporal

根据 Temporal 官方文档和 Go SDK说明，Temporal 的核心价值是 durable execution：工作流在进程崩溃、网络异常、worker 重启后，仍能从上一次持久化状态继续推进，而不是重新从头编排或依赖额外的补偿脚本。

它天然提供的能力，刚好覆盖这个项目后续最贵的复杂度：

- 工作流状态持久化
- Activity 重试和超时
- 定时等待
- Signal / Query 交互
- 子工作流拆分
- 任务队列和 worker 执行模型
- 运行中可观测性

这类能力如果继续堆在 RabbitMQ + 自定义状态表 + 锁 + 补偿逻辑上，维护成本会持续上升，而且每增加一个阶段都要重复处理：

- 当前推进到哪一步
- 远端是否已收到请求
- 失败后从哪里恢复
- 同一请求是否已经执行过
- 另一个节点接手后如何继续

Temporal 适合替代的正是这层“自研可靠编排系统”。

参考：

- <https://docs.temporal.io/>
- <https://temporal.io/>
- <https://github.com/temporalio/sdk-go>

## 为什么不是继续增强 RabbitMQ 编排

当前项目里，分布式 crawler 已经是 RabbitMQ request-reply 模式，见 `internal/app/crawler/distributed/client.go`。这个模式适合：

- 把任务投递给远端 worker
- 等一个结果返回
- 做普通异步并发消费

但它并不天然适合长流程场景，尤其是下面这些需求一旦叠加，复杂度会迅速上涨：

- 阶段可见
- 长时间等待
- 远端状态确认
- 同键幂等
- 人工介入后恢复
- 跨节点续跑
- 同一实体串行化处理

这些问题 RabbitMQ 不是不能做，而是做出来后，你们维护的已经不是“消息队列接入层”，而是一套自己的工作流引擎。

## 为什么不把 Machinery 作为主方案

`Machinery` 更适合“把任务丢给 worker 执行”的分布式任务队列场景。它能减少部分任务分发和 worker 注册样板代码，但它不是这个项目后续最需要替代的那一层。

这里真正昂贵的不是“怎么投递任务”，而是：

- 怎么保证长流程不中断
- 怎么在失败后从正确阶段恢复
- 怎么处理等待和确认
- 怎么暴露运行态给 UI 和运维

因此：

- 对 crawler 或通用后台任务，`Machinery` 可以作为候选
- 对 SHEIN 提交主链路和后续多阶段工作流，不建议把 `Machinery` 当最终底座

## 与当前代码的关系

当前代码并不是完全没有状态机。SHEIN submit 这块已经有一版进程内状态机基础：

- 提交阶段结构：`internal/publishing/shein/submission.go`
- 提交流程推进：`internal/listingkit/service_submit.go`
- 幂等键和阶段状态辅助：`internal/listingkit/submission/state.go`

这意味着下一步不应该“推倒重来”，而应该做边界切分：

- 保留现有 payload 构造、校验、图片上传、远端接口调用这些业务逻辑
- 把“谁来编排阶段推进、等待、恢复、重试、确认”逐步迁到 Temporal Workflow

也就是说，Temporal 替代的是编排层，不是平台业务规则层。

## 适合首先迁入 Temporal 的场景

最适合作为第一批 Temporal 工作流的，不是整个 ListingKit 主链路，而是已经开始呈现长流程特征的提交链路：

1. SHEIN publish / save_draft 工作流
2. submit_remote 之后的远端确认流程
3. 失败后可恢复重放的提交尝试
4. 需要等待外部状态变化的确认型流程

原因：

- 阶段明确
- 外部依赖多
- 幂等要求高
- 人工排查成本高
- 已经有 submit phase 模型可映射

## 暂时不建议迁入 Temporal 的部分

以下能力暂时不建议第一阶段就迁：

- 资料生成主流程
- 普通 HTTP 同步查询接口
- 只在单次请求内完成的短事务
- 纯定时 cron 型任务
- 简单 crawler 投递链路

原因是这些场景还没有出现足够强的 durable workflow 需求，过早迁移只会增加系统复杂度。

## 建议的 Temporal 领域建模

第一阶段可以先建立非常克制的模型：

- Workflow ID：`shein-submit:<taskID>:<action>`
- Signal：
  - `retry`
  - `cancel`
  - `confirm_remote`
- Query：
  - `current_state`
  - `current_phase`
  - `last_error`
- Activities：
  - `LoadTaskResult`
  - `ValidateReadiness`
  - `PrepareSubmitProduct`
  - `UploadImages`
  - `PreValidate`
  - `SubmitRemote`
  - `PersistSubmissionResult`
  - `ConfirmRemoteStatus`

原则是：

- Workflow 只做编排和状态推进
- I/O、网络、数据库写入都放进 Activity
- 现有业务函数尽量直接下沉复用，不重写平台规则

## 建议的最小 PoC

建议先做一个最小可验证 PoC，只覆盖 SHEIN `publish`：

### PoC 目标

- 同一 `taskID + action` 只有一个运行中的 Temporal Workflow
- UI / API 能查询当前 phase
- worker 重启后可以继续
- `submit_remote` 成功后可以继续做远端确认
- 同 idempotency key 重放不会重复提交远端

### PoC 范围

- 新增一个独立 Temporal worker 进程或在现有服务中挂 worker
- 只接管 SHEIN `publish`
- `save_draft` 和其他平台暂时保留现有实现
- 只做本地开发和测试环境验证，不马上替换生产主链路

### PoC 成功标准

- 人为 kill worker 后，重启可以恢复进行中的 workflow
- 重复提交同一个请求，不会重复调用远端 publish
- API 能返回当前 workflow phase 和最近错误
- 远端确认失败时，可以通过 signal 或重试继续

## 落地顺序建议

1. 先做 Temporal 技术验证，不改现网主链路
2. 把 SHEIN `publish` 提交编排抽成 Workflow PoC
3. 保持现有业务 Activity 复用，避免重写平台逻辑
4. 验证可观测性、恢复、幂等和开发调试成本
5. 再决定是否把 `save_draft`、远端确认和其他平台提交流程迁入

## 风险和前提

采用 Temporal 之前，需要先接受几个事实：

- 它会引入新的基础设施组件，不是一个纯 Go 库替换
- 工作流代码需要遵守 Temporal 的 deterministic 约束
- 团队需要学习 workflow / activity / signal / query 的建模方式
- 本地开发、测试、部署和监控都要补一层运维规范

如果团队不准备接受这些约束，就不应该上 Temporal；否则只会把系统复杂度从业务层搬到半成品基础设施层。

但如果目标明确是“把长流程可靠性做成产品能力”，这些成本是合理的。

## 最终建议

明确建议如下：

- 当前项目的长流程编排方向，选 Temporal。
- 不把 Machinery 作为主链路最终方案。
- 不立刻全量迁移。
- 从 SHEIN `publish` 的最小 PoC 开始，验证 durable workflow 是否真正解决当前恢复、幂等和可观测性问题。

## PoC 落地结果

截至 `2026-05-18`，最小 PoC 已经落地到代码，范围仍然保持克制：

- 只接管 `ListingKit` 的 `shein + publish`
- `save_draft` 继续走原来的同步路径
- 现有 payload 组装、图片上传、远端 publish、远端确认逻辑继续复用
- Temporal 只接管 workflow identity、活动编排、失败持久化和 worker 恢复

### 已证明的点

当前 PoC 已经证明了下面几件事：

1. `workflow ID = shein-submit:<taskID>:publish` 可以稳定约束同一任务的并发 publish。
2. HTTP API 可以只负责启动 workflow，而不是把完整提交链路卡在请求线程里。
3. workflow 阶段仍然能回写到现有 `SubmissionReport` / `SubmissionEvents`，UI 不需要再维护第二套状态来源。
4. 远端 publish 的具体业务逻辑可以继续留在 ListingKit 现有 helper 中，不需要把平台规则改写成另一套实现。
5. worker 注册、client 封装、HTTP API 侧接线可以在不重构全局 runtime 的情况下先跑通。

### 当前仍然保留在线内的部分

PoC 落地后，下面这些能力还没有迁过去：

- `save_draft`
- 非 SHEIN 平台提交流程
- 专门的 workflow query API
- 独立 worker 进程拆分
- 更丰富的 signal / manual retry / cancel 入口

这意味着当前 PoC 更接近“证明编排层可替换”，还不是“提交系统已经全面 Temporal 化”。

### 下一步真正需要验证的风险

后续如果继续推进，需要重点验证三类问题：

1. **恢复语义**
   - worker 在 `submit_remote` 之后、`confirm_remote` 之前重启，是否还能稳定续跑
2. **重复提交保护**
   - 不同请求键、相同任务键、老 workflow 历史与新请求之间的冲突处理是否足够清晰
3. **运维可见性**
   - 现在主要还是通过 ListingKit preview / submission events 看状态；如果要产品化，还需要更明确的 workflow 运维入口

### 结论调整

PoC 结果没有推翻之前的结论，反而把边界验证清楚了：

- Temporal 适合作为 SHEIN 长流程提交的 durable orchestration 底座。
- 现有 ListingKit submit 业务逻辑可以被保留下来，迁移成本主要落在编排层和运行时接线上。
- 下一阶段如果继续推进，优先级应当是：
  1. 增加专门的 query / retry / cancel 入口
  2. 做 worker 重启与重复提交的手工回归
  3. 再决定是否把 `save_draft` 一并迁入
