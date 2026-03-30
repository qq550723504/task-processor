# Centralized Deployment Plan

本文档描述 `task-processor` 从“按店铺分散部署”逐步演进到“中心化调度 + 少量任务节点 + 独立爬虫节点”的改造方案。

目标不是把所有任务强行收敛到 1 台机器，而是把当前“1-2 个店铺部署 1 台机器”的运行方式，改造成更容易扩缩容、运维成本更低、店铺不再绑定单机的架构。

## 1. 背景与现状

当前代码已经具备分布式运行基础：

- RabbitMQ 统一分发任务
- 爬虫任务支持异步投递并回收结果
- 店铺任务已经使用 `*.tasks.store.*` 队列做串行控制
- 调度器已经具备分布式锁入口

相关实现位置：

- [config-prod.yaml](D:/code/task-processor/config/config-prod.yaml)
- [rabbitmq_service.go](D:/code/task-processor/internal/app/consumer/rabbitmq_service.go)
- [client.go](D:/code/task-processor/internal/app/crawler/distributed/client.go)
- [queue_initializer.go](D:/code/task-processor/internal/infra/rabbitmq/queue_initializer.go)
- [manager_with_lock.go](D:/code/task-processor/internal/app/scheduler/manager_with_lock.go)

这说明当前系统并不是“程序结构上只能一店一机”，而更像是“为了规避爬虫、Cookie、浏览器资源冲突，运行策略上按店铺拆机器”。

当前集中部署的主要阻碍不在消息队列，而在运行时状态仍偏单机内存：

- `CookieManager`
- `ShopPauseManager`
- `DailyCountManager`
- 本地浏览器池
- 静态 `ownedStores` 归属

其中内存状态入口位于：

- [manager.go](D:/code/task-processor/internal/app/state/manager.go)

## 2. 改造目标

改造后的目标形态：

1. `task` 节点集中部署，负责任务消费、任务编排、调度和结果回传
2. `crawler` 节点独立部署，负责 Amazon、1688 等重浏览器任务
3. 同店铺仍然串行执行，但不再独占某一台机器
4. 节点逐步无状态化，支持弹性扩缩容
5. 节点故障后，店铺执行权限可以自动迁移

核心原则：

- 店铺串行，不等于机器独占
- 爬虫隔离，不等于整套程序隔离
- 节点无状态，才能真正中心化部署

## 3. 推荐目标架构

```text
                         +----------------------+
                         |  Java Management API |
                         +----------+-----------+
                                    |
                                    v
                          +--------------------+
                          |      RabbitMQ      |
                          +----+----------+----+
                               |          |
                +--------------+          +------------------+
                |                                            |
                v                                            v
      +----------------------+                    +----------------------+
      | Task Node Pool       |                    | Crawler Node Pool    |
      | - task consumer      |                    | - amazon crawler     |
      | - scheduler          |                    | - 1688 crawler       |
      | - result reporter    |                    | - browser pool       |
      | - task orchestration |                    | - anti-bot resources |
      +----------+-----------+                    +----------+-----------+
                 |                                           |
                 +----------------+   +----------------------+
                                  |   |
                                  v   v
                             +------------+
                             |   Redis    |
                             | runtime    |
                             | state      |
                             +------------+
                                  |
                                  v
                             +------------+
                             | PostgreSQL |
                             | persistence|
                             +------------+
```

推荐部署规模：

- `task` 节点：2 到 4 个实例
- `crawler` 节点：2 到 N 个实例，按平台或 region 扩容
- RabbitMQ：独立部署
- Redis：独立部署
- PostgreSQL：独立部署

## 3.1 当前推荐的落地方式

在现阶段代码里，最推荐优先落地的不是 “task + RabbitMQ distributed crawler”，而是：

- `task` 程序单独部署
- `amazon-crawler-api` 单独部署
- `task` 通过 HTTP 调用 crawler API 获取商品数据

这样做的原因是：

- 任务编排和浏览器资源彻底分离
- 爬虫服务可以独立扩容、重启和限流
- task 节点可以真正关闭本地浏览器依赖
- 接口边界比“进程内依赖注入 crawler 实现”更清楚

