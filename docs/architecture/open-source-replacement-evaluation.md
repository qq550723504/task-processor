# 开源替代方案评估

## 目的

这份文档用于评估项目中已有模块、以及路线图里待增强的基础设施能力，哪些适合直接采用成熟开源实现，哪些适合在保留业务编排的前提下引入开源库重构，哪些则不建议替换。

评估原则：

- 优先替换通用基础设施，不替换平台业务壁垒。
- 优先选维护稳定、社区成熟、Go/Next.js 生态内主流方案。
- 优先做低风险高收益的收敛，避免在核心业务链路上做大迁移。

## 结论摘要

## Phase 1 状态

Phase 1 已完成，当前已落地并验证的替换范围如下：

- 指标导出已收敛到 Prometheus 官方客户端，并补齐对应测试覆盖。
- 限流、重试退避、断路器能力已收敛到统一基础包，供消费、任务状态回写和 Amazon API 等链路复用。
- 自动更新能力已整理出统一接入层，便于后续继续评估底层自更新实现。

2026-05-18 本地验证结果：

- 聚焦测试：`go test ./internal/infra/metrics ./internal/infra/resilience ./internal/app/consumer ./internal/app/taskstatus ./internal/amazon/api ./internal/app/updater` 通过。
- 全量安全检查：`go test ./...` 通过。
- 备注：首次全量执行因 124s 超时被中断；在延长超时窗口后复跑通过，不属于代码失败。

### 已在 Phase 1 完成的建议

1. 指标导出：已从手写 Prometheus 文本输出收敛到官方 `prometheus/client_golang`
2. 限流 / 断路器 / 重试退避：已完成第一阶段统一收敛
3. 自动更新器：已完成接入层整理，后续只剩底层替换可行性评估

### 仍建议后续评估或推进的事项

1. 自动更新器底层实现：继续评估是否替换为现成自更新库
2. AI 结果缓存层：建议保留业务键设计，引入通用缓存抽象
3. 调度器：如果长期只承担进程内周期任务，可考虑切换到 `robfig/cron`
4. 长流程状态机：如果后续继续增强提交状态机、失败恢复、幂等与跨节点续跑，优先评估 Temporal

### 不建议替换

1. SHEIN / TEMU / Amazon 平台业务规则、属性映射、提交校验
2. 现有对象存储薄封装
3. 前端基础栈

## 详细评估

### 1. 指标导出

现状：

- `internal/app/consumer/http_servers.go` 中 `/metrics` 由代码手工拼接文本输出。
- 当前实现已经覆盖较多任务指标、SHEIN 指标和系统指标。

问题：

- 指标定义、label、类型声明完全依赖手写，后续继续扩展容易漂移。
- 不利于统一 registry、histogram、默认 runtime/process 指标接入。
- 不利于和中间件、HTTP handler、业务子模块做一致封装。

建议方案：

- 使用官方库 `github.com/prometheus/client_golang`

建议替换级别：

- 高，且属于低风险改造。

推荐原因：

- 这是 Go 生态里的标准方案。
- 和现有 HTTP 服务兼容，不影响业务协议。
- 可以逐步迁移，不需要一次性改完全部指标。

参考：

- <https://github.com/prometheus/client_golang>

### 2. 限流 / 断路器 / 重试退避

现状：

- `internal/amazon/api/ratelimit.go` 中自实现了令牌桶和断路器。
- `internal/app/taskstatus/service.go`、`internal/app/consumer/result_reporter.go` 中又分别实现了不同风格的 retry/backoff。

问题：

- 同类能力分散在多个模块，语义和边界不统一。
- 自实现容易遗漏抖动、取消传播、最大等待时长、熔断恢复等细节。
- 后续如果 HTTP client、平台 client、状态回写都要复用，会继续复制逻辑。

建议方案：

- 限流：`golang.org/x/time/rate`
- 断路器：`github.com/sony/gobreaker`
- 重试退避：`github.com/cenkalti/backoff/v5`

建议替换级别：

- 高。

推荐原因：

- 都是成熟、广泛使用的基础库。
- 适合做一个统一的 `internal/infra/resilience` 包，对外暴露项目自己的薄接口。
- 可以先从 Amazon API client 和任务状态回写这两条链路开始收敛。

参考：

- <https://pkg.go.dev/golang.org/x/time/rate>
- <https://github.com/sony/gobreaker>
- <https://github.com/cenkalti/backoff>

### 3. 自动更新器

现状：

- `internal/app/updater` 下维护了版本检查、下载、文件替换、自更新调度。

问题：

- 自动更新本身不是业务壁垒。
- 自更新涉及平台兼容、替换时机、回滚、安全校验，维护成本高。

建议方案：

- 优先评估 `go-selfupdate` 生态方案。

候选：

- `github.com/sanbornm/go-selfupdate`
- `github.com/creativeprojects/go-selfupdate`

建议替换级别：

- 中高。

推荐原因：

- 模块职责单一，容易整体替换。
- 替换后可减少对下载、替换、版本检测的自维护代码。

注意事项：

- 需要确认你们当前分发方式、版本源、Windows 本地替换策略是否与目标库匹配。
- 如果现有实现已经深度耦合私有发布流程，则可只替换底层下载与版本比较部分。

参考：

- <https://github.com/sanbornm/go-selfupdate>
- <https://github.com/creativeprojects/go-selfupdate>

### 4. AI 结果缓存

现状：

- `internal/shein/aicache/cache.go` 采用 PostgreSQL 永久存储 + 进程内 `sync.Map` TTL 二级缓存。

问题：

- 当前实现能工作，但缓存抽象较薄，扩展到 Redis、多级缓存、指标、统一 codec 时会重复造基础设施。
- 后续如果更多模块需要 AI 结果缓存，当前实现不利于复用。