当前仓库已经提供了这两个独立入口和配置文件：

- `task`: [config-task.yaml](D:/code/task-processor/config/config-task.yaml)
- `amazon-crawler-api`: [config-amazon-crawler-api.yaml](D:/code/task-processor/config/config-amazon-crawler-api.yaml)

当前推荐链路是：

```text
task -> ProductFetcher -> RemoteAPIProductFetcher -> amazon-crawler-api
```

其中：

- 单个商品抓取统一使用 `POST /api/v1/products/fetch`
- 变体抓取不单独暴露接口，由调用方循环调用同一个商品抓取接口

## 4. 当前代码与目标架构的对应关系

### 4.1 适合保留的能力

以下能力已经接近目标架构，可以直接保留：

- RabbitMQ 作为任务总线
- 店铺专属队列模型
- 分布式爬虫客户端
- 调度器分布式锁入口
- 多入口模式
  - `cmd/rabbitmq-consumer`
  - `cmd/crawler-consumer`
  - `cmd/task`

### 4.2 需要逐步收口的部分

以下部分是集中部署的主要阻碍：

1. 运行时状态驻留进程内存
2. `ownedStores` 静态配置导致节点和店铺耦合
3. 浏览器池与任务执行进程共存，导致任务节点过重
4. Cookie、暂停状态、日限额未统一进入共享状态存储

## 5. 三阶段改造路线

## 第一阶段：拆分部署角色

目标：

- 保持现有业务逻辑基本不变
- 先把“任务节点”和“爬虫节点”拆开
- 立即降低机器数量

部署策略：

- `task` 节点只消费 `amazon.tasks.store.*`、`temu.tasks.store.*`、`shein.tasks.store.*`
- `crawler` 节点只消费 `amazon.crawler`、`1688.crawler`
- 调度任务优先放在 `task` 节点
- 浏览器相关资源只在 `crawler` 节点保留

建议新增节点角色配置：

```yaml
node:
  role: "task" # task | crawler | hybrid
```

角色说明：

- `task`: 只跑消费、调度、编排，不承担本地爬虫
- `crawler`: 只跑爬虫队列和浏览器资源
- `hybrid`: 兼容过渡期，允许一台机器同时承担两类职责

这一阶段的实现重点：

1. 按角色控制处理器注册和队列消费
2. 在部署文件中拆分 task / crawler 服务
3. 明确每种角色需要的最小配置

建议优先调整的模块：

- [rabbitmq_service.go](D:/code/task-processor/internal/app/consumer/rabbitmq_service.go)
- [platform_registry.go](D:/code/task-processor/internal/app/consumer/platform_registry.go)
- [crawler_registry.go](D:/code/task-processor/internal/app/consumer/crawler_registry.go)
- [config.go](D:/code/task-processor/internal/core/config/config.go)
- [type_rabbitmq.go](D:/code/task-processor/internal/core/config/type_rabbitmq.go)

阶段收益：

- 部署数量明显下降
- 爬虫压力与业务编排解耦
- 后续状态外移时改动边界更清楚

## 第二阶段：共享运行时状态

目标：

- 去掉“必须在同一台机器上执行才能读到状态”的假设
- 为动态调度和弹性迁移打基础

推荐把状态拆成两类。

### 5.2.1 Redis 运行时状态

适合放 Redis 的状态：

- 店铺暂停状态
- 节点租约
- 分布式锁
- 每日计数器
- 短期 Cookie 缓存
- 爬虫 pending task 映射
- 节点心跳

建议 Redis key 设计：

```text
tp:store:pause:{store_id}
tp:store:cookie:{platform}:{store_id}
tp:store:daily_count:{platform}:{store_id}:{yyyyMMdd}
tp:node:heartbeat:{node_id}
tp:store:lease:{platform}:{store_id}
tp:lock:scheduler:{platform}:{task_name}
tp:crawler:pending:{task_id}
tp:crawler:result:{task_id}
```

字段建议：

- `tp:store:pause:{store_id}`
  - `reason`
  - `until`
  - `updated_at`

- `tp:store:cookie:{platform}:{store_id}`
  - `cookie_payload`
  - `updated_at`
  - `version`

- `tp:store:lease:{platform}:{store_id}`
  - `node_id`
  - `expires_at`
  - `epoch`

TTL 建议：

- pause：按业务暂停时长设置
- cookie：按登录有效期和刷新策略设置
- heartbeat：30 秒到 60 秒
- lease：60 秒到 120 秒
- crawler pending/result：5 分钟到 30 分钟

### 5.2.2 PostgreSQL 持久状态

适合放数据库的状态：

- 店铺长期配置
- 长期授权信息
- 任务执行结果
- 调度策略配置
- 节点历史、审计和告警数据

建议原则：

- Redis 存“运行中”和“快速失效”的状态
- DB 存“需要追溯”和“需要审计”的状态

### 5.2.3 代码改造建议

先引入仓储接口，再保留内存实现作为 fallback：

- `PauseStateRepository`
- `CookieRepository`
- `DailyCounterRepository`
- `NodeLeaseRepository`

推荐改造顺序：

1. `ShopPauseManager`
2. `DailyCountManager`
3. `CookieManager`
4. crawler pending / result registry

对应代码入口：

- [manager.go](D:/code/task-processor/internal/app/state/manager.go)
- [cookie_manager.go](D:/code/task-processor/internal/app/state/cookie_manager.go)
- [shop_pause_manager.go](D:/code/task-processor/internal/app/state/shop_pause_manager.go)
- [daily_count_manager.go](D:/code/task-processor/internal/app/state/daily_count_manager.go)
- [pending_registry.go](D:/code/task-processor/internal/app/crawler/distributed/pending_registry.go)

## 第三阶段：动态店铺租约与无状态节点

目标：

- 用动态租约替代静态 `ownedStores`
- 节点不再提前写死负责哪些店铺
- 故障自动漂移，扩容自动接管

推荐模型：

1. 节点启动后注册 `heartbeat`
2. 节点按平台或 region 领取店铺租约
3. 同一店铺在任一时刻只允许一个活跃 lease
4. lease 到期后自动被其他节点接管

建议保留兼容模式：

- `ownedStores` 继续保留一段时间
- 当 `lease.enabled=false` 时，仍走当前静态配置
- 当 `lease.enabled=true` 时，优先使用动态租约

建议配置草案：

```yaml
node:
  role: "task"
  nodeID: "task-node-1"

state:
  backend: "redis" # memory | redis

lease:
  enabled: true
  ttl: 90s
  heartbeatInterval: 30s
  rebalanceInterval: 60s
  maxStoresPerNode: 50
```

建议租约规则：

- 一个店铺只允许一个活跃 `lease`
- lease 使用原子 `SET NX EX` 或 Lua 脚本续约
- 节点停止续约后，租约自动过期
- 调度、消费和店铺串行控制都以 lease 为准

## 6. 需要修改的模块清单

建议按影响范围拆成 4 组任务。

### 6.1 配置与装配层

- [config.go](D:/code/task-processor/internal/core/config/config.go)
- [type_rabbitmq.go](D:/code/task-processor/internal/core/config/type_rabbitmq.go)
- [type_worker.go](D:/code/task-processor/internal/core/config/type_worker.go)
- [bootstrap/app.go](D:/code/task-processor/internal/app/bootstrap/app.go)
- [shared_resources.go](D:/code/task-processor/internal/app/bootstrap/shared_resources.go)

目标：

- 新增节点角色配置
- 新增状态后端配置
- 新增租约配置
- 根据角色决定是否装配 crawler / scheduler / consumer

### 6.2 consumer 运行时

- [rabbitmq_service.go](D:/code/task-processor/internal/app/consumer/rabbitmq_service.go)
- [service_manager.go](D:/code/task-processor/internal/app/consumer/service_manager.go)
- [processor_registry.go](D:/code/task-processor/internal/app/consumer/processor_registry.go)
- [crawler_registry.go](D:/code/task-processor/internal/app/consumer/crawler_registry.go)

目标：

- 不同角色只注册需要的消费者
- 不同角色只声明和消费对应队列
- 为过渡期保留 `hybrid` 模式

### 6.3 state 层