建议方案：

- 保留现有业务键、缓存类型、跨租户共享语义。
- 将底层缓存能力切换到通用缓存抽象库。

候选：

- `github.com/eko/gocache`
- `github.com/hypermodeinc/ristretto`

建议替换级别：

- 中。

推荐原因：

- 这块不建议“整块推翻”，而是建议把缓存存取层换掉。
- `gocache` 更适合多后端与链式缓存。
- `ristretto` 更适合高性能本地缓存。

注意事项：

- PostgreSQL 永久缓存本身不是典型缓存后端，更像持久化结果表；这一层可以保留。
- 更合理的改法是把本地内存缓存替换为成熟库，再决定是否补 Redis 层。

参考：

- <https://github.com/eko/gocache>
- <https://github.com/hypermodeinc/ristretto>

### 5. 进程内调度器

现状：

- `internal/app/scheduler` 自建了 manager、executor、dependency manager、monitor 等调度框架。
- SHEIN/TEMU 的核价、库存、活动等定时任务都挂在这一层。

问题：

- 如果需求只是“按固定周期跑任务”，当前框架偏重。
- 框架越重，后续维护越容易变成内部平台成本。

建议方案：

- 如果目标长期只是单进程/单节点周期任务，评估切换到 `github.com/robfig/cron/v3`

建议替换级别：

- 中，前提是先确认需求边界。

推荐原因：

- `robfig/cron` 足够覆盖大多数周期任务场景。
- 可以减少自研调度器维护成本。

不建议贸然替换的情况：

- 如果你们后面需要更复杂的任务依赖、分布式排他、执行观测、暂停恢复和任务动态编排，简单 cron 不够。

参考：

- <https://github.com/robfig/cron>

### 6. 长流程状态机 / 工作流编排

现状：

- 路线图中明确要增强 SHEIN 提交状态机、失败恢复、幂等保护。
- 当前分布式抓取已存在基于 RabbitMQ 的 request-reply 风格编排。

问题：

- 如果继续在现有基础上叠加“阶段可见、失败恢复、跨节点续跑、同 key 幂等”，会持续增长自研编排复杂度。
- 这类问题本质上已经接近工作流引擎场景。

建议方案：

- 如果只想保留 MQ 任务语义，可评估 `github.com/RichardKnop/machinery`
- 如果目标是 durable workflow，优先评估 Temporal

当前判断：

- 2026-05-18 起，项目在“长流程状态机 / 工作流编排”方向的首选方案明确为 Temporal。
- 原因不是它更适合替代普通异步队列，而是它更适合替代后续可能继续膨胀的“阶段持久化 + 幂等 + 恢复 + 跨节点续跑”自研编排层。
- `Machinery` 仍可作为 crawler 或普通后台异步任务的候选，但不建议承担 SHEIN 提交主链路和后续多阶段提交流程的最终编排底座。

候选：

- `github.com/RichardKnop/machinery`
- `github.com/temporalio/sdk-go`

建议替换级别：

- 中高，但这是架构级决策，不建议直接开改。

推荐原因：

- Temporal 对“长流程、补偿、可见状态、重试、幂等、跨节点恢复”更贴合。
- 比继续把 RabbitMQ 回调机制往内部工作流引擎方向演进，整体风险更可控。

注意事项：

- 这不是基础库替换，而是架构升级。
- 只建议在 SHEIN 提交状态机成为核心主链路痛点时推进。

参考：

- <https://github.com/RichardKnop/machinery>
- <https://github.com/temporalio/sdk-go>
- <https://docs.temporal.io/>
- <https://docs.temporal.io/>

## 不建议替换的模块

### 1. 平台业务规则与编排

包括但不限于：

- SHEIN 类目、属性、图片、提交校验
- TEMU 图片处理与发布规则
- Amazon 上架字段映射与 schema 适配

原因：

- 这部分是项目的业务壁垒，不存在可直接替代的通用开源实现。
- 即便有相似项目，也很难对接你们当前平台差异、店铺规则和任务模型。

### 2. 对象存储薄封装

包括：

- `internal/listingkit/upload_s3_store.go`
- `internal/listingkit/upload_local_store.go`

原因：

- 当前只是对 S3 / 本地文件的轻量适配层。
- 已经足够薄，没有必要再包一层更重的框架。

### 3. 前端基础栈

现状：

- `Next.js`、`next-auth`、`TanStack Query`、`TanStack Table`、`Radix`、`react-hook-form`、`zod`

原因：

- 这些已经是成熟选型，没有明显重复造轮子问题。
- 后续前端更多应聚焦业务页与状态流，不是换栈。

## 推荐落地顺序

建议分三层推进：

### 第一层：低风险立即收益

1. 指标导出切 Prometheus 官方库
2. 重试 / 退避 / 限流 / 断路器统一收口
3. 自动更新器评估替换

### 第二层：局部基础设施重构

1. AI 缓存层抽象收敛
2. 统一 resilience / cache 基础包

### 第三层：架构决策

1. 调度器是否继续自维护
2. SHEIN 提交状态机是否升级为工作流引擎方案

## 建议的决策方式

对每个候选项都按以下维度过一遍再开工：

- 业务差异是否足够小，适合替换
- 是否已有稳定开源实现且维护活跃
- 替换后是否减少长期维护成本
- 是否能分阶段迁移
- 是否会影响当前主链路稳定性

如果只允许先做一轮，我建议先做：

1. Prometheus 指标收敛
2. retry / rate limit / circuit breaker 收敛
3. updater 替换可行性验证

这三项最像“通用基础设施”，收益最大，业务风险最低。