- [manager.go](D:/code/task-processor/internal/app/state/manager.go)
- [cookie_manager.go](D:/code/task-processor/internal/app/state/cookie_manager.go)
- [shop_pause_manager.go](D:/code/task-processor/internal/app/state/shop_pause_manager.go)
- [daily_count_manager.go](D:/code/task-processor/internal/app/state/daily_count_manager.go)

目标：

- 增加 repository 抽象
- 增加 Redis 实现
- 保留 memory 实现作为 fallback

### 6.4 scheduler 与店铺归属

- [manager_with_lock.go](D:/code/task-processor/internal/app/scheduler/manager_with_lock.go)
- [task_dispatch_guard.go](D:/code/task-processor/internal/app/task/task_dispatch_guard.go)
- [task_dispatcher.go](D:/code/task-processor/internal/app/task/task_dispatcher.go)

目标：

- 把“是否允许处理某店铺”从静态配置改为 lease 判断
- 调度任务也受统一锁与租约约束

## 7. 分阶段上线策略

建议按下面顺序上线。

### 阶段 A：角色拆分，不改状态

动作：

- 上线 `task` / `crawler` / `hybrid` 三种角色
- 保持 `ownedStores` 不变
- 保持内存状态不变

验收标准：

- `task` 节点不再跑本地重爬虫
- `crawler` 节点能独立承担爬虫任务
- 整体机器数量开始下降

### 阶段 B：暂停状态与日计数迁移到 Redis

动作：

- `ShopPauseManager` 改为 Redis
- `DailyCountManager` 改为 Redis

验收标准：

- 节点切换后暂停状态不丢失
- 日计数不再依赖单机

### 阶段 C：Cookie 和 crawler pending 状态迁移

动作：

- Cookie 改为共享状态
- distributed crawler pending/result 改为共享状态或可恢复状态

验收标准：

- task 节点和 crawler 节点故障恢复能力提升
- 节点重启后状态恢复更平滑

### 阶段 D：引入动态租约

动作：

- 增加 `lease.enabled`
- 节点按租约自动接管店铺
- 逐步废弃静态 `ownedStores`

验收标准：

- 新增节点无需手工分配店铺
- 节点故障后店铺自动漂移

## 8. 风险与注意事项

### 8.1 不建议直接“一台中心机”

虽然目标是“集中部署”，但不建议把所有任务收敛到 1 台机器，原因包括：

- 浏览器任务波动大
- Cookie/风控问题会放大影响面
- 单点故障风险高
- 任务高峰期很难平滑扩容

因此更合理的是：

- 少量 `task` 节点集中部署
- `crawler` 节点按负载弹性扩容

### 8.2 Cookie 共享要注意版本冲突

当多个节点可能刷新同一店铺 Cookie 时，需要避免覆盖新值：

- 建议加入 `version` 或 `updated_at`
- 写入时做 compare-and-set 或乐观锁判断

### 8.3 lease 与店铺串行控制不要重复打架

当前已经有店铺队列串行语义，后续引入 lease 时，要避免重复约束导致任务无法消费：

- 队列负责“顺序”
- lease 负责“归属”
- 调度锁负责“分布式调度互斥”

三者角色应明确分离。

## 9. 推荐近期执行顺序

如果需要一条最稳的落地路径，建议优先做以下事项：

1. 拆 `task` / `crawler` 角色部署
2. 给配置层新增 `node.role`
3. 将 `ShopPauseManager` 迁到 Redis
4. 将 `DailyCountManager` 迁到 Redis
5. 最后再评估 Cookie 与 lease

这条路径可以先把机器数量降下来，再逐步去掉单机依赖。

## 10. 结论

`task-processor` 已经具备向中心化部署演进的结构基础，当前真正需要解决的不是“消息队列怎么分发”，而是“运行时状态如何共享、店铺归属如何动态化”。

推荐路线不是“所有店铺集中到一台”，而是：

- 任务层集中部署
- 爬虫层独立扩容
- 状态外移到 Redis / DB
- 店铺归属从静态配置演进到动态租约

这样可以在不推翻现有代码主结构的前提下，把部署模式从“按店铺堆机器”平滑迁移到“少量中心节点 + 弹性爬虫池”。
